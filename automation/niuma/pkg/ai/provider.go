// pkg/ai/provider.go
// AI Provider 接口定义和 CLI 桩实现
package ai

import (
	"context"
	"fmt"
)

// Provider AI 后端抽象接口
type Provider interface {
	// Complete 向 AI 发送 prompt，返回补全结果
	Complete(ctx context.Context, prompt string, opts ...Option) (string, error)
	// Name 返回 provider 名称
	Name() string
}

// Option 用于配置 Complete 调用的选项
type Option func(*options)

type options struct {
	WorkDir     string
	MaxTokens   int
	Temperature float64
}

// WithWorkDir 设置工作目录
func WithWorkDir(dir string) Option {
	return func(o *options) {
		o.WorkDir = dir
	}
}

// WithMaxTokens 设置最大 token 数
func WithMaxTokens(n int) Option {
	return func(o *options) {
		o.MaxTokens = n
	}
}

// WithTemperature 设置温度参数
func WithTemperature(t float64) Option {
	return func(o *options) {
		o.Temperature = t
	}
}

func buildOptions(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// CLIProvider 通过 CLI 命令调用 AI 工具的桩实现
// Phase 2 实现实际调用逻辑
type CLIProvider struct {
	ProviderName string
	Cmd          string // CLI 命令模板
}

func (p *CLIProvider) Name() string {
	return p.ProviderName
}

func (p *CLIProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	return "", fmt.Errorf("AI provider %q not implemented: Phase 2 will add CLI execution support", p.ProviderName)
}
