// pkg/ai/mock.go
// MockProvider 用于测试的 AI Provider 实现
package ai

import (
	"context"
	"fmt"
	"sync"
)

// Call 记录一次对 MockProvider 的调用
type Call struct {
	Prompt  string
	Options *options
}

// MockProvider 使用预设响应的测试用 Provider
type MockProvider struct {
	mu             sync.Mutex
	responses      []string // 按顺序返回的响应
	calls          []Call   // 记录所有调用
	index          int
	err            error    // 如果设置了则每次调用都返回此错误
	executeResults []string // Execute 专用响应（可选）
	executeIndex   int
}

// NewMockProvider 创建 MockProvider，按顺序返回 responses
func NewMockProvider(responses ...string) *MockProvider {
	return &MockProvider{
		responses: responses,
	}
}

// NewMockProviderWithError 创建总是返回错误的 MockProvider
func NewMockProviderWithError(err error) *MockProvider {
	return &MockProvider{err: err}
}

func (m *MockProvider) Name() string {
	return "mock"
}

func (m *MockProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	o := buildOptions(opts)
	m.calls = append(m.calls, Call{Prompt: prompt, Options: o})

	if m.err != nil {
		return "", m.err
	}

	if m.index >= len(m.responses) {
		return "", fmt.Errorf("mock: no more responses (called %d times, have %d responses)", m.index+1, len(m.responses))
	}

	resp := m.responses[m.index]
	m.index++
	return resp, nil
}

func (m *MockProvider) Execute(ctx context.Context, prompt string, opts ...Option) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	o := buildOptions(opts)
	m.calls = append(m.calls, Call{Prompt: prompt, Options: o})

	if m.err != nil {
		return "", m.err
	}

	// 如果有专用 Execute 响应，优先使用
	if len(m.executeResults) > 0 {
		if m.executeIndex >= len(m.executeResults) {
			return "", fmt.Errorf("mock: no more execute responses (called %d times, have %d)", m.executeIndex+1, len(m.executeResults))
		}
		resp := m.executeResults[m.executeIndex]
		m.executeIndex++
		return resp, nil
	}

	// 回退到共享 responses
	if m.index >= len(m.responses) {
		return "", fmt.Errorf("mock: no more responses (called %d times, have %d responses)", m.index+1, len(m.responses))
	}

	resp := m.responses[m.index]
	m.index++
	return resp, nil
}

// SetExecuteResults 设置 Execute 专用的响应序列
func (m *MockProvider) SetExecuteResults(results ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executeResults = results
	m.executeIndex = 0
}

// Calls 返回所有调用记录
func (m *MockProvider) Calls() []Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Call, len(m.calls))
	copy(result, m.calls)
	return result
}

// CallCount 返回调用次数
func (m *MockProvider) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// LastPrompt 返回最后一次调用的 prompt
func (m *MockProvider) LastPrompt() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return ""
	}
	return m.calls[len(m.calls)-1].Prompt
}

// Reset 重置调用记录和响应索引
func (m *MockProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
	m.index = 0
}
