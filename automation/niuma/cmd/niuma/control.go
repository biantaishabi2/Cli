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
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/config"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	ghapi "github.com/google/go-github/v68/github"
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

var controlDagSyncCmd = &cobra.Command{
	Use:   "dag-sync",
	Short: "手动触发 DAG -> GitHub 展示依赖同步",
	RunE:  runControlDagSync,
}

var controlDagReconcileCmd = &cobra.Command{
	Use:   "dag-reconcile",
	Short: "手动触发 DAG/GitHub 展示依赖对账纠偏",
	RunE:  runControlDagReconcile,
}

var flagControlIssues string // --issues "40,41,42"
var flagIntegrationGateMaxRetries int
var flagControlDagDryRun bool
var flagPRConflictRetryThreshold int
var flagPRConflictUnknownBackoffs string
var flagPRConflictEnableAI bool
var flagPRConflictAIMaxAttempts int
var flagPRConflictSmokeTestCmd string
var flagPRConflictGoProfileEnabled bool
var flagPRConflictElixirProfileEnabled bool
var flagPRConflictRustProfileEnabled bool
var flagPRConflictGoGateCmd string
var flagPRConflictElixirGateCmd string
var flagPRConflictRustGateCmd string
var flagPRConflictMaxHunks int
var flagPRConflictMaxHunkLines int
var flagPRConflictMaxTotalLines int

