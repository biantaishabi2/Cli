package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	ghapi "github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractIntegratedIssueNumbers(t *testing.T) {
	messages := []string{
		"Merge feat/214-close (issue #214)",
		"Merge feat/215-parent (issue #215)\n\nextra",
		"chore: update docs",
		"Merge feat/214-close (issue #214)",
	}

	got := extractIntegratedIssueNumbers("", "", messages)
	assert.Equal(t, []int{214, 215}, got)
}

func TestExtractIntegratedIssueNumbers_FromPRTextAndCommits(t *testing.T) {
	prTitle := "fix: sub(#25): Gateway Session Coordinator (#25)"
	prBody := "Closes #25\n\nparent(#24)"
	messages := []string{
		"Merge pull request #300 from foo/bar",
		"chore: update docs",
		"fix: test (#215)",
	}

	got := extractIntegratedIssueNumbers(prTitle, prBody, messages)
	assert.Equal(t, []int{24, 25, 215}, got)
}

func TestExtractIntegratedIssueNumbers_IgnorePRNumber(t *testing.T) {
	messages := []string{
		"Merge pull request #300 from foo/bar",
	}

	got := extractIntegratedIssueNumbers("", "", messages)
	assert.Empty(t, got)
}

type premergedMarkerMock struct {
	issueNums []int
	trigger   string
	reason    string
	callCount int
}

func (m *premergedMarkerMock) MarkIssuesPremerged(_ context.Context, issueNums []int, trigger, reason string) error {
	m.callCount++
	m.issueNums = append([]int(nil), issueNums...)
	m.trigger = trigger
	m.reason = reason
	return nil
}

func TestApplyPremergedTransitionFromEvent_TransitionsMatchedIssues(t *testing.T) {
	flagRepo = ""
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	eventPath := filepath.Join(t.TempDir(), "event.json")
	require.NoError(t, os.WriteFile(eventPath, []byte(`{
  "action": "closed",
  "pull_request": {
    "number": 501,
    "title": "feat: stabilize adapter (refs #321)",
    "body": "Closes #322",
    "merged": true,
    "base": {"ref": "integration/main"}
  }
}`), 0o644))
	t.Setenv("GITHUB_EVENT_PATH", eventPath)

	marker := &premergedMarkerMock{}

	err := applyPremergedTransitionFromEvent(context.Background(), marker)
	require.NoError(t, err)
	assert.Equal(t, 1, marker.callCount)
	assert.Equal(t, []int{321, 322}, marker.issueNums)
	assert.Equal(t, "pull_request.closed", marker.trigger)
	assert.Equal(t, "merged_to_integration_main", marker.reason)
}

func TestApplyPremergedTransitionFromEvent_SkipsNonIntegrationMain(t *testing.T) {
	flagRepo = ""
	t.Setenv("GITHUB_EVENT_NAME", "pull_request")
	eventPath := filepath.Join(t.TempDir(), "event.json")
	require.NoError(t, os.WriteFile(eventPath, []byte(`{
  "action": "closed",
  "pull_request": {
    "number": 502,
    "title": "feat: stabilize adapter (refs #321)",
    "body": "",
    "merged": true,
    "base": {"ref": "feature/x"}
  }
}`), 0o644))
	t.Setenv("GITHUB_EVENT_PATH", eventPath)

	marker := &premergedMarkerMock{}

	err := applyPremergedTransitionFromEvent(context.Background(), marker)
	require.NoError(t, err)
	assert.Equal(t, 0, marker.callCount)
}

func TestGitHubControlOps_ResolvePRMetadata_Success(t *testing.T) {
	client := &stubGitHubControlClient{
		findMarkerResp: &gh.MarkerComment{
			Marker: &marker.Marker{
				Type:  marker.TypePRCreated,
				Issue: 321,
				PR:    123,
			},
		},
		prs: map[int]*ghapi.PullRequest{
			123: {
				State: ghapi.Ptr("open"),
				Head: &ghapi.PullRequestBranch{
					Ref: ghapi.Ptr("feat/321-fix"),
				},
			},
		},
	}
	ops := &gitHubControlOps{client: client}

	metadata, err := ops.ResolvePRMetadata(context.Background(), 321)
	require.NoError(t, err)
	assert.Equal(t, control.PRMetadata{PRNum: 123, Branch: "feat/321-fix"}, metadata)
}

