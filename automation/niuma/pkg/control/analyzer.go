// pkg/control/analyzer.go
// AI 依赖分析器：解析 issue 间依赖关系
package control

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
)

// dependsOnRe 匹配 issue body 中的 depends-on 声明
// 支持格式：depends-on: #40, #41 或 depends-on: #40 #41
var dependsOnRe = regexp.MustCompile(`(?i)depends[- ]on:\s*((?:#\d+[\s,]*)+)`)

// parentRe 匹配 issue body 中的 parent 声明（Sub-Issue 模式）
// 支持格式：parent: #40 或 parent issue: #40
var parentRe = regexp.MustCompile(`(?i)parent(?:\s+issue)?:\s*#(\d+)`)

var issueNumRe = regexp.MustCompile(`#(\d+)`)

// DependencyAnalyzer AI 分析 issue 间依赖
type DependencyAnalyzer struct {
	provider ai.Provider
}

// NewDependencyAnalyzer 创建分析器
func NewDependencyAnalyzer(provider ai.Provider) *DependencyAnalyzer {
	return &DependencyAnalyzer{provider: provider}
}

// Analyze 分析多个 issue 之间的依赖关系
// 优先使用 issue body 中的 depends-on 声明，其次调 AI 分析
func (a *DependencyAnalyzer) Analyze(ctx context.Context, issues []IssueInfo) (*AnalysisResult, error) {
	result := &AnalysisResult{
		Dependencies: make(map[int][]int),
	}

	// 收集所有 issue 编号（用于验证依赖有效性）
	issueSet := make(map[int]bool)
	for _, issue := range issues {
		issueSet[issue.Number] = true
	}

	// Phase 1: 解析 depends-on 人工声明
	declared := make(map[int]bool) // 仅用于保护显式声明，不允许 AI 覆盖
	for _, issue := range issues {
		deps := parseDependsOn(issue.Body)
		if len(deps) > 0 {
			// 过滤：只保留在当前 issue 集合中的依赖
			validDeps := filterIssueDeps(issue.Number, deps, issueSet)
			if len(validDeps) > 0 {
				result.Dependencies[issue.Number] = validDeps
			}
			// 只要写了 depends-on，就视为显式声明（即使最终过滤为空）
			// 这样可以保证 "显式 depends-on > AI 推断" 的优先级边界不被破坏。
			declared[issue.Number] = true
		}
	}

	// Phase 2: 对未声明依赖的 issue 调 AI 分析
	var undeclared []IssueInfo
	for _, issue := range issues {
		if !declared[issue.Number] {
			undeclared = append(undeclared, issue)
		}
	}

	if len(undeclared) > 0 && a.provider != nil {
		aiResult, err := a.aiAnalyze(ctx, issues, undeclared)
		if err != nil {
			// AI 失败降级：保留人工依赖结果，未声明项按独立处理。
			fmt.Printf("[control][analyzer] degraded=true stage=ai-dependency undeclared=%v err=%q\n", issueNumbers(undeclared), err)
		} else {
			// 合并 AI 结果（仅补全未显式声明 depends-on 的 issue）
			for issueNum, deps := range aiResult.Dependencies {
				// 仅接受当前 issue 集合内的 owner，避免 AI 幻觉 key 污染 blocked_by 落盘门禁。
				if !issueSet[issueNum] {
					continue
				}
				if declared[issueNum] {
					continue // 显式声明优先，AI 不允许覆盖
				}
				validDeps := filterIssueDeps(issueNum, deps, issueSet)
				if len(validDeps) > 0 {
					result.Dependencies[issueNum] = validDeps
				}
			}
			result.PotentialConflicts = aiResult.PotentialConflicts
		}
	}

	return result, nil
}

// filterIssueDeps 过滤非法依赖：只保留当前 issue 集合内、且非自引用的依赖。
func filterIssueDeps(issueNum int, deps []int, issueSet map[int]bool) []int {
	if len(deps) == 0 {
		return nil
	}
	validDeps := make([]int, 0, len(deps))
	seen := make(map[int]struct{}, len(deps))
	for _, d := range deps {
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		if issueSet[d] && d != issueNum {
			validDeps = append(validDeps, d)
		}
	}
	return validDeps
}

