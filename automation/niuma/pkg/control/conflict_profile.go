package control

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	conflictProfileUnknown = "unknown"
	conflictProfileGo      = "go"
	conflictProfileElixir  = "elixir"
	conflictProfileRust    = "rust"

	defaultPRConflictMaxHunks      = 3
	defaultPRConflictMaxHunkLines  = 15
	defaultPRConflictMaxTotalLines = 50

	defaultGoProfileGateCommand     = "go test {pkg}"
	defaultElixirProfileGateCommand = "mix test {scope}"
	defaultRustProfileGateCommand   = "cargo test --manifest-path {path}"
)

// ConflictProfile 定义语言 profile 的最小契约。
type ConflictProfile interface {
	Name() string
	Detect(repoDir, relPath string) bool
	Allow(relPath string, summary conflictFileSummary, threshold PRConflictThresholdConfig) (bool, string)
	ApplyRuleFix(repoDir, relPath string) (bool, error)
	PromptAddon() string
	GateCommands(repoDir string, files []string, cfg PRConflictProfileConfig) ([]profileGateCommand, error)
}

type profileGateCommand struct {
	Dir     string
	Command string
}

type conflictProfileGroup struct {
	Name    string
	Profile ConflictProfile
	Files   []string
}

// ProfileRegistry 用于管理 profile 注册与识别。
type ProfileRegistry struct {
	profiles map[string]ConflictProfile
}

// NewProfileRegistry 创建空 profile 注册表。
func NewProfileRegistry() *ProfileRegistry {
	return &ProfileRegistry{profiles: make(map[string]ConflictProfile)}
}

func defaultConflictProfileRegistry() *ProfileRegistry {
	registry := NewProfileRegistry()
	registry.Register(goConflictProfile{})
	registry.Register(elixirConflictProfile{})
	registry.Register(rustConflictProfile{})
	return registry
}

// Register 注册 profile。
func (r *ProfileRegistry) Register(profile ConflictProfile) {
	if r == nil || profile == nil {
		return
	}
	r.profiles[profile.Name()] = profile
}

func (r *ProfileRegistry) Profile(name string) (ConflictProfile, bool) {
	if r == nil {
		return nil, false
	}
	profile, ok := r.profiles[strings.TrimSpace(name)]
	return profile, ok
}

// DetectByFile 按“后缀优先、目录标记兜底”识别 profile。
func (r *ProfileRegistry) DetectByFile(repoDir, relPath string) string {
	ext := strings.ToLower(filepath.Ext(relPath))
	switch ext {
	case ".go":
		return conflictProfileGo
	case ".ex", ".exs":
		return conflictProfileElixir
	case ".rs":
		return conflictProfileRust
	}
	return detectProfileByProjectMarker(repoDir, relPath)
}

