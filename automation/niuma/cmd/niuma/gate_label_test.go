package main

import (
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
