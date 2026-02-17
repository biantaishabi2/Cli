// cmd/niuma/control.go
// control 子命令：多 Issue 协调
package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/config"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/spf13/cobra"
)

var controlCmd = &cobra.Command{
	Use:   "control",
	Short: "多 Issue 协调控制（扫描 → 分析依赖 → DAG → 推进 → integration）",
}

var controlRunCmd = &cobra.Command{
	Use:   "run",
	Short: "执行一次完整协调循环",
	RunE:  runControlRun,
}

var controlStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看全局状态（DAG + 各 task 进度）",
	RunE:  runControlStatus,
}

var controlMergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "按 topo 序合并 PR",
	RunE:  runControlMerge,
}

var controlCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "检查 issue 是否在编排队列中",
	RunE:  runControlCheck,
}

var controlCloseMergedCmd = &cobra.Command{
	Use:   "close-merged",
	Short: "integration PR 合并后自动收口（关闭 sub/parent issue）",
	RunE:  runControlCloseMerged,
}

var flagControlIssues string // --issues "40,41,42"
var flagIntegrationGateMaxRetries int

func init() {
	controlCmd.AddCommand(controlRunCmd)
	controlCmd.AddCommand(controlStatusCmd)
	controlCmd.AddCommand(controlMergeCmd)
	controlCmd.AddCommand(controlCheckCmd)
	controlCmd.AddCommand(controlCloseMergedCmd)

	controlMergeCmd.Flags().StringVar(&flagControlIssues, "issues", "", "要合并的 issue 编号列表（逗号分隔）")
	controlMergeCmd.MarkFlagRequired("issues")
	controlRunCmd.Flags().IntVar(&flagIntegrationGateMaxRetries, "integration-gate-max-retries", -1, "integration gate 最大重试次数（flag > env > default=2）")
	controlCloseMergedCmd.MarkFlagRequired("pr")
}

func buildController() (*control.Controller, error) {
	if flagRepo == "" {
		return nil, fmt.Errorf("必须指定 --repo")
	}

	// 确定 repo 目录
	repoDir := flagWorkDir
	if flagRepoDir != "" {
		repoDir = flagRepoDir
	}

	// 加载配置
	configDir := "."
	if flagRepoDir != "" {
		configDir = flagRepoDir
	}
	cfg := config.LoadWithDefaults(configDir)

	// 创建 GitHub 客户端
	ghClient, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return nil, err
	}

	// 创建 taskctl client
	taskctlBin := ""
	if cfg.Control.TaskCtlBin != "" {
		taskctlBin = cfg.Control.TaskCtlBin
	}
	taskctl, err := control.NewTaskCtlClient(taskctlBin, repoDir)
	if err != nil {
		return nil, fmt.Errorf("初始化 taskctl 失败: %w", err)
	}

	// 构建 provider
	providers, err := resolveProviders(cfg)
	if err != nil {
		return nil, err
	}
	defaultProvider := providers[cfg.AI.Default]

	// 创建各组件
	analyzer := control.NewDependencyAnalyzer(defaultProvider)

	controlCfg := &control.ControlConfig{
		MergeStrategy:             cfg.Control.MergeStrategy,
		IntegrationBranchPrefix:   cfg.Control.IntegrationBranchPrefix,
		MaxOldBranches:            cfg.Control.MaxOldBranches,
		MinPRsForIntegration:      cfg.Control.MinPRsForIntegration,
		IntegrationGateMaxRetries: resolveIntegrationGateMaxRetries(flagIntegrationGateMaxRetries, os.Getenv("NIUMA_INTEGRATION_GATE_MAX_RETRIES")),
		RepoDir:                   repoDir,
	}

	builder := control.NewIntegrationBuilder(
		repoDir,
		"master",
	)

	ghOps := &gitHubControlOps{client: ghClient}

	return control.NewController(taskctl, analyzer, ghOps, builder, controlCfg), nil
}

