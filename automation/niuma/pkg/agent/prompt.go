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
  "test_strategy": {
    "unit_tests": [
      {"name": "单元测试名", "scenario": "测试场景", "assertions": ["断言1", "断言2"]}
    ],
    "integration_tests": [
      {"name": "集成测试名", "setup": "前置条件", "verification": "验证点"}
    ],
    "bdd_tests": [
      {"name": "BDD 场景名", "given": "前置条件", "when": "操作", "then": "预期结果"}
    ],
    "ci_inclusion": {
      "unit": "是否纳入 CI（是/否，原因）",
      "integration": "是否纳入 CI（是/否，原因）",
      "bdd": "是否纳入 CI（是/否，原因）"
    }
  },
  "test_scenarios": [
    {"name": "测试场景名", "input": "输入", "expected": "预期输出"}
  ],
  "cli_interface": {
    "commands": [
      {"name": "命令名", "args": ["--flag", "value"], "description": "功能描述"}
    ],
    "consistency": "CLI 参数风格是否统一（是/否，说明）"
  },
  "error_handling": {
    "error_types": [
      {"name": "错误类型", "exit_code": 1, "description": "触发场景"}
    ],
    "recovery_strategy": "恢复策略"
  }
}
` + "```" + `

注意：
1. test_strategy 必须明确单元测试、集成测试、BDD 测试的分层
2. ci_inclusion 必须说明每种测试是否纳入 CI，以及原因
3. cli_interface 必须检查参数风格一致性
4. error_handling 必须定义错误码和恢复策略`

const implementTmpl = `你是一个高级软件工程师。请根据以下方案实现代码。

## Issue: {{.IssueTitle}}

## 最终方案

{{.FinalPlan}}

## 要求

1. 严格按照方案实现
2. 包含所有文件变更
3. 确保代码可编译和通过测试
4. 输出每个文件的完整变更内容`

const reviewTmpl = `你是一个务实的代码审查员。请审查以下 PR 的代码变更。

## Issue: {{.IssueTitle}}

## 最终方案

{{.FinalPlan}}

## PR Diff

{{.PRDiff}}

{{- if .ReviewComment}}

## 之前的 Review 讨论

{{.ReviewComment}}
{{- end}}

## 审查要求

### 必须检查（严格）

1. **测试覆盖缺口**
   - 并发安全测试（如多线程访问同一资源）
   - 边界条件测试（空值、极值、循环边界）
   - 深度/递归限制测试（搜索深度、继承链深度）

2. **CLI 接口一致性**
   - 参数风格是否统一（全用 --flag 或混合 positional）
   - 命名是否一致（--id vs --from vs --input）

3. **错误处理完整性**
   - 错误码是否定义
   - 恢复策略是否说明
   - 边界错误是否处理

### 不报告（避免噪音）

- 设计选择和代码风格偏好（除非明显违反 Go 惯例）
- 理论上的竞态条件（GitHub Actions concurrency group 已在 workflow 层面防护）
- 你不确定的推测（如"可能会..."、"也许..."）

## 分级标准

- P0：运行时必然崩溃（nil panic、死循环、数据丢失）
- P1：特定条件下的 bug（边界情况、错误处理遗漏）
- P2：可以改进但不影响正确性

## 验证要求

报告问题前请自行验证：
- 引用具体的代码行号和逻辑
- 确认问题确实存在（不要猜测 build tag 或 API 行为）
- 如果涉及外部系统行为（如 GitHub API），标明"需确认"

**approved 判定规则（必须严格遵守）：**
**- 存在任何 P0 或 P1 问题 → 必须设 approved=false**
**- 只有 P2 问题或无问题 → 设 approved=true**

{{- if .ReviewComment}}

## 讨论回应要求

PR 历史中包含之前的 review 和回复。你必须：
1. 逐条检查之前提出的每个问题，确认是否已修复
2. 对于 implementer 反驳的问题（认为不是 bug），明确表态你是否接受其解释
3. **resolved_items 是必填字段，不能为空数组**——必须逐条列出每个历史问题的结论
4. 当 resolved_items 中有"仍需修改"的条目 → 也必须设 approved=false
{{- end}}

**最终输出格式（必须严格遵守）：**

在你的回复最末尾，必须输出一个 ` + "```json```" + ` 代码块，不要在 JSON 后面再写任何文字：
` + "```json" + `
{
  "approved": true或false,
  "summary": "一句话审查总结",
  "resolved_items": ["之前的问题X：已修复/接受解释/仍需修改"],
  "issues": ["P0/P1/P2 - 具体问题描述（仅新发现的问题）"]
}
` + "```" + `

注意：JSON 代码块必须是你回复的最后一部分。`

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