func init() {
	controlCmd.AddCommand(controlRunCmd)
	controlCmd.AddCommand(controlStatusCmd)
	controlCmd.AddCommand(controlMergeCmd)
	controlCmd.AddCommand(controlCheckCmd)
	controlCmd.AddCommand(controlCloseMergedCmd)
	controlCmd.AddCommand(controlDagSyncCmd)
	controlCmd.AddCommand(controlDagReconcileCmd)

	controlMergeCmd.Flags().StringVar(&flagControlIssues, "issues", "", "要合并的 issue 编号列表（逗号分隔）")
	controlMergeCmd.MarkFlagRequired("issues")
	controlRunCmd.Flags().IntVar(&flagIntegrationGateMaxRetries, "integration-gate-max-retries", -1, "integration gate 最大重试次数（flag > env > default=2）")
	controlDagSyncCmd.Flags().BoolVar(&flagControlDagDryRun, "dry-run", false, "仅计算 diff，不写入 GitHub")
	controlDagReconcileCmd.Flags().BoolVar(&flagControlDagDryRun, "dry-run", false, "仅计算 diff，不写入 GitHub")
	controlRunCmd.Flags().IntVar(&flagPRConflictRetryThreshold, "pr-conflict-retry-threshold", -1, "pr-reviewable 冲突自动回退阈值（flag > env > default=3）")
	controlRunCmd.Flags().StringVar(&flagPRConflictUnknownBackoffs, "pr-conflict-unknown-backoffs", "", "pr-reviewable merge 状态 UNKNOWN 时重试退避列表，逗号分隔（flag > env > default=5s,15s,30s）")
	controlRunCmd.Flags().BoolVar(&flagPRConflictEnableAI, "pr-conflict-enable-ai", true, "是否启用冲突 AI 修复层（Rule 失败后触发）")
	controlRunCmd.Flags().IntVar(&flagPRConflictAIMaxAttempts, "pr-conflict-ai-max-attempts", 2, "冲突 AI 修复最大尝试次数（默认 2）")
	controlRunCmd.Flags().StringVar(&flagPRConflictSmokeTestCmd, "pr-conflict-smoke-test-cmd", "", "冲突修复门禁可选 smoke test 命令（默认关闭）")
	controlRunCmd.Flags().BoolVar(&flagPRConflictGoProfileEnabled, "pr-conflict-profile-go-enabled", true, "是否启用 Go 冲突 profile")
	controlRunCmd.Flags().BoolVar(&flagPRConflictElixirProfileEnabled, "pr-conflict-profile-elixir-enabled", true, "是否启用 Elixir 冲突 profile")
	controlRunCmd.Flags().BoolVar(&flagPRConflictRustProfileEnabled, "pr-conflict-profile-rust-enabled", true, "是否启用 Rust 冲突 profile")
	controlRunCmd.Flags().StringVar(&flagPRConflictGoGateCmd, "pr-conflict-profile-go-gate-cmd", "go test {pkg}", "Go profile 门禁命令模板")
	controlRunCmd.Flags().StringVar(&flagPRConflictElixirGateCmd, "pr-conflict-profile-elixir-gate-cmd", "mix test {scope}", "Elixir profile 门禁命令模板")
	controlRunCmd.Flags().StringVar(&flagPRConflictRustGateCmd, "pr-conflict-profile-rust-gate-cmd", "cargo test --manifest-path {path}", "Rust profile 门禁命令模板")
	controlRunCmd.Flags().IntVar(&flagPRConflictMaxHunks, "pr-conflict-max-hunks", 3, "冲突自动修复阈值：单文件 hunk 数上限")
	controlRunCmd.Flags().IntVar(&flagPRConflictMaxHunkLines, "pr-conflict-max-hunk-lines", 15, "冲突自动修复阈值：单 hunk 行数上限")
	controlRunCmd.Flags().IntVar(&flagPRConflictMaxTotalLines, "pr-conflict-max-total-lines", 50, "冲突自动修复阈值：总冲突行数上限")
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

	integrationGateMaxRetries, err := resolveIntegrationGateMaxRetries(flagIntegrationGateMaxRetries, os.Getenv("NIUMA_INTEGRATION_GATE_MAX_RETRIES"))
	if err != nil {
		return nil, err
	}
	prConflictRetryThreshold, err := resolvePRConflictRetryThreshold(flagPRConflictRetryThreshold, os.Getenv("NIUMA_PR_CONFLICT_RETRY_THRESHOLD"))
	if err != nil {
		return nil, err
	}
	prConflictUnknownBackoffs, err := resolvePRConflictUnknownBackoffs(flagPRConflictUnknownBackoffs, os.Getenv("NIUMA_PR_CONFLICT_UNKNOWN_BACKOFFS"))
	if err != nil {
		return nil, err
	}

	controlCfg := &control.ControlConfig{
		MergeStrategy:             cfg.Control.MergeStrategy,
		IntegrationBranchPrefix:   cfg.Control.IntegrationBranchPrefix,
		MaxOldBranches:            cfg.Control.MaxOldBranches,
		MinPRsForIntegration:      cfg.Control.MinPRsForIntegration,
		IntegrationGateMaxRetries: integrationGateMaxRetries,
		DagSync: control.DagSyncConfig{
			PollInterval:         cfg.Control.DagSync.GetPollInterval(),
			MaxRetry:             cfg.Control.DagSync.GetMaxRetry(),
			RetryBackoff:         cfg.Control.DagSync.GetRetryBackoff(),
			RateLimitRPS:         cfg.Control.DagSync.GetRateLimitRPS(),
			Timeout:              cfg.Control.DagSync.GetTimeout(),
			SkippedEdgeThreshold: cfg.Control.DagSync.GetSkippedEdgeThresholdRatio(),
		},
		PRConflictRetryThreshold:  prConflictRetryThreshold,
		PRConflictUnknownBackoffs: prConflictUnknownBackoffs,
		PRConflictEnableAI:        flagPRConflictEnableAI,
		PRConflictAIMaxAttempts:   flagPRConflictAIMaxAttempts,
		PRConflictSmokeTestCmd:    flagPRConflictSmokeTestCmd,
		ConflictResolution: control.PRConflictResolutionConfig{
			Profiles: map[string]control.PRConflictProfileConfig{
				"go": {
					Enabled:     flagPRConflictGoProfileEnabled,
					GateCommand: flagPRConflictGoGateCmd,
					Threshold: control.PRConflictThresholdConfig{
						MaxHunks:      flagPRConflictMaxHunks,
						MaxHunkLines:  flagPRConflictMaxHunkLines,
						MaxTotalLines: flagPRConflictMaxTotalLines,
					},
				},
				"elixir": {
					Enabled:     flagPRConflictElixirProfileEnabled,
					GateCommand: flagPRConflictElixirGateCmd,
					Threshold: control.PRConflictThresholdConfig{
						MaxHunks:      flagPRConflictMaxHunks,
						MaxHunkLines:  flagPRConflictMaxHunkLines,
						MaxTotalLines: flagPRConflictMaxTotalLines,
					},
				},
				"rust": {
					Enabled:     flagPRConflictRustProfileEnabled,
					GateCommand: flagPRConflictRustGateCmd,
					Threshold: control.PRConflictThresholdConfig{
						MaxHunks:      flagPRConflictMaxHunks,
						MaxHunkLines:  flagPRConflictMaxHunkLines,
						MaxTotalLines: flagPRConflictMaxTotalLines,
					},
				},
			},
		},
		RepoDir: repoDir,
	}

	builder := control.NewIntegrationBuilder(
		repoDir,
		"master",
	)

	ghOps := &gitHubControlOps{client: ghClient}

	return control.NewController(taskctl, analyzer, ghOps, builder, controlCfg), nil
}

