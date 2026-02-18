//go:build !ci

package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDraftPrompt(t *testing.T) {
	input := &PromptInput{
		IssueTitle: "Fix login bug",
		IssueBody:  "Login fails when password contains special chars",
		Comments:   []string{"I can reproduce this", "Same issue here"},
	}

	result, err := BuildDraftPrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Fix login bug")
	assert.Contains(t, result, "special chars")
	assert.Contains(t, result, "I can reproduce this")
}

func TestBuildDebatePrompt(t *testing.T) {
	input := &PromptInput{
		IssueTitle:     "Debate cache strategy",
		IssueBody:      "Need choose cache ttl",
		Comments:       []string{"A: prefer 30m", "B: prefer 60m"},
		Round:          2,
		MaxRounds:      5,
		DiscussionRole: "B",
		Counterpart:    "A",
	}

	result, err := BuildDebatePrompt(input)
	require.NoError(t, err)
	assert.Contains(t, result, "第 2/5 轮")
	assert.Contains(t, result, "讨论方 B")
	assert.Contains(t, result, "\"should_finish\"")
}

func TestBuildFinalPlanPrompt(t *testing.T) {
	input := &PromptInput{
		IssueTitle: "Refactor auth",
		IssueBody:  "Move to JWT",
		Comments:   []string{"JWT with refresh tokens"},
	}

	result, err := BuildFinalPlanPrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Refactor auth")
	assert.Contains(t, result, "JSON")
}

func TestBuildImplementPrompt(t *testing.T) {
	input := &PromptInput{
		IssueTitle: "Add user API",
		FinalPlan:  "## 方案\n创建 /api/users 端点",
	}

	result, err := BuildImplementPrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Add user API")
	assert.Contains(t, result, "/api/users")
}

func TestBuildIteratePrompt(t *testing.T) {
	input := &PromptInput{
		IssueTitle:    "Add user API",
		FinalPlan:     "## 方案\n创建端点",
		ReviewComment: "请添加输入验证",
		PRDiff:        "+func CreateUser() {}",
	}

	result, err := BuildIteratePrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "请添加输入验证")
	assert.Contains(t, result, "CreateUser")
}

func TestBuildReviewPrompt_CrossFunctionAndPlanChecks(t *testing.T) {
	input := &PromptInput{
		IssueTitle: "test issue",
		FinalPlan:  "some plan",
		PRDiff:     "some diff",
	}

	result, err := BuildReviewPrompt(input)
	require.NoError(t, err)

	// 新增检查维度存在
	assert.Contains(t, result, "4. **跨函数逻辑一致性**")
	assert.Contains(t, result, "5. **方案 ↔ 实现完整性**")

	// P1 分级标准包含新描述
	assert.Contains(t, result, "跨函数逻辑不一致、方案要求未实现")

	// 现有检查项未被破坏
	assert.Contains(t, result, "测试覆盖缺口")
	assert.Contains(t, result, "CLI 接口一致性")
	assert.Contains(t, result, "错误处理完整性")
}

func TestBuildPrompt_EmptyInput(t *testing.T) {
	input := &PromptInput{}
	result, err := BuildDraftPrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result) // 模板本身有固定文本
}