func TestGitHubControlOps_ResolvePRMetadata_MarkerNotFound(t *testing.T) {
	client := &stubGitHubControlClient{}
	ops := &gitHubControlOps{client: client}

	_, err := ops.ResolvePRMetadata(context.Background(), 321)
	require.Error(t, err)
	assert.ErrorIs(t, err, control.ErrPRMarkerNotFound)
}

func TestGitHubControlOps_ResolvePRMetadata_PRClosed(t *testing.T) {
	client := &stubGitHubControlClient{
		findMarkerResp: &gh.MarkerComment{
			Marker: &marker.Marker{
				Type:  marker.TypePRCreated,
				Issue: 321,
				PR:    123,
			},
		},
		prs: map[int]*ghapi.PullRequest{
			123: {
				State: ghapi.Ptr("closed"),
				Head: &ghapi.PullRequestBranch{
					Ref: ghapi.Ptr("feat/321-fix"),
				},
			},
		},
	}
	ops := &gitHubControlOps{client: client}

	_, err := ops.ResolvePRMetadata(context.Background(), 321)
	require.Error(t, err)
	assert.ErrorIs(t, err, control.ErrPRClosed)
}

func TestGitHubControlOps_ResolvePRMetadata_BranchUnavailable(t *testing.T) {
	client := &stubGitHubControlClient{
		findMarkerResp: &gh.MarkerComment{
			Marker: &marker.Marker{
				Type:  marker.TypePRCreated,
				Issue: 321,
				PR:    123,
			},
		},
		prs: map[int]*ghapi.PullRequest{
			123: {
				State: ghapi.Ptr("open"),
				Head: &ghapi.PullRequestBranch{
					Ref: ghapi.Ptr("  "),
				},
			},
		},
	}
	ops := &gitHubControlOps{client: client}

	_, err := ops.ResolvePRMetadata(context.Background(), 321)
	require.Error(t, err)
	assert.ErrorIs(t, err, control.ErrPRBranchUnavailable)
}

func TestGitHubControlOps_ResolvePRMetadata_APIError(t *testing.T) {
	client := &stubGitHubControlClient{
		findMarkerResp: &gh.MarkerComment{
			Marker: &marker.Marker{
				Type:  marker.TypePRCreated,
				Issue: 321,
				PR:    123,
			},
		},
		prErr: map[int]error{
			123: errors.New("github api down"),
		},
	}
	ops := &gitHubControlOps{client: client}

	_, err := ops.ResolvePRMetadata(context.Background(), 321)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github api down")
}

func TestGitHubControlOps_ResolvePRReviewStatus_Conflicting(t *testing.T) {
	client := &stubGitHubControlClient{
		findMarkerResp: &gh.MarkerComment{
			Marker: &marker.Marker{
				Type:  marker.TypePRCreated,
				Issue: 321,
				PR:    123,
			},
		},
		prs: map[int]*ghapi.PullRequest{
			123: {
				State:          ghapi.Ptr("open"),
				MergeableState: ghapi.Ptr("dirty"),
				Head: &ghapi.PullRequestBranch{
					Ref: ghapi.Ptr("feat/321-fix"),
					SHA: ghapi.Ptr("abc123"),
				},
			},
		},
	}
	ops := &gitHubControlOps{client: client}

	status, err := ops.ResolvePRReviewStatus(context.Background(), 321)
	require.NoError(t, err)
	assert.Equal(t, control.PRMergeableConflicting, status.Mergeable)
	assert.Equal(t, "DIRTY", status.MergeStateStatus)
	assert.Equal(t, "abc123", status.HeadSHA)
}

func TestGitHubControlOps_ResolvePRReviewStatus_MergeableFalseIsConflicting(t *testing.T) {
	client := &stubGitHubControlClient{
		findMarkerResp: &gh.MarkerComment{
			Marker: &marker.Marker{
				Type:  marker.TypePRCreated,
				Issue: 321,
				PR:    123,
			},
		},
		prs: map[int]*ghapi.PullRequest{
			123: {
				State:          ghapi.Ptr("open"),
				Mergeable:      ghapi.Ptr(false),
				MergeableState: ghapi.Ptr("clean"),
				Head: &ghapi.PullRequestBranch{
					Ref: ghapi.Ptr("feat/321-fix"),
					SHA: ghapi.Ptr("abc123"),
				},
			},
		},
	}
	ops := &gitHubControlOps{client: client}

	status, err := ops.ResolvePRReviewStatus(context.Background(), 321)
	require.NoError(t, err)
	assert.Equal(t, control.PRMergeableConflicting, status.Mergeable)
	assert.Equal(t, "CLEAN", status.MergeStateStatus)
}

