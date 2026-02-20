// pkg/config/config.go
// 配置加载：读取 .niuma.yml，支持环境变量覆盖
// 支持模板化 provider 配置
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 顶层配置
type Config struct {
	AI         AIConfig         `yaml:"ai"`
	Workflow   WorkflowConfig   `yaml:"workflow"`
	Control    ControlConfig    `yaml:"control"`
	LabelGuard LabelGuardConfig `yaml:"label_guard"`
}

// LabelGuardConfig label-guard 配置
type LabelGuardConfig struct {
	Allowlist []string `yaml:"allowlist"` // 允许直接操作 bot: 标签的账号
	Mode      string   `yaml:"mode"`      // dry-run 或 enforce（默认 dry-run）
}

// ControlConfig 多 Issue 协调配置
type ControlConfig struct {
	TaskCtlBin              string        `yaml:"taskctl_bin"`               // taskctl 二进制路径（可选，自动查找）
	MergeStrategy           string        `yaml:"merge_strategy"`            // merge/squash，默认 merge
	IntegrationBranchPrefix string        `yaml:"integration_branch_prefix"` // 默认 integration/batch-
	MaxOldBranches          int           `yaml:"max_old_branches"`          // 保留旧 integration 分支数，默认 3
	MinPRsForIntegration    int           `yaml:"min_prs_for_integration"`   // 触发 integration 构建的最少 PR 数，默认 2
	DagSync                 DagSyncConfig `yaml:"dag_sync"`                  // DAG -> GitHub 展示层单向同步
}

// DagSyncConfig DAG 展示层同步配置
type DagSyncConfig struct {
	PollInterval         string   `yaml:"poll_interval"`          // 默认 5m
	MaxRetry             int      `yaml:"max_retry"`              // 默认 3
	RetryBackoff         []string `yaml:"retry_backoff"`          // 默认 [10s,30s,60s]
	RateLimitRPS         int      `yaml:"rate_limit_rps"`         // 默认 10
	Timeout              string   `yaml:"timeout"`                // 默认 30s
	SkippedEdgeThreshold int      `yaml:"skipped_edge_threshold"` // 默认 20（百分比）
}

var defaultDagSyncRetryBackoff = []string{"10s", "30s", "60s"}

// GetPollInterval 返回 DAG 同步巡检间隔，越界回退默认值。
func (c *DagSyncConfig) GetPollInterval() time.Duration {
	d, err := time.ParseDuration(c.PollInterval)
	if err != nil || d <= 0 {
		return 5 * time.Minute
	}
	return d
}

// GetMaxRetry 返回 DAG 同步最大重试次数，越界回退默认值。
func (c *DagSyncConfig) GetMaxRetry() int {
	if c.MaxRetry < 0 || c.MaxRetry > 10 {
		return 3
	}
	return c.MaxRetry
}

// GetRetryBackoff 返回 DAG 同步重试退避时长列表，越界回退默认值。
func (c *DagSyncConfig) GetRetryBackoff() []time.Duration {
	if len(c.RetryBackoff) == 0 {
		return []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second}
	}
	result := make([]time.Duration, 0, len(c.RetryBackoff))
	for _, item := range c.RetryBackoff {
		d, err := time.ParseDuration(item)
		if err != nil || d <= 0 {
			return []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second}
		}
		result = append(result, d)
	}
	return result
}

// GetRateLimitRPS 返回 DAG 同步 API 限速，越界回退默认值。
func (c *DagSyncConfig) GetRateLimitRPS() int {
	if c.RateLimitRPS <= 0 || c.RateLimitRPS > 100 {
		return 10
	}
	return c.RateLimitRPS
}

// GetTimeout 返回 DAG 同步单次请求超时，越界回退默认值。
func (c *DagSyncConfig) GetTimeout() time.Duration {
	d, err := time.ParseDuration(c.Timeout)
	if err != nil || d <= 0 {
		return 30 * time.Second
	}
	return d
}

// GetSkippedEdgeThresholdRatio 返回跳边告警阈值（0-1 浮点），越界回退默认值。
func (c *DagSyncConfig) GetSkippedEdgeThresholdRatio() float64 {
	if c.SkippedEdgeThreshold <= 0 || c.SkippedEdgeThreshold > 100 {
		return 0.20
	}
	return float64(c.SkippedEdgeThreshold) / 100
}

