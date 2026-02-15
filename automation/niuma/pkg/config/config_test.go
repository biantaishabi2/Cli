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
      cmd_agent: "kimi --agent --prompt {prompt_file} --workdir {workdir}"
    codex:
      cmd: "codex exec {prompt}"
  discussion:
    providers: [kimi, codex]
    consolidator: kimi
  implementation:
    provider: kimi
`
	err := os.WriteFile(filepath.Join(dir, ".niuma.yml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "kimi", cfg.AI.Default)
	assert.Len(t, cfg.AI.Providers, 2)
	assert.Equal(t, "kimi --prompt {prompt}", cfg.AI.Providers["kimi"].Cmd)
	assert.Equal(t, "kimi --agent --prompt {prompt_file} --workdir {workdir}", cfg.AI.Providers["kimi"].CmdAgent)

	// 讨论配置
	assert.Equal(t, []string{"kimi", "codex"}, cfg.AI.Discussion.Providers)
	assert.Equal(t, "kimi", cfg.AI.Discussion.Consolidator)

	// 实现配置
	assert.Equal(t, "kimi", cfg.AI.Implementation.Provider)
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
