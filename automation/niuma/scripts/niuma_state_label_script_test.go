package scripts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNiumaStateLabelScript_DefaultsToCAS(t *testing.T) {
	scriptPath := filepath.Join(".", "niuma_state_label.sh")
	content, err := os.ReadFile(scriptPath)
	require.NoError(t, err)
	text := string(content)

	assert.Contains(t, text, "auto-detected from=$FROM_STATE (CAS)")
	assert.Contains(t, text, "state-label set --repo \"$REPO\" --issue \"$ISSUE_NUMBER\" --to \"$TARGET_STATE\"")
	assert.Contains(t, text, "ARGS+=(--from \"$FROM_STATE\")")
}
