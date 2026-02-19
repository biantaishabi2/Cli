package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrateEventFlow_CompletedEventReevaluatesReady(t *testing.T) {
	const issueNumber = 420
	const inputHash = "completed-event"

	taskctl, logPath, readyFlagPath := newEventFlowTaskCtlClient(t, issueNumber, inputHash)
	mockGH := newControlFlowGitHubMock(issueNumber)
	ctrl := newDiscussFlowController(taskctl, mockGH, newFlowIssueLockStore(), "runner-event", time.Now)

	require.NoError(t, ctrl.Run(context.Background()))
	assert.Equal(t, 0, mockGH.stateTransitionCount(issueNumber), "未收到 completed 事件前不应推进下游 ready")
	assert.Equal(t, 0, countDiscussFlowTaskctlLogMatches(t, logPath, "--status in-progress"))

	require.NoError(t, os.WriteFile(readyFlagPath, []byte("ready"), 0o644))
	require.NoError(t, ctrl.Run(context.Background()))

	assert.Equal(t, 1, mockGH.stateTransitionCount(issueNumber), "completed 事件后应立即触发一次推进")
	assert.Equal(t, 1, mockGH.replaceCallCount(issueNumber))
	assert.Equal(t, 1, countDiscussFlowTaskctlLogMatches(t, logPath, "--status in-progress"))
}

func TestOrchestrateEventFlow_EventDrivenProgressWithoutSchedule(t *testing.T) {
	const issueNumber = 421
	const inputHash = "event-only"

	taskctl, logPath, readyFlagPath := newEventFlowTaskCtlClient(t, issueNumber, inputHash)
	mockGH := newControlFlowGitHubMock(issueNumber)
	ctrl := newDiscussFlowController(taskctl, mockGH, newFlowIssueLockStore(), "runner-event-only", time.Now)

	require.NoError(t, os.WriteFile(readyFlagPath, []byte("ready"), 0o644))
	require.NoError(t, ctrl.Run(context.Background()))
	require.NoError(t, ctrl.Run(context.Background()))

	assert.Equal(t, 1, mockGH.stateTransitionCount(issueNumber), "重复事件唤醒应由幂等逻辑降级为 no-op")
	assert.Equal(t, 1, mockGH.replaceCallCount(issueNumber))
	assert.Equal(t, 1, countDiscussFlowTaskctlLogMatches(t, logPath, "--status in-progress"))
}

func newEventFlowTaskCtlClient(t *testing.T, issueNumber int, inputHash string) (*control.TaskCtlClient, string, string) {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "taskctl.log")
	statePath := filepath.Join(dir, "state")
	readyFlagPath := filepath.Join(dir, "ready.flag")
	binPath := filepath.Join(dir, "taskctl")
	idempotencyKey := buildEventFlowIdempotencyKey(
		discussFlowDefaultRepo,
		issueNumber,
		discussFlowDefaultPhase,
		inputHash,
	)
	idempotencyMetadataKey := "idempotency.key." + discussFlowDefaultPhase

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
cmd="${1:-}"
if [[ -n "$cmd" ]]; then
	shift
fi
printf '%%s %%s\n' "$cmd" "$*" >> %q
state=""
if [[ -f %q ]]; then
	state="$(cat %q)"
fi
case "$cmd" in
	list)
		cat <<'JSON'
[{"id":"task-%d","subject":"issue %d","description":"issue %d","status":"pending","blocked_by":[],"metadata":{"issue_num":"%d","repo":"%s","phase":"%s","input_hash":"%s"}}]
JSON
		;;
	ready)
		if [[ -f %q ]]; then
			cat <<'JSON'
[{"id":"task-%d","subject":"issue %d","description":"issue %d","status":"pending","blocked_by":[],"metadata":{"issue_num":"%d","repo":"%s","phase":"%s","input_hash":"%s"}}]
JSON
		else
			echo '[]'
		fi
		;;
	get)
		if [[ "$state" == "recorded" ]]; then
			cat <<'JSON'
{"id":"task-%d","subject":"issue %d","description":"issue %d","status":"pending","blocked_by":[],"metadata":{"issue_num":"%d","repo":"%s","phase":"%s","input_hash":"%s","%s":"%s"}}
JSON
		else
			cat <<'JSON'
{"id":"task-%d","subject":"issue %d","description":"issue %d","status":"pending","blocked_by":[],"metadata":{"issue_num":"%d","repo":"%s","phase":"%s","input_hash":"%s"}}
JSON
		fi
		;;
	update)
		if [[ "$*" == *"%s"* ]]; then
			printf 'recorded' > %q
		fi
		echo '{}'
		;;
	*)
		echo '{}'
		;;
esac
`,
		logPath,
		statePath,
		statePath,
		issueNumber,
		issueNumber,
		issueNumber,
		issueNumber,
		discussFlowDefaultRepo,
		discussFlowDefaultPhase,
		inputHash,
		readyFlagPath,
		issueNumber,
		issueNumber,
		issueNumber,
		issueNumber,
		discussFlowDefaultRepo,
		discussFlowDefaultPhase,
		inputHash,
		issueNumber,
		issueNumber,
		issueNumber,
		issueNumber,
		discussFlowDefaultRepo,
		discussFlowDefaultPhase,
		inputHash,
		idempotencyMetadataKey,
		idempotencyKey,
		issueNumber,
		issueNumber,
		issueNumber,
		issueNumber,
		discussFlowDefaultRepo,
		discussFlowDefaultPhase,
		inputHash,
		idempotencyMetadataKey,
		statePath,
	)
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	return &control.TaskCtlClient{
		BinPath:   binPath,
		StorePath: filepath.Join(dir, "tasks.json"),
	}, logPath, readyFlagPath
}

func buildEventFlowIdempotencyKey(repo string, issueNum int, phase, inputHash string) string {
	payload := strings.Join([]string{
		strings.TrimSpace(repo),
		strconv.Itoa(issueNum),
		strings.TrimSpace(phase),
		strings.TrimSpace(inputHash),
	}, "\n")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