func TestResolveIntegrationGateMaxRetries(t *testing.T) {
	value, err := resolveIntegrationGateMaxRetries(-1, "")
	require.NoError(t, err)
	assert.Equal(t, 2, value)

	value, err = resolveIntegrationGateMaxRetries(-1, "4")
	require.NoError(t, err)
	assert.Equal(t, 4, value)

	value, err = resolveIntegrationGateMaxRetries(6, "4")
	require.NoError(t, err)
	assert.Equal(t, 6, value)
}

func TestResolveIntegrationGateMaxRetries_InvalidEnv(t *testing.T) {
	_, err := resolveIntegrationGateMaxRetries(-1, "bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NIUMA_INTEGRATION_GATE_MAX_RETRIES")
}

func TestParseIssueNumberList(t *testing.T) {
	issues, err := parseIssueNumberList("40, 41,40, 42")
	require.NoError(t, err)
	assert.Equal(t, []int{40, 41, 42}, issues)
}

func TestParseIssueNumberList_Invalid(t *testing.T) {
	_, err := parseIssueNumberList("40,foo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效的 issue 编号")

	_, err = parseIssueNumberList("0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效的 issue 编号")
}

func TestResolvePRConflictRetryThreshold(t *testing.T) {
	value, err := resolvePRConflictRetryThreshold(-1, "")
	require.NoError(t, err)
	assert.Equal(t, 3, value)

	value, err = resolvePRConflictRetryThreshold(-1, "5")
	require.NoError(t, err)
	assert.Equal(t, 5, value)

	value, err = resolvePRConflictRetryThreshold(2, "5")
	require.NoError(t, err)
	assert.Equal(t, 2, value)
}

func TestResolvePRConflictRetryThreshold_InvalidEnv(t *testing.T) {
	_, err := resolvePRConflictRetryThreshold(-1, "-2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NIUMA_PR_CONFLICT_RETRY_THRESHOLD")
}

func TestResolvePRConflictUnknownBackoffs(t *testing.T) {
	backoffs, err := resolvePRConflictUnknownBackoffs("", "")
	require.NoError(t, err)
	assert.Equal(t, []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}, backoffs)

	backoffs, err = resolvePRConflictUnknownBackoffs("", "1s,2s")
	require.NoError(t, err)
	assert.Equal(t, []time.Duration{1 * time.Second, 2 * time.Second}, backoffs)

	backoffs, err = resolvePRConflictUnknownBackoffs("3s,4s", "1s,2s")
	require.NoError(t, err)
	assert.Equal(t, []time.Duration{3 * time.Second, 4 * time.Second}, backoffs)
}

func TestResolvePRConflictUnknownBackoffs_InvalidInput(t *testing.T) {
	_, err := resolvePRConflictUnknownBackoffs("", "bad")
	require.Error(t, err)

	_, err = resolvePRConflictUnknownBackoffs("", "1s,-2s")
	require.Error(t, err)
}

func TestControlRunFlags_PRConflictLayeredOptionsExist(t *testing.T) {
	enableAI := controlRunCmd.Flags().Lookup("pr-conflict-enable-ai")
	require.NotNil(t, enableAI)
	assert.Equal(t, "true", enableAI.DefValue)

	maxAttempts := controlRunCmd.Flags().Lookup("pr-conflict-ai-max-attempts")
	require.NotNil(t, maxAttempts)
	assert.Equal(t, "2", maxAttempts.DefValue)

	smoke := controlRunCmd.Flags().Lookup("pr-conflict-smoke-test-cmd")
	require.NotNil(t, smoke)
	assert.Equal(t, "", smoke.DefValue)
}

func TestControlRunFlags_ProfileExists(t *testing.T) {
	f := controlRunCmd.Flags().Lookup("profile")
	require.NotNil(t, f)
	assert.Equal(t, "", f.DefValue)
}

