//go:build !ci

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	content := `ai:
  default: kimi
  providers:
    kimi:
      cmd: "kimi --prompt {prompt}"
    codex:
      cmd: "codex exec {prompt}"
`
	err := os.WriteFile(filepath.Join(dir, ".niuma.yml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "kimi", cfg.AI.Default)
	assert.Len(t, cfg.AI.Providers, 2)
	assert.Equal(t, "kimi --prompt {prompt}", cfg.AI.Providers["kimi"].Cmd)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(t.TempDir())
	assert.Error(t, err)
}

func TestLoadWithDefaults_FileNotFound(t *testing.T) {
	cfg := LoadWithDefaults(t.TempDir())
	assert.Equal(t, "codex", cfg.AI.Default)
	assert.NotNil(t, cfg.AI.Providers)
}

func TestLoad_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	content := `ai:
  default: kimi
  providers: {}
`
	err := os.WriteFile(filepath.Join(dir, ".niuma.yml"), []byte(content), 0644)
	require.NoError(t, err)

	t.Setenv("NIUMA_AI_DEFAULT", "opencode")
	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "opencode", cfg.AI.Default)
}
