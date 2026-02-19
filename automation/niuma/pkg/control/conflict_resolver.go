package control

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
)

const (
	prConflictLayerCommentMarkerFmt  = "<!-- BOT:CONFLICT_LAYER_SWITCH sha:%s step:%s -->"
	prConflictLayerStepRuleFail      = "rule-fail"
	prConflictLayerStepAITry         = "ai-try"
	prConflictLayerStepHumanEscalate = "human-escalate"
	prConflictErrorLimit             = 800
)

type prConflictAIEdit struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type prConflictAIResponse struct {
	Edits []prConflictAIEdit `json:"edits"`
}

type fileSnapshot struct {
	exists  bool
	content []byte
}

type prConflictAttemptSnapshot struct {
	preserveSet map[string]struct{}
	snapshots   map[string]fileSnapshot
}

type goTestTarget struct {
	moduleRoot string
	pkgPattern string
}

// resolvePRConflictWithLayers 执行 Rule -> AI -> Human 分层冲突修复。
// 返回 handled=true 表示已在本层完成处理（成功/no-op/升级），false 表示交回旧回退逻辑。
func (c *Controller) resolvePRConflictWithLayers(ctx context.Context, task Task, reviewStatus PRReviewStatus) (bool, error) {
	repoDir := c.prConflictRepoDir()
	if repoDir == "" {
		return false, nil
	}

	conflictFiles, summaries, err := c.collectPRConflictDetails(ctx, repoDir)
	if err != nil {
		return false, nil
	}
	if len(conflictFiles) == 0 {
		return true, nil
	}

	if err := c.tryResolveConflictByRule(ctx, repoDir, conflictFiles); err == nil {
		if err := c.stageConflictFiles(ctx, repoDir, conflictFiles); err != nil {
			return false, err
		}
		if err := c.persistConflictResolutionMetadata(task, conflictResolutionLayerRule, 0, "", time.Time{}); err != nil {
			return false, err
		}
		return true, nil
	} else {
		if persistErr := c.persistConflictResolutionMetadata(task, conflictResolutionLayerRule, 0, err.Error(), time.Now().UTC()); persistErr != nil {
			return false, persistErr
		}
		if commentErr := c.emitConflictLayerSwitchComment(
			ctx,
			task.IssueNum(),
			reviewStatus.HeadSHA,
			prConflictLayerStepRuleFail,
			fmt.Sprintf("## ⚠️ Rule 层冲突修复失败\n\n- 层级: `rule`\n- 错误: `%s`", trimPRConflictError(err)),
		); commentErr != nil {
			return false, commentErr
		}
	}

	if !c.prConflictAIEnabled() {
		return c.escalateConflictToHuman(ctx, task, reviewStatus, 0, fmt.Errorf("AI 层已禁用，Rule 层失败后升级人工"))
	}
	if allowed, reason := c.allowAIConflictResolution(conflictFiles, summaries); !allowed {
		return c.escalateConflictToHuman(ctx, task, reviewStatus, 0, fmt.Errorf("冲突超出 AI 安全边界: %s", reason))
	}

	if err := c.emitConflictLayerSwitchComment(
		ctx,
		task.IssueNum(),
		reviewStatus.HeadSHA,
		prConflictLayerStepAITry,
		"## 🤖 进入 AI 层冲突修复\n\n- 层级切换: `rule-fail -> ai-try`\n- 将执行统一门禁（结构/范围/质量）",
	); err != nil {
		return false, err
	}

	maxAttempts := c.prConflictAIMaxAttempts()
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := c.tryResolveConflictByAIOnce(ctx, repoDir, conflictFiles, summaries)
		if err == nil {
			if err := c.stageConflictFiles(ctx, repoDir, conflictFiles); err != nil {
				return false, err
			}
			if err := c.persistConflictResolutionMetadata(task, conflictResolutionLayerAI, attempt, "", time.Time{}); err != nil {
				return false, err
			}
			return true, nil
		}

		lastErr = err
		if persistErr := c.persistConflictResolutionMetadata(task, conflictResolutionLayerAI, attempt, err.Error(), time.Now().UTC()); persistErr != nil {
			return false, persistErr
		}
	}

	return c.escalateConflictToHuman(ctx, task, reviewStatus, maxAttempts, lastErr)
}

func (c *Controller) prConflictRepoDir() string {
	if c == nil || c.cfg == nil {
		return ""
	}
	return strings.TrimSpace(c.cfg.RepoDir)
}

