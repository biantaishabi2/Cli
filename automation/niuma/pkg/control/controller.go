// pkg/control/controller.go
// Controller 核心：多 Issue 协调循环
package control

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// GitHubOps 控制层需要的 GitHub 操作接口（独立于 agent 包的接口）
type GitHubOps interface {
	ListIssuesWithLabel(ctx context.Context, label string) ([]IssueInfo, error)
	MergePR(ctx context.Context, prNum int, method string) error
	ReplaceLabel(ctx context.Context, issueNumber int, oldLabel, newLabel string) error
}

// Controller 多 Issue 协调控制器
type Controller struct {
	taskctl  *TaskCtlClient
	analyzer *DependencyAnalyzer
	github   GitHubOps
	builder  *IntegrationBuilder
	cfg      *ControlConfig
}

// ControlConfig 控制层配置
type ControlConfig struct {
	TaskCtlBin              string `yaml:"taskctl_bin"`
	MergeStrategy           string `yaml:"merge_strategy"`            // merge/squash，默认 merge
	IntegrationBranchPrefix string `yaml:"integration_branch_prefix"` // 默认 integration/
	MaxOldBranches          int    `yaml:"max_old_branches"`          // 默认 3
	MinPRsForIntegration    int    `yaml:"min_prs_for_integration"`   // 默认 2
}

// DefaultControlConfig 返回默认配置
func DefaultControlConfig() *ControlConfig {
	return &ControlConfig{
		MergeStrategy:           "merge",
		IntegrationBranchPrefix: "integration/",
		MaxOldBranches:          3,
		MinPRsForIntegration:    2,
	}
}

// NewController 创建 Controller
func NewController(
	taskctl *TaskCtlClient,
	analyzer *DependencyAnalyzer,
	github GitHubOps,
	builder *IntegrationBuilder,
	cfg *ControlConfig,
) *Controller {
	if cfg == nil {
		cfg = DefaultControlConfig()
	}
	return &Controller{
		taskctl:  taskctl,
		analyzer: analyzer,
		github:   github,
		builder:  builder,
		cfg:      cfg,
	}
}

// getIntegrationBranchName 获取当前任务的 integration 分支名
// 从 task metadata 读取 meta_issue_slug，没有则使用 "main"
func (c *Controller) getIntegrationBranchName(task *Task) string {
	slug := ""
	if task.Metadata != nil {
		slug = task.Metadata["meta_issue_slug"]
	}
	return IntegrationBranchName(slug)
}