// issueNumbers 返回 issue 编号列表（升序），用于结构化日志。
func issueNumbers(issues []IssueInfo) []int {
	if len(issues) == 0 {
		return nil
	}
	nums := make([]int, 0, len(issues))
	for _, issue := range issues {
		nums = append(nums, issue.Number)
	}
	sort.Ints(nums)
	return nums
}

// parseDependsOn 从 issue body 解析 depends-on 声明
func parseDependsOn(body string) []int {
	matches := dependsOnRe.FindStringSubmatch(body)
	if len(matches) < 2 {
		return nil
	}

	numMatches := issueNumRe.FindAllStringSubmatch(matches[1], -1)
	var deps []int
	for _, m := range numMatches {
		n, err := strconv.Atoi(m[1])
		if err == nil {
			deps = append(deps, n)
		}
	}
	return deps
}

// aiAnalyze 调 AI 分析 issue 间依赖
func (a *DependencyAnalyzer) aiAnalyze(ctx context.Context, allIssues []IssueInfo, toAnalyze []IssueInfo) (*AnalysisResult, error) {
	prompt := buildAnalysisPrompt(allIssues, toAnalyze)
	resp, err := a.provider.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI 调用失败: %w", err)
	}

	return parseAnalysisResponse(resp)
}

// buildAnalysisPrompt 构建 AI 分析的 prompt
func buildAnalysisPrompt(allIssues []IssueInfo, toAnalyze []IssueInfo) string {
	var sb strings.Builder
	sb.WriteString("分析以下 GitHub issues 之间的依赖关系。\n\n")
	sb.WriteString("所有 issues:\n")
	for _, issue := range allIssues {
		// 截断 body 避免 token 过多
		body := issue.Body
		if len(body) > 500 {
			body = body[:500] + "..."
		}
		sb.WriteString(fmt.Sprintf("- #%d: %s\n  %s\n", issue.Number, issue.Title, body))
	}

	sb.WriteString("\n需要分析依赖的 issues: ")
	nums := make([]string, len(toAnalyze))
	for i, issue := range toAnalyze {
		nums[i] = fmt.Sprintf("#%d", issue.Number)
	}
	sb.WriteString(strings.Join(nums, ", "))

	sb.WriteString("\n\n请返回 JSON 格式（无其他文本）：\n")
	sb.WriteString(`{"dependencies": {"<issue_num>": [<dep_issue_nums>]}, "potential_conflicts": [[<issue_num1>, <issue_num2>]]}`)
	sb.WriteString("\n\n规则：\n")
	sb.WriteString("- dependencies: 如果 issue A 的实现依赖 issue B 先完成，则 A 依赖 B\n")
	sb.WriteString("- potential_conflicts: 如果两个 issue 可能修改同一文件/模块，标记为潜在冲突\n")
	sb.WriteString("- 没有依赖的 issue 不要出现在 dependencies 中\n")

	return sb.String()
}

// parseAnalysisResponse 解析 AI 返回的 JSON
func parseAnalysisResponse(resp string) (*AnalysisResult, error) {
	// 尝试提取 JSON（AI 可能包裹在 markdown code block 中）
	resp = extractJSON(resp)

	// 先解析为中间格式（key 是字符串）
	var raw struct {
		Dependencies       map[string][]int `json:"dependencies"`
		PotentialConflicts [][]int          `json:"potential_conflicts"`
	}
	if err := json.Unmarshal([]byte(resp), &raw); err != nil {
		return nil, fmt.Errorf("解析 AI 响应 JSON 失败: %w\nresponse: %s", err, resp)
	}

	result := &AnalysisResult{
		Dependencies:       make(map[int][]int),
		PotentialConflicts: raw.PotentialConflicts,
	}
	for k, v := range raw.Dependencies {
		n, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		result.Dependencies[n] = v
	}
	return result, nil
}

// extractJSON 从可能包含 markdown code block 的响应中提取 JSON
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// 去掉 ```json ... ``` 包裹
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) == 2 {
			s = lines[1]
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
	}
	return strings.TrimSpace(s)
}

// parseParent 解析 issue body 中的 parent 声明
// 返回 0 表示没有 parent
func parseParent(body string) int {
	matches := parentRe.FindStringSubmatch(body)
	if len(matches) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(matches[1])
	return n
}