func resolveIntegrationGateMaxRetries(flagValue int, envValue string) (int, error) {
	defaultValue := 2
	envValue = strings.TrimSpace(envValue)
	if envValue != "" {
		parsed, err := strconv.Atoi(envValue)
		if err != nil || parsed < 0 {
			return 0, fmt.Errorf("环境变量 NIUMA_INTEGRATION_GATE_MAX_RETRIES 非法: %q", envValue)
		}
		defaultValue = parsed
	}
	if flagValue >= 0 {
		return flagValue, nil
	}
	if flagValue < -1 {
		return 0, fmt.Errorf("参数 --integration-gate-max-retries 非法: %d", flagValue)
	}
	return defaultValue, nil
}

func resolvePRConflictRetryThreshold(flagValue int, envValue string) (int, error) {
	defaultValue := 3
	envValue = strings.TrimSpace(envValue)
	if envValue != "" {
		parsed, err := strconv.Atoi(envValue)
		if err != nil || parsed < 0 {
			return 0, fmt.Errorf("环境变量 NIUMA_PR_CONFLICT_RETRY_THRESHOLD 非法: %q", envValue)
		}
		defaultValue = parsed
	}
	if flagValue >= 0 {
		return flagValue, nil
	}
	if flagValue < -1 {
		return 0, fmt.Errorf("参数 --pr-conflict-retry-threshold 非法: %d", flagValue)
	}
	return defaultValue, nil
}

func resolvePRConflictUnknownBackoffs(flagValue string, envValue string) ([]time.Duration, error) {
	defaultValue := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
	if strings.TrimSpace(flagValue) != "" {
		return parseBackoffDurations(flagValue)
	}
	if strings.TrimSpace(envValue) != "" {
		return parseBackoffDurations(envValue)
	}
	return defaultValue, nil
}

func parseBackoffDurations(raw string) ([]time.Duration, error) {
	parts := strings.Split(raw, ",")
	backoffs := make([]time.Duration, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		d, err := time.ParseDuration(part)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("无效退避时长: %q", part)
		}
		backoffs = append(backoffs, d)
	}
	if len(backoffs) == 0 {
		return nil, fmt.Errorf("退避列表不能为空")
	}
	return backoffs, nil
}

// gitHubControlOps 适配 gh.Client 到 control.GitHubOps
type gitHubControlOps struct {
	client githubControlClient
}

