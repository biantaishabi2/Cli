// pkg/marker/marker.go
// 幂等 Marker：解析和渲染 HTML 注释标记，用于跟踪机器人操作状态
package marker

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Type 定义 Marker 类型
type Type string

const (
	TypePlanDraft                  Type = "BOT:PLAN_DRAFT"
	TypeDiscussionSummary          Type = "BOT:DISCUSSION_SUMMARY"
	TypePlanFinal                  Type = "BOT:PLAN_FINAL"
	TypePRCreated                  Type = "BOT:PR_CREATED"
	TypeConvergeWarning            Type = "BOT:CONVERGE_WARNING"
	TypeDiscussionRoundLimitNotice Type = "BOT:DISCUSSION_ROUND_LIMIT_NOTICE"
	TypeLabelGuard                 Type = "BOT:LABEL_GUARD"
	TypeGateRetry                  Type = "BOT:GATE_RETRY"
	TypeImplementProgress          Type = "BOT:IMPLEMENT_PROGRESS"
	TypeIterateProgress            Type = "BOT:ITERATE_PROGRESS"
)

// AllTypes 返回所有有效的 Marker 类型
var AllTypes = []Type{
	TypePlanDraft,
	TypeDiscussionSummary,
	TypePlanFinal,
	TypePRCreated,
	TypeConvergeWarning,
	TypeDiscussionRoundLimitNotice,
	TypeLabelGuard,
	TypeGateRetry,
	TypeImplementProgress,
	TypeIterateProgress,
}

// Marker 表示一个嵌入在 GitHub 评论中的幂等标记
type Marker struct {
	Type          Type
	Issue         int
	Revision      int
	PR            int    // 仅 PR_CREATED 使用
	Finish        bool   // 仅 DISCUSSION_SUMMARY 使用：AI 建议结束讨论
	Decision      string // 仅 DISCUSSION_SUMMARY 使用：决策（adopt_A|adopt_B|merge|defer）
	Human         bool   // 仅 DISCUSSION_SUMMARY 使用：是否需要人工决策
	Risk          string // 仅 DISCUSSION_SUMMARY 使用：最高风险（low|medium|high）
	DisagreeCount int    // 仅 DISCUSSION_SUMMARY 使用：分歧数量
	Mode          string // 仅 DISCUSSION_SUMMARY 使用：讨论模式（当前固定 debate_ab）
	Label         string // 仅 LABEL_GUARD 使用
	Action        string // 仅 LABEL_GUARD 使用
	Actor         string // 仅 LABEL_GUARD 使用
	AttemptKey    string // 仅 GATE_RETRY 使用：重试作用域 key
}

// markerRe 匹配 <!-- BOT:TYPE key=value key=value ... -->
var markerRe = regexp.MustCompile(`<!--\s+(BOT:\w+)((?:\s+\w+=\S+)*)\s+-->`)

// kvRe 匹配单个 key=value 对
var kvRe = regexp.MustCompile(`(\w+)=(\S+)`)

// Parse 从单行文本中解析一个 Marker，无匹配返回 nil
func Parse(line string) *Marker {
	match := markerRe.FindStringSubmatch(line)
	if match == nil {
		return nil
	}

	t := Type(match[1])
	if !isValidType(t) {
		return nil
	}

	m := &Marker{Type: t}

	// 解析 key=value 对
	kvMatches := kvRe.FindAllStringSubmatch(match[2], -1)
	for _, kv := range kvMatches {
		key, val := kv[1], kv[2]
		switch key {
		case "issue":
			if n, err := strconv.Atoi(val); err == nil {
				m.Issue = n
			}
		case "rev":
			if n, err := strconv.Atoi(val); err == nil {
				m.Revision = n
			}
		case "pr":
			if n, err := strconv.Atoi(val); err == nil {
				m.PR = n
			}
		case "finish":
			if val == "1" || val == "true" {
				m.Finish = true
			}
		case "decision":
			m.Decision = val
		case "human":
			if val == "1" || val == "true" {
				m.Human = true
			}
		case "risk":
			m.Risk = val
		case "dcount":
			if n, err := strconv.Atoi(val); err == nil {
				m.DisagreeCount = n
			}
		case "mode":
			m.Mode = val
		case "label":
			m.Label = val
		case "action":
			m.Action = val
		case "actor":
			m.Actor = val
		case "akey":
			m.AttemptKey = val
		}
	}

	return m
}

// ParseAll 从多行文本中解析所有 Marker
func ParseAll(text string) []*Marker {
	var markers []*Marker
	for _, line := range strings.Split(text, "\n") {
		if m := Parse(line); m != nil {
			markers = append(markers, m)
		}
	}
	return markers
}

// Find 在 markers 列表中查找指定类型和 issue 的所有 Marker
func Find(markers []*Marker, t Type, issue int) []*Marker {
	var result []*Marker
	for _, m := range markers {
		if m.Type == t && m.Issue == issue {
			result = append(result, m)
		}
	}
	return result
}

// FindLatest 在 markers 列表中查找指定类型和 issue 的最新 Marker（最高 revision）
func FindLatest(markers []*Marker, t Type, issue int) *Marker {
	var latest *Marker
	for _, m := range markers {
		if m.Type == t && m.Issue == issue {
			if latest == nil || m.Revision > latest.Revision {
				latest = m
			}
		}
	}
	return latest
}

// Render 将 Marker 渲染为 HTML 注释格式
func Render(m *Marker) string {
	parts := []string{fmt.Sprintf("<!-- %s", string(m.Type))}

	if m.Issue > 0 {
		parts = append(parts, fmt.Sprintf("issue=%d", m.Issue))
	}
	if m.Revision > 0 {
		parts = append(parts, fmt.Sprintf("rev=%d", m.Revision))
	}
	if m.PR > 0 {
		parts = append(parts, fmt.Sprintf("pr=%d", m.PR))
	}
	if m.Finish {
		parts = append(parts, "finish=1")
	}
	if m.Mode != "" {
		parts = append(parts, fmt.Sprintf("mode=%s", m.Mode))
	}
	if m.Label != "" {
		parts = append(parts, fmt.Sprintf("label=%s", m.Label))
	}
	if m.Action != "" {
		parts = append(parts, fmt.Sprintf("action=%s", m.Action))
	}
	if m.Actor != "" {
		parts = append(parts, fmt.Sprintf("actor=%s", m.Actor))
	}
	if m.AttemptKey != "" {
		parts = append(parts, fmt.Sprintf("akey=%s", m.AttemptKey))
	}

	return strings.Join(parts, " ") + " -->"
}

// StripMarkerLines 从文本中移除所有 marker HTML 注释行，返回干净内容
func StripMarkerLines(text string) string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		if !markerRe.MatchString(line) {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func isValidType(t Type) bool {
	for _, vt := range AllTypes {
		if t == vt {
			return true
		}
	}
	return false
}
