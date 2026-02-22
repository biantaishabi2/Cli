// pkg/agent/github_iface.go
// GitHubOps 接口：抽象 GitHub 操作，方便测试 mock
package agent

import (
	"context"

	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
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
	ReplaceLabelIfPresent(ctx context.Context, issueNumber int, oldLabel, newLabel string) (bool, error)
	ReplaceLabels(ctx context.Context, issueNumber int, labels []string) error
	EnsureLabelsExist(ctx context.Context, labels []string) error

	// PR 操作
	GetPR(ctx context.Context, number int) (*github.PullRequest, error)
	CreatePR(ctx context.Context, title, body, head, base string) (*github.PullRequest, error)
	UpdatePRBody(ctx context.Context, number int, body string) error
	ListPRFiles(ctx context.Context, number int) ([]gh.PRFile, error)
	GetPRDiff(ctx context.Context, number int) (string, error)
	CreatePRReview(ctx context.Context, number int, body, event string) (*github.PullRequestReview, error)
	ListPRReviews(ctx context.Context, number int) ([]*github.PullRequestReview, error)

	// 批量操作（Phase 3 control）
	ListIssuesWithLabel(ctx context.Context, label string) ([]*github.Issue, error)
	MergePR(ctx context.Context, number int, method string) error
}
