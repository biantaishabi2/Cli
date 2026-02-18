//go:build !ci

package agent

import (
	"context"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoDiscuss_ConsolidateMode_Default(t *testing.T) {
	consolidator := ai.NewMockProvider(
		`{"agreements":["已收敛"],"disagreements":[],"decision":"merge","requires_human_decision":false,"should_finish":true}`,
		`{"title":"最终方案","approach":"按共识实现","file_changes":[{"path":"src/a.go","action":"modify","description":"x"}],"test_scenarios":[{"name":"n","input":"i","expected":"o"}]}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestratorWithConfig(mockGH, 1, &OrchestratorConfig{
		DiscussionProviders: []ai.Provider{ai.NewMockProvider(`{"summary":"unused"}`)},
		Consolidator:        consolidator,
		DiscussionMode:      "consolidate",
	})

	require.NoError(t, orch.DoDiscuss(context.Background(), 2))
	assert.Contains(t, mockGH.Labels[1], string(state.StatePlanFinal))
	require.NotNil(t, mockGH.GetMarker(1, marker.TypePlanFinal))
}

func TestDoDiscuss_DebateABMode_AlternatingAndFinalize(t *testing.T) {
	providerA := ai.NewMockProvider(
		`{"agreements":["a1"],"disagreements":[{"topic":"t1","options":["A","B"],"recommendation":"A","risk":"low"}],"suggestion":"继续"}`,
	)
	providerB := ai.NewMockProvider(
		`{"agreements":["a2"],"disagreements":[],"suggestion":"可定稿"}`,
	)
	consolidator := ai.NewMockProvider(
		`{"title":"最终方案","approach":"按收敛结果实现","file_changes":[{"path":"src/b.go","action":"modify","description":"x"}],"test_scenarios":[{"name":"n","input":"i","expected":"o"}]}`,
	)

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestratorWithConfig(mockGH, 1, &OrchestratorConfig{
		DiscussionProviders: []ai.Provider{providerA, providerB},
		Consolidator:        consolidator,
		DiscussionMode:      "debate_ab",
		VisibleOnlyOnDiff:   true,
	})

	require.NoError(t, orch.DoDiscuss(context.Background(), 3))
	assert.Equal(t, 1, providerA.CallCount())
	assert.Equal(t, 1, providerB.CallCount())
	assert.Contains(t, mockGH.Labels[1], string(state.StatePlanFinal))

	comments := mockGH.Comments[1]
	joined := ""
	for _, c := range comments {
		joined += c.GetBody() + "\n"
	}
	assert.Contains(t, joined, "Debate A")
	assert.Contains(t, joined, "Debate B")
}

func TestShouldPublishDiscussionSummary_VisibleOnlyOnDiff(t *testing.T) {
	orch := NewOrchestratorWithConfig(NewMockGitHub(), 1, &OrchestratorConfig{
		DiscussionProviders:  []ai.Provider{ai.NewMockProvider(`{"summary":"unused"}`)},
		Consolidator:         ai.NewMockProvider(`{"summary":"unused"}`),
		VisibleRoundInterval: 1,
		VisibleOnlyOnDiff:    true,
	})
	summary := &DiscussionSummary{
		Decision:              DecisionMerge,
		RequiresHumanDecision: false,
		Disagreements: []DisagreementItem{
			{Topic: "t1", Options: []string{"A", "B"}, Recommendation: "A", Risk: RiskMedium},
		},
	}
	previous := &gh.MarkerComment{
		Marker: &marker.Marker{
			Decision:      "merge",
			Human:         false,
			Risk:          "medium",
			DisagreeCount: 1,
		},
	}

	assert.False(t, orch.shouldPublishDiscussionSummary(2, previous, summary))

	summary.Disagreements = append(summary.Disagreements, DisagreementItem{
		Topic: "t2", Options: []string{"A", "B"}, Recommendation: "B", Risk: RiskLow,
	})
	assert.True(t, orch.shouldPublishDiscussionSummary(3, previous, summary))
}
