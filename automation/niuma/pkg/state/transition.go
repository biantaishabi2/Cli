package state

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrMultipleBotStates 保持兼容：同一 issue 上检测到多个 bot 状态。
	ErrMultipleBotStates = errors.New("multiple bot states")
	// ErrInvariantViolation 表示 issue 存在多个 bot 状态等不变量破坏。
	ErrInvariantViolation = ErrMultipleBotStates
	// ErrFromStateMismatch 保持兼容：from 与当前状态不匹配。
	ErrFromStateMismatch = errors.New("from state mismatch")
	// ErrConflict 表示 CAS 前置状态不满足或并发覆盖。
	ErrConflict = ErrFromStateMismatch
	// ErrInvalidState 表示 from/to 或现存 bot 标签非法。
	ErrInvalidState = errors.New("invalid state")

	// ErrInvalidTransition 保持兼容：from->to 不在状态机允许边中。
	ErrInvalidTransition = errors.New("invalid state transition")
	// ErrBootstrapTarget 保持兼容：无状态 bootstrap 目标非法。
	ErrBootstrapTarget = errors.New("bootstrap target must be bot:queued")
)

// BotStateLabelOps 定义状态迁移所需的最小 label 操作集合。
type BotStateLabelOps interface {
	ListLabels(ctx context.Context, issueNumber int) ([]string, error)
	ReplaceLabels(ctx context.Context, issueNumber int, labels []string) error
}

// CurrentBotState 从 labels 读取当前 bot 状态；无状态时 found=false。
func CurrentBotState(labels []string) (current State, found bool, err error) {
	states, invalid := CollectBotStatesWithInvalid(labels)
	if len(invalid) > 0 {
		return "", false, fmt.Errorf("%w: %v", ErrInvalidState, invalid)
	}
	switch len(states) {
	case 0:
		return "", false, nil
	case 1:
		return states[0], true, nil
	default:
		return "", false, fmt.Errorf("%w: %v", ErrInvariantViolation, states)
	}
}

// Transition 统一执行 bot 状态迁移（原子替换语义）。
// - to 必须为合法 bot 状态
// - from 为空表示不做前置约束；非空时执行 CAS 约束
// - 迁移写入使用一次 ReplaceLabels，保证最终仅一个 bot 状态标签
func Transition(ctx context.Context, ops BotStateLabelOps, issueNumber int, from, to State) error {
	if _, err := ParseState(string(to)); err != nil {
		return fmt.Errorf("迁移 issue #%d 状态失败: %w (to=%s)", issueNumber, ErrInvalidState, to)
	}

	if from != "" {
		if _, err := ParseState(string(from)); err != nil {
			return fmt.Errorf("迁移 issue #%d 状态失败: %w (from=%s)", issueNumber, ErrInvalidState, from)
		}
		if from != to && !IsValidTransition(from, to) {
			return fmt.Errorf("迁移 issue #%d 状态失败: %w (%s->%s)", issueNumber, ErrInvalidTransition, from, to)
		}
	}

	labels, err := ops.ListLabels(ctx, issueNumber)
	if err != nil {
		return fmt.Errorf("读取 issue #%d labels 失败: %w", issueNumber, err)
	}

	states, invalid := CollectBotStatesWithInvalid(labels)
	if len(invalid) > 0 {
		return fmt.Errorf("迁移 issue #%d 状态失败: %w (invalid=%v)", issueNumber, ErrInvalidState, invalid)
	}
	if len(states) > 1 {
		return fmt.Errorf("迁移 issue #%d 状态失败: %w (states=%v)", issueNumber, ErrInvariantViolation, states)
	}

	current := State("")
	found := len(states) == 1
	if found {
		current = states[0]
	}

	if from != "" {
		if !found || current != from {
			actual := "<none>"
			if found {
				actual = string(current)
			}
			return fmt.Errorf("迁移 issue #%d 状态失败: %w (want=%s got=%s)", issueNumber, ErrConflict, from, actual)
		}
	}

	// 幂等：当前已是目标状态且不变量成立。
	if found && current == to {
		return nil
	}

	nextLabels := BuildStateLabels(labels, to)
	if equalStringSlices(labels, nextLabels) {
		return nil
	}

	if err := ops.ReplaceLabels(ctx, issueNumber, nextLabels); err != nil {
		return fmt.Errorf("迁移 issue #%d 状态失败: %w", issueNumber, err)
	}

	// 写后校验：若被并发覆盖，尽量显式返回冲突。
	verifyLabels, err := ops.ListLabels(ctx, issueNumber)
	if err != nil {
		return fmt.Errorf("迁移 issue #%d 状态后校验失败: %w", issueNumber, err)
	}
	verified, ok, err := CurrentBotState(verifyLabels)
	if err != nil {
		return fmt.Errorf("迁移 issue #%d 状态后校验失败: %w", issueNumber, err)
	}
	if !ok || verified != to {
		actual := "<none>"
		if ok {
			actual = string(verified)
		}
		return fmt.Errorf("迁移 issue #%d 状态失败: %w (target=%s got=%s)", issueNumber, ErrConflict, to, actual)
	}

	return nil
}

