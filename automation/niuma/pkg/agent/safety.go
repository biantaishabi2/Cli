// pkg/agent/safety.go
// 安全校验：路径白名单、高风险变更检测
package agent

import (
	"fmt"
	"path/filepath"
	"strings"
)

// defaultAllowedPrefixes 默认允许修改的目录前缀
var defaultAllowedPrefixes = []string{
	"src/",
	"lib/",
	"pkg/",
	"app/",
	"cmd/",
	"internal/",
	"tests/",
	"test/",
	"spec/",
}

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

// ValidationResult 路径校验结果
type ValidationResult struct {
	Allowed  []string // 允许的路径
	Rejected []string // 被拒绝的路径
	HighRisk []string // 高风险但允许的路径
}

// IsClean 是否全部通过校验（无拒绝项）
func (r *ValidationResult) IsClean() bool {
	return len(r.Rejected) == 0
}

// ValidateChanges 校验文件变更路径，extraPrefixes 为额外允许的目录前缀
func ValidateChanges(changes []FileChange, extraPrefixes ...string) *ValidationResult {
	result := &ValidationResult{}

	for _, c := range changes {
		// 先检查原始路径中的 .. （防止 filepath.Clean 将 src/../etc/passwd 解析为 etc/passwd）
		if strings.Contains(c.Path, "..") {
			result.Rejected = append(result.Rejected, c.Path)
			continue
		}

		path := filepath.Clean(c.Path)

		if IsHighRiskChange(path) {
			result.HighRisk = append(result.HighRisk, path)
			// 高风险路径仍然允许（只是标记）
			result.Allowed = append(result.Allowed, path)
			continue
		}

		if isAllowedPath(path, extraPrefixes) {
			result.Allowed = append(result.Allowed, path)
		} else {
			result.Rejected = append(result.Rejected, path)
		}
	}

	return result
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

func isAllowedPath(path string, extraPrefixes []string) bool {
	for _, prefix := range defaultAllowedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	for _, prefix := range extraPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// FormatHighRiskWarning 格式化高风险路径警告
func FormatHighRiskWarning(result *ValidationResult) string {
	if len(result.HighRisk) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## ⚠️ 高风险路径警告\n\n以下 %d 个路径需要人工审查：\n\n", len(result.HighRisk)))
	for _, p := range result.HighRisk {
		sb.WriteString(fmt.Sprintf("- `%s`\n", p))
	}
	return sb.String()
}

// FormatValidationError 格式化校验错误信息
func FormatValidationError(result *ValidationResult) string {
	if result.IsClean() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 路径校验失败\n\n")
	sb.WriteString(fmt.Sprintf("以下 %d 个路径不在允许列表中：\n\n", len(result.Rejected)))
	for _, p := range result.Rejected {
		sb.WriteString(fmt.Sprintf("- `%s`\n", p))
	}

	if len(result.HighRisk) > 0 {
		sb.WriteString(fmt.Sprintf("\n以下 %d 个高风险路径需要人工审查：\n\n", len(result.HighRisk)))
		for _, p := range result.HighRisk {
			sb.WriteString(fmt.Sprintf("- ⚠️ `%s`\n", p))
		}
	}

	return sb.String()
}
