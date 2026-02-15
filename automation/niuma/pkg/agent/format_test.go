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
		Consensus:  "使用 JWT 替换 session",
		OpenItems:  []string{"Token 过期时间", "刷新机制"},
		Suggestion: "建议 TTL 设为 24 小时",
	}
	m := &marker.Marker{
		Type:     marker.TypeDiscussionSummary,
		Issue:    42,
		Revision: 2,
	}

	result := FormatDiscussionSummary(summary, m)
	assert.Contains(t, result, "<!-- BOT:DISCUSSION_SUMMARY")
	assert.Contains(t, result, "讨论汇总")
	assert.Contains(t, result, "使用 JWT")
	assert.Contains(t, result, "Token 过期时间")
	assert.Contains(t, result, "TTL 设为 24 小时")
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
