// pkg/daemon/executor.go
// 派发器：调用 niuma fix 子进程执行实际工作
package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// Executor 任务执行接口
type Executor interface {
	// Fix 派发 niuma fix 处理指定 issue
	Fix(ctx context.Context, issue Issue) error
}

// CLIExecutor 通过 CLI 子进程执行 niuma fix
type CLIExecutor struct {
	NiumaBin string // niuma 二进制路径（默认使用自身）
	RepoDir  string // 仓库目录
}

// Fix 调用 niuma fix --repo <repo> --issue <N> --workdir <dir>
func (e *CLIExecutor) Fix(ctx context.Context, issue Issue) error {
	bin := e.NiumaBin
	if bin == "" {
		var err error
		bin, err = os.Executable()
		if err != nil {
			return fmt.Errorf("无法获取 niuma 二进制路径: %w", err)
		}
	}

	args := []string{
		"fix",
		"--repo", issue.Repo,
		"--issue", strconv.Itoa(issue.Number),
	}
	if e.RepoDir != "" {
		args = append(args, "--repo-dir", e.RepoDir, "--workdir", e.RepoDir)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// 继承环境变量（GITHUB_TOKEN 等）
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("niuma fix #%d 执行失败: %w", issue.Number, err)
	}
	return nil
}
