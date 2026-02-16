// pkg/github/issues.go
// Issue 相关 API 操作
package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v68/github"
)

// GetIssue 获取指定 issue
func (c *Client) GetIssue(ctx context.Context, number int) (*github.Issue, error) {
	issue, _, err := c.gh.Issues.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("获取 issue #%d 失败: %w", number, err)
	}
	return issue, nil
}

// CreateIssue 创建新 issue
func (c *Client) CreateIssue(ctx context.Context, title, body string, labels []string) (*github.Issue, error) {
	req := &github.IssueRequest{
		Title:  &title,
		Body:   &body,
		Labels: &labels,
	}
	issue, _, err := c.gh.Issues.Create(ctx, c.owner, c.repo, req)
	if err != nil {
		return nil, fmt.Errorf("创建 issue 失败: %w", err)
	}
	return issue, nil
}

// CloseIssue 关闭指定 issue
func (c *Client) CloseIssue(ctx context.Context, number int) error {
	state := "closed"
	req := &github.IssueRequest{State: &state}
	_, _, err := c.gh.Issues.Edit(ctx, c.owner, c.repo, number, req)
	if err != nil {
		return fmt.Errorf("关闭 issue #%d 失败: %w", number, err)
	}
	return nil
}
