//go:build integration

package github

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/internal/testutil"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T) *Client {
	token := testutil.SkipIfNoToken(t)
	client, err := NewClient(token, testutil.ResolveTestRepoFull())
	require.NoError(t, err)
	return client
}

func createTestIssue(t *testing.T, client *Client) int {
	ctx := context.Background()
	suffix := fmt.Sprintf("%05d", rand.Intn(100000))
	title := fmt.Sprintf("[niuma-test-%s] Integration Test", suffix)
	issue, err := client.CreateIssue(ctx, title, "Automated integration test issue", nil)
	require.NoError(t, err)

	number := issue.GetNumber()
	t.Cleanup(func() {
		_ = client.CloseIssue(context.Background(), number)
	})
	return number
}

func TestIntegration_IssueLifecycle(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	// 创建 issue
	number := createTestIssue(t, client)

	// 获取 issue
	issue, err := client.GetIssue(ctx, number)
	require.NoError(t, err)
	assert.Contains(t, issue.GetTitle(), "niuma-test-")
}

func TestIntegration_CommentsAndMarkers(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	number := createTestIssue(t, client)

	// 添加评论
	comment, err := client.AddComment(ctx, number, "Test comment body")
	require.NoError(t, err)
	assert.NotZero(t, comment.GetID())

	// 列出评论
	comments, err := client.ListComments(ctx, number)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(comments), 1)

	// 创建带 marker 的评论
	m := &marker.Marker{Type: marker.TypePlanDraft, Issue: number, Revision: 1}
	err = client.CreateOrUpdateMarker(ctx, number, m, "Draft plan content")
	require.NoError(t, err)

	// 查找 marker
	found, err := client.FindMarker(ctx, number, marker.TypePlanDraft)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, 1, found.Marker.Revision)

	// 更新 marker
	m2 := &marker.Marker{Type: marker.TypePlanDraft, Issue: number, Revision: 2}
	err = client.CreateOrUpdateMarker(ctx, number, m2, "Updated draft plan")
	require.NoError(t, err)

	found2, err := client.FindMarker(ctx, number, marker.TypePlanDraft)
	require.NoError(t, err)
	require.NotNil(t, found2)
	assert.Equal(t, 2, found2.Marker.Revision)

	// 删除评论
	err = client.DeleteComment(ctx, comment.GetID())
	require.NoError(t, err)
}

func TestIntegration_Labels(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	number := createTestIssue(t, client)

	// 确保 label 存在
	err := client.EnsureLabelsExist(ctx, []string{"bug", "priority:high"})
	require.NoError(t, err)

	// 添加 label
	err = client.AddLabel(ctx, number, "bug")
	require.NoError(t, err)

	// 列出 label
	labels, err := client.ListLabels(ctx, number)
	require.NoError(t, err)
	assert.Contains(t, labels, "bug")

	// 替换 label
	err = client.ReplaceLabel(ctx, number, "bug", "priority:high")
	require.NoError(t, err)

	labels, err = client.ListLabels(ctx, number)
	require.NoError(t, err)
	assert.Contains(t, labels, "priority:high")
	assert.NotContains(t, labels, "bug")

	// 原子替换整组 labels（保留单一 bot 状态）
	err = client.ReplaceLabels(ctx, number, []string{"bug", "bot:needs-discussion"})
	require.NoError(t, err)
	labels, err = client.ListLabels(ctx, number)
	require.NoError(t, err)
	assert.Contains(t, labels, "bug")
	assert.Contains(t, labels, "bot:needs-discussion")
	assert.NotContains(t, labels, "priority:high")

	// 移除 label
	err = client.RemoveLabel(ctx, number, "bug")
	require.NoError(t, err)
}

func TestIntegration_ToCommentBodies(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	number := createTestIssue(t, client)

	_, err := client.AddComment(ctx, number, "Comment 1")
	require.NoError(t, err)
	_, err = client.AddComment(ctx, number, "Comment 2")
	require.NoError(t, err)

	comments, err := client.ListComments(ctx, number)
	require.NoError(t, err)

	bodies := ToCommentBodies(comments)
	assert.Len(t, bodies, 2)
	assert.Equal(t, "Comment 1", bodies[0])
	assert.Equal(t, "Comment 2", bodies[1])
}

func TestIntegration_IssueDependenciesMirror(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	depIssue := createTestIssue(t, client)
	targetIssue := createTestIssue(t, client)

	err := client.AddIssueBlockedBy(ctx, targetIssue, depIssue)
	if err != nil {
		if ClassifyDependencyError(err) == DependencyErrorTypeUnsupported {
			t.Skipf("issue dependency API unsupported: %v", err)
		}
		require.NoError(t, err)
	}

	blockedBy, err := client.ListIssueBlockedBy(ctx, targetIssue)
	if err != nil {
		if ClassifyDependencyError(err) == DependencyErrorTypeUnsupported {
			t.Skipf("issue dependency API unsupported: %v", err)
		}
		require.NoError(t, err)
	}
	assert.Contains(t, blockedBy, depIssue)

	err = client.RemoveIssueBlockedBy(ctx, targetIssue, depIssue)
	if err != nil {
		if ClassifyDependencyError(err) == DependencyErrorTypeUnsupported {
			t.Skipf("issue dependency API unsupported: %v", err)
		}
		require.NoError(t, err)
	}
}
