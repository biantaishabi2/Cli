package control

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGitHubOps 用于 controller 测试的 GitHub mock
type mockGitHubOps struct {
	issues                []IssueInfo
	issuesByLabel         map[string][]IssueInfo
	issuesByNumber        map[int]IssueInfo
	mergedPRs             []int
	mergeError            map[int]error
	replaceLabelCalls     []replaceLabelCall
	replaceLabelError     map[string]error
	replaceLabelPairError map[string]error
	replaceLabelFails     map[string]int
	closeIssueCalls       []int
	closeIssueError       map[int]error
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
		issues:                issues,
		issuesByLabel:         make(map[string][]IssueInfo),
		issuesByNumber:        issuesByNumber,
		mergeError:            make(map[int]error),
		replaceLabelError:     make(map[string]error),
		replaceLabelPairError: make(map[string]error),
		replaceLabelFails:     make(map[string]int),
		closeIssueError:       make(map[int]error),
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
	if err, ok := m.replaceLabelPairError[fmt.Sprintf("%s=>%s", oldLabel, newLabel)]; ok {
		return err
	}

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
	tasks                  []Task
	nextID                 int
	readyList              []Task
	readyCallCount         int
	readyHook              func([]Task) error
	failBlockedByForTaskID map[string]error
}

func newMockTaskCtlClient() *mockTaskCtlClient {
	return &mockTaskCtlClient{
		failBlockedByForTaskID: make(map[string]error),
	}
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
	if opts.BlockedBy != nil {
		if err, ok := m.failBlockedByForTaskID[taskID]; ok {
			return err
		}
	}
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
	m.readyCallCount++
	if m.readyHook != nil {
		if err := m.readyHook(m.tasks); err != nil {
			return nil, err
		}
	}
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
	mockAI      *ai.MockProvider
}

type heartbeatFailIssueLockStore struct {
	base              IssueLockStore
	mu                sync.Mutex
	refreshCount      int
	lastReleaseResult IssueLockResult
}

func newHeartbeatFailIssueLockStore() *heartbeatFailIssueLockStore {
	return &heartbeatFailIssueLockStore{
		base: newInMemoryIssueLockStore(),
	}
}

func (s *heartbeatFailIssueLockStore) TryAcquire(issueNumber int, owner string, now time.Time, ttl time.Duration) (IssueLockRecord, bool, error) {
	return s.base.TryAcquire(issueNumber, owner, now, ttl)
}

func (s *heartbeatFailIssueLockStore) Refresh(issueNumber int, owner string, now time.Time, ttl time.Duration) (IssueLockRecord, error) {
	s.mu.Lock()
	s.refreshCount++
	s.mu.Unlock()
	return IssueLockRecord{}, fmt.Errorf("inject refresh failure")
}

func (s *heartbeatFailIssueLockStore) Release(issueNumber int, owner string, now time.Time, lastResult IssueLockResult) error {
	s.mu.Lock()
	s.lastReleaseResult = lastResult
	s.mu.Unlock()
	return s.base.Release(issueNumber, owner, now, lastResult)
}

func (s *heartbeatFailIssueLockStore) RefreshCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshCount
}

func (s *heartbeatFailIssueLockStore) LastReleaseResult() IssueLockResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastReleaseResult
}

func newInMemController(issues []IssueInfo, aiResp string) *inMemController {
	mockTC := newMockTaskCtlClient()
	mockGH := newMockGitHubOps(issues...)

	var provider ai.Provider
	var mockAI *ai.MockProvider
	if aiResp != "" {
		mockAI = ai.NewMockProvider(aiResp)
		provider = mockAI
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
		mockAI:      mockAI,
	}
}

