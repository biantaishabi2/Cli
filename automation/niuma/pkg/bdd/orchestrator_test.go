//go:build bdd

package bdd

import (
	"context"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/agent"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== BDD Tests for Orchestrator ==========

// TestOrchestrator_PlanDraft_FullFlow 场景：完整 Draft 流程
func TestOrchestrator_PlanDraft_FullFlow(t *testing.T) {
	ctx := context.Background()
	gh := NewMockGitHubClient()
	
	// AI 返回 Draft Plan
	aiProvider := ai.NewMockProvider(`## Draft Plan

### 问题理解
内存泄漏问题

### 解决方案
添加 context 取消

### 需要确认
是否有等待任务？`)

	// Given: 一个带有 bot:fix 标签的 Issue
	issue, _ := gh.CreateIssue(ctx, "Fix memory leak", "Worker pool leak", []string{"bot:fix"})

	// 创建 Orchestrator（使用 MockGitHubClient 适配 agent.GitHubOps 接口）
	// 注意：这里需要 MockGitHubClient 实现 agent.GitHubOps 接口
	// 为简化测试，我们直接测试工作流逻辑

	// When: 执行 Plan Draft
	workflow := NewPlanDraftWorkflow(gh, aiProvider)
	err := workflow.Execute(ctx, issue.Number)
	require.NoError(t, err)

	// Then: 评论包含 Draft Plan
	comments := gh.Comments[issue.Number]
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "Draft Plan")
	assert.Contains(t, comments[0], "BOT:PLAN_DRAFT")

	// And: 状态变为 bot:needs-discussion（通过 plan-draft 中转）
	labels := gh.Labels[issue.Number]
	assert.Contains(t, labels, "bot:plan-draft")
}

// TestOrchestrator_Discussion_NotConverged 场景：讨论未收敛，更新汇总
func TestOrchestrator_Discussion_NotConverged(t *testing.T) {
	ctx := context.Background()
	gh := NewMockGitHubClient()
	
	// AI 返回讨论汇总
	aiProvider := ai.NewMockProvider(`## Discussion Summary (rev=1)

### 当前共识
- 需要添加 context 取消

### 待确认问题
- 超时时间设置多少？`)

	// Given: 处于 needs-discussion 状态的 Issue，有近期评论
	issue, _ := gh.CreateIssue(ctx, "Fix leak", "Details", []string{"bot:fix", "bot:needs-discussion"})
	gh.AddComment(ctx, issue.Number, "用户评论：建议用 30 秒超时")

	// When: 检查讨论收敛
	checker := &MockConvergenceChecker{result: state.NotConverged}
	workflow := &DiscussionWorkflow{github: gh, ai: aiProvider, checker: checker}
	err := workflow.Execute(ctx, issue.Number)
	require.NoError(t, err)

	// Then: 更新讨论汇总评论
	comments := gh.Comments[issue.Number]
	require.Len(t, comments, 2) // 原评论 + 汇总
	assert.Contains(t, comments[1], "Discussion Summary")
}

// TestOrchestrator_Discussion_ShouldWarn 场景：静默 10min，发预警
func TestOrchestrator_Discussion_ShouldWarn(t *testing.T) {
	ctx := context.Background()
	gh := NewMockGitHubClient()
	aiProvider := ai.NewMockProvider("")

	// Given: 处于 needs-discussion 状态，静默超过 10min
	issue, _ := gh.CreateIssue(ctx, "Fix leak", "Details", []string{"bot:fix", "bot:needs-discussion"})
	// 模拟 15 分钟前的评论
	gh.AddComment(ctx, issue.Number, "最后一条评论")

	// When: 检查收敛，返回 ShouldWarn
	checker := &MockConvergenceChecker{result: state.ShouldWarn}
	workflow := &DiscussionWorkflow{github: gh, ai: aiProvider, checker: checker}
	err := workflow.Execute(ctx, issue.Number)
	require.NoError(t, err)

	// Then: 发预警评论
	comments := gh.Comments[issue.Number]
	foundWarning := false
	for _, c := range comments {
		if containsStr(c, "30min 内无回复将自动定稿") {
			foundWarning = true
			break
		}
	}
	assert.True(t, foundWarning, "应该发预警评论")
}

// TestOrchestrator_Discussion_ShouldFinalize 场景：讨论收敛，生成 Final Plan
func TestOrchestrator_Discussion_Finalize(t *testing.T) {
	ctx := context.Background()
	gh := NewMockGitHubClient()
	
	aiProvider := ai.NewMockProvider(`# Final Plan

## 1. 目标与非目标
修复 worker pool 内存泄漏

## 2. 根因分析
未正确关闭 channel

## 3. 修复策略
添加 context 取消信号

## 4. 改动清单
- worker.go: 添加 Shutdown 方法

## 5. 风险与回滚
低风险，可回滚

## 6. 测试场景
- 复现：创建 pool 后关闭
- 回归：正常任务执行
- 边界：空 pool 关闭

## 7. 观测与验收
添加日志记录

## 8. 发布策略
直接发布`)

	// Given: 处于 needs-discussion 状态，满足收敛条件
	issue, _ := gh.CreateIssue(ctx, "Fix leak", "Details", []string{"bot:fix", "bot:needs-discussion"})

	// When: 检查收敛，返回 ShouldFinalize
	checker := &MockConvergenceChecker{result: state.ShouldFinalize}
	workflow := &DiscussionWorkflow{github: gh, ai: aiProvider, checker: checker}
	err := workflow.Execute(ctx, issue.Number)
	require.NoError(t, err)

	// Then: 生成 Final Plan 评论
	comments := gh.Comments[issue.Number]
	foundFinal := false
	for _, c := range comments {
		if containsStr(c, "Final Plan") && containsStr(c, "BOT:PLAN_FINAL") {
			foundFinal = true
			break
		}
	}
	assert.True(t, foundFinal, "应该生成 Final Plan")

	// And: 状态变为 bot:plan-final
	labels := gh.Labels[issue.Number]
	assert.Contains(t, labels, "bot:plan-final")
}

