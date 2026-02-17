package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGitHubOps 用于 controller 测试的 GitHub mock
type mockGitHubOps struct {
	issues            []IssueInfo
	issuesByLabel     map[string][]IssueInfo
	issuesByNumber    map[int]IssueInfo
	mergedPRs         []int
	mergeError        map[int]error
	replaceLabelCalls []replaceLabelCall
	replaceLabelError map[string]error
	replaceLabelFails map[string]int
	closeIssueCalls   []int
	closeIssueError   map[int]error
}

type replaceLabelCall struct {
	issueNumber int
	oldLabel    string
	newLabel    string
}

func newMockGitHubOps(issues ...IssueInfo) *mockGitHubOps {
	issuesByNumber := make(map[int]IssueInfo, len(issues))
	for _, issue := range issues {
		issuesByNumber[issue.Number] = issue
	}

	return &mockGitHubOps{
		issues:            issues,
		issuesByLabel:     make(map[string][]IssueInfo),
		issuesByNumber:    issuesByNumber,
		mergeError:        make(map[int]error),
		replaceLabelError: make(map[string]error),
		replaceLabelFails: make(map[string]int),
		closeIssueError:   make(map[int]error),
	}
}

func (m *mockGitHubOps) ListIssuesWithLabel(_ context.Context, label string) ([]IssueInfo, error) {
	if issues, ok := m.issuesByLabel[label]; ok {
		return issues, nil
	}
	return m.issues, nil
}

func (m *mockGitHubOps) ListIssuesByState(_ context.Context, state string) ([]IssueInfo, error) {
	if state == "all" {
		result := make([]IssueInfo, 0, len(m.issuesByNumber))
		for _, issue := range m.issuesByNumber {
			result = append(result, issue)
		}
		return result, nil
	}

	result := make([]IssueInfo, 0, len(m.issuesByNumber))
	for _, issue := range m.issuesByNumber {
		if strings.EqualFold(issue.State, state) {
			result = append(result, issue)
		}
	}
	return result, nil
}

func (m *mockGitHubOps) GetIssue(_ context.Context, issueNumber int) (IssueInfo, error) {
	issue, ok := m.issuesByNumber[issueNumber]
	if !ok {
		return IssueInfo{}, fmt.Errorf("issue #%d not found", issueNumber)
	}
	return issue, nil
}

func (m *mockGitHubOps) CloseIssue(_ context.Context, issueNumber int) error {
	if err, ok := m.closeIssueError[issueNumber]; ok {
		return err
	}
	issue, ok := m.issuesByNumber[issueNumber]
	if ok {
		issue.State = "closed"
		m.issuesByNumber[issueNumber] = issue
	}
	m.closeIssueCalls = append(m.closeIssueCalls, issueNumber)
	return nil
}

func (m *mockGitHubOps) MergePR(_ context.Context, prNum int, _ string) error {
	if err, ok := m.mergeError[prNum]; ok {
		return err
	}
	m.mergedPRs = append(m.mergedPRs, prNum)
	return nil
}

