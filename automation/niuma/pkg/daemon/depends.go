// pkg/daemon/depends.go
// 解析 issue body 中的 depends-on 依赖声明
package daemon

import (
	"regexp"
	"strings"
)

// dependsOnRe 匹配 "depends-on: #123, #456" 或 "depends-on: owner/repo#789"
var dependsOnRe = regexp.MustCompile(`(?im)^depends-on:\s*(.+)$`)

// issueRefRe 匹配单个 issue 引用：#123 或 owner/repo#123
var issueRefRe = regexp.MustCompile(`(?:[\w\-]+/[\w\-]+)?#(\d+)`)

// ParseDependsOn 从 issue body 中提取所有 depends-on 引用
// 返回去重后的 issue 编号字符串列表
func ParseDependsOn(body string) []string {
	if body == "" {
		return nil
	}

	seen := make(map[string]bool)
	var result []string

	for _, match := range dependsOnRe.FindAllStringSubmatch(body, -1) {
		refs := match[1]
		for _, refMatch := range issueRefRe.FindAllStringSubmatch(refs, -1) {
			num := strings.TrimSpace(refMatch[1])
			if num != "" && !seen[num] {
				seen[num] = true
				result = append(result, num)
			}
		}
	}

	return result
}