func TestResolveProfileFlag(t *testing.T) {
	cases := []struct {
		name     string
		flagVal  string
		envVal   string
		expected string
	}{
		{"默认 auto", "", "", "auto"},
		{"flag 优先", "go,rust", "elixir", "go,rust"},
		{"env 降级", "", "none", "none"},
		{"flag=none", "none", "auto", "none"},
		{"空格 flag 忽略", "  ", "elixir", "elixir"},
		{"空格 env 忽略", "", "  ", "auto"},
		{"flag=auto 显式设置优先于 env", "auto", "go,rust", "auto"},
		{"flag=auto 无 env", "auto", "", "auto"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveProfileFlag(tc.flagVal, tc.envVal)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestWorkflowGateStatusJQ_ObjectTasksPending(t *testing.T) {
	status := runWorkflowGateStatusJQ(t, `{"version":"1","tasks":{"a":{"metadata":{"integration_gate_status":"pending"}}}}`)
	assert.Equal(t, "pending", status)
}

func TestWorkflowGateStatusJQ_DefaultPassedWhenStatusesEmpty(t *testing.T) {
	status := runWorkflowGateStatusJQ(t, `{"version":"1","tasks":{"a":{"metadata":{}},"b":{"metadata":{}}}}`)
	assert.Equal(t, "passed", status)
}

func TestWorkflowGateStatusJQ_DefaultPassedWhenTasksMissing(t *testing.T) {
	status := runWorkflowGateStatusJQ(t, `{"version":"1"}`)
	assert.Equal(t, "passed", status)
}

func TestWorkflowGateStatusJQ_PriorityEscalatedFirst(t *testing.T) {
	status := runWorkflowGateStatusJQ(t, `{"version":"1","tasks":{"a":{"metadata":{"integration_gate_status":"passed"}},"b":{"metadata":{"integration_gate_status":"pending"}},"c":{"metadata":{"integration_gate_status":"escalated"}}}}`)
	assert.Equal(t, "escalated", status)
}

func runWorkflowGateStatusJQ(t *testing.T, tasksJSON string) string {
	t.Helper()

	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not installed")
	}

	storePath := filepath.Join(t.TempDir(), "tasks.json")
	require.NoError(t, os.WriteFile(storePath, []byte(tasksJSON), 0o644))

	expr := `
      [ (.tasks // {}) | to_entries[] | .value.metadata.integration_gate_status // empty ] as $s |
      if ($s | index("escalated")) then "escalated"
      elif ($s | index("retrying")) then "retrying"
      elif ($s | index("pending")) then "pending"
      elif ($s | index("passed")) then "passed"
      else "passed" end
    `
	out, err := exec.Command("jq", "-r", expr, storePath).CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

func TestGenerateEventID_WithRunID(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "12345")
	t.Setenv("GITHUB_RUN_ATTEMPT", "2")

	eventID, source := generateEventID(50)
	assert.Equal(t, "pr-50-run-12345-2", eventID)
	assert.Equal(t, "run_id", source)
}

func TestGenerateEventID_WithRunIDDefaultAttempt(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "99999")
	t.Setenv("GITHUB_RUN_ATTEMPT", "")

	eventID, source := generateEventID(42)
	assert.Equal(t, "pr-42-run-99999-1", eventID)
	assert.Equal(t, "run_id", source)
}

func TestGenerateEventID_FallbackTimestamp(t *testing.T) {
	t.Setenv("GITHUB_RUN_ID", "")
	t.Setenv("GITHUB_RUN_ATTEMPT", "")

	eventID, source := generateEventID(50)
	assert.True(t, strings.HasPrefix(eventID, "pr-50-ts-"), "event_id should start with pr-50-ts-, got: %s", eventID)
	assert.Equal(t, "timestamp", source)
}

func TestDispatchWakeupFlag_Exists(t *testing.T) {
	f := controlCloseMergedCmd.Flags().Lookup("dispatch-wakeup")
	require.NotNil(t, f)
	assert.Equal(t, "false", f.DefValue)
}

// stubDispatchSender 模拟 dispatch 操作
type stubDispatchSender struct {
	called      bool
	eventType   string
	payload     map[string]interface{}
	dispatchErr error
}

func (s *stubDispatchSender) CreateRepositoryDispatch(_ context.Context, eventType string, clientPayload map[string]interface{}) error {
	s.called = true
	s.eventType = eventType
	s.payload = clientPayload
	return s.dispatchErr
}

func TestDispatchTaskCompleted_Success(t *testing.T) {
	// 场景 5：dispatch payload 正常发送
	t.Setenv("GITHUB_RUN_ID", "123")
	t.Setenv("GITHUB_RUN_ATTEMPT", "1")

	sender := &stubDispatchSender{}
	ctx := context.Background()
	err := dispatchTaskCompleted(ctx, sender, 50, []int{10, 11})
	require.NoError(t, err)

	assert.True(t, sender.called, "dispatch should be called")
	assert.Equal(t, "niuma.task.completed", sender.eventType)

	// 验证 payload 字段
	assert.Equal(t, 10, sender.payload["source_issue"])
	assert.Equal(t, []int{10, 11}, sender.payload["source_issues"])
	assert.Equal(t, 50, sender.payload["trigger_pr"])
	assert.Equal(t, "close-after-integration-merge", sender.payload["event_source"])
	assert.Equal(t, "pr-50-run-123-1", sender.payload["event_id"])
	assert.Equal(t, "run_id", sender.payload["event_id_source"])
	assert.NotEmpty(t, sender.payload["completed_at"])
}

func TestDispatchTaskCompleted_APIFailureWarningOnly(t *testing.T) {
	// 场景 7：API 失败仅返回 error（调用方 log warning），不影响主流程
	t.Setenv("GITHUB_RUN_ID", "456")
	t.Setenv("GITHUB_RUN_ATTEMPT", "1")

	sender := &stubDispatchSender{
		dispatchErr: errors.New("403 Forbidden"),
	}
	ctx := context.Background()
	err := dispatchTaskCompleted(ctx, sender, 50, []int{10})

	// dispatchTaskCompleted 返回 error，但调用方（runControlCloseMerged）仅 warning 不 exit
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "403 Forbidden")
	// 确认 dispatch 确实被调用了
	assert.True(t, sender.called)
}

