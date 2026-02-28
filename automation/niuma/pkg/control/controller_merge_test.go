package control

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveIntegrationBranchForIssues_Success(t *testing.T) {
	listJSON := `[
{"id":"task-40","subject":"a","description":"a","status":"pending","metadata":{"issue_num":"40","meta_issue_slug":"phase-3"}},
{"id":"task-41","subject":"b","description":"b","status":"in-progress","metadata":{"issue_num":"41","meta_issue_slug":"phase-3"}}
]`
	ctrl := &Controller{
		taskctl: newScriptTaskCtlClient(t, listJSON, `{"nodes":[]}`),
	}

	branch, err := ctrl.ResolveIntegrationBranchForIssues([]int{41, 40, 41})
	require.NoError(t, err)
	assert.Equal(t, "integration/phase-3", branch)
}

func TestResolveIntegrationBranchForIssues_Mismatch(t *testing.T) {
	listJSON := `[
{"id":"task-40","subject":"a","description":"a","status":"pending","metadata":{"issue_num":"40","meta_issue_slug":"phase-3"}},
{"id":"task-41","subject":"b","description":"b","status":"pending","metadata":{"issue_num":"41","meta_issue_slug":"phase-4"}}
]`
	ctrl := &Controller{
		taskctl: newScriptTaskCtlClient(t, listJSON, `{"nodes":[]}`),
	}

	_, err := ctrl.ResolveIntegrationBranchForIssues([]int{40, 41})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "多个 integration 分支")
}

func TestResolveIntegrationBranchForIssues_MissingTask(t *testing.T) {
	listJSON := `[
{"id":"task-40","subject":"a","description":"a","status":"pending","metadata":{"issue_num":"40","meta_issue_slug":"phase-3"}}
]`
	ctrl := &Controller{
		taskctl: newScriptTaskCtlClient(t, listJSON, `{"nodes":[]}`),
	}

	_, err := ctrl.ResolveIntegrationBranchForIssues([]int{40, 41})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未找到任务映射")
}

func TestControllerMerge_FastForwardAndIdempotent_MainBase(t *testing.T) {
	repoDir, remoteDir := setupGitRepoWithBareRemote(t)

	// 构造 main 与 integration/main，模拟默认分支为 main。
	runGit(t, repoDir, "checkout", "-b", "main")
	runGit(t, repoDir, "push", "-u", "origin", "main")
	runGit(t, repoDir, "checkout", "-b", "integration/main")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "feature.txt"), []byte("v1\n"), 0o644))
	runGit(t, repoDir, "add", "feature.txt")
	runGit(t, repoDir, "commit", "-m", "feat: integration change")
	runGit(t, repoDir, "push", "-u", "origin", "integration/main")
	runGit(t, repoDir, "checkout", "main")
	mainBefore := runGitOutput(t, "", "--git-dir", remoteDir, "rev-parse", "refs/heads/main")

	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		builder: NewIntegrationBuilder(repoDir, "main"),
		github:  mockGH,
	}

	require.NoError(t, ctrl.Merge(context.Background(), "integration/main"))
	require.Len(t, mockGH.createPRCalls, 1)
	require.Equal(t, "integration/main", mockGH.createPRCalls[0].head)
	require.Equal(t, "main", mockGH.createPRCalls[0].base)

	// 幂等要求：重复执行应 no-op 成功。
	require.NoError(t, ctrl.Merge(context.Background(), "integration/main"))
	require.Len(t, mockGH.createPRCalls, 1)
	require.Len(t, mockGH.findOpenPRCalls, 2)

	// PR 模式不应直接 push main。
	mainAfter := runGitOutput(t, "", "--git-dir", remoteDir, "rev-parse", "refs/heads/main")
	assert.Equal(t, mainBefore, mainAfter)
}

