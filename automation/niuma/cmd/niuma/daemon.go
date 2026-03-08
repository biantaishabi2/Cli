// cmd/niuma/daemon.go
// daemon 命令：轮询 tracker 并自动派发 issue 处理
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/config"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/daemon"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/linear"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/project"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "轮询 tracker 并自动派发 issue 处理",
	Long:  "长驻进程，定期检查 GitHub Project 或 Linear 中的 issue 状态，自动调用 AI provider 处理",
	RunE:  runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}

func runDaemon(cmd *cobra.Command, args []string) error {
	configDir := "."
	if flagRepoDir != "" {
		configDir = flagRepoDir
	}
	cfg := config.LoadWithDefaults(configDir)

	tracker, err := initTracker(cfg)
	if err != nil {
		return fmt.Errorf("初始化 tracker 失败: %w", err)
	}

	provider, err := initProvider(cfg)
	if err != nil {
		return fmt.Errorf("初始化 AI provider 失败: %w", err)
	}

	workDir := flagRepoDir
	if workDir == "" {
		workDir = flagWorkDir
	}

	executor := &daemon.AIExecutor{
		Provider: provider,
		WorkDir:  workDir,
	}

	daemonCfg := &daemon.Config{
		PollInterval:  cfg.Daemon.GetPollInterval(),
		MaxConcurrent: cfg.Daemon.GetMaxConcurrent(),
		MaxRetries:    3,
		StatusMapping: cfg.Daemon.GetStatusMapping(),
	}

	d := daemon.New(tracker, executor, daemonCfg)

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("[daemon] 启动，tracker=%s，provider=%s，轮询间隔=%s",
		cfg.Daemon.Tracker, provider.Name(), daemonCfg.PollInterval)
	return d.Run(ctx)
}

func initTracker(cfg *config.Config) (daemon.Tracker, error) {
	switch cfg.Daemon.Tracker {
	case "github", "":
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("GITHUB_TOKEN 环境变量未设置")
		}
		c := project.NewClient(cfg.Daemon.GitHub.Owner, cfg.Daemon.GitHub.ProjectNumber, token)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := c.Init(ctx); err != nil {
			return nil, fmt.Errorf("GitHub Project 初始化失败: %w", err)
		}
		return c, nil

	case "linear":
		apiKey := os.Getenv(cfg.Daemon.Linear.APIKeyEnv)
		if apiKey == "" {
			apiKey = os.Getenv("LINEAR_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("Linear API key 未设置（env: %s）", cfg.Daemon.Linear.APIKeyEnv)
		}
		c := linear.NewClient(cfg.Daemon.Linear.Team, apiKey)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := c.Init(ctx); err != nil {
			return nil, fmt.Errorf("Linear 初始化失败: %w", err)
		}
		return c, nil

	default:
		return nil, fmt.Errorf("未知 tracker 类型: %q", cfg.Daemon.Tracker)
	}
}

func initProvider(cfg *config.Config) (ai.Provider, error) {
	implName := cfg.ResolveDefaultPlaceholder(cfg.AI.Implementation.Provider)
	pcfg, ok := cfg.AI.Providers[implName]
	if !ok {
		return nil, fmt.Errorf("AI provider %q 未配置", implName)
	}
	if pcfg.Cmd == "" && pcfg.CmdAgent == "" {
		return nil, fmt.Errorf("AI provider %q 缺少 cmd 或 cmd_agent", implName)
	}
	return &ai.CLIProvider{
		ProviderName: implName,
		Cmd:          pcfg.Cmd,
		CmdAgent:     pcfg.CmdAgent,
	}, nil
}
