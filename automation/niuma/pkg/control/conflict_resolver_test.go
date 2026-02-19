package control

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveGoImportConflictsInContent_MergeUnionAndSort(t *testing.T) {
	content := `package pkg

import (
<<<<<<< HEAD
	"os"
	_ "fmt"
=======
	"os"
	_ "strings"
>>>>>>> feature
)

func Env() string {
	return os.Getenv("X")
}
`

	resolved, changed, err := resolveGoImportConflictsInContent(content)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.NotContains(t, resolved, "<<<<<<<")
	assert.NotContains(t, resolved, "=======")
	assert.NotContains(t, resolved, ">>>>>>>")
	assert.Contains(t, resolved, "\t\"os\"")
	assert.Contains(t, resolved, "\t_ \"fmt\"")
	assert.Contains(t, resolved, "\t_ \"strings\"")
}

func TestResolveGoImportConflictFile_PreservesMode(t *testing.T) {
	dir := t.TempDir()
	relPath := "pkg.go"
	content := `package main

import (
<<<<<<< HEAD
	"fmt"
=======
	"os"
>>>>>>> feature
)
`
	absPath := filepath.Join(dir, relPath)
	require.NoError(t, os.WriteFile(absPath, []byte(content), 0o644))
	require.NoError(t, os.Chmod(absPath, 0o755))

	err := resolveGoImportConflictFile(dir, relPath)
	require.NoError(t, err)

	info, statErr := os.Stat(absPath)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o755), info.Mode().Perm())
}

func TestTryResolveConflictByAIOnce_RollbackOnOutOfScopeChange(t *testing.T) {
	dir := setupGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg.go"), []byte("package main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644))
	require.NoError(t, os.Chmod(filepath.Join(dir, "README.md"), 0o755))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add files")

	conflictContent := `package main

<<<<<<< HEAD
func Value() int { return 1 }
=======
func Value() int { return 2 }
>>>>>>> feature
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg.go"), []byte(conflictContent), 0o644))

	provider := ai.NewMockProvider(`{"edits":[{"path":"pkg.go","content":"package main\n\nfunc Value() int { return 1 }\n"},{"path":"README.md","content":"oops\n"}]}`)
	ctrl := &Controller{
		analyzer: NewDependencyAnalyzer(provider),
		cfg: &ControlConfig{
			RepoDir:                 dir,
			PRConflictEnableAI:      true,
			PRConflictAIMaxAttempts: 2,
		},
	}

	summaries := map[string]conflictFileSummary{
		"pkg.go": {
			hunks:  1,
			blocks: []conflictBlock{{ours: "func Value() int { return 1 }", theirs: "func Value() int { return 2 }"}},
		},
	}
	profileGroups, groupErr := ResolveConflictProfileGroups([]string{"pkg.go"})
	require.NoError(t, groupErr)

	err := ctrl.tryResolveConflictByAIOnce(context.Background(), dir, []string{"pkg.go"}, summaries, profileGroups)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of scope")

	readme, readErr := os.ReadFile(filepath.Join(dir, "README.md"))
	require.NoError(t, readErr)
	assert.Equal(t, "# test\n", string(readme))
	readmeInfo, modeErr := os.Stat(filepath.Join(dir, "README.md"))
	require.NoError(t, modeErr)
	assert.Equal(t, os.FileMode(0o755), readmeInfo.Mode().Perm())

	current, currentErr := os.ReadFile(filepath.Join(dir, "pkg.go"))
	require.NoError(t, currentErr)
	assert.Equal(t, conflictContent, string(current))
}

func TestGateChangedFileScope_RejectsUntrackedFiles(t *testing.T) {
	dir := setupGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg.go"), []byte("package main\n"), 0o644))
	runGit(t, dir, "add", "pkg.go")
	runGit(t, dir, "commit", "-m", "add pkg.go")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg.go"), []byte("package main\n\nfunc Value() int { return 1 }\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "unexpected.txt"), []byte("oops\n"), 0o644))

	ctrl := &Controller{
		cfg: &ControlConfig{RepoDir: dir},
	}
	err := ctrl.gateChangedFileScope(context.Background(), dir, []string{"pkg.go"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changed files out of scope")
	assert.Contains(t, err.Error(), "unexpected.txt")
}

func TestRunPRConflictGates_SmokeSideEffectsBlockedByScopeGate(t *testing.T) {
	dir := setupGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("resolved\n"), 0o644))
	runGit(t, dir, "add", "conflict.txt")
	runGit(t, dir, "commit", "-m", "add conflict file")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "conflict.txt"), []byte("resolved by gate\n"), 0o644))
	ctrl := &Controller{
		cfg: &ControlConfig{
			RepoDir:                dir,
			PRConflictSmokeTestCmd: "echo smoke > smoke-side-effect.txt",
		},
	}

	err := ctrl.runPRConflictGates(context.Background(), dir, []string{"conflict.txt"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changed files out of scope")
	assert.Contains(t, err.Error(), "smoke-side-effect.txt")
}

func TestTryResolveConflictByAIOnce_RollbackOnGoTestSideEffects(t *testing.T) {
	dir, conflictFile := setupPRConflictRepoWithFailingGoTestSideEffects(t)
	require.NoError(t, os.Chmod(filepath.Join(dir, conflictFile), 0o755))
	originalConflict, readErr := os.ReadFile(filepath.Join(dir, conflictFile))
	require.NoError(t, readErr)

	provider := ai.NewMockProvider(fmt.Sprintf(
		`{"edits":[{"path":"%s","content":"package pkg\n\nfunc helperValue() string {\n\treturn \"merged\"\n}\n"}]}`,
		conflictFile,
	))
	ctrl := &Controller{
		analyzer: NewDependencyAnalyzer(provider),
		cfg: &ControlConfig{
			RepoDir:                 dir,
			PRConflictEnableAI:      true,
			PRConflictAIMaxAttempts: 2,
		},
	}

	summaries := map[string]conflictFileSummary{
		conflictFile: {
			hunks:  1,
			blocks: []conflictBlock{{ours: "func helperValue() int { return 1 }", theirs: "func helperValue() string { return \"feature\" }"}},
		},
	}
	profileGroups, groupErr := ResolveConflictProfileGroups([]string{conflictFile})
	require.NoError(t, groupErr)

	err := ctrl.tryResolveConflictByAIOnce(context.Background(), dir, []string{conflictFile}, summaries, profileGroups)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "质量门禁失败")

	_, statErr := os.Stat(filepath.Join(dir, "pkg", "go-test-side-effect.txt"))
	assert.True(t, os.IsNotExist(statErr))

	currentConflict, currentErr := os.ReadFile(filepath.Join(dir, conflictFile))
	require.NoError(t, currentErr)
	assert.Equal(t, string(originalConflict), string(currentConflict))
	conflictInfo, modeErr := os.Stat(filepath.Join(dir, conflictFile))
	require.NoError(t, modeErr)
	assert.Equal(t, os.FileMode(0o755), conflictInfo.Mode().Perm())
}

func TestTryResolveConflictByAIOnce_MixedProfilesDispatchSeparately(t *testing.T) {
	dir := setupGitRepo(t)
	conflictGo := "pkg.go"
	conflictEx := "lib/app.ex"
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "lib"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, conflictGo), []byte(`package main