func (c *Controller) collectPRConflictDetails(ctx context.Context, repoDir string) ([]string, map[string]conflictFileSummary, error) {
	out, err := c.runCommand(ctx, repoDir, "git", "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, nil, err
	}
	files := splitNonEmptyLines(out)
	sort.Strings(files)

	summaries := make(map[string]conflictFileSummary, len(files))
	for _, file := range files {
		summary, err := parseConflictFileSummary(repoDir, file)
		if err != nil {
			return nil, nil, err
		}
		summaries[file] = summary
	}
	return files, summaries, nil
}

func (c *Controller) tryResolveConflictByRule(ctx context.Context, repoDir string, conflictFiles []string) error {
	if len(conflictFiles) == 0 {
		return fmt.Errorf("无冲突文件")
	}

	attemptSnapshot, err := c.capturePRConflictAttemptSnapshot(ctx, repoDir, conflictFiles, conflictFiles)
	if err != nil {
		return err
	}
	rollbackWithCause := func(cause error) error {
		return wrapConflictAttemptRollbackError(cause, c.rollbackPRConflictAttempt(ctx, repoDir, attemptSnapshot))
	}

	for _, file := range conflictFiles {
		if strings.ToLower(filepath.Ext(file)) != ".go" {
			return rollbackWithCause(fmt.Errorf("规则层仅支持 Go import 冲突，发现非 Go 文件: %s", file))
		}
		if err := resolveGoImportConflictFile(repoDir, file); err != nil {
			return rollbackWithCause(err)
		}
	}

	if err := c.runPRConflictGates(ctx, repoDir, conflictFiles); err != nil {
		return rollbackWithCause(err)
	}
	return nil
}

func resolveGoImportConflictFile(repoDir, relPath string) error {
	raw, err := os.ReadFile(filepath.Join(repoDir, relPath))
	if err != nil {
		return fmt.Errorf("读取冲突文件失败 %s: %w", relPath, err)
	}

	resolved, changed, err := resolveGoImportConflictsInContent(string(raw))
	if err != nil {
		return fmt.Errorf("Rule 层处理文件 %s 失败: %w", relPath, err)
	}
	if !changed {
		return fmt.Errorf("Rule 层未匹配 import 冲突: %s", relPath)
	}
	if err := os.WriteFile(filepath.Join(repoDir, relPath), []byte(resolved), 0o644); err != nil {
		return fmt.Errorf("写回 Rule 结果失败 %s: %w", relPath, err)
	}
	return nil
}

func resolveGoImportConflictsInContent(content string) (string, bool, error) {
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

		merged, err := mergeGoImportConflictSides(ours, theirs)
		if err != nil {
			return "", false, err
		}
		result = append(result, merged...)
		changed = true
	}

	return strings.Join(result, "\n"), changed, nil
}

