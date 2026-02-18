// pkg/agent/loop_core.go
// 通用循环骨架：统一多轮执行、收敛判断与日志输出
package agent

import (
	"context"
	"fmt"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
)

// LoopCoreResult 循环执行结果
type LoopCoreResult struct {
	Converged      bool
	LastDecision   *state.ConvergenceDecision
	RoundsExecuted int
}

// runLoopCore 执行通用循环骨架。
//
// scopeTag 用于日志前缀（例如 discuss）。
// scopeName 用于错误文案（例如 讨论）。
func (o *Orchestrator) runLoopCore(
	ctx context.Context,
	scopeTag string,
	scopeName string,
	maxRounds int,
	runRound func(context.Context, int, int) (*state.ConvergenceDecision, error),
) (*LoopCoreResult, error) {
	result := &LoopCoreResult{}
	for round := 1; round <= maxRounds; round++ {
		decision, err := runRound(ctx, round, maxRounds)
		if err != nil {
			return nil, fmt.Errorf("%s轮次失败 (issue=%d round=%d max=%d): %w", scopeName, o.issueNumber, round, maxRounds, err)
		}

		result.RoundsExecuted = round
		result.LastDecision = decision

		fmt.Printf("[%s] issue=%d round=%d max=%d converged=%t result=%s reason=%s\n",
			scopeTag, o.issueNumber, round, maxRounds, decision.Converged(), decision.Result.String(), decision.Reason)

		if decision.Converged() {
			result.Converged = true
			return result, nil
		}
	}

	fmt.Printf("[%s] issue=%d round=%d max=%d converged=false reason=max_rounds_reached\n",
		scopeTag, o.issueNumber, maxRounds, maxRounds)

	return result, nil
}
