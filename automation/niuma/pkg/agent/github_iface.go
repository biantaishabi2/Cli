// pkg/agent/github_iface.go
// GitHubOps 接口：抽象 GitHub 操作，方便测试 mock
package agent

import (
	"context"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/google/go-github/v68/github"
)

// GitHubOps GitHub 操作接口
// *gh.Client 隐式满足此接口
type GitHubOps interface {
	// Issue 操作
	GetIssue(ctx context.Context, number int) (*github.Issue, error)

	// 评论操作
	ListComments(ctx context.Context, issueNumber int) ([]*github.IssueComment, error)
	AddComment(ctx context.Context, issueNumber int, body string) (*github.IssueComment, error)

	// Marker 操作
	FindMarker(ctx context.Context, issueNumber int, t marker.Type) (*gh.MarkerComment, error)
	CreateOrUpdateMarker(ctx context.Context, issueNumber int, m *marker.Marker, body string) error

	// Label 操作
	ListLabels(ctx context.Context, issueNumber int) ([]string, error)
	AddLabel(ctx context.Context, issueNumber int, label string) error
	RemoveLabel(ctx context.Context, issueNumber int, label string) error
	ReplaceLabel(ctx context.Context, issueNumber int, oldLabel, newLabel string) error
	EnsureLabelsExist(ctx context.Context, labels []string) error

	// PR 操作
	CreatePR(ctx context.Context, title, body, head, base string) (*github.PullRequest, error)
	ListPRReviews(ctx context.Context, number int) ([]*github.PullRequestReview, error)
}
