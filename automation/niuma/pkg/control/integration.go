// pkg/control/integration.go
// IntegrationBuilder 构建 integration 分支
// 支持两种模式：增量模式（EnsureBranch + MergeOne）和批量模式（Build，作为 fallback）
package control

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// IntegrationBuilder 构建 integration 分支
type IntegrationBuilder struct {
	repoDir    string
	baseBranch string
}

const (
	// integrationMergeExecutorVersion 用于记录执行器版本，便于审计与回溯。
	integrationMergeExecutorVersion = "integration-merge-executor/v1"
	maxWhitelistConflictHunks       = 3
	maxAIConflictHunks              = 6
	maxAIMildConflictBlockLines     = 12
	maxAIConflictTotalLines         = 120
)

// ConflictBlock 表示单个冲突块的 ours/theirs 内容。
type ConflictBlock struct {
	Ours   string
	Theirs string
}

// ConflictFileSummary 表示单个文件的冲突摘要。
type ConflictFileSummary struct {
	Hunks  int
	Blocks []ConflictBlock
}

// NewIntegrationBuilder 创建构建器
func NewIntegrationBuilder(repoDir, baseBranch string) *IntegrationBuilder {
	return &IntegrationBuilder{
		repoDir:    repoDir,
		baseBranch: baseBranch,
	}
}

// IntegrationBranchName 生成 integration 分支名
// metaIssueSlug: 总任务的 slug，如 "phase-3"，生成 "integration/phase-3"
func IntegrationBranchName(metaIssueSlug string) string {
	if metaIssueSlug == "" {
		return "integration/main"
	}
	return "integration/" + metaIssueSlug
}

// EnsureBranch 确保 integration 分支存在（首次创建）
// branchName: integration 分支名，如 "integration/phase-3"
// startPoint: 非空时作为创建起点；为空时 fallback 到 baseBranch
func (b *IntegrationBuilder) EnsureBranch(branchName, startPoint string) (string, error) {
	// 检查 integration 分支是否已存在
	out, err := b.gitOutput("branch", "--list", branchName)
	if err == nil && strings.TrimSpace(out) != "" {
		return branchName, nil
	}

	// 决定创建起点
	base := b.baseBranch
	if startPoint != "" {
		base = startPoint
	}

	// 从 base 创建 integration 分支
	if err := b.git("branch", branchName, base); err != nil {
		return "", fmt.Errorf("创建 integration 分支 %s 失败: %w", branchName, err)
	}

	return branchName, nil
}

// MergeOne 将单个完成的 PR 分支 merge 进 integration
// integrationBranch: 目标 integration 分支名
// startPoint: 非空时传给 EnsureBranch 作为创建起点
func (b *IntegrationBuilder) MergeOne(integrationBranch string, bi BranchInfo, startPoint string) error {
	outcome, err := b.ExecuteIntegrationMerge(integrationBranch, bi, startPoint)
	if err != nil {
		return err
	}

	if outcome.Status == MergeStatusEscalated {
		reason := "冲突需要人工处理"
		if outcome.Conflict != nil && outcome.Conflict.Reason != "" {
			reason = outcome.Conflict.Reason
		}
		return fmt.Errorf("merge %s 升级人工: %s", bi.Branch, reason)
	}

	return nil
}

// ExecuteIntegrationMerge 执行统一 integration 合并流程。
// 结果分为 merged / auto_resolved / escalated。
// startPoint: 非空时传给 EnsureBranch 作为创建起点
func (b *IntegrationBuilder) ExecuteIntegrationMerge(integrationBranch string, bi BranchInfo, startPoint string) (MergeOutcome, error) {
	outcome := MergeOutcome{
		Status:            MergeStatusMerged,
		IntegrationBranch: integrationBranch,
		SourceBranch:      bi.Branch,
		IssueNum:          bi.IssueNum,
		PRNum:             bi.PRNum,
		ExecutorVersion:   integrationMergeExecutorVersion,
		ExecutedAt:        time.Now().UTC().Format(time.RFC3339),
	}

	// 确保 integration 分支存在
	if _, err := b.EnsureBranch(integrationBranch, startPoint); err != nil {
		return MergeOutcome{}, err
	}

	// checkout integration 分支
	if err := b.git("checkout", integrationBranch); err != nil {
		return MergeOutcome{}, fmt.Errorf("checkout %s 失败: %w", integrationBranch, err)
	}
	defer b.git("checkout", b.baseBranch)

	// merge PR 分支
	msg := fmt.Sprintf("Merge %s (issue #%d)", bi.Branch, bi.IssueNum)
	if err := b.git("merge", "--no-ff", bi.Branch, "-m", msg); err == nil {
		return outcome, nil
	}

	// merge 失败时走统一冲突处理器
	return b.handleConflict(outcome)
}

