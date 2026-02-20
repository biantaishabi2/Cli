package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	ghapi "github.com/google/go-github/v68/github"
)

// mockLGClient 模拟 label-guard 的 GitHub 操作
type mockLGClient struct {
	comments       []*ghapi.IssueComment
	labels         []string
	addedComments  []string
	replacedWith   []string
	replaceErr     error
	addCommentErr  error
	listLabelsErr  error
}

func (m *mockLGClient) ListComments(ctx context.Context, issue int) ([]*ghapi.IssueComment, error) {
	return m.comments, nil
}

func (m *mockLGClient) AddComment(ctx context.Context, issue int, body string) (*ghapi.IssueComment, error) {
	if m.addCommentErr != nil {
		return nil, m.addCommentErr
	}
	m.addedComments = append(m.addedComments, body)
	return &ghapi.IssueComment{}, nil
}

func (m *mockLGClient) ListLabels(ctx context.Context, issue int) ([]string, error) {
	if m.listLabelsErr != nil {
		return nil, m.listLabelsErr
	}
	return m.labels, nil
}

func (m *mockLGClient) ReplaceLabels(ctx context.Context, issue int, labels []string) error {
	if m.replaceErr != nil {
		return m.replaceErr
	}
	m.replacedWith = labels
	return nil
}

func TestExecuteLabelGuard_AllowlistPass(t *testing.T) {
	// allowlist 放行不会调用 executeLabelGuard，测试 isInAllowlist
	if !isInAllowlist("github-actions[bot]", []string{"github-actions[bot]", "niuma-bot"}) {
		t.Error("expected github-actions[bot] to be in allowlist")
	}
}

func TestExecuteLabelGuard_DryRunIntercept(t *testing.T) {
	mock := &mockLGClient{}
	ctx := context.Background()
	err := executeLabelGuard(ctx, mock, "dry-run", 1, "random-user", "labeled", "bot:fix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.addedComments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(mock.addedComments))
	}
	comment := mock.addedComments[0]
	if !strings.Contains(comment, "dry-run") {
		t.Errorf("comment should mention dry-run, got: %s", comment)
	}
	// 应包含 marker
	m := marker.Parse(extractMarkerLine(comment))
	if m == nil {
		t.Fatal("comment should contain a valid marker")
	}
	if m.Type != marker.TypeLabelGuard {
		t.Errorf("marker type = %s, want BOT:LABEL_GUARD", m.Type)
	}
	if m.Label != "bot:fix" {
		t.Errorf("marker label = %s, want bot:fix", m.Label)
	}
	// 不应有回滚操作
	if mock.replacedWith != nil {
		t.Errorf("dry-run should not replace labels")
	}
}

func TestExecuteLabelGuard_EnforceRollbackSuccess(t *testing.T) {
	mock := &mockLGClient{
		labels: []string{"bot:fix", "enhancement"},
	}
	ctx := context.Background()
	err := executeLabelGuard(ctx, mock, "enforce", 1, "random-user", "labeled", "bot:fix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 应回滚：移除 bot:fix
	if mock.replacedWith == nil {
		t.Fatal("enforce should replace labels")
	}
	for _, l := range mock.replacedWith {
		if l == "bot:fix" {
			t.Error("bot:fix should have been removed from labels")
		}
	}

	// 评论应包含"已自动回滚"
	if len(mock.addedComments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(mock.addedComments))
	}
	if !strings.Contains(mock.addedComments[0], "已自动回滚") {
		t.Errorf("comment should mention 已自动回滚")
	}
}

func TestExecuteLabelGuard_EnforceRollbackFailure(t *testing.T) {
	mock := &mockLGClient{
		labels:     []string{"bot:fix", "enhancement"},
		replaceErr: fmt.Errorf("API 403 forbidden"),
	}
	ctx := context.Background()
	err := executeLabelGuard(ctx, mock, "enforce", 1, "random-user", "labeled", "bot:fix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 评论应提到回滚失败，不应说"已自动回滚"
	if len(mock.addedComments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(mock.addedComments))
	}
	comment := mock.addedComments[0]
	if strings.Contains(comment, "已自动回滚") {
		t.Errorf("comment should NOT mention 已自动回滚 when rollback failed")
	}
	if !strings.Contains(comment, "自动回滚失败") {
		t.Errorf("comment should mention 自动回滚失败")
	}
	if !strings.Contains(comment, "API 403 forbidden") {
		t.Errorf("comment should contain the error message")
	}
}

