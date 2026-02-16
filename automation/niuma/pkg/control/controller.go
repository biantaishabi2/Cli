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
	IntegrationBranchPrefix string `yaml:"integration_branch_prefix"` // 默认 integration/batch-
	MaxOldBranches          int    `yaml:"max_old_branches"`          // 默认 3
	MinPRsForIntegration    int    `yaml:"min_prs_for_integration"`   // 默认 2
}

// DefaultControlConfig 返回默认配置
func DefaultControlConfig() *ControlConfig {
	return &ControlConfig{
		MergeStrategy:           "merge",
		IntegrationBranchPrefix: "integration/batch-",
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

// Run 执行一次完整协调循环
func (c *Controller) Run(ctx context.Context) error {
	// ① 扫描 GitHub：ListIssuesWithLabel("bot:fix")
	issues, err := c.github.ListIssuesWithLabel(ctx, "bot:fix")
	if err != nil {
		return fmt.Errorf("扫描 GitHub issues 失败: %w", err)
	}

	if len(issues) == 0 {
		fmt.Println("[control] 没有发现 bot:fix issues")
		return nil
	}

	fmt.Printf("[control] 发现 %d 个 bot:fix issues\n", len(issues))

	// ② 对比 taskctl store：找出新 issue
	existingTasks, err := c.taskctl.List("")
	if err != nil {
		// store 不存在时可能报错，忽略
		existingTasks = nil
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

	// ③ AI 分析依赖
	var analysis *AnalysisResult
	if len(issues) > 1 {
		analysis, err = c.analyzer.Analyze(ctx, issues)
		if err != nil {
			fmt.Printf("[control] 依赖分析失败: %v\n", err)
			analysis = &AnalysisResult{Dependencies: make(map[int][]int)}
		}
	} else {
		analysis = &AnalysisResult{Dependencies: make(map[int][]int)}
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
			fmt.Printf("[control] 推进 ready task %s (issue #%d)\n", task.ID, task.IssueNum())
			status := TaskStatusInProgress
			if err := c.taskctl.Update(task.ID, UpdateOpts{Status: &status}); err != nil {
				fmt.Printf("[control] 更新任务状态失败 (task %s): %v\n", task.ID, err)
			}
		}
	}

	// ⑥ 检查是否需要构建 integration 分支
	if c.builder != nil {
		allTasks, _ := c.taskctl.List("")
		var prBranches []BranchInfo
		for _, t := range allTasks {
			if t.PRNum() > 0 && t.Branch() != "" {
				prBranches = append(prBranches, BranchInfo{
					Branch:   t.Branch(),
					IssueNum: t.IssueNum(),
					PRNum:    t.PRNum(),
					TaskID:   t.ID,
				})
			}
		}

		minPRs := c.cfg.MinPRsForIntegration
		if minPRs <= 0 {
			minPRs = 2
		}
		if len(prBranches) >= minPRs {
			fmt.Printf("[control] 有 %d 个 PR，开始构建 integration 分支\n", len(prBranches))
			result, err := c.builder.Build(prBranches, analysis.Dependencies)
			if err != nil {
				fmt.Printf("[control] 构建 integration 分支失败: %v\n", err)
			} else {
				fmt.Printf("[control] Integration 分支 %s: merged=%v conflicts=%v skipped=%v\n",
					result.Branch, result.Merged, result.Conflicts, result.Skipped)
			}

			// 清理旧 integration 分支
			_ = c.builder.CleanOld()
		}
	}

	return nil
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

// Merge 按 topo 序逐个合并 PR
func (c *Controller) Merge(ctx context.Context, issueNums []int) error {
	// 获取所有任务
	allTasks, err := c.taskctl.List("")
	if err != nil {
		return fmt.Errorf("列出任务失败: %w", err)
	}

	// 建立 issueNum → task 映射
	taskMap := make(map[int]*Task)
	for i := range allTasks {
		n := allTasks[i].IssueNum()
		if n > 0 {
			taskMap[n] = &allTasks[i]
		}
	}

	// 按 topo 序排列（简单实现：有依赖的排后面）
	sorted := topoSort(issueNums, taskMap)

	method := c.cfg.MergeStrategy
	if method == "" {
		method = "merge"
	}

	for _, issueNum := range sorted {
		task, ok := taskMap[issueNum]
		if !ok {
			return fmt.Errorf("issue #%d 没有对应的 task", issueNum)
		}
		prNum := task.PRNum()
		if prNum == 0 {
			return fmt.Errorf("issue #%d 的 task 没有关联 PR", issueNum)
		}

		fmt.Printf("[control] 合并 PR #%d (issue #%d)...\n", prNum, issueNum)
		if err := c.github.MergePR(ctx, prNum, method); err != nil {
			return fmt.Errorf("合并 PR #%d (issue #%d) 失败: %w", prNum, issueNum, err)
		}

		// 更新任务状态
		status := TaskStatusCompleted
		if err := c.taskctl.Update(task.ID, UpdateOpts{Status: &status}); err != nil {
			fmt.Printf("[control] 更新任务状态失败 (task %s): %v\n", task.ID, err)
		}

		fmt.Printf("[control] PR #%d 合并成功\n", prNum)
	}

	return nil
}

// topoSort 对 issue 编号做简单拓扑排序
func topoSort(issueNums []int, taskMap map[int]*Task) []int {
	// 构建依赖图
	issueSet := make(map[int]bool)
	for _, n := range issueNums {
		issueSet[n] = true
	}

	// 从 task 的 blocked_by 推导依赖
	deps := make(map[int][]int) // issue → 依赖的 issues
	taskIDToIssue := make(map[string]int)
	for issueNum, task := range taskMap {
		taskIDToIssue[task.ID] = issueNum
	}

	for issueNum, task := range taskMap {
		if !issueSet[issueNum] {
			continue
		}
		for _, depID := range task.BlockedBy {
			if depIssue, ok := taskIDToIssue[depID]; ok && issueSet[depIssue] {
				deps[issueNum] = append(deps[issueNum], depIssue)
			}
		}
	}

	// Kahn 算法
	inDegree := make(map[int]int)
	for _, n := range issueNums {
		inDegree[n] = 0
	}
	for n, d := range deps {
		inDegree[n] = len(d)
		_ = d // 确保所有被依赖的也在 map 中
	}

	var queue []int
	for _, n := range issueNums {
		if inDegree[n] == 0 {
			queue = append(queue, n)
		}
	}

	var result []int
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		result = append(result, n)

		// 找所有依赖 n 的
		for issueNum, depList := range deps {
			for _, dep := range depList {
				if dep == n {
					inDegree[issueNum]--
					if inDegree[issueNum] == 0 {
						queue = append(queue, issueNum)
					}
				}
			}
		}
	}

	// 没被排序到的追加到末尾（循环依赖情况）
	sorted := make(map[int]bool)
	for _, n := range result {
		sorted[n] = true
	}
	for _, n := range issueNums {
		if !sorted[n] {
			result = append(result, n)
		}
	}

	return result
}

// FormatStatus 格式化状态输出
func FormatStatus(status *ControlStatus) string {
	var sb strings.Builder

	sb.WriteString("=== 任务列表 ===\n")
	sb.WriteString(fmt.Sprintf("%-8s %-12s %-30s %-10s %-10s\n", "Issue", "Status", "Subject", "PR", "Deps"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	for _, task := range status.Tasks {
		issueStr := fmt.Sprintf("#%d", task.IssueNum())
		prStr := "-"
		if task.PRNum() > 0 {
			prStr = fmt.Sprintf("#%d", task.PRNum())
		}
		depsStr := "-"
		if len(task.BlockedBy) > 0 {
			depsStr = strings.Join(task.BlockedBy, ",")
		}

		// 截断 subject
		subject := task.Subject
		if len(subject) > 28 {
			subject = subject[:28] + ".."
		}

		sb.WriteString(fmt.Sprintf("%-8s %-12s %-30s %-10s %-10s\n",
			issueStr, task.Status, subject, prStr, depsStr))
	}

	if status.Integration != nil {
		sb.WriteString(fmt.Sprintf("\n=== Integration 分支: %s ===\n", status.Integration.Branch))
		sb.WriteString(fmt.Sprintf("Merged: %v\n", status.Integration.Merged))
		if len(status.Integration.Conflicts) > 0 {
			sb.WriteString(fmt.Sprintf("Conflicts: %v\n", status.Integration.Conflicts))
		}
		if len(status.Integration.Skipped) > 0 {
			sb.WriteString(fmt.Sprintf("Skipped: %v\n", status.Integration.Skipped))
		}
	}

	return sb.String()
}
