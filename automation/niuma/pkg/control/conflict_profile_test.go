package control

import (
	"errors"
	"strings"
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
