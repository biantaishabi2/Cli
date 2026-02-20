// pkg/agent/orchestrator.go
// 核心编排器：协调 AI、GitHub、状态机完成自动化流程
// Phase 2.5：支持多 provider "左右互搏"、worktree 隔离、agentic 模式
package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/google/go-github/v68/github"
)

// OrchestratorConfig 编排器配置
type OrchestratorConfig struct {
	// 讨论阶段：多 provider 参与讨论
	DiscussionProviders  []ai.Provider // 参与"左右互搏"的 provider 列表
	VisibleRoundInterval int           // 可见评论每 N 轮输出（默认 1）
	VisibleOnlyOnDiff    bool          // 仅在分歧/决策变化时输出可见评论

	// 实现阶段：单 provider 执行
	ImplementProvider ai.Provider // 实现/迭代用的 provider

	// 仓库目录（用于 worktree 管理）
	RepoDir string

	// 工作流配置
	RequirePlanApproval bool // 方案定稿后是否需要人工审批
	MaxIterateRounds    int  // 最大自动迭代轮数（0=默认3）
	MaxDiscussionRounds int  // discuss 单次 run 内最大轮数（0=默认5）

	// plan-final 重试与降级
	PlanFinalProviders []ai.Provider // 降级 provider 列表（为空则使用默认 provider）
	PlanFinalMaxRetries int          // 每个 provider 的重试次数（0=默认1）

	// review 重试与降级
	ReviewProviders  []ai.Provider // review 降级 provider 列表（为空则使用 ImplementProvider）
	ReviewMaxRetries int           // review 每个 provider 的重试次数（0=默认1）

	// discuss 评论裁剪
	MaxHumanComments int // discuss 评论裁剪上限（0=默认10）
}

// getMaxIterateRounds 获取最大迭代轮数，默认3
func (c *OrchestratorConfig) getMaxIterateRounds() int {
	if c == nil || c.MaxIterateRounds <= 0 {
		return 3
	}
	return c.MaxIterateRounds
}

// getMaxDiscussionRounds 获取 discuss 最大轮数，默认 5
func (c *OrchestratorConfig) getMaxDiscussionRounds() int {
	if c == nil || c.MaxDiscussionRounds < 1 || c.MaxDiscussionRounds > 20 {
		return 5
	}
	return c.MaxDiscussionRounds
}

// getVisibleRoundInterval 获取可见评论轮次节流，默认 1。
func (c *OrchestratorConfig) getVisibleRoundInterval() int {
	if c == nil || c.VisibleRoundInterval <= 0 {
		return 1
	}
	return c.VisibleRoundInterval
}

// getVisibleOnlyOnDiff 获取可见评论是否仅差异变化时输出，默认 true。
func (c *OrchestratorConfig) getVisibleOnlyOnDiff() bool {
	if c == nil {
		return true
	}
	return c.VisibleOnlyOnDiff
}

// getMaxHumanComments 获取 discuss 评论裁剪上限，默认 10
func (c *OrchestratorConfig) getMaxHumanComments() int {
	if c == nil || c.MaxHumanComments <= 0 {
		return 10
	}
	return c.MaxHumanComments
}

// Orchestrator 核心编排器
type Orchestrator struct {
	github      GitHubOps
	provider    ai.Provider // 默认 provider（兼容旧接口）
	issueNumber int
	plan        *PlanEngine
	config      *OrchestratorConfig
}

// NewOrchestrator 创建编排器（兼容旧接口：单 provider）
func NewOrchestrator(ghOps GitHubOps, provider ai.Provider, issueNumber int) *Orchestrator {
	return &Orchestrator{
		github:      ghOps,
		provider:    provider,
		issueNumber: issueNumber,
		plan:        NewPlanEngine(provider),
	}
}

// NewOrchestratorWithConfig 创建支持多 provider 的编排器
// 至少需要 ImplementProvider 或 DiscussionProviders 中有一个 provider
func NewOrchestratorWithConfig(ghOps GitHubOps, issueNumber int, cfg *OrchestratorConfig) *Orchestrator {
	// 确定默认 provider：优先 ImplementProvider，否则用第一个讨论 provider
	defaultProvider := cfg.ImplementProvider
	if defaultProvider == nil && len(cfg.DiscussionProviders) > 0 {
		defaultProvider = cfg.DiscussionProviders[0]
	}
	if defaultProvider == nil {
		panic("OrchestratorConfig 必须提供至少一个 provider（ImplementProvider 或 DiscussionProviders）")
	}

	return &Orchestrator{
		github:      ghOps,
		provider:    defaultProvider,
		issueNumber: issueNumber,
		plan:        NewPlanEngine(defaultProvider),
		config:      cfg,
	}
}

// DoPlanDraft 生成方案草案
// 前置：bot:fix 状态 + 无 PLAN_DRAFT marker
// 后置：发评论 + 创建 marker → 转状态 bot:plan-draft → bot:needs-discussion
func (o *Orchestrator) DoPlanDraft(ctx context.Context) error {
	// 1. 检查前置状态
	currentState, err := o.currentState(ctx)
	if err != nil {
		return fmt.Errorf("读取状态失败: %w", err)
	}
	if currentState != state.StateFixRequested {
		return fmt.Errorf("当前状态 %s 不允许生成草案（需要 %s）", currentState, state.StateFixRequested)
	}

	// 2. 幂等检查：是否已有 PLAN_DRAFT marker
	existing, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypePlanDraft)
	if err != nil {
		return fmt.Errorf("检查 marker 失败: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("已存在方案草案 (rev=%d)，跳过", existing.Marker.Revision)
	}

	// 3. 读取 issue 和评论
	input, err := o.buildPromptInput(ctx)
	if err != nil {
		return err
	}

	// 4. AI 生成草案
	draft, err := o.plan.Draft(ctx, input)
	if err != nil {
		return err
	}

	// 5. 发评论 + 创建 marker
	m := &marker.Marker{
		Type:     marker.TypePlanDraft,
		Issue:    o.issueNumber,
		Revision: 1,
	}
	body := FormatDraftPlan(draft, m)
	if err := o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, body); err != nil {
		return fmt.Errorf("创建草案评论失败: %w", err)
	}

	// 6. 转状态：fix → plan-draft → needs-discussion
	if err := o.transition(ctx, state.StatePlanDraft); err != nil {
		return err
	}
	return o.transition(ctx, state.StateNeedsDiscussion)
}

