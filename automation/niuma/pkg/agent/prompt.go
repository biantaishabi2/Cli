// pkg/agent/prompt.go
// Prompt 模板构建函数
package agent

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// PromptInput 构建 prompt 的输入数据
type PromptInput struct {
	IssueTitle    string
	IssueBody     string
	Comments      []string // 评论内容列表
	FinalPlan     string   // 最终方案文本（用于实现/迭代）
	ReviewComment string   // PR review 意见（用于迭代）
	PRDiff        string   // PR diff 内容（用于迭代）
}

// BuildDraftPrompt 生成方案草案的 prompt
func BuildDraftPrompt(input *PromptInput) (string, error) {
	return renderTemplate("draft", draftTmpl, input)
}

// BuildConsolidatePrompt 生成讨论汇总的 prompt
func BuildConsolidatePrompt(input *PromptInput) (string, error) {
	return renderTemplate("consolidate", consolidateTmpl, input)
}

// BuildFinalPlanPrompt 生成最终方案的 prompt
func BuildFinalPlanPrompt(input *PromptInput) (string, error) {
	return renderTemplate("final", finalPlanTmpl, input)
}

// BuildImplementPrompt 生成代码实现的 prompt
func BuildImplementPrompt(input *PromptInput) (string, error) {
	return renderTemplate("implement", implementTmpl, input)
}

// BuildIteratePrompt 生成迭代修复的 prompt
func BuildIteratePrompt(input *PromptInput) (string, error) {
	return renderTemplate("iterate", iterateTmpl, input)
}

// BuildReviewPrompt 生成自审 prompt
func BuildReviewPrompt(input *PromptInput) (string, error) {
	return renderTemplate("review", reviewTmpl, input)
}

func renderTemplate(name, tmplStr string, input *PromptInput) (string, error) {
	tmpl, err := template.New(name).Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("解析模板 %s 失败: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, input); err != nil {
		return "", fmt.Errorf("渲染模板 %s 失败: %w", name, err)
	}
	return buf.String(), nil
}

const draftTmpl = `你是一个高级软件工程师。请分析以下 GitHub Issue 并生成一个方案草案。

## Issue: {{.IssueTitle}}

{{.IssueBody}}

{{- if .Comments}}

## 已有讨论

{{range .Comments}}
---
{{.}}
{{end}}
{{- end}}

## 要求

请以 JSON 格式返回方案草案：
` + "```json" + `
{
  "summary": "问题概述",
  "approach": "解决方案描述",
  "affected_files": ["file1.go", "file2.go"],
  "risks": "潜在风险（可选）"
}
` + "```"

const consolidateTmpl = `你是一个项目经理。请汇总以下 GitHub Issue 的讨论进展。

## Issue: {{.IssueTitle}}

{{.IssueBody}}

## 讨论内容

{{range .Comments}}
---
{{.}}
{{end}}

## 要求

请以 JSON 格式返回讨论汇总：
` + "```json" + `
{
  "consensus": "已达成共识的内容",
  "open_items": ["待讨论项1", "待讨论项2"],
  "suggestion": "你的建议",
  "should_finish": false
}
` + "```"

const finalPlanTmpl = `你是一个高级软件架构师。请根据以下讨论生成最终实现方案。

## Issue: {{.IssueTitle}}

{{.IssueBody}}

## 讨论内容

{{range .Comments}}
---
{{.}}
{{end}}

## 要求

请以 JSON 格式返回最终方案：
` + "```json" + `
{
  "title": "方案标题",
  "approach": "详细实现方案",
  "file_changes": [
    {"path": "path/to/file.go", "action": "modify", "description": "变更描述"}
  ],
  "test_scenarios": [
    {"name": "测试场景名", "input": "输入", "expected": "预期输出"}
  ]
}
` + "```"

const implementTmpl = `你是一个高级软件工程师。请根据以下方案实现代码。

## Issue: {{.IssueTitle}}

## 最终方案

{{.FinalPlan}}

## 要求

1. 严格按照方案实现
2. 包含所有文件变更
3. 确保代码可编译和通过测试
4. 输出每个文件的完整变更内容`

const reviewTmpl = `你是一个严格的代码审查员。请审查以下 PR 的代码变更。

## Issue: {{.IssueTitle}}

## 最终方案

{{.FinalPlan}}

## PR Diff

{{.PRDiff}}

## 审查要求

请检查：
1. 代码是否符合最终方案的要求
2. 是否有明显的 bug 或逻辑错误
3. 是否缺少必要的错误处理
4. 代码风格是否合理

请以 JSON 格式返回审查结果：
` + "```json" + `
{
  "approved": true或false,
  "summary": "审查总结",
  "issues": ["问题1", "问题2"]
}
` + "```"

const iterateTmpl = `你是一个高级软件工程师。请根据 review 意见修改代码。

## Issue: {{.IssueTitle}}

## 最终方案

{{.FinalPlan}}

## Review 意见

{{.ReviewComment}}

{{- if .PRDiff}}

## 当前 PR Diff

{{.PRDiff}}
{{- end}}

## 要求

1. 只修改 review 指出的问题
2. 不要改动无关代码
3. 确保修改后代码可编译和通过测试`
