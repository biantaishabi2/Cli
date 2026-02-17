// cmd/niuma/control.go
// control 子命令：多 Issue 协调
package main

import (
	"context"
	"fmt"
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

var flagControlIssues string // --issues "40,41,42"

func init() {
	controlCmd.AddCommand(controlRunCmd)
	controlCmd.AddCommand(controlStatusCmd)
	controlCmd.AddCommand(controlMergeCmd)

	controlMergeCmd.Flags().StringVar(&flagControlIssues, "issues", "", "要合并的 issue 编号列表（逗号分隔）")
	controlMergeCmd.MarkFlagRequired("issues")
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
		MergeStrategy:           cfg.Control.MergeStrategy,
		IntegrationBranchPrefix: cfg.Control.IntegrationBranchPrefix,
		MaxOldBranches:          cfg.Control.MaxOldBranches,
		MinPRsForIntegration:    cfg.Control.MinPRsForIntegration,
	}

	builder := control.NewIntegrationBuilder(
		repoDir,
		"master",
	)

	ghOps := &gitHubControlOps{client: ghClient}

	return control.NewController(taskctl, analyzer, ghOps, builder, controlCfg), nil
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
			Labels: labels,
		})
	}
	return result, nil
}

func (g *gitHubControlOps) MergePR(ctx context.Context, prNum int, method string) error {
	return g.client.MergePR(ctx, prNum, method)
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
