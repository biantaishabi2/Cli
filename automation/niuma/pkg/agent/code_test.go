//go:build !ci

package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeGen_Implement_HappyPath(t *testing.T) {
	mock := ai.NewMockProvider("// 修改 auth.go\nfunc Login() { /* new code */ }")
	gen := NewCodeGen(mock)

	input := &PromptInput{
		IssueTitle: "Add login",
		FinalPlan:  "创建 Login 函数处理用户登录",
	}

	result, err := gen.Implement(context.Background(), input, "/tmp/workdir")
	require.NoError(t, err)
	assert.Contains(t, result.RawOutput, "auth.go")

	// 验证 prompt 包含 plan 细节
	calls := mock.Calls()
	assert.Contains(t, calls[0].Prompt, "Login 函数")
	assert.Equal(t, "/tmp/workdir", calls[0].Options.WorkDir)
}

func TestCodeGen_Implement_AIError(t *testing.T) {
	mock := ai.NewMockProviderWithError(fmt.Errorf("timeout"))
	gen := NewCodeGen(mock)

	_, err := gen.Implement(context.Background(), &PromptInput{IssueTitle: "test"}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI 代码实现失败")
}

func TestCodeGen_Iterate_PromptContainsReview(t *testing.T) {
	mock := ai.NewMockProvider("// 修复：添加了输入验证")
	gen := NewCodeGen(mock)

	input := &PromptInput{
		IssueTitle:    "Add login",
		FinalPlan:     "创建 Login 函数",
		ReviewComment: "请添加输入验证和错误处理",
		PRDiff:        "+func Login(name string) {}",
	}

	result, err := gen.Iterate(context.Background(), input, "")
	require.NoError(t, err)
	assert.Contains(t, result.RawOutput, "输入验证")

	// 验证 prompt 包含 review 意见
	calls := mock.Calls()
	assert.Contains(t, calls[0].Prompt, "请添加输入验证和错误处理")
	assert.Contains(t, calls[0].Prompt, "Login(name string)")
}

func TestCodeGen_Iterate_AIError(t *testing.T) {
	mock := ai.NewMockProviderWithError(fmt.Errorf("rate limit"))
	gen := NewCodeGen(mock)

	_, err := gen.Iterate(context.Background(), &PromptInput{IssueTitle: "test"}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI 迭代修复失败")
}
