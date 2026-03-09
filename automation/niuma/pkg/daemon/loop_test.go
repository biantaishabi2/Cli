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
	mu         sync.Mutex
	issues     map[string][]Issue           // status → issues
	states     map[string]string            // itemID → status
	deps       map[string]map[string]string // 依赖检查结果
	noPR       map[string]bool              // issueID → true 表示不模拟 PR
	comments   map[string][]string          // issueID → 评论列表
	mergeErr   error                        // MergePR 返回的错误
	mergeCalls int                          // MergePR 调用次数
}

func newMockTracker() *mockTracker {
	return &mockTracker{
		issues:   make(map[string][]Issue),
		states:   make(map[string]string),
		deps:     make(map[string]map[string]string),
		noPR:     make(map[string]bool),
		comments: make(map[string][]string),
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
				// 模拟 AI 创建了 PR（除非标记为 noPR）
				if issue.PRNumber == 0 && !m.noPR[issue.ID] {
					issue.PRNumber = issue.Number * 10
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

func (m *mockTracker) AddComment(_ context.Context, issue Issue, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.comments[issue.ID] = append(m.comments[issue.ID], body)
	return nil
}

func (m *mockTracker) MergePR(_ context.Context, _ Issue) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mergeCalls++
	return m.mergeErr
}

func (m *mockTracker) getComments(itemID string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.comments[itemID]
}

func (m *mockTracker) getMergeCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mergeCalls
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
	assert.Equal(t, StatusReview, tracker.getStatus("item-1"), "有 PR 时成功后应该进入 Review")
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
	assert.Equal(t, StatusReview, tracker.getStatus("item-3"), "有 PR 时成功后应该进入 Review")
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

func TestDaemonTickNoPRFailure(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{}
	cfg := DefaultConfig()

	tracker.addIssue(Issue{
		ID:     "item-nopr",
		Number: 50,
		Repo:   "owner/repo",
		Title:  "AI 没创建 PR",
		Status: StatusQueued,
	})
	// 标记此 issue 不模拟 PR 创建
	tracker.noPR["item-nopr"] = true

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, int32(1), executor.fixCount.Load(), "AI 应该执行了一次")
	assert.Equal(t, StatusQueued, tracker.getStatus("item-nopr"), "无 PR 时应该回退到 Queued")
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

func TestDaemonFailureComment(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{fixErr: fmt.Errorf("编译失败: undefined function foo/0")}
	cfg := DefaultConfig()

	tracker.addIssue(Issue{
		ID:     "item-cmt",
		Number: 60,
		Repo:   "owner/repo",
		Title:  "Will fail with comment",
		Status: StatusQueued,
	})

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, StatusQueued, tracker.getStatus("item-cmt"), "失败后应该回退到 Queued")
	comments := tracker.getComments("item-cmt")
	assert.Equal(t, 1, len(comments), "应该有一条失败评论")
	assert.Contains(t, comments[0], "自动处理失败", "评论应该包含失败标题")
	assert.Contains(t, comments[0], "编译失败", "评论应该包含错误详情")
	assert.Contains(t, comments[0], "1/3", "评论应该包含重试次数")
}

func TestDaemonGateFailureComment(t *testing.T) {
	tracker := newMockTracker()
	// executor 返回 GateError（AI 成功但 gate 失败）
	executor := &mockExecutor{fixErr: &GateError{
		Output: "test_quality_check ... FAILED\n  expected: :ok\n  got: {:error, \"validation failed\"}",
		Err:    fmt.Errorf("exit status 1"),
	}}
	cfg := DefaultConfig()

	tracker.addIssue(Issue{
		ID:     "item-gate",
		Number: 61,
		Repo:   "owner/repo",
		Title:  "Gate will fail",
		Status: StatusQueued,
	})

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, StatusQueued, tracker.getStatus("item-gate"), "gate 失败后应该回退到 Queued")
	comments := tracker.getComments("item-gate")
	assert.Equal(t, 1, len(comments), "应该有一条失败评论")
	assert.Contains(t, comments[0], "gate 验证", "评论应该标明是 gate 阶段失败")
	assert.Contains(t, comments[0], "FAILED", "评论应该包含 gate 输出")
}

func TestDaemonNoPRFailureComment(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{}
	cfg := DefaultConfig()

	tracker.addIssue(Issue{
		ID:     "item-nopr2",
		Number: 62,
		Repo:   "owner/repo",
		Title:  "No PR created",
		Status: StatusQueued,
	})
	tracker.noPR["item-nopr2"] = true

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, StatusQueued, tracker.getStatus("item-nopr2"))
	comments := tracker.getComments("item-nopr2")
	assert.Equal(t, 1, len(comments), "应该有一条失败评论")
	assert.Contains(t, comments[0], "未创建 PR", "评论应该说明无 PR")
}

