// pkg/daemon/executor.go
// 派发器：调用 AI provider 处理 issue + gate 验证
package daemon

import (
	"context"
	"fmt"
	"log"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/agent"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
)

// Executor 任务执行接口
type Executor interface {
	Fix(ctx context.Context, issue Issue) error
}

// GateRunner gate 脚本执行接口（可选）
type GateRunner interface {
	// RunGate 执行本地测试 gate，返回 (输出, 错误)
	RunGate(ctx context.Context, workDir string) (string, error)
}

// AIExecutor 直接调用 AI provider 执行
type AIExecutor struct {
	Provider ai.Provider // AI provider（claude/codex）
	WorkDir  string      // 工作目录
	Gate     GateRunner  // gate 验证器（nil = 跳过）
}

// Fix 组装 prompt 并调用 AI provider 处理 issue
func (e *AIExecutor) Fix(ctx context.Context, issue Issue) error {
	prompt, err := buildPrompt(issue)
	if err != nil {
		return fmt.Errorf("构建 prompt 失败: %w", err)
	}

	log.Printf("[executor] 调用 %s 处理 #%d", e.Provider.Name(), issue.Number)

	_, err = e.Provider.Execute(ctx, prompt, ai.WithWorkDir(e.WorkDir))
	if err != nil {
		return fmt.Errorf("AI 执行失败: %w", err)
	}

	// gate 后置验证
	if e.Gate != nil {
		log.Printf("[executor] 运行 gate 验证 #%d", issue.Number)
		output, gateErr := e.Gate.RunGate(ctx, e.WorkDir)
		if gateErr != nil {
			return &GateError{Output: output, Err: gateErr}
		}
	}

	return nil
}

// GateError gate 验证失败（区分于 AI 执行失败）
type GateError struct {
	Output string // gate 脚本输出（stderr）
	Err    error
}

func (e *GateError) Error() string {
	return fmt.Sprintf("gate 验证失败: %v", e.Err)
}

func (e *GateError) Unwrap() error { return e.Err }

// gitPRSuffix daemon 模式下追加的 git/PR 指令（原 orchestrator 负责的步骤）
const gitPRSuffix = `

## 提交与 PR（必须完成）

代码改完并验证通过后，你必须执行以下步骤：

0. 先同步最新代码：git checkout main && git pull origin main
1. 创建新分支：git checkout -b fix/%d-auto
2. 暂存所有改动：git add -A
3. 提交：git commit -m "fix: %s (#%d)"
4. 推送：git push origin fix/%d-auto
5. 创建 PR：gh pr create --title "fix: %s (#%d)" --body "Closes #%d"

如果分支已存在，先删除再创建（git branch -D fix/%d-auto）。
如果 push 失败，检查远程配置。
必须完成以上所有步骤，否则视为失败。
`

// buildPrompt 用 agent.BuildImplementPrompt 组装完整 prompt + git/PR 指令
func buildPrompt(issue Issue) (string, error) {
	input := &agent.PromptInput{
		IssueTitle: fmt.Sprintf("#%d %s", issue.Number, issue.Title),
		IssueBody:  issue.Body,
	}
	prompt, err := agent.BuildImplementPrompt(input)
	if err != nil {
		log.Printf("[executor] BuildImplementPrompt 失败，使用简单模板: %v", err)
		prompt = fmt.Sprintf(`你需要处理以下 GitHub Issue，请阅读需求并实现代码，然后创建 PR。

## Issue #%d: %s

仓库: %s

%s
`, issue.Number, issue.Title, issue.Repo, issue.Body)
	}

	// 追加 git/PR 指令
	suffix := fmt.Sprintf(gitPRSuffix,
		issue.Number, issue.Title, issue.Number,
		issue.Number, issue.Title, issue.Number, issue.Number,
		issue.Number)
	return prompt + suffix, nil
}
