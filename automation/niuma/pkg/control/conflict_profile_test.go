package control

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveConflictProfile_BySuffix(t *testing.T) {
	cases := map[string]string{
		"pkg/main.go":      "go",
		"lib/app.ex":       "elixir",
		"lib/test_app.exs": "elixir",
		"src/main.rs":      "rust",
	}

	for path, expected := range cases {
		profile, err := ResolveConflictProfile(path)
		require.NoError(t, err)
		assert.Equal(t, expected, profile.Name())
	}
}

func TestResolveConflictProfileGroups_UnknownSuffixRejected(t *testing.T) {
	_, err := ResolveConflictProfileGroups([]string{"pkg/main.go", "README.md"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnknownProfile))
}

func TestAssemblePrompt_CommonThenProfile(t *testing.T) {
	common := buildCommonConflictPrompt([]string{"b.go", "a.go"})
	profile := "PROFILE_PROMPT"
	combined := assemblePrompt(common, profile)

	assert.Contains(t, combined, "通用约束")
	assert.Contains(t, combined, "允许修改文件")
	assert.True(t, strings.Index(combined, "- a.go") < strings.Index(combined, "- b.go"))
	assert.True(t, strings.Index(combined, "通用约束") < strings.Index(combined, profile))
}

func TestAssemblePrompt_EmptyBranches(t *testing.T) {
	assert.Equal(t, "PROFILE_PROMPT", assemblePrompt(" ", "PROFILE_PROMPT"))
	assert.Equal(t, "COMMON_PROMPT", assemblePrompt("COMMON_PROMPT", " "))
	assert.Equal(t, "", assemblePrompt(" ", " "))
}

func TestResolveConflictProfileGroups_EmptyInput(t *testing.T) {
	groups, err := ResolveConflictProfileGroups(nil)
	require.NoError(t, err)
	assert.Empty(t, groups)
}

func TestResolveConflictProfileGroups_ConcurrentSafe(t *testing.T) {
	files := []string{
		"pkg/service.go",
		"apps/web/lib/web.ex",
		"apps/web/test/web_test.exs",
		"crates/core/src/lib.rs",
	}
	const workers = 32

	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			groups, err := ResolveConflictProfileGroups(files)
			if err != nil {
				errCh <- err
				return
			}
			if len(groups) != 3 {
				errCh <- fmt.Errorf("unexpected group count: %d", len(groups))
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
}

func TestIsElixirLightweightConflict(t *testing.T) {
	cases := []struct {
		name     string
		file     string
		summary  ConflictFileSummary
		expected bool
	}{
		{
			name: "alias/import/use 轻度冲突命中",
			file: "lib/my_app.ex",
			summary: ConflictFileSummary{
				Hunks: 2,
				Blocks: []ConflictBlock{
					{Ours: "alias MyApp.Repo", Theirs: "alias MyApp.Schema"},
					{Ours: "import Ecto.Query", Theirs: "import Ecto.Changeset"},
				},
			},
			expected: true,
		},
		{
			name: "use 语句命中",
			file: "lib/web.ex",
			summary: ConflictFileSummary{
				Hunks: 1,
				Blocks: []ConflictBlock{
					{Ours: "use Phoenix.Controller", Theirs: "use Phoenix.LiveView"},
				},
			},
			expected: true,
		},
		{
			name: ".exs 文件命中",
			file: "test/my_test.exs",
			summary: ConflictFileSummary{
				Hunks: 1,
				Blocks: []ConflictBlock{
					{Ours: "alias MyApp.Factory", Theirs: "alias MyApp.TestHelper"},
				},
			},
			expected: true,
		},
		{
			name: "非 Elixir 文件不命中",
			file: "pkg/main.go",
			summary: ConflictFileSummary{
				Hunks: 1,
				Blocks: []ConflictBlock{
					{Ours: "alias Foo", Theirs: "alias Bar"},
				},
			},
			expected: false,
		},
		{
			name: "包含 def 等非 alias/import/use 语句不命中",
			file: "lib/app.ex",
			summary: ConflictFileSummary{
				Hunks: 1,
				Blocks: []ConflictBlock{
					{Ours: "def value, do: 1", Theirs: "def value, do: 2"},
				},
			},
			expected: false,
		},
		{
			name: "hunks 超过阈值不命中",
			file: "lib/app.ex",
			summary: ConflictFileSummary{
				Hunks: 4,
				Blocks: []ConflictBlock{
					{Ours: "alias A", Theirs: "alias B"},
					{Ours: "alias C", Theirs: "alias D"},
					{Ours: "alias E", Theirs: "alias F"},
					{Ours: "alias G", Theirs: "alias H"},
				},
			},
			expected: false,
		},
		{
			name: "单侧行数过多不命中",
			file: "lib/app.ex",
			summary: ConflictFileSummary{
				Hunks: 1,
				Blocks: []ConflictBlock{
					{
						Ours:   "alias A\nalias B\nalias C\nalias D\nalias E\nalias F\nalias G\nalias H\nalias I\nalias J\nalias K\nalias L\nalias M",
						Theirs: "alias Z",
					},
				},
			},
			expected: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := isElixirLightweightConflict(tc.file, tc.summary)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestResolveConflictProfileGroups_MixedLanguagesStableGrouping(t *testing.T) {
	groups, err := ResolveConflictProfileGroups([]string{
		"pkg/service.go",
		"apps/web/lib/web.ex",
		"apps/web/test/web_test.exs",
		"crates/core/src/lib.rs",
	})
	require.NoError(t, err)
	require.Len(t, groups, 3)

	assert.Equal(t, "elixir", groups[0].Profile.Name())
	assert.Equal(t, []string{"apps/web/lib/web.ex", "apps/web/test/web_test.exs"}, groups[0].Files)

	assert.Equal(t, "go", groups[1].Profile.Name())
	assert.Equal(t, []string{"pkg/service.go"}, groups[1].Files)

	assert.Equal(t, "rust", groups[2].Profile.Name())
	assert.Equal(t, []string{"crates/core/src/lib.rs"}, groups[2].Files)
}
