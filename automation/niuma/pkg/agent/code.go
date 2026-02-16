// pkg/agent/code.go
// CodeGen：代码实现与迭代修复
package agent

import (
	"context"
	"fmt"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
)

// CodeGen 代码生成引擎
type CodeGen struct {
	provider ai.Provider
}

// NewCodeGen 创建 CodeGen
func NewCodeGen(provider ai.Provider) *CodeGen {
	return &CodeGen{provider: provider}
}

// Implement 根据最终方案生成代码
func (g *CodeGen) Implement(ctx context.Context, input *PromptInput, workDir string) (*ImplementResult, error) {
	prompt, err := BuildImplementPrompt(input)
	if err != nil {
		return nil, fmt.Errorf("构建 implement prompt 失败: %w", err)
	}

	opts := []ai.Option{}
	if workDir != "" {
		opts = append(opts, ai.WithWorkDir(workDir))
	}

	raw, err := g.provider.Complete(ctx, prompt, opts...)
	if err != nil {
		return nil, fmt.Errorf("AI 代码实现失败: %w", err)
	}

	return &ImplementResult{RawOutput: raw}, nil
}

// Iterate 根据 review 意见修改代码
func (g *CodeGen) Iterate(ctx context.Context, input *PromptInput, workDir string) (*IterateResult, error) {
	prompt, err := BuildIteratePrompt(input)
	if err != nil {
		return nil, fmt.Errorf("构建 iterate prompt 失败: %w", err)
	}

	opts := []ai.Option{}
	if workDir != "" {
		opts = append(opts, ai.WithWorkDir(workDir))
	}

	raw, err := g.provider.Complete(ctx, prompt, opts...)
	if err != nil {
		return nil, fmt.Errorf("AI 迭代修复失败: %w", err)
	}

	return &IterateResult{RawOutput: raw}, nil
}