// RunInMem 模拟 Run 逻辑但使用内存 mock
func (c *inMemController) RunInMem(ctx context.Context) error {
	issues, _, err := c.collectAutomationIssues(ctx)
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

	// 分析依赖：显式 depends-on 优先，AI 仅补全未声明依赖。
	analysis := c.buildDependencyAnalysis(ctx, issues)

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

	// 先落盘 blocked_by，再决定是否推进 ready。
	blockedByPersisted := true
	for issueNum, deps := range analysis.Dependencies {
		taskID, ok := issueToTask[issueNum]
		if !ok {
			blockedByPersisted = false
			continue
		}
		var blockedBy []string
		for _, dep := range deps {
			depTaskID, ok := issueToTask[dep]
			if !ok {
				blockedByPersisted = false
				continue
			}
			blockedBy = append(blockedBy, depTaskID)
		}
		if len(blockedBy) > 0 {
			if err := c.mockTaskCtl.update(taskID, UpdateOpts{BlockedBy: &blockedBy}); err != nil {
				blockedByPersisted = false
			}
		} else if len(deps) > 0 {
			blockedByPersisted = false
		}
	}

	// 推进 ready tasks
	if !blockedByPersisted {
		return nil
	}
	readyTasks, err := c.mockTaskCtl.ready()
	if err != nil {
		return err
	}
	for _, task := range readyTasks {
		status := TaskStatusInProgress
		if err := c.mockTaskCtl.update(task.ID, UpdateOpts{Status: &status}); err != nil {
			return err
		}
	}

	return nil
}