func TestHandleCloseMergedDispatch_APIFailure_WarningOnly(t *testing.T) {
	// 场景 7 命令入口级验证：handleCloseMergedDispatch（runControlCloseMerged 调用）
	// 即使 dispatch API 失败，也不 panic/不返回 error，主流程退出码不受影响
	t.Setenv("GITHUB_RUN_ID", "789")
	t.Setenv("GITHUB_RUN_ATTEMPT", "1")

	sender := &stubDispatchSender{
		dispatchErr: errors.New("403 Forbidden"),
	}
	ctx := context.Background()

	// handleCloseMergedDispatch 不返回 error（warning-only 语义），不应 panic
	assert.NotPanics(t, func() {
		handleCloseMergedDispatch(ctx, sender, 50, []int{10, 11})
	})
	// 确认 dispatch 确实被调用了
	assert.True(t, sender.called)
}

func TestHandleCloseMergedDispatch_Success(t *testing.T) {
	// handleCloseMergedDispatch 成功路径
	t.Setenv("GITHUB_RUN_ID", "100")
	t.Setenv("GITHUB_RUN_ATTEMPT", "1")

	sender := &stubDispatchSender{}
	ctx := context.Background()

	assert.NotPanics(t, func() {
		handleCloseMergedDispatch(ctx, sender, 50, []int{10})
	})
	assert.True(t, sender.called)
	assert.Equal(t, "niuma.task.completed", sender.eventType)
}

func TestDispatchTaskCompleted_TimestampFallback(t *testing.T) {
	// 场景 6：无 GITHUB_RUN_ID 时降级为 timestamp
	t.Setenv("GITHUB_RUN_ID", "")
	t.Setenv("GITHUB_RUN_ATTEMPT", "")

	sender := &stubDispatchSender{}
	ctx := context.Background()
	err := dispatchTaskCompleted(ctx, sender, 50, []int{10})
	require.NoError(t, err)

	assert.True(t, sender.called)
	eventID, ok := sender.payload["event_id"].(string)
	assert.True(t, ok)
	assert.True(t, strings.HasPrefix(eventID, "pr-50-ts-"), "event_id should start with pr-50-ts-, got: %s", eventID)
	assert.Equal(t, "timestamp", sender.payload["event_id_source"])
}

