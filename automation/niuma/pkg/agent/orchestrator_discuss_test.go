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
		"第一轮讨论：补充边界。\n```json\n{\"should_finish\":false}\n```",
		"第二轮讨论：已达成一致。\n```json\n{\"should_finish\":true}\n```",
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
		"r1 继续讨论。\n```json\n{\"should_finish\":false}\n```",
		"r2 继续讨论。\n```json\n{\"should_finish\":false}\n```",
		"r3 继续讨论。\n```json\n{\"should_finish\":false}\n```",
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
	assert.Contains(t, warnMC.Comment.GetBody(), "达到轮次上限后仍未满足自动定稿条件")
	assert.Nil(t, mockGH.GetMarker(1, marker.TypeConvergeWarning))

	// 每轮都更新 summary，revision 应等于 max rounds
	summaryMC := mockGH.GetMarker(1, marker.TypeDiscussionSummary)
	require.NotNil(t, summaryMC)
	assert.Equal(t, 3, summaryMC.Marker.Revision)
}

func TestDoDiscuss_RoundLimitNoticeDoesNotAutoFinalize(t *testing.T) {
	mockAI := ai.NewMockProvider(
		"r1 继续讨论。\n```json\n{\"should_finish\":false}\n```",
		"继续讨论。\n```json\n{\"should_finish\":false}\n```",
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
		"继续讨论，补充约束。\n```json\n{\"should_finish\":false}\n```",
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
		"继续讨论 a。\n```json\n{\"should_finish\":false}\n```",
		"继续讨论 b。\n```json\n{\"should_finish\":false}\n```",
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

// ===== PlanFinal 重试与降级集成测试 =====

func TestDoPlanFinal_RetryOnNonJSON(t *testing.T) {
	// 主 provider 第 1 次返回纯文本，第 2 次返回有效 JSON
	mockAI := ai.NewMockProvider(
		"This is a plan in natural language, not JSON.",
		`{"title":"最终方案","approach":"按共识实现","file_changes":[{"path":"src/login.go","action":"modify","description":"修复编码"}],"test_scenarios":[{"name":"特殊字符","input":"p@ss","expected":"success"}]}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	cfg := &OrchestratorConfig{
		PlanFinalProviders:  []ai.Provider{mockAI},
		PlanFinalMaxRetries: 1,
		ImplementProvider:   mockAI,
	}
	orch := NewOrchestratorWithConfig(mockGH, 1, cfg)
	err := orch.DoPlanFinal(context.Background())
	require.NoError(t, err)

	finalMC := mockGH.GetMarker(1, marker.TypePlanFinal)
	require.NotNil(t, finalMC)
	assert.Contains(t, mockGH.Labels[1], string(state.StatePlanFinal))
	assert.Equal(t, 2, mockAI.CallCount())
}

func TestDoPlanFinal_FallbackProvider(t *testing.T) {
	// 主 provider 全部失败，fallback provider 成功
	// WithRecovery 在首次 parse 失败后尝试 repair（消耗 1 次额外调用），
	// 因此 primary 需要 3 个响应：原始调用 + repair + 重试。
	primary := ai.NewMockProvider(
		"not json 1",
		"repair also bad",
		"not json 2",
	)
	fallback := ai.NewMockProvider(
		`{"title":"方案B","approach":"fallback方案","file_changes":[],"test_scenarios":[]}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	cfg := &OrchestratorConfig{
		PlanFinalProviders:  []ai.Provider{primary, fallback},
		PlanFinalMaxRetries: 1,
		ImplementProvider:   primary,
	}
	orch := NewOrchestratorWithConfig(mockGH, 1, cfg)
	err := orch.DoPlanFinal(context.Background())
	require.NoError(t, err)

	finalMC := mockGH.GetMarker(1, marker.TypePlanFinal)
	require.NotNil(t, finalMC)
	assert.Contains(t, finalMC.Comment.GetBody(), "方案B")
	assert.Equal(t, 3, primary.CallCount()) // 原始调用 + repair + 重试
	assert.Equal(t, 1, fallback.CallCount())
}

func TestDoPlanFinal_AllParseFail_FallbackRawText(t *testing.T) {
	// 所有 provider 返回非 JSON，降级为原文定稿，流程继续
	p1 := ai.NewMockProvider("not json", "not json")
	p2 := ai.NewMockProvider("still not json", "still not json")
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	cfg := &OrchestratorConfig{
		PlanFinalProviders:  []ai.Provider{p1, p2},
		PlanFinalMaxRetries: 1,
		ImplementProvider:   p1,
	}
	orch := NewOrchestratorWithConfig(mockGH, 1, cfg)
	err := orch.DoPlanFinal(context.Background())
	require.NoError(t, err)

	// 降级后仍然创建 marker 并迁移状态
	assert.NotNil(t, mockGH.GetMarker(1, marker.TypePlanFinal))
	assert.Contains(t, mockGH.Labels[1], string(state.StatePlanFinal))
}

func TestDoDiscuss_ReturnsErrorWhenRoundFails(t *testing.T) {
	mockAI := ai.NewMockProvider(
		"r1 继续讨论。\n```json\n{\"should_finish\":false}\n```",
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoDiscuss(context.Background(), 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "round=2")
}