// handleConflict 处理 merge 冲突：可自动处理则继续提交，否则升级人工。
func (b *IntegrationBuilder) handleConflict(outcome MergeOutcome) (MergeOutcome, error) {
	files, perFile, err := b.collectConflictDetails()
	if err != nil {
		return b.abortAndEscalate(outcome, &ConflictSummary{}, fmt.Sprintf("采集冲突信息失败: %v", err)), nil
	}
	if len(files) == 0 {
		return b.abortAndEscalate(outcome, &ConflictSummary{}, "merge 失败但未发现可解析冲突文件"), nil
	}

	summary := buildConflictSummary(files, perFile)
	var autoResolvedFiles []string

	for _, file := range files {
		fileSummary := perFile[file]
		auto, reason := canAutoResolveConflict(file, fileSummary)
		if !auto {
			return b.abortAndEscalate(outcome, summary, fmt.Sprintf("文件 %s 不满足自动处理规则: %s", file, reason)), nil
		}

		if err := b.resolveConflictWithTheirs(file); err != nil {
			return b.abortAndEscalate(outcome, summary, fmt.Sprintf("自动处理文件 %s 失败: %v", file, err)), nil
		}
		autoResolvedFiles = append(autoResolvedFiles, file)
	}

	unmergedFiles, err := b.listUnmergedFiles()
	if err != nil {
		return b.abortAndEscalate(outcome, summary, fmt.Sprintf("校验未解决冲突失败: %v", err)), nil
	}
	if len(unmergedFiles) > 0 {
		return b.abortAndEscalate(outcome, summary, fmt.Sprintf("仍有未解决冲突文件: %s", strings.Join(unmergedFiles, ","))), nil
	}

	if err := b.git("commit", "--no-edit"); err != nil {
		return b.abortAndEscalate(outcome, summary, fmt.Sprintf("自动处理后提交 merge 失败: %v", err)), nil
	}

	outcome.Status = MergeStatusAutoResolved
	outcome.AutoResolvedFiles = autoResolvedFiles
	return outcome, nil
}

func (b *IntegrationBuilder) abortAndEscalate(outcome MergeOutcome, summary *ConflictSummary, reason string) MergeOutcome {
	abortErr := b.git("merge", "--abort")
	if abortErr != nil {
		if reason == "" {
			reason = fmt.Sprintf("merge --abort 失败: %v", abortErr)
		} else {
			reason = fmt.Sprintf("%s; merge --abort 失败: %v", reason, abortErr)
		}
	}

	if summary == nil {
		summary = &ConflictSummary{}
	}
	if reason != "" {
		summary.Reason = reason
	}
	if summary.SuggestedAction == "" {
		summary.SuggestedAction = "请人工解决冲突后重新触发 integration"
	}

	outcome.Status = MergeStatusEscalated
	outcome.Conflict = summary
	return outcome
}

func (b *IntegrationBuilder) collectConflictDetails() ([]string, map[string]ConflictFileSummary, error) {
	out, err := b.gitOutput("diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, nil, err
	}
	files := splitNonEmptyLines(out)
	sort.Strings(files)

	perFile := make(map[string]ConflictFileSummary, len(files))
	for _, file := range files {
		fileSummary, err := b.parseConflictBlocks(file)
		if err != nil {
			return nil, nil, err
		}
		perFile[file] = fileSummary
	}

	return files, perFile, nil
}

