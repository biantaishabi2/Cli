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
	runGit(t, dir, "init", "-b", "master")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// 创建初始文件并提交（在创建分支之前）
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial commit")

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

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))
	return strings.TrimSpace(string(out))
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

func TestIntegrationBuilder_Build_UsesUnifiedExecutorForAutoResolvedConflict(t *testing.T) {
	dir := setupGitRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docs", "guide.md"), []byte("# guide\nline\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add docs guide")

	createBranchModifyFile(t, dir, "feat/40-doc-a", "docs/guide.md", "# guide\nline from A\n")
	createBranchModifyFile(t, dir, "feat/41-doc-b", "docs/guide.md", "# guide\nline from B\n")

	builder := NewIntegrationBuilder(dir, "master")
	branches := []BranchInfo{
		{Branch: "feat/40-doc-a", IssueNum: 40},
		{Branch: "feat/41-doc-b", IssueNum: 41},
	}

	result, err := builder.Build("integration/test-build-auto", branches, nil)
	require.NoError(t, err)
	assert.Equal(t, []int{40, 41}, result.Merged)
	assert.Empty(t, result.Conflicts)

	content := runGitOutput(t, dir, "show", "integration/test-build-auto:docs/guide.md")
	assert.Contains(t, content, "line from B")
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

func TestIntegrationBuilder_ExecuteIntegrationMerge_AutoResolvedForWhitelistedFile(t *testing.T) {
	dir := setupGitRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docs", "guide.md"), []byte("# guide\nline\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add docs guide")

	createBranchModifyFile(t, dir, "feat/40-doc-a", "docs/guide.md", "# guide\nline from A\n")
	createBranchModifyFile(t, dir, "feat/41-doc-b", "docs/guide.md", "# guide\nline from B\n")

	builder := NewIntegrationBuilder(dir, "master")

	first, err := builder.ExecuteIntegrationMerge("integration/test-auto", BranchInfo{
		Branch:   "feat/40-doc-a",
		IssueNum: 40,
		PRNum:    400,
	})
	require.NoError(t, err)
	assert.Equal(t, MergeStatusMerged, first.Status)

	second, err := builder.ExecuteIntegrationMerge("integration/test-auto", BranchInfo{
		Branch:   "feat/41-doc-b",
		IssueNum: 41,
		PRNum:    401,
	})
	require.NoError(t, err)
	assert.Equal(t, MergeStatusAutoResolved, second.Status)
	assert.Contains(t, second.AutoResolvedFiles, "docs/guide.md")

	unmerged := runGitOutput(t, dir, "diff", "--name-only", "--diff-filter=U")
	assert.Empty(t, unmerged)

	content := runGitOutput(t, dir, "show", "integration/test-auto:docs/guide.md")
	assert.Contains(t, content, "line from B")
}

func TestIntegrationBuilder_ExecuteIntegrationMerge_EscalatedForCoreSemanticConflict(t *testing.T) {
	dir := setupGitRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "service.go"), []byte("package pkg\n\nfunc version() int {\n\treturn 0\n}\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add core service")

	createBranchModifyFile(t, dir, "feat/40-core-a", "pkg/service.go", "package pkg\n\nfunc version() int {\n\treturn 1\n}\n")
	createBranchModifyFile(t, dir, "feat/41-core-b", "pkg/service.go", "package pkg\n\nfunc version() int {\n\treturn 2\n}\n")

	builder := NewIntegrationBuilder(dir, "master")

	first, err := builder.ExecuteIntegrationMerge("integration/test-escalated", BranchInfo{
		Branch:   "feat/40-core-a",
		IssueNum: 40,
		PRNum:    500,
	})
	require.NoError(t, err)
	assert.Equal(t, MergeStatusMerged, first.Status)

	second, err := builder.ExecuteIntegrationMerge("integration/test-escalated", BranchInfo{
		Branch:   "feat/41-core-b",
		IssueNum: 41,
		PRNum:    501,
	})
	require.NoError(t, err)
	assert.Equal(t, MergeStatusEscalated, second.Status)
	require.NotNil(t, second.Conflict)
	assert.Contains(t, second.Conflict.Files, "pkg/service.go")
	assert.GreaterOrEqual(t, second.Conflict.TotalHunkCount, 1)
	assert.NotEmpty(t, second.Conflict.Reason)

	// 升级后应已执行 merge --abort，不应残留进行中的 merge 状态。
	checkMergeHead := exec.Command("git", "rev-parse", "-q", "--verify", "MERGE_HEAD")
	checkMergeHead.Dir = dir
	_, mergeHeadErr := checkMergeHead.CombinedOutput()
	assert.Error(t, mergeHeadErr)

	// integration 分支应保持在已成功合入的状态（仅包含 feat/40-core-a）。
	content := runGitOutput(t, dir, "show", "integration/test-escalated:pkg/service.go")
	assert.Contains(t, content, "return 1")
	assert.NotContains(t, content, "return 2")
}

