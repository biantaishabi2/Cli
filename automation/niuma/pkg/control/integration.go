// pkg/control/integration.go
// IntegrationBuilder 构建 integration 分支（从 master 按 topo 序逐个 merge）
package control

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// IntegrationBuilder 构建 integration 分支
type IntegrationBuilder struct {
	repoDir        string
	baseBranch     string
	branchPrefix   string // 默认 "integration/batch-"
	maxOldBranches int    // 保留最近 N 个旧分支，默认 3
}

// NewIntegrationBuilder 创建构建器
func NewIntegrationBuilder(repoDir, baseBranch, branchPrefix string, maxOldBranches int) *IntegrationBuilder {
	if branchPrefix == "" {
		branchPrefix = "integration/batch-"
	}
	if maxOldBranches <= 0 {
		maxOldBranches = 3
	}
	return &IntegrationBuilder{
		repoDir:        repoDir,
		baseBranch:     baseBranch,
		branchPrefix:   branchPrefix,
		maxOldBranches: maxOldBranches,
	}
}

// Build 按 topo 序构建 integration 分支
// branches 应已按 topo 序排列
// deps 记录依赖关系：key 的分支依赖 value 列表中的 issue
func (b *IntegrationBuilder) Build(branches []BranchInfo, deps map[int][]int) (*IntegrationResult, error) {
	branchName := b.branchPrefix + time.Now().Format("20060102-150405")

	// 从 baseBranch 创建新分支
	if err := b.git("checkout", b.baseBranch); err != nil {
		return nil, fmt.Errorf("checkout %s 失败: %w", b.baseBranch, err)
	}
	if err := b.git("checkout", "-b", branchName); err != nil {
		return nil, fmt.Errorf("创建 integration 分支失败: %w", err)
	}

	result := &IntegrationResult{Branch: branchName}

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

// CleanOld 清理旧的 integration 分支，保留最近 N 个
func (b *IntegrationBuilder) CleanOld() error {
	out, err := b.gitOutput("branch", "--list", b.branchPrefix+"*")
	if err != nil {
		return nil // 没有匹配的分支
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var branches []string
	for _, line := range lines {
		branch := strings.TrimSpace(line)
		branch = strings.TrimPrefix(branch, "* ") // 当前分支标记
		if branch != "" {
			branches = append(branches, branch)
		}
	}

	if len(branches) <= b.maxOldBranches {
		return nil
	}

	// 按名称排序（时间戳在名称中，所以字典序即时间序）
	sort.Strings(branches)

	// 删除最老的
	toDelete := branches[:len(branches)-b.maxOldBranches]
	for _, branch := range toDelete {
		if err := b.git("branch", "-D", branch); err != nil {
			fmt.Printf("[control] 删除旧分支 %s 失败: %v\n", branch, err)
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
