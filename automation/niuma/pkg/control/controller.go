// pkg/control/controller.go
// Controller 核心：多 Issue 协调循环
package control

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
)

// GitHubOps 控制层需要的 GitHub 操作接口（独立于 agent 包的接口）
type GitHubOps interface {
	ListIssuesWithLabel(ctx context.Context, label string) ([]IssueInfo, error)
	ListIssuesByState(ctx context.Context, state string) ([]IssueInfo, error)
	ListLabels(ctx context.Context, issueNumber int) ([]string, error)
	AddLabel(ctx context.Context, issueNumber int, label string) error
	GetIssue(ctx context.Context, issueNumber int) (IssueInfo, error)
	UpdateIssueBody(ctx context.Context, issueNumber int, body string) error
	ListCommentBodies(ctx context.Context, issueNumber int) ([]string, error)
	AddIssueComment(ctx context.Context, issueNumber int, body string) error
	CloseIssue(ctx context.Context, issueNumber int) error
	MergePR(ctx context.Context, prNum int, method string) error
	ReplaceLabel(ctx context.Context, issueNumber int, oldLabel, newLabel string) error
	ReplaceLabelIfPresent(ctx context.Context, issueNumber int, oldLabel, newLabel string) (bool, error)
	ReplaceLabels(ctx context.Context, issueNumber int, labels []string) error
	ListIssueBlockedBy(ctx context.Context, issueNumber int) ([]int, error)
	AddIssueBlockedBy(ctx context.Context, issueNumber int, blockedByIssueNumber int) error
	RemoveIssueBlockedBy(ctx context.Context, issueNumber int, blockedByIssueNumber int) error
	ResolvePRMetadata(ctx context.Context, issueNumber int) (PRMetadata, error)
	ResolvePRReviewStatus(ctx context.Context, issueNumber int) (PRReviewStatus, error)
}

// PRMetadata 表示用于 integration 候选筛选的最小 PR 元数据。
type PRMetadata struct {
	PRNum  int
	Branch string
}

// PRMergeable 表示 PR mergeable 字段的归一化结果。
type PRMergeable string

const (
	PRMergeableUnknown     PRMergeable = "UNKNOWN"
	PRMergeableMergeable   PRMergeable = "MERGEABLE"
	PRMergeableConflicting PRMergeable = "CONFLICTING"
)

// PRReviewStatus 表示 pr-reviewable 协调所需的 PR 状态快照。
type PRReviewStatus struct {
	PRNum            int
	HeadSHA          string
	Mergeable        PRMergeable
	MergeStateStatus string
}

func (s PRReviewStatus) normalizedMergeStateStatus() string {
	status := strings.ToUpper(strings.TrimSpace(s.MergeStateStatus))
	if status == "" {
		return "UNKNOWN"
	}
	return status
}

func (s PRReviewStatus) IsConflicting() bool {
	status := s.normalizedMergeStateStatus()
	return s.Mergeable == PRMergeableConflicting || status == "DIRTY" || status == "BLOCKED"
}

func (s PRReviewStatus) IsUnknown() bool {
	if s.IsConflicting() {
		return false
	}
	status := s.normalizedMergeStateStatus()
	return s.Mergeable == PRMergeableUnknown && status == "UNKNOWN"
}

func (s PRReviewStatus) IsMergeable() bool {
	if s.IsConflicting() || s.IsUnknown() {
		return false
	}
	status := s.normalizedMergeStateStatus()
	if status == "CLEAN" {
		return true
	}
	return s.Mergeable == PRMergeableMergeable
}

var (
	// ErrPRMarkerNotFound 表示 issue 上未找到 BOT:PR_CREATED marker。
	ErrPRMarkerNotFound = errors.New("pr marker not found")
	// ErrPRClosed 表示 marker 指向的 PR 已关闭（含已合并）。
	ErrPRClosed = errors.New("pr is closed")
	// ErrPRBranchUnavailable 表示 PR head branch 不可用。
	ErrPRBranchUnavailable = errors.New("pr branch unavailable")
)

const (
	integrationConflictLabel = "integration-conflict"
	integrationGateFailLabel = "integration-gate-failed"
	needsHumanLabel          = "needs-human"
	botDoneLabel             = "bot:done"

	metaKeyIntegrated                     = "integrated"
	metaKeyIntegrationMergeStatus         = "integration_merge_status"
	metaKeyIntegrationMergeExecutedAt     = "integration_merge_executed_at"
	metaKeyIntegrationExecutorVersion     = "integration_executor_version"
	metaKeyIntegrationAutoResolvedFiles   = "integration_auto_resolved_files"
	metaKeyIntegrationConflictSummary     = "integration_conflict_summary"
	metaKeyIntegrationConflictFiles       = "integration_conflict_files"
	metaKeyIntegrationConflictTotalHunks  = "integration_conflict_total_hunks"
	metaKeyIntegrationConflictReason      = "integration_conflict_reason"
	metaKeyIntegrationConflictSuggestion  = "integration_conflict_suggestion"
	metaKeyIntegrationConflictRecordedAt  = "integration_conflict_recorded_at"
	metaKeyIntegrationConflictLabelSynced = "integration_conflict_labeled"
	metaKeyEscalatedAt                    = "escalated_at"

	metaKeyIntegrationGateStatus                = "integration_gate_status"
	metaKeyIntegrationGateRetryCount            = "integration_gate_retry_count"
	metaKeyIntegrationGateLastError             = "integration_gate_last_error"
	metaKeyIntegrationGateLastCheckedAt         = "integration_gate_last_checked_at"
	metaKeyIntegrationGateAttemptKey            = "integration_gate_attempt_key"
	metaKeyIntegrationGateEscalationLabelSynced = "integration_gate_escalation_labeled"

	integrationGateStatusPending   = "pending"
	integrationGateStatusPassed    = "passed"
	integrationGateStatusRetrying  = "retrying"
	integrationGateStatusEscalated = "escalated"

	integrationGateDefaultMaxRetries         = 2
	integrationGateErrorLimit                = 800
	prConflictRetryDefaultThreshold          = 3
	integrationConflictRetryDefaultThreshold = 3
	prConflictAIDefaultMaxAttempts           = 2

	issueLockDefaultTTL       = 5 * time.Minute
	issueLockDefaultHeartbeat = 100 * time.Second

	metaKeyTaskRepo      = "repo"
	metaKeyTaskPhase     = "phase"
	metaKeyTaskInputHash = "input_hash"

	metadataSyncSkipMarkerNotFound    = "marker_not_found"
	metadataSyncSkipPRClosed          = "pr_closed"
	metadataSyncSkipBranchUnavailable = "branch_unavailable"
	metadataSyncSkipAlreadyUpToDate   = "already_up_to_date"
)

var (
	prConflictRetryMarkerRe             = regexp.MustCompile(`<!--\s*PR_CONFLICT_RETRY:(\d+)\s*-->`)
	integrationConflictRetryMarkerRe    = regexp.MustCompile(`<!--\s*INTEGRATION_CONFLICT_RETRY:(\d+)\s*-->`)
	defaultPRConflictUnknownBackoffs    = []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
	prConflictDetectedCommentMarkerFmt  = "<!-- BOT:CONFLICT_DETECTED sha:%s -->"
	prConflictEscalatedCommentMarkerFmt = "<!-- BOT:CONFLICT_ESCALATED sha:%s -->"
	integrationConflictCommentMarker    = "<!-- BOT:INTEGRATION_CONFLICT_RETRY -->"
	needsHumanRecoveryCooldown          = 5 * time.Minute
)

// Controller 多 Issue 协调控制器
type Controller struct {
	taskctl            *TaskCtlClient
	analyzer           *DependencyAnalyzer
	github             GitHubOps
	builder            *IntegrationBuilder
	cfg                *ControlConfig
	dagSyncStore       *dagSyncStateStore
	issueLocks         IssueLockStore
	issueLockTTL       time.Duration
	issueLockHeartbeat time.Duration
	nowFn              func() time.Time
	ownerID            string
}

// ControlConfig 控制层配置
type ControlConfig struct {
	TaskCtlBin                 string `yaml:"taskctl_bin"`
	MergeStrategy              string `yaml:"merge_strategy"`            // merge/squash，默认 merge
	IntegrationBranchPrefix    string `yaml:"integration_branch_prefix"` // 默认 integration/
	MaxOldBranches             int    `yaml:"max_old_branches"`          // 默认 3
	MinPRsForIntegration       int    `yaml:"min_prs_for_integration"`   // 默认 2
	IntegrationGateMaxRetries  int    `yaml:"integration_gate_max_retries"`
	DagSync                    DagSyncConfig
	PRConflictRetryThreshold   int
	PRConflictUnknownBackoffs  []time.Duration
	PRConflictEnableAI         bool
	PRConflictAIMaxAttempts    int
	PRConflictSmokeTestCmd     string
	PRConflictElixirTestCmd    string `yaml:"pr_conflict_elixir_test_cmd"`
	PRConflictProfile          string
	RepoDir                    string           `yaml:"-"`
	IssueLockTTL               time.Duration    `yaml:"-"`
	IssueLockHeartbeatInterval time.Duration    `yaml:"-"`
	IssueLockStore             IssueLockStore   `yaml:"-"`
	NowFn                      func() time.Time `yaml:"-"`
	OwnerID                    string           `yaml:"-"`
}

// DefaultControlConfig 返回默认配置
func DefaultControlConfig() *ControlConfig {
	return &ControlConfig{
		MergeStrategy:             "merge",
		IntegrationBranchPrefix:   "integration/",
		MaxOldBranches:            3,
		MinPRsForIntegration:      2,
		IntegrationGateMaxRetries: integrationGateDefaultMaxRetries,
		DagSync: DagSyncConfig{
			PollInterval:         5 * time.Minute,
			MaxRetry:             3,
			RetryBackoff:         []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second},
			RateLimitRPS:         10,
			Timeout:              30 * time.Second,
			SkippedEdgeThreshold: 0.2,
		},
		PRConflictRetryThreshold:   prConflictRetryDefaultThreshold,
		PRConflictUnknownBackoffs:  append([]time.Duration(nil), defaultPRConflictUnknownBackoffs...),
		PRConflictEnableAI:         true,
		PRConflictAIMaxAttempts:    prConflictAIDefaultMaxAttempts,
		PRConflictElixirTestCmd:    "mix test",
		PRConflictProfile:          "auto",
		RepoDir:                    ".",
		IssueLockTTL:               issueLockDefaultTTL,
		IssueLockHeartbeatInterval: issueLockDefaultHeartbeat,
		OwnerID:                    defaultIssueLockOwnerID(),
	}
}

func defaultIssueLockOwnerID() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", hostname, os.Getpid())
}

// NewController 创建 Controller
func NewController(
	taskctl *TaskCtlClient,
	analyzer *DependencyAnalyzer,
	github GitHubOps,
	builder *IntegrationBuilder,
	cfg *ControlConfig,
) *Controller {
	if cfg == nil {
		cfg = DefaultControlConfig()
	}
	if cfg.RepoDir == "" {
		cfg.RepoDir = "."
	}
	if cfg.IntegrationGateMaxRetries < 0 {
		cfg.IntegrationGateMaxRetries = integrationGateDefaultMaxRetries
	}
	cfg.DagSync = normalizeDagSyncConfig(cfg.DagSync, cfg.RepoDir)
	if cfg.PRConflictRetryThreshold < 0 {
		cfg.PRConflictRetryThreshold = prConflictRetryDefaultThreshold
	}
	if len(cfg.PRConflictUnknownBackoffs) == 0 {
		cfg.PRConflictUnknownBackoffs = append([]time.Duration(nil), defaultPRConflictUnknownBackoffs...)
	}
	if cfg.PRConflictAIMaxAttempts <= 0 {
		cfg.PRConflictAIMaxAttempts = prConflictAIDefaultMaxAttempts
	}
	if cfg.IssueLockTTL <= 0 {
		cfg.IssueLockTTL = issueLockDefaultTTL
	}
	if cfg.IssueLockHeartbeatInterval <= 0 {
		cfg.IssueLockHeartbeatInterval = issueLockDefaultHeartbeat
	}
	if cfg.IssueLockHeartbeatInterval >= cfg.IssueLockTTL {
		cfg.IssueLockHeartbeatInterval = cfg.IssueLockTTL / 3
		if cfg.IssueLockHeartbeatInterval <= 0 {
			cfg.IssueLockHeartbeatInterval = time.Second
		}
	}
	if strings.TrimSpace(cfg.OwnerID) == "" {
		cfg.OwnerID = defaultIssueLockOwnerID()
	}
	if cfg.NowFn == nil {
		cfg.NowFn = time.Now
	}
	if cfg.IssueLockStore == nil {
		cfg.IssueLockStore = newInMemoryIssueLockStore()
	}
	return &Controller{
		taskctl:            taskctl,
		analyzer:           analyzer,
		github:             github,
		builder:            builder,
		cfg:                cfg,
		dagSyncStore:       newDagSyncStateStore(cfg.DagSync.StateFile),
		issueLocks:         cfg.IssueLockStore,
		issueLockTTL:       cfg.IssueLockTTL,
		issueLockHeartbeat: cfg.IssueLockHeartbeatInterval,
		nowFn:              cfg.NowFn,
		ownerID:            cfg.OwnerID,
	}
}

type inMemoryIssueLockStore struct {
	mu    sync.Mutex
	locks map[int]IssueLockRecord
}

func newInMemoryIssueLockStore() IssueLockStore {
	return &inMemoryIssueLockStore{
		locks: make(map[int]IssueLockRecord),
	}
}

