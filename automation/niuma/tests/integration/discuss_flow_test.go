package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/agent"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	ghapi "github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// flowGitHubMock 为 discuss 集成流程提供最小 GitHub 行为模拟
type flowGitHubMock struct {
	issues   map[int]*ghapi.Issue
	comments map[int][]*ghapi.IssueComment
	labels   map[int][]string
	markers  map[string]*gh.MarkerComment
	nextID   int64
}

func newFlowGitHubMock() *flowGitHubMock {
	return &flowGitHubMock{
		issues:   map[int]*ghapi.Issue{},
		comments: map[int][]*ghapi.IssueComment{},
		labels:   map[int][]string{},
		markers:  map[string]*gh.MarkerComment{},
	}
}

func (m *flowGitHubMock) markerKey(issueNumber int, t marker.Type) string {
	return fmt.Sprintf("%d:%s", issueNumber, t)
}

func (m *flowGitHubMock) nextCommentID() int64 {
	m.nextID++
	return m.nextID
}

func (m *flowGitHubMock) GetIssue(_ context.Context, number int) (*ghapi.Issue, error) {
	issue, ok := m.issues[number]
	if !ok {
		return nil, fmt.Errorf("issue #%d not found", number)
	}
	return issue, nil
}

func (m *flowGitHubMock) ListComments(_ context.Context, issueNumber int) ([]*ghapi.IssueComment, error) {
	return m.comments[issueNumber], nil
}

func (m *flowGitHubMock) AddComment(_ context.Context, issueNumber int, body string) (*ghapi.IssueComment, error) {
	comment := &ghapi.IssueComment{
		ID:   ghapi.Ptr(m.nextCommentID()),
		Body: ghapi.Ptr(body),
	}
	m.comments[issueNumber] = append(m.comments[issueNumber], comment)
	return comment, nil
}

func (m *flowGitHubMock) FindMarker(_ context.Context, issueNumber int, t marker.Type) (*gh.MarkerComment, error) {
	return m.markers[m.markerKey(issueNumber, t)], nil
}

func (m *flowGitHubMock) CreateOrUpdateMarker(_ context.Context, issueNumber int, mk *marker.Marker, body string) error {
	key := m.markerKey(issueNumber, mk.Type)
	if existing, ok := m.markers[key]; ok {
		existing.Marker = mk
		existing.Comment.Body = ghapi.Ptr(body)
		for _, c := range m.comments[issueNumber] {
			if c.GetID() == existing.CommentID {
				c.Body = ghapi.Ptr(body)
				break
			}
		}
		return nil
	}

	comment := &ghapi.IssueComment{
		ID:   ghapi.Ptr(m.nextCommentID()),
		Body: ghapi.Ptr(body),
	}
	m.comments[issueNumber] = append(m.comments[issueNumber], comment)
	m.markers[key] = &gh.MarkerComment{
		Marker:    mk,
		CommentID: comment.GetID(),
		Comment:   comment,
	}
	return nil
}

func (m *flowGitHubMock) ListLabels(_ context.Context, issueNumber int) ([]string, error) {
	return m.labels[issueNumber], nil
}

func (m *flowGitHubMock) AddLabel(_ context.Context, issueNumber int, label string) error {
	m.labels[issueNumber] = append(m.labels[issueNumber], label)
	return nil
}

func (m *flowGitHubMock) RemoveLabel(_ context.Context, issueNumber int, label string) error {
	labels := m.labels[issueNumber]
	for i, item := range labels {
		if item == label {
			m.labels[issueNumber] = append(labels[:i], labels[i+1:]...)
			break
		}
	}
	return nil
}

func (m *flowGitHubMock) ReplaceLabel(_ context.Context, issueNumber int, oldLabel, newLabel string) error {
	labels := m.labels[issueNumber]
	for i, item := range labels {
		if item == oldLabel {
			m.labels[issueNumber][i] = newLabel
			return nil
		}
	}
	m.labels[issueNumber] = append(m.labels[issueNumber], newLabel)
	return nil
}

func (m *flowGitHubMock) EnsureLabelsExist(_ context.Context, _ []string) error {
	return nil
}

// 以下 PR 相关方法为接口桩，本测试不会触发
func (m *flowGitHubMock) CreatePR(_ context.Context, _, _, _, _ string) (*ghapi.PullRequest, error) {
	return nil, fmt.Errorf("unexpected CreatePR call")
}

func (m *flowGitHubMock) GetPRDiff(_ context.Context, _ int) (string, error) {
	return "", nil
}

func (m *flowGitHubMock) CreatePRReview(_ context.Context, _ int, _, _ string) (*ghapi.PullRequestReview, error) {
	return nil, fmt.Errorf("unexpected CreatePRReview call")
}

func (m *flowGitHubMock) ListPRReviews(_ context.Context, _ int) ([]*ghapi.PullRequestReview, error) {
	return nil, nil
}

func (m *flowGitHubMock) ListIssuesWithLabel(_ context.Context, _ string) ([]*ghapi.Issue, error) {
	return nil, nil
}

func (m *flowGitHubMock) MergePR(_ context.Context, _ int, _ string) error {
	return nil
}

func TestDiscussFlow_SingleRunMultipleRoundsToPlanFinal(t *testing.T) {
	mockGH := newFlowGitHubMock()
	mockGH.issues[42] = &ghapi.Issue{
		Number: ghapi.Ptr(42),
		Title:  ghapi.Ptr("恢复讨论自动收敛"),
		Body:   ghapi.Ptr("needs-discussion 自动推进"),
	}
	mockGH.labels[42] = []string{string(state.StateNeedsDiscussion)}
	// 初始加入一条人工评论，模拟 issue_comment 事件触发
	mockGH.comments[42] = []*ghapi.IssueComment{{
		ID:   ghapi.Ptr(int64(1)),
		Body: ghapi.Ptr("请开始自动多轮讨论并尽快收敛"),
	}}
	mockGH.nextID = 1

	mockAI := ai.NewMockProvider(
		`{"consensus":"第一轮：补充限制","open_items":["补充边界"],"should_finish":false}`,
		`{"consensus":"第二轮：已收敛","open_items":[],"should_finish":true}`,
		`{"title":"最终方案","approach":"按共识执行","file_changes":[{"path":"automation/niuma/pkg/agent/orchestrator.go","action":"modify","description":"增加 discuss 多轮循环"}],"test_scenarios":[{"name":"自动收敛","input":"bot:needs-discussion","expected":"进入 bot:plan-final"}]}`,
	)

	orch := agent.NewOrchestrator(mockGH, mockAI, 42)
	err := orch.DoDiscuss(context.Background(), 5)
	require.NoError(t, err)

	// 单次 run 内两轮收敛，并进入 plan-final
	labels := mockGH.labels[42]
	assert.Contains(t, labels, string(state.StatePlanFinal))
	assert.NotContains(t, labels, string(state.StateNeedsDiscussion))

	summaryMC := mockGH.markers[mockGH.markerKey(42, marker.TypeDiscussionSummary)]
	require.NotNil(t, summaryMC)
	assert.Equal(t, 2, summaryMC.Marker.Revision)

	finalMC := mockGH.markers[mockGH.markerKey(42, marker.TypePlanFinal)]
	require.NotNil(t, finalMC)
	assert.Contains(t, finalMC.Comment.GetBody(), "最终方案")
}
