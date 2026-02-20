// pkg/agent/recovery.go
// 通用 AI 响应恢复骨架：格式修复重试 → fallback provider → 中止上报
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

// AbortError 三级降级全部失败后返回的最终错误。
// 调用方可通过 errors.As 提取 LastRaw 做软降级。
type AbortError struct {
	Err      error          // 最终错误
	LastRaw  string         // 最后一次 AI 原始响应
	Attempts []retryAttempt // 所有重试记录
}

func (e *AbortError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("所有 provider 均失败（共 %d 次尝试）: %v\n", len(e.Attempts), e.Err))
	for _, a := range e.Attempts {
		sb.WriteString(fmt.Sprintf("  - %s (attempt %d) [%s]: %s\n", a.Provider, a.Attempt, a.Kind, a.Error))
	}
	return sb.String()
}

func (e *AbortError) Unwrap() error {
	return e.Err
}

// RecoveryConfig 通用恢复配置（泛型）
type RecoveryConfig[T any] struct {
	Providers   []ai.Provider                                                      // provider 列表（按优先级）
	MaxRetries  int                                                                // 每个 provider 的最大重试次数（不含首次）
	CallAI      func(ctx context.Context, provider ai.Provider) (string, error)    // 调用 AI 并返回原始响应
	Parse       func(raw string) (T, error)                                        // 解析响应
	BuildRepair func(raw string) string                                            // 构造格式修复 prompt（nil 则跳过）
	RepairParse func(originalRaw, repairRaw string) (T, error)                     // 修复后解析（nil 则跳过）
	OnAbort     func(err error, lastRaw string)                                    // 中止回调（nil 则跳过）
}

// maxJitterMS 重试间隔的最大抖动毫秒数（测试可覆盖为 0）
var maxJitterMS = 500

// WithRecovery 通用的 AI 调用 + 解析 + 降级链路。
// 三级降级：Level 1 格式修复重试 → Level 2 fallback provider → Level 3 中止上报。
func WithRecovery[T any](ctx context.Context, cfg RecoveryConfig[T]) (T, error) {
	var zero T
	var attempts []retryAttempt
	var lastRaw string
	var lastErr error

	for _, p := range cfg.Providers {
		maxAttempts := cfg.MaxRetries + 1
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			// 调用 AI
			raw, aiErr := cfg.CallAI(ctx, p)
			if aiErr != nil {
				// provider 错误（网络/限流），不做格式修复，直接切下一个 provider
				attempts = append(attempts, retryAttempt{
					Provider: p.Name(),
					Attempt:  attempt,
					Error:    aiErr.Error(),
					Kind:     "ProviderError",
				})
				lastErr = aiErr
				break
			}

			// 解析响应
			result, parseErr := cfg.Parse(raw)
			if parseErr == nil {
				return result, nil
			}

			kind := "ParseError"
			var re *RecoverableError
			if errors.As(parseErr, &re) {
				kind = string(re.Kind)
			}
			attempts = append(attempts, retryAttempt{
				Provider: p.Name(),
				Attempt:  attempt,
				Error:    parseErr.Error(),
				Kind:     kind,
			})
			lastRaw = raw
			lastErr = parseErr

			// Level 1：格式修复重试（仅第一次 parse 失败时尝试）
			if attempt == 1 && cfg.BuildRepair != nil && cfg.RepairParse != nil {
				repairPrompt := cfg.BuildRepair(raw)
				repairRaw, repairErr := p.Complete(ctx, repairPrompt)
				if repairErr == nil {
					repaired, rErr := cfg.RepairParse(raw, repairRaw)
					if rErr == nil {
						return repaired, nil
					}
				}
				// 修复失败，继续常规重试
			}

			// 后续重试加随机 jitter
			if attempt < maxAttempts && maxJitterMS > 0 {
				jitter := time.Duration(rand.Intn(maxJitterMS)) * time.Millisecond
				select {
				case <-ctx.Done():
					return zero, ctx.Err()
				case <-time.After(jitter):
				}
			}
		}
	}

	// 全部失败
	if cfg.OnAbort != nil {
		cfg.OnAbort(lastErr, lastRaw)
	}
	return zero, &AbortError{Err: lastErr, LastRaw: lastRaw, Attempts: attempts}
}