func (s *inMemoryIssueLockStore) TryAcquire(issueNumber int, owner string, now time.Time, ttl time.Duration) (IssueLockRecord, bool, error) {
	if issueNumber <= 0 {
		return IssueLockRecord{}, false, fmt.Errorf("issue 编号无效: %d", issueNumber)
	}
	if owner == "" {
		return IssueLockRecord{}, false, fmt.Errorf("owner 不能为空")
	}
	if ttl <= 0 {
		return IssueLockRecord{}, false, fmt.Errorf("ttl 必须大于 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.locks[issueNumber]
	if ok && record.Owner != "" && now.Before(record.ExpiresAt) {
		return record, false, nil
	}

	lastResult := record.LastResult
	record = IssueLockRecord{
		IssueNumber: issueNumber,
		Owner:       owner,
		AcquiredAt:  now,
		ExpiresAt:   now.Add(ttl),
		HeartbeatAt: now,
		LastResult:  lastResult,
	}
	s.locks[issueNumber] = record
	return record, true, nil
}

func (s *inMemoryIssueLockStore) Refresh(issueNumber int, owner string, now time.Time, ttl time.Duration) (IssueLockRecord, error) {
	if issueNumber <= 0 {
		return IssueLockRecord{}, fmt.Errorf("issue 编号无效: %d", issueNumber)
	}
	if owner == "" {
		return IssueLockRecord{}, fmt.Errorf("owner 不能为空")
	}
	if ttl <= 0 {
		return IssueLockRecord{}, fmt.Errorf("ttl 必须大于 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.locks[issueNumber]
	if !ok || record.Owner == "" {
		return IssueLockRecord{}, fmt.Errorf("issue #%d 未持锁", issueNumber)
	}
	if record.Owner != owner {
		return IssueLockRecord{}, fmt.Errorf("issue #%d 锁 owner 不匹配: current=%s expect=%s", issueNumber, record.Owner, owner)
	}
	if !now.Before(record.ExpiresAt) {
		return IssueLockRecord{}, fmt.Errorf("issue #%d 锁已过期: expires_at=%s", issueNumber, record.ExpiresAt.Format(time.RFC3339))
	}

	record.HeartbeatAt = now
	record.ExpiresAt = now.Add(ttl)
	s.locks[issueNumber] = record
	return record, nil
}

func (s *inMemoryIssueLockStore) Release(issueNumber int, owner string, now time.Time, lastResult IssueLockResult) error {
	if issueNumber <= 0 {
		return nil
	}
	if owner == "" {
		return fmt.Errorf("owner 不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.locks[issueNumber]
	if !ok {
		return nil
	}
	if record.Owner != "" && record.Owner != owner {
		return fmt.Errorf("issue #%d 锁 owner 不匹配: current=%s expect=%s", issueNumber, record.Owner, owner)
	}

	record.Owner = ""
	record.HeartbeatAt = now
	record.ExpiresAt = now
	record.LastResult = lastResult
	s.locks[issueNumber] = record
	return nil
}

func (c *Controller) controllerNow() time.Time {
	if c.nowFn == nil {
		return time.Now().UTC()
	}
	return c.nowFn().UTC()
}

func (c *Controller) lockOwnerID() string {
	if strings.TrimSpace(c.ownerID) == "" {
		return defaultIssueLockOwnerID()
	}
	return c.ownerID
}

func (c *Controller) lockTTL() time.Duration {
	if c.issueLockTTL <= 0 {
		return issueLockDefaultTTL
	}
	return c.issueLockTTL
}

func (c *Controller) tryAcquireIssueLock(issue IssueInfo) (bool, IssueLockRecord, error) {
	if issue.Number <= 0 || c.issueLocks == nil {
		return true, IssueLockRecord{}, nil
	}

	record, acquired, err := c.issueLocks.TryAcquire(issue.Number, c.lockOwnerID(), c.controllerNow(), c.lockTTL())
	if err != nil {
		return false, IssueLockRecord{}, err
	}
	return acquired, record, nil
}

func (c *Controller) refreshIssueLock(issue IssueInfo) error {
	if issue.Number <= 0 || c.issueLocks == nil {
		return nil
	}
	_, err := c.issueLocks.Refresh(issue.Number, c.lockOwnerID(), c.controllerNow(), c.lockTTL())
	return err
}

func (c *Controller) releaseIssueLock(issue IssueInfo, lastResult IssueLockResult) error {
	if issue.Number <= 0 || c.issueLocks == nil {
		return nil
	}
	return c.issueLocks.Release(issue.Number, c.lockOwnerID(), c.controllerNow(), lastResult)
}

func (c *Controller) withIssueLock(ctx context.Context, issue IssueInfo, fn func(context.Context) error) (retErr error) {
	if fn == nil {
		return nil
	}

	acquired, current, err := c.tryAcquireIssueLock(issue)
	if err != nil {
		return err
	}
	if !acquired {
		fmt.Printf(
			"[control][issue_lock] issue=%d status=%s reason=%s owner=%s lock_owner=%s expires_at=%s\n",
			issue.Number,
			IssueLockResultSkipped,
			IssueLockResultLocked,
			c.lockOwnerID(),
			current.Owner,
			current.ExpiresAt.Format(time.RFC3339),
		)
		return nil
	}

	lastResult := IssueLockResultSucceeded
	defer func() {
		if releaseErr := c.releaseIssueLock(issue, lastResult); releaseErr != nil {
			fmt.Printf(
				"[control][issue_lock] issue=%d status=%s reason=release_failed owner=%s err=%v\n",
				issue.Number,
				IssueLockResultFailed,
				c.lockOwnerID(),
				releaseErr,
			)
			if retErr == nil {
				retErr = releaseErr
			}
		}
	}()

	lockCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	heartbeatErrCh := make(chan error, 1)
	stopHeartbeat := make(chan struct{})
	defer close(stopHeartbeat)
	if c.issueLockHeartbeat > 0 {
		go func() {
			ticker := time.NewTicker(c.issueLockHeartbeat)
			defer ticker.Stop()
			for {
				select {
				case <-stopHeartbeat:
					return
				case <-lockCtx.Done():
					return
				case <-ticker.C:
					if refreshErr := c.refreshIssueLock(issue); refreshErr != nil {
						select {
						case heartbeatErrCh <- refreshErr:
						default:
						}
						cancel()
						return
					}
				}
			}
		}()
	}

	runErr := fn(lockCtx)
	var heartbeatErr error
	select {
	case heartbeatErr = <-heartbeatErrCh:
	default:
	}
	if heartbeatErr != nil {
		lastResult = IssueLockResultFailed
		fmt.Printf(
			"[control][issue_lock] issue=%d status=%s reason=heartbeat_refresh_failed owner=%s err=%v\n",
			issue.Number,
			IssueLockResultFailed,
			c.lockOwnerID(),
			heartbeatErr,
		)
		if runErr == nil || runErr == context.Canceled {
			runErr = fmt.Errorf("issue #%d 锁心跳刷新失败: %w", issue.Number, heartbeatErr)
		}
	}
	if runErr != nil {
		lastResult = IssueLockResultFailed
		return runErr
	}
	return nil
}

type processIssueIdempotencyContext struct {
	Repo      string
	IssueNum  int
	Phase     string
	InputHash string
	Key       string
}

func buildIssueIdempotencyKey(repo string, issueNum int, phase, inputHash string) string {
	payload := strings.Join([]string{
		strings.TrimSpace(repo),
		strconv.Itoa(issueNum),
		strings.TrimSpace(phase),
		strings.TrimSpace(inputHash),
	}, "\n")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func buildProcessIssueIdempotencyContext(task Task) (processIssueIdempotencyContext, bool) {
	issueNum := task.IssueNum()
	if issueNum <= 0 {
		return processIssueIdempotencyContext{}, false
	}

	phase := strings.TrimSpace(valueOrEmpty(task.Metadata, metaKeyTaskPhase))
	inputHash := strings.TrimSpace(valueOrEmpty(task.Metadata, metaKeyTaskInputHash))
	if phase == "" || inputHash == "" {
		return processIssueIdempotencyContext{}, false
	}

	repo := strings.TrimSpace(valueOrEmpty(task.Metadata, metaKeyTaskRepo))
	return processIssueIdempotencyContext{
		Repo:      repo,
		IssueNum:  issueNum,
		Phase:     phase,
		InputHash: inputHash,
		Key:       buildIssueIdempotencyKey(repo, issueNum, phase, inputHash),
	}, true
}

func (c *Controller) loadLatestTaskMetadata(task Task) (map[string]string, error) {
	if c == nil || c.taskctl == nil || strings.TrimSpace(task.ID) == "" {
		return task.Metadata, nil
	}

	latestTask, err := c.taskctl.Get(task.ID)
	if err != nil {
		return nil, err
	}
	if latestTask == nil {
		return task.Metadata, nil
	}
	if strings.TrimSpace(latestTask.ID) == "" || latestTask.ID != task.ID {
		return task.Metadata, nil
	}
	return latestTask.Metadata, nil
}

// ProcessIssue 推进单个 issue 的控制流程（主入口）。
func (c *Controller) ProcessIssue(ctx context.Context, task Task) error {
	issue := IssueInfo{
		Number: task.IssueNum(),
		Title:  task.Subject,
	}

	return c.withIssueLock(ctx, issue, func(runCtx context.Context) error {
		idempotencyContext, enableIdempotency := buildProcessIssueIdempotencyContext(task)
		idempotencyMetadata := task.Metadata
		if enableIdempotency {
			var err error
			idempotencyMetadata, err = c.loadLatestTaskMetadata(task)
			if err != nil {
				return fmt.Errorf("读取任务最新 metadata 失败 (task %s): %w", task.ID, err)
			}
			if latestKey := readPhaseIdempotencyKey(idempotencyMetadata, idempotencyContext.Phase); latestKey == idempotencyContext.Key {
				fmt.Printf(
					"[control][idempotency] repo=%s issue=%d phase=%s key=%s action=no-op\n",
					idempotencyContext.Repo,
					idempotencyContext.IssueNum,
					idempotencyContext.Phase,
					idempotencyContext.Key,
				)
				return nil
			}
		}

		status := TaskStatusInProgress
		if err := c.taskctl.Update(task.ID, UpdateOpts{Status: &status}); err != nil {
			return fmt.Errorf("更新任务状态失败 (task %s): %w", task.ID, err)
		}

		issueNum := task.IssueNum()
		if issueNum <= 0 {
			return nil
		}

		// 严格按状态机迁移：仅允许 bot:queued -> bot:fix。
		if err := c.transitionWithSelfHeal(runCtx, issueNum, state.StateQueued, state.StateFixRequested); err != nil {
			fmt.Printf("[control] 迁移状态失败 (issue #%d): %v\n", issueNum, err)
			return nil
		}

		fmt.Printf("[control] 已将 issue #%d 标签 bot:queued → bot:fix\n", issueNum)
		if !enableIdempotency {
			return nil
		}

		update, err := c.taskctl.recordPhaseIdempotency(
			task.ID,
			idempotencyMetadata,
			idempotencyContext.Phase,
			idempotencyContext.Key,
			idempotencyContext.InputHash,
			c.controllerNow(),
		)
		if err != nil {
			return fmt.Errorf("写入幂等 metadata 失败 (task %s): %w", task.ID, err)
		}
		task.Metadata = mergeMetadataPatch(task.Metadata, update)
		fmt.Printf(
			"[control][idempotency] repo=%s issue=%d phase=%s key=%s action=recorded\n",
			idempotencyContext.Repo,
			idempotencyContext.IssueNum,
			idempotencyContext.Phase,
			idempotencyContext.Key,
		)
		return nil
	})
}

// getIntegrationBranchName 获取当前任务的 integration 分支名
// 从 task metadata 读取 meta_issue_slug，没有则使用 "main"
func (c *Controller) getIntegrationBranchName(task *Task) string {
	slug := ""
	if task.Metadata != nil {
		slug = task.Metadata["meta_issue_slug"]
	}
	return IntegrationBranchName(slug)
}

// Run 执行一次完整协调循环
func (c *Controller) Run(ctx context.Context) error {
	// ① intake：仅扫描 bot:orchestrate 新入口。
	intakeIssues, orchestrateCount, err := c.collectAutomationIssues(ctx)
	if err != nil {
		return fmt.Errorf("扫描自动化 issues 失败: %w", err)
	}

	if orchestrateCount == 0 {
		fmt.Println("[control] 没有发现新的 bot:orchestrate issues，将继续推进已有任务")
	} else {
		fmt.Printf("[control] 发现 %d 个 issues (bot:orchestrate)\n", orchestrateCount)
	}

	// ② 读取 taskctl store，并建立 issue->task 索引。
	existingTasks, err := c.taskctl.List("")
	if err != nil {
		// 区分 store 不存在（首次运行）和真实错误
		if strings.Contains(err.Error(), "no such file") || strings.Contains(err.Error(), "not found") {
			existingTasks = nil
		} else {
			return fmt.Errorf("列出现有任务失败: %w", err)
		}
	}

	issueToTask := make(map[int]string)       // issueNum → taskID
	activeIssueToTask := make(map[int]string) // issueNum(active) → taskID
	for _, t := range existingTasks {
		if n := t.IssueNum(); n > 0 {
			issueToTask[n] = t.ID
			if isTaskActiveStatus(t.Status) {
				activeIssueToTask[n] = t.ID
			}
		}
	}

	// ②.5 缓存全量 issues，供 hydrate 和 ⑦ 步复用。
	// 当 hydrate 被禁用时，延迟到 ⑦ 步再调用，回退到原有执行时序。
	hydrateDisabled := os.Getenv("NIUMA_DISABLE_HYDRATE") == "1"
	var allIssuesCache []IssueInfo
	var hydratedEntries []intakeIssueEntry
	if !hydrateDisabled {
		allIssuesCache, err = c.github.ListIssuesByState(ctx, "all")
		if err != nil {
			return fmt.Errorf("扫描全量 issues 失败: %w", err)
		}

		// ②.6 hydrate：从 GitHub 重建跨 workflow run 丢失的 task 状态。
		hydratedEntries = c.hydrateTasksFromGitHub(ctx, allIssuesCache, activeIssueToTask, issueToTask)
	} else {
		fmt.Println("[control][hydrate] disabled by NIUMA_DISABLE_HYDRATE=1, skipping early ListIssuesByState")
	}

	// ③ 统一依赖分析：显式 depends-on 优先，AI 仅补全未声明项。
	// 注意：parent 仅表示结构归属/收口关系，不隐式注入执行依赖边。
	// 将 intake 和 hydrated issues 合并到依赖分析输入中，使用 intakeIssueEntry 标记来源。
	intakeEntries := make([]intakeIssueEntry, 0, len(intakeIssues))
	for _, issue := range intakeIssues {
		intakeEntries = append(intakeEntries, intakeIssueEntry{IssueInfo: issue, FromIntake: true})
	}
	allEntries := make([]intakeIssueEntry, 0, len(intakeEntries)+len(hydratedEntries))
	allEntries = append(allEntries, intakeEntries...)
	allEntries = append(allEntries, hydratedEntries...)
	allAnalysisIssues := make([]IssueInfo, 0, len(allEntries))
	for _, e := range allEntries {
		allAnalysisIssues = append(allAnalysisIssues, e.IssueInfo)
	}
	analysis := c.buildDependencyAnalysis(ctx, allAnalysisIssues)

	// ④ intake 入队：原子去重创建 active task，并将 orchestrate 迁移为 queued。
	// 只处理 FromIntake==true 的 entries，hydrate 来源跳过 label 迁移。
	for _, entry := range allEntries {
		if !entry.FromIntake {
			continue
		}
		issue := entry.IssueInfo
		if _, ok := activeIssueToTask[issue.Number]; ok {
			fmt.Printf("[control] action=intake_reused issue_num=%d task_id=%s reason=active_task_exists\n", issue.Number, activeIssueToTask[issue.Number])
		}
		meta := map[string]string{
			"issue_num": strconv.Itoa(issue.Number),
		}
		task, reused, err := c.taskctl.CreateOrReuseActiveIssueTask(issue.Title, issue.Body, meta)
		if err != nil {
			fmt.Printf("[control] 创建任务失败 (issue #%d): %v\n", issue.Number, err)
			continue
		}
		issueToTask[issue.Number] = task.ID
		activeIssueToTask[issue.Number] = task.ID
		if reused {
			fmt.Printf("[control] action=intake_reused issue_num=%d task_id=%s reason=deduplicated\n", issue.Number, task.ID)
		} else {
			fmt.Printf("[control] 创建任务 %s (issue #%d)\n", task.ID, issue.Number)
		}

		if err := c.transitionWithSelfHeal(ctx, issue.Number, state.StateOrchestrate, state.StateQueued); err != nil {
			if errors.Is(err, state.ErrInvariantViolation) || errors.Is(err, state.ErrMultipleBotStates) {
				fmt.Printf("[control] action=intake_blocked issue_num=%d reason=dirty_multi_state err=%v\n", issue.Number, err)
				continue
			}
			fmt.Printf("[control] 迁移状态失败 (issue #%d): %v\n", issue.Number, err)
		} else {
			fmt.Printf("[control] 已将 issue #%d 标签 bot:orchestrate → bot:queued\n", issue.Number)
		}
	}

	// ⑤ 先落盘 blocked_by，再判定 ready（流程不变量门禁）。
	// DAG 是依赖调度的唯一真相（SSOT），ready/blocking 只读 taskctl DAG + metadata。
	blockedByPersisted := c.persistBlockedByDependencies(issueToTask, analysis.Dependencies)

	// ⑤.1 DAG -> GitHub 依赖展示镜像（单向）；失败降级为日志，不阻塞主流程。
	if err := c.runDagSyncEvent(ctx); err != nil {
		fmt.Printf("[control][dag-sync] event 同步失败（已降级，不阻塞主流程）: %v\n", err)
	}

	// ⑥ 获取 ready tasks 并推进
	if !blockedByPersisted {
		fmt.Println("[control] blocked_by 落盘未完成，跳过本轮 ready 放行，等待下轮重试")
	} else {
		readyTasks, err := c.taskctl.Ready()
		if err != nil {
			fmt.Printf("[control] 获取 ready tasks 失败: %v\n", err)
		} else {
			for _, task := range readyTasks {
				issueNum := task.IssueNum()
				fmt.Printf("[control] 推进 ready task %s (issue #%d)\n", task.ID, issueNum)
				if err := c.ProcessIssue(ctx, task); err != nil {
					fmt.Printf("[control] 推进 ready task 失败 (task %s): %v\n", task.ID, err)
					continue
				}
			}
		}
	}

	// ⑦ 增量 integration：将刚完成的 PR 合入对应 integration 分支
	if c.builder != nil {
		allTasks, err := c.taskctl.List("")
		if err != nil {
			return fmt.Errorf("列出现有任务失败: %w", err)
		}

		// 延迟加载：hydrate 禁用时在此处才调用 ListIssuesByState
		if allIssuesCache == nil {
			allIssuesCache, err = c.github.ListIssuesByState(ctx, "all")
			if err != nil {
				return fmt.Errorf("扫描全量 issues 失败: %w", err)
			}
		}

		issueByNumber := make(map[int]IssueInfo, len(allIssuesCache))
		for _, issue := range allIssuesCache {
			issueByNumber[issue.Number] = issue
		}
		if err := c.syncPRReviewableMetadata(ctx, allTasks, issueByNumber); err != nil {
			return fmt.Errorf("同步 PR 元数据失败: %w", err)
		}
		if err := c.reconcilePRReviewableConflicts(ctx, allTasks, issueByNumber); err != nil {
			return fmt.Errorf("协调 pr-reviewable 冲突失败: %w", err)
		}

		// 按 integration 分支分组 task
		branchTasks := make(map[string][]Task)              // integrationBranch → 待合入 tasks
		escalationRetryTasks := make(map[string][]Task)     // integrationBranch → merge 冲突升级待补打标签 tasks
		gateEscalationRetryTasks := make(map[string][]Task) // integrationBranch → gate 升级待补打标签 tasks
		for _, t := range allTasks {
			if t.PRNum() > 0 && t.Branch() != "" {
				// 检查是否已合入 integration（从 metadata 读）
				integrated := false
				if t.Metadata != nil && t.Metadata[metaKeyIntegrated] == "true" {
					integrated = true
				}
				if integrated {
					continue
				}

				branchName := c.getIntegrationBranchName(&t)
				if shouldRetryEscalationLabels(t.Metadata) {
					escalationRetryTasks[branchName] = append(escalationRetryTasks[branchName], t)
					continue
				}
				if shouldRetryIntegrationGateEscalationLabels(t.Metadata) {
					gateEscalationRetryTasks[branchName] = append(gateEscalationRetryTasks[branchName], t)
					continue
				}

				if isEscalatedIntegrationTask(t.Metadata) && isEscalationLabelSynced(t.Metadata) {
					fmt.Printf("[control] 跳过已升级人工且已完成打标 task %s (issue #%d)\n", t.ID, t.IssueNum())
					continue
				}
				if isEscalatedIntegrationGateTask(t.Metadata) && isIntegrationGateEscalationLabelSynced(t.Metadata) {
					fmt.Printf("[control] 跳过 gate 已升级人工 task %s (issue #%d)\n", t.ID, t.IssueNum())
					continue
				}

				if !c.shouldEnqueueIntegrationMergeTask(ctx, t, issueByNumber) {
					continue
				}
				branchTasks[branchName] = append(branchTasks[branchName], t)
			}
		}

		for branchName, tasks := range escalationRetryTasks {
			if len(tasks) == 0 {
				continue
			}

			fmt.Printf("[control] Integration 分支 %s: 有 %d 个升级任务待补打标签\n", branchName, len(tasks))
			for _, task := range tasks {
				outcome := mergeOutcomeFromEscalatedTask(task, branchName)
				c.escalateIntegrationConflict(ctx, task, outcome)
			}
		}
		for branchName, tasks := range gateEscalationRetryTasks {
			if len(tasks) == 0 {
				continue
			}

			fmt.Printf("[control] Integration 分支 %s: 有 %d 个 gate 升级任务待补打标签\n", branchName, len(tasks))
			for _, task := range tasks {
				c.syncIntegrationGateEscalationLabels(ctx, task)
			}
		}

		// 对每个 integration 分支，合入未集成的 PR
		for branchName, tasks := range branchTasks {
			if len(tasks) == 0 {
				continue
			}

			fmt.Printf("[control] Integration 分支 %s: 有 %d 个 PR 待合入\n", branchName, len(tasks))

			// 收集所有 PR 分支名，计算最旧 merge-base 作为 integration 分支起点
			var prBranches []string
			for _, task := range tasks {
				prBranches = append(prBranches, task.Branch())
			}
			startPoint, err := c.builder.ComputeOldestMergeBase(prBranches)
			if err != nil {
				fmt.Printf("[control] 计算 merge-base 失败，fallback 到 baseBranch: %v\n", err)
				startPoint = ""
			}

			for _, task := range tasks {
				bi := BranchInfo{
					Branch:   task.Branch(),
					IssueNum: task.IssueNum(),
					PRNum:    task.PRNum(),
					TaskID:   task.ID,
				}

				outcome, err := c.builder.ExecuteIntegrationMerge(branchName, bi, startPoint)
				if err != nil {
					fmt.Printf("[control] 合入 %s 失败: %v\n", bi.Branch, err)
					continue
				}

				switch outcome.Status {
				case MergeStatusMerged, MergeStatusAutoResolved:
					integrated, err := c.runIntegrationGateAndDecide(ctx, task, outcome)
					if err != nil {
						fmt.Printf("[control] integration gate 决策失败 (task %s): %v\n", task.ID, err)
						continue
					}
					if integrated {
						fmt.Printf("[control] 已合入 %s (issue #%d) 到 %s, status=%s gate=passed\n", bi.Branch, bi.IssueNum, branchName, outcome.Status)
						continue
					}
					fmt.Printf("[control] 合入 %s 后 gate 未通过，等待修复 (issue #%d)\n", bi.Branch, bi.IssueNum)

				case MergeStatusEscalated:
					if c.tryResolveIntegrationConflictWithAI(ctx, task, outcome) {
						integrated, err := c.runIntegrationGateAndDecide(ctx, task, outcome)
						if err != nil {
							fmt.Printf("[control] integration gate 决策失败 (AI 解冲突后, task %s): %v\n", task.ID, err)
							continue
						}
						if integrated {
							fmt.Printf("[control] AI 解冲突后合入 %s (issue #%d) 到 %s\n", bi.Branch, bi.IssueNum, branchName)
							continue
						}
						fmt.Printf("[control] AI 解冲突后 gate 未通过，等待修复 (issue #%d)\n", bi.IssueNum)
						continue
					}
					c.handleIntegrationConflictRetry(ctx, task, outcome)

				default:
					fmt.Printf("[control] 合入 %s 返回未知状态 %q，跳过\n", bi.Branch, outcome.Status)
					continue
				}
			}
		}

		// ⑦.5 检查 needs-human + integration-conflict 的自动恢复
		c.reconcileNeedsHumanRecovery(ctx, allTasks)

		// ⑧ 检查父 issue 进度（Sub-Issue 模式）
		c.checkParentProgress(ctx, allIssuesCache)
		return nil
	}

	// 延迟加载：当 hydrate 禁用且 builder==nil 时，此处仍需全量 issues 供 checkParentProgress 使用
	if allIssuesCache == nil {
		allIssuesCache, err = c.github.ListIssuesByState(ctx, "all")
		if err != nil {
			return fmt.Errorf("扫描全量 issues 失败: %w", err)
		}
	}
	c.checkParentProgress(ctx, allIssuesCache)

	// ⑨ 定时巡检纠偏：即使 hash 未变化也会做轻量对账，失败不阻塞主流程。
	if err := c.maybeRunDagReconcile(ctx); err != nil {
		fmt.Printf("[control][dag-sync] reconcile 失败（已降级，不阻塞主流程）: %v\n", err)
	}

	return nil
}

// buildDependencyAnalysis 构建统一依赖视图。
// 顺序固定为：解析显式 depends-on → AI 补全未声明依赖（不覆盖显式声明）。
func (c *Controller) buildDependencyAnalysis(ctx context.Context, issues []IssueInfo) *AnalysisResult {
	manualDeps, declared := buildManualDependencySeed(issues)
	analysis := &AnalysisResult{
		Dependencies: manualDeps,
	}

	if c.analyzer == nil || len(issues) <= 1 {
		return analysis
	}

	aiResult, err := c.analyzer.Analyze(ctx, issues)
	if err != nil {
		// Analyze 当前实现通常降级返回 nil error；这里保留兜底防护。
		fmt.Printf("[control][analyzer] degraded=true stage=merge err=%q\n", err)
		return analysis
	}

	for issueNum, deps := range aiResult.Dependencies {
		if _, ok := declared[issueNum]; ok {
			// 人工 depends-on 优先，AI 不允许覆盖。
			continue
		}
		analysis.Dependencies[issueNum] = deps
	}
	analysis.PotentialConflicts = aiResult.PotentialConflicts
	return analysis
}

// buildManualDependencySeed 解析显式 depends-on，并记录声明集合。
func buildManualDependencySeed(issues []IssueInfo) (map[int][]int, map[int]struct{}) {
	dependencies := make(map[int][]int)
	declared := make(map[int]struct{})
	issueSet := make(map[int]bool, len(issues))
	for _, issue := range issues {
		issueSet[issue.Number] = true
	}

	for _, issue := range issues {
		deps := parseDependsOn(issue.Body)
		if len(deps) == 0 {
			continue
		}
		declared[issue.Number] = struct{}{}
		validDeps := filterIssueDeps(issue.Number, deps, issueSet)
		if len(validDeps) > 0 {
			dependencies[issue.Number] = validDeps
		}
	}
	return dependencies, declared
}

// persistBlockedByDependencies 将依赖关系落盘到 taskctl blocked_by 字段。
// 返回 false 表示本轮未完成完整落盘，调用方应跳过 ready 放行。
func (c *Controller) persistBlockedByDependencies(issueToTask map[int]string, dependencies map[int][]int) bool {
	if len(dependencies) == 0 {
		return true
	}

	persisted := true
	issueNums := make([]int, 0, len(dependencies))
	for issueNum := range dependencies {
		issueNums = append(issueNums, issueNum)
	}
	sort.Ints(issueNums)

	for _, issueNum := range issueNums {
		deps := dependencies[issueNum]
		taskID, ok := issueToTask[issueNum]
		if !ok {
			fmt.Printf("[control] issue #%d 对应 task 不存在，blocked_by 落盘未完成\n", issueNum)
			persisted = false
			continue
		}

		blockedBy := make([]string, 0, len(deps))
		missingDeps := make([]int, 0)
		for _, dep := range deps {
			depTaskID, ok := issueToTask[dep]
			if !ok {
				missingDeps = append(missingDeps, dep)
				continue
			}
			blockedBy = append(blockedBy, depTaskID)
		}

		if len(missingDeps) > 0 {
			fmt.Printf("[control] issue #%d 依赖任务缺失: %v，blocked_by 落盘未完成\n", issueNum, missingDeps)
			persisted = false
		}
		if len(blockedBy) == 0 {
			if len(deps) > 0 {
				fmt.Printf("[control] issue #%d 依赖为空映射，blocked_by 落盘未完成\n", issueNum)
				persisted = false
			}
			continue
		}

		if err := c.taskctl.Update(taskID, UpdateOpts{BlockedBy: &blockedBy}); err != nil {
			fmt.Printf("[control] 设置 blocked_by 失败 (task %s): %v\n", taskID, err)
			persisted = false
			continue
		}
	}

	return persisted
}

// hydrateTasksFromGitHub 从 GitHub 重建跨 workflow run 丢失的 task 状态。
// 扫描所有 open + 带 bot:* 标签的 issues，对 tasks.json 中不存在的 issue 重建 task，
// 对已存在但 PR 类标签缺 pr_num/branch 的 task 补全元数据。
// 返回 hydrated issues 列表（用于后续依赖分析，不参与 label 迁移）。
func (c *Controller) hydrateTasksFromGitHub(
	ctx context.Context,
	allIssuesCache []IssueInfo,
	activeIssueToTask map[int]string,
	issueToTask map[int]string,
) []intakeIssueEntry {
	var hydratedIssues []intakeIssueEntry

	for _, issue := range allIssuesCache {
		if !strings.EqualFold(issue.State, "open") {
			continue
		}
		if !hasBotLabel(issue.Labels) {
			continue
		}
		// 跳过 bot:orchestrate（由 intake 处理）
		if hasLabel(issue.Labels, string(state.StateOrchestrate)) {
			continue
		}
		// 跳过已有 active task 的 issue
		if _, ok := activeIssueToTask[issue.Number]; ok {
			// 但检查是否需要补全 PR metadata
			c.hydrateExistingTaskMetadata(ctx, issue, activeIssueToTask[issue.Number])
			continue
		}

		// 重建 task
		meta := map[string]string{
			"issue_num": strconv.Itoa(issue.Number),
		}
		task, reused, err := c.taskctl.CreateOrReuseActiveIssueTask(issue.Title, issue.Body, meta)
		if err != nil {
			fmt.Printf("[control][hydrate] 创建任务失败 (issue #%d): %v\n", issue.Number, err)
			continue
		}
		issueToTask[issue.Number] = task.ID
		activeIssueToTask[issue.Number] = task.ID

		if reused {
			fmt.Printf("[control][hydrate] action=reused issue_num=%d task_id=%s\n", issue.Number, task.ID)
		} else {
			fmt.Printf("[control][hydrate] action=created issue_num=%d task_id=%s\n", issue.Number, task.ID)
		}

		// PR 类标签：补全 pr_num/branch
		if isPRStateLabel(issue.Labels) {
			resolved, err := c.github.ResolvePRMetadata(ctx, issue.Number)
			if err != nil {
				if skipReason, skippable := classifyPRMetadataSkipReason(err); skippable {
					fmt.Printf("[control][hydrate] action=pr_metadata_skipped issue_num=%d reason=%s\n", issue.Number, skipReason)
				} else {
					fmt.Printf("[control][hydrate] action=pr_metadata_failed issue_num=%d err=%v\n", issue.Number, err)
				}
			} else {
				metaUpdate := map[string]string{
					"pr_num": strconv.Itoa(resolved.PRNum),
					"branch": resolved.Branch,
				}
				if err := c.taskctl.Update(task.ID, UpdateOpts{Metadata: &metaUpdate}); err != nil {
					fmt.Printf("[control][hydrate] action=metadata_update_failed task_id=%s err=%v\n", task.ID, err)
				} else {
					fmt.Printf("[control][hydrate] action=metadata_synced issue_num=%d pr_num=%d branch=%s\n", issue.Number, resolved.PRNum, resolved.Branch)
				}
			}
		}

		hydratedIssues = append(hydratedIssues, intakeIssueEntry{IssueInfo: issue, FromIntake: false})
	}

	if len(hydratedIssues) > 0 {
		fmt.Printf("[control][hydrate] 重建 %d 个 tasks\n", len(hydratedIssues))
	}
	return hydratedIssues
}

// hydrateExistingTaskMetadata 对已有 active task 但缺 pr_num 的 PR 类 issue 补全元数据。
func (c *Controller) hydrateExistingTaskMetadata(ctx context.Context, issue IssueInfo, taskID string) {
	if !isPRStateLabel(issue.Labels) {
		return
	}

	// 检查 task 是否已有 pr_num
	tasks, err := c.taskctl.List("")
	if err != nil {
		return
	}
	for _, t := range tasks {
		if t.ID == taskID && (t.PRNum() == 0 || t.Branch() == "") {
			resolved, err := c.github.ResolvePRMetadata(ctx, issue.Number)
			if err != nil {
				if skipReason, skippable := classifyPRMetadataSkipReason(err); skippable {
					fmt.Printf("[control][hydrate] action=existing_pr_metadata_skipped issue_num=%d reason=%s\n", issue.Number, skipReason)
				} else {
					fmt.Printf("[control][hydrate] action=existing_pr_metadata_failed issue_num=%d err=%v\n", issue.Number, err)
				}
				return
			}
			metaUpdate := map[string]string{
				"pr_num": strconv.Itoa(resolved.PRNum),
				"branch": resolved.Branch,
			}
			if err := c.taskctl.Update(taskID, UpdateOpts{Metadata: &metaUpdate}); err != nil {
				fmt.Printf("[control][hydrate] action=existing_metadata_update_failed task_id=%s err=%v\n", taskID, err)
			} else {
				fmt.Printf("[control][hydrate] action=existing_metadata_synced issue_num=%d pr_num=%d branch=%s\n", issue.Number, resolved.PRNum, resolved.Branch)
			}
			return
		}
	}
}

// collectAutomationIssues 仅扫描 orchestrate 入口 issue。
// 返回值中的 orchestrateCount 表示本轮入口数量。
func (c *Controller) collectAutomationIssues(ctx context.Context) ([]IssueInfo, int, error) {
	issues, err := c.github.ListIssuesWithLabel(ctx, string(state.StateOrchestrate))
	if err != nil {
		return nil, 0, fmt.Errorf("扫描 label=%s 失败: %w", state.StateOrchestrate, err)
	}

	result := make([]IssueInfo, 0, len(issues))
	for _, issue := range issues {
		if issue.Number <= 0 {
			continue
		}
		result = append(result, issue)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Number < result[j].Number
	})

	return result, len(result), nil
}

// syncPRReviewableMetadata 在 integration 候选筛选前回填 PR 元数据。
// 仅处理 GitHub 状态为 bot:pr-reviewable 的任务，支持幂等 no-op 与可跳过错误。
func (c *Controller) syncPRReviewableMetadata(ctx context.Context, tasks []Task, issueByNumber map[int]IssueInfo) error {
	for i := range tasks {
		task := &tasks[i]
		issueNum := task.IssueNum()
		if issueNum <= 0 {
			continue
		}

		issue, ok := issueByNumber[issueNum]
		if !ok || !hasLabel(issue.Labels, "bot:pr-reviewable") {
			continue
		}

		resolved, err := c.github.ResolvePRMetadata(ctx, issueNum)
		if err != nil {
			if skipReason, skippable := classifyPRMetadataSkipReason(err); skippable {
				c.logMetadataSyncSkip(*task, skipReason)
				continue
			}
			return fmt.Errorf("issue #%d 解析 PR 元数据失败: %w", issueNum, err)
		}

		metaUpdate := buildPRMetadataUpdate(task, resolved)
		if len(metaUpdate) == 0 {
			c.logMetadataSyncSkip(*task, metadataSyncSkipAlreadyUpToDate)
			continue
		}

		if err := c.taskctl.Update(task.ID, UpdateOpts{Metadata: &metaUpdate}); err != nil {
			return fmt.Errorf("task %s 持久化 PR 元数据失败: %w", task.ID, err)
		}

		if task.Metadata == nil {
			task.Metadata = make(map[string]string)
		}
		for key, value := range metaUpdate {
			task.Metadata[key] = value
		}

		fmt.Printf("[control] action=metadata_synced task_key=%s issue_num=%d pr_num=%d branch=%s\n", task.ID, issueNum, resolved.PRNum, resolved.Branch)
	}
	return nil
}

func (c *Controller) logMetadataSyncSkip(task Task, reason string) {
	fmt.Printf("[control] action=metadata_sync_skipped task_key=%s issue_num=%d skip_reason=%s\n", task.ID, task.IssueNum(), reason)
}

func buildPRMetadataUpdate(task *Task, resolved PRMetadata) map[string]string {
	if task.Metadata == nil {
		task.Metadata = make(map[string]string)
	}

	metaUpdate := make(map[string]string)
	resolvedPRNum := strconv.Itoa(resolved.PRNum)
	if task.Metadata["pr_num"] != resolvedPRNum {
		metaUpdate["pr_num"] = resolvedPRNum
	}
	if task.Metadata["branch"] != resolved.Branch {
		metaUpdate["branch"] = resolved.Branch
	}

	if strings.TrimSpace(task.Metadata["meta_issue_slug"]) == "" {
		metaUpdate["meta_issue_slug"] = inferMetaIssueSlug(resolved.Branch)
	}

	return metaUpdate
}

func inferMetaIssueSlug(branch string) string {
	if strings.HasPrefix(branch, "integration/") {
		if slug := strings.TrimPrefix(branch, "integration/"); strings.TrimSpace(slug) != "" {
			return slug
		}
	}
	return "main"
}

func classifyPRMetadataSkipReason(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrPRMarkerNotFound):
		return metadataSyncSkipMarkerNotFound, true
	case errors.Is(err, ErrPRClosed):
		return metadataSyncSkipPRClosed, true
	case errors.Is(err, ErrPRBranchUnavailable):
		return metadataSyncSkipBranchUnavailable, true
	default:
		return "", false
	}
}

func (c *Controller) shouldEnqueueIntegrationMergeTask(ctx context.Context, task Task, issueByNumber map[int]IssueInfo) bool {
	issueNum := task.IssueNum()
	if issueNum <= 0 {
		return false
	}

	issue, err := c.github.GetIssue(ctx, issueNum)
	if err != nil {
		fmt.Printf("[control] 读取 issue #%d 最新标签失败，跳过本轮 integration 合入 (task %s): %v\n", issueNum, task.ID, err)
		return false
	}
	issueByNumber[issueNum] = issue
	return hasLabel(issue.Labels, "bot:pr-reviewable")
}

// reconcilePRReviewableConflicts 协调 bot:pr-reviewable 的冲突回退逻辑。
func (c *Controller) reconcilePRReviewableConflicts(ctx context.Context, tasks []Task, issueByNumber map[int]IssueInfo) error {
	for i := range tasks {
		task := &tasks[i]
		issueNum := task.IssueNum()
		if issueNum <= 0 {
			continue
		}

		issue, ok := issueByNumber[issueNum]
		if !ok || !hasLabel(issue.Labels, "bot:pr-reviewable") {
			continue
		}

		reviewStatus, unknownExhausted, err := c.resolvePRReviewStatusWithBackoff(ctx, issueNum)
		if err != nil {
			if skipReason, skippable := classifyPRMetadataSkipReason(err); skippable {
				fmt.Printf("[control] action=pr_reviewable_conflict_skipped issue=%d skip_reason=%s\n", issueNum, skipReason)
				continue
			}
			return fmt.Errorf("issue #%d 解析 PR review 状态失败: %w", issueNum, err)
		}

		if reviewStatus.IsConflicting() {
			handled, err := c.resolvePRConflictWithLayers(ctx, *task, reviewStatus)
			if err != nil {
				return err
			}
			if !handled {
				if err := c.handlePRReviewableConflict(ctx, issueNum, reviewStatus); err != nil {
					return err
				}
			}
			continue
		}

		if reviewStatus.IsUnknown() || unknownExhausted {
			c.logPRReviewableDecision(issueNum, reviewStatus, "bot:pr-reviewable", "bot:pr-reviewable", parsePRConflictRetryCount(issue.Body), c.prConflictRetryThreshold(), "noop_unknown")
			continue
		}

		if !reviewStatus.IsMergeable() {
			c.logPRReviewableDecision(issueNum, reviewStatus, "bot:pr-reviewable", "bot:pr-reviewable", parsePRConflictRetryCount(issue.Body), c.prConflictRetryThreshold(), "noop_non_conflicting")
			continue
		}

		if err := c.handlePRReviewableMergeable(ctx, issueNum, reviewStatus); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) handlePRReviewableMergeable(ctx context.Context, issueNum int, reviewStatus PRReviewStatus) error {
	issue, err := c.github.GetIssue(ctx, issueNum)
	if err != nil {
		return fmt.Errorf("issue #%d 读取最新状态失败: %w", issueNum, err)
	}
	if !hasLabel(issue.Labels, "bot:pr-reviewable") {
		c.logPRReviewableDecision(issueNum, reviewStatus, "bot:pr-reviewable", currentAutomationLabel(issue.Labels), parsePRConflictRetryCount(issue.Body), c.prConflictRetryThreshold(), "skip_race")
		return nil
	}

	retryCount := parsePRConflictRetryCount(issue.Body)
	if retryCount <= 0 {
		c.logPRReviewableDecision(issueNum, reviewStatus, "bot:pr-reviewable", "bot:pr-reviewable", 0, c.prConflictRetryThreshold(), "noop_mergeable")
		return nil
	}

	if err := c.persistPRConflictRetryCount(ctx, issue, 0); err != nil {
		return err
	}

	c.logPRReviewableDecision(issueNum, reviewStatus, "bot:pr-reviewable", "bot:pr-reviewable", 0, c.prConflictRetryThreshold(), "reset_retry")
	return nil
}

func (c *Controller) handlePRReviewableConflict(ctx context.Context, issueNum int, reviewStatus PRReviewStatus) error {
	issue, err := c.github.GetIssue(ctx, issueNum)
	if err != nil {
		return fmt.Errorf("issue #%d 读取最新状态失败: %w", issueNum, err)
	}
	if !hasLabel(issue.Labels, "bot:pr-reviewable") {
		c.logPRReviewableDecision(issueNum, reviewStatus, "bot:pr-reviewable", currentAutomationLabel(issue.Labels), parsePRConflictRetryCount(issue.Body), c.prConflictRetryThreshold(), "skip_race")
		return nil
	}

	threshold := c.prConflictRetryThreshold()
	retryCount := parsePRConflictRetryCount(issue.Body) + 1

	headSHA := normalizedHeadSHA(reviewStatus.HeadSHA)
	oldLabel := "bot:pr-reviewable"
	newLabel := "bot:pr-needs-fix"
	decision := "rollback_to_needs_fix"
	if retryCount > threshold {
		newLabel = needsHumanLabel
		decision = "escalate_needs_human"
	}

	if err := c.ensurePRConflictDetectedComment(ctx, issueNum, reviewStatus, retryCount, threshold, headSHA, newLabel); err != nil {
		return err
	}

	if retryCount > threshold {
		if err := c.ensurePRConflictEscalationComment(ctx, issueNum, reviewStatus, retryCount, threshold, headSHA); err != nil {
			return err
		}
	}

	if err := c.syncIssueStateLabel(ctx, issueNum, newLabel); err != nil {
		return fmt.Errorf("issue #%d 状态回退失败: %w", issueNum, err)
	}
	if err := c.persistPRConflictRetryCount(ctx, issue, retryCount); err != nil {
		return err
	}
	c.logPRReviewableDecision(issueNum, reviewStatus, oldLabel, newLabel, retryCount, threshold, decision)
	return nil
}

func (c *Controller) resolvePRReviewStatusWithBackoff(ctx context.Context, issueNum int) (PRReviewStatus, bool, error) {
	backoffs := c.prConflictUnknownBackoffs()
	for attempt := 0; ; attempt++ {
		reviewStatus, err := c.github.ResolvePRReviewStatus(ctx, issueNum)
		if err != nil {
			return PRReviewStatus{}, false, err
		}
		reviewStatus.MergeStateStatus = strings.ToUpper(strings.TrimSpace(reviewStatus.MergeStateStatus))
		if !reviewStatus.IsUnknown() {
			return reviewStatus, false, nil
		}
		if attempt >= len(backoffs) {
			return reviewStatus, true, nil
		}

		wait := backoffs[attempt]
		fmt.Printf(
			"[control] action=pr_reviewable_unknown_retry issue=%d pr=%d mergeable=%s merge_state_status=%s attempt=%d wait=%s\n",
			issueNum,
			reviewStatus.PRNum,
			reviewStatus.Mergeable,
			reviewStatus.normalizedMergeStateStatus(),
			attempt+1,
			wait,
		)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return PRReviewStatus{}, false, ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *Controller) ensurePRConflictDetectedComment(
	ctx context.Context,
	issueNum int,
	reviewStatus PRReviewStatus,
	retryCount int,
	threshold int,
	headSHA string,
	nextLabel string,
) error {
	marker := fmt.Sprintf(prConflictDetectedCommentMarkerFmt, headSHA)
	followup := "已触发既有 iterate 链路继续自动修复。"
	if nextLabel == needsHumanLabel {
		followup = "已停止自动回退循环，等待人工介入处理。"
	}
	body := fmt.Sprintf(
		"## ⚠️ 检测到 PR 冲突\n\n- PR: #%d\n- mergeable: `%s`\n- mergeStateStatus: `%s`\n- headSha: `%s`\n- conflict_retry: `%d/%d`\n- 状态变更: `bot:pr-reviewable -> %s`\n\n%s\n\n%s",
		reviewStatus.PRNum,
		reviewStatus.Mergeable,
		reviewStatus.normalizedMergeStateStatus(),
		headSHA,
		retryCount,
		threshold,
		nextLabel,
		followup,
		marker,
	)
	return c.ensureIssueCommentWithMarker(ctx, issueNum, marker, body)
}

func (c *Controller) ensurePRConflictEscalationComment(
	ctx context.Context,
	issueNum int,
	reviewStatus PRReviewStatus,
	retryCount int,
	threshold int,
	headSHA string,
) error {
	marker := fmt.Sprintf(prConflictEscalatedCommentMarkerFmt, headSHA)
	body := fmt.Sprintf(
		"## ⚠️ 自动冲突修复已超限，转人工处理\n\n- PR: #%d\n- mergeable: `%s`\n- mergeStateStatus: `%s`\n- headSha: `%s`\n- conflict_retry: `%d`（threshold=%d）\n- 状态变更: `bot:pr-reviewable -> needs-human`\n\n请人工介入处理冲突后再继续流程。\n\n%s",
		reviewStatus.PRNum,
		reviewStatus.Mergeable,
		reviewStatus.normalizedMergeStateStatus(),
		headSHA,
		retryCount,
		threshold,
		marker,
	)
	return c.ensureIssueCommentWithMarker(ctx, issueNum, marker, body)
}

func (c *Controller) ensureIssueCommentWithMarker(ctx context.Context, issueNum int, markerLine string, body string) error {
	commentBodies, err := c.github.ListCommentBodies(ctx, issueNum)
	if err != nil {
		return fmt.Errorf("issue #%d 读取评论失败: %w", issueNum, err)
	}
	for _, commentBody := range commentBodies {
		if strings.Contains(commentBody, markerLine) {
			return nil
		}
	}
	if err := c.github.AddIssueComment(ctx, issueNum, body); err != nil {
		return fmt.Errorf("issue #%d 写入评论失败: %w", issueNum, err)
	}
	return nil
}

func (c *Controller) persistPRConflictRetryCount(ctx context.Context, issue IssueInfo, retryCount int) error {
	updatedBody, changed := upsertPRConflictRetryMarker(issue.Body, retryCount)
	if !changed {
		return nil
	}
	if err := c.github.UpdateIssueBody(ctx, issue.Number, updatedBody); err != nil {
		return fmt.Errorf("issue #%d 更新 PR_CONFLICT_RETRY 失败: %w", issue.Number, err)
	}
	return nil
}

func parsePRConflictRetryCount(body string) int {
	matches := prConflictRetryMarkerRe.FindStringSubmatch(body)
	if len(matches) < 2 {
		return 0
	}
	count, err := strconv.Atoi(matches[1])
	if err != nil || count < 0 {
		return 0
	}
	return count
}

func upsertPRConflictRetryMarker(body string, retryCount int) (string, bool) {
	markerLine := fmt.Sprintf("<!-- PR_CONFLICT_RETRY:%d -->", retryCount)
	if loc := prConflictRetryMarkerRe.FindStringIndex(body); loc != nil {
		current := body[loc[0]:loc[1]]
		if current == markerLine {
			return body, false
		}
		return body[:loc[0]] + markerLine + body[loc[1]:], true
	}

	trimmed := strings.TrimRight(body, "\n")
	if strings.TrimSpace(trimmed) == "" {
		return markerLine, true
	}
	return trimmed + "\n\n" + markerLine, true
}

func normalizedHeadSHA(headSHA string) string {
	headSHA = strings.TrimSpace(headSHA)
	if headSHA == "" {
		return "unknown"
	}
	return headSHA
}

func currentAutomationLabel(labels []string) string {
	for _, candidate := range integrationAutomationLabels {
		if hasLabel(labels, candidate) {
			return candidate
		}
	}
	return "-"
}

func (c *Controller) prConflictRetryThreshold() int {
	if c.cfg == nil {
		return prConflictRetryDefaultThreshold
	}
	if c.cfg.PRConflictRetryThreshold < 0 {
		return prConflictRetryDefaultThreshold
	}
	return c.cfg.PRConflictRetryThreshold
}

func (c *Controller) prConflictUnknownBackoffs() []time.Duration {
	if c.cfg == nil || len(c.cfg.PRConflictUnknownBackoffs) == 0 {
		return append([]time.Duration(nil), defaultPRConflictUnknownBackoffs...)
	}
	return append([]time.Duration(nil), c.cfg.PRConflictUnknownBackoffs...)
}

func (c *Controller) prConflictAIEnabled() bool {
	if c.cfg == nil {
		return true
	}
	return c.cfg.PRConflictEnableAI
}

func (c *Controller) prConflictAIMaxAttempts() int {
	if c.cfg == nil || c.cfg.PRConflictAIMaxAttempts <= 0 {
		return prConflictAIDefaultMaxAttempts
	}
	return c.cfg.PRConflictAIMaxAttempts
}

func (c *Controller) prConflictSmokeTestCmd() string {
	if c.cfg == nil {
		return ""
	}
	return strings.TrimSpace(c.cfg.PRConflictSmokeTestCmd)
}

func (c *Controller) prConflictElixirTestCmd() string {
	if c.cfg == nil {
		return "mix test"
	}
	return c.cfg.PRConflictElixirTestCmd
}

// prConflictProfileMode 返回 profile 模式："auto"/"none"/"whitelist"。
func (c *Controller) prConflictProfileMode() string {
	if c.cfg == nil {
		return "auto"
	}
	mode, _ := ParseProfileFlag(c.cfg.PRConflictProfile)
	return mode
}

// prConflictProfileWhitelist 返回白名单语言列表（仅 mode=whitelist 时非空）。
func (c *Controller) prConflictProfileWhitelist() []string {
	if c.cfg == nil {
		return nil
	}
	_, langs := ParseProfileFlag(c.cfg.PRConflictProfile)
	return langs
}

func (c *Controller) logPRReviewableDecision(
	issueNum int,
	reviewStatus PRReviewStatus,
	oldLabel string,
	newLabel string,
	retryCount int,
	threshold int,
	decision string,
) {
	fmt.Printf(
		"[control] action=pr_reviewable_reconcile issue=%d pr=%d head_sha=%s old_label=%s new_label=%s retry_count=%d threshold=%d mergeable=%s merge_state_status=%s decision=%s\n",
		issueNum,
		reviewStatus.PRNum,
		normalizedHeadSHA(reviewStatus.HeadSHA),
		oldLabel,
		newLabel,
		retryCount,
		threshold,
		reviewStatus.Mergeable,
		reviewStatus.normalizedMergeStateStatus(),
		decision,
	)
}

// checkParentProgress 检查父 issue 的所有 sub-issues 是否完成
func (c *Controller) checkParentProgress(ctx context.Context, issues []IssueInfo) {
	// 构建 parent → sub-issues 映射
	parentToSubs := make(map[int][]int) // parentNum → []subNum
	for _, issue := range issues {
		if parentNum := parseParent(issue.Body); parentNum > 0 {
			parentToSubs[parentNum] = append(parentToSubs[parentNum], issue.Number)
		}
	}

	if len(parentToSubs) == 0 {
		return
	}

	// 获取所有 task 状态
	tasks, err := c.taskctl.List("")
	if err != nil {
		return
	}

	taskStatus := make(map[int]TaskStatus)
	for _, t := range tasks {
		if n := t.IssueNum(); n > 0 {
			taskStatus[n] = t.Status
		}
	}

	// 检查每个 parent 的 sub-issues 是否都完成
	for parentNum, subNums := range parentToSubs {
		allCompleted := true
		for _, subNum := range subNums {
			if status, ok := taskStatus[subNum]; !ok || status != TaskStatusCompleted {
				allCompleted = false
				break
			}
		}

		if allCompleted {
			fmt.Printf("[control] 父 issue #%d 的所有 sub-issues 已完成: %v\n", parentNum, subNums)
			// TODO: 可以在这里关闭父 issue 或添加评论
		}
	}
}

// FinalizeIntegratedIssues 在 integration PR 合并后关闭对应 sub-issue，并尝试收口 parent。
func (c *Controller) FinalizeIntegratedIssues(ctx context.Context, issueNums []int) error {
	targets := uniquePositiveIssueNumbers(issueNums)
	if len(targets) == 0 {
		fmt.Println("[control] integration merge 未提取到可收口 issue，跳过")
		return nil
	}

	parentNums := make(map[int]struct{})
	var firstErr error

	for _, issueNum := range targets {
		issue, err := c.github.GetIssue(ctx, issueNum)
		if err != nil {
			fmt.Printf("[control] 获取 issue #%d 失败: %v\n", issueNum, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		if parentNum := parseParent(issue.Body); parentNum > 0 {
			parentNums[parentNum] = struct{}{}
		}

		if strings.EqualFold(issue.State, "closed") {
			fmt.Printf("[control] issue #%d 已关闭，跳过重复收口\n", issueNum)
			continue
		}

		if err := c.syncIssueStateLabel(ctx, issueNum, botDoneLabel); err != nil {
			fmt.Printf("[control] issue #%d 打标 %s 失败: %v\n", issueNum, botDoneLabel, err)
			if firstErr == nil {
				firstErr = err
			}
		}
		if err := c.github.CloseIssue(ctx, issueNum); err != nil {
			fmt.Printf("[control] 关闭 issue #%d 失败: %v\n", issueNum, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Printf("[control] 已关闭 sub issue #%d\n", issueNum)
	}

	if err := c.closeParentIssuesIfReady(ctx, parentNums); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// closeParentIssuesIfReady 当 parent 的 sub-issue 全部关闭时，自动关闭 parent。
func (c *Controller) closeParentIssuesIfReady(ctx context.Context, parentNums map[int]struct{}) error {
	if len(parentNums) == 0 {
		return nil
	}

	issues, err := c.github.ListIssuesByState(ctx, "all")
	if err != nil {
		return fmt.Errorf("列出全量 issues 失败: %w", err)
	}

	parentToSubs := make(map[int][]IssueInfo)
	for _, issue := range issues {
		if parentNum := parseParent(issue.Body); parentNum > 0 {
			parentToSubs[parentNum] = append(parentToSubs[parentNum], issue)
		}
	}

	var firstErr error
	for parentNum := range parentNums {
		subs := parentToSubs[parentNum]
		if len(subs) == 0 {
			fmt.Printf("[control] parent #%d 未找到 sub-issue，跳过自动关闭\n", parentNum)
			continue
		}

		var openSubs []int
		for _, sub := range subs {
			if !strings.EqualFold(sub.State, "closed") {
				openSubs = append(openSubs, sub.Number)
			}
		}
		if len(openSubs) > 0 {
			sort.Ints(openSubs)
			fmt.Printf("[control] parent #%d 未满足关闭条件，仍有未关闭 sub-issue: %v\n", parentNum, openSubs)
			continue
		}

		parentIssue, err := c.github.GetIssue(ctx, parentNum)
		if err != nil {
			fmt.Printf("[control] 获取 parent issue #%d 失败: %v\n", parentNum, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if strings.EqualFold(parentIssue.State, "closed") {
			fmt.Printf("[control] parent issue #%d 已关闭，跳过重复收口\n", parentNum)
			continue
		}

		if err := c.syncIssueStateLabel(ctx, parentNum, botDoneLabel); err != nil {
			fmt.Printf("[control] parent issue #%d 打标 %s 失败: %v\n", parentNum, botDoneLabel, err)
			if firstErr == nil {
				firstErr = err
			}
		}
		if err := c.github.CloseIssue(ctx, parentNum); err != nil {
			fmt.Printf("[control] 关闭 parent issue #%d 失败: %v\n", parentNum, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Printf("[control] 已关闭 parent issue #%d（sub-issue 全部完成）\n", parentNum)
	}

	return firstErr
}

// Status 返回全局控制状态
func (c *Controller) Status(ctx context.Context) (*ControlStatus, error) {
	tasks, err := c.taskctl.List("")
	if err != nil {
		return nil, fmt.Errorf("列出任务失败: %w", err)
	}

	dag, err := c.taskctl.Dag()
	if err != nil {
		// DAG 不可用时不阻塞
		dag = &DagGraph{}
	}

	return &ControlStatus{
		Dag:   dag,
		Tasks: tasks,
	}, nil
}

// Merge 将 integration 分支合并到 master
// 人工批准后执行：integration/{slug} → master
func (c *Controller) Merge(ctx context.Context, integrationBranch string) error {
	if c.builder == nil {
		return fmt.Errorf("IntegrationBuilder 未配置")
	}

	// 使用 GitHub API 创建 PR 或直接 merge
	// 这里简化处理：直接 fast-forward merge
	fmt.Printf("[control] 合并 %s 到 master...\n", integrationBranch)

	// 实际实现需要调用 GitHub API 或 git 命令
	// 这里只是一个占位，具体实现取决于 GitHubOps 接口的扩展

	return fmt.Errorf("Merge 实现待完成：需要扩展 GitHubOps 接口支持分支合并")
}

// FormatStatus 格式化输出控制状态
func FormatStatus(status *ControlStatus) string {
	var sb strings.Builder
	sb.WriteString("## 控制状态\n\n")

	if len(status.Dag.Nodes) > 0 {
		sb.WriteString(fmt.Sprintf("DAG: %d 个节点\n", len(status.Dag.Nodes)))
	}

	sb.WriteString(fmt.Sprintf("任务总数: %d\n\n", len(status.Tasks)))

	if len(status.Tasks) > 0 {
		sb.WriteString("| Task ID | Issue | 状态 | 阻塞于 |\n")
		sb.WriteString("|---------|-------|------|--------|\n")
		for _, t := range status.Tasks {
			blockedBy := strings.Join(t.BlockedBy, ", ")
			if blockedBy == "" {
				blockedBy = "-"
			}
			sb.WriteString(fmt.Sprintf("| %s | #%d | %s | %s |\n",
				t.ID, t.IssueNum(), t.Status, blockedBy))
		}
	}

	return sb.String()
}

// hasLabel 检查 label 列表中是否包含指定 label
func hasLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

// isBotLabel 判断标签是否为 bot:* 前缀
func isBotLabel(label string) bool {
	return strings.HasPrefix(label, "bot:")
}

// hasBotLabel 判断 labels 列表中是否包含任意 bot:* 标签
func hasBotLabel(labels []string) bool {
	for _, l := range labels {
		if isBotLabel(l) {
			return true
		}
	}
	return false
}

// isPRStateLabel 判断 labels 是否包含 PR 类状态标签（需要查询 PR API 补全元数据）
func isPRStateLabel(labels []string) bool {
	for _, l := range labels {
		switch l {
		case string(state.StatePRReviewable), "bot:pr-approved":
			return true
		}
	}
	return false
}

func (c *Controller) ensureIssueLabel(ctx context.Context, issueNum int, targetLabel string) error {
	labels, err := c.github.ListLabels(ctx, issueNum)
	if err != nil {
		return err
	}
	if hasLabel(labels, targetLabel) {
		return nil
	}
	return c.github.AddLabel(ctx, issueNum, targetLabel)
}

func (c *Controller) clearIssueBotStates(ctx context.Context, issueNum int) error {
	_, err := state.Clear(ctx, c.github, issueNum)
	return err
}

func (c *Controller) transitionWithSelfHeal(ctx context.Context, issueNum int, from, to state.State) error {
	err := state.TransitionWithRetry(ctx, c.github, issueNum, from, to, nil)
	if err == nil {
		return nil
	}
	if !errors.Is(err, state.ErrInvariantViolation) && !errors.Is(err, state.ErrMultipleBotStates) {
		return err
	}

	target, changed, healErr := c.normalizeIssueState(ctx, issueNum)
	if healErr != nil {
		return healErr
	}
	if changed {
		fmt.Printf("[control] issue #%d 状态自愈完成，收敛到 %s\n", issueNum, target)
	}
	return state.TransitionWithRetry(ctx, c.github, issueNum, from, to, nil)
}

func (c *Controller) normalizeIssueState(ctx context.Context, issueNum int) (state.State, bool, error) {
	labels, err := c.github.ListLabels(ctx, issueNum)
	if err != nil {
		return "", false, err
	}
	states, invalid := state.CollectBotStatesWithInvalid(labels)
	if len(invalid) > 0 {
		return "", false, fmt.Errorf("issue #%d 存在非法 bot 标签: %v", issueNum, invalid)
	}
	if len(states) <= 1 {
		if len(states) == 1 {
			return states[0], false, nil
		}
		return "", false, nil
	}

	priority, err := state.ParseStatePriority(os.Getenv("NIUMA_STATE_PRIORITY"))
	if err != nil {
		priority = append([]state.State(nil), state.DefaultStatePriority...)
	}
	target, changed, err := state.Normalize(ctx, c.github, issueNum, priority)
	if err != nil {
		return "", false, err
	}
	if changed {
		c.emitStateHealComment(ctx, issueNum, states, target)
	}
	return target, changed, nil
}

func (c *Controller) emitStateHealComment(ctx context.Context, issueNum int, states []state.State, target state.State) {
	marker := fmt.Sprintf("<!-- BOT:STATE_CONVERGED issue=%d target=%s -->", issueNum, target)
	bodies, err := c.github.ListCommentBodies(ctx, issueNum)
	if err == nil {
		for _, body := range bodies {
			if strings.Contains(body, marker) {
				return
			}
		}
	}

	_ = c.github.AddIssueComment(ctx, issueNum, fmt.Sprintf(
		"## ⚠️ 状态自愈\n\n检测到多个 `bot:*` 状态标签（%v），已自动收敛为 `%s` 并继续推进。\n\n%s",
		states,
		target,
		marker,
	))
}

var integrationAutomationLabels = []string{
	"bot:orchestrate",
	"bot:queued",
	"bot:fix",
	"bot:plan-draft",
	"bot:needs-discussion",
	"bot:plan-final",
	"bot:plan-approved",
	"bot:implementing",
	"bot:pr-created",
	"bot:pr-reviewable",
	"bot:pr-needs-fix",
	"bot:iterating",
	integrationConflictLabel,
	integrationGateFailLabel,
	needsHumanLabel,
}

func (c *Controller) runIntegrationGateAndDecide(ctx context.Context, task Task, outcome MergeOutcome) (bool, error) {
	attemptKey, err := c.buildIntegrationGateAttemptKey(outcome)
	if err != nil {
		return false, err
	}
	if shouldSkipProcessedIntegrationGateAttempt(task.Metadata, attemptKey) {
		return false, nil
	}

	if err := c.markIntegrationGatePending(task, attemptKey); err != nil {
		return false, err
	}

	if err := c.runIntegrationGate(ctx, outcome); err != nil {
		if err := c.handleIntegrationGateFailure(ctx, task, attemptKey, err); err != nil {
			return false, err
		}
		return false, nil
	}

	if err := c.markIntegrationGatePassed(task, attemptKey); err != nil {
		return false, err
	}
	if err := c.markTaskIntegrated(task, outcome); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Controller) buildIntegrationGateAttemptKey(outcome MergeOutcome) (string, error) {
	if outcome.IntegrationBranch == "" {
		return "", fmt.Errorf("integration branch 为空，无法生成 attempt_key")
	}
	if outcome.SourceBranch == "" {
		return "", fmt.Errorf("source branch 为空，无法生成 attempt_key")
	}

	// attempt_key 仅跟随当前任务源分支的新提交变化，避免 integration 头部被他人推进时误增计数。
	headSHA, err := c.currentBranchHeadSHA(outcome.SourceBranch)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s:%s", outcome.IntegrationBranch, outcome.SourceBranch, headSHA), nil
}

func (c *Controller) currentBranchHeadSHA(branch string) (string, error) {
	out, err := c.runCommand(context.Background(), c.cfg.RepoDir, "git", "rev-parse", "--verify", branch)
	if err != nil {
		return "", fmt.Errorf("获取 %s HEAD 失败: %w", branch, err)
	}
	sha := strings.TrimSpace(out)
	if sha == "" {
		return "", fmt.Errorf("分支 %s HEAD 为空", branch)
	}
	return sha, nil
}

func (c *Controller) runIntegrationGate(ctx context.Context, outcome MergeOutcome) error {
	if outcome.IntegrationBranch == "" {
		return fmt.Errorf("integration branch 为空")
	}

	originalRefOut, err := c.runCommand(ctx, c.cfg.RepoDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("读取当前分支失败: %w", err)
	}
	originalRef := strings.TrimSpace(originalRefOut)
	if originalRef == "" {
		originalRef = "master"
	}

	if _, err := c.runCommand(ctx, c.cfg.RepoDir, "git", "checkout", outcome.IntegrationBranch); err != nil {
		return fmt.Errorf("切换到 integration 分支失败: %w", err)
	}
	defer func() {
		if _, restoreErr := c.runCommand(context.Background(), c.cfg.RepoDir, "git", "checkout", originalRef); restoreErr != nil {
			fmt.Printf("[control] 恢复分支 %s 失败: %v\n", originalRef, restoreErr)
		}
	}()

	runGo, runRust, err := c.resolveIntegrationGateScopes(ctx, outcome.SourceBranch)
	if err != nil {
		return err
	}
	if !runGo && !runRust {
		fmt.Printf("[control] integration gate: source=%s 无 niuma/bcc 变更，跳过项目 gate\n", outcome.SourceBranch)
		return nil
	}

	if runGo {
		goDir := filepath.Join(c.cfg.RepoDir, "automation", "niuma")
		if _, err := c.runCommand(ctx, goDir, "go", "test", "./..."); err != nil {
			return fmt.Errorf("integration gate(go) 失败: %w", err)
		}
	}

	if runRust {
		if _, err := c.runCommand(ctx, c.cfg.RepoDir, "cargo", "test", "-p", "bcc", "--no-run"); err != nil {
			return fmt.Errorf("integration gate(rust) 失败: %w", err)
		}
	}

	return nil
}

func (c *Controller) resolveIntegrationGateScopes(ctx context.Context, sourceBranch string) (bool, bool, error) {
	if sourceBranch == "" {
		return true, true, nil
	}

	out, err := c.runCommand(ctx, c.cfg.RepoDir, "git", "diff", "--name-only", "master..."+sourceBranch)
	if err != nil {
		// 差异读取失败时按全量 gate 执行，避免误放过失败。
		return true, true, nil
	}

	runGo := false
	runRust := false
	for _, file := range splitNonEmptyLines(out) {
		if strings.HasPrefix(file, "automation/niuma/") {
			runGo = true
		}
		if strings.HasPrefix(file, "compiler/bcc/") {
			runRust = true
		}
	}

	return runGo, runRust, nil
}

func (c *Controller) runCommand(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\noutput: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Controller) markIntegrationGatePending(task Task, attemptKey string) error {
	metaUpdate := map[string]string{
		metaKeyIntegrationGateStatus:        integrationGateStatusPending,
		metaKeyIntegrationGateAttemptKey:    attemptKey,
		metaKeyIntegrationGateLastCheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return c.taskctl.Update(task.ID, UpdateOpts{Metadata: &metaUpdate})
}

func (c *Controller) markIntegrationGatePassed(task Task, attemptKey string) error {
	metaUpdate := map[string]string{
		metaKeyIntegrationGateStatus:        integrationGateStatusPassed,
		metaKeyIntegrationGateAttemptKey:    attemptKey,
		metaKeyIntegrationGateLastCheckedAt: time.Now().UTC().Format(time.RFC3339),
		metaKeyIntegrationGateLastError:     "",
	}
	return c.taskctl.Update(task.ID, UpdateOpts{Metadata: &metaUpdate})
}

func shouldSkipProcessedIntegrationGateAttempt(meta map[string]string, attemptKey string) bool {
	if attemptKey == "" {
		return false
	}
	if valueOrEmpty(meta, metaKeyIntegrationGateAttemptKey) != attemptKey {
		return false
	}
	status := valueOrEmpty(meta, metaKeyIntegrationGateStatus)
	return status == integrationGateStatusRetrying || status == integrationGateStatusEscalated
}

func (c *Controller) handleIntegrationGateFailure(ctx context.Context, task Task, attemptKey string, gateErr error) error {
	if shouldSkipProcessedIntegrationGateAttempt(task.Metadata, attemptKey) {
		return nil
	}

	maxRetries := c.integrationGateMaxRetries()
	retryCount := integrationGateRetryCount(task.Metadata) + 1
	status := integrationGateStatusRetrying
	if retryCount > maxRetries {
		status = integrationGateStatusEscalated
	}

	metaUpdate := map[string]string{
		metaKeyIntegrationGateStatus:        status,
		metaKeyIntegrationGateRetryCount:    strconv.Itoa(retryCount),
		metaKeyIntegrationGateLastError:     trimIntegrationGateError(gateErr),
		metaKeyIntegrationGateLastCheckedAt: time.Now().UTC().Format(time.RFC3339),
		metaKeyIntegrationGateAttemptKey:    attemptKey,
	}
	if status == integrationGateStatusEscalated {
		metaUpdate[metaKeyIntegrationGateEscalationLabelSynced] = "false"
	}

	if err := c.taskctl.Update(task.ID, UpdateOpts{Metadata: &metaUpdate}); err != nil {
		return fmt.Errorf("写入 gate 失败 metadata 失败: %w", err)
	}

	if status == integrationGateStatusRetrying {
		c.signalIntegrationGateRetry(ctx, task, retryCount, maxRetries)
		return nil
	}

	c.syncIntegrationGateEscalationLabels(ctx, task)
	return nil
}

func integrationGateRetryCount(meta map[string]string) int {
	raw := valueOrEmpty(meta, metaKeyIntegrationGateRetryCount)
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func trimIntegrationGateError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	if len(msg) <= integrationGateErrorLimit {
		return msg
	}
	return msg[:integrationGateErrorLimit]
}

func (c *Controller) integrationGateMaxRetries() int {
	if c.cfg == nil {
		return integrationGateDefaultMaxRetries
	}
	if c.cfg.IntegrationGateMaxRetries < 0 {
		return integrationGateDefaultMaxRetries
	}
	return c.cfg.IntegrationGateMaxRetries
}

func (c *Controller) signalIntegrationGateRetry(ctx context.Context, task Task, retryCount, maxRetries int) {
	if task.IssueNum() <= 0 {
		return
	}
	if err := c.syncIssueStateLabel(ctx, task.IssueNum(), "bot:pr-needs-fix"); err != nil {
		fmt.Printf("[control] issue #%d gate retrying 打标失败: %v\n", task.IssueNum(), err)
		return
	}
	fmt.Printf("[control] issue #%d gate 失败，触发自动修复 retry=%d/%d\n", task.IssueNum(), retryCount, maxRetries)
}

func shouldRetryIntegrationGateEscalationLabels(meta map[string]string) bool {
	return isEscalatedIntegrationGateTask(meta) && !isIntegrationGateEscalationLabelSynced(meta)
}

func isEscalatedIntegrationGateTask(meta map[string]string) bool {
	return valueOrEmpty(meta, metaKeyIntegrationGateStatus) == integrationGateStatusEscalated
}

func isIntegrationGateEscalationLabelSynced(meta map[string]string) bool {
	return valueOrEmpty(meta, metaKeyIntegrationGateEscalationLabelSynced) == "true"
}

func (c *Controller) syncIntegrationGateEscalationLabels(ctx context.Context, task Task) {
	if task.IssueNum() <= 0 {
		fmt.Printf("[control] issue 编号缺失，无法升级 gate 失败 (task %s)\n", task.ID)
		return
	}

	if task.Metadata != nil && task.Metadata[metaKeyIntegrationGateEscalationLabelSynced] == "true" {
		return
	}

	if err := c.syncIssueStateLabel(ctx, task.IssueNum(), needsHumanLabel); err != nil {
		fmt.Printf("[control] issue #%d 打标 %s 失败: %v\n", task.IssueNum(), needsHumanLabel, err)
		return
	}
	if err := c.ensureIssueLabel(ctx, task.IssueNum(), integrationGateFailLabel); err != nil {
		fmt.Printf("[control] issue #%d 打标 %s 失败: %v\n", task.IssueNum(), integrationGateFailLabel, err)
		return
	}

	meta := map[string]string{metaKeyIntegrationGateEscalationLabelSynced: "true"}
	if err := c.taskctl.Update(task.ID, UpdateOpts{Metadata: &meta}); err != nil {
		fmt.Printf("[control] 写入 gate 升级打标状态失败 (task %s): %v\n", task.ID, err)
		return
	}
	fmt.Printf("[control] issue #%d gate 超限升级人工\n", task.IssueNum())
}

func (c *Controller) syncIssueStateLabel(ctx context.Context, issueNum int, targetLabel string) error {
	if targetState, err := state.ParseState(targetLabel); err == nil {
		return c.transitionWithSelfHeal(ctx, issueNum, "", targetState)
	}
	if err := c.clearIssueBotStates(ctx, issueNum); err != nil {
		return err
	}

	var firstErr error
	replacedAny := false
	for _, oldLabel := range integrationAutomationLabels {
		if state.IsBotLabel(oldLabel) {
			continue
		}
		// 避免 old==new 导致“先删后加”同一标签，触发无意义的 labeled/unlabeled 事件。
		if oldLabel == targetLabel {
			continue
		}
		replaced, err := c.github.ReplaceLabelIfPresent(ctx, issueNum, oldLabel, targetLabel)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if replaced {
			replacedAny = true
		}
	}
	if firstErr != nil {
		return firstErr
	}
	if !replacedAny {
		return c.ensureIssueLabel(ctx, issueNum, targetLabel)
	}
	return nil
}

func shouldRetryEscalationLabels(meta map[string]string) bool {
	return isEscalatedIntegrationTask(meta) && !isEscalationLabelSynced(meta)
}

func isEscalatedIntegrationTask(meta map[string]string) bool {
	return valueOrEmpty(meta, metaKeyIntegrationMergeStatus) == string(MergeStatusEscalated)
}

func isEscalationLabelSynced(meta map[string]string) bool {
	return valueOrEmpty(meta, metaKeyIntegrationConflictLabelSynced) == "true"
}

func mergeOutcomeFromEscalatedTask(task Task, integrationBranch string) MergeOutcome {
	return MergeOutcome{
		Status:            MergeStatusEscalated,
		IntegrationBranch: integrationBranch,
		SourceBranch:      task.Branch(),
		IssueNum:          task.IssueNum(),
		PRNum:             task.PRNum(),
		Conflict:          conflictSummaryFromMetadata(task.Metadata),
		ExecutorVersion:   valueOrEmpty(task.Metadata, metaKeyIntegrationExecutorVersion),
		ExecutedAt:        valueOrEmpty(task.Metadata, metaKeyIntegrationMergeExecutedAt),
	}
}

func conflictSummaryFromMetadata(meta map[string]string) *ConflictSummary {
	rawSummary := valueOrEmpty(meta, metaKeyIntegrationConflictSummary)
	if rawSummary != "" {
		var summary ConflictSummary
		if err := json.Unmarshal([]byte(rawSummary), &summary); err == nil {
			return &summary
		}
	}

	files := splitMetadataList(valueOrEmpty(meta, metaKeyIntegrationConflictFiles))
	totalHunks := 0
	if rawHunks := valueOrEmpty(meta, metaKeyIntegrationConflictTotalHunks); rawHunks != "" {
		if parsed, err := strconv.Atoi(rawHunks); err == nil {
			totalHunks = parsed
		}
	}

	reason := valueOrEmpty(meta, metaKeyIntegrationConflictReason)
	suggestion := valueOrEmpty(meta, metaKeyIntegrationConflictSuggestion)
	if len(files) == 0 && totalHunks == 0 && reason == "" && suggestion == "" {
		return nil
	}

	return &ConflictSummary{
		Files:           files,
		TotalHunkCount:  totalHunks,
		Reason:          reason,
		SuggestedAction: suggestion,
	}
}

func splitMetadataList(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return items
}

func (c *Controller) markTaskIntegrated(task Task, outcome MergeOutcome) error {
	metaUpdate := map[string]string{
		metaKeyIntegrated:                 "true",
		metaKeyIntegrationMergeStatus:     string(outcome.Status),
		metaKeyIntegrationMergeExecutedAt: outcome.ExecutedAt,
		metaKeyIntegrationExecutorVersion: outcome.ExecutorVersion,
	}
	if len(outcome.AutoResolvedFiles) > 0 {
		files := append([]string(nil), outcome.AutoResolvedFiles...)
		sort.Strings(files)
		metaUpdate[metaKeyIntegrationAutoResolvedFiles] = strings.Join(files, ",")
	}
	return c.taskctl.Update(task.ID, UpdateOpts{Metadata: &metaUpdate})
}

func (c *Controller) escalateIntegrationConflict(ctx context.Context, task Task, outcome MergeOutcome) {
	metadataUpdate := buildEscalationMetadata(task.Metadata, outcome)
	if len(metadataUpdate) > 0 {
		if err := c.taskctl.Update(task.ID, UpdateOpts{Metadata: &metadataUpdate}); err != nil {
			fmt.Printf("[control] 写入冲突 metadata 失败 (task %s): %v\n", task.ID, err)
		}
	}

	if task.IssueNum() <= 0 {
		fmt.Printf("[control] issue 编号缺失，无法打冲突标签 (task %s)\n", task.ID)
		return
	}

	if task.Metadata != nil && task.Metadata[metaKeyIntegrationConflictLabelSynced] == "true" {
		fmt.Printf("[control] issue #%d 已存在升级标签，跳过重复打标\n", task.IssueNum())
		return
	}

	if err := c.clearIssueBotStates(ctx, task.IssueNum()); err != nil {
		fmt.Printf("[control] issue #%d 清理 bot 状态失败: %v\n", task.IssueNum(), err)
		return
	}
	if err := c.ensureIssueLabel(ctx, task.IssueNum(), integrationConflictLabel); err != nil {
		fmt.Printf("[control] issue #%d 打标 %s 失败: %v\n", task.IssueNum(), integrationConflictLabel, err)
		return
	}
	if err := c.ensureIssueLabel(ctx, task.IssueNum(), needsHumanLabel); err != nil {
		fmt.Printf("[control] issue #%d 打标 %s 失败: %v\n", task.IssueNum(), needsHumanLabel, err)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	labelMeta := map[string]string{
		metaKeyIntegrationConflictLabelSynced: "true",
		metaKeyEscalatedAt:                    now,
	}
	if err := c.taskctl.Update(task.ID, UpdateOpts{Metadata: &labelMeta}); err != nil {
		fmt.Printf("[control] 写入打标状态失败 (task %s): %v\n", task.ID, err)
	}

	if outcome.Conflict != nil {
		fmt.Printf(
			"[control] 合入 %s 升级人工: issue #%d files=%d hunks=%d reason=%s\n",
			outcome.SourceBranch,
			task.IssueNum(),
			len(outcome.Conflict.Files),
			outcome.Conflict.TotalHunkCount,
			outcome.Conflict.Reason,
		)
	} else {
		fmt.Printf("[control] 合入 %s 升级人工: issue #%d\n", outcome.SourceBranch, task.IssueNum())
	}
}

func buildEscalationMetadata(existing map[string]string, outcome MergeOutcome) map[string]string {
	update := make(map[string]string)

	setMetadataIfChanged(update, existing, metaKeyIntegrationMergeStatus, string(MergeStatusEscalated))
	setMetadataIfChanged(update, existing, metaKeyIntegrationExecutorVersion, outcome.ExecutorVersion)
	setMetadataIfChanged(update, existing, metaKeyIntegrationMergeExecutedAt, outcome.ExecutedAt)

	if valueOrEmpty(existing, metaKeyIntegrationConflictRecordedAt) != "" {
		return update
	}

	setMetadataIfChanged(update, existing, metaKeyIntegrationConflictRecordedAt, time.Now().UTC().Format(time.RFC3339))
	if outcome.Conflict == nil {
		return update
	}

	payload, err := json.Marshal(outcome.Conflict)
	if err == nil {
		setMetadataIfChanged(update, existing, metaKeyIntegrationConflictSummary, string(payload))
	}

	if len(outcome.Conflict.Files) > 0 {
		files := append([]string(nil), outcome.Conflict.Files...)
		sort.Strings(files)
		setMetadataIfChanged(update, existing, metaKeyIntegrationConflictFiles, strings.Join(files, ","))
	}

	if outcome.Conflict.TotalHunkCount > 0 {
		setMetadataIfChanged(update, existing, metaKeyIntegrationConflictTotalHunks, strconv.Itoa(outcome.Conflict.TotalHunkCount))
	}
	setMetadataIfChanged(update, existing, metaKeyIntegrationConflictReason, outcome.Conflict.Reason)
	setMetadataIfChanged(update, existing, metaKeyIntegrationConflictSuggestion, outcome.Conflict.SuggestedAction)

	return update
}

func setMetadataIfChanged(update, existing map[string]string, key, value string) {
	if value == "" {
		return
	}
	if valueOrEmpty(existing, key) == value {
		return
	}
	update[key] = value
}

func valueOrEmpty(meta map[string]string, key string) string {
	if meta == nil {
		return ""
	}
	return meta[key]
}

func uniquePositiveIssueNumbers(nums []int) []int {
	unique := make(map[int]struct{}, len(nums))
	for _, num := range nums {
		if num <= 0 {
			continue
		}
		unique[num] = struct{}{}
	}

	result := make([]int, 0, len(unique))
	for num := range unique {
		result = append(result, num)
	}
	sort.Ints(result)
	return result
}

// tryResolveIntegrationConflictWithAI 在 integration merge 冲突后尝试 AI 解冲突。
// 成功返回 true（merge commit 已完成），失败返回 false（已 abort）。
func (c *Controller) tryResolveIntegrationConflictWithAI(ctx context.Context, task Task, outcome MergeOutcome) bool {
	repoDir := c.prConflictRepoDir()
	if repoDir == "" {
		fmt.Printf("[control] repoDir 为空，跳过 AI 解冲突 (task %s)\n", task.ID)
		return false
	}

	// abortAndEscalate 已执行 git merge --abort，需要重新 merge 重现冲突
	if _, err := c.runCommand(ctx, repoDir, "git", "checkout", outcome.IntegrationBranch); err != nil {
		fmt.Printf("[control] checkout %s 失败，跳过 AI 解冲突: %v\n", outcome.IntegrationBranch, err)
		return false
	}

	if _, err := c.runCommand(ctx, repoDir, "git", "merge", "--no-ff", "--no-commit", outcome.SourceBranch); err == nil {
		// merge 成功说明冲突已消解，直接 commit
		if _, commitErr := c.runCommand(ctx, repoDir, "git", "commit", "--no-edit"); commitErr != nil {
			fmt.Printf("[control] 冲突已消解但 commit 失败: %v\n", commitErr)
			c.runCommand(ctx, repoDir, "git", "merge", "--abort")
			return false
		}
		fmt.Printf("[control] 冲突已消解，直接合入 %s (issue #%d)\n", outcome.SourceBranch, task.IssueNum())
		return true
	}

	// 收集冲突详情
	conflictFiles, summaries, err := c.collectPRConflictDetails(ctx, repoDir)
	if err != nil || len(conflictFiles) == 0 {
		fmt.Printf("[control] 收集冲突详情失败或无冲突文件，放弃 AI 解冲突: %v\n", err)
		c.runCommand(ctx, repoDir, "git", "merge", "--abort")
		return false
	}

	// 检查 AI 白名单
	allowed, reason := c.allowAIConflictResolution(conflictFiles, summaries)
	if !allowed {
		fmt.Printf("[control] 冲突不在 AI 白名单内 (%s)，放弃 AI 解冲突 (issue #%d)\n", reason, task.IssueNum())
		c.runCommand(ctx, repoDir, "git", "merge", "--abort")
		return false
	}

	// 构建 profile groups
	profileGroups, err := ResolveConflictProfileGroups(conflictFiles)
	if err != nil {
		fmt.Printf("[control] 解析 profile groups 失败: %v\n", err)
		c.runCommand(ctx, repoDir, "git", "merge", "--abort")
		return false
	}

	// 调用 AI 解冲突
	result, err := c.tryResolveConflictByAIOnce(ctx, repoDir, conflictFiles, summaries, profileGroups)
	if err != nil || result == nil || len(result.SuccessFiles) != len(conflictFiles) {
		fmt.Printf("[control] AI 解冲突失败 (issue #%d): err=%v\n", task.IssueNum(), err)
		c.runCommand(ctx, repoDir, "git", "merge", "--abort")
		return false
	}

	// AI 解冲突成功，只 git add 冲突文件（避免将无关变更带入 merge commit）
	gitAddArgs := append([]string{"add", "--"}, conflictFiles...)
	if _, err := c.runCommand(ctx, repoDir, "git", gitAddArgs...); err != nil {
		fmt.Printf("[control] AI 解冲突后 git add 失败: %v\n", err)
		c.runCommand(ctx, repoDir, "git", "merge", "--abort")
		return false
	}
	if _, err := c.runCommand(ctx, repoDir, "git", "commit", "--no-edit"); err != nil {
		fmt.Printf("[control] AI 解冲突后 commit 失败: %v\n", err)
		c.runCommand(ctx, repoDir, "git", "merge", "--abort")
		return false
	}

	fmt.Printf("[control] AI 解冲突成功，已合入 %s (issue #%d, files=%d)\n",
		outcome.SourceBranch, task.IssueNum(), len(conflictFiles))
	return true
}

// handleIntegrationConflictRetry 在 AI 解冲突失败后执行 retry 机制。
// retryCount <= threshold: 回退到 bot:pr-needs-fix 让 bot rebase PR
// retryCount > threshold: 走原有 escalate 路径
func (c *Controller) handleIntegrationConflictRetry(ctx context.Context, task Task, outcome MergeOutcome) {
	issueNum := task.IssueNum()
	if issueNum <= 0 {
		c.escalateIntegrationConflict(ctx, task, outcome)
		return
	}

	issue, err := c.github.GetIssue(ctx, issueNum)
	if err != nil {
		fmt.Printf("[control] 读取 issue #%d 失败，直接 escalate: %v\n", issueNum, err)
		c.escalateIntegrationConflict(ctx, task, outcome)
		return
	}

	retryCount := parseIntegrationConflictRetryCount(issue.Body) + 1
	threshold := integrationConflictRetryDefaultThreshold

	if retryCount > threshold {
		fmt.Printf("[control] integration 冲突 retry 超限 (%d > %d)，escalate issue #%d\n", retryCount, threshold, issueNum)
		c.escalateIntegrationConflict(ctx, task, outcome)
		return
	}

	// 更新 retry marker（失败则 escalate，避免 retryCount 无法递增导致无限重试）
	if err := c.persistIntegrationConflictRetryCount(ctx, issue, retryCount); err != nil {
		fmt.Printf("[control] 写入 integration retry marker 失败 (issue #%d)，escalate: %v\n", issueNum, err)
		c.escalateIntegrationConflict(ctx, task, outcome)
		return
	}

	// 写 comment 标注来源
	commentBody := fmt.Sprintf(
		"%s\n\n**Integration merge 冲突** (retry %d/%d)\n\n"+
			"源分支 `%s` 在合入 integration 分支 `%s` 时发生冲突，AI 解冲突未能处理。\n"+
			"已回退到 `bot:pr-needs-fix`，等待 bot rebase 后重新尝试。",
		integrationConflictCommentMarker, retryCount, threshold,
		outcome.SourceBranch, outcome.IntegrationBranch,
	)
	if err := c.github.AddIssueComment(ctx, issueNum, commentBody); err != nil {
		fmt.Printf("[control] 写入 integration 冲突 comment 失败 (issue #%d): %v\n", issueNum, err)
	}

	// 回退到 bot:pr-needs-fix
	if err := c.syncIssueStateLabel(ctx, issueNum, "bot:pr-needs-fix"); err != nil {
		fmt.Printf("[control] 回退 issue #%d 到 bot:pr-needs-fix 失败: %v\n", issueNum, err)
		c.escalateIntegrationConflict(ctx, task, outcome)
		return
	}

	fmt.Printf("[control] integration 冲突 retry %d/%d: issue #%d 回退到 bot:pr-needs-fix\n", retryCount, threshold, issueNum)
}

func parseIntegrationConflictRetryCount(body string) int {
	matches := integrationConflictRetryMarkerRe.FindStringSubmatch(body)
	if len(matches) < 2 {
		return 0
	}
	count, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return count
}

func upsertIntegrationConflictRetryMarker(body string, retryCount int) (string, bool) {
	markerLine := fmt.Sprintf("<!-- INTEGRATION_CONFLICT_RETRY:%d -->", retryCount)
	if loc := integrationConflictRetryMarkerRe.FindStringIndex(body); loc != nil {
		current := body[loc[0]:loc[1]]
		if current == markerLine {
			return body, false
		}
		return body[:loc[0]] + markerLine + body[loc[1]:], true
	}

	trimmed := strings.TrimRight(body, "\n")
	if strings.TrimSpace(trimmed) == "" {
		return markerLine, true
	}
	return trimmed + "\n\n" + markerLine, true
}

func (c *Controller) persistIntegrationConflictRetryCount(ctx context.Context, issue IssueInfo, retryCount int) error {
	updatedBody, changed := upsertIntegrationConflictRetryMarker(issue.Body, retryCount)
	if !changed {
		return nil
	}
	return c.github.UpdateIssueBody(ctx, issue.Number, updatedBody)
}

// reconcileNeedsHumanRecovery 检查带 needs-human + integration-conflict 标签的 issue，
// 如果关联 PR 恢复 MERGEABLE，则自动恢复到 bot:pr-reviewable。
func (c *Controller) reconcileNeedsHumanRecovery(ctx context.Context, tasks []Task) {
	for _, task := range tasks {
		issueNum := task.IssueNum()
		if issueNum <= 0 {
			continue
		}

		// 冷却期检查（需要 metadata 中有 escalated_at）
		if escalatedAt := valueOrEmpty(task.Metadata, metaKeyEscalatedAt); escalatedAt != "" {
			if t, err := time.Parse(time.RFC3339, escalatedAt); err == nil {
				nowFn := c.nowFn
				if nowFn == nil {
					nowFn = time.Now
				}
				if nowFn().Sub(t) < needsHumanRecoveryCooldown {
					continue
				}
			}
		}

		// 确认 issue 确实带 needs-human + integration-conflict
		labels, err := c.github.ListLabels(ctx, issueNum)
		if err != nil {
			continue
		}
		if !hasLabel(labels, needsHumanLabel) || !hasLabel(labels, integrationConflictLabel) {
			continue
		}

		// 查询 PR mergeable 状态
		reviewStatus, err := c.github.ResolvePRReviewStatus(ctx, issueNum)
		if err != nil {
			continue
		}
		if !reviewStatus.IsMergeable() {
			continue
		}

		// PR 恢复 MERGEABLE，清除标签恢复流转
		fmt.Printf("[control] issue #%d PR 已恢复 MERGEABLE，自动恢复到 bot:pr-reviewable\n", issueNum)

		// 恢复到 bot:pr-reviewable
		if err := c.syncIssueStateLabel(ctx, issueNum, "bot:pr-reviewable"); err != nil {
			fmt.Printf("[control] issue #%d 恢复到 bot:pr-reviewable 失败: %v\n", issueNum, err)
			continue
		}

		// 清除 integration-conflict 和 needs-human 标签（syncIssueStateLabel 已处理 bot:* 标签切换，
		// 但 integration-conflict 和 needs-human 不是 bot:* 前缀，需要通过 ReplaceLabels 处理）
		labelsCleaned := false
		currentLabels, err := c.github.ListLabels(ctx, issueNum)
		if err == nil {
			filtered := make([]string, 0, len(currentLabels))
			for _, l := range currentLabels {
				if l != integrationConflictLabel && l != needsHumanLabel {
					filtered = append(filtered, l)
				}
			}
			if len(filtered) != len(currentLabels) {
				if err := c.github.ReplaceLabels(ctx, issueNum, filtered); err != nil {
					fmt.Printf("[control] issue #%d 清除冲突标签失败: %v\n", issueNum, err)
				} else {
					labelsCleaned = true
				}
			} else {
				labelsCleaned = true
			}
		}

		// 只有标签清理成功才清除 metadata 和写恢复 comment，否则保留以便下次 reconcile 重试
		if labelsCleaned {
			// 恢复流转后重置 integration conflict retry 计数，
			// 避免下一轮因历史超限计数再次立即升级到 needs-human。
			if issue, err := c.github.GetIssue(ctx, issueNum); err == nil {
				if parseIntegrationConflictRetryCount(issue.Body) > 0 {
					if err := c.persistIntegrationConflictRetryCount(ctx, issue, 0); err != nil {
						fmt.Printf("[control] issue #%d 重置 integration retry marker 失败: %v\n", issueNum, err)
					}
				}
			} else {
				fmt.Printf("[control] issue #%d 读取 body 失败，跳过 retry marker 重置: %v\n", issueNum, err)
			}

			clearMeta := map[string]string{
				metaKeyIntegrationConflictLabelSynced: "",
				metaKeyEscalatedAt:                    "",
			}
			if err := c.taskctl.Update(task.ID, UpdateOpts{Metadata: &clearMeta}); err != nil {
				fmt.Printf("[control] 清除 task %s 的 escalation metadata 失败: %v\n", task.ID, err)
			}

			recoveryComment := fmt.Sprintf(
				"<!-- BOT:NEEDS_HUMAN_RECOVERY -->\n\n"+
					"**自动恢复**: issue #%d PR 已恢复 MERGEABLE 状态，从 `needs-human` + `integration-conflict` 恢复到 `bot:pr-reviewable`。",
				issueNum,
			)
			if err := c.github.AddIssueComment(ctx, issueNum, recoveryComment); err != nil {
				fmt.Printf("[control] issue #%d 写恢复 comment 失败: %v\n", issueNum, err)
			}
		}
	}
}
