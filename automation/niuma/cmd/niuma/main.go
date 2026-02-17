// cmd/niuma/main.go
// niuma CLI 入口：cobra 命令定义
// Phase 2.5：支持多 provider、--repo-dir、worktree 自动管理
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/agent"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/config"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/spf13/cobra"
)

var (
	flagRepo                string
	flagIssue               int
	flagPR                  int
	flagWorkDir             string
	flagRepoDir             string // 主仓库本地路径（用于 worktree）
	flagMaxDiscussionRounds int    // discuss 最大轮次（0=使用配置）
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "niuma",
	Short: "niuma - AI 驱动的全自动开发机器人",
	Long:  "niuma 自动处理 GitHub Issue：分析问题 → 制定方案 → 编写代码 → 创建 PR → 迭代修复",
}

func init() {
	// 全局 flags
	rootCmd.PersistentFlags().StringVar(&flagRepo, "repo", "", "GitHub 仓库 (owner/repo)")
	rootCmd.PersistentFlags().IntVar(&flagIssue, "issue", 0, "Issue 编号")
	rootCmd.PersistentFlags().StringVar(&flagRepoDir, "repo-dir", "", "主仓库本地路径（用于 worktree 隔离）")
	rootCmd.PersistentFlags().IntVar(&flagPR, "pr", 0, "PR 编号")
	rootCmd.PersistentFlags().StringVar(&flagWorkDir, "workdir", ".", "工作目录（无 --repo-dir 时使用）")

	// 注册子命令
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(planDraftCmd)
	rootCmd.AddCommand(discussCmd)
	rootCmd.AddCommand(planFinalCmd)
	rootCmd.AddCommand(fixCmd)
	rootCmd.AddCommand(iterateCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(controlCmd)

	// discuss 特有 flags
	discussCmd.Flags().IntVar(&flagMaxDiscussionRounds, "max-discussion-rounds", 0, "讨论最大轮次（1-20，0 表示使用配置）")
}

// ===== status 命令：完整实现 =====

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "显示 issue 当前状态、marker 和收敛情况",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 {
		return fmt.Errorf("必须指定 --repo 和 --issue")
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	// 获取 issue 信息
	issue, err := client.GetIssue(ctx, flagIssue)
	if err != nil {
		return err
	}
	fmt.Printf("Issue #%d: %s\n", flagIssue, issue.GetTitle())
	fmt.Printf("State: %s\n", issue.GetState())

	// 读取状态机
	m := state.NewMachine(client, flagIssue)
	current, err := m.Current(ctx)
	if err != nil {
		fmt.Printf("Bot State: (none)\n")
	} else {
		fmt.Printf("Bot State: %s\n", current)
	}

	// 列出所有 label
	labels, err := client.ListLabels(ctx, flagIssue)
	if err != nil {
		return err
	}
	fmt.Printf("Labels: %v\n", labels)

	// 查找各类 marker
	fmt.Println("\nMarkers:")
	for _, t := range marker.AllTypes {
		mc, err := client.FindMarker(ctx, flagIssue, t)
		if err != nil {
			fmt.Printf("  %s: error (%v)\n", t, err)
			continue
		}
		if mc != nil {
			fmt.Printf("  %s: rev=%d (comment #%d)\n", t, mc.Marker.Revision, mc.CommentID)
		} else {
			fmt.Printf("  %s: (not found)\n", t)
		}
	}

	// 检查收敛（仅在 needs-discussion 状态下有意义）
	if current == state.StateNeedsDiscussion {
		checker := state.DefaultChecker()
		result, err := m.CheckConvergence(ctx, checker)
		if err != nil {
			fmt.Printf("\nConvergence: error (%v)\n", err)
		} else {
			fmt.Printf("\nConvergence: %s\n", result)
		}
	}

	return nil
}

// ===== plan-draft 命令 =====

var planDraftCmd = &cobra.Command{
	Use:   "plan-draft",
	Short: "生成方案草案（需要 AI provider）",
	RunE:  runPlanDraft,
}

func runPlanDraft(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 {
		return fmt.Errorf("必须指定 --repo 和 --issue")
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	orch, err := buildOrchestrator(client, flagIssue)
	if err != nil {
		return err
	}

	fmt.Printf("正在为 issue #%d 生成方案草案...\n", flagIssue)

	if err := orch.DoPlanDraft(ctx); err != nil {
		return fmt.Errorf("生成草案失败: %w", err)
	}

	fmt.Println("方案草案已生成并发布到 issue 评论。")
	return nil
}

// ===== discuss 命令 =====

var discussCmd = &cobra.Command{
	Use:   "discuss",
	Short: "触发讨论检查：多 provider 左右互搏或收敛定稿",
	RunE:  runDiscuss,
}

func runDiscuss(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 {
		return fmt.Errorf("必须指定 --repo 和 --issue")
	}

	configDir := "."
	if flagRepoDir != "" {
		configDir = flagRepoDir
	}
	cfg := config.LoadWithDefaults(configDir)

	discussTimeout := time.Duration(cfg.Workflow.GetDiscussTimeoutMinutes()) * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), discussTimeout)
	defer cancel()

	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	orch, err := buildOrchestrator(client, flagIssue)
	if err != nil {
		return err
	}

	if err := validateMaxDiscussionRounds(flagMaxDiscussionRounds); err != nil {
		return err
	}

	maxRounds := resolveDiscussMaxRounds(flagMaxDiscussionRounds, cfg.Workflow.GetMaxDiscussionRounds())

	fmt.Printf("正在为 issue #%d 进行讨论检查（max_rounds=%d timeout=%s）...\n", flagIssue, maxRounds, discussTimeout)

	if err := orch.DoDiscuss(ctx, maxRounds); err != nil {
		return fmt.Errorf("讨论检查失败: %w", err)
	}

	fmt.Println("讨论检查完成。")
	return nil
}

