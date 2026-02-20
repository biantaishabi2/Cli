package gate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// markerStore 模拟 marker 持久化
type markerStore struct {
	revision int
	found    bool
	updated  []*marker.Marker
	bodies   []string
}

func (s *markerStore) find(_ context.Context, _ int, _ marker.Type) (int, bool, error) {
	return s.revision, s.found, nil
}

func (s *markerStore) update(_ context.Context, _ int, m *marker.Marker, body string) error {
	s.updated = append(s.updated, m)
	s.bodies = append(s.bodies, body)
	s.revision = m.Revision
	s.found = true
	return nil
}

func baseOpts(store *markerStore) Options {
	return Options{
		Repo:         "biantaishabi2/Cli",
		Issue:        358,
		PR:           9010,
		RepoDir:      "/tmp/test",
		MaxRetries:   2,
		Now:          fixedNow,
		MarkNeedsFix: func(context.Context, string, int) error { return nil },
		AddLabels:    func(context.Context, string, int, []string) error { return nil },
		AddComment:   func(context.Context, string, int, string) error { return nil },
		FindMarker:   store.find,
		UpdateMarker: store.update,
	}
}

func TestRunner_PassDoesNotMutateState(t *testing.T) {
	store := &markerStore{revision: 1, found: true}

	opts := baseOpts(store)
	opts.RunGate = func(context.Context, string, int) (string, error) {
		return "ok", nil
	}

	runner := newRunnerForTest(t, opts)
	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Len(t, store.updated, 0, "gate 通过时不应更新 marker")
}

func TestRunner_FirstFailureMarksNeedsFix(t *testing.T) {
	store := &markerStore{found: false}
	markCalled := 0
	labelCalled := 0
	comments := make([]string, 0)
	prComments := make([]string, 0)

	opts := baseOpts(store)
	opts.RunGate = func(context.Context, string, int) (string, error) {
		return "go test failed\n--- FAIL: TestFoo", errors.New("exit status 1")
	}
	opts.MarkNeedsFix = func(context.Context, string, int) error {
		markCalled++
		return nil
	}
	opts.AddLabels = func(context.Context, string, int, []string) error {
		labelCalled++
		return nil
	}
	opts.AddComment = func(_ context.Context, _ string, _ int, body string) error {
		comments = append(comments, body)
		return nil
	}
	opts.AddPRComment = func(_ context.Context, _ string, _ int, body string) error {
		prComments = append(prComments, body)
		return nil
	}

	runner := newRunnerForTest(t, opts)
	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.False(t, result.Passed)
	assert.False(t, result.Escalated)
	assert.Equal(t, 1, result.RetryCount)
	assert.Equal(t, "issue-358-pr-9010-20260219", result.AttemptKey)

	// MarkNeedsFix 被调用
	assert.Equal(t, 1, markCalled)
	assert.Equal(t, 0, labelCalled)

	// issue 评论包含 retry_count
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "retry_count=1")
	assert.Contains(t, comments[0], "max_retries=2")

	// marker 被更新为 revision=1
	require.Len(t, store.updated, 1)
	assert.Equal(t, 1, store.updated[0].Revision)
	assert.Equal(t, marker.TypeGateRetry, store.updated[0].Type)
	assert.Equal(t, 358, store.updated[0].Issue)

	// PR 评论包含测试失败详情
	require.Len(t, prComments, 1)
	assert.Contains(t, prComments[0], "FAIL: TestFoo")
	assert.Contains(t, prComments[0], "Gate 测试失败详情")
}

func TestRunner_RetryCountAccumulatesAcrossRuns(t *testing.T) {
	// 模拟第一次失败后 marker revision=1
	store := &markerStore{revision: 1, found: true}
	markCalled := 0

	opts := baseOpts(store)
	opts.RunGate = func(context.Context, string, int) (string, error) {
		return "test failed", errors.New("exit status 1")
	}
	opts.MarkNeedsFix = func(context.Context, string, int) error {
		markCalled++
		return nil
	}

	runner := newRunnerForTest(t, opts)
	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.Equal(t, 2, result.RetryCount, "应该从 marker revision=1 累加到 2")
	assert.False(t, result.Escalated, "retry=2 不超过 max=2，不应 escalate")
	assert.Equal(t, 1, markCalled)

	// marker 更新为 revision=2
	require.Len(t, store.updated, 1)
	assert.Equal(t, 2, store.updated[0].Revision)
}