func resolveIntegrationGateMaxRetries(flagValue int, envValue string) int {
	defaultValue := 2
	if parsed, err := strconv.Atoi(strings.TrimSpace(envValue)); err == nil && parsed >= 0 {
		defaultValue = parsed
	}
	if flagValue >= 0 {
		return flagValue
	}
	return defaultValue
}

// gitHubControlOps 适配 gh.Client 到 control.GitHubOps
type gitHubControlOps struct {
	client *gh.Client
}

func (g *gitHubControlOps) ListIssuesWithLabel(ctx context.Context, label string) ([]control.IssueInfo, error) {
	issues, err := g.client.ListIssuesWithLabel(ctx, label)
	if err != nil {
		return nil, err
	}

	var result []control.IssueInfo
	for _, issue := range issues {
		var labels []string
		for _, l := range issue.Labels {
			labels = append(labels, l.GetName())
		}
		result = append(result, control.IssueInfo{
			Number: issue.GetNumber(),
			Title:  issue.GetTitle(),
			Body:   issue.GetBody(),
			State:  issue.GetState(),
			Labels: labels,
		})
	}
	return result, nil
}

func (g *gitHubControlOps) ListIssuesByState(ctx context.Context, state string) ([]control.IssueInfo, error) {
	issues, err := g.client.ListIssuesByState(ctx, state)
	if err != nil {
		return nil, err
	}

	var result []control.IssueInfo
	for _, issue := range issues {
		var labels []string
		for _, l := range issue.Labels {
			labels = append(labels, l.GetName())
		}
		result = append(result, control.IssueInfo{
			Number: issue.GetNumber(),
			Title:  issue.GetTitle(),
			Body:   issue.GetBody(),
			State:  issue.GetState(),
			Labels: labels,
		})
	}
	return result, nil
}

func (g *gitHubControlOps) GetIssue(ctx context.Context, issueNumber int) (control.IssueInfo, error) {
	issue, err := g.client.GetIssue(ctx, issueNumber)
	if err != nil {
		return control.IssueInfo{}, err
	}
	var labels []string
	for _, l := range issue.Labels {
		labels = append(labels, l.GetName())
	}
	return control.IssueInfo{
		Number: issue.GetNumber(),
		Title:  issue.GetTitle(),
		Body:   issue.GetBody(),
		State:  issue.GetState(),
		Labels: labels,
	}, nil
}

func (g *gitHubControlOps) CloseIssue(ctx context.Context, issueNumber int) error {
	return g.client.CloseIssue(ctx, issueNumber)
}

func (g *gitHubControlOps) MergePR(ctx context.Context, prNum int, method string) error {
	return g.client.MergePR(ctx, prNum, method)
}

func (g *gitHubControlOps) ReplaceLabel(ctx context.Context, issueNumber int, oldLabel, newLabel string) error {
	return g.client.ReplaceLabel(ctx, issueNumber, oldLabel, newLabel)
}

func runControlRun(cmd *cobra.Command, args []string) error {
	ctrl, err := buildController()
	if err != nil {
		return err
	}

	ctx := context.Background()
	fmt.Println("开始协调循环...")
	if err := ctrl.Run(ctx); err != nil {
		return fmt.Errorf("协调循环失败: %w", err)
	}

	fmt.Println("协调循环完成。")
	return nil
}

func runControlStatus(cmd *cobra.Command, args []string) error {
	ctrl, err := buildController()
	if err != nil {
		return err
	}

	ctx := context.Background()
	status, err := ctrl.Status(ctx)
	if err != nil {
		return fmt.Errorf("获取状态失败: %w", err)
	}

	fmt.Print(control.FormatStatus(status))
	return nil
}

