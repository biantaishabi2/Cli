//go:build !ci

package marker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_PlanDraft(t *testing.T) {
	m := Parse("<!-- BOT:PLAN_DRAFT issue=42 rev=3 -->")
	require.NotNil(t, m)
	assert.Equal(t, TypePlanDraft, m.Type)
	assert.Equal(t, 42, m.Issue)
	assert.Equal(t, 3, m.Revision)
	assert.Equal(t, 0, m.PR)
	assert.False(t, m.Finish)
}

func TestParse_PRCreated(t *testing.T) {
	m := Parse("<!-- BOT:PR_CREATED issue=10 rev=1 pr=55 -->")
	require.NotNil(t, m)
	assert.Equal(t, TypePRCreated, m.Type)
	assert.Equal(t, 10, m.Issue)
	assert.Equal(t, 1, m.Revision)
	assert.Equal(t, 55, m.PR)
}

func TestParse_ConvergeWarning(t *testing.T) {
	m := Parse("<!-- BOT:CONVERGE_WARNING issue=7 rev=1 -->")
	require.NotNil(t, m)
	assert.Equal(t, TypeConvergeWarning, m.Type)
	assert.Equal(t, 7, m.Issue)
}

func TestParse_NoMatch(t *testing.T) {
	assert.Nil(t, Parse("这不是一个 marker"))
	assert.Nil(t, Parse("<!-- 普通注释 -->"))
	assert.Nil(t, Parse(""))
}

func TestParse_InvalidType(t *testing.T) {
	m := Parse("<!-- BOT:UNKNOWN_TYPE issue=1 rev=1 -->")
	assert.Nil(t, m)
}

func TestParse_MinimalMarker(t *testing.T) {
	// 只有类型，没有 key=value
	m := Parse("<!-- BOT:PLAN_DRAFT -->")
	require.NotNil(t, m)
	assert.Equal(t, TypePlanDraft, m.Type)
	assert.Equal(t, 0, m.Issue)
	assert.Equal(t, 0, m.Revision)
}

func TestParseAll(t *testing.T) {
	text := `# 方案草案
<!-- BOT:PLAN_DRAFT issue=1 rev=1 -->
一些内容
<!-- BOT:DISCUSSION_SUMMARY issue=1 rev=3 -->
更多内容`

	markers := ParseAll(text)
	require.Len(t, markers, 2)
	assert.Equal(t, TypePlanDraft, markers[0].Type)
	assert.Equal(t, TypeDiscussionSummary, markers[1].Type)
	assert.Equal(t, 3, markers[1].Revision)
}

func TestParseAll_Empty(t *testing.T) {
	markers := ParseAll("")
	assert.Empty(t, markers)
}

func TestFind(t *testing.T) {
	markers := []*Marker{
		{Type: TypePlanDraft, Issue: 1, Revision: 1},
		{Type: TypePlanDraft, Issue: 1, Revision: 2},
		{Type: TypePlanDraft, Issue: 2, Revision: 1},
		{Type: TypeDiscussionSummary, Issue: 1, Revision: 1},
	}

	found := Find(markers, TypePlanDraft, 1)
	assert.Len(t, found, 2)

	found = Find(markers, TypePlanDraft, 2)
	assert.Len(t, found, 1)

	found = Find(markers, TypePRCreated, 1)
	assert.Empty(t, found)
}

func TestFindLatest(t *testing.T) {
	markers := []*Marker{
		{Type: TypePlanDraft, Issue: 1, Revision: 1},
		{Type: TypePlanDraft, Issue: 1, Revision: 3},
		{Type: TypePlanDraft, Issue: 1, Revision: 2},
	}

	latest := FindLatest(markers, TypePlanDraft, 1)
	require.NotNil(t, latest)
	assert.Equal(t, 3, latest.Revision)
}

func TestFindLatest_NoMatch(t *testing.T) {
	markers := []*Marker{
		{Type: TypePlanDraft, Issue: 1, Revision: 1},
	}
	assert.Nil(t, FindLatest(markers, TypePRCreated, 1))
	assert.Nil(t, FindLatest(markers, TypePlanDraft, 99))
}

func TestRender(t *testing.T) {
	m := &Marker{Type: TypePlanDraft, Issue: 42, Revision: 3}
	assert.Equal(t, "<!-- BOT:PLAN_DRAFT issue=42 rev=3 -->", Render(m))
}

