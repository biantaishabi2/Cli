package control

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileRegistry_DetectBySuffix(t *testing.T) {
	registry := defaultConflictProfileRegistry()

	assert.Equal(t, conflictProfileGo, registry.DetectByFile(t.TempDir(), "pkg/a.go"))
	assert.Equal(t, conflictProfileElixir, registry.DetectByFile(t.TempDir(), "lib/a.exs"))
	assert.Equal(t, conflictProfileRust, registry.DetectByFile(t.TempDir(), "src/lib.rs"))
	assert.Equal(t, conflictProfileUnknown, registry.DetectByFile(t.TempDir(), "docs/a.txt"))
}

func TestProfileRegistry_DetectByProjectMarkerFallback(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "nested", "doc"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644))

	registry := defaultConflictProfileRegistry()
	assert.Equal(t, conflictProfileGo, registry.DetectByFile(repoDir, filepath.ToSlash(filepath.Join("nested", "doc", "a.txt"))))
}

func TestGoProfileAllow_ThresholdBoundary(t *testing.T) {
	profile := goConflictProfile{}
	threshold := PRConflictThresholdConfig{MaxHunks: 3, MaxHunkLines: 15, MaxTotalLines: 50}

	summaryAllow := conflictFileSummary{
		hunks: 3,
		blocks: []conflictBlock{
			{ours: "\t\"fmt\"", theirs: "\t\"os\""},
			{ours: "\t\"net/http\"", theirs: "\t\"strings\""},
			{ours: "\t\"time\"", theirs: "\t\"context\""},
		},
	}
	allowed, reason := profile.Allow("pkg/a.go", summaryAllow, threshold)
	assert.True(t, allowed)
	assert.Contains(t, reason, "import")

	summaryRejectHunks := summaryAllow
	summaryRejectHunks.hunks = 4
	allowed, reason = profile.Allow("pkg/a.go", summaryRejectHunks, threshold)
	assert.False(t, allowed)
	assert.Contains(t, reason, "冲突块过多")

	summaryRejectHunkLines := conflictFileSummary{
		hunks: 1,
		blocks: []conflictBlock{
			{ours: strings.TrimSpace(buildProfileConflictLines(16)), theirs: "\t\"fmt\""},
		},
	}
	allowed, reason = profile.Allow("pkg/a.go", summaryRejectHunkLines, threshold)
	assert.False(t, allowed)
	assert.Contains(t, reason, "单块冲突行数过多")

	summaryRejectTotal := conflictFileSummary{
		hunks: 3,
		blocks: []conflictBlock{
			{ours: buildProfileConflictLines(9), theirs: buildProfileConflictLines(9)},
			{ours: buildProfileConflictLines(9), theirs: buildProfileConflictLines(9)},
			{ours: buildProfileConflictLines(8), theirs: buildProfileConflictLines(8)},
		},
	}
	allowed, reason = profile.Allow("pkg/a.go", summaryRejectTotal, threshold)
	assert.False(t, allowed)
	assert.Contains(t, reason, "冲突行数过多")
}

func TestBuildPRConflictAIPrompt_CommonAndProfileAddon(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "lib"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "lib", "a.exs"), []byte("ok"), 0o644))

	ctrl := &Controller{}
	group := conflictProfileGroup{
		Name:    conflictProfileElixir,
		Profile: elixirConflictProfile{},
		Files:   []string{"lib/a.exs"},
	}
	summaries := map[string]conflictFileSummary{
		"lib/a.exs": {
			hunks: 1,
			blocks: []conflictBlock{{
				ours:   "alias Foo.Bar",
				theirs: "alias Foo.Baz",
			}},
		},
	}

	prompt, err := ctrl.buildPRConflictAIPrompt(context.Background(), repoDir, group, summaries)
	require.NoError(t, err)
	assert.Contains(t, prompt, "Common 约束")
	assert.Contains(t, prompt, "输出 schema")
	assert.Contains(t, prompt, "[Profile: Elixir]")
	assert.Contains(t, prompt, "### file: lib/a.exs")
}

func TestProfileGateCommands_RouteByLanguage(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "pkg"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "lib"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "test"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "pkg", "a.go"), []byte("package pkg\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "mix.exs"), []byte("defmodule Demo.MixProject do\nend\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "test", "a_test.exs"), []byte("defmodule ATest do\nend\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "Cargo.toml"), []byte("[package]\nname = \"demo\"\nversion = \"0.1.0\"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "src", "lib.rs"), []byte("pub fn value() -> i32 { 1 }\n"), 0o644))

	goCmds, err := goConflictProfile{}.GateCommands(repoDir, []string{"pkg/a.go"}, PRConflictProfileConfig{})
	require.NoError(t, err)
	require.NotEmpty(t, goCmds)
	assert.Contains(t, goCmds[0].Command, "go test")

	elixirCmds, err := elixirConflictProfile{}.GateCommands(repoDir, []string{"test/a_test.exs"}, PRConflictProfileConfig{})
	require.NoError(t, err)
	require.NotEmpty(t, elixirCmds)
	assert.Contains(t, elixirCmds[0].Command, "mix test")

	rustCmds, err := rustConflictProfile{}.GateCommands(repoDir, []string{"src/lib.rs"}, PRConflictProfileConfig{GateCommand: "cargo test -p {pkg}"})
	require.NoError(t, err)
	require.NotEmpty(t, rustCmds)
	assert.Equal(t, "cargo test -p demo", rustCmds[0].Command)
}

func buildProfileConflictLines(lines int) string {
	var b strings.Builder
	for i := 0; i < lines; i++ {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("line_%d", i))
	}
	return b.String()
}
