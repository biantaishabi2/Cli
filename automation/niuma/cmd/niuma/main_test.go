package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDiscussMaxRounds_Priority(t *testing.T) {
	tests := []struct {
		name         string
		cliRounds    int
		configRounds int
		expected     int
	}{
		{
			name:         "CLI 优先于配置",
			cliRounds:    9,
			configRounds: 7,
			expected:     9,
		},
		{
			name:         "配置优先于默认值",
			cliRounds:    0,
			configRounds: 7,
			expected:     7,
		},
		{
			name:         "配置缺省时回退默认值",
			cliRounds:    0,
			configRounds: 0,
			expected:     5,
		},
		{
			name:         "CLI 上边界 20 生效",
			cliRounds:    20,
			configRounds: 7,
			expected:     20,
		},
		{
			name:         "配置上边界 20 生效",
			cliRounds:    0,
			configRounds: 20,
			expected:     20,
		},
		{
			name:         "配置超过上界回退默认值",
			cliRounds:    0,
			configRounds: 21,
			expected:     5,
		},
		{
			name:         "配置小于下界回退默认值",
			cliRounds:    0,
			configRounds: -1,
			expected:     5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, resolveDiscussMaxRounds(tt.cliRounds, tt.configRounds))
		})
	}
}

func TestValidateMaxDiscussionRounds(t *testing.T) {
	tests := []struct {
		name      string
		rounds    int
		wantError bool
	}{
		{name: "默认值 0 合法", rounds: 0, wantError: false},
		{name: "下界 1 合法", rounds: 1, wantError: false},
		{name: "上界 20 合法", rounds: 20, wantError: false},
		{name: "小于下界非法", rounds: -1, wantError: true},
		{name: "大于上界非法", rounds: 21, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMaxDiscussionRounds(tt.rounds)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestEvaluateDiscussDecision(t *testing.T) {
	decision, reason, action := evaluateDiscussDecision([]string{"bug"})
	assert.Equal(t, control.DecisionSkip, decision)
	assert.Equal(t, control.ReasonSkipStateNotApplicable, reason)
	assert.Equal(t, control.ActionNone, action)

	decision, reason, action = evaluateDiscussDecision([]string{"bug", string(state.StateNeedsDiscussion)})
	assert.Equal(t, control.DecisionRun, decision)
	assert.Equal(t, control.ReasonRunRoutable, reason)
	assert.Equal(t, control.ActionDiscuss, action)
}

func TestShouldUpgradeIterateToNeedsHuman(t *testing.T) {
	assert.True(t, shouldUpgradeIterateToNeedsHuman("review", "closed"))
	assert.True(t, shouldUpgradeIterateToNeedsHuman("human", "closed"))
	assert.False(t, shouldUpgradeIterateToNeedsHuman("review", "open"))
	assert.False(t, shouldUpgradeIterateToNeedsHuman("human", "open"))
	assert.False(t, shouldUpgradeIterateToNeedsHuman("human", ""))
	assert.False(t, shouldUpgradeIterateToNeedsHuman("unknown", "closed"))
}

func TestPickIterateIssue(t *testing.T) {
	t.Run("human path without issue refs should skip", func(t *testing.T) {
		issue, skipNoop, err := pickIterateIssue(0, nil, 42, "human")
		require.NoError(t, err)
		assert.True(t, skipNoop)
		assert.Equal(t, 0, issue)
	})

	t.Run("review path without issue refs should fail", func(t *testing.T) {
		issue, skipNoop, err := pickIterateIssue(0, nil, 42, "review")
		require.Error(t, err)
		assert.False(t, skipNoop)
		assert.Equal(t, 0, issue)
	})

	t.Run("explicit issue should take priority", func(t *testing.T) {
		issue, skipNoop, err := pickIterateIssue(99, []int{1, 2}, 42, "human")
		require.NoError(t, err)
		assert.False(t, skipNoop)
		assert.Equal(t, 99, issue)
	})

	t.Run("parse refs should pick smallest issue", func(t *testing.T) {
		issue, skipNoop, err := pickIterateIssue(0, []int{14, 12, 13}, 42, "human")
		require.NoError(t, err)
		assert.False(t, skipNoop)
		assert.Equal(t, 12, issue)
	})
}

func TestResolveIntegrationGateMaxRetries_Priority(t *testing.T) {
	tests := []struct {
		name     string
		flag     int
		env      string
		expected int
		wantErr  bool
	}{
		{
			name:     "flag 优先",
			flag:     5,
			env:      "2",
			expected: 5,
		},
		{
			name:     "env 生效",
			flag:     -1,
			env:      "4",
			expected: 4,
		},
		{
			name:    "env 非法时返回错误",
			flag:    -1,
			env:     "bad",
			wantErr: true,
		},
		{
			name:    "env 负数时返回错误",
			flag:    -1,
			env:     "-1",
			wantErr: true,
		},
		{
			name:     "全缺省回退默认",
			flag:     -1,
			env:      "",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveIntegrationGateMaxRetries(tt.flag, tt.env)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestBuildOrchestrator_InvalidImplementationProvider_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	content := `ai:
  default: claude
  providers:
    claude:
      cmd: "claude -p {prompt_file}"
    kimi:
      cmd: "kimi --print {prompt_file}"
  discussion:
    providers: [default, kimi]
  implementation:
    provider: claudee
`
	err := os.WriteFile(filepath.Join(dir, ".niuma.yml"), []byte(content), 0644)
	require.NoError(t, err)

	origRepoDir := flagRepoDir
	flagRepoDir = dir
	defer func() {
		flagRepoDir = origRepoDir
	}()

	_, err = buildOrchestrator(nil, 123)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `实现 provider "claudee" 未配置`)
}

func TestExitCodeFromError_DefaultAndWrapped(t *testing.T) {
	assert.Equal(t, 0, exitCodeFromError(nil))
	assert.Equal(t, 1, exitCodeFromError(errors.New("boom")))
	assert.Equal(t, 23, exitCodeFromError(withExitCode(23, errors.New("mapped"))))
}

func TestMapStateLabelError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{name: "conflict", err: state.ErrConflict, expected: exitCodeStateConflict},
		{name: "invalid state", err: state.ErrInvalidState, expected: exitCodeStateInvalid},
		{name: "invalid transition", err: state.ErrInvalidTransition, expected: exitCodeStateTransitionEdge},
		{name: "invariant", err: state.ErrInvariantViolation, expected: exitCodeStateInvariant},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := mapStateLabelError(tt.err)
			require.Error(t, mapped)
			assert.Equal(t, tt.expected, exitCodeFromError(mapped))
			assert.True(t, errors.Is(mapped, tt.err))
		})
	}
}

