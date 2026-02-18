package control

import (
	"context"
	"errors"
	"testing"
	"time"

	ghpkg "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDagSyncPlan_HashIdempotent(t *testing.T) {
	tasks := []Task{
		{ID: "task-40", Metadata: map[string]string{"issue_num": "40"}},
		{ID: "task-41", Metadata: map[string]string{"issue_num": "41"}},
		{ID: "task-42", Metadata: map[string]string{"issue_num": "42"}},
	}
	dagA := &DagGraph{
		Nodes: []DagNode{
			{ID: "task-41", Deps: []string{"task-40"}, Status: "pending"},
			{ID: "task-42", Deps: []string{"task-40", "task-41"}, Status: "pending"},
		},
	}
	dagB := &DagGraph{
		Nodes: []DagNode{
			{ID: "task-42", Deps: []string{"task-41", "task-40"}, Status: "pending"},
			{ID: "task-41", Deps: []string{"task-40"}, Status: "pending"},
		},
	}

	planA := buildDagSyncPlan(tasks, dagA)
	planB := buildDagSyncPlan(tasks, dagB)
	require.Equal(t, planA.edges, planB.edges)
	assert.Equal(t, planA.hash, planB.hash)
}

func TestBuildDagSyncPlan_SkippedEdgeForMissingIssueNum(t *testing.T) {
	tasks := []Task{
		{ID: "task-40", Metadata: map[string]string{"issue_num": "40"}},
		{ID: "task-41", Metadata: map[string]string{"issue_num": "41"}},
		{ID: "task-x", Metadata: map[string]string{}},
	}
	dag := &DagGraph{
		Nodes: []DagNode{
			{ID: "task-41", Deps: []string{"task-40", "task-x"}, Status: "pending"},
			{ID: "task-x", Deps: []string{"task-40"}, Status: "pending"},
		},
	}

	plan := buildDagSyncPlan(tasks, dag)
	assert.Equal(t, 3, plan.totalEdges)
	assert.Equal(t, 2, plan.skippedEdges)
	assert.Equal(t, []DagEdge{{FromIssue: 40, ToIssue: 41}}, plan.edges)
}

func TestBuildDagSyncPlan_EmptyDagNodes(t *testing.T) {
	tasks := []Task{
		{ID: "task-40", Metadata: map[string]string{"issue_num": "40"}},
		{ID: "task-41", Metadata: map[string]string{"issue_num": "41"}},
	}
	dag := &DagGraph{
		Nodes: []DagNode{},
	}

	plan := buildDagSyncPlan(tasks, dag)
	assert.Equal(t, 0, plan.totalEdges)
	assert.Equal(t, 0, plan.skippedEdges)
	assert.Empty(t, plan.edges)
	assert.NotEmpty(t, plan.hash)
}

func TestDagSync_EmptyDagNodesSyncHasHashAndNoWriteError(t *testing.T) {
	listJSON := `[
{"id":"task-40","subject":"dep","description":"dep","status":"pending","metadata":{"issue_num":"40"}},
{"id":"task-41","subject":"target","description":"target","status":"pending","metadata":{"issue_num":"41"}}
]`
	dagJSON := `{"nodes":[]}`
	taskctl := newScriptTaskCtlClient(t, listJSON, dagJSON)
	mockGH := newMockGitHubOps()
	cfg := DefaultControlConfig()
	cfg.RepoDir = t.TempDir()
	cfg.DagSync = normalizeDagSyncConfig(DagSyncConfig{
		MaxRetry:     0,
		RetryBackoff: []time.Duration{time.Millisecond},
		Timeout:      100 * time.Millisecond,
	}, cfg.RepoDir)

	ctrl := &Controller{
		taskctl: taskctl,
		github:  mockGH,
		cfg:     cfg,
	}

	result, err := ctrl.syncDagToGitHub(context.Background(), DagSyncModeEvent, false, false)
	require.NoError(t, err)
	assert.Contains(t, []DagSyncStatus{DagSyncStatusSuccess, DagSyncStatusSkipped}, result.Status)
	assert.NotEmpty(t, result.DagHash)
	assert.Equal(t, 0, result.TotalEdges)
	assert.Empty(t, mockGH.addBlockedByCalls)
	assert.Empty(t, mockGH.removeBlockedByCalls)
}

