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

type prConflictAIEditPayload struct {
	Path    *string `json:"path"`
	Content *string `json:"content"`
}

type prConflictAIResponse struct {
	Edits []prConflictAIEditPayload `json:"edits"`
}

type fileSnapshot struct {
	exists  bool
	mode    os.FileMode
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

	registry := defaultConflictProfileRegistry()
	groups, unknownFiles := groupConflictFilesByProfile(repoDir, conflictFiles, registry)
	meta := conflictMetadataFromGroups(groups, conflictFiles)
	if len(unknownFiles) > 0 {
		return c.escalateConflictToHuman(
			ctx,
			task,
			reviewStatus,
			0,
			fmt.Errorf("存在未知冲突文件类型: %s", strings.Join(unknownFiles, ", ")),
			meta,
		)
	}

	for _, group := range groups {
		profileCfg := c.prConflictProfileConfig(group.Name)
		if !profileCfg.Enabled {
			return c.escalateConflictToHuman(
				ctx,
				task,
				reviewStatus,
				0,
				fmt.Errorf("profile %s 已禁用: %s", group.Name, strings.Join(group.Files, ", ")),
				meta,
			)
		}
	}

	sessionSnapshot, err := c.capturePRConflictAttemptSnapshot(ctx, repoDir, conflictFiles, conflictFiles)
	if err != nil {
		return false, err
	}
	rollbackSessionWithCause := func(attempts int, cause error) (bool, error) {
		rollbackErr := c.rollbackPRConflictAttempt(ctx, repoDir, sessionSnapshot)
		return c.escalateConflictToHuman(ctx, task, reviewStatus, attempts, wrapConflictAttemptRollbackError(cause, rollbackErr), meta)
	}

	layer := conflictResolutionLayerRule
	totalAIAttempts := 0
	enteredAI := false

	for _, group := range groups {
		err := c.tryResolveConflictByRule(ctx, repoDir, conflictFiles, group)
		if err == nil {
			continue
		}

		ruleMeta := meta
		ruleMeta.ResolutionPath = conflictResolutionLayerRule
		if persistErr := c.persistConflictResolutionMetadata(
			task,
			conflictResolutionLayerRule,
			totalAIAttempts,
			err.Error(),
			time.Now().UTC(),
			ruleMeta,
		); persistErr != nil {
			return false, persistErr
		}
		if commentErr := c.emitConflictLayerSwitchComment(
			ctx,
			task.IssueNum(),
			reviewStatus.HeadSHA,
			prConflictLayerStepRuleFail,
			fmt.Sprintf(
				"## ⚠️ Rule 层冲突修复失败\n\n- profile: `%s`\n- 文件: `%s`\n- 错误: `%s`",
				group.Name,
				strings.Join(group.Files, ", "),
				trimPRConflictError(err),
			),
		); commentErr != nil {
			return false, commentErr
		}

		if !c.prConflictAIEnabled() {
			return rollbackSessionWithCause(totalAIAttempts, fmt.Errorf("AI 层已禁用，Rule 层失败后升级人工"))
		}
		if allowed, reason := c.allowAIConflictResolution(group, summaries); !allowed {
			return rollbackSessionWithCause(totalAIAttempts, fmt.Errorf("冲突超出 AI 安全边界: %s", reason))
		}
		if !enteredAI {
			if commentErr := c.emitConflictLayerSwitchComment(
				ctx,
				task.IssueNum(),
				reviewStatus.HeadSHA,
				prConflictLayerStepAITry,
				"## 🤖 进入 AI 层冲突修复\n\n- 层级切换: `rule-fail -> ai-try`\n- 将执行 Common Gate + Profile Gate",
			); commentErr != nil {
				return false, commentErr
			}
			enteredAI = true
		}

		layer = conflictResolutionLayerAI
		resolvedByAI := false
		var lastErr error
		for attempt := 1; attempt <= c.prConflictAIMaxAttempts(); attempt++ {
			totalAIAttempts++
			err = c.tryResolveConflictByAIOnce(ctx, repoDir, conflictFiles, group, summaries)
			if err == nil {
				resolvedByAI = true
				break
			}

			lastErr = err
			aiMeta := meta
			aiMeta.ResolutionPath = conflictResolutionLayerAI
			if persistErr := c.persistConflictResolutionMetadata(
				task,
				conflictResolutionLayerAI,
				totalAIAttempts,
				err.Error(),
				time.Now().UTC(),
				aiMeta,
			); persistErr != nil {
				return false, persistErr
			}
		}
		if !resolvedByAI {
			return rollbackSessionWithCause(totalAIAttempts, lastErr)
		}
	}

	if err := c.stageConflictFiles(ctx, repoDir, conflictFiles); err != nil {
		rollbackErr := c.rollbackPRConflictAttempt(ctx, repoDir, sessionSnapshot)
		return false, wrapConflictAttemptRollbackError(err, rollbackErr)
	}

	meta.GatePassed = true
	meta.ResolutionPath = layer
	if err := c.persistConflictResolutionMetadata(task, layer, totalAIAttempts, "", time.Time{}, meta); err != nil {
		return false, err
	}
	return true, nil
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

func (c *Controller) tryResolveConflictByRule(
	ctx context.Context,
	repoDir string,
	allConflictFiles []string,
	group conflictProfileGroup,
) error {
	if len(group.Files) == 0 {
		return fmt.Errorf("无冲突文件")
	}

	attemptSnapshot, err := c.capturePRConflictAttemptSnapshot(ctx, repoDir, allConflictFiles, group.Files)
	if err != nil {
		return err
	}
	rollbackWithCause := func(cause error) error {
		return wrapConflictAttemptRollbackError(cause, c.rollbackPRConflictAttempt(ctx, repoDir, attemptSnapshot))
	}

	for _, file := range group.Files {
		changed, err := group.Profile.ApplyRuleFix(repoDir, file)
		if err != nil {
			return rollbackWithCause(err)
		}
		if !changed {
			return rollbackWithCause(fmt.Errorf("Rule 层未修改文件: %s", file))
		}
	}

	if err := c.runPRConflictGates(ctx, repoDir, allConflictFiles, group); err != nil {
		return rollbackWithCause(err)
	}
	return nil
}

func resolveGoImportConflictFile(repoDir, relPath string) error {
	absPath := filepath.Join(repoDir, relPath)
	raw, err := os.ReadFile(absPath)
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
	mode := fileModeOrDefault(absPath, 0o644)
	if err := os.WriteFile(absPath, []byte(resolved), mode); err != nil {
		return fmt.Errorf("写回 Rule 结果失败 %s: %w", relPath, err)
	}
	if err := os.Chmod(absPath, mode); err != nil {
		return fmt.Errorf("恢复 Rule 结果权限失败 %s: %w", relPath, err)
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

func (c *Controller) prConflictProfileConfig(name string) PRConflictProfileConfig {
	if c == nil || c.cfg == nil {
		return lookupPRConflictProfileConfig(defaultPRConflictResolutionConfig(), name)
	}
	return lookupPRConflictProfileConfig(c.cfg.ConflictResolution, name)
}

func (c *Controller) allowAIConflictResolution(group conflictProfileGroup, summaries map[string]conflictFileSummary) (bool, string) {
	profileCfg := c.prConflictProfileConfig(group.Name)
	for _, file := range group.Files {
		summary := summaries[file]
		allowed, reason := group.Profile.Allow(file, summary, profileCfg.Threshold)
		if !allowed {
			return false, fmt.Sprintf("%s: %s", file, reason)
		}
	}
	return true, ""
}

func (c *Controller) tryResolveConflictByAIOnce(
	ctx context.Context,
	repoDir string,
	allConflictFiles []string,
	group conflictProfileGroup,
	summaries map[string]conflictFileSummary,
) error {
	provider := c.prConflictAIProvider()
	if provider == nil {
		return fmt.Errorf("AI provider 不可用")
	}

	prompt, err := c.buildPRConflictAIPrompt(ctx, repoDir, group, summaries)
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
	if err := validateProfileEditScope(group.Files, edits); err != nil {
		return err
	}

	editedPaths := append(collectEditPaths(edits), group.Files...)
	attemptSnapshot, err := c.capturePRConflictAttemptSnapshot(ctx, repoDir, allConflictFiles, editedPaths)
	if err != nil {
		return err
	}
	rollbackWithCause := func(cause error) error {
		return wrapConflictAttemptRollbackError(cause, c.rollbackPRConflictAttempt(ctx, repoDir, attemptSnapshot))
	}

	if err := applyPRConflictAIEdits(repoDir, edits); err != nil {
		return rollbackWithCause(err)
	}

	if err := c.runPRConflictGates(ctx, repoDir, allConflictFiles, group); err != nil {
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
		if edit.Path == nil {
			return nil, fmt.Errorf("AI 冲突修复响应缺少 path 字段")
		}
		if edit.Content == nil {
			return nil, fmt.Errorf("AI 冲突修复响应缺少 content 字段: %s", strings.TrimSpace(*edit.Path))
		}

		cleanPath, err := sanitizeEditPath(*edit.Path)
		if err != nil {
			return nil, err
		}
		edits = append(edits, prConflictAIEdit{
			Path:    cleanPath,
			Content: *edit.Content,
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

func validateProfileEditScope(profileFiles []string, edits []prConflictAIEdit) error {
	allowed := make(map[string]struct{}, len(profileFiles))
	for _, file := range profileFiles {
		allowed[file] = struct{}{}
	}
	for _, edit := range edits {
		if _, ok := allowed[edit.Path]; ok {
			continue
		}
		return fmt.Errorf("范围门禁失败: AI 输出越组修改 %s", edit.Path)
	}
	return nil
}

func captureFileSnapshots(repoDir string, files []string) (map[string]fileSnapshot, error) {
	snapshots := make(map[string]fileSnapshot, len(files))
	for _, file := range files {
		abs := filepath.Join(repoDir, file)
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				snapshots[file] = fileSnapshot{exists: false}
				continue
			}
			return nil, fmt.Errorf("读取快照文件失败 %s: %w", file, err)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf("读取快照文件失败 %s: %w", file, err)
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		snapshots[file] = fileSnapshot{exists: true, mode: mode, content: data}
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
		mode := snapshot.mode
		if mode == 0 {
			mode = 0o644
		}
		_ = os.WriteFile(abs, snapshot.content, mode)
		_ = os.Chmod(abs, mode)
	}
}

func applyPRConflictAIEdits(repoDir string, edits []prConflictAIEdit) error {
	for _, edit := range edits {
		abs := filepath.Join(repoDir, edit.Path)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return fmt.Errorf("创建 AI 编辑目录失败 %s: %w", edit.Path, err)
		}
		mode := fileModeOrDefault(abs, 0o644)
		if err := os.WriteFile(abs, []byte(edit.Content), mode); err != nil {
			return fmt.Errorf("写入 AI 编辑结果失败 %s: %w", edit.Path, err)
		}
		if err := os.Chmod(abs, mode); err != nil {
			return fmt.Errorf("恢复 AI 编辑结果权限失败 %s: %w", edit.Path, err)
		}
	}
	return nil
}

func (c *Controller) runPRConflictGates(
	ctx context.Context,
	repoDir string,
	allConflictFiles []string,
	group conflictProfileGroup,
) error {
	if err := gateConflictMarkers(repoDir, group.Files); err != nil {
		return err
	}
	if err := c.gateChangedFileScope(ctx, repoDir, allConflictFiles); err != nil {
		return err
	}

	profileCfg := c.prConflictProfileConfig(group.Name)
	commands, err := group.Profile.GateCommands(repoDir, group.Files, profileCfg)
	if err != nil {
		return fmt.Errorf("profile gate 命令构建失败 (%s): %w", group.Name, err)
	}
	if err := c.runProfileGateCommands(ctx, group.Name, commands); err != nil {
		return err
	}

	if smoke := c.prConflictSmokeTestCmd(); smoke != "" {
		if _, err := c.runCommand(ctx, repoDir, "bash", "-lc", smoke); err != nil {
			return fmt.Errorf("smoke tests 失败: %w", err)
		}
	}
	if err := c.gateChangedFileScope(ctx, repoDir, allConflictFiles); err != nil {
		return err
	}
	return nil
}

func (c *Controller) runProfileGateCommands(ctx context.Context, profileName string, commands []profileGateCommand) error {
	for _, command := range commands {
		if strings.TrimSpace(command.Command) == "" {
			continue
		}
		if _, err := c.runCommand(ctx, command.Dir, "bash", "-lc", command.Command); err != nil {
			return fmt.Errorf("质量门禁失败 [%s] (%s): %w", profileName, command.Command, err)
		}
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
	mode, err := readPathModeFromHead(ctx, repoDir, file)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("创建回滚目录失败: %w", err)
	}
	if err := os.WriteFile(abs, content, mode); err != nil {
		return fmt.Errorf("恢复 tracked 文件失败: %w", err)
	}
	if err := os.Chmod(abs, mode); err != nil {
		return fmt.Errorf("恢复 tracked 文件权限失败: %w", err)
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

func readPathModeFromHead(ctx context.Context, repoDir, file string) (os.FileMode, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-tree", "HEAD", "--", file)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("读取 HEAD 文件权限失败 %s: %w\noutput: %s", file, err, strings.TrimSpace(string(out)))
	}

	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return 0, fmt.Errorf("读取 HEAD 文件权限失败 %s: 空输出", file)
	}

	modeBits, parseErr := strconv.ParseUint(fields[0], 8, 32)
	if parseErr != nil {
		return 0, fmt.Errorf("解析 HEAD 文件权限失败 %s: %w", file, parseErr)
	}

	mode := os.FileMode(modeBits & 0o777)
	if mode == 0 {
		mode = 0o644
	}
	return mode, nil
}

func fileModeOrDefault(path string, defaultMode os.FileMode) os.FileMode {
	info, err := os.Stat(path)
	if err != nil {
		return defaultMode
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		return defaultMode
	}
	return mode
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
	group conflictProfileGroup,
	summaries map[string]conflictFileSummary,
) (string, error) {
	var sb strings.Builder
	sb.WriteString("你是 Git 冲突修复助手。请仅返回 JSON，不要返回其他文本。\\n")
	sb.WriteString("输出 schema：{\"edits\":[{\"path\":\"<file>\",\"content\":\"<full file content>\"}]}\\n")
	sb.WriteString("Common 约束：\\n")
	sb.WriteString("1) 仅允许修改当前 profile 的冲突文件；\\n")
	sb.WriteString("2) 仅修改冲突块，不得越界修改无关区域；\\n")
	sb.WriteString("3) 输出必须彻底移除冲突标记；\\n")
	sb.WriteString("4) 保持最小改动，禁止跨文件重构；\\n")
	sb.WriteString("5) 仅输出合法 JSON。\\n\\n")
	sb.WriteString(group.Profile.PromptAddon())
	sb.WriteString("\\n\\n")

	for _, file := range group.Files {
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
	ext ConflictResolutionMeta,
) error {
	if c == nil || c.taskctl == nil || strings.TrimSpace(task.ID) == "" {
		return nil
	}

	trimmedLastErr := strings.TrimSpace(lastErr)
	if len(trimmedLastErr) > prConflictErrorLimit {
		trimmedLastErr = trimmedLastErr[:prConflictErrorLimit]
	}

	meta := map[string]string{
		metaKeyConflictResolutionLayer:      strings.TrimSpace(layer),
		metaKeyConflictResolutionAttempts:   strconv.Itoa(attempts),
		metaKeyConflictResolutionProfile:    strings.TrimSpace(ext.Profile),
		metaKeyConflictResolutionFiles:      formatConflictFileList(ext.Files),
		metaKeyConflictResolutionLastError:  trimmedLastErr,
		metaKeyConflictResolutionGatePassed: strconv.FormatBool(ext.GatePassed),
		metaKeyConflictResolutionPath:       strings.TrimSpace(ext.ResolutionPath),
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
	meta ConflictResolutionMeta,
) (bool, error) {
	now := time.Now().UTC()
	meta.GatePassed = false
	meta.ResolutionPath = conflictResolutionLayerHuman
	if err := c.persistConflictResolutionMetadata(task, conflictResolutionLayerHuman, attempts, trimPRConflictError(cause), now, meta); err != nil {
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
