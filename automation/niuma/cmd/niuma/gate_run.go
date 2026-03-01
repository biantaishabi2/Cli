package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/gate"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
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
var flagGateSelfCheck bool

const defaultGateMaxRetries = 2

func init() {
	gateCmd.AddCommand(gateRunCmd)
	gateRunCmd.Flags().StringVar(&flagGateMaxRetries, "max-retries", strconv.Itoa(defaultGateMaxRetries), "gate 自动修复最大重试次数")
	gateRunCmd.Flags().BoolVar(&flagGateSelfCheck, "self-check", false, "self-check 模式：gate 失败时不设标签/不写 issue 评论，仅写 PR review")
}

// parseMaxRetries 将 string 解析为 int，解析失败回退默认值 2。
// 语义约束：
// - 0: 仅首轮执行，不做自动重试
// - <0: 视为非法参数
func parseMaxRetries(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	n, err := strconv.Atoi(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: --max-retries 值 '%s' 非法，回退为 %d\n", raw, defaultGateMaxRetries)
		return defaultGateMaxRetries, nil
	}
	if n < 0 {
		return 0, fmt.Errorf("参数 --max-retries 非法: %d（必须 >= 0）", n)
	}
	return n, nil
}

// retry_count 契约语义：仅统计自动重试次数，不包含首轮失败。
// gate.Runner 内部计数是失败次数（首轮=1），这里做边界映射以保持外部语义稳定。
func retryCountForStorage(internalCount int, attemptKey string) int {
	if attemptKey == "" || internalCount <= 0 {
		return internalCount
	}
	return internalCount - 1
}

func retryCountFromStorage(storedCount int, attemptKey string) int {
	if attemptKey == "" || storedCount < 0 {
		return storedCount
	}
	return storedCount + 1
}

func retryCountForDisplay(internalCount int, attemptKey string) int {
	return retryCountForStorage(internalCount, attemptKey)
}

func runGateRun(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 || flagPR == 0 {
		return fmt.Errorf("必须指定 --repo、--issue 和 --pr")
	}

	maxRetries, err := parseMaxRetries(flagGateMaxRetries)
	if err != nil {
		return err
	}

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
		SelfCheck:  flagGateSelfCheck,
		RunID:      os.Getenv("GITHUB_RUN_ID"),
		RunAttempt: os.Getenv("GITHUB_RUN_ATTEMPT"),
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
		FindGateRetryState: func(ctx context.Context, issue int) (int, string, error) {
			mc, err := client.FindMarker(ctx, issue, marker.TypeGateRetry)
			if err != nil {
				return 0, "", err
			}
			if mc == nil {
				return 0, "", nil
			}
			return retryCountFromStorage(mc.Marker.Revision, mc.Marker.AttemptKey), mc.Marker.AttemptKey, nil
		},
		UpsertGateRetryCount: func(ctx context.Context, issue int, count int, attemptKey string) error {
			storedCount := retryCountForStorage(count, attemptKey)
			m := &marker.Marker{
				Type:       marker.TypeGateRetry,
				Issue:      issue,
				Revision:   storedCount,
				AttemptKey: attemptKey,
			}
			body := fmt.Sprintf("gate retry_count=%d", storedCount)
			return client.CreateOrUpdateMarker(ctx, issue, m, body)
		},
		HasLabel: func(ctx context.Context, issue int, label string) (bool, error) {
			labels, err := client.ListLabels(ctx, issue)
			if err != nil {
				return false, err
			}
			for _, l := range labels {
				if l == label {
					return true, nil
				}
			}
			return false, nil
		},
		AddPRReview: func(ctx context.Context, repo string, pr int, body string) error {
			_, err := client.CreatePRReview(ctx, pr, body, "COMMENT")
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
		displayRetryCount := retryCountForDisplay(result.RetryCount, result.AttemptKey)
		fmt.Printf(
			"gate 未通过: issue=%d pr=%d reason_code=%s failure_class=%s retry_count=%d max_retries=%d attempt_key=%s escalated=%t dedup=%t\n",
			flagIssue,
			flagPR,
			result.ReasonCode,
			result.FailureClass,
			displayRetryCount,
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
