// pkg/agent/gitops.go
// GitOps：在 worktree 目录中执行 git 操作（commit、push 等）
package agent

import (
	"fmt"
	"os/exec"
	"path/filepath"
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

// sensitiveExactNames 精确匹配的敏感文件名（commit 前检查，拒绝提交）
var sensitiveExactNames = []string{
	"credentials.json",
	"secrets.yml",
	"secrets.yaml",
	"id_rsa",
	"id_ed25519",
	"id_ecdsa",
}

// sensitiveSuffixes 敏感文件后缀
var sensitiveSuffixes = []string{
	".key",
	".pem",
	".p12",
	".pfx",
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

	// 检查暂存区是否包含敏感文件
	if err := g.checkSensitiveFiles(); err != nil {
		// 回滚暂存区
		resetCmd := exec.Command("git", "reset", "HEAD")
		resetCmd.Dir = g.WorkDir
		_ = resetCmd.Run()
		return err
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

// checkSensitiveFiles 检查暂存区是否包含敏感文件
func (g *GitOps) checkSensitiveFiles() error {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = g.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("检查暂存区失败（安全检查不可跳过）: %w", err)
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil // 无暂存文件
	}

	files := strings.Split(raw, "\n")
	var found []string
	for _, f := range files {
		if isSensitiveFile(f) {
			found = append(found, f)
		}
	}

	if len(found) > 0 {
		return fmt.Errorf("暂存区包含敏感文件，已阻止提交: %v", found)
	}
	return nil
}

// isSensitiveFile 检查文件是否为敏感文件（基于文件名，不含路径）
func isSensitiveFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))

	// .env 文件（.env, .env.local, .env.staging, .env.production 等）
	if name == ".env" || strings.HasPrefix(name, ".env.") {
		return true
	}

	// 精确匹配已知敏感文件名
	for _, pattern := range sensitiveExactNames {
		if name == pattern {
			return true
		}
	}

	// 危险后缀
	for _, suffix := range sensitiveSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}

	return false
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

// ChangedFiles 获取当前分支相对于 base 的改动文件列表
// 使用 merge-base 找到分叉点，再用两点号 diff 获取精确改动
func (g *GitOps) ChangedFiles(base string) ([]string, error) {
	// 先找 merge-base（分叉点）
	mbCmd := exec.Command("git", "merge-base", base, "HEAD")
	mbCmd.Dir = g.WorkDir
	mbOut, err := mbCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git merge-base 失败: %w", err)
	}
	mergeBase := strings.TrimSpace(string(mbOut))

	// 用两点号 diff：merge-base..HEAD
	cmd := exec.Command("git", "diff", "--name-only", mergeBase+"..HEAD")
	cmd.Dir = g.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only 失败: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}

// DefaultBranch 获取远程默认分支名（origin/HEAD 指向）
// 失败时回退到 "master"
func (g *GitOps) DefaultBranch() string {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = g.WorkDir
	out, err := cmd.Output()
	if err != nil {
		return "master"
	}
	// 输出格式：refs/remotes/origin/main
	ref := strings.TrimSpace(string(out))
	parts := strings.Split(ref, "/")
	if branch := parts[len(parts)-1]; branch != "" {
		return branch
	}
	return "master"
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
