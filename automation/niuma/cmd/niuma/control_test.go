package main

import (
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractIntegratedIssueNumbers(t *testing.T) {
	messages := []string{
		"Merge feat/214-close (issue #214)",
		"Merge feat/215-parent (issue #215)\n\nextra",
		"chore: update docs",
		"Merge feat/214-close (issue #214)",
	}

	got := extractIntegratedIssueNumbers(messages)
	assert.Equal(t, []int{214, 215}, got)
}

func TestExtractIntegratedIssueNumbers_IgnoreNonIntegrationPattern(t *testing.T) {
	messages := []string{
		"Merge pull request #300 from foo/bar",
		"Closes #214",
		"fix: test (#215)",
	}

	got := extractIntegratedIssueNumbers(messages)
	assert.Empty(t, got)
}

func TestFormatDagSyncResult(t *testing.T) {
	lines := formatDagSyncResult("dag-sync", control.DagSyncResult{
		Status:        control.DagSyncStatusSuccess,
		Mode:          control.DagSyncModeManual,
		DagHash:       "abc",
		TotalEdges:    2,
		AppliedAdd:    1,
		AppliedRemove: 0,
		SkippedEdges:  1,
		ErrorType:     "",
	}, true)

	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "dag-sync 完成")
	assert.Contains(t, lines[0], "dry_run=true")
	assert.Contains(t, lines[0], "hash=abc")
}

func TestControlDagSyncCmd_DryRunFlagParsing(t *testing.T) {
	flagControlDagDryRun = false
	require.NoError(t, controlDagSyncCmd.ParseFlags([]string{"--dry-run"}))
	assert.True(t, flagControlDagDryRun)
}

func TestRunControlDagSync_RequiresRepo(t *testing.T) {
	oldRepo := flagRepo
	oldRepoDir := flagRepoDir
	oldWorkDir := flagWorkDir
	flagRepo = ""
	flagRepoDir = ""
	flagWorkDir = "."
	t.Cleanup(func() {
		flagRepo = oldRepo
		flagRepoDir = oldRepoDir
		flagWorkDir = oldWorkDir
	})

	err := runControlDagSync(controlDagSyncCmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "必须指定 --repo")
}
