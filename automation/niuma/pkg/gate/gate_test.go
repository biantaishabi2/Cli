package gate

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_PassResetsRetryCount(t *testing.T) {
	var upsertedCount int
	var upsertedAttemptKey string
	upsertCalled := false
	markCalled := 0
	labelCalled := 0
	commentCalled := 0

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
		MarkNeedsFix: func(context.Context, string, int) error {
			markCalled++
			return nil
		},
		AddLabels: func(context.Context, string, int, []string) error {
			labelCalled++
			return nil
		},
		AddComment: func(context.Context, string, int, string) error {
			commentCalled++
			return nil
		},
		FindGateRetryState: func(context.Context, int) (int, string, error) {
			return 2, defaultAttemptKey(358, 9001), nil
		},
		UpsertGateRetryCount: func(_ context.Context, _ int, count int, attemptKey string) error {
			upsertCalled = true
			upsertedCount = count
			upsertedAttemptKey = attemptKey
			return nil
		},
		HasLabel: func(context.Context, int, string) (bool, error) { return false, nil },
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.True(t, upsertCalled, "gate 通过后应调用 UpsertGateRetryCount")
	assert.Equal(t, 0, upsertedCount, "gate 通过后 retry_count 应重置为 0")
	assert.Equal(t, "", upsertedAttemptKey, "gate 通过后应清空 attempt_key")
	assert.Equal(t, 0, markCalled, "gate 通过不应回退到 needs-fix")
	assert.Equal(t, 0, labelCalled, "gate 通过不应打 escalation 标签")
	assert.Equal(t, 0, commentCalled, "gate 通过不应写 issue 评论")
}

func TestRunner_FirstFailureMarksNeedsFix(t *testing.T) {
	markCalled := 0
	labelCalled := 0
	comments := make([]string, 0, 1)
	var upsertedCount int
	var upsertedAttemptKey string

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
		FindGateRetryState: func(context.Context, int) (int, string, error) {
			return 0, "", nil // 无历史 marker
		},
		UpsertGateRetryCount: func(_ context.Context, _ int, count int, attemptKey string) error {
			upsertedCount = count
			upsertedAttemptKey = attemptKey
			return nil
		},
		HasLabel: func(context.Context, int, string) (bool, error) { return false, nil },
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.False(t, result.Passed)
	assert.Equal(t, "UNKNOWN", result.ReasonCode)
	assert.Equal(t, FailureClassCode, result.FailureClass)
	assert.False(t, result.Escalated)
	assert.Equal(t, defaultAttemptKey(358, 9010), result.AttemptKey)
	assert.Equal(t, 1, result.RetryCount)
	assert.Equal(t, 1, upsertedCount, "应写入 retry_count=1")
	assert.Equal(t, defaultAttemptKey(358, 9010), upsertedAttemptKey)
	assert.Equal(t, 1, markCalled)
	assert.Equal(t, 0, labelCalled)
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "retry_count=1")
	assert.Contains(t, comments[0], "max_retries=2")
	assert.Contains(t, comments[0], defaultAttemptKey(358, 9010))
}

func TestRunner_DeferredFailureSkipsNeedsFix(t *testing.T) {
	markCalled := 0
	labelCalled := 0
	issueComments := make([]string, 0, 1)
	prReviews := make([]string, 0, 1)
	upsertCalls := make([]struct {
		count      int
		attemptKey string
	}, 0, 1)

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9015,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "[gate] reason_code=CHECKS_QUERY_FAILED\n[gate] head_sha=abc123\n[gate][warning] 查询 PR checks 失败", errors.New("exit status 1")
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
			issueComments = append(issueComments, body)
			return nil
		},
		FindGateRetryState: func(context.Context, int) (int, string, error) {
			return 1, "issue-358-pr-9015-head-abc123", nil
		},
		UpsertGateRetryCount: func(_ context.Context, _ int, count int, attemptKey string) error {
			upsertCalls = append(upsertCalls, struct {
				count      int
				attemptKey string
			}{count: count, attemptKey: attemptKey})
			return nil
		},
		HasLabel: func(context.Context, int, string) (bool, error) { return false, nil },
		AddPRReview: func(_ context.Context, _ string, _ int, body string) error {
			prReviews = append(prReviews, body)
			return nil
		},
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.False(t, result.Passed)
	assert.Equal(t, "CHECKS_QUERY_FAILED", result.ReasonCode)
	assert.Equal(t, FailureClassDeferred, result.FailureClass)
	assert.Equal(t, "issue-358-pr-9015-head-abc123", result.AttemptKey)
	assert.Equal(t, 0, result.RetryCount, "deferred 场景不应进入自动修复计数")
	assert.Equal(t, 0, markCalled, "deferred 场景不应回退到 pr-needs-fix")
	assert.Equal(t, 0, labelCalled, "deferred 场景不应升级标签")
	require.Len(t, issueComments, 1)
	assert.Contains(t, issueComments[0], "Gate 暂未形成代码失败结论")
	assert.Contains(t, issueComments[0], "CHECKS_QUERY_FAILED")
	require.Len(t, prReviews, 1)
	assert.Contains(t, prReviews[0], "reason_code=CHECKS_QUERY_FAILED")
	require.Len(t, upsertCalls, 1)
	assert.Equal(t, 0, upsertCalls[0].count)
	assert.Equal(t, "", upsertCalls[0].attemptKey)
}

