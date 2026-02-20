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

func TestExecuteLabelGuard_InvalidAction(t *testing.T) {
	mock := &mockLGClient{}
	ctx := context.Background()

	// 未知 action 应返回错误
	err := executeLabelGuard(ctx, mock, "dry-run", 1, "random-user", "reopened", "bot:fix")
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
	if !strings.Contains(err.Error(), "不支持的 label-guard action") {
		t.Errorf("error should mention invalid action, got: %v", err)
	}
	// 不应发评论或操作标签
	if len(mock.addedComments) != 0 {
		t.Errorf("should not post comments for invalid action")
	}
	if mock.replacedWith != nil {
		t.Errorf("should not replace labels for invalid action")
	}

	// enforce 模式下也应拒绝
	err = executeLabelGuard(ctx, mock, "enforce", 1, "random-user", "unknown", "bot:fix")
	if err == nil {
		t.Fatal("expected error for invalid action in enforce mode")
	}
}

func TestValidateLabelGuardAction(t *testing.T) {
	tests := []struct {
		action  string
		wantErr bool
	}{
		{"labeled", false},
		{"unlabeled", false},
		{"", true},
		{"reopened", true},
		{"closed", true},
		{"LABELED", true},
	}
	for _, tt := range tests {
		err := validateLabelGuardAction(tt.action)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateLabelGuardAction(%q) error = %v, wantErr = %v", tt.action, err, tt.wantErr)
		}
	}
}

func TestExecuteLabelGuard_EnvOverrideAllowlist(t *testing.T) {
	// 设置环境变量 override allowlist
	t.Setenv("NIUMA_LABEL_ALLOWLIST", "env-user,another-env-user")

	overridden := splitAndTrim("env-user,another-env-user", ",")
	if !isInAllowlist("env-user", overridden) {
		t.Error("env-user should be in env-overridden allowlist")
	}
	if isInAllowlist("github-actions[bot]", overridden) {
		t.Error("github-actions[bot] should NOT be in env-overridden allowlist")
	}
}

func TestExecuteLabelGuard_EnvOverrideMode(t *testing.T) {
	// 测试 mode 环境变量 override
	t.Setenv("NIUMA_LABEL_GUARD_MODE", "enforce")

	mode := "enforce"
	if err := validateLabelGuardMode(mode); err != nil {
		t.Errorf("enforce should be a valid mode: %v", err)
	}

	// 无效 mode 也能通过 validateLabelGuardMode 校验拦截
	if err := validateLabelGuardMode("bad-mode"); err == nil {
		t.Error("bad-mode should be rejected by validateLabelGuardMode")
	}
}

// runLabelGuardWithFlags 通过设置全局 flag 变量和 mock client 调用真实 runLabelGuard
func runLabelGuardWithFlags(t *testing.T, mock *mockLGClient, repo string, issue int, actor, action, label, repoDir string) error {
	t.Helper()
	// 保存并恢复全局状态
	oldRepo := flagRepo
	oldIssue := flagIssue
	oldRepoDir := flagRepoDir
	oldActor := flagLabelGuardActor
	oldAction := flagLabelGuardAction
	oldLabel := flagLabelGuardLabel
	oldFactory := labelGuardClientFactory
	t.Cleanup(func() {
		flagRepo = oldRepo
		flagIssue = oldIssue
		flagRepoDir = oldRepoDir
		flagLabelGuardActor = oldActor
		flagLabelGuardAction = oldAction
		flagLabelGuardLabel = oldLabel
		labelGuardClientFactory = oldFactory
	})

	flagRepo = repo
	flagIssue = issue
	flagRepoDir = repoDir
	flagLabelGuardActor = actor
	flagLabelGuardAction = action
	flagLabelGuardLabel = label
	labelGuardClientFactory = func(_ string) (labelGuardGitHubClient, error) {
		return mock, nil
	}
	return runLabelGuard(nil, nil)
}