func detectProfileByProjectMarker(repoDir, relPath string) string {
	start := filepath.Join(repoDir, filepath.Dir(relPath))
	if strings.TrimSpace(start) == "" {
		start = repoDir
	}

	for dir := start; dir != ""; dir = filepath.Dir(dir) {
		for _, marker := range []struct {
			filename string
			profile  string
		}{
			{filename: "go.mod", profile: conflictProfileGo},
			{filename: "mix.exs", profile: conflictProfileElixir},
			{filename: "Cargo.toml", profile: conflictProfileRust},
		} {
			if _, err := os.Stat(filepath.Join(dir, marker.filename)); err == nil {
				return marker.profile
			}
		}
		if samePath(dir, repoDir) {
			break
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
	}
	return conflictProfileUnknown
}

func groupConflictFilesByProfile(repoDir string, files []string, registry *ProfileRegistry) ([]conflictProfileGroup, []string) {
	if registry == nil {
		registry = defaultConflictProfileRegistry()
	}

	groupIndex := make(map[string]int)
	groups := make([]conflictProfileGroup, 0)
	unknown := make([]string, 0)

	for _, file := range files {
		profileName := registry.DetectByFile(repoDir, file)
		if profileName == conflictProfileUnknown {
			unknown = append(unknown, file)
			continue
		}

		profile, ok := registry.Profile(profileName)
		if !ok || !profile.Detect(repoDir, file) {
			unknown = append(unknown, file)
			continue
		}

		idx, exists := groupIndex[profileName]
		if !exists {
			idx = len(groups)
			groupIndex[profileName] = idx
			groups = append(groups, conflictProfileGroup{
				Name:    profileName,
				Profile: profile,
				Files:   make([]string, 0),
			})
		}
		groups[idx].Files = append(groups[idx].Files, file)
	}

	for i := range groups {
		sort.Strings(groups[i].Files)
	}
	sort.Strings(unknown)
	return groups, unknown
}

func summarizeConflictProfileGroups(groups []conflictProfileGroup) string {
	if len(groups) == 0 {
		return ""
	}
	if len(groups) == 1 {
		return groups[0].Name
	}
	parts := make([]string, 0, len(groups))
	for _, group := range groups {
		parts = append(parts, fmt.Sprintf("%s:%d", group.Name, len(group.Files)))
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func formatConflictFileList(files []string) string {
	items := append([]string(nil), files...)
	sort.Strings(items)
	return strings.Join(items, ",")
}

func defaultPRConflictResolutionConfig() PRConflictResolutionConfig {
	threshold := PRConflictThresholdConfig{
		MaxHunks:      defaultPRConflictMaxHunks,
		MaxHunkLines:  defaultPRConflictMaxHunkLines,
		MaxTotalLines: defaultPRConflictMaxTotalLines,
	}
	return PRConflictResolutionConfig{
		Profiles: map[string]PRConflictProfileConfig{
			conflictProfileGo: {
				Enabled:     true,
				GateCommand: defaultGoProfileGateCommand,
				Threshold:   threshold,
			},
			conflictProfileElixir: {
				Enabled:     true,
				GateCommand: defaultElixirProfileGateCommand,
				Threshold:   threshold,
			},
			conflictProfileRust: {
				Enabled:     true,
				GateCommand: defaultRustProfileGateCommand,
				Threshold:   threshold,
			},
		},
	}
}

func normalizePRConflictResolutionConfig(cfg PRConflictResolutionConfig) PRConflictResolutionConfig {
	defaults := defaultPRConflictResolutionConfig()
	if len(cfg.Profiles) == 0 {
		return defaults
	}

	normalized := PRConflictResolutionConfig{Profiles: make(map[string]PRConflictProfileConfig, len(defaults.Profiles))}
	for profileName, defaultCfg := range defaults.Profiles {
		candidate := defaultCfg
		if userCfg, ok := cfg.Profiles[profileName]; ok {
			candidate.Enabled = userCfg.Enabled
			if strings.TrimSpace(userCfg.GateCommand) != "" {
				candidate.GateCommand = strings.TrimSpace(userCfg.GateCommand)
			}
			if userCfg.Threshold.MaxHunks > 0 {
				candidate.Threshold.MaxHunks = userCfg.Threshold.MaxHunks
			}
			if userCfg.Threshold.MaxHunkLines > 0 {
				candidate.Threshold.MaxHunkLines = userCfg.Threshold.MaxHunkLines
			}
			if userCfg.Threshold.MaxTotalLines > 0 {
				candidate.Threshold.MaxTotalLines = userCfg.Threshold.MaxTotalLines
			}
		}
		candidate.Threshold = normalizePRConflictThresholdConfig(candidate.Threshold)
		normalized.Profiles[profileName] = candidate
	}
	return normalized
}

func normalizePRConflictThresholdConfig(cfg PRConflictThresholdConfig) PRConflictThresholdConfig {
	if cfg.MaxHunks <= 0 {
		cfg.MaxHunks = defaultPRConflictMaxHunks
	}
	if cfg.MaxHunkLines <= 0 {
		cfg.MaxHunkLines = defaultPRConflictMaxHunkLines
	}
	if cfg.MaxTotalLines <= 0 {
		cfg.MaxTotalLines = defaultPRConflictMaxTotalLines
	}
	return cfg
}

func lookupPRConflictProfileConfig(cfg PRConflictResolutionConfig, profileName string) PRConflictProfileConfig {
	normalized := normalizePRConflictResolutionConfig(cfg)
	if profileCfg, ok := normalized.Profiles[profileName]; ok {
		return profileCfg
	}
	threshold := PRConflictThresholdConfig{
		MaxHunks:      defaultPRConflictMaxHunks,
		MaxHunkLines:  defaultPRConflictMaxHunkLines,
		MaxTotalLines: defaultPRConflictMaxTotalLines,
	}
	return PRConflictProfileConfig{Enabled: false, Threshold: threshold}
}

func conflictMetadataFromGroups(groups []conflictProfileGroup, files []string) ConflictResolutionMeta {
	return ConflictResolutionMeta{
		Profile:    summarizeConflictProfileGroups(groups),
		Files:      append([]string(nil), files...),
		GatePassed: false,
	}
}

func allowConflictByThreshold(summary conflictFileSummary, threshold PRConflictThresholdConfig) (bool, string) {
	if summary.hunks == 0 || len(summary.blocks) == 0 {
		return false, "未识别到可处理的文本冲突块"
	}
	if summary.hunks > threshold.MaxHunks {
		return false, fmt.Sprintf("冲突块过多: %d", summary.hunks)
	}

	totalLines := 0
	for _, block := range summary.blocks {
		ours := countConflictSideLines(block.ours)
		theirs := countConflictSideLines(block.theirs)
		if ours > threshold.MaxHunkLines || theirs > threshold.MaxHunkLines {
			return false, fmt.Sprintf("单块冲突行数过多: ours=%d theirs=%d", ours, theirs)
		}
		totalLines += ours + theirs
	}
	if totalLines > threshold.MaxTotalLines {
		return false, fmt.Sprintf("冲突行数过多: %d", totalLines)
	}
	return true, ""
}

func hasProfileHighRiskSignal(file string, summary conflictFileSummary) (bool, string) {
	fileLower := strings.ToLower(file)
	if strings.Contains(fileLower, "migration") || strings.HasSuffix(fileLower, ".sql") {
		return true, "检测到迁移脚本冲突"
	}
	for _, block := range summary.blocks {
		if containsCoreInterfaceSignal(block.ours + "\n" + block.theirs) {
			return true, "检测到核心接口语义变更信号"
		}
	}
	return false, ""
}

func isAdjacentMildConflictWithLimit(fileSummary conflictFileSummary, maxLines int) bool {
	if fileSummary.hunks > 2 {
		return false
	}
	for _, block := range fileSummary.blocks {
		if countConflictSideLines(block.ours) > maxLines {
			return false
		}
		if countConflictSideLines(block.theirs) > maxLines {
			return false
		}
	}
	return true
}

func isLightweightGoTestConflictWithLimit(file string, fileSummary conflictFileSummary, maxLines int) bool {
	if !strings.HasSuffix(strings.ToLower(file), "_test.go") {
		return false
	}
	if fileSummary.hunks > 2 {
		return false
	}
	for _, block := range fileSummary.blocks {
		if countConflictSideLines(block.ours) > maxLines {
			return false
		}
		if countConflictSideLines(block.theirs) > maxLines {
			return false
		}
	}
	return true
}

type goConflictProfile struct{}

func (goConflictProfile) Name() string {
	return conflictProfileGo
}

func (goConflictProfile) Detect(repoDir string, relPath string) bool {
	if strings.EqualFold(filepath.Ext(relPath), ".go") {
		return true
	}
	return detectProfileByProjectMarker(repoDir, relPath) == conflictProfileGo
}

func (goConflictProfile) Allow(relPath string, summary conflictFileSummary, threshold PRConflictThresholdConfig) (bool, string) {
	if ok, reason := allowConflictByThreshold(summary, threshold); !ok {
		return false, reason
	}
	if risky, reason := hasProfileHighRiskSignal(relPath, summary); risky {
		return false, reason
	}
	if isGoImportConflictOnly(summary) {
		return true, "go import 冲突"
	}
	if isLightweightGoTestConflictWithLimit(relPath, summary, threshold.MaxHunkLines) {
		return true, "测试辅助代码轻度并合"
	}
	if isAdjacentMildConflictWithLimit(summary, threshold.MaxHunkLines) {
		return true, "轻度相邻块冲突"
	}
	return false, "未命中 Go 白名单冲突类型"
}

func (goConflictProfile) ApplyRuleFix(repoDir, relPath string) (bool, error) {
	if strings.ToLower(filepath.Ext(relPath)) != ".go" {
		return false, fmt.Errorf("Rule 层仅支持 Go import 冲突，发现非 Go 文件: %s", relPath)
	}
	if err := resolveGoImportConflictFile(repoDir, relPath); err != nil {
		return false, err
	}
	return true, nil
}

func (goConflictProfile) PromptAddon() string {
	return strings.TrimSpace(`
[Profile: Go]
- 优先处理 import 语句冲突，保留并集合并、去重与稳定排序。
- 保持 gofmt 兼容，不引入无关语义修改。
- 禁止删除非冲突区域的函数、类型、接口定义。
`)
}

func (goConflictProfile) GateCommands(repoDir string, files []string, cfg PRConflictProfileConfig) ([]profileGateCommand, error) {
	template := strings.TrimSpace(cfg.GateCommand)
	if template == "" {
		template = defaultGoProfileGateCommand
	}
	targets := collectGoTestTargets(repoDir, files)
	if len(targets) == 0 {
		return nil, fmt.Errorf("未找到 Go 测试目标")
	}

	commands := make([]profileGateCommand, 0, len(targets))
	for _, target := range targets {
		command := renderGateCommandTemplate(template, map[string]string{
			"pkg":      target.pkgPattern,
			"scope":    target.pkgPattern,
			"path":     target.pkgPattern,
			"manifest": filepath.ToSlash(filepath.Join(target.moduleRoot, "go.mod")),
		})
		if strings.TrimSpace(command) == "" {
			continue
		}
		commands = append(commands, profileGateCommand{Dir: target.moduleRoot, Command: command})
	}
	return commands, nil
}

type elixirConflictProfile struct{}

func (elixirConflictProfile) Name() string {
	return conflictProfileElixir
}

func (elixirConflictProfile) Detect(repoDir, relPath string) bool {
	ext := strings.ToLower(filepath.Ext(relPath))
	if ext == ".ex" || ext == ".exs" {
		return true
	}
	return detectProfileByProjectMarker(repoDir, relPath) == conflictProfileElixir
}

func (elixirConflictProfile) Allow(_ string, summary conflictFileSummary, threshold PRConflictThresholdConfig) (bool, string) {
	if ok, reason := allowConflictByThreshold(summary, threshold); !ok {
		return false, reason
	}
	if isElixirDirectiveConflictOnly(summary) {
		return true, "elixir alias/import/use 轻度冲突"
	}
	return false, "未命中 Elixir 白名单冲突类型"
}

func (elixirConflictProfile) ApplyRuleFix(repoDir, relPath string) (bool, error) {
	if err := resolveElixirDirectiveConflictFile(repoDir, relPath); err != nil {
		return false, err
	}
	return true, nil
}

func (elixirConflictProfile) PromptAddon() string {
	return strings.TrimSpace(`
[Profile: Elixir]
- 仅做 alias/import/use 冲突并合，保持最小改动。
- 保持模块结构、do/end 配对与缩进风格。
- 禁止跨文件重构与宏语义重写。
`)
}

func (elixirConflictProfile) GateCommands(repoDir string, files []string, cfg PRConflictProfileConfig) ([]profileGateCommand, error) {
	template := strings.TrimSpace(cfg.GateCommand)
	if template == "" {
		template = defaultElixirProfileGateCommand
	}
	roots := collectProjectMarkerRoots(repoDir, files, "mix.exs")
	if len(roots) == 0 {
		return nil, fmt.Errorf("未找到 Elixir 项目根目录 mix.exs")
	}

	commands := make([]profileGateCommand, 0, len(roots))
	for _, root := range roots {
		scope := deriveElixirGateScope(root, repoDir, files)
		command := renderGateCommandTemplate(template, map[string]string{
			"scope": scope,
			"path":  scope,
		})
		if strings.TrimSpace(command) == "" {
			continue
		}
		commands = append(commands, profileGateCommand{Dir: root, Command: command})
	}
	return commands, nil
}

type rustConflictProfile struct{}

func (rustConflictProfile) Name() string {
	return conflictProfileRust
}

func (rustConflictProfile) Detect(repoDir, relPath string) bool {
	if strings.EqualFold(filepath.Ext(relPath), ".rs") {
		return true
	}
	return detectProfileByProjectMarker(repoDir, relPath) == conflictProfileRust
}

func (rustConflictProfile) Allow(_ string, summary conflictFileSummary, threshold PRConflictThresholdConfig) (bool, string) {
	if ok, reason := allowConflictByThreshold(summary, threshold); !ok {
		return false, reason
	}
	if isRustDirectiveConflictOnly(summary) {
		return true, "rust use/mod 轻度冲突"
	}
	return false, "未命中 Rust 白名单冲突类型"
}

func (rustConflictProfile) ApplyRuleFix(repoDir, relPath string) (bool, error) {
	if err := resolveRustDirectiveConflictFile(repoDir, relPath); err != nil {
		return false, err
	}
	return true, nil
}

func (rustConflictProfile) PromptAddon() string {
	return strings.TrimSpace(`
[Profile: Rust]
- 仅做 use/mod 冲突并合，保持最小改动。
- 保持 Rust 语法完整，不改函数签名与 trait 语义。
- 禁止新增跨模块重构与非冲突区域修改。
`)
}

func (rustConflictProfile) GateCommands(repoDir string, files []string, cfg PRConflictProfileConfig) ([]profileGateCommand, error) {
	template := strings.TrimSpace(cfg.GateCommand)
	if template == "" {
		template = defaultRustProfileGateCommand
	}
	roots := collectProjectMarkerRoots(repoDir, files, "Cargo.toml")
	if len(roots) == 0 {
		return nil, fmt.Errorf("未找到 Rust 项目根目录 Cargo.toml")
	}

	commands := make([]profileGateCommand, 0, len(roots))
	for _, root := range roots {
		manifest := "Cargo.toml"
		pkgName, err := readCargoPackageName(filepath.Join(root, manifest))
		if err != nil {
			return nil, err
		}
		if templateContainsToken(template, "pkg") && strings.TrimSpace(pkgName) == "" {
			return nil, fmt.Errorf("Rust gate 命令需要 {pkg}，但未解析到包名: %s", root)
		}

		command := renderGateCommandTemplate(template, map[string]string{
			"pkg":      pkgName,
			"path":     manifest,
			"manifest": manifest,
		})
		if strings.TrimSpace(command) == "" {
			continue
		}
		commands = append(commands, profileGateCommand{Dir: root, Command: command})
	}
	return commands, nil
}

func isElixirDirectiveConflictOnly(fileSummary conflictFileSummary) bool {
	for _, block := range fileSummary.blocks {
		if !isElixirDirectiveSide(block.ours) || !isElixirDirectiveSide(block.theirs) {
			return false
		}
	}
	return true
}

func isElixirDirectiveSide(side string) bool {
	seen := 0
	for _, line := range strings.Split(side, "\n") {
		item, ok := normalizeElixirDirectiveItem(line)
		if !ok {
			return false
		}
		if item != "" {
			seen++
		}
	}
	return seen > 0
}

func normalizeElixirDirectiveItem(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", true
	}
	if strings.HasPrefix(trimmed, "#") {
		return "", true
	}
	if idx := strings.Index(trimmed, "#"); idx >= 0 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	if trimmed == "" {
		return "", true
	}
	if strings.HasPrefix(trimmed, "alias ") || strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "use ") {
		return trimmed, true
	}
	return "", false
}

func isRustDirectiveConflictOnly(fileSummary conflictFileSummary) bool {
	for _, block := range fileSummary.blocks {
		if !isRustDirectiveSide(block.ours) || !isRustDirectiveSide(block.theirs) {
			return false
		}
	}
	return true
}

func isRustDirectiveSide(side string) bool {
	seen := 0
	for _, line := range strings.Split(side, "\n") {
		item, ok := normalizeRustDirectiveItem(line)
		if !ok {
			return false
		}
		if item != "" {
			seen++
		}
	}
	return seen > 0
}

func normalizeRustDirectiveItem(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", true
	}
	if strings.HasPrefix(trimmed, "//") {
		return "", true
	}
	if idx := strings.Index(trimmed, "//"); idx >= 0 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	if trimmed == "" {
		return "", true
	}
	if strings.HasPrefix(trimmed, "use ") || strings.HasPrefix(trimmed, "pub use ") || strings.HasPrefix(trimmed, "mod ") || strings.HasPrefix(trimmed, "pub mod ") {
		return trimmed, true
	}
	return "", false
}

