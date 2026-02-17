//go:build bdd

// pkg/bdd/control_test.go
// BDD 集成测试：多 Issue 协调流程
package bdd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== 测试辅助 =====

// mockControlGitHub 用于 BDD 测试的 GitHub mock
type mockControlGitHub struct {
	issues    []control.IssueInfo
	mergedPRs []int
}

func (m *mockControlGitHub) ListIssuesWithLabel(_ context.Context, _ string) ([]control.IssueInfo, error) {
	return m.issues, nil
}

func (m *mockControlGitHub) MergePR(_ context.Context, prNum int, _ string) error {
	m.mergedPRs = append(m.mergedPRs, prNum)
	return nil
}

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", "-b", "master"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "cmd=%v output=%s", args, string(out))
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	return dir
}

func createBranch(t *testing.T, dir, branch, filename, content string) {
	t.Helper()
	runGit(t, dir, "checkout", "master")
	runGit(t, dir, "checkout", "-b", branch)
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add "+filename)
	runGit(t, dir, "checkout", "master")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))
}

// ===== 场景 1: AI 分析依赖关系并正确建 DAG =====

func TestBDD_AIAnalyzeDependencies(t *testing.T) {
	// Given: 3 个 issue，#42 的实现依赖 #40
	mockAI := ai.NewMockProvider(`{"dependencies": {"42": [40]}, "potential_conflicts": [[41, 42]]}`)
	analyzer := control.NewDependencyAnalyzer(mockAI)

	issues := []control.IssueInfo{
		{Number: 40, Title: "重构 auth 模块", Body: "重构认证模块，提取公共接口"},
		{Number: 41, Title: "新增 payment", Body: "添加支付功能"},
		{Number: 42, Title: "payment 测试", Body: "为支付功能编写集成测试"},
	}

	// When: 分析依赖
	result, err := analyzer.Analyze(context.Background(), issues)

	// Then: #42 依赖 #40
	require.NoError(t, err)
	assert.Equal(t, []int{40}, result.Dependencies[42])
	assert.Equal(t, [][]int{{41, 42}}, result.PotentialConflicts)
}

// ===== 场景 2: depends-on 人工声明覆盖 AI =====

func TestBDD_ManualDependsOnOverridesAI(t *testing.T) {
	// Given: #42 body 明确声明 depends-on: #40
	// AI 认为 #42 依赖 #41
	mockAI := ai.NewMockProvider(`{"dependencies": {"42": [41]}, "potential_conflicts": []}`)
	analyzer := control.NewDependencyAnalyzer(mockAI)

	issues := []control.IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Payment", Body: "fix payment"},
		{Number: 42, Title: "Tests", Body: "测试\ndepends-on: #40"},
	}

	// When
	result, err := analyzer.Analyze(context.Background(), issues)

	// Then: 人工声明优先
	require.NoError(t, err)
	assert.Equal(t, []int{40}, result.Dependencies[42])
}

// ===== 场景 3: Integration 分支冲突检测 =====

func TestBDD_IntegrationConflictDetection(t *testing.T) {
	// Given: 两个 issue 修改同一文件
	dir := setupTestRepo(t)
	createBranch(t, dir, "feat/40-auth", "shared.go", "package shared // A\n")

	// 41 也创建 shared.go（从 master 分支，所以和 40 冲突）
	runGit(t, dir, "checkout", "master")
	runGit(t, dir, "checkout", "-b", "feat/41-payment")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "shared.go"), []byte("package shared // B\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add shared.go conflict")
	runGit(t, dir, "checkout", "master")

	builder := control.NewIntegrationBuilder(dir, "master")

	branches := []control.BranchInfo{
		{Branch: "feat/40-auth", IssueNum: 40},
		{Branch: "feat/41-payment", IssueNum: 41},
	}

	// When
	result, err := builder.Build("integration/test-conflict", branches, nil)

	// Then: 40 成功，41 冲突
	require.NoError(t, err)
	assert.Contains(t, result.Merged, 40)
	assert.Contains(t, result.Conflicts, 41)
}

// ===== 场景 4: Integration 无冲突全部合并 =====

func TestBDD_IntegrationAllMerged(t *testing.T) {
	// Given: 3 个 issue 各改不同文件
	dir := setupTestRepo(t)
	createBranch(t, dir, "feat/40-auth", "auth.go", "package auth\n")
	createBranch(t, dir, "feat/41-payment", "payment.go", "package payment\n")
	createBranch(t, dir, "feat/42-tests", "tests.go", "package tests\n")

	builder := control.NewIntegrationBuilder(dir, "master")

	branches := []control.BranchInfo{
		{Branch: "feat/40-auth", IssueNum: 40},
		{Branch: "feat/41-payment", IssueNum: 41},
		{Branch: "feat/42-tests", IssueNum: 42},
	}

	// When
	result, err := builder.Build("integration/test-all-merged", branches, nil)

	// Then
	require.NoError(t, err)
	assert.Equal(t, []int{40, 41, 42}, result.Merged)
	assert.Empty(t, result.Conflicts)
	assert.Empty(t, result.Skipped)
}