// TestRunLabelGuard_EnvOverrideAllowlist 通过 runLabelGuard 完整路径测试 env override allowlist
func TestRunLabelGuard_EnvOverrideAllowlist(t *testing.T) {
	// 配置文件中 allowlist 不含 env-user
	dir := t.TempDir()
	writeNiumaYml(t, dir, `
label_guard:
  allowlist:
    - github-actions[bot]
  mode: dry-run
`)

	// env override 让 env-user 成为 allowlist 成员 → 应放行
	t.Setenv("NIUMA_LABEL_ALLOWLIST", "env-user,another-env-user")

	mock := &mockLGClient{}
	err := runLabelGuardWithFlags(t, mock, "owner/repo", 1, "env-user", "labeled", "bot:fix", dir)
	if err != nil {
		t.Fatalf("expected no error when actor is in env-overridden allowlist, got: %v", err)
	}
	// 放行不应发评论或操作标签
	if len(mock.addedComments) != 0 {
		t.Errorf("allowlist pass should not post comments, got %d", len(mock.addedComments))
	}
}

// TestRunLabelGuard_EnvOverrideMode 通过 runLabelGuard 完整路径测试 env override mode
func TestRunLabelGuard_EnvOverrideMode(t *testing.T) {
	// 配置文件中 mode=dry-run
	dir := t.TempDir()
	writeNiumaYml(t, dir, `
label_guard:
  allowlist:
    - github-actions[bot]
  mode: dry-run
`)

	// env override mode 为 enforce
	t.Setenv("NIUMA_LABEL_GUARD_MODE", "enforce")

	mock := &mockLGClient{
		labels: []string{"bot:fix", "enhancement"},
	}
	err := runLabelGuardWithFlags(t, mock, "owner/repo", 1, "random-user", "labeled", "bot:fix", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// env override 为 enforce 应触发回滚
	if mock.replacedWith == nil {
		t.Error("enforce mode (via env override) should trigger label rollback")
	}
	if len(mock.addedComments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(mock.addedComments))
	}
	if !strings.Contains(mock.addedComments[0], "已自动回滚") {
		t.Error("enforce mode comment should mention 已自动回滚")
	}
}

// TestRunLabelGuard_EnvOverrideInvalidMode 通过 runLabelGuard 测试无效 mode env override 被拦截
func TestRunLabelGuard_EnvOverrideInvalidMode(t *testing.T) {
	dir := t.TempDir()
	writeNiumaYml(t, dir, `
label_guard:
  allowlist: []
  mode: dry-run
`)

	t.Setenv("NIUMA_LABEL_GUARD_MODE", "bad-mode")

	mock := &mockLGClient{}
	err := runLabelGuardWithFlags(t, mock, "owner/repo", 1, "random-user", "labeled", "bot:fix", dir)
	if err == nil {
		t.Fatal("expected error for invalid mode from env override")
	}
	if !strings.Contains(err.Error(), "不支持的 label-guard mode") {
		t.Errorf("error should mention invalid mode, got: %v", err)
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

// TestRunLabelGuard_AllowlistActorInvalidAction 验证 allowlisted actor + 非法 action 应报错（不应被 allowlist 短路放行）
func TestRunLabelGuard_AllowlistActorInvalidAction(t *testing.T) {
	dir := t.TempDir()
	writeNiumaYml(t, dir, `
label_guard:
  allowlist:
    - github-actions[bot]
  mode: dry-run
`)

	mock := &mockLGClient{}
	err := runLabelGuardWithFlags(t, mock, "owner/repo", 1, "github-actions[bot]", "reopened", "bot:fix", dir)
	if err == nil {
		t.Fatal("expected error for allowlisted actor with invalid action")
	}
	if !strings.Contains(err.Error(), "不支持的 label-guard action") {
		t.Errorf("error should mention invalid action, got: %v", err)
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
