// pkg/daemon/tracker.go
// Tracker 统一接口：抽象 GitHub Projects 和 Linear 的状态读写
package daemon

import "context"

// Issue 统一的 issue 数据结构
type Issue struct {
	ID        string   // GitHub: Project item node ID / Linear: issue ID
	Number    int      // issue 编号
	Repo      string   // "owner/repo"
	Title     string   // issue 标题
	Body      string   // issue 描述
	Status    string   // 当前状态（Backlog/Queued/Working/Review/Done）
	DependsOn []string // 解析 body 中的 depends-on（issue 编号字符串）
	PRNumber  int      // 关联的 PR 编号（0 = 无）
	PRMerged  bool     // PR 是否已合并
}

// Tracker 统一的 issue tracker 接口
type Tracker interface {
	// FetchByStatus 拉取指定状态的 issues
	FetchByStatus(ctx context.Context, status string) ([]Issue, error)

	// SetStatus 更新 issue 状态
	SetStatus(ctx context.Context, itemID string, status string) error

	// GetIssue 获取单个 issue 详情（含 PR 状态）
	GetIssue(ctx context.Context, itemID string) (*Issue, error)

	// CheckDependencies 批量检查依赖 issue 的状态
	// 返回 map[issueID]status
	CheckDependencies(ctx context.Context, ids []string) (map[string]string, error)
}

// 标准状态常量
const (
	StatusBacklog = "Backlog"
	StatusQueued  = "Queued"
	StatusWorking = "Working"
	StatusReview  = "Review"
	StatusDone    = "Done"
)
