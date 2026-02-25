package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(gitBin(), args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %s\n%s", strings.Join(args, " "), string(out))
	return strings.TrimSpace(string(out))
}

func TestWorktreeDefaultBranchIntegration_MainOnlyRepoWithoutOriginHead(t *testing.T) {
	requireLocalPushSupport(t)

	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)
	remoteDir := filepath.Join(t.TempDir(), "main-only-remote.git")

	runGit(t, "", "init", "--bare", remoteDir)
	runGit(t, "", "--git-dir", remoteDir, "config", "core.hardlinks", "false")
	runGit(t, repoDir, "remote", "add", "origin", asFileRemote(remoteDir))
	runGit(t, repoDir, "checkout", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "MAIN_DEFAULT.txt"), []byte("main default"), 0644))
	runGit(t, repoDir, "add", "MAIN_DEFAULT.txt")
	runGit(t, repoDir, "commit", "-m", "main default commit")
	runGit(t, repoDir, "push", "-u", "origin", "main")

	// 明确移除 origin/HEAD，验证 fallback 链仍可解析到 main。
	cmd := exec.Command(gitBin(), "update-ref", "-d", "refs/remotes/origin/HEAD")
	cmd.Dir = repoDir
	_ = cmd.Run()

	wtPath, err := ws.Create(201, "main-only-default", "")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(wtPath, "MAIN_DEFAULT.txt"))

	cmd = exec.Command(gitBin(), "merge-base", "--is-ancestor", "origin/main", "HEAD")
	cmd.Dir = wtPath
	require.NoError(t, cmd.Run())
}

func TestWorktreeDefaultBranchIntegration_CustomDefaultDevelop(t *testing.T) {
	requireLocalPushSupport(t)

	repoDir := initTestRepo(t)
	ws := NewWorkspace(repoDir)
	remoteDir := filepath.Join(t.TempDir(), "develop-default-remote.git")

	runGit(t, "", "init", "--bare", remoteDir)
	runGit(t, "", "--git-dir", remoteDir, "config", "core.hardlinks", "false")
	runGit(t, repoDir, "remote", "add", "origin", asFileRemote(remoteDir))
	runGit(t, repoDir, "push", "-u", "origin", "master")
	runGit(t, repoDir, "checkout", "-b", "develop")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "DEVELOP_DEFAULT.txt"), []byte("develop default"), 0644))
	runGit(t, repoDir, "add", "DEVELOP_DEFAULT.txt")
	runGit(t, repoDir, "commit", "-m", "develop default commit")
	runGit(t, repoDir, "push", "-u", "origin", "develop")
	runGit(t, "", "--git-dir", remoteDir, "symbolic-ref", "HEAD", "refs/heads/develop")

	cmd := exec.Command(gitBin(), "update-ref", "-d", "refs/remotes/origin/HEAD")
	cmd.Dir = repoDir
	_ = cmd.Run()

	wtPath, err := ws.Create(202, "develop-default", "")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(wtPath, "DEVELOP_DEFAULT.txt"))

	cmd = exec.Command(gitBin(), "merge-base", "--is-ancestor", "origin/develop", "HEAD")
	cmd.Dir = wtPath
	require.NoError(t, cmd.Run())
}