func buildConflictSummary(files []string, perFile map[string]ConflictFileSummary) *ConflictSummary {
	fileHunkCount := make(map[string]int, len(files))
	totalHunks := 0
	for _, file := range files {
		hunks := perFile[file].Hunks
		fileHunkCount[file] = hunks
		totalHunks += hunks
	}

	return &ConflictSummary{
		Files:           files,
		FileHunkCount:   fileHunkCount,
		TotalHunkCount:  totalHunks,
		SuggestedAction: "请人工解决冲突后重新触发 integration",
	}
}

func (b *IntegrationBuilder) parseConflictBlocks(relPath string) (ConflictFileSummary, error) {
	return parseConflictFileSummary(b.repoDir, relPath)
}

func parseConflictFileSummary(repoDir, relPath string) (ConflictFileSummary, error) {
	raw, err := os.ReadFile(filepath.Join(repoDir, relPath))
	if err != nil {
		return ConflictFileSummary{}, fmt.Errorf("读取冲突文件 %s 失败: %w", relPath, err)
	}
	return parseConflictBlocksContent(relPath, string(raw))
}

func parseConflictBlocksContent(relPath string, raw string) (ConflictFileSummary, error) {
	lines := strings.Split(raw, "\n")
	summary := ConflictFileSummary{}

	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "<<<<<<<") {
			continue
		}

		i++
		var ours []string
		for i < len(lines) && !strings.HasPrefix(lines[i], "=======") {
			ours = append(ours, lines[i])
			i++
		}
		if i >= len(lines) {
			return ConflictFileSummary{}, fmt.Errorf("解析冲突文件 %s 失败: 缺少 =======", relPath)
		}

		i++
		var theirs []string
		for i < len(lines) && !strings.HasPrefix(lines[i], ">>>>>>>") {
			theirs = append(theirs, lines[i])
			i++
		}
		if i >= len(lines) {
			return ConflictFileSummary{}, fmt.Errorf("解析冲突文件 %s 失败: 缺少 >>>>>>>", relPath)
		}

		summary.Hunks++
		summary.Blocks = append(summary.Blocks, ConflictBlock{
			Ours:   strings.Join(ours, "\n"),
			Theirs: strings.Join(theirs, "\n"),
		})
	}

	return summary, nil
}

func canAutoResolveConflict(file string, fileSummary ConflictFileSummary) (bool, string) {
	if fileSummary.Hunks == 0 || len(fileSummary.Blocks) == 0 {
		return false, "未识别到可处理的文本冲突块"
	}

	// 非核心白名单文件允许小规模冲突直接选择 theirs。
	if isNonCoreWhitelistedFile(file) {
		if fileSummary.Hunks <= maxWhitelistConflictHunks {
			return true, "非核心白名单文件小规模冲突"
		}
		return false, fmt.Sprintf("白名单文件冲突块过多: %d", fileSummary.Hunks)
	}

	// 其他文件仅允许注释/空白差异。
	for _, block := range fileSummary.Blocks {
		if !isCommentWhitespaceEquivalent(file, block.Ours, block.Theirs) {
			return false, "冲突内容包含语义差异"
		}
	}
	return true, "注释或空白差异"
}

// isAIConflictWhitelisted 判断冲突是否允许进入 AI 层。
func isAIConflictWhitelisted(file string, fileSummary ConflictFileSummary) (bool, string) {
	if fileSummary.Hunks == 0 || len(fileSummary.Blocks) == 0 {
		return false, "未识别到可处理的文本冲突块"
	}
	if risky, reason := hasHighRiskConflict(file, fileSummary); risky {
		return false, reason
	}
	if isGoImportConflictOnly(fileSummary) {
		return true, "go import 冲突"
	}
	if strings.ToLower(filepath.Ext(file)) == ".rs" && isRustUseConflictOnly(fileSummary) {
		return true, "rust-use-only"
	}
	if isElixirLightweightConflict(file, fileSummary) {
		return true, "elixir alias/import/use 轻度冲突"
	}
	if isLightweightGoTestConflict(file, fileSummary) {
		return true, "测试辅助代码轻度并合"
	}
	if isAdjacentMildConflict(fileSummary) {
		return true, "轻度相邻块冲突"
	}
	return false, "未命中 AI 白名单冲突类型"
}