// ===== plan-final 命令 =====

var planFinalCmd = &cobra.Command{
	Use:   "plan-final",
	Short: "生成最终方案（需要 AI provider）",
	RunE:  runPlanFinal,
}

func runPlanFinal(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 {
		return fmt.Errorf("必须指定 --repo 和 --issue")
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	orch, err := buildOrchestrator(client, flagIssue)
	if err != nil {
		return err
	}

	fmt.Printf("正在为 issue #%d 生成最终方案...\n", flagIssue)

	if err := orch.DoPlanFinal(ctx); err != nil {
		return fmt.Errorf("生成最终方案失败: %w", err)
	}

	fmt.Println("最终方案已生成并发布。")
	return nil
}

// ===== fix 命令 =====

var fixCmd = &cobra.Command{
	Use:   "fix",
	Short: "自动实现代码（需要 AI provider）",
	RunE:  runFix,
}

func runFix(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 {
		return fmt.Errorf("必须指定 --repo 和 --issue")
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	orch, err := buildOrchestrator(client, flagIssue)
	if err != nil {
		return err
	}

	fmt.Printf("正在为 issue #%d 实现代码...\n", flagIssue)

	if err := orch.DoImplement(ctx, flagWorkDir); err != nil {
		return fmt.Errorf("代码实现失败: %w", err)
	}

	fmt.Println("代码实现完成。")
	return nil
}

// ===== iterate 命令 =====

var iterateCmd = &cobra.Command{
	Use:   "iterate",
	Short: "根据 review 意见迭代修改（需要 AI provider）",
	RunE:  runIterate,
}

func runIterate(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 || flagPR == 0 {
		return fmt.Errorf("必须指定 --repo、--issue 和 --pr")
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	orch, err := buildOrchestrator(client, flagIssue)
	if err != nil {
		return err
	}

	fmt.Printf("正在为 issue #%d / PR #%d 迭代修改...\n", flagIssue, flagPR)

	if err := orch.DoIterate(ctx, flagPR, flagWorkDir); err != nil {
		return fmt.Errorf("迭代修改失败: %w", err)
	}

	fmt.Println("迭代修改完成。")
	return nil
}

// ===== review 命令 =====

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "AI 自审 PR（需要 AI provider）",
	RunE:  runReview,
}

func runReview(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 || flagPR == 0 {
		return fmt.Errorf("必须指定 --repo、--issue 和 --pr")
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	orch, err := buildOrchestrator(client, flagIssue)
	if err != nil {
		return err
	}

	fmt.Printf("正在为 issue #%d / PR #%d 进行 AI 自审...\n", flagIssue, flagPR)

	if err := orch.DoReview(ctx, flagPR); err != nil {
		return fmt.Errorf("AI 自审失败: %w", err)
	}

	fmt.Println("AI 自审完成。")
	return nil
}

// ===== 辅助函数 =====

// buildOrchestrator 根据配置创建 Orchestrator
// 支持多 provider 和 worktree 模式
func buildOrchestrator(client *gh.Client, issueNumber int) (*agent.Orchestrator, error) {
	// 优先从 --repo-dir 加载配置（CI 中 working-directory 不是仓库根目录）
	configDir := "."
	if flagRepoDir != "" {
		configDir = flagRepoDir
	}
	cfg := config.LoadWithDefaults(configDir)

	// 构建所有 provider
	providers, err := resolveProviders(cfg)
	if err != nil {
		return nil, err
	}

	// 确定默认 provider
	defaultName := cfg.AI.Default
	defaultProvider, ok := providers[defaultName]
	if !ok {
		return nil, fmt.Errorf("默认 AI provider %q 未配置", defaultName)
	}

	// 构建完整配置
	orchCfg := &agent.OrchestratorConfig{
		RepoDir:             flagRepoDir,
		RequirePlanApproval: cfg.Workflow.RequirePlanApproval,
		MaxIterateRounds:    cfg.Workflow.GetMaxIterateRounds(),
		MaxDiscussionRounds: cfg.Workflow.GetMaxDiscussionRounds(),
	}

	// 讨论 provider
	for _, name := range cfg.AI.Discussion.Providers {
		p, ok := providers[name]
		if !ok {
			return nil, fmt.Errorf("讨论 provider %q 未配置", name)
		}
		orchCfg.DiscussionProviders = append(orchCfg.DiscussionProviders, p)
	}

	// Consolidator
	if cfg.AI.Discussion.Consolidator != "" {
		p, ok := providers[cfg.AI.Discussion.Consolidator]
		if !ok {
			return nil, fmt.Errorf("汇总 provider %q 未配置", cfg.AI.Discussion.Consolidator)
		}
		orchCfg.Consolidator = p
	}

	// 实现 provider
	if cfg.AI.Implementation.Provider != "" {
		p, ok := providers[cfg.AI.Implementation.Provider]
		if !ok {
			return nil, fmt.Errorf("实现 provider %q 未配置", cfg.AI.Implementation.Provider)
		}
		orchCfg.ImplementProvider = p
	} else {
		orchCfg.ImplementProvider = defaultProvider
	}

	return agent.NewOrchestratorWithConfig(client, issueNumber, orchCfg), nil
}

// resolveProviders 从配置创建所有 AI Provider
func resolveProviders(cfg *config.Config) (map[string]ai.Provider, error) {
	providers := make(map[string]ai.Provider)

	for name, pcfg := range cfg.AI.Providers {
		if pcfg.Cmd == "" && pcfg.CmdAgent == "" {
			return nil, fmt.Errorf("AI provider %q 缺少 cmd 或 cmd_agent 配置", name)
		}

		providers[name] = &ai.CLIProvider{
			ProviderName: name,
			Cmd:          pcfg.Cmd,
			CmdAgent:     pcfg.CmdAgent,
		}
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("没有配置任何 AI provider，请在 .niuma.yml 中定义或设置 NIUMA_AI_DEFAULT")
	}

	return providers, nil
}

// resolveDiscussMaxRounds 解析 discuss 最大轮次优先级：CLI > 配置 > 默认值。
func resolveDiscussMaxRounds(cliRounds, configRounds int) int {
	if cliRounds > 0 {
		return cliRounds
	}
	if configRounds >= 1 && configRounds <= 20 {
		return configRounds
	}
	return 5
}

// validateMaxDiscussionRounds 校验 discuss 轮次 CLI 参数范围。
func validateMaxDiscussionRounds(rounds int) error {
	if rounds < 0 || rounds > 20 {
		return fmt.Errorf("--max-discussion-rounds 必须在 1-20，或使用 0 表示走配置默认值")
	}
	return nil
}
