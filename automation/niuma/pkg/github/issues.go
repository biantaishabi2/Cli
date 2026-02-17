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
	if labels == nil {
		labels = []string{}
	}
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

// ListIssuesWithLabel 列出带指定 label 的所有 open issue
func (c *Client) ListIssuesWithLabel(ctx context.Context, label string) ([]*github.Issue, error) {
	var allIssues []*github.Issue
	opts := &github.IssueListByRepoOptions{
		Labels:      []string{label},
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		issues, resp, err := c.gh.Issues.ListByRepo(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("列出 label=%s 的 issues 失败: %w", label, err)
		}
		// 过滤掉 PR（GitHub API 把 PR 也算 issue）
		for _, issue := range issues {
			if issue.PullRequestLinks == nil {
				allIssues = append(allIssues, issue)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allIssues, nil
}

// ListIssuesByState 列出指定状态的 issue（过滤掉 PR）
// state 支持 open/closed/all，空值默认 open。
func (c *Client) ListIssuesByState(ctx context.Context, state string) ([]*github.Issue, error) {
	if state == "" {
		state = "open"
	}

	var allIssues []*github.Issue
	opts := &github.IssueListByRepoOptions{
		State:       state,
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		issues, resp, err := c.gh.Issues.ListByRepo(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, fmt.Errorf("列出 state=%s 的 issues 失败: %w", state, err)
		}
		for _, issue := range issues {
			if issue.PullRequestLinks == nil {
				allIssues = append(allIssues, issue)
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allIssues, nil
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