// DoDiscuss 在单次 run 内循环推进讨论，直到收敛或达到轮次上限
func (o *Orchestrator) DoDiscuss(ctx context.Context, maxRounds int) error {
	if maxRounds <= 0 {
		if o.config != nil {
			maxRounds = o.config.getMaxDiscussionRounds()
		} else {
			maxRounds = 5
		}
	}

	loopResult, err := o.runLoopCore(ctx, "discuss", "讨论", maxRounds, o.runDiscussionRound)
	if err != nil {
		return err
	}

	if loopResult.Converged {
		if err := o.DoPlanFinal(ctx); err != nil {
			return fmt.Errorf("讨论已收敛但定稿失败: %w", err)
		}
		return nil
	}

	if err := o.notifyMaxDiscussionRoundsReached(ctx, maxRounds); err != nil {
		return fmt.Errorf("写入讨论轮次上限提醒失败: %w", err)
	}
	return nil
}

// DoDiscussionCheck 执行单轮讨论检查（兼容旧接口）
func (o *Orchestrator) DoDiscussionCheck(ctx context.Context) error {
	decision, err := o.runDiscussionRound(ctx, 1, 1)
	if err != nil {
		return err
	}

	if decision.Converged() {
		if err := o.DoPlanFinal(ctx); err != nil {
			return fmt.Errorf("讨论已收敛但定稿失败: %w", err)
		}
	}
	return nil
}

// runDiscussionRound 执行单轮讨论推进，返回结构化收敛判定
func (o *Orchestrator) runDiscussionRound(ctx context.Context, round, maxRounds int) (*state.ConvergenceDecision, error) {
	currentState, err := o.currentState(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取状态失败: %w", err)
	}
	if currentState != state.StateNeedsDiscussion {
		return nil, fmt.Errorf("当前状态 %s 不是讨论中（需要 %s）", currentState, state.StateNeedsDiscussion)
	}

	// 读取上一轮摘要，用于计算 diff 与修订号。
	summaryMC, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypeDiscussionSummary)
	if err != nil {
		return nil, fmt.Errorf("查找汇总 marker 失败: %w", err)
	}
	var previousFinish *bool
	if summaryMC != nil && summaryMC.Marker != nil {
		last := summaryMC.Marker.Finish
		previousFinish = &last
	}

	// 生成本轮讨论摘要（仅保留 debate_ab 模式）。
	summary, err := o.doDebateABDiscussion(ctx, round, maxRounds)
	if err != nil {
		return nil, err
	}

	// 读取最新上下文，进行收敛判定
	comments, err := o.github.ListComments(ctx, o.issueNumber)
	if err != nil {
		return nil, fmt.Errorf("获取评论失败: %w", err)
	}

	warningMC, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypeConvergeWarning)
	if err != nil {
		return nil, fmt.Errorf("查找预警 marker 失败: %w", err)
	}

	// 构建收敛检查输入
	checker := state.DefaultChecker()
	input := &state.ConvergenceInput{
		Comments:       comments,
		Round:          round,
		MaxRound:       maxRounds,
		AIShouldFinish: summary.ShouldFinish,
	}
	if warningMC != nil {
		input.ConvergeWarning = warningMC.Marker
		input.WarningTime = warningMC.Comment.GetCreatedAt().Time
	}

	decision := checker.CheckWithDecision(input)
	summary.ShouldFinish = decision.ShouldFinish

	// 每轮 upsert 机器可读汇总 marker。
	rev := 1
	if summaryMC != nil {
		rev = summaryMC.Marker.Revision + 1
	}
	m := buildDiscussionMarker(o.issueNumber, rev, summary)
	body := FormatDiscussionSummary(summary, m)
	if err := o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, body); err != nil {
		return nil, fmt.Errorf("更新讨论汇总失败: %w", err)
	}

	if o.shouldPublishDiscussionSummary(round, summaryMC, summary) {
		if _, err := o.github.AddComment(ctx, o.issueNumber,
			FormatDiscussionRoundSummary(round, maxRounds, "debate_ab", summary, previousFinish)); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] 发布讨论摘要失败: %v\n", err)
		}
	}

	if decision.Result == state.ShouldWarn {
		if err := o.upsertConvergeWarning(ctx, warningMC, FormatConvergeWarning); err != nil {
			return nil, err
		}
	}
	return decision, nil
}

func (o *Orchestrator) upsertConvergeWarning(ctx context.Context, existing *gh.MarkerComment, formatter func(*marker.Marker) string) error {
	rev := 1
	if existing != nil {
		rev = existing.Marker.Revision + 1
	}
	m := &marker.Marker{
		Type:     marker.TypeConvergeWarning,
		Issue:    o.issueNumber,
		Revision: rev,
	}
	body := formatter(m)
	return o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, body)
}

func (o *Orchestrator) upsertDiscussionRoundLimitNotice(ctx context.Context, existing *gh.MarkerComment, formatter func(*marker.Marker) string) error {
	rev := 1
	if existing != nil {
		rev = existing.Marker.Revision + 1
	}
	m := &marker.Marker{
		Type:     marker.TypeDiscussionRoundLimitNotice,
		Issue:    o.issueNumber,
		Revision: rev,
	}
	body := formatter(m)
	return o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, body)
}