type githubControlClient interface {
	ListIssuesWithLabel(ctx context.Context, label string) ([]*ghapi.Issue, error)
	ListIssuesByState(ctx context.Context, state string) ([]*ghapi.Issue, error)
	ListLabels(ctx context.Context, issueNumber int) ([]string, error)
	AddLabel(ctx context.Context, issueNumber int, label string) error
	GetIssue(ctx context.Context, number int) (*ghapi.Issue, error)
	UpdateIssueBody(ctx context.Context, number int, body string) error
	ListComments(ctx context.Context, issueNumber int) ([]*ghapi.IssueComment, error)
	AddComment(ctx context.Context, issueNumber int, body string) (*ghapi.IssueComment, error)
	CloseIssue(ctx context.Context, number int) error
	MergePR(ctx context.Context, number int, method string) error
	ReplaceLabel(ctx context.Context, issueNumber int, oldLabel, newLabel string) error
	ReplaceLabelIfPresent(ctx context.Context, issueNumber int, oldLabel, newLabel string) (bool, error)
	ReplaceLabels(ctx context.Context, issueNumber int, labels []string) error
	ListIssueBlockedBy(ctx context.Context, issueNumber int) ([]int, error)
	AddIssueBlockedBy(ctx context.Context, issueNumber int, blockedByIssueNumber int) error
	RemoveIssueBlockedBy(ctx context.Context, issueNumber int, blockedByIssueNumber int) error
	FindMarker(ctx context.Context, issueNumber int, t marker.Type) (*gh.MarkerComment, error)
	GetPR(ctx context.Context, number int) (*ghapi.PullRequest, error)
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

func (g *gitHubControlOps) ListLabels(ctx context.Context, issueNumber int) ([]string, error) {
	return g.client.ListLabels(ctx, issueNumber)
}

func (g *gitHubControlOps) AddLabel(ctx context.Context, issueNumber int, label string) error {
	return g.client.AddLabel(ctx, issueNumber, label)
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

func (g *gitHubControlOps) UpdateIssueBody(ctx context.Context, issueNumber int, body string) error {
	return g.client.UpdateIssueBody(ctx, issueNumber, body)
}

func (g *gitHubControlOps) ListCommentBodies(ctx context.Context, issueNumber int) ([]string, error) {
	comments, err := g.client.ListComments(ctx, issueNumber)
	if err != nil {
		return nil, err
	}
	return gh.ToCommentBodies(comments), nil
}

func (g *gitHubControlOps) AddIssueComment(ctx context.Context, issueNumber int, body string) error {
	_, err := g.client.AddComment(ctx, issueNumber, body)
	return err
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

func (g *gitHubControlOps) ReplaceLabelIfPresent(ctx context.Context, issueNumber int, oldLabel, newLabel string) (bool, error) {
	return g.client.ReplaceLabelIfPresent(ctx, issueNumber, oldLabel, newLabel)
}

func (g *gitHubControlOps) ReplaceLabels(ctx context.Context, issueNumber int, labels []string) error {
	return g.client.ReplaceLabels(ctx, issueNumber, labels)
}

func (g *gitHubControlOps) ListIssueBlockedBy(ctx context.Context, issueNumber int) ([]int, error) {
	return g.client.ListIssueBlockedBy(ctx, issueNumber)
}

func (g *gitHubControlOps) AddIssueBlockedBy(ctx context.Context, issueNumber int, blockedByIssueNumber int) error {
	return g.client.AddIssueBlockedBy(ctx, issueNumber, blockedByIssueNumber)
}

func (g *gitHubControlOps) RemoveIssueBlockedBy(ctx context.Context, issueNumber int, blockedByIssueNumber int) error {
	return g.client.RemoveIssueBlockedBy(ctx, issueNumber, blockedByIssueNumber)
}

func (g *gitHubControlOps) ResolvePRMetadata(ctx context.Context, issueNumber int) (control.PRMetadata, error) {
	prNum, pr, err := g.resolveOpenPR(ctx, issueNumber)
	if err != nil {
		return control.PRMetadata{}, err
	}

	branch := strings.TrimSpace(pr.GetHead().GetRef())
	if branch == "" {
		return control.PRMetadata{}, fmt.Errorf("PR #%d: %w", prNum, control.ErrPRBranchUnavailable)
	}

	return control.PRMetadata{
		PRNum:  prNum,
		Branch: branch,
	}, nil
}

func (g *gitHubControlOps) ResolvePRReviewStatus(ctx context.Context, issueNumber int) (control.PRReviewStatus, error) {
	prNum, pr, err := g.resolveOpenPR(ctx, issueNumber)
	if err != nil {
		return control.PRReviewStatus{}, err
	}

	mergeStateStatus := strings.ToUpper(strings.TrimSpace(pr.GetMergeableState()))
	if mergeStateStatus == "" {
		mergeStateStatus = "UNKNOWN"
	}

	mergeable := control.PRMergeableUnknown
	switch mergeStateStatus {
	case "DIRTY", "BLOCKED":
		mergeable = control.PRMergeableConflicting
	default:
		if pr.Mergeable != nil {
			if pr.GetMergeable() {
				mergeable = control.PRMergeableMergeable
			} else {
				mergeable = control.PRMergeableConflicting
			}
		}
	}

	return control.PRReviewStatus{
		PRNum:            prNum,
		HeadSHA:          strings.TrimSpace(pr.GetHead().GetSHA()),
		Mergeable:        mergeable,
		MergeStateStatus: mergeStateStatus,
	}, nil
}

func (g *gitHubControlOps) resolveOpenPR(ctx context.Context, issueNumber int) (int, *ghapi.PullRequest, error) {
	found, err := g.client.FindMarker(ctx, issueNumber, marker.TypePRCreated)
	if err != nil {
		return 0, nil, fmt.Errorf("读取 issue #%d marker 失败: %w", issueNumber, err)
	}
	if found == nil || found.Marker == nil || found.Marker.PR <= 0 {
		return 0, nil, fmt.Errorf("issue #%d: %w", issueNumber, control.ErrPRMarkerNotFound)
	}

	prNum := found.Marker.PR
	pr, err := g.client.GetPR(ctx, prNum)
	if err != nil {
		return 0, nil, fmt.Errorf("读取 PR #%d 失败: %w", prNum, err)
	}
	if strings.EqualFold(pr.GetState(), "closed") {
		return 0, nil, fmt.Errorf("PR #%d: %w", prNum, control.ErrPRClosed)
	}

	return prNum, pr, nil
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

func runControlDagSync(cmd *cobra.Command, args []string) error {
	ctrl, err := buildController()
	if err != nil {
		return err
	}

	result, err := ctrl.DagSync(context.Background(), flagControlDagDryRun)
	printDagSyncResult("dag-sync", result, flagControlDagDryRun)
	if err != nil {
		return fmt.Errorf("dag-sync 失败: %w", err)
	}
	return nil
}

func runControlDagReconcile(cmd *cobra.Command, args []string) error {
	ctrl, err := buildController()
	if err != nil {
		return err
	}

	result, err := ctrl.DagReconcile(context.Background(), flagControlDagDryRun)
	printDagSyncResult("dag-reconcile", result, flagControlDagDryRun)
	if err != nil {
		return fmt.Errorf("dag-reconcile 失败: %w", err)
	}
	return nil
}

func printDagSyncResult(cmdName string, result control.DagSyncResult, dryRun bool) {
	for _, line := range formatDagSyncResult(cmdName, result, dryRun) {
		fmt.Println(line)
	}
}

func formatDagSyncResult(cmdName string, result control.DagSyncResult, dryRun bool) []string {
	lines := []string{fmt.Sprintf(
		"%s 完成: status=%s mode=%s dry_run=%t hash=%s total_edges=%d add=%d remove=%d skipped_edges=%d error_type=%s",
		cmdName,
		result.Status,
		result.Mode,
		dryRun,
		result.DagHash,
		result.TotalEdges,
		result.AppliedAdd,
		result.AppliedRemove,
		result.SkippedEdges,
		result.ErrorType,
	)}
	if result.Error != "" {
		lines = append(lines, fmt.Sprintf("%s 错误详情: %s", cmdName, result.Error))
	}
	return lines
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
