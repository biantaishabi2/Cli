// pkg/agent/safety.go
// 安全校验：高风险变更检测、计划 vs 实际 diff 对比
package agent

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// 高风险路径（需要额外审查）
var highRiskPrefixes = []string{
	"auth/",
	"infra/",
	"deploy/",
	".github/",
}

var highRiskFiles = []string{
	"Dockerfile",
	"docker-compose.yml",
	"Makefile",
	".env",
	".env.example",
}

// IsHighRiskChange 检测是否为高风险变更路径
func IsHighRiskChange(path string) bool {
	for _, prefix := range highRiskPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	for _, name := range highRiskFiles {
		// 匹配文件名（可能在子目录下）
		if path == name || strings.HasSuffix(path, "/"+name) {
			return true
		}
	}
	return false
}

// FormatHighRiskWarning 格式化高风险路径警告
func FormatHighRiskWarning(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## ⚠️ 高风险路径警告\n\n以下 %d 个路径需要人工审查：\n\n", len(paths)))
	for _, p := range paths {
		sb.WriteString(fmt.Sprintf("- `%s`\n", p))
	}
	return sb.String()
}

// DiffCheckResult diff 对比结果
type DiffCheckResult struct {
	Extra   []string // 实际改了但不在计划中的文件
	Missing []string // 计划中但实际没改的文件
}

func (r *DiffCheckResult) IsClean() bool {
	return len(r.Extra) == 0 && len(r.Missing) == 0
}

// ContainsPathTraversal 检查路径是否包含 .. 路径遍历
func ContainsPathTraversal(path string) bool {
	return strings.Contains(path, "..")
}

// CheckDiffAgainstPlan 对比实际 diff 和计划声明的文件
// 路径统一用 filepath.Clean 规范化，处理 ./ 前缀等格式差异
// 包含 .. 的路径会被标记为 Extra（路径遍历防护）
// plannedChanges 为空时跳过对比（避免空计划触发假警告）
func CheckDiffAgainstPlan(actualFiles []string, plannedChanges []FileChange) *DiffCheckResult {
	if len(plannedChanges) == 0 {
		return &DiffCheckResult{} // 空计划不检查
	}
	planned := make(map[string]bool)
	for _, fc := range plannedChanges {
		// 路径遍历防护：包含 .. 的计划路径直接跳过（不加入白名单）
		if ContainsPathTraversal(fc.Path) {
			continue
		}
		planned[filepath.Clean(fc.Path)] = true
	}
	actual := make(map[string]bool)
	for _, f := range actualFiles {
		actual[filepath.Clean(f)] = true
	}
	result := &DiffCheckResult{}
	for f := range actual {
		if !planned[f] {
			result.Extra = append(result.Extra, f)
		}
	}
	for p := range planned {
		if !actual[p] {
			result.Missing = append(result.Missing, p)
		}
	}
	// 排序保证输出稳定（方便测试）
	sort.Strings(result.Extra)
	sort.Strings(result.Missing)
	return result
}

// FormatDiffCheckComment 格式化为 PR comment，用表格展示计划 vs 实际
func FormatDiffCheckComment(result *DiffCheckResult) string {
	if result.IsClean() {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## 📋 计划 vs 实际 diff 对比\n\n")
	sb.WriteString("| 文件 | 状态 |\n|------|------|\n")
	for _, f := range result.Extra {
		sb.WriteString(fmt.Sprintf("| `%s` | ⚠️ 不在计划内 |\n", f))
	}
	for _, f := range result.Missing {
		sb.WriteString(fmt.Sprintf("| `%s` | ⚠️ 计划中但未修改 |\n", f))
	}
	return sb.String()
}

// planFilesRe 匹配嵌入在 HTML comment 中的 FileChanges JSON
var planFilesRe = regexp.MustCompile(`<!-- PLAN_FILES:(.*?) -->`)

// ParseFileChangesFromComment 从 marker comment body 中提取嵌入的 FileChanges
func ParseFileChangesFromComment(body string) []FileChange {
	m := planFilesRe.FindStringSubmatch(body)
	if len(m) < 2 {
		return nil
	}
	var changes []FileChange
	if err := json.Unmarshal([]byte(m[1]), &changes); err != nil {
		return nil
	}
	return changes
}
