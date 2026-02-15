//go:build !ci

package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupOrchestrator(mockAI *ai.MockProvider) (*Orchestrator, *MockGitHub) {
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login bug", "Login fails with special chars")
	mockGH.SetLabel(1, string(state.StateFixRequested))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	return orch, mockGH
}

func TestDoPlanDraft_HappyPath(t *testing.T) {
	mockAI := ai.NewMockProvider(`{"summary": "修复登录", "approach": "编码处理", "affected_files": ["auth.go"]}`)
	orch, mockGH := setupOrchestrator(mockAI)

	err := orch.DoPlanDraft(context.Background())
	require.NoError(t, err)

	// 验证 marker 已创建
	mc := mockGH.GetMarker(1, marker.TypePlanDraft)
	require.NotNil(t, mc)
	assert.Equal(t, 1, mc.Marker.Revision)

	// 验证状态转到 needs-discussion
	labels := mockGH.Labels[1]
	assert.Contains(t, labels, string(state.StateNeedsDiscussion))

	// 验证调用了 AI
	assert.Equal(t, 1, mockAI.CallCount())
}

func TestDoPlanDraft_WrongState(t *testing.T) {
	mockAI := ai.NewMockProvider("unused")
	orch, mockGH := setupOrchestrator(mockAI)
	mockGH.SetLabel(1, string(state.StatePlanDraft)) // 不是 fix 状态

	err := orch.DoPlanDraft(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不允许生成草案")

	// 验证未调用 AI
	assert.Equal(t, 0, mockAI.CallCount())
}

func TestDoPlanDraft_Idempotent(t *testing.T) {
	mockAI := ai.NewMockProvider("unused")
	orch, mockGH := setupOrchestrator(mockAI)

	// 预设已有 PLAN_DRAFT marker
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanDraft, Issue: 1, Revision: 1,
	}, "existing draft")

	err := orch.DoPlanDraft(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已存在方案草案")
	assert.Equal(t, 0, mockAI.CallCount())
}

func TestDoPlanDraft_AIError(t *testing.T) {
	mockAI := ai.NewMockProviderWithError(fmt.Errorf("AI unavailable"))
	orch, _ := setupOrchestrator(mockAI)

	err := orch.DoPlanDraft(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI 生成草案失败")
}

func TestDoPlanFinal_HappyPath(t *testing.T) {
	mockAI := ai.NewMockProvider(`{
		"title": "修复登录",
		"approach": "添加编码处理",
		"file_changes": [{"path": "src/auth.go", "action": "modify", "description": "fix"}],
		"test_scenarios": [{"name": "特殊字符", "input": "p@ss!", "expected": "success"}]
	}`)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoPlanFinal(context.Background())
	require.NoError(t, err)

	// 验证 marker
	mc := mockGH.GetMarker(1, marker.TypePlanFinal)
	require.NotNil(t, mc)

	// 验证状态
	labels := mockGH.Labels[1]
	assert.Contains(t, labels, string(state.StatePlanFinal))
}

func TestDoPlanFinal_PathValidationFailure(t *testing.T) {
	// AI 返回的方案包含不在白名单的路径
	mockAI := ai.NewMockProvider(`{
		"title": "危险操作",
		"approach": "修改系统文件",
		"file_changes": [{"path": "etc/passwd", "action": "modify", "description": "hack"}],
		"test_scenarios": []
	}`)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Bad plan", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoPlanFinal(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "路径校验失败")

	// 验证没有转状态
	labels := mockGH.Labels[1]
	assert.Contains(t, labels, string(state.StateNeedsDiscussion))
}

func TestDoImplement_HappyPath(t *testing.T) {
	// DoImplement 现在使用 Execute 而非 Complete
	mockAI := ai.NewMockProvider()
	mockAI.SetExecuteResults("// 实现代码\nfunc Login() {}")

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePlanFinal))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoImplement(context.Background(), "/tmp/work")
	require.NoError(t, err)

	// 验证 PR marker
	mc := mockGH.GetMarker(1, marker.TypePRCreated)
	require.NotNil(t, mc)

	// 验证状态
	labels := mockGH.Labels[1]
	assert.Contains(t, labels, string(state.StatePRCreated))
}

