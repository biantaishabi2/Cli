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

	assert.Contains(t, text, "name: niuma - Label Guard")
	assert.Contains(t, text, "types: [labeled, unlabeled]")
	assert.Contains(t, text, "if: startsWith(github.event.label.name, 'bot:')")
	assert.Contains(t, text, "go build -o \"$GITHUB_WORKSPACE/.tmp/niuma\" ./cmd/niuma")
	assert.Contains(t, text, "\"$NIUMA_BIN\" label-guard \\")
	assert.Contains(t, text, "--repo \"$REPO\" \\")
	assert.Contains(t, text, "--issue \"$ISSUE_NUMBER\" \\")
	assert.Contains(t, text, "--actor \"$ACTOR\" \\")
	assert.Contains(t, text, "--action \"$ACTION\" \\")
	assert.Contains(t, text, "--label \"$LABEL\"")
}
