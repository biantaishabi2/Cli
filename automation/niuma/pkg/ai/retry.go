// pkg/ai/retry.go
// 指数退避重试工具
package ai

import (
	"context"
	"fmt"
	"time"
)

// WithRetry 执行 fn，失败时以指数退避重试，最多 maxRetries 次
// 退避序列：1s, 2s, 4s, 8s...
// 尊重 context 取消
func WithRetry(ctx context.Context, maxRetries int, fn func() error) error {
	var lastErr error
	backoff := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// 最后一次不等待
		if attempt == maxRetries {
			break
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w (last error: %v)", ctx.Err(), lastErr)
		case <-time.After(backoff):
			backoff *= 2
		}
	}

	return fmt.Errorf("exhausted %d retries: %w", maxRetries, lastErr)
}
