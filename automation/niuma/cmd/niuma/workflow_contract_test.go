package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type orchestrateWorkflowContract struct {
	On struct {
		RepositoryDispatch struct {
			Types []string `yaml:"types"`
		} `yaml:"repository_dispatch"`
	} `yaml:"on"`
	Jobs struct {
		Orchestrate struct {
			If string `yaml:"if"`
		} `yaml:"orchestrate"`
		CloseAfterIntegrationMerge struct {
			If    string `yaml:"if"`
			Steps []struct {
				Name            string `yaml:"name"`
				If              string `yaml:"if"`
				ContinueOnError bool   `yaml:"continue-on-error"`
				Run             string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"close-after-integration-merge"`
	} `yaml:"jobs"`
}

func TestWorkflowContract_OrchestrateAllowsQueuedLabel(t *testing.T) {
	content, wf := loadOrchestrateWorkflowContract(t)

	assert.Contains(t, content, "github.event.label.name == 'bot:queued'")
	assert.Contains(t, wf.Jobs.Orchestrate.If, "github.event.label.name == 'bot:queued'")
}

func TestWorkflowContract_RegistersRepositoryDispatchEvent(t *testing.T) {
	_, wf := loadOrchestrateWorkflowContract(t)

	assert.Contains(t, wf.On.RepositoryDispatch.Types, "niuma.task.completed")
	assert.Contains(t, wf.Jobs.Orchestrate.If, "github.event_name == 'repository_dispatch'")
	assert.Contains(t, wf.Jobs.Orchestrate.If, "github.event.action == 'niuma.task.completed'")
	assert.Contains(t, wf.Jobs.Orchestrate.If, "github.event.client_payload.event_source == 'close-after-integration-merge'")
}

func TestWorkflowContract_DispatchStepHasLoopGuardAndPayloadFields(t *testing.T) {
	content, wf := loadOrchestrateWorkflowContract(t)

	assert.Contains(t, wf.Jobs.CloseAfterIntegrationMerge.If, "github.event_name == 'pull_request'")
	assert.Contains(t, wf.Jobs.CloseAfterIntegrationMerge.If, "github.event.pull_request.merged == true")
	assert.Contains(t, wf.Jobs.CloseAfterIntegrationMerge.If, "startsWith(github.event.pull_request.head.ref, 'integration/')")

	dispatchStepCount := 0
	for _, step := range wf.Jobs.CloseAfterIntegrationMerge.Steps {
		if step.Name != "Dispatch orchestrate wakeup" {
			continue
		}
		dispatchStepCount++
		assert.Contains(t, step.If, "success()")
		assert.True(t, step.ContinueOnError)
		assert.Contains(t, step.Run, "event_type: \"niuma.task.completed\"")
		assert.Contains(t, step.Run, "source_issue")
		assert.Contains(t, step.Run, "source_issues")
		assert.Contains(t, step.Run, "trigger_pr")
		assert.Contains(t, step.Run, "completed_at")
		assert.Contains(t, step.Run, "event_source: \"close-after-integration-merge\"")
		assert.Contains(t, step.Run, "event_id")
	}
	assert.Equal(t, 1, dispatchStepCount, "应仅允许 close-after-integration-merge 路径发起一次 dispatch")
	assert.Equal(t, 1, strings.Count(content, "Dispatch orchestrate wakeup"))
}

func loadOrchestrateWorkflowContract(t *testing.T) (string, orchestrateWorkflowContract) {
	t.Helper()

	workflowPath := findOrchestrateWorkflowPath(t)
	raw, err := os.ReadFile(workflowPath)
	require.NoError(t, err)

	var wf orchestrateWorkflowContract
	require.NoError(t, yaml.Unmarshal(raw, &wf))
	return string(raw), wf
}

func findOrchestrateWorkflowPath(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, ".github", "workflows", "niuma-orchestrate.yml")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatalf("未找到 .github/workflows/niuma-orchestrate.yml")
	return ""
}
