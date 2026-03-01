package gate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	maxGateErrorMessageLen          = 800
	needsHumanLabel                 = "needs-human"
	integrationGateFailedLabel      = "integration-gate-failed" // 瞬时失败辅助标签：仅表达“当前 gate 阻塞”，恢复后由 control 归一化清理
	defaultGateScriptRelativePath   = ".github/scripts/niuma-test-gate.sh"
	defaultNeedsFixCommentTemplate  = "## ❌ Gate 未通过\n\n自动测试门禁失败，已回退为 `bot:pr-needs-fix`，请继续迭代修复。\n\n- retry_count=%d\n- max_retries=%d\n- attempt_key=`%s`"
	defaultDeferredCommentTemplate  = "## ⏳ Gate 暂未形成代码失败结论\n\n本轮 gate 失败原因为 `%s`（通常是 checks 未就绪或查询异常），已保留当前状态，不回退 `bot:pr-needs-fix`，避免无效 iterate 循环。\n\n- attempt_key=`%s`"
	defaultEscalatedCommentTemplate = "## 🚨 Gate 已超限，升级人工处理\n\nintegration gate 失败次数已超过上限，本轮不再触发自动修复。\n\n- retry_count=%d\n- max_retries=%d\n- attempt_key=`%s`"
)

var ErrGateFailed = errors.New("gate failed")

type FailureClass string

const (
	FailureClassCode     FailureClass = "code_failure"
	FailureClassDeferred FailureClass = "deferred"
)

type RunGateFunc func(ctx context.Context, repoDir string, pr int) (string, error)
type MarkNeedsFixFunc func(ctx context.Context, repo string, issue int) error
type AddLabelsFunc func(ctx context.Context, repo string, issue int, labels []string) error
type AddCommentFunc func(ctx context.Context, repo string, issue int, body string) error
type FindGateRetryStateFunc func(ctx context.Context, issue int) (int, string, error)
type UpsertGateRetryCountFunc func(ctx context.Context, issue int, count int, attemptKey string) error
type HasLabelFunc func(ctx context.Context, issue int, label string) (bool, error)
type AddPRReviewFunc func(ctx context.Context, repo string, pr int, body string) error

type Options struct {
	Repo       string
	Issue      int
	PR         int
	RepoDir    string
	MaxRetries int
	RunID      string
	RunAttempt string
	SelfCheck  bool // self-check 模式：gate 失败时不设标签/不写 issue 评论，仅写 PR review

	Now                  func() time.Time
	RunGate              RunGateFunc
	MarkNeedsFix         MarkNeedsFixFunc
	AddLabels            AddLabelsFunc
	AddComment           AddCommentFunc
	FindGateRetryState   FindGateRetryStateFunc
	UpsertGateRetryCount UpsertGateRetryCountFunc
	HasLabel             HasLabelFunc
	AddPRReview          AddPRReviewFunc // 可选：gate 失败时将错误详情写到 PR review，供 iterate 读取
}

type Runner struct {
	opts Options
}

type Result struct {
	Passed                   bool
	RetryCount               int
	MaxRetries               int
	AttemptKey               string
	ReasonCode               string
	FailureClass             FailureClass
	Escalated                bool
	EscalationCommentSkipped bool
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
	if opts.FindGateRetryState == nil {
		return nil, fmt.Errorf("FindGateRetryState 未配置")
	}
	if opts.UpsertGateRetryCount == nil {
		return nil, fmt.Errorf("UpsertGateRetryCount 未配置")
	}
	if opts.HasLabel == nil {
		return nil, fmt.Errorf("HasLabel 未配置")
	}
	return &Runner{opts: opts}, nil
}

