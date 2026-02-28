//go:build !ci

package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
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

type diffFailureGitHub struct {
	*MockGitHub
}

func (m *diffFailureGitHub) GetPRDiff(_ context.Context, _ int) (string, error) {
	return "", fmt.Errorf("diff unavailable")
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
	_, err := ws.Create(1, "login", "")
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

type writeFileProvider struct {
	repoDir   string
	issueNum  int
	fileName  string
	writeBody string
}

func (p *writeFileProvider) Name() string { return "write-file-provider" }

func (p *writeFileProvider) Complete(_ context.Context, _ string, _ ...ai.Option) (string, error) {
	return "unused", nil
}

func (p *writeFileProvider) Execute(_ context.Context, _ string, _ ...ai.Option) (string, error) {
	wtPath := NewWorkspace(p.repoDir).Path(p.issueNum)
	if err := os.WriteFile(filepath.Join(wtPath, p.fileName), []byte(p.writeBody), 0644); err != nil {
		return "", err
	}
	return "file written", nil
}

func TestDoImplement_SubIssueCreatePR_BaseIntegrationMain(t *testing.T) {
	requireLocalPushSupport(t)

	repoDir := initTestRepo(t)
	git := gitBin()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")

	// 预置 integration/main 分支，确保 DAG 子 issue 可从该基线创建 worktree。
	initBareRemote(t, remoteDir)
	cmd := exec.Command(git, "remote", "add", "origin", asFileRemote(remoteDir))
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "push", "-u", "origin", "master")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command(git, "checkout", "-b", "integration/main")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "push", "-u", "origin", "integration/main")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "checkout", "master")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix gateway chain", "parent: #24\ndepends-on: #26")
	mockGH.SetLabel(1, string(state.StatePlanFinal))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")

	orch := NewOrchestratorWithConfig(mockGH, 1, &OrchestratorConfig{
		ImplementProvider: &writeFileProvider{
			repoDir:   repoDir,
			issueNum:  1,
			fileName:  "sub_issue_change.txt",
			writeBody: "changed",
		},
		RepoDir: repoDir,
	})

	err := orch.DoImplement(context.Background(), "/tmp/fallback")
	require.NoError(t, err)
	require.NotEmpty(t, mockGH.PRs)
	assert.Equal(t, "integration/main", mockGH.PRs[0].GetBase().GetRef())
}