<<<<<<< HEAD
func Value() int { return 1 }
=======
func Value() int { return 2 }
>>>>>>> feature
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, conflictEx), []byte(`defmodule Demo do
<<<<<<< HEAD
  def value, do: 1
=======
  def value, do: 2
>>>>>>> feature
end
`), 0o644))

	provider := ai.NewMockProvider(
		fmt.Sprintf(`{"edits":[{"path":"%s","content":"defmodule Demo do\n  def value, do: 2\nend\n"}]}`, conflictEx),
		fmt.Sprintf(`{"edits":[{"path":"%s","content":"package main\n\nfunc Value() int { return 2 }\n"}]}`, conflictGo),
	)
	ctrl := &Controller{
		analyzer: NewDependencyAnalyzer(provider),
		cfg: &ControlConfig{
			RepoDir:                 dir,
			PRConflictEnableAI:      true,
			PRConflictAIMaxAttempts: 2,
		},
	}

	conflictFiles := []string{conflictGo, conflictEx}
	summaries := map[string]conflictFileSummary{
		conflictGo: {
			hunks:  1,
			blocks: []conflictBlock{{ours: "func Value() int { return 1 }", theirs: "func Value() int { return 2 }"}},
		},
		conflictEx: {
			hunks:  1,
			blocks: []conflictBlock{{ours: "def value, do: 1", theirs: "def value, do: 2"}},
		},
	}
	profileGroups, groupErr := ResolveConflictProfileGroups(conflictFiles)
	require.NoError(t, groupErr)

	err := ctrl.tryResolveConflictByAIOnce(context.Background(), dir, conflictFiles, summaries, profileGroups)
	require.NoError(t, err)

	assert.Equal(t, 2, provider.CallCount())
	calls := provider.Calls()
	require.Len(t, calls, 2)
	assert.Contains(t, calls[0].Prompt, "你是 elixir 冲突修复助手")
	assert.Contains(t, calls[0].Prompt, conflictEx)
	assert.NotContains(t, calls[0].Prompt, conflictGo)
	assert.Contains(t, calls[1].Prompt, "你是 go 冲突修复助手")
	assert.Contains(t, calls[1].Prompt, conflictGo)
	assert.NotContains(t, calls[1].Prompt, conflictEx)
}

