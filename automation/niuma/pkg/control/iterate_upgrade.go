package control

import (
	"context"
	"fmt"
	"strings"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
)

const needsHumanIssueLabel = "needs-human"

// IterateUpgradeOps 封装 iterate 人工升级所需的最小 GitHub 操作。
type IterateUpgradeOps struct {
	ListLabels    func(ctx context.Context, issueNumber int) ([]string, error)
	ReplaceLabels func(ctx context.Context, issueNumber int, labels []string) error
	AddLabel      func(ctx context.Context, issueNumber int, label string) error
	AddComment    func(ctx context.Context, issueNumber int, body string) error
}

// UpgradeIterateToNeedsHuman 执行 iterate 的人工升级流程。
func UpgradeIterateToNeedsHuman(ctx context.Context, issueNumber, prNumber int, triggerSource, prState string, ops IterateUpgradeOps) error {
	if issueNumber <= 0 {
		return fmt.Errorf("issue_number 非法: %d", issueNumber)
	}
	if ops.ListLabels == nil || ops.ReplaceLabels == nil || ops.AddLabel == nil || ops.AddComment == nil {
		return fmt.Errorf("iterate upgrade 操作器未完整配置")
	}

	labels, err := ops.ListLabels(ctx, issueNumber)
	if err != nil {
		return fmt.Errorf("读取 issue labels 失败: %w", err)
	}

	filtered := make([]string, 0, len(labels))
	for _, label := range labels {
		if state.IsBotLabel(label) {
			continue
		}
		filtered = append(filtered, label)
	}
	if err := ops.ReplaceLabels(ctx, issueNumber, filtered); err != nil {
		return fmt.Errorf("清理 bot 状态标签失败: %w", err)
	}
	if err := ops.AddLabel(ctx, issueNumber, needsHumanIssueLabel); err != nil {
		return fmt.Errorf("添加 needs-human 标签失败: %w", err)
	}

	comment := fmt.Sprintf("## ⚠️ 自动迭代已停止\n\n触发来源：`%s`\nPR：#%d\nPR 状态：`%s`\n\n系统判定该状态不适合继续自动迭代，已升级为人工处理（`needs-human`）。", strings.TrimSpace(triggerSource), prNumber, strings.ToUpper(strings.TrimSpace(prState)))
	if err := ops.AddComment(ctx, issueNumber, comment); err != nil {
		return fmt.Errorf("写入人工升级审计评论失败: %w", err)
	}

	return nil
}