func TestDoImplement_SubIssueCreatePR_AutoEnsureIntegrationMain(t *testing.T) {
	requireLocalPushSupport(t)

	repoDir := initTestRepo(t)
	git := gitBin()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")

	// 仅准备远端 master，故意不创建 integration/main。
	initBareRemote(t, remoteDir)
	cmd := exec.Command(git, "remote", "add", "origin", asFileRemote(remoteDir))
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "push", "-u", "origin", "master")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix gateway chain", "parent: #24\ndepends-on: #26")
	mockGH.SetLabel(1, string(state.StatePlanFinal))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")

	orch := NewOrchestratorWithConfig(mockGH, 1, &OrchestratorConfig{
		ImplementProvider: &writeFileProvider{
			repoDir:   repoDir,
			issueNum:  1,
			fileName:  "sub_issue_change_auto_base.txt",
			writeBody: "changed",
		},
		RepoDir: repoDir,
	})

	err := orch.DoImplement(context.Background(), "/tmp/fallback")
	require.NoError(t, err)
	require.NotEmpty(t, mockGH.PRs)
	assert.Equal(t, "integration/main", mockGH.PRs[0].GetBase().GetRef())

	// 远端基线分支应已被自动创建。
	cmd = exec.Command(git, "--git-dir", remoteDir, "rev-parse", "--verify", "refs/heads/integration/main")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestSelectImplementBaseBranch(t *testing.T) {
	assert.Equal(t, "master", selectImplementBaseBranch("no parent"))
	assert.Equal(t, "integration/main", selectImplementBaseBranch("parent: #24"))
	assert.Equal(t, "integration/main", selectImplementBaseBranch("  Parent: #99  "))
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
	// Mock AI 返回 debate 评论
	mockAI := ai.NewMockProvider(
		"进展中。\n```json\n{\"should_finish\":false}\n```",
		"进展中。\n```json\n{\"should_finish\":false}\n```", // buildPromptInput 也可能触发
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

func TestDoDiscussionCheck_NotConverged_UpsertSummaryMarker(t *testing.T) {
	// 两轮讨论都未收敛，summary 应该更新同一条 marker 评论。
	mockAI := ai.NewMockProvider(
		"第一轮。\n```json\n{\"should_finish\":false}\n```",
		"第二轮。\n```json\n{\"should_finish\":false}\n```",
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	require.NoError(t, orch.DoDiscussionCheck(context.Background()))
	require.NoError(t, orch.DoDiscussionCheck(context.Background()))

	mc := mockGH.GetMarker(1, marker.TypeDiscussionSummary)
	require.NotNil(t, mc)
	assert.Equal(t, 2, mc.Marker.Revision)
	assert.Contains(t, mc.Comment.GetBody(), "BOT:DISCUSSION_SUMMARY issue=1 rev=2")
	assert.Contains(t, mc.Comment.GetBody(), "第二轮")
}

func TestDoDiscussionCheck_Converged_Finalizes(t *testing.T) {
	mockAI := ai.NewMockProvider(
		"已达成一致，可定稿。\n```json\n{\"should_finish\":true}\n```",
		`{"title":"最终方案","approach":"按共识实现","file_changes":[{"path":"src/login.go","action":"modify","description":"修复编码"}],"test_scenarios":[{"name":"特殊字符","input":"p@ss","expected":"success"}]}`,
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoDiscussionCheck(context.Background())
	require.NoError(t, err)

	finalMC := mockGH.GetMarker(1, marker.TypePlanFinal)
	require.NotNil(t, finalMC)
	assert.Contains(t, mockGH.Labels[1], string(state.StatePlanFinal))
}

func TestDoDiscussionCheck_Converged_FinalizeFailed(t *testing.T) {
	mockAI := ai.NewMockProvider(
		"已达成一致，可定稿。\n```json\n{\"should_finish\":true}\n```",
	)
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "existing final")

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoDiscussionCheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "讨论已收敛但定稿失败")
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

	cfg := &OrchestratorConfig{
		DiscussionProviders: []ai.Provider{discProvider1, discProvider2},
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

func TestDoDiscuss_DebateAlternatesProviders(t *testing.T) {
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	discProvider1 := ai.NewMockProvider("A 观点。\n```json\n{\"should_finish\":false}\n```")
	discProvider2 := ai.NewMockProvider("B 观点。\n```json\n{\"should_finish\":false}\n```")

	cfg := &OrchestratorConfig{
		DiscussionProviders: []ai.Provider{discProvider1, discProvider2},
		ImplementProvider:   ai.NewMockProvider("unused"),
	}

	orch := NewOrchestratorWithConfig(mockGH, 1, cfg)
	err := orch.DoDiscuss(context.Background(), 2)
	require.NoError(t, err)

	// 验证讨论汇总 marker 已创建
	mc := mockGH.GetMarker(1, marker.TypeDiscussionSummary)
	require.NotNil(t, mc)

	// 验证两个讨论 provider 都至少被调用一次
	assert.Equal(t, 1, discProvider1.CallCount())
	assert.Equal(t, 1, discProvider2.CallCount())
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

func TestDoIterate_AutoSyncsPlanFilesInPRBody(t *testing.T) {
	mockAI := ai.NewMockProvider()
	mockAI.SetExecuteResults("// 修复完成")

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix plan files", "Body")
	mockGH.SetLabel(1, string(state.StatePRNeedsFix))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "## 最终方案\n\n<!-- PLAN_FILES:[{\"path\":\"src/a.go\",\"action\":\"modify\",\"description\":\"keep\"}] -->")
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePRCreated, Issue: 1, Revision: 1, PR: 20,
	}, "PR created")
	mockGH.PRs = append(mockGH.PRs, &github.PullRequest{
		Number: github.Ptr(20),
		Body:   github.Ptr("Closes #1\n\n<!-- PLAN_FILES:[{\"path\":\"src/a.go\",\"action\":\"modify\",\"description\":\"keep\"}] -->"),
	})
	mockGH.PRFiles[20] = []gh.PRFile{
		{Filename: "src/a.go", Status: "modified"},
		{Filename: "src/b.go", Status: "modified"},
	}

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoIterate(context.Background(), 20, "/tmp/work")
	require.NoError(t, err)

	pr, err := mockGH.GetPR(context.Background(), 20)
	require.NoError(t, err)
	assert.Contains(t, pr.GetBody(), "src/b.go", "应自动补齐 missing PLAN_FILES 路径")
	assert.Contains(t, pr.GetBody(), "自动同步：根据 PR 实际改动补齐")
}

func TestTransition_ImplementingWillNotDowngradeToFix(t *testing.T) {
	mockAI := ai.NewMockProvider("unused")
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateImplementing))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.transition(context.Background(), state.StateFixRequested)
	require.Error(t, err)

	labels := mockGH.Labels[1]
	assert.Equal(t, []string{string(state.StateImplementing)}, labels)
}

