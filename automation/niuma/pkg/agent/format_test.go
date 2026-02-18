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
		Conclusion:   "双方同意先保留兼容逻辑，默认值后续再拍板。",
		ShouldFinish: false,
	}
	m := &marker.Marker{
		Type:     marker.TypeDiscussionSummary,
		Issue:    42,
		Revision: 2,
	}

	result := FormatDiscussionSummary(summary, m)
	assert.Contains(t, result, "<!-- BOT:DISCUSSION_SUMMARY")
	assert.Contains(t, result, "讨论汇总")
	assert.Contains(t, result, "双方同意先保留兼容逻辑")
	assert.Contains(t, result, "should_finish: `false`")
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
		Conclusion:   "仍需确认默认值策略",
		ShouldFinish: false,
	}

	prevFinish := true
	result := FormatDiscussionRoundSummary(2, 5, "debate_ab", summary, &prevFinish)
	assert.Contains(t, result, "<!-- BOT:DISCUSSION_VISIBLE -->")
	assert.Contains(t, result, "第 2/5 轮")
	assert.Contains(t, result, "仍需确认默认值策略")
	assert.Contains(t, result, "should_finish_change: `changed`")
}

func TestFormatDebateRoundComment(t *testing.T) {
	comment := &DebateComment{
		Body:         "我同意先补上输入校验，但默认值建议先保持兼容。",
		ShouldFinish: false,
	}
	result := FormatDebateRoundComment(1, 3, "A", comment)
	assert.Contains(t, result, "<!-- BOT:DEBATE_VISIBLE -->")
	assert.Contains(t, result, "Debate A")
	assert.Contains(t, result, "我同意先补上输入校验")
	assert.Contains(t, result, "should_finish: `false`")
}

func TestFormatDiscussionRoundLimitWarning(t *testing.T) {
	m := &marker.Marker{
		Type:     marker.TypeDiscussionRoundLimitNotice,
		Issue:    42,
		Revision: 1,
	}
	result := FormatDiscussionRoundLimitWarning(m, 5, "仍有 2 个未决分歧，且存在 high 风险", []string{
		"请维护者拍板分歧 A/B 取舍",
		"请先确认高风险项的缓解与回滚方案",
	})
	assert.Contains(t, result, "<!-- BOT:DISCUSSION_ROUND_LIMIT_NOTICE")
	assert.Contains(t, result, "未收敛原因")
	assert.Contains(t, result, "仍有 2 个未决分歧")
	assert.Contains(t, result, "下一步人工决策项")
	assert.Contains(t, result, "拍板分歧 A/B 取舍")
}