func (o *Orchestrator) notifyMaxDiscussionRoundsReached(ctx context.Context, maxRounds int) error {
	noticeMC, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypeDiscussionRoundLimitNotice)
	if err != nil {
		return fmt.Errorf("读取预警 marker 失败: %w", err)
	}
	summaryMC, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypeDiscussionSummary)
	if err != nil {
		return fmt.Errorf("读取讨论汇总 marker 失败: %w", err)
	}
	reason, nextActions := buildRoundLimitReasonAndActions(summaryMC)
	return o.upsertDiscussionRoundLimitNotice(ctx, noticeMC, func(m *marker.Marker) string {
		return FormatDiscussionRoundLimitWarning(m, maxRounds, reason, nextActions)
	})
}

// DoPlanFinal 生成最终方案
func (o *Orchestrator) DoPlanFinal(ctx context.Context) error {
	currentState, err := o.currentState(ctx)
	if err != nil {
		return fmt.Errorf("读取状态失败: %w", err)
	}
	if currentState != state.StateNeedsDiscussion {
		return fmt.Errorf("当前状态 %s 不允许定稿（需要 %s）", currentState, state.StateNeedsDiscussion)
	}

	// 幂等检查
	existing, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypePlanFinal)
	if err != nil {
		return fmt.Errorf("检查 marker 失败: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("已存在最终方案 (rev=%d)，跳过", existing.Marker.Revision)
	}

	// 读取 issue 和评论
	input, err := o.buildPromptInput(ctx)
	if err != nil {
		return err
	}

	// AI 生成最终方案（带重试与 provider 降级）
	providers := []ai.Provider{o.provider}
	maxRetries := 1
	if o.config != nil {
		if len(o.config.PlanFinalProviders) > 0 {
			providers = o.config.PlanFinalProviders
		}
		if o.config.PlanFinalMaxRetries > 0 {
			maxRetries = o.config.PlanFinalMaxRetries
		}
	}

	finalPlan, err := o.plan.FinalWithRetry(ctx, input, providers, maxRetries)
	if err != nil {
		// 全失败时发布结构化错误评论
		o.postPlanFinalFailureComment(ctx, err)
		return err
	}

	// 降级时 title 为空，用 issue 编号兜底
	if finalPlan.Title == "" {
		finalPlan.Title = fmt.Sprintf("Issue #%d 定稿方案", o.issueNumber)
	}

	// 高风险路径警告（允许通过但提醒）
	var highRiskPaths []string
	for _, fc := range finalPlan.FileChanges {
		if IsHighRiskChange(fc.Path) {
			highRiskPaths = append(highRiskPaths, fc.Path)
		}
	}
	if len(highRiskPaths) > 0 {
		_, _ = o.github.AddComment(ctx, o.issueNumber, FormatHighRiskWarning(highRiskPaths))
	}

	// 发评论 + 创建 marker
	m := &marker.Marker{
		Type:     marker.TypePlanFinal,
		Issue:    o.issueNumber,
		Revision: 1,
	}
	body := FormatFinalPlan(finalPlan, m)
	if err := o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, body); err != nil {
		return fmt.Errorf("创建定稿评论失败: %w", err)
	}

	// 转状态
	return o.transition(ctx, state.StatePlanFinal)
}

// DoImplement 执行代码实现
// Phase 2.5：创建 worktree → AI agent 模式 Execute → commit → push → 创建 PR
// Phase 2.7：支持 plan-approved 审批门
func (o *Orchestrator) DoImplement(ctx context.Context, workDir string) error {
	currentState, err := o.currentState(ctx)
	if err != nil {
		return fmt.Errorf("读取状态失败: %w", err)
	}

	// 审批门：如果配置了 require_plan_approval 且当前是 plan-final，等人工审批
	if currentState == state.StatePlanFinal && o.config != nil && o.config.RequirePlanApproval {
		_, _ = o.github.AddComment(ctx, o.issueNumber,
			"## ⏸️ 等待人工审批\n\n最终方案已生成，请审阅后添加 `bot:plan-approved` 标签以继续实现。")
		return nil
	}

	// 允许从 plan-final（自动模式）或 plan-approved（审批模式）进入
	if currentState != state.StatePlanFinal && currentState != state.StatePlanApproved {
		return fmt.Errorf("当前状态 %s 不允许实现（需要 %s 或 %s）", currentState, state.StatePlanFinal, state.StatePlanApproved)
	}

	// 读取最终方案
	finalMC, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypePlanFinal)
	if err != nil {
		return fmt.Errorf("查找最终方案失败: %w", err)
	}
	if finalMC == nil {
		return fmt.Errorf("未找到最终方案")
	}

	input, err := o.buildPromptInput(ctx)
	if err != nil {
		return err
	}
	input.FinalPlan = marker.StripMarkerLines(finalMC.Comment.GetBody())

	// 记录进入前的状态，用于失败回滚
	prevState := currentState

	// 转状态到 implementing
	if err := o.transition(ctx, state.StateImplementing); err != nil {
		return err
	}

	// 解析最终方案中的 FileChanges（用于 implement 后 diff 对比）
	plannedChanges := ParseFileChangesFromComment(finalMC.Comment.GetBody())

	// 实现逻辑封装，失败时回滚状态
	prNumber, implErr := o.doImplementInner(ctx, input, workDir, plannedChanges)
	if implErr != nil {
		// 回滚状态并通知
		_ = o.transitionFrom(ctx, state.StateImplementing, prevState, false)
		_, _ = o.github.AddComment(ctx, o.issueNumber,
			fmt.Sprintf("## ❌ 实现失败\n\n%s\n\n状态已回滚到 `%s`。", implErr.Error(), prevState))
		return implErr
	}

	// 无变更时不创建 PR marker，提前返回
	if prNumber == 0 {
		_, _ = o.github.AddComment(ctx, o.issueNumber,
			"## ℹ️ 代码实现\n\nAI 执行完成，但 worktree 中无文件变更。状态已回滚。")
		_ = o.transitionFrom(ctx, state.StateImplementing, prevState, false)
		return nil
	}

	// 创建 PR marker
	m := &marker.Marker{
		Type:     marker.TypePRCreated,
		Issue:    o.issueNumber,
		Revision: 1,
		PR:       prNumber,
	}
	if err := o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, "PR 已创建"); err != nil {
		return fmt.Errorf("创建 PR marker 失败: %w", err)
	}

	// 转状态
	return o.transition(ctx, state.StatePRCreated)
}

