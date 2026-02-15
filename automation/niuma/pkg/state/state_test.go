package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidTransition_HappyPath(t *testing.T) {
	// 正常流程全路径
	transitions := []struct{ from, to State }{
		{StateFixRequested, StatePlanDraft},
		{StatePlanDraft, StateNeedsDiscussion},
		{StateNeedsDiscussion, StatePlanFinal},
		{StatePlanFinal, StateImplementing},
		{StateImplementing, StatePRCreated},
		{StatePRCreated, StatePRReviewable},
		{StatePRReviewable, StateDone},
	}
	for _, tt := range transitions {
		assert.True(t, IsValidTransition(tt.from, tt.to),
			"expected %s → %s to be valid", tt.from, tt.to)
	}
}

func TestIsValidTransition_SkipDiscussion(t *testing.T) {
	// 信息充分时可以跳过讨论
	assert.True(t, IsValidTransition(StatePlanDraft, StatePlanFinal))
}

func TestIsValidTransition_PRNeedsFix(t *testing.T) {
	assert.True(t, IsValidTransition(StatePRCreated, StatePRNeedsFix))
	assert.True(t, IsValidTransition(StatePRReviewable, StatePRNeedsFix))
	assert.True(t, IsValidTransition(StatePRNeedsFix, StateIterating))
	assert.True(t, IsValidTransition(StateIterating, StatePRCreated))
}

func TestIsValidTransition_InvalidBackward(t *testing.T) {
	// 不允许回退
	assert.False(t, IsValidTransition(StatePlanFinal, StatePlanDraft))
	assert.False(t, IsValidTransition(StateDone, StateFixRequested))
	assert.False(t, IsValidTransition(StatePRReviewable, StatePlanFinal))
}

func TestIsValidTransition_InvalidSkip(t *testing.T) {
	// 不允许跳步
	assert.False(t, IsValidTransition(StateFixRequested, StatePlanFinal))
	assert.False(t, IsValidTransition(StatePlanDraft, StateImplementing))
	assert.False(t, IsValidTransition(StateNeedsDiscussion, StatePRCreated))
}

func TestIsValidTransition_SameState(t *testing.T) {
	assert.False(t, IsValidTransition(StatePlanDraft, StatePlanDraft))
}

func TestIsValidTransition_DoneIsTerminal(t *testing.T) {
	for _, s := range AllStates {
		if s == StateDone {
			continue
		}
		assert.False(t, IsValidTransition(StateDone, s),
			"done → %s should be invalid", s)
	}
}

func TestIsValidTransition_UnknownState(t *testing.T) {
	assert.False(t, IsValidTransition(State("unknown"), StatePlanDraft))
	assert.False(t, IsValidTransition(StateFixRequested, State("unknown")))
}

func TestParseState_Valid(t *testing.T) {
	for _, s := range AllStates {
		parsed, err := ParseState(string(s))
		assert.NoError(t, err)
		assert.Equal(t, s, parsed)
	}
}

func TestParseState_Invalid(t *testing.T) {
	_, err := ParseState("invalid")
	assert.Error(t, err)

	_, err = ParseState("")
	assert.Error(t, err)

	_, err = ParseState("bot:unknown")
	assert.Error(t, err)
}

func TestAllStates_Count(t *testing.T) {
	assert.Len(t, AllStates, 10)
}

func TestAllBotLabels(t *testing.T) {
	labels := AllBotLabels()
	assert.Len(t, labels, 10)
	assert.Equal(t, "bot:fix", labels[0])
}
