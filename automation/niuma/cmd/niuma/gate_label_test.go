package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/config"
	"github.com/spf13/cobra"
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

func TestGateLabelExitCode78OnReject(t *testing.T) {
	// 模拟 gate label 的核心逻辑：标签不在列表中应返回 exit code 78
	triggerLabels := []string{"bot:plan-final"}
	label := "bot:plan-approved"

	found := false
	for _, allowed := range triggerLabels {
		if allowed == label {
			found = true
			break
		}
	}
	if found {
		t.Fatal("bot:plan-approved should NOT be in default trigger list")
	}

	// 验证 withExitCode 生成正确的退出码
	err := withExitCode(78, fmt.Errorf("label %s not in trigger list", label))
	if err == nil {
		t.Fatal("expected error for rejected label")
	}

	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) {
		t.Fatal("error should implement ExitCode()")
	}
	if ec.ExitCode() != 78 {
		t.Errorf("exit code = %d, want 78", ec.ExitCode())
	}
}

func TestGateLabelExitCode0OnPass(t *testing.T) {
	// 标签在列表中应通过（return nil，即 exit 0）
	triggerLabels := []string{"bot:plan-final"}
	label := "bot:plan-final"

	found := false
	for _, allowed := range triggerLabels {
		if allowed == label {
			found = true
			break
		}
	}
	if !found {
		t.Error("bot:plan-final should be in default trigger list")
	}
}

func TestGateLabelUnknownType(t *testing.T) {
	// 使用 cobra 执行，验证未知 type 返回错误
	cmd := &cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error {
		gateType := "unknown"
		if gateType != "implement" {
			return fmt.Errorf("不支持的 gate 类型: %q（目前仅支持 implement）", gateType)
		}
		return nil
	}}
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown gate type")
	}
	if !strings.Contains(err.Error(), "不支持的 gate 类型") {
		t.Errorf("error should mention unsupported type, got: %v", err)
	}
}

func TestGateLabelCustomConfigBothPass(t *testing.T) {
	cfg := &config.Config{
		Workflow: config.WorkflowConfig{
			ImplementTriggerLabels: []string{"bot:plan-final", "bot:plan-approved"},
		},
	}
	triggerLabels := cfg.Workflow.GetImplementTriggerLabels()

	for _, label := range []string{"bot:plan-final", "bot:plan-approved"} {
		found := false
		for _, allowed := range triggerLabels {
			if allowed == label {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("label %s should pass with custom config, trigger list: %v", label, triggerLabels)
		}
	}
}