func TestDaemonSuccessNoComment(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{}
	cfg := DefaultConfig()

	tracker.addIssue(Issue{
		ID:     "item-ok",
		Number: 63,
		Repo:   "owner/repo",
		Title:  "Success",
		Status: StatusQueued,
	})

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	time.Sleep(200 * time.Millisecond)

	assert.Equal(t, StatusReview, tracker.getStatus("item-ok"))
	comments := tracker.getComments("item-ok")
	assert.Equal(t, 0, len(comments), "成功不应该留评论")
}

func TestDaemonRetryCountIncrement(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{fixErr: fmt.Errorf("always fail")}
	cfg := DefaultConfig()
	cfg.MaxRetries = 3

	tracker.addIssue(Issue{
		ID:     "item-retry",
		Number: 64,
		Repo:   "owner/repo",
		Title:  "Retry test",
		Status: StatusQueued,
	})

	d := New(tracker, executor, cfg)
	ctx := context.Background()

	// 第一次
	d.tick(ctx)
	time.Sleep(200 * time.Millisecond)
	comments := tracker.getComments("item-retry")
	assert.Equal(t, 1, len(comments))
	assert.Contains(t, comments[0], "1/3")

	// 第二次
	d.tick(ctx)
	time.Sleep(200 * time.Millisecond)
	comments = tracker.getComments("item-retry")
	assert.Equal(t, 2, len(comments))
	assert.Contains(t, comments[1], "2/3")

	// 第三次
	d.tick(ctx)
	time.Sleep(200 * time.Millisecond)
	comments = tracker.getComments("item-retry")
	assert.Equal(t, 3, len(comments))
	assert.Contains(t, comments[2], "3/3")

	// 第四次：应该被跳过（达到上限）
	d.tick(ctx)
	time.Sleep(200 * time.Millisecond)
	comments = tracker.getComments("item-retry")
	assert.Equal(t, 3, len(comments), "达到上限后不应该再留评论")
	assert.Equal(t, int32(3), executor.fixCount.Load(), "达到上限后不应该再执行")
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

func TestDaemonAutoMergeMergeablePR(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{}
	cfg := DefaultConfig()

	// PR 可合并但未合并
	tracker.addIssue(Issue{
		ID:          "item-am",
		Number:      70,
		Repo:        "owner/repo",
		Title:       "Auto merge test",
		Status:      StatusReview,
		PRNumber:    700,
		PRMerged:    false,
		PRMergeable: "MERGEABLE",
	})

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	assert.Equal(t, StatusDone, tracker.getStatus("item-am"), "MERGEABLE PR 应该自动合并后变 Done")
	assert.Equal(t, 1, tracker.getMergeCalls(), "应该调用一次 MergePR")
}

func TestDaemonAutoMergeConflicting(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{}
	cfg := DefaultConfig()

	// PR 有冲突
	tracker.addIssue(Issue{
		ID:          "item-cf",
		Number:      71,
		Repo:        "owner/repo",
		Title:       "Conflicting PR",
		Status:      StatusReview,
		PRNumber:    710,
		PRMerged:    false,
		PRMergeable: "CONFLICTING",
	})

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	assert.Equal(t, StatusQueued, tracker.getStatus("item-cf"), "CONFLICTING PR 应该回退到 Queued 重做")
	assert.Equal(t, 0, tracker.getMergeCalls(), "不应该调用 MergePR")
}

func TestDaemonAutoMergeConflictingRetry(t *testing.T) {
	tracker := newMockTracker()
	executor := &mockExecutor{}
	cfg := DefaultConfig()

	// PR 有冲突
	tracker.addIssue(Issue{
		ID:          "item-conflict",
		Number:      73,
		Repo:        "owner/repo",
		Title:       "Conflicting PR retry",
		Status:      StatusReview,
		PRNumber:    730,
		PRMerged:    false,
		PRMergeable: "CONFLICTING",
	})

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	assert.Equal(t, StatusQueued, tracker.getStatus("item-conflict"), "CONFLICTING PR 应该回退到 Queued")
	assert.Equal(t, 0, tracker.getMergeCalls(), "不应该尝试合并")
	comments := tracker.getComments("item-conflict")
	assert.Equal(t, 1, len(comments), "应该留一条冲突评论")
	assert.Contains(t, comments[0], "合并冲突", "评论应提到冲突")
}

func TestDaemonAutoMergeFailure(t *testing.T) {
	tracker := newMockTracker()
	tracker.mergeErr = fmt.Errorf("merge conflict")
	executor := &mockExecutor{}
	cfg := DefaultConfig()

	tracker.addIssue(Issue{
		ID:          "item-mf",
		Number:      72,
		Repo:        "owner/repo",
		Title:       "Merge will fail",
		Status:      StatusReview,
		PRNumber:    720,
		PRMerged:    false,
		PRMergeable: "MERGEABLE",
	})

	d := New(tracker, executor, cfg)
	ctx := context.Background()
	d.tick(ctx)

	assert.Equal(t, StatusReview, tracker.getStatus("item-mf"), "合并失败应该保持 Review")
	assert.Equal(t, 1, tracker.getMergeCalls(), "应该尝试合并")
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
