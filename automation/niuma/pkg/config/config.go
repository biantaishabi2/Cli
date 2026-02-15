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
	AI AIConfig `yaml:"ai"`
}

// AIConfig AI 相关配置
type AIConfig struct {
	Default   string                    `yaml:"default"`
	Providers map[string]ProviderConfig `yaml:"providers"`
}

// ProviderConfig 单个 AI Provider 配置
type ProviderConfig struct {
	Cmd string `yaml:"cmd"`
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