// TransitionWithRetry 在冲突时按退避重试。
func TransitionWithRetry(ctx context.Context, ops BotStateLabelOps, issueNumber int, from, to State, backoffs []time.Duration) error {
	if len(backoffs) == 0 {
		backoffs = []time.Duration{100 * time.Millisecond, 300 * time.Millisecond, 900 * time.Millisecond}
	}

	var lastErr error
	for i := 0; i <= len(backoffs); i++ {
		err := Transition(ctx, ops, issueNumber, from, to)
		if err == nil {
			return nil
		}
		lastErr = err
		if !errors.Is(err, ErrConflict) {
			return err
		}
		if i == len(backoffs) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoffs[i]):
		}
	}
	return lastErr
}

// Normalize 收敛多状态为单状态（按优先级），并返回收敛后的状态。
func Normalize(ctx context.Context, ops BotStateLabelOps, issueNumber int, priority []State) (State, bool, error) {
	labels, err := ops.ListLabels(ctx, issueNumber)
	if err != nil {
		return "", false, fmt.Errorf("读取 issue #%d labels 失败: %w", issueNumber, err)
	}

	states, invalid := CollectBotStatesWithInvalid(labels)
	if len(invalid) > 0 {
		return "", false, fmt.Errorf("收敛 issue #%d 状态失败: %w (invalid=%v)", issueNumber, ErrInvalidState, invalid)
	}
	if len(states) == 0 {
		return "", false, nil
	}
	if len(states) == 1 {
		return states[0], false, nil
	}

	target, ok := ResolveStateByPriority(states, priority)
	if !ok {
		return "", false, fmt.Errorf("收敛 issue #%d 状态失败: %w (no target)", issueNumber, ErrInvariantViolation)
	}

	nextLabels := BuildStateLabels(labels, target)
	if err := ops.ReplaceLabels(ctx, issueNumber, nextLabels); err != nil {
		return "", false, fmt.Errorf("收敛 issue #%d 状态失败: %w", issueNumber, err)
	}

	return target, true, nil
}

// Clear 移除 issue 上所有 bot 状态标签。
func Clear(ctx context.Context, ops BotStateLabelOps, issueNumber int) (bool, error) {
	labels, err := ops.ListLabels(ctx, issueNumber)
	if err != nil {
		return false, fmt.Errorf("读取 issue #%d labels 失败: %w", issueNumber, err)
	}
	_, invalid := CollectBotStatesWithInvalid(labels)
	if len(invalid) > 0 {
		return false, fmt.Errorf("清理 issue #%d 状态失败: %w (invalid=%v)", issueNumber, ErrInvalidState, invalid)
	}

	next := make([]string, 0, len(labels))
	changed := false
	for _, label := range labels {
		if IsBotLabel(label) {
			changed = true
			continue
		}
		next = append(next, label)
	}
	if !changed {
		return false, nil
	}
	if err := ops.ReplaceLabels(ctx, issueNumber, next); err != nil {
		return false, fmt.Errorf("清理 issue #%d 状态失败: %w", issueNumber, err)
	}
	return true, nil
}

// TransitionBotState 保持兼容，默认 strict=true。
func TransitionBotState(ctx context.Context, ops BotStateLabelOps, issueNumber int, from State, to State, strict ...bool) error {
	enforce := true
	if len(strict) > 0 {
		enforce = strict[0]
	}

	if enforce {
		if from == "" {
			if to != StateQueued {
				return fmt.Errorf("迁移 issue #%d 状态失败: %w (to=%s)", issueNumber, ErrBootstrapTarget, to)
			}
			return Transition(ctx, ops, issueNumber, "", to)
		}
		return Transition(ctx, ops, issueNumber, from, to)
	}

	// 兼容非 strict：允许无前置条件直接迁移。
	return Transition(ctx, ops, issueNumber, "", to)
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
