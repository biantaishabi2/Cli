// resolve-pr 子命令：从 issue 评论中分页解析 BOT:PR_CREATED marker，输出 PR 号
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	"github.com/spf13/cobra"
)

var resolvePRCmd = &cobra.Command{
	Use:   "resolve-pr",
	Short: "从 issue 评论中解析 BOT:PR_CREATED marker，输出 open 状态的 PR 号",
	Long: `分页读取 issue 全部评论，查找最新的 BOT:PR_CREATED marker，
校验对应 PR 为 open 状态后输出纯数字 PR 号到 stdout。
失败时 exit 1 并在 stderr 输出分类原因。`,
	RunE: runResolvePR,
}

func runResolvePR(cmd *cobra.Command, args []string) error {
	if flagRepo == "" || flagIssue == 0 {
		return fmt.Errorf("必须指定 --repo 和 --issue")
	}

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(flagRepo)
	if err != nil {
		return err
	}

	ops := &gitHubControlOps{client: client}
	prNum, _, err := ops.resolveOpenPR(ctx, flagIssue)
	if err != nil {
		// 分类错误输出到 stderr
		if errors.Is(err, control.ErrPRMarkerNotFound) {
			fmt.Fprintf(os.Stderr, "pr marker not found on issue #%d\n", flagIssue)
		} else if errors.Is(err, control.ErrPRClosed) {
			fmt.Fprintf(os.Stderr, "pr is closed (issue #%d)\n", flagIssue)
		} else {
			fmt.Fprintf(os.Stderr, "resolve-pr failed: %v\n", err)
		}
		return withExitCode(1, err)
	}

	// 成功：stdout 输出纯数字 PR 号
	fmt.Println(prNum)
	return nil
}
