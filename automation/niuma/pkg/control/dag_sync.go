package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	ghpkg "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
)

type dagSyncPlan struct {
	edges        []DagEdge
	issueNums    []int
	expectedByTo map[int]map[int]struct{}
	totalEdges   int
	skippedEdges int
	hash         string
}

type dagIssueDiff struct {
	issueNum int
	add      []int
	remove   []int
}

func normalizeDagSyncConfig(cfg DagSyncConfig, repoDir string) DagSyncConfig {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 5 * time.Minute
	}
	if cfg.MaxRetry < 0 {
		cfg.MaxRetry = 3
	}
	if len(cfg.RetryBackoff) == 0 {
		cfg.RetryBackoff = []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second}
	}
	if cfg.RateLimitRPS <= 0 {
		cfg.RateLimitRPS = 10
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.SkippedEdgeThreshold <= 0 || cfg.SkippedEdgeThreshold > 1 {
		cfg.SkippedEdgeThreshold = 0.2
	}
	if cfg.StateFile == "" {
		cfg.StateFile = resolveDagSyncStateFile(repoDir)
	}
	return cfg
}

func (c *Controller) syncDagToGitHub(ctx context.Context, mode DagSyncMode, force bool, dryRun bool) (DagSyncResult, error) {
	result := DagSyncResult{
		Mode:   mode,
		Status: DagSyncStatusSkipped,
	}
	if c.taskctl == nil || c.github == nil {
		result.Status = DagSyncStatusFailed
		result.Error = "taskctl 或 github 客户端未初始化"
		result.ErrorType = ghpkg.DependencyErrorTypeUnknown
		c.logDagSyncResult(result)
			return result, fmt.Errorf("%s", result.Error)
	}

	tasks, err := c.taskctl.List("")
	if err != nil {
		result.Status = DagSyncStatusFailed
		result.ErrorType = ghpkg.DependencyErrorTypeUnknown
		result.Error = err.Error()
		c.logDagSyncResult(result)
		return result, fmt.Errorf("列出 task 失败: %w", err)
	}
	dag, err := c.taskctl.Dag()
	if err != nil {
		result.Status = DagSyncStatusFailed
		result.ErrorType = ghpkg.DependencyErrorTypeUnknown
		result.Error = err.Error()
		c.logDagSyncResult(result)
		return result, fmt.Errorf("读取 DAG 失败: %w", err)
	}

	plan := buildDagSyncPlan(tasks, dag)
	result.DagHash = plan.hash
	result.TotalEdges = plan.totalEdges
	result.SkippedEdges = plan.skippedEdges

	if plan.totalEdges > 0 && float64(plan.skippedEdges)/float64(plan.totalEdges) > c.cfg.DagSync.SkippedEdgeThreshold {
		fmt.Printf(
			"[control][dag-sync][error] skipped_edges_ratio_exceeded sync_mode=%s skipped_edges=%d total_edges=%d threshold=%.2f\n",
			mode, plan.skippedEdges, plan.totalEdges, c.cfg.DagSync.SkippedEdgeThreshold,
		)
	}

	store := c.dagSyncStore
	if store == nil {
		store = newDagSyncStateStore(c.cfg.DagSync.StateFile)
		c.dagSyncStore = store
	}
	state, loadErr := store.Load()
	if loadErr != nil {
		fmt.Printf("[control][dag-sync] 读取状态失败，已降级到内存态: %v\n", loadErr)
	}

	if !force && mode != DagSyncModeReconcile && state.LastHash == plan.hash {
		result.Status = DagSyncStatusSkipped
		c.logDagSyncResult(result)
		return result, nil
	}

	diffList, addCount, removeCount, err := c.buildDagSyncDiff(ctx, plan)
	if err != nil {
		result.Status = DagSyncStatusFailed
		result.ErrorType = ghpkg.ClassifyDependencyError(err)
		result.Error = err.Error()
		if !dryRun {
			state.FailCount++
			state.LastError = result.Error
			state.LastErrorAt = time.Now().UTC().Format(time.RFC3339)
			state.SkippedEdges = result.SkippedEdges
			if mode == DagSyncModeReconcile {
				state.LastReconcileAt = state.LastErrorAt
			}
			if saveErr := store.Save(state); saveErr != nil {
				fmt.Printf("[control][dag-sync] 持久化失败状态失败: %v\n", saveErr)
			}
		}
		c.logDagSyncResult(result)
		return result, err
	}

	if addCount == 0 && removeCount == 0 {
		if !dryRun {
			now := time.Now().UTC().Format(time.RFC3339)
			if mode == DagSyncModeReconcile {
				state.LastReconcileAt = now
				state.LastSuccessAt = now
			}
			state.LastHash = plan.hash
			state.SkippedEdges = result.SkippedEdges
			state.LastError = ""
			if saveErr := store.Save(state); saveErr != nil {
				fmt.Printf("[control][dag-sync] 持久化状态失败: %v\n", saveErr)
			}
		}
		result.Status = DagSyncStatusSkipped
		c.logDagSyncResult(result)
		return result, nil
	}

	if !dryRun {
		if err := c.applyDagSyncDiff(ctx, diffList); err != nil {
			result.Status = DagSyncStatusFailed
			result.ErrorType = ghpkg.ClassifyDependencyError(err)
			result.Error = err.Error()
			state.FailCount++
			state.LastError = result.Error
			state.LastErrorAt = time.Now().UTC().Format(time.RFC3339)
			state.SkippedEdges = result.SkippedEdges
			if mode == DagSyncModeReconcile {
				state.LastReconcileAt = state.LastErrorAt
			}
			if saveErr := store.Save(state); saveErr != nil {
				fmt.Printf("[control][dag-sync] 持久化失败状态失败: %v\n", saveErr)
			}
			c.logDagSyncResult(result)
			return result, err
		}
	}

	result.Status = DagSyncStatusSuccess
	result.AppliedAdd = addCount
	result.AppliedRemove = removeCount

	// dry-run 只做演练，不写入持久化状态，避免污染真实同步判定。
	if !dryRun {
		now := time.Now().UTC().Format(time.RFC3339)
		state.LastHash = plan.hash
		state.LastSuccessAt = now
		state.SuccessCount++
		state.LastError = ""
		state.SkippedEdges = result.SkippedEdges
		if mode == DagSyncModeReconcile {
			state.LastReconcileAt = now
		}
		if saveErr := store.Save(state); saveErr != nil {
			fmt.Printf("[control][dag-sync] 持久化成功状态失败: %v\n", saveErr)
		}
	}

	c.logDagSyncResult(result)
	return result, nil
}

