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

// setupGitRepoWithBareRemote 创建带 bare origin 的测试仓库，返回工作仓库路径与远端路径。
func setupGitRepoWithBareRemote(t *testing.T) (string, string) {
	t.Helper()

	remoteDir := filepath.Join(t.TempDir(), "origin.git")
	runGit(t, "", "init", "--bare", remoteDir)

	workDir := setupGitRepo(t)
	runGit(t, workDir, "remote", "add", "origin", remoteDir)
	runGit(t, workDir, "push", "-u", "origin", "master")
	return workDir, remoteDir
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
	}, "")
	require.NoError(t, err)
	assert.Equal(t, MergeStatusMerged, first.Status)

	second, err := builder.ExecuteIntegrationMerge("integration/test-auto", BranchInfo{
		Branch:   "feat/41-doc-b",
		IssueNum: 41,
		PRNum:    401,
	}, "")
	require.NoError(t, err)
	assert.Equal(t, MergeStatusAutoResolved, second.Status)
	assert.Contains(t, second.AutoResolvedFiles, "docs/guide.md")

	unmerged := runGitOutput(t, dir, "diff", "--name-only", "--diff-filter=U")
	assert.Empty(t, unmerged)

	content := runGitOutput(t, dir, "show", "integration/test-auto:docs/guide.md")
	assert.Contains(t, content, "line from B")
}

func TestIntegrationBuilder_PushBranch_Success(t *testing.T) {
	dir, remoteDir := setupGitRepoWithBareRemote(t)
	createBranch(t, dir, "feat/40-auth", "auth.go", "package auth\n")

	builder := NewIntegrationBuilder(dir, "master")
	outcome, err := builder.ExecuteIntegrationMerge("integration/main", BranchInfo{
		Branch:   "feat/40-auth",
		IssueNum: 40,
		PRNum:    400,
	}, "")
	require.NoError(t, err)
	assert.Equal(t, MergeStatusMerged, outcome.Status)

	err = builder.PushBranch("integration/main")
	require.NoError(t, err)

	cmd := exec.Command("git", "--git-dir", remoteDir, "rev-parse", "refs/heads/integration/main")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	assert.NotEmpty(t, strings.TrimSpace(string(out)))
}

func TestIntegrationBuilder_PushBranch_EmptyBranch(t *testing.T) {
	dir := setupGitRepo(t)
	builder := NewIntegrationBuilder(dir, "master")

	err := builder.PushBranch("   ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "分支名为空")
}

func TestIntegrationBuilder_EnsureBranch_UsesRemoteIntegrationAsBase(t *testing.T) {
	// 第一次：在 repo A 中完成一轮 integration 合入并推送。
	repoA, remoteDir := setupGitRepoWithBareRemote(t)
	createBranch(t, repoA, "feat/40-auth", "auth.go", "package auth\n")
	runGit(t, repoA, "push", "-u", "origin", "feat/40-auth")

	builderA := NewIntegrationBuilder(repoA, "master")
	outcomeA, err := builderA.ExecuteIntegrationMerge("integration/main", BranchInfo{
		Branch:   "feat/40-auth",
		IssueNum: 40,
		PRNum:    400,
	}, "")
	require.NoError(t, err)
	assert.Equal(t, MergeStatusMerged, outcomeA.Status)
	require.NoError(t, builderA.PushBranch("integration/main"))

	// 第二次：在 repo B（新工作目录）中继续合入下一条分支。
	repoB := filepath.Join(t.TempDir(), "repo-b")
	runGit(t, "", "clone", remoteDir, repoB)
	runGit(t, repoB, "config", "user.email", "test@test.com")
	runGit(t, repoB, "config", "user.name", "Test")

	createBranch(t, repoB, "feat/41-payment", "payment.go", "package payment\n")
	runGit(t, repoB, "push", "-u", "origin", "feat/41-payment")

	builderB := NewIntegrationBuilder(repoB, "master")
	outcomeB, err := builderB.ExecuteIntegrationMerge("integration/main", BranchInfo{
		Branch:   "feat/41-payment",
		IssueNum: 41,
		PRNum:    401,
	}, "")
	require.NoError(t, err)
	assert.Equal(t, MergeStatusMerged, outcomeB.Status)
	require.NoError(t, builderB.PushBranch("integration/main"))

	// 验证远端 integration/main 已连续包含两次合入结果。
	cmd := exec.Command("git", "--git-dir", remoteDir, "show", "refs/heads/integration/main:auth.go")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "package auth")

	cmd = exec.Command("git", "--git-dir", remoteDir, "show", "refs/heads/integration/main:payment.go")
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "package payment")
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
	}, "")
	require.NoError(t, err)
	assert.Equal(t, MergeStatusMerged, first.Status)

	second, err := builder.ExecuteIntegrationMerge("integration/test-escalated", BranchInfo{
		Branch:   "feat/41-core-b",
		IssueNum: 41,
		PRNum:    501,
	}, "")
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

