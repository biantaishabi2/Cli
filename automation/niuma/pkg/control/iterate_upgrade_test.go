package control

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgradeIterateToNeedsHuman(t *testing.T) {
	ctx := context.Background()
	labels := []string{"bug", "bot:pr-needs-fix", "bot:fix"}
	replaced := make([]string, 0)
	added := make([]string, 0)
	comments := make([]string, 0)

	err := UpgradeIterateToNeedsHuman(ctx, 12, 34, "human", "closed", IterateUpgradeOps{
		ListLabels: func(_ context.Context, issueNumber int) ([]string, error) {
			require.Equal(t, 12, issueNumber)
			return append([]string(nil), labels...), nil
		},
		ReplaceLabels: func(_ context.Context, issueNumber int, next []string) error {
			require.Equal(t, 12, issueNumber)
			replaced = append([]string(nil), next...)
			return nil
		},
		AddLabel: func(_ context.Context, issueNumber int, label string) error {
			require.Equal(t, 12, issueNumber)
			added = append(added, label)
			return nil
		},
		AddComment: func(_ context.Context, issueNumber int, body string) error {
			require.Equal(t, 12, issueNumber)
			comments = append(comments, body)
			return nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"bug"}, replaced)
	assert.Equal(t, []string{needsHumanIssueLabel}, added)
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0], "自动迭代已停止")
	assert.Contains(t, comments[0], "触发来源：`human`")
	assert.Contains(t, comments[0], "PR：#34")
	assert.Contains(t, comments[0], "PR 状态：`CLOSED`")
}
