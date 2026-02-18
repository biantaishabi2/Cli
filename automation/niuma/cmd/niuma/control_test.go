package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	ghapi "github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestGitHubControlOps_ResolvePRMetadata_Success(t *testing.T) {
	client := &stubGitHubControlClient{
		findMarkerResp: &gh.MarkerComment{
			Marker: &marker.Marker{
				Type:  marker.TypePRCreated,
				Issue: 321,
				PR:    123,
			},
		},
		prs: map[int]*ghapi.PullRequest{
			123: {
				State: ghapi.Ptr("open"),
				Head: &ghapi.PullRequestBranch{
					Ref: ghapi.Ptr("feat/321-fix"),
				},
			},
		},
	}
	ops := &gitHubControlOps{client: client}

	metadata, err := ops.ResolvePRMetadata(context.Background(), 321)
	require.NoError(t, err)
	assert.Equal(t, control.PRMetadata{PRNum: 123, Branch: "feat/321-fix"}, metadata)
}

func TestGitHubControlOps_ResolvePRMetadata_MarkerNotFound(t *testing.T) {
	client := &stubGitHubControlClient{}
	ops := &gitHubControlOps{client: client}

	_, err := ops.ResolvePRMetadata(context.Background(), 321)
	require.Error(t, err)
	assert.ErrorIs(t, err, control.ErrPRMarkerNotFound)
}

func TestGitHubControlOps_ResolvePRMetadata_PRClosed(t *testing.T) {
	client := &stubGitHubControlClient{
		findMarkerResp: &gh.MarkerComment{
			Marker: &marker.Marker{
				Type:  marker.TypePRCreated,
				Issue: 321,
				PR:    123,
			},
		},
		prs: map[int]*ghapi.PullRequest{
			123: {
				State: ghapi.Ptr("closed"),
				Head: &ghapi.PullRequestBranch{
					Ref: ghapi.Ptr("feat/321-fix"),
				},
			},
		},
	}
	ops := &gitHubControlOps{client: client}

	_, err := ops.ResolvePRMetadata(context.Background(), 321)
	require.Error(t, err)
	assert.ErrorIs(t, err, control.ErrPRClosed)
}

func TestGitHubControlOps_ResolvePRMetadata_BranchUnavailable(t *testing.T) {
	client := &stubGitHubControlClient{
		findMarkerResp: &gh.MarkerComment{
			Marker: &marker.Marker{
				Type:  marker.TypePRCreated,
				Issue: 321,
				PR:    123,
			},
		},
		prs: map[int]*ghapi.PullRequest{
			123: {
				State: ghapi.Ptr("open"),
				Head: &ghapi.PullRequestBranch{
					Ref: ghapi.Ptr("  "),
				},
			},
		},
	}
	ops := &gitHubControlOps{client: client}

	_, err := ops.ResolvePRMetadata(context.Background(), 321)
	require.Error(t, err)
	assert.ErrorIs(t, err, control.ErrPRBranchUnavailable)
}

func TestGitHubControlOps_ResolvePRMetadata_APIError(t *testing.T) {
	client := &stubGitHubControlClient{
		findMarkerResp: &gh.MarkerComment{
			Marker: &marker.Marker{
				Type:  marker.TypePRCreated,
				Issue: 321,
				PR:    123,
			},
		},
		prErr: map[int]error{
			123: errors.New("github api down"),
		},
	}
	ops := &gitHubControlOps{client: client}

	_, err := ops.ResolvePRMetadata(context.Background(), 321)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github api down")
}

func TestWorkflowGateStatusJQ_ObjectTasksPending(t *testing.T) {
	status := runWorkflowGateStatusJQ(t, `{"version":"1","tasks":{"a":{"metadata":{"integration_gate_status":"pending"}}}}`)
	assert.Equal(t, "pending", status)
}

func TestWorkflowGateStatusJQ_DefaultPassedWhenStatusesEmpty(t *testing.T) {
	status := runWorkflowGateStatusJQ(t, `{"version":"1","tasks":{"a":{"metadata":{}},"b":{"metadata":{}}}}`)
	assert.Equal(t, "passed", status)
}

func TestWorkflowGateStatusJQ_PriorityEscalatedFirst(t *testing.T) {
	status := runWorkflowGateStatusJQ(t, `{"version":"1","tasks":{"a":{"metadata":{"integration_gate_status":"passed"}},"b":{"metadata":{"integration_gate_status":"pending"}},"c":{"metadata":{"integration_gate_status":"escalated"}}}}`)
	assert.Equal(t, "escalated", status)
}

func runWorkflowGateStatusJQ(t *testing.T, tasksJSON string) string {
	t.Helper()

	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not installed")
	}

	storePath := filepath.Join(t.TempDir(), "tasks.json")
	require.NoError(t, os.WriteFile(storePath, []byte(tasksJSON), 0o644))

	expr := `
      [ .tasks | to_entries[] | .value.metadata.integration_gate_status // empty ] as $s |
      if ($s | index("escalated")) then "escalated"
      elif ($s | index("retrying")) then "retrying"
      elif ($s | index("pending")) then "pending"
      elif ($s | index("passed")) then "passed"
      else "passed" end
    `
	out, err := exec.Command("jq", "-r", expr, storePath).CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

type stubGitHubControlClient struct {
	findMarkerResp *gh.MarkerComment
	findMarkerErr  error
	prs            map[int]*ghapi.PullRequest
	prErr          map[int]error
}

func (s *stubGitHubControlClient) ListIssuesWithLabel(_ context.Context, _ string) ([]*ghapi.Issue, error) {
	return nil, nil
}

func (s *stubGitHubControlClient) ListIssuesByState(_ context.Context, _ string) ([]*ghapi.Issue, error) {
	return nil, nil
}

func (s *stubGitHubControlClient) GetIssue(_ context.Context, _ int) (*ghapi.Issue, error) {
	return nil, nil
}

func (s *stubGitHubControlClient) CloseIssue(_ context.Context, _ int) error {
	return nil
}

func (s *stubGitHubControlClient) MergePR(_ context.Context, _ int, _ string) error {
	return nil
}

func (s *stubGitHubControlClient) ReplaceLabel(_ context.Context, _ int, _, _ string) error {
	return nil
}

func (s *stubGitHubControlClient) FindMarker(_ context.Context, _ int, _ marker.Type) (*gh.MarkerComment, error) {
	if s.findMarkerErr != nil {
		return nil, s.findMarkerErr
	}
	return s.findMarkerResp, nil
}

func (s *stubGitHubControlClient) GetPR(_ context.Context, number int) (*ghapi.PullRequest, error) {
	if err, ok := s.prErr[number]; ok {
		return nil, err
	}
	if pr, ok := s.prs[number]; ok {
		return pr, nil
	}
	return nil, errors.New("pr not found")
}
