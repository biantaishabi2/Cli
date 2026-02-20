package control

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

var (
	// ErrUnknownProfile 表示冲突文件未命中任何已注册 profile。
	ErrUnknownProfile = errors.New("unknown conflict profile")

	defaultConflictProfiles = []ConflictProfile{
		newSuffixConflictProfileWithRule("go", []string{".go"}, "仅处理 Go 语法，优先最小化语义改动并保持 import 结构稳定。", resolveGoImportConflictFile),
		newSuffixConflictProfile("elixir", []string{".ex", ".exs"}, "仅处理 Elixir 语法，保持 module/do-end 结构及模式匹配语义。"),
		newSuffixConflictProfileWithRule("rust", []string{".rs"}, "仅处理 Rust 语法，保持 ownership/borrow 语义与类型约束。", resolveRustUseConflictFile),
	}
)

// ConflictPromptFile 表示构造 profile prompt 所需的文件上下文。
type ConflictPromptFile struct {
	Path    string
	Base    string
	Content string
	Summary ConflictFileSummary
}

// ConflictProfile 定义语言 profile 的匹配与 prompt 构造能力。
type ConflictProfile interface {
	Name() string
	Match(path string) bool
	BuildPrompt(files []ConflictPromptFile) (string, error)
}

// RuleResolver 表示 profile 可选的 Rule 层冲突修复能力。
type RuleResolver interface {
	TryResolveByRule(repoDir string, relPath string) error
}

// ConflictProfileGroup 表示同一 profile 命中的冲突文件集合。
type ConflictProfileGroup struct {
	Profile ConflictProfile
	Files   []string
}

func registeredConflictProfiles() []ConflictProfile {
	profiles := make([]ConflictProfile, len(defaultConflictProfiles))
	copy(profiles, defaultConflictProfiles)
	return profiles
}

// ResolveConflictProfile 根据文件路径解析匹配的冲突 profile。
func ResolveConflictProfile(path string) (ConflictProfile, error) {
	for _, profile := range registeredConflictProfiles() {
		if profile.Match(path) {
			return profile, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrUnknownProfile, path)
}

// ResolveConflictProfileGroups 将冲突文件按 profile 分组并按名称稳定排序。
func ResolveConflictProfileGroups(conflictFiles []string) ([]ConflictProfileGroup, error) {
	indexByProfile := make(map[string]int)
	groups := make([]ConflictProfileGroup, 0)

	for _, file := range conflictFiles {
		profile, err := ResolveConflictProfile(file)
		if err != nil {
			return nil, err
		}

		name := profile.Name()
		idx, exists := indexByProfile[name]
		if !exists {
			idx = len(groups)
			indexByProfile[name] = idx
			groups = append(groups, ConflictProfileGroup{
				Profile: profile,
				Files:   []string{},
			})
		}
		groups[idx].Files = append(groups[idx].Files, file)
	}

	for i := range groups {
		sort.Strings(groups[i].Files)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return groups[i].Profile.Name() < groups[j].Profile.Name()
	})
	return groups, nil
}

type suffixConflictProfile struct {
	name        string
	suffixes    map[string]struct{}
	profileHint string
}

func newSuffixConflictProfile(name string, suffixes []string, profileHint string) ConflictProfile {
	normalized := make(map[string]struct{}, len(suffixes))
	for _, suffix := range suffixes {
		normalized[strings.ToLower(strings.TrimSpace(suffix))] = struct{}{}
	}
	return &suffixConflictProfile{
		name:        strings.TrimSpace(name),
		suffixes:    normalized,
		profileHint: strings.TrimSpace(profileHint),
	}
}

// suffixRuleResolverProfile 包装 suffixConflictProfile 并添加 RuleResolver 能力。
// 仅 Go/Rust 等注册了 ruleResolverFn 的 profile 使用此包装，
// Elixir 等不注册的 profile 不实现 RuleResolver 接口。
type suffixRuleResolverProfile struct {
	*suffixConflictProfile
	resolverFn func(repoDir, relPath string) error
}

func newSuffixConflictProfileWithRule(name string, suffixes []string, profileHint string, resolverFn func(repoDir, relPath string) error) ConflictProfile {
	base := newSuffixConflictProfile(name, suffixes, profileHint).(*suffixConflictProfile)
	return &suffixRuleResolverProfile{
		suffixConflictProfile: base,
		resolverFn:            resolverFn,
	}
}

// TryResolveByRule 通过注册的 resolverFn 执行 Rule 层冲突修复。
func (p *suffixRuleResolverProfile) TryResolveByRule(repoDir string, relPath string) error {
	if p.resolverFn == nil {
		return fmt.Errorf("profile %s: no rule resolver registered", p.Name())
	}
	return p.resolverFn(repoDir, relPath)
}

func (p *suffixConflictProfile) Name() string {
	return p.name
}

func (p *suffixConflictProfile) Match(path string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	_, ok := p.suffixes[ext]
	return ok
}

// ParseProfileFlag 解析 --profile 标志值，返回 (mode, langs)。
// mode 为 "auto"/"none"/"whitelist"；当 mode="whitelist" 时 langs 为非空语言列表。
func ParseProfileFlag(raw string) (string, []string) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "auto" {
		return "auto", nil
	}
	if raw == "none" {
		return "none", nil
	}
	parts := strings.Split(raw, ",")
	langs := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			langs = append(langs, p)
		}
	}
	if len(langs) == 0 {
		return "auto", nil
	}
	return "whitelist", langs
}

// FilterConflictProfileGroups 根据白名单过滤 profile groups，
// 仅保留 Profile.Name() 在 whitelist 中的 group。
func FilterConflictProfileGroups(groups []ConflictProfileGroup, whitelist []string) []ConflictProfileGroup {
	if len(whitelist) == 0 {
		return groups
	}
	allowed := make(map[string]struct{}, len(whitelist))
	for _, lang := range whitelist {
		allowed[strings.TrimSpace(strings.ToLower(lang))] = struct{}{}
	}
	filtered := make([]ConflictProfileGroup, 0, len(groups))
	for _, g := range groups {
		if _, ok := allowed[strings.ToLower(g.Profile.Name())]; ok {
			filtered = append(filtered, g)
		}
	}
	return filtered
}

func (p *suffixConflictProfile) BuildPrompt(files []ConflictPromptFile) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("profile %s 没有可处理的冲突文件", p.name)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("你是 %s 冲突修复助手。\n", p.name))
	if p.profileHint != "" {
		sb.WriteString("语言策略：")
		sb.WriteString(p.profileHint)
		sb.WriteString("\n")
	}
	sb.WriteString("仅处理本段提供的文件，不要生成额外说明。\n\n")

	for _, file := range files {
		sb.WriteString(fmt.Sprintf("### file: %s\n", file.Path))
		sb.WriteString("[base side]\n")
		sb.WriteString(file.Base)
		sb.WriteString("\n[conflict file content]\n")
		sb.WriteString(file.Content)
		sb.WriteString("\n[conflict hunks]\n")
		for idx, block := range file.Summary.Blocks {
			sb.WriteString(fmt.Sprintf("hunk-%d ours:\n%s\n", idx+1, block.Ours))
			sb.WriteString(fmt.Sprintf("hunk-%d theirs:\n%s\n", idx+1, block.Theirs))
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}