func runControlMerge(cmd *cobra.Command, args []string) error {
	ctrl, err := buildController()
	if err != nil {
		return err
	}

	// 解析 issue 编号
	parts := strings.Split(flagControlIssues, ",")
	var issueNums []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return fmt.Errorf("无效的 issue 编号: %s", p)
		}
		issueNums = append(issueNums, n)
	}

	if len(issueNums) == 0 {
		return fmt.Errorf("请指定至少一个 issue 编号")
	}

	ctx := context.Background()

	// 简化处理：固定使用 integration/main
	// TODO: 根据 issueNums 确定对应的 integration 分支
	integrationBranch := "integration/main"

	fmt.Printf("开始合并 %s 到 master...\n", integrationBranch)
	if err := ctrl.Merge(ctx, integrationBranch); err != nil {
		return fmt.Errorf("合并失败: %w", err)
	}

	fmt.Println("合并完成。")
	return nil
}

func runControlCheck(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 {
		return fmt.Errorf("必须指定 --repo 和 --issue")
	}

	// 确定 repo 目录
	repoDir := flagWorkDir
	if flagRepoDir != "" {
		repoDir = flagRepoDir
	}

	// 加载配置
	configDir := "."
	if flagRepoDir != "" {
		configDir = flagRepoDir
	}
	cfg := config.LoadWithDefaults(configDir)

	// 创建 taskctl client
	taskctlBin := ""
	if cfg.Control.TaskCtlBin != "" {
		taskctlBin = cfg.Control.TaskCtlBin
	}
	taskctl, err := control.NewTaskCtlClient(taskctlBin, repoDir)
	if err != nil {
		return fmt.Errorf("初始化 taskctl 失败: %w", err)
	}

	// 查找 issue 是否在 taskctl 中
	task, err := taskctl.FindByIssueNum(flagIssue)
	if err != nil {
		return fmt.Errorf("查询 taskctl 失败: %w", err)
	}

	if task != nil {
		fmt.Printf("Issue #%d 在编排队列中 (task: %s, status: %s)\n", flagIssue, task.ID, task.Status)
		// 返回 exit code 0 表示在队列中
		return nil
	}

	fmt.Printf("Issue #%d 不在编排队列中\n", flagIssue)
	// 返回 exit code 1 表示不在队列中（用于 workflow 判断）
	return fmt.Errorf("not in queue")
}

func runControlCloseMerged(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagPR <= 0 {
		return fmt.Errorf("必须指定 --repo 和 --pr")
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	pr, err := client.GetPR(ctx, flagPR)
	if err != nil {
		return err
	}
	if !pr.GetMerged() {
		fmt.Printf("PR #%d 未合并，跳过收口。\n", flagPR)
		return nil
	}
	headRef := pr.GetHead().GetRef()
	baseRef := pr.GetBase().GetRef()
	if !strings.HasPrefix(headRef, "integration/") || baseRef != "master" {
		fmt.Printf("PR #%d (%s -> %s) 非 integration 合并到 master，跳过收口。\n", flagPR, headRef, baseRef)
		return nil
	}

	commitMessages, err := client.ListPRCommitMessages(ctx, flagPR)
	if err != nil {
		return err
	}
	issueNums := extractIntegratedIssueNumbers(commitMessages)
	if len(issueNums) == 0 {
		fmt.Printf("PR #%d 未识别到 integration 子任务 issue，跳过收口。\n", flagPR)
		return nil
	}

	ctrl, err := buildController()
	if err != nil {
		return err
	}
	if err := ctrl.FinalizeIntegratedIssues(ctx, issueNums); err != nil {
		return err
	}

	fmt.Printf("PR #%d 收口完成，目标 issues: %v\n", flagPR, issueNums)
	return nil
}

var integrationIssuePattern = regexp.MustCompile(`(?i)issue\s*#([0-9]+)`)

func extractIntegratedIssueNumbers(messages []string) []int {
	unique := make(map[int]struct{})
	for _, message := range messages {
		matches := integrationIssuePattern.FindAllStringSubmatch(message, -1)
		for _, groups := range matches {
			if len(groups) < 2 {
				continue
			}
			num, err := strconv.Atoi(groups[1])
			if err != nil || num <= 0 {
				continue
			}
			unique[num] = struct{}{}
		}
	}

	var result []int
	for num := range unique {
		result = append(result, num)
	}
	sort.Ints(result)
	return result
}
