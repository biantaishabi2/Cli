// pkg/agent/plan.go
// PlanEngine：方案草案 → 讨论汇总 → 最终方案
package agent

import (
	"context"
	"fmt"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
)

// PlanEngine 方案生成引擎
type PlanEngine struct {
	provider ai.Provider
}

// NewPlanEngine 创建 PlanEngine
func NewPlanEngine(provider ai.Provider) *PlanEngine {
	return &PlanEngine{provider: provider}
}

// Draft 生成方案草案
func (e *PlanEngine) Draft(ctx context.Context, input *PromptInput) (*DraftPlan, error) {
	prompt, err := BuildDraftPrompt(input)
	if err != nil {
		return nil, fmt.Errorf("构建 draft prompt 失败: %w", err)
	}

	raw, err := e.provider.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI 生成草案失败: %w", err)
	}

	return ParseDraftResponse(raw)
}

// Consolidate 生成讨论汇总
func (e *PlanEngine) Consolidate(ctx context.Context, input *PromptInput) (*DiscussionSummary, error) {
	prompt, err := BuildConsolidatePrompt(input)
	if err != nil {
		return nil, fmt.Errorf("构建 consolidate prompt 失败: %w", err)
	}

	raw, err := e.provider.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI 汇总讨论失败: %w", err)
	}

	return ParseConsolidateResponse(raw)
}

// Final 生成最终方案
func (e *PlanEngine) Final(ctx context.Context, input *PromptInput) (*FinalPlan, error) {
	prompt, err := BuildFinalPlanPrompt(input)
	if err != nil {
		return nil, fmt.Errorf("构建 final plan prompt 失败: %w", err)
	}

	raw, err := e.provider.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI 生成最终方案失败: %w", err)
	}

	return ParseFinalPlanResponse(raw)
}