func newIssueLockTestController(owner string, store IssueLockStore, ttl, heartbeat time.Duration, nowFn func() time.Time) *Controller {
	if store == nil {
		store = newInMemoryIssueLockStore()
	}
	if owner == "" {
		owner = "test-owner"
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if heartbeat <= 0 {
		heartbeat = 100 * time.Second
	}
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Controller{
		issueLocks:         store,
		issueLockTTL:       ttl,
		issueLockHeartbeat: heartbeat,
		nowFn:              nowFn,
		ownerID:            owner,
	}
}

func TestController_WithIssueLock_MutualExclusion(t *testing.T) {
	store := newInMemoryIssueLockStore()
	ctrl := newIssueLockTestController("owner-a", store, 5*time.Minute, 100*time.Second, time.Now)
	issue := IssueInfo{Number: 315}

	started := make(chan struct{})
	done := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- ctrl.withIssueLock(context.Background(), issue, func(context.Context) error {
			close(started)
			<-done
			return nil
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("等待首个持锁流程超时")
	}

	secondExecuted := false
	err := ctrl.withIssueLock(context.Background(), issue, func(context.Context) error {
		secondExecuted = true
		return nil
	})
	require.NoError(t, err)
	assert.False(t, secondExecuted)

	close(done)
	require.NoError(t, <-errCh)
}

func TestController_WithIssueLock_ReleasesAfterCompletion(t *testing.T) {
	ctrl := newIssueLockTestController("owner-a", nil, 5*time.Minute, 100*time.Second, time.Now)
	issue := IssueInfo{Number: 315}

	runCount := 0
	err := ctrl.withIssueLock(context.Background(), issue, func(context.Context) error {
		runCount++
		return nil
	})
	require.NoError(t, err)

	err = ctrl.withIssueLock(context.Background(), issue, func(context.Context) error {
		runCount++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, runCount)
}

func TestController_IssueLock_TTLExpiryRecovery(t *testing.T) {
	store := newInMemoryIssueLockStore()
	ttl := 5 * time.Minute

	var mu sync.Mutex
	now := time.Date(2026, 2, 18, 0, 0, 0, 0, time.UTC)
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	advance := func(d time.Duration) {
		mu.Lock()
		now = now.Add(d)
		mu.Unlock()
	}

	ctrlA := newIssueLockTestController("owner-a", store, ttl, 100*time.Second, nowFn)
	ctrlB := newIssueLockTestController("owner-b", store, ttl, 100*time.Second, nowFn)
	issue := IssueInfo{Number: 315}

	acquired, _, err := ctrlA.tryAcquireIssueLock(issue)
	require.NoError(t, err)
	require.True(t, acquired)

	acquired, _, err = ctrlB.tryAcquireIssueLock(issue)
	require.NoError(t, err)
	assert.False(t, acquired)

	advance(ttl + time.Second)

	acquired, _, err = ctrlB.tryAcquireIssueLock(issue)
	require.NoError(t, err)
	assert.True(t, acquired)
}

func TestController_WithIssueLock_HeartbeatRefreshFailureReturnsError(t *testing.T) {
	store := newHeartbeatFailIssueLockStore()
	ctrl := newIssueLockTestController("owner-a", store, 5*time.Minute, 10*time.Millisecond, time.Now)
	issue := IssueInfo{Number: 315}

	err := ctrl.withIssueLock(context.Background(), issue, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "锁心跳刷新失败")
	assert.GreaterOrEqual(t, store.RefreshCount(), 1)
	assert.Equal(t, IssueLockResultFailed, store.LastReleaseResult())
}

func TestInMemoryIssueLockStore_ValidationAndOwnerMismatch(t *testing.T) {
	now := time.Date(2026, 2, 18, 0, 0, 0, 0, time.UTC)
	store := newInMemoryIssueLockStore()

	_, _, err := store.TryAcquire(0, "owner-a", now, time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "issue 编号无效")

	_, _, err = store.TryAcquire(1, "", now, time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "owner 不能为空")

	_, _, err = store.TryAcquire(1, "owner-a", now, 0)
	require.Error(t, err)
	assert.ErrorContains(t, err, "ttl 必须大于 0")

	_, err = store.Refresh(0, "owner-a", now, time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "issue 编号无效")

	_, err = store.Refresh(1, "", now, time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "owner 不能为空")

	_, err = store.Refresh(1, "owner-a", now, 0)
	require.Error(t, err)
	assert.ErrorContains(t, err, "ttl 必须大于 0")

	_, err = store.Refresh(1, "owner-a", now, time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "未持锁")

	_, acquired, err := store.TryAcquire(1, "owner-a", now, time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)

	_, err = store.Refresh(1, "owner-b", now.Add(10*time.Second), time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "owner 不匹配")

	err = store.Release(1, "", now, IssueLockResultFailed)
	require.Error(t, err)
	assert.ErrorContains(t, err, "owner 不能为空")

	err = store.Release(1, "owner-b", now, IssueLockResultFailed)
	require.Error(t, err)
	assert.ErrorContains(t, err, "owner 不匹配")
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

func TestController_ManualDependsOnWinsAndAIFillsUndeclared(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Payment", Body: "fix payment"},
		{Number: 42, Title: "Tests", Body: "depends-on: #40"},
	}
	aiResp := `{"dependencies":{"41":[40],"42":[41]},"potential_conflicts":[]}`

	ctrl := newInMemController(issues, aiResp)
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	taskByIssue := make(map[int]Task)
	taskIDByIssue := make(map[int]string)
	for _, task := range ctrl.mockTaskCtl.tasks {
		taskByIssue[task.IssueNum()] = task
		taskIDByIssue[task.IssueNum()] = task.ID
	}

	require.Contains(t, taskByIssue, 42)
	assert.Equal(t, []string{taskIDByIssue[40]}, taskByIssue[42].BlockedBy)
	assert.Equal(t, TaskStatusPending, taskByIssue[42].Status)
	assert.Equal(t, []string{taskIDByIssue[40]}, taskByIssue[41].BlockedBy)
}

func TestController_SubIssueWithParentOnlyUsesUnifiedAnalyzePath(t *testing.T) {
	issues := []IssueInfo{
		{Number: 210, Title: "parent", Body: "parent root"},
		{Number: 214, Title: "sub", Body: "parent: #210"},
	}
	aiResp := `{"dependencies":{"214":[210]},"potential_conflicts":[]}`

	ctrl := newInMemController(issues, aiResp)
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	require.NotNil(t, ctrl.mockAI)
	assert.Equal(t, 1, ctrl.mockAI.CallCount())
	assert.Contains(t, ctrl.mockAI.LastPrompt(), "#214")

	taskByIssue := make(map[int]Task)
	taskIDByIssue := make(map[int]string)
	for _, task := range ctrl.mockTaskCtl.tasks {
		taskByIssue[task.IssueNum()] = task
		taskIDByIssue[task.IssueNum()] = task.ID
	}

	assert.Equal(t, TaskStatusInProgress, taskByIssue[210].Status)
	assert.Equal(t, TaskStatusPending, taskByIssue[214].Status)
	assert.Equal(t, []string{taskIDByIssue[210]}, taskByIssue[214].BlockedBy)
}

func TestController_SubIssueParentDoesNotOverrideDependsOn(t *testing.T) {
	issues := []IssueInfo{
		{Number: 210, Title: "parent", Body: "parent root"},
		{Number: 300, Title: "base", Body: "base impl"},
		{Number: 214, Title: "sub", Body: "parent: #210\ndepends-on: #300"},
	}
	aiResp := `{"dependencies":{"214":[210]},"potential_conflicts":[]}`

	ctrl := newInMemController(issues, aiResp)
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	taskByIssue := make(map[int]Task)
	taskIDByIssue := make(map[int]string)
	for _, task := range ctrl.mockTaskCtl.tasks {
		taskByIssue[task.IssueNum()] = task
		taskIDByIssue[task.IssueNum()] = task.ID
	}

	assert.Equal(t, []string{taskIDByIssue[300]}, taskByIssue[214].BlockedBy)
}

func TestController_ReadyWaitsForBlockedByPersistence(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Tests", Body: "depends-on: #40"},
	}

	ctrl := newInMemController(issues, "")
	ctrl.mockTaskCtl.readyHook = func(tasks []Task) error {
		for _, task := range tasks {
			if task.IssueNum() == 41 && len(task.BlockedBy) == 0 {
				return fmt.Errorf("ready called before blocked_by persisted for issue #41")
			}
		}
		return nil
	}

	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, ctrl.mockTaskCtl.readyCallCount)
}

func TestController_SkipReadyWhenBlockedByPersistFails(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Tests", Body: "depends-on: #40"},
	}

	ctrl := newInMemController(issues, "")
	ctrl.mockTaskCtl.failBlockedByForTaskID["task-2"] = fmt.Errorf("write blocked_by failed")

	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, ctrl.mockTaskCtl.readyCallCount)
	for _, task := range ctrl.mockTaskCtl.tasks {
		assert.Equal(t, TaskStatusPending, task.Status)
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

func TestRunIntegrationGateAndDecide_FirstFailThenPass_MarksCompletedBeforeIntegrated(t *testing.T) {
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
	firstLogContent, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.NotContains(t, string(firstLogContent), "--status completed")

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
		ID:     "task-1",
		Status: TaskStatusInProgress,
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
	assert.Contains(t, string(logContent), "--status completed")

	logLines := splitNonEmpty(string(logContent))
	completedIdx := findLineIndexContaining(logLines, "--status completed")
	integratedIdx := findLineIndexContaining(logLines, `"integrated":"true"`)
	require.GreaterOrEqual(t, completedIdx, 0)
	require.GreaterOrEqual(t, integratedIdx, 0)
	assert.Less(t, completedIdx, integratedIdx)
}

func TestRunIntegrationGateAndDecide_CompletedTaskOnlyBackfillsIntegrated(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(binDir, "taskctl.log")
	binPath := filepath.Join(binDir, "taskctl")
	script := fmt.Sprintf("#!/usr/bin/env bash\nprintf '%%s\\n' \"$*\" >> %q\nexit 0\n", logPath)
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	ctrl := &Controller{
		taskctl: &TaskCtlClient{
			BinPath:   binPath,
			StorePath: filepath.Join(binDir, "tasks.json"),
		},
	}

	task := Task{
		ID:     "task-1",
		Status: TaskStatusCompleted,
		Metadata: map[string]string{
			"issue_num": "41",
		},
	}

	integrated, err := ctrl.runIntegrationGateAndDecide(context.Background(), task, MergeOutcome{})
	require.NoError(t, err)
	assert.True(t, integrated)

	logContent, err := os.ReadFile(logPath)
	require.NoError(t, err)
	logText := string(logContent)
	assert.Contains(t, strings.ToLower(logText), `"integrated":"true"`)
	assert.NotContains(t, logText, "--status completed")
	assert.NotContains(t, strings.ToLower(logText), "integration_gate_status")
}

func TestShouldBackfillIntegratedMetadata(t *testing.T) {
	assert.True(t, shouldBackfillIntegratedMetadata(Task{
		Status:   TaskStatusCompleted,
		Metadata: map[string]string{"issue_num": "41"},
	}))
	assert.False(t, shouldBackfillIntegratedMetadata(Task{
		Status:   TaskStatusCompleted,
		Metadata: map[string]string{"issue_num": "41", metaKeyIntegrated: "true"},
	}))
	assert.False(t, shouldBackfillIntegratedMetadata(Task{
		Status:   TaskStatusInProgress,
		Metadata: map[string]string{"issue_num": "41"},
	}))
}

func TestRunIntegrationGateAndDecide_EscalatedDoesNotMarkCompleted(t *testing.T) {
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
			IntegrationGateMaxRetries: 0,
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

	task := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num": "41",
		},
	}
	integrated, err := ctrl.runIntegrationGateAndDecide(context.Background(), task, outcome)
	require.NoError(t, err)
	assert.False(t, integrated)
	assert.Greater(t, countReplaceLabelByTarget(mockGH.replaceLabelCalls, needsHumanLabel), 0)

	logContent, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.NotContains(t, string(logContent), "--status completed")
	assert.NotContains(t, strings.ToLower(string(logContent)), `"integrated":"true"`)
}

