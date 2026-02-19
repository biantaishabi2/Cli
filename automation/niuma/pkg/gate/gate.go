package gate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	metaKeyIssueNum                 = "issue_num"
	metaKeyGateStatus               = "integration_gate_status"
	metaKeyGateRetryCount           = "integration_gate_retry_count"
	metaKeyGateLastError            = "integration_gate_last_error"
	metaKeyGateLastCheckedAt        = "integration_gate_last_checked_at"
	metaKeyGateAttemptKey           = "integration_gate_attempt_key"
	metaKeyLastEscalatedAttemptKey  = "last_escalated_attempt_key"
	gateStatusRetrying              = "retrying"
	gateStatusEscalated             = "escalated"
	maxGateErrorMessageLen          = 800
	needsHumanLabel                 = "needs-human"
	integrationGateFailedLabel      = "integration-gate-failed"
	defaultGateScriptRelativePath   = ".github/scripts/niuma-test-gate.sh"
	defaultNeedsFixCommentTemplate  = "## ❌ Gate 未通过\n\n自动测试门禁失败，已回退为 `bot:pr-needs-fix`，请继续迭代修复。\n\n- retry_count=%d\n- max_retries=%d\n- attempt_key=`%s`"
	defaultEscalatedCommentTemplate = "## 🚨 Gate 已超限，升级人工处理\n\nintegration gate 失败次数已超过上限，本轮不再触发自动修复。\n\n- retry_count=%d\n- max_retries=%d\n- attempt_key=`%s`"
)

var ErrGateFailed = errors.New("gate failed")

type RunGateFunc func(ctx context.Context, repoDir string, pr int) (string, error)
type MarkNeedsFixFunc func(ctx context.Context, repo string, issue int) error
type AddLabelsFunc func(ctx context.Context, repo string, issue int, labels []string) error
type AddCommentFunc func(ctx context.Context, repo string, issue int, body string) error

type Options struct {
	Repo       string
	Issue      int
	PR         int
	RepoDir    string
	MaxRetries int

	Now          func() time.Time
	RunGate      RunGateFunc
	MarkNeedsFix MarkNeedsFixFunc
	AddLabels    AddLabelsFunc
	AddComment   AddCommentFunc
}

type Runner struct {
	opts Options
}

type Result struct {
	Passed                   bool
	RetryCount               int
	MaxRetries               int
	AttemptKey               string
	Escalated                bool
	EscalationCommentSkipped bool
	StatePersisted           bool
}