func mergeGoImportConflictSides(ours, theirs []string) ([]string, error) {
	entries := make(map[string]struct{})
	indent := "\t"

	collect := func(lines []string) error {
		for _, line := range lines {
			if spaces := leadingWhitespace(line); spaces != "" {
				indent = spaces
			}
			item, ok := normalizeImportItem(line)
			if !ok {
				return fmt.Errorf("Rule 层仅支持 import 项冲突，发现非 import 行: %q", strings.TrimSpace(line))
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
		return nil, fmt.Errorf("Rule 层未解析到有效 import 项")
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

func leadingWhitespace(line string) string {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			if i == 0 {
				return ""
			}
			return line[:i]
		}
	}
	return ""
}

func (c *Controller) allowAIConflictResolution(conflictFiles []string, summaries map[string]conflictFileSummary) (bool, string) {
	for _, file := range conflictFiles {
		summary := summaries[file]
		allowed, reason := isAIConflictWhitelisted(file, summary)
		if !allowed {
			return false, fmt.Sprintf("%s: %s", file, reason)
		}
	}
	return true, ""
}

func (c *Controller) tryResolveConflictByAIOnce(
	ctx context.Context,
	repoDir string,
	conflictFiles []string,
	summaries map[string]conflictFileSummary,
) error {
	provider := c.prConflictAIProvider()
	if provider == nil {
		return fmt.Errorf("AI provider 不可用")
	}

	prompt, err := c.buildPRConflictAIPrompt(ctx, repoDir, conflictFiles, summaries)
	if err != nil {
		return err
	}

	resp, err := provider.Complete(ctx, prompt, ai.WithWorkDir(repoDir))
	if err != nil {
		return fmt.Errorf("AI 冲突修复调用失败: %w", err)
	}

	edits, err := parsePRConflictAIEdits(resp)
	if err != nil {
		return err
	}
	attemptSnapshot, err := c.capturePRConflictAttemptSnapshot(ctx, repoDir, conflictFiles, collectEditPaths(edits))
	if err != nil {
		return err
	}
	rollbackWithCause := func(cause error) error {
		return wrapConflictAttemptRollbackError(cause, c.rollbackPRConflictAttempt(ctx, repoDir, attemptSnapshot))
	}

	if err := applyPRConflictAIEdits(repoDir, edits); err != nil {
		return rollbackWithCause(err)
	}

	if err := c.runPRConflictGates(ctx, repoDir, conflictFiles); err != nil {
		return rollbackWithCause(err)
	}
	return nil
}

func (c *Controller) prConflictAIProvider() ai.Provider {
	if c == nil || c.analyzer == nil {
		return nil
	}
	return c.analyzer.provider
}

func parsePRConflictAIEdits(resp string) ([]prConflictAIEdit, error) {
	resp = extractJSON(resp)
	var payload prConflictAIResponse
	if err := json.Unmarshal([]byte(resp), &payload); err != nil {
		return nil, fmt.Errorf("解析 AI 冲突修复响应失败: %w", err)
	}
	if len(payload.Edits) == 0 {
		return nil, fmt.Errorf("AI 冲突修复响应为空")
	}

	edits := make([]prConflictAIEdit, 0, len(payload.Edits))
	for _, edit := range payload.Edits {
		cleanPath, err := sanitizeEditPath(edit.Path)
		if err != nil {
			return nil, err
		}
		edits = append(edits, prConflictAIEdit{
			Path:    cleanPath,
			Content: edit.Content,
		})
	}
	return edits, nil
}

func sanitizeEditPath(path string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." || clean == "" {
		return "", fmt.Errorf("AI 输出包含非法空路径")
	}
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("AI 输出包含越界路径: %s", path)
	}
	return clean, nil
}

func collectEditPaths(edits []prConflictAIEdit) []string {
	seen := make(map[string]struct{}, len(edits))
	paths := make([]string, 0, len(edits))
	for _, edit := range edits {
		if _, ok := seen[edit.Path]; ok {
			continue
		}
		seen[edit.Path] = struct{}{}
		paths = append(paths, edit.Path)
	}
	return paths
}

func captureFileSnapshots(repoDir string, files []string) (map[string]fileSnapshot, error) {
	snapshots := make(map[string]fileSnapshot, len(files))
	for _, file := range files {
		abs := filepath.Join(repoDir, file)
		data, err := os.ReadFile(abs)
		if err != nil {
			if os.IsNotExist(err) {
				snapshots[file] = fileSnapshot{exists: false}
				continue
			}
			return nil, fmt.Errorf("读取快照文件失败 %s: %w", file, err)
		}
		snapshots[file] = fileSnapshot{exists: true, content: data}
	}
	return snapshots, nil
}

func restoreFileSnapshots(repoDir string, snapshots map[string]fileSnapshot) {
	for file, snapshot := range snapshots {
		abs := filepath.Join(repoDir, file)
		if !snapshot.exists {
			_ = os.Remove(abs)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(abs), 0o755)
		_ = os.WriteFile(abs, snapshot.content, 0o644)
	}
}

func applyPRConflictAIEdits(repoDir string, edits []prConflictAIEdit) error {
	for _, edit := range edits {
		abs := filepath.Join(repoDir, edit.Path)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return fmt.Errorf("创建 AI 编辑目录失败 %s: %w", edit.Path, err)
		}
		if err := os.WriteFile(abs, []byte(edit.Content), 0o644); err != nil {
			return fmt.Errorf("写入 AI 编辑结果失败 %s: %w", edit.Path, err)
		}
	}
	return nil
}

func (c *Controller) runPRConflictGates(ctx context.Context, repoDir string, conflictFiles []string) error {
	if err := gateConflictMarkers(repoDir, conflictFiles); err != nil {
		return err
	}
	if err := c.gateConflictGoTests(ctx, repoDir, conflictFiles); err != nil {
		return err
	}
	if smoke := c.prConflictSmokeTestCmd(); smoke != "" {
		if _, err := c.runCommand(ctx, repoDir, "bash", "-lc", smoke); err != nil {
			return fmt.Errorf("smoke tests 失败: %w", err)
		}
	}
	if err := c.gateChangedFileScope(ctx, repoDir, conflictFiles); err != nil {
		return err
	}
	return nil
}