// doImplementInner 执行实现的内部逻辑，返回 PR 号（0 表示无变更或无 worktree）
// 有 worktree 时：创建 worktree → AI 执行 → commit → push → 创建 PR
// 无 worktree 时：AI 在 workDir 中执行，返回 prNumber=0（调用方回滚状态）
func (o *Orchestrator) doImplementInner(ctx context.Context, input *PromptInput, workDir string, plannedChanges []FileChange) (int, error) {
	actualWorkDir := workDir
	var gitOps *GitOps
	var branchName string

	var cleanupWorktree func() // worktree 清理函数（失败时调用）
	if o.config != nil && o.config.RepoDir != "" {
		// 使用 worktree 隔离
		ws := NewWorkspace(o.config.RepoDir)
		slug := slugFromTitle(input.IssueTitle)
		wtPath, err := ws.Create(o.issueNumber, slug)
		if err != nil {
			return 0, fmt.Errorf("创建 worktree 失败: %w", err)
		}
		actualWorkDir = wtPath
		branchName = ws.BranchName(o.issueNumber, slug)
		gitOps = NewGitOps(wtPath)
		// 失败时清理 worktree（成功创建 PR 后置 nil 跳过清理，供 iterate 复用）
		cleanupWorktree = func() { _ = ws.Remove(o.issueNumber) }
	}

	// 失败时清理 worktree（defer 在 return 前执行）
	defer func() {
		if cleanupWorktree != nil {
			cleanupWorktree()
		}
	}()

	// AI agent 模式执行
	implProvider := o.getImplementProvider()
	result, err := implProvider.Execute(ctx, input.FinalPlan+"\n\n"+BuildImplementContext(input), ai.WithWorkDir(actualWorkDir))
	if err != nil {
		return 0, fmt.Errorf("AI 代码实现失败: %w", err)
	}

	// 如果有 worktree，执行 git 操作
	prNumber := 0
	if gitOps != nil {
		hasChanges, err := gitOps.HasChanges()
		if err != nil {
			return 0, fmt.Errorf("检查变更失败: %w", err)
		}

		if hasChanges {
			commitMsg := fmt.Sprintf("feat: implement fix for #%d\n\n%s", o.issueNumber, input.IssueTitle)
			if err := gitOps.CommitAll(commitMsg); err != nil {
				return 0, fmt.Errorf("提交失败: %w", err)
			}

			if err := gitOps.Push(branchName); err != nil {
				return 0, fmt.Errorf("推送失败: %w", err)
			}

			prTitle := fmt.Sprintf("fix: %s (#%d)", input.IssueTitle, o.issueNumber)
			prBody := fmt.Sprintf("Closes #%d\n\n## Summary\n\n%s", o.issueNumber, input.FinalPlan)
			pr, err := o.github.CreatePR(ctx, prTitle, prBody, branchName, "master")
			if err != nil {
				return 0, fmt.Errorf("创建 PR 失败: %w", err)
			}
			prNumber = pr.GetNumber()
			// 成功创建 PR，保留 worktree 供 iterate 复用
			cleanupWorktree = nil

			// diff 对比：计划声明 vs 实际改动
			if len(plannedChanges) > 0 {
				baseBranch := gitOps.DefaultBranch()
				changedFiles, err := gitOps.ChangedFiles(baseBranch)
				if err != nil {
					_, _ = o.github.AddComment(ctx, prNumber,
						fmt.Sprintf("## ⚠️ diff 对比失败\n\n无法获取改动文件列表: %v", err))
				} else {
					diffResult := CheckDiffAgainstPlan(changedFiles, plannedChanges)
					if !diffResult.IsClean() {
						comment := FormatDiffCheckComment(diffResult)
						_, _ = o.github.AddComment(ctx, prNumber, comment)
					}
				}
			}
		}
	}

	// 发布实现结果评论
	commentBody := fmt.Sprintf("## 🔧 代码实现\n\nAI 已完成代码生成。\n\n<details>\n<summary>实现详情</summary>\n\n%s\n\n</details>", result)
	if prNumber > 0 {
		commentBody += fmt.Sprintf("\n\n✅ PR #%d 已创建。", prNumber)
	}
	_, err = o.github.AddComment(ctx, o.issueNumber, commentBody)
	if err != nil {
		return 0, fmt.Errorf("发布实现结果失败: %w", err)
	}

	return prNumber, nil
}

