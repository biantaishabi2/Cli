package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithTemplates(t *testing.T) {
	// 创建临时配置文件
	tmpDir := t.TempDir()
	configContent := `
ai:
  default: claude
  
  templates:
    claude:
      type: claude
      cmd: "claude -p {prompt_file}"
    
    codex:
      type: openai
      base_url: "https://api.openai.com/v1"
      model: "gpt-4o"
  
  providers:
    my_claude:
      template: claude
    
    my_codex:
      template: codex
      model: "gpt-4o-mini"  # 覆盖模板
  
  implementation:
    provider: my_codex

workflow:
  max_iterate_rounds: 5
`
	configPath := filepath.Join(tmpDir, ".niuma.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	// 加载配置
	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证模板加载
	if len(cfg.AI.Templates) != 2 {
		t.Errorf("期望 2 个模板，实际 %d", len(cfg.AI.Templates))
	}

	// 验证 provider 加载
	if len(cfg.AI.Providers) != 2 {
		t.Errorf("期望 2 个 provider，实际 %d", len(cfg.AI.Providers))
	}

	// 测试 GetProvider - 继承模板的 provider
	provider, err := cfg.GetProvider("my_claude")
	if err != nil {
		t.Fatalf("获取 provider 失败: %v", err)
	}
	if provider.Type != "claude" {
		t.Errorf("期望 type=claude，实际 %s", provider.Type)
	}
	if provider.Cmd != "claude -p {prompt_file}" {
		t.Errorf("期望 cmd=claude -p {prompt_file}，实际 %s", provider.Cmd)
	}

	// 测试 GetProvider - 覆盖模板的 provider
	codexProvider, err := cfg.GetProvider("my_codex")
	if err != nil {
		t.Fatalf("获取 codex provider 失败: %v", err)
	}
	if codexProvider.Model != "gpt-4o-mini" {
		t.Errorf("期望 model=gpt-4o-mini（覆盖值），实际 %s", codexProvider.Model)
	}

	// 测试 GetImplementationProvider
	implProvider, err := cfg.GetImplementationProvider()
	if err != nil {
		t.Fatalf("获取实现 provider 失败: %v", err)
	}
	if implProvider.Model != "gpt-4o-mini" {
		t.Errorf("期望实现 provider model=gpt-4o-mini，实际 %s", implProvider.Model)
	}
}

func TestGetProvider_NotFound(t *testing.T) {
	cfg := &Config{
		AI: AIConfig{
			Providers: map[string]ProviderConfig{},
			Templates: map[string]TemplateConfig{},
		},
	}

	_, err := cfg.GetProvider("nonexistent")
	if err == nil {
		t.Error("期望返回错误，实际 nil")
	}
}

func TestGetProvider_TemplateNotFound(t *testing.T) {
	cfg := &Config{
		AI: AIConfig{
			Providers: map[string]ProviderConfig{
				"bad": {Template: "nonexistent"},
			},
			Templates: map[string]TemplateConfig{},
		},
	}

	_, err := cfg.GetProvider("bad")
	if err == nil {
		t.Error("期望返回模板未找到错误，实际 nil")
	}
}

func TestEnvOverride(t *testing.T) {
	os.Setenv("NIUMA_IMPLEMENTATION_PROVIDER", "env_provider")
	defer os.Unsetenv("NIUMA_IMPLEMENTATION_PROVIDER")

	tmpDir := t.TempDir()
	configContent := `
ai:
  default: claude
  implementation:
    provider: config_provider
`
	os.WriteFile(filepath.Join(tmpDir, ".niuma.yml"), []byte(configContent), 0644)

	cfg := LoadWithDefaults(tmpDir)
	if cfg.AI.Implementation.Provider != "env_provider" {
		t.Errorf("期望环境变量覆盖 provider=env_provider，实际 %s", cfg.AI.Implementation.Provider)
	}
}
