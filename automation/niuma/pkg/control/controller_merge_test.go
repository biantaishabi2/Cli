package control

import (
	"context"
	"os"
	"path/filepath"
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

func TestControllerMerge_FastForwardAndIdempotent(t *testing.T) {
	repoDir, remoteDir := setupGitRepoWithBareRemote(t)

	// 构造 integration/main 提前于 master 的分支状态。
	runGit(t, repoDir, "checkout", "-b", "integration/main")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "feature.txt"), []byte("v1\n"), 0o644))
	runGit(t, repoDir, "add", "feature.txt")
	runGit(t, repoDir, "commit", "-m", "feat: integration change")
	runGit(t, repoDir, "push", "-u", "origin", "integration/main")
	runGit(t, repoDir, "checkout", "master")
	masterBefore := runGitOutput(t, "", "--git-dir", remoteDir, "rev-parse", "refs/heads/master")

	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		builder: NewIntegrationBuilder(repoDir, "master"),
		github:  mockGH,
	}

	require.NoError(t, ctrl.Merge(context.Background(), "integration/main"))
	require.Len(t, mockGH.createPRCalls, 1)
	require.Equal(t, "integration/main", mockGH.createPRCalls[0].head)
	require.Equal(t, "master", mockGH.createPRCalls[0].base)

	// 幂等要求：重复执行应 no-op 成功。
	require.NoError(t, ctrl.Merge(context.Background(), "integration/main"))
	require.Len(t, mockGH.createPRCalls, 1)
	require.Len(t, mockGH.findOpenPRCalls, 2)

	// PR 模式不应直接 push master。
	masterAfter := runGitOutput(t, "", "--git-dir", remoteDir, "rev-parse", "refs/heads/master")
	assert.Equal(t, masterBefore, masterAfter)
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
