//go:build !ci

package state

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestIsValidTransition_PlanApproval(t *testing.T) {
	// 人工审批路径
	assert.True(t, IsValidTransition(StatePlanFinal, StatePlanApproved))
	assert.True(t, IsValidTransition(StatePlanApproved, StateImplementing))
	// 跳过审批直接实现（自动模式）
	assert.True(t, IsValidTransition(StatePlanFinal, StateImplementing))
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
	assert.Len(t, AllStates, 13)
}

func TestAllBotLabels(t *testing.T) {
	labels := AllBotLabels()
	assert.Len(t, labels, 13)
	assert.Equal(t, "bot:orchestrate", labels[0])
}

type transitionLabelOpsMock struct {
	labels map[int][]string
}

func newTransitionLabelOpsMock(issueNumber int, labels ...string) *transitionLabelOpsMock {
	return &transitionLabelOpsMock{
		labels: map[int][]string{
			issueNumber: labels,
		},
	}
}

func (m *transitionLabelOpsMock) ListLabels(_ context.Context, issueNumber int) ([]string, error) {
	current := m.labels[issueNumber]
	dup := make([]string, len(current))
	copy(dup, current)
	return dup, nil
}

func (m *transitionLabelOpsMock) ReplaceLabelIfPresent(_ context.Context, issueNumber int, oldLabel, newLabel string) (bool, error) {
	labels := m.labels[issueNumber]
	for i, label := range labels {
		if label == oldLabel {
			labels[i] = newLabel
			m.labels[issueNumber] = labels
			return true, nil
		}
	}
	return false, nil
}

func (m *transitionLabelOpsMock) AddLabel(_ context.Context, issueNumber int, label string) error {
	m.labels[issueNumber] = append(m.labels[issueNumber], label)
	return nil
}

func TestTransitionBotState_FromMismatchRejected(t *testing.T) {
	mock := newTransitionLabelOpsMock(1, string(StateImplementing))

	err := TransitionBotState(context.Background(), mock, 1, StateQueued, StateFixRequested)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFromStateMismatch))
	assert.Equal(t, []string{string(StateImplementing)}, mock.labels[1])
}

func TestTransitionBotState_InvalidEdgeRejected(t *testing.T) {
	mock := newTransitionLabelOpsMock(1, string(StateFixRequested))

	err := TransitionBotState(context.Background(), mock, 1, StateFixRequested, StatePRCreated)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidTransition))
	assert.Equal(t, []string{string(StateFixRequested)}, mock.labels[1])
}

func TestTransitionBotState_DirtyMultipleStatesRejected(t *testing.T) {
	mock := newTransitionLabelOpsMock(1, string(StateFixRequested), string(StateImplementing))

	err := TransitionBotState(context.Background(), mock, 1, StateFixRequested, StatePlanDraft)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMultipleBotStates))
	assert.Equal(t, []string{string(StateFixRequested), string(StateImplementing)}, mock.labels[1])
}

func TestTransitionBotState_BootstrapOnlyQueued(t *testing.T) {
	mock := newTransitionLabelOpsMock(1)
	err := TransitionBotState(context.Background(), mock, 1, "", StateFixRequested)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrBootstrapTarget))

	err = TransitionBotState(context.Background(), mock, 1, "", StateQueued)
	require.NoError(t, err)
	assert.Equal(t, []string{string(StateQueued)}, mock.labels[1])
}
