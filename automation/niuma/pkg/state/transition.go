package state

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrMultipleBotStates 表示同一 issue 上检测到多个 bot 状态。
	ErrMultipleBotStates = errors.New("multiple bot states")
	// ErrFromStateMismatch 表示 from 与当前状态不匹配。
	ErrFromStateMismatch = errors.New("from state mismatch")
	// ErrInvalidTransition 表示 from->to 不在状态机允许边中。
	ErrInvalidTransition = errors.New("invalid state transition")
	// ErrBootstrapTarget 表示无状态 bootstrap 目标非法。
	ErrBootstrapTarget = errors.New("bootstrap target must be bot:queued")
)

// BotStateLabelOps 定义状态迁移所需的最小 label 操作集合。
type BotStateLabelOps interface {
	ListLabels(ctx context.Context, issueNumber int) ([]string, error)
	ReplaceLabelIfPresent(ctx context.Context, issueNumber int, oldLabel, newLabel string) (bool, error)
}

// botStateBootstrapOps 定义 bootstrap（无状态打首个状态）所需能力。
type botStateBootstrapOps interface {
	AddLabel(ctx context.Context, issueNumber int, label string) error
}

// CurrentBotState 从 labels 读取当前 bot 状态；无状态时 found=false。
func CurrentBotState(labels []string) (current State, found bool, err error) {
	states := CollectBotStates(labels)
	switch len(states) {
	case 0:
		return "", false, nil
	case 1:
		return states[0], true, nil
	default:
		return "", false, fmt.Errorf("%w: %v", ErrMultipleBotStates, states)
	}
}

// TransitionBotState 统一执行 bot 状态迁移。
// strict 默认为 true：
// 1) from 必须命中当前状态；
// 2) 必须通过 IsValidTransition(from, to)；
// 3) 无状态仅允许 bootstrap 到 bot:queued。
func TransitionBotState(
	ctx context.Context,
	ops BotStateLabelOps,
	issueNumber int,
	from State,
	to State,
	strict ...bool,
) error {
	enforce := true
	if len(strict) > 0 {
		enforce = strict[0]
	}

	labels, err := ops.ListLabels(ctx, issueNumber)
	if err != nil {
		return fmt.Errorf("读取 issue #%d labels 失败: %w", issueNumber, err)
	}

	current, found, err := CurrentBotState(labels)
	if err != nil {
		return fmt.Errorf("读取 issue #%d 当前 bot 状态失败: %w", issueNumber, err)
	}

	// 兼容模式：尽量保留旧行为，允许无状态直接打目标标签。
	if !enforce {
		if found {
			replaced, err := ops.ReplaceLabelIfPresent(ctx, issueNumber, string(current), string(to))
			if err != nil {
				return fmt.Errorf("迁移 issue #%d 状态失败: %w", issueNumber, err)
			}
			if replaced {
				return nil
			}
			return fmt.Errorf("迁移 issue #%d 状态失败: %w", issueNumber, ErrFromStateMismatch)
		}
		bootstrap, ok := ops.(botStateBootstrapOps)
		if !ok {
			return fmt.Errorf("迁移 issue #%d 状态失败: 缺少 AddLabel 能力", issueNumber)
		}
		if err := bootstrap.AddLabel(ctx, issueNumber, string(to)); err != nil {
			return fmt.Errorf("为 issue #%d 添加 bootstrap 状态失败: %w", issueNumber, err)
		}
		return nil
	}

	if !found {
		if from != "" {
			return fmt.Errorf("迁移 issue #%d 状态失败: %w (want=%s got=<none>)", issueNumber, ErrFromStateMismatch, from)
		}
		if to != StateQueued {
			return fmt.Errorf("迁移 issue #%d 状态失败: %w (to=%s)", issueNumber, ErrBootstrapTarget, to)
		}

		bootstrap, ok := ops.(botStateBootstrapOps)
		if !ok {
			return fmt.Errorf("迁移 issue #%d 状态失败: 缺少 AddLabel 能力", issueNumber)
		}
		if err := bootstrap.AddLabel(ctx, issueNumber, string(to)); err != nil {
			return fmt.Errorf("为 issue #%d 添加 bootstrap 状态失败: %w", issueNumber, err)
		}
		return nil
	}

	if from == "" {
		return fmt.Errorf("迁移 issue #%d 状态失败: strict 模式必须提供 from", issueNumber)
	}
	if current != from {
		return fmt.Errorf("迁移 issue #%d 状态失败: %w (want=%s got=%s)", issueNumber, ErrFromStateMismatch, from, current)
	}
	if !IsValidTransition(from, to) {
		return fmt.Errorf("迁移 issue #%d 状态失败: %w (%s->%s)", issueNumber, ErrInvalidTransition, from, to)
	}

	replaced, err := ops.ReplaceLabelIfPresent(ctx, issueNumber, string(from), string(to))
	if err != nil {
		return fmt.Errorf("迁移 issue #%d 状态失败: %w", issueNumber, err)
	}
	if !replaced {
		return fmt.Errorf("迁移 issue #%d 状态失败: %w (from=%s)", issueNumber, ErrFromStateMismatch, from)
	}
	return nil
}
