package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractIntegratedIssueNumbers(t *testing.T) {
	messages := []string{
		"Merge feat/214-close (issue #214)",
		"Merge feat/215-parent (issue #215)\n\nextra",
		"chore: update docs",
		"Merge feat/214-close (issue #214)",
	}

	got := extractIntegratedIssueNumbers(messages)
	assert.Equal(t, []int{214, 215}, got)
}

func TestExtractIntegratedIssueNumbers_IgnoreNonIntegrationPattern(t *testing.T) {
	messages := []string{
		"Merge pull request #300 from foo/bar",
		"Closes #214",
		"fix: test (#215)",
	}

	got := extractIntegratedIssueNumbers(messages)
	assert.Empty(t, got)
}