func TestRunStateLabelSet_InvalidInputMapsStateExitCode(t *testing.T) {
	restore := snapshotCLIFlags()
	defer restore()

	flagRepo = "owner/repo"
	flagIssue = 325

	t.Run("invalid --to", func(t *testing.T) {
		flagStateTo = "bot:unknown"
		flagStateFrom = ""

		err := runStateLabelSet(nil, nil)
		require.Error(t, err)
		assert.Equal(t, exitCodeStateInvalid, exitCodeFromError(err))
		assert.True(t, errors.Is(err, state.ErrInvalidState))
	})

	t.Run("invalid --from", func(t *testing.T) {
		flagStateTo = string(state.StateFixRequested)
		flagStateFrom = "bot:unknown"

		err := runStateLabelSet(nil, nil)
		require.Error(t, err)
		assert.Equal(t, exitCodeStateInvalid, exitCodeFromError(err))
		assert.True(t, errors.Is(err, state.ErrInvalidState))
	})
}

func TestRunStateLabelNormalize_InvalidPriorityMapsStateExitCode(t *testing.T) {
	restore := snapshotCLIFlags()
	defer restore()

	flagRepo = "owner/repo"
	flagIssue = 325
	flagStatePriority = "bot:needs-discussion,bot:unknown"

	err := runStateLabelNormalize(nil, nil)
	require.Error(t, err)
	assert.Equal(t, exitCodeStateInvalid, exitCodeFromError(err))
	assert.True(t, errors.Is(err, state.ErrInvalidState))
}