func TestCurrentState_DirtyStateAutoHeals(t *testing.T) {
	mockAI := ai.NewMockProvider("unused")
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StateFixRequested), string(state.StateImplementing))

	orch := NewOrchestrator(mockGH, mockAI, 1)
	current, err := orch.currentState(context.Background())
	require.NoError(t, err)
	assert.Equal(t, state.StateImplementing, current)

	comments := mockGH.Comments[1]
	require.NotEmpty(t, comments)
	assert.Contains(t, comments[len(comments)-1].GetBody(), "状态自愈")
	assert.Equal(t, []string{string(state.StateImplementing)}, mockGH.Labels[1])
}

// ===== buildPromptInputWithFilter 集成测试 =====

func TestBuildPromptInputWithFilter_CompatWithBuildPromptInput(t *testing.T) {
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mockGH.Comments[1] = []*github.IssueComment{
		{Body: github.Ptr("comment1"), CreatedAt: &github.Timestamp{Time: base},
			User: &github.User{Type: github.Ptr("User"), Login: github.Ptr("alice")}},
		{Body: github.Ptr("comment2"), CreatedAt: &github.Timestamp{Time: base.Add(time.Minute)},
			User: &github.User{Type: github.Ptr("User"), Login: github.Ptr("bob")}},
	}

	mockAI := ai.NewMockProvider("unused")
	orch := NewOrchestrator(mockGH, mockAI, 1)

	ctx := context.Background()
	full, err := orch.buildPromptInput(ctx)
	require.NoError(t, err)

	filtered, err := orch.buildPromptInputWithFilter(ctx, 0)
	require.NoError(t, err)

	assert.Equal(t, full.IssueTitle, filtered.IssueTitle)
	assert.Equal(t, full.IssueBody, filtered.IssueBody)
	assert.Equal(t, full.Comments, filtered.Comments)
}

func TestBuildPromptInputWithFilter_DebatePathTrimsComments(t *testing.T) {
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	var comments []*github.IssueComment
	// 100 条 BOT 评论
	for i := 0; i < 100; i++ {
		comments = append(comments, &github.IssueComment{
			Body:      github.Ptr(fmt.Sprintf("BOT %d", i)),
			CreatedAt: &github.Timestamp{Time: base.Add(time.Duration(i) * time.Minute)},
			User:      &github.User{Type: github.Ptr("Bot"), Login: github.Ptr("niuma[bot]")},
		})
	}
	// 15 条人类评论
	for i := 0; i < 15; i++ {
		comments = append(comments, &github.IssueComment{
			Body:      github.Ptr(fmt.Sprintf("Human %d", i)),
			CreatedAt: &github.Timestamp{Time: base.Add(time.Duration(100+i) * time.Minute)},
			User:      &github.User{Type: github.Ptr("User"), Login: github.Ptr("wangbo")},
		})
	}
	mockGH.Comments[1] = comments

	mockAI := ai.NewMockProvider("unused")
	orch := NewOrchestrator(mockGH, mockAI, 1)

	ctx := context.Background()
	result, err := orch.buildPromptInputWithFilter(ctx, 10)
	require.NoError(t, err)
	// 最多 10 条（无 summary marker，故无 BOT 附加）
	assert.LessOrEqual(t, len(result.Comments), 10)
	// 全量模式返回 115 条（100 BOT + 15 human，空 body 已过滤）
	full, err := orch.buildPromptInput(ctx)
	require.NoError(t, err)
	assert.Equal(t, 115, len(full.Comments))
}

func TestGetMaxHumanComments_Default(t *testing.T) {
	var cfg *OrchestratorConfig
	assert.Equal(t, 10, cfg.getMaxHumanComments())

	cfg = &OrchestratorConfig{}
	assert.Equal(t, 10, cfg.getMaxHumanComments())

	cfg = &OrchestratorConfig{MaxHumanComments: 20}
	assert.Equal(t, 20, cfg.getMaxHumanComments())
}

