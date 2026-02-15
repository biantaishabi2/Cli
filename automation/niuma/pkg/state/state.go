// pkg/state/state.go
// 状态定义和转换合法性校验
package state

import "fmt"

// State 状态机的状态类型，对应 GitHub label
type State string

const (
	StateFixRequested    State = "bot:fix"
	StatePlanDraft       State = "bot:plan-draft"
	StateNeedsDiscussion State = "bot:needs-discussion"
	StatePlanFinal       State = "bot:plan-final"
	StateImplementing    State = "bot:implementing"
	StatePRCreated       State = "bot:pr-created"
	StatePRReviewable    State = "bot:pr-reviewable"
	StatePRNeedsFix      State = "bot:pr-needs-fix"
	StateIterating       State = "bot:iterating"
	StateDone            State = "bot:done"
)

// AllStates 所有有效状态
var AllStates = []State{
	StateFixRequested,
	StatePlanDraft,
	StateNeedsDiscussion,
	StatePlanFinal,
	StateImplementing,
	StatePRCreated,
	StatePRReviewable,
	StatePRNeedsFix,
	StateIterating,
	StateDone,
}

// AllBotLabels 返回所有 bot: 前缀的 label 名
func AllBotLabels() []string {
	labels := make([]string, len(AllStates))
	for i, s := range AllStates {
		labels[i] = string(s)
	}
	return labels
}

// validTransitions 定义合法的状态转换
var validTransitions = map[State][]State{
	StateFixRequested:    {StatePlanDraft},
	StatePlanDraft:       {StateNeedsDiscussion, StatePlanFinal},
	StateNeedsDiscussion: {StatePlanFinal},
	StatePlanFinal:       {StateImplementing},
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