func TestRunner_SameRunRetryCountAccumulates(t *testing.T) {
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
		FindGateRetryState: func(context.Context, int) (int, string, error) {
			return 1, defaultAttemptKey(358, 9010), nil // 同一次 run 已有 1 次
		},
		UpsertGateRetryCount: func(_ context.Context, _ int, count int, _ string) error {
			upsertedCount = count
			return nil
		},
		HasLabel: func(context.Context, int, string) (bool, error) { return false, nil },
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.Equal(t, 2, result.RetryCount, "同 run 内 retry_count 应为 1+1=2")
	assert.Equal(t, 2, upsertedCount)
	assert.False(t, result.Escalated, "retry_count=2 == maxRetries=2，不应 escalate")
}

func TestRunner_HeadScopedAttemptKeyAccumulatesAcrossRuns(t *testing.T) {
	var upsertedCount int
	var upsertedAttemptKey string

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9020,
		RepoDir:    t.TempDir(),
		MaxRetries: 5,
		Now:        fixedNow,
		RunID:      "new-run-id",
		RunAttempt: "9",
		RunGate: func(context.Context, string, int) (string, error) {
			return "[gate] reason_code=REQUIRED_JOBS_FAILED\n[gate] head_sha=abc999\n[gate] run_mode=critical", errors.New("exit status 1")
		},
		MarkNeedsFix: func(context.Context, string, int) error { return nil },
		AddLabels:    func(context.Context, string, int, []string) error { return nil },
		AddComment:   func(context.Context, string, int, string) error { return nil },
		FindGateRetryState: func(context.Context, int) (int, string, error) {
			return 2, "issue-358-pr-9020-head-abc999", nil
		},
		UpsertGateRetryCount: func(_ context.Context, _ int, count int, attemptKey string) error {
			upsertedCount = count
			upsertedAttemptKey = attemptKey
			return nil
		},
		HasLabel: func(context.Context, int, string) (bool, error) { return false, nil },
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.Equal(t, FailureClassCode, result.FailureClass)
	assert.Equal(t, "REQUIRED_JOBS_FAILED", result.ReasonCode)
	assert.Equal(t, "issue-358-pr-9020-head-abc999", result.AttemptKey)
	assert.Equal(t, 3, result.RetryCount, "同一 head 跨 run 应继续累计")
	assert.Equal(t, 3, upsertedCount)
	assert.Equal(t, "issue-358-pr-9020-head-abc999", upsertedAttemptKey)
}

func TestRunner_CrossRunRetryCountResets(t *testing.T) {
	var upsertedCount int

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9010,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		Now:        fixedNow,
		RunID:      "new-run-id",
		RunAttempt: "1",
		RunGate: func(context.Context, string, int) (string, error) {
			return "go test failed", errors.New("exit status 1")
		},
		MarkNeedsFix: func(context.Context, string, int) error { return nil },
		AddLabels:    func(context.Context, string, int, []string) error { return nil },
		AddComment:   func(context.Context, string, int, string) error { return nil },
		FindGateRetryState: func(context.Context, int) (int, string, error) {
			return 2, "issue-358-pr-9010-run-old-run-id-attempt-1", nil // 上一次 run
		},
		UpsertGateRetryCount: func(_ context.Context, _ int, count int, _ string) error {
			upsertedCount = count
			return nil
		},
		HasLabel: func(context.Context, int, string) (bool, error) { return false, nil },
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.Equal(t, "issue-358-pr-9010-run-new-run-id-attempt-1", result.AttemptKey)
	assert.Equal(t, 1, result.RetryCount, "跨 run 后应从 0 重新计数")
	assert.Equal(t, 1, upsertedCount)
	assert.False(t, result.Escalated)
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
		FindGateRetryState: func(context.Context, int) (int, string, error) {
			return 2, defaultAttemptKey(358, 9011), nil // 同 run 已有 2 次
		},
		UpsertGateRetryCount: func(context.Context, int, int, string) error { return nil },
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
	assert.Contains(t, comments[0], defaultAttemptKey(358, 9011))
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
		FindGateRetryState: func(context.Context, int) (int, string, error) {
			return 3, defaultAttemptKey(358, 9012), nil // 已经超限
		},
		UpsertGateRetryCount: func(context.Context, int, int, string) error { return nil },
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
	if opts.RunID == "" {
		opts.RunID = "12345"
	}
	if opts.RunAttempt == "" {
		opts.RunAttempt = "1"
	}
	runner, err := NewRunner(opts)
	require.NoError(t, err)
	return runner
}