func resolveElixirDirectiveConflictFile(repoDir, relPath string) error {
	return resolveDirectiveConflictFile(repoDir, relPath, normalizeElixirDirectiveItem, "elixir(alias/import/use)")
}

func resolveRustDirectiveConflictFile(repoDir, relPath string) error {
	return resolveDirectiveConflictFile(repoDir, relPath, normalizeRustDirectiveItem, "rust(use/mod)")
}

func resolveDirectiveConflictFile(
	repoDir string,
	relPath string,
	normalize func(string) (string, bool),
	description string,
) error {
	absPath := filepath.Join(repoDir, relPath)
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("读取冲突文件失败 %s: %w", relPath, err)
	}

	resolved, changed, err := resolveDirectiveConflictsInContent(string(raw), normalize, description)
	if err != nil {
		return fmt.Errorf("Rule 层处理文件 %s 失败: %w", relPath, err)
	}
	if !changed {
		return fmt.Errorf("Rule 层未匹配 %s 冲突: %s", description, relPath)
	}

	mode := fileModeOrDefault(absPath, 0o644)
	if err := os.WriteFile(absPath, []byte(resolved), mode); err != nil {
		return fmt.Errorf("写回 Rule 结果失败 %s: %w", relPath, err)
	}
	if err := os.Chmod(absPath, mode); err != nil {
		return fmt.Errorf("恢复 Rule 结果权限失败 %s: %w", relPath, err)
	}
	return nil
}