func TestControllerMerge_MasterBaseCompatible(t *testing.T) {
	repoDir, _ := setupGitRepoWithBareRemote(t)
	runGit(t, repoDir, "checkout", "-b", "integration/main")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "feature.txt"), []byte("master\n"), 0o644))
	runGit(t, repoDir, "add", "feature.txt")
	runGit(t, repoDir, "commit", "-m", "feat: integration change")
	runGit(t, repoDir, "push", "-u", "origin", "integration/main")
	runGit(t, repoDir, "checkout", "master")

	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		builder: NewIntegrationBuilder(repoDir, "master"),
		github:  mockGH,
	}

	require.NoError(t, ctrl.Merge(context.Background(), "integration/main"))
	require.Len(t, mockGH.createPRCalls, 1)
	require.Equal(t, "master", mockGH.createPRCalls[0].base)
}

func TestControllerMerge_BranchNotFound(t *testing.T) {
	repoDir, _ := setupGitRepoWithBareRemote(t)
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		builder: NewIntegrationBuilder(repoDir, "master"),
		github:  mockGH,
	}

	err := ctrl.Merge(context.Background(), "integration/not-exists")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "远端分支不存在")
}

func TestControllerMerge_ExplicitOverrideBaseBranch(t *testing.T) {
	repoDir, _ := setupGitRepoWithBareRemote(t)
	runGit(t, repoDir, "checkout", "-b", "release")
	runGit(t, repoDir, "push", "-u", "origin", "release")
	runGit(t, repoDir, "checkout", "-b", "integration/main")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "release.txt"), []byte("release\n"), 0o644))
	runGit(t, repoDir, "add", "release.txt")
	runGit(t, repoDir, "commit", "-m", "feat: release integration")
	runGit(t, repoDir, "push", "-u", "origin", "integration/main")
	runGit(t, repoDir, "checkout", "release")

	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		builder: NewIntegrationBuilder(repoDir, "release"),
		github:  mockGH,
	}

	require.NoError(t, ctrl.Merge(context.Background(), "integration/main"))
	require.Len(t, mockGH.createPRCalls, 1)
	require.Equal(t, "release", mockGH.createPRCalls[0].base)
}

func TestControllerMerge_AlreadyMerged_NoOp(t *testing.T) {
	repoDir, _ := setupGitRepoWithBareRemote(t)

	// integration/main 与 master 指向同一提交。
	runGit(t, repoDir, "checkout", "-b", "integration/main")
	runGit(t, repoDir, "push", "-u", "origin", "integration/main")
	runGit(t, repoDir, "checkout", "master")

	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		builder: NewIntegrationBuilder(repoDir, "master"),
		github:  mockGH,
	}
	require.NoError(t, ctrl.Merge(context.Background(), "integration/main"))
	assert.Empty(t, mockGH.createPRCalls)
}

func TestControllerMerge_ClosedPRNotReused(t *testing.T) {
	repoDir, _ := setupGitRepoWithBareRemote(t)

	runGit(t, repoDir, "checkout", "-b", "integration/main")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "integration.txt"), []byte("integration\n"), 0o644))
	runGit(t, repoDir, "add", "integration.txt")
	runGit(t, repoDir, "commit", "-m", "feat: integration")
	runGit(t, repoDir, "push", "-u", "origin", "integration/main")
	runGit(t, repoDir, "checkout", "master")

	mockGH := newMockGitHubOps()
	mockGH.findOpenPR[prRefKey("integration/main", "master")] = &PullRequestInfo{
		Number: 88,
		URL:    "https://example.invalid/pr/88",
		State:  "closed",
		Head:   "integration/main",
		Base:   "master",
	}

	ctrl := &Controller{
		builder: NewIntegrationBuilder(repoDir, "master"),
		github:  mockGH,
	}

	require.NoError(t, ctrl.Merge(context.Background(), "integration/main"))
	require.Len(t, mockGH.createPRCalls, 1)
	assert.NotEqual(t, 88, mockGH.findOpenPR[prRefKey("integration/main", "master")].Number)
}

