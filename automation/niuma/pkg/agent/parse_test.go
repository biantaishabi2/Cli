//go:build !ci

package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDraftResponse_ValidJSON(t *testing.T) {
	raw := `{"summary": "修复登录bug", "approach": "检查特殊字符编码", "affected_files": ["auth.go"]}`

	plan, err := ParseDraftResponse(raw)
	require.NoError(t, err)
	assert.Equal(t, "修复登录bug", plan.Summary)
	assert.Equal(t, "检查特殊字符编码", plan.Approach)
	assert.Equal(t, []string{"auth.go"}, plan.AffectedFiles)
}

func TestParseDraftResponse_JSONInCodeBlock(t *testing.T) {
	raw := "这是我的分析：\n```json\n{\"summary\": \"概述\", \"approach\": \"方案\"}\n```\n\n以上是分析结果。"

	plan, err := ParseDraftResponse(raw)
	require.NoError(t, err)
	assert.Equal(t, "概述", plan.Summary)
	assert.Equal(t, "方案", plan.Approach)
}

func TestParseDraftResponse_NonJSONFallback(t *testing.T) {
	raw := "这是一段纯文本的分析结果，没有 JSON 格式。"

	plan, err := ParseDraftResponse(raw)
	require.NoError(t, err)
	assert.Equal(t, raw, plan.Summary)
	assert.Equal(t, raw, plan.Approach)
}

func TestParseDraftResponse_Empty(t *testing.T) {
	_, err := ParseDraftResponse("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "空响应")
}

func TestParseConsolidateResponse_ValidJSON(t *testing.T) {
	raw := `{"consensus": "使用 Redis", "open_items": ["TTL 策略"], "should_finish": true}`

	summary, err := ParseConsolidateResponse(raw)
	require.NoError(t, err)
	assert.Equal(t, "使用 Redis", summary.Consensus)
	assert.Equal(t, []string{"TTL 策略"}, summary.OpenItems)
	assert.True(t, summary.ShouldFinish)
}

func TestParseConsolidateResponse_Fallback(t *testing.T) {
	raw := "大家基本同意使用 Redis 作为缓存方案。"

	summary, err := ParseConsolidateResponse(raw)
	require.NoError(t, err)
	assert.Equal(t, raw, summary.Consensus)
}

func TestParseFinalPlanResponse_ValidJSON(t *testing.T) {
	raw := `{
		"title": "实现用户API",
		"approach": "创建 RESTful 端点",
		"file_changes": [
			{"path": "api/users.go", "action": "create", "description": "用户API处理器"}
		],
		"test_scenarios": [
			{"name": "创建用户", "input": "POST /users", "expected": "201 Created"}
		]
	}`

	plan, err := ParseFinalPlanResponse(raw)
	require.NoError(t, err)
	assert.Equal(t, "实现用户API", plan.Title)
	assert.Len(t, plan.FileChanges, 1)
	assert.Equal(t, "api/users.go", plan.FileChanges[0].Path)
	assert.Len(t, plan.TestScenarios, 1)
}

func TestParseFinalPlanResponse_MissingTitle(t *testing.T) {
	raw := `{"approach": "some approach", "file_changes": []}`

	_, err := ParseFinalPlanResponse(raw)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestParseFinalPlanResponse_MissingApproach(t *testing.T) {
	raw := `{"title": "Some Title", "file_changes": []}`

	_, err := ParseFinalPlanResponse(raw)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "approach")
}

func TestParseFinalPlanResponse_NonJSON(t *testing.T) {
	raw := "这是纯文本，不是 JSON"

	_, err := ParseFinalPlanResponse(raw)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "非 JSON")
}

func TestParseFinalPlanResponse_JSONWithSurroundingText(t *testing.T) {
	raw := `好的，这是最终方案：
{"title": "重构认证", "approach": "迁移到 JWT", "file_changes": [], "test_scenarios": []}
以上就是完整方案。`

	plan, err := ParseFinalPlanResponse(raw)
	require.NoError(t, err)
	assert.Equal(t, "重构认证", plan.Title)
	assert.Equal(t, "迁移到 JWT", plan.Approach)
}

// ===== ParseReviewResponse 测试 =====

func TestParseReviewResponse_Approved(t *testing.T) {
	raw := `{"approved": true, "summary": "代码质量良好，符合方案要求", "issues": []}`

	result, err := ParseReviewResponse(raw)
	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Equal(t, "代码质量良好，符合方案要求", result.Summary)
	assert.Empty(t, result.Issues)
}

func TestParseReviewResponse_NotApproved(t *testing.T) {
	raw := `{"approved": false, "summary": "发现若干问题", "issues": ["缺少错误处理", "变量命名不规范"]}`

	result, err := ParseReviewResponse(raw)
	require.NoError(t, err)
	assert.False(t, result.Approved)
	assert.Equal(t, "发现若干问题", result.Summary)
	assert.Len(t, result.Issues, 2)
	assert.Equal(t, "缺少错误处理", result.Issues[0])
}

func TestParseReviewResponse_InCodeBlock(t *testing.T) {
	raw := "审查结果如下：\n```json\n{\"approved\": true, \"summary\": \"通过\"}\n```"

	result, err := ParseReviewResponse(raw)
	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Equal(t, "通过", result.Summary)
}

func TestParseReviewResponse_Fallback(t *testing.T) {
	raw := "代码有一些问题需要修复。"

	result, err := ParseReviewResponse(raw)
	require.NoError(t, err)
	assert.False(t, result.Approved)
	assert.Equal(t, raw, result.Summary)
}

func TestParseReviewResponse_Empty(t *testing.T) {
	_, err := ParseReviewResponse("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "空响应")
}