func TestRecoverableKind_Helper(t *testing.T) {
	recErr := &RecoverableError{Kind: MissingJSON, Message: "test"}
	assert.Equal(t, "MissingJSON", recoverableKind(recErr))
	assert.Equal(t, "Unknown", recoverableKind(fmt.Errorf("plain error")))
}

func TestTruncatePrefix(t *testing.T) {
	assert.Equal(t, "abc", truncatePrefix("abc", 10))
	assert.Equal(t, "ab...", truncatePrefix("abcde", 2))
	assert.Equal(t, "", truncatePrefix("", 10))
}

func TestDoDebateABDiscussion_ParseFailure_StructuredLog(t *testing.T) {
	// 验证 parse 失败时 abort 评论包含 raw_prefix
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mockGH.Comments[1] = []*github.IssueComment{
		{Body: github.Ptr("讨论开始"), CreatedAt: &github.Timestamp{Time: base},
			User: &github.User{Type: github.Ptr("User"), Login: github.Ptr("wangbo")}},
	}

	// provider 返回缺少 JSON 的文本
	mockAI := ai.NewMockProvider("纯文本回复，没有 JSON 代码块")

	orch := NewOrchestratorWithConfig(mockGH, 1, &OrchestratorConfig{
		DiscussionProviders: []ai.Provider{mockAI},
		ImplementProvider:   mockAI,
	})

	_, err := orch.doDebateABDiscussion(context.Background(), 1, 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3 级降级均失败")

	// abort 评论应包含 raw_prefix
	comments := mockGH.Comments[1]
	abortFound := false
	for _, c := range comments {
		if strings.Contains(c.GetBody(), "DISCUSS_ABORT") {
			abortFound = true
			assert.Contains(t, c.GetBody(), "raw_prefix")
		}
	}
	assert.True(t, abortFound, "应有 abort 评论")
}

func TestDoDebateABDiscussion_LargeCommentScenario(t *testing.T) {
	// 验证 120+ 评论场景下 debate 路径评论裁剪生效，prompt 中评论数 <= maxHuman+1
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Large comment test", "Body")
	mockGH.SetLabel(1, string(state.StateNeedsDiscussion))

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var comments []*github.IssueComment
	// 117 条 BOT 评论（其中 1 条含 DISCUSSION_SUMMARY marker）
	for i := 0; i < 117; i++ {
		body := fmt.Sprintf("BOT 评论 %d", i)
		if i == 100 {
			body = "<!-- BOT:DISCUSSION_SUMMARY issue=1 rev=1 -->汇总内容"
		}
		comments = append(comments, &github.IssueComment{
			Body:      github.Ptr(body),
			CreatedAt: &github.Timestamp{Time: base.Add(time.Duration(i) * time.Minute)},
			User:      &github.User{Type: github.Ptr("Bot"), Login: github.Ptr("niuma[bot]")},
		})
	}
	// 15 条人类评论
	for i := 0; i < 15; i++ {
		comments = append(comments, &github.IssueComment{
			Body:      github.Ptr(fmt.Sprintf("人类评论[%d]", i)),
			CreatedAt: &github.Timestamp{Time: base.Add(time.Duration(117+i) * time.Minute)},
			User:      &github.User{Type: github.Ptr("User"), Login: github.Ptr("wangbo")},
		})
	}
	mockGH.Comments[1] = comments

	// provider 返回合法的 debate JSON 响应（should_finish=true 使第 1 轮即结束）
	validResponse := "我认为方案A更优。\n\n```json\n{\"should_finish\": true}\n```"
	mockAI := ai.NewMockProvider(validResponse)

	maxHuman := 10
	orch := NewOrchestratorWithConfig(mockGH, 1, &OrchestratorConfig{
		DiscussionProviders: []ai.Provider{mockAI},
		ImplementProvider:   mockAI,
		MaxHumanComments:    maxHuman,
	})

	summary, err := orch.doDebateABDiscussion(context.Background(), 1, 5)
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.True(t, summary.ShouldFinish)

	// 验证传给 AI 的 prompt 中评论数受限：
	// buildPromptInputWithFilter(ctx, 10) 应只保留 10 条人类 + 1 条 summary BOT = 11 条
	calls := mockAI.Calls()
	require.GreaterOrEqual(t, len(calls), 1)
	prompt := calls[0].Prompt
	// 统计 prompt 中出现的人类评论数量，早期人类评论（0-4）不应出现
	for i := 0; i < 5; i++ {
		assert.NotContains(t, prompt, fmt.Sprintf("人类评论[%d]", i),
			"早期人类评论 %d 不应出现在裁剪后的 prompt 中", i)
	}
	// 最近 10 条人类评论（5-14）应出现
	for i := 5; i < 15; i++ {
		assert.Contains(t, prompt, fmt.Sprintf("人类评论[%d]", i),
			"最近的人类评论 %d 应出现在 prompt 中", i)
	}
	// summary BOT 评论应保留
	assert.Contains(t, prompt, "汇总内容", "DISCUSSION_SUMMARY BOT 评论应保留在 prompt 中")

	// 显式验证 maxHuman+1 上限：统计 prompt 中各类评论出现次数
	humanInPrompt := 0
	for i := 0; i < 15; i++ {
		if strings.Contains(prompt, fmt.Sprintf("人类评论[%d]", i)) {
			humanInPrompt++
		}
	}
	botInPrompt := 0
	for i := 0; i < 117; i++ {
		if strings.Contains(prompt, fmt.Sprintf("BOT 评论 %d", i)) {
			botInPrompt++
		}
	}
	summaryInPrompt := 0
	if strings.Contains(prompt, "汇总内容") {
		summaryInPrompt = 1
	}
	totalComments := humanInPrompt + botInPrompt + summaryInPrompt
	assert.LessOrEqual(t, totalComments, maxHuman+1,
		"prompt 中评论总数 (%d) 应 <= maxHuman+1 (%d)", totalComments, maxHuman+1)
	assert.Equal(t, maxHuman, humanInPrompt,
		"应恰好包含 maxHuman (%d) 条人类评论", maxHuman)
	assert.Equal(t, 0, botInPrompt, "普通 BOT 评论应全部被过滤")
}

// ===== Prompt 上下文一致性（Issue/Plan-final/PR body + 阶段增量） =====

func TestDoImplement_InjectsPRBodyFromExistingPR(t *testing.T) {
	mockAI := ai.NewMockProvider()
	mockAI.SetExecuteResults("// 实现代码")

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePlanFinal))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePRCreated, Issue: 1, Revision: 1, PR: 42,
	}, "PR created")
	mockGH.PRs = append(mockGH.PRs, &github.PullRequest{
		Number: github.Ptr(42),
		Body:   github.Ptr("Closes #1\n\nImplement PR Body"),
	})

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoImplement(context.Background(), "/tmp/work")
	require.NoError(t, err)

	lastPrompt := mockAI.LastPrompt()
	assert.Contains(t, lastPrompt, "Implement PR Body", "implement prompt 应注入 PR body")
}