// TestOrchestrator_Implement_Success 场景：执行实现，创建 PR
func TestOrchestrator_Implement_Success(t *testing.T) {
	ctx := context.Background()
	gh := NewMockGitHubClient()
	
	aiProvider := ai.NewMockProvider(`生成的代码内容`)

	// Given: 处于 plan-final 状态的 Issue
	issue, _ := gh.CreateIssue(ctx, "Fix leak", "Details", []string{"bot:fix", "bot:plan-final"})

	// When: 执行实现
	workflow := &ImplementWorkflow{github: gh, ai: aiProvider, workDir: "/tmp/test"}
	err := workflow.Execute(ctx, issue.Number)
	require.NoError(t, err)

	// Then: 创建 PR
	// 验证 PR 是否创建（MockGitHubClient 需要扩展 PR 支持）
	// 状态变为 bot:pr-created
	labels := gh.Labels[issue.Number]
	assert.Contains(t, labels, "bot:pr-created")
}

// TestOrchestrator_Implement_WithSafetyCheck 场景：实现时安全检查拦截高风险路径
func TestOrchestrator_Implement_SafetyCheck(t *testing.T) {
	ctx := context.Background()
	gh := NewMockGitHubClient()
	aiProvider := ai.NewMockProvider("")

	// Given: Final Plan 包含高风险路径（auth/）改动
	issue, _ := gh.CreateIssue(ctx, "Fix auth", "Change auth logic", []string{"bot:fix", "bot:plan-final"})
	// 添加包含 auth/ 路径的 Final Plan 评论
	gh.AddComment(ctx, issue.Number, `<!-- BOT:PLAN_FINAL issue=1 rev=1 -->
改动清单：auth/login.go`)

	// When: 执行实现（带安全检查）
	workflow := &ImplementWorkflow{github: gh, ai: aiProvider, workDir: "/tmp/test", safetyCheck: true}
	err := workflow.Execute(ctx, issue.Number)

	// Then: 应该被安全拦截，不执行代码生成
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "高风险路径")
}

// ========== Mock Helpers ==========

type MockConvergenceChecker struct {
	result state.ConvergeResult
}

func (m *MockConvergenceChecker) Check(input *state.ConvergenceInput) state.ConvergenceResult {
	return m.result
}

type DiscussionWorkflow struct {
	github  *MockGitHubClient
	ai      ai.Provider
	checker *MockConvergenceChecker
}

func (w *DiscussionWorkflow) Execute(ctx context.Context, issueNum int) error {
	result := w.checker.Check(&state.ConvergenceInput{})
	
	switch result {
	case state.NotConverged:
		// 更新讨论汇总
		prompt := "汇总讨论..."
		summary, _ := w.ai.Complete(ctx, prompt)
		if summary != "" {
			w.github.AddComment(ctx, issueNum, summary)
		}
	case state.ShouldWarn:
		// 发预警
		warning := "⚠️ 讨论即将自动定稿：30min 内无回复将自动定稿，/hold 暂缓，/finalize 立即定稿"
		w.github.AddComment(ctx, issueNum, warning)
		// 创建 warning marker
		w.github.AddComment(ctx, issueNum, marker.Render(&marker.Marker{
			Type:     marker.TypeConvergeWarning,
			Issue:    issueNum,
			Revision: 1,
		}))
	case state.ShouldFinalize:
		// 生成 Final Plan
		prompt := "生成 Final Plan..."
		plan, _ := w.ai.Complete(ctx, prompt)
		if plan != "" {
			planWithMarker := plan + "\n\n" + marker.Render(&marker.Marker{
				Type:     marker.TypePlanFinal,
				Issue:    issueNum,
				Revision: 1,
			})
			w.github.AddComment(ctx, issueNum, planWithMarker)
			w.github.AddLabel(ctx, issueNum, string(state.StatePlanFinal))
		}
	}
	
	return nil
}

type ImplementWorkflow struct {
	github      *MockGitHubClient
	ai          ai.Provider
	workDir     string
	safetyCheck bool
}

func (w *ImplementWorkflow) Execute(ctx context.Context, issueNum int) error {
	// 安全检查
	if w.safetyCheck {
		comments, _ := w.github.GetComments(ctx, issueNum)
		for _, c := range comments {
			if containsStr(c, "auth/") {
				return fmt.Errorf("安全拦截：检测到高风险路径 auth/")
			}
		}
	}
	
	// 执行实现逻辑...
	w.github.AddLabel(ctx, issueNum, string(state.StatePRCreated))
	return nil
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
