// pkg/control/taskctl.go
// TaskCtlClient 封装 taskctl 二进制调用
package control

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TaskCtlClient 封装 taskctl CLI 操作
type TaskCtlClient struct {
	BinPath   string // taskctl 二进制路径
	StorePath string // 任务存储路径（{repoDir}/.niuma/tasks.json）
}

// NewTaskCtlClient 创建 TaskCtlClient，三级 fallback 发现二进制
func NewTaskCtlClient(configBin, repoDir string) (*TaskCtlClient, error) {
	binPath, err := discoverBin(configBin, repoDir)
	if err != nil {
		return nil, err
	}

	storePath := filepath.Join(repoDir, ".niuma", "tasks.json")
	// 确保 .niuma 目录存在
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		return nil, fmt.Errorf("创建 .niuma 目录失败: %w", err)
	}

	return &TaskCtlClient{
		BinPath:   binPath,
		StorePath: storePath,
	}, nil
}

// discoverBin 三级 fallback 发现 taskctl 二进制
// 1. configBin（用户配置）
// 2. $PATH 查找
// 3. {repoDir}/orchestration/taskctl/target/release/taskctl
func discoverBin(configBin, repoDir string) (string, error) {
	// Level 1: 用户显式配置
	if configBin != "" {
		if _, err := os.Stat(configBin); err == nil {
			return configBin, nil
		}
		return "", fmt.Errorf("配置的 taskctl 路径不存在: %s", configBin)
	}

	// Level 2: $PATH
	if p, err := exec.LookPath("taskctl"); err == nil {
		return p, nil
	}

	// Level 3: 仓库内构建产物
	localBin := filepath.Join(repoDir, "orchestration", "taskctl", "target", "release", "taskctl")
	if _, err := os.Stat(localBin); err == nil {
		return localBin, nil
	}

	return "", fmt.Errorf("未找到 taskctl 二进制：请设置 control.taskctl_bin 或将 taskctl 加入 $PATH 或执行 cargo build --release -p taskctl")
}

// Create 创建新任务
func (c *TaskCtlClient) Create(subject, desc string, meta map[string]string) (*Task, error) {
	description := strings.TrimSpace(desc)
	if description == "" {
		description = subject
	}

	args := []string{
		"create",
		"--store", c.StorePath,
		"--subject", subject,
		"--description", description,
	}

	// 新版 taskctl 使用 JSON 对象传递 metadata。
	if len(meta) > 0 {
		metadataJSON, err := json.Marshal(meta)
		if err != nil {
			return nil, fmt.Errorf("编码 metadata 失败: %w", err)
		}
		args = append(args, "--metadata", string(metadataJSON))
	}

	out, err := c.run(args...)
	if err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	var task Task
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		return nil, fmt.Errorf("解析任务 JSON 失败: %w\noutput: %s", err, out)
	}
	normalizeTask(&task)
	return &task, nil
}

// Update 更新任务
func (c *TaskCtlClient) Update(taskID string, opts UpdateOpts) error {
	args := []string{"update", "--store", c.StorePath, "--task-id", taskID}

	if opts.Status != nil {
		statusArg, err := toTaskctlStatus(*opts.Status)
		if err != nil {
			return fmt.Errorf("更新任务 %s 失败: %w", taskID, err)
		}
		args = append(args, "--status", statusArg)
	}
	if opts.BlockedBy != nil && len(*opts.BlockedBy) > 0 {
		args = append(args, "--add-blocked-by", strings.Join(*opts.BlockedBy, ","))
	}
	if opts.Metadata != nil {
		metadataJSON, err := json.Marshal(*opts.Metadata)
		if err != nil {
			return fmt.Errorf("更新任务 %s 失败: 编码 metadata 失败: %w", taskID, err)
		}
		args = append(args, "--metadata", string(metadataJSON))
	}

	_, err := c.run(args...)
	if err != nil {
		return fmt.Errorf("更新任务 %s 失败: %w", taskID, err)
	}
	return nil
}

