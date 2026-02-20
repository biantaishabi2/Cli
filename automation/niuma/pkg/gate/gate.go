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

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
)

const (
	maxGateErrorMessageLen          = 2000
	needsHumanLabel                 = "needs-human"
	integrationGateFailedLabel      = "integration-gate-failed"
	defaultGateScriptRelativePath   = ".github/scripts/niuma-test-gate.sh"
	defaultNeedsFixCommentTemplate  = "## ❌ Gate 未通过\n\n自动测试门禁失败，已回退为 `bot:pr-needs-fix`，请继续迭代修复。\n\n- retry_count=%d\n- max_retries=%d\n- attempt_key=`%s`"
	defaultEscalatedCommentTemplate = "## 🚨 Gate 已超限，升级人工处理\n\nintegration gate 失败次数已超过上限，本轮不再触发自动修复。\n\n- retry_count=%d\n- max_retries=%d\n- attempt_key=`%s`"
	gateFailurePRCommentTemplate    = "## ❌ Gate 测试失败详情\n\n以下是 gate 门禁的测试输出（retry_count=%d/%d）：\n\n```\n%s\n```"
)

var ErrGateFailed = errors.New("gate failed")

type RunGateFunc func(ctx context.Context, repoDir string, pr int) (string, error)
type MarkNeedsFixFunc func(ctx context.Context, repo string, issue int) error
type AddLabelsFunc func(ctx context.Context, repo string, issue int, labels []string) error
type AddCommentFunc func(ctx context.Context, repo string, issue int, body string) error

// FindMarkerFunc 从 issue 评论中查找指定类型的最新 marker
type FindMarkerFunc func(ctx context.Context, issue int, t marker.Type) (revision int, found bool, err error)

// UpdateMarkerFunc 创建或更新 marker（revision 用于计数）
type UpdateMarkerFunc func(ctx context.Context, issue int, m *marker.Marker, body string) error

// AddPRCommentFunc 往 PR 上发评论（iterate 通过 ListPRReviews 可读到）
type AddPRCommentFunc func(ctx context.Context, repo string, pr int, body string) error

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

	// 新增：marker 持久化 + PR 失败信息
	FindMarker   FindMarkerFunc
	UpdateMarker UpdateMarkerFunc
	AddPRComment AddPRCommentFunc
}

type Runner struct {
	opts Options
}

type Result struct {
	Passed     bool
	RetryCount int
	MaxRetries int
	AttemptKey string
	Escalated  bool
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
	if opts.FindMarker == nil {
		return nil, fmt.Errorf("FindMarker 未配置")
	}
	if opts.UpdateMarker == nil {
		return nil, fmt.Errorf("UpdateMarker 未配置")
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
	result.AttemptKey = attemptKey
	gateFailure := trimGateError(gateOutput, gateErr)

	// 从 marker 读取上次 retry_count
	retryCount := 1
	prevRev, found, findErr := r.opts.FindMarker(ctx, r.opts.Issue, marker.TypeGateRetry)
	if findErr != nil {
		// marker 读取失败不阻塞流程，降级为 retryCount=1
		fmt.Printf("WARNING: FindMarker 失败: %v，降级 retryCount=1\n", findErr)
	} else if found {
		retryCount = prevRev + 1
	}

	result.RetryCount = retryCount
	if retryCount > r.opts.MaxRetries {
		result.Escalated = true
	}

	// 持久化 retry_count 到 marker
	m := &marker.Marker{
		Type:     marker.TypeGateRetry,
		Issue:    r.opts.Issue,
		Revision: retryCount,
		PR:       r.opts.PR,
	}
	markerBody := fmt.Sprintf("gate retry_count=%d, attempt_key=`%s`", retryCount, attemptKey)
	if updateErr := r.opts.UpdateMarker(ctx, r.opts.Issue, m, markerBody); updateErr != nil {
		fmt.Printf("WARNING: UpdateMarker 失败: %v\n", updateErr)
	}

	// 往 PR 上发测试失败详情（iterate 通过 ListPRReviews 可读到）
	if r.opts.AddPRComment != nil {
		prComment := fmt.Sprintf(gateFailurePRCommentTemplate, retryCount, r.opts.MaxRetries, gateFailure)
		if prErr := r.opts.AddPRComment(ctx, r.opts.Repo, r.opts.PR, prComment); prErr != nil {
			fmt.Printf("WARNING: AddPRComment 失败: %v\n", prErr)
		}
	}

	if !result.Escalated {
		if err := r.opts.MarkNeedsFix(ctx, r.opts.Repo, r.opts.Issue); err != nil {
			return result, fmt.Errorf("%w: gate 失败后设置 bot:pr-needs-fix 失败: %v", ErrGateFailed, err)
		}
		commentBody := fmt.Sprintf(defaultNeedsFixCommentTemplate, retryCount, r.opts.MaxRetries, attemptKey)
		if err := r.opts.AddComment(ctx, r.opts.Repo, r.opts.Issue, commentBody); err != nil {
			return result, fmt.Errorf("%w: gate 失败评论发布失败: %v", ErrGateFailed, err)
		}
		return result, fmt.Errorf("%w: %s", ErrGateFailed, gateFailure)
	}

	// escalated
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
