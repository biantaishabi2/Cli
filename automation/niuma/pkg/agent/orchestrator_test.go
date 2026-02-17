//go:build !ci

package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/google/go-github/v68/github"
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

// 路径校验从静态白名单改为 implement 阶段 diff 对比（#60），旧测试已删除

func TestDoImplement_NoWorktree(t *testing.T) {
	// 无 worktree 模式：AI 执行完成但无 git 操作，状态回滚
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

	// 无 worktree → prNumber=0 → 状态回滚到 plan-final
	labels := mockGH.Labels[1]
	assert.Contains(t, labels, string(state.StatePlanFinal))

	// 验证发了"无文件变更"评论
	comments := mockGH.Comments[1]
	require.NotEmpty(t, comments)
	assert.Contains(t, comments[len(comments)-1].GetBody(), "无文件变更")
}

func TestDoImplement_WithWorktree(t *testing.T) {
	// 有 worktree 模式：创建 worktree → AI 在 worktree 目录执行 → commit → push → 创建 PR
	repoDir := initTestRepo(t)

	mockAI := ai.NewMockProvider()
	mockAI.SetExecuteResults("// 实现代码")

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePlanFinal))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")

	orch := NewOrchestratorWithConfig(mockGH, 1, &OrchestratorConfig{
		ImplementProvider: mockAI,
		RepoDir:           repoDir,
	})

	// 模拟 AI Execute 在 worktree 中写了文件（否则 HasChanges=false）
	// 由于 MockProvider 不实际写文件，worktree 会创建但无变更
	// 这里重点验证 worktree 路径正确传递
	err := orch.DoImplement(context.Background(), "/tmp/fallback")
	require.NoError(t, err)

	// 验证 AI Execute 收到的 WorkDir 是 .worktrees/fix-1（不是 /tmp/fallback）
	expectedWT := NewWorkspace(repoDir).Path(1)
	assert.Equal(t, expectedWT, mockAI.LastWorkDir())
	assert.Contains(t, expectedWT, ".worktrees/fix-1")

	// 验证 worktree 目录存在（无变更时会被清理，但 AI 已被调用过，路径是对的）
}

func TestDoIterate_WithWorktree(t *testing.T) {
	// 有 worktree 模式：复用已有 worktree → AI 在 worktree 中执行
	repoDir := initTestRepo(t)

	// 先创建 worktree（模拟 implement 阶段已创建）
	ws := NewWorkspace(repoDir)
	_, err := ws.Create(1, "login")
	require.NoError(t, err)
	require.True(t, ws.Exists(1))

	mockAI := ai.NewMockProvider()
	mockAI.SetExecuteResults("// 迭代修复代码")

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePRNeedsFix))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePRCreated, Issue: 1, Revision: 1, PR: 10,
	}, "PR created")
	mockGH.Reviews[10] = nil

	orch := NewOrchestratorWithConfig(mockGH, 1, &OrchestratorConfig{
		ImplementProvider: mockAI,
		RepoDir:           repoDir,
	})

	err = orch.DoIterate(context.Background(), 10, "/tmp/fallback")
	require.NoError(t, err)

	// 验证 AI Execute 收到的 WorkDir 是 worktree 路径（不是 /tmp/fallback）
	expectedWT := ws.Path(1)
	assert.Equal(t, expectedWT, mockAI.LastWorkDir())
	assert.Contains(t, expectedWT, ".worktrees/fix-1")
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

