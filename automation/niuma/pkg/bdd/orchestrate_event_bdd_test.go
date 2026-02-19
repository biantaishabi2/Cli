//go:build bdd

package bdd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	bddEventRepo  = "biantaishabi2/Cli-niuma-test"
	bddEventPhase = "fix"
)

// bddEventGitHubMock 提供事件驱动场景所需的最小 GitHub 行为。
type bddEventGitHubMock struct {
	mu            sync.Mutex
	issue         control.IssueInfo
	labels        map[int][]string
	replaceCalls  int
	transitions   int
	blockedByDeps map[int]map[int]struct{}
}

func newBDDEventGitHubMock(issueNumber int) *bddEventGitHubMock {
	return &bddEventGitHubMock{
		issue: control.IssueInfo{
			Number: issueNumber,
			Title:  fmt.Sprintf("issue %d", issueNumber),
			State:  "open",
			Labels: []string{"bot:queued"},
		},
		labels: map[int][]string{
			issueNumber: {"bot:queued"},
		},
		blockedByDeps: make(map[int]map[int]struct{}),
	}
}

func (m *bddEventGitHubMock) ListIssuesWithLabel(_ context.Context, label string) ([]control.IssueInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	labels := m.labels[m.issue.Number]
	for _, item := range labels {
		if item == label {
			issue := m.issue
			issue.Labels = append([]string(nil), labels...)
			return []control.IssueInfo{issue}, nil
		}
	}
	return []control.IssueInfo{}, nil
}

func (m *bddEventGitHubMock) ListIssuesByState(_ context.Context, stateName string) ([]control.IssueInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.EqualFold(stateName, "all") || strings.EqualFold(stateName, m.issue.State) {
		issue := m.issue
		issue.Labels = append([]string(nil), m.labels[m.issue.Number]...)
		return []control.IssueInfo{issue}, nil
	}
	return []control.IssueInfo{}, nil
}

func (m *bddEventGitHubMock) GetIssue(_ context.Context, issueNumber int) (control.IssueInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if issueNumber != m.issue.Number {
		return control.IssueInfo{}, fmt.Errorf("issue #%d not found", issueNumber)
	}
	issue := m.issue
	issue.Labels = append([]string(nil), m.labels[m.issue.Number]...)
	return issue, nil
}

func (m *bddEventGitHubMock) UpdateIssueBody(_ context.Context, issueNumber int, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if issueNumber != m.issue.Number {
		return fmt.Errorf("issue #%d not found", issueNumber)
	}
	m.issue.Body = body
	return nil
}

func (m *bddEventGitHubMock) ListCommentBodies(_ context.Context, _ int) ([]string, error) {
	return nil, nil
}

func (m *bddEventGitHubMock) AddIssueComment(_ context.Context, _ int, _ string) error {
	return nil
}

func (m *bddEventGitHubMock) CloseIssue(_ context.Context, issueNumber int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if issueNumber != m.issue.Number {
		return fmt.Errorf("issue #%d not found", issueNumber)
	}
	m.issue.State = "closed"
	return nil
}

func (m *bddEventGitHubMock) MergePR(_ context.Context, _ int, _ string) error {
	return nil
}

func (m *bddEventGitHubMock) ListLabels(_ context.Context, issueNumber int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if issueNumber != m.issue.Number {
		return nil, fmt.Errorf("issue #%d not found", issueNumber)
	}
	return append([]string(nil), m.labels[issueNumber]...), nil
}

func (m *bddEventGitHubMock) AddLabel(_ context.Context, issueNumber int, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if issueNumber != m.issue.Number {
		return fmt.Errorf("issue #%d not found", issueNumber)
	}
	m.labels[issueNumber] = append(m.labels[issueNumber], label)
	m.issue.Labels = append([]string(nil), m.labels[issueNumber]...)
	return nil
}

func (m *bddEventGitHubMock) ReplaceLabel(_ context.Context, issueNumber int, oldLabel, newLabel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if issueNumber != m.issue.Number {
		return fmt.Errorf("issue #%d not found", issueNumber)
	}

	m.replaceCalls++
	labels := m.labels[issueNumber]
	for i, item := range labels {
		if item != oldLabel {
			continue
		}
		if item != newLabel {
			labels[i] = newLabel
			m.transitions++
		}
		m.labels[issueNumber] = labels
		m.issue.Labels = append([]string(nil), labels...)
		return nil
	}

	m.labels[issueNumber] = append(labels, newLabel)
	m.issue.Labels = append([]string(nil), m.labels[issueNumber]...)
	return nil
}

