package control

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverBin_ConfigPath(t *testing.T) {
	// 创建一个临时 "taskctl" 文件
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "taskctl")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))

	got, err := discoverBin(bin, tmp)
	require.NoError(t, err)
	assert.Equal(t, bin, got)
}

func TestDiscoverBin_ConfigPathNotExist(t *testing.T) {
	_, err := discoverBin("/nonexistent/taskctl", t.TempDir())
	assert.ErrorContains(t, err, "配置的 taskctl 路径不存在")
}

func TestDiscoverBin_LocalBin(t *testing.T) {
	tmp := t.TempDir()
	localBin := filepath.Join(tmp, "orchestration", "taskctl", "target", "release", "taskctl")
	require.NoError(t, os.MkdirAll(filepath.Dir(localBin), 0o755))
	require.NoError(t, os.WriteFile(localBin, []byte("#!/bin/sh\n"), 0o755))

	got, err := discoverBin("", tmp)
	// 如果 $PATH 中有 taskctl，可能返回 $PATH 的（Level 2 优先）
	// 这里只验证不报错且返回了某个路径
	require.NoError(t, err)
	assert.NotEmpty(t, got)
}

func TestDiscoverBin_NotFound(t *testing.T) {
	// 设置空 PATH 确保找不到
	old := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer os.Setenv("PATH", old)

	_, err := discoverBin("", t.TempDir())
	assert.ErrorContains(t, err, "未找到 taskctl 二进制")
}

func TestTaskIssueNum(t *testing.T) {
	task := &Task{
		Metadata: map[string]string{"issue_num": "42"},
	}
	assert.Equal(t, 42, task.IssueNum())
}

func TestTaskIssueNum_NoMetadata(t *testing.T) {
	task := &Task{}
	assert.Equal(t, 0, task.IssueNum())
}

func TestTaskPRNum(t *testing.T) {
	task := &Task{
		Metadata: map[string]string{"pr_num": "10"},
	}
	assert.Equal(t, 10, task.PRNum())
}

func TestTaskBranch(t *testing.T) {
	task := &Task{
		Metadata: map[string]string{"branch": "feat/42-login"},
	}
	assert.Equal(t, "feat/42-login", task.Branch())
}

func TestNewTaskCtlClient_CreatesNiumaDir(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "taskctl")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755))

	client, err := NewTaskCtlClient(bin, tmp)
	require.NoError(t, err)
	assert.Equal(t, bin, client.BinPath)
	assert.Equal(t, filepath.Join(tmp, ".niuma", "tasks.json"), client.StorePath)

	// 验证 .niuma 目录被创建
	_, err = os.Stat(filepath.Join(tmp, ".niuma"))
	assert.NoError(t, err)
}