func TestIsRustUseConflictOnly_PureUseConflict(t *testing.T) {
	fileSummary := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{
				Ours:   "use std::collections::HashMap;",
				Theirs: "use std::collections::HashSet;",
			},
		},
	}
	assert.True(t, isRustUseConflictOnly(fileSummary))
}

func TestIsRustUseConflictOnly_BracedExpansion(t *testing.T) {
	fileSummary := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{
				Ours:   "use std::collections::{HashMap, HashSet};",
				Theirs: "use std::io::{Read, Write};",
			},
		},
	}
	assert.True(t, isRustUseConflictOnly(fileSummary))
}

func TestIsRustUseConflictOnly_WithTraitRejectsFalse(t *testing.T) {
	fileSummary := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{
				Ours:   "trait Serializable { fn serialize(&self) -> Vec<u8>; }",
				Theirs: "trait Serializable { fn serialize(&self) -> String; }",
			},
		},
	}
	assert.False(t, isRustUseConflictOnly(fileSummary))
}

func TestIsRustUseConflictOnly_GlobImportRejected(t *testing.T) {
	fileSummary := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{
				Ours:   "use std::collections::*;",
				Theirs: "use std::collections::HashMap;",
			},
		},
	}
	assert.False(t, isRustUseConflictOnly(fileSummary))
}

func TestIsRustUseConflictOnly_RenameAsRejected(t *testing.T) {
	fileSummary := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{
				Ours:   "use std::collections::HashMap as Map;",
				Theirs: "use std::collections::HashMap;",
			},
		},
	}
	assert.False(t, isRustUseConflictOnly(fileSummary))
}

func TestContainsRustCoreSignal_DetectsTraitImplUnsafeMacro(t *testing.T) {
	cases := map[string]bool{
		"trait Foo {}":                          true,
		"pub trait Bar { fn bar(); }":           true,
		"impl Foo for Bar {}":                   true,
		"impl<T> Foo for Vec<T> {}":             true,
		"unsafe fn do_thing() {}":               true,
		"macro_rules! my_macro {}":              true,
		"use std::collections::HashMap;":        false,
		"fn normal_function() {}":               false,
		"let x = 42;":                           false,
	}

	for input, expected := range cases {
		result := containsRustCoreSignal(input)
		assert.Equal(t, expected, result, "input: %s", input)
	}
}

func TestIsAIConflictWhitelisted_RustUseOnly(t *testing.T) {
	fileSummary := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{
				Ours:   "use std::collections::HashMap;",
				Theirs: "use std::collections::HashSet;",
			},
		},
	}
	allowed, reason := isAIConflictWhitelisted("src/lib.rs", fileSummary)
	assert.True(t, allowed)
	assert.Equal(t, "rust-use-only", reason)
}

func TestHasHighRiskConflict_RustTraitChange(t *testing.T) {
	fileSummary := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{
				Ours:   "trait Serializable {\n    fn serialize(&self) -> Vec<u8>;\n}",
				Theirs: "trait Serializable {\n    fn serialize(&self) -> String;\n}",
			},
		},
	}
	risky, reason := hasHighRiskConflict("src/lib.rs", fileSummary)
	assert.True(t, risky)
	assert.Contains(t, reason, "Rust 核心 trait/impl")
}

func TestHasHighRiskConflict_RustUseNotHighRisk(t *testing.T) {
	fileSummary := ConflictFileSummary{
		Hunks: 1,
		Blocks: []ConflictBlock{
			{
				Ours:   "use std::collections::HashMap;",
				Theirs: "use std::collections::HashSet;",
			},
		},
	}
	risky, _ := hasHighRiskConflict("src/lib.rs", fileSummary)
	assert.False(t, risky)
}

