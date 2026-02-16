// pkg/agent/orchestrator.go
// 核心编排器：协调 AI、GitHub、状态机完成自动化流程
// Phase 2.5：支持多 provider "左右互搏"、worktree 隔离、agentic 模式
package agent

import (
	"context"
	"fmt"
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
	DiscussionProviders []ai.Provider // 参与"左右互搏"的 provider 列表
	Consolidator        ai.Provider   // 汇总讨论用的 provider

	// 实现阶段：单 provider 执行
	ImplementProvider ai.Provider // 实现/迭代用的 provider

	// 仓库目录（用于 worktree 管理）
	RepoDir string

	// 工作流配置
	RequirePlanApproval bool     // 方案定稿后是否需要人工审批
	MaxIterateRounds    int      // 最大自动迭代轮数（0=默认3）
	AllowedPrefixes     []string // 额外允许修改的目录前缀
}

// getMaxIterateRounds 获取最大迭代轮数，默认3
func (c *OrchestratorConfig) getMaxIterateRounds() int {
	if c == nil || c.MaxIterateRounds <= 0 {
		return 3
	}
	return c.MaxIterateRounds
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
func NewOrchestratorWithConfig(ghOps GitHubOps, issueNumber int, cfg *OrchestratorConfig) *Orchestrator {
	// 确定默认 provider：优先 ImplementProvider，否则用第一个讨论 provider
	defaultProvider := cfg.ImplementProvider
	if defaultProvider == nil && len(cfg.DiscussionProviders) > 0 {
		defaultProvider = cfg.DiscussionProviders[0]
	}

	// consolidator 默认用 defaultProvider
	consolidator := cfg.Consolidator
	if consolidator == nil {
		consolidator = defaultProvider
	}

	return &Orchestrator{
		github:      ghOps,
		provider:    defaultProvider,
		issueNumber: issueNumber,
		plan:        NewPlanEngine(consolidator),
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

	// 6. 转状态：fix → needs-discussion（跳过中间态 plan-draft，避免两次 API 调用的竞态）
	return o.github.ReplaceLabel(ctx, o.issueNumber, string(state.StateFixRequested), string(state.StateNeedsDiscussion))
}

// DoDiscussionCheck 检查讨论收敛状态
// 支持多 provider "左右互搏"：每个 provider 独立出方案，consolidator 汇总
func (o *Orchestrator) DoDiscussionCheck(ctx context.Context) error {
	currentState, err := o.currentState(ctx)
	if err != nil {
		return fmt.Errorf("读取状态失败: %w", err)
	}
	if currentState != state.StateNeedsDiscussion {
		return fmt.Errorf("当前状态 %s 不是讨论中（需要 %s）", currentState, state.StateNeedsDiscussion)
	}

	// 获取评论和 markers
	comments, err := o.github.ListComments(ctx, o.issueNumber)
	if err != nil {
		return fmt.Errorf("获取评论失败: %w", err)
	}

	summaryMC, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypeDiscussionSummary)
	if err != nil {
		return fmt.Errorf("查找汇总 marker 失败: %w", err)
	}

	warningMC, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypeConvergeWarning)
	if err != nil {
		return fmt.Errorf("查找预警 marker 失败: %w", err)
	}

	// 构建收敛检查输入
	checker := state.DefaultChecker()
	input := &state.ConvergenceInput{
		Comments: comments,
	}
	if summaryMC != nil {
		input.DiscussionSummary = summaryMC.Marker
	}
	if warningMC != nil {
		input.ConvergeWarning = warningMC.Marker
		input.WarningTime = warningMC.Comment.GetCreatedAt().Time
	}

	result := checker.Check(input)

	switch result {
	case state.ShouldFinalize:
		return o.DoPlanFinal(ctx)

	case state.ShouldWarn:
		rev := 1
		if warningMC != nil {
			rev = warningMC.Marker.Revision + 1
		}
		m := &marker.Marker{
			Type:     marker.TypeConvergeWarning,
			Issue:    o.issueNumber,
			Revision: rev,
		}
		body := FormatConvergeWarning(m)
		return o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, body)

	default: // NotConverged
		// 多 provider 讨论模式
		if o.hasMultipleDiscussionProviders() {
			return o.doMultiProviderDiscussion(ctx, summaryMC)
		}
		// 单 provider 模式
		return o.updateDiscussionSummary(ctx, summaryMC)
	}
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

	// AI 生成最终方案
	finalPlan, err := o.plan.Final(ctx, input)
	if err != nil {
		return err
	}

	// 安全校验
	var extraPrefixes []string
	if o.config != nil {
		extraPrefixes = o.config.AllowedPrefixes
	}
	validation := ValidateChanges(finalPlan.FileChanges, extraPrefixes...)
	if !validation.IsClean() {
		errorBody := FormatValidationError(validation)
		_, _ = o.github.AddComment(ctx, o.issueNumber, errorBody)
		return fmt.Errorf("方案路径校验失败: %d 个路径被拒绝", len(validation.Rejected))
	}
	// 高风险路径警告（允许通过但提醒）
	if len(validation.HighRisk) > 0 {
		_, _ = o.github.AddComment(ctx, o.issueNumber, FormatHighRiskWarning(validation))
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

	// 实现逻辑封装，失败时回滚状态
	prNumber, implErr := o.doImplementInner(ctx, input, workDir)
	if implErr != nil {
		// 回滚状态并通知
		_ = o.github.ReplaceLabel(ctx, o.issueNumber, string(state.StateImplementing), string(prevState))
		_, _ = o.github.AddComment(ctx, o.issueNumber,
			fmt.Sprintf("## ❌ 实现失败\n\n%s\n\n状态已回滚到 `%s`。", implErr.Error(), prevState))
		return implErr
	}

	// 无变更时不创建 PR marker，提前返回
	if prNumber == 0 {
		_, _ = o.github.AddComment(ctx, o.issueNumber,
			"## ℹ️ 代码实现\n\nAI 执行完成，但 worktree 中无文件变更。状态已回滚。")
		_ = o.github.ReplaceLabel(ctx, o.issueNumber, string(state.StateImplementing), string(prevState))
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

// doImplementInner 执行实现的内部逻辑，返回 PR 号（0 表示无变更）
func (o *Orchestrator) doImplementInner(ctx context.Context, input *PromptInput, workDir string) (int, error) {
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
		_ = o.github.ReplaceLabel(ctx, o.issueNumber, string(state.StateIterating), string(prevState))
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
			// worktree 不存在（CI runner 重启等），从远程分支重建
			slug := slugFromTitle(input.IssueTitle)
			wtPath, err := ws.Create(o.issueNumber, slug)
			if err != nil {
				return fmt.Errorf("重建 worktree 失败: %w", err)
			}
			actualWorkDir = wtPath
			branchName = ws.BranchName(o.issueNumber, slug)
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

	// 发布修复结果
	_, err = o.github.AddComment(ctx, o.issueNumber,
		fmt.Sprintf("## 🔄 迭代修复\n\n根据 review 意见进行了修改。\n\n<details>\n<summary>修复详情</summary>\n\n%s\n\n</details>", result))
	if err != nil {
		return fmt.Errorf("发布修复结果失败: %w", err)
	}

	return nil
}

// DoReview AI 自审 PR
// 前置：bot:pr-created 状态
// 后置：通过 → bot:pr-reviewable；不通过 → bot:pr-needs-fix
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

	// 构建 review prompt
	reviewPrompt, err := BuildReviewPrompt(input)
	if err != nil {
		return fmt.Errorf("构建 review prompt 失败: %w", err)
	}

	// AI 审查
	raw, err := o.provider.Complete(ctx, reviewPrompt)
	if err != nil {
		return fmt.Errorf("AI 审查失败: %w", err)
	}

	// 解析审查结果
	result, err := ParseReviewResponse(raw)
	if err != nil {
		return fmt.Errorf("解析审查结果失败: %w", err)
	}

	// 发 PR review
	// 注意：GitHub 不允许对自己的 PR 提 REQUEST_CHANGES，
	// 所以不通过时用 COMMENT，通过时尝试 APPROVE 失败则回退到 COMMENT
	reviewBody := FormatReviewResult(result)
	event := "COMMENT"
	if result.Approved {
		event = "APPROVE"
	}
	_, reviewErr := o.github.CreatePRReview(ctx, prNumber, reviewBody, event)
	if reviewErr != nil && result.Approved {
		// APPROVE 失败（可能是自己的 PR），回退到 COMMENT
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

// ===== 内部方法 =====

// currentState 读取当前状态
func (o *Orchestrator) currentState(ctx context.Context) (state.State, error) {
	labels, err := o.github.ListLabels(ctx, o.issueNumber)
	if err != nil {
		return "", err
	}

	var found []state.State
	for _, label := range labels {
		if s, err := state.ParseState(label); err == nil {
			found = append(found, s)
		}
	}

	switch len(found) {
	case 0:
		return "", fmt.Errorf("issue #%d 没有 bot: 状态 label", o.issueNumber)
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("issue #%d 有多个 bot: 状态 label: %v（请手动清理）", o.issueNumber, found)
	}
}

// transition 执行状态转换
func (o *Orchestrator) transition(ctx context.Context, to state.State) error {
	current, err := o.currentState(ctx)
	if err != nil {
		return fmt.Errorf("读取状态失败: %w", err)
	}

	if !state.IsValidTransition(current, to) {
		return fmt.Errorf("非法状态转换: %s → %s", current, to)
	}

	return o.github.ReplaceLabel(ctx, o.issueNumber, string(current), string(to))
}

// buildPromptInput 从 GitHub 读取 issue 和评论构建 PromptInput
func (o *Orchestrator) buildPromptInput(ctx context.Context) (*PromptInput, error) {
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
		Comments:   gh.ToCommentBodies(comments),
	}, nil
}

// updateDiscussionSummary 更新讨论汇总（单 provider 模式）
func (o *Orchestrator) updateDiscussionSummary(ctx context.Context, existing *gh.MarkerComment) error {
	input, err := o.buildPromptInput(ctx)
	if err != nil {
		return err
	}

	summary, err := o.plan.Consolidate(ctx, input)
	if err != nil {
		return err
	}

	rev := 1
	if existing != nil {
		rev = existing.Marker.Revision + 1
	}

	m := &marker.Marker{
		Type:     marker.TypeDiscussionSummary,
		Issue:    o.issueNumber,
		Revision: rev,
	}
	body := FormatDiscussionSummary(summary, m)
	return o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, body)
}

// hasMultipleDiscussionProviders 检查是否配置了多个讨论 provider
func (o *Orchestrator) hasMultipleDiscussionProviders() bool {
	return o.config != nil && len(o.config.DiscussionProviders) > 1
}

// doMultiProviderDiscussion 多 provider "左右互搏"讨论
// 每个 provider 独立给出方案，然后 consolidator 汇总
func (o *Orchestrator) doMultiProviderDiscussion(ctx context.Context, existing *gh.MarkerComment) error {
	input, err := o.buildPromptInput(ctx)
	if err != nil {
		return err
	}

	prompt, err := BuildDraftPrompt(input)
	if err != nil {
		return fmt.Errorf("构建讨论 prompt 失败: %w", err)
	}

	// 收集各 provider 的方案
	var opinions []string
	for _, p := range o.config.DiscussionProviders {
		raw, err := p.Complete(ctx, prompt)
		if err != nil {
			opinions = append(opinions, fmt.Sprintf("[%s] (错误: %v)", p.Name(), err))
			continue
		}
		opinions = append(opinions, fmt.Sprintf("[%s 的方案]\n%s", p.Name(), raw))
	}

	// 用 consolidator 汇总（先 copy 再 append，避免污染 input.Comments 底层数组）
	merged := make([]string, 0, len(input.Comments)+len(opinions))
	merged = append(merged, input.Comments...)
	merged = append(merged, opinions...)
	consolidateInput := &PromptInput{
		IssueTitle: input.IssueTitle,
		IssueBody:  input.IssueBody,
		Comments:   merged,
	}
	summary, err := o.plan.Consolidate(ctx, consolidateInput)
	if err != nil {
		return fmt.Errorf("汇总讨论失败: %w", err)
	}

	rev := 1
	if existing != nil {
		rev = existing.Marker.Revision + 1
	}

	m := &marker.Marker{
		Type:     marker.TypeDiscussionSummary,
		Issue:    o.issueNumber,
		Revision: rev,
	}
	body := FormatDiscussionSummary(summary, m)
	return o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, body)
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

// BuildImplementContext 从 PromptInput 构建实现上下文
func BuildImplementContext(input *PromptInput) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Issue: %s\n\n", input.IssueTitle))
	sb.WriteString(input.IssueBody)
	return sb.String()
}
