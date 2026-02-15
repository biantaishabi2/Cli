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
	Cmd          string // CLI 命令模板
	MaxRetries   int    // 重试次数：0=默认3次，>0=自定义，<0=禁用重试
}

func (p *CLIProvider) Name() string {
	return p.ProviderName
}

// Complete 执行 CLI 命令，将 prompt 写入临时文件，捕获 stdout 作为响应
func (p *CLIProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	o := buildOptions(opts)

	maxRetries := p.MaxRetries
	if maxRetries == 0 {
		maxRetries = defaultMaxRetries // 默认 3 次
	} else if maxRetries < 0 {
		maxRetries = 0 // 禁用重试
	}

	var result string
	err := WithRetry(ctx, maxRetries, func() error {
		out, execErr := p.execute(ctx, prompt, o)
		if execErr != nil {
			return execErr
		}
		result = out
		return nil
	})

	return result, err
}

// execute 单次执行 CLI 命令
func (p *CLIProvider) execute(ctx context.Context, prompt string, o *options) (string, error) {
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

	// 模板替换
	cmdStr := p.Cmd
	cmdStr = strings.ReplaceAll(cmdStr, "{prompt_file}", tmpFile.Name())
	cmdStr = strings.ReplaceAll(cmdStr, "{prompt}", tmpFile.Name()) // 兼容旧模板
	if o.WorkDir != "" {
		cmdStr = strings.ReplaceAll(cmdStr, "{workdir}", o.WorkDir)
	}

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