func (c *Controller) capturePRConflictAttemptSnapshot(
	ctx context.Context,
	repoDir string,
	preserveFiles []string,
	snapshotFiles []string,
) (*prConflictAttemptSnapshot, error) {
	baselineFiles, err := c.listScopeGateChangedFiles(ctx, repoDir)
	if err != nil {
		return nil, fmt.Errorf("捕获冲突修复回滚基线失败: %w", err)
	}

	preserveList := uniqueConflictFiles(append(append([]string(nil), baselineFiles...), preserveFiles...))
	targets := uniqueConflictFiles(append(append([]string(nil), preserveList...), snapshotFiles...))
	snapshots, err := captureFileSnapshots(repoDir, targets)
	if err != nil {
		return nil, err
	}

	preserveSet := make(map[string]struct{}, len(preserveList))
	for _, file := range preserveList {
		preserveSet[file] = struct{}{}
	}
	return &prConflictAttemptSnapshot{
		preserveSet: preserveSet,
		snapshots:   snapshots,
	}, nil
}

func (c *Controller) rollbackPRConflictAttempt(
	ctx context.Context,
	repoDir string,
	attemptSnapshot *prConflictAttemptSnapshot,
) error {
	if attemptSnapshot == nil {
		return nil
	}
	restoreFileSnapshots(repoDir, attemptSnapshot.snapshots)

	changedFiles, err := c.listScopeGateChangedFiles(ctx, repoDir)
	if err != nil {
		return fmt.Errorf("回滚后检查变更失败: %w", err)
	}

	var rollbackErrs []string
	for _, file := range changedFiles {
		if _, keep := attemptSnapshot.preserveSet[file]; keep {
			continue
		}
		if err := c.restorePathFromHeadOrRemove(ctx, repoDir, file); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Sprintf("%s: %v", file, err))
		}
	}
	if len(rollbackErrs) > 0 {
		return fmt.Errorf("回滚未完全恢复: %s", strings.Join(rollbackErrs, "; "))
	}

	remainingChangedFiles, err := c.listScopeGateChangedFiles(ctx, repoDir)
	if err != nil {
		return fmt.Errorf("回滚后复检变更失败: %w", err)
	}
	outOfScope := make([]string, 0)
	for _, file := range remainingChangedFiles {
		if _, keep := attemptSnapshot.preserveSet[file]; keep {
			continue
		}
		outOfScope = append(outOfScope, file)
	}
	if len(outOfScope) > 0 {
		return fmt.Errorf("回滚后仍存在越界变更: %s", strings.Join(outOfScope, ", "))
	}
	return nil
}

func wrapConflictAttemptRollbackError(cause error, rollbackErr error) error {
	if rollbackErr == nil {
		return cause
	}
	return fmt.Errorf("%s; 回滚失败: %v", cause.Error(), rollbackErr)
}

func uniqueConflictFiles(files []string) []string {
	seen := make(map[string]struct{}, len(files))
	result := make([]string, 0, len(files))
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		result = append(result, file)
	}
	sort.Strings(result)
	return result
}

func (c *Controller) restorePathFromHeadOrRemove(ctx context.Context, repoDir, file string) error {
	tracked, err := isPathTrackedInGit(ctx, repoDir, file)
	if err != nil {
		return err
	}
	abs := filepath.Join(repoDir, file)
	if !tracked {
		if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("删除越界文件失败: %w", err)
		}
		return nil
	}

	content, err := readPathContentFromHead(ctx, repoDir, file)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("创建回滚目录失败: %w", err)
	}
	if err := os.WriteFile(abs, content, 0o644); err != nil {
		return fmt.Errorf("恢复 tracked 文件失败: %w", err)
	}
	return nil
}

func isPathTrackedInGit(ctx context.Context, repoDir, file string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--error-unmatch", "--", file)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, fmt.Errorf("检查 tracked 文件失败 %s: %w", file, err)
	}
	return true, nil
}