// DoIterate 根据 review 意见迭代修改
// Phase 2.5：复用 worktree → AI agent 模式 Execute → commit → push
// Phase 2.7：迭代次数保护
func (o *Orchestrator) DoIterate(ctx context.Context, prNumber int, workDir string) error {
	currentState, err := o.currentState(ctx)
	if err != nil {
		return fmt.Errorf("读取状态失败: %w", err)
	}
	if currentState != state.StatePRNeedsFix {
		return fmt.Errorf("当前状态 %s 不允许迭代（需要 %s）", currentState, state.StatePRNeedsFix)
	}

	// 迭代次数保护
	maxRounds := 3
	if o.config != nil {
		maxRounds = o.config.getMaxIterateRounds()
	}
	prMC, _ := o.github.FindMarker(ctx, o.issueNumber, marker.TypePRCreated)
	if prMC != nil && prMC.Marker.Revision >= maxRounds {
		_, _ = o.github.AddComment(ctx, o.issueNumber,
			fmt.Sprintf("## ⚠️ 迭代次数已达上限\n\n已自动迭代 %d 轮，仍未通过审查。请人工介入处理。", prMC.Marker.Revision))
		return nil
	}

	// 读取 PR review 意见
	reviews, err := o.github.ListPRReviews(ctx, prNumber)
	if err != nil {
		return fmt.Errorf("获取 PR reviews 失败: %w", err)
	}

	// 汇总 review 意见
	reviewText := summarizeReviews(reviews)

	// 读取最终方案
	finalMC, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypePlanFinal)
	if err != nil {
		return fmt.Errorf("查找最终方案失败: %w", err)
	}
	if finalMC == nil {
		return fmt.Errorf("未找到最终方案")
	}

	input, err := o.buildPromptInput(ctx)
	if err != nil {
		return err
	}
	input.FinalPlan = marker.StripMarkerLines(finalMC.Comment.GetBody())
	input.ReviewComment = reviewText

	// 转状态到 iterating
	prevState := currentState
	if err := o.transition(ctx, state.StateIterating); err != nil {
		return err
	}

	// 执行迭代逻辑，失败时回滚状态
	iterateErr := o.doIterateInner(ctx, input, prNumber, workDir)
	if iterateErr != nil {
		_ = o.transitionFrom(ctx, state.StateIterating, prevState, false)
		_, _ = o.github.AddComment(ctx, o.issueNumber,
			fmt.Sprintf("## ❌ 迭代失败\n\n%s\n\n状态已回滚到 `%s`。", iterateErr.Error(), prevState))
		return iterateErr
	}

	// 更新 PR marker revision
	prMC, _ = o.github.FindMarker(ctx, o.issueNumber, marker.TypePRCreated)
	rev := 1
	if prMC != nil {
		rev = prMC.Marker.Revision + 1
	}
	m := &marker.Marker{
		Type:     marker.TypePRCreated,
		Issue:    o.issueNumber,
		Revision: rev,
		PR:       prNumber,
	}
	if err := o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, "PR 已更新"); err != nil {
		return fmt.Errorf("更新 PR marker 失败: %w", err)
	}

	// 转状态回 pr-created
	return o.transition(ctx, state.StatePRCreated)
}

// doIterateInner 执行迭代的内部逻辑
// 有 worktree 时：复用/重建 worktree → AI 修改 → commit → push
// 无 worktree 时：AI 修改 workDir 中的文件，不做 git 操作（用户需手动 push）
func (o *Orchestrator) doIterateInner(ctx context.Context, input *PromptInput, prNumber int, workDir string) error {
	actualWorkDir := workDir
	var gitOps *GitOps
	var branchName string

	if o.config != nil && o.config.RepoDir != "" {
		ws := NewWorkspace(o.config.RepoDir)
		if ws.Exists(o.issueNumber) {
			// 复用已有 worktree
			actualWorkDir = ws.Path(o.issueNumber)
			gitOps = NewGitOps(actualWorkDir)
			branchName, _ = gitOps.CurrentBranch()
		} else {
			// worktree 不存在（CI runner 重启等），checkout 已有远程分支重建
			slug := slugFromTitle(input.IssueTitle)
			branch := ws.BranchName(o.issueNumber, slug)
			wtPath, err := ws.Checkout(o.issueNumber, branch)
			if err != nil {
				return fmt.Errorf("重建 worktree 失败: %w", err)
			}
			actualWorkDir = wtPath
			branchName = branch
			gitOps = NewGitOps(actualWorkDir)
		}
	}

	// AI agent 模式修复
	implProvider := o.getImplementProvider()
	iteratePrompt, err := BuildIteratePrompt(input)
	if err != nil {
		return fmt.Errorf("构建 iterate prompt 失败: %w", err)
	}

	result, err := implProvider.Execute(ctx, iteratePrompt, ai.WithWorkDir(actualWorkDir))
	if err != nil {
		return fmt.Errorf("AI 迭代修复失败: %w", err)
	}

	// 如果有 worktree，执行 git 操作
	if gitOps != nil {
		hasChanges, err := gitOps.HasChanges()
		if err != nil {
			return fmt.Errorf("检查变更失败: %w", err)
		}

		if hasChanges {
			commitMsg := fmt.Sprintf("fix: iterate on review for #%d", o.issueNumber)
			if err := gitOps.CommitAll(commitMsg); err != nil {
				return fmt.Errorf("提交失败: %w", err)
			}

			if branchName != "" {
				if err := gitOps.Push(branchName); err != nil {
					return fmt.Errorf("推送失败: %w", err)
				}
			}
		}
	}

	// 在 PR 上发回复评论（reviewer 在 PR 上看到回复，形成对话）
	_, err = o.github.AddComment(ctx, prNumber,
		fmt.Sprintf("## 🔄 迭代修复\n\n根据 review 意见进行了修改。\n\n<details>\n<summary>修复详情</summary>\n\n%s\n\n</details>", result))
	if err != nil {
		return fmt.Errorf("发布修复结果失败: %w", err)
	}

	return nil
}

