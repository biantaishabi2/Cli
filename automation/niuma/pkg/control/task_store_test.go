package control

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateOrReuseActiveIssueTask_ReusesExistingActiveTask(t *testing.T) {
	tmp := t.TempDir()
	bin, tasksFile, countFile := writeStoreAwareTaskCtlBin(t, tmp)

	require.NoError(t, os.WriteFile(tasksFile, []byte(`[{"id":"task-1","subject":"demo","description":"demo","status":"in-progress","blocked_by":[],"metadata":{"issue_num":"42"}}]`), 0o644))

	client := &TaskCtlClient{
		BinPath:   bin,
		StorePath: filepath.Join(tmp, "tasks.json"),
	}

	task, reused, err := client.CreateOrReuseActiveIssueTask("demo", "demo", map[string]string{"issue_num": "42"})
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.True(t, reused)
	assert.Equal(t, "task-1", task.ID)
	_, err = os.Stat(countFile)
	assert.Error(t, err)
}

func TestCreateOrReuseActiveIssueTask_ConcurrentCreateIsDeduplicated(t *testing.T) {
	tmp := t.TempDir()
	bin, _, countFile := writeStoreAwareTaskCtlBin(t, tmp)

	client := &TaskCtlClient{
		BinPath:   bin,
		StorePath: filepath.Join(tmp, "tasks.json"),
	}

	type result struct {
		taskID string
		reused bool
		err    error
	}

	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, reused, err := client.CreateOrReuseActiveIssueTask("demo", "demo", map[string]string{"issue_num": "42"})
			if err != nil {
				results <- result{err: err}
				return
			}
			results <- result{
				taskID: task.ID,
				reused: reused,
			}
		}()
	}
	wg.Wait()
	close(results)

	var got []result
	for r := range results {
		got = append(got, r)
	}
	require.Len(t, got, 2)
	require.NoError(t, got[0].err)
	require.NoError(t, got[1].err)
	assert.Equal(t, "task-1", got[0].taskID)
	assert.Equal(t, "task-1", got[1].taskID)
	assert.NotEqual(t, got[0].reused, got[1].reused)

	countRaw, err := os.ReadFile(countFile)
	require.NoError(t, err)
	assert.Equal(t, "1", strings.TrimSpace(string(countRaw)))
}

func writeStoreAwareTaskCtlBin(t *testing.T, dir string) (binPath, tasksFile, countFile string) {
	t.Helper()

	tasksFile = filepath.Join(dir, "tasks.data.json")
	countFile = filepath.Join(dir, "create.count")
	binPath = filepath.Join(dir, "taskctl")
	script := `#!/usr/bin/env bash
set -euo pipefail
cmd="${1:-}"

if [ "$cmd" = "list" ]; then
  if [ -f "$TASKS_FILE" ]; then
    cat "$TASKS_FILE"
  else
    echo "[]"
  fi
  exit 0
fi

if [ "$cmd" = "create" ]; then
  count=0
  if [ -f "$COUNT_FILE" ]; then
    count="$(cat "$COUNT_FILE")"
  fi
  count=$((count + 1))
  echo "$count" > "$COUNT_FILE"
  task='{"id":"task-'$count'","subject":"demo","description":"demo","status":"pending","blocked_by":[],"metadata":{"issue_num":"42"}}'
  echo "[$task]" > "$TASKS_FILE"
  echo "$task"
  exit 0
fi

echo "{}"
`
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))
	t.Setenv("TASKS_FILE", tasksFile)
	t.Setenv("COUNT_FILE", countFile)
	return binPath, tasksFile, countFile
}
