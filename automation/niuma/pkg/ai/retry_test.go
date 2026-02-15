//go:build !ci

package ai

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithRetry_FirstSuccess(t *testing.T) {
	calls := 0
	err := WithRetry(context.Background(), 3, func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestWithRetry_SucceedsOnRetry(t *testing.T) {
	calls := 0
	err := WithRetry(context.Background(), 3, func() error {
		calls++
		if calls < 3 {
			return fmt.Errorf("attempt %d failed", calls)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestWithRetry_ExhaustedRetries(t *testing.T) {
	calls := 0
	err := WithRetry(context.Background(), 2, func() error {
		calls++
		return fmt.Errorf("always fails")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exhausted 2 retries")
	assert.Equal(t, 3, calls) // 初始 + 2 次重试
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	go func() {
		// 在第一次退避等待时取消
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := WithRetry(ctx, 5, func() error {
		calls++
		return fmt.Errorf("fails")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry cancelled")
	// 应该只执行了 1-2 次（第一次立即执行，context 取消后不再重试）
	assert.LessOrEqual(t, calls, 2)
}
