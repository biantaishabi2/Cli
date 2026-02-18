// pkg/github/labels.go
// Issue Label 相关 API 操作
package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v68/github"
)

// ListLabels 列出 issue 的所有 label
func (c *Client) ListLabels(ctx context.Context, issueNumber int) ([]string, error) {
	var allLabels []string
	opts := &github.ListOptions{PerPage: 100}

	for {
		labels, resp, err := c.gh.Issues.ListLabelsByIssue(ctx, c.owner, c.repo, issueNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("列出 issue #%d labels 失败: %w", issueNumber, err)
		}
		for _, l := range labels {
			allLabels = append(allLabels, l.GetName())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allLabels, nil
}

// AddLabel 为 issue 添加 label
func (c *Client) AddLabel(ctx context.Context, issueNumber int, label string) error {
	_, _, err := c.gh.Issues.AddLabelsToIssue(ctx, c.owner, c.repo, issueNumber, []string{label})
	if err != nil {
		return fmt.Errorf("添加 label %q 失败: %w", label, err)
	}
	return nil
}

// RemoveLabel 移除 issue 的 label
func (c *Client) RemoveLabel(ctx context.Context, issueNumber int, label string) error {
	_, err := c.removeLabelIfPresent(ctx, issueNumber, label)
	return err
}

// removeLabelIfPresent 移除 issue 的 label，并返回是否命中。
func (c *Client) removeLabelIfPresent(ctx context.Context, issueNumber int, label string) (bool, error) {
	_, err := c.gh.Issues.RemoveLabelForIssue(ctx, c.owner, c.repo, issueNumber, label)
	if err != nil {
		// label 不存在不算错误
		if ghErr, ok := err.(*github.ErrorResponse); ok && ghErr.Response.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, fmt.Errorf("移除 label %q 失败: %w", label, err)
	}
	return true, nil
}

// ReplaceLabel 替换 label：移除 old，添加 new
func (c *Client) ReplaceLabel(ctx context.Context, issueNumber int, oldLabel, newLabel string) error {
	if err := c.RemoveLabel(ctx, issueNumber, oldLabel); err != nil {
		return err
	}
	return c.AddLabel(ctx, issueNumber, newLabel)
}

// ReplaceLabelIfPresent 替换 label：仅当 old 存在时才会写入 new。
// 返回 replaced=false 表示 old 不存在，未做任何写入。
func (c *Client) ReplaceLabelIfPresent(ctx context.Context, issueNumber int, oldLabel, newLabel string) (bool, error) {
	replaced, err := c.removeLabelIfPresent(ctx, issueNumber, oldLabel)
	if err != nil {
		return false, err
	}
	if !replaced {
		return false, nil
	}
	if oldLabel == newLabel {
		return true, nil
	}
	if err := c.AddLabel(ctx, issueNumber, newLabel); err != nil {
		return false, err
	}
	return true, nil
}

// EnsureLabelsExist 确保仓库中存在指定的 label，不存在则创建
func (c *Client) EnsureLabelsExist(ctx context.Context, labels []string) error {
	for _, name := range labels {
		_, resp, err := c.gh.Issues.GetLabel(ctx, c.owner, c.repo, name)
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				// 创建 label
				color := "0075ca" // 默认蓝色
				_, _, createErr := c.gh.Issues.CreateLabel(ctx, c.owner, c.repo, &github.Label{
					Name:  &name,
					Color: &color,
				})
				if createErr != nil {
					return fmt.Errorf("创建 label %q 失败: %w", name, createErr)
				}
				continue
			}
			return fmt.Errorf("检查 label %q 失败: %w", name, err)
		}
	}
	return nil
}
