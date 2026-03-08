package daemon

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockTracker 用于测试的 tracker 实现
type mockTracker struct {
	mu     sync.Mutex
	issues map[string][]Issue         // status → issues
	states map[string]string          // itemID → status
	deps   map[string]map[string]string // 依赖检查结果
}

func newMockTracker() *mockTracker {
	return &mockTracker{
		issues: make(map[string][]Issue),
		states: make(map[string]string),
		deps:   make(map[string]map[string]string),
	}
}

func (m *mockTracker) FetchByStatus(_ context.Context, status string) ([]Issue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// 根据当前 states 过滤
	var result []Issue
	for _, issues := range m.issues {
		for _, issue := range issues {
			if s, ok := m.states[issue.ID]; ok && s == status {
				issue.Status = s
				result = append(result, issue)
			}
		}
	}
	return result, nil
}

func (m *mockTracker) SetStatus(_ context.Context, itemID string, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[itemID] = status
	return nil
}

func (m *mockTracker) GetIssue(_ context.Context, itemID string) (*Issue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, issues := range m.issues {
		for _, issue := range issues {
			if issue.ID == itemID {
				if s, ok := m.states[issue.ID]; ok {
					issue.Status = s
				}
				return &issue, nil
			}
		}
	}
	return nil, fmt.Errorf("issue %s not found", itemID)
}

func (m *mockTracker) CheckDependencies(_ context.Context, ids []string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]string)
	for _, id := range ids {
		if depMap, ok := m.deps["default"]; ok {
			if s, ok := depMap[id]; ok {
				result[id] = s
			}
		}
	}
	return result, nil
}

func (m *mockTracker) addIssue(issue Issue) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.issues[issue.Status] = append(m.issues[issue.Status], issue)
	m.states[issue.ID] = issue.Status
}

func (m *mockTracker) getStatus(itemID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states[itemID]
}

func (m *mockTracker) setDepStatus(issueNum, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deps["default"] == nil {
		m.deps["default"] = make(map[string]string)
	}
	m.deps["default"][issueNum] = status
}

// mockExecutor 用于测试的执行器
type mockExecutor struct {
	fixCount atomic.Int32
	fixErr   error
	fixDelay time.Duration
}

func (e *mockExecutor) Fix(_ context.Context, _ Issue) error {
	if e.fixDelay > 0 {
		time.Sleep(e.fixDelay)
	}
	e.fixCount.Add(1)
	return e.fixErr
}

func TestDaemonTickQueuedToWorking(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{}
	cfg := DefaultConfig()
	cfg.PollInterval = 100 * time.Millisecond

	tracker.addIssue(Issue{
		ID:     "item-1",
		Number: 42,
		Repo:   "owner/repo",
		Title:  "Test issue",
		Status: StatusQueued,
	})

	d := New(tracker, executor, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	d.tick(ctx)

	// 等待异步 dispatch 完成
	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(1), executor.fixCount.Load(), "应该执行了一次 fix")
	assert.Equal(t, StatusDone, tracker.getStatus("item-1"), "无 PR 时成功后应该直接 Done")
}

func TestDaemonTickDependencyBlocked(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{}
	cfg := DefaultConfig()

	tracker.addIssue(Issue{
		ID:        "item-2",
		Number:    43,
		Repo:      "owner/repo",
		Title:     "Blocked issue",
		Status:    StatusQueued,
		DependsOn: []string{"10"},
	})
	// 依赖 #10 还在 Working，未完成
	tracker.setDepStatus("10", StatusWorking)

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(0), executor.fixCount.Load(), "依赖未满足，不应该执行")
	assert.Equal(t, StatusQueued, tracker.getStatus("item-2"), "应该保持 Queued")
}

func TestDaemonTickDependencySatisfied(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{}
	cfg := DefaultConfig()

	tracker.addIssue(Issue{
		ID:        "item-3",
		Number:    44,
		Repo:      "owner/repo",
		Title:     "Ready issue",
		Status:    StatusQueued,
		DependsOn: []string{"10"},
	})
	// 依赖 #10 已完成
	tracker.setDepStatus("10", StatusDone)

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(1), executor.fixCount.Load(), "依赖满足，应该执行")
	assert.Equal(t, StatusDone, tracker.getStatus("item-3"), "无 PR 时成功后应该直接 Done")
}

func TestDaemonTickFixFailureRollback(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{fixErr: fmt.Errorf("编译失败")}
	cfg := DefaultConfig()

	tracker.addIssue(Issue{
		ID:     "item-4",
		Number: 45,
		Repo:   "owner/repo",
		Title:  "Will fail",
		Status: StatusQueued,
	})

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, StatusQueued, tracker.getStatus("item-4"), "失败后应该回退到 Queued")
}

func TestDaemonTickReviewToDone(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{}
	cfg := DefaultConfig()

	tracker.addIssue(Issue{
		ID:       "item-5",
		Number:   46,
		Repo:     "owner/repo",
		Title:    "Merged PR",
		Status:   StatusReview,
		PRNumber: 100,
		PRMerged: true,
	})

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	assert.Equal(t, StatusDone, tracker.getStatus("item-5"), "PR 已合并应该变成 Done")
}

func TestDaemonMaxConcurrent(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{fixDelay: 300 * time.Millisecond}
	cfg := DefaultConfig()
	cfg.MaxConcurrent = 1

	tracker.addIssue(Issue{ID: "a", Number: 1, Repo: "o/r", Title: "A", Status: StatusQueued})
	tracker.addIssue(Issue{ID: "b", Number: 2, Repo: "o/r", Title: "B", Status: StatusQueued})

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	// 一轮只应该派发 1 个（max_concurrent=1）
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, d.RunningCount(), "并发数应该被限制为 1")

	// 等任务完成
	time.Sleep(500 * time.Millisecond)
	assert.Equal(t, int32(1), executor.fixCount.Load(), "只应该完成 1 个任务")
}

func TestDaemonGracefulShutdown(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{fixDelay: 200 * time.Millisecond}
	cfg := DefaultConfig()
	cfg.PollInterval = 50 * time.Millisecond

	tracker.addIssue(Issue{ID: "s1", Number: 99, Repo: "o/r", Title: "Slow", Status: StatusQueued})

	d := New(tracker, executor, cfg)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	// 等 tick 派发任务后取消
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err, "优雅退出不应该报错")
	case <-time.After(2 * time.Second):
		t.Fatal("优雅退出超时")
	}
}
