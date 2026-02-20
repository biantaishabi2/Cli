// pkg/agent/parse.go
// AI 响应解析：优先 JSON，失败则 fallback 到原始文本
package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	// reviewApprovedSentinelKey 是 review 结果的机器判定键（主键）。
	reviewApprovedSentinelKey = "__niuma_review_approved_v1__"
	// reviewApprovedLegacyKey 是历史兼容键（兜底读取）。
	reviewApprovedLegacyKey = "approved"
)

// RecoverableErrorKind 可恢复错误的分类
type RecoverableErrorKind string

const (
	// EmptyResponse AI 返回空响应
	EmptyResponse RecoverableErrorKind = "EmptyResponse"
	// MissingJSON AI 返回中缺少 JSON
	MissingJSON RecoverableErrorKind = "MissingJSON"
	// JSONParseError JSON 解析失败
	JSONParseError RecoverableErrorKind = "JSONParseError"
	// MissingField JSON 解析成功但缺少必填字段
	MissingField RecoverableErrorKind = "MissingField"
)

// RecoverableError 可恢复的解析错误，供重试层判断是否值得重试
type RecoverableError struct {
	Kind    RecoverableErrorKind
	Err     error
	Message string
}

func (e *RecoverableError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Kind, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *RecoverableError) Unwrap() error {
	return e.Err
}


// jsonBlockRe 匹配 ```json ... ``` 代码块
var jsonBlockRe = regexp.MustCompile("(?s)```json\\s*\n?(.*?)\n?```")

// ParseDraftResponse 解析 AI 返回的方案草案
func ParseDraftResponse(raw string) (*DraftPlan, error) {
	if raw == "" {
		return nil, fmt.Errorf("空响应")
	}

	jsonStr := extractJSON(raw)
	if jsonStr != "" {
		var plan DraftPlan
		if err := json.Unmarshal([]byte(jsonStr), &plan); err == nil && plan.Summary != "" {
			return &plan, nil
		}
	}

	// Fallback：原始文本作为 summary
	return &DraftPlan{
		Summary:  strings.TrimSpace(raw),
		Approach: strings.TrimSpace(raw),
	}, nil
}

// ParseDebateResponse 解析 AB 轮流评论输出。
// 协议：自然语言正文 + 末尾 JSON（仅 should_finish）。
// 所有解析错误均返回 *RecoverableError，对齐 ParseFinalPlanResponse 模式。
func ParseDebateResponse(raw string) (*DebateComment, error) {
	if raw == "" {
		return nil, &RecoverableError{Kind: EmptyResponse, Message: "debate 响应为空"}
	}

	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return nil, &RecoverableError{Kind: MissingJSON, Message: "缺少 should_finish JSON 代码块"}
	}

	type debateJSON struct {
		ShouldFinish *bool `json:"should_finish"`
	}
	var parsed debateJSON
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, &RecoverableError{Kind: JSONParseError, Err: err, Message: "debate JSON 解析失败"}
	}
	if parsed.ShouldFinish == nil {
		return nil, &RecoverableError{Kind: MissingField, Message: "debate JSON 缺少 should_finish 字段"}
	}

	body := strings.TrimSpace(stripLastJSONSegment(raw, jsonStr))

	return &DebateComment{
		Body:         body,
		ShouldFinish: *parsed.ShouldFinish,
	}, nil
}

// ParseFinalPlanResponse 解析 AI 返回的最终方案
// 所有解析错误均返回 RecoverableError，供重试层判断是否值得重试。
func ParseFinalPlanResponse(raw string) (*FinalPlan, error) {
	if raw == "" {
		return nil, &RecoverableError{Kind: EmptyResponse, Message: "AI 返回空响应"}
	}

	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return nil, &RecoverableError{Kind: MissingJSON, Message: "无法解析最终方案，AI 返回非 JSON 格式"}
	}

	var plan FinalPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, &RecoverableError{Kind: JSONParseError, Err: err, Message: "JSON 解析失败"}
	}

	// 必填字段校验——AI 偶发遗漏，重试可修复
	if plan.Title == "" {
		return nil, &RecoverableError{Kind: MissingField, Message: "最终方案缺少 title 字段"}
	}
	if plan.Approach == "" {
		return nil, &RecoverableError{Kind: MissingField, Message: "最终方案缺少 approach 字段"}
	}
	return &plan, nil
}