// DoReview AI 自审 PR
// 前置：bot:pr-created 状态
// 后置：通过 → bot:pr-reviewable；不通过 → bot:pr-needs-fix
// 使用 WithRecovery 实现格式修复重试 + provider 降级 + 中止上报。
func (o *Orchestrator) DoReview(ctx context.Context, prNumber int) error {
	currentState, err := o.currentState(ctx)
	if err != nil {
		return fmt.Errorf("读取状态失败: %w", err)
	}
	if currentState != state.StatePRCreated {
		return fmt.Errorf("当前状态 %s 不允许审查（需要 %s）", currentState, state.StatePRCreated)
	}

	// 读取 PR diff
	diff, err := o.github.GetPRDiff(ctx, prNumber)
	if err != nil {
		return fmt.Errorf("获取 PR diff 失败: %w", err)
	}

	// 读取最终方案
	finalMC, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypePlanFinal)
	if err != nil {
		return fmt.Errorf("查找最终方案失败: %w", err)
	}
	if finalMC == nil {
		return fmt.Errorf("未找到最终方案")
	}

	input, err := o.buildPromptInput(ctx)
	if err != nil {
		return err
	}
	input.FinalPlan = marker.StripMarkerLines(finalMC.Comment.GetBody())
	input.PRDiff = diff

	// 读取 PR 完整历史（reviews + comments），让 reviewer 看到之前的讨论
	prHistory, err := o.buildPRHistory(ctx, prNumber)
	if err != nil {
		// 非致命，记录但继续
		prHistory = fmt.Sprintf("(读取 PR 历史失败: %v)", err)
	}
	if prHistory != "" {
		input.ReviewComment = prHistory
	}

	// 构建 review prompt
	reviewPrompt, err := BuildReviewPrompt(input)
	if err != nil {
		return fmt.Errorf("构建 review prompt 失败: %w", err)
	}

	// 构建 review provider 列表
	reviewProviders := []ai.Provider{o.getImplementProvider()}
	reviewMaxRetries := 1
	if o.config != nil {
		if len(o.config.ReviewProviders) > 0 {
			reviewProviders = o.config.ReviewProviders
		}
		if o.config.ReviewMaxRetries > 0 {
			reviewMaxRetries = o.config.ReviewMaxRetries
		}
	}

	// 使用 WithRecovery 统一降级链路
	cfg := RecoveryConfig[*ReviewResult]{
		Providers:  reviewProviders,
		MaxRetries: reviewMaxRetries,
		CallAI: func(ctx context.Context, provider ai.Provider) (string, error) {
			return provider.Complete(ctx, reviewPrompt)
		},
		Parse:       ParseReviewResponse,
		BuildRepair: func(raw string) string { return BuildFormatRepairPromptFor(raw, reviewFields) },
		RepairParse: RepairAndParseReview,
		OnAbort: func(abortErr error, lastRaw string) {
			rawPrefix := truncatePrefix(lastRaw, 500)
			abortBody := fmt.Sprintf("<!-- BOT:REVIEW_ABORT issue=%d pr=%d -->\n\n## ⛔ 审查中止\n\n"+
				"AI 审查响应解析连续失败，已中止本次审查。\n\n"+
				"**错误**: %s\n\n**raw_prefix**: %.500s\n\n请人工检查后重新触发审查。",
				o.issueNumber, prNumber, abortErr.Error(), rawPrefix)
			_, _ = o.github.AddComment(ctx, o.issueNumber, abortBody)
		},
	}

	result, err := WithRecovery(ctx, cfg)
	if err != nil {
		return fmt.Errorf("AI 审查失败（3 级降级均失败）: %w", err)
	}

	// 发 PR review
	// 注意：GitHub 不允许对自己的 PR 提 REQUEST_CHANGES，也可能不允许 APPROVE。
	// 策略：通过时尝试 APPROVE，失败（403 权限/422 规则限制）则回退到 COMMENT。
	// 不通过时直接用 COMMENT（避免 REQUEST_CHANGES 权限问题）。
	// 如果 APPROVE 因网络错误失败，COMMENT 也会失败，此时返回两个错误供排查。
	reviewBody := FormatReviewResult(result)
	event := "COMMENT"
	if result.Approved {
		event = "APPROVE"
	}
	_, reviewErr := o.github.CreatePRReview(ctx, prNumber, reviewBody, event)
	if reviewErr != nil && result.Approved {
		_, fallbackErr := o.github.CreatePRReview(ctx, prNumber, reviewBody, "COMMENT")
		if fallbackErr != nil {
			return fmt.Errorf("发布审查结果失败（APPROVE: %v, COMMENT: %v）", reviewErr, fallbackErr)
		}
		reviewErr = nil
	}
	if reviewErr != nil {
		return fmt.Errorf("发布审查结果失败: %w", reviewErr)
	}

	// 根据审查结果转状态
	if result.Approved {
		return o.transition(ctx, state.StatePRReviewable)
	}
	return o.transition(ctx, state.StatePRNeedsFix)
}

// postPlanFinalFailureComment 全失败时在 issue 上发布结构化错误评论
func (o *Orchestrator) postPlanFinalFailureComment(ctx context.Context, err error) {
	var sb strings.Builder
	sb.WriteString("## ❌ 定稿失败：所有 provider 均未返回有效 JSON\n\n")

	var aggErr *AggregateRetryError
	if errors.As(err, &aggErr) {
		sb.WriteString("| Provider | Attempt | 错误分类 | 错误信息 |\n")
		sb.WriteString("|----------|---------|----------|----------|\n")
		for _, a := range aggErr.Attempts {
			sb.WriteString(fmt.Sprintf("| %s | %d | %s | %s |\n", a.Provider, a.Attempt, a.Kind, a.Error))
		}
	} else {
		sb.WriteString(fmt.Sprintf("错误: %s\n", err.Error()))
	}

	sb.WriteString("\n### 建议操作\n\n")
	sb.WriteString("- 检查 provider 配置和网络连接\n")
	sb.WriteString("- 手动恢复（CAS）: `niuma state-label set --repo <owner/repo> --issue ")
	sb.WriteString(fmt.Sprintf("%d", o.issueNumber))
	sb.WriteString(" --from <bot:current> --to bot:plan-final`\n")

	_, _ = o.github.AddComment(ctx, o.issueNumber, sb.String())
}

// ===== 内部方法 =====

