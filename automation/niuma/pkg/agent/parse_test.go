//go:build !ci

package agent

import (
	"errors"
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

func TestParseDebateResponse_ValidJSON(t *testing.T) {
	raw := `本轮我接受对方关于兼容性的观点，但默认值仍建议保守推进。
` + "```json" + `
{"should_finish": false}
` + "```"
	comment, err := ParseDebateResponse(raw)
	require.NoError(t, err)
	assert.Contains(t, comment.Body, "默认值仍建议保守推进")
	assert.False(t, comment.ShouldFinish)
}

func TestParseDebateResponse_MissingFields(t *testing.T) {
	raw := `讨论还需要继续。`
	_, err := ParseDebateResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "should_finish")
}

func TestParseDebateResponse_MissingShouldFinishField(t *testing.T) {
	raw := `继续讨论。
` + "```json" + `
{}
` + "```"
	_, err := ParseDebateResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "should_finish")
}

func TestParseDebateResponse_NakedJSONTail(t *testing.T) {
	raw := `目前还有一个边界条件待确认。
{"should_finish":true}`
	comment, err := ParseDebateResponse(raw)
	require.NoError(t, err)
	assert.Contains(t, comment.Body, "边界条件")
	assert.True(t, comment.ShouldFinish)
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

func TestParseReviewResponse_RejectsNonJSON(t *testing.T) {
	raw := "代码有一些问题需要修复。"
	_, err := ParseReviewResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "缺少 JSON")
}

func TestParseReviewResponse_Empty(t *testing.T) {
	_, err := ParseReviewResponse("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "空响应")
}

func TestParseReviewResponse_JSONBuriedInMarkdown(t *testing.T) {
	// 模拟真实场景：AI 输出大量 markdown 分析，JSON 埋在末尾
	raw := `## 审查结论

| # | 问题 | 状态 |
|---|------|------|
| 1 | shell 注入 | **已修复** |
| 2 | 路径穿越 | **已修复** |

所有问题已解决，建议合并。

` + "```json" + `
{"approved": true, "summary": "所有历史问题已解决", "resolved_items": ["shell注入：已修复", "路径穿越：已修复"], "issues": []}
` + "```"

	result, err := ParseReviewResponse(raw)
	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Equal(t, "所有历史问题已解决", result.Summary)
	assert.Len(t, result.ResolvedItems, 2)
}

func TestParseReviewResponse_NakedJSONAfterMarkdown(t *testing.T) {
	// JSON 不在代码块中，前面有包含 {} 的 markdown 内容
	raw := `分析 {结构体} 和 {接口} 后，结论如下：
{"approved": true, "summary": "通过审查", "issues": []}`

	result, err := ParseReviewResponse(raw)
	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Equal(t, "通过审查", result.Summary)
}

func TestExtractJSON_MultipleCodeBlocks(t *testing.T) {
	// 多个 json 代码块，应取最后一个（结论 JSON）
	text := "分析：\n```json\n{\"type\": \"analysis\"}\n```\n\n结论：\n```json\n{\"approved\": true}\n```"
	jsonStr := extractJSON(text)
	assert.Contains(t, jsonStr, "approved")
	assert.NotContains(t, jsonStr, "analysis")
}

func TestParseReviewResponse_MissingApprovedField(t *testing.T) {
	raw := `{"summary":"有结论但没有 approved", "issues":[]}`
	_, err := ParseReviewResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "approved")
}

func TestParseReviewResponse_MissingSummaryField(t *testing.T) {
	raw := `{"approved":false, "issues":["P1 - 仍需修改"]}`
	_, err := ParseReviewResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "summary")
}

func TestExtractJSON_BracesInText(t *testing.T) {
	// 文本中有大量 {}，但末尾有合法 JSON
	text := `代码中 func() { return } 和 type Foo struct { Bar string } 都正常。
{"approved": false, "summary": "有问题", "issues": ["bug1"]}`
	jsonStr := extractJSON(text)
	assert.Contains(t, jsonStr, `"approved"`)
	assert.Contains(t, jsonStr, `"bug1"`)
}

// ===== ParseDebateResponse RecoverableError 测试 =====

func TestParseDebateResponse_EmptyResponse_RecoverableError(t *testing.T) {
	_, err := ParseDebateResponse("")
	require.Error(t, err)
	var recErr *RecoverableError
	require.True(t, errors.As(err, &recErr))
	assert.Equal(t, EmptyResponse, recErr.Kind)
}

func TestParseDebateResponse_MissingJSON_RecoverableError(t *testing.T) {
	_, err := ParseDebateResponse("我认为方案合理，建议采纳")
	require.Error(t, err)
	var recErr *RecoverableError
	require.True(t, errors.As(err, &recErr))
	assert.Equal(t, MissingJSON, recErr.Kind)
}

func TestParseDebateResponse_JSONParseError_RecoverableError(t *testing.T) {
	raw := "分析如下\n```json\n{should_finish: yes}\n```"
	_, err := ParseDebateResponse(raw)
	require.Error(t, err)
	var recErr *RecoverableError
	require.True(t, errors.As(err, &recErr))
	assert.Equal(t, JSONParseError, recErr.Kind)
	// Unwrap 返回底层 json 解析错误
	assert.NotNil(t, recErr.Unwrap())
}

func TestParseDebateResponse_MissingField_RecoverableError(t *testing.T) {
	raw := "分析如下\n```json\n{\"other_field\": true}\n```"
	_, err := ParseDebateResponse(raw)
	require.Error(t, err)
	var recErr *RecoverableError
	require.True(t, errors.As(err, &recErr))
	assert.Equal(t, MissingField, recErr.Kind)
}

