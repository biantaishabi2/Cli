// pkg/agent/orchestrator.go
// 核心编排器：协调 AI、GitHub、状态机完成自动化流程
package agent

import (
	"context"
	"fmt"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/google/go-github/v68/github"
)

// Orchestrator 核心编排器
type Orchestrator struct {
	github      GitHubOps
	provider    ai.Provider
	issueNumber int
	plan        *PlanEngine
	codeGen     *CodeGen
}

// NewOrchestrator 创建编排器
func NewOrchestrator(ghOps GitHubOps, provider ai.Provider, issueNumber int) *Orchestrator {
	return &Orchestrator{
		github:      ghOps,
		provider:    provider,
		issueNumber: issueNumber,
		plan:        NewPlanEngine(provider),
		codeGen:     NewCodeGen(provider),
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

	// 6. 转状态：fix → plan-draft
	if err := o.transition(ctx, state.StatePlanDraft); err != nil {
		return err
	}

	// 7. 继续转到 needs-discussion
	return o.transition(ctx, state.StateNeedsDiscussion)
}

// DoDiscussionCheck 检查讨论收敛状态，根据结果执行不同操作
// - NotConverged: 更新讨论汇总
// - ShouldWarn: 发预警评论
// - ShouldFinalize: 自动定稿
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
		// 更新讨论汇总
		return o.updateDiscussionSummary(ctx, comments, summaryMC)
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
	validation := ValidateChanges(finalPlan.FileChanges)
	if !validation.IsClean() {
		// 发评论告知校验失败，但不转状态
		errorBody := FormatValidationError(validation)
		_, _ = o.github.AddComment(ctx, o.issueNumber, errorBody)
		return fmt.Errorf("方案路径校验失败: %d 个路径被拒绝", len(validation.Rejected))
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
func (o *Orchestrator) DoImplement(ctx context.Context, workDir string) error {
	currentState, err := o.currentState(ctx)
	if err != nil {
		return fmt.Errorf("读取状态失败: %w", err)
	}
	if currentState != state.StatePlanFinal {
		return fmt.Errorf("当前状态 %s 不允许实现（需要 %s）", currentState, state.StatePlanFinal)
	}

	// 读取最终方案
	finalMC, err := o.github.FindMarker(ctx, o.issueNumber, marker.TypePlanFinal)
	if err != nil || finalMC == nil {
		return fmt.Errorf("未找到最终方案")
	}

	input, err := o.buildPromptInput(ctx)
	if err != nil {
		return err
	}
	input.FinalPlan = finalMC.Comment.GetBody()

	// 转状态到 implementing
	if err := o.transition(ctx, state.StateImplementing); err != nil {
		return err
	}

	// AI 生成代码
	result, err := o.codeGen.Implement(ctx, input, workDir)
	if err != nil {
		return err
	}

	// 将实现结果作为评论发布
	_, err = o.github.AddComment(ctx, o.issueNumber,
		fmt.Sprintf("## 🔧 代码实现\n\nAI 已完成代码生成。\n\n<details>\n<summary>实现详情</summary>\n\n%s\n\n</details>", result.RawOutput))
	if err != nil {
		return fmt.Errorf("发布实现结果失败: %w", err)
	}

	// 创建 PR marker
	m := &marker.Marker{
		Type:     marker.TypePRCreated,
		Issue:    o.issueNumber,
		Revision: 1,
	}
	if err := o.github.CreateOrUpdateMarker(ctx, o.issueNumber, m, "PR 已创建"); err != nil {
		return fmt.Errorf("创建 PR marker 失败: %w", err)
	}

	// 转状态
	return o.transition(ctx, state.StatePRCreated)
}

// DoIterate 根据 review 意见迭代修改
func (o *Orchestrator) DoIterate(ctx context.Context, prNumber int, workDir string) error {
	currentState, err := o.currentState(ctx)
	if err != nil {
		return fmt.Errorf("读取状态失败: %w", err)
	}
	if currentState != state.StatePRNeedsFix {
		return fmt.Errorf("当前状态 %s 不允许迭代（需要 %s）", currentState, state.StatePRNeedsFix)
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
	if err != nil || finalMC == nil {
		return fmt.Errorf("未找到最终方案")
	}

	input, err := o.buildPromptInput(ctx)
	if err != nil {
		return err
	}
	input.FinalPlan = finalMC.Comment.GetBody()
	input.ReviewComment = reviewText

	// 转状态到 iterating
	if err := o.transition(ctx, state.StateIterating); err != nil {
		return err
	}

	// AI 修复
	result, err := o.codeGen.Iterate(ctx, input, workDir)
	if err != nil {
		return err
	}

	// 发布修复结果
	_, err = o.github.AddComment(ctx, o.issueNumber,
		fmt.Sprintf("## 🔄 迭代修复\n\n根据 review 意见进行了修改。\n\n<details>\n<summary>修复详情</summary>\n\n%s\n\n</details>", result.RawOutput))
	if err != nil {
		return fmt.Errorf("发布修复结果失败: %w", err)
	}

	// 更新 PR marker revision
	prMC, _ := o.github.FindMarker(ctx, o.issueNumber, marker.TypePRCreated)
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

// ===== 内部方法 =====

// currentState 读取当前状态
func (o *Orchestrator) currentState(ctx context.Context) (state.State, error) {
	labels, err := o.github.ListLabels(ctx, o.issueNumber)
	if err != nil {
		return "", err
	}

	for _, label := range labels {
		if s, err := state.ParseState(label); err == nil {
			return s, nil
		}
	}

	return "", fmt.Errorf("issue #%d 没有 bot: 状态 label", o.issueNumber)
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

// updateDiscussionSummary 更新讨论汇总
func (o *Orchestrator) updateDiscussionSummary(ctx context.Context, comments []*github.IssueComment, existing *gh.MarkerComment) error {
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
	return fmt.Sprintf("共 %d 条 review:\n\n%s", len(parts), fmt.Sprintf("- %s", fmt.Sprintf("%s", parts)))
}