// currentState 读取当前状态
func (o *Orchestrator) currentState(ctx context.Context) (state.State, error) {
	labels, err := o.github.ListLabels(ctx, o.issueNumber)
	if err != nil {
		return "", err
	}

	current, found, err := state.CurrentBotState(labels)
	if err != nil {
		if errors.Is(err, state.ErrInvariantViolation) || errors.Is(err, state.ErrMultipleBotStates) {
			priority, perr := state.ParseStatePriority(os.Getenv("NIUMA_STATE_PRIORITY"))
			if perr != nil {
				priority = append([]state.State(nil), state.DefaultStatePriority...)
			}
			target, changed, nerr := state.Normalize(ctx, o.github, o.issueNumber, priority)
			if nerr != nil {
				_, _ = o.github.AddComment(ctx, o.issueNumber,
					"## ⚠️ 状态自愈失败\n\n检测到多个 `bot:*` 状态标签，自动收敛失败，请人工处理后重试。")
				return "", nerr
				}
				if changed {
					marker := fmt.Sprintf("<!-- BOT:STATE_CONVERGED issue=%d target=%s -->", o.issueNumber, target)
					comments, _ := o.github.ListComments(ctx, o.issueNumber)
					duplicated := false
				for _, c := range comments {
					if strings.Contains(c.GetBody(), marker) {
						duplicated = true
						break
					}
				}
				if !duplicated {
					_, _ = o.github.AddComment(ctx, o.issueNumber,
						fmt.Sprintf("## ⚠️ 状态自愈\n\n检测到多个 `bot:*` 状态标签，已自动收敛为 `%s` 并继续流程。\n\n%s", target, marker))
				}
			}
			return target, nil
		}
		return "", err
	}
	if !found {
		return "", fmt.Errorf("issue #%d 没有 bot: 状态 label", o.issueNumber)
	}
	return current, nil
}

// transition 执行状态转换
func (o *Orchestrator) transition(ctx context.Context, to state.State) error {
	current, err := o.currentState(ctx)
	if err != nil {
		return fmt.Errorf("读取状态失败: %w", err)
	}

	return o.transitionFrom(ctx, current, to)
}

func (o *Orchestrator) transitionFrom(ctx context.Context, from, to state.State, strict ...bool) error {
	enforce := true
	if len(strict) > 0 {
		enforce = strict[0]
	}
	if !enforce {
		return state.TransitionWithRetry(ctx, o.github, o.issueNumber, "", to, nil)
	}
	return state.TransitionWithRetry(ctx, o.github, o.issueNumber, from, to, nil)
}

// buildPromptInput 从 GitHub 读取 issue 和评论构建 PromptInput（全量模式）
func (o *Orchestrator) buildPromptInput(ctx context.Context) (*PromptInput, error) {
	return o.buildPromptInputWithFilter(ctx, 0)
}

// buildPromptInputWithFilter 从 GitHub 读取 issue 和评论构建 PromptInput。
// maxHuman <= 0 全量返回；maxHuman > 0 裁剪评论（用于 debate 阶段上下文瘦身）。
func (o *Orchestrator) buildPromptInputWithFilter(ctx context.Context, maxHuman int) (*PromptInput, error) {
	issue, err := o.github.GetIssue(ctx, o.issueNumber)
	if err != nil {
		return nil, fmt.Errorf("获取 issue 失败: %w", err)
	}

	comments, err := o.github.ListComments(ctx, o.issueNumber)
	if err != nil {
		return nil, fmt.Errorf("获取评论失败: %w", err)
	}

	return &PromptInput{
		IssueTitle: issue.GetTitle(),
		IssueBody:  issue.GetBody(),
		Comments:   FilterCommentsForPrompt(comments, maxHuman),
	}, nil
}

// doDebateABDiscussion 执行 AB 轮流评论模式。
// 使用 WithRecovery 统一 3 级降级：格式修复重试 → fallback provider → 中止上报。
func (o *Orchestrator) doDebateABDiscussion(ctx context.Context, round, maxRounds int) (*DiscussionSummary, error) {
	providers := o.discussionProviders()
	if len(providers) == 0 {
		return nil, fmt.Errorf("debate_ab 缺少 discussion providers")
	}

	speakerIdx := (round - 1) % 2
	speaker := "A"
	if speakerIdx == 1 {
		speaker = "B"
	}

	// debate 路径使用评论裁剪
	maxHuman := 10
	if o.config != nil {
		maxHuman = o.config.getMaxHumanComments()
	}
	input, err := o.buildPromptInputWithFilter(ctx, maxHuman)
	if err != nil {
		return nil, err
	}
	input.Round = round
	input.MaxRounds = maxRounds
	input.DiscussionRole = speaker
	input.Counterpart = map[string]string{"A": "B", "B": "A"}[speaker]

	prompt, err := BuildDebatePrompt(input)
	if err != nil {
		return nil, fmt.Errorf("构建 debate_ab prompt 失败: %w", err)
	}

	// 构建 provider 列表：主 provider 在前，备选在后（若不同）
	primaryProvider := providers[speakerIdx%len(providers)]
	debateProviders := []ai.Provider{primaryProvider}
	if len(providers) > 1 {
		fallbackIdx := (speakerIdx + 1) % len(providers)
		if fallbackIdx != speakerIdx%len(providers) {
			debateProviders = append(debateProviders, providers[fallbackIdx])
		}
	}

	cfg := RecoveryConfig[*DebateComment]{
		Providers:  debateProviders,
		MaxRetries: 0, // debate 每个 provider 只尝试一次（格式修复算额外）
		CallAI: func(ctx context.Context, provider ai.Provider) (string, error) {
			raw, err := provider.Complete(ctx, prompt)
			if err != nil {
				return "", fmt.Errorf("debate_%s 生成评论失败 (provider=%s): %w", strings.ToLower(speaker), provider.Name(), err)
			}
			return raw, nil
		},
		Parse:       ParseDebateResponse,
		BuildRepair: func(raw string) string { return BuildFormatRepairPromptFor(raw, debateFields) },
		RepairParse: RepairAndParseDebate,
		OnAbort: func(err error, lastRaw string) {
			rawPrefix := truncatePrefix(lastRaw, 500)
			lastProviderName := debateProviders[len(debateProviders)-1].Name()
			abortBody := fmt.Sprintf("<!-- BOT:DISCUSS_ABORT issue=%d round=%d -->\n\n## ⛔ 讨论中止\n\n"+
				"第 %d/%d 轮 debate_%s 解析连续失败，已中止本次讨论。\n\n"+
				"**错误**: %s\n\n**provider**: %s\n\n**raw_prefix**: %.500s\n\n请人工检查后重新触发讨论。",
				o.issueNumber, round, round, maxRounds, strings.ToLower(speaker),
				err.Error(), lastProviderName, rawPrefix)
			_, _ = o.github.AddComment(ctx, o.issueNumber, abortBody)
		},
	}

	comment, err := WithRecovery(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("debate_%s 3 级降级均失败: %w", strings.ToLower(speaker), err)
	}

	// debate_ab 下每轮直接对 issue 可见。
	if _, err := o.github.AddComment(ctx, o.issueNumber, FormatDebateRoundComment(round, maxRounds, speaker, comment)); err != nil {
		return nil, fmt.Errorf("写入 debate_%s 可见评论失败: %w", strings.ToLower(speaker), err)
	}

	return &DiscussionSummary{
		Conclusion:   comment.Body,
		ShouldFinish: comment.ShouldFinish,
	}, nil
}

