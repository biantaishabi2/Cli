// cmd/niuma/main.go
// niuma CLI 入口：cobra 命令定义
// Phase 2.5：支持多 provider、--repo-dir、worktree 自动管理
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/agent"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/config"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/spf13/cobra"
)

var (
	flagRepo    string
	flagIssue   int
	flagPR      int
	flagWorkDir string
	flagRepoDir string // 主仓库本地路径（用于 worktree）
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

	// 注册子命令
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(planDraftCmd)
	rootCmd.AddCommand(planFinalCmd)
	rootCmd.AddCommand(fixCmd)
	rootCmd.AddCommand(iterateCmd)
	rootCmd.AddCommand(reviewCmd)
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

func init() {
	fixCmd.Flags().StringVar(&flagWorkDir, "workdir", ".", "工作目录（无 --repo-dir 时使用）")
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

func init() {
	iterateCmd.Flags().IntVar(&flagPR, "pr", 0, "PR 编号")
	iterateCmd.Flags().StringVar(&flagWorkDir, "workdir", ".", "工作目录（无 --repo-dir 时使用）")
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

func init() {
	reviewCmd.Flags().IntVar(&flagPR, "pr", 0, "PR 编号")
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
	cfg := config.LoadWithDefaults(".")

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

	// 如果没有配置多 provider 讨论，使用简单模式
	if len(cfg.AI.Discussion.Providers) == 0 && flagRepoDir == "" {
		return agent.NewOrchestrator(client, defaultProvider, issueNumber), nil
	}

	// 构建完整配置
	orchCfg := &agent.OrchestratorConfig{
		RepoDir: flagRepoDir,
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
