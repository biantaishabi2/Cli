package ai

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockProvider_Name(t *testing.T) {
	m := NewMockProvider()
	assert.Equal(t, "mock", m.Name())
}

func TestMockProvider_ReturnsResponses(t *testing.T) {
	m := NewMockProvider("response1", "response2")
	ctx := context.Background()

	r1, err := m.Complete(ctx, "prompt1")
	require.NoError(t, err)
	assert.Equal(t, "response1", r1)

	r2, err := m.Complete(ctx, "prompt2")
	require.NoError(t, err)
	assert.Equal(t, "response2", r2)
}

func TestMockProvider_ExhaustedResponses(t *testing.T) {
	m := NewMockProvider("only-one")
	ctx := context.Background()

	_, err := m.Complete(ctx, "first")
	require.NoError(t, err)

	_, err = m.Complete(ctx, "second")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no more responses")
}

func TestMockProvider_WithError(t *testing.T) {
	expectedErr := fmt.Errorf("ai unavailable")
	m := NewMockProviderWithError(expectedErr)
	ctx := context.Background()

	_, err := m.Complete(ctx, "anything")
	assert.ErrorIs(t, err, expectedErr)
}

func TestMockProvider_RecordsCalls(t *testing.T) {
	m := NewMockProvider("r1", "r2")
	ctx := context.Background()

	_, _ = m.Complete(ctx, "p1", WithWorkDir("/tmp"))
	_, _ = m.Complete(ctx, "p2", WithMaxTokens(100))

	calls := m.Calls()
	require.Len(t, calls, 2)
	assert.Equal(t, "p1", calls[0].Prompt)
	assert.Equal(t, "/tmp", calls[0].Options.WorkDir)
	assert.Equal(t, "p2", calls[1].Prompt)
	assert.Equal(t, 100, calls[1].Options.MaxTokens)
	assert.Equal(t, 2, m.CallCount())
}

func TestMockProvider_Reset(t *testing.T) {
	m := NewMockProvider("r1")
	ctx := context.Background()

	_, _ = m.Complete(ctx, "p1")
	assert.Equal(t, 1, m.CallCount())

	m.Reset()
	assert.Equal(t, 0, m.CallCount())

	// 重置后可以重新使用响应
	r, err := m.Complete(ctx, "p1-again")
	require.NoError(t, err)
	assert.Equal(t, "r1", r)
}