func hasHighRiskConflict(file string, fileSummary ConflictFileSummary) (bool, string) {
	fileLower := strings.ToLower(file)
	if strings.Contains(fileLower, "migration") || strings.HasSuffix(fileLower, ".sql") {
		return true, "检测到迁移脚本冲突"
	}
	if fileSummary.Hunks > maxAIConflictHunks {
		return true, fmt.Sprintf("冲突块过多: %d", fileSummary.Hunks)
	}

	isRust := strings.ToLower(filepath.Ext(file)) == ".rs"
	totalLines := 0
	for _, block := range fileSummary.Blocks {
		totalLines += countConflictSideLines(block.Ours)
		totalLines += countConflictSideLines(block.Theirs)
		merged := block.Ours + "\n" + block.Theirs
		if isRust {
			if containsRustCoreSignal(merged) {
				return true, "检测到 Rust 核心 trait/impl 语义变更"
			}
		} else if containsCoreInterfaceSignal(merged) {
			return true, "检测到核心接口语义变更信号"
		}
	}
	if totalLines > maxAIConflictTotalLines {
		return true, fmt.Sprintf("冲突行数过多: %d", totalLines)
	}
	return false, ""
}

func containsCoreInterfaceSignal(s string) bool {
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		normalized := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(normalized, "type ") && strings.Contains(normalized, " interface") {
			return true
		}
		if strings.Contains(normalized, "interface{") {
			return true
		}
	}
	return false
}

func isGoImportConflictOnly(fileSummary ConflictFileSummary) bool {
	for _, block := range fileSummary.Blocks {
		if !isImportSideText(block.Ours) || !isImportSideText(block.Theirs) {
			return false
		}
	}
	return true
}

func isImportSideText(side string) bool {
	lines := strings.Split(side, "\n")
	seen := 0
	for _, line := range lines {
		item, ok := normalizeImportItem(line)
		if !ok {
			return false
		}
		if item != "" {
			seen++
		}
	}
	return seen > 0
}

// rustUseLineRe 匹配单行 use 语句（含花括号展开），排除 glob * 和 rename as。
var rustUseLineRe = regexp.MustCompile(`^use\s+[\w:]+(::\{\s*[\w\s,]+\s*\})?\s*;\s*$`)

// isRustUseConflictOnly 判断冲突块是否全部为低风险 use 语句。
func isRustUseConflictOnly(fileSummary ConflictFileSummary) bool {
	for _, block := range fileSummary.Blocks {
		if !isRustUseSideText(block.Ours) || !isRustUseSideText(block.Theirs) {
			return false
		}
	}
	return true
}

// isElixirLightweightConflict 判断 .ex/.exs 文件是否仅包含 alias/import/use 级别的轻度冲突。
// 条件：hunks ≤ 3，每侧 ≤ 12 行非空行，且所有非空冲突行以 alias/import/use 开头。
func isElixirLightweightConflict(file string, fileSummary ConflictFileSummary) bool {
	ext := strings.ToLower(filepath.Ext(file))
	if ext != ".ex" && ext != ".exs" {
		return false
	}
	if fileSummary.Hunks > maxWhitelistConflictHunks {
		return false
	}
	for _, block := range fileSummary.Blocks {
		if countConflictSideLines(block.Ours) > maxAIMildConflictBlockLines {
			return false
		}
		if countConflictSideLines(block.Theirs) > maxAIMildConflictBlockLines {
			return false
		}
		if !isElixirLightweightSide(block.Ours) || !isElixirLightweightSide(block.Theirs) {
			return false
		}
	}
	return true
}

// isRustUseSideText 逐行检查是否为 use 语句、空行或注释。
func isRustUseSideText(side string) bool {
	lines := strings.Split(side, "\n")
	seen := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		if !rustUseLineRe.MatchString(trimmed) {
			return false
		}
		seen++
	}
	return seen > 0
}