func TestDoIterate_HappyPath(t *testing.T) {
	// DoIterate 现在使用 Execute
	mockAI := ai.NewMockProvider()
	mockAI.SetExecuteResults("// 修复代码\nfunc Login() { validate() }")

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePRNeedsFix))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePRCreated, Issue: 1, Revision: 1, PR: 10,
	}, "PR created")

	// 设置 review
	mockGH.Reviews[10] = nil

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoIterate(context.Background(), 10, "/tmp/work")
	require.NoError(t, err)

	// 验证状态回到 pr-created
	labels := mockGH.Labels[1]
	assert.Contains(t, labels, string(state.StatePRCreated))
}

func TestDoDiscussionCheck_NotConverged(t *testing.T) {
	// Mock AI 返回讨论汇总
	mockAI := ai.NewMockProvider(
		`{"consensus": "进展中", "open_items": ["待定"], "should_finish": false}`,
		`{"consensus": "进展中", "open_items": ["待定"], "should_finish": false}`, // buildPromptInput 也可能触发
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoDiscussionCheck(context.Background())
	require.NoError(t, err)

	// 应创建了讨论汇总 marker
	mc := mockGH.GetMarker(1, marker.TypeDiscussionSummary)
	require.NotNil(t, mc)
}

func TestDoDiscussionCheck_WrongState(t *testing.T) {
	mockAI := ai.NewMockProvider("unused")
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StatePlanFinal)) // 不是讨论中

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoDiscussionCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不是讨论中")
}

// ===== Phase 2.5 新增测试 =====

func TestNewOrchestratorWithConfig(t *testing.T) {
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")

	implProvider := ai.NewMockProvider("impl result")
	discProvider1 := ai.NewMockProvider("opinion 1")
	discProvider2 := ai.NewMockProvider("opinion 2")
	consolidator := ai.NewMockProvider(`{"consensus": "汇总", "open_items": []}`)

	cfg := &OrchestratorConfig{
		DiscussionProviders: []ai.Provider{discProvider1, discProvider2},
		Consolidator:        consolidator,
		ImplementProvider:   implProvider,
		RepoDir:             "/tmp/repo",
	}

	orch := NewOrchestratorWithConfig(mockGH, 1, cfg)
	require.NotNil(t, orch)
	assert.Equal(t, implProvider, orch.getImplementProvider())
}

func TestDoDiscussionCheck_MultiProvider(t *testing.T) {
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	// 两个讨论 provider + 一个 consolidator
	discProvider1 := ai.NewMockProvider(`{"summary": "方案A"}`)
	discProvider2 := ai.NewMockProvider(`{"summary": "方案B"}`)
	consolidator := ai.NewMockProvider(
		`{"consensus": "综合方案A和B", "open_items": ["待定"], "should_finish": false}`,
	)

	cfg := &OrchestratorConfig{
		DiscussionProviders: []ai.Provider{discProvider1, discProvider2},
		Consolidator:        consolidator,
		ImplementProvider:   ai.NewMockProvider("unused"),
	}

	orch := NewOrchestratorWithConfig(mockGH, 1, cfg)
	err := orch.DoDiscussionCheck(context.Background())
	require.NoError(t, err)

	// 验证讨论汇总 marker 已创建
	mc := mockGH.GetMarker(1, marker.TypeDiscussionSummary)
	require.NotNil(t, mc)

	// 验证两个讨论 provider 都被调用
	assert.Equal(t, 1, discProvider1.CallCount())
	assert.Equal(t, 1, discProvider2.CallCount())

	// 验证 consolidator 被调用
	assert.Equal(t, 1, consolidator.CallCount())
}

func TestSlugFromTitle(t *testing.T) {
	tests := []struct {
		title    string
		expected string
	}{
		{"Fix login bug", "fix-login-bug"},
		{"Add user authentication", "add-user-authentication"},
		{"修复中文标题", "fix"},
		{"  spaces  and  dashes  ", "spaces-and-dashes"},
		{"A very long title that should be truncated to thirty characters max", "a-very-long-title-that-should-"},
		{"", "fix"},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := slugFromTitle(tt.title)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// ===== Phase 2.7 审批门 + 迭代限制测试 =====

func TestDoImplement_RequireApproval_WaitsAtPlanFinal(t *testing.T) {
	mockAI := ai.NewMockProvider()
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePlanFinal))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")

	cfg := &OrchestratorConfig{
		ImplementProvider:   mockAI,
		RequirePlanApproval: true,
	}
	orch := NewOrchestratorWithConfig(mockGH, 1, cfg)

	err := orch.DoImplement(context.Background(), "/tmp/work")
	require.NoError(t, err) // 不报错，只是不执行

	// 验证发了等待审批的评论
	comments := mockGH.Comments[1]
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0].GetBody(), "等待人工审批")

	// 验证状态没变（仍是 plan-final）
	labels := mockGH.Labels[1]
	assert.Contains(t, labels, string(state.StatePlanFinal))

	// 验证没调用 AI
	assert.Equal(t, 0, mockAI.CallCount())
}

