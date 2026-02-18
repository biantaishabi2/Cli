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

func TestDoDiscuss_DebateABMode_Default(t *testing.T) {
	providerA := ai.NewMockProvider(
		"第一轮先补充边界条件。\n```json\n{\"should_finish\":false}\n```",
	)
	providerB := ai.NewMockProvider(
		"第二轮已达成一致，可以定稿。\n```json\n{\"should_finish\":true}\n```",
	)
	finalProvider := ai.NewMockProvider(
		`{"title":"最终方案","approach":"按共识实现","file_changes":[{"path":"src/a.go","action":"modify","description":"x"}],"test_scenarios":[{"name":"n","input":"i","expected":"o"}]}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestratorWithConfig(mockGH, 1, &OrchestratorConfig{
		DiscussionProviders: []ai.Provider{providerA, providerB},
		ImplementProvider:   finalProvider,
	})

	require.NoError(t, orch.DoDiscuss(context.Background(), 2))
	assert.Contains(t, mockGH.Labels[1], string(state.StatePlanFinal))
	require.NotNil(t, mockGH.GetMarker(1, marker.TypePlanFinal))
}

func TestDoDiscuss_DebateABMode_AlternatingAndFinalize(t *testing.T) {
	providerA := ai.NewMockProvider(
		"A 方建议继续讨论。\n```json\n{\"should_finish\":false}\n```",
	)
	providerB := ai.NewMockProvider(
		"B 方确认收敛。\n```json\n{\"should_finish\":true}\n```",
	)
	consolidator := ai.NewMockProvider(
		`{"title":"最终方案","approach":"按收敛结果实现","file_changes":[{"path":"src/b.go","action":"modify","description":"x"}],"test_scenarios":[{"name":"n","input":"i","expected":"o"}]}`,
	)

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestratorWithConfig(mockGH, 1, &OrchestratorConfig{
		DiscussionProviders: []ai.Provider{providerA, providerB},
		ImplementProvider:   consolidator,
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
		VisibleRoundInterval: 1,
		VisibleOnlyOnDiff:    true,
	})
	summary := &DiscussionSummary{
		ShouldFinish: false,
	}
	previous := &gh.MarkerComment{
		Marker: &marker.Marker{
			Finish: false,
		},
	}

	assert.False(t, orch.shouldPublishDiscussionSummary(2, previous, summary))

	summary.ShouldFinish = true
	assert.True(t, orch.shouldPublishDiscussionSummary(3, previous, summary))
}