// WorkflowConfig 工作流配置
type WorkflowConfig struct {
	RequirePlanApproval   bool     `yaml:"require_plan_approval"`   // 方案定稿后是否需要人工审批
	MaxIterateRounds      int      `yaml:"max_iterate_rounds"`      // 最大自动迭代轮数（0=默认3）
	MaxDiscussionRounds   int      `yaml:"max_discussion_rounds"`   // discuss 单次 run 的最大讨论轮数（默认 5）
	DiscussTimeoutMinutes int      `yaml:"discuss_timeout_minutes"` // discuss 命令超时（分钟，默认 20）
	VisibleRoundInterval  int      `yaml:"visible_round_interval"`  // 可见评论节流：每 N 轮可见一次（默认 1）
	VisibleOnlyOnDiff     *bool    `yaml:"visible_only_on_diff"`    // 可见评论节流：仅差异变化时评论（默认 true）
	AllowedPrefixes        []string `yaml:"allowed_prefixes"`         // 允许修改的路径前缀
	ImplementTriggerLabels []string `yaml:"implement_trigger_labels"` // implement 流程的触发标签（默认 ["bot:plan-final"]）
}

// GetMaxIterateRounds 获取最大迭代轮数，默认3
func (w *WorkflowConfig) GetMaxIterateRounds() int {
	if w.MaxIterateRounds <= 0 {
		return 3
	}
	return w.MaxIterateRounds
}

// GetMaxDiscussionRounds 获取 discuss 最大轮数。
// 合法范围是 1-20，越界或未配置都回退到默认值 5。
func (w *WorkflowConfig) GetMaxDiscussionRounds() int {
	if w.MaxDiscussionRounds < 1 || w.MaxDiscussionRounds > 20 {
		return 5
	}
	return w.MaxDiscussionRounds
}

// GetDiscussTimeoutMinutes 获取 discuss 超时分钟数。
// 合法范围 1-120，越界或未配置都回退默认值 20。
func (w *WorkflowConfig) GetDiscussTimeoutMinutes() int {
	if w.DiscussTimeoutMinutes < 1 || w.DiscussTimeoutMinutes > 120 {
		return 20
	}
	return w.DiscussTimeoutMinutes
}

// GetVisibleRoundInterval 获取可见评论轮次间隔（默认 1）。
func (w *WorkflowConfig) GetVisibleRoundInterval() int {
	if w.VisibleRoundInterval <= 0 {
		return 1
	}
	return w.VisibleRoundInterval
}

// GetVisibleOnlyOnDiff 获取是否仅在差异变化时输出可见评论（默认 true）。
func (w *WorkflowConfig) GetVisibleOnlyOnDiff() bool {
	if w.VisibleOnlyOnDiff == nil {
		return true
	}
	return *w.VisibleOnlyOnDiff
}

// GetImplementTriggerLabels 获取 implement 触发标签列表（默认 ["bot:plan-final"]）。
func (w *WorkflowConfig) GetImplementTriggerLabels() []string {
	if len(w.ImplementTriggerLabels) == 0 {
		return []string{"bot:plan-final"}
	}
	return w.ImplementTriggerLabels
}

// GetLabelGuardMode 获取 label-guard 模式（默认 dry-run）。
func (c *LabelGuardConfig) GetMode() string {
	if c.Mode == "" {
		return "dry-run"
	}
	return c.Mode
}

// AIConfig AI 相关配置
type AIConfig struct {
	Default        string                    `yaml:"default"`
	Templates      map[string]TemplateConfig `yaml:"templates"`      // 预定义模板
	Providers      map[string]ProviderConfig `yaml:"providers"`      // 实际 provider 实例
	Discussion     DiscussionConfig          `yaml:"discussion"`     // 讨论配置
	Implementation ImplementationConfig      `yaml:"implementation"` // 实现配置
}

// TemplateConfig 模板配置
type TemplateConfig struct {
	Type    string `yaml:"type"`     // 模板类型: claude | openai | ollama
	Cmd     string `yaml:"cmd"`      // 文本模式命令（可选，用于 claude 等）
	BaseURL string `yaml:"base_url"` // API 基础 URL（用于 openai/ollama）
	Model   string `yaml:"model"`    // 模型名称
	APIKey  string `yaml:"api_key"`  // API Key（可选，优先从环境变量读取）
}

// ProviderConfig 单个 AI Provider 配置
type ProviderConfig struct {
	Template string `yaml:"template"`  // 引用的模板名称
	Cmd      string `yaml:"cmd"`       // 覆盖模板的 cmd（可选）
	CmdAgent string `yaml:"cmd_agent"` // 向后兼容：agentic 模式命令
	BaseURL  string `yaml:"base_url"`  // 覆盖模板的 base_url（可选）
	Model    string `yaml:"model"`     // 覆盖模板的 model（可选）
	APIKey   string `yaml:"api_key"`   // 覆盖模板的 api_key（可选）
}

// DiscussionConfig 讨论（左右互搏）配置
type DiscussionConfig struct {
	Providers []string `yaml:"providers"` // 参与互搏的 provider 列表
}

// ImplementationConfig 实现配置
type ImplementationConfig struct {
	Provider string `yaml:"provider"` // 实现用哪个 provider
}

