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
	raw := `{
  "agreements": ["使用 Redis"],
  "disagreements": [
    {"topic":"TTL 策略","options":["30m","60m"],"recommendation":"优先 30m","risk":"low"}
  ],
  "decision": "merge",
  "requires_human_decision": false,
  "should_finish": true
}`

	summary, err := ParseConsolidateResponse(raw)
	require.NoError(t, err)
	assert.Equal(t, []string{"使用 Redis"}, summary.Agreements)
	assert.Equal(t, DecisionMerge, summary.Decision)
	require.Len(t, summary.Disagreements, 1)
	assert.Equal(t, "TTL 策略", summary.Disagreements[0].Topic)
	assert.Equal(t, RiskLow, summary.Disagreements[0].Risk)
	assert.True(t, summary.ShouldFinish)
}

func TestParseConsolidateResponse_MissingRequiredFields(t *testing.T) {
	raw := `{
  "agreements": ["a"],
  "disagreements": [],
  "requires_human_decision": false
}`
	_, err := ParseConsolidateResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "缺少必填字段")
	assert.Contains(t, err.Error(), "decision")
	assert.Contains(t, err.Error(), "should_finish")
}

func TestParseConsolidateResponse_InvalidDecision(t *testing.T) {
	raw := `{
  "agreements": [],
  "disagreements": [],
  "decision": "accept",
  "requires_human_decision": false,
  "should_finish": false
}`
	_, err := ParseConsolidateResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decision 非法")
}

func TestParseConsolidateResponse_InvalidRisk(t *testing.T) {
	raw := `{
  "agreements": [],
  "disagreements": [{"topic":"t","options":["a"],"recommendation":"x","risk":"critical"}],
  "decision": "merge",
  "requires_human_decision": false,
  "should_finish": false
}`
	_, err := ParseConsolidateResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "risk 非法")
}

func TestParseConsolidateResponse_TypeError(t *testing.T) {
	raw := `{
  "agreements": "bad",
  "disagreements": [],
  "decision": "merge",
  "requires_human_decision": false,
  "should_finish": false
}`
	_, err := ParseConsolidateResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agreements 类型错误")
}

func TestParseConsolidateResponse_RejectNullListFields(t *testing.T) {
	raw := `{
  "agreements": null,
  "disagreements": [],
  "decision": "merge",
  "requires_human_decision": false,
  "should_finish": false
}`
	_, err := ParseConsolidateResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agreements 不能为 null")

	raw = `{
  "agreements": [],
  "disagreements": null,
  "decision": "merge",
  "requires_human_decision": false,
  "should_finish": false
}`
	_, err = ParseConsolidateResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disagreements 不能为 null")
}

func TestParseDebateResponse_ValidJSON(t *testing.T) {
	raw := `{
  "agreements": ["同意使用 Redis"],
  "disagreements": [{"topic":"TTL","options":["30m","60m"],"recommendation":"30m","risk":"medium"}],
  "suggestion": "先压测再定默认值"
}`
	comment, err := ParseDebateResponse(raw)
	require.NoError(t, err)
	assert.Equal(t, "先压测再定默认值", comment.Suggestion)
	require.Len(t, comment.Disagreements, 1)
	assert.Equal(t, RiskMedium, comment.Disagreements[0].Risk)
}

func TestParseDebateResponse_MissingFields(t *testing.T) {
	raw := `{"agreements":[],"disagreements":[]}`
	_, err := ParseDebateResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "缺少必填字段")
	assert.Contains(t, err.Error(), "suggestion")
}

func TestParseDebateResponse_RejectNullListFields(t *testing.T) {
	raw := `{
  "agreements": null,
  "disagreements": [],
  "suggestion": "继续"
}`
	_, err := ParseDebateResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agreements 不能为 null")

	raw = `{
  "agreements": [],
  "disagreements": null,
  "suggestion": "继续"
}`
	_, err = ParseDebateResponse(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disagreements 不能为 null")
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