// Run 执行一次完整协调循环
func (c *Controller) Run(ctx context.Context) error {
	// ① 扫描 GitHub：ListIssuesWithLabel("bot:orchestrate") 和 ListIssuesWithLabel("bot:fix")
	orchestrateIssues, err := c.github.ListIssuesWithLabel(ctx, "bot:orchestrate")
	if err != nil {
		return fmt.Errorf("扫描 bot:orchestrate issues 失败: %w", err)
	}

	fixIssues, err := c.github.ListIssuesWithLabel(ctx, "bot:fix")
	if err != nil {
		return fmt.Errorf("扫描 bot:fix issues 失败: %w", err)
	}

	// 合并所有需要处理的 issues
	allIssues := make(map[int]IssueInfo)
	for _, issue := range orchestrateIssues {
		allIssues[issue.Number] = issue
	}
	for _, issue := range fixIssues {
		allIssues[issue.Number] = issue
	}

	if len(allIssues) == 0 {
		fmt.Println("[control] 没有发现 bot:orchestrate 或 bot:fix issues")
		return nil
	}

	// 转换为 slice
	var issues []IssueInfo
	for _, issue := range allIssues {
		issues = append(issues, issue)
	}

	fmt.Printf("[control] 发现 %d 个 issues (bot:orchestrate: %d, bot:fix: %d)\n",
		len(issues), len(orchestrateIssues), len(fixIssues))

	// ② 对比 taskctl store：找出新 issue
	existingTasks, err := c.taskctl.List("")
	if err != nil {
		// 区分 store 不存在（首次运行）和真实错误
		if strings.Contains(err.Error(), "no such file") || strings.Contains(err.Error(), "not found") {
			existingTasks = nil
		} else {
			return fmt.Errorf("列出现有任务失败: %w", err)
		}
	}

	existingIssues := make(map[int]string) // issueNum → taskID
	for _, t := range existingTasks {
		if n := t.IssueNum(); n > 0 {
			existingIssues[n] = t.ID
		}
	}

	var newIssues []IssueInfo
	for _, issue := range issues {
		if _, ok := existingIssues[issue.Number]; !ok {
			newIssues = append(newIssues, issue)
		}
	}

	// ③ 分析依赖（包括 depends-on 和 parent 关系）
	analysis := &AnalysisResult{Dependencies: make(map[int][]int)}

	// 先解析 parent 关系（Sub-Issue 模式）
	for _, issue := range issues {
		if parentNum := parseParent(issue.Body); parentNum > 0 {
			// 检查 parent 是否也在处理列表中
			for _, other := range issues {
				if other.Number == parentNum {
					// sub-issue 依赖 parent
					analysis.Dependencies[issue.Number] = append(analysis.Dependencies[issue.Number], parentNum)
					fmt.Printf("[control] 发现 Sub-Issue 关系: #%d → parent #%d\n", issue.Number, parentNum)
					break
				}
			}
		}
	}

	// 再调 AI 分析（补充 depends-on 和其他依赖）
	if len(issues) > 1 && c.analyzer != nil {
		aiAnalysis, err := c.analyzer.Analyze(ctx, issues)
		if err != nil {
			fmt.Printf("[control] AI 依赖分析失败: %v\n", err)
		} else {
			// 合并 AI 结果（parent 关系优先）
			for issueNum, deps := range aiAnalysis.Dependencies {
				if _, hasParent := analysis.Dependencies[issueNum]; !hasParent {
					analysis.Dependencies[issueNum] = deps
				}
			}
		}
	}

	// ④ 为新 issue 创建 task + 设 blocked_by
	issueToTask := make(map[int]string) // issueNum → taskID
	for k, v := range existingIssues {
		issueToTask[k] = v
	}

	for _, issue := range newIssues {
		meta := map[string]string{
			"issue_num": strconv.Itoa(issue.Number),
		}
		task, err := c.taskctl.Create(issue.Title, issue.Body, meta)
		if err != nil {
			fmt.Printf("[control] 创建任务失败 (issue #%d): %v\n", issue.Number, err)
			continue
		}
		issueToTask[issue.Number] = task.ID
		fmt.Printf("[control] 创建任务 %s (issue #%d)\n", task.ID, issue.Number)

		// 如果 issue 有 bot:orchestrate 标签，替换为 bot:queued
		if hasLabel(issue.Labels, "bot:orchestrate") {
			if err := c.github.ReplaceLabel(ctx, issue.Number, "bot:orchestrate", "bot:queued"); err != nil {
				fmt.Printf("[control] 替换标签失败 (issue #%d): %v\n", issue.Number, err)
			} else {
				fmt.Printf("[control] 已将 issue #%d 标签 bot:orchestrate → bot:queued\n", issue.Number)
			}
		}
	}

	// 设置 blocked_by
	for issueNum, deps := range analysis.Dependencies {
		taskID, ok := issueToTask[issueNum]
		if !ok {
			continue
		}
		var blockedBy []string
		for _, dep := range deps {
			if depTaskID, ok := issueToTask[dep]; ok {
				blockedBy = append(blockedBy, depTaskID)
			}
		}
		if len(blockedBy) > 0 {
			if err := c.taskctl.Update(taskID, UpdateOpts{BlockedBy: &blockedBy}); err != nil {
				fmt.Printf("[control] 设置 blocked_by 失败 (task %s): %v\n", taskID, err)
			}
		}
	}

	// ⑤ 获取 ready tasks 并推进
	readyTasks, err := c.taskctl.Ready()
	if err != nil {
		fmt.Printf("[control] 获取 ready tasks 失败: %v\n", err)
	} else {
		for _, task := range readyTasks {
			issueNum := task.IssueNum()
			fmt.Printf("[control] 推进 ready task %s (issue #%d)\n", task.ID, issueNum)
			status := TaskStatusInProgress
			if err := c.taskctl.Update(task.ID, UpdateOpts{Status: &status}); err != nil {
				fmt.Printf("[control] 更新任务状态失败 (task %s): %v\n", task.ID, err)
				continue
			}
			// 将 bot:queued 改为 bot:fix，触发单issue流程
			if issueNum > 0 {
				if err := c.github.ReplaceLabel(ctx, issueNum, "bot:queued", "bot:fix"); err != nil {
					fmt.Printf("[control] 替换标签失败 (issue #%d): %v\n", issueNum, err)
				} else {
					fmt.Printf("[control] 已将 issue #%d 标签 bot:queued → bot:fix\n", issueNum)
				}
			}
		}
	}

	// ⑥ 增量 integration：将刚完成的 PR 合入对应 integration 分支
	if c.builder != nil {
		allTasks, _ := c.taskctl.List("")

		// 按 integration 分支分组 task
		branchTasks := make(map[string][]Task) // integrationBranch → tasks
		for _, t := range allTasks {
			if t.PRNum() > 0 && t.Branch() != "" {
				// 检查是否已合入 integration（从 metadata 读）
				integrated := false
				if t.Metadata != nil && t.Metadata["integrated"] == "true" {
					integrated = true
				}
				if !integrated {
					branchName := c.getIntegrationBranchName(&t)
					branchTasks[branchName] = append(branchTasks[branchName], t)
				}
			}
		}

		// 对每个 integration 分支，合入未集成的 PR
		for branchName, tasks := range branchTasks {
			if len(tasks) == 0 {
				continue
			}

			fmt.Printf("[control] Integration 分支 %s: 有 %d 个 PR 待合入\n", branchName, len(tasks))

			for _, task := range tasks {
				bi := BranchInfo{
					Branch:   task.Branch(),
					IssueNum: task.IssueNum(),
					PRNum:    task.PRNum(),
					TaskID:   task.ID,
				}

				if err := c.builder.MergeOne(branchName, bi); err != nil {
					fmt.Printf("[control] 合入 %s 失败: %v\n", bi.Branch, err)
					// 冲突时跳过，继续下一个
					continue
				}

				// 标记为已集成
				metaUpdate := map[string]string{"integrated": "true"}
				if err := c.taskctl.Update(task.ID, UpdateOpts{Metadata: &metaUpdate}); err != nil {
					fmt.Printf("[control] 标记 integrated 失败 (task %s): %v\n", task.ID, err)
				}

				fmt.Printf("[control] 已合入 %s (issue #%d) 到 %s\n", bi.Branch, bi.IssueNum, branchName)
			}
		}
	}

	// ⑦ 检查父 issue 进度（Sub-Issue 模式）
	c.checkParentProgress(ctx, issues)

	return nil
}

