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
// base 指定从哪个分支切出，为空时默认从 master
// 返回 worktree 的绝对路径
func (w *Workspace) Create(issueNum int, slug string, base string) (string, error) {
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

	// base 为空时默认 master
	if base == "" {
		base = "master"
	}

	// 确保 base 是最新的（有 origin 时 fetch）
	if w.hasOrigin() {
		// fetch 失败不阻断，使用本地分支
		_ = w.fetchRef(base)
		// 优先使用 origin/base，不存在时回退到本地 base
		if out, err := w.gitOutput("branch", "--list", "-r", "origin/"+base); err == nil && strings.TrimSpace(out) != "" {
			base = "origin/" + base
		}
	}

	// git worktree add -b <branch> <path> <base>
	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath, base)
	cmd.Dir = w.RepoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("创建 worktree 失败: %w\n%s", err, string(out))
	}

	return wtPath, nil
}

// Checkout 从已有分支创建 worktree（不创建新分支）
// 用于 iterate 阶段重建已被清理的 worktree（分支在 implement 阶段已创建）
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

	// git worktree add <path> <branch>（不带 -b，使用已有分支）
	cmd := exec.Command("git", "worktree", "add", wtPath, branch)
	cmd.Dir = w.RepoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("checkout worktree 失败: %w\n%s", err, string(out))
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

// gitOutput 执行 git 命令并返回输出
func (w *Workspace) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = w.RepoDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