func TestControlMergeBaseBranchFlag_Exists(t *testing.T) {
	mergeFlag := controlMergeCmd.Flags().Lookup("merge-base-branch")
	require.NotNil(t, mergeFlag)
	assert.Equal(t, "", mergeFlag.DefValue)

	closeFlag := controlCloseMergedCmd.Flags().Lookup("merge-base-branch")
	require.NotNil(t, closeFlag)
	assert.Equal(t, "", closeFlag.DefValue)
}

func TestControlMergeBaseBranchFlag_ParseConsistency(t *testing.T) {
	orig := flagMergeBaseBranch
	defer func() {
		flagMergeBaseBranch = orig
		_ = controlMergeCmd.Flags().Set("merge-base-branch", orig)
		_ = controlCloseMergedCmd.Flags().Set("merge-base-branch", orig)
	}()

	require.NoError(t, controlMergeCmd.Flags().Set("merge-base-branch", "release"))
	assert.Equal(t, "release", flagMergeBaseBranch)

	require.NoError(t, controlCloseMergedCmd.Flags().Set("merge-base-branch", "main"))
	assert.Equal(t, "main", flagMergeBaseBranch)
}

func TestResolveControlMergeBaseBranch_Priority(t *testing.T) {
	orig := flagMergeBaseBranch
	defer func() { flagMergeBaseBranch = orig }()

	provider := &stubMergeBaseDefaultBranchProvider{branch: "main"}
	executor := &stubMergeBaseGitExecutor{
		results: map[string]stubMergeBaseExecResult{
			"remote show origin":                    {out: "  HEAD branch: develop\n"},
			"symbolic-ref refs/remotes/origin/HEAD": {out: "refs/remotes/origin/develop\n"},
		},
	}
	resolver := control.NewMergeBaseResolverWithExecutor(t.TempDir(), provider, executor)

	flagMergeBaseBranch = "release"
	result := resolveControlMergeBaseBranchWithResolver(context.Background(), resolver, "config-branch")
	assert.Equal(t, "release", result.Branch)
	assert.Equal(t, "cli-flag", result.Source)

	flagMergeBaseBranch = ""
	result = resolveControlMergeBaseBranchWithResolver(context.Background(), resolver, "config-branch")
	assert.Equal(t, "config-branch", result.Branch)
	assert.Equal(t, "config", result.Source)

	result = resolveControlMergeBaseBranchWithResolver(context.Background(), resolver, "")
	assert.Equal(t, "main", result.Branch)
	assert.Equal(t, "github-default-branch", result.Source)

	provider.err = errors.New("github api unavailable")
	executor.results = map[string]stubMergeBaseExecResult{
		"remote show origin":                    {err: errors.New("remote show failed")},
		"symbolic-ref refs/remotes/origin/HEAD": {err: errors.New("origin head missing")},
	}
	result = resolveControlMergeBaseBranchWithResolver(context.Background(), resolver, "")
	assert.Equal(t, "master", result.Branch)
	assert.Equal(t, "fallback-master", result.Source)
	assert.Contains(t, result.Warning, "fallback=master")
}

func TestIsCloseMergedBaseRef(t *testing.T) {
	assert.True(t, control.IsCloseMergedBaseRef("main", "main"))
	assert.True(t, control.IsCloseMergedBaseRef("integration/main", "main"))
	assert.True(t, control.IsCloseMergedBaseRef("integration/feature-x", "main"))
	assert.True(t, control.IsCloseMergedBaseRef(" integration/main ", "main"))
	assert.False(t, control.IsCloseMergedBaseRef("master", "main"))
	assert.False(t, control.IsCloseMergedBaseRef("feature/foo", "main"))
	assert.False(t, control.IsCloseMergedBaseRef("", "main"))
}

type stubMergeBaseDefaultBranchProvider struct {
	branch string
	err    error
}

func (s *stubMergeBaseDefaultBranchProvider) GetDefaultBranch(_ context.Context) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.branch, nil
}

type stubMergeBaseExecResult struct {
	out string
	err error
}

type stubMergeBaseGitExecutor struct {
	results map[string]stubMergeBaseExecResult
}

func (s *stubMergeBaseGitExecutor) CombinedOutput(_ string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	result, ok := s.results[key]
	if !ok {
		return nil, errors.New("unexpected git command: " + key)
	}
	return []byte(result.out), result.err
}

