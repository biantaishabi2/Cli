package control

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const fallbackMergeBaseBranch = "master"

// MergeBaseResult 表示 merge base 分支解析结果。
type MergeBaseResult struct {
	Branch  string
	Source  string
	Warning string
}

// MergeBaseDefaultBranchProvider 提供仓库默认分支读取能力。
type MergeBaseDefaultBranchProvider interface {
	GetDefaultBranch(ctx context.Context) (string, error)
}

// MergeBaseGitExecutor 抽象 git 命令执行，便于单测注入。
type MergeBaseGitExecutor interface {
	CombinedOutput(workDir string, args ...string) ([]byte, error)
}

type osMergeBaseGitExecutor struct{}

func (osMergeBaseGitExecutor) CombinedOutput(workDir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	return cmd.CombinedOutput()
}

// MergeBaseResolver 统一 control merge/close-merged 的 base 分支解析逻辑。
type MergeBaseResolver struct {
	repoDir               string
	defaultBranchProvider MergeBaseDefaultBranchProvider
	gitExecutor           MergeBaseGitExecutor
}

// NewMergeBaseResolver 创建解析器。
func NewMergeBaseResolver(repoDir string, defaultBranchProvider MergeBaseDefaultBranchProvider) *MergeBaseResolver {
	return NewMergeBaseResolverWithExecutor(repoDir, defaultBranchProvider, osMergeBaseGitExecutor{})
}

// NewMergeBaseResolverWithExecutor 创建带执行器注入的解析器（测试用）。
func NewMergeBaseResolverWithExecutor(
	repoDir string,
	defaultBranchProvider MergeBaseDefaultBranchProvider,
	gitExecutor MergeBaseGitExecutor,
) *MergeBaseResolver {
	repoDir = strings.TrimSpace(repoDir)
	if repoDir == "" {
		repoDir = "."
	}
	if gitExecutor == nil {
		gitExecutor = osMergeBaseGitExecutor{}
	}
	return &MergeBaseResolver{
		repoDir:               repoDir,
		defaultBranchProvider: defaultBranchProvider,
		gitExecutor:           gitExecutor,
	}
}

// Resolve 按固定优先级解析 merge base：
// 1) CLI --merge-base-branch
// 2) config control.merge_base_branch
// 3) GitHub API 默认分支
// 4) git remote show origin / refs/remotes/origin/HEAD
// 5) fallback master
func (r *MergeBaseResolver) Resolve(ctx context.Context, cliBranch, configBranch string) MergeBaseResult {
	warnings := make([]string, 0, 4)

	if strings.TrimSpace(cliBranch) != "" {
		if branch, ok := normalizeMergeBaseBranch(cliBranch); ok {
			return MergeBaseResult{
				Branch:  branch,
				Source:  "cli-flag",
				Warning: "",
			}
		}
		warnings = append(warnings, fmt.Sprintf("忽略非法 CLI merge-base 分支: %q", cliBranch))
	}

	if strings.TrimSpace(configBranch) != "" {
		if branch, ok := normalizeMergeBaseBranch(configBranch); ok {
			return MergeBaseResult{
				Branch:  branch,
				Source:  "config",
				Warning: strings.Join(warnings, " | "),
			}
		}
		warnings = append(warnings, fmt.Sprintf("忽略非法配置 merge-base 分支: %q", configBranch))
	}

	probeFailures := make([]string, 0, 4)

	if r.defaultBranchProvider == nil {
		probeFailures = append(probeFailures, "github-default-branch: provider 未配置")
	} else {
		branch, err := r.defaultBranchProvider.GetDefaultBranch(ctx)
		if err != nil {
			probeFailures = append(probeFailures, fmt.Sprintf("github-default-branch: %v", err))
		} else if normalized, ok := normalizeMergeBaseBranch(branch); ok {
			return MergeBaseResult{
				Branch:  normalized,
				Source:  "github-default-branch",
				Warning: strings.Join(warnings, " | "),
			}
		} else {
			probeFailures = append(probeFailures, fmt.Sprintf("github-default-branch: 返回非法分支 %q", branch))
		}
	}

	if out, err := r.gitExecutor.CombinedOutput(r.repoDir, "remote", "show", "origin"); err != nil {
		probeFailures = append(probeFailures, fmt.Sprintf("git-remote-show-origin: %v (%s)", err, trimOutputForLog(out)))
	} else if branch, ok := parseMergeBaseBranchFromRemoteShow(string(out)); ok {
		return MergeBaseResult{
			Branch:  branch,
			Source:  "git-remote-show-origin",
			Warning: strings.Join(warnings, " | "),
		}
	} else {
		probeFailures = append(probeFailures, "git-remote-show-origin: 未解析出 HEAD branch")
	}

	if out, err := r.gitExecutor.CombinedOutput(r.repoDir, "symbolic-ref", "refs/remotes/origin/HEAD"); err != nil {
		probeFailures = append(probeFailures, fmt.Sprintf("git-origin-head: %v (%s)", err, trimOutputForLog(out)))
	} else if branch, ok := parseMergeBaseBranchFromOriginHead(string(out)); ok {
		return MergeBaseResult{
			Branch:  branch,
			Source:  "git-origin-head",
			Warning: strings.Join(warnings, " | "),
		}
	} else {
		probeFailures = append(probeFailures, "git-origin-head: 未解析出 refs/remotes/origin/<branch>")
	}

	warning := append([]string(nil), warnings...)
	warning = append(warning,
		fmt.Sprintf("默认分支自动探测失败，fallback=%s，失败详情=%s", fallbackMergeBaseBranch, strings.Join(probeFailures, " | ")),
	)
	return MergeBaseResult{
		Branch:  fallbackMergeBaseBranch,
		Source:  "fallback-master",
		Warning: strings.Join(warning, " | "),
	}
}

func normalizeMergeBaseBranch(raw string) (string, bool) {
	branch := strings.TrimSpace(raw)
	if branch == "" {
		return "", false
	}
	if strings.ContainsAny(branch, " \t\r\n") {
		return "", false
	}
	lowered := strings.ToLower(branch)
	switch lowered {
	case "(unknown)", "unknown", "head":
		return "", false
	}
	return branch, true
}

func parseMergeBaseBranchFromRemoteShow(raw string) (string, bool) {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "HEAD branch:") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		return normalizeMergeBaseBranch(parts[1])
	}
	return "", false
}

func parseMergeBaseBranchFromOriginHead(raw string) (string, bool) {
	ref := strings.TrimSpace(raw)
	const prefix = "refs/remotes/origin/"
	if !strings.HasPrefix(ref, prefix) {
		return "", false
	}
	return normalizeMergeBaseBranch(strings.TrimPrefix(ref, prefix))
}

func trimOutputForLog(out []byte) string {
	text := strings.TrimSpace(string(out))
	if text == "" {
		return "empty-output"
	}
	if len(text) > 160 {
		return text[:160] + "..."
	}
	return text
}
