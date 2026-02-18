// pkg/agent/types.go
// Agent 核心数据类型
package agent

// DraftPlan AI 生成的方案草案
type DraftPlan struct {
	Summary       string   `json:"summary"`         // 问题概述
	Approach      string   `json:"approach"`        // 解决方案
	AffectedFiles []string `json:"affected_files"`  // 涉及文件列表
	Risks         string   `json:"risks,omitempty"` // 风险点
}

// Decision 讨论决策类型
type Decision string

const (
	DecisionAdoptA Decision = "adopt_A"
	DecisionAdoptB Decision = "adopt_B"
	DecisionMerge  Decision = "merge"
	DecisionDefer  Decision = "defer"
)

// RiskLevel 风险等级
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// DisagreementItem 分歧项
type DisagreementItem struct {
	Topic          string    `json:"topic"`          // 分歧主题
	Options        []string  `json:"options"`        // 备选方案
	Recommendation string    `json:"recommendation"` // 推荐方案
	Risk           RiskLevel `json:"risk"`           // 风险等级
}

// DiscussionSummary 讨论汇总（收敛判定输入）
type DiscussionSummary struct {
	Agreements            []string           `json:"agreements"`              // 已达成一致
	Disagreements         []DisagreementItem `json:"disagreements"`           // 分歧清单
	Decision              Decision           `json:"decision"`                // consolidator 决策
	RequiresHumanDecision bool               `json:"requires_human_decision"` // 是否需要人工决策
	ShouldFinish          bool               `json:"should_finish"`           // 是否应结束讨论
}

// DebateComment AB 轮流评论输出
type DebateComment struct {
	Agreements    []string           `json:"agreements"`    // 同意点
	Disagreements []DisagreementItem `json:"disagreements"` // 分歧点
	Suggestion    string             `json:"suggestion"`    // 建议
}

// FinalPlan 最终方案
type FinalPlan struct {
	Title         string         `json:"title"`          // 方案标题
	Approach      string         `json:"approach"`       // 实现方案
	FileChanges   []FileChange   `json:"file_changes"`   // 文件变更列表
	TestScenarios []TestScenario `json:"test_scenarios"` // 测试场景
}

// FileChange 单个文件变更描述
type FileChange struct {
	Path        string `json:"path"`        // 文件路径
	Action      string `json:"action"`      // 操作：create / modify / delete
	Description string `json:"description"` // 变更描述
}

// TestScenario 测试场景
type TestScenario struct {
	Name     string `json:"name"`     // 场景名
	Input    string `json:"input"`    // 输入
	Expected string `json:"expected"` // 预期输出
}

// ImplementResult AI 生成的代码实现结果（原始文本）
type ImplementResult struct {
	RawOutput string // AI 原始输出
}

// IterateResult AI 迭代修复结果（原始文本）
type IterateResult struct {
	RawOutput string // AI 原始输出
}

// ReviewResult AI 自审结果
type ReviewResult struct {
	Approved      bool     `json:"approved"`                 // 是否通过
	Summary       string   `json:"summary"`                  // 审查总结
	ResolvedItems []string `json:"resolved_items,omitempty"` // 之前讨论已达成共识的条目
	Issues        []string `json:"issues,omitempty"`         // 问题列表（仅新发现）
}
