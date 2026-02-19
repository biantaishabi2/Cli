// pkg/state/machine.go
// 状态机：通过 GitHub Label 读写当前状态
package state

import (
	"context"
	"fmt"

	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
)

// Machine 管理单个 issue 的状态机
type Machine struct {
	client      *gh.Client
	issueNumber int
}

// NewMachine 创建状态机实例
func NewMachine(client *gh.Client, issueNumber int) *Machine {
	return &Machine{
		client:      client,
		issueNumber: issueNumber,
	}
}

// Current 读取当前状态（从 issue 的 label 中识别 bot:* label）
func (m *Machine) Current(ctx context.Context) (State, error) {
	labels, err := m.client.ListLabels(ctx, m.issueNumber)
	if err != nil {
		return "", fmt.Errorf("读取状态失败: %w", err)
	}

	current, found, err := CurrentBotState(labels)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("issue #%d 没有 bot: 状态 label", m.issueNumber)
	}

	return current, nil
}

// Transition 执行状态转换：校验合法性，替换 label
func (m *Machine) Transition(ctx context.Context, to State) error {
	current, err := m.Current(ctx)
	if err != nil {
		return err
	}

	return TransitionBotState(ctx, m.client, m.issueNumber, current, to)
}

// IssueNumber 返回关联的 issue 编号
func (m *Machine) IssueNumber() int {
	return m.issueNumber
}

// Client 返回 GitHub 客户端
func (m *Machine) Client() *gh.Client {
	return m.client
}