func TestDoImplement_PlanApproved_Proceeds(t *testing.T) {
	mockAI := ai.NewMockProvider()
	mockAI.SetExecuteResults("// 实现代码")
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePlanApproved))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")

	cfg := &OrchestratorConfig{
		ImplementProvider:   mockAI,
		RequirePlanApproval: true,
	}
	orch := NewOrchestratorWithConfig(mockGH, 1, cfg)

	err := orch.DoImplement(context.Background(), "/tmp/work")
	require.NoError(t, err)

	// 验证状态到了 pr-created
	labels := mockGH.Labels[1]
	assert.Contains(t, labels, string(state.StatePRCreated))
}

func TestDoIterate_ExceedsMaxRounds(t *testing.T) {
	mockAI := ai.NewMockProvider()
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePRNeedsFix))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案")
	// PR marker revision=3 → 已经迭代了3轮
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePRCreated, Issue: 1, Revision: 3, PR: 10,
	}, "PR created")

	cfg := &OrchestratorConfig{
		ImplementProvider: mockAI,
		MaxIterateRounds:  3,
	}
	orch := NewOrchestratorWithConfig(mockGH, 1, cfg)

	err := orch.DoIterate(context.Background(), 10, "/tmp/work")
	require.NoError(t, err) // 不报错，只是不执行

	// 验证发了上限评论
	comments := mockGH.Comments[1]
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0].GetBody(), "迭代次数已达上限")

	// 验证没调用 AI
	assert.Equal(t, 0, mockAI.CallCount())
}

// ===== Phase 2.6 DoReview 测试 =====

func TestDoReview_Approved(t *testing.T) {
	// AI 审查通过
	mockAI := ai.NewMockProvider(`{"approved": true, "summary": "代码质量良好", "issues": []}`)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePRCreated))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")
	mockGH.PRDiffs[10] = "diff --git a/auth.go b/auth.go\n+func Login() {}"

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoReview(context.Background(), 10)
	require.NoError(t, err)

	// 验证状态转到 pr-reviewable
	labels := mockGH.Labels[1]
	assert.Contains(t, labels, string(state.StatePRReviewable))

	// 验证发了评论
	comments := mockGH.Comments[1]
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0].GetBody(), "自审通过")
}

func TestDoReview_NotApproved(t *testing.T) {
	// AI 审查不通过
	mockAI := ai.NewMockProvider(`{"approved": false, "summary": "有问题", "issues": ["缺少错误处理", "变量命名不规范"]}`)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePRCreated))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")
	mockGH.PRDiffs[10] = "diff content"

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoReview(context.Background(), 10)
	require.NoError(t, err)

	// 验证状态转到 pr-needs-fix
	labels := mockGH.Labels[1]
	assert.Contains(t, labels, string(state.StatePRNeedsFix))

	// 验证评论包含问题列表
	comments := mockGH.Comments[1]
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0].GetBody(), "自审未通过")
	assert.Contains(t, comments[0].GetBody(), "缺少错误处理")
}

func TestDoReview_WrongState(t *testing.T) {
	mockAI := ai.NewMockProvider("unused")
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePlanFinal)) // 不是 pr-created

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoReview(context.Background(), 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不允许审查")
	assert.Equal(t, 0, mockAI.CallCount())
}

func TestDoReview_NoPlanFinal(t *testing.T) {
	mockAI := ai.NewMockProvider("unused")
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePRCreated))
	mockGH.PRDiffs[10] = "diff content"
	// 没有设置 PlanFinal marker

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoReview(context.Background(), 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未找到最终方案")
}

func TestBuildImplementContext(t *testing.T) {
	input := &PromptInput{
		IssueTitle: "Fix bug",
		IssueBody:  "Something is broken",
	}
	ctx := BuildImplementContext(input)
	assert.Contains(t, ctx, "Fix bug")
	assert.Contains(t, ctx, "Something is broken")
}
