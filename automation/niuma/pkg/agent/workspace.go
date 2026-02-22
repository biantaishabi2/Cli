// pkg/agent/workspace.go
// Workspace：通过 git worktree 隔离每个 issue 的开发环境
package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Workspace 管理 git worktree，为每个 issue 创建独立工作目录
type Workspace struct {
	RepoDir string // 主仓库本地路径
}

// NewWorkspace 创建 Workspace
func NewWorkspace(repoDir string) *Workspace {
	// 规范化路径，防止路径穿越
	absPath, err := filepath.Abs(repoDir)
	if err != nil {
		absPath = filepath.Clean(repoDir)
	}
	return &Workspace{RepoDir: absPath}
}

// Create 创建 worktree 并切出新分支
// base 为空时默认使用 master，返回 worktree 的绝对路径
func (w *Workspace) Create(issueNum int, slug, base string) (string, error) {
	wtPath := w.Path(issueNum)
	branch := w.branchName(issueNum, slug)

	// 如果已存在，直接返回
	if w.Exists(issueNum) {
		return wtPath, nil
	}

	// 确保 .worktrees 目录存在
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return "", fmt.Errorf("创建 .worktrees 目录失败: %w", err)
	}

	// 解析创建分支的基线；有 origin 时优先使用 origin/<base>
	baseRef, err := w.resolveWorktreeBaseRef(base)
	if err != nil {
		return "", err
	}

	// 基于 base 创建新分支的 worktree
	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath, baseRef)
	cmd.Dir = w.RepoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("创建 worktree 失败: %w\n%s", err, string(out))
	}

	return wtPath, nil
}

// Checkout 从已有分支创建 worktree（不创建新分支）
// 用于 iterate 阶段重建已被清理的 worktree（分支在 implement 阶段已创建并推到远程）
// branch 参数不带 origin/ 前缀，如 "fix/37-slug"
func (w *Workspace) Checkout(issueNum int, branch string) (string, error) {
	wtPath := w.Path(issueNum)

	// 如果已存在，直接返回
	if w.Exists(issueNum) {
		return wtPath, nil
	}

	// 确保 .worktrees 目录存在
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return "", fmt.Errorf("创建 .worktrees 目录失败: %w", err)
	}

	// 有 origin 时先 fetch，再用 -b 创建本地分支跟踪远程（确保 push 时有本地分支）
	// 没有 origin 时直接用本地分支
	if w.hasOrigin() {
		if err := w.fetchRef(branch); err != nil {
			return "", fmt.Errorf("fetch %s 失败: %w", branch, err)
		}
		// 删除可能残留的同名本地分支（避免 -b 冲突）
		del := exec.Command("git", "branch", "-D", branch)
		del.Dir = w.RepoDir
		del.Run() // 忽略错误（分支不存在时会失败）

		// 创建 worktree 并建立本地分支跟踪 origin/<branch>
		cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath, "origin/"+branch)
		cmd.Dir = w.RepoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("checkout worktree 失败: %w\n%s", err, string(out))
		}
	} else {
		cmd := exec.Command("git", "worktree", "add", wtPath, branch)
		cmd.Dir = w.RepoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("checkout worktree 失败: %w\n%s", err, string(out))
		}
	}

	return wtPath, nil
}

// Remove 移除 worktree
func (w *Workspace) Remove(issueNum int) error {
	wtPath := w.Path(issueNum)

	if !w.Exists(issueNum) {
		return nil // 不存在则无需操作
	}

	cmd := exec.Command("git", "worktree", "remove", wtPath, "--force")
	cmd.Dir = w.RepoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("移除 worktree 失败: %w\n%s", err, string(out))
	}

	return nil
}

// Path 返回 worktree 路径：{RepoDir}/.worktrees/fix-{issueNum}
func (w *Workspace) Path(issueNum int) string {
	return filepath.Join(w.RepoDir, ".worktrees", fmt.Sprintf("fix-%d", issueNum))
}

// Exists 检查 worktree 是否存在
func (w *Workspace) Exists(issueNum int) bool {
	wtPath := w.Path(issueNum)
	info, err := os.Stat(wtPath)
	return err == nil && info.IsDir()
}

// EnsureBranch 确保分支存在并指向正确的 base
// 如果 worktree 已存在，检查分支名是否匹配
func (w *Workspace) EnsureBranch(issueNum int, slug, base string) error {
	branch := w.branchName(issueNum, slug)

	// 检查分支是否已存在
	cmd := exec.Command("git", "branch", "--list", branch)
	cmd.Dir = w.RepoDir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("检查分支失败: %w", err)
	}

	if strings.TrimSpace(string(out)) != "" {
		return nil // 分支已存在
	}

	// 创建分支
	cmd = exec.Command("git", "branch", branch, base)
	cmd.Dir = w.RepoDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("创建分支 %s 失败: %w\n%s", branch, err, string(out))
	}

	return nil
}

// BranchName 返回分支名（公开供外部使用）
func (w *Workspace) BranchName(issueNum int, slug string) string {
	return w.branchName(issueNum, slug)
}

// branchName 返回分支名：fix/{issueNum}-{slug}
func (w *Workspace) branchName(issueNum int, slug string) string {
	if slug == "" {
		return fmt.Sprintf("fix/%d", issueNum)
	}
	return fmt.Sprintf("fix/%d-%s", issueNum, slug)
}

// resolveWorktreeBaseRef 解析 worktree 创建时使用的基线引用。
// 有 origin 时优先 fetch 并使用 origin/<base>，否则回退到本地分支。
func (w *Workspace) resolveWorktreeBaseRef(base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "master"
	}

	if w.hasOrigin() {
		if err := w.fetchRef(base); err == nil {
			return "origin/" + base, nil
		}
		if w.branchExists(base) {
			return base, nil
		}
		return "", fmt.Errorf("基线分支不存在: %s", base)
	}

	if !w.branchExists(base) {
		return "", fmt.Errorf("基线分支不存在: %s", base)
	}
	return base, nil
}

// hasOrigin 检查是否存在 origin remote
func (w *Workspace) hasOrigin() bool {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = w.RepoDir
	return cmd.Run() == nil
}

// fetchRef 从 origin fetch 指定引用
func (w *Workspace) fetchRef(ref string) error {
	cmd := exec.Command("git", "fetch", "origin", ref)
	cmd.Dir = w.RepoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch origin %s: %w\n%s", ref, err, string(out))
	}
	return nil
}

// branchExists 检查本地分支是否存在。
func (w *Workspace) branchExists(branch string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = w.RepoDir
	return cmd.Run() == nil
}
