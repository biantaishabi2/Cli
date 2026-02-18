//go:build !ci

package agent

import (
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/stretchr/testify/assert"
)

func TestFormatDraftPlan(t *testing.T) {
	plan := &DraftPlan{
		Summary:       "修复登录问题",
		Approach:      "添加特殊字符编码处理",
		AffectedFiles: []string{"auth.go", "utils.go"},
		Risks:         "可能影响已有用户",
	}
	m := &marker.Marker{
		Type:     marker.TypePlanDraft,
		Issue:    42,
		Revision: 1,
	}

	result := FormatDraftPlan(plan, m)
	assert.Contains(t, result, "<!-- BOT:PLAN_DRAFT")
	assert.Contains(t, result, "方案草案")
	assert.Contains(t, result, "修复登录问题")
	assert.Contains(t, result, "auth.go")
	assert.Contains(t, result, "可能影响已有用户")
}

func TestFormatDiscussionSummary(t *testing.T) {
	summary := &DiscussionSummary{
		Agreements: []string{"使用 JWT 替换 session"},
		Disagreements: []DisagreementItem{
			{
				Topic:          "Token 过期时间",
				Options:        []string{"15m", "30m"},
				Recommendation: "默认 15m",
				Risk:           RiskLow,
			},
		},
		Decision:              DecisionMerge,
		RequiresHumanDecision: false,
		ShouldFinish:          false,
	}
	m := &marker.Marker{
		Type:     marker.TypeDiscussionSummary,
		Issue:    42,
		Revision: 2,
	}

	result := FormatDiscussionSummary(summary, m)
	assert.Contains(t, result, "<!-- BOT:DISCUSSION_SUMMARY")
	assert.Contains(t, result, "讨论汇总")
	assert.Contains(t, result, "使用 JWT 替换 session")
	assert.Contains(t, result, "Token 过期时间")
	assert.Contains(t, result, "decision: `merge`")
	assert.Contains(t, result, "DISCUSSION_SNAPSHOT")
}

func TestFormatFinalPlan(t *testing.T) {
	plan := &FinalPlan{
		Title:    "JWT 认证迁移",
		Approach: "分三步迁移到 JWT",
		FileChanges: []FileChange{
			{Path: "auth.go", Action: "modify", Description: "添加JWT验证"},
		},
		TestScenarios: []TestScenario{
			{Name: "有效token", Input: "Bearer xxx", Expected: "200 OK"},
		},
	}
	m := &marker.Marker{
		Type:     marker.TypePlanFinal,
		Issue:    42,
		Revision: 1,
	}

	result := FormatFinalPlan(plan, m)
	assert.Contains(t, result, "<!-- BOT:PLAN_FINAL")
	assert.Contains(t, result, "JWT 认证迁移")
	assert.Contains(t, result, "auth.go")
	assert.Contains(t, result, "有效token")
	assert.Contains(t, result, "方案已定稿")
}

func TestFormatConvergeWarning(t *testing.T) {
	m := &marker.Marker{
		Type:     marker.TypeConvergeWarning,
		Issue:    42,
		Revision: 1,
	}

	result := FormatConvergeWarning(m)
	assert.Contains(t, result, "<!-- BOT:CONVERGE_WARNING")
	assert.Contains(t, result, "收敛预警")
	assert.Contains(t, result, "30 分钟")
	assert.Contains(t, result, "/finalize")
	assert.Contains(t, result, "/hold")
}

func TestFormatDiscussionRoundSummary(t *testing.T) {
	summary := &DiscussionSummary{
		Agreements: []string{"a"},
		Disagreements: []DisagreementItem{
			{Topic: "t1", Options: []string{"A", "B"}, Recommendation: "A", Risk: RiskMedium},
		},
		Decision:              DecisionMerge,
		RequiresHumanDecision: false,
		ShouldFinish:          false,
	}

	result := FormatDiscussionRoundSummary(2, 5, "consolidate", summary, 2)
	assert.Contains(t, result, "<!-- BOT:DISCUSSION_VISIBLE -->")
	assert.Contains(t, result, "第 2/5 轮")
	assert.Contains(t, result, "变化: -1")
	assert.Contains(t, result, "最高风险: `medium`")
}

func TestFormatDebateRoundComment(t *testing.T) {
	comment := &DebateComment{
		Agreements: []string{"同意输入校验"},
		Disagreements: []DisagreementItem{
			{Topic: "默认值", Options: []string{"A", "B"}, Recommendation: "A", Risk: RiskHigh},
		},
		Suggestion: "先保持兼容默认值",
	}
	result := FormatDebateRoundComment(1, 3, "A", comment)
	assert.Contains(t, result, "<!-- BOT:DEBATE_VISIBLE -->")
	assert.Contains(t, result, "Debate A")
	assert.Contains(t, result, "同意输入校验")
	assert.Contains(t, result, "risk=high")
	assert.Contains(t, result, "先保持兼容默认值")
}
