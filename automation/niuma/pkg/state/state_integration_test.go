//go:build integration

package state

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_StateLifecycle(t *testing.T) {
	token := testutil.SkipIfNoToken(t)
	client, err := gh.NewClient(token, testutil.ResolveTestRepoFull())
	require.NoError(t, err)

	ctx := context.Background()

	// 确保 labels 存在
	err = client.EnsureLabelsExist(ctx, AllBotLabels())
	require.NoError(t, err)

	// 创建测试 issue
	suffix := fmt.Sprintf("%05d", rand.Intn(100000))
	title := fmt.Sprintf("[niuma-test-%s] State Lifecycle", suffix)
	issue, err := client.CreateIssue(ctx, title, "State machine integration test", nil)
	require.NoError(t, err)
	number := issue.GetNumber()
	t.Cleanup(func() {
		_ = client.CloseIssue(context.Background(), number)
	})

	// 添加初始状态 label
	err = client.AddLabel(ctx, number, string(StateFixRequested))
	require.NoError(t, err)

	// 创建状态机
	m := NewMachine(client, number)

	// 读取当前状态
	current, err := m.Current(ctx)
	require.NoError(t, err)
	assert.Equal(t, StateFixRequested, current)

	// 转换：fix → plan-draft
	err = m.Transition(ctx, StatePlanDraft)
	require.NoError(t, err)

	current, err = m.Current(ctx)
	require.NoError(t, err)
	assert.Equal(t, StatePlanDraft, current)

	// 非法转换应该失败
	err = m.Transition(ctx, StateImplementing)
	assert.Error(t, err)

	// 继续合法转换：plan-draft → plan-final
	err = m.Transition(ctx, StatePlanFinal)
	require.NoError(t, err)

	current, err = m.Current(ctx)
	require.NoError(t, err)
	assert.Equal(t, StatePlanFinal, current)
}