func TestParseDebateResponse_Success_Unchanged(t *testing.T) {
	raw := "本轮达成共识。\n```json\n{\"should_finish\": true}\n```"
	comment, err := ParseDebateResponse(raw)
	require.NoError(t, err)
	require.NotNil(t, comment)
	assert.True(t, comment.ShouldFinish)
	assert.NotContains(t, comment.Body, "should_finish")
	assert.Contains(t, comment.Body, "本轮达成共识")
}

// ===== ParseReviewResponse RecoverableError 测试 =====

func TestParseReviewResponse_EmptyResponse_RecoverableError(t *testing.T) {
	_, err := ParseReviewResponse("")
	require.Error(t, err)
	var recErr *RecoverableError
	require.True(t, errors.As(err, &recErr))
	assert.Equal(t, EmptyResponse, recErr.Kind)
}

func TestParseReviewResponse_MissingJSON_RecoverableError(t *testing.T) {
	_, err := ParseReviewResponse("代码有一些问题需要修复。")
	require.Error(t, err)
	var recErr *RecoverableError
	require.True(t, errors.As(err, &recErr))
	assert.Equal(t, MissingJSON, recErr.Kind)
}

func TestParseReviewResponse_JSONParseError_RecoverableError(t *testing.T) {
	raw := "```json\n{approved: yes}\n```"
	_, err := ParseReviewResponse(raw)
	require.Error(t, err)
	var recErr *RecoverableError
	require.True(t, errors.As(err, &recErr))
	assert.Equal(t, JSONParseError, recErr.Kind)
	assert.NotNil(t, recErr.Unwrap())
}

func TestParseReviewResponse_MissingApproved_RecoverableError(t *testing.T) {
	raw := `{"summary":"有结论但没有 approved", "issues":[]}`
	_, err := ParseReviewResponse(raw)
	require.Error(t, err)
	var recErr *RecoverableError
	require.True(t, errors.As(err, &recErr))
	assert.Equal(t, MissingField, recErr.Kind)
	assert.Contains(t, recErr.Message, "approved")
}

func TestParseReviewResponse_MissingSummary_RecoverableError(t *testing.T) {
	raw := `{"approved":false, "issues":["P1 - 仍需修改"]}`
	_, err := ParseReviewResponse(raw)
	require.Error(t, err)
	var recErr *RecoverableError
	require.True(t, errors.As(err, &recErr))
	assert.Equal(t, MissingField, recErr.Kind)
	assert.Contains(t, recErr.Message, "summary")
}

// ===== BuildFormatRepairPromptFor 测试 =====

func TestBuildFormatRepairPromptFor_DebateFields(t *testing.T) {
	prompt := BuildFormatRepairPromptFor("原始内容", debateFields)
	assert.Contains(t, prompt, "should_finish")
	assert.Contains(t, prompt, "bool")
	assert.Contains(t, prompt, "原始内容")
	assert.Contains(t, prompt, "必填")
}

func TestBuildFormatRepairPromptFor_PlanFinalFields(t *testing.T) {
	prompt := BuildFormatRepairPromptFor("纯文本方案", planFinalFields)
	assert.Contains(t, prompt, "title")
	assert.Contains(t, prompt, "approach")
	assert.Contains(t, prompt, "file_changes")
	assert.Contains(t, prompt, "test_scenarios")
	assert.Contains(t, prompt, "纯文本方案")
}

func TestBuildFormatRepairPromptFor_ReviewFields(t *testing.T) {
	prompt := BuildFormatRepairPromptFor("审查原文", reviewFields)
	assert.Contains(t, prompt, "approved")
	assert.Contains(t, prompt, "summary")
	assert.Contains(t, prompt, "resolved_items")
	assert.Contains(t, prompt, "issues")
}

// ===== RepairAndParseFinalPlan 测试 =====

func TestRepairAndParseFinalPlan_Success(t *testing.T) {
	repairRaw := "```json\n{\"title\":\"修复方案\",\"approach\":\"实现修复\"}\n```"
	plan, err := RepairAndParseFinalPlan("原始文本", repairRaw)
	require.NoError(t, err)
	assert.Equal(t, "修复方案", plan.Title)
	assert.Equal(t, "实现修复", plan.Approach)
}

func TestRepairAndParseFinalPlan_MissingTitle(t *testing.T) {
	repairRaw := `{"approach":"有方案但没标题"}`
	_, err := RepairAndParseFinalPlan("原始", repairRaw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestRepairAndParseFinalPlan_MissingApproach(t *testing.T) {
	repairRaw := `{"title":"有标题但没方案"}`
	_, err := RepairAndParseFinalPlan("原始", repairRaw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "approach")
}

func TestRepairAndParseFinalPlan_NoJSON(t *testing.T) {
	_, err := RepairAndParseFinalPlan("原始", "依然是纯文本")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON")
}

// ===== RepairAndParseReview 测试 =====

func TestRepairAndParseReview_Success(t *testing.T) {
	repairRaw := `{"approved":true,"summary":"代码质量良好"}`
	result, err := RepairAndParseReview("原始文本", repairRaw)
	require.NoError(t, err)
	assert.True(t, result.Approved)
	assert.Equal(t, "代码质量良好", result.Summary)
}

func TestRepairAndParseReview_MissingApproved(t *testing.T) {
	repairRaw := `{"summary":"有总结但没结论"}`
	_, err := RepairAndParseReview("原始", repairRaw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "approved")
}

func TestRepairAndParseReview_MissingSummary(t *testing.T) {
	repairRaw := `{"approved":false}`
	_, err := RepairAndParseReview("原始", repairRaw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "summary")
}

func TestRepairAndParseReview_NoJSON(t *testing.T) {
	_, err := RepairAndParseReview("原始", "没有JSON")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON")
}