// ResolvedProvider 解析后的 provider 配置（包含模板解析）
type ResolvedProvider struct {
	Type    string // claude | openai | ollama
	Cmd     string // 最终命令
	BaseURL string
	Model   string
	APIKey  string
}

// GetProvider 获取解析后的 provider 配置
// 支持模板继承和覆盖
func (c *Config) GetProvider(name string) (*ResolvedProvider, error) {
	providerCfg, ok := c.AI.Providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q 未找到", name)
	}

	// 如果有 template，先加载模板
	var templateCfg TemplateConfig
	if providerCfg.Template != "" {
		tmpl, ok := c.AI.Templates[providerCfg.Template]
		if !ok {
			return nil, fmt.Errorf("provider %q 引用的模板 %q 未找到", name, providerCfg.Template)
		}
		templateCfg = tmpl
	}

	// 合并配置：provider 覆盖 template
	rp := &ResolvedProvider{
		Type:    templateCfg.Type,
		Cmd:     firstNonEmpty(providerCfg.Cmd, templateCfg.Cmd),
		BaseURL: firstNonEmpty(providerCfg.BaseURL, templateCfg.BaseURL),
		Model:   firstNonEmpty(providerCfg.Model, templateCfg.Model),
		APIKey:  firstNonEmpty(providerCfg.APIKey, templateCfg.APIKey),
	}

	// 如果 APIKey 为空，尝试从环境变量读取
	if rp.APIKey == "" {
		rp.APIKey = c.getAPIKeyFromEnv(name, templateCfg.Type)
	}

	// 根据类型生成 Cmd（如果没有直接指定）
	if rp.Cmd == "" {
		cmd, err := c.generateCmd(rp)
		if err != nil {
			return nil, fmt.Errorf("为 provider %q 生成命令失败: %w", name, err)
		}
		rp.Cmd = cmd
	}

	return rp, nil
}

// ResolveDefaultPlaceholder 将 "default" 占位符或空字符串替换为 ai.default 的实际值
func (c *Config) ResolveDefaultPlaceholder(name string) string {
	if name == "" || name == "default" {
		return c.AI.Default
	}
	return name
}

// GetImplementationProvider 获取实现阶段的 provider
func (c *Config) GetImplementationProvider() (*ResolvedProvider, error) {
	name := c.ResolveDefaultPlaceholder(c.AI.Implementation.Provider)
	return c.GetProvider(name)
}

// GetDiscussionProviderNames 获取讨论阶段的 provider 名称列表，自动解析 "default" 占位符
func (c *Config) GetDiscussionProviderNames() []string {
	resolved := make([]string, len(c.AI.Discussion.Providers))
	for i, name := range c.AI.Discussion.Providers {
		resolved[i] = c.ResolveDefaultPlaceholder(name)
	}
	return resolved
}

// firstNonEmpty 返回第一个非空字符串
func firstNonEmpty(strs ...string) string {
	for _, s := range strs {
		if s != "" {
			return s
		}
	}
	return ""
}

// getAPIKeyFromEnv 从环境变量获取 API Key
func (c *Config) getAPIKeyFromEnv(providerName, providerType string) string {
	// 优先尝试 provider 特定的环境变量
	if key := os.Getenv(fmt.Sprintf("%s_API_KEY", providerName)); key != "" {
		return key
	}
	// 然后尝试类型特定的环境变量
	switch providerType {
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	case "openrouter":
		return os.Getenv("OPENROUTER_API_KEY")
	}
	return ""
}

// generateCmd 根据 provider 类型生成命令
func (c *Config) generateCmd(rp *ResolvedProvider) (string, error) {
	switch rp.Type {
	case "claude":
		return "cat {prompt_file} | CLAUDECODE= claude -p --output-format text --max-budget-usd 10 --permission-mode acceptEdits --add-dir {workdir}", nil
	case "openai":
		// 使用自定义的 openai 客户端命令
		return fmt.Sprintf("niuma-openai --base-url %s --model %s --api-key %s --workdir {workdir} --prompt {prompt_file}",
			rp.BaseURL, rp.Model, rp.APIKey), nil
	case "ollama":
		return fmt.Sprintf("cat {prompt_file} | ollama run %s", rp.Model), nil
	default:
		return "", fmt.Errorf("未知的 provider 类型: %s", rp.Type)
	}
}

// Load 从指定目录加载 .niuma.yml
func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, ".niuma.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	applyEnvOverrides(&cfg)
	applyConfigDefaults(&cfg)
	return &cfg, nil
}

// LoadWithDefaults 加载配置，文件不存在时返回默认值
func LoadWithDefaults(dir string) *Config {
	cfg, err := Load(dir)
	if err != nil {
		return defaultConfig()
	}
	applyEnvOverrides(cfg)
	applyConfigDefaults(cfg)
	return cfg
}