// Ready 获取所有就绪任务（无 blocked_by 且状态为 pending）
func (c *TaskCtlClient) Ready() ([]Task, error) {
	out, err := c.run("ready", "--store", c.StorePath)
	if err != nil {
		return nil, fmt.Errorf("获取就绪任务失败: %w", err)
	}

	var tasks []Task
	if err := json.Unmarshal([]byte(out), &tasks); err != nil {
		return nil, fmt.Errorf("解析就绪任务 JSON 失败: %w\noutput: %s", err, out)
	}
	normalizeTasks(tasks)
	return tasks, nil
}

// Dag 获取 DAG 图
func (c *TaskCtlClient) Dag() (*DagGraph, error) {
	out, err := c.run("dag", "--store", c.StorePath)
	if err != nil {
		return nil, fmt.Errorf("获取 DAG 失败: %w", err)
	}

	var dag DagGraph
	if err := json.Unmarshal([]byte(out), &dag); err != nil {
		return nil, fmt.Errorf("解析 DAG JSON 失败: %w\noutput: %s", err, out)
	}
	return &dag, nil
}

// List 列出指定状态的任务（空字符串列出所有）
func (c *TaskCtlClient) List(status string) ([]Task, error) {
	args := []string{"list", "--store", c.StorePath}
	if status != "" {
		statusArg, err := toTaskctlStatus(TaskStatus(status))
		if err != nil {
			return nil, fmt.Errorf("列出任务失败: %w", err)
		}
		args = append(args, "--status", statusArg)
	}

	out, err := c.run(args...)
	if err != nil {
		return nil, fmt.Errorf("列出任务失败: %w", err)
	}

	var tasks []Task
	if err := json.Unmarshal([]byte(out), &tasks); err != nil {
		return nil, fmt.Errorf("解析任务列表 JSON 失败: %w\noutput: %s", err, out)
	}
	normalizeTasks(tasks)
	return tasks, nil
}

// Get 获取单个任务
func (c *TaskCtlClient) Get(taskID string) (*Task, error) {
	out, err := c.run("get", "--task-id", taskID, "--store", c.StorePath)
	if err != nil {
		return nil, fmt.Errorf("获取任务 %s 失败: %w", taskID, err)
	}

	var task Task
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		return nil, fmt.Errorf("解析任务 JSON 失败: %w\noutput: %s", err, out)
	}
	normalizeTask(&task)
	return &task, nil
}

// FindByIssueNum 根据 issue 编号查找任务
// 返回 nil 表示未找到
func (c *TaskCtlClient) FindByIssueNum(issueNum int) (*Task, error) {
	tasks, err := c.List("")
	if err != nil {
		return nil, err
	}

	for _, t := range tasks {
		if t.IssueNum() == issueNum {
			return &t, nil
		}
	}
	return nil, nil
}

// run 执行 taskctl 命令
func (c *TaskCtlClient) run(args ...string) (string, error) {
	cmd := exec.Command(c.BinPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("taskctl %s: %w\noutput: %s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// normalizeTasks 归一化 taskctl 输出，兼容历史字段值。
func normalizeTasks(tasks []Task) {
	for i := range tasks {
		normalizeTask(&tasks[i])
	}
}

// normalizeTask 归一化单个任务结构。
func normalizeTask(task *Task) {
	if task.Description == "" && task.Desc != "" {
		task.Description = task.Desc
	}
	task.Status = normalizeTaskStatus(task.Status)
}

// normalizeTaskStatus 将历史状态值统一到当前约定。
func normalizeTaskStatus(status TaskStatus) TaskStatus {
	switch strings.ToLower(string(status)) {
	case "pending":
		return TaskStatusPending
	case "in-progress", "in_progress":
		return TaskStatusInProgress
	case "completed":
		return TaskStatusCompleted
	case "deleted":
		return TaskStatusDeleted
	default:
		return status
	}
}

// toTaskctlStatus 将控制层状态映射为 taskctl CLI 可接受的状态值。
func toTaskctlStatus(status TaskStatus) (string, error) {
	switch normalizeTaskStatus(status) {
	case TaskStatusPending:
		return "pending", nil
	case TaskStatusInProgress:
		return "in-progress", nil
	case TaskStatusCompleted:
		return "completed", nil
	case TaskStatusDeleted:
		return "deleted", nil
	default:
		return "", fmt.Errorf("不支持的 task 状态: %s", status)
	}
}
