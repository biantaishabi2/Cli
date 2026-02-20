//go:build !ci

package agent

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDraftPrompt(t *testing.T) {
	input := &PromptInput{
		IssueTitle: "Fix login bug",
		IssueBody:  "Login fails when password contains special chars",
		Comments:   []string{"I can reproduce this", "Same issue here"},
	}

	result, err := BuildDraftPrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Fix login bug")
	assert.Contains(t, result, "special chars")
	assert.Contains(t, result, "I can reproduce this")
}

func TestBuildDebatePrompt(t *testing.T) {
	input := &PromptInput{
		IssueTitle:     "Debate cache strategy",
		IssueBody:      "Need choose cache ttl",
		Comments:       []string{"A: prefer 30m", "B: prefer 60m"},
		Round:          2,
		MaxRounds:      5,
		DiscussionRole: "B",
		Counterpart:    "A",
	}

	result, err := BuildDebatePrompt(input)
	require.NoError(t, err)
	assert.Contains(t, result, "第 2/5 轮")
	assert.Contains(t, result, "讨论方 B")
	assert.Contains(t, result, "\"should_finish\"")
}

func TestBuildFinalPlanPrompt(t *testing.T) {
	input := &PromptInput{
		IssueTitle: "Refactor auth",
		IssueBody:  "Move to JWT",
		Comments:   []string{"JWT with refresh tokens"},
	}

	result, err := BuildFinalPlanPrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Refactor auth")
	assert.Contains(t, result, "JSON")
}

func TestBuildImplementPrompt(t *testing.T) {
	input := &PromptInput{
		IssueTitle: "Add user API",
		FinalPlan:  "## 方案\n创建 /api/users 端点",
	}

	result, err := BuildImplementPrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "Add user API")
	assert.Contains(t, result, "/api/users")
}

func TestBuildIteratePrompt(t *testing.T) {
	input := &PromptInput{
		IssueTitle:    "Add user API",
		FinalPlan:     "## 方案\n创建端点",
		ReviewComment: "请添加输入验证",
		PRDiff:        "+func CreateUser() {}",
	}

	result, err := BuildIteratePrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "请添加输入验证")
	assert.Contains(t, result, "CreateUser")
}

func TestBuildReviewPrompt_CrossFunctionAndPlanChecks(t *testing.T) {
	input := &PromptInput{
		IssueTitle: "test issue",
		FinalPlan:  "some plan",
		PRDiff:     "some diff",
	}

	result, err := BuildReviewPrompt(input)
	require.NoError(t, err)

	// 新增检查维度存在
	assert.Contains(t, result, "4. **跨函数逻辑一致性**")
	assert.Contains(t, result, "5. **方案 ↔ 实现完整性**")

	// P1 分级标准包含新描述
	assert.Contains(t, result, "跨函数逻辑不一致、方案要求未实现")

	// 现有检查项未被破坏
	assert.Contains(t, result, "测试覆盖缺口")
	assert.Contains(t, result, "CLI 接口一致性")
	assert.Contains(t, result, "错误处理完整性")
}

func TestBuildPrompt_EmptyInput(t *testing.T) {
	input := &PromptInput{}
	result, err := BuildDraftPrompt(input)
	require.NoError(t, err)
	assert.NotEmpty(t, result) // 模板本身有固定文本
}

// ===== FilterCommentsForPrompt 单元测试 =====

func makeComment(body string, createdAt time.Time, userType string, login string) *github.IssueComment {
	return &github.IssueComment{
		Body:      github.Ptr(body),
		CreatedAt: &github.Timestamp{Time: createdAt},
		User: &github.User{
			Type:  github.Ptr(userType),
			Login: github.Ptr(login),
		},
	}
}

