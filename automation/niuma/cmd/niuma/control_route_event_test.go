package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunControlRouteEvent(t *testing.T) {
	eventPath := filepath.Join(t.TempDir(), "event.json")
	require.NoError(t, os.WriteFile(eventPath, []byte(`{"label":{"name":"bot:premerged"}}`), 0o644))

	flagRouteWorkflow = "orchestrate"
	flagRouteEventName = "issues"
	flagRouteEventPath = eventPath

	err := runControlRouteEvent(nil, nil)
	require.NoError(t, err)
}

func TestRunControlRouteEvent_InvalidPayloadFails(t *testing.T) {
	eventPath := filepath.Join(t.TempDir(), "event.json")
	require.NoError(t, os.WriteFile(eventPath, []byte(`{"label":`), 0o644))

	flagRouteWorkflow = "orchestrate"
	flagRouteEventName = "issues"
	flagRouteEventPath = eventPath

	err := runControlRouteEvent(nil, nil)
	require.Error(t, err)
	assert.Equal(t, 1, exitCodeFromError(err))
}

func TestRunControlRouteEvent_ReadEventPathFailurePrintsFailDecision(t *testing.T) {
	flagRouteWorkflow = "orchestrate"
	flagRouteEventName = "issues"
	flagRouteEventPath = filepath.Join(t.TempDir(), "missing-event.json")

	var runErr error
	stdout, stderr := captureStdoutStderr(t, func() {
		runErr = runControlRouteEvent(nil, nil)
	})
	require.Error(t, runErr)
	assert.Equal(t, 1, exitCodeFromError(runErr))

	assert.Contains(t, stdout, "decision=fail")
	assert.Contains(t, stdout, "reason=fail_invalid_event_payload")
	assert.Contains(t, stdout, "action=none")
	assert.Contains(t, stderr, "[control.route-event] ")

	var audit map[string]string
	line := bytes.TrimSpace([]byte(stderr))
	line = bytes.TrimPrefix(line, []byte("[control.route-event] "))
	require.NoError(t, json.Unmarshal(line, &audit))
	assert.Equal(t, "orchestrate", audit["workflow"])
	assert.Equal(t, "issues", audit["event_name"])
	assert.Equal(t, string(control.DecisionFail), audit["decision"])
	assert.Equal(t, string(control.ReasonFailInvalidEventPayload), audit["reason"])
	assert.Equal(t, string(control.ActionNone), audit["action"])
	assert.NotEmpty(t, audit["correlation_id"])
}
