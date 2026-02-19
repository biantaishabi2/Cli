package gate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_PassDoesNotMutateState(t *testing.T) {
	repoDir := t.TempDir()
	storePath := writeTaskStore(t, repoDir, 358, map[string]string{
		metaKeyGateRetryCount: "1",
		metaKeyGateStatus:     gateStatusRetrying,
	})

	markCalled := 0
	labelCalled := 0
	commentCalled := 0

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9001,
		RepoDir:    repoDir,
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
	})

	result, err := runner.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, 0, markCalled)
	assert.Equal(t, 0, labelCalled)
	assert.Equal(t, 0, commentCalled)

	meta := readIssueMetadata(t, storePath, 358)
	assert.Equal(t, "1", meta[metaKeyGateRetryCount])
	assert.Equal(t, gateStatusRetrying, meta[metaKeyGateStatus])
}

func TestRunner_FirstFailureMarksNeedsFix(t *testing.T) {
	repoDir := t.TempDir()
	storePath := writeTaskStore(t, repoDir, 358, map[string]string{
		metaKeyGateRetryCount: "0",
	})

	markCalled := 0
	labelCalled := 0
	comments := make([]string, 0, 1)

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9010,
		RepoDir:    repoDir,
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
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.False(t, result.Passed)
	assert.False(t, result.Escalated)
	assert.Equal(t, "issue-358-pr-9010-20260219", result.AttemptKey)
	assert.Equal(t, 1, result.RetryCount)
	assert.Equal(t, 1, markCalled)
	assert.Equal(t, 0, labelCalled)
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "retry_count=1")
	assert.Contains(t, comments[0], "max_retries=2")
	assert.Contains(t, comments[0], "issue-358-pr-9010-20260219")

	meta := readIssueMetadata(t, storePath, 358)
	assert.Equal(t, "1", meta[metaKeyGateRetryCount])
	assert.Equal(t, gateStatusRetrying, meta[metaKeyGateStatus])
	assert.Equal(t, "issue-358-pr-9010-20260219", meta[metaKeyGateAttemptKey])
}

func TestRunner_ExceedRetriesEscalates(t *testing.T) {
	repoDir := t.TempDir()
	storePath := writeTaskStore(t, repoDir, 358, map[string]string{
		metaKeyGateRetryCount: "2",
	})

	markCalled := 0
	labels := make([][]string, 0, 1)
	comments := make([]string, 0, 1)

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9011,
		RepoDir:    repoDir,
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
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.True(t, result.Escalated)
	assert.Equal(t, 3, result.RetryCount)
	assert.Equal(t, 0, markCalled)
	require.Len(t, labels, 1)
	assert.Equal(t, []string{needsHumanLabel, integrationGateFailedLabel}, labels[0])
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "retry_count=3")
	assert.Contains(t, comments[0], "issue-358-pr-9011-20260219")

	meta := readIssueMetadata(t, storePath, 358)
	assert.Equal(t, "3", meta[metaKeyGateRetryCount])
	assert.Equal(t, gateStatusEscalated, meta[metaKeyGateStatus])
	assert.Equal(t, "issue-358-pr-9011-20260219", meta[metaKeyLastEscalatedAttemptKey])
}

func TestRunner_DedupEscalationCommentByAttemptKey(t *testing.T) {
	repoDir := t.TempDir()
	storePath := writeTaskStore(t, repoDir, 358, map[string]string{
		metaKeyGateRetryCount:          "3",
		metaKeyLastEscalatedAttemptKey: "issue-358-pr-9012-20260219",
	})

	labels := 0
	comments := 0

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9012,
		RepoDir:    repoDir,
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
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.True(t, result.Escalated)
	assert.True(t, result.EscalationCommentSkipped)
	assert.Equal(t, 4, result.RetryCount)
	assert.Equal(t, 1, labels)
	assert.Equal(t, 0, comments)

	meta := readIssueMetadata(t, storePath, 358)
	assert.Equal(t, "4", meta[metaKeyGateRetryCount])
	assert.Equal(t, gateStatusEscalated, meta[metaKeyGateStatus])
	assert.Equal(t, "issue-358-pr-9012-20260219", meta[metaKeyLastEscalatedAttemptKey])
}

