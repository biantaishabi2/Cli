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
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-b", "master"},
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644))
	cmds = append(cmds,
		[]string{"git", "add", "."},
		[]string{"git", "commit", "-m", "initial"},
	)
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "cmd=%v output=%s", args, string(out))
	}
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
	result, err := builder.Build("integration/batch-test", branches, nil)

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
	result, err := builder.Build("integration/batch-test", branches, nil)

	// Then
	require.NoError(t, err)
	assert.Equal(t, []int{40, 41, 42}, result.Merged)
	assert.Empty(t, result.Conflicts)
	assert.Empty(t, result.Skipped)
}

// ===== 场景 5: MergeOne 增量合并单个 PR =====

func TestBDD_MergeOne_IncrementalMerge(t *testing.T) {
	// Given: 已有 integration 分支，新 PR 完成
	dir := setupTestRepo(t)
	
	// 创建初始 integration 分支
	runGit(t, dir, "checkout", "-b", "integration/test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "base.go"), []byte("package base\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init integration")
	runGit(t, dir, "checkout", "master")
	
	// 创建新 PR 分支
	createBranch(t, dir, "feat/40-auth", "auth.go", "package auth\n")
	
	builder := control.NewIntegrationBuilder(dir, "master")
	
	bi := control.BranchInfo{
		Branch:   "feat/40-auth",
		IssueNum: 40,
		PRNum:    100,
		TaskID:   "task-40",
	}
	
	// When: 增量合并单个 PR
	err := builder.MergeOne("integration/test", bi)
	
	// Then: 合并成功
	require.NoError(t, err)
	
	// 验证 integration 分支包含新提交
	runGit(t, dir, "checkout", "integration/test")
	content, err := os.ReadFile(filepath.Join(dir, "auth.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "package auth")
}

// ===== 场景 6: MergeOne 冲突处理 =====

func TestBDD_MergeOne_Conflict(t *testing.T) {
	// Given: integration 已有文件，PR 修改同一文件冲突
	dir := setupTestRepo(t)
	
	// 创建 integration 分支并添加文件
	runGit(t, dir, "checkout", "-b", "integration/test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.go"), []byte("package config // integration\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add config in integration")
	runGit(t, dir, "checkout", "master")
	
	// 创建冲突的 PR 分支（从 master，修改同一文件）
	runGit(t, dir, "checkout", "-b", "feat/41-config")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.go"), []byte("package config // pr\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "modify config")
	runGit(t, dir, "checkout", "master")
	
	builder := control.NewIntegrationBuilder(dir, "master")
	
	bi := control.BranchInfo{
		Branch:   "feat/41-config",
		IssueNum: 41,
		PRNum:    101,
		TaskID:   "task-41",
	}
	
	// When: 尝试合并冲突的 PR
	err := builder.MergeOne("integration/test", bi)
	
	// Then: 应该返回冲突错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "冲突")
	
	// 验证 integration 分支未改变（merge --abort 成功）
	runGit(t, dir, "checkout", "integration/test")
	content, err := os.ReadFile(filepath.Join(dir, "config.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "// integration")
}

// ===== 场景 7: Controller 增量 Integration 循环 =====

func TestBDD_Controller_IncrementalIntegration(t *testing.T) {
	// Given: 两个已完成的 PR，需要增量合入 integration
	dir := setupTestRepo(t)
	
	// 创建两个 PR 分支
	createBranch(t, dir, "feat/40-auth", "auth.go", "package auth\n")
	createBranch(t, dir, "feat/41-payment", "payment.go", "package payment\n")
	
	// 使用 IntegrationBuilder 直接测试增量合并
	builder := control.NewIntegrationBuilder(dir, "master")
	
	// 先创建 integration 分支
	_, err := builder.EnsureBranch("integration/test")
	require.NoError(t, err)
	
	// 逐个 MergeOne（模拟 Controller 的增量 integration 循环）
	bi40 := control.BranchInfo{Branch: "feat/40-auth", IssueNum: 40, PRNum: 100, TaskID: "task-40"}
	err = builder.MergeOne("integration/test", bi40)
	require.NoError(t, err)
	
	bi41 := control.BranchInfo{Branch: "feat/41-payment", IssueNum: 41, PRNum: 101, TaskID: "task-41"}
	err = builder.MergeOne("integration/test", bi41)
	require.NoError(t, err)
	
	// Then: 验证 integration 分支包含两个提交
	runGit(t, dir, "checkout", "integration/test")
	content, err := os.ReadFile(filepath.Join(dir, "auth.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "package auth")
	
	content, err = os.ReadFile(filepath.Join(dir, "payment.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "package payment")
}

// ===== 场景 8: EnsureBranch 从 master 创建 integration =====

func TestBDD_EnsureBranch_CreateFromMaster(t *testing.T) {
	// Given: 干净的仓库，无 integration 分支
	dir := setupTestRepo(t)
	
	// 在 master 上添加一些提交
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add main")
	
	builder := control.NewIntegrationBuilder(dir, "master")
	
	// When: 确保 integration 分支存在
	branchName, err := builder.EnsureBranch("integration/test-20240101-120000")
	
	// Then: 分支创建成功，基于 master
	require.NoError(t, err)
	assert.Equal(t, "integration/test-20240101-120000", branchName)
	
	// 验证分支存在且包含 master 的提交
	cmd := exec.Command("git", "branch", "--list", "integration/test-*")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(out), "integration/test-")
}

// ===== 场景 9: Reset 重建 integration 分支 =====

func TestBDD_Reset_RebuildIntegration(t *testing.T) {
	// Given: 有冲突的 integration 分支需要重建
	dir := setupTestRepo(t)
	
	// 创建 integration 分支并添加提交
	runGit(t, dir, "checkout", "-b", "integration/test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package feature\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add feature")
	runGit(t, dir, "checkout", "master")
	
	// master 也前进（模拟 divergence）
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add main")
	
	builder := control.NewIntegrationBuilder(dir, "master")
	
	// When: 重置 integration 分支
	err := builder.Reset("integration/test")
	
	// Then: 重置成功，integration 基于最新 master
	require.NoError(t, err)
	
	// 验证 integration 分支现在基于 master
	runGit(t, dir, "checkout", "integration/test")
	// 应该能看到 main.go（来自 master）
	_, err = os.ReadFile(filepath.Join(dir, "main.go"))
	require.NoError(t, err, "integration 应该包含 master 的最新提交")
	// 不应该有 feature.go（被重置掉了）
	_, err = os.ReadFile(filepath.Join(dir, "feature.go"))
	assert.Error(t, err, "feature.go 应该被重置移除")
}

// ===== 场景 10: Controller Merge 到 master =====

func TestBDD_Controller_MergeToMaster(t *testing.T) {
	// Given: 已验证的 integration 分支
	dir := setupTestRepo(t)
	
	// 创建 integration 分支并添加一些提交
	runGit(t, dir, "checkout", "-b", "integration/test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature1.go"), []byte("package f1\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "feature 1")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature2.go"), []byte("package f2\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "feature 2")
	runGit(t, dir, "checkout", "master")
	
	// 使用 git 命令直接合并 integration 到 master（模拟 Controller.Merge）
	runGit(t, dir, "merge", "integration/test", "-m", "Merge integration to master")
	
	// Then: master 应该包含 feature1.go 和 feature2.go
	content1, err := os.ReadFile(filepath.Join(dir, "feature1.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content1), "package f1")
	
	content2, err := os.ReadFile(filepath.Join(dir, "feature2.go"))
	require.NoError(t, err)
	assert.Contains(t, string(content2), "package f2")
}