func TestResolvePRConflictWithLayers_UnknownProfileEscalatesWithoutAICall(t *testing.T) {
	dir := setupGitRepo(t)
	conflictFile := "notes.txt"
	require.NoError(t, os.WriteFile(filepath.Join(dir, conflictFile), []byte("base\n"), 0o644))
	runGit(t, dir, "add", conflictFile)
	runGit(t, dir, "commit", "-m", "add notes")

	runGit(t, dir, "checkout", "-b", "feat/unknown-profile")
	require.NoError(t, os.WriteFile(filepath.Join(dir, conflictFile), []byte("from feature\n"), 0o644))
	runGit(t, dir, "add", conflictFile)
	runGit(t, dir, "commit", "-m", "feature notes")

	runGit(t, dir, "checkout", "master")
	require.NoError(t, os.WriteFile(filepath.Join(dir, conflictFile), []byte("from master\n"), 0o644))
	runGit(t, dir, "add", conflictFile)
	runGit(t, dir, "commit", "-m", "master notes")
	runGitWithExpectedFailure(t, dir, "merge", "--no-ff", "feat/unknown-profile")

	provider := ai.NewMockProvider(`{"edits":[{"path":"notes.txt","content":"resolved\n"}]}`)
	ctrl := &Controller{
		analyzer: NewDependencyAnalyzer(provider),
		cfg: &ControlConfig{
			RepoDir:                 dir,
			PRConflictEnableAI:      true,
			PRConflictAIMaxAttempts: 2,
		},
	}

	handled, err := ctrl.resolvePRConflictWithLayers(context.Background(), Task{}, PRReviewStatus{})
	require.NoError(t, err)
	assert.True(t, handled)
	assert.Equal(t, 0, provider.CallCount())
}

func TestPersistConflictResolutionMetadata_WritesAllFields(t *testing.T) {
	taskctlClient, logPath := newRecordingTaskCtlClient(t)
	ctrl := &Controller{taskctl: taskctlClient}
	task := Task{ID: "task-1", Metadata: map[string]string{"issue_num": "321"}}

	failedAt := time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC)
	err := ctrl.persistConflictResolutionMetadata(task, conflictResolutionLayerAI, 2, "gate failed", failedAt)
	require.NoError(t, err)

	raw, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	text := string(raw)
	assert.Contains(t, text, metaKeyConflictResolutionLayer)
	assert.Contains(t, text, conflictResolutionLayerAI)
	assert.Contains(t, text, metaKeyConflictResolutionAttempts)
	assert.Contains(t, text, "2")
	assert.Contains(t, text, metaKeyConflictResolutionLastError)
	assert.Contains(t, text, "gate failed")
	assert.Contains(t, text, metaKeyConflictResolutionLastFailedAt)
	assert.Contains(t, text, failedAt.Format(time.RFC3339))
}

func setupPRConflictRepoWithFailingGoTestSideEffects(t *testing.T) (string, string) {
	t.Helper()

	dir := setupGitRepo(t)
	conflictFile := filepath.ToSlash(filepath.Join("pkg", "helper_test.go"))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/prconflict\n\ngo 1.22\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, conflictFile), []byte(`package pkg

func helperValue() string {
	return "base"
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "helper_value_test.go"), []byte(`package pkg

import (
	"os"
	"testing"
)

func TestHelperValue(t *testing.T) {
	if err := os.WriteFile("go-test-side-effect.txt", []byte("side"), 0o644); err != nil {
		t.Fatalf("write side effect: %v", err)
	}
	t.Fatalf("quality gate failed")
}
`), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add helper conflict fixture")

	runGit(t, dir, "checkout", "-b", "feat/test-helper-conflict")
	require.NoError(t, os.WriteFile(filepath.Join(dir, conflictFile), []byte(`package pkg

func helperValue() string {
	return "feature"
}
`), 0o644))
	runGit(t, dir, "add", conflictFile)
	runGit(t, dir, "commit", "-m", "feature helper change")

	runGit(t, dir, "checkout", "master")
	require.NoError(t, os.WriteFile(filepath.Join(dir, conflictFile), []byte(`package pkg

func helperValue() int {
	return 1
}
`), 0o644))
	runGit(t, dir, "add", conflictFile)
	runGit(t, dir, "commit", "-m", "master helper change")

	runGitWithExpectedFailure(t, dir, "merge", "--no-ff", "feat/test-helper-conflict")
	return dir, conflictFile
}
