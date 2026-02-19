package control

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// CreateOrReuseActiveIssueTask 按 issue_num 创建或复用 active task。
// 返回 reused=true 表示复用了已存在 active task，调用方应视为 no-op。
func (c *TaskCtlClient) CreateOrReuseActiveIssueTask(subject, desc string, meta map[string]string) (*Task, bool, error) {
	issueNum, ok := parseIssueNum(meta)
	if !ok {
		task, err := c.Create(subject, desc, meta)
		if err != nil {
			return nil, false, err
		}
		return task, false, nil
	}

	var created *Task
	var reused *Task
	lockPath := c.StorePath + ".lock"
	if err := withStoreLock(lockPath, func() error {
		existing, err := c.findActiveTaskByIssue(issueNum)
		if err != nil {
			return err
		}
		if existing != nil {
			reused = existing
			return nil
		}
		created, err = c.Create(subject, desc, meta)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, false, err
	}
	if reused != nil {
		return reused, true, nil
	}
	return created, false, nil
}

func (c *TaskCtlClient) findActiveTaskByIssue(issueNum int) (*Task, error) {
	tasks, err := c.List("")
	if err != nil {
		// 首次运行 store 可能不存在，视为无任务。
		if strings.Contains(err.Error(), "no such file") || strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}
	for i := range tasks {
		if tasks[i].IssueNum() == issueNum && isTaskActiveStatus(tasks[i].Status) {
			task := tasks[i]
			return &task, nil
		}
	}
	return nil, nil
}

func parseIssueNum(meta map[string]string) (int, bool) {
	if meta == nil {
		return 0, false
	}
	raw := strings.TrimSpace(meta["issue_num"])
	if raw == "" {
		return 0, false
	}
	num, err := strconv.Atoi(raw)
	if err != nil || num <= 0 {
		return 0, false
	}
	return num, true
}

func isTaskActiveStatus(status TaskStatus) bool {
	switch normalizeTaskStatus(status) {
	case TaskStatusPending, TaskStatusInProgress, TaskStatusBlocked, TaskStatusFailed:
		return true
	default:
		return false
	}
}

func withStoreLock(lockPath string, fn func() error) error {
	fd, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("打开 store 锁失败: %w", err)
	}
	defer fd.Close()

	if err := syscall.Flock(int(fd.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("加锁 store 失败: %w", err)
	}
	defer func() {
		_ = syscall.Flock(int(fd.Fd()), syscall.LOCK_UN)
	}()

	return fn()
}
