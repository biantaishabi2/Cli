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
	args := []string{"task", "create", "--subject", subject}
	if desc != "" {
		args = append(args, "--desc", desc)
	}
	args = append(args, "--store", c.StorePath)

	// 以 key=value 格式传递 metadata
	for k, v := range meta {
		args = append(args, "--meta", fmt.Sprintf("%s=%s", k, v))
	}

	out, err := c.run(args...)
	if err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	var task Task
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		return nil, fmt.Errorf("解析任务 JSON 失败: %w\noutput: %s", err, out)
	}
	return &task, nil
}

// Update 更新任务
func (c *TaskCtlClient) Update(taskID string, opts UpdateOpts) error {
	args := []string{"task", "update", taskID, "--store", c.StorePath}

	if opts.Status != nil {
		args = append(args, "--status", string(*opts.Status))
	}
	if opts.BlockedBy != nil {
		for _, dep := range *opts.BlockedBy {
			args = append(args, "--blocked-by", dep)
		}
	}
	if opts.Metadata != nil {
		for k, v := range *opts.Metadata {
			args = append(args, "--meta", fmt.Sprintf("%s=%s", k, v))
		}
	}

	_, err := c.run(args...)
	if err != nil {
		return fmt.Errorf("更新任务 %s 失败: %w", taskID, err)
	}
	return nil
}

// Ready 获取所有就绪任务（无 blocked_by 且状态为 pending）
func (c *TaskCtlClient) Ready() ([]Task, error) {
	out, err := c.run("task", "ready", "--store", c.StorePath, "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("获取就绪任务失败: %w", err)
	}

	var tasks []Task
	if err := json.Unmarshal([]byte(out), &tasks); err != nil {
		return nil, fmt.Errorf("解析就绪任务 JSON 失败: %w\noutput: %s", err, out)
	}
	return tasks, nil
}

// Dag 获取 DAG 图
func (c *TaskCtlClient) Dag() (*DagGraph, error) {
	out, err := c.run("task", "dag", "--store", c.StorePath, "--format", "json")
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
	args := []string{"task", "list", "--store", c.StorePath, "--format", "json"}
	if status != "" {
		args = append(args, "--status", status)
	}

	out, err := c.run(args...)
	if err != nil {
		return nil, fmt.Errorf("列出任务失败: %w", err)
	}

	var tasks []Task
	if err := json.Unmarshal([]byte(out), &tasks); err != nil {
		return nil, fmt.Errorf("解析任务列表 JSON 失败: %w\noutput: %s", err, out)
	}
	return tasks, nil
}

// Get 获取单个任务
func (c *TaskCtlClient) Get(taskID string) (*Task, error) {
	out, err := c.run("task", "get", taskID, "--store", c.StorePath, "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("获取任务 %s 失败: %w", taskID, err)
	}

	var task Task
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		return nil, fmt.Errorf("解析任务 JSON 失败: %w\noutput: %s", err, out)
	}
	return &task, nil
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
