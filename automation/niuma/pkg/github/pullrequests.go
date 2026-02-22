// pkg/github/pullrequests.go
// Pull Request 相关 API 操作
package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v68/github"
)

// PRFile 表示 PR 中一个变更文件（用于 plan_files 自动同步）。
type PRFile struct {
	Filename string
	Status   string
}

// GetPR 获取指定 Pull Request
func (c *Client) GetPR(ctx context.Context, number int) (*github.PullRequest, error) {
	pr, _, err := c.gh.PullRequests.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("获取 PR #%d 失败: %w", number, err)
	}
	return pr, nil
}

// UpdatePRBody 更新 PR 描述正文。
func (c *Client) UpdatePRBody(ctx context.Context, number int, body string) error {
	_, _, err := c.gh.PullRequests.Edit(ctx, c.owner, c.repo, number, &github.PullRequest{
		Body: github.Ptr(body),
	})
	if err != nil {
		return fmt.Errorf("更新 PR #%d body 失败: %w", number, err)
	}
	return nil
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

// CreatePRReview 创建 PR review
// event: "APPROVE" 或 "REQUEST_CHANGES"
func (c *Client) CreatePRReview(ctx context.Context, number int, body, event string) (*github.PullRequestReview, error) {
	review, _, err := c.gh.PullRequests.CreateReview(ctx, c.owner, c.repo, number, &github.PullRequestReviewRequest{
		Body:  &body,
		Event: &event,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 PR #%d review 失败: %w", number, err)
	}
	return review, nil
}

// MergePR 合并 Pull Request
// method: "merge", "squash", "rebase"
func (c *Client) MergePR(ctx context.Context, number int, method string) error {
	opts := &github.PullRequestOptions{
		MergeMethod: method,
	}
	_, _, err := c.gh.PullRequests.Merge(ctx, c.owner, c.repo, number, "", opts)
	if err != nil {
		return fmt.Errorf("合并 PR #%d 失败: %w", number, err)
	}
	return nil
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

// ListPRFiles 列出 PR 当前变更文件（用于 PLAN_FILES 与实际 diff 对齐）。
func (c *Client) ListPRFiles(ctx context.Context, number int) ([]PRFile, error) {
	var out []PRFile
	opts := &github.ListOptions{PerPage: 100}

	for {
		files, resp, err := c.gh.PullRequests.ListFiles(ctx, c.owner, c.repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("列出 PR #%d files 失败: %w", number, err)
		}
		for _, f := range files {
			out = append(out, PRFile{
				Filename: f.GetFilename(),
				Status:   f.GetStatus(),
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return out, nil
}

// ListPRCommitMessages 列出 PR 中所有 commit message（含标题+正文）
func (c *Client) ListPRCommitMessages(ctx context.Context, number int) ([]string, error) {
	var messages []string
	opts := &github.ListOptions{PerPage: 100}

	for {
		commits, resp, err := c.gh.PullRequests.ListCommits(ctx, c.owner, c.repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("列出 PR #%d commits 失败: %w", number, err)
		}
		for _, commit := range commits {
			if commit == nil || commit.Commit == nil {
				continue
			}
			messages = append(messages, commit.Commit.GetMessage())
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return messages, nil
}