// ParseReviewResponse 解析 AI 返回的审查结果
// 审查结果必须是结构化 JSON，禁止纯文本兜底推断。
// 所有解析错误均返回 *RecoverableError，对齐 ParseDebateResponse / ParseFinalPlanResponse 模式。
func ParseReviewResponse(raw string) (*ReviewResult, error) {
	if raw == "" {
		return nil, &RecoverableError{Kind: EmptyResponse, Message: "审查响应为空"}
	}

	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return nil, &RecoverableError{Kind: MissingJSON, Message: "无法解析审查结果：缺少 JSON 代码块或裸 JSON 对象"}
	}

	type reviewJSON struct {
		ApprovedV1    *bool    `json:"__niuma_review_approved_v1__"`
		Approved      *bool    `json:"approved"`
		Summary       string   `json:"summary"`
		ResolvedItems []string `json:"resolved_items,omitempty"`
		Issues        []string `json:"issues,omitempty"`
	}

	var parsed reviewJSON
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, &RecoverableError{Kind: JSONParseError, Err: err, Message: "审查结果 JSON 解析失败"}
	}
	approved := parsed.ApprovedV1
	if approved == nil {
		approved = parsed.Approved
	}
	if approved == nil {
		return nil, &RecoverableError{Kind: MissingField, Message: "审查结果缺少必填字段 __niuma_review_approved_v1__"}
	}
	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" {
		return nil, &RecoverableError{Kind: MissingField, Message: "审查结果缺少必填字段 summary"}
	}

	return &ReviewResult{
		Approved:      *approved,
		Summary:       summary,
		ResolvedItems: parsed.ResolvedItems,
		Issues:        parsed.Issues,
	}, nil
}

// extractJSON 从文本中提取 JSON，支持 ```json ... ``` 代码块和裸 JSON
func extractJSON(text string) string {
	// 优先尝试 ```json ``` 代码块（可能有多个，取最后一个——通常是结论 JSON）
	matches := jsonBlockRe.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		return strings.TrimSpace(matches[len(matches)-1][1])
	}

	// 尝试找裸 JSON 对象（从后往前，因为结论 JSON 通常在末尾）
	// 逐层尝试：从最后一个 } 开始，向前找匹配的 {
	end := strings.LastIndex(text, "}")
	if end < 0 {
		return ""
	}
	// 从 end 往前找每个 {，尝试解析
	for i := end; i >= 0; i-- {
		if text[i] == '{' {
			candidate := text[i : end+1]
			var js json.RawMessage
			if json.Unmarshal([]byte(candidate), &js) == nil {
				return candidate
			}
		}
	}

	return ""
}

// stripLastJSONSegment 从文本中剥离最后一个 JSON 片段，保留自然语言正文。
func stripLastJSONSegment(text, jsonStr string) string {
	matches := jsonBlockRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		return strings.TrimSpace(text[:last[0]] + text[last[1]:])
	}

	idx := strings.LastIndex(text, jsonStr)
	if idx >= 0 {
		return strings.TrimSpace(text[:idx] + text[idx+len(jsonStr):])
	}
	return strings.TrimSpace(text)
}

func parseJSONObject(raw string) (map[string]json.RawMessage, error) {
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &rawMap); err != nil {
		return nil, err
	}
	return rawMap, nil
}

func missingFields(rawMap map[string]json.RawMessage, required ...string) []string {
	var missing []string
	for _, f := range required {
		if _, ok := rawMap[f]; !ok {
			missing = append(missing, f)
		}
	}
	sort.Strings(missing)
	return missing
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

// FieldSpec 描述 JSON 响应中一个字段的规格，用于构造泛化的格式修复 prompt。
type FieldSpec struct {
	Name     string // 字段名
	Type     string // 字段类型："string", "bool", "array", "object"
	Required bool   // 是否必填
	Desc     string // 字段描述
}

// 各阶段的字段规格定义
var (
	debateFields = []FieldSpec{
		{Name: "should_finish", Type: "bool", Required: true, Desc: "是否认为讨论已经可以结束"},
	}
	planFinalFields = []FieldSpec{
		{Name: "title", Type: "string", Required: true, Desc: "方案标题"},
		{Name: "approach", Type: "string", Required: true, Desc: "实现方案"},
		{Name: "file_changes", Type: "array", Required: false, Desc: "文件变更列表"},
		{Name: "test_scenarios", Type: "array", Required: false, Desc: "测试场景"},
	}
	reviewFields = []FieldSpec{
		{Name: reviewApprovedSentinelKey, Type: "bool", Required: true, Desc: "是否通过审查（机器判定键，必须输出）"},
		{Name: "summary", Type: "string", Required: true, Desc: "审查总结"},
		{Name: "resolved_items", Type: "array", Required: false, Desc: "已解决的问题列表"},
		{Name: "issues", Type: "array", Required: false, Desc: "新发现的问题列表"},
	}
)

// BuildFormatRepairPromptFor 生成泛化的格式修复 prompt，根据 FieldSpec 描述目标 JSON 格式。
func BuildFormatRepairPromptFor(rawText string, fields []FieldSpec) string {
	var sb strings.Builder
	sb.WriteString("以下是你刚才生成的内容，但无法从中提取到有效的 JSON，导致解析失败。\n\n")
	sb.WriteString("请**仅**输出一个 JSON 代码块（使用 markdown ```json ... ``` 格式），包含以下字段：\n")
	for _, f := range fields {
		req := ""
		if f.Required {
			req = "（必填）"
		}
		sb.WriteString(fmt.Sprintf("- %s: %s类型，%s%s\n", f.Name, f.Type, f.Desc, req))
	}
	sb.WriteString("\n不要重复原始内容，不要添加任何其他文字，只输出 JSON 代码块。\n\n")
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("原始内容：\n%s", rawText))
	return sb.String()
}

