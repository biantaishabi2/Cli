package control

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouteEvent_OrchestrateIssuesLabel(t *testing.T) {
	decision, err := RouteEvent("orchestrate", "issues", []byte(`{"label":{"name":"bot:premerged"}}`))
	require.NoError(t, err)
	assert.Equal(t, DecisionRun, decision.Decision)
	assert.Equal(t, ReasonRunRoutable, decision.Reason)
	assert.Equal(t, ActionOrchestrate, decision.Action)

	decision, err = RouteEvent("orchestrate", "issues", []byte(`{"label":{"name":"bot:unknown"}}`))
	require.NoError(t, err)
	assert.Equal(t, DecisionSkip, decision.Decision)
	assert.Equal(t, ReasonSkipLabelNotRoutable, decision.Reason)
}

func TestRouteEvent_OrchestrateDispatchSourceGate(t *testing.T) {
	okPayload := []byte(`{"action":"niuma.task.completed","client_payload":{"event_source":"close-after-integration-merge"}}`)
	skipPayload := []byte(`{"action":"niuma.task.completed","client_payload":{"event_source":"other-source"}}`)

	decision, err := RouteEvent("orchestrate", "repository_dispatch", okPayload)
	require.NoError(t, err)
	assert.Equal(t, DecisionRun, decision.Decision)

	decision, err = RouteEvent("orchestrate", "repository_dispatch", skipPayload)
	require.NoError(t, err)
	assert.Equal(t, DecisionSkip, decision.Decision)
	assert.Equal(t, ReasonSkipDispatchSourceNotAllowed, decision.Reason)
}

func TestRouteEvent_P1IssueLabelWorkflow(t *testing.T) {
	decision, err := RouteEvent("plan", "issues", []byte(`{"label":{"name":"bot:fix"}}`))
	require.NoError(t, err)
	assert.Equal(t, DecisionRun, decision.Decision)
	assert.Equal(t, ActionPlan, decision.Action)

	decision, err = RouteEvent("implement", "issues", []byte(`{"label":{"name":"bot:plan-approved"}}`))
	require.NoError(t, err)
	assert.Equal(t, DecisionRun, decision.Decision)
	assert.Equal(t, ActionImplement, decision.Action)

	decision, err = RouteEvent("review", "issues", []byte(`{"label":{"name":"bot:other"}}`))
	require.NoError(t, err)
	assert.Equal(t, DecisionSkip, decision.Decision)
	assert.Equal(t, ReasonSkipLabelNotRoutable, decision.Reason)
}