func (m *bddEventGitHubMock) ReplaceLabelIfPresent(_ context.Context, issueNumber int, oldLabel, newLabel string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if issueNumber != m.issue.Number {
		return false, fmt.Errorf("issue #%d not found", issueNumber)
	}

	m.replaceCalls++
	labels := m.labels[issueNumber]
	for i, item := range labels {
		if item != oldLabel {
			continue
		}
		if item != newLabel {
			labels[i] = newLabel
			m.transitions++
		}
		m.labels[issueNumber] = labels
		m.issue.Labels = append([]string(nil), labels...)
		return true, nil
	}
	return false, nil
}

func (m *bddEventGitHubMock) ReplaceLabels(_ context.Context, issueNumber int, labels []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if issueNumber != m.issue.Number {
		return fmt.Errorf("issue #%d not found", issueNumber)
	}
	m.replaceCalls++
	next := append([]string(nil), labels...)
	if !equalLabels(next, m.labels[issueNumber]) {
		m.transitions++
	}
	m.labels[issueNumber] = next
	m.issue.Labels = append([]string(nil), next...)
	return nil
}

func (m *bddEventGitHubMock) ListIssueBlockedBy(_ context.Context, issueNumber int) ([]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	deps := m.blockedByDeps[issueNumber]
	result := make([]int, 0, len(deps))
	for dep := range deps {
		result = append(result, dep)
	}
	return result, nil
}

func (m *bddEventGitHubMock) AddIssueBlockedBy(_ context.Context, issueNumber int, blockedByIssueNumber int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.blockedByDeps[issueNumber]; !ok {
		m.blockedByDeps[issueNumber] = make(map[int]struct{})
	}
	m.blockedByDeps[issueNumber][blockedByIssueNumber] = struct{}{}
	return nil
}

func (m *bddEventGitHubMock) RemoveIssueBlockedBy(_ context.Context, issueNumber int, blockedByIssueNumber int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if deps, ok := m.blockedByDeps[issueNumber]; ok {
		delete(deps, blockedByIssueNumber)
	}
	return nil
}

func (m *bddEventGitHubMock) ResolvePRMetadata(_ context.Context, issueNumber int) (control.PRMetadata, error) {
	return control.PRMetadata{}, fmt.Errorf("issue #%d: %w", issueNumber, control.ErrPRMarkerNotFound)
}

func (m *bddEventGitHubMock) ResolvePRReviewStatus(_ context.Context, issueNumber int) (control.PRReviewStatus, error) {
	return control.PRReviewStatus{}, fmt.Errorf("issue #%d: %w", issueNumber, control.ErrPRMarkerNotFound)
}

func (m *bddEventGitHubMock) transitionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.transitions
}

func (m *bddEventGitHubMock) replaceCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.replaceCalls
}

