//go:build !ci

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

	// 实现配置
	assert.Equal(t, "kimi", cfg.AI.Implementation.Provider)
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load(t.TempDir())
	assert.Error(t, err)
}

func TestLoadWithDefaults_FileNotFound(t *testing.T) {
	cfg := LoadWithDefaults(t.TempDir())
	assert.Equal(t, "claude", cfg.AI.Default)
	assert.NotNil(t, cfg.AI.Providers)
	assert.Equal(t, 5, cfg.Workflow.GetMaxDiscussionRounds())
	assert.Equal(t, 20, cfg.Workflow.GetDiscussTimeoutMinutes())
	assert.Equal(t, 1, cfg.Workflow.GetVisibleRoundInterval())
	assert.True(t, cfg.Workflow.GetVisibleOnlyOnDiff())
	assert.Equal(t, 5*time.Minute, cfg.Control.DagSync.GetPollInterval())
	assert.Equal(t, 3, cfg.Control.DagSync.GetMaxRetry())
	assert.Equal(t, []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second}, cfg.Control.DagSync.GetRetryBackoff())
	assert.Equal(t, 10, cfg.Control.DagSync.GetRateLimitRPS())
	assert.Equal(t, 30*time.Second, cfg.Control.DagSync.GetTimeout())
	assert.Equal(t, 0.20, cfg.Control.DagSync.GetSkippedEdgeThresholdRatio())
}

func TestLoad_WorkflowConfig(t *testing.T) {
	dir := t.TempDir()
	content := `ai:
  default: claude
  providers:
    claude:
      cmd: "claude -p {prompt_file}"
workflow:
  require_plan_approval: true
  max_iterate_rounds: 5
  max_discussion_rounds: 7
  discuss_timeout_minutes: 25
  visible_round_interval: 2
  visible_only_on_diff: false
`
	err := os.WriteFile(filepath.Join(dir, ".niuma.yml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.True(t, cfg.Workflow.RequirePlanApproval)
	assert.Equal(t, 5, cfg.Workflow.MaxIterateRounds)
	assert.Equal(t, 5, cfg.Workflow.GetMaxIterateRounds())
	assert.Equal(t, 7, cfg.Workflow.MaxDiscussionRounds)
	assert.Equal(t, 7, cfg.Workflow.GetMaxDiscussionRounds())
	assert.Equal(t, 25, cfg.Workflow.GetDiscussTimeoutMinutes())
	assert.Equal(t, 2, cfg.Workflow.GetVisibleRoundInterval())
	assert.False(t, cfg.Workflow.GetVisibleOnlyOnDiff())
}

func TestWorkflowConfig_DefaultMaxRounds(t *testing.T) {
	w := &WorkflowConfig{}
	assert.Equal(t, 3, w.GetMaxIterateRounds())
	assert.Equal(t, 5, w.GetMaxDiscussionRounds())

	w.MaxIterateRounds = -1
	assert.Equal(t, 3, w.GetMaxIterateRounds())

	w.MaxDiscussionRounds = 21
	assert.Equal(t, 5, w.GetMaxDiscussionRounds())

	w.MaxDiscussionRounds = -1
	assert.Equal(t, 5, w.GetMaxDiscussionRounds())

	assert.Equal(t, 20, w.GetDiscussTimeoutMinutes())
	w.DiscussTimeoutMinutes = 121
	assert.Equal(t, 20, w.GetDiscussTimeoutMinutes())
	w.DiscussTimeoutMinutes = -1
	assert.Equal(t, 20, w.GetDiscussTimeoutMinutes())
	w.DiscussTimeoutMinutes = 30
	assert.Equal(t, 30, w.GetDiscussTimeoutMinutes())

	assert.Equal(t, 1, w.GetVisibleRoundInterval())
	w.VisibleRoundInterval = 3
	assert.Equal(t, 3, w.GetVisibleRoundInterval())

	assert.True(t, w.GetVisibleOnlyOnDiff())
	v := false
	w.VisibleOnlyOnDiff = &v
	assert.False(t, w.GetVisibleOnlyOnDiff())
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
	t.Setenv("NIUMA_VISIBLE_ROUND_INTERVAL", "4")
	t.Setenv("NIUMA_VISIBLE_ONLY_ON_DIFF", "false")
	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "opencode", cfg.AI.Default)
	assert.Equal(t, 4, cfg.Workflow.GetVisibleRoundInterval())
	assert.False(t, cfg.Workflow.GetVisibleOnlyOnDiff())
}

func TestResolveDefaultPlaceholder(t *testing.T) {
	cfg := &Config{AI: AIConfig{Default: "claude"}}

	// "default" 占位符替换为 ai.default
	assert.Equal(t, "claude", cfg.ResolveDefaultPlaceholder("default"))
	// 空字符串也 fallback 到 ai.default
	assert.Equal(t, "claude", cfg.ResolveDefaultPlaceholder(""))
	// 具体 provider 名不受影响
	assert.Equal(t, "kimi", cfg.ResolveDefaultPlaceholder("kimi"))
}

func TestGetDiscussionProviderNames_DefaultPlaceholder(t *testing.T) {
	cfg := &Config{
		AI: AIConfig{
			Default: "claude",
			Discussion: DiscussionConfig{
				Providers: []string{"default", "kimi"},
			},
		},
	}

	names := cfg.GetDiscussionProviderNames()
	assert.Equal(t, []string{"claude", "kimi"}, names)
}

func TestGetDiscussionProviderNames_NoPlaceholder(t *testing.T) {
	cfg := &Config{
		AI: AIConfig{
			Default: "claude",
			Discussion: DiscussionConfig{
				Providers: []string{"codex", "kimi"},
			},
		},
	}

	// 直接写 provider 名字仍然有效
	names := cfg.GetDiscussionProviderNames()
	assert.Equal(t, []string{"codex", "kimi"}, names)
}

func TestGetImplementationProvider_DefaultPlaceholder(t *testing.T) {
	cfg := &Config{
		AI: AIConfig{
			Default: "claude",
			Providers: map[string]ProviderConfig{
				"claude": {Cmd: "claude -p {prompt_file}"},
			},
			Implementation: ImplementationConfig{Provider: "default"},
		},
	}

	rp, err := cfg.GetImplementationProvider()
	require.NoError(t, err)
	assert.Equal(t, "claude -p {prompt_file}", rp.Cmd)
}

func TestEnvOverride_AffectsDefaultPlaceholder(t *testing.T) {
	dir := t.TempDir()
	content := `ai:
  default: claude
  providers:
    claude:
      cmd: "claude -p {prompt_file}"
    opencode:
      cmd: "opencode {prompt_file}"
  discussion:
    providers: [default]
  implementation:
    provider: default
`
	err := os.WriteFile(filepath.Join(dir, ".niuma.yml"), []byte(content), 0644)
	require.NoError(t, err)

	t.Setenv("NIUMA_AI_DEFAULT", "opencode")
	cfg, err := Load(dir)
	require.NoError(t, err)

	// 环境变量覆盖后，default 占位符跟随变化
	names := cfg.GetDiscussionProviderNames()
	assert.Equal(t, []string{"opencode"}, names)

	rp, err := cfg.GetImplementationProvider()
	require.NoError(t, err)
	assert.Equal(t, "opencode {prompt_file}", rp.Cmd)
}

func TestLoad_ControlDagSyncConfig(t *testing.T) {
	dir := t.TempDir()
	content := `ai:
  default: codex
  providers: {}
control:
  dag_sync:
    poll_interval: 7m
    max_retry: 2
    retry_backoff: [5s, 15s]
    rate_limit_rps: 6
    timeout: 45s
    skipped_edge_threshold: 35
`
	err := os.WriteFile(filepath.Join(dir, ".niuma.yml"), []byte(content), 0o644)
	require.NoError(t, err)

	cfg, err := Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 7*time.Minute, cfg.Control.DagSync.GetPollInterval())
	assert.Equal(t, 2, cfg.Control.DagSync.GetMaxRetry())
	assert.Equal(t, []time.Duration{5 * time.Second, 15 * time.Second}, cfg.Control.DagSync.GetRetryBackoff())
	assert.Equal(t, 6, cfg.Control.DagSync.GetRateLimitRPS())
	assert.Equal(t, 45*time.Second, cfg.Control.DagSync.GetTimeout())
	assert.Equal(t, 0.35, cfg.Control.DagSync.GetSkippedEdgeThresholdRatio())
}

