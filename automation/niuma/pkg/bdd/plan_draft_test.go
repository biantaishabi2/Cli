//go:build bdd

package bdd

import (
	"context"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockGitHubClient 模拟 GitHub API 操作
type MockGitHubClient struct {
	Issues   map[int]*mockIssue
	Comments map[int][]string
	Labels   map[int][]string
	nextID   int
}

type mockIssue struct {
	Number  int
	Title   string
	Body    string
	State   string
	Labels  []string
}

func NewMockGitHubClient() *MockGitHubClient {
	return &MockGitHubClient{
		Issues:   make(map[int]*mockIssue),
		Comments: make(map[int][]string),
		Labels:   make(map[int][]string),
		nextID:   1,
	}
}

func (m *MockGitHubClient) CreateIssue(ctx context.Context, title, body string, labels []string) (*mockIssue, error) {
	issue := &mockIssue{
		Number: m.nextID,
		Title:  title,
		Body:   body,
		State:  "open",
		Labels: labels,
	}
	m.Issues[issue.Number] = issue
	m.Labels[issue.Number] = labels
	m.nextID++
	return issue, nil
}

func (m *MockGitHubClient) AddLabel(ctx context.Context, issueNum int, label string) error {
	m.Labels[issueNum] = append(m.Labels[issueNum], label)
	return nil
}

func (m *MockGitHubClient) AddComment(ctx context.Context, issueNum int, body string) error {
	m.Comments[issueNum] = append(m.Comments[issueNum], body)
	return nil
}

func (m *MockGitHubClient) GetComments(ctx context.Context, issueNum int) ([]string, error) {
	return m.Comments[issueNum], nil
}

func (m *MockGitHubClient) GetLabels(ctx context.Context, issueNum int) ([]string, error) {
	return m.Labels[issueNum], nil
}

// PlanDraftWorkflow 执行 plan draft 工作流
type PlanDraftWorkflow struct {
	github *MockGitHubClient
	ai     ai.Provider
}

func NewPlanDraftWorkflow(gh *MockGitHubClient, aiProvider ai.Provider) *PlanDraftWorkflow {
	return &PlanDraftWorkflow{
		github: gh,
		ai:     aiProvider,
	}
}

func (w *PlanDraftWorkflow) Execute(ctx context.Context, issueNum int) error {
	// 检查当前状态
	labels, _ := w.github.GetLabels(ctx, issueNum)
	if !contains(labels, string(state.StateFixRequested)) {
		return nil // 不是我们的 issue，跳过
	}

	// 检查是否已有 marker
	comments, _ := w.github.GetComments(ctx, issueNum)
	for _, c := range comments {
		if m := marker.Parse(c); m != nil && m.Type == marker.TypePlanDraft {
			return nil // 已经执行过，幂等
		}
	}

	// 调用 AI 生成 Draft Plan
	issue := w.github.Issues[issueNum]
	prompt := buildDraftPrompt(issue.Title, issue.Body)
	plan, err := w.ai.Complete(ctx, prompt)
	if err != nil {
		return err
	}

	// 添加 marker
	planWithMarker := plan + "\n\n" + marker.Render(&marker.Marker{
		Type:     marker.TypePlanDraft,
		Issue:    issueNum,
		Revision: 1,
	})

	// 发布评论
	if err := w.github.AddComment(ctx, issueNum, planWithMarker); err != nil {
		return err
	}

	// 更新状态 label
	if err := w.github.AddLabel(ctx, issueNum, string(state.StatePlanDraft)); err != nil {
		return err
	}

	return nil
}

func buildDraftPrompt(title, body string) string {
	return `请分析以下 Issue 并输出 Draft Plan：

标题: ` + title + `

内容: ` + body + `

请输出：
1. 问题理解
2. 可能的解决方案（草案）
3. 需要确认的信息（如果有）`
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ========== BDD Tests ==========

// TestPlanDraft_NewIssue 场景：新 Issue 触发 Draft Plan
func TestPlanDraft_NewIssue(t *testing.T) {
	ctx := context.Background()
	gh := NewMockGitHubClient()
	aiProvider := ai.NewMockProvider(`## Draft Plan

### 问题理解
Worker pool 在关闭后没有释放 goroutine。

### 解决方案
1. 添加 context 取消信号
2. 在 Shutdown() 中关闭 channel

### 需要确认
- 是否有正在处理的任务需要等待？`)

	workflow := NewPlanDraftWorkflow(gh, aiProvider)

	// Given: 一个刚创建的 Issue，带有 bot:fix 标签
	issue, err := gh.CreateIssue(ctx,
		"Fix memory leak in worker pool",
		"Worker pool doesn't release goroutines after shutdown",
		[]string{"bot:fix"})
	require.NoError(t, err)

	// When: 执行 plan draft 工作流
	err = workflow.Execute(ctx, issue.Number)
	require.NoError(t, err)

	// Then: 应该添加 Draft Plan 评论
	comments := gh.Comments[issue.Number]
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "Draft Plan")
	assert.Contains(t, comments[0], "BOT:PLAN_DRAFT")

	// And: 应该打上 bot:plan-draft 标签
	labels := gh.Labels[issue.Number]
	assert.Contains(t, labels, "bot:plan-draft")

	// And: AI 被调用了一次
	assert.Equal(t, 1, aiProvider.CallCount())
}

// TestPlanDraft_Idempotent 场景：幂等性，重复执行不会重复评论
func TestPlanDraft_Idempotent(t *testing.T) {
	ctx := context.Background()
	gh := NewMockGitHubClient()
	aiProvider := ai.NewMockProvider("Draft Plan content")

	workflow := NewPlanDraftWorkflow(gh, aiProvider)

	// Given: 已经执行过 plan draft 的 Issue
	issue, _ := gh.CreateIssue(ctx, "Test", "Body", []string{"bot:fix"})
	workflow.Execute(ctx, issue.Number) // 第一次执行

	// When: 再次执行
	err := workflow.Execute(ctx, issue.Number)
	require.NoError(t, err)

	// Then: 应该只有一条评论（幂等）
	comments := gh.Comments[issue.Number]
	assert.Len(t, comments, 1)

	// And: AI 只被调用了一次
	assert.Equal(t, 1, aiProvider.CallCount())
}

// TestPlanDraft_NotOurIssue 场景：没有 bot:fix 标签的 Issue 不处理
func TestPlanDraft_NotOurIssue(t *testing.T) {
	ctx := context.Background()
	gh := NewMockGitHubClient()
	aiProvider := ai.NewMockProvider("Draft Plan")

	workflow := NewPlanDraftWorkflow(gh, aiProvider)

	// Given: 一个普通 Issue，没有 bot:fix 标签
	issue, _ := gh.CreateIssue(ctx, "Regular Issue", "Some bug", []string{"bug"})

	// When: 执行 plan draft
	err := workflow.Execute(ctx, issue.Number)
	require.NoError(t, err)

	// Then: 没有评论，没有调用 AI
	assert.Len(t, gh.Comments[issue.Number], 0)
	assert.Equal(t, 0, aiProvider.CallCount())
}

// TestPlanDraft_AIError 场景：AI 调用失败时应该优雅处理
func TestPlanDraft_AIError(t *testing.T) {
	ctx := context.Background()
	gh := NewMockGitHubClient()
	aiProvider := ai.NewMockProviderWithError(assert.AnError)

	workflow := NewPlanDraftWorkflow(gh, aiProvider)

	// Given: 一个有效的 Issue
	issue, _ := gh.CreateIssue(ctx, "Test", "Body", []string{"bot:fix"})

	// When: 执行 plan draft（AI 会失败）
	err := workflow.Execute(ctx, issue.Number)

	// Then: 应该返回错误
	assert.Error(t, err)

	// And: 没有添加评论（不输出半成品）
	assert.Len(t, gh.Comments[issue.Number], 0)
}