func NewRunner(opts Options) (*Runner, error) {
	if strings.TrimSpace(opts.Repo) == "" {
		return nil, fmt.Errorf("repo 不能为空")
	}
	if opts.Issue <= 0 {
		return nil, fmt.Errorf("issue 必须大于 0")
	}
	if opts.PR <= 0 {
		return nil, fmt.Errorf("pr 必须大于 0")
	}
	if opts.MaxRetries < 0 {
		return nil, fmt.Errorf("max_retries 不能小于 0")
	}
	if opts.RepoDir == "" {
		opts.RepoDir = "."
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.RunGate == nil {
		opts.RunGate = runGateScript
	}
	if opts.MarkNeedsFix == nil {
		return nil, fmt.Errorf("MarkNeedsFix 未配置")
	}
	if opts.AddLabels == nil {
		return nil, fmt.Errorf("AddLabels 未配置")
	}
	if opts.AddComment == nil {
		return nil, fmt.Errorf("AddComment 未配置")
	}
	return &Runner{opts: opts}, nil
}

func (r *Runner) Run(ctx context.Context) (Result, error) {
	result := Result{MaxRetries: r.opts.MaxRetries}

	gateOutput, gateErr := r.opts.RunGate(ctx, r.opts.RepoDir, r.opts.PR)
	if gateErr == nil {
		result.Passed = true
		return result, nil
	}

	attemptKey := buildAttemptKey(r.opts.Issue, r.opts.PR, r.opts.Now())
	storePath := filepath.Join(r.opts.RepoDir, ".niuma", "tasks.json")
	result.AttemptKey = attemptKey
	gateFailure := trimGateError(gateOutput, gateErr)

	lockPath := storePath + ".lock"
	if err := withTaskStoreLock(lockPath, func() error {
		store, err := loadTaskStore(storePath, r.opts.Issue)
		if err != nil {
			return fmt.Errorf("读取 tasks.json 失败: %w", err)
		}

		retryCount := 1
		lastEscalatedAttemptKey := ""
		if store.hasMatchedTask() {
			retryCount = parseNonNegativeInt(store.metadata[metaKeyGateRetryCount]) + 1
			lastEscalatedAttemptKey = parseString(store.metadata[metaKeyLastEscalatedAttemptKey])
		}

		result.RetryCount = retryCount
		status := gateStatusRetrying
		if retryCount > r.opts.MaxRetries {
			status = gateStatusEscalated
			result.Escalated = true
		}

		if store.hasMatchedTask() {
			store.metadata[metaKeyGateStatus] = status
			store.metadata[metaKeyGateRetryCount] = strconv.Itoa(retryCount)
			store.metadata[metaKeyGateAttemptKey] = attemptKey
			store.metadata[metaKeyGateLastCheckedAt] = r.opts.Now().UTC().Format(time.RFC3339)
			store.metadata[metaKeyGateLastError] = gateFailure
			if err := store.save(); err != nil {
				return fmt.Errorf("写入 tasks.json 失败: %w", err)
			}
			result.StatePersisted = true
		}

		if !result.Escalated {
			if err := r.opts.MarkNeedsFix(ctx, r.opts.Repo, r.opts.Issue); err != nil {
				return fmt.Errorf("%w: gate 失败后设置 bot:pr-needs-fix 失败: %v", ErrGateFailed, err)
			}
			commentBody := fmt.Sprintf(defaultNeedsFixCommentTemplate, retryCount, r.opts.MaxRetries, attemptKey)
			if err := r.opts.AddComment(ctx, r.opts.Repo, r.opts.Issue, commentBody); err != nil {
				return fmt.Errorf("%w: gate 失败评论发布失败: %v", ErrGateFailed, err)
			}
			return fmt.Errorf("%w: %s", ErrGateFailed, gateFailure)
		}

		if err := r.opts.AddLabels(ctx, r.opts.Repo, r.opts.Issue, []string{needsHumanLabel, integrationGateFailedLabel}); err != nil {
			return fmt.Errorf("%w: gate 超限后打标失败: %v", ErrGateFailed, err)
		}

		if lastEscalatedAttemptKey == attemptKey {
			result.EscalationCommentSkipped = true
			return fmt.Errorf("%w: %s", ErrGateFailed, gateFailure)
		}

		escalatedComment := fmt.Sprintf(defaultEscalatedCommentTemplate, retryCount, r.opts.MaxRetries, attemptKey)
		if err := r.opts.AddComment(ctx, r.opts.Repo, r.opts.Issue, escalatedComment); err != nil {
			return fmt.Errorf("%w: gate 超限升级评论发布失败: %v", ErrGateFailed, err)
		}

		if store.hasMatchedTask() {
			store.metadata[metaKeyLastEscalatedAttemptKey] = attemptKey
			if err := store.save(); err != nil {
				return fmt.Errorf("写入 last_escalated_attempt_key 失败: %w", err)
			}
			result.StatePersisted = true
		}

		return fmt.Errorf("%w: %s", ErrGateFailed, gateFailure)
	}); err != nil {
		return result, err
	}

	return result, fmt.Errorf("%w: %s", ErrGateFailed, gateFailure)
}

func runGateScript(ctx context.Context, repoDir string, pr int) (string, error) {
	scriptPath := filepath.Join(repoDir, defaultGateScriptRelativePath)
	cmd := exec.CommandContext(ctx, scriptPath, strconv.Itoa(pr))
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func buildAttemptKey(issue, pr int, now time.Time) string {
	return fmt.Sprintf("issue-%d-pr-%d-%s", issue, pr, now.UTC().Format("20060102"))
}

func trimGateError(output string, runErr error) string {
	parts := make([]string, 0, 2)
	trimmedOutput := strings.TrimSpace(output)
	trimmedErr := ""
	if runErr != nil {
		trimmedErr = strings.TrimSpace(runErr.Error())
	}

	if trimmedOutput != "" {
		parts = append(parts, trimmedOutput)
	}
	if trimmedErr != "" && (trimmedOutput == "" || !strings.Contains(trimmedOutput, trimmedErr)) {
		parts = append(parts, trimmedErr)
	}
	joined := strings.TrimSpace(strings.Join(parts, " | "))
	if joined == "" {
		joined = "gate 命令失败"
	}
	if len(joined) <= maxGateErrorMessageLen {
		return joined
	}
	return joined[:maxGateErrorMessageLen]
}

type taskStore struct {
	path     string
	root     any
	metadata map[string]any
}

func loadTaskStore(path string, issue int) (*taskStore, error) {
	store := &taskStore{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return store, nil
	}

	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	store.root = root
	store.metadata = findTaskMetadataByIssue(root, issue)
	return store, nil
}

func (s *taskStore) hasMatchedTask() bool {
	return s != nil && s.root != nil && s.metadata != nil
}

func (s *taskStore) save() error {
	if !s.hasMatchedTask() {
		return nil
	}
	payload, err := json.MarshalIndent(s.root, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return writeAtomicJSON(s.path, payload)
}

func findTaskMetadataByIssue(root any, issue int) map[string]any {
	switch typed := root.(type) {
	case []any:
		for _, item := range typed {
			task, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if taskIssueNum(task) != issue {
				continue
			}
			return ensureTaskMetadata(task)
		}
	case map[string]any:
		if taskIssueNum(typed) == issue {
			return ensureTaskMetadata(typed)
		}
		tasksObj, ok := typed["tasks"]
		if !ok {
			return nil
		}
		switch tasksTyped := tasksObj.(type) {
		case map[string]any:
			for _, value := range tasksTyped {
				task, ok := value.(map[string]any)
				if !ok {
					continue
				}
				if taskIssueNum(task) != issue {
					continue
				}
				return ensureTaskMetadata(task)
			}
		case []any:
			for _, value := range tasksTyped {
				task, ok := value.(map[string]any)
				if !ok {
					continue
				}
				if taskIssueNum(task) != issue {
					continue
				}
				return ensureTaskMetadata(task)
			}
		}
	}
	return nil
}

func taskIssueNum(task map[string]any) int {
	if task == nil {
		return 0
	}
	meta := taskMetadata(task)
	if meta != nil {
		if issue := parseNonNegativeInt(meta[metaKeyIssueNum]); issue > 0 {
			return issue
		}
	}
	return parseNonNegativeInt(task[metaKeyIssueNum])
}

func taskMetadata(task map[string]any) map[string]any {
	if task == nil {
		return nil
	}
	existing, ok := task["metadata"]
	if !ok || existing == nil {
		return nil
	}
	if metadata, ok := existing.(map[string]any); ok {
		return metadata
	}
	if metadata, ok := existing.(map[string]string); ok {
		converted := make(map[string]any, len(metadata))
		for k, v := range metadata {
			converted[k] = v
		}
		return converted
	}
	return nil
}

func ensureTaskMetadata(task map[string]any) map[string]any {
	if task == nil {
		return nil
	}

	existing := taskMetadata(task)
	if existing == nil {
		meta := map[string]any{}
		task["metadata"] = meta
		return meta
	}
	task["metadata"] = existing
	return existing
}

func parseNonNegativeInt(v any) int {
	switch typed := v.(type) {
	case nil:
		return 0
	case int:
		if typed < 0 {
			return 0
		}
		return typed
	case int64:
		if typed < 0 {
			return 0
		}
		return int(typed)
	case float64:
		if typed < 0 {
			return 0
		}
		return int(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil || parsed < 0 {
			return 0
		}
		return parsed
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil || parsed < 0 {
			return 0
		}
		return int(parsed)
	default:
		return 0
	}
}

func parseString(v any) string {
	switch typed := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func writeAtomicJSON(path string, payload []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := file.Name()
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := file.Write(payload); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	tmpPath = ""
	return nil
}

func withTaskStoreLock(lockPath string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return fmt.Errorf("创建锁目录失败: %w", err)
	}
	fd, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("打开 store 锁失败: %w", err)
	}
	defer fd.Close()

	if err := syscall.Flock(int(fd.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("加锁 store 失败: %w", err)
	}
	defer func() {
		_ = syscall.Flock(int(fd.Fd()), syscall.LOCK_UN)
	}()
	return fn()
}