func readPathContentFromHead(ctx context.Context, repoDir, file string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", "show", fmt.Sprintf("HEAD:%s", file))
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("读取 HEAD 文件失败 %s: %w\noutput: %s", file, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func gateConflictMarkers(repoDir string, conflictFiles []string) error {
	for _, file := range conflictFiles {
		raw, err := os.ReadFile(filepath.Join(repoDir, file))
		if err != nil {
			return fmt.Errorf("读取门禁文件失败 %s: %w", file, err)
		}
		content := string(raw)
		if strings.Contains(content, "<<<<<<<") || strings.Contains(content, "=======") || strings.Contains(content, ">>>>>>>") {
			return fmt.Errorf("结构门禁失败: 冲突标记仍存在于 %s", file)
		}
	}
	return nil
}

func (c *Controller) gateChangedFileScope(ctx context.Context, repoDir string, conflictFiles []string) error {
	changedFiles, err := c.listScopeGateChangedFiles(ctx, repoDir)
	if err != nil {
		return fmt.Errorf("范围门禁失败: %w", err)
	}
	allowed := make(map[string]struct{}, len(conflictFiles))
	for _, file := range conflictFiles {
		allowed[file] = struct{}{}
	}
	for _, file := range changedFiles {
		if _, ok := allowed[file]; !ok {
			return fmt.Errorf("changed files out of scope: %s", file)
		}
	}
	return nil
}

func (c *Controller) listScopeGateChangedFiles(ctx context.Context, repoDir string) ([]string, error) {
	unstagedRaw, err := c.runCommand(ctx, repoDir, "git", "diff", "--name-only")
	if err != nil {
		return nil, err
	}
	stagedRaw, err := c.runCommand(ctx, repoDir, "git", "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}
	untrackedRaw, err := c.runCommand(ctx, repoDir, "git", "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	files := make([]string, 0)
	for _, raw := range []string{unstagedRaw, stagedRaw, untrackedRaw} {
		for _, file := range splitNonEmptyLines(raw) {
			if _, exists := seen[file]; exists {
				continue
			}
			seen[file] = struct{}{}
			files = append(files, file)
		}
	}
	sort.Strings(files)
	return files, nil
}

func (c *Controller) gateConflictGoTests(ctx context.Context, repoDir string, conflictFiles []string) error {
	targets := collectGoTestTargets(repoDir, conflictFiles)
	for _, target := range targets {
		if _, err := c.runCommand(ctx, target.moduleRoot, "go", "test", target.pkgPattern); err != nil {
			return fmt.Errorf("质量门禁失败 (%s %s): %w", target.moduleRoot, target.pkgPattern, err)
		}
	}
	return nil
}

func collectGoTestTargets(repoDir string, files []string) []goTestTarget {
	seen := make(map[string]struct{})
	targets := make([]goTestTarget, 0)

	for _, file := range files {
		if strings.ToLower(filepath.Ext(file)) != ".go" {
			continue
		}
		moduleRoot := findGoModuleRoot(repoDir, file)
		if moduleRoot == "" {
			continue
		}
		pkgDir := filepath.Join(repoDir, filepath.Dir(file))
		rel, err := filepath.Rel(moduleRoot, pkgDir)
		if err != nil {
			continue
		}
		pkg := "./"
		if rel != "." {
			pkg = "./" + filepath.ToSlash(rel)
		}
		key := moduleRoot + "::" + pkg
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, goTestTarget{moduleRoot: moduleRoot, pkgPattern: pkg})
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].moduleRoot == targets[j].moduleRoot {
			return targets[i].pkgPattern < targets[j].pkgPattern
		}
		return targets[i].moduleRoot < targets[j].moduleRoot
	})
	return targets
}

func findGoModuleRoot(repoDir, relFile string) string {
	dir := filepath.Join(repoDir, filepath.Dir(relFile))
	for {
		if dir == "" || dir == "/" || dir == "." {
			return ""
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
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

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	return a == b
}

func (c *Controller) stageConflictFiles(ctx context.Context, repoDir string, files []string) error {
	if len(files) == 0 {
		return nil
	}
	args := []string{"add", "--"}
	args = append(args, files...)
	if _, err := c.runCommand(ctx, repoDir, "git", args...); err != nil {
		return fmt.Errorf("标记冲突文件已解决失败: %w", err)
	}
	return nil
}

func (c *Controller) buildPRConflictAIPrompt(
	ctx context.Context,
	repoDir string,
	conflictFiles []string,
	summaries map[string]conflictFileSummary,
) (string, error) {
	var sb strings.Builder
	sb.WriteString("你是 Go 冲突修复助手。请仅返回 JSON，不要返回其他文本。\\n")
	sb.WriteString("输出格式：{\"edits\":[{\"path\":\"<file>\",\"content\":\"<full file content>\"}]}\\n")
	sb.WriteString("约束：\\n")
	sb.WriteString("1) 仅允许修改下面给出的冲突文件；\\n")
	sb.WriteString("2) 输出必须彻底移除冲突标记；\\n")
	sb.WriteString("3) 保持最小改动，不改语义；\\n")
	sb.WriteString("4) 仅输出 JSON。\\n\\n")

	for _, file := range conflictFiles {
		raw, err := os.ReadFile(filepath.Join(repoDir, file))
		if err != nil {
			return "", fmt.Errorf("构造 AI prompt 读取文件失败 %s: %w", file, err)
		}
		base, _ := c.readConflictBaseSide(ctx, repoDir, file)
		summary := summaries[file]

		sb.WriteString(fmt.Sprintf("### file: %s\\n", file))
		sb.WriteString("[base side]\\n")
		sb.WriteString(base)
		sb.WriteString("\\n[conflict file content]\\n")
		sb.Write(raw)
		sb.WriteString("\\n[conflict hunks]\\n")
		for idx, block := range summary.blocks {
			sb.WriteString(fmt.Sprintf("hunk-%d ours:\\n%s\\n", idx+1, block.ours))
			sb.WriteString(fmt.Sprintf("hunk-%d theirs:\\n%s\\n", idx+1, block.theirs))
		}
		sb.WriteString("\\n")
	}

	return sb.String(), nil
}

func (c *Controller) readConflictBaseSide(ctx context.Context, repoDir, file string) (string, error) {
	return c.runCommand(ctx, repoDir, "git", "show", fmt.Sprintf(":1:%s", file))
}

func (c *Controller) persistConflictResolutionMetadata(
	task Task,
	layer string,
	attempts int,
	lastErr string,
	lastFailedAt time.Time,
) error {
	if c == nil || c.taskctl == nil || strings.TrimSpace(task.ID) == "" {
		return nil
	}

	trimmedLastErr := strings.TrimSpace(lastErr)
	if len(trimmedLastErr) > prConflictErrorLimit {
		trimmedLastErr = trimmedLastErr[:prConflictErrorLimit]
	}

	meta := map[string]string{
		metaKeyConflictResolutionLayer:     strings.TrimSpace(layer),
		metaKeyConflictResolutionAttempts:  strconv.Itoa(attempts),
		metaKeyConflictResolutionLastError: trimmedLastErr,
	}
	if lastFailedAt.IsZero() {
		meta[metaKeyConflictResolutionLastFailedAt] = ""
	} else {
		meta[metaKeyConflictResolutionLastFailedAt] = lastFailedAt.UTC().Format(time.RFC3339)
	}
	return c.taskctl.Update(task.ID, UpdateOpts{Metadata: &meta})
}

func trimPRConflictError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) <= prConflictErrorLimit {
		return msg
	}
	return msg[:prConflictErrorLimit]
}

func (c *Controller) emitConflictLayerSwitchComment(
	ctx context.Context,
	issueNum int,
	headSHA string,
	step string,
	body string,
) error {
	if issueNum <= 0 {
		return nil
	}
	marker := fmt.Sprintf(prConflictLayerCommentMarkerFmt, normalizedHeadSHA(headSHA), step)
	payload := strings.TrimSpace(body)
	if payload == "" {
		payload = "冲突修复层级切换"
	}
	payload = payload + "\n\n" + marker
	return c.ensureIssueCommentWithMarker(ctx, issueNum, marker, payload)
}

func (c *Controller) escalateConflictToHuman(
	ctx context.Context,
	task Task,
	reviewStatus PRReviewStatus,
	attempts int,
	cause error,
) (bool, error) {
	now := time.Now().UTC()
	if err := c.persistConflictResolutionMetadata(task, conflictResolutionLayerHuman, attempts, trimPRConflictError(cause), now); err != nil {
		return true, err
	}

	if err := c.emitConflictLayerSwitchComment(
		ctx,
		task.IssueNum(),
		reviewStatus.HeadSHA,
		prConflictLayerStepHumanEscalate,
		fmt.Sprintf(
			"## ⚠️ 冲突修复升级人工处理\\n\\n- 层级: `human`\\n- attempts: `%d`\\n- last_error: `%s`\\n- failed_at: `%s`",
			attempts,
			trimPRConflictError(cause),
			now.Format(time.RFC3339),
		),
	); err != nil {
		return true, err
	}

	if task.IssueNum() > 0 {
		if err := c.syncIssueStateLabel(ctx, task.IssueNum(), needsHumanLabel); err != nil {
			return true, err
		}
	}
	return true, nil
}
