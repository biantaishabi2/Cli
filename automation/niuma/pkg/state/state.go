// pkg/state/state.go
// 状态定义和转换合法性校验
package state

import (
	"fmt"
	"strings"
)

// State 状态机的状态类型，对应 GitHub label
type State string

const (
	StateOrchestrate     State = "bot:orchestrate"
	StateQueued          State = "bot:queued"
	StateFixRequested    State = "bot:fix"
	StatePlanDraft       State = "bot:plan-draft"
	StateNeedsDiscussion State = "bot:needs-discussion"
	StatePlanFinal       State = "bot:plan-final"
	StatePlanApproved    State = "bot:plan-approved"
	StateImplementing    State = "bot:implementing"
	StatePRCreated       State = "bot:pr-created"
	StatePRReviewable    State = "bot:pr-reviewable"
	StatePRNeedsFix      State = "bot:pr-needs-fix"
	StateIterating       State = "bot:iterating"
	StateDone            State = "bot:done"
)

// AllStates 所有有效状态
var AllStates = []State{
	StateOrchestrate,
	StateQueued,
	StateFixRequested,
	StatePlanDraft,
	StateNeedsDiscussion,
	StatePlanFinal,
	StatePlanApproved,
	StateImplementing,
	StatePRCreated,
	StatePRReviewable,
	StatePRNeedsFix,
	StateIterating,
	StateDone,
}

// DefaultStatePriority 定义多状态自愈时的默认优先级（从高到低）。
// 优先保障讨论和方案阶段，避免流程误推进到实现/收尾阶段。
var DefaultStatePriority = []State{
	StateNeedsDiscussion,
	StatePlanDraft,
	StateImplementing,
	StateIterating,
	StateFixRequested,
	StatePRReviewable,
	StateDone,
	StateOrchestrate,
	StateQueued,
	StatePlanFinal,
	StatePlanApproved,
	StatePRCreated,
	StatePRNeedsFix,
}

// AllBotLabels 返回所有 bot: 前缀的 label 名
func AllBotLabels() []string {
	labels := make([]string, len(AllStates))
	for i, s := range AllStates {
		labels[i] = string(s)
	}
	return labels
}

// IsBotLabel 判断 label 是否为 bot:* 命名空间。
func IsBotLabel(label string) bool {
	return strings.HasPrefix(strings.TrimSpace(label), "bot:")
}

// validTransitions 定义合法的状态转换
var validTransitions = map[State][]State{
	StateOrchestrate:     {StateQueued},
	StateQueued:          {StateFixRequested},
	StateFixRequested:    {StatePlanDraft},
	StatePlanDraft:       {StateNeedsDiscussion, StatePlanFinal},
	StateNeedsDiscussion: {StatePlanFinal},
	StatePlanFinal:       {StatePlanApproved, StateImplementing}, // 可选人工审批
	StatePlanApproved:    {StateImplementing},
	StateImplementing:    {StatePRCreated},
	StatePRCreated:       {StatePRReviewable, StatePRNeedsFix},
	StatePRReviewable:    {StatePRNeedsFix, StateDone},
	StatePRNeedsFix:      {StateIterating},
	StateIterating:       {StatePRCreated},
	StateDone:            {},
}

// IsValidTransition 检查从 from 到 to 的转换是否合法
func IsValidTransition(from, to State) bool {
	targets, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

// ParseState 将字符串解析为 State，无效则返回错误
func ParseState(s string) (State, error) {
	state := State(s)
	for _, valid := range AllStates {
		if state == valid {
			return state, nil
		}
	}
	return "", fmt.Errorf("invalid state: %q", s)
}

// CollectBotStates 从 labels 中提取所有 bot 状态标签（去重，保序）。
func CollectBotStates(labels []string) []State {
	states, _ := CollectBotStatesWithInvalid(labels)
	return states
}

// CollectBotStatesWithInvalid 从 labels 中提取 bot 状态标签和非法 bot 标签。
func CollectBotStatesWithInvalid(labels []string) ([]State, []string) {
	seen := make(map[State]struct{})
	states := make([]State, 0, len(labels))
	invalid := make([]string, 0)
	for _, label := range labels {
		if !IsBotLabel(label) {
			continue
		}
		s, err := ParseState(label)
		if err != nil {
			invalid = append(invalid, label)
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		states = append(states, s)
	}
	return states, invalid
}

// BuildStateLabels 生成只保留单一 bot 状态的 label 集合（保留非 bot 标签顺序）。
func BuildStateLabels(labels []string, target State) []string {
	next := make([]string, 0, len(labels)+1)
	for _, label := range labels {
		if IsBotLabel(label) {
			continue
		}
		next = append(next, label)
	}
	next = append(next, string(target))
	return next
}

// ResolveStateByPriority 按优先级选择一个状态（常用于多状态自愈）。
func ResolveStateByPriority(states []State, priority []State) (State, bool) {
	if len(states) == 0 {
		return "", false
	}
	if len(priority) == 0 {
		return states[0], true
	}

	index := make(map[State]int, len(priority))
	for i, item := range priority {
		index[item] = i
	}

	best := states[0]
	bestIdx, ok := index[best]
	if !ok {
		bestIdx = len(priority) + 1
	}

	for _, item := range states[1:] {
		idx, ok := index[item]
		if !ok {
			idx = len(priority) + 1
		}
		if idx < bestIdx {
			best = item
			bestIdx = idx
		}
	}
	return best, true
}

// ParseStatePriority 解析优先级配置，格式示例：bot:needs-discussion,bot:plan-draft,bot:fix
func ParseStatePriority(raw string) ([]State, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return append([]State(nil), DefaultStatePriority...), nil
	}

	parts := strings.Split(raw, ",")
	priority := make([]State, 0, len(parts))
	seen := make(map[State]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		s, err := ParseState(part)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		priority = append(priority, s)
	}
	if len(priority) == 0 {
		return nil, fmt.Errorf("state priority is empty")
	}
	return priority, nil
}