func buildDagSyncPlan(tasks []Task, dag *DagGraph) dagSyncPlan {
	taskIssue := make(map[string]int, len(tasks))
	issueSet := make(map[int]struct{})
	for _, t := range tasks {
		issueNum := t.IssueNum()
		if issueNum <= 0 {
			continue
		}
		taskIssue[t.ID] = issueNum
		issueSet[issueNum] = struct{}{}
	}

	edgeSet := make(map[string]DagEdge)
	totalEdges := 0
	skippedEdges := 0
	if dag != nil {
		for _, node := range dag.Nodes {
			toIssue := taskIssue[node.ID]
			for _, depTaskID := range node.Deps {
				totalEdges++
				fromIssue := taskIssue[depTaskID]
				if toIssue <= 0 || fromIssue <= 0 {
					skippedEdges++
					continue
				}
				if fromIssue == toIssue {
					continue
				}
				key := fmt.Sprintf("%d->%d", fromIssue, toIssue)
				edgeSet[key] = DagEdge{FromIssue: fromIssue, ToIssue: toIssue}
			}
		}
	}

	edges := make([]DagEdge, 0, len(edgeSet))
	for _, edge := range edgeSet {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromIssue == edges[j].FromIssue {
			return edges[i].ToIssue < edges[j].ToIssue
		}
		return edges[i].FromIssue < edges[j].FromIssue
	})

	issueNums := make([]int, 0, len(issueSet))
	for issueNum := range issueSet {
		issueNums = append(issueNums, issueNum)
	}
	sort.Ints(issueNums)

	expectedByTo := make(map[int]map[int]struct{}, len(issueNums))
	for _, issueNum := range issueNums {
		expectedByTo[issueNum] = make(map[int]struct{})
	}
	for _, edge := range edges {
		if _, ok := expectedByTo[edge.ToIssue]; !ok {
			expectedByTo[edge.ToIssue] = make(map[int]struct{})
		}
		expectedByTo[edge.ToIssue][edge.FromIssue] = struct{}{}
	}

	dagHash := hashDagEdges(edges)
	return dagSyncPlan{
		edges:        edges,
		issueNums:    issueNums,
		expectedByTo: expectedByTo,
		totalEdges:   totalEdges,
		skippedEdges: skippedEdges,
		hash:         dagHash,
	}
}

