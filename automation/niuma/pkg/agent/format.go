// pkg/agent/format.go
// 输出格式化：将数据结构渲染为 Markdown + Marker 注释
package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
)

const (
	visibleDiscussionRoundMarker = "<!-- BOT:DISCUSSION_VISIBLE -->"
	visibleDebateRoundMarker     = "<!-- BOT:DEBATE_VISIBLE -->"
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
	sb.WriteString("### 已达成一致\n\n")
	if len(summary.Agreements) == 0 {
		sb.WriteString("- （暂无）\n")
	} else {
		for _, item := range summary.Agreements {
			sb.WriteString(fmt.Sprintf("- %s\n", item))
		}
	}

	sb.WriteString("\n\n### 分歧清单\n\n")
	if len(summary.Disagreements) == 0 {
		sb.WriteString("- （无）\n")
	} else {
		for i, item := range summary.Disagreements {
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, item.Topic))
			sb.WriteString(fmt.Sprintf("   - 选项: %s\n", strings.Join(item.Options, " / ")))
			sb.WriteString(fmt.Sprintf("   - 建议: %s\n", item.Recommendation))
			sb.WriteString(fmt.Sprintf("   - 风险: `%s`\n", item.Risk))
		}
	}

	sb.WriteString("\n### 当前决策\n\n")
	sb.WriteString(fmt.Sprintf("- decision: `%s`\n", summary.Decision))
	sb.WriteString(fmt.Sprintf("- requires_human_decision: `%t`\n", summary.RequiresHumanDecision))
	sb.WriteString(fmt.Sprintf("- should_finish: `%t`\n", summary.ShouldFinish))

	// 机器可读快照：供后续回放和自动化解析。
	if raw, err := json.Marshal(summary); err == nil {
		sb.WriteString(fmt.Sprintf("\n<!-- DISCUSSION_SNAPSHOT:%s -->", string(raw)))
	}

	return sb.String()
}

// FormatDiscussionRoundSummary 格式化每轮可见摘要（用于 issue comment）。
func FormatDiscussionRoundSummary(round, maxRounds int, mode string, summary *DiscussionSummary, previousCount int) string {
	var sb strings.Builder
	sb.WriteString(visibleDiscussionRoundMarker)
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("## 🧭 讨论进展（第 %d/%d 轮）\n\n", round, maxRounds))
	sb.WriteString(fmt.Sprintf("- mode: `%s`\n", mode))
	sb.WriteString(fmt.Sprintf("- 分歧数量: `%d`", len(summary.Disagreements)))
	if previousCount >= 0 {
		diff := len(summary.Disagreements) - previousCount
		sb.WriteString(fmt.Sprintf("（变化: %+d）\n", diff))
	} else {
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("- decision: `%s`\n", summary.Decision))
	sb.WriteString(fmt.Sprintf("- 最高风险: `%s`\n", maxRisk(summary.Disagreements)))
	sb.WriteString(fmt.Sprintf("- should_finish: `%t`\n", summary.ShouldFinish))
	sb.WriteString(fmt.Sprintf("- requires_human_decision: `%t`\n", summary.RequiresHumanDecision))
	return sb.String()
}

// FormatDebateRoundComment 格式化 AB 轮流评论可见内容。
func FormatDebateRoundComment(round, maxRounds int, speaker string, comment *DebateComment) string {
	var sb strings.Builder
	sb.WriteString(visibleDebateRoundMarker)
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("## 🗣️ Debate %s（第 %d/%d 轮）\n\n", speaker, round, maxRounds))
	sb.WriteString("### 同意点\n")
	if len(comment.Agreements) == 0 {
		sb.WriteString("- （暂无）\n")
	} else {
		for _, item := range comment.Agreements {
			sb.WriteString(fmt.Sprintf("- %s\n", item))
		}
	}

	sb.WriteString("\n### 分歧点\n")
	if len(comment.Disagreements) == 0 {
		sb.WriteString("- （无）\n")
	} else {
		for _, item := range comment.Disagreements {
			sb.WriteString(fmt.Sprintf("- %s（risk=%s）\n", item.Topic, item.Risk))
		}
	}

	sb.WriteString("\n### 建议\n")
	if strings.TrimSpace(comment.Suggestion) == "" {
		sb.WriteString("- （无）\n")
	} else {
		sb.WriteString(comment.Suggestion)
		sb.WriteString("\n")
	}
	return sb.String()
}

func maxRisk(items []DisagreementItem) RiskLevel {
	max := RiskLow
	for _, item := range items {
		switch item.Risk {
		case RiskHigh:
			return RiskHigh
		case RiskMedium:
			if max != RiskHigh {
				max = RiskMedium
			}
		}
	}
	return max
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

	// 嵌入 FileChanges JSON 到 HTML comment，供 implement 阶段 diff 对比
	if len(plan.FileChanges) > 0 {
		jsonBytes, _ := json.Marshal(plan.FileChanges)
		sb.WriteString(fmt.Sprintf("\n<!-- PLAN_FILES:%s -->", string(jsonBytes)))
	}

	return sb.String()
}

// FormatReviewResult 格式化审查结果为 Markdown
func FormatReviewResult(result *ReviewResult) string {
	var sb strings.Builder

	if result.Approved {
		sb.WriteString("## ✅ AI 自审通过\n\n")
	} else {
		sb.WriteString("## ❌ AI 自审未通过\n\n")
	}

	sb.WriteString("### 审查总结\n\n")
	sb.WriteString(result.Summary)

	if len(result.ResolvedItems) > 0 {
		sb.WriteString("\n\n### 讨论共识\n\n")
		for i, item := range result.ResolvedItems {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
		}
	}

	if len(result.Issues) > 0 {
		sb.WriteString("\n\n### 发现的问题\n\n")
		for i, issue := range result.Issues {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, issue))
		}
	}

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

// FormatDiscussionRoundLimitWarning 格式化 discuss 达到轮次上限提醒
func FormatDiscussionRoundLimitWarning(m *marker.Marker, maxRounds int, unresolvedReason string, nextActions []string) string {
	var sb strings.Builder

	sb.WriteString(marker.Render(m))
	sb.WriteString("\n\n")
	sb.WriteString("## ⚠️ 讨论已达自动轮次上限\n\n")
	sb.WriteString(fmt.Sprintf("当前自动讨论已达到 **%d** 轮，仍未收敛，已停止本次自动推进。\n\n", maxRounds))
	sb.WriteString("### 未收敛原因\n\n")
	if strings.TrimSpace(unresolvedReason) == "" {
		sb.WriteString("- 达到轮次上限后，仍存在待决分歧\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("- %s\n\n", unresolvedReason))
	}
	sb.WriteString("### 下一步人工决策项\n\n")
	if len(nextActions) == 0 {
		sb.WriteString("- 在分歧清单中逐项拍板（接受 A/B 或给出新方案）\n")
		sb.WriteString("- 如涉及高风险项，请先明确风险缓解条件和回滚方案\n")
		sb.WriteString("- 补充结论后可评论 `/finalize` 触发定稿\n\n")
	} else {
		for _, action := range nextActions {
			item := strings.TrimSpace(action)
			if item == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("- %s\n", item))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("- 当前状态保持为 `bot:needs-discussion`\n")
	sb.WriteString("- 请补充更多上下文、边界条件或明确人工决策\n")
	sb.WriteString("- 新增一条非 BOT 评论后会触发新一轮 discuss")

	return sb.String()
}