func TestDagSync_SameHashSecondRunSkippedWithoutGitHubCalls(t *testing.T) {
	listJSON := `[
{"id":"task-40","subject":"dep","description":"dep","status":"pending","metadata":{"issue_num":"40"}},
{"id":"task-41","subject":"target","description":"target","status":"pending","metadata":{"issue_num":"41"}}
]`
	dagJSON := `{"nodes":[
{"id":"task-40","deps":[],"status":"pending"},
{"id":"task-41","deps":["task-40"],"status":"pending"}
]}`
	taskctl := newScriptTaskCtlClient(t, listJSON, dagJSON)
	mockGH := newMockGitHubOps()
	cfg := DefaultControlConfig()
	cfg.RepoDir = t.TempDir()
	cfg.DagSync = normalizeDagSyncConfig(DagSyncConfig{
		MaxRetry:     0,
		RetryBackoff: []time.Duration{time.Millisecond},
		Timeout:      100 * time.Millisecond,
	}, cfg.RepoDir)

	ctrl := &Controller{
		taskctl: taskctl,
		github:  mockGH,
		cfg:     cfg,
	}

	first, err := ctrl.syncDagToGitHub(context.Background(), DagSyncModeEvent, false, false)
	require.NoError(t, err)
	assert.Equal(t, DagSyncStatusSuccess, first.Status)

	mockGH.listBlockedByCalls = 0
	mockGH.addBlockedByCalls = nil
	mockGH.removeBlockedByCalls = nil

	second, err := ctrl.syncDagToGitHub(context.Background(), DagSyncModeEvent, false, false)
	require.NoError(t, err)
	assert.Equal(t, DagSyncStatusSkipped, second.Status)
	assert.Equal(t, 0, mockGH.listBlockedByCalls)
	assert.Empty(t, mockGH.addBlockedByCalls)
	assert.Empty(t, mockGH.removeBlockedByCalls)
}

func TestDagSync_RateLimitRetryAndFail(t *testing.T) {
	listJSON := `[
{"id":"task-40","subject":"dep","description":"dep","status":"pending","metadata":{"issue_num":"40"}},
{"id":"task-41","subject":"target","description":"target","status":"pending","metadata":{"issue_num":"41"}}
]`
	dagJSON := `{"nodes":[
{"id":"task-40","deps":[],"status":"pending"},
{"id":"task-41","deps":["task-40"],"status":"pending"}
]}`
	taskctl := newScriptTaskCtlClient(t, listJSON, dagJSON)
	mockGH := newMockGitHubOps()
	mockGH.addBlockedByErr["40->41"] = &ghpkg.DependencyError{
		Operation: "add blocked_by",
		Type:      ghpkg.DependencyErrorTypeRateLimit,
		Err:       errors.New("429"),
	}

	cfg := DefaultControlConfig()
	cfg.RepoDir = t.TempDir()
	cfg.DagSync = normalizeDagSyncConfig(DagSyncConfig{
		MaxRetry:     2,
		RetryBackoff: []time.Duration{time.Millisecond},
		Timeout:      100 * time.Millisecond,
	}, cfg.RepoDir)

	ctrl := &Controller{
		taskctl: taskctl,
		github:  mockGH,
		cfg:     cfg,
	}

	result, err := ctrl.syncDagToGitHub(context.Background(), DagSyncModeEvent, false, false)
	require.Error(t, err)
	assert.Equal(t, DagSyncStatusFailed, result.Status)
	assert.Equal(t, ghpkg.DependencyErrorTypeRateLimit, result.ErrorType)
	assert.Len(t, mockGH.addBlockedByCalls, 3)
}

