// pkg/config/config.go
// 配置加载：读取 .niuma.yml，支持环境变量覆盖
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 顶层配置
type Config struct {
	AI       AIConfig       `yaml:"ai"`
	Workflow WorkflowConfig `yaml:"workflow"`
}

// WorkflowConfig 工作流配置
type WorkflowConfig struct {
	RequirePlanApproval bool `yaml:"require_plan_approval"` // 方案定稿后是否需要人工审批
	MaxIterateRounds    int  `yaml:"max_iterate_rounds"`    // 最大自动迭代轮数（0=默认3）
}

// GetMaxIterateRounds 获取最大迭代轮数，默认3
func (w *WorkflowConfig) GetMaxIterateRounds() int {
	if w.MaxIterateRounds <= 0 {
		return 3
	}
	return w.MaxIterateRounds
}

// AIConfig AI 相关配置
type AIConfig struct {
	Default        string                    `yaml:"default"`
	Providers      map[string]ProviderConfig `yaml:"providers"`
	Discussion     DiscussionConfig          `yaml:"discussion"`
	Implementation ImplementationConfig      `yaml:"implementation"`
}

// ProviderConfig 单个 AI Provider 配置
type ProviderConfig struct {
	Cmd      string `yaml:"cmd"`       // 文本模式命令（只读，AI 返回文本）
	CmdAgent string `yaml:"cmd_agent"` // agentic 模式命令（AI 可读写文件）
}

// DiscussionConfig 讨论（左右互搏）配置
type DiscussionConfig struct {
	Providers    []string `yaml:"providers"`    // 参与互搏的 provider 列表
	Consolidator string   `yaml:"consolidator"` // 汇总用哪个 provider
}

// ImplementationConfig 实现配置
type ImplementationConfig struct {
	Provider string `yaml:"provider"` // 实现用哪个 provider
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
	return &cfg, nil
}

// LoadWithDefaults 加载配置，文件不存在时返回默认值
func LoadWithDefaults(dir string) *Config {
	cfg, err := Load(dir)
	if err != nil {
		return defaultConfig()
	}
	applyEnvOverrides(cfg)
	return cfg
}

func defaultConfig() *Config {
	return &Config{
		AI: AIConfig{
			Default:   "codex",
			Providers: map[string]ProviderConfig{},
		},
	}
}

// applyEnvOverrides 用环境变量覆盖配置
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("NIUMA_AI_DEFAULT"); v != "" {
		cfg.AI.Default = v
	}
}