func TestFilterCommentsForPrompt_LargeInput(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var comments []*github.IssueComment

	// 117 条 BOT 评论
	for i := 0; i < 117; i++ {
		body := fmt.Sprintf("BOT 评论 %d", i)
		if i == 100 {
			body = "<!-- BOT:DISCUSSION_SUMMARY -->汇总内容"
		}
		comments = append(comments, makeComment(body, base.Add(time.Duration(i)*time.Minute), "Bot", "niuma[bot]"))
	}
	// 15 条人类评论
	for i := 0; i < 15; i++ {
		comments = append(comments, makeComment(
			fmt.Sprintf("人类评论 %d", i),
			base.Add(time.Duration(117+i)*time.Minute),
			"User", "wangbo",
		))
	}

	result := FilterCommentsForPrompt(comments, 10)
	// 最多 11 条：10 条人类 + 1 条 summary BOT
	assert.LessOrEqual(t, len(result), 11)
	// 包含最近 10 条人类评论（编号 5-14）
	for i := 5; i < 15; i++ {
		assert.Contains(t, result, fmt.Sprintf("人类评论 %d", i))
	}
	// 不包含早期人类评论
	for i := 0; i < 5; i++ {
		found := false
		for _, r := range result {
			if r == fmt.Sprintf("人类评论 %d", i) {
				found = true
			}
		}
		assert.False(t, found, "不应包含早期人类评论 %d", i)
	}
	// 包含 summary BOT
	hasSummary := false
	for _, r := range result {
		if r == "<!-- BOT:DISCUSSION_SUMMARY -->汇总内容" {
			hasSummary = true
		}
	}
	assert.True(t, hasSummary, "应包含 DISCUSSION_SUMMARY BOT 评论")
}

func TestFilterCommentsForPrompt_FewHumanComments(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var comments []*github.IssueComment

	// 20 条 BOT
	for i := 0; i < 20; i++ {
		comments = append(comments, makeComment(
			fmt.Sprintf("BOT %d", i), base.Add(time.Duration(i)*time.Minute), "Bot", "bot[bot]"))
	}
	// 5 条人类
	for i := 0; i < 5; i++ {
		comments = append(comments, makeComment(
			fmt.Sprintf("人类 %d", i), base.Add(time.Duration(20+i)*time.Minute), "User", "alice"))
	}

	result := FilterCommentsForPrompt(comments, 10)
	// 全部 5 条人类评论应保留
	humanCount := 0
	for _, r := range result {
		for i := 0; i < 5; i++ {
			if r == fmt.Sprintf("人类 %d", i) {
				humanCount++
			}
		}
	}
	assert.Equal(t, 5, humanCount, "不足 N 条时全部保留")
}

func TestFilterCommentsForPrompt_ZeroMaxHuman(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	comments := []*github.IssueComment{
		makeComment("BOT 1", base, "Bot", "bot[bot]"),
		makeComment("人类 1", base.Add(time.Minute), "User", "alice"),
		makeComment("BOT 2", base.Add(2*time.Minute), "Bot", "niuma[bot]"),
	}

	result := FilterCommentsForPrompt(comments, 0)
	assert.Equal(t, 3, len(result), "maxHuman=0 全量返回")
}

func TestFilterCommentsForPrompt_BotDetection(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	comments := []*github.IssueComment{
		// Type=Bot
		makeComment("TypeBot", base, "Bot", "somebot"),
		// login 含 [bot] 后缀
		makeComment("LoginBot", base.Add(time.Minute), "", "niuma[bot]"),
		// 普通用户
		makeComment("Human", base.Add(2*time.Minute), "User", "alice"),
	}

	result := FilterCommentsForPrompt(comments, 10)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, "Human", result[0])
}

func TestFilterCommentsForPrompt_NoSummaryMarker(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var comments []*github.IssueComment
	// 50 条 BOT 但无 DISCUSSION_SUMMARY
	for i := 0; i < 50; i++ {
		comments = append(comments, makeComment(
			fmt.Sprintf("BOT %d", i), base.Add(time.Duration(i)*time.Minute), "Bot", "bot[bot]"))
	}
	// 3 条人类
	for i := 0; i < 3; i++ {
		comments = append(comments, makeComment(
			fmt.Sprintf("人 %d", i), base.Add(time.Duration(50+i)*time.Minute), "User", "bob"))
	}

	result := FilterCommentsForPrompt(comments, 10)
	// 仅 3 条人类，无 BOT
	assert.Equal(t, 3, len(result))
}

func TestFilterCommentsForPrompt_SortOrder(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	// 乱序输入
	comments := []*github.IssueComment{
		makeComment("后", base.Add(2*time.Minute), "User", "alice"),
		makeComment("先", base, "User", "bob"),
		makeComment("中", base.Add(time.Minute), "User", "charlie"),
	}

	result := FilterCommentsForPrompt(comments, 10)
	require.Equal(t, 3, len(result))
	assert.Equal(t, "先", result[0])
	assert.Equal(t, "中", result[1])
	assert.Equal(t, "后", result[2])
}
