package gate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_PassResetsRetryCount(t *testing.T) {
	var upsertedCount int
	upsertCalled := false

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9001,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "ok", nil
		},
		MarkNeedsFix: func(context.Context, string, int) error { return nil },
		AddLabels:    func(context.Context, string, int, []string) error { return nil },
		AddComment:   func(context.Context, string, int, string) error { return nil },
		FindGateRetryCount: func(context.Context, int) (int, error) {
			return 2, nil
		},
		UpsertGateRetryCount: func(_ context.Context, _ int, count int) error {
			upsertCalled = true
			upsertedCount = count
			return nil
		},
		HasLabel: func(context.Context, int, string) (bool, error) { return false, nil },
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.True(t, upsertCalled, "gate 通过后应调用 UpsertGateRetryCount")
	assert.Equal(t, 0, upsertedCount, "gate 通过后 retry_count 应重置为 0")
}

func TestRunner_FirstFailureMarksNeedsFix(t *testing.T) {
	markCalled := 0
	labelCalled := 0
	comments := make([]string, 0, 1)
	var upsertedCount int

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9010,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "go test failed", errors.New("exit status 1")
		},
		MarkNeedsFix: func(context.Context, string, int) error {
			markCalled++
			return nil
		},
		AddLabels: func(context.Context, string, int, []string) error {
			labelCalled++
			return nil
		},
		AddComment: func(_ context.Context, _ string, _ int, body string) error {
			comments = append(comments, body)
			return nil
		},
		FindGateRetryCount: func(context.Context, int) (int, error) {
			return 0, nil // 无历史 marker
		},
		UpsertGateRetryCount: func(_ context.Context, _ int, count int) error {
			upsertedCount = count
			return nil
		},
		HasLabel: func(context.Context, int, string) (bool, error) { return false, nil },
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.False(t, result.Passed)
	assert.False(t, result.Escalated)
	assert.Equal(t, "issue-358-pr-9010-20260219", result.AttemptKey)
	assert.Equal(t, 1, result.RetryCount)
	assert.Equal(t, 1, upsertedCount, "应写入 retry_count=1")
	assert.Equal(t, 1, markCalled)
	assert.Equal(t, 0, labelCalled)
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "retry_count=1")
	assert.Contains(t, comments[0], "max_retries=2")
	assert.Contains(t, comments[0], "issue-358-pr-9010-20260219")
}

func TestRunner_CrossRunRetryCountAccumulates(t *testing.T) {
	// 模拟跨 CI run：上次 marker 记录 retry_count=1
	var upsertedCount int

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9010,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "go test failed", errors.New("exit status 1")
		},
		MarkNeedsFix: func(context.Context, string, int) error { return nil },
		AddLabels:    func(context.Context, string, int, []string) error { return nil },
		AddComment:   func(context.Context, string, int, string) error { return nil },
		FindGateRetryCount: func(context.Context, int) (int, error) {
			return 1, nil // 上次已有 1 次
		},
		UpsertGateRetryCount: func(_ context.Context, _ int, count int) error {
			upsertedCount = count
			return nil
		},
		HasLabel: func(context.Context, int, string) (bool, error) { return false, nil },
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.Equal(t, 2, result.RetryCount, "retry_count 应为 1+1=2")
	assert.Equal(t, 2, upsertedCount)
	assert.False(t, result.Escalated, "retry_count=2 == maxRetries=2，不应 escalate")
}

func TestRunner_ExceedRetriesEscalates(t *testing.T) {
	markCalled := 0
	labels := make([][]string, 0, 1)
	comments := make([]string, 0, 1)

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9011,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "gate failed", errors.New("exit status 1")
		},
		MarkNeedsFix: func(context.Context, string, int) error {
			markCalled++
			return nil
		},
		AddLabels: func(_ context.Context, _ string, _ int, set []string) error {
			labels = append(labels, append([]string(nil), set...))
			return nil
		},
		AddComment: func(_ context.Context, _ string, _ int, body string) error {
			comments = append(comments, body)
			return nil
		},
		FindGateRetryCount: func(context.Context, int) (int, error) {
			return 2, nil // 已有 2 次
		},
		UpsertGateRetryCount: func(context.Context, int, int) error { return nil },
		HasLabel: func(context.Context, int, string) (bool, error) {
			return false, nil // needs-human 标签不存在
		},
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.True(t, result.Escalated)
	assert.Equal(t, 3, result.RetryCount)
	assert.Equal(t, 0, markCalled, "escalation 不应调用 MarkNeedsFix")
	require.Len(t, labels, 1)
	assert.Equal(t, []string{needsHumanLabel, integrationGateFailedLabel}, labels[0])
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "retry_count=3")
	assert.Contains(t, comments[0], "issue-358-pr-9011-20260219")
}

