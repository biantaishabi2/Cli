package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitBin 返回系统 git 路径（跳过可能存在的 wrapper）
func gitBin() string {
	for _, p := range []string{"/usr/bin/git", "/bin/git"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "git" // 回退到 PATH 中的 git
}

// initTestRepo 创建临时 git 仓库用于测试
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "testrepo")
	require.NoError(t, os.MkdirAll(repoDir, 0755))

	git := gitBin()

	// git init -b master + 配置
	cmds := [][]string{
		{git, "init", "-b", "master"},
		{git, "config", "user.email", "test@test.com"},
		{git, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "cmd %v failed: %s", args, string(out))
	}

	// 创建初始文件并提交
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# test"), 0644))
	cmd := exec.Command(git, "add", ".")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "commit", "-m", "initial")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	return repoDir
}

func TestWorkspace_Path(t *testing.T) {
	ws := NewWorkspace("/home/user/project/Cli")
	path := ws.Path(42)
	assert.Equal(t, "/home/user/project/Cli/.worktrees/fix-42", path)
}

func TestWorkspace_BranchName(t *testing.T) {
	ws := NewWorkspace("/tmp/repo")

	assert.Equal(t, "fix/42-login-bug", ws.BranchName(42, "login-bug"))
	assert.Equal(t, "fix/7", ws.BranchName(7, ""))
}

func TestWorkspace_CreateAndRemove(t *testing.T) {
	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)

	// 创建 worktree（默认从 master）
	wtPath, err := ws.Create(1, "test-fix", "")
	require.NoError(t, err)
	assert.DirExists(t, wtPath)
	assert.True(t, ws.Exists(1))

	// 重复创建应幂等
	wtPath2, err := ws.Create(1, "test-fix", "")
	require.NoError(t, err)
	assert.Equal(t, wtPath, wtPath2)

	// 移除 worktree
	err = ws.Remove(1)
	require.NoError(t, err)
	assert.False(t, ws.Exists(1))

	// 重复移除应幂等
	err = ws.Remove(1)
	require.NoError(t, err)
}

func TestWorkspace_CreateWithIntegrationBase(t *testing.T) {
	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)
	git := gitBin()

	// 准备 integration/main 分支，并写入仅该分支存在的文件。
	cmd := exec.Command(git, "checkout", "-b", "integration/main")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "INTEGRATION_ONLY.txt"), []byte("from integration"), 0644))
	cmd = exec.Command(git, "add", "INTEGRATION_ONLY.txt")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "commit", "-m", "integration base commit")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	// 回到 master，确保 master 上没有该文件。
	cmd = exec.Command(git, "checkout", "master")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	_, err := os.Stat(filepath.Join(repoDir, "INTEGRATION_ONLY.txt"))
	assert.True(t, os.IsNotExist(err))

	wtPath, err := ws.Create(2, "from-integration", "integration/main")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(wtPath, "INTEGRATION_ONLY.txt"))
}

func TestWorkspace_CreateWithMissingBase_ReturnsError(t *testing.T) {
	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)

	_, err := ws.Create(3, "missing-base", "integration/not-exists")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "基线分支不存在")
}

func TestWorkspace_CreateWithMissingIntegrationBase_AutoCreatesFromDefaultBranch(t *testing.T) {
	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)
	git := gitBin()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")

	// 准备远端，仅推送 master，不创建 integration/main。
	cmd := exec.Command(git, "init", "--bare", remoteDir)
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "remote", "add", "origin", remoteDir)
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "push", "-u", "origin", "master")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	// 从缺失的 integration/main 创建 worktree，预期自动创建基线后成功。
	wtPath, err := ws.Create(9, "auto-integration-base", "integration/main")
	require.NoError(t, err)
	assert.DirExists(t, wtPath)

	// 验证远端 integration/main 已被自动创建。
	cmd = exec.Command(git, "--git-dir", remoteDir, "rev-parse", "--verify", "refs/heads/integration/main")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestWorkspace_Checkout(t *testing.T) {
	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)

	// 先创建一个分支（模拟 implement 阶段已创建分支）
	git := gitBin()
	cmd := exec.Command(git, "branch", "fix/1-checkout-test")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	// 使用 Checkout 从已有分支创建 worktree
	wtPath, err := ws.Checkout(1, "fix/1-checkout-test")
	require.NoError(t, err)
	assert.DirExists(t, wtPath)
	assert.True(t, ws.Exists(1))

	// 验证分支正确
	gitOps := NewGitOps(wtPath)
	branch, err := gitOps.CurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "fix/1-checkout-test", branch)

	// 清理
	require.NoError(t, ws.Remove(1))
}

func TestWorkspace_Exists_NotExist(t *testing.T) {
	ws := NewWorkspace("/tmp/nonexistent-repo")
	assert.False(t, ws.Exists(999))
}

func TestWorkspace_EnsureBranch(t *testing.T) {
	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)

	// 确保分支存在
	err := ws.EnsureBranch(5, "new-feature", "master")
	require.NoError(t, err)

	// 验证分支已创建
	cmd := exec.Command("git", "branch", "--list", "fix/5-new-feature")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "fix/5-new-feature")

	// 幂等：再次调用不应报错
	err = ws.EnsureBranch(5, "new-feature", "master")
	require.NoError(t, err)
}
