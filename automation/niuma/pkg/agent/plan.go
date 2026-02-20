// pkg/agent/plan.go
// PlanEngine：方案草案 → 最终方案
package agent

import (
	"context"
	"errors"
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

// retryAttempt 记录单次重试尝试
type retryAttempt struct {
	Provider string
	Attempt  int
	Error    string
	Kind     string // 错误分类（RecoverableError.Kind 或 "ProviderError"）
}

// FinalWithRetry 带受控重试和 provider 降级的最终方案生成。
// providers 列表按优先级排列，对每个 provider 最多尝试 maxRetries+1 次。
// 内部使用 WithRecovery 统一降级链路，新增格式修复重试能力。
func (e *PlanEngine) FinalWithRetry(ctx context.Context, input *PromptInput, providers []ai.Provider, maxRetries int) (*FinalPlan, error) {
	prompt, err := BuildFinalPlanPrompt(input)
	if err != nil {
		return nil, fmt.Errorf("构建 final plan prompt 失败: %w", err)
	}

	cfg := RecoveryConfig[*FinalPlan]{
		Providers:  providers,
		MaxRetries: maxRetries,
		CallAI: func(ctx context.Context, provider ai.Provider) (string, error) {
			return provider.Complete(ctx, prompt)
		},
		Parse:       ParseFinalPlanResponse,
		BuildRepair: func(raw string) string { return BuildFormatRepairPromptFor(raw, planFinalFields) },
		RepairParse: RepairAndParseFinalPlan,
		OnAbort:     nil, // FinalWithRetry 自己处理软降级
	}

	plan, err := WithRecovery(ctx, cfg)
	if err != nil {
		// 从 AbortError 提取 LastRaw 做软降级（保留原有行为）
		var abortErr *AbortError
		if errors.As(err, &abortErr) && abortErr.LastRaw != "" {
			return &FinalPlan{Approach: abortErr.LastRaw}, nil
		}
		// 连一次有效响应都没拿到
		if errors.As(err, &abortErr) {
			return nil, &AggregateRetryError{Message: abortErr.Error(), Attempts: abortErr.Attempts}
		}
		return nil, err
	}
	return plan, nil
}

// AggregateRetryError 聚合所有重试尝试的错误
type AggregateRetryError struct {
	Message  string
	Attempts []retryAttempt
}

func (e *AggregateRetryError) Error() string {
	return e.Message
}
