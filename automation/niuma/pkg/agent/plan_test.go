//go:build !ci

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

// ===== FinalWithRetry 测试 =====

func TestFinalWithRetry_RetrySuccess(t *testing.T) {
	// 主 provider 第 1 次返回非 JSON，第 2 次返回有效 JSON
	mock := ai.NewMockProvider(
		"Here is the plan in natural language...",
		`{"title":"JWT认证","approach":"使用JWT","file_changes":[],"test_scenarios":[]}`,
	)
	engine := NewPlanEngine(mock)

	input := &PromptInput{
		IssueTitle: "JWT migration",
		IssueBody:  "Migrate auth",
	}

	plan, err := engine.FinalWithRetry(context.Background(), input, []ai.Provider{mock}, 1)
	require.NoError(t, err)
	assert.Equal(t, "JWT认证", plan.Title)
	assert.Equal(t, 2, mock.CallCount())
}

func TestFinalWithRetry_FallbackSuccess(t *testing.T) {
	// 主 provider 持续返回非 JSON，fallback provider 首次返回有效 JSON
	// WithRecovery 会在首次 parse 失败后尝试 repair（消耗 1 次额外调用），
	// 因此 primary 需要 3 个响应：原始调用 + repair + 重试。
	primary := ai.NewMockProvider(
		"not json 1",
		"repair also bad",
		"not json 2",
	)
	fallback := ai.NewMockProvider(
		`{"title":"方案B","approach":"fallback方案","file_changes":[],"test_scenarios":[]}`,
	)
	engine := NewPlanEngine(primary)

	input := &PromptInput{
		IssueTitle: "Test",
		IssueBody:  "Body",
	}

	plan, err := engine.FinalWithRetry(context.Background(), input, []ai.Provider{primary, fallback}, 1)
	require.NoError(t, err)
	assert.Equal(t, "方案B", plan.Title)
	assert.Equal(t, 3, primary.CallCount()) // 原始调用 + repair + 重试
	assert.Equal(t, 1, fallback.CallCount())
}

func TestFinalWithRetry_AllParseFail_FallbackRawText(t *testing.T) {
	// 两个 provider 均返回非 JSON，降级为原文
	// WithRecovery 在首次 parse 失败后尝试 repair（消耗 1 次额外调用），
	// 因此每个 provider 需要 3 个响应：原始调用 + repair + 重试。
	p1 := ai.NewMockProvider("not json", "repair also bad", "not json retry")
	p2 := ai.NewMockProvider("still not json", "repair also bad", "still not json retry")
	engine := NewPlanEngine(p1)

	input := &PromptInput{
		IssueTitle: "Test",
		IssueBody:  "Body",
	}

	plan, err := engine.FinalWithRetry(context.Background(), input, []ai.Provider{p1, p2}, 1)
	require.NoError(t, err)
	assert.Equal(t, "still not json retry", plan.Approach) // 最后一次的原文
	assert.Equal(t, "", plan.Title)                         // title 为空，由调用方兜底
}

func TestFinalWithRetry_AllProviderError(t *testing.T) {
	// 所有 provider 网络错误，无有效响应，仍返回错误
	p1 := ai.NewMockProviderWithError(fmt.Errorf("rate limit"))
	p2 := ai.NewMockProviderWithError(fmt.Errorf("timeout"))
	engine := NewPlanEngine(p1)

	input := &PromptInput{
		IssueTitle: "Test",
		IssueBody:  "Body",
	}

	_, err := engine.FinalWithRetry(context.Background(), input, []ai.Provider{p1, p2}, 1)
	require.Error(t, err)

	var aggErr *AggregateRetryError
	require.ErrorAs(t, err, &aggErr)
	assert.Len(t, aggErr.Attempts, 2) // 每个 provider 1 次（网络错误不重试）
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