func TestRunIntegrationGateAndDecide_CompletedUpdateFailureDoesNotWriteIntegrated(t *testing.T) {
	dir := setupGitRepo(t)

	runGit(t, dir, "checkout", "-b", "feat/41-gate")
	niumaDir := filepath.Join(dir, "automation", "niuma")
	require.NoError(t, os.MkdirAll(filepath.Join(niumaDir, "gate"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(niumaDir, "go.mod"), []byte("module example.com/niuma\n\ngo 1.20\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(niumaDir, "gate", "gate_test.go"), []byte(`package gate

import "testing"

func TestIntegrationGate(t *testing.T) {}
`), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add passing integration gate test")

	runGit(t, dir, "checkout", "-b", "integration/main")
	runGit(t, dir, "checkout", "master")

	binDir := t.TempDir()
	logPath := filepath.Join(binDir, "taskctl.log")
	binPath := filepath.Join(binDir, "taskctl")
	script := fmt.Sprintf(`#!/usr/bin/env bash
printf '%%s\n' "$*" >> %q
if [[ "$*" == *"--status completed"* ]]; then
  echo "forced completed update failure" >&2
  exit 1
fi
exit 0
`, logPath)
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

	task := Task{
		ID:     "task-1",
		Status: TaskStatusInProgress,
		Metadata: map[string]string{
			"issue_num": "41",
		},
	}
	integrated, err := ctrl.runIntegrationGateAndDecide(context.Background(), task, outcome)
	require.Error(t, err)
	assert.False(t, integrated)
	assert.Contains(t, err.Error(), "更新任务 task-1 失败")

	logContent, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(logContent), "--status completed")
	assert.NotContains(t, strings.ToLower(string(logContent)), `"integrated":"true"`)
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

func TestSyncIssueStateLabel_SkipsSelfReplacement(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.replaceLabelPairError["bot:fix=>bot:fix"] = errors.New("self replacement should not happen")
	ctrl := &Controller{github: mockGH}

	err := ctrl.syncIssueStateLabel(context.Background(), 41, "bot:fix")
	require.NoError(t, err)
	require.NotEmpty(t, mockGH.replaceLabelCalls)
	for _, call := range mockGH.replaceLabelCalls {
		assert.NotEqual(t, call.oldLabel, call.newLabel)
	}
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

func findLineIndexContaining(lines []string, target string) int {
	for idx, line := range lines {
		if strings.Contains(line, target) {
			return idx
		}
	}
	return -1
}