// isElixirLightweightSide 检查冲突块的一侧是否全部为 alias/import/use 语句。
func isElixirLightweightSide(side string) bool {
	seen := 0
	for _, line := range strings.Split(side, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "alias ") &&
			!strings.HasPrefix(trimmed, "import ") &&
			!strings.HasPrefix(trimmed, "use ") {
			return false
		}
		seen++
	}
	return seen > 0
}

// containsRustCoreSignal 检测 trait/impl/unsafe/macro_rules! 高风险关键字。
func containsRustCoreSignal(s string) bool {
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "trait ") || strings.HasPrefix(trimmed, "pub trait ") {
			return true
		}
		if strings.HasPrefix(trimmed, "impl ") || strings.HasPrefix(trimmed, "impl<") {
			return true
		}
		if strings.HasPrefix(trimmed, "unsafe ") {
			return true
		}
		if strings.HasPrefix(trimmed, "macro_rules!") {
			return true
		}
	}
	return false
}

func isLightweightGoTestConflict(file string, fileSummary ConflictFileSummary) bool {
	if !strings.HasSuffix(strings.ToLower(file), "_test.go") {
		return false
	}
	if fileSummary.Hunks > 2 {
		return false
	}
	for _, block := range fileSummary.Blocks {
		if countConflictSideLines(block.Ours) > maxAIMildConflictBlockLines {
			return false
		}
		if countConflictSideLines(block.Theirs) > maxAIMildConflictBlockLines {
			return false
		}
	}
	return true
}

func isAdjacentMildConflict(fileSummary ConflictFileSummary) bool {
	if fileSummary.Hunks > 2 {
		return false
	}
	for _, block := range fileSummary.Blocks {
		if countConflictSideLines(block.Ours) > maxAIMildConflictBlockLines {
			return false
		}
		if countConflictSideLines(block.Theirs) > maxAIMildConflictBlockLines {
			return false
		}
	}
	return true
}

