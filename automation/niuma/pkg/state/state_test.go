//go:build !ci

package state

import (
	"context"
	"errors"
	"testing"
	"time"

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
	labels            map[int][]string
	replaceLabelsCall int
	listSequences     map[int][][]string
}

func newTransitionLabelOpsMock(issueNumber int, labels ...string) *transitionLabelOpsMock {
	return &transitionLabelOpsMock{
		labels: map[int][]string{
			issueNumber: labels,
		},
		listSequences: map[int][][]string{},
	}
}

func (m *transitionLabelOpsMock) ListLabels(_ context.Context, issueNumber int) ([]string, error) {
	if seq := m.listSequences[issueNumber]; len(seq) > 0 {
		current := seq[0]
		m.listSequences[issueNumber] = seq[1:]
		dup := make([]string, len(current))
		copy(dup, current)
		return dup, nil
	}
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

func (m *transitionLabelOpsMock) ReplaceLabels(_ context.Context, issueNumber int, labels []string) error {
	m.replaceLabelsCall++
	next := make([]string, len(labels))
	copy(next, labels)
	m.labels[issueNumber] = next
	return nil
}

func (m *transitionLabelOpsMock) setListSequence(issueNumber int, steps ...[]string) {
	copied := make([][]string, 0, len(steps))
	for _, step := range steps {
		item := make([]string, len(step))
		copy(item, step)
		copied = append(copied, item)
	}
	m.listSequences[issueNumber] = copied
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

func TestTransition_AtomicReplaceKeepsNonBotLabels(t *testing.T) {
	mock := newTransitionLabelOpsMock(325, "bug", string(StatePlanDraft), "priority:high")

	err := Transition(context.Background(), mock, 325, "", StateNeedsDiscussion)
	require.NoError(t, err)
	assert.Equal(t, []string{"bug", "priority:high", string(StateNeedsDiscussion)}, mock.labels[325])
}

func TestTransition_IdempotentNoWrite(t *testing.T) {
	mock := newTransitionLabelOpsMock(325, "bug", string(StateNeedsDiscussion))

	err := Transition(context.Background(), mock, 325, "", StateNeedsDiscussion)
	require.NoError(t, err)
	assert.Equal(t, 0, mock.replaceLabelsCall)
}

func TestTransition_WithFromMismatchReturnsConflict(t *testing.T) {
	mock := newTransitionLabelOpsMock(325, string(StatePlanDraft))

	err := Transition(context.Background(), mock, 325, StateFixRequested, StatePlanDraft)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrConflict))
}

func TestNormalize_PicksPriorityState(t *testing.T) {
	mock := newTransitionLabelOpsMock(325, string(StateFixRequested), string(StateNeedsDiscussion))

	target, changed, err := Normalize(context.Background(), mock, 325, DefaultStatePriority)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, StateNeedsDiscussion, target)
	assert.Equal(t, []string{string(StateNeedsDiscussion)}, mock.labels[325])
}

func TestTransitionWithRetry_ConflictThenSuccess(t *testing.T) {
	mock := newTransitionLabelOpsMock(325, string(StateFixRequested))
	mock.setListSequence(
		325,
		[]string{string(StateFixRequested)},
		[]string{string(StatePlanDraft)},
		[]string{string(StatePlanDraft)},
		[]string{string(StateNeedsDiscussion)},
	)

	err := TransitionWithRetry(
		context.Background(),
		mock,
		325,
		"",
		StateNeedsDiscussion,
		[]time.Duration{0, 0},
	)
	require.NoError(t, err)
	assert.Equal(t, 2, mock.replaceLabelsCall)
	assert.Equal(t, []string{string(StateNeedsDiscussion)}, mock.labels[325])
}
