package main

import (
	"os"
	"path/filepath"
	"testing"

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
