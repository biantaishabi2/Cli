//go:build !ci

package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// 测试中禁用随机 jitter，避免延迟和非确定性
	maxJitterMS = 0
}

// ===== WithRecovery 单元测试 =====

func TestWithRecovery_ParseSuccess(t *testing.T) {
	// 首次 parse 即成功，无需任何降级
	mock := ai.NewMockProvider(`{"title":"方案","approach":"实现"}`)
	abortCalled := false

	cfg := RecoveryConfig[*FinalPlan]{
		Providers:  []ai.Provider{mock},
		MaxRetries: 1,
		CallAI: func(ctx context.Context, p ai.Provider) (string, error) {
			return p.Complete(ctx, "test prompt")
		},
		Parse:       ParseFinalPlanResponse,
		BuildRepair: func(raw string) string { return "repair" },
		RepairParse: RepairAndParseFinalPlan,
		OnAbort:     func(err error, lastRaw string) { abortCalled = true },
	}

	result, err := WithRecovery(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "方案", result.Title)
	assert.Equal(t, "实现", result.Approach)
	assert.False(t, abortCalled, "OnAbort 不应被调用")
	assert.Equal(t, 1, mock.CallCount(), "仅调用 1 次 CallAI")
}

func TestWithRecovery_Level1_RepairSuccess(t *testing.T) {
	// 首次 parse 失败，格式修复重试后 RepairParse 成功
	mock := ai.NewMockProvider(
		"这是纯文本方案", // 第1次：parse 失败
		`{"title":"修复后方案","approach":"格式修复成功"}`, // 第2次：repair prompt 的回复
	)
	abortCalled := false

	cfg := RecoveryConfig[*FinalPlan]{
		Providers:  []ai.Provider{mock},
		MaxRetries: 1,
		CallAI: func(ctx context.Context, p ai.Provider) (string, error) {
			return p.Complete(ctx, "test prompt")
		},
		Parse:       ParseFinalPlanResponse,
		BuildRepair: func(raw string) string { return "请修复格式" },
		RepairParse: RepairAndParseFinalPlan,
		OnAbort:     func(err error, lastRaw string) { abortCalled = true },
	}

	result, err := WithRecovery(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "修复后方案", result.Title)
	assert.Equal(t, "格式修复成功", result.Approach)
	assert.False(t, abortCalled)
	assert.Equal(t, 2, mock.CallCount(), "CallAI 1次 + repair Complete 1次")
}

func TestWithRecovery_Level2_FallbackSuccess(t *testing.T) {
	// 主 provider 格式修复失败，fallback provider parse 成功
	primary := ai.NewMockProvider(
		"not json",       // 第1次：parse 失败
		"repair also bad", // 第2次：repair 也失败
		"retry also bad",  // 第3次：重试也失败（如果有 maxRetries）
	)
	fallback := ai.NewMockProvider(
		`{"title":"fallback方案","approach":"备选provider成功"}`,
	)
	abortCalled := false

	cfg := RecoveryConfig[*FinalPlan]{
		Providers:  []ai.Provider{primary, fallback},
		MaxRetries: 0,
		CallAI: func(ctx context.Context, p ai.Provider) (string, error) {
			return p.Complete(ctx, "test prompt")
		},
		Parse:       ParseFinalPlanResponse,
		BuildRepair: func(raw string) string { return "请修复" },
		RepairParse: RepairAndParseFinalPlan,
		OnAbort:     func(err error, lastRaw string) { abortCalled = true },
	}

	result, err := WithRecovery(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "fallback方案", result.Title)
	assert.False(t, abortCalled)
	assert.Equal(t, 1, fallback.CallCount(), "fallback provider 被使用")
}

func TestWithRecovery_Level3_AllFail_AbortCalled(t *testing.T) {
	// 所有 provider 和重试全部失败
	p1 := ai.NewMockProvider("not json", "repair fail")
	p2 := ai.NewMockProvider("also not json", "repair fail too")

	var abortErr error
	var abortLastRaw string

	cfg := RecoveryConfig[*FinalPlan]{
		Providers:  []ai.Provider{p1, p2},
		MaxRetries: 0,
		CallAI: func(ctx context.Context, p ai.Provider) (string, error) {
			return p.Complete(ctx, "test prompt")
		},
		Parse:       ParseFinalPlanResponse,
		BuildRepair: func(raw string) string { return "请修复" },
		RepairParse: RepairAndParseFinalPlan,
		OnAbort: func(err error, lastRaw string) {
			abortErr = err
			abortLastRaw = lastRaw
		},
	}

	_, err := WithRecovery(context.Background(), cfg)
	require.Error(t, err)

	var ae *AbortError
	require.True(t, errors.As(err, &ae))
	assert.NotEmpty(t, ae.LastRaw, "AbortError.LastRaw 应包含最后一次原始响应")
	assert.NotEmpty(t, ae.Attempts, "AbortError.Attempts 应记录所有尝试")
	assert.NotNil(t, abortErr, "OnAbort 应被调用")
	assert.Equal(t, ae.LastRaw, abortLastRaw)
}

