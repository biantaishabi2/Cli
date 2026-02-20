// cmd/niuma/label_guard.go
// label-guard 子命令：校验 bot: 标签操作权限，非授权账号警告或回滚
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/config"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	ghapi "github.com/google/go-github/v68/github"
	"github.com/spf13/cobra"
)

var labelGuardCmd = &cobra.Command{
	Use:   "label-guard",
	Short: "校验 bot: 标签操作权限（allowlist + dry-run/enforce）",
	RunE:  runLabelGuard,
}

var (
	flagLabelGuardActor  string
	flagLabelGuardAction string
	flagLabelGuardLabel  string
)

func init() {
	labelGuardCmd.Flags().StringVar(&flagLabelGuardActor, "actor", "", "触发操作的 GitHub 用户")
	labelGuardCmd.Flags().StringVar(&flagLabelGuardAction, "action", "", "操作类型（labeled/unlabeled）")
	labelGuardCmd.Flags().StringVar(&flagLabelGuardLabel, "label", "", "被操作的标签名")
	labelGuardCmd.MarkFlagRequired("actor")
	labelGuardCmd.MarkFlagRequired("action")
	labelGuardCmd.MarkFlagRequired("label")
}

// labelGuardGitHubClient 定义 label-guard 需要的 GitHub 操作接口
type labelGuardGitHubClient interface {
	ListComments(ctx context.Context, issueNumber int) ([]*ghapi.IssueComment, error)
	AddComment(ctx context.Context, issueNumber int, body string) (*ghapi.IssueComment, error)
	ListLabels(ctx context.Context, issueNumber int) ([]string, error)
	ReplaceLabels(ctx context.Context, issueNumber int, labels []string) error
}