func TestLoadWithDefaults_MissingLabelGuardSection_AppliesDefaults(t *testing.T) {
	// 配置文件存在但缺少 label_guard 段时，应回退到默认值
	dir := t.TempDir()
	content := `ai:
  default: claude
  providers:
    claude:
      cmd: "claude -p {prompt_file}"
`
	err := os.WriteFile(filepath.Join(dir, ".niuma.yml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg := LoadWithDefaults(dir)
	assert.Equal(t, []string{"github-actions[bot]", "niuma-bot"}, cfg.LabelGuard.Allowlist)
	assert.Equal(t, "dry-run", cfg.LabelGuard.Mode)
}

func TestDagSyncConfig_InvalidValueFallback(t *testing.T) {
	cfg := DagSyncConfig{
		PollInterval:         "bad",
		MaxRetry:             -1,
		RetryBackoff:         []string{"-1s"},
		RateLimitRPS:         0,
		Timeout:              "0s",
		SkippedEdgeThreshold: 101,
	}

	assert.Equal(t, 5*time.Minute, cfg.GetPollInterval())
	assert.Equal(t, 3, cfg.GetMaxRetry())
	assert.Equal(t, []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second}, cfg.GetRetryBackoff())
	assert.Equal(t, 10, cfg.GetRateLimitRPS())
	assert.Equal(t, 30*time.Second, cfg.GetTimeout())
	assert.Equal(t, 0.20, cfg.GetSkippedEdgeThresholdRatio())
}
