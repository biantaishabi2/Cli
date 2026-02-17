// pkg/control/integration.go
// IntegrationBuilder 构建 integration 分支
// 支持两种模式：增量模式（EnsureBranch + MergeOne）和批量模式（Build，作为 fallback）
package control

import (
	"fmt"
	"os/exec"
	"strings"
)

// IntegrationBuilder 构建 integration 分支
type IntegrationBuilder struct {
	repoDir    string
	baseBranch string
}

// NewIntegrationBuilder 创建构建器
func NewIntegrationBuilder(repoDir, baseBranch string) *IntegrationBuilder {
	return &IntegrationBuilder{
		repoDir:    repoDir,
		baseBranch: baseBranch,
	}
}

// IntegrationBranchName 生成 integration 分支名
// metaIssueSlug: 总任务的 slug，如 "phase-3"，生成 "integration/phase-3"
func IntegrationBranchName(metaIssueSlug string) string {
	if metaIssueSlug == "" {
		return "integration/main"
	}
	return "integration/" + metaIssueSlug
}

// EnsureBranch 确保 integration 分支存在（首次从 master 创建）
// branchName: integration 分支名，如 "integration/phase-3"
func (b *IntegrationBuilder) EnsureBranch(branchName string) (string, error) {
	// 检查 integration 分支是否已存在
	out, err := b.gitOutput("branch", "--list", branchName)
	if err == nil && strings.TrimSpace(out) != "" {
		return branchName, nil
	}

	// 从 baseBranch 创建 integration 分支
	if err := b.git("branch", branchName, b.baseBranch); err != nil {
		return "", fmt.Errorf("创建 integration 分支 %s 失败: %w", branchName, err)
	}

	return branchName, nil
}

// MergeOne 将单个完成的 PR 分支 merge 进 integration
// integrationBranch: 目标 integration 分支名
func (b *IntegrationBuilder) MergeOne(integrationBranch string, bi BranchInfo) error {
	// 确保 integration 分支存在
	if _, err := b.EnsureBranch(integrationBranch); err != nil {
		return err
	}

	// checkout integration 分支
	if err := b.git("checkout", integrationBranch); err != nil {
		return fmt.Errorf("checkout %s 失败: %w", integrationBranch, err)
	}
	defer b.git("checkout", b.baseBranch)

	// merge PR 分支
	msg := fmt.Sprintf("Merge %s (issue #%d)", bi.Branch, bi.IssueNum)
	if err := b.git("merge", "--no-ff", bi.Branch, "-m", msg); err != nil {
		_ = b.git("merge", "--abort")
		return fmt.Errorf("merge %s 冲突: %w", bi.Branch, err)
	}

	return nil
}

// Reset 从 master 重建 integration 分支（冲突无法解决时）
// branchName: 要重建的 integration 分支名
func (b *IntegrationBuilder) Reset(branchName string) error {
	// 确保不在 integration 分支上
	_ = b.git("checkout", b.baseBranch)

	// 删除旧的 integration 分支（如果存在）
	_ = b.git("branch", "-D", branchName)

	// 从 baseBranch 重新创建
	if err := b.git("branch", branchName, b.baseBranch); err != nil {
		return fmt.Errorf("重建 integration 分支 %s 失败: %w", branchName, err)
	}

	return nil
}

// Build 按 topo 序构建 integration 分支（批量模式，作为 fallback）
// integrationBranch: 目标 integration 分支名
// branches: 应已按 topo 序排列
// deps: 依赖关系，key 的分支依赖 value 列表中的 issue
func (b *IntegrationBuilder) Build(integrationBranch string, branches []BranchInfo, deps map[int][]int) (*IntegrationResult, error) {
	// 重置 integration 分支，从头开始批量 merge
	if err := b.Reset(integrationBranch); err != nil {
		return nil, fmt.Errorf("重置 integration 分支失败: %w", err)
	}

	if err := b.git("checkout", integrationBranch); err != nil {
		return nil, fmt.Errorf("checkout %s 失败: %w", integrationBranch, err)
	}
	defer b.git("checkout", b.baseBranch)

	result := &IntegrationResult{Branch: integrationBranch}

	// 构建 issue → 是否失败的映射（用于级联跳过）
	failed := make(map[int]bool)

	for _, bi := range branches {
		// 检查是否需要级联跳过
		if shouldSkip(bi.IssueNum, deps, failed) {
			result.Skipped = append(result.Skipped, bi.IssueNum)
			continue
		}

		// 尝试 merge
		if err := b.git("merge", "--no-ff", bi.Branch, "-m", fmt.Sprintf("Merge %s (issue #%d)", bi.Branch, bi.IssueNum)); err != nil {
			// merge 冲突，abort 并记录
			_ = b.git("merge", "--abort")
			result.Conflicts = append(result.Conflicts, bi.IssueNum)
			failed[bi.IssueNum] = true
			continue
		}

		result.Merged = append(result.Merged, bi.IssueNum)
	}

	return result, nil
}

// shouldSkip 检查是否因依赖失败需要级联跳过
func shouldSkip(issueNum int, deps map[int][]int, failed map[int]bool) bool {
	for _, dep := range deps[issueNum] {
		if failed[dep] {
			return true
		}
	}
	return false
}

// CleanOld 清理旧的 integration 分支，保留最近 3 个
// 删除 pattern: integration/batch-* 或 integration/test-* 的旧分支
func (b *IntegrationBuilder) CleanOld() error {
	// 获取所有 integration/batch-* 和 integration/test-* 分支，按创建时间排序
	out, err := b.gitOutput("branch", "--list", "integration/batch-*", "integration/test-*", "--sort=-creatordate")
	if err != nil {
		return fmt.Errorf("列出 integration 分支失败: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 移除开头的 * (当前分支标记)
		line = strings.TrimPrefix(line, "* ")
		if line != "" {
			branches = append(branches, line)
		}
	}

	// 保留最近 3 个，删除其余的
	if len(branches) > 3 {
		for _, branch := range branches[3:] {
			if err := b.git("branch", "-D", branch); err != nil {
				// 忽略删除失败（可能分支不存在）
				continue
			}
		}
	}

	return nil
}

// git 执行 git 命令
func (b *IntegrationBuilder) git(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = b.repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return nil
}

// gitOutput 执行 git 命令并返回输出
func (b *IntegrationBuilder) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = b.repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}