func (o *Orchestrator) discussionProviders() []ai.Provider {
	if o.config != nil && len(o.config.DiscussionProviders) > 0 {
		return o.config.DiscussionProviders
	}
	if o.provider == nil {
		return nil
	}
	return []ai.Provider{o.provider}
}

func (o *Orchestrator) shouldPublishDiscussionSummary(round int, previous *gh.MarkerComment, summary *DiscussionSummary) bool {
	if o.config == nil {
		return false
	}
	intervalHit := true
	if interval := o.getVisibleRoundInterval(); interval > 1 {
		intervalHit = round%interval == 0
	}

	if !o.getVisibleOnlyOnDiff() {
		return intervalHit
	}
	if previous == nil {
		return true
	}

	prev := previous.Marker
	changed := prev.Finish != summary.ShouldFinish
	return changed
}

func (o *Orchestrator) getVisibleRoundInterval() int {
	if o.config == nil {
		return 1
	}
	return o.config.getVisibleRoundInterval()
}

func (o *Orchestrator) getVisibleOnlyOnDiff() bool {
	if o.config == nil {
		return true
	}
	return o.config.getVisibleOnlyOnDiff()
}

func buildDiscussionMarker(issue, rev int, summary *DiscussionSummary) *marker.Marker {
	return &marker.Marker{
		Type:     marker.TypeDiscussionSummary,
		Issue:    issue,
		Revision: rev,
		Finish:   summary.ShouldFinish,
		Mode:     "debate_ab",
	}
}

func buildRoundLimitReasonAndActions(summaryMC *gh.MarkerComment) (string, []string) {
	if summaryMC == nil || summaryMC.Marker == nil {
		return "未读取到最新讨论摘要，暂无法自动归纳分歧结论", []string{
			"请补充一条总结评论，明确当前分歧点和推荐方案",
			"请维护者拍板最终取舍后评论 `/finalize` 或补充新约束继续讨论",
		}
	}

	m := summaryMC.Marker
	reason := "达到轮次上限后仍未满足自动定稿条件"
	if m != nil && m.Finish {
		reason = "已出现 should_finish=true，但仍未进入定稿，请人工确认是否继续推进"
	}

	nextActions := []string{
		"请维护者基于最新讨论内容明确最终取舍并补充一条结论评论",
		"补充结论后新增一条非 BOT 评论触发下一轮 discuss，或评论 `/finalize` 直接定稿",
	}
	return reason, nextActions
}

// buildPRHistory 读取 PR 上的全部 reviews 和 comments，构建完整历史上下文
// GitHub 中 PR 也是 issue，ListComments(prNumber) 能读到 PR 上的普通评论
func (o *Orchestrator) buildPRHistory(ctx context.Context, prNumber int) (string, error) {
	reviews, err := o.github.ListPRReviews(ctx, prNumber)
	if err != nil {
		return "", fmt.Errorf("获取 PR reviews 失败: %w", err)
	}

	comments, err := o.github.ListComments(ctx, prNumber)
	if err != nil {
		return "", fmt.Errorf("获取 PR comments 失败: %w", err)
	}

	var parts []string
	for _, r := range reviews {
		body := r.GetBody()
		if body != "" {
			parts = append(parts, fmt.Sprintf("[Review - %s]\n%s", r.GetState(), body))
		}
	}
	for _, c := range comments {
		body := c.GetBody()
		if body != "" {
			parts = append(parts, fmt.Sprintf("[PR Comment]\n%s", body))
		}
	}

	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, "\n\n---\n\n"), nil
}

// getImplementProvider 获取实现用的 provider
func (o *Orchestrator) getImplementProvider() ai.Provider {
	if o.config != nil && o.config.ImplementProvider != nil {
		return o.config.ImplementProvider
	}
	return o.provider
}

// summarizeReviews 汇总 PR review 意见
func summarizeReviews(reviews []*github.PullRequestReview) string {
	var parts []string
	for _, r := range reviews {
		body := r.GetBody()
		if body != "" {
			parts = append(parts, fmt.Sprintf("[%s] %s", r.GetState(), body))
		}
	}
	if len(parts) == 0 {
		return "(无 review 意见)"
	}
	return fmt.Sprintf("共 %d 条 review:\n\n- %s", len(parts), strings.Join(parts, "\n- "))
}

// slugFromTitle 从 issue 标题生成 slug
func slugFromTitle(title string) string {
	// 简单实现：取前 30 个字符，替换空格为 -，去掉特殊字符
	slug := strings.ToLower(title)
	slug = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, slug)
	// 合并连续 -
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if len(slug) > 30 {
		slug = slug[:30]
	}
	return slug
}

// recoverableKind 从 error 中提取 RecoverableErrorKind，未匹配时返回 "Unknown"
func recoverableKind(err error) string {
	var recErr *RecoverableError
	if errors.As(err, &recErr) {
		return string(recErr.Kind)
	}
	return "Unknown"
}

// truncatePrefix 截取字符串前 maxLen 个字符，超长时追加 "..."
func truncatePrefix(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// BuildImplementContext 从 PromptInput 构建实现上下文
func BuildImplementContext(input *PromptInput) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Issue: %s\n\n", input.IssueTitle))
	sb.WriteString(input.IssueBody)
	return sb.String()
}
