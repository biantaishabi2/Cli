package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/spf13/cobra"
)

var (
	flagStateFrom     string
	flagStateTo       string
	flagStatePriority string
)

const (
	exitCodeStateConflict       = 20
	exitCodeStateInvalid        = 21
	exitCodeStateInvariant      = 22
	exitCodeStateTransitionEdge = 23
)

var stateLabelCmd = &cobra.Command{
	Use:   "state-label",
	Short: "受控迁移 bot 状态标签",
}

var stateLabelSetCmd = &cobra.Command{
	Use:   "set",
	Short: "原子迁移到目标 bot 状态",
	RunE:  runStateLabelSet,
}

var stateLabelNormalizeCmd = &cobra.Command{
	Use:   "normalize",
	Short: "按优先级收敛多 bot 状态",
	RunE:  runStateLabelNormalize,
}

var stateLabelClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "清理全部 bot 状态标签",
	RunE:  runStateLabelClear,
}

func init() {
	stateLabelSetCmd.Flags().StringVar(&flagStateFrom, "from", "", "前置状态（可选，CAS 语义）")
	stateLabelSetCmd.Flags().StringVar(&flagStateTo, "to", "", "目标状态（必填，bot:*）")
	_ = stateLabelSetCmd.MarkFlagRequired("to")

	stateLabelNormalizeCmd.Flags().StringVar(&flagStatePriority, "priority", "", "状态优先级，逗号分隔")

	stateLabelCmd.AddCommand(stateLabelSetCmd)
	stateLabelCmd.AddCommand(stateLabelNormalizeCmd)
	stateLabelCmd.AddCommand(stateLabelClearCmd)
}

func runStateLabelSet(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 {
		return fmt.Errorf("必须指定 --repo 和 --issue")
	}

	to, err := state.ParseState(strings.TrimSpace(flagStateTo))
	if err != nil {
		return mapInvalidStateFlagError("--to", err)
	}

	from := state.State("")
	if strings.TrimSpace(flagStateFrom) != "" {
		from, err = state.ParseState(strings.TrimSpace(flagStateFrom))
		if err != nil {
			return mapInvalidStateFlagError("--from", err)
		}
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	if err := state.TransitionWithRetry(ctx, client, flagIssue, from, to, nil); err != nil {
		return mapStateLabelError(err)
	}

	fmt.Printf("issue #%d 状态已迁移到 %s\n", flagIssue, to)
	return nil
}

func runStateLabelNormalize(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 {
		return fmt.Errorf("必须指定 --repo 和 --issue")
	}

	rawPriority := strings.TrimSpace(flagStatePriority)
	if rawPriority == "" {
		rawPriority = strings.TrimSpace(os.Getenv("NIUMA_STATE_PRIORITY"))
	}
	priority, err := state.ParseStatePriority(rawPriority)
	if err != nil {
		return mapInvalidStateFlagError("--priority", err)
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	target, changed, err := state.Normalize(ctx, client, flagIssue, priority)
	if err != nil {
		return mapStateLabelError(err)
	}
	if !changed {
		if target == "" {
			fmt.Printf("issue #%d 当前无 bot 状态，无需收敛\n", flagIssue)
			return nil
		}
		fmt.Printf("issue #%d 已是单一状态 %s，无需收敛\n", flagIssue, target)
		return nil
	}

	fmt.Printf("issue #%d 已自愈收敛到 %s\n", flagIssue, target)
	return nil
}

func mapInvalidStateFlagError(flagName string, err error) error {
	return mapStateLabelError(fmt.Errorf("%s 非法: %w (%v)", flagName, state.ErrInvalidState, err))
}

func runStateLabelClear(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 {
		return fmt.Errorf("必须指定 --repo 和 --issue")
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	changed, err := state.Clear(ctx, client, flagIssue)
	if err != nil {
		return mapStateLabelError(err)
	}
	if !changed {
		fmt.Printf("issue #%d 无 bot 状态可清理\n", flagIssue)
		return nil
	}
	fmt.Printf("issue #%d 已清理全部 bot 状态\n", flagIssue)
	return nil
}

func mapStateLabelError(err error) error {
	switch {
	case errors.Is(err, state.ErrConflict):
		return withExitCode(exitCodeStateConflict, err)
	case errors.Is(err, state.ErrInvalidState):
		return withExitCode(exitCodeStateInvalid, err)
	case errors.Is(err, state.ErrInvalidTransition):
		return withExitCode(exitCodeStateTransitionEdge, err)
	case errors.Is(err, state.ErrInvariantViolation), errors.Is(err, state.ErrMultipleBotStates):
		return withExitCode(exitCodeStateInvariant, err)
	default:
		return err
	}
}
