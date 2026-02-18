package control

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDagSyncStateStore_ConcurrentLoadSave(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), dagSyncStateFileName)
	store := newDagSyncStateStore(statePath)

	const (
		workers = 8
		rounds  = 20
	)
	errCh := make(chan error, workers*rounds*2)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		workerID := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < rounds; j++ {
				state := DagSyncState{
					LastHash:     fmt.Sprintf("worker-%d-round-%d", workerID, j),
					SuccessCount: workerID*rounds + j + 1,
				}
				if err := store.Save(state); err != nil {
					errCh <- err
					return
				}
				if _, err := store.Load(); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	loaded, err := store.Load()
	require.NoError(t, err)
	assert.NotEmpty(t, loaded.LastHash)

	payload, err := os.ReadFile(statePath)
	require.NoError(t, err)
	var diskState DagSyncState
	require.NoError(t, json.Unmarshal(payload, &diskState))
	assert.NotEmpty(t, diskState.LastHash)
}

func TestDagSyncStateStore_DegradedModeConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	store := newDagSyncStateStore(filepath.Join(blocker, dagSyncStateFileName))

	// 第一次保存触发磁盘异常并进入降级模式。
	err := store.Save(DagSyncState{LastHash: "seed", SuccessCount: 1})
	require.Error(t, err)
	assert.True(t, store.diskDegraded)

	const workers = 20
	errCh := make(chan error, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				saveErr := store.Save(DagSyncState{
					LastHash:     fmt.Sprintf("mem-%d", i),
					SuccessCount: i,
				})
				if saveErr == nil || !strings.Contains(saveErr.Error(), "降级到内存模式") {
					errCh <- fmt.Errorf("save 降级错误不符合预期: %v", saveErr)
				}
				return
			}
			if _, loadErr := store.Load(); loadErr != nil {
				errCh <- fmt.Errorf("load 降级模式失败: %w", loadErr)
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for checkErr := range errCh {
		require.NoError(t, checkErr)
	}

	state, loadErr := store.Load()
	require.NoError(t, loadErr)
	assert.NotEmpty(t, state.LastHash)
}
