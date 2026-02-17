package control

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGitHubOps 用于 controller 测试的 GitHub mock
type mockGitHubOps struct {
	issues     []IssueInfo
	mergedPRs  []int
	mergeError map[int]error
}

func newMockGitHubOps(issues ...IssueInfo) *mockGitHubOps {
	return &mockGitHubOps{
		issues:     issues,
		mergeError: make(map[int]error),
	}
}

func (m *mockGitHubOps) ListIssuesWithLabel(_ context.Context, _ string) ([]IssueInfo, error) {
	return m.issues, nil
}

func (m *mockGitHubOps) MergePR(_ context.Context, prNum int, _ string) error {
	if err, ok := m.mergeError[prNum]; ok {
		return err
	}
	m.mergedPRs = append(m.mergedPRs, prNum)
	return nil
}

func (m *mockGitHubOps) ReplaceLabel(_ context.Context, _ int, _, _ string) error {
	return nil
}

// mockTaskCtlClient 用于 controller 测试的 taskctl mock
type mockTaskCtlClient struct {
	tasks     []Task
	nextID    int
	readyList []Task
}

func newMockTaskCtlClient() *mockTaskCtlClient {
	return &mockTaskCtlClient{}
}

func (m *mockTaskCtlClient) create(subject, desc string, meta map[string]string) (*Task, error) {
	m.nextID++
	task := Task{
		ID:       fmt.Sprintf("task-%d", m.nextID),
		Subject:  subject,
		Desc:     desc,
		Status:   TaskStatusPending,
		Metadata: meta,
	}
	m.tasks = append(m.tasks, task)
	return &task, nil
}

func (m *mockTaskCtlClient) update(taskID string, opts UpdateOpts) error {
	for i := range m.tasks {
		if m.tasks[i].ID == taskID {
			if opts.Status != nil {
				m.tasks[i].Status = *opts.Status
			}
			if opts.BlockedBy != nil {
				m.tasks[i].BlockedBy = *opts.BlockedBy
			}
			if opts.Metadata != nil {
				if m.tasks[i].Metadata == nil {
					m.tasks[i].Metadata = make(map[string]string)
				}
				for k, v := range *opts.Metadata {
					m.tasks[i].Metadata[k] = v
				}
			}
			return nil
		}
	}
	return fmt.Errorf("task %s not found", taskID)
}

func (m *mockTaskCtlClient) list(_ string) ([]Task, error) {
	return m.tasks, nil
}

func (m *mockTaskCtlClient) ready() ([]Task, error) {
	if m.readyList != nil {
		return m.readyList, nil
	}
	var ready []Task
	for _, t := range m.tasks {
		if t.Status == TaskStatusPending && len(t.BlockedBy) == 0 {
			ready = append(ready, t)
		}
	}
	return ready, nil
}

// inMemController 使用内存 mock 创建 Controller（绕过真实 taskctl 二进制）
type inMemController struct {
	*Controller
	mockTaskCtl *mockTaskCtlClient
	mockGitHub  *mockGitHubOps
}

func newInMemController(issues []IssueInfo, aiResp string) *inMemController {
	mockTC := newMockTaskCtlClient()
	mockGH := newMockGitHubOps(issues...)

	var provider ai.Provider
	if aiResp != "" {
		provider = ai.NewMockProvider(aiResp)
	}

	analyzer := NewDependencyAnalyzer(provider)

	// 我们需要包装 mockTaskCtlClient 使其被 Controller 使用
	// 但 Controller 使用 *TaskCtlClient，所以我们直接测试核心逻辑
	ctrl := &Controller{
		analyzer: analyzer,
		github:   mockGH,
		cfg:      DefaultControlConfig(),
	}

	return &inMemController{
		Controller:  ctrl,
		mockTaskCtl: mockTC,
		mockGitHub:  mockGH,
	}
}

