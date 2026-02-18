//go:build !ci

package agent

import (
	"context"
	"sync"
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
		`{"agreements":["第一轮讨论"],"disagreements":[{"topic":"补充边界","options":["A","B"],"recommendation":"A","risk":"medium"}],"decision":"merge","requires_human_decision":false,"should_finish":false}`,
		`{"agreements":["已达成一致"],"disagreements":[],"decision":"merge","requires_human_decision":false,"should_finish":true}`,
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
		`{"agreements":["r1"],"disagreements":[{"topic":"a","options":["A","B"],"recommendation":"A","risk":"medium"}],"decision":"merge","requires_human_decision":false,"should_finish":false}`,
		`{"agreements":["r2"],"disagreements":[{"topic":"b","options":["A","B"],"recommendation":"A","risk":"medium"}],"decision":"merge","requires_human_decision":false,"should_finish":false}`,
		`{"agreements":["r3"],"disagreements":[{"topic":"c","options":["A","B"],"recommendation":"A","risk":"medium"}],"decision":"merge","requires_human_decision":false,"should_finish":false}`,
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
	assert.Contains(t, warnMC.Comment.GetBody(), "未收敛原因")
	assert.Contains(t, warnMC.Comment.GetBody(), "下一步人工决策项")
	assert.Contains(t, warnMC.Comment.GetBody(), "仍有 1 个未决分歧")
	assert.Nil(t, mockGH.GetMarker(1, marker.TypeConvergeWarning))

	// 每轮都更新 summary，revision 应等于 max rounds
	summaryMC := mockGH.GetMarker(1, marker.TypeDiscussionSummary)
	require.NotNil(t, summaryMC)
	assert.Equal(t, 3, summaryMC.Marker.Revision)
}

func TestDoDiscuss_RoundLimitNoticeDoesNotAutoFinalize(t *testing.T) {
	mockAI := ai.NewMockProvider(
		`{"agreements":["r1"],"disagreements":[{"topic":"a","options":["A","B"],"recommendation":"A","risk":"medium"}],"decision":"merge","requires_human_decision":false,"should_finish":false}`,
		`{"agreements":["继续讨论"],"disagreements":[{"topic":"b","options":["A","B"],"recommendation":"A","risk":"medium"}],"decision":"merge","requires_human_decision":false,"should_finish":false}`,
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
		`{"agreements":["继续讨论"],"disagreements":[{"topic":"补充约束","options":["A","B"],"recommendation":"A","risk":"medium"}],"decision":"merge","requires_human_decision":false,"should_finish":false}`,
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

func TestDoDiscuss_ConcurrentDuplicateTriggers_StateRemainsStable(t *testing.T) {
	// 模拟同一 issue 被重复触发 discuss（并发两次），验证不会出现状态损坏或 marker 异常。
	// 该场景固定为“不收敛”，避免并发下汇总/定稿响应串线导致测试抖动。
	mockAI := ai.NewMockProvider(
		`{"agreements":["继续讨论"],"disagreements":[{"topic":"a","options":["A","B"],"recommendation":"A","risk":"medium"}],"decision":"merge","requires_human_decision":false,"should_finish":false}`,
		`{"agreements":["继续讨论"],"disagreements":[{"topic":"b","options":["A","B"],"recommendation":"A","risk":"medium"}],"decision":"merge","requires_human_decision":false,"should_finish":false}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)

	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- orch.DoDiscuss(context.Background(), 1)
		}()
	}
	wg.Wait()
	close(errs)

	// 两次重复触发都应安全完成（不出现状态异常错误）。
	for err := range errs {
		require.NoError(t, err)
	}

	assert.Contains(t, mockGH.Labels[1], string(state.StateNeedsDiscussion))
	assert.Nil(t, mockGH.GetMarker(1, marker.TypePlanFinal))
	require.NotNil(t, mockGH.GetMarker(1, marker.TypeDiscussionSummary))
	require.NotNil(t, mockGH.GetMarker(1, marker.TypeDiscussionRoundLimitNotice))
}

func TestDoDiscuss_ReturnsErrorWhenRoundFails(t *testing.T) {
	mockAI := ai.NewMockProvider(
		`{"agreements":["r1"],"disagreements":[{"topic":"a","options":["A","B"],"recommendation":"A","risk":"medium"}],"decision":"merge","requires_human_decision":false,"should_finish":false}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoDiscuss(context.Background(), 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "round=2")
}