func TestControllerMerge_LogsPRRefKey(t *testing.T) {
	repoDir, _ := setupGitRepoWithBareRemote(t)
	runGit(t, repoDir, "checkout", "-b", "integration/main")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "integration.txt"), []byte("integration\n"), 0o644))
	runGit(t, repoDir, "add", "integration.txt")
	runGit(t, repoDir, "commit", "-m", "feat: integration")
	runGit(t, repoDir, "push", "-u", "origin", "integration/main")
	runGit(t, repoDir, "checkout", "master")

	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		builder: NewIntegrationBuilder(repoDir, "master"),
		github:  mockGH,
	}

	logOutput := captureStdout(t, func() {
		require.NoError(t, ctrl.Merge(context.Background(), "integration/main"))
	})
	assert.Contains(t, logOutput, "action=merge")
	assert.Contains(t, logOutput, "pr_ref=integration/main->master")
}

func TestIsCloseMergedBaseRef_StrictWhitelist(t *testing.T) {
	assert.True(t, IsCloseMergedBaseRef("main", "main"))
	assert.True(t, IsCloseMergedBaseRef(" integration/main ", "main"))
	assert.False(t, IsCloseMergedBaseRef("master", "main"))
	assert.False(t, IsCloseMergedBaseRef("feature/foo", "main"))
	assert.False(t, IsCloseMergedBaseRef("", "main"))
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

type stubMergeBaseGitResult struct {
	out string
	err error
}

type stubMergeBaseGitExecutor struct {
	results map[string]stubMergeBaseGitResult
	calls   []string
}

func (s *stubMergeBaseGitExecutor) CombinedOutput(_ string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	s.calls = append(s.calls, key)
	result, ok := s.results[key]
	if !ok {
		return nil, fmt.Errorf("unexpected git command: %s", key)
	}
	return []byte(result.out), result.err
}

func TestMergeBaseResolver_Priority(t *testing.T) {
	resolver := NewMergeBaseResolverWithExecutor(
		t.TempDir(),
		&stubMergeBaseDefaultBranchProvider{branch: "main"},
		&stubMergeBaseGitExecutor{
			results: map[string]stubMergeBaseGitResult{},
		},
	)

	result := resolver.Resolve(context.Background(), "release/v2", "develop")
	assert.Equal(t, "release/v2", result.Branch)
	assert.Equal(t, "cli-flag", result.Source)

	result = resolver.Resolve(context.Background(), "", "develop")
	assert.Equal(t, "develop", result.Branch)
	assert.Equal(t, "config", result.Source)

	result = resolver.Resolve(context.Background(), "", "")
	assert.Equal(t, "main", result.Branch)
	assert.Equal(t, "github-default-branch", result.Source)
}

func TestMergeBaseResolver_AutoDetectFromGitRemoteShow(t *testing.T) {
	executor := &stubMergeBaseGitExecutor{
		results: map[string]stubMergeBaseGitResult{
			"remote show origin": {out: "  HEAD branch: release\n"},
		},
	}
	resolver := NewMergeBaseResolverWithExecutor(
		t.TempDir(),
		&stubMergeBaseDefaultBranchProvider{err: fmt.Errorf("github unavailable")},
		executor,
	)

	result := resolver.Resolve(context.Background(), "", "")
	assert.Equal(t, "release", result.Branch)
	assert.Equal(t, "git-remote-show-origin", result.Source)
	assert.Len(t, executor.calls, 1)
	assert.Equal(t, "remote show origin", executor.calls[0])
}

func TestMergeBaseResolver_FallbackWarning(t *testing.T) {
	executor := &stubMergeBaseGitExecutor{
		results: map[string]stubMergeBaseGitResult{
			"remote show origin":                    {err: fmt.Errorf("origin unavailable")},
			"symbolic-ref refs/remotes/origin/HEAD": {err: fmt.Errorf("missing origin/HEAD")},
		},
	}
	resolver := NewMergeBaseResolverWithExecutor(
		t.TempDir(),
		&stubMergeBaseDefaultBranchProvider{err: fmt.Errorf("github unavailable")},
		executor,
	)

	result := resolver.Resolve(context.Background(), "", "")
	assert.Equal(t, "master", result.Branch)
	assert.Equal(t, "fallback-master", result.Source)
	assert.Contains(t, result.Warning, "fallback=master")
	assert.Contains(t, result.Warning, "github-default-branch")
	assert.Contains(t, result.Warning, "git-remote-show-origin")
	assert.Contains(t, result.Warning, "git-origin-head")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	require.NoError(t, w.Close())
	return <-done
}
