package agent

import (
	"os"
	"os/exec"
	"path/filepath"
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
		{"environment.go", false},  // 不应误匹配 .env
		{"requirements.txt", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, isSensitiveFile(tt.path))
		})
	}
}
