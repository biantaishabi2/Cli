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
