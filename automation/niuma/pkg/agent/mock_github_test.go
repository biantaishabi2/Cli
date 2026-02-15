package agent

import (
	"context"
	"fmt"
	"sync"

	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/google/go-github/v68/github"
)

// MockGitHub 用于测试的 GitHub 操作 mock
type MockGitHub struct {
	mu sync.Mutex

	// 数据存储
	Issues   map[int]*github.Issue
	Comments map[int][]*github.IssueComment // issueNumber → comments
	Labels   map[int][]string               // issueNumber → labels
	Markers  map[string]*gh.MarkerComment   // "type:issueNumber" → marker comment
	PRs      []*github.PullRequest
	PRDiffs  map[int]string                      // prNumber → diff
	Reviews  map[int][]*github.PullRequestReview // prNumber → reviews

	// 计数器
	commentID int64
	prNumber  int

	// 错误注入
	Error error
}

// NewMockGitHub 创建 MockGitHub
func NewMockGitHub() *MockGitHub {
	return &MockGitHub{
		Issues:   make(map[int]*github.Issue),
		Comments: make(map[int][]*github.IssueComment),
		Labels:   make(map[int][]string),
		Markers:  make(map[string]*gh.MarkerComment),
		PRDiffs:  make(map[int]string),
		Reviews:  make(map[int][]*github.PullRequestReview),
	}
}

func (m *MockGitHub) GetIssue(_ context.Context, number int) (*github.Issue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return nil, m.Error
	}

	issue, ok := m.Issues[number]
	if !ok {
		return nil, fmt.Errorf("issue #%d not found", number)
	}
	return issue, nil
}

func (m *MockGitHub) ListComments(_ context.Context, issueNumber int) ([]*github.IssueComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return nil, m.Error
	}

	return m.Comments[issueNumber], nil
}

func (m *MockGitHub) AddComment(_ context.Context, issueNumber int, body string) (*github.IssueComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return nil, m.Error
	}

	m.commentID++
	comment := &github.IssueComment{
		ID:   github.Ptr(m.commentID),
		Body: github.Ptr(body),
	}
	m.Comments[issueNumber] = append(m.Comments[issueNumber], comment)
	return comment, nil
}

func (m *MockGitHub) FindMarker(_ context.Context, issueNumber int, t marker.Type) (*gh.MarkerComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return nil, m.Error
	}

	key := fmt.Sprintf("%s:%d", t, issueNumber)
	return m.Markers[key], nil
}

func (m *MockGitHub) CreateOrUpdateMarker(_ context.Context, issueNumber int, mk *marker.Marker, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return m.Error
	}

	key := fmt.Sprintf("%s:%d", mk.Type, issueNumber)
	m.commentID++
	m.Markers[key] = &gh.MarkerComment{
		Marker:    mk,
		CommentID: m.commentID,
		Comment: &github.IssueComment{
			ID:   github.Ptr(m.commentID),
			Body: github.Ptr(body),
		},
	}
	return nil
}

func (m *MockGitHub) ListLabels(_ context.Context, issueNumber int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return nil, m.Error
	}

	return m.Labels[issueNumber], nil
}

func (m *MockGitHub) AddLabel(_ context.Context, issueNumber int, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return m.Error
	}

	m.Labels[issueNumber] = append(m.Labels[issueNumber], label)
	return nil
}

func (m *MockGitHub) RemoveLabel(_ context.Context, issueNumber int, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return m.Error
	}

	labels := m.Labels[issueNumber]
	for i, l := range labels {
		if l == label {
			m.Labels[issueNumber] = append(labels[:i], labels[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockGitHub) ReplaceLabel(_ context.Context, issueNumber int, oldLabel, newLabel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return m.Error
	}

	labels := m.Labels[issueNumber]
	for i, l := range labels {
		if l == oldLabel {
			m.Labels[issueNumber][i] = newLabel
			return nil
		}
	}
	// 没找到旧 label，直接添加新的
	m.Labels[issueNumber] = append(m.Labels[issueNumber], newLabel)
	return nil
}

func (m *MockGitHub) EnsureLabelsExist(_ context.Context, _ []string) error {
	return m.Error
}

func (m *MockGitHub) CreatePR(_ context.Context, title, body, head, base string) (*github.PullRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return nil, m.Error
	}

	m.prNumber++
	pr := &github.PullRequest{
		Number: github.Ptr(m.prNumber),
		Title:  github.Ptr(title),
		Body:   github.Ptr(body),
		Head:   &github.PullRequestBranch{Ref: github.Ptr(head)},
		Base:   &github.PullRequestBranch{Ref: github.Ptr(base)},
	}
	m.PRs = append(m.PRs, pr)
	return pr, nil
}

func (m *MockGitHub) GetPRDiff(_ context.Context, number int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return "", m.Error
	}

	if diff, ok := m.PRDiffs[number]; ok {
		return diff, nil
	}
	return "", nil
}

func (m *MockGitHub) ListPRReviews(_ context.Context, number int) ([]*github.PullRequestReview, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Error != nil {
		return nil, m.Error
	}

	return m.Reviews[number], nil
}

// 辅助方法

// SetIssue 设置 mock issue
func (m *MockGitHub) SetIssue(number int, title, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Issues[number] = &github.Issue{
		Number: github.Ptr(number),
		Title:  github.Ptr(title),
		Body:   github.Ptr(body),
	}
}

// SetLabel 设置 mock label
func (m *MockGitHub) SetLabel(issueNumber int, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Labels[issueNumber] = labels
}

// SetMarker 设置 mock marker
func (m *MockGitHub) SetMarker(issueNumber int, mk *marker.Marker, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%d", mk.Type, issueNumber)
	m.commentID++
	m.Markers[key] = &gh.MarkerComment{
		Marker:    mk,
		CommentID: m.commentID,
		Comment: &github.IssueComment{
			ID:   github.Ptr(m.commentID),
			Body: github.Ptr(body),
		},
	}
}

// GetMarker 获取 mock marker
func (m *MockGitHub) GetMarker(issueNumber int, t marker.Type) *gh.MarkerComment {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s:%d", t, issueNumber)
	return m.Markers[key]
}
