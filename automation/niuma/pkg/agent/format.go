// pkg/agent/format.go
// 输出格式化：将数据结构渲染为 Markdown + Marker 注释
package agent

import (
	"fmt"
	"strings"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
)

// FormatDraftPlan 格式化方案草案为 Markdown
func FormatDraftPlan(plan *DraftPlan, m *marker.Marker) string {
	var sb strings.Builder

	sb.WriteString(marker.Render(m))
	sb.WriteString("\n\n")
	sb.WriteString("## 📋 方案草案\n\n")
	sb.WriteString("### 概述\n\n")
	sb.WriteString(plan.Summary)
	sb.WriteString("\n\n")
	sb.WriteString("### 方案\n\n")
	sb.WriteString(plan.Approach)

	if len(plan.AffectedFiles) > 0 {
		sb.WriteString("\n\n### 涉及文件\n\n")
		for _, f := range plan.AffectedFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}

	if plan.Risks != "" {
		sb.WriteString("\n### 风险点\n\n")
		sb.WriteString(plan.Risks)
	}

	sb.WriteString("\n\n---\n*如有意见请评论，讨论收敛后将自动定稿。*")

	return sb.String()
}

// FormatDiscussionSummary 格式化讨论汇总为 Markdown
func FormatDiscussionSummary(summary *DiscussionSummary, m *marker.Marker) string {
	var sb strings.Builder

	sb.WriteString(marker.Render(m))
	sb.WriteString("\n\n")
	sb.WriteString("## 📊 讨论汇总\n\n")
	sb.WriteString("### 已达成共识\n\n")
	sb.WriteString(summary.Consensus)

	if len(summary.OpenItems) > 0 {
		sb.WriteString("\n\n### 待讨论项\n\n")
		for _, item := range summary.OpenItems {
			sb.WriteString(fmt.Sprintf("- %s\n", item))
		}
	}

	if summary.Suggestion != "" {
		sb.WriteString("\n### 建议\n\n")
		sb.WriteString(summary.Suggestion)
	}

	return sb.String()
}

// FormatFinalPlan 格式化最终方案为 Markdown
func FormatFinalPlan(plan *FinalPlan, m *marker.Marker) string {
	var sb strings.Builder

	sb.WriteString(marker.Render(m))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("## ✅ 最终方案：%s\n\n", plan.Title))
	sb.WriteString("### 实现方案\n\n")
	sb.WriteString(plan.Approach)

	if len(plan.FileChanges) > 0 {
		sb.WriteString("\n\n### 文件变更\n\n")
		sb.WriteString("| 路径 | 操作 | 描述 |\n|------|------|------|\n")
		for _, fc := range plan.FileChanges {
			sb.WriteString(fmt.Sprintf("| `%s` | %s | %s |\n", fc.Path, fc.Action, fc.Description))
		}
	}

	if len(plan.TestScenarios) > 0 {
		sb.WriteString("\n### 测试场景\n\n")
		for i, ts := range plan.TestScenarios {
			sb.WriteString(fmt.Sprintf("%d. **%s**: 输入 `%s` → 预期 `%s`\n", i+1, ts.Name, ts.Input, ts.Expected))
		}
	}

	sb.WriteString("\n\n---\n*方案已定稿，即将开始实现。*")

	return sb.String()
}

// FormatConvergeWarning 格式化收敛预警为 Markdown
func FormatConvergeWarning(m *marker.Marker) string {
	var sb strings.Builder

	sb.WriteString(marker.Render(m))
	sb.WriteString("\n\n")
	sb.WriteString("## ⚠️ 讨论收敛预警\n\n")
	sb.WriteString("讨论已进入静默期。如果没有新的评论，将在 **30 分钟后自动定稿**。\n\n")
	sb.WriteString("- 如需继续讨论，请添加评论\n")
	sb.WriteString("- 如需立即定稿，请评论 `/finalize`\n")
	sb.WriteString("- 如需暂缓定稿，请评论 `/hold`")

	return sb.String()
}
