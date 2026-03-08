// pkg/daemon/executor.go
// 派发器：直接调用 AI provider 处理 issue
package daemon

import (
	"context"
	"fmt"
	"log"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
)

// Executor 任务执行接口
type Executor interface {
	Fix(ctx context.Context, issue Issue) error
}

// AIExecutor 直接调用 AI provider 执行
type AIExecutor struct {
	Provider ai.Provider // AI provider（claude/codex）
	WorkDir  string      // 工作目录
}

// Fix 组装 prompt 并调用 AI provider 处理 issue
func (e *AIExecutor) Fix(ctx context.Context, issue Issue) error {
	prompt := buildPrompt(issue)

	log.Printf("[executor] 调用 %s 处理 #%d", e.Provider.Name(), issue.Number)

	_, err := e.Provider.Execute(ctx, prompt, ai.WithWorkDir(e.WorkDir))
	if err != nil {
		return fmt.Errorf("AI 执行失败: %w", err)
	}
	return nil
}

// buildPrompt 从 issue 信息组装 prompt
func buildPrompt(issue Issue) string {
	return fmt.Sprintf(`你需要处理以下 GitHub Issue，请阅读需求并实现代码，然后创建 PR。

## Issue #%d: %s

仓库: %s

%s

## 要求

1. 阅读 issue 描述，理解需求
2. 实现代码修改
3. 编写或更新相关测试
4. 提交代码并创建 PR（标题包含 #%d，body 包含 Closes #%d）
`, issue.Number, issue.Title, issue.Repo, issue.Body, issue.Number, issue.Number)
}
