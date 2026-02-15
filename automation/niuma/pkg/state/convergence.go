// pkg/state/convergence.go
// 讨论收敛检查：轮次收敛、静默收敛、命令收敛
package state

import (
	"context"
	"strings"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	ghapi "github.com/google/go-github/v68/github"
)

// ConvergeResult 收敛检查结果
type ConvergeResult int

const (
	NotConverged   ConvergeResult = iota // 继续讨论
	ShouldWarn                           // 发预警评论
	ShouldFinalize                       // 可以定稿
)

func (r ConvergeResult) String() string {
	switch r {
	case NotConverged:
		return "not_converged"
	case ShouldWarn:
		return "should_warn"
	case ShouldFinalize:
		return "should_finalize"
	default:
		return "unknown"
	}
}

// ConvergenceChecker 纯函数收敛检查器（可注入时间，测试不依赖真实时间）
type ConvergenceChecker struct {
	Now             func() time.Time // 可注入的时间源
	SilenceWarn     time.Duration    // 静默多久发预警，默认 10 分钟
	SilenceFinalize time.Duration    // 预警后静默多久定稿，默认 30 分钟
	MaxRounds       int              // 轮次收敛阈值，默认 5
}

// DefaultChecker 返回默认配置的收敛检查器
func DefaultChecker() *ConvergenceChecker {
	return &ConvergenceChecker{
		Now:             time.Now,
		SilenceWarn:     10 * time.Minute,
		SilenceFinalize: 30 * time.Minute,
		MaxRounds:       5,
	}
}

// ConvergenceInput 收敛检查的输入数据
type ConvergenceInput struct {
	Comments          []*ghapi.IssueComment
	DiscussionSummary *marker.Marker // 可为 nil
	ConvergeWarning   *marker.Marker // 可为 nil
	WarningTime       time.Time      // 预警评论的创建时间
}

// Check 纯函数：根据输入数据检查收敛条件
func (c *ConvergenceChecker) Check(input *ConvergenceInput) ConvergeResult {
	now := c.Now()

	// 0. 检查最后一条命令（只看最后发出的 /finalize 或 /hold）
	if lastCmd := findLastCommand(input.Comments); lastCmd != "" {
		switch lastCmd {
		case "/finalize":
			return ShouldFinalize
		case "/hold":
			return NotConverged
		}
	}

	// 2. 轮次收敛：讨论汇总更新 ≥ MaxRounds 次
	if input.DiscussionSummary != nil && input.DiscussionSummary.Revision >= c.MaxRounds {
		return ShouldFinalize
	}

	// 3. 静默收敛
	lastCommentTime := getLastHumanCommentTime(input.Comments)
	if lastCommentTime.IsZero() {
		return NotConverged
	}

	// 已发过预警？
	if input.ConvergeWarning != nil {
		// 预警后静默 ≥ SilenceFinalize → 定稿
		if now.Sub(input.WarningTime) >= c.SilenceFinalize {
			return ShouldFinalize
		}
		return NotConverged
	}

	// 未预警，静默 ≥ SilenceWarn → 发预警
	if now.Sub(lastCommentTime) >= c.SilenceWarn {
		return ShouldWarn
	}

	return NotConverged
}

// CheckConvergence Machine 上的便捷方法，从 GitHub 读取数据后检查收敛
func (m *Machine) CheckConvergence(ctx context.Context, checker *ConvergenceChecker) (ConvergeResult, error) {
	comments, err := m.client.ListComments(ctx, m.issueNumber)
	if err != nil {
		return NotConverged, err
	}

	// 查找 discussion summary marker
	summaryMC, err := m.client.FindMarker(ctx, m.issueNumber, marker.TypeDiscussionSummary)
	if err != nil {
		return NotConverged, err
	}

	// 查找 converge warning marker
	warningMC, err := m.client.FindMarker(ctx, m.issueNumber, marker.TypeConvergeWarning)
	if err != nil {
		return NotConverged, err
	}

	input := &ConvergenceInput{
		Comments: comments,
	}
	if summaryMC != nil {
		input.DiscussionSummary = summaryMC.Marker
	}
	if warningMC != nil {
		input.ConvergeWarning = warningMC.Marker
		input.WarningTime = warningMC.Comment.GetCreatedAt().Time
	}

	return checker.Check(input), nil
}

// findLastCommand 从评论中倒序查找最后一条 /finalize 或 /hold 命令
// 只看最后一条命令，忽略之前的命令
func findLastCommand(comments []*ghapi.IssueComment) string {
	for i := len(comments) - 1; i >= 0; i-- {
		body := strings.TrimSpace(comments[i].GetBody())
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "/finalize" || trimmed == "/hold" {
				return trimmed
			}
		}
	}
	return ""
}

// getLastHumanCommentTime 获取最后一条非 bot 评论的时间
func getLastHumanCommentTime(comments []*ghapi.IssueComment) time.Time {
	var latest time.Time
	for _, c := range comments {
		body := c.GetBody()
		// 跳过包含 BOT: marker 的评论（机器人评论）
		if strings.Contains(body, "<!-- BOT:") {
			continue
		}
		t := c.GetCreatedAt().Time
		if t.After(latest) {
			latest = t
		}
	}
	return latest
}