func TestDoImplement_NoPRMarker_DegradesWithoutPRBody(t *testing.T) {
	mockAI := ai.NewMockProvider()
	mockAI.SetExecuteResults("// 实现代码")

	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix login", "Body")
	mockGH.SetLabel(1, string(state.StatePlanFinal))
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")

	orch := NewOrchestrator(mockGH, mockAI, 1)
	err := orch.DoImplement(context.Background(), "/tmp/work")
	require.NoError(t, err)

	lastPrompt := mockAI.LastPrompt()
	assert.Contains(t, lastPrompt, "未读取到 PR Body，已降级继续", "无 PR marker 时应降级继续")
}

func TestBuildPhasePromptInput_ContextMatrixByPhase(t *testing.T) {
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix prompt", "issue body")
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePRCreated, Issue: 1, Revision: 1, PR: 20,
	}, "PR created")
	mockGH.PRs = append(mockGH.PRs, &github.PullRequest{
		Number: github.Ptr(20),
		Body:   github.Ptr("Closes #1\n\nPR body context"),
	})
	mockGH.Reviews[20] = []*github.PullRequestReview{
		{Body: github.Ptr("history review"), State: github.Ptr("COMMENT")},
	}
	mockGH.Comments[20] = []*github.IssueComment{
		{Body: github.Ptr("history comment")},
	}
	mockGH.PRDiffs[20] = "diff --git a/a.go b/a.go"

	orch := NewOrchestrator(mockGH, ai.NewMockProvider("unused"), 1)
	finalPlan := "plan-final content"

	implementInput, err := orch.buildPhasePromptInput(context.Background(), promptContextPhaseImplement, finalPlan, 0)
	require.NoError(t, err)
	assert.Contains(t, implementInput.PRBody, "PR body context")
	assert.Empty(t, implementInput.ReviewComment, "implement 阶段不应注入 PR 历史增量")
	assert.Empty(t, implementInput.PRDiff, "implement 阶段不应注入 PR diff")

	reviewInput, err := orch.buildPhasePromptInput(context.Background(), promptContextPhaseReview, finalPlan, 20)
	require.NoError(t, err)
	assert.Contains(t, reviewInput.PRBody, "PR body context")
	assert.Contains(t, reviewInput.ReviewComment, "history review")
	assert.Contains(t, reviewInput.ReviewComment, "history comment")
	assert.Contains(t, reviewInput.PRDiff, "diff --git")

	iterateInput, err := orch.buildPhasePromptInput(context.Background(), promptContextPhaseIterate, finalPlan, 20)
	require.NoError(t, err)
	assert.Contains(t, iterateInput.PRBody, "PR body context")
	assert.Contains(t, iterateInput.ReviewSummary, "共 1 条 review")
	assert.Contains(t, iterateInput.ReviewComment, "history comment")
	assert.Contains(t, iterateInput.PRDiff, "diff --git")
}

