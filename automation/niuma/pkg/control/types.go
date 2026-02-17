// pkg/control/types.go
// 控制层核心类型定义
package control

import "strconv"

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusBlocked    TaskStatus = "blocked"
	TaskStatusFailed     TaskStatus = "failed"
)

// Task 表示一个受 taskctl 管理的任务
type Task struct {
	ID        string            `json:"id"`
	Subject   string            `json:"subject"`
	Desc      string            `json:"desc,omitempty"`
	Status    TaskStatus        `json:"status"`
	BlockedBy []string          `json:"blocked_by,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// IssueNum 从 metadata 获取 issue 编号
func (t *Task) IssueNum() int {
	if t.Metadata == nil {
		return 0
	}
	v, ok := t.Metadata["issue_num"]
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

// PRNum 从 metadata 获取 PR 编号
func (t *Task) PRNum() int {
	if t.Metadata == nil {
		return 0
	}
	v, ok := t.Metadata["pr_num"]
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

// Branch 从 metadata 获取分支名
func (t *Task) Branch() string {
	if t.Metadata == nil {
		return ""
	}
	return t.Metadata["branch"]
}

// DagNode DAG 图的节点
type DagNode struct {
	ID     string   `json:"id"`
	Deps   []string `json:"deps,omitempty"`
	Status string   `json:"status"`
}

// DagGraph DAG 图（节点 + 边）
type DagGraph struct {
	Nodes []DagNode `json:"nodes"`
}

// BranchInfo 分支信息（用于 integration 构建）
type BranchInfo struct {
	Branch   string `json:"branch"`
	IssueNum int    `json:"issue_num"`
	PRNum    int    `json:"pr_num"`
	TaskID   string `json:"task_id"`
}

// IssueInfo GitHub issue 信息
type IssueInfo struct {
	Number int      `json:"number"`
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
}

// AnalysisResult AI 依赖分析结果
type AnalysisResult struct {
	// Dependencies issue 间依赖：key 依赖 value 列表中的 issue
	Dependencies map[int][]int `json:"dependencies"`
	// PotentialConflicts 可能冲突的 issue 组
	PotentialConflicts [][]int `json:"potential_conflicts,omitempty"`
}

// IntegrationResult integration 分支构建结果
type IntegrationResult struct {
	Branch    string `json:"branch"`
	Merged    []int  `json:"merged"`
	Conflicts []int  `json:"conflicts,omitempty"`
	Skipped   []int  `json:"skipped,omitempty"`
}

// ControlStatus 全局控制状态
type ControlStatus struct {
	Dag         *DagGraph          `json:"dag"`
	Tasks       []Task             `json:"tasks"`
	Integration *IntegrationResult `json:"integration,omitempty"`
}

// UpdateOpts 更新任务的选项
type UpdateOpts struct {
	Status    *TaskStatus        `json:"status,omitempty"`
	BlockedBy *[]string          `json:"blocked_by,omitempty"`
	Metadata  *map[string]string `json:"metadata,omitempty"`
}

// ParseParent 解析 issue body 中的 parent 声明（Sub-Issue 模式）
// 返回 0 表示没有 parent
// 支持格式：parent: #40 或 parent issue: #40
func ParseParent(body string) int {
	// 这个函数在 analyzer.go 中实现，这里只是声明
	return 0
}
