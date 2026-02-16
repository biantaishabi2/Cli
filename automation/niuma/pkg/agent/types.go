// pkg/agent/types.go
// Agent 核心数据类型
package agent

// DraftPlan AI 生成的方案草案
type DraftPlan struct {
	Summary       string   `json:"summary"`        // 问题概述
	Approach      string   `json:"approach"`        // 解决方案
	AffectedFiles []string `json:"affected_files"`  // 涉及文件列表
	Risks         string   `json:"risks,omitempty"` // 风险点
}

// DiscussionSummary 讨论汇总
type DiscussionSummary struct {
	Consensus    string   `json:"consensus"`              // 已达成共识
	OpenItems    []string `json:"open_items"`              // 待讨论项
	Suggestion   string   `json:"suggestion,omitempty"`    // AI 建议
	ShouldFinish bool     `json:"should_finish,omitempty"` // 是否建议结束讨论
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