func (r *Runner) Run(ctx context.Context) (Result, error) {
	result := Result{MaxRetries: r.opts.MaxRetries}

	gateOutput, gateErr := r.opts.RunGate(ctx, r.opts.RepoDir, r.opts.PR)
	if gateErr == nil {
		// gate 通过，重置计数
		result.Passed = true
		result.ReasonCode = "PASS"
		if err := r.opts.UpsertGateRetryCount(ctx, r.opts.Issue, 0, ""); err != nil {
			return result, fmt.Errorf("gate 通过后重置 retry_count 失败: %w", err)
		}
		return result, nil
	}

	reasonCode := normalizeReasonCode(extractGateField(gateOutput, "reason_code"))
	if reasonCode == "" {
		reasonCode = "UNKNOWN"
	}
	result.ReasonCode = reasonCode
	result.FailureClass = classifyFailureReason(reasonCode)

	headSHA := extractGateField(gateOutput, "head_sha")
	attemptKey := buildAttemptKey(r.opts.Issue, r.opts.PR, r.opts.Now(), r.opts.RunID, r.opts.RunAttempt, headSHA)
	result.AttemptKey = attemptKey
	gateFailure := trimGateError(gateOutput, gateErr)

	// checks 未就绪/查询异常等未决失败，不应触发 pr-needs-fix 循环。
	if result.FailureClass == FailureClassDeferred {
		result.RetryCount = 0
		if err := r.opts.UpsertGateRetryCount(ctx, r.opts.Issue, 0, ""); err != nil {
			return result, fmt.Errorf("gate 未决场景重置 retry_count 失败: %w", err)
		}
		if !r.opts.SelfCheck {
			commentBody := fmt.Sprintf(defaultDeferredCommentTemplate, result.ReasonCode, attemptKey)
			if err := r.opts.AddComment(ctx, r.opts.Repo, r.opts.Issue, commentBody); err != nil {
				return result, fmt.Errorf("%w: gate 未决评论发布失败: %v", ErrGateFailed, err)
			}
		}
		if r.opts.AddPRReview != nil {
			reviewBody := fmt.Sprintf("## ⏳ Gate 暂未形成代码失败结论\n\n```\n%s\n```\n\n- reason_code=%s\n- attempt_key=`%s`", gateFailure, result.ReasonCode, attemptKey)
			if err := r.opts.AddPRReview(ctx, r.opts.Repo, r.opts.PR, reviewBody); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: gate 未决信息写入 PR review 失败: %v\n", err)
			}
		}
		return result, fmt.Errorf("%w: gate deferred (%s): %s", ErrGateFailed, result.ReasonCode, gateFailure)
	}

	// 从 marker 读取上次 retry_count（仅同 attempt_key 延续；否则从 0 开始）
	prevCount, prevAttemptKey, err := r.opts.FindGateRetryState(ctx, r.opts.Issue)
	if err != nil {
		return result, fmt.Errorf("读取 gate retry_count 失败: %w", err)
	}
	if prevAttemptKey != attemptKey {
		prevCount = 0
	}
	retryCount := prevCount + 1
	result.RetryCount = retryCount

	// 写回新计数
	if err := r.opts.UpsertGateRetryCount(ctx, r.opts.Issue, retryCount, attemptKey); err != nil {
		return result, fmt.Errorf("写入 gate retry_count 失败: %w", err)
	}

	if retryCount <= r.opts.MaxRetries {
		if !r.opts.SelfCheck {
			// 正常模式：设标签 + 写 issue 评论
			if err := r.opts.MarkNeedsFix(ctx, r.opts.Repo, r.opts.Issue); err != nil {
				return result, fmt.Errorf("%w: gate 失败后设置 bot:pr-needs-fix 失败: %v", ErrGateFailed, err)
			}
			commentBody := fmt.Sprintf(defaultNeedsFixCommentTemplate, retryCount, r.opts.MaxRetries, attemptKey)
			if err := r.opts.AddComment(ctx, r.opts.Repo, r.opts.Issue, commentBody); err != nil {
				return result, fmt.Errorf("%w: gate 失败评论发布失败: %v", ErrGateFailed, err)
			}
		}
		// 两种模式都写 PR review（失败详情供 iterate 读取）
		if r.opts.AddPRReview != nil {
			reviewBody := fmt.Sprintf("## ❌ Gate 测试失败详情\n\n```\n%s\n```\n\n- retry_count=%d\n- max_retries=%d\n- attempt_key=`%s`", gateFailure, retryCount, r.opts.MaxRetries, attemptKey)
			if err := r.opts.AddPRReview(ctx, r.opts.Repo, r.opts.PR, reviewBody); err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: gate 失败信息写入 PR review 失败: %v\n", err)
			}
		}
		return result, fmt.Errorf("%w: %s", ErrGateFailed, gateFailure)
	}

	// 超限 escalation
	result.Escalated = true

	// 检查 needs-human 标签是否已存在，避免重复 escalation
	hasLabel, err := r.opts.HasLabel(ctx, r.opts.Issue, needsHumanLabel)
	if err != nil {
		return result, fmt.Errorf("%w: 检查 needs-human 标签失败: %v", ErrGateFailed, err)
	}

	if hasLabel {
		// 已 escalated，跳过重复操作
		result.EscalationCommentSkipped = true
		return result, fmt.Errorf("%w: %s", ErrGateFailed, gateFailure)
	}

	// gate 超限时只负责升级标记；恢复后的标签归一化由 control 层统一处理。
	if err := r.opts.AddLabels(ctx, r.opts.Repo, r.opts.Issue, []string{needsHumanLabel, integrationGateFailedLabel}); err != nil {
		return result, fmt.Errorf("%w: gate 超限后打标失败: %v", ErrGateFailed, err)
	}

	escalatedComment := fmt.Sprintf(defaultEscalatedCommentTemplate, retryCount, r.opts.MaxRetries, attemptKey)
	if err := r.opts.AddComment(ctx, r.opts.Repo, r.opts.Issue, escalatedComment); err != nil {
		return result, fmt.Errorf("%w: gate 超限升级评论发布失败: %v", ErrGateFailed, err)
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

func buildAttemptKey(issue, pr int, now time.Time, runID, runAttempt, headSHA string) string {
	if sanitizedHead := sanitizeHeadSHA(headSHA); sanitizedHead != "" {
		return fmt.Sprintf("issue-%d-pr-%d-head-%s", issue, pr, sanitizedHead)
	}
	if strings.TrimSpace(runID) != "" {
		attempt := strings.TrimSpace(runAttempt)
		if attempt == "" {
			attempt = "1"
		}
		return fmt.Sprintf("issue-%d-pr-%d-run-%s-attempt-%s", issue, pr, strings.TrimSpace(runID), attempt)
	}
	// 兼容本地/手工运行场景
	return fmt.Sprintf("issue-%d-pr-%d-local-%s", issue, pr, now.UTC().Format("20060102150405"))
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

func classifyFailureReason(reasonCode string) FailureClass {
	switch normalizeReasonCode(reasonCode) {
	case
		"CHECKS_QUERY_FAILED",
		"REQUIRED_JOBS_PENDING",
		"PENDING_RETRYING",
		"PENDING_BLOCKED",
		"REQUIRED_JOBS_MISSING",
		"CRITICAL_REGRESSION_MISSING",
		"INSUFFICIENT_COVERAGE_FOR_HIGH_RISK",
		"REQUIRED_JOBS_NOT_EXECUTED",
		"REQUIRED_JOBS_TIMEOUT",
		"TIMEOUT_RETRYING",
		"TIMEOUT_BLOCKED":
		return FailureClassDeferred
	default:
		return FailureClassCode
	}
}

func normalizeReasonCode(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func extractGateField(output, key string) string {
	wantPrefix := key + "="
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "[gate] ")
		if line == "" {
			continue
		}
		for _, token := range strings.Fields(line) {
			if strings.HasPrefix(token, wantPrefix) {
				return strings.TrimSpace(strings.TrimPrefix(token, wantPrefix))
			}
		}
	}
	return ""
}

func sanitizeHeadSHA(raw string) string {
	head := strings.ToLower(strings.TrimSpace(raw))
	if head == "" || head == "unknown" {
		return ""
	}
	return head
}