func hashDagEdges(edges []DagEdge) string {
	data, _ := json.Marshal(edges)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (c *Controller) buildDagSyncDiff(ctx context.Context, plan dagSyncPlan) ([]dagIssueDiff, int, int, error) {
	var (
		diffList    []dagIssueDiff
		totalAdd    int
		totalRemove int
	)

	for _, issueNum := range plan.issueNums {
		var currentList []int
		err := c.withDagSyncRetry(ctx, "list blocked_by", func(callCtx context.Context) error {
			var callErr error
			currentList, callErr = c.github.ListIssueBlockedBy(callCtx, issueNum)
			return callErr
		})
		if err != nil {
			return nil, 0, 0, err
		}

		currentSet := make(map[int]struct{}, len(currentList))
		for _, dep := range currentList {
			if dep > 0 && dep != issueNum {
				currentSet[dep] = struct{}{}
			}
		}
		expectedSet := plan.expectedByTo[issueNum]

		add := make([]int, 0)
		remove := make([]int, 0)
		for dep := range expectedSet {
			if _, ok := currentSet[dep]; !ok {
				add = append(add, dep)
			}
		}
		for dep := range currentSet {
			if _, ok := expectedSet[dep]; !ok {
				remove = append(remove, dep)
			}
		}
		sort.Ints(add)
		sort.Ints(remove)

		if len(add) == 0 && len(remove) == 0 {
			continue
		}
		diffList = append(diffList, dagIssueDiff{
			issueNum: issueNum,
			add:      add,
			remove:   remove,
		})
		totalAdd += len(add)
		totalRemove += len(remove)
	}

	sort.Slice(diffList, func(i, j int) bool {
		return diffList[i].issueNum < diffList[j].issueNum
	})
	return diffList, totalAdd, totalRemove, nil
}

func (c *Controller) applyDagSyncDiff(ctx context.Context, diffList []dagIssueDiff) error {
	if len(diffList) == 0 {
		return nil
	}

	interval := time.Duration(0)
	if c.cfg.DagSync.RateLimitRPS > 0 {
		interval = time.Second / time.Duration(c.cfg.DagSync.RateLimitRPS)
	}

	wait := func() error {
		if interval <= 0 {
			return nil
		}
		timer := time.NewTimer(interval)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}

	for _, diff := range diffList {
		for _, depIssue := range diff.add {
			if err := c.withDagSyncRetry(ctx, "add blocked_by", func(callCtx context.Context) error {
				return c.github.AddIssueBlockedBy(callCtx, diff.issueNum, depIssue)
			}); err != nil {
				return err
			}
			if err := wait(); err != nil {
				return err
			}
		}
		for _, depIssue := range diff.remove {
			if err := c.withDagSyncRetry(ctx, "remove blocked_by", func(callCtx context.Context) error {
				return c.github.RemoveIssueBlockedBy(callCtx, diff.issueNum, depIssue)
			}); err != nil {
				return err
			}
			if err := wait(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Controller) withDagSyncRetry(ctx context.Context, op string, fn func(context.Context) error) error {
	maxRetries := c.cfg.DagSync.MaxRetry
	if maxRetries < 0 {
		maxRetries = 0
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.cfg.DagSync.Timeout)
		err := fn(callCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err

		errType := ghpkg.ClassifyDependencyError(err)
		retryable := shouldRetryDagSyncErrorType(errType)
		if !retryable || attempt == maxRetries {
			break
		}

		backoff := c.cfg.DagSync.RetryBackoff[minInt(attempt, len(c.cfg.DagSync.RetryBackoff)-1)]
		if backoff <= 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return wrapDagSyncError(op, ctx.Err())
		case <-time.After(backoff):
		}
	}
	return wrapDagSyncError(op, lastErr)
}

func wrapDagSyncError(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", op, err)
}

func (c *Controller) logDagSyncResult(result DagSyncResult) {
	fmt.Printf(
		"[control][dag-sync] status=%s sync_mode=%s dag_hash=%s total_edges=%d applied_add=%d applied_remove=%d skipped_edges=%d error_type=%s\n",
		result.Status,
		result.Mode,
		result.DagHash,
		result.TotalEdges,
		result.AppliedAdd,
		result.AppliedRemove,
		result.SkippedEdges,
		result.ErrorType,
	)
	if result.Error != "" {
		fmt.Printf("[control][dag-sync] error=%s\n", result.Error)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func shouldRetryDagSyncErrorType(errType string) bool {
	switch errType {
	case ghpkg.DependencyErrorTypeAuth,
		ghpkg.DependencyErrorTypePermission,
		ghpkg.DependencyErrorTypeRateLimit,
		ghpkg.DependencyErrorTypeNetworkTimeout,
		ghpkg.DependencyErrorTypeUnsupported:
		return true
	default:
		return false
	}
}

func shouldRunDagReconcile(state DagSyncState, pollInterval time.Duration) bool {
	if pollInterval <= 0 {
		return true
	}
	reference := state.LastReconcileAt
	if reference == "" {
		reference = state.LastSuccessAt
	}
	if reference == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, reference)
	if err != nil {
		return true
	}
	return time.Since(last) >= pollInterval
}