func TestRender_WithPR(t *testing.T) {
	m := &Marker{Type: TypePRCreated, Issue: 10, Revision: 1, PR: 55}
	assert.Equal(t, "<!-- BOT:PR_CREATED issue=10 rev=1 pr=55 -->", Render(m))
}

func TestRoundTrip(t *testing.T) {
	// 渲染后再解析，应该得到一致结果
	original := &Marker{Type: TypePlanFinal, Issue: 7, Revision: 2}
	rendered := Render(original)
	parsed := Parse(rendered)

	require.NotNil(t, parsed)
	assert.Equal(t, original.Type, parsed.Type)
	assert.Equal(t, original.Issue, parsed.Issue)
	assert.Equal(t, original.Revision, parsed.Revision)
}

// === Finish 字段测试（修复 #77）===

func TestParse_DiscussionSummaryWithFinish(t *testing.T) {
	m := Parse("<!-- BOT:DISCUSSION_SUMMARY issue=1 rev=2 finish=1 -->")
	require.NotNil(t, m)
	assert.Equal(t, TypeDiscussionSummary, m.Type)
	assert.Equal(t, 1, m.Issue)
	assert.Equal(t, 2, m.Revision)
	assert.True(t, m.Finish)
}

func TestParse_DiscussionSummaryWithoutFinish(t *testing.T) {
	m := Parse("<!-- BOT:DISCUSSION_SUMMARY issue=1 rev=2 -->")
	require.NotNil(t, m)
	assert.Equal(t, TypeDiscussionSummary, m.Type)
	assert.False(t, m.Finish)
}

func TestParse_FinishTrue(t *testing.T) {
	// 测试 finish=1
	m := Parse("<!-- BOT:DISCUSSION_SUMMARY issue=1 rev=1 finish=1 -->")
	require.NotNil(t, m)
	assert.True(t, m.Finish)
}

func TestParse_FinishFalse(t *testing.T) {
	// 测试 finish=0 或没有 finish
	m := Parse("<!-- BOT:DISCUSSION_SUMMARY issue=1 rev=1 finish=0 -->")
	require.NotNil(t, m)
	assert.False(t, m.Finish)
}

func TestRender_WithFinish(t *testing.T) {
	m := &Marker{Type: TypeDiscussionSummary, Issue: 1, Revision: 2, Finish: true}
	rendered := Render(m)
	assert.Equal(t, "<!-- BOT:DISCUSSION_SUMMARY issue=1 rev=2 finish=1 -->", rendered)
}

func TestRender_WithoutFinish(t *testing.T) {
	m := &Marker{Type: TypeDiscussionSummary, Issue: 1, Revision: 2, Finish: false}
	rendered := Render(m)
	assert.Equal(t, "<!-- BOT:DISCUSSION_SUMMARY issue=1 rev=2 -->", rendered)
}

func TestRoundTrip_WithFinish(t *testing.T) {
	original := &Marker{Type: TypeDiscussionSummary, Issue: 7, Revision: 2, Finish: true}
	rendered := Render(original)
	parsed := Parse(rendered)

	require.NotNil(t, parsed)
	assert.Equal(t, original.Type, parsed.Type)
	assert.Equal(t, original.Issue, parsed.Issue)
	assert.Equal(t, original.Revision, parsed.Revision)
	assert.Equal(t, original.Finish, parsed.Finish)
}

func TestParse_DiscussionSummaryExtendedFields(t *testing.T) {
	m := Parse("<!-- BOT:DISCUSSION_SUMMARY issue=1 rev=3 finish=1 decision=merge human=1 risk=high dcount=2 mode=debate_ab -->")
	require.NotNil(t, m)
	assert.Equal(t, "merge", m.Decision)
	assert.True(t, m.Human)
	assert.Equal(t, "high", m.Risk)
	assert.Equal(t, 2, m.DisagreeCount)
	assert.Equal(t, "debate_ab", m.Mode)
}

func TestRender_DiscussionSummaryExtendedFields(t *testing.T) {
	m := &Marker{
		Type:          TypeDiscussionSummary,
		Issue:         1,
		Revision:      3,
		Finish:        true,
		Decision:      "merge",
		Human:         true,
		Risk:          "high",
		DisagreeCount: 2,
		Mode:          "debate_ab",
	}
	rendered := Render(m)
	assert.Contains(t, rendered, "decision=merge")
	assert.Contains(t, rendered, "human=1")
	assert.Contains(t, rendered, "risk=high")
	assert.Contains(t, rendered, "dcount=2")
	assert.Contains(t, rendered, "mode=debate_ab")
}