type stubGitHubControlClient struct {
	findMarkerResp *gh.MarkerComment
	findMarkerErr  error
	prs            map[int]*ghapi.PullRequest
	prErr          map[int]error
	openPRByRef    map[string]*ghapi.PullRequest
	nextPRNumber   int
}

func (s *stubGitHubControlClient) ListIssuesWithLabel(_ context.Context, _ string) ([]*ghapi.Issue, error) {
	return nil, nil
}

func (s *stubGitHubControlClient) ListIssuesByState(_ context.Context, _ string) ([]*ghapi.Issue, error) {
	return nil, nil
}

func (s *stubGitHubControlClient) ListLabels(_ context.Context, _ int) ([]string, error) {
	return nil, nil
}

func (s *stubGitHubControlClient) AddLabel(_ context.Context, _ int, _ string) error {
	return nil
}

func (s *stubGitHubControlClient) GetIssue(_ context.Context, _ int) (*ghapi.Issue, error) {
	return nil, nil
}

func (s *stubGitHubControlClient) UpdateIssueBody(_ context.Context, _ int, _ string) error {
	return nil
}

func (s *stubGitHubControlClient) ListComments(_ context.Context, _ int) ([]*ghapi.IssueComment, error) {
	return nil, nil
}

func (s *stubGitHubControlClient) AddComment(_ context.Context, _ int, _ string) (*ghapi.IssueComment, error) {
	return &ghapi.IssueComment{}, nil
}

func (s *stubGitHubControlClient) CloseIssue(_ context.Context, _ int) error {
	return nil
}

func (s *stubGitHubControlClient) MergePR(_ context.Context, _ int, _ string) error {
	return nil
}

func (s *stubGitHubControlClient) ReplaceLabel(_ context.Context, _ int, _, _ string) error {
	return nil
}

func (s *stubGitHubControlClient) ReplaceLabelIfPresent(_ context.Context, _ int, _, _ string) (bool, error) {
	return false, nil
}

func (s *stubGitHubControlClient) ReplaceLabels(_ context.Context, _ int, _ []string) error {
	return nil
}

func (s *stubGitHubControlClient) ListIssueBlockedBy(_ context.Context, _ int) ([]int, error) {
	return nil, nil
}

func (s *stubGitHubControlClient) AddIssueBlockedBy(_ context.Context, _, _ int) error {
	return nil
}

func (s *stubGitHubControlClient) RemoveIssueBlockedBy(_ context.Context, _, _ int) error {
	return nil
}

func (s *stubGitHubControlClient) FindMarker(_ context.Context, _ int, _ marker.Type) (*gh.MarkerComment, error) {
	if s.findMarkerErr != nil {
		return nil, s.findMarkerErr
	}
	return s.findMarkerResp, nil
}

func (s *stubGitHubControlClient) GetPR(_ context.Context, number int) (*ghapi.PullRequest, error) {
	if err, ok := s.prErr[number]; ok {
		return nil, err
	}
	if pr, ok := s.prs[number]; ok {
		return pr, nil
	}
	return nil, errors.New("pr not found")
}

func (s *stubGitHubControlClient) FindOpenPR(_ context.Context, head, base string) (*ghapi.PullRequest, error) {
	key := head + "->" + base
	if s.openPRByRef == nil {
		return nil, nil
	}
	return s.openPRByRef[key], nil
}

func (s *stubGitHubControlClient) CreatePR(_ context.Context, title, body, head, base string) (*ghapi.PullRequest, error) {
	if s.nextPRNumber <= 0 {
		s.nextPRNumber = 1000
	}
	number := s.nextPRNumber
	s.nextPRNumber++
	url := "https://example.invalid/pr/" + strconv.Itoa(number)
	pr := &ghapi.PullRequest{
		Number:  ghapi.Int(number),
		State:   ghapi.String("open"),
		HTMLURL: ghapi.String(url),
		Title:   ghapi.String(title),
		Body:    ghapi.String(body),
		Head: &ghapi.PullRequestBranch{
			Ref: ghapi.String(head),
		},
		Base: &ghapi.PullRequestBranch{
			Ref: ghapi.String(base),
		},
	}
	if s.openPRByRef == nil {
		s.openPRByRef = make(map[string]*ghapi.PullRequest)
	}
	s.openPRByRef[head+"->"+base] = pr
	return pr, nil
}
