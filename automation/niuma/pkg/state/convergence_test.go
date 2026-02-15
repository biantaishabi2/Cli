package state

import (
	"testing"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/assert"
)

// 辅助函数：创建评论
func makeComment(body string, createdAt time.Time) *github.IssueComment {
	ts := &github.Timestamp{Time: createdAt}
	return &github.IssueComment{
		Body:      &body,
		CreatedAt: ts,
	}
}

// 辅助函数：创建固定时间的 checker
func fixedChecker(now time.Time) *ConvergenceChecker {
	return &ConvergenceChecker{
		Now:             func() time.Time { return now },
		SilenceWarn:     10 * time.Minute,
		SilenceFinalize: 30 * time.Minute,
		MaxRounds:       5,
	}
}

func TestConvergence_NotConverged_RecentComment(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	checker := fixedChecker(now)
	input := &ConvergenceInput{
		Comments: []*github.IssueComment{
			makeComment("有个问题想讨论", now.Add(-5*time.Minute)),
		},
	}
	assert.Equal(t, NotConverged, checker.Check(input))
}

func TestConvergence_ShouldWarn_SilentLong(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	checker := fixedChecker(now)
	input := &ConvergenceInput{
		Comments: []*github.IssueComment{
			makeComment("最后一条消息", now.Add(-15*time.Minute)),
		},
	}
	assert.Equal(t, ShouldWarn, checker.Check(input))
}

func TestConvergence_ShouldFinalize_AfterWarning(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	warnTime := now.Add(-35 * time.Minute)
	checker := fixedChecker(now)
	input := &ConvergenceInput{
		Comments: []*github.IssueComment{
			makeComment("一些讨论", now.Add(-60*time.Minute)),
		},
		ConvergeWarning: &marker.Marker{Type: marker.TypeConvergeWarning, Issue: 1, Revision: 1},
		WarningTime:     warnTime,
	}
	assert.Equal(t, ShouldFinalize, checker.Check(input))
}

func TestConvergence_NotConverged_WarningTooRecent(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	warnTime := now.Add(-10 * time.Minute) // 预警后只过了 10 分钟
	checker := fixedChecker(now)
	input := &ConvergenceInput{
		Comments: []*github.IssueComment{
			makeComment("讨论内容", now.Add(-30*time.Minute)),
		},
		ConvergeWarning: &marker.Marker{Type: marker.TypeConvergeWarning, Issue: 1, Revision: 1},
		WarningTime:     warnTime,
	}
	assert.Equal(t, NotConverged, checker.Check(input))
}

func TestConvergence_ShouldFinalize_RoundBased(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	checker := fixedChecker(now)
	input := &ConvergenceInput{
		Comments: []*github.IssueComment{
			makeComment("最近的评论", now.Add(-1*time.Minute)),
		},
		DiscussionSummary: &marker.Marker{
			Type:     marker.TypeDiscussionSummary,
			Issue:    1,
			Revision: 5, // ≥ 5 轮次
		},
	}
	assert.Equal(t, ShouldFinalize, checker.Check(input))
}

func TestConvergence_NotConverged_RoundInsufficient(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	checker := fixedChecker(now)
	input := &ConvergenceInput{
		Comments: []*github.IssueComment{
			makeComment("评论", now.Add(-1*time.Minute)),
		},
		DiscussionSummary: &marker.Marker{
			Type:     marker.TypeDiscussionSummary,
			Issue:    1,
			Revision: 3, // < 5 轮次
		},
	}
	assert.Equal(t, NotConverged, checker.Check(input))
}

func TestConvergence_FinalizeCommand(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	checker := fixedChecker(now)
	input := &ConvergenceInput{
		Comments: []*github.IssueComment{
			makeComment("讨论...", now.Add(-2*time.Minute)),
			makeComment("/finalize", now.Add(-1*time.Minute)),
		},
	}
	assert.Equal(t, ShouldFinalize, checker.Check(input))
}

func TestConvergence_HoldCommand(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	checker := fixedChecker(now)
	input := &ConvergenceInput{
		Comments: []*github.IssueComment{
			makeComment("讨论", now.Add(-60*time.Minute)),
			makeComment("/hold", now.Add(-1*time.Minute)),
		},
		DiscussionSummary: &marker.Marker{
			Type: marker.TypeDiscussionSummary, Issue: 1, Revision: 6,
		},
	}
	// /hold 优先于轮次收敛
	assert.Equal(t, NotConverged, checker.Check(input))
}

func TestConvergence_FinalizeOverridesHold(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	checker := fixedChecker(now)
	input := &ConvergenceInput{
		Comments: []*github.IssueComment{
			makeComment("/hold", now.Add(-10*time.Minute)),
			makeComment("/finalize", now.Add(-1*time.Minute)),
		},
	}
	// /finalize 优先级最高
	assert.Equal(t, ShouldFinalize, checker.Check(input))
}

func TestConvergence_NoComments(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	checker := fixedChecker(now)
	input := &ConvergenceInput{
		Comments: nil,
	}
	assert.Equal(t, NotConverged, checker.Check(input))
}

func TestConvergence_IgnoresBotComments(t *testing.T) {
	now := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	checker := fixedChecker(now)
	input := &ConvergenceInput{
		Comments: []*github.IssueComment{
			makeComment("人的评论", now.Add(-5*time.Minute)),
			// bot 评论不应该影响静默时间
			makeComment("<!-- BOT:PLAN_DRAFT issue=1 rev=1 -->\n方案草案", now.Add(-1*time.Minute)),
		},
	}
	// 最后一条非 bot 评论是 5 分钟前，不到 10 分钟
	assert.Equal(t, NotConverged, checker.Check(input))
}