func TestRunner_BackwardCompatibleObjectStoreFormat(t *testing.T) {
	repoDir := t.TempDir()
	storePath := writeObjectTaskStore(t, repoDir, 358)

	runner := newRunnerForTest(t, Options{
		Repo:       "biantaishabi2/Cli",
		Issue:      358,
		PR:         9020,
		RepoDir:    repoDir,
		MaxRetries: 2,
		Now:        fixedNow,
		RunGate: func(context.Context, string, int) (string, error) {
			return "gate failed", errors.New("exit status 1")
		},
		MarkNeedsFix: func(context.Context, string, int) error { return nil },
		AddLabels:    func(context.Context, string, int, []string) error { return nil },
		AddComment:   func(context.Context, string, int, string) error { return nil },
	})

	result, err := runner.Run(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrGateFailed)
	assert.Equal(t, 1, result.RetryCount)

	meta := readIssueMetadataFromObjectStore(t, storePath, 358)
	assert.Equal(t, "1", meta[metaKeyGateRetryCount])
	assert.Equal(t, gateStatusRetrying, meta[metaKeyGateStatus])
	assert.Equal(t, "issue-358-pr-9020-20260219", meta[metaKeyGateAttemptKey])
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

func TestWithTaskStoreLock_SerializesCriticalSection(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), ".niuma", "tasks.json.lock")
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)

	go func() {
		firstDone <- withTaskStoreLock(lockPath, func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()

	select {
	case <-firstEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("第一个协程未能进入临界区")
	}

	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- withTaskStoreLock(lockPath, func() error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("第二个协程在第一个协程释放锁前进入了临界区")
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseFirst)
	require.NoError(t, <-firstDone)
	require.NoError(t, <-secondDone)
}

func writeTaskStore(t *testing.T, repoDir string, issue int, extraMetadata map[string]string) string {
	t.Helper()

	storePath := filepath.Join(repoDir, ".niuma", "tasks.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(storePath), 0o755))

	metadata := map[string]any{
		metaKeyIssueNum: strconv.Itoa(issue),
	}
	for k, v := range extraMetadata {
		metadata[k] = v
	}

	payload := []map[string]any{
		{
			"id":       "task-358",
			"subject":  "issue 358",
			"status":   "in-progress",
			"metadata": metadata,
		},
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	raw = append(raw, '\n')
	require.NoError(t, os.WriteFile(storePath, raw, 0o644))
	return storePath
}

func readIssueMetadata(t *testing.T, storePath string, issue int) map[string]string {
	t.Helper()

	raw, err := os.ReadFile(storePath)
	require.NoError(t, err)

	var tasks []map[string]any
	require.NoError(t, json.Unmarshal(raw, &tasks))

	for _, task := range tasks {
		meta, ok := task["metadata"].(map[string]any)
		if !ok {
			continue
		}
		if parseNonNegativeInt(meta[metaKeyIssueNum]) != issue {
			continue
		}

		out := make(map[string]string, len(meta))
		for k, v := range meta {
			out[k] = parseString(v)
		}
		return out
	}

	t.Fatalf("未找到 issue=%d 的 metadata", issue)
	return nil
}

func writeObjectTaskStore(t *testing.T, repoDir string, issue int) string {
	t.Helper()

	storePath := filepath.Join(repoDir, ".niuma", "tasks.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(storePath), 0o755))

	payload := map[string]any{
		"version": "1",
		"tasks": map[string]any{
			"task-358": map[string]any{
				"id":      "task-358",
				"subject": "issue 358",
				"status":  "in-progress",
				"metadata": map[string]any{
					metaKeyIssueNum: strconv.Itoa(issue),
				},
			},
		},
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	raw = append(raw, '\n')
	require.NoError(t, os.WriteFile(storePath, raw, 0o644))
	return storePath
}

func readIssueMetadataFromObjectStore(t *testing.T, storePath string, issue int) map[string]string {
	t.Helper()

	raw, err := os.ReadFile(storePath)
	require.NoError(t, err)

	var root map[string]any
	require.NoError(t, json.Unmarshal(raw, &root))
	tasks, ok := root["tasks"].(map[string]any)
	require.True(t, ok)
	for _, item := range tasks {
		task, ok := item.(map[string]any)
		if !ok {
			continue
		}
		meta, ok := task["metadata"].(map[string]any)
		if !ok {
			continue
		}
		if parseNonNegativeInt(meta[metaKeyIssueNum]) != issue {
			continue
		}
		out := make(map[string]string, len(meta))
		for k, v := range meta {
			out[k] = parseString(v)
		}
		return out
	}
	t.Fatalf("未找到 object store issue=%d 的 metadata", issue)
	return nil
}