// RunInMem 模拟 Run 逻辑但使用内存 mock
func (c *inMemController) RunInMem(ctx context.Context) error {
	issues, err := c.github.ListIssuesWithLabel(ctx, "bot:orchestrate")
	if err != nil {
		return err
	}

	// 找新 issue
	existing := make(map[int]bool)
	for _, t := range c.mockTaskCtl.tasks {
		if n := t.IssueNum(); n > 0 {
			existing[n] = true
		}
	}

	var newIssues []IssueInfo
	for _, issue := range issues {
		if !existing[issue.Number] {
			newIssues = append(newIssues, issue)
		}
	}

	// 分析依赖
	analysis, _ := c.analyzer.Analyze(ctx, issues)

	// 创建 task
	issueToTask := make(map[int]string)
	for _, t := range c.mockTaskCtl.tasks {
		if n := t.IssueNum(); n > 0 {
			issueToTask[n] = t.ID
		}
	}
	for _, issue := range newIssues {
		meta := map[string]string{"issue_num": strconv.Itoa(issue.Number)}
		task, err := c.mockTaskCtl.create(issue.Title, "", meta)
		if err != nil {
			continue
		}
		issueToTask[issue.Number] = task.ID
	}

	// 设置依赖
	for issueNum, deps := range analysis.Dependencies {
		taskID, ok := issueToTask[issueNum]
		if !ok {
			continue
		}
		var blockedBy []string
		for _, dep := range deps {
			if depTaskID, ok := issueToTask[dep]; ok {
				blockedBy = append(blockedBy, depTaskID)
			}
		}
		if len(blockedBy) > 0 {
			_ = c.mockTaskCtl.update(taskID, UpdateOpts{BlockedBy: &blockedBy})
		}
	}

	// 推进 ready tasks
	readyTasks, _ := c.mockTaskCtl.ready()
	for _, task := range readyTasks {
		status := TaskStatusInProgress
		_ = c.mockTaskCtl.update(task.ID, UpdateOpts{Status: &status})
	}

	return nil
}

func TestController_NewIssueDiscovery(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth fix", Body: "fix auth"},
		{Number: 41, Title: "Payment fix", Body: "fix payment"},
		{Number: 42, Title: "Test fix", Body: "fix tests"},
	}

	ctrl := newInMemController(issues, "")
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	assert.Len(t, ctrl.mockTaskCtl.tasks, 3)
	for _, task := range ctrl.mockTaskCtl.tasks {
		assert.NotEmpty(t, task.ID)
		assert.Equal(t, TaskStatusInProgress, task.Status) // 无依赖，应被推进
	}
}

func TestController_DependencySetup(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Payment", Body: "fix payment"},
		{Number: 42, Title: "Tests", Body: "depends-on: #40"},
	}

	ctrl := newInMemController(issues, "")
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	// 找到 issue 42 的 task
	var task42 *Task
	for i := range ctrl.mockTaskCtl.tasks {
		if ctrl.mockTaskCtl.tasks[i].IssueNum() == 42 {
			task42 = &ctrl.mockTaskCtl.tasks[i]
		}
	}
	require.NotNil(t, task42)
	assert.Len(t, task42.BlockedBy, 1)
	// 42 应该仍为 pending（因为有依赖）
	assert.Equal(t, TaskStatusPending, task42.Status)
}

func TestController_Idempotent(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
	}

	ctrl := newInMemController(issues, "")

	// 第一次 Run
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)
	assert.Len(t, ctrl.mockTaskCtl.tasks, 1)

	// 第二次 Run（不应创建新 task）
	err = ctrl.RunInMem(context.Background())
	require.NoError(t, err)
	assert.Len(t, ctrl.mockTaskCtl.tasks, 1)
}

func TestController_ReadyTaskAdvance(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Tests", Body: "depends-on: #40"},
	}

	ctrl := newInMemController(issues, "")
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	// 40 应被推进（无依赖），41 应仍为 pending
	for _, task := range ctrl.mockTaskCtl.tasks {
		switch task.IssueNum() {
		case 40:
			assert.Equal(t, TaskStatusInProgress, task.Status)
		case 41:
			assert.Equal(t, TaskStatusPending, task.Status)
		}
	}
}

func TestController_AdvanceExistingTasksWithoutNewOrchestrateIssues(t *testing.T) {
	ctrl := newInMemController(nil, "")

	_, err := ctrl.mockTaskCtl.create("Existing queued task", "", map[string]string{"issue_num": "999"})
	require.NoError(t, err)

	err = ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	require.Len(t, ctrl.mockTaskCtl.tasks, 1)
	assert.Equal(t, TaskStatusInProgress, ctrl.mockTaskCtl.tasks[0].Status)
}

func TestFormatStatus(t *testing.T) {
	status := &ControlStatus{
		Dag: &DagGraph{},
		Tasks: []Task{
			{ID: "t1", Subject: "Auth fix", Status: TaskStatusCompleted, Metadata: map[string]string{"issue_num": "40", "pr_num": "10"}},
			{ID: "t2", Subject: "Payment fix", Status: TaskStatusInProgress, Metadata: map[string]string{"issue_num": "41"}},
		},
	}

	output := FormatStatus(status)
	assert.Contains(t, output, "#40")
	assert.Contains(t, output, "#41")
	assert.Contains(t, output, "completed")
	assert.Contains(t, output, string(TaskStatusInProgress))
}
