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
// 要求严格 JSON，字段缺失/类型错误会直接报错。
func ParseDebateResponse(raw string) (*DebateComment, error) {
	if raw == "" {
		return nil, fmt.Errorf("空响应")
	}

	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return nil, fmt.Errorf("无法解析 AB 评论：缺少 JSON 代码块或裸 JSON 对象")
	}

	rawMap, err := parseJSONObject(jsonStr)
	if err != nil {
		return nil, fmt.Errorf("无法解析 AB 评论 JSON: %w", err)
	}

	required := []string{"agreements", "disagreements", "suggestion"}
	missing := missingFields(rawMap, required...)
	if len(missing) > 0 {
		return nil, fmt.Errorf("AB 评论缺少必填字段: %s", strings.Join(missing, ", "))
	}

	if isJSONNull(rawMap["agreements"]) {
		return nil, fmt.Errorf("字段 agreements 不能为 null")
	}
	var agreements []string
	if err := json.Unmarshal(rawMap["agreements"], &agreements); err != nil {
		return nil, fmt.Errorf("字段 agreements 类型错误: %w", err)
	}
	disagreements, err := parseDisagreements(rawMap["disagreements"])
	if err != nil {
		return nil, err
	}
	var suggestion string
	if err := json.Unmarshal(rawMap["suggestion"], &suggestion); err != nil {
		return nil, fmt.Errorf("字段 suggestion 类型错误: %w", err)
	}

	return &DebateComment{
		Agreements:    agreements,
		Disagreements: disagreements,
		Suggestion:    strings.TrimSpace(suggestion),
	}, nil
}

// ParseFinalPlanResponse 解析 AI 返回的最终方案
func ParseFinalPlanResponse(raw string) (*FinalPlan, error) {
	if raw == "" {
		return nil, fmt.Errorf("空响应")
	}

	jsonStr := extractJSON(raw)
	if jsonStr != "" {
		var plan FinalPlan
		if err := json.Unmarshal([]byte(jsonStr), &plan); err == nil {
			// 必填字段校验
			if plan.Title == "" {
				return nil, fmt.Errorf("最终方案缺少 title 字段")
			}
			if plan.Approach == "" {
				return nil, fmt.Errorf("最终方案缺少 approach 字段")
			}
			return &plan, nil
		}
	}

	// FinalPlan 不做 fallback（结构化要求更高）
	return nil, fmt.Errorf("无法解析最终方案，AI 返回非 JSON 格式")
}

// ParseReviewResponse 解析 AI 返回的审查结果
// 审查结果必须是结构化 JSON，禁止纯文本兜底推断。
func ParseReviewResponse(raw string) (*ReviewResult, error) {
	if raw == "" {
		return nil, fmt.Errorf("空响应")
	}

	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return nil, fmt.Errorf("无法解析审查结果：缺少 JSON 代码块或裸 JSON 对象")
	}

	type reviewJSON struct {
		Approved      *bool    `json:"approved"`
		Summary       string   `json:"summary"`
		ResolvedItems []string `json:"resolved_items,omitempty"`
		Issues        []string `json:"issues,omitempty"`
	}

	var parsed reviewJSON
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("无法解析审查结果 JSON: %w", err)
	}
	if parsed.Approved == nil {
		return nil, fmt.Errorf("审查结果缺少必填字段 approved")
	}
	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" {
		return nil, fmt.Errorf("审查结果缺少必填字段 summary")
	}

	return &ReviewResult{
		Approved:      *parsed.Approved,
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

func parseDisagreements(raw json.RawMessage) ([]DisagreementItem, error) {
	if isJSONNull(raw) {
		return nil, fmt.Errorf("字段 disagreements 不能为 null")
	}
	var disagreements []DisagreementItem
	if err := json.Unmarshal(raw, &disagreements); err != nil {
		return nil, fmt.Errorf("字段 disagreements 类型错误: %w", err)
	}
	for i, item := range disagreements {
		if strings.TrimSpace(item.Topic) == "" {
			return nil, fmt.Errorf("disagreements[%d].topic 不能为空", i)
		}
		if len(item.Options) == 0 {
			return nil, fmt.Errorf("disagreements[%d].options 不能为空", i)
		}
		if strings.TrimSpace(item.Recommendation) == "" {
			return nil, fmt.Errorf("disagreements[%d].recommendation 不能为空", i)
		}
		if !isValidRisk(item.Risk) {
			return nil, fmt.Errorf("disagreements[%d].risk 非法: %s", i, item.Risk)
		}
	}
	return disagreements, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

func isValidDecision(d Decision) bool {
	switch d {
	case DecisionAdoptA, DecisionAdoptB, DecisionMerge, DecisionDefer:
		return true
	default:
		return false
	}
}

func isValidRisk(r RiskLevel) bool {
	switch r {
	case RiskLow, RiskMedium, RiskHigh:
		return true
	default:
		return false
	}
}