// BuildFormatRepairPrompt 生成格式修复 prompt，要求 AI 仅输出缺失的 JSON 代码块。
// 用于 debate parse 失败后的第1级降级重试。保留向后兼容。
func BuildFormatRepairPrompt(rawText string) string {
	return BuildFormatRepairPromptFor(rawText, debateFields)
}

// RepairAndParseFinalPlan 从修复回复中提取 JSON，解析为 FinalPlan。
func RepairAndParseFinalPlan(originalRaw, repairRaw string) (*FinalPlan, error) {
	jsonStr := extractJSON(repairRaw)
	if jsonStr == "" {
		return nil, fmt.Errorf("修复回复中未找到 JSON 代码块")
	}
	var plan FinalPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("修复回复 JSON 解析失败: %w", err)
	}
	if plan.Title == "" {
		return nil, fmt.Errorf("修复回复缺少 title 字段")
	}
	if plan.Approach == "" {
		return nil, fmt.Errorf("修复回复缺少 approach 字段")
	}
	return &plan, nil
}

// RepairAndParseReview 从修复回复中提取 JSON，解析为 ReviewResult。
func RepairAndParseReview(originalRaw, repairRaw string) (*ReviewResult, error) {
	jsonStr := extractJSON(repairRaw)
	if jsonStr == "" {
		return nil, fmt.Errorf("修复回复中未找到 JSON 代码块")
	}
	type reviewJSON struct {
		ApprovedV1    *bool    `json:"__niuma_review_approved_v1__"`
		Approved      *bool    `json:"approved"`
		Summary       string   `json:"summary"`
		ResolvedItems []string `json:"resolved_items,omitempty"`
		Issues        []string `json:"issues,omitempty"`
	}
	var parsed reviewJSON
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("修复回复 JSON 解析失败: %w", err)
	}
	approved := parsed.ApprovedV1
	if approved == nil {
		approved = parsed.Approved
	}
	if approved == nil {
		return nil, fmt.Errorf("修复回复缺少 %s 字段", reviewApprovedSentinelKey)
	}
	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" {
		return nil, fmt.Errorf("修复回复缺少 summary 字段")
	}
	return &ReviewResult{
		Approved:      *approved,
		Summary:       summary,
		ResolvedItems: parsed.ResolvedItems,
		Issues:        parsed.Issues,
	}, nil
}

// RepairAndParseDebate 尝试用修复后的 JSON 拼接原始 body 构造 DebateComment。
// repairRaw 是 AI 对 BuildFormatRepairPrompt 的回复（应仅含 JSON 代码块）。
func RepairAndParseDebate(originalRaw, repairRaw string) (*DebateComment, error) {
	// 先尝试从修复回复中提取 JSON
	jsonStr := extractJSON(repairRaw)
	if jsonStr == "" {
		return nil, fmt.Errorf("修复回复中未找到 JSON 代码块")
	}

	type debateJSON struct {
		ShouldFinish *bool `json:"should_finish"`
	}
	var parsed debateJSON
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("修复回复 JSON 解析失败: %w", err)
	}
	if parsed.ShouldFinish == nil {
		return nil, fmt.Errorf("修复回复缺少 should_finish 字段")
	}

	// 用原始文本作为 body（去掉可能残留的 JSON 片段）
	body := strings.TrimSpace(originalRaw)

	return &DebateComment{
		Body:         body,
		ShouldFinish: *parsed.ShouldFinish,
	}, nil
}
