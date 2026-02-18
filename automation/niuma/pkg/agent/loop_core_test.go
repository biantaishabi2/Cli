package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunLoopCore_ConvergesEarly(t *testing.T) {
	orch := &Orchestrator{issueNumber: 42}
	callCount := 0

	result, err := orch.runLoopCore(
		context.Background(),
		"test",
		"测试",
		5,
		func(_ context.Context, round, maxRounds int) (*state.ConvergenceDecision, error) {
			callCount++
			_ = maxRounds
			if round == 2 {
				return &state.ConvergenceDecision{
					Result: state.ShouldFinalize,
					Reason: "done",
				}, nil
			}
			return &state.ConvergenceDecision{
				Result: state.NotConverged,
				Reason: "continue",
			}, nil
		},
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Converged)
	assert.Equal(t, 2, result.RoundsExecuted)
	assert.Equal(t, 2, callCount)
	assert.Equal(t, state.ShouldFinalize, result.LastDecision.Result)
}

func TestRunLoopCore_ReachesLimit(t *testing.T) {
	orch := &Orchestrator{issueNumber: 42}

	result, err := orch.runLoopCore(
		context.Background(),
		"test",
		"测试",
		3,
		func(_ context.Context, _, _ int) (*state.ConvergenceDecision, error) {
			return &state.ConvergenceDecision{
				Result: state.NotConverged,
				Reason: "continue",
			}, nil
		},
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Converged)
	assert.Equal(t, 3, result.RoundsExecuted)
	assert.Equal(t, state.NotConverged, result.LastDecision.Result)
}

func TestRunLoopCore_RoundError(t *testing.T) {
	orch := &Orchestrator{issueNumber: 7}
	innerErr := errors.New("boom")

	result, err := orch.runLoopCore(
		context.Background(),
		"test",
		"讨论",
		5,
		func(_ context.Context, round, _ int) (*state.ConvergenceDecision, error) {
			if round == 2 {
				return nil, innerErr
			}
			return &state.ConvergenceDecision{
				Result: state.NotConverged,
				Reason: "continue",
			}, nil
		},
	)
	require.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "讨论轮次失败")
	assert.Contains(t, err.Error(), "issue=7")
	assert.Contains(t, err.Error(), "round=2")
	assert.ErrorIs(t, err, innerErr)
}
