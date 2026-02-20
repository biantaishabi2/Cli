package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowContract_EntrypointUsesReusableAndKeepsTriggers(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-orchestrate.yml")

	assert.Contains(t, content, "issues:")
	assert.Contains(t, content, "types: [labeled]")
	assert.Contains(t, content, "repository_dispatch:")
	assert.Contains(t, content, "types: [niuma.task.completed]")
	assert.Contains(t, content, "schedule:")
	assert.Contains(t, content, "github.event.label.name == 'bot:queued'")
	assert.Contains(t, content, "github.event.label.name == 'bot:pr-reviewable'")
	assert.Contains(t, content, "github.event.client_payload.event_source == 'close-after-integration-merge'")
	assert.Contains(t, content, "uses: ./.github/workflows/niuma-orchestrate-reusable.yml")
}

func TestWorkflowContract_ReusableWorkflowCallInputDefaults(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-orchestrate-reusable.yml")

	assert.Contains(t, content, "workflow_call:")
	assert.Contains(t, content, "repo:")
	assert.Contains(t, content, "required: true")
	assert.Contains(t, content, "repo_dir:")
	assert.Contains(t, content, "default: \".\"")
	assert.Contains(t, content, "build_niuma:")
	assert.Contains(t, content, "default: true")
	assert.Contains(t, content, "label_whitelist:")
	assert.Contains(t, content, "default: \"bot:queued,bot:pr-reviewable\"")
	assert.Contains(t, content, "enable_dispatch_wakeup:")
	assert.Contains(t, content, "event_id:")
	assert.Contains(t, content, "default: \"\"")
	assert.Contains(t, content, "dedup_window_hours:")
	assert.Contains(t, content, "default: 24")
	assert.Contains(t, content, "concurrency_key:")
	assert.Contains(t, content, "default: \"niuma-orchestrate-${repo}\"")
}

func TestWorkflowContract_ReusableHasIdempotencyLoopGuardAndConcurrency(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-orchestrate-reusable.yml")

	assert.Contains(t, content, "concurrency:")
	assert.Contains(t, content, "cancel-in-progress: false")
	assert.Contains(t, content, "Idempotency Guard")
	assert.Contains(t, content, "/tmp/niuma-orchestrate-dedup")
	assert.Contains(t, content, "duplicate_event")
	assert.Contains(t, content, "Loop Guard")
	assert.Contains(t, content, "github-actions[bot]")
	assert.Contains(t, content, "event_id=\"run-${GITHUB_RUN_ID}-attempt-${GITHUB_RUN_ATTEMPT}\"")
}

func TestWorkflowContract_DispatchCompletedUsesNiumaBinary(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-dispatch-completed.yml")

	assert.Contains(t, content, "pull_request:")
	assert.Contains(t, content, "types: [closed]")
	assert.Contains(t, content, "github.event.pull_request.merged == true")
	assert.Contains(t, content, "startsWith(github.event.pull_request.head.ref, 'integration/')")
	assert.Contains(t, content, "control close-merged")
	assert.Contains(t, content, "--dispatch-wakeup")
}

func TestWorkflowContract_ImplementGateHasMaxRetriesValidation(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-implement-reusable.yml")

	assert.Contains(t, content, "GATE_MAX_RETRIES=2",
		"niuma-implement-reusable.yml 必须包含 GATE_MAX_RETRIES 默认值回退")
}

func TestWorkflowContract_IterateGateHasMaxRetriesValidation(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-iterate-reusable.yml")

	assert.Contains(t, content, "GATE_MAX_RETRIES=2",
		"niuma-iterate-reusable.yml 必须包含 GATE_MAX_RETRIES 默认值回退")
}

func TestWorkflowContract_ImplementGateUsesUnifiedCommand(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-implement-reusable.yml")

	assert.Contains(t, content, "\"$NIUMA_BIN\" gate run")
	assert.Contains(t, content, "GATE_MAX_RETRIES=2")
	assert.Contains(t, content, "--max-retries \"$GATE_MAX_RETRIES\"")
}

func TestWorkflowContract_IterateGateUsesUnifiedCommand(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-iterate-reusable.yml")

	assert.Equal(t, 2, strings.Count(content, "\"$NIUMA_BIN\" gate run"))
	assert.Equal(t, 2, strings.Count(content, "GATE_MAX_RETRIES=2"))
	assert.Equal(t, 2, strings.Count(content, "--max-retries \"$GATE_MAX_RETRIES\""))
}

func loadWorkflowFile(t *testing.T, workflowName string) string {
	t.Helper()

	workflowPath := findWorkflowPath(t, workflowName)
	raw, err := os.ReadFile(workflowPath)
	require.NoError(t, err)
	return string(raw)
}

func findWorkflowPath(t *testing.T, workflowName string) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, ".github", "workflows", workflowName)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatalf("未找到 .github/workflows/%s", workflowName)
	return ""
}
