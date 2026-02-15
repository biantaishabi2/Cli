package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanEngine_Draft_HappyPath(t *testing.T) {
	mock := ai.NewMockProvider(`{"summary": "修复bug", "approach": "添加验证", "affected_files": ["auth.go"]}`)
	engine := NewPlanEngine(mock)

	input := &PromptInput{
		IssueTitle: "Fix auth bug",
		IssueBody:  "Auth fails",
	}

	plan, err := engine.Draft(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "修复bug", plan.Summary)
	assert.Equal(t, "添加验证", plan.Approach)
	assert.Equal(t, 1, mock.CallCount())

	// 验证 prompt 包含 issue 信息
	calls := mock.Calls()
	assert.Contains(t, calls[0].Prompt, "Fix auth bug")
}

func TestPlanEngine_Draft_AIError(t *testing.T) {
	mock := ai.NewMockProviderWithError(fmt.Errorf("network timeout"))
	engine := NewPlanEngine(mock)

	_, err := engine.Draft(context.Background(), &PromptInput{IssueTitle: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI 生成草案失败")
}

func TestPlanEngine_Consolidate_HappyPath(t *testing.T) {
	mock := ai.NewMockProvider(`{"consensus": "用JWT", "open_items": ["TTL"], "should_finish": false}`)
	engine := NewPlanEngine(mock)

	input := &PromptInput{
		IssueTitle: "Auth refactor",
		IssueBody:  "Move to JWT",
		Comments:   []string{"JWT sounds good", "What about TTL?"},
	}

	summary, err := engine.Consolidate(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, "用JWT", summary.Consensus)
	assert.Equal(t, []string{"TTL"}, summary.OpenItems)

	// 验证 prompt 包含评论
	calls := mock.Calls()
	assert.Contains(t, calls[0].Prompt, "JWT sounds good")
}

func TestPlanEngine_Final_HappyPath(t *testing.T) {
	mock := ai.NewMockProvider(`{
		"title": "JWT认证",
		"approach": "使用JWT替换session",
		"file_changes": [{"path": "auth.go", "action": "modify", "description": "添加JWT验证"}],
		"test_scenarios": [{"name": "有效token", "input": "valid JWT", "expected": "200 OK"}]
	}`)
	engine := NewPlanEngine(mock)

	plan, err := engine.Final(context.Background(), &PromptInput{
		IssueTitle: "JWT migration",
		IssueBody:  "Migrate auth",
		Comments:   []string{"Approved"},
	})
	require.NoError(t, err)
	assert.Equal(t, "JWT认证", plan.Title)
	assert.Len(t, plan.FileChanges, 1)
	assert.Len(t, plan.TestScenarios, 1)
}

func TestPlanEngine_Final_AIError(t *testing.T) {
	mock := ai.NewMockProviderWithError(fmt.Errorf("rate limit"))
	engine := NewPlanEngine(mock)

	_, err := engine.Final(context.Background(), &PromptInput{IssueTitle: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI 生成最终方案失败")
}

func TestPlanEngine_Final_ParseError(t *testing.T) {
	// AI 返回缺少必填字段的 JSON
	mock := ai.NewMockProvider(`{"file_changes": []}`)
	engine := NewPlanEngine(mock)

	_, err := engine.Final(context.Background(), &PromptInput{IssueTitle: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestPlanEngine_Draft_PromptContainsComments(t *testing.T) {
	mock := ai.NewMockProvider(`{"summary": "s", "approach": "a"}`)
	engine := NewPlanEngine(mock)

	input := &PromptInput{
		IssueTitle: "Test",
		IssueBody:  "Body",
		Comments:   []string{"comment1", "comment2"},
	}

	_, err := engine.Draft(context.Background(), input)
	require.NoError(t, err)

	calls := mock.Calls()
	assert.Contains(t, calls[0].Prompt, "comment1")
	assert.Contains(t, calls[0].Prompt, "comment2")
}