func countConflictSideLines(side string) int {
	count := 0
	for _, line := range strings.Split(side, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func normalizeImportItem(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed == "(" || trimmed == ")" {
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
	if strings.HasPrefix(trimmed, "import ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "import "))
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", true
	}
	if len(fields) > 2 {
		return "", false
	}

	path := fields[len(fields)-1]
	if !strings.HasPrefix(path, "\"") || !strings.HasSuffix(path, "\"") || len(path) < 2 {
		return "", false
	}

	if len(fields) == 1 {
		return path, true
	}

	alias := fields[0]
	if !isValidGoImportAlias(alias) {
		return "", false
	}
	return alias + " " + path, true
}

func isValidGoImportAlias(alias string) bool {
	if alias == "." || alias == "_" {
		return true
	}
	for i, r := range alias {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return alias != ""
}

func isNonCoreWhitelistedFile(file string) bool {
	if strings.HasPrefix(file, "docs/") || strings.HasPrefix(file, ".github/") {
		return true
	}

	ext := strings.ToLower(filepath.Ext(file))
	return ext == ".md" || ext == ".mdx"
}

func isCommentWhitespaceEquivalent(file, ours, theirs string) bool {
	left := normalizeConflictSide(file, ours)
	right := normalizeConflictSide(file, theirs)
	return left == right
}

func normalizeConflictSide(file, text string) string {
	lines := strings.Split(text, "\n")
	hashComments := supportsHashComments(file)

	var sb strings.Builder
	inBlockComment := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line, inBlockComment = trimLeadingBlockComment(line, inBlockComment)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "//") ||
			strings.HasPrefix(line, "*/") ||
			strings.HasPrefix(line, "<!--") ||
			strings.HasPrefix(line, "-->") {
			continue
		}
		if hashComments && strings.HasPrefix(line, "#") {
			continue
		}

		sb.WriteString(removeWhitespace(line))
	}

	return sb.String()
}

func trimLeadingBlockComment(line string, inBlockComment bool) (string, bool) {
	for {
		line = strings.TrimSpace(line)
		if line == "" {
			return "", inBlockComment
		}
		if inBlockComment {
			end := strings.Index(line, "*/")
			if end < 0 {
				return "", true
			}
			line = line[end+2:]
			inBlockComment = false
			continue
		}
		if strings.HasPrefix(line, "/*") {
			end := strings.Index(line, "*/")
			if end < 0 {
				return "", true
			}
			line = line[end+2:]
			continue
		}
		return line, false
	}
}

func supportsHashComments(file string) bool {
	base := strings.ToLower(filepath.Base(file))
	if base == ".gitignore" {
		return true
	}

	switch strings.ToLower(filepath.Ext(file)) {
	case ".md", ".markdown", ".yml", ".yaml", ".toml", ".sh", ".py", ".ini", ".conf", ".txt":
		return true
	default:
		return false
	}
}

func removeWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func (b *IntegrationBuilder) resolveConflictWithTheirs(file string) error {
	if err := b.git("checkout", "--theirs", "--", file); err != nil {
		return err
	}
	if err := b.git("add", "--", file); err != nil {
		return err
	}
	return nil
}

func (b *IntegrationBuilder) listUnmergedFiles() ([]string, error) {
	out, err := b.gitOutput("diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	return splitNonEmptyLines(out), nil
}

func splitNonEmptyLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func buildCommonConflictPrompt(conflictFiles []string) string {
	files := append([]string(nil), conflictFiles...)
	sort.Strings(files)

	var sb strings.Builder
	sb.WriteString("你是代码冲突修复助手。请仅返回 JSON，不要返回其他文本。\n")
	sb.WriteString("输出格式：{\"edits\":[{\"path\":\"<file>\",\"content\":\"<full file content>\"}]}\n")
	sb.WriteString("通用约束：\n")
	sb.WriteString("1) 仅允许修改输入中声明的冲突文件；\n")
	sb.WriteString("2) 输出必须完全移除冲突标记（<<<<<<<、=======、>>>>>>>）；\n")
	sb.WriteString("3) 保持最小改动，不要越权重构；\n")
	sb.WriteString("4) 严禁修改与冲突无关的文件；\n")
	sb.WriteString("5) 仅输出 JSON。\n")
	if len(files) > 0 {
		sb.WriteString("\n允许修改文件：\n")
		for _, file := range files {
			sb.WriteString("- ")
			sb.WriteString(file)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func assemblePrompt(commonPrompt, profilePrompt string) string {
	common := strings.TrimSpace(commonPrompt)
	profile := strings.TrimSpace(profilePrompt)

	if common == "" {
		return profile
	}
	if profile == "" {
		return common
	}
	return common + "\n\n" + profile
}

// ComputeOldestMergeBase 对每个 PR 分支计算与 baseBranch 的 merge-base，
// 返回拓扑序最旧的那个 commit（即最早的公共祖先）。
// 如果 prBranches 为空或全部计算失败，返回空字符串（调用方应 fallback 到 baseBranch）。
// TODO(#411): add merge-base distance check
func (b *IntegrationBuilder) ComputeOldestMergeBase(prBranches []string) (string, error) {
	if len(prBranches) == 0 {
		return "", nil
	}

	var mergeBases []string
	for _, branch := range prBranches {
		mb, err := b.gitOutput("merge-base", b.baseBranch, branch)
		if err != nil {
			continue
		}
		if mb != "" {
			mergeBases = append(mergeBases, mb)
		}
	}

	if len(mergeBases) == 0 {
		return "", fmt.Errorf("无法计算任何 PR 分支的 merge-base")
	}

	// 取拓扑序最旧的 merge-base：两两比较，用 --is-ancestor 找到最旧的
	oldest := mergeBases[0]
	for _, candidate := range mergeBases[1:] {
		if candidate == oldest {
			continue
		}
		// 如果 candidate 是 oldest 的祖先，candidate 更旧
		if err := b.git("merge-base", "--is-ancestor", candidate, oldest); err == nil {
			oldest = candidate
		}
	}

	return oldest, nil
}

// Reset 重建 integration 分支（冲突无法解决时）
// branchName: 要重建的 integration 分支名
// startPoint: 非空时作为创建起点；为空时 fallback 到 baseBranch
func (b *IntegrationBuilder) Reset(branchName, startPoint string) error {
	// 确保不在 integration 分支上
	_ = b.git("checkout", b.baseBranch)

	// 删除旧的 integration 分支（如果存在）
	_ = b.git("branch", "-D", branchName)

	// 决定创建起点
	base := b.baseBranch
	if startPoint != "" {
		base = startPoint
	}

	// 从 base 重新创建
	if err := b.git("branch", branchName, base); err != nil {
		return fmt.Errorf("重建 integration 分支 %s 失败: %w", branchName, err)
	}

	return nil
}

// Build 按 topo 序构建 integration 分支（批量模式，作为 fallback）
// integrationBranch: 目标 integration 分支名
// branches: 应已按 topo 序排列
// deps: 依赖关系，key 的分支依赖 value 列表中的 issue
func (b *IntegrationBuilder) Build(integrationBranch string, branches []BranchInfo, deps map[int][]int) (*IntegrationResult, error) {
	// 收集所有 PR 分支名，计算最旧 merge-base 作为 integration 分支起点
	var prBranches []string
	for _, bi := range branches {
		prBranches = append(prBranches, bi.Branch)
	}
	startPoint, _ := b.ComputeOldestMergeBase(prBranches)

	// 重置 integration 分支，从 merge-base 开始批量 merge
	if err := b.Reset(integrationBranch, startPoint); err != nil {
		return nil, fmt.Errorf("重置 integration 分支失败: %w", err)
	}

	result := &IntegrationResult{Branch: integrationBranch}

	// 构建 issue → 是否失败的映射（用于级联跳过）
	failed := make(map[int]bool)

	for _, bi := range branches {
		// 检查是否需要级联跳过
		if shouldSkip(bi.IssueNum, deps, failed) {
			result.Skipped = append(result.Skipped, bi.IssueNum)
			continue
		}

		outcome, err := b.ExecuteIntegrationMerge(integrationBranch, bi, "")
		if err != nil {
			return nil, err
		}

		switch outcome.Status {
		case MergeStatusMerged, MergeStatusAutoResolved:
			result.Merged = append(result.Merged, bi.IssueNum)
		case MergeStatusEscalated:
			result.Conflicts = append(result.Conflicts, bi.IssueNum)
			failed[bi.IssueNum] = true
		default:
			result.Conflicts = append(result.Conflicts, bi.IssueNum)
			failed[bi.IssueNum] = true
		}
	}

	return result, nil
}

// shouldSkip 检查是否因依赖失败需要级联跳过
func shouldSkip(issueNum int, deps map[int][]int, failed map[int]bool) bool {
	for _, dep := range deps[issueNum] {
		if failed[dep] {
			return true
		}
	}
	return false
}

// CleanOld 清理旧的 integration 分支，保留最近 3 个
// 删除 pattern: integration/batch-* 或 integration/test-* 的旧分支
func (b *IntegrationBuilder) CleanOld() error {
	// 获取所有 integration/batch-* 和 integration/test-* 分支，按创建时间排序
	out, err := b.gitOutput("branch", "--list", "integration/batch-*", "integration/test-*", "--sort=-creatordate")
	if err != nil {
		return fmt.Errorf("列出 integration 分支失败: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 移除开头的 * (当前分支标记)
		line = strings.TrimPrefix(line, "* ")
		if line != "" {
			branches = append(branches, line)
		}
	}

	// 保留最近 3 个，删除其余的
	if len(branches) > 3 {
		for _, branch := range branches[3:] {
			if err := b.git("branch", "-D", branch); err != nil {
				// 忽略删除失败（可能分支不存在）
				continue
			}
		}
	}

	return nil
}

// git 执行 git 命令
func (b *IntegrationBuilder) git(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = b.repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return nil
}

// gitOutput 执行 git 命令并返回输出
func (b *IntegrationBuilder) gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = b.repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}