func (m *mockGitHubOps) ReplaceLabel(_ context.Context, issueNumber int, oldLabel, newLabel string) error {
	m.replaceLabelCalls = append(m.replaceLabelCalls, replaceLabelCall{
		issueNumber: issueNumber,
		oldLabel:    oldLabel,
		newLabel:    newLabel,
	})

	if remaining := m.replaceLabelFails[newLabel]; remaining > 0 {
		m.replaceLabelFails[newLabel] = remaining - 1
		return fmt.Errorf("replace label %q temporary failed", newLabel)
	}
	if err, ok := m.replaceLabelError[newLabel]; ok {
		return err
	}
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

func TestBuildEscalationMetadata_FirstWrite(t *testing.T) {
	outcome := MergeOutcome{
		Status:          MergeStatusEscalated,
		ExecutedAt:      "2026-02-17T10:00:00Z",
		ExecutorVersion: "integration-merge-executor/v1",
		Conflict: &ConflictSummary{
			Files:           []string{"docs/b.md", "docs/a.md"},
			TotalHunkCount:  3,
			Reason:          "核心文件语义冲突",
			SuggestedAction: "请人工处理",
		},
	}

	update := buildEscalationMetadata(nil, outcome)
	assert.Equal(t, string(MergeStatusEscalated), update[metaKeyIntegrationMergeStatus])
	assert.Equal(t, "integration-merge-executor/v1", update[metaKeyIntegrationExecutorVersion])
	assert.Equal(t, "2026-02-17T10:00:00Z", update[metaKeyIntegrationMergeExecutedAt])
	assert.Equal(t, "docs/a.md,docs/b.md", update[metaKeyIntegrationConflictFiles])
	assert.Equal(t, "3", update[metaKeyIntegrationConflictTotalHunks])
	assert.NotEmpty(t, update[metaKeyIntegrationConflictSummary])
	assert.NotEmpty(t, update[metaKeyIntegrationConflictRecordedAt])
}

func TestBuildEscalationMetadata_IdempotentRetry(t *testing.T) {
	outcome := MergeOutcome{
		Status:          MergeStatusEscalated,
		ExecutedAt:      "2026-02-17T10:00:00Z",
		ExecutorVersion: "integration-merge-executor/v1",
		Conflict: &ConflictSummary{
			Files:           []string{"docs/a.md"},
			TotalHunkCount:  1,
			Reason:          "复杂冲突",
			SuggestedAction: "人工处理",
		},
	}

	first := buildEscalationMetadata(nil, outcome)
	existing := make(map[string]string, len(first))
	for k, v := range first {
		existing[k] = v
	}

	second := buildEscalationMetadata(existing, outcome)
	assert.Empty(t, second)
}

func TestController_EscalateIntegrationConflict_LabelIdempotentRetry(t *testing.T) {
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   "/bin/true",
			StorePath: t.TempDir() + "/tasks.json",
		},
	}

	outcome := MergeOutcome{
		Status:          MergeStatusEscalated,
		SourceBranch:    "feat/41-core-b",
		ExecutedAt:      "2026-02-17T10:00:00Z",
		ExecutorVersion: "integration-merge-executor/v1",
		Conflict: &ConflictSummary{
			Files:           []string{"pkg/service.go"},
			TotalHunkCount:  1,
			Reason:          "复杂冲突",
			SuggestedAction: "人工处理",
		},
	}

	firstTask := Task{
		ID:       "task-1",
		Metadata: map[string]string{"issue_num": "41"},
	}
	ctrl.escalateIntegrationConflict(context.Background(), firstTask, outcome)
	require.Len(t, mockGH.replaceLabelCalls, 2)
	assert.Equal(t, integrationConflictLabel, mockGH.replaceLabelCalls[0].newLabel)
	assert.Equal(t, needsHumanLabel, mockGH.replaceLabelCalls[1].newLabel)

	retryTask := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                           "41",
			metaKeyIntegrationMergeStatus:         string(MergeStatusEscalated),
			metaKeyIntegrationMergeExecutedAt:     outcome.ExecutedAt,
			metaKeyIntegrationExecutorVersion:     outcome.ExecutorVersion,
			metaKeyIntegrationConflictRecordedAt:  "2026-02-17T10:00:01Z",
			metaKeyIntegrationConflictLabelSynced: "true",
		},
	}
	ctrl.escalateIntegrationConflict(context.Background(), retryTask, outcome)
	assert.Len(t, mockGH.replaceLabelCalls, 2)
}

func TestShouldRetryEscalationLabels(t *testing.T) {
	assert.True(t, shouldRetryEscalationLabels(map[string]string{
		metaKeyIntegrationMergeStatus: string(MergeStatusEscalated),
	}))
	assert.False(t, shouldRetryEscalationLabels(map[string]string{
		metaKeyIntegrationMergeStatus:         string(MergeStatusEscalated),
		metaKeyIntegrationConflictLabelSynced: "true",
	}))
	assert.False(t, shouldRetryEscalationLabels(map[string]string{
		metaKeyIntegrationMergeStatus: string(MergeStatusMerged),
	}))
}