func fixedNow() time.Time {
	return time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC)
}

func defaultAttemptKey(issue, pr int) string {
	return fmt.Sprintf("issue-%d-pr-%d-run-12345-attempt-1", issue, pr)
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
		FindGateRetryState:   func(context.Context, int) (int, string, error) { return 0, "", nil },
		UpsertGateRetryCount: func(context.Context, int, int, string) error { return nil },
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
		FindGateRetryState:   func(context.Context, int) (int, string, error) { return 0, "", nil },
		UpsertGateRetryCount: func(context.Context, int, int, string) error { return nil },
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
		FindGateRetryState:   func(context.Context, int) (int, string, error) { return 0, "", nil },
		UpsertGateRetryCount: func(context.Context, int, int, string) error { return nil },
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
		FindGateRetryState:   func(context.Context, int) (int, string, error) { return 0, "", nil },
		UpsertGateRetryCount: func(context.Context, int, int, string) error { return nil },
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

// ===== SelfCheck 模式测试 =====

func TestRunner_SelfCheck_SkipsMarkNeedsFixAndComment(t *testing.T) {
	markCalled := 0
	commentCalled := 0
	prReviewCalled := 0

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9040,
		RepoDir:    t.TempDir(),
		MaxRetries: 2,
		SelfCheck:  true,
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
		FindGateRetryState:   func(context.Context, int) (int, string, error) { return 0, "", nil },
		UpsertGateRetryCount: func(context.Context, int, int, string) error { return nil },
		HasLabel:             func(context.Context, int, string) (bool, error) { return false, nil },
		AddPRReview: func(context.Context, string, int, string) error {
			prReviewCalled++
			return nil
		},
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.False(t, result.Passed)
	assert.Equal(t, 0, markCalled, "self-check 模式不应调用 MarkNeedsFix")
	assert.Equal(t, 0, commentCalled, "self-check 模式不应写 issue 评论")
	assert.Equal(t, 1, prReviewCalled, "self-check 模式仍应写 PR review")
}

func TestRunner_SelfCheck_EscalationStillWorks(t *testing.T) {
	// self-check 模式下超限 escalation 行为不变
	labelCalled := 0
	commentCalled := 0

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9041,
		RepoDir:    t.TempDir(),
		MaxRetries: 1,
		SelfCheck:  true,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "gate failed", errors.New("exit status 1")
		},
		MarkNeedsFix: func(context.Context, string, int) error { return nil },
		AddLabels: func(context.Context, string, int, []string) error {
			labelCalled++
			return nil
		},
		AddComment: func(context.Context, string, int, string) error {
			commentCalled++
			return nil
		},
		FindGateRetryState: func(context.Context, int) (int, string, error) {
			return 1, defaultAttemptKey(358, 9041), nil // 已有 1 次，超限
		},
		UpsertGateRetryCount: func(context.Context, int, int, string) error { return nil },
		HasLabel:             func(context.Context, int, string) (bool, error) { return false, nil },
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.True(t, result.Escalated, "超限时 escalation 行为不变")
	assert.Equal(t, 1, labelCalled, "超限时应打标签")
	assert.Equal(t, 1, commentCalled, "超限时应写评论")
}

func TestClassifyFailureReason_NewInfraReasonsAreDeferred(t *testing.T) {
	cases := []string{
		"NETWORK_TRANSIENT",
		"AUTH_FAILED",
		"RATE_LIMITED",
	}

	for _, reason := range cases {
		assert.Equalf(t, FailureClassDeferred, classifyFailureReason(reason), "%s 应归类为 deferred", reason)
	}
}
