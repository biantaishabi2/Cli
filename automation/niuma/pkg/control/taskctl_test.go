package control

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestTaskCtlClientCreate_UsesFlatCommandAndJsonMetadata(t *testing.T) {
	tmp := t.TempDir()
	argsFile := filepath.Join(tmp, "args.txt")
	bin := filepath.Join(tmp, "taskctl")
	script := `#!/usr/bin/env bash
set -e
: > "$ARGS_FILE"
for arg in "$@"; do
  printf '%s\n' "$arg" >> "$ARGS_FILE"
done
cat <<'JSON'
{"id":"t1","subject":"demo","description":"desc","status":"pending","blocked_by":[],"metadata":{"issue_num":"42"}}
JSON
`
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	t.Setenv("ARGS_FILE", argsFile)

	client := &TaskCtlClient{BinPath: bin, StorePath: filepath.Join(tmp, "tasks.json")}
	task, err := client.Create("demo", "desc", map[string]string{"issue_num": "42"})
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, TaskStatusPending, task.Status)

	content, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	args := strings.Fields(string(content))

	require.NotEmpty(t, args)
	assert.Equal(t, "create", args[0])
	assert.NotContains(t, args, "task")
	assert.Contains(t, args, "--description")
	assert.Contains(t, args, "--metadata")

	metadataArg := ""
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--metadata" {
			metadataArg = args[i+1]
			break
		}
	}
	require.NotEmpty(t, metadataArg)

	var metadata map[string]string
	require.NoError(t, json.Unmarshal([]byte(metadataArg), &metadata))
	assert.Equal(t, "42", metadata["issue_num"])
}

func TestTaskCtlClientUpdate_MapsStatusAndBlockedBy(t *testing.T) {
	tmp := t.TempDir()
	argsFile := filepath.Join(tmp, "args.txt")
	bin := filepath.Join(tmp, "taskctl")
	script := `#!/usr/bin/env bash
set -e
: > "$ARGS_FILE"
for arg in "$@"; do
  printf '%s\n' "$arg" >> "$ARGS_FILE"
done
echo "{}"
`
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	t.Setenv("ARGS_FILE", argsFile)

	client := &TaskCtlClient{BinPath: bin, StorePath: filepath.Join(tmp, "tasks.json")}
	status := TaskStatus("in_progress") // 验证旧状态值可映射到新版 in-progress
	blockedBy := []string{"t1", "t2"}
	metadata := map[string]string{"k": "v"}
	err := client.Update("task-9", UpdateOpts{
		Status:    &status,
		BlockedBy: &blockedBy,
		Metadata:  &metadata,
	})
	require.NoError(t, err)

	content, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	args := strings.Fields(string(content))

	assert.Equal(t, "update", args[0])
	assert.Contains(t, args, "--task-id")
	assert.Contains(t, args, "task-9")
	assert.Contains(t, args, "--status")
	assert.Contains(t, args, "in-progress")
	assert.Contains(t, args, "--add-blocked-by")
	assert.Contains(t, args, "t1,t2")
}

func TestTaskCtlClientList_NormalizesLegacyStatus(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "taskctl")
	script := `#!/usr/bin/env bash
set -e
cat <<'JSON'
[{"id":"t1","subject":"demo","description":"d","status":"in_progress","blocked_by":[],"metadata":{"issue_num":"42"}}]
JSON
`
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))

	client := &TaskCtlClient{BinPath: bin, StorePath: filepath.Join(tmp, "tasks.json")}
	tasks, err := client.List("")
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, TaskStatusInProgress, tasks[0].Status)
	assert.Equal(t, "d", tasks[0].Description)
}

func TestBuildPhaseIdempotencyMetadataPatch_EmptyPhaseOrKeyReturnsNil(t *testing.T) {
	recordedAt := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)

	assert.Nil(t, buildPhaseIdempotencyMetadataPatch(nil, "", "key", "hash", recordedAt))
	assert.Nil(t, buildPhaseIdempotencyMetadataPatch(nil, "fix", "", "hash", recordedAt))
	assert.Nil(t, buildPhaseIdempotencyMetadataPatch(nil, "   ", "key", "hash", recordedAt))
	assert.Nil(t, buildPhaseIdempotencyMetadataPatch(nil, "fix", "   ", "hash", recordedAt))
}

func TestBuildPhaseIdempotencyMetadataPatch_NilMetadataWritesAllFields(t *testing.T) {
	recordedAt := time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)

	update := buildPhaseIdempotencyMetadataPatch(nil, "fix", "key-1", "hash-1", recordedAt)
	require.Len(t, update, 3)
	assert.Equal(t, "key-1", update["idempotency.key.fix"])
	assert.Equal(t, "hash-1", update["idempotency.input_hash.fix"])
	assert.Equal(t, "2026-02-18T12:00:00Z", update["idempotency.timestamp.fix"])
}

func TestPhaseScopedMetadataKey_EmptyPrefixOrPhaseReturnsEmpty(t *testing.T) {
	assert.Equal(t, "idempotency.key.fix", phaseScopedMetadataKey(" idempotency.key ", " fix "))
	assert.Empty(t, phaseScopedMetadataKey("", "fix"))
	assert.Empty(t, phaseScopedMetadataKey("idempotency.key", ""))
	assert.Empty(t, phaseScopedMetadataKey("   ", "fix"))
	assert.Empty(t, phaseScopedMetadataKey("idempotency.key", "   "))
}