func TestCanAutoResolveConflict_DoesNotStripCommentMarkersInsideStringLiteral(t *testing.T) {
	fileSummary := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{
				Ours:   `const endpoint = "http://service-a/*v1*/"` + "\n" + `const path = "api//v1"`,
				Theirs: `const endpoint = "http://service-b/*v2*/"` + "\n" + `const path = "api//v2"`,
			},
		},
	}

	auto, reason := canAutoResolveConflict("pkg/service.go", fileSummary)
	assert.False(t, auto)
	assert.Contains(t, reason, "语义差异")
}

func TestCanAutoResolveConflict_DoesNotTreatLeadingAsteriskCodeAsComment(t *testing.T) {
	fileSummary := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{
				Ours:   "\t*ptr = 1\nreturn value",
				Theirs: "\t*ptr = 2\nreturn value",
			},
		},
	}

	auto, reason := canAutoResolveConflict("pkg/service.go", fileSummary)
	assert.False(t, auto)
	assert.Contains(t, reason, "语义差异")
}

func TestCanAutoResolveConflict_DoesNotDropCodeAfterLeadingBlockComment(t *testing.T) {
	fileSummary := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{
				Ours:   "/*note*/ return 1",
				Theirs: "/*note*/ return 2",
			},
		},
	}

	auto, reason := canAutoResolveConflict("pkg/service.go", fileSummary)
	assert.False(t, auto)
	assert.Contains(t, reason, "语义差异")
}

func TestHasHighRiskConflict_HunkBoundary(t *testing.T) {
	summaryAtLimit := ConflictFileSummary{
		Hunks: 6,
		Blocks: []ConflictBlock{
			{Ours: "valueA", Theirs: "valueB"},
		},
	}
	risky, reason := hasHighRiskConflict("pkg/service.go", summaryAtLimit)
	assert.False(t, risky)
	assert.Empty(t, reason)

	summaryOverLimit := ConflictFileSummary{
		Hunks: 7,
		Blocks: []ConflictBlock{
			{Ours: "valueA", Theirs: "valueB"},
		},
	}
	risky, reason = hasHighRiskConflict("pkg/service.go", summaryOverLimit)
	assert.True(t, risky)
	assert.Contains(t, reason, "冲突块过多")
}

func TestIsAdjacentMildConflict_BlockLineBoundary(t *testing.T) {
	summaryAtLimit := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{Ours: buildConflictSideLines(12), Theirs: buildConflictSideLines(12)},
		},
	}
	assert.True(t, isAdjacentMildConflict(summaryAtLimit))

	summaryOverLimit := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{Ours: buildConflictSideLines(13), Theirs: buildConflictSideLines(12)},
		},
	}
	assert.False(t, isAdjacentMildConflict(summaryOverLimit))
}

func TestHasHighRiskConflict_TotalLineBoundary(t *testing.T) {
	summaryAtLimit := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{Ours: buildConflictSideLines(60), Theirs: buildConflictSideLines(60)},
		},
	}
	risky, reason := hasHighRiskConflict("pkg/service.go", summaryAtLimit)
	assert.False(t, risky)
	assert.Empty(t, reason)

	summaryOverLimit := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{Ours: buildConflictSideLines(61), Theirs: buildConflictSideLines(60)},
		},
	}
	risky, reason = hasHighRiskConflict("pkg/service.go", summaryOverLimit)
	assert.True(t, risky)
	assert.Contains(t, reason, "冲突行数过多")
}

func buildConflictSideLines(lines int) string {
	var b strings.Builder
	for i := 0; i < lines; i++ {
		b.WriteString(fmt.Sprintf("line_%d\n", i))
	}
	return strings.TrimSuffix(b.String(), "\n")
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