func TestRunner_DedupEscalationByLabelCheck(t *testing.T) {
	labels := 0
	comments := 0

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9012,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "gate failed", errors.New("exit status 1")
		},
		MarkNeedsFix: func(context.Context, string, int) error { return nil },
		AddLabels: func(context.Context, string, int, []string) error {
			labels++
			return nil
		},
		AddComment: func(context.Context, string, int, string) error {
			comments++
			return nil
		},
		FindGateRetryCount: func(context.Context, int) (int, error) {
			return 3, nil // 已经超限
		},
		UpsertGateRetryCount: func(context.Context, int, int) error { return nil },
		HasLabel: func(_ context.Context, _ int, label string) (bool, error) {
			if label == needsHumanLabel {
				return true, nil // needs-human 已存在
			}
			return false, nil
		},
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.True(t, result.Escalated)
	assert.True(t, result.EscalationCommentSkipped)
	assert.Equal(t, 4, result.RetryCount)
	assert.Equal(t, 0, labels, "已有 needs-human 标签时不应重复打标")
	assert.Equal(t, 0, comments, "已有 needs-human 标签时不应重复发评论")
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

func TestRunner_FailurePostsPRReview(t *testing.T) {
	prReviewCalled := 0
	var prReviewBody string
	var prReviewPR int

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9030,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "go test failed: expected foo got bar", errors.New("exit status 1")
		},
		MarkNeedsFix:         func(context.Context, string, int) error { return nil },
		AddLabels:            func(context.Context, string, int, []string) error { return nil },
		AddComment:           func(context.Context, string, int, string) error { return nil },
		FindGateRetryCount:   func(context.Context, int) (int, error) { return 0, nil },
		UpsertGateRetryCount: func(context.Context, int, int) error { return nil },
		HasLabel:             func(context.Context, int, string) (bool, error) { return false, nil },
		AddPRReview: func(_ context.Context, _ string, pr int, body string) error {
			prReviewCalled++
			prReviewPR = pr
			prReviewBody = body
			return nil
		},
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.False(t, result.Escalated)
	assert.Equal(t, 1, prReviewCalled, "gate 失败应发 PR review")
	assert.Equal(t, 9030, prReviewPR)
	assert.Contains(t, prReviewBody, "Gate 测试失败详情")
	assert.Contains(t, prReviewBody, "go test failed")
}

func TestRunner_PassDoesNotPostPRReview(t *testing.T) {
	prReviewCalled := 0

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9031,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "ok", nil
		},
		MarkNeedsFix:         func(context.Context, string, int) error { return nil },
		AddLabels:            func(context.Context, string, int, []string) error { return nil },
		AddComment:           func(context.Context, string, int, string) error { return nil },
		FindGateRetryCount:   func(context.Context, int) (int, error) { return 0, nil },
		UpsertGateRetryCount: func(context.Context, int, int) error { return nil },
		HasLabel:             func(context.Context, int, string) (bool, error) { return false, nil },
		AddPRReview: func(context.Context, string, int, string) error {
			prReviewCalled++
			return nil
		},
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 0, prReviewCalled, "gate 通过不应发 PR review")
}

func TestRunner_NilAddPRReviewBackwardCompatible(t *testing.T) {
	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9032,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "go test failed", errors.New("exit status 1")
		},
		MarkNeedsFix:         func(context.Context, string, int) error { return nil },
		AddLabels:            func(context.Context, string, int, []string) error { return nil },
		AddComment:           func(context.Context, string, int, string) error { return nil },
		FindGateRetryCount:   func(context.Context, int) (int, error) { return 0, nil },
		UpsertGateRetryCount: func(context.Context, int, int) error { return nil },
		HasLabel:             func(context.Context, int, string) (bool, error) { return false, nil },
		// AddPRReview 不设置（nil），不应 panic
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.False(t, result.Passed)
}

func TestRunner_AddPRReviewFailureDoesNotBlock(t *testing.T) {
	markCalled := 0
	commentCalled := 0

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9033,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "go test failed", errors.New("exit status 1")
		},
		MarkNeedsFix: func(context.Context, string, int) error {
			markCalled++
			return nil
		},
		AddLabels: func(context.Context, string, int, []string) error { return nil },
		AddComment: func(context.Context, string, int, string) error {
			commentCalled++
			return nil
		},
		FindGateRetryCount:   func(context.Context, int) (int, error) { return 0, nil },
		UpsertGateRetryCount: func(context.Context, int, int) error { return nil },
		HasLabel:             func(context.Context, int, string) (bool, error) { return false, nil },
		AddPRReview: func(context.Context, string, int, string) error {
			return errors.New("API timeout")
		},
	})

	_, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.Equal(t, 1, markCalled, "PR review 失败不应阻塞主流程")
	assert.Equal(t, 1, commentCalled)
}
