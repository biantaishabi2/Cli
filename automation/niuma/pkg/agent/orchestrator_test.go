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
	mockAI := ai.NewMockProvider("// 实现代码\nfunc Login() {}")
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

	// 验证 prompt 包含 workdir
	calls := mockAI.Calls()
	assert.Equal(t, "/tmp/work", calls[0].Options.WorkDir)
}

func TestDoIterate_HappyPath(t *testing.T) {
	mockAI := ai.NewMockProvider("// 修复代码\nfunc Login() { validate() }")
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
