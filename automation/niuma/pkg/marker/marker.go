// pkg/marker/marker.go
// 幂等 Marker：解析和渲染 HTML 注释标记，用于跟踪机器人操作状态
package marker

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Type 定义 Marker 的五种类型
type Type string

const (
	TypePlanDraft         Type = "BOT:PLAN_DRAFT"
	TypeDiscussionSummary Type = "BOT:DISCUSSION_SUMMARY"
	TypePlanFinal         Type = "BOT:PLAN_FINAL"
	TypePRCreated         Type = "BOT:PR_CREATED"
	TypeConvergeWarning   Type = "BOT:CONVERGE_WARNING"
)

// AllTypes 返回所有有效的 Marker 类型
var AllTypes = []Type{
	TypePlanDraft,
	TypeDiscussionSummary,
	TypePlanFinal,
	TypePRCreated,
	TypeConvergeWarning,
}

// Marker 表示一个嵌入在 GitHub 评论中的幂等标记
type Marker struct {
	Type     Type
	Issue    int
	Revision int
	PR       int // 仅 PR_CREATED 使用
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