func TestNewOrchestratorWithConfig_PanicsWithNoProvider(t *testing.T) {
	mockGH := NewMockGitHub()
	cfg := &OrchestratorConfig{} // 没有任何 provider

	assert.Panics(t, func() {
		NewOrchestratorWithConfig(mockGH, 1, cfg)
	})
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
		{"修复中文标题", ""},
		{"  spaces  and  dashes  ", "spaces-and-dashes"},
		{"A very long title that should be truncated to thirty characters max", "a-very-long-title-that-should-"},
		{"", ""},
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
	// plan-approved 状态下允许执行实现（无 worktree → 状态回滚到 plan-approved）
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

	// 验证 AI 执行了（评论包含实现结果）
	comments := mockGH.Comments[1]
	require.NotEmpty(t, comments)

	// 无 worktree → prNumber=0 → 状态回滚到 plan-approved
	labels := mockGH.Labels[1]
	assert.Contains(t, labels, string(state.StatePlanApproved))
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

	// 验证发了 PR review（APPROVE）
	reviews := mockGH.Reviews[10]
	require.Len(t, reviews, 1)
	assert.Equal(t, "APPROVE", reviews[0].GetState())
	assert.Contains(t, reviews[0].GetBody(), "自审通过")
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

	// 验证发了 PR review（COMMENT，因为 GitHub 不允许对自己的 PR 提 REQUEST_CHANGES）
	reviews := mockGH.Reviews[10]
	require.Len(t, reviews, 1)
	assert.Equal(t, "COMMENT", reviews[0].GetState())
	assert.Contains(t, reviews[0].GetBody(), "自审未通过")
	assert.Contains(t, reviews[0].GetBody(), "缺少错误处理")
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

// ===== 交流机制测试 =====

func TestBuildPRHistory_WithReviewsAndComments(t *testing.T) {
	mockAI := ai.NewMockProvider("unused")
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePRCreated))

	// 设置 PR reviews
	mockGH.Reviews[10] = []*github.PullRequestReview{
		{Body: github.Ptr("R1: 缺少错误处理"), State: github.Ptr("COMMENT")},
		{Body: github.Ptr("R2: 已修复"), State: github.Ptr("APPROVE")},
		{Body: github.Ptr(""), State: github.Ptr("COMMENT")}, // 空 body 应被跳过
	}

	// 设置 PR comments（GitHub 中 PR 也是 issue，ListComments(prNumber) 能读到）
	mockGH.Comments[10] = []*github.IssueComment{
		{Body: github.Ptr("修复了第一个问题")},
		{Body: github.Ptr("")}, // 空 body 应被跳过
		{Body: github.Ptr("第二轮修复完成")},
	}

	orch := NewOrchestrator(mockGH, mockAI, 1)
	history, err := orch.buildPRHistory(context.Background(), 10)
	require.NoError(t, err)

	// 验证包含 review 内容
	assert.Contains(t, history, "[Review - COMMENT]")
	assert.Contains(t, history, "R1: 缺少错误处理")
	assert.Contains(t, history, "[Review - APPROVE]")
	assert.Contains(t, history, "R2: 已修复")

	// 验证包含 PR comment 内容
	assert.Contains(t, history, "[PR Comment]")
	assert.Contains(t, history, "修复了第一个问题")
	assert.Contains(t, history, "第二轮修复完成")

	// 验证分隔符
	assert.Contains(t, history, "---")
}

func TestBuildPRHistory_Empty(t *testing.T) {
	mockAI := ai.NewMockProvider("unused")
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")

	orch := NewOrchestrator(mockGH, mockAI, 1)
	history, err := orch.buildPRHistory(context.Background(), 10)
	require.NoError(t, err)
	assert.Empty(t, history)
}

func TestBuildPRHistory_ReviewError(t *testing.T) {
	mockAI := ai.NewMockProvider("unused")
	mockGH := NewMockGitHub()
	mockGH.Error = fmt.Errorf("API rate limit")

	orch := NewOrchestrator(mockGH, mockAI, 1)
	_, err := orch.buildPRHistory(context.Background(), 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "获取 PR reviews 失败")
}

func TestDoReview_ReadsPRHistory(t *testing.T) {
	// 验证 DoReview 将 PR 历史传递给 AI
	mockAI := ai.NewMockProvider(`{"approved": true, "summary": "代码良好", "issues": []}`)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePRCreated))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")
	mockGH.PRDiffs[10] = "diff content"

	// 设置 PR 上已有的交流历史
	mockGH.Reviews[10] = []*github.PullRequestReview{
		{Body: github.Ptr("上一轮 review: 变量命名问题"), State: github.Ptr("COMMENT")},
	}
	mockGH.Comments[10] = []*github.IssueComment{
		{Body: github.Ptr("已修复命名问题")},
	}

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoReview(context.Background(), 10)
	require.NoError(t, err)

	// 验证 AI 收到的 prompt 包含历史上下文
	lastPrompt := mockAI.LastPrompt()
	assert.Contains(t, lastPrompt, "上一轮 review: 变量命名问题")
	assert.Contains(t, lastPrompt, "已修复命名问题")
}

func TestDoIterate_PostsOnPR(t *testing.T) {
	// 验证 DoIterate 将修复结果评论发在 PR 上，而不是 issue 上
	mockAI := ai.NewMockProvider()
	mockAI.SetExecuteResults("修复了 XSS 漏洞")

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix XSS", "Body")
	mockGH.SetLabel(1, string(state.StatePRNeedsFix))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案")
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePRCreated, Issue: 1, Revision: 1, PR: 20,
	}, "PR created")

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoIterate(context.Background(), 20, "/tmp/work")
	require.NoError(t, err)

	// 验证 PR 上有评论（prNumber=20）
	prComments := mockGH.Comments[20]
	require.NotEmpty(t, prComments, "PR 上应该有迭代修复评论")
	assert.Contains(t, prComments[0].GetBody(), "迭代修复")
	assert.Contains(t, prComments[0].GetBody(), "修复了 XSS 漏洞")

	// 验证 issue 上没有迭代修复评论（只有可能的失败通知等，但不应该有修复详情）
	issueComments := mockGH.Comments[1]
	for _, c := range issueComments {
		assert.NotContains(t, c.GetBody(), "迭代修复", "issue 上不应该有迭代修复评论")
	}
}
