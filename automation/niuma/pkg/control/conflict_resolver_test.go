package control

import (
	"context"
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

func TestTryResolveConflictByAIOnce_RollbackOnOutOfScopeChange(t *testing.T) {
	dir := setupGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg.go"), []byte("package main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644))
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

	err := ctrl.tryResolveConflictByAIOnce(context.Background(), dir, []string{"pkg.go"}, summaries)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of scope")

	readme, readErr := os.ReadFile(filepath.Join(dir, "README.md"))
	require.NoError(t, readErr)
	assert.Equal(t, "# test\n", string(readme))

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