func TestRunner_ExceedRetriesEscalates(t *testing.T) {
	// marker revision=2, 下次失败 retryCount=3 > maxRetries=2 → escalate
	store := &markerStore{revision: 2, found: true}
	markCalled := 0
	labels := make([][]string, 0)
	comments := make([]string, 0)

	opts := baseOpts(store)
	opts.PR = 9011
	opts.RunGate = func(context.Context, string, int) (string, error) {
		return "gate failed", errors.New("exit status 1")
	}
	opts.MarkNeedsFix = func(context.Context, string, int) error {
		markCalled++
		return nil
	}
	opts.AddLabels = func(_ context.Context, _ string, _ int, set []string) error {
		labels = append(labels, append([]string(nil), set...))
		return nil
	}
	opts.AddComment = func(_ context.Context, _ string, _ int, body string) error {
		comments = append(comments, body)
		return nil
	}

	runner := newRunnerForTest(t, opts)
	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.True(t, result.Escalated)
	assert.Equal(t, 3, result.RetryCount)

	// 不调用 MarkNeedsFix，改为打标 needs-human
	assert.Equal(t, 0, markCalled)
	require.Len(t, labels, 1)
	assert.Equal(t, []string{needsHumanLabel, integrationGateFailedLabel}, labels[0])

	// 发 escalation 评论
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "retry_count=3")
	assert.Contains(t, comments[0], "已超限")

	// marker 更新为 revision=3
	require.Len(t, store.updated, 1)
	assert.Equal(t, 3, store.updated[0].Revision)
}

func TestRunner_NoPRCommentWhenCallbackNil(t *testing.T) {
	store := &markerStore{found: false}

	opts := baseOpts(store)
	opts.AddPRComment = nil // 不配置 PR 评论回调
	opts.RunGate = func(context.Context, string, int) (string, error) {
		return "test failed", errors.New("exit status 1")
	}

	runner := newRunnerForTest(t, opts)
	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.Equal(t, 1, result.RetryCount)
	// 不应 panic
}

func TestRunner_FindMarkerErrorDegrades(t *testing.T) {
	// FindMarker 返回错误时降级为 retryCount=1
	opts := baseOpts(nil)
	opts.FindMarker = func(context.Context, int, marker.Type) (int, bool, error) {
		return 0, false, errors.New("API error")
	}
	opts.UpdateMarker = func(context.Context, int, *marker.Marker, string) error { return nil }
	opts.RunGate = func(context.Context, string, int) (string, error) {
		return "test failed", errors.New("exit status 1")
	}

	runner := newRunnerForTest(t, opts)
	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.Equal(t, 1, result.RetryCount, "FindMarker 失败时降级为 1")
}

func TestTrimGateError_Truncates(t *testing.T) {
	long := make([]byte, 3000)
	for i := range long {
		long[i] = 'x'
	}
	result := trimGateError(string(long), nil)
	assert.Equal(t, maxGateErrorMessageLen, len(result))
}

func TestTrimGateError_CombinesOutputAndErr(t *testing.T) {
	result := trimGateError("test output", errors.New("exit 1"))
	assert.Contains(t, result, "test output")
	assert.Contains(t, result, "exit 1")
}

func TestTrimGateError_EmptyFallback(t *testing.T) {
	result := trimGateError("", nil)
	assert.Equal(t, "gate 命令失败", result)
}

func newRunnerForTest(t *testing.T, opts Options) *Runner {
	t.Helper()
	runner, err := NewRunner(opts)
	require.NoError(t, err)
	return runner
}

func fixedNow() time.Time {
	return time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC)
}