func resolveDirectiveConflictsInContent(
	content string,
	normalize func(string) (string, bool),
	description string,
) (string, bool, error) {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	changed := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "<<<<<<<") {
			result = append(result, line)
			continue
		}

		i++
		var ours []string
		for i < len(lines) && !strings.HasPrefix(lines[i], "=======") {
			ours = append(ours, lines[i])
			i++
		}
		if i >= len(lines) {
			return "", false, fmt.Errorf("冲突块缺少 =======")
		}

		i++
		var theirs []string
		for i < len(lines) && !strings.HasPrefix(lines[i], ">>>>>>>") {
			theirs = append(theirs, lines[i])
			i++
		}
		if i >= len(lines) {
			return "", false, fmt.Errorf("冲突块缺少 >>>>>>>")
		}

		merged, err := mergeDirectiveSides(ours, theirs, normalize, description)
		if err != nil {
			return "", false, err
		}
		result = append(result, merged...)
		changed = true
	}

	return strings.Join(result, "\n"), changed, nil
}

func mergeDirectiveSides(
	ours []string,
	theirs []string,
	normalize func(string) (string, bool),
	description string,
) ([]string, error) {
	entries := make(map[string]struct{})
	indent := ""

	collect := func(lines []string) error {
		for _, line := range lines {
			if spaces := leadingWhitespace(line); spaces != "" {
				indent = spaces
			}
			item, ok := normalize(line)
			if !ok {
				return fmt.Errorf("Rule 层仅支持 %s 轻度冲突，发现非白名单行: %q", description, strings.TrimSpace(line))
			}
			if item != "" {
				entries[item] = struct{}{}
			}
		}
		return nil
	}

	if err := collect(ours); err != nil {
		return nil, err
	}
	if err := collect(theirs); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("Rule 层未解析到有效 %s 项", description)
	}

	items := make([]string, 0, len(entries))
	for item := range entries {
		items = append(items, item)
	}
	sort.Strings(items)

	merged := make([]string, 0, len(items))
	for _, item := range items {
		merged = append(merged, indent+item)
	}
	return merged, nil
}

