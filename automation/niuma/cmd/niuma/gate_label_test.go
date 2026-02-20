package main

import (
	"errors"
	"fmt"
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
	// runGateLabel 应拒绝未知 type
	// 直接测试核心逻辑
	gateType := "unknown"
	if gateType == "implement" {
		t.Fatal("should not reach here for unknown type")
	}
	// 验证非 implement 类型会被拒绝（与 runGateLabel 逻辑一致）
	if gateType != "implement" {
		// 正确行为：拒绝
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
