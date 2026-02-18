package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

const dagSyncStateFileName = "dag_sync.json"

type dagSyncStateStore struct {
	path         string
	lockPath     string
	mu           sync.Mutex
	mem          DagSyncState
	diskDegraded bool
}

func newDagSyncStateStore(path string) *dagSyncStateStore {
	return &dagSyncStateStore{
		path:     path,
		lockPath: path + ".lock",
	}
}

func resolveDagSyncStateFile(repoDir string) string {
	niumaRoot := resolveNiumaRoot(repoDir)
	return filepath.Join(niumaRoot, ".state", dagSyncStateFileName)
}

func resolveNiumaRoot(repoDir string) string {
	if repoDir == "" {
		repoDir = "."
	}
	candidate := filepath.Join(repoDir, "automation", "niuma")
	if st, err := os.Stat(candidate); err == nil && st.IsDir() {
		return candidate
	}
	return repoDir
}

// Load 读取 dag sync 状态；磁盘异常时降级到内存态。
func (s *dagSyncStateStore) Load() (DagSyncState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.diskDegraded {
		return s.mem, nil
	}

	state, err := s.loadFromDiskLocked()
	if err != nil {
		s.diskDegraded = true
		return s.mem, err
	}
	s.mem = state
	return state, nil
}

// Save 保存 dag sync 状态；磁盘异常时降级到内存态。
func (s *dagSyncStateStore) Save(state DagSyncState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mem = state
	if s.diskDegraded {
		return fmt.Errorf("dag sync 状态已降级到内存模式")
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		s.diskDegraded = true
		return fmt.Errorf("创建 dag sync 状态目录失败: %w", err)
	}

	lockFile, err := os.OpenFile(s.lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		s.diskDegraded = true
		return fmt.Errorf("打开 dag sync 锁文件失败: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		s.diskDegraded = true
		return fmt.Errorf("获取 dag sync 锁失败: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 dag sync 状态失败: %w", err)
	}

	if err := writeAtomicFile(s.path, payload, 0o644); err != nil {
		s.diskDegraded = true
		return fmt.Errorf("写入 dag sync 状态失败: %w", err)
	}
	return nil
}

func (s *dagSyncStateStore) loadFromDiskLocked() (DagSyncState, error) {
	var state DagSyncState

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return state, fmt.Errorf("创建 dag sync 状态目录失败: %w", err)
	}

	lockFile, err := os.OpenFile(s.lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return state, fmt.Errorf("打开 dag sync 锁文件失败: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return state, fmt.Errorf("获取 dag sync 锁失败: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DagSyncState{}, nil
		}
		return state, fmt.Errorf("读取 dag sync 状态失败: %w", err)
	}
	if len(data) == 0 {
		return DagSyncState{}, nil
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return DagSyncState{}, fmt.Errorf("解析 dag sync 状态失败: %w", err)
	}
	return state, nil
}

func writeAtomicFile(path string, data []byte, perm os.FileMode) error {
	tmpPath := path + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
