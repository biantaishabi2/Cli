// pkg/github/comments.go
// Issue 评论相关 API 操作 + Marker 查找与更新
package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/google/go-github/v68/github"
)

// ListComments 列出 issue 的所有评论
func (c *Client) ListComments(ctx context.Context, issueNumber int) ([]*github.IssueComment, error) {
	var allComments []*github.IssueComment
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		comments, resp, err := c.gh.Issues.ListComments(ctx, c.owner, c.repo, issueNumber, opts)
		if err != nil {
			return nil, fmt.Errorf("列出 issue #%d 评论失败: %w", issueNumber, err)
		}
		allComments = append(allComments, comments...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// AddComment 添加评论
func (c *Client) AddComment(ctx context.Context, issueNumber int, body string) (*github.IssueComment, error) {
	comment, _, err := c.gh.Issues.CreateComment(ctx, c.owner, c.repo, issueNumber, &github.IssueComment{
		Body: &body,
	})
	if err != nil {
		return nil, fmt.Errorf("添加评论失败: %w", err)
	}
	return comment, nil
}

// UpdateComment 更新评论
func (c *Client) UpdateComment(ctx context.Context, commentID int64, body string) error {
	_, _, err := c.gh.Issues.EditComment(ctx, c.owner, c.repo, commentID, &github.IssueComment{
		Body: &body,
	})
	if err != nil {
		return fmt.Errorf("更新评论 %d 失败: %w", commentID, err)
	}
	return nil
}

// DeleteComment 删除评论
func (c *Client) DeleteComment(ctx context.Context, commentID int64) error {
	_, err := c.gh.Issues.DeleteComment(ctx, c.owner, c.repo, commentID)
	if err != nil {
		return fmt.Errorf("删除评论 %d 失败: %w", commentID, err)
	}
	return nil
}

// MarkerComment 包含 Marker 及其所在评论的信息
type MarkerComment struct {
	Marker    *marker.Marker
	Comment   *github.IssueComment
	CommentID int64
}

// FindMarker 在 issue 评论中查找指定类型的最新 Marker
func (c *Client) FindMarker(ctx context.Context, issueNumber int, t marker.Type) (*MarkerComment, error) {
	comments, err := c.ListComments(ctx, issueNumber)
	if err != nil {
		return nil, err
	}

	var best *MarkerComment
	for _, comment := range comments {
		body := comment.GetBody()
		markers := marker.ParseAll(body)
		for _, m := range markers {
			if m.Type == t && m.Issue == issueNumber {
				if best == nil || m.Revision > best.Marker.Revision {
					best = &MarkerComment{
						Marker:    m,
						Comment:   comment,
						CommentID: comment.GetID(),
					}
				}
			}
		}
	}

	return best, nil
}

// CreateOrUpdateMarker 创建或更新包含 Marker 的评论
// 如果已存在同类型 Marker 的评论，则更新该评论；否则创建新评论
func (c *Client) CreateOrUpdateMarker(ctx context.Context, issueNumber int, m *marker.Marker, body string) error {
	markerLine := marker.Render(m)
	fullBody := markerLine + "\n\n" + body

	// 查找已有的 marker 评论
	existing, err := c.FindMarker(ctx, issueNumber, m.Type)
	if err != nil {
		return err
	}

	if existing != nil {
		return c.UpdateComment(ctx, existing.CommentID, fullBody)
	}

	_, err = c.AddComment(ctx, issueNumber, fullBody)
	return err
}

// ToCommentBodies 将评论列表转为 body 文本列表
func ToCommentBodies(comments []*github.IssueComment) []string {
	bodies := make([]string, 0, len(comments))
	for _, c := range comments {
		body := strings.TrimSpace(c.GetBody())
		if body != "" {
			bodies = append(bodies, body)
		}
	}
	return bodies
}
