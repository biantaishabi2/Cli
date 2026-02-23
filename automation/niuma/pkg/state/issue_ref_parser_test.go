package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseIssueRefsFromPR(t *testing.T) {
	prTitle := "feat: sub(#12): migrate refs #13"
	prBody := "Closes #14\n\nparent(#15)"
	messages := []string{
		"Merge pull request #500 from foo/bar",
		"chore: docs",
		"fix: bug (#16)",
	}

	got := ParseIssueRefsFromPR(prTitle, prBody, messages)
	assert.Equal(t, []int{12, 13, 14, 15, 16}, got)
}

func TestParseIssueRefs_DeduplicateAndIgnorePRNumber(t *testing.T) {
	got := ParseIssueRefs(
		"Closes #22",
		"refs #22",
		"Merge pull request #22 from foo/bar",
	)
	assert.Equal(t, []int{22}, got)
}