func TestMergeOutcomeFromEscalatedTask_FromMetadata(t *testing.T) {
	task := Task{
		Metadata: map[string]string{
			"issue_num":                           "41",
			"pr_num":                              "410",
			"branch":                              "feat/41-core",
			metaKeyIntegrationMergeStatus:         string(MergeStatusEscalated),
			metaKeyIntegrationMergeExecutedAt:     "2026-02-17T10:00:00Z",
			metaKeyIntegrationExecutorVersion:     "integration-merge-executor/v1",
			metaKeyIntegrationConflictSummary:     `{"files":["pkg/service.go"],"total_hunk_count":2,"reason":"复杂冲突","suggested_action":"人工处理"}`,
			metaKeyIntegrationConflictLabelSynced: "false",
		},
	}

	outcome := mergeOutcomeFromEscalatedTask(task, "integration/main")
	assert.Equal(t, MergeStatusEscalated, outcome.Status)
	assert.Equal(t, "integration/main", outcome.IntegrationBranch)
	assert.Equal(t, "feat/41-core", outcome.SourceBranch)
	assert.Equal(t, 41, outcome.IssueNum)
	assert.Equal(t, 410, outcome.PRNum)
	require.NotNil(t, outcome.Conflict)
	assert.Equal(t, []string{"pkg/service.go"}, outcome.Conflict.Files)
	assert.Equal(t, 2, outcome.Conflict.TotalHunkCount)
	assert.Equal(t, "复杂冲突", outcome.Conflict.Reason)
}

func TestController_EscalateIntegrationConflict_LabelCanRetryAfterFailure(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.replaceLabelFails[integrationConflictLabel] = 1
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   "/bin/true",
			StorePath: t.TempDir() + "/tasks.json",
		},
	}

	task := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                   "41",
			metaKeyIntegrationMergeStatus: string(MergeStatusEscalated),
		},
	}
	outcome := MergeOutcome{
		Status:       MergeStatusEscalated,
		SourceBranch: "feat/41-core-b",
		Conflict: &ConflictSummary{
			Files:           []string{"pkg/service.go"},
			TotalHunkCount:  1,
			Reason:          "复杂冲突",
			SuggestedAction: "人工处理",
		},
	}

	ctrl.escalateIntegrationConflict(context.Background(), task, outcome)
	require.Len(t, mockGH.replaceLabelCalls, 1)
	assert.Equal(t, integrationConflictLabel, mockGH.replaceLabelCalls[0].newLabel)

	ctrl.escalateIntegrationConflict(context.Background(), task, outcome)
	require.Len(t, mockGH.replaceLabelCalls, 3)
	assert.Equal(t, integrationConflictLabel, mockGH.replaceLabelCalls[1].newLabel)
	assert.Equal(t, needsHumanLabel, mockGH.replaceLabelCalls[2].newLabel)
}

func TestHandleIntegrationGateFailure_TriggersRetryLabelUnderLimit(t *testing.T) {
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   "/bin/true",
			StorePath: t.TempDir() + "/tasks.json",
		},
		cfg: &ControlConfig{
			IntegrationGateMaxRetries: 2,
			RepoDir:                   t.TempDir(),
		},
	}

	task := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                         "41",
			metaKeyIntegrationGateRetryCount:    "0",
			metaKeyIntegrationGateAttemptKey:    "old",
			metaKeyIntegrationGateStatus:        integrationGateStatusPassed,
			metaKeyIntegrationGateLastError:     "",
			metaKeyIntegrationGateLastCheckedAt: "2026-02-17T10:00:00Z",
		},
	}

	err := ctrl.handleIntegrationGateFailure(context.Background(), task, "attempt-1", errors.New("gate failed once"))
	require.NoError(t, err)

	assert.Greater(t, countReplaceLabelByTarget(mockGH.replaceLabelCalls, "bot:pr-needs-fix"), 0)
	assert.Equal(t, 0, countReplaceLabelByTarget(mockGH.replaceLabelCalls, integrationGateFailLabel))
}

