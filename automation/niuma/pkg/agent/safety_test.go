//go:build !ci

package agent

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsHighRiskChange(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"auth/login.go", true},
		{".github/workflows/ci.yml", true},
		{"Dockerfile", true},
		{"deploy/scripts/run.sh", true},
		{"infra/terraform/main.tf", true},
		{"docker-compose.yml", true},
		{"src/auth.go", false},
		{"lib/utils.go", false},
		{"tests/test.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsHighRiskChange(tt.path))
		})
	}
}

func TestFormatHighRiskWarning(t *testing.T) {
	paths := []string{".github/workflows/ci.yml", "Dockerfile"}
	warning := FormatHighRiskWarning(paths)
	assert.Contains(t, warning, "高风险路径警告")
	assert.Contains(t, warning, ".github/workflows/ci.yml")
	assert.Contains(t, warning, "Dockerfile")
}

func TestFormatHighRiskWarning_Empty(t *testing.T) {
	assert.Empty(t, FormatHighRiskWarning(nil))
}

func TestCheckDiff_Match(t *testing.T) {
	plan := []FileChange{
		{Path: "a.go", Action: "modify"},
		{Path: "b.go", Action: "create"},
	}
	diff := []string{"a.go", "b.go"}

	result := CheckDiffAgainstPlan(diff, plan)
	assert.True(t, result.IsClean())
}

func TestCheckDiff_Extra(t *testing.T) {
	plan := []FileChange{
		{Path: "a.go", Action: "modify"},
	}
	diff := []string{"a.go", "c.go"}

	result := CheckDiffAgainstPlan(diff, plan)
	assert.False(t, result.IsClean())
	assert.Equal(t, []string{"c.go"}, result.Extra)
	assert.Empty(t, result.Missing)
}

func TestCheckDiff_Missing(t *testing.T) {
	plan := []FileChange{
		{Path: "a.go", Action: "modify"},
		{Path: "b.go", Action: "modify"},
	}
	diff := []string{"a.go"}

	result := CheckDiffAgainstPlan(diff, plan)
	assert.False(t, result.IsClean())
	assert.Empty(t, result.Extra)
	assert.Equal(t, []string{"b.go"}, result.Missing)
}

func TestCheckDiff_Both(t *testing.T) {
	plan := []FileChange{
		{Path: "a.go", Action: "modify"},
		{Path: "b.go", Action: "modify"},
	}
	diff := []string{"a.go", "c.go"}

	result := CheckDiffAgainstPlan(diff, plan)
	assert.False(t, result.IsClean())
	assert.Equal(t, []string{"c.go"}, result.Extra)
	assert.Equal(t, []string{"b.go"}, result.Missing)
}

func TestCheckDiff_Empty(t *testing.T) {
	result := CheckDiffAgainstPlan(nil, nil)
	assert.True(t, result.IsClean())
}

func TestCheckDiff_EmptyPlan(t *testing.T) {
	// 空计划跳过检查，即使有实际改动也不报错
	result := CheckDiffAgainstPlan([]string{"a.go"}, nil)
	assert.True(t, result.IsClean())
}

func TestCheckDiff_PathNormalize(t *testing.T) {
	plan := []FileChange{
		{Path: "./src/a.go", Action: "modify"},
	}
	diff := []string{"src/a.go"}

	result := CheckDiffAgainstPlan(diff, plan)
	assert.True(t, result.IsClean())
}

func TestFormatDiffCheckComment(t *testing.T) {
	result := &DiffCheckResult{
		Extra:   []string{"c.go"},
		Missing: []string{"b.go"},
	}
	comment := FormatDiffCheckComment(result)
	assert.Contains(t, comment, "计划 vs 实际 diff 对比")
	assert.Contains(t, comment, "c.go")
	assert.Contains(t, comment, "不在计划内")
	assert.Contains(t, comment, "b.go")
	assert.Contains(t, comment, "计划中但未修改")
}

func TestFormatDiffCheckComment_Clean(t *testing.T) {
	result := &DiffCheckResult{}
	assert.Empty(t, FormatDiffCheckComment(result))
}

func TestParseFileChangesFromComment(t *testing.T) {
	body := `## 最终方案

一些内容

<!-- PLAN_FILES:[{"path":"src/main.go","action":"modify","description":"修改入口"},{"path":"pkg/util.go","action":"create","description":"新增工具"}] -->`

	changes := ParseFileChangesFromComment(body)
	assert.Len(t, changes, 2)
	assert.Equal(t, "src/main.go", changes[0].Path)
	assert.Equal(t, "modify", changes[0].Action)
	assert.Equal(t, "pkg/util.go", changes[1].Path)
	assert.Equal(t, "create", changes[1].Action)
}

func TestParseFileChangesFromComment_None(t *testing.T) {
	body := "## 最终方案\n\n一些内容，没有 PLAN_FILES 标记"
	changes := ParseFileChangesFromComment(body)
	assert.Nil(t, changes)
}

func TestCheckDiff_PathTraversal(t *testing.T) {
	// 计划中包含 .. 路径遍历的条目会被忽略（不加入白名单）
	plan := []FileChange{
		{Path: "src/main.go", Action: "modify"},
		{Path: "src/../etc/passwd", Action: "modify"}, // 路径遍历，应被忽略
	}
	diff := []string{"src/main.go"}

	result := CheckDiffAgainstPlan(diff, plan)
	assert.True(t, result.IsClean()) // etc/passwd 不在白名单也不算 Missing
}

func TestContainsPathTraversal(t *testing.T) {
	assert.True(t, ContainsPathTraversal("src/../etc/passwd"))
	assert.True(t, ContainsPathTraversal("../../etc/shadow"))
	assert.False(t, ContainsPathTraversal("src/main.go"))
	assert.False(t, ContainsPathTraversal("pkg/agent/safety.go"))
}

func TestDefaultBranch_Fallback(t *testing.T) {
	requireLocalPushSupport(t)

	repoDir := initTestRepo(t)
	git := gitBin()
	remoteDir := filepath.Join(t.TempDir(), "remote.git")

	// main-only 远端 + 本地无 origin/HEAD，应优先解析到 main。
	initBareRemote(t, remoteDir)
	cmd := exec.Command(git, "remote", "add", "origin", asFileRemote(remoteDir))
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "checkout", "-b", "main")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "push", "-u", "origin", "main")
	cmd.Dir = repoDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command(git, "update-ref", "-d", "refs/remotes/origin/HEAD")
	cmd.Dir = repoDir
	_ = cmd.Run()

	g := NewGitOps(repoDir)
	branch := g.DefaultBranch()
	assert.Equal(t, "main", branch)
}