func defaultConfig() *Config {
	return &Config{
		AI: AIConfig{
			Default:   "claude",
			Providers: map[string]ProviderConfig{},
			Templates: map[string]TemplateConfig{},
		},
		Workflow: WorkflowConfig{
			MaxIterateRounds:      3,
			MaxDiscussionRounds:   5,
			DiscussTimeoutMinutes: 20,
			VisibleRoundInterval:  1,
			VisibleOnlyOnDiff:     boolPtr(true),
		},
		LabelGuard: LabelGuardConfig{
			Allowlist: []string{"github-actions[bot]", "niuma-bot"},
			Mode:      "dry-run",
		},
		Control: ControlConfig{
			DagSync: DagSyncConfig{
				PollInterval:         "5m",
				MaxRetry:             3,
				RetryBackoff:         append([]string(nil), defaultDagSyncRetryBackoff...),
				RateLimitRPS:         10,
				Timeout:              "30s",
				SkippedEdgeThreshold: 20,
			},
		},
	}
}

func applyConfigDefaults(cfg *Config) {
	if cfg.Control.DagSync.PollInterval == "" {
		cfg.Control.DagSync.PollInterval = "5m"
	}
	if cfg.Control.DagSync.MaxRetry == 0 {
		cfg.Control.DagSync.MaxRetry = 3
	}
	if len(cfg.Control.DagSync.RetryBackoff) == 0 {
		cfg.Control.DagSync.RetryBackoff = append([]string(nil), defaultDagSyncRetryBackoff...)
	}
	if cfg.Control.DagSync.RateLimitRPS == 0 {
		cfg.Control.DagSync.RateLimitRPS = 10
	}
	if cfg.Control.DagSync.Timeout == "" {
		cfg.Control.DagSync.Timeout = "30s"
	}
	if cfg.Control.DagSync.SkippedEdgeThreshold == 0 {
		cfg.Control.DagSync.SkippedEdgeThreshold = 20
	}

	// 越界值统一回退，避免配置写错导致运行时异常。
	if d, err := time.ParseDuration(cfg.Control.DagSync.PollInterval); err != nil || d <= 0 {
		cfg.Control.DagSync.PollInterval = "5m"
	}
	if cfg.Control.DagSync.MaxRetry < 0 || cfg.Control.DagSync.MaxRetry > 10 {
		cfg.Control.DagSync.MaxRetry = 3
	}
	if cfg.Control.DagSync.RateLimitRPS <= 0 || cfg.Control.DagSync.RateLimitRPS > 100 {
		cfg.Control.DagSync.RateLimitRPS = 10
	}
	if d, err := time.ParseDuration(cfg.Control.DagSync.Timeout); err != nil || d <= 0 {
		cfg.Control.DagSync.Timeout = "30s"
	}
	if cfg.Control.DagSync.SkippedEdgeThreshold <= 0 || cfg.Control.DagSync.SkippedEdgeThreshold > 100 {
		cfg.Control.DagSync.SkippedEdgeThreshold = 20
	}

	// LabelGuard 默认值：配置文件存在但缺少 label_guard 段时回退
	if len(cfg.LabelGuard.Allowlist) == 0 {
		cfg.LabelGuard.Allowlist = []string{"github-actions[bot]", "niuma-bot"}
	}
	if cfg.LabelGuard.Mode == "" {
		cfg.LabelGuard.Mode = "dry-run"
	}

	if len(cfg.Control.DagSync.RetryBackoff) == 0 {
		cfg.Control.DagSync.RetryBackoff = append([]string(nil), defaultDagSyncRetryBackoff...)
	} else {
		valid := true
		for _, item := range cfg.Control.DagSync.RetryBackoff {
			d, err := time.ParseDuration(item)
			if err != nil || d <= 0 {
				valid = false
				break
			}
		}
		if !valid {
			cfg.Control.DagSync.RetryBackoff = append([]string(nil), defaultDagSyncRetryBackoff...)
		}
	}
}

// applyEnvOverrides 用环境变量覆盖配置
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("NIUMA_AI_DEFAULT"); v != "" {
		cfg.AI.Default = v
	}
	// 支持用环境变量切换实现 provider
	if v := os.Getenv("NIUMA_IMPLEMENTATION_PROVIDER"); v != "" {
		cfg.AI.Implementation.Provider = v
	}
	if v := os.Getenv("NIUMA_VISIBLE_ROUND_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Workflow.VisibleRoundInterval = n
		}
	}
	if v := os.Getenv("NIUMA_VISIBLE_ONLY_ON_DIFF"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Workflow.VisibleOnlyOnDiff = boolPtr(b)
		}
	}
}

func boolPtr(v bool) *bool {
	return &v
}
