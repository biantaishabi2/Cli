package state

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	issueKeywordPattern = regexp.MustCompile(`(?i)\b(?:issue|closes?|closed|fixes?|fixed|resolves?|resolved|refs?)\s*#([0-9]+)\b`)
	subPattern          = regexp.MustCompile(`(?i)\bsub\(\s*#([0-9]+)\s*\)`)
	parentPattern       = regexp.MustCompile(`(?i)\bparent\(\s*#([0-9]+)\s*\)`)
	parenPattern        = regexp.MustCompile(`\(\s*#([0-9]+)\s*\)`)
	pullRequestPattern  = regexp.MustCompile(`(?i)pull request #([0-9]+)`)
)

// ParseIssueRefs 从多段文本中提取 issue 引用，结果去重并升序。
func ParseIssueRefs(texts ...string) []int {
	unique := make(map[int]struct{})
	for _, text := range texts {
		cleaned := pullRequestPattern.ReplaceAllString(text, "")
		collectIssueRefs(unique, cleaned, issueKeywordPattern)
		collectIssueRefs(unique, cleaned, subPattern)
		collectIssueRefs(unique, cleaned, parentPattern)
		collectIssueRefs(unique, cleaned, parenPattern)
	}
	result := make([]int, 0, len(unique))
	for issueNum := range unique {
		result = append(result, issueNum)
	}
	sort.Ints(result)
	return result
}

// ParseIssueRefsFromPR 从 PR 标题、正文和 commit message 中提取 issue 引用。
func ParseIssueRefsFromPR(prTitle, prBody string, messages []string) []int {
	texts := make([]string, 0, len(messages)+2)
	if strings.TrimSpace(prTitle) != "" {
		texts = append(texts, prTitle)
	}
	if strings.TrimSpace(prBody) != "" {
		texts = append(texts, prBody)
	}
	texts = append(texts, messages...)
	return ParseIssueRefs(texts...)
}

func collectIssueRefs(dst map[int]struct{}, text string, pattern *regexp.Regexp) {
	matches := pattern.FindAllStringSubmatch(text, -1)
	for _, groups := range matches {
		if len(groups) < 2 {
			continue
		}
		num, err := strconv.Atoi(groups[1])
		if err != nil || num <= 0 {
			continue
		}
		dst[num] = struct{}{}
	}
}
