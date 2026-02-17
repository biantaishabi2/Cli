package control

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupGitRepo 创建一个用于测试的 git 仓库，包含 master 分支
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// 初始化仓库
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// 创建初始文件并提交（在创建分支之前）
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial commit")

	// 确保在 master 分支上（某些 git 版本默认是 main）
	runGit(t, dir, "branch", "-m", "master")

	return dir
}

// createBranch 在仓库中创建分支并添加文件
func createBranch(t *testing.T, dir, branch, filename, content string) {
	t.Helper()
	runGit(t, dir, "checkout", "master")
	runGit(t, dir, "checkout", "-b", branch)
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add "+filename)
	runGit(t, dir, "checkout", "master")
}

// createBranchModifyFile 在仓库中创建分支并修改已有文件的特定内容
func createBranchModifyFile(t *testing.T, dir, branch, filename, content string) {
	t.Helper()
	runGit(t, dir, "checkout", "master")
	runGit(t, dir, "checkout", "-b", branch)
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "modify "+filename)
	runGit(t, dir, "checkout", "master")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))
}

func TestIntegrationBuilder_NoConflict(t *testing.T) {
	dir := setupGitRepo(t)
	createBranch(t, dir, "feat/40-auth", "auth.go", "package auth\n")
	createBranch(t, dir, "feat/41-payment", "payment.go", "package payment\n")
	createBranch(t, dir, "feat/42-tests", "tests.go", "package tests\n")

	builder := NewIntegrationBuilder(dir, "master")

	branches := []BranchInfo{
		{Branch: "feat/40-auth", IssueNum: 40},
		{Branch: "feat/41-payment", IssueNum: 41},
		{Branch: "feat/42-tests", IssueNum: 42},
	}

	result, err := builder.Build("integration/test", branches, nil)
	require.NoError(t, err)
	assert.Equal(t, []int{40, 41, 42}, result.Merged)
	assert.Empty(t, result.Conflicts)
	assert.Empty(t, result.Skipped)
	assert.Equal(t, "integration/test", result.Branch)
}

func TestIntegrationBuilder_WithConflict(t *testing.T) {
	dir := setupGitRepo(t)

	// 两个分支修改同一文件
	createBranch(t, dir, "feat/40-auth", "shared.go", "package shared // version A\n")
	createBranchModifyFile(t, dir, "feat/41-payment", "shared.go", "package shared // version B\n")
	createBranch(t, dir, "feat/42-tests", "tests.go", "package tests\n")

	builder := NewIntegrationBuilder(dir, "master")

	branches := []BranchInfo{
		{Branch: "feat/40-auth", IssueNum: 40},
		{Branch: "feat/41-payment", IssueNum: 41},
		{Branch: "feat/42-tests", IssueNum: 42},
	}

	result, err := builder.Build("integration/test", branches, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Merged, 40)
	// 41 和 40 都创建 shared.go（冲突）
	assert.Contains(t, result.Conflicts, 41)
	// 42 应该成功（不冲突）
	assert.Contains(t, result.Merged, 42)
}

func TestIntegrationBuilder_CascadeSkip(t *testing.T) {
	dir := setupGitRepo(t)

	createBranch(t, dir, "feat/40-auth", "shared.go", "package shared // version A\n")
	createBranchModifyFile(t, dir, "feat/41-payment", "shared.go", "package shared // version B\n")
	createBranch(t, dir, "feat/42-tests", "tests.go", "package tests\n")

	builder := NewIntegrationBuilder(dir, "master")

	branches := []BranchInfo{
		{Branch: "feat/40-auth", IssueNum: 40},
		{Branch: "feat/41-payment", IssueNum: 41},
		{Branch: "feat/42-tests", IssueNum: 42},
	}

	// 42 依赖 41
	deps := map[int][]int{42: {41}}

	result, err := builder.Build("integration/test", branches, deps)
	require.NoError(t, err)
	assert.Contains(t, result.Merged, 40)
	assert.Contains(t, result.Conflicts, 41)
	assert.Contains(t, result.Skipped, 42) // 因依赖 41 失败被跳过
}

func TestIntegrationBuilder_CleanOld(t *testing.T) {
	dir := setupGitRepo(t)

	// 创建 5 个 integration 分支
	for i := 0; i < 5; i++ {
		runGit(t, dir, "branch", fmt.Sprintf("integration/batch-2024010%d-120000", i))
	}

	builder := NewIntegrationBuilder(dir, "master")
	err := builder.CleanOld()
	require.NoError(t, err)

	// 列出剩余的 integration 分支
	cmd := exec.Command("git", "branch", "--list", "integration/batch-*")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)

	lines := splitNonEmpty(string(out))
	assert.LessOrEqual(t, len(lines), 3)
}

func splitNonEmpty(s string) []string {
	var result []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if strings.TrimSpace(line) != "" {
			result = append(result, strings.TrimSpace(line))
		}
	}
	return result
}