func TestHandleIntegrationGateFailure_EscalatesWhenExceeded(t *testing.T) {
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   "/bin/true",
			StorePath: t.TempDir() + "/tasks.json",
		},
		cfg: &ControlConfig{
			IntegrationGateMaxRetries: 2,
			RepoDir:                   t.TempDir(),
		},
	}

	task := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                      "41",
			metaKeyIntegrationGateRetryCount: "2",
			metaKeyIntegrationGateAttemptKey: "old-attempt",
			metaKeyIntegrationGateStatus:     integrationGateStatusRetrying,
		},
	}

	err := ctrl.handleIntegrationGateFailure(context.Background(), task, "attempt-3", errors.New("gate failed third time"))
	require.NoError(t, err)

	assert.Greater(t, countReplaceLabelByTarget(mockGH.replaceLabelCalls, needsHumanLabel), 0)
	assert.Greater(t, countReplaceLabelByTarget(mockGH.replaceLabelCalls, integrationGateFailLabel), 0)
}

func TestHandleIntegrationGateFailure_DuplicateAttemptNoop(t *testing.T) {
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   "/bin/true",
			StorePath: t.TempDir() + "/tasks.json",
		},
		cfg: &ControlConfig{
			IntegrationGateMaxRetries: 2,
			RepoDir:                   t.TempDir(),
		},
	}

	task := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                      "41",
			metaKeyIntegrationGateRetryCount: "1",
			metaKeyIntegrationGateAttemptKey: "attempt-same",
			metaKeyIntegrationGateStatus:     integrationGateStatusRetrying,
		},
	}

	err := ctrl.handleIntegrationGateFailure(context.Background(), task, "attempt-same", errors.New("duplicate fail"))
	require.NoError(t, err)
	assert.Len(t, mockGH.replaceLabelCalls, 0)
}

func TestBuildIntegrationGateAttemptKey_IgnoresIntegrationHeadChanges(t *testing.T) {
	dir := setupGitRepo(t)
	createBranch(t, dir, "feat/41-gate", "feature.txt", "feature v1\n")

	runGit(t, dir, "checkout", "master")
	runGit(t, dir, "checkout", "-b", "integration/main")
	runGit(t, dir, "merge", "--no-ff", "feat/41-gate", "-m", "merge feat/41-gate")
	runGit(t, dir, "checkout", "master")

	ctrl := &Controller{
		cfg: &ControlConfig{
			RepoDir: dir,
		},
	}
	outcome := MergeOutcome{
		IntegrationBranch: "integration/main",
		SourceBranch:      "feat/41-gate",
	}
	firstKey, err := ctrl.buildIntegrationGateAttemptKey(outcome)
	require.NoError(t, err)

	createBranch(t, dir, "feat/99-other", "unrelated.txt", "other change\n")
	runGit(t, dir, "checkout", "integration/main")
	runGit(t, dir, "merge", "--no-ff", "feat/99-other", "-m", "merge feat/99-other")
	runGit(t, dir, "checkout", "master")

	secondKey, err := ctrl.buildIntegrationGateAttemptKey(outcome)
	require.NoError(t, err)
	assert.Equal(t, firstKey, secondKey)
}

