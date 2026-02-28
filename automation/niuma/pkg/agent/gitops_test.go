package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitOps_HasChanges_NoChanges(t *testing.T) {
	repoDir := initTestRepo(t)
	g := NewGitOps(repoDir)

	has, err := g.HasChanges()
	require.NoError(t, err)
	assert.False(t, has)
}

func TestGitOps_HasChanges_WithChanges(t *testing.T) {
	repoDir := initTestRepo(t)
	g := NewGitOps(repoDir)

	// 创建新文件
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "new.txt"), []byte("hello"), 0644))

	has, err := g.HasChanges()
	require.NoError(t, err)
	assert.True(t, has)
}

func TestGitOps_CommitAll(t *testing.T) {
	repoDir := initTestRepo(t)
	g := NewGitOps(repoDir)

	// 创建文件
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "feature.go"), []byte("package main"), 0644))

	err := g.CommitAll("feat: add feature #1")
	require.NoError(t, err)

	// 验证没有未提交变更
	has, err := g.HasChanges()
	require.NoError(t, err)
	assert.False(t, has)

	// 验证 commit 消息
	cmd := exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "feat: add feature #1")
}

func TestGitOps_CurrentBranch(t *testing.T) {
	repoDir := initTestRepo(t)
	g := NewGitOps(repoDir)

	branch, err := g.CurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "master", branch)
}

func TestGitOps_CommitAll_NoChanges(t *testing.T) {
	repoDir := initTestRepo(t)
	g := NewGitOps(repoDir)

	// 没有变更时 commit 应该失败
	err := g.CommitAll("empty commit")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git commit 失败")
}

func TestGitOps_CommitAll_BlocksSensitiveFiles(t *testing.T) {
	repoDir := initTestRepo(t)
	g := NewGitOps(repoDir)

	// 创建敏感文件
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".env"), []byte("SECRET=x"), 0644))

	err := g.CommitAll("should fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "敏感文件")
	assert.Contains(t, err.Error(), ".env")

	// 验证暂存区已回滚（文件仍是 untracked）
	has, err := g.HasChanges()
	require.NoError(t, err)
	assert.True(t, has) // 文件还在但未暂存
}

func TestGitOps_CommitAll_BlocksEnvVariants(t *testing.T) {
	repoDir := initTestRepo(t)
	g := NewGitOps(repoDir)

	// .env.staging 也应该被拦截
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".env.staging"), []byte("DB=x"), 0644))

	err := g.CommitAll("should fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "敏感文件")
}

func TestGitOps_CommitAll_BlocksKeyFiles(t *testing.T) {
	repoDir := initTestRepo(t)
	g := NewGitOps(repoDir)

	// .pem 文件
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "server.pem"), []byte("CERT"), 0644))

	err := g.CommitAll("should fail")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "敏感文件")
}

func TestIsSensitiveFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{".env", true},
		{"config/.env", true},
		{".env.local", true},
		{".env.staging", true},
		{".env.production", true},
		{"credentials.json", true},
		{"path/to/credentials.json", true},
		{"id_rsa", true},
		{"~/.ssh/id_rsa", true},
		{"id_ed25519", true},
		{"server.key", true},
		{"cert.pem", true},
		{"store.p12", true},
		{"keystore.pfx", true},
		{"id_ecdsa", true},
		// 正常文件不应被拦截
		{"main.go", false},
		{"src/auth.go", false},
		{"README.md", false},
		{"environment.go", false}, // 不应误匹配 .env
		{"requirements.txt", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, isSensitiveFile(tt.path))
		})
	}
}

type mockGitOutputResult struct {
	out string
	err error
}

type mockGitOutputExecutor struct {
	results map[string]mockGitOutputResult
	calls   []string
}

func (m *mockGitOutputExecutor) CombinedOutput(_ string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	m.calls = append(m.calls, key)
	if result, ok := m.results[key]; ok {
		return []byte(result.out), result.err
	}
	return nil, fmt.Errorf("unexpected git command: %s", key)
}

