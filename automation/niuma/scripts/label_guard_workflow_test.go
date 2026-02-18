package scripts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabelGuardWorkflow_EnforceRollbackAndAllowlist(t *testing.T) {
	workflowPath := filepath.Join("..", "..", "..", ".github", "workflows", "niuma-label-guard.yml")
	content, err := os.ReadFile(workflowPath)
	require.NoError(t, err)
	text := string(content)

	assert.Contains(t, text, "DEFAULT_ALLOWLIST=\"github-actions[bot],niuma-bot\"")
	assert.Contains(t, text, "EXTRA_ALLOWLIST: ${{ vars.NIUMA_LABEL_ALLOWLIST || '' }}")
	assert.Contains(t, text, "if [ \"$ACTION\" = \"labeled\" ]; then")
	assert.Contains(t, text, "--remove-label \"$LABEL\"")
	assert.Contains(t, text, "elif [ \"$ACTION\" = \"unlabeled\" ]; then")
	assert.Contains(t, text, "--add-label \"$LABEL\"")
}
