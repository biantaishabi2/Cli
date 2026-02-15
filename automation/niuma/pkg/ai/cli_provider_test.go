//go:build !ci

package ai

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIProvider_Name(t *testing.T) {
	p := &CLIProvider{ProviderName: "test-cli", Cmd: "echo hello"}
	assert.Equal(t, "test-cli", p.Name())
}

func TestCLIProvider_TemplateSubstitution(t *testing.T) {
	p := &CLIProvider{
		ProviderName: "test",
		Cmd:          "cat {prompt_file}",
		MaxRetries:   -1,
	}

	result, err := p.Complete(context.Background(), "hello world")
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestCLIProvider_ExecuteEcho(t *testing.T) {
	p := &CLIProvider{
		ProviderName: "echo",
		Cmd:          "echo 'AI response'",
		MaxRetries:   -1,
	}

	result, err := p.Complete(context.Background(), "any prompt")
	require.NoError(t, err)
	assert.Equal(t, "AI response", result)
}

func TestCLIProvider_Timeout(t *testing.T) {
	p := &CLIProvider{
		ProviderName: "slow",
		Cmd:          "sleep 10",
		MaxRetries:   -1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := p.Complete(ctx, "prompt")
	require.Error(t, err)
}

func TestCLIProvider_NonZeroExitCode(t *testing.T) {
	p := &CLIProvider{
		ProviderName: "fail",
		Cmd:          "exit 1",
		MaxRetries:   -1,
	}

	_, err := p.Complete(context.Background(), "prompt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "命令执行失败")
}

func TestCLIProvider_Stderr(t *testing.T) {
	p := &CLIProvider{
		ProviderName: "stderr",
		Cmd:          "echo 'error info' >&2; exit 1",
		MaxRetries:   -1,
	}

	_, err := p.Complete(context.Background(), "prompt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error info")
}

func TestCLIProvider_Retry(t *testing.T) {
	// 创建一个计数器文件来跟踪重试
	dir := t.TempDir()
	counterFile := filepath.Join(dir, "counter")
	os.WriteFile(counterFile, []byte("0"), 0644)

	// 命令：前两次失败，第三次成功
	cmd := `count=$(cat ` + counterFile + `); count=$((count+1)); echo $count > ` + counterFile + `; if [ $count -lt 3 ]; then exit 1; fi; echo "success"`
	p := &CLIProvider{
		ProviderName: "retry-test",
		Cmd:          cmd,
		MaxRetries:   3,
	}

	result, err := p.Complete(context.Background(), "prompt")
	require.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestCLIProvider_EmptyOutput(t *testing.T) {
	p := &CLIProvider{
		ProviderName: "empty",
		Cmd:          "echo -n ''",
		MaxRetries:   -1,
	}

	_, err := p.Complete(context.Background(), "prompt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "命令输出为空")
}

func TestCLIProvider_WorkDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("found"), 0644)

	p := &CLIProvider{
		ProviderName: "workdir",
		Cmd:          "cat marker.txt",
		MaxRetries:   -1,
	}

	result, err := p.Complete(context.Background(), "prompt", WithWorkDir(dir))
	require.NoError(t, err)
	assert.Equal(t, "found", result)
}
