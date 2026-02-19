// pkg/agent/plan.go
// PlanEngine：方案草案 → 最终方案
package agent

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

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
// 可恢复错误（RecoverableError）触发同 provider 重试，其他错误直接切下一个 provider。
func (e *PlanEngine) FinalWithRetry(ctx context.Context, input *PromptInput, providers []ai.Provider, maxRetries int) (*FinalPlan, error) {
	prompt, err := BuildFinalPlanPrompt(input)
	if err != nil {
		return nil, fmt.Errorf("构建 final plan prompt 失败: %w", err)
	}

	var attempts []retryAttempt

	for _, p := range providers {
		maxAttempts := maxRetries + 1
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			raw, completeErr := p.Complete(ctx, prompt)
			if completeErr != nil {
				// provider.Complete 错误（网络/限流等），不做同 provider 重试
				attempts = append(attempts, retryAttempt{
					Provider: p.Name(),
					Attempt:  attempt,
					Error:    completeErr.Error(),
					Kind:     "ProviderError",
				})
				break // 切下一个 provider
			}

			plan, parseErr := ParseFinalPlanResponse(raw)
			if parseErr == nil {
				return plan, nil
			}

			var re *RecoverableError
			if errors.As(parseErr, &re) {
				attempts = append(attempts, retryAttempt{
					Provider: p.Name(),
					Attempt:  attempt,
					Error:    parseErr.Error(),
					Kind:     string(re.Kind),
				})
				if attempt < maxAttempts {
					// 格式错误：短随机抖动后重试
					jitter := time.Duration(rand.Intn(500)) * time.Millisecond
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(jitter):
					}
				}
				continue
			}

			// 不可恢复的解析错误（字段校验失败等），不重试也不降级
			return nil, parseErr
		}
	}

	// 全部 provider 耗尽
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("所有 provider 均失败（共 %d 次尝试）:\n", len(attempts)))
	for _, a := range attempts {
		sb.WriteString(fmt.Sprintf("  - %s (attempt %d) [%s]: %s\n", a.Provider, a.Attempt, a.Kind, a.Error))
	}
	return nil, &AggregateRetryError{Message: sb.String(), Attempts: attempts}
}

// AggregateRetryError 聚合所有重试尝试的错误
type AggregateRetryError struct {
	Message  string
	Attempts []retryAttempt
}

func (e *AggregateRetryError) Error() string {
	return e.Message
}