func TestWithRecovery_ProviderError_SkipsToNext(t *testing.T) {
	// CallAI 返回 provider 错误，不做格式修复，直接切换 provider
	primary := ai.NewMockProviderWithError(fmt.Errorf("rate limit"))
	fallback := ai.NewMockProvider(`{"title":"OK","approach":"fallback成功"}`)

	repairCalled := false

	cfg := RecoveryConfig[*FinalPlan]{
		Providers:  []ai.Provider{primary, fallback},
		MaxRetries: 1,
		CallAI: func(ctx context.Context, p ai.Provider) (string, error) {
			return p.Complete(ctx, "test prompt")
		},
		Parse: ParseFinalPlanResponse,
		BuildRepair: func(raw string) string {
			repairCalled = true
			return "repair"
		},
		RepairParse: RepairAndParseFinalPlan,
	}

	result, err := WithRecovery(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "OK", result.Title)
	assert.False(t, repairCalled, "provider 错误不应触发格式修复")
}

func TestWithRecovery_NilBuildRepair_SkipsRepair(t *testing.T) {
	// BuildRepair 为 nil 时跳过格式修复，直接重试
	mock := ai.NewMockProvider(
		"not json",
		`{"title":"重试成功","approach":"第二次OK"}`,
	)

	cfg := RecoveryConfig[*FinalPlan]{
		Providers:   []ai.Provider{mock},
		MaxRetries:  1,
		CallAI:      func(ctx context.Context, p ai.Provider) (string, error) { return p.Complete(ctx, "p") },
		Parse:       ParseFinalPlanResponse,
		BuildRepair: nil,
		RepairParse: nil,
	}

	result, err := WithRecovery(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, "重试成功", result.Title)
	assert.Equal(t, 2, mock.CallCount())
}

// ===== FinalWithRetry 集成测试（格式修复恢复） =====

func TestFinalWithRetry_FormatRepairRecovery(t *testing.T) {
	// mock provider 第一次返回非 JSON 文本，格式修复 prompt 后返回正确 JSON
	mock := ai.NewMockProvider(
		"我建议使用方案A来解决这个问题...", // 第1次：纯文本
		`{"title":"方案A","approach":"使用方案A解决","file_changes":[],"test_scenarios":[]}`, // 第2次：repair 成功
	)
	engine := NewPlanEngine(mock)

	input := &PromptInput{
		IssueTitle: "Test issue",
		IssueBody:  "Test body",
	}

	plan, err := engine.FinalWithRetry(context.Background(), input, []ai.Provider{mock}, 1)
	require.NoError(t, err)
	assert.Equal(t, "方案A", plan.Title)
	assert.Equal(t, "使用方案A解决", plan.Approach)
	assert.Equal(t, 2, mock.CallCount()) // 原始调用 + repair
}

func TestFinalWithRetry_SoftDegrade_Preserved(t *testing.T) {
	// 所有 provider 和修复全部失败，验证 lastRaw 软降级行为保持不变
	p1 := ai.NewMockProvider("纯文本1", "修复也失败1")
	p2 := ai.NewMockProvider("纯文本2", "修复也失败2")
	engine := NewPlanEngine(p1)

	input := &PromptInput{
		IssueTitle: "Test",
		IssueBody:  "Body",
	}

	plan, err := engine.FinalWithRetry(context.Background(), input, []ai.Provider{p1, p2}, 0)
	require.NoError(t, err)
	// 软降级：lastRaw 作为 Approach
	assert.NotEmpty(t, plan.Approach)
	assert.Equal(t, "", plan.Title)
}

// ===== AbortError 类型测试 =====

func TestAbortError_ErrorMessage(t *testing.T) {
	ae := &AbortError{
		Err:     fmt.Errorf("parse failed"),
		LastRaw: "some raw text",
		Attempts: []retryAttempt{
			{Provider: "mock", Attempt: 1, Error: "parse failed", Kind: "MissingJSON"},
		},
	}
	assert.Contains(t, ae.Error(), "parse failed")
	assert.Contains(t, ae.Error(), "mock")
}

func TestAbortError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	ae := &AbortError{Err: inner}
	assert.True(t, errors.Is(ae, inner))
}
