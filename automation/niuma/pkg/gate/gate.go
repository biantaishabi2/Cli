package gate

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
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
type FindGateRetryCountFunc func(ctx context.Context, issue int) (int, error)
type UpsertGateRetryCountFunc func(ctx context.Context, issue int, count int) error
type HasLabelFunc func(ctx context.Context, issue int, label string) (bool, error)

type Options struct {
	Repo       string
	Issue      int
	PR         int
	RepoDir    string
	MaxRetries int

	Now                  func() time.Time
	RunGate              RunGateFunc
	MarkNeedsFix         MarkNeedsFixFunc
	AddLabels            AddLabelsFunc
	AddComment           AddCommentFunc
	FindGateRetryCount   FindGateRetryCountFunc
	UpsertGateRetryCount UpsertGateRetryCountFunc
	HasLabel             HasLabelFunc
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
	if opts.FindGateRetryCount == nil {
		return nil, fmt.Errorf("FindGateRetryCount 未配置")
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
		if err := r.opts.UpsertGateRetryCount(ctx, r.opts.Issue, 0); err != nil {
			return result, fmt.Errorf("gate 通过后重置 retry_count 失败: %w", err)
		}
		return result, nil
	}

	attemptKey := buildAttemptKey(r.opts.Issue, r.opts.PR, r.opts.Now())
	result.AttemptKey = attemptKey
	gateFailure := trimGateError(gateOutput, gateErr)

	// 从 marker 读取上次 retry_count
	prevCount, err := r.opts.FindGateRetryCount(ctx, r.opts.Issue)
	if err != nil {
		return result, fmt.Errorf("读取 gate retry_count 失败: %w", err)
	}
	retryCount := prevCount + 1
	result.RetryCount = retryCount

	// 写回新计数
	if err := r.opts.UpsertGateRetryCount(ctx, r.opts.Issue, retryCount); err != nil {
		return result, fmt.Errorf("写入 gate retry_count 失败: %w", err)
	}

	if retryCount <= r.opts.MaxRetries {
		// 未超限，标记 needs-fix
		if err := r.opts.MarkNeedsFix(ctx, r.opts.Repo, r.opts.Issue); err != nil {
			return result, fmt.Errorf("%w: gate 失败后设置 bot:pr-needs-fix 失败: %v", ErrGateFailed, err)
		}
		commentBody := fmt.Sprintf(defaultNeedsFixCommentTemplate, retryCount, r.opts.MaxRetries, attemptKey)
		if err := r.opts.AddComment(ctx, r.opts.Repo, r.opts.Issue, commentBody); err != nil {
			return result, fmt.Errorf("%w: gate 失败评论发布失败: %v", ErrGateFailed, err)
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
