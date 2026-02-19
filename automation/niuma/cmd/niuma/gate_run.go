package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/gate"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/spf13/cobra"
)

var gateCmd = &cobra.Command{
	Use:   "gate",
	Short: "PR gate 相关命令",
}

var gateRunCmd = &cobra.Command{
	Use:   "run",
	Short: "执行 PR gate 并处理重试/升级",
	RunE:  runGateRun,
}

var flagGateMaxRetries string

func init() {
	gateCmd.AddCommand(gateRunCmd)
	gateRunCmd.Flags().StringVar(&flagGateMaxRetries, "max-retries", "2", "gate 自动修复最大重试次数")
}

// parseMaxRetries 将 string 解析为 int，解析失败回退默认值 2，值 ≤0 clamp 到 1。
func parseMaxRetries(raw string) int {
	const defaultVal = 2
	n, err := strconv.Atoi(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: --max-retries 值 '%s' 非法，回退为 %d\n", raw, defaultVal)
		return defaultVal
	}
	if n <= 0 {
		fmt.Fprintf(os.Stderr, "WARNING: --max-retries 值 %d ≤ 0，clamp 到 1\n", n)
		return 1
	}
	return n
}

func runGateRun(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 || flagPR == 0 {
		return fmt.Errorf("必须指定 --repo、--issue 和 --pr")
	}

	maxRetries := parseMaxRetries(flagGateMaxRetries)

	repoDir := "."
	if flagRepoDir != "" {
		repoDir = flagRepoDir
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	runner, err := gate.NewRunner(gate.Options{
		Repo:       flagRepo,
		Issue:      flagIssue,
		PR:         flagPR,
		RepoDir:    repoDir,
		MaxRetries: maxRetries,
		MarkNeedsFix: func(ctx context.Context, repo string, issue int) error {
			return markIssueNeedsFix(ctx, client, issue)
		},
		AddLabels: func(ctx context.Context, repo string, issue int, labels []string) error {
			for _, label := range labels {
				if err := client.AddLabel(ctx, issue, label); err != nil {
					return err
				}
			}
			return nil
		},
		AddComment: func(ctx context.Context, repo string, issue int, body string) error {
			_, err := client.AddComment(ctx, issue, body)
			return err
		},
	})
	if err != nil {
		return err
	}

	result, err := runner.Run(ctx)
	if err == nil {
		fmt.Printf("gate 通过: issue=%d pr=%d\n", flagIssue, flagPR)
		return nil
	}

	if errors.Is(err, gate.ErrGateFailed) {
		fmt.Printf(
			"gate 未通过: issue=%d pr=%d retry_count=%d max_retries=%d attempt_key=%s escalated=%t dedup=%t\n",
			flagIssue,
			flagPR,
			result.RetryCount,
			result.MaxRetries,
			result.AttemptKey,
			result.Escalated,
			result.EscalationCommentSkipped,
		)
		return withExitCode(1, err)
	}
	return err
}

func markIssueNeedsFix(ctx context.Context, client *gh.Client, issue int) error {
	if err := state.TransitionWithRetry(ctx, client, issue, state.StatePRCreated, state.StatePRNeedsFix, nil); err != nil {
		return state.TransitionWithRetry(ctx, client, issue, "", state.StatePRNeedsFix, nil)
	}
	return nil
}