func snapshotCLIFlags() func() {
	prevRepo := flagRepo
	prevIssue := flagIssue
	prevStateFrom := flagStateFrom
	prevStateTo := flagStateTo
	prevStatePriority := flagStatePriority

	return func() {
		flagRepo = prevRepo
		flagIssue = prevIssue
		flagStateFrom = prevStateFrom
		flagStateTo = prevStateTo
		flagStatePriority = prevStatePriority
	}
}

func TestBuildWorkflowAudit_ContainsRequiredFields(t *testing.T) {
	t.Setenv("GITHUB_WORKFLOW", "niuma - Iterate")
	t.Setenv("GITHUB_EVENT_NAME", "pull_request_review")
	t.Setenv("GITHUB_RUN_ID", "123")
	t.Setenv("GITHUB_RUN_ATTEMPT", "2")

	audit := buildWorkflowAudit(control.DecisionSkip, control.ReasonSkipStateNotApplicable, control.ActionIterate, 11, 22)

	assert.Equal(t, "niuma - Iterate", audit["workflow"])
	assert.Equal(t, "pull_request_review", audit["event_name"])
	assert.Equal(t, string(control.ActionIterate), audit["action"])
	assert.Equal(t, string(control.DecisionSkip), audit["decision"])
	assert.Equal(t, string(control.ReasonSkipStateNotApplicable), audit["reason"])
	assert.Equal(t, "run-123-attempt-2", audit["correlation_id"])
	assert.Equal(t, "11", audit["issue"])
	assert.Equal(t, "22", audit["pr"])
}

func TestPrintWorkflowDecision_EmitsStructuredAuditAndKV(t *testing.T) {
	t.Setenv("GITHUB_WORKFLOW", "niuma - Discuss")
	t.Setenv("GITHUB_EVENT_NAME", "issues")
	t.Setenv("GITHUB_RUN_ID", "88")
	t.Setenv("GITHUB_RUN_ATTEMPT", "1")

	stdout, stderr := captureStdoutStderr(t, func() {
		printWorkflowDecision(control.DecisionFail, control.ReasonFailInternalError, control.ActionDiscuss, 9, 0)
	})

	assert.Contains(t, stdout, "decision=fail")
	assert.Contains(t, stdout, "reason=fail_internal_error")
	assert.Contains(t, stdout, "action=discuss")
	assert.Contains(t, stdout, "issue=9")

	var audit map[string]string
	line := bytes.TrimSpace([]byte(stderr))
	line = bytes.TrimPrefix(line, []byte("[control.workflow] "))
	require.NoError(t, json.Unmarshal(line, &audit))
	assert.Equal(t, "niuma - Discuss", audit["workflow"])
	assert.Equal(t, "issues", audit["event_name"])
	assert.Equal(t, "discuss", audit["action"])
	assert.Equal(t, "fail", audit["decision"])
	assert.Equal(t, "fail_internal_error", audit["reason"])
	assert.Equal(t, "run-88-attempt-1", audit["correlation_id"])
}

func captureStdoutStderr(t *testing.T, fn func()) (string, string) {
	t.Helper()

	origStdout := os.Stdout
	origStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)
	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	fn()

	require.NoError(t, stdoutWriter.Close())
	require.NoError(t, stderrWriter.Close())

	stdoutBytes, err := io.ReadAll(stdoutReader)
	require.NoError(t, err)
	stderrBytes, err := io.ReadAll(stderrReader)
	require.NoError(t, err)
	return string(stdoutBytes), string(stderrBytes)
}
