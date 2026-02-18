// pkg/control/types.go
// 控制层核心类型定义
package control

import (
	"strconv"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in-progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusDeleted    TaskStatus = "deleted"
	TaskStatusBlocked    TaskStatus = "blocked"
	TaskStatusFailed     TaskStatus = "failed"
)

// Task 表示一个受 taskctl 管理的任务
type Task struct {
	ID          string            `json:"id"`
	Subject     string            `json:"subject"`
	Description string            `json:"description,omitempty"`
	Desc        string            `json:"desc,omitempty"` // 兼容旧字段
	ActiveForm  string            `json:"active_form,omitempty"`
	Status      TaskStatus        `json:"status"`
	BlockedBy   []string          `json:"blocked_by,omitempty"`
	Blocks      []string          `json:"blocks,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
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

// DagEdge DAG 依赖边：FromIssue -> ToIssue（ToIssue 被 FromIssue 阻塞）
type DagEdge struct {
	FromIssue int `json:"from_issue"`
	ToIssue   int `json:"to_issue"`
}

// DagSyncMode DAG 同步触发模式
type DagSyncMode string

const (
	DagSyncModeEvent     DagSyncMode = "event"
	DagSyncModeReconcile DagSyncMode = "reconcile"
	DagSyncModeManual    DagSyncMode = "manual"
)

// DagSyncStatus DAG 同步结果状态
type DagSyncStatus string

const (
	DagSyncStatusSuccess DagSyncStatus = "success"
	DagSyncStatusFailed  DagSyncStatus = "failed"
	DagSyncStatusSkipped DagSyncStatus = "skipped"
)

// DagSyncResult DAG 同步执行结果
type DagSyncResult struct {
	Mode          DagSyncMode   `json:"mode"`
	Status        DagSyncStatus `json:"status"`
	DagHash       string        `json:"dag_hash"`
	TotalEdges    int           `json:"total_edges"`
	AppliedAdd    int           `json:"applied_add"`
	AppliedRemove int           `json:"applied_remove"`
	SkippedEdges  int           `json:"skipped_edges"`
	ErrorType     string        `json:"error_type,omitempty"`
	Error         string        `json:"error,omitempty"`
}

// DagSyncState DAG 同步持久化状态
type DagSyncState struct {
	LastHash        string `json:"last_hash,omitempty"`
	LastSuccessAt   string `json:"last_success_at,omitempty"`
	SuccessCount    int    `json:"success_count,omitempty"`
	FailCount       int    `json:"fail_count,omitempty"`
	LastError       string `json:"last_error,omitempty"`
	LastErrorAt     string `json:"last_error_at,omitempty"`
	SkippedEdges    int    `json:"skipped_edges,omitempty"`
	LastReconcileAt string `json:"last_reconcile_at,omitempty"`
}

// DagSyncConfig DAG 展示层同步配置
type DagSyncConfig struct {
	PollInterval         time.Duration   `json:"poll_interval"`
	MaxRetry             int             `json:"max_retry"`
	RetryBackoff         []time.Duration `json:"retry_backoff"`
	RateLimitRPS         int             `json:"rate_limit_rps"`
	Timeout              time.Duration   `json:"timeout"`
	SkippedEdgeThreshold float64         `json:"skipped_edge_threshold"`
	StateFile            string          `json:"state_file"`
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
	State  string   `json:"state,omitempty"`
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

// MergeStatus integration 合并执行状态
type MergeStatus string

const (
	MergeStatusMerged       MergeStatus = "merged"
	MergeStatusAutoResolved MergeStatus = "auto_resolved"
	MergeStatusEscalated    MergeStatus = "escalated"
)

// ConflictSummary 冲突摘要（用于 metadata、日志和审计）
type ConflictSummary struct {
	Files           []string       `json:"files"`
	FileHunkCount   map[string]int `json:"file_hunk_count,omitempty"`
	TotalHunkCount  int            `json:"total_hunk_count"`
	Reason          string         `json:"reason,omitempty"`
	SuggestedAction string         `json:"suggested_action,omitempty"`
}

// MergeOutcome 统一 merge 执行结果
type MergeOutcome struct {
	Status            MergeStatus      `json:"status"`
	IntegrationBranch string           `json:"integration_branch"`
	SourceBranch      string           `json:"source_branch"`
	IssueNum          int              `json:"issue_num,omitempty"`
	PRNum             int              `json:"pr_num,omitempty"`
	AutoResolvedFiles []string         `json:"auto_resolved_files,omitempty"`
	Conflict          *ConflictSummary `json:"conflict,omitempty"`
	ExecutorVersion   string           `json:"executor_version,omitempty"`
	ExecutedAt        string           `json:"executed_at,omitempty"`
}

// IssueLockResult issue 锁执行结果
type IssueLockResult string

const (
	IssueLockResultSucceeded IssueLockResult = "succeeded"
	IssueLockResultFailed    IssueLockResult = "failed"
	IssueLockResultSkipped   IssueLockResult = "skipped"
	IssueLockResultLocked    IssueLockResult = "locked"
)

// IssueLockRecord issue 锁记录
type IssueLockRecord struct {
	IssueNumber int             `json:"issue_number"`
	Owner       string          `json:"owner"`
	AcquiredAt  time.Time       `json:"acquired_at"`
	ExpiresAt   time.Time       `json:"expires_at"`
	HeartbeatAt time.Time       `json:"heartbeat_at"`
	LastResult  IssueLockResult `json:"last_result,omitempty"`
}

// IssueLockStore issue 锁存储接口
type IssueLockStore interface {
	TryAcquire(issueNumber int, owner string, now time.Time, ttl time.Duration) (IssueLockRecord, bool, error)
	Refresh(issueNumber int, owner string, now time.Time, ttl time.Duration) (IssueLockRecord, error)
	Release(issueNumber int, owner string, now time.Time, lastResult IssueLockResult) error
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