func TestBuildPhasePromptInput_OversizedContextTrimmed(t *testing.T) {
	mockGH := NewMockGitHub()
	mockGH.SetIssue(1, "Fix oversized context", "issue body")
	mockGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePRCreated, Issue: 1, Revision: 1, PR: 30,
	}, "PR created")
	mockGH.PRs = append(mockGH.PRs, &github.PullRequest{
		Number: github.Ptr(30),
		Body:   github.Ptr(strings.Repeat("B", 5000)),
	})

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var comments []*github.IssueComment
	for i := 0; i < 30; i++ {
		comments = append(comments, &github.IssueComment{
			Body:      github.Ptr(fmt.Sprintf("comment-%d-%s", i, strings.Repeat("x", 700))),
			CreatedAt: &github.Timestamp{Time: base.Add(time.Duration(i) * time.Minute)},
		})
	}
	mockGH.Comments[30] = comments
	mockGH.PRDiffs[30] = strings.Repeat("D", 76000)

	orch := NewOrchestrator(mockGH, ai.NewMockProvider("unused"), 1)
	input, err := orch.buildPhasePromptInput(context.Background(), promptContextPhaseReview, "plan", 30)
	require.NoError(t, err)

	assert.True(t, input.ContextTrimmed, "超长上下文应触发裁剪标记")
	assert.Contains(t, input.TrimmedReason, "PR body 超长已裁剪")
	assert.Contains(t, input.TrimmedReason, "PR comments 仅保留最近 20 条")
	assert.Contains(t, input.TrimmedReason, "软上限触发，压缩了")
	assert.LessOrEqual(t, len([]rune(input.PRBody)), maxPromptPRBodyChars)
	assert.NotContains(t, input.ReviewComment, "comment-0-", "旧 comment 应优先被压缩")
}

func TestDoReview_DiffFailureDegradesButStillBuildsPrompt(t *testing.T) {
	mockAI := ai.NewMockProvider(`{"approved": true, "summary": "ok", "issues": []}`)
	baseGH := NewMockGitHub()
	baseGH.SetIssue(1, "Fix review context", "Body")
	baseGH.SetLabel(1, string(state.StatePRCreated))
	baseGH.SetMarker(1, &marker.Marker{
		Type: marker.TypePlanFinal, Issue: 1, Revision: 1,
	}, "最终方案内容")
	baseGH.PRs = append(baseGH.PRs, &github.PullRequest{
		Number: github.Ptr(10),
		Body:   github.Ptr("PR body for review"),
	})
	baseGH.Reviews[10] = []*github.PullRequestReview{
		{Body: github.Ptr("review history")},
	}
	baseGH.Comments[10] = []*github.IssueComment{
		{Body: github.Ptr("pr comment history")},
	}

	ghWithDiffErr := &diffFailureGitHub{MockGitHub: baseGH}
	orch := NewOrchestrator(ghWithDiffErr, mockAI, 1)
	err := orch.DoReview(context.Background(), 10)
	require.NoError(t, err, "GetPRDiff 失败应降级而非直接失败")

	lastPrompt := mockAI.LastPrompt()
	assert.Contains(t, lastPrompt, "PR body for review", "应保留 PR body")
	assert.Contains(t, lastPrompt, "pr comment history", "应保留 PR 历史")
	assert.Contains(t, lastPrompt, "未读取到 PR Diff，已降级继续", "diff 失败应显示降级提示")

	labels := baseGH.Labels[1]
	assert.Contains(t, labels, string(state.StatePRReviewable))
}