func TestDagSync_RetryCoverageForClassifiedTypes(t *testing.T) {
	retryTypes := []string{
		ghpkg.DependencyErrorTypeAuth,
		ghpkg.DependencyErrorTypePermission,
		ghpkg.DependencyErrorTypeRateLimit,
		ghpkg.DependencyErrorTypeNetworkTimeout,
		ghpkg.DependencyErrorTypeUnsupported,
	}

	for _, errType := range retryTypes {
		t.Run(errType, func(t *testing.T) {
			listJSON := `[
{"id":"task-40","subject":"dep","description":"dep","status":"pending","metadata":{"issue_num":"40"}},
{"id":"task-41","subject":"target","description":"target","status":"pending","metadata":{"issue_num":"41"}}
]`
			dagJSON := `{"nodes":[
{"id":"task-40","deps":[],"status":"pending"},
{"id":"task-41","deps":["task-40"],"status":"pending"}
]}`
			taskctl := newScriptTaskCtlClient(t, listJSON, dagJSON)
			mockGH := newMockGitHubOps()
			mockGH.addBlockedByErr["40->41"] = &ghpkg.DependencyError{
				Operation: "add blocked_by",
				Type:      errType,
				Err:       errors.New("mock retry error"),
			}
			cfg := DefaultControlConfig()
			cfg.RepoDir = t.TempDir()
			cfg.DagSync = normalizeDagSyncConfig(DagSyncConfig{
				MaxRetry:     2,
				RetryBackoff: []time.Duration{time.Millisecond},
				Timeout:      100 * time.Millisecond,
			}, cfg.RepoDir)

			ctrl := &Controller{
				taskctl: taskctl,
				github:  mockGH,
				cfg:     cfg,
			}

			result, err := ctrl.syncDagToGitHub(context.Background(), DagSyncModeEvent, false, false)
			require.Error(t, err)
			assert.Equal(t, DagSyncStatusFailed, result.Status)
			assert.Equal(t, errType, result.ErrorType)
			assert.Len(t, mockGH.addBlockedByCalls, 3)
		})
	}
}

func TestDagSync_ReconcileDetectsDriftAndCorrects(t *testing.T) {
	listJSON := `[
{"id":"task-40","subject":"dep","description":"dep","status":"pending","metadata":{"issue_num":"40"}},
{"id":"task-41","subject":"target","description":"target","status":"pending","metadata":{"issue_num":"41"}}
]`
	dagJSON := `{"nodes":[
{"id":"task-40","deps":[],"status":"pending"},
{"id":"task-41","deps":["task-40"],"status":"pending"}
]}`
	taskctl := newScriptTaskCtlClient(t, listJSON, dagJSON)
	mockGH := newMockGitHubOps()
	cfg := DefaultControlConfig()
	cfg.RepoDir = t.TempDir()
	cfg.DagSync = normalizeDagSyncConfig(DagSyncConfig{
		MaxRetry:     0,
		RetryBackoff: []time.Duration{time.Millisecond},
		Timeout:      100 * time.Millisecond,
	}, cfg.RepoDir)

	ctrl := &Controller{
		taskctl: taskctl,
		github:  mockGH,
		cfg:     cfg,
	}

	first, err := ctrl.syncDagToGitHub(context.Background(), DagSyncModeEvent, false, false)
	require.NoError(t, err)
	assert.Equal(t, DagSyncStatusSuccess, first.Status)

	mockGH.blockedBy[41] = map[int]struct{}{42: {}}
	mockGH.addBlockedByCalls = nil
	mockGH.removeBlockedByCalls = nil

	second, err := ctrl.syncDagToGitHub(context.Background(), DagSyncModeReconcile, false, false)
	require.NoError(t, err)
	assert.Equal(t, DagSyncStatusSuccess, second.Status)
	assert.Equal(t, 1, second.AppliedAdd)
	assert.Equal(t, 1, second.AppliedRemove)
	assert.Equal(t, map[int]struct{}{40: {}}, mockGH.blockedBy[41])
}