func TestRunIntegrationGateAndDecide_FirstFailThenPass_MarksIntegrated(t *testing.T) {
	dir := setupGitRepo(t)

	runGit(t, dir, "checkout", "-b", "feat/41-gate")
	niumaDir := filepath.Join(dir, "automation", "niuma")
	require.NoError(t, os.MkdirAll(filepath.Join(niumaDir, "gate"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(niumaDir, "go.mod"), []byte("module example.com/niuma\n\ngo 1.20\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(niumaDir, "gate", "gate_test.go"), []byte(`package gate

import "testing"

func TestIntegrationGate(t *testing.T) {
	t.Fatalf("gate failed")
}
`), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add failing integration gate test")

	runGit(t, dir, "checkout", "-b", "integration/main")
	runGit(t, dir, "checkout", "master")

	binDir := t.TempDir()
	logPath := filepath.Join(binDir, "taskctl.log")
	binPath := filepath.Join(binDir, "taskctl")
	script := fmt.Sprintf("#!/usr/bin/env bash\nprintf '%%s\\n' \"$*\" >> %q\nexit 0\n", logPath)
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   binPath,
			StorePath: filepath.Join(dir, ".niuma", "tasks.json"),
		},
		cfg: &ControlConfig{
			IntegrationGateMaxRetries: 2,
			RepoDir:                   dir,
		},
	}

	outcome := MergeOutcome{
		Status:            MergeStatusMerged,
		IntegrationBranch: "integration/main",
		SourceBranch:      "feat/41-gate",
		IssueNum:          41,
		PRNum:             410,
		ExecutorVersion:   "integration-merge-executor/v1",
		ExecutedAt:        "2026-02-17T10:00:00Z",
	}

	firstTask := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num": "41",
		},
	}
	integrated, err := ctrl.runIntegrationGateAndDecide(context.Background(), firstTask, outcome)
	require.NoError(t, err)
	assert.False(t, integrated)
	assert.Greater(t, countReplaceLabelByTarget(mockGH.replaceLabelCalls, "bot:pr-needs-fix"), 0)

	firstAttemptKey, err := ctrl.buildIntegrationGateAttemptKey(outcome)
	require.NoError(t, err)

	runGit(t, dir, "checkout", "feat/41-gate")
	require.NoError(t, os.WriteFile(filepath.Join(niumaDir, "gate", "gate_test.go"), []byte(`package gate

import "testing"

func TestIntegrationGate(t *testing.T) {}
`), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "fix integration gate test")
	runGit(t, dir, "checkout", "integration/main")
	runGit(t, dir, "merge", "--ff-only", "feat/41-gate")
	runGit(t, dir, "checkout", "master")

	secondAttemptKey, err := ctrl.buildIntegrationGateAttemptKey(outcome)
	require.NoError(t, err)
	assert.NotEqual(t, firstAttemptKey, secondAttemptKey)

	retryTask := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                      "41",
			metaKeyIntegrationGateStatus:     integrationGateStatusRetrying,
			metaKeyIntegrationGateRetryCount: "1",
			metaKeyIntegrationGateAttemptKey: firstAttemptKey,
		},
	}
	integrated, err = ctrl.runIntegrationGateAndDecide(context.Background(), retryTask, outcome)
	require.NoError(t, err)
	assert.True(t, integrated)

	logContent, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(string(logContent)), `"integrated":"true"`)
}

func TestCollectAutomationIssues_IncludesQueuedIssues(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.issuesByLabel["bot:queued"] = []IssueInfo{
		{Number: 214, Title: "queued task", State: "open", Labels: []string{"bot:queued"}},
	}
	mockGH.issuesByLabel["bot:fix"] = []IssueInfo{
		{Number: 215, Title: "in progress task", State: "open", Labels: []string{"bot:fix"}},
	}

	ctrl := &Controller{github: mockGH}
	issues, orchestrateCount, err := ctrl.collectAutomationIssues(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, orchestrateCount)
	assert.Len(t, issues, 2)
}

func TestFinalizeIntegratedIssues_ClosesSubAndParent(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{Number: 210, Title: "parent", State: "open"},
		IssueInfo{Number: 214, Title: "sub-214", Body: "parent: #210", State: "open", Labels: []string{"bot:pr-reviewable"}},
		IssueInfo{Number: 215, Title: "sub-215", Body: "parent: #210", State: "closed"},
	)

	ctrl := &Controller{github: mockGH}
	err := ctrl.FinalizeIntegratedIssues(context.Background(), []int{214})
	require.NoError(t, err)

	assert.Contains(t, mockGH.closeIssueCalls, 214)
	assert.Contains(t, mockGH.closeIssueCalls, 210)
	assert.Greater(t, countReplaceLabelByIssueAndTarget(mockGH.replaceLabelCalls, 214, botDoneLabel), 0)
	assert.Greater(t, countReplaceLabelByIssueAndTarget(mockGH.replaceLabelCalls, 210, botDoneLabel), 0)
}

func TestFinalizeIntegratedIssues_ParentRemainsOpenWhenSubNotAllClosed(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{Number: 210, Title: "parent", State: "open"},
		IssueInfo{Number: 214, Title: "sub-214", Body: "parent: #210", State: "open"},
		IssueInfo{Number: 215, Title: "sub-215", Body: "parent: #210", State: "open"},
	)

	ctrl := &Controller{github: mockGH}
	err := ctrl.FinalizeIntegratedIssues(context.Background(), []int{214})
	require.NoError(t, err)

	assert.Contains(t, mockGH.closeIssueCalls, 214)
	assert.NotContains(t, mockGH.closeIssueCalls, 210)
}

func TestFinalizeIntegratedIssues_ClosedIssueIsIdempotent(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{Number: 210, Title: "parent", State: "open"},
		IssueInfo{Number: 214, Title: "sub-214", Body: "parent: #210", State: "closed"},
		IssueInfo{Number: 215, Title: "sub-215", Body: "parent: #210", State: "open"},
	)

	ctrl := &Controller{github: mockGH}
	err := ctrl.FinalizeIntegratedIssues(context.Background(), []int{214})
	require.NoError(t, err)

	assert.NotContains(t, mockGH.closeIssueCalls, 214)
	assert.NotContains(t, mockGH.closeIssueCalls, 210)
}

func TestFinalizeIntegratedIssues_SkipDoneLabelWhenAlreadyPresent(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{Number: 214, Title: "sub-214", State: "open", Labels: []string{botDoneLabel}},
	)

	ctrl := &Controller{github: mockGH}
	err := ctrl.FinalizeIntegratedIssues(context.Background(), []int{214})
	require.NoError(t, err)

	assert.Contains(t, mockGH.closeIssueCalls, 214)
	assert.Equal(t, 0, countReplaceLabelByIssueAndTarget(mockGH.replaceLabelCalls, 214, botDoneLabel))
}

func TestSelectClosableIssueNums_FiltersCompletedAndIntegrated(t *testing.T) {
	ctrl := &Controller{}
	tasks := []Task{
		{
			Status: TaskStatusCompleted,
			Metadata: map[string]string{
				"issue_num":       "214",
				"meta_issue_slug": "214",
				metaKeyIntegrated: "true",
			},
		},
		{
			Status: TaskStatusCompleted,
			Metadata: map[string]string{
				"issue_num":       "215",
				"meta_issue_slug": "214",
				metaKeyIntegrated: "false",
			},
		},
		{
			Status: TaskStatusInProgress,
			Metadata: map[string]string{
				"issue_num":       "216",
				"meta_issue_slug": "214",
				metaKeyIntegrated: "true",
			},
		},
		{
			Status: TaskStatusCompleted,
			Metadata: map[string]string{
				"issue_num":       "217",
				metaKeyIntegrated: "true",
			},
		},
	}

	filtered, branchBuckets, skipped := ctrl.selectClosableIssueNums(tasks, []int{214, 215, 216, 217, 218})
	assert.Equal(t, []int{214, 217}, filtered)
	assert.Equal(t, []int{214}, branchBuckets["integration/214"])
	assert.Equal(t, []int{217}, branchBuckets["integration/main"])
	assert.Equal(t, "task 尚未 integrated=true", skipped[215])
	assert.Equal(t, "task 状态为 in-progress", skipped[216])
	assert.Equal(t, "未找到关联 task", skipped[218])
}

func countReplaceLabelByTarget(calls []replaceLabelCall, label string) int {
	count := 0
	for _, call := range calls {
		if call.newLabel == label {
			count++
		}
	}
	return count
}

func countReplaceLabelByIssueAndTarget(calls []replaceLabelCall, issueNum int, label string) int {
	count := 0
	for _, call := range calls {
		if call.issueNumber == issueNum && call.newLabel == label {
			count++
		}
	}
	return count
}