// checkParentProgress 检查父 issue 的所有 sub-issues 是否完成
func (c *Controller) checkParentProgress(ctx context.Context, issues []IssueInfo) {
	// 构建 parent → sub-issues 映射
	parentToSubs := make(map[int][]int) // parentNum → []subNum
	for _, issue := range issues {
		if parentNum := parseParent(issue.Body); parentNum > 0 {
			parentToSubs[parentNum] = append(parentToSubs[parentNum], issue.Number)
		}
	}

	if len(parentToSubs) == 0 {
		return
	}

	// 获取所有 task 状态
	tasks, err := c.taskctl.List("")
	if err != nil {
		return
	}

	taskStatus := make(map[int]TaskStatus)
	for _, t := range tasks {
		if n := t.IssueNum(); n > 0 {
			taskStatus[n] = t.Status
		}
	}

	// 检查每个 parent 的 sub-issues 是否都完成
	for parentNum, subNums := range parentToSubs {
		allCompleted := true
		for _, subNum := range subNums {
			if status, ok := taskStatus[subNum]; !ok || status != TaskStatusCompleted {
				allCompleted = false
				break
			}
		}

		if allCompleted {
			fmt.Printf("[control] 父 issue #%d 的所有 sub-issues 已完成: %v\n", parentNum, subNums)
			// TODO: 可以在这里关闭父 issue 或添加评论
		}
	}
}

// Status 返回全局控制状态
func (c *Controller) Status(ctx context.Context) (*ControlStatus, error) {
	tasks, err := c.taskctl.List("")
	if err != nil {
		return nil, fmt.Errorf("列出任务失败: %w", err)
	}

	dag, err := c.taskctl.Dag()
	if err != nil {
		// DAG 不可用时不阻塞
		dag = &DagGraph{}
	}

	return &ControlStatus{
		Dag:   dag,
		Tasks: tasks,
	}, nil
}

// Merge 将 integration 分支合并到 master
// 人工批准后执行：integration/{slug} → master
func (c *Controller) Merge(ctx context.Context, integrationBranch string) error {
	if c.builder == nil {
		return fmt.Errorf("IntegrationBuilder 未配置")
	}

	// 使用 GitHub API 创建 PR 或直接 merge
	// 这里简化处理：直接 fast-forward merge
	fmt.Printf("[control] 合并 %s 到 master...\n", integrationBranch)

	// 实际实现需要调用 GitHub API 或 git 命令
	// 这里只是一个占位，具体实现取决于 GitHubOps 接口的扩展

	return fmt.Errorf("Merge 实现待完成：需要扩展 GitHubOps 接口支持分支合并")
}

// FormatStatus 格式化输出控制状态
func FormatStatus(status *ControlStatus) string {
	var sb strings.Builder
	sb.WriteString("## 控制状态\n\n")

	if len(status.Dag.Nodes) > 0 {
		sb.WriteString(fmt.Sprintf("DAG: %d 个节点\n", len(status.Dag.Nodes)))
	}

	sb.WriteString(fmt.Sprintf("任务总数: %d\n\n", len(status.Tasks)))

	if len(status.Tasks) > 0 {
		sb.WriteString("| Task ID | Issue | 状态 | 阻塞于 |\n")
		sb.WriteString("|---------|-------|------|--------|\n")
		for _, t := range status.Tasks {
			blockedBy := strings.Join(t.BlockedBy, ", ")
			if blockedBy == "" {
				blockedBy = "-"
			}
			sb.WriteString(fmt.Sprintf("| %s | #%d | %s | %s |\n",
				t.ID, t.IssueNum(), t.Status, blockedBy))
		}
	}

	return sb.String()
}

// hasLabel 检查 label 列表中是否包含指定 label
func hasLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}
