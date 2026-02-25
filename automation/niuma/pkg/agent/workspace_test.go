package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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

// initBareRemote 创建 bare remote，并关闭 hardlinks 以避免 CI 文件系统跨设备报错。
func initBareRemote(t *testing.T, remoteDir string) {
	t.Helper()
	git := gitBin()

	cmd := exec.Command(git, "init", "--bare", remoteDir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	cmd = exec.Command(git, "--git-dir", remoteDir, "config", "core.hardlinks", "false")
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func asFileRemote(remoteDir string) string {
	return "file://" + remoteDir
}

var (
	localPushCheckOnce sync.Once
	localPushErr       string
)

// requireLocalPushSupport 检测当前文件系统是否支持本地 git push。
func requireLocalPushSupport(t *testing.T) {
	t.Helper()

	localPushCheckOnce.Do(func() {
		base := t.TempDir()
		srcDir := filepath.Join(base, "src")
		remoteDir := filepath.Join(base, "remote.git")
		git := gitBin()

		if err := os.MkdirAll(srcDir, 0755); err != nil {
			localPushErr = err.Error()
			return
		}

		cmds := [][]string{
			{git, "init", "-b", "master"},
			{git, "config", "user.email", "test@test.com"},
			{git, "config", "user.name", "Test"},
		}
		for _, args := range cmds {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = srcDir
			if out, err := cmd.CombinedOutput(); err != nil {
				localPushErr = string(out)
				return
			}
		}

		if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("probe"), 0644); err != nil {
			localPushErr = err.Error()
			return
		}
		cmd := exec.Command(git, "add", "README.md")
		cmd.Dir = srcDir
		if out, err := cmd.CombinedOutput(); err != nil {
			localPushErr = string(out)
			return
		}
		cmd = exec.Command(git, "commit", "-m", "probe")
		cmd.Dir = srcDir
		if out, err := cmd.CombinedOutput(); err != nil {
			localPushErr = string(out)
			return
		}

		initBareRemote(t, remoteDir)
		cmd = exec.Command(git, "remote", "add", "origin", asFileRemote(remoteDir))
		cmd.Dir = srcDir
		if out, err := cmd.CombinedOutput(); err != nil {
			localPushErr = string(out)
			return
		}
		cmd = exec.Command(git, "push", "-u", "origin", "master")
		cmd.Dir = srcDir
		if out, err := cmd.CombinedOutput(); err != nil {
			localPushErr = string(out)
		}
	})

	if localPushErr != "" {
		t.Skipf("跳过依赖本地 git push 的测试：%s", strings.TrimSpace(localPushErr))
	}
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
	requireLocalPushSupport(t)

	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)
	git := gitBin()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")

	// 准备远端，仅推送 master，不创建 integration/main。
	initBareRemote(t, remoteDir)
	cmd := exec.Command(git, "remote", "add", "origin", asFileRemote(remoteDir))
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

	// master-only 仓库下，integration/main 应以 master 为来源保持兼容。
	cmd = exec.Command(git, "--git-dir", remoteDir, "rev-parse", "--verify", "refs/heads/master")
	masterOut, err := cmd.CombinedOutput()
	require.NoError(t, err, string(masterOut))
	assert.Equal(t, strings.TrimSpace(string(masterOut)), strings.TrimSpace(string(out)))
}

func TestWorkspace_CreateWithMainOnlyRemote_MissingOriginHead(t *testing.T) {
	requireLocalPushSupport(t)

	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)
	git := gitBin()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")

	// 准备 main-only 远端。
	initBareRemote(t, remoteDir)
	cmd := exec.Command(git, "remote", "add", "origin", asFileRemote(remoteDir))
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command(git, "checkout", "-b", "main")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "MAIN_ONLY.txt"), []byte("main branch"), 0644))
	cmd = exec.Command(git, "add", "MAIN_ONLY.txt")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "commit", "-m", "main only")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "push", "-u", "origin", "main")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	// 显式删除本地 origin/HEAD，模拟 main-only 且无 origin/HEAD 的仓库。
	cmd = exec.Command(git, "update-ref", "-d", "refs/remotes/origin/HEAD")
	cmd.Dir = repoDir
	_ = cmd.Run()

	wtPath, err := ws.Create(10, "main-default", "")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(wtPath, "MAIN_ONLY.txt"))
}

func TestWorkspace_CreateWithCustomDefaultBranch_Develop(t *testing.T) {
	requireLocalPushSupport(t)

	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)
	git := gitBin()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")

	initBareRemote(t, remoteDir)
	cmd := exec.Command(git, "remote", "add", "origin", asFileRemote(remoteDir))
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	// 推送 master，并创建 develop 专属提交。
	cmd = exec.Command(git, "push", "-u", "origin", "master")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "checkout", "-b", "develop")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "DEVELOP_ONLY.txt"), []byte("develop default"), 0644))
	cmd = exec.Command(git, "add", "DEVELOP_ONLY.txt")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "commit", "-m", "develop only")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "push", "-u", "origin", "develop")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "--git-dir", remoteDir, "symbolic-ref", "HEAD", "refs/heads/develop")
	require.NoError(t, cmd.Run())

	// 删除本地 origin/HEAD，确保使用统一 fallback 链解析到 develop。
	cmd = exec.Command(git, "update-ref", "-d", "refs/remotes/origin/HEAD")
	cmd.Dir = repoDir
	_ = cmd.Run()

	wtPath, err := ws.Create(11, "develop-default", "")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(wtPath, "DEVELOP_ONLY.txt"))
}

func TestWorkspace_CreateWithOriginMainLocalFallbackWhenRemoteUnavailable(t *testing.T) {
	requireLocalPushSupport(t)

	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)
	git := gitBin()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")

	initBareRemote(t, remoteDir)
	cmd := exec.Command(git, "remote", "add", "origin", asFileRemote(remoteDir))
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	cmd = exec.Command(git, "checkout", "-b", "main")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "MAIN_LOCAL_ONLY.txt"), []byte("local main"), 0644))
	cmd = exec.Command(git, "add", "MAIN_LOCAL_ONLY.txt")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "commit", "-m", "local main")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "push", "-u", "origin", "main")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "checkout", "master")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "branch", "-D", "main")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	// 构造：origin/HEAD 缺失 + origin 不可达，且仅保留本地 refs/remotes/origin/main。
	cmd = exec.Command(git, "update-ref", "-d", "refs/remotes/origin/HEAD")
	cmd.Dir = repoDir
	_ = cmd.Run()
	cmd = exec.Command(git, "remote", "set-url", "origin", asFileRemote(filepath.Join(t.TempDir(), "missing-remote.git")))
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	wtPath, err := ws.Create(12, "origin-main-local-fallback", "")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(wtPath, "MAIN_LOCAL_ONLY.txt"))
}

func TestWorkspace_CreateWithAllDefaultBranchProbesFail_ReturnsDiagnosticError(t *testing.T) {
	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)
	git := gitBin()

	// origin 存在但不可达，且无 origin/main 本地引用。
	cmd := exec.Command(git, "remote", "add", "origin", asFileRemote(filepath.Join(t.TempDir(), "missing-remote.git")))
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())

	_, err := ws.Create(13, "all-probes-fail", "integration/main")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "自动创建基线分支失败")
	assert.Contains(t, err.Error(), "默认分支探测已回退")
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
