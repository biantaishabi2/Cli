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

func TestBuildConsolidatePrompt(t *testing.T) {
	input := &PromptInput{
		IssueTitle: "Add caching",
		IssueBody:  "We need Redis caching",
		Comments:   []string{"Redis is fine", "Consider TTL settings"},
	}

	result, err := BuildConsolidatePrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Add caching")
	assert.Contains(t, result, "Redis is fine")
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

func TestBuildPrompt_EmptyInput(t *testing.T) {
	input := &PromptInput{}
	result, err := BuildDraftPrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result) // 模板本身有固定文本
}