func TestDagSync_DryRunDoesNotPersistStateAndDoesNotBlockRealSync(t *testing.T) {
	listJSON := `[
{"id":"task-40","subject":"dep","description":"dep","status":"pending","metadata":{"issue_num":"40"}},
{"id":"task-41","subject":"target","description":"target","status":"pending","metadata":{"issue_num":"41"}}
]`
	dagJSON := `{"nodes":[
{"id":"task-40","deps":[],"status":"pending"},
{"id":"task-41","deps":["task-40"],"status":"pending"}
]}`
	taskctl := newScriptTaskCtlClient(t, listJSON, dagJSON)
	mockGH := newMockGitHubOps()
	cfg := DefaultControlConfig()
	cfg.RepoDir = t.TempDir()
	cfg.DagSync = normalizeDagSyncConfig(DagSyncConfig{
		MaxRetry:     0,
		RetryBackoff: []time.Duration{time.Millisecond},
		Timeout:      100 * time.Millisecond,
	}, cfg.RepoDir)

	ctrl := &Controller{
		taskctl: taskctl,
		github:  mockGH,
		cfg:     cfg,
	}

	dryRunResult, err := ctrl.syncDagToGitHub(context.Background(), DagSyncModeManual, false, true)
	require.NoError(t, err)
	assert.Equal(t, DagSyncStatusSuccess, dryRunResult.Status)
	assert.Equal(t, 1, dryRunResult.AppliedAdd)
	assert.Equal(t, 0, dryRunResult.AppliedRemove)
	assert.Empty(t, mockGH.addBlockedByCalls)

	store := newDagSyncStateStore(cfg.DagSync.StateFile)
	stateAfterDryRun, loadErr := store.Load()
	require.NoError(t, loadErr)
	assert.Empty(t, stateAfterDryRun.LastHash)
	assert.Empty(t, stateAfterDryRun.LastSuccessAt)
	assert.Equal(t, 0, stateAfterDryRun.SuccessCount)

	realResult, err := ctrl.syncDagToGitHub(context.Background(), DagSyncModeEvent, false, false)
	require.NoError(t, err)
	assert.Equal(t, DagSyncStatusSuccess, realResult.Status)
	assert.Len(t, mockGH.addBlockedByCalls, 1)
}

func TestWithDagSyncRetry_RetryCoverageForClassifiedTypes(t *testing.T) {
	cfg := DefaultControlConfig()
	cfg.RepoDir = t.TempDir()
	cfg.DagSync = normalizeDagSyncConfig(DagSyncConfig{
		MaxRetry:     2,
		RetryBackoff: []time.Duration{time.Millisecond},
		Timeout:      100 * time.Millisecond,
	}, cfg.RepoDir)
	ctrl := &Controller{cfg: cfg}

	retryTypes := []string{
		ghpkg.DependencyErrorTypeAuth,
		ghpkg.DependencyErrorTypePermission,
		ghpkg.DependencyErrorTypeRateLimit,
		ghpkg.DependencyErrorTypeNetworkTimeout,
		ghpkg.DependencyErrorTypeUnsupported,
	}
	for _, errType := range retryTypes {
		attempts := 0
		err := ctrl.withDagSyncRetry(context.Background(), "retry-test", func(context.Context) error {
			attempts++
			return &ghpkg.DependencyError{
				Operation: "retry-test",
				Type:      errType,
				Err:       errors.New("mock error"),
			}
		})
		require.Error(t, err)
		assert.Equal(t, 3, attempts, "error type should be retried: %s", errType)
	}

	noRetryAttempts := 0
	err := ctrl.withDagSyncRetry(context.Background(), "retry-test", func(context.Context) error {
		noRetryAttempts++
		return &ghpkg.DependencyError{
			Operation: "retry-test",
			Type:      ghpkg.DependencyErrorTypeUnknown,
			Err:       errors.New("unknown"),
		}
	})
	require.Error(t, err)
	assert.Equal(t, 1, noRetryAttempts)
}

func TestShouldRunDagReconcile(t *testing.T) {
	assert.True(t, shouldRunDagReconcile(DagSyncState{}, 5*time.Minute))

	now := time.Now().UTC()
	state := DagSyncState{LastReconcileAt: now.Format(time.RFC3339)}
	assert.False(t, shouldRunDagReconcile(state, 5*time.Minute))

	state = DagSyncState{LastReconcileAt: now.Add(-10 * time.Minute).Format(time.RFC3339)}
	assert.True(t, shouldRunDagReconcile(state, 5*time.Minute))
}
