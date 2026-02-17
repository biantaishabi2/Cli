// pkg/ai/cli_provider.go
// CLIProvider 通过 CLI 命令调用 AI 工具
package ai

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const defaultMaxRetries = 3

// CLIProvider 通过 CLI 命令模板调用外部 AI 工具
// 模板变量：{prompt_file} → 临时文件路径，{workdir} → 工作目录
type CLIProvider struct {
	ProviderName string
	Cmd          string // 文本模式命令模板（Complete 使用）
	CmdAgent     string // agentic 模式命令模板（Execute 使用）
	MaxRetries   int    // 重试次数：0=默认3次，>0=自定义，<0=禁用重试
}

func (p *CLIProvider) Name() string {
	return p.ProviderName
}

// Complete 文本模式：使用 Cmd 模板执行，AI 只返回文本
func (p *CLIProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	if p.Cmd == "" {
		return "", fmt.Errorf("CLIProvider %q 未配置 Cmd（文本模式命令模板）", p.ProviderName)
	}

	o := buildOptions(opts)

	maxRetries := p.MaxRetries
	if maxRetries == 0 {
		maxRetries = defaultMaxRetries // 默认 3 次
	} else if maxRetries < 0 {
		maxRetries = 0 // 禁用重试
	}

	var result string
	err := WithRetry(ctx, maxRetries, func() error {
		out, execErr := p.executeWithCmd(ctx, prompt, o, p.Cmd)
		if execErr != nil {
			return execErr
		}
		result = out
		return nil
	})

	return result, err
}

// Execute agentic 模式：使用 CmdAgent 模板执行，AI 可读写文件
// 如果没有配置 CmdAgent，回退到 Cmd
func (p *CLIProvider) Execute(ctx context.Context, prompt string, opts ...Option) (string, error) {
	o := buildOptions(opts)

	cmdTemplate := p.CmdAgent
	if cmdTemplate == "" {
		cmdTemplate = p.Cmd // 回退到文本模式
	}
	if cmdTemplate == "" {
		return "", fmt.Errorf("CLIProvider %q 未配置 CmdAgent 或 Cmd", p.ProviderName)
	}

	maxRetries := p.MaxRetries
	if maxRetries == 0 {
		maxRetries = defaultMaxRetries
	} else if maxRetries < 0 {
		maxRetries = 0
	}

	var result string
	err := WithRetry(ctx, maxRetries, func() error {
		out, execErr := p.executeWithCmd(ctx, prompt, o, cmdTemplate)
		if execErr != nil {
			return execErr
		}
		result = out
		return nil
	})

	return result, err
}

// executeWithCmd 单次执行 CLI 命令，使用指定的命令模板
func (p *CLIProvider) executeWithCmd(ctx context.Context, prompt string, o *options, cmdTemplate string) (string, error) {
	// 将 prompt 写入临时文件
	tmpFile, err := os.CreateTemp("", "niuma-prompt-*.txt")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(prompt); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("写入临时文件失败: %w", err)
	}
	tmpFile.Close()

	// 模板替换（对路径进行 shell 转义，防止路径含空格导致命令注入）
	// {prompt_file} → prompt 内容写入的临时文件路径（推荐）
	// {prompt} → 同 {prompt_file}（旧模板兼容，注意：是文件路径而非 prompt 文本）
	// {workdir} → 工作目录路径
	cmdStr := cmdTemplate
	escaped := shellQuote(tmpFile.Name())
	cmdStr = strings.ReplaceAll(cmdStr, "{prompt_file}", escaped)
	cmdStr = strings.ReplaceAll(cmdStr, "{prompt}", escaped)
	// 对于未显式传入 workdir 的场景（如 review/plan），回退到当前目录，
	// 避免命令模板中的 {workdir} 残留为字面量导致执行失败。
	workDir := o.WorkDir
	if workDir == "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr == nil {
			workDir = cwd
		} else {
			workDir = "."
		}
	}
	cmdStr = strings.ReplaceAll(cmdStr, "{workdir}", shellQuote(workDir))

	// 执行命令
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	if o.WorkDir != "" {
		cmd.Dir = o.WorkDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr != "" {
			return "", fmt.Errorf("命令执行失败: %w (stderr: %s)", err, stderrStr)
		}
		return "", fmt.Errorf("命令执行失败: %w", err)
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "", fmt.Errorf("命令输出为空")
	}

	return result, nil
}

// shellQuote 对路径进行 shell 安全转义（用单引号包裹，转义内部单引号）
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