func TestComputeOldestMergeBase(t *testing.T) {
	dir := setupGitRepo(t)

	// master 有初始 commit (c0)
	// 创建分支 A 基于 c0
	createBranch(t, dir, "feat/50-a", "a.go", "package a\n")

	// master 前进 (c1)
	runGit(t, dir, "checkout", "master")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.go"), []byte("package extra\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "master advance c1")

	// 创建分支 B 基于 c1
	createBranch(t, dir, "feat/51-b", "b.go", "package b\n")

	// master 再前进 (c2)
	runGit(t, dir, "checkout", "master")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra2.go"), []byte("package extra2\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "master advance c2")

	builder := NewIntegrationBuilder(dir, "master")

	// 分支 A 的 merge-base 是 c0，分支 B 的 merge-base 是 c1
	// 最旧的应该是 c0
	oldest, err := builder.ComputeOldestMergeBase([]string{"feat/50-a", "feat/51-b"})
	require.NoError(t, err)
	assert.NotEmpty(t, oldest)

	// 验证 oldest 是 feat/50-a 的 merge-base
	mbA := runGitOutput(t, dir, "merge-base", "master", "feat/50-a")
	assert.Equal(t, mbA, oldest)

	// 且 oldest 是 feat/51-b merge-base 的祖先
	mbB := runGitOutput(t, dir, "merge-base", "master", "feat/51-b")
	assert.NotEqual(t, mbA, mbB, "两个 merge-base 应不同")

	// 验证 oldest 是其他 merge-base 的祖先（关键断言）
	isAncestorCmd := exec.Command("git", "merge-base", "--is-ancestor", oldest, mbB)
	isAncestorCmd.Dir = dir
	assert.NoError(t, isAncestorCmd.Run(), "oldest merge-base 应是其他 merge-base 的祖先")

	// 空列表返回空
	empty, err := builder.ComputeOldestMergeBase(nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestIntegrationBuilder_MasterAdvanced_NoFalseConflict(t *testing.T) {
	dir := setupGitRepo(t)

	// master 有初始文件 (c0)
	// 创建分支 A 基于 c0，修改不同文件
	createBranch(t, dir, "feat/60-a", "a.go", "package a\n")
	createBranch(t, dir, "feat/61-b", "b.go", "package b\n")

	// master 前进：新增文件（不与 PR 分支冲突）
	runGit(t, dir, "checkout", "master")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new_on_master.go"), []byte("package newmaster\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "master advance")

	builder := NewIntegrationBuilder(dir, "master")

	branches := []BranchInfo{
		{Branch: "feat/60-a", IssueNum: 60},
		{Branch: "feat/61-b", IssueNum: 61},
	}

	// Build 应成功，两个分支都能 merge，不产生假冲突
	result, err := builder.Build("integration/test-no-false-conflict", branches, nil)
	require.NoError(t, err)
	assert.Equal(t, []int{60, 61}, result.Merged)
	assert.Empty(t, result.Conflicts)

	// integration 分支不应包含 master 新增的文件（因为起点是 c0）
	cmd := exec.Command("git", "show", "integration/test-no-false-conflict:new_on_master.go")
	cmd.Dir = dir
	_, showErr := cmd.CombinedOutput()
	assert.Error(t, showErr, "integration 分支不应包含 master 新增的文件")
}

func TestIntegrationBuilder_SinglePRBranch_MasterAdvanced(t *testing.T) {
	dir := setupGitRepo(t)

	// 创建分支 A 基于 c0
	createBranch(t, dir, "feat/80-single", "single.go", "package single\n")

	// master 前进
	runGit(t, dir, "checkout", "master")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "master_new.go"), []byte("package masternew\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "master advance")

	builder := NewIntegrationBuilder(dir, "master")

	branches := []BranchInfo{
		{Branch: "feat/80-single", IssueNum: 80},
	}

	// Build 应成功，单个分支正常 merge
	result, err := builder.Build("integration/test-single-pr", branches, nil)
	require.NoError(t, err)
	assert.Equal(t, []int{80}, result.Merged)
	assert.Empty(t, result.Conflicts)

	// integration 分支应从该分支的 merge-base 创建，不包含 master 新增文件
	cmd := exec.Command("git", "show", "integration/test-single-pr:master_new.go")
	cmd.Dir = dir
	_, showErr := cmd.CombinedOutput()
	assert.Error(t, showErr, "integration 分支不应包含 master 新增的文件")

	// 但应包含 PR 分支的文件
	content := runGitOutput(t, dir, "show", "integration/test-single-pr:single.go")
	assert.Contains(t, content, "package single")
}

func TestIntegrationBuilder_RealConflictStillDetected(t *testing.T) {
	dir := setupGitRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "shared.go"), []byte("package pkg\n\nfunc Version() int { return 0 }\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add shared.go")

	// 两个分支修改同文件同区域
	createBranchModifyFile(t, dir, "feat/70-a", "pkg/shared.go", "package pkg\n\nfunc Version() int { return 1 }\n")
	createBranchModifyFile(t, dir, "feat/71-b", "pkg/shared.go", "package pkg\n\nfunc Version() int { return 2 }\n")

	// master 前进
	runGit(t, dir, "checkout", "master")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "unrelated.go"), []byte("package unrelated\n"), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "master advance")

	builder := NewIntegrationBuilder(dir, "master")

	branches := []BranchInfo{
		{Branch: "feat/70-a", IssueNum: 70},
		{Branch: "feat/71-b", IssueNum: 71},
	}

	result, err := builder.Build("integration/test-real-conflict", branches, nil)
	require.NoError(t, err)
	assert.Contains(t, result.Merged, 70)
	assert.Contains(t, result.Conflicts, 71)
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