func equalLabels(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Given 上游任务完成且存在下游 queued 任务，When completed 事件触发一次 orchestrate，Then 应立即重评估并推进 ready。
func TestBDD_CompletedEventTriggersReadyReevaluation(t *testing.T) {
	const issueNumber = 530
	const inputHash = "completed-ready"

	taskctl, readyFlagPath := newBDDEventTaskCtlClient(t, issueNumber, inputHash)
	mockGH := newBDDEventGitHubMock(issueNumber)
	ctrl := newBDDEventController(taskctl, mockGH)

	require.NoError(t, ctrl.Run(context.Background()))
	assert.Equal(t, 0, mockGH.transitionCount())

	require.NoError(t, os.WriteFile(readyFlagPath, []byte("ready"), 0o644))
	require.NoError(t, ctrl.Run(context.Background()))
	assert.Equal(t, 1, mockGH.transitionCount())
}

// Given issue 被标记为 bot:queued，When issues:labeled 触发 orchestrate，Then job if 不应跳过该标签。
func TestBDD_QueuedLabelEventIsAllowedByWorkflowContract(t *testing.T) {
	content := loadOrchestrateWorkflowContent(t)
	assert.Contains(t, content, "github.event.label.name == 'bot:queued'")
}

// Given schedule 通道暂不可用，When completed 事件主动唤醒 orchestrate，Then 流程仍可推进。
func TestBDD_EventDrivenPathProgressesWithoutSchedule(t *testing.T) {
	const issueNumber = 531
	const inputHash = "event-only"

	taskctl, readyFlagPath := newBDDEventTaskCtlClient(t, issueNumber, inputHash)
	mockGH := newBDDEventGitHubMock(issueNumber)
	ctrl := newBDDEventController(taskctl, mockGH)

	require.NoError(t, os.WriteFile(readyFlagPath, []byte("ready"), 0o644))
	require.NoError(t, ctrl.Run(context.Background()))
	assert.Equal(t, 1, mockGH.transitionCount())
}

// Given 同一 issue 短时间重复唤醒，When orchestrate 连续重跑，Then 仅首次推进，后续 no-op。
func TestBDD_RepeatedWakeupIsIdempotent(t *testing.T) {
	const issueNumber = 532
	const inputHash = "repeat-noop"

	taskctl, readyFlagPath := newBDDEventTaskCtlClient(t, issueNumber, inputHash)
	mockGH := newBDDEventGitHubMock(issueNumber)
	ctrl := newBDDEventController(taskctl, mockGH)

	require.NoError(t, os.WriteFile(readyFlagPath, []byte("ready"), 0o644))
	require.NoError(t, ctrl.Run(context.Background()))
	require.NoError(t, ctrl.Run(context.Background()))
	require.NoError(t, ctrl.Run(context.Background()))

	assert.Equal(t, 1, mockGH.transitionCount())
	assert.Equal(t, 1, mockGH.replaceCount())
}

func newBDDEventController(taskctl *control.TaskCtlClient, github control.GitHubOps) *control.Controller {
	cfg := control.DefaultControlConfig()
	cfg.IssueLockHeartbeatInterval = 0
	cfg.OwnerID = "bdd-runner"
	return control.NewController(taskctl, nil, github, nil, cfg)
}

func newBDDEventTaskCtlClient(t *testing.T, issueNumber int, inputHash string) (*control.TaskCtlClient, string) {
	t.Helper()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "state")
	readyFlagPath := filepath.Join(dir, "ready.flag")
	binPath := filepath.Join(dir, "taskctl")
	idempotencyKey := buildBDDEventIdempotencyKey(bddEventRepo, issueNumber, bddEventPhase, inputHash)
	idempotencyMetadataKey := "idempotency.key." + bddEventPhase

	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
cmd="${1:-}"
if [[ -n "$cmd" ]]; then
	shift
fi
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
		statePath,
		statePath,
		issueNumber,
		issueNumber,
		issueNumber,
		issueNumber,
		bddEventRepo,
		bddEventPhase,
		inputHash,
		readyFlagPath,
		issueNumber,
		issueNumber,
		issueNumber,
		issueNumber,
		bddEventRepo,
		bddEventPhase,
		inputHash,
		issueNumber,
		issueNumber,
		issueNumber,
		issueNumber,
		bddEventRepo,
		bddEventPhase,
		inputHash,
		idempotencyMetadataKey,
		idempotencyKey,
		issueNumber,
		issueNumber,
		issueNumber,
		issueNumber,
		bddEventRepo,
		bddEventPhase,
		inputHash,
		idempotencyMetadataKey,
		statePath,
	)
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	return &control.TaskCtlClient{
		BinPath:   binPath,
		StorePath: filepath.Join(dir, "tasks.json"),
	}, readyFlagPath
}

func buildBDDEventIdempotencyKey(repo string, issueNum int, phase, inputHash string) string {
	payload := strings.Join([]string{
		strings.TrimSpace(repo),
		strconv.Itoa(issueNum),
		strings.TrimSpace(phase),
		strings.TrimSpace(inputHash),
	}, "\n")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func loadOrchestrateWorkflowContent(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		path := filepath.Join(dir, ".github", "workflows", "niuma-orchestrate.yml")
		if _, statErr := os.Stat(path); statErr == nil {
			content, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			return string(content)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("未找到 .github/workflows/niuma-orchestrate.yml")
	return ""
}