func runLabelGuard(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 {
		return fmt.Errorf("必须指定 --repo 和 --issue")
	}

	configDir := "."
	if flagRepoDir != "" {
		configDir = flagRepoDir
	}
	cfg := config.LoadWithDefaults(configDir)

	// 环境变量 override
	allowlist := cfg.LabelGuard.Allowlist
	if v := os.Getenv("NIUMA_LABEL_ALLOWLIST"); v != "" {
		allowlist = splitAndTrim(v, ",")
	}

	mode := cfg.LabelGuard.GetMode()
	if v := os.Getenv("NIUMA_LABEL_GUARD_MODE"); v != "" {
		mode = v
	}

	// 校验 mode 合法性
	if err := validateLabelGuardMode(mode); err != nil {
		return err
	}

	// 校验 actor 是否在 allowlist
	if isInAllowlist(flagLabelGuardActor, allowlist) {
		fmt.Printf("Actor %s is in allowlist, skipping\n", flagLabelGuardActor)
		return nil
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	return executeLabelGuard(ctx, client, mode, flagIssue, flagLabelGuardActor, flagLabelGuardAction, flagLabelGuardLabel)
}

// validateLabelGuardMode 校验 mode 值合法性
func validateLabelGuardMode(mode string) error {
	switch mode {
	case "dry-run", "enforce":
		return nil
	default:
		return fmt.Errorf("不支持的 label-guard mode: %q（仅支持 dry-run/enforce）", mode)
	}
}

func executeLabelGuard(ctx context.Context, client labelGuardGitHubClient, mode string, issue int, actor, action, label string) error {
	// 幂等评论去重：通过 pkg/marker 检查是否已有相同 key 的 marker
	comments, err := client.ListComments(ctx, issue)
	if err != nil {
		return fmt.Errorf("读取评论失败: %w", err)
	}
	if hasLabelGuardMarker(comments, issue, label, action) {
		fmt.Printf("Duplicate guard comment found, skipping\n")
		return nil
	}

	if mode == "enforce" {
		// 回滚标签操作
		rollbackErr := rollbackLabel(ctx, client, issue, action, label)
		if rollbackErr != nil {
			log.Printf("WARNING: 回滚标签失败: %v", rollbackErr)
			// 回滚失败：发布评论告知回滚失败
			m := renderLabelGuardMarker(mode, issue, action, label, actor)
			body := fmt.Sprintf("## 🚫 Label Guard\n\n非授权账号 `%s` 修改了状态标签 `%s`（action=`%s`），自动回滚失败：%v\n\n请手动检查标签状态，并使用受控命令：`niuma state-label`。\n\n%s", actor, label, action, rollbackErr, m)
			if _, cerr := client.AddComment(ctx, issue, body); cerr != nil {
				return fmt.Errorf("发布评论失败: %w", cerr)
			}
			fmt.Printf("Enforce failed: rollback error for label %s (action=%s) by %s\n", label, action, actor)
		} else {
			// 回滚成功
			m := renderLabelGuardMarker(mode, issue, action, label, actor)
			body := fmt.Sprintf("## 🚫 Label Guard\n\n非授权账号 `%s` 修改了状态标签 `%s`（action=`%s`），已自动回滚。\n\n请使用受控命令：`niuma state-label`。\n\n%s", actor, label, action, m)
			if _, cerr := client.AddComment(ctx, issue, body); cerr != nil {
				return fmt.Errorf("发布评论失败: %w", cerr)
			}
			fmt.Printf("Enforced: rolled back label %s (action=%s) by %s\n", label, action, actor)
		}
	} else {
		// dry-run：仅评论
		m := renderLabelGuardMarker(mode, issue, action, label, actor)
		body := fmt.Sprintf("## ⚠️ Label Guard（dry-run）\n\n检测到非授权账号 `%s` 修改了状态标签 `%s`（action=`%s`）。\n\n当前为 dry-run 模式，仅记录不回滚。\n\n%s", actor, label, action, m)
		if _, err := client.AddComment(ctx, issue, body); err != nil {
			return fmt.Errorf("发布评论失败: %w", err)
		}
		fmt.Printf("Dry-run: warned about label %s (action=%s) by %s\n", label, action, actor)
	}

	return nil
}

// rollbackLabel 使用 ReplaceLabels（绕过 bot: guard）回滚标签操作
func rollbackLabel(ctx context.Context, client labelGuardGitHubClient, issue int, action, label string) error {
	currentLabels, err := client.ListLabels(ctx, issue)
	if err != nil {
		return err
	}

	var newLabels []string
	if action == "labeled" {
		// 被非法添加了，移除它
		for _, l := range currentLabels {
			if l != label {
				newLabels = append(newLabels, l)
			}
		}
	} else if action == "unlabeled" {
		// 被非法移除了，加回来
		newLabels = append(currentLabels, label)
	} else {
		return fmt.Errorf("未知 action: %s", action)
	}

	return client.ReplaceLabels(ctx, issue, newLabels)
}

// hasLabelGuardMarker 检查评论中是否已存在相同 key 的 LABEL_GUARD marker
// 幂等 key: Type=LABEL_GUARD + Issue + Label + Action
func hasLabelGuardMarker(comments []*ghapi.IssueComment, issue int, label, action string) bool {
	for _, c := range comments {
		body := c.GetBody()
		markers := marker.ParseAll(body)
		for _, m := range markers {
			if m.Type == marker.TypeLabelGuard && m.Issue == issue && m.Label == label && m.Action == action {
				return true
			}
		}
	}
	return false
}

// renderLabelGuardMarker 使用 pkg/marker 渲染 LABEL_GUARD marker
func renderLabelGuardMarker(mode string, issue int, action, label, actor string) string {
	m := &marker.Marker{
		Type:   marker.TypeLabelGuard,
		Issue:  issue,
		Mode:   mode,
		Label:  label,
		Action: action,
		Actor:  actor,
	}
	return marker.Render(m)
}

func isInAllowlist(actor string, allowlist []string) bool {
	for _, a := range allowlist {
		if a == actor {
			return true
		}
	}
	return false
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