func TestResolveDefaultBranchWithExecutor(t *testing.T) {
	tests := []struct {
		name           string
		results        map[string]mockGitOutputResult
		expectedBranch string
		expectedSource string
		expectedCalls  []string
		usedFallback   bool
	}{
		{
			name: "symbolic-ref success",
			results: map[string]mockGitOutputResult{
				"symbolic-ref refs/remotes/origin/HEAD": {out: "refs/remotes/origin/main\n"},
			},
			expectedBranch: "main",
			expectedSource: "symbolic-ref",
			expectedCalls:  []string{"symbolic-ref refs/remotes/origin/HEAD"},
		},
		{
			name: "ls-remote symref fallback",
			results: map[string]mockGitOutputResult{
				"symbolic-ref refs/remotes/origin/HEAD": {err: fmt.Errorf("missing origin/HEAD")},
				"ls-remote --symref origin HEAD":        {out: "ref: refs/heads/develop\tHEAD\nabc\tHEAD\n"},
			},
			expectedBranch: "develop",
			expectedSource: "ls-remote-symref",
			expectedCalls: []string{
				"symbolic-ref refs/remotes/origin/HEAD",
				"ls-remote --symref origin HEAD",
			},
		},
		{
			name: "local origin/main fallback",
			results: map[string]mockGitOutputResult{
				"symbolic-ref refs/remotes/origin/HEAD": {err: fmt.Errorf("missing origin/HEAD")},
				"ls-remote --symref origin HEAD":        {err: fmt.Errorf("origin unavailable")},
				"rev-parse --verify refs/remotes/origin/main": {
					out: "0123456789abcdef\n",
				},
			},
			expectedBranch: "main",
			expectedSource: "local-origin-main",
			expectedCalls: []string{
				"symbolic-ref refs/remotes/origin/HEAD",
				"ls-remote --symref origin HEAD",
				"rev-parse --verify refs/remotes/origin/main",
			},
		},
		{
			name: "remote show fallback",
			results: map[string]mockGitOutputResult{
				"symbolic-ref refs/remotes/origin/HEAD":       {err: fmt.Errorf("missing origin/HEAD")},
				"ls-remote --symref origin HEAD":              {err: fmt.Errorf("origin unavailable")},
				"rev-parse --verify refs/remotes/origin/main": {err: fmt.Errorf("no origin/main")},
				"remote show origin":                          {out: "  HEAD branch: release/v2\n"},
			},
			expectedBranch: "release/v2",
			expectedSource: "remote-show",
			expectedCalls: []string{
				"symbolic-ref refs/remotes/origin/HEAD",
				"ls-remote --symref origin HEAD",
				"rev-parse --verify refs/remotes/origin/main",
				"remote show origin",
			},
		},
		{
			name: "fallback master when all probes fail",
			results: map[string]mockGitOutputResult{
				"symbolic-ref refs/remotes/origin/HEAD":       {err: fmt.Errorf("missing origin/HEAD")},
				"ls-remote --symref origin HEAD":              {err: fmt.Errorf("origin unavailable")},
				"rev-parse --verify refs/remotes/origin/main": {err: fmt.Errorf("no origin/main")},
				"remote show origin":                          {err: fmt.Errorf("origin unavailable")},
			},
			expectedBranch: "master",
			expectedSource: "fallback-master",
			usedFallback:   true,
			expectedCalls: []string{
				"symbolic-ref refs/remotes/origin/HEAD",
				"ls-remote --symref origin HEAD",
				"rev-parse --verify refs/remotes/origin/main",
				"remote show origin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockGitOutputExecutor{results: tt.results}
			result := resolveDefaultBranchWithExecutor(t.TempDir(), executor, nil)

			assert.Equal(t, tt.expectedBranch, result.Branch)
			assert.Equal(t, tt.expectedSource, result.Source)
			assert.Equal(t, tt.usedFallback, result.UsedFallback)
			assert.Equal(t, tt.expectedCalls, executor.calls)
		})
	}
}

func TestResolveDefaultBranchWithExecutor_StrictParsing(t *testing.T) {
	executor := &mockGitOutputExecutor{
		results: map[string]mockGitOutputResult{
			"symbolic-ref refs/remotes/origin/HEAD":       {out: "origin/main\n"},
			"ls-remote --symref origin HEAD":              {out: "ref: refs/heads/feature/x\tHEAD\n"},
			"rev-parse --verify refs/remotes/origin/main": {err: fmt.Errorf("should not reach")},
			"remote show origin":                          {err: fmt.Errorf("should not reach")},
		},
	}

	result := resolveDefaultBranchWithExecutor(t.TempDir(), executor, nil)
	assert.Equal(t, "feature/x", result.Branch)
	assert.Equal(t, "ls-remote-symref", result.Source)
	assert.Len(t, result.ProbeFailures, 1)
}
