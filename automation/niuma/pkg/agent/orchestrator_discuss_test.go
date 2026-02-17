//go:build !ci

package agent

import (
	"context"
	"testing"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoDiscuss_ConvergesAndFinalize(t *testing.T) {
	mockAI := ai.NewMockProvider(
		`{"consensus":"第一轮讨论","open_items":["补充边界"],"should_finish":false}`,
		`{"consensus":"已达成一致","open_items":[],"should_finish":true}`,
		`{"title":"最终方案","approach":"按共识实现","file_changes":[{"path":"src/login.go","action":"modify","description":"修复编码"}],"test_scenarios":[{"name":"特殊字符","input":"p@ss","expected":"success"}]}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoDiscuss(context.Background(), 5)
	require.NoError(t, err)

	// 已收敛并定稿
	finalMC := mockGH.GetMarker(1, marker.TypePlanFinal)
	require.NotNil(t, finalMC)
	assert.Contains(t, mockGH.Labels[1], string(state.StatePlanFinal))

	// discussion summary 按轮次 upsert
	summaryMC := mockGH.GetMarker(1, marker.TypeDiscussionSummary)
	require.NotNil(t, summaryMC)
	assert.Equal(t, 2, summaryMC.Marker.Revision)
}

func TestDoDiscuss_StopsAtRoundLimit(t *testing.T) {
	mockAI := ai.NewMockProvider(
		`{"consensus":"r1","open_items":["a"],"should_finish":false}`,
		`{"consensus":"r2","open_items":["b"],"should_finish":false}`,
		`{"consensus":"r3","open_items":["c"],"should_finish":false}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoDiscuss(context.Background(), 3)
	require.NoError(t, err)

	// 未收敛时保持在 needs-discussion，不进入 plan-final
	assert.Contains(t, mockGH.Labels[1], string(state.StateNeedsDiscussion))
	assert.Nil(t, mockGH.GetMarker(1, marker.TypePlanFinal))

	// 轮次上限提醒使用 marker upsert
	warnMC := mockGH.GetMarker(1, marker.TypeDiscussionRoundLimitNotice)
	require.NotNil(t, warnMC)
	assert.Contains(t, warnMC.Comment.GetBody(), "讨论已达自动轮次上限")
	assert.Nil(t, mockGH.GetMarker(1, marker.TypeConvergeWarning))

	// 每轮都更新 summary，revision 应等于 max rounds
	summaryMC := mockGH.GetMarker(1, marker.TypeDiscussionSummary)
	require.NotNil(t, summaryMC)
	assert.Equal(t, 3, summaryMC.Marker.Revision)
}

func TestDoDiscuss_RoundLimitNoticeDoesNotAutoFinalize(t *testing.T) {
	mockAI := ai.NewMockProvider(
		`{"consensus":"r1","open_items":["a"],"should_finish":false}`,
		`{"consensus":"继续讨论","open_items":["b"],"should_finish":false}`,
		`{"title":"最终方案","approach":"按共识实现","file_changes":[{"path":"src/login.go","action":"modify","description":"修复编码"}],"test_scenarios":[{"name":"特殊字符","input":"p@ss","expected":"success"}]}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	require.NoError(t, orch.DoDiscuss(context.Background(), 1))

	noticeMC := mockGH.GetMarker(1, marker.TypeDiscussionRoundLimitNotice)
	require.NotNil(t, noticeMC)

	now := time.Now()
	noticeMC.Comment.CreatedAt = &github.Timestamp{Time: now.Add(-31 * time.Minute)}
	mockGH.Comments[1] = append(mockGH.Comments[1], &github.IssueComment{
		ID:        github.Ptr(int64(999)),
		Body:      github.Ptr("人工补充了新的约束条件"),
		CreatedAt: &github.Timestamp{Time: now},
	})

	require.NoError(t, orch.DoDiscussionCheck(context.Background()))
	assert.Nil(t, mockGH.GetMarker(1, marker.TypePlanFinal))
	assert.Contains(t, mockGH.Labels[1], string(state.StateNeedsDiscussion))
}

func TestDoDiscussionCheck_StaleConvergeWarningDoesNotAutoFinalize(t *testing.T) {
	mockAI := ai.NewMockProvider(
		`{"consensus":"继续讨论","open_items":["补充约束"],"should_finish":false}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypeConvergeWarning, Issue: 1, Revision: 1,
	}, "warning")

	now := time.Now()
	warnMC := mockGH.GetMarker(1, marker.TypeConvergeWarning)
	require.NotNil(t, warnMC)
	warnMC.Comment.CreatedAt = &github.Timestamp{Time: now.Add(-31 * time.Minute)}

	mockGH.Comments[1] = []*github.IssueComment{
		{
			ID:        github.Ptr(int64(1001)),
			Body:      github.Ptr("人工补充了新的约束条件"),
			CreatedAt: &github.Timestamp{Time: now},
		},
	}

	orch := NewOrchestrator(mockGH, mockAI, 1)
	require.NoError(t, orch.DoDiscussionCheck(context.Background()))

	assert.Nil(t, mockGH.GetMarker(1, marker.TypePlanFinal))
	assert.Contains(t, mockGH.Labels[1], string(state.StateNeedsDiscussion))
	assert.Equal(t, 1, mockGH.GetMarker(1, marker.TypeConvergeWarning).Marker.Revision)
}

func TestDoDiscuss_ReturnsErrorWhenRoundFails(t *testing.T) {
	mockAI := ai.NewMockProvider(
		`{"consensus":"r1","open_items":["a"],"should_finish":false}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoDiscuss(context.Background(), 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "round=2")
}
