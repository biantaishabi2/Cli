// pkg/agent/gitops.go
// GitOps：在 worktree 目录中执行 git 操作（commit、push 等）
package agent

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitOps 封装工作目录中的 git 操作
type GitOps struct {
	WorkDir string // 工作目录（通常是 worktree 路径）
}

// NewGitOps 创建 GitOps
func NewGitOps(workDir string) *GitOps {
	return &GitOps{WorkDir: workDir}
}

// HasChanges 检查是否有未提交的变更（包括 untracked 文件）
func (g *GitOps) HasChanges() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status 失败: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// CommitAll 将所有变更加入暂存区并提交
func (g *GitOps) CommitAll(message string) error {
	// git add -A（worktree 为隔离环境，需要暂存新建文件；.gitignore 过滤敏感文件）
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = g.WorkDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add 失败: %w\n%s", err, string(out))
	}

	// git commit -m <message>
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = g.WorkDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit 失败: %w\n%s", err, string(out))
	}

	return nil
}

// Push 推送到远程
func (g *GitOps) Push(branch string) error {
	cmd := exec.Command("git", "push", "-u", "origin", branch)
	cmd.Dir = g.WorkDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push 失败: %w\n%s", err, string(out))
	}
	return nil
}

// CurrentBranch 返回当前分支名
func (g *GitOps) CurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = g.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取当前分支失败: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
