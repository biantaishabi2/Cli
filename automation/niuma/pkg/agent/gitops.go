// pkg/agent/gitops.go
// GitOps：在 worktree 目录中执行 git 操作（commit、push 等）
package agent

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const fallbackDefaultBranch = "master"

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
	return resolveDefaultBranch(g.WorkDir).Branch
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

// gitOutputExecutor 抽象 git 命令执行，便于默认分支解析单测注入 mock。
type gitOutputExecutor interface {
	CombinedOutput(workDir string, args ...string) ([]byte, error)
}

// osGitOutputExecutor 使用系统 git 执行命令。
type osGitOutputExecutor struct{}

func (osGitOutputExecutor) CombinedOutput(workDir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	return cmd.CombinedOutput()
}

// defaultBranchResolution 表示默认分支探测结果与诊断信息。
type defaultBranchResolution struct {
	Branch        string
	Source        string
	ProbeFailures []string
	UsedFallback  bool
}

func (r defaultBranchResolution) probeSummary() string {
	if len(r.ProbeFailures) == 0 {
		return "无"
	}
	return strings.Join(r.ProbeFailures, " | ")
}

// resolveDefaultBranch 统一默认分支解析入口（供 GitOps / Workspace 复用）。
func resolveDefaultBranch(workDir string) defaultBranchResolution {
	return resolveDefaultBranchWithExecutor(workDir, osGitOutputExecutor{}, defaultBranchLogf)
}

// resolveDefaultBranchWithExecutor 默认分支解析链路：
// 1) symbolic-ref refs/remotes/origin/HEAD
// 2) remote set-head origin -a 后重试 symbolic-ref
// 3) ls-remote --symref origin HEAD
// 4) rev-parse --verify refs/remotes/origin/main
// 5) remote show origin (HEAD branch)
// 6) fallback master
func resolveDefaultBranchWithExecutor(workDir string, executor gitOutputExecutor, logf func(string, ...any)) defaultBranchResolution {
	result := defaultBranchResolution{}

	// 1) 本地 origin/HEAD
	if out, err := executor.CombinedOutput(workDir, "symbolic-ref", "refs/remotes/origin/HEAD"); err == nil {
		if branch, ok := parseBranchFromRef(string(out), "refs/remotes/origin/"); ok {
			result.Branch = branch
			result.Source = "symbolic-ref"
			return result
		}
		appendProbeFailure(&result, "symbolic-ref", fmt.Errorf("输出无法解析为 refs/remotes/origin/<branch>"), out, logf)
	} else {
		appendProbeFailure(&result, "symbolic-ref", err, out, logf)
	}

	// 2) CI 浅克隆等场景常缺失 origin/HEAD，先尝试同步后再读一次。
	if out, err := executor.CombinedOutput(workDir, "remote", "set-head", "origin", "-a"); err == nil {
		if retryOut, retryErr := executor.CombinedOutput(workDir, "symbolic-ref", "refs/remotes/origin/HEAD"); retryErr == nil {
			if branch, ok := parseBranchFromRef(string(retryOut), "refs/remotes/origin/"); ok {
				result.Branch = branch
				result.Source = "symbolic-ref-after-set-head"
				return result
			}
			appendProbeFailure(&result, "symbolic-ref(after set-head)", fmt.Errorf("输出无法解析为 refs/remotes/origin/<branch>"), retryOut, logf)
		} else {
			appendProbeFailure(&result, "symbolic-ref(after set-head)", retryErr, retryOut, logf)
		}
	} else {
		appendProbeFailure(&result, "remote set-head origin -a", err, out, logf)
	}

	// 3) 远端 symref（标准输出）
	if out, err := executor.CombinedOutput(workDir, "ls-remote", "--symref", "origin", "HEAD"); err == nil {
		if branch, ok := parseBranchFromLSRemoteSymref(string(out)); ok {
			result.Branch = branch
			result.Source = "ls-remote-symref"
			return result
		}
		appendProbeFailure(&result, "ls-remote --symref", fmt.Errorf("输出未包含可解析的 refs/heads/<branch>"), out, logf)
	} else {
		appendProbeFailure(&result, "ls-remote --symref", err, out, logf)
	}

	// 4) 本地 origin/main 存在性探测
	if out, err := executor.CombinedOutput(workDir, "rev-parse", "--verify", "refs/remotes/origin/main"); err == nil {
		result.Branch = "main"
		result.Source = "local-origin-main"
		return result
	} else {
		appendProbeFailure(&result, "rev-parse origin/main", err, out, logf)
	}

	// 5) remote show 文本兜底
	if out, err := executor.CombinedOutput(workDir, "remote", "show", "origin"); err == nil {
		if branch, ok := parseBranchFromRemoteShow(string(out)); ok {
			result.Branch = branch
			result.Source = "remote-show"
			return result
		}
		appendProbeFailure(&result, "remote show origin", fmt.Errorf("输出未包含可解析的 HEAD branch"), out, logf)
	} else {
		appendProbeFailure(&result, "remote show origin", err, out, logf)
	}

	// 6) 最终兜底 master
	result.Branch = fallbackDefaultBranch
	result.Source = "fallback-master"
	result.UsedFallback = true
	if logf != nil {
		logf("warn: 默认分支探测全部失败，回退到 %q，失败详情: %s", fallbackDefaultBranch, result.probeSummary())
	}
	return result
}

func parseBranchFromLSRemoteSymref(raw string) (string, bool) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "ref:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "ref:" || fields[2] != "HEAD" {
			continue
		}
		return parseBranchFromRef(fields[1], "refs/heads/")
	}
	return "", false
}

func parseBranchFromRemoteShow(raw string) (string, bool) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "HEAD branch:") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		return normalizeBranch(parts[1])
	}
	return "", false
}

func parseBranchFromRef(ref, prefix string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if !strings.HasPrefix(ref, prefix) {
		return "", false
	}
	return normalizeBranch(strings.TrimPrefix(ref, prefix))
}

func normalizeBranch(raw string) (string, bool) {
	branch := strings.TrimSpace(raw)
	if branch == "" || strings.EqualFold(branch, "(unknown)") || strings.EqualFold(branch, "HEAD") {
		return "", false
	}
	if strings.HasPrefix(branch, "refs/heads/") {
		return parseBranchFromRef(branch, "refs/heads/")
	}
	if strings.HasPrefix(branch, "refs/remotes/origin/") {
		return parseBranchFromRef(branch, "refs/remotes/origin/")
	}
	if strings.ContainsAny(branch, " \t\r\n") {
		return "", false
	}
	return branch, true
}

func appendProbeFailure(result *defaultBranchResolution, step string, err error, output []byte, logf func(string, ...any)) {
	detail := fmt.Sprintf("%s: %v", step, err)
	outputText := oneLine(strings.TrimSpace(string(output)))
	if outputText != "" {
		detail = detail + " (output=" + outputText + ")"
	}
	result.ProbeFailures = append(result.ProbeFailures, detail)
	if logf != nil {
		logf("debug: 默认分支探测失败: %s", detail)
	}
}

func oneLine(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.ReplaceAll(raw, "\n", " | ")
}

func defaultBranchLogf(format string, args ...any) {
	fmt.Printf("[niuma][default-branch] "+format+"\n", args...)
}
