// pkg/github/pullrequests.go
// Pull Request 相关 API 操作
package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v68/github"
)

// GetPR 获取指定 Pull Request
func (c *Client) GetPR(ctx context.Context, number int) (*github.PullRequest, error) {
	pr, _, err := c.gh.PullRequests.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("获取 PR #%d 失败: %w", number, err)
	}
	return pr, nil
}

// CreatePR 创建 Pull Request
func (c *Client) CreatePR(ctx context.Context, title, body, head, base string) (*github.PullRequest, error) {
	pr, _, err := c.gh.PullRequests.Create(ctx, c.owner, c.repo, &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 PR 失败: %w", err)
	}
	return pr, nil
}

// GetPRDiff 获取 PR 的 diff 内容
func (c *Client) GetPRDiff(ctx context.Context, number int) (string, error) {
	diff, _, err := c.gh.PullRequests.GetRaw(ctx, c.owner, c.repo, number, github.RawOptions{Type: github.Diff})
	if err != nil {
		return "", fmt.Errorf("获取 PR #%d diff 失败: %w", number, err)
	}
	return diff, nil
}

// ListPRReviews 列出 PR 的所有 review
func (c *Client) ListPRReviews(ctx context.Context, number int) ([]*github.PullRequestReview, error) {
	var allReviews []*github.PullRequestReview
	opts := &github.ListOptions{PerPage: 100}

	for {
		reviews, resp, err := c.gh.PullRequests.ListReviews(ctx, c.owner, c.repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("列出 PR #%d reviews 失败: %w", number, err)
		}
		allReviews = append(allReviews, reviews...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allReviews, nil
}
