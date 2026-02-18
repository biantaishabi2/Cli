package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGHWrapper_BlocksDirectBotLabelEdit_Local(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash wrapper test is not supported on windows")
	}

	output, err := runGHWrapper(t, []string{"issue", "edit", "325", "--add-label", "bot:fix"}, map[string]string{"CI": "false"})
	require.Error(t, err)
	assert.Equal(t, 1, exitCode(err))
	assert.Contains(t, string(output), "禁止直接通过 gh 修改 bot:* 状态标签")
}

func TestGHWrapper_BlocksDirectBotLabelEdit_CI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash wrapper test is not supported on windows")
	}

	output, err := runGHWrapper(t, []string{"issue", "edit", "325", "--remove-label=bot:fix"}, map[string]string{"CI": "true"})
	require.Error(t, err)
	assert.Equal(t, 2, exitCode(err))
	assert.Contains(t, string(output), "禁止直接通过 gh 修改 bot:* 状态标签")
}

func TestGHWrapper_BlocksDirectBotLabelEdit_WithGlobalRepoFlag(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash wrapper test is not supported on windows")
	}

	output, err := runGHWrapper(
		t,
		[]string{"-R", "owner/repo", "issue", "edit", "325", "--add-label", "bot:fix"},
		map[string]string{"CI": "true"},
	)
	require.Error(t, err)
	assert.Equal(t, 2, exitCode(err))
	assert.Contains(t, string(output), "禁止直接通过 gh 修改 bot:* 状态标签")
}

func TestGHWrapper_AllowsNonBotLabelEdit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash wrapper test is not supported on windows")
	}

	realGH := filepath.Join(t.TempDir(), "gh-real")
	err := os.WriteFile(realGH, []byte("#!/usr/bin/env bash\necho REAL_GH_OK \"$@\"\n"), 0o755)
	require.NoError(t, err)

	output, runErr := runGHWrapper(t, []string{"issue", "edit", "325", "--add-label", "bug"}, map[string]string{"GH_REAL_BIN": realGH})
	require.NoError(t, runErr)
	assert.Contains(t, string(output), "REAL_GH_OK issue edit 325 --add-label bug")
}

func runGHWrapper(t *testing.T, args []string, extraEnv map[string]string) ([]byte, error) {
	t.Helper()

	scriptPath := filepath.Join(".", "gh")
	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Env = filteredEnv("CI", "GH_REAL_BIN")
	for key, value := range extraEnv {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	return cmd.CombinedOutput()
}

func filteredEnv(skipKeys ...string) []string {
	if len(skipKeys) == 0 {
		return os.Environ()
	}
	skip := make(map[string]struct{}, len(skipKeys))
	for _, key := range skipKeys {
		skip[key] = struct{}{}
	}
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, item := range env {
		parts := splitEnv(item)
		if _, ok := skip[parts[0]]; ok {
			continue
		}
		out = append(out, item)
	}
	return out
}

func splitEnv(item string) [2]string {
	for i := 0; i < len(item); i++ {
		if item[i] == '=' {
			return [2]string{item[:i], item[i+1:]}
		}
	}
	return [2]string{item, ""}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