func TestExecuteLabelGuard_EnforceUnlabeled(t *testing.T) {
	mock := &mockLGClient{
		labels: []string{"enhancement"},
	}
	ctx := context.Background()
	err := executeLabelGuard(ctx, mock, "enforce", 1, "random-user", "unlabeled", "bot:fix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 应回滚：添加回 bot:fix
	if mock.replacedWith == nil {
		t.Fatal("enforce should replace labels")
	}
	found := false
	for _, l := range mock.replacedWith {
		if l == "bot:fix" {
			found = true
		}
	}
	if !found {
		t.Error("bot:fix should have been added back to labels")
	}
}

func TestExecuteLabelGuard_MarkerDedup(t *testing.T) {
	// 构造已有 marker 的评论
	existingMarker := renderLabelGuardMarker("enforce", 1, "labeled", "bot:fix", "old-user")
	body := "some comment\n" + existingMarker
	mock := &mockLGClient{
		comments: []*ghapi.IssueComment{
			{Body: &body},
		},
	}
	ctx := context.Background()
	err := executeLabelGuard(ctx, mock, "enforce", 1, "another-user", "labeled", "bot:fix")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 应跳过，不发评论，不回滚
	if len(mock.addedComments) != 0 {
		t.Errorf("should skip due to duplicate marker, but posted %d comments", len(mock.addedComments))
	}
	if mock.replacedWith != nil {
		t.Error("should skip due to duplicate marker, but replaced labels")
	}
}

func TestValidateLabelGuardMode(t *testing.T) {
	tests := []struct {
		mode    string
		wantErr bool
	}{
		{"dry-run", false},
		{"enforce", false},
		{"", true},
		{"invalid", true},
		{"DRY-RUN", true},
		{"warn", true},
	}
	for _, tt := range tests {
		err := validateLabelGuardMode(tt.mode)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateLabelGuardMode(%q) error = %v, wantErr = %v", tt.mode, err, tt.wantErr)
		}
	}
}

func TestIsInAllowlist(t *testing.T) {
	tests := []struct {
		name      string
		actor     string
		allowlist []string
		want      bool
	}{
		{"actor in allowlist", "github-actions[bot]", []string{"github-actions[bot]", "niuma-bot"}, true},
		{"actor not in allowlist", "random-user", []string{"github-actions[bot]", "niuma-bot"}, false},
		{"empty allowlist", "github-actions[bot]", []string{}, false},
		{"nil allowlist", "github-actions[bot]", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInAllowlist(tt.actor, tt.allowlist)
			if got != tt.want {
				t.Errorf("isInAllowlist(%q, %v) = %v, want %v", tt.actor, tt.allowlist, got, tt.want)
			}
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a, b, c", []string{"a", "b", "c"}},
		{"github-actions[bot],niuma-bot", []string{"github-actions[bot]", "niuma-bot"}},
		{"  a ,  , b  ", []string{"a", "b"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := splitAndTrim(tt.input, ",")
		if len(got) != len(tt.want) {
			t.Errorf("splitAndTrim(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitAndTrim(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestRenderLabelGuardMarker(t *testing.T) {
	got := renderLabelGuardMarker("dry-run", 42, "labeled", "bot:fix", "random-user")
	m := marker.Parse(got)
	if m == nil {
		t.Fatalf("renderLabelGuardMarker output should be parseable by marker.Parse, got: %s", got)
	}
	if m.Type != marker.TypeLabelGuard {
		t.Errorf("type = %s, want BOT:LABEL_GUARD", m.Type)
	}
	if m.Issue != 42 {
		t.Errorf("issue = %d, want 42", m.Issue)
	}
	if m.Label != "bot:fix" {
		t.Errorf("label = %s, want bot:fix", m.Label)
	}
	if m.Action != "labeled" {
		t.Errorf("action = %s, want labeled", m.Action)
	}
	if m.Actor != "random-user" {
		t.Errorf("actor = %s, want random-user", m.Actor)
	}
	if m.Mode != "dry-run" {
		t.Errorf("mode = %s, want dry-run", m.Mode)
	}
}

// extractMarkerLine 从多行文本中提取包含 marker 的行
func extractMarkerLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "<!-- BOT:") {
			return line
		}
	}
	return ""
}