func collectProjectMarkerRoots(repoDir string, files []string, marker string) []string {
	seen := make(map[string]struct{})
	roots := make([]string, 0)
	for _, file := range files {
		root := findProjectMarkerRoot(repoDir, file, marker)
		if root == "" {
			continue
		}
		if _, exists := seen[root]; exists {
			continue
		}
		seen[root] = struct{}{}
		roots = append(roots, root)
	}
	sort.Strings(roots)
	return roots
}

func findProjectMarkerRoot(repoDir, relPath, marker string) string {
	dir := filepath.Join(repoDir, filepath.Dir(relPath))
	for {
		if dir == "" || dir == "/" || dir == "." {
			return ""
		}
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return dir
		}
		if samePath(dir, repoDir) {
			return ""
		}
		next := filepath.Dir(dir)
		if next == dir {
			return ""
		}
		dir = next
	}
}

func deriveElixirGateScope(root string, repoDir string, files []string) string {
	candidates := make([]string, 0)
	for _, file := range files {
		abs := filepath.Join(repoDir, file)
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "../") {
			continue
		}
		if strings.HasPrefix(rel, "test/") && strings.HasSuffix(strings.ToLower(rel), ".exs") {
			candidates = append(candidates, rel)
		}
	}
	sort.Strings(candidates)
	if len(candidates) == 0 {
		return ""
	}
	return strings.Join(candidates, " ")
}

func readCargoPackageName(manifestPath string) (string, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", fmt.Errorf("读取 Cargo manifest 失败 %s: %w", manifestPath, err)
	}

	inPackage := false
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inPackage = strings.EqualFold(trimmed, "[package]")
			continue
		}
		if !inPackage {
			continue
		}
		if !strings.HasPrefix(trimmed, "name") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"")
		if value != "" {
			return value, nil
		}
	}
	return "", nil
}
