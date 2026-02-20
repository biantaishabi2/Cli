package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/config"
)

func TestGateLabelDefaultConfig(t *testing.T) {
	cfg := &config.Config{}
	triggerLabels := cfg.Workflow.GetImplementTriggerLabels()

	// bot:plan-final 应通过
	found := false
	for _, l := range triggerLabels {
		if l == "bot:plan-final" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("default trigger labels should contain bot:plan-final, got %v", triggerLabels)
	}

	// bot:plan-approved 应不通过
	found = false
	for _, l := range triggerLabels {
		if l == "bot:plan-approved" {
			found = true
			break
		}
	}
	if found {
		t.Errorf("default trigger labels should NOT contain bot:plan-approved, got %v", triggerLabels)
	}
}

func TestGateLabelCustomConfig(t *testing.T) {
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			ImplementTriggerLabels: []string{"bot:plan-final", "bot:plan-approved"},
		},
	}
	triggerLabels := cfg.Workflow.GetImplementTriggerLabels()

	for _, want := range []string{"bot:plan-final", "bot:plan-approved"} {
		found := false
		for _, l := range triggerLabels {
			if l == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("custom trigger labels should contain %s, got %v", want, triggerLabels)
		}
	}
}

// writeNiumaYml 在指定目录写入 .niuma.yml 配置
func writeNiumaYml(t *testing.T, dir, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, ".niuma.yml"), []byte(content), 0o644)
	if err != nil {
		t.Fatalf("write .niuma.yml: %v", err)
	}
}

// runGateLabelWithFlags 通过设置全局 flag 变量调用真实 runGateLabel
func runGateLabelWithFlags(t *testing.T, repoDir, label, gateType string) error {
	t.Helper()
	// 保存并恢复全局状态
	oldRepoDir := flagRepoDir
	oldLabel := flagGateLabelLabel
	oldType := flagGateLabelType
	t.Cleanup(func() {
		flagRepoDir = oldRepoDir
		flagGateLabelLabel = oldLabel
		flagGateLabelType = oldType
	})

	flagRepoDir = repoDir
	flagGateLabelLabel = label
	flagGateLabelType = gateType
	return runGateLabel(nil, nil)
}

// 场景 8：默认配置下 bot:plan-final 通过（exit 0）
func TestRunGateLabel_DefaultConfigPass(t *testing.T) {
	// 无 .niuma.yml 时使用默认配置（implement_trigger_labels = ["bot:plan-final"]）
	dir := t.TempDir()

	err := runGateLabelWithFlags(t, dir, "bot:plan-final", "implement")
	if err != nil {
		t.Errorf("expected no error for bot:plan-final with default config, got: %v", err)
	}
}

// 场景 9：默认配置下 bot:plan-approved 被拒（exit 78）
func TestRunGateLabel_DefaultConfigReject(t *testing.T) {
	dir := t.TempDir()

	err := runGateLabelWithFlags(t, dir, "bot:plan-approved", "implement")
	if err == nil {
		t.Fatal("expected error for bot:plan-approved with default config")
	}

	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) {
		t.Fatalf("error should implement ExitCode(), got: %v", err)
	}
	if ec.ExitCode() != 78 {
		t.Errorf("exit code = %d, want 78", ec.ExitCode())
	}
}

// 场景 10：自定义配置下 bot:plan-final 和 bot:plan-approved 都通过
func TestRunGateLabel_CustomConfigBothPass(t *testing.T) {
	dir := t.TempDir()
	writeNiumaYml(t, dir, `
workflow:
  implement_trigger_labels:
    - bot:plan-final
    - bot:plan-approved
`)

	for _, label := range []string{"bot:plan-final", "bot:plan-approved"} {
		err := runGateLabelWithFlags(t, dir, label, "implement")
		if err != nil {
			t.Errorf("expected no error for %s with custom config, got: %v", label, err)
		}
	}
}

// 未知 --type 报错
func TestRunGateLabel_UnknownType(t *testing.T) {
	dir := t.TempDir()

	err := runGateLabelWithFlags(t, dir, "bot:plan-final", "unknown")
	if err == nil {
		t.Fatal("expected error for unknown gate type")
	}
	if got := err.Error(); got != `不支持的 gate 类型: "unknown"（目前仅支持 implement）` {
		t.Errorf("unexpected error message: %s", got)
	}
}
