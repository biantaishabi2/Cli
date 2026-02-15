// cmd/niuma/main.go
// niuma CLI 入口：cobra 命令定义
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

	// 注册子命令
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(planDraftCmd)
	rootCmd.AddCommand(planFinalCmd)
	rootCmd.AddCommand(fixCmd)
	rootCmd.AddCommand(iterateCmd)
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

	provider, err := resolveProvider()
	if err != nil {
		return err
	}

	orch := agent.NewOrchestrator(client, provider, flagIssue)
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

	provider, err := resolveProvider()
	if err != nil {
		return err
	}

	orch := agent.NewOrchestrator(client, provider, flagIssue)
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
	fixCmd.Flags().StringVar(&flagWorkDir, "workdir", ".", "工作目录")
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

	provider, err := resolveProvider()
	if err != nil {
		return err
	}

	orch := agent.NewOrchestrator(client, provider, flagIssue)
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
	iterateCmd.Flags().StringVar(&flagWorkDir, "workdir", ".", "工作目录")
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

	provider, err := resolveProvider()
	if err != nil {
		return err
	}

	orch := agent.NewOrchestrator(client, provider, flagIssue)
	fmt.Printf("正在为 issue #%d / PR #%d 迭代修改...\n", flagIssue, flagPR)

	if err := orch.DoIterate(ctx, flagPR, flagWorkDir); err != nil {
		return fmt.Errorf("迭代修改失败: %w", err)
	}

	fmt.Println("迭代修改完成。")
	return nil
}

// ===== 辅助函数 =====

// resolveProvider 根据配置创建 AI Provider
func resolveProvider() (ai.Provider, error) {
	cfg := config.LoadWithDefaults(".")

	providerName := cfg.AI.Default
	providerCfg, ok := cfg.AI.Providers[providerName]
	if !ok {
		return nil, fmt.Errorf("AI provider %q 未配置，请在 .niuma.yml 中定义或设置 NIUMA_AI_DEFAULT", providerName)
	}

	if providerCfg.Cmd == "" {
		return nil, fmt.Errorf("AI provider %q 缺少 cmd 配置", providerName)
	}

	return &ai.CLIProvider{
		ProviderName: providerName,
		Cmd:          providerCfg.Cmd,
	}, nil
}
