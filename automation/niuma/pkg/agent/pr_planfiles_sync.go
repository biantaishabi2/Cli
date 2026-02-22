package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var planFilesMarkerRe = regexp.MustCompile(`<!-- PLAN_FILES:(.*?) -->`)

// syncPRPlanFilesMarker 将 PR body 中的 PLAN_FILES 与实际 diff 对齐。
// 设计目标：仅维护 marker 注释块，避免覆盖人工正文。
func (o *Orchestrator) syncPRPlanFilesMarker(ctx context.Context, prNumber int, finalPlanBody string) error {
	pr, err := o.github.GetPR(ctx, prNumber)
	if err != nil {
		return fmt.Errorf("读取 PR 失败: %w", err)
	}
	prBody := pr.GetBody()
	files, err := o.github.ListPRFiles(ctx, prNumber)
	if err != nil {
		return fmt.Errorf("读取 PR 文件列表失败: %w", err)
	}
	if len(files) == 0 {
		return nil
	}

	// 合并来源：PR 现有 marker -> 最终方案 marker -> PR 实际文件
	existing := ParseFileChangesFromComment(prBody)
	fromFinal := ParseFileChangesFromComment(finalPlanBody)

	merged := make(map[string]FileChange)
	order := make([]string, 0, len(existing)+len(fromFinal)+len(files))
	addIfMissing := func(fc FileChange) {
		path := strings.TrimSpace(fc.Path)
		if path == "" {
			return
		}
		if _, ok := merged[path]; !ok {
			order = append(order, path)
		}
		merged[path] = fc
	}
	for _, fc := range existing {
		addIfMissing(fc)
	}
	for _, fc := range fromFinal {
		addIfMissing(fc)
	}

	missing := make([]string, 0)
	for _, f := range files {
		path := strings.TrimSpace(f.Filename)
		if path == "" {
			continue
		}
		if _, ok := merged[path]; ok {
			continue
		}
		missing = append(missing, path)
		addIfMissing(FileChange{
			Path:        path,
			Action:      statusToPlanAction(f.Status),
			Description: fmt.Sprintf("自动同步：根据 PR 实际改动补齐（status=%s）", strings.TrimSpace(f.Status)),
		})
	}
	if len(missing) == 0 && len(existing) > 0 {
		return nil
	}

	// 稳定顺序：已有顺序优先，新增按路径排序。
	sort.Strings(missing)
	if len(missing) > 0 {
		baseSet := make(map[string]struct{}, len(order)-len(missing))
		for _, p := range order {
			baseSet[p] = struct{}{}
		}
		filtered := make([]string, 0, len(order))
		for _, p := range order {
			if containsString(missing, p) {
				continue
			}
			filtered = append(filtered, p)
		}
		for _, p := range missing {
			if _, ok := baseSet[p]; ok {
				filtered = append(filtered, p)
			}
		}
		order = filtered
	}

	changes := make([]FileChange, 0, len(order))
	for _, p := range order {
		changes = append(changes, merged[p])
	}

	updatedBody, changed, err := upsertPlanFilesComment(prBody, changes)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if err := o.github.UpdatePRBody(ctx, prNumber, updatedBody); err != nil {
		return fmt.Errorf("更新 PR PLAN_FILES 失败: %w", err)
	}

	_, _ = o.github.AddComment(ctx, prNumber,
		fmt.Sprintf("## 🧩 PLAN_FILES 已自动同步\n\n已按实际 diff 自动补齐 %d 个文件到 PR 描述中的 `PLAN_FILES`。", len(missing)))
	return nil
}

func upsertPlanFilesComment(prBody string, changes []FileChange) (string, bool, error) {
	if len(changes) == 0 {
		return prBody, false, nil
	}
	raw, err := json.Marshal(changes)
	if err != nil {
		return "", false, fmt.Errorf("序列化 PLAN_FILES 失败: %w", err)
	}
	marker := fmt.Sprintf("<!-- PLAN_FILES:%s -->", string(raw))

	if planFilesMarkerRe.MatchString(prBody) {
		next := planFilesMarkerRe.ReplaceAllString(prBody, marker)
		return next, next != prBody, nil
	}

	trimmed := strings.TrimRight(prBody, "\n")
	next := strings.TrimSpace(trimmed + "\n\n" + marker + "\n")
	return next, next != prBody, nil
}

func statusToPlanAction(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "added":
		return "new"
	case "removed":
		return "delete"
	default:
		return "modify"
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

