package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/agent"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	gh "github.com/biantaishabi2/Cli/automation/niuma/pkg/github"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/marker"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	ghapi "github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	discussFlowLockTTL            = 30 * time.Second
	discussFlowConcurrencyWaitMax = 60 * time.Second
	discussFlowLockRecoveryBuffer = 5 * time.Second
	discussFlowDefaultRepo        = "biantaishabi2/Cli-niuma-test"
	discussFlowDefaultPhase       = "fix"
)

// flowGitHubMock 为 discuss 集成流程提供最小 GitHub 行为模拟。
type flowGitHubMock struct {
	mu       sync.Mutex
	issues   map[int]*ghapi.Issue
	comments map[int][]*ghapi.IssueComment
	labels   map[int][]string
	markers  map[string]*gh.MarkerComment
	nextID   int64
}

func newFlowGitHubMock() *flowGitHubMock {
	return &flowGitHubMock{
		issues:   map[int]*ghapi.Issue{},
		comments: map[int][]*ghapi.IssueComment{},
		labels:   map[int][]string{},
		markers:  map[string]*gh.MarkerComment{},
	}
}

func (m *flowGitHubMock) markerKey(issueNumber int, t marker.Type) string {
	return fmt.Sprintf("%d:%s", issueNumber, t)
}

func (m *flowGitHubMock) nextCommentID() int64 {
	m.nextID++
	return m.nextID
}

func (m *flowGitHubMock) GetIssue(_ context.Context, number int) (*ghapi.Issue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	issue, ok := m.issues[number]
	if !ok {
		return nil, fmt.Errorf("issue #%d not found", number)
	}
	return issue, nil
}

func (m *flowGitHubMock) ListComments(_ context.Context, issueNumber int) ([]*ghapi.IssueComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]*ghapi.IssueComment(nil), m.comments[issueNumber]...), nil
}

func (m *flowGitHubMock) AddComment(_ context.Context, issueNumber int, body string) (*ghapi.IssueComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	comment := &ghapi.IssueComment{
		ID:   ghapi.Ptr(m.nextCommentID()),
		Body: ghapi.Ptr(body),
	}
	m.comments[issueNumber] = append(m.comments[issueNumber], comment)
	return comment, nil
}

func (m *flowGitHubMock) FindMarker(_ context.Context, issueNumber int, t marker.Type) (*gh.MarkerComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.markers[m.markerKey(issueNumber, t)], nil
}

func (m *flowGitHubMock) CreateOrUpdateMarker(_ context.Context, issueNumber int, mk *marker.Marker, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.markerKey(issueNumber, mk.Type)
	if existing, ok := m.markers[key]; ok {
		existing.Marker = mk
		existing.Comment.Body = ghapi.Ptr(body)
		for _, c := range m.comments[issueNumber] {
			if c.GetID() == existing.CommentID {
				c.Body = ghapi.Ptr(body)
				break
			}
		}
		return nil
	}

	comment := &ghapi.IssueComment{
		ID:   ghapi.Ptr(m.nextCommentID()),
		Body: ghapi.Ptr(body),
	}
	m.comments[issueNumber] = append(m.comments[issueNumber], comment)
	m.markers[key] = &gh.MarkerComment{
		Marker:    mk,
		CommentID: comment.GetID(),
		Comment:   comment,
	}
	return nil
}

func (m *flowGitHubMock) ListLabels(_ context.Context, issueNumber int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.labels[issueNumber]...), nil
}

func (m *flowGitHubMock) AddLabel(_ context.Context, issueNumber int, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.labels[issueNumber] = append(m.labels[issueNumber], label)
	return nil
}

func (m *flowGitHubMock) RemoveLabel(_ context.Context, issueNumber int, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	labels := m.labels[issueNumber]
	for i, item := range labels {
		if item == label {
			m.labels[issueNumber] = append(labels[:i], labels[i+1:]...)
			break
		}
	}
	return nil
}

func (m *flowGitHubMock) ReplaceLabel(_ context.Context, issueNumber int, oldLabel, newLabel string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	labels := m.labels[issueNumber]
	for i, item := range labels {
		if item == oldLabel {
			m.labels[issueNumber][i] = newLabel
			return nil
		}
	}
	m.labels[issueNumber] = append(m.labels[issueNumber], newLabel)
	return nil
}

func (m *flowGitHubMock) ReplaceLabelIfPresent(_ context.Context, issueNumber int, oldLabel, newLabel string) (bool, error) {
	labels := m.labels[issueNumber]
	for i, item := range labels {
		if item == oldLabel {
			m.labels[issueNumber][i] = newLabel
			return true, nil
		}
	}
	return false, nil
}

func (m *flowGitHubMock) ReplaceLabels(_ context.Context, issueNumber int, labels []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.labels[issueNumber] = append([]string(nil), labels...)
	return nil
}

func (m *flowGitHubMock) EnsureLabelsExist(_ context.Context, _ []string) error {
	return nil
}

// 以下 PR 相关方法为接口桩，本测试不会触发。
func (m *flowGitHubMock) CreatePR(_ context.Context, _, _, _, _ string) (*ghapi.PullRequest, error) {
	return nil, fmt.Errorf("unexpected CreatePR call")
}

func (m *flowGitHubMock) GetPRDiff(_ context.Context, _ int) (string, error) {
	return "", nil
}

func (m *flowGitHubMock) CreatePRReview(_ context.Context, _ int, _, _ string) (*ghapi.PullRequestReview, error) {
	return nil, fmt.Errorf("unexpected CreatePRReview call")
}

func (m *flowGitHubMock) ListPRReviews(_ context.Context, _ int) ([]*ghapi.PullRequestReview, error) {
	return nil, nil
}

func (m *flowGitHubMock) ListIssuesWithLabel(_ context.Context, _ string) ([]*ghapi.Issue, error) {
	return nil, nil
}

func (m *flowGitHubMock) MergePR(_ context.Context, _ int, _ string) error {
	return nil
}

// controlFlowGitHubMock 为 control.ProcessIssue 集成回归提供可观测断言。
type controlFlowGitHubMock struct {
	mu                sync.Mutex
	issues            map[int]control.IssueInfo
	labels            map[int][]string
	blockedBy         map[int]map[int]struct{}
	commentBodies     map[int][]string
	stateTransitions  map[int]int
	replaceLabelCalls map[int]int
	replaceLabelHook  func(issueNumber int)
}

func newControlFlowGitHubMock(issueNumber int) *controlFlowGitHubMock {
	return &controlFlowGitHubMock{
		issues: map[int]control.IssueInfo{
			issueNumber: {
				Number: issueNumber,
				Title:  fmt.Sprintf("issue %d", issueNumber),
				State:  "open",
			},
		},
		labels: map[int][]string{
			issueNumber: {"bot:queued"},
		},
		blockedBy:         map[int]map[int]struct{}{},
		commentBodies:     map[int][]string{},
		stateTransitions:  map[int]int{},
		replaceLabelCalls: map[int]int{},
	}
}

func (m *controlFlowGitHubMock) ListIssuesWithLabel(_ context.Context, label string) ([]control.IssueInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]control.IssueInfo, 0)
	for issueNum, issue := range m.issues {
		for _, item := range m.labels[issueNum] {
			if item == label {
				result = append(result, issue)
				break
			}
		}
	}
	return result, nil
}

func (m *controlFlowGitHubMock) ListIssuesByState(_ context.Context, stateName string) ([]control.IssueInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if strings.EqualFold(stateName, "all") {
		result := make([]control.IssueInfo, 0, len(m.issues))
		for _, issue := range m.issues {
			result = append(result, issue)
		}
		return result, nil
	}

	result := make([]control.IssueInfo, 0)
	for _, issue := range m.issues {
		if strings.EqualFold(issue.State, stateName) {
			result = append(result, issue)
		}
	}
	return result, nil
}

func (m *controlFlowGitHubMock) GetIssue(_ context.Context, issueNumber int) (control.IssueInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	issue, ok := m.issues[issueNumber]
	if !ok {
		return control.IssueInfo{}, fmt.Errorf("issue #%d not found", issueNumber)
	}
	return issue, nil
}

func (m *controlFlowGitHubMock) UpdateIssueBody(_ context.Context, issueNumber int, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	issue, ok := m.issues[issueNumber]
	if !ok {
		return fmt.Errorf("issue #%d not found", issueNumber)
	}
	issue.Body = body
	m.issues[issueNumber] = issue
	return nil
}

func (m *controlFlowGitHubMock) ListCommentBodies(_ context.Context, issueNumber int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.commentBodies[issueNumber]...), nil
}

func (m *controlFlowGitHubMock) AddIssueComment(_ context.Context, issueNumber int, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.issues[issueNumber]; !ok {
		return fmt.Errorf("issue #%d not found", issueNumber)
	}
	m.commentBodies[issueNumber] = append(m.commentBodies[issueNumber], body)
	return nil
}

func (m *controlFlowGitHubMock) CloseIssue(_ context.Context, issueNumber int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	issue, ok := m.issues[issueNumber]
	if !ok {
		return fmt.Errorf("issue #%d not found", issueNumber)
	}
	issue.State = "closed"
	m.issues[issueNumber] = issue
	return nil
}

func (m *controlFlowGitHubMock) MergePR(_ context.Context, _ int, _ string) error {
	return nil
}

func (m *controlFlowGitHubMock) ListLabels(_ context.Context, issueNumber int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.labels[issueNumber]...), nil
}

func (m *controlFlowGitHubMock) AddLabel(_ context.Context, issueNumber int, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.labels[issueNumber] = append(m.labels[issueNumber], label)
	return nil
}

func (m *controlFlowGitHubMock) ReplaceLabel(_ context.Context, issueNumber int, oldLabel, newLabel string) error {
	m.mu.Lock()
	labels := m.labels[issueNumber]
	m.replaceLabelCalls[issueNumber]++
	for i, item := range labels {
		if item == oldLabel {
			if item != newLabel {
				m.labels[issueNumber][i] = newLabel
				m.stateTransitions[issueNumber]++
			}
			hook := m.replaceLabelHook
			m.mu.Unlock()
			if hook != nil {
				hook(issueNumber)
			}
			return nil
		}
	}
	m.labels[issueNumber] = append(m.labels[issueNumber], newLabel)
	hook := m.replaceLabelHook
	m.mu.Unlock()
	if hook != nil {
		hook(issueNumber)
	}
	return nil
}

func (m *controlFlowGitHubMock) ReplaceLabelIfPresent(_ context.Context, issueNumber int, oldLabel, newLabel string) (bool, error) {
	m.mu.Lock()
	labels := m.labels[issueNumber]
	m.replaceLabelCalls[issueNumber]++
	for i, item := range labels {
		if item == oldLabel {
			if item != newLabel {
				m.labels[issueNumber][i] = newLabel
				m.stateTransitions[issueNumber]++
			}
			hook := m.replaceLabelHook
			m.mu.Unlock()
			if hook != nil {
				hook(issueNumber)
			}
			return true, nil
		}
	}
	m.mu.Unlock()
	return false, nil
}

func (m *controlFlowGitHubMock) ReplaceLabels(_ context.Context, issueNumber int, labels []string) error {
	m.mu.Lock()
	m.replaceLabelCalls[issueNumber]++
	next := append([]string(nil), labels...)
	if !equalLabels(next, m.labels[issueNumber]) {
		m.stateTransitions[issueNumber]++
	}
	m.labels[issueNumber] = next
	if _, ok := m.issues[issueNumber]; !ok {
		m.issues[issueNumber] = control.IssueInfo{Number: issueNumber, State: "open"}
	}
	hook := m.replaceLabelHook
	m.mu.Unlock()
	if hook != nil {
		hook(issueNumber)
	}
	return nil
}

func (m *controlFlowGitHubMock) ListIssueBlockedBy(_ context.Context, issueNumber int) ([]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := m.blockedBy[issueNumber]
	result := make([]int, 0, len(set))
	for dep := range set {
		result = append(result, dep)
	}
	return result, nil
}

func (m *controlFlowGitHubMock) AddIssueBlockedBy(_ context.Context, issueNumber int, blockedByIssueNumber int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.blockedBy[issueNumber]; !ok {
		m.blockedBy[issueNumber] = make(map[int]struct{})
	}
	m.blockedBy[issueNumber][blockedByIssueNumber] = struct{}{}
	return nil
}

func (m *controlFlowGitHubMock) RemoveIssueBlockedBy(_ context.Context, issueNumber int, blockedByIssueNumber int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if set, ok := m.blockedBy[issueNumber]; ok {
		delete(set, blockedByIssueNumber)
	}
	return nil
}

func (m *controlFlowGitHubMock) ResolvePRMetadata(_ context.Context, issueNumber int) (control.PRMetadata, error) {
	return control.PRMetadata{}, fmt.Errorf("issue #%d: %w", issueNumber, control.ErrPRMarkerNotFound)
}

func (m *controlFlowGitHubMock) ResolvePRReviewStatus(_ context.Context, issueNumber int) (control.PRReviewStatus, error) {
	return control.PRReviewStatus{}, fmt.Errorf("issue #%d: %w", issueNumber, control.ErrPRMarkerNotFound)
}

func (m *controlFlowGitHubMock) stateTransitionCount(issueNumber int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stateTransitions[issueNumber]
}

func (m *controlFlowGitHubMock) replaceCallCount(issueNumber int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.replaceLabelCalls[issueNumber]
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

type flowIssueLockStore struct {
	mu    sync.Mutex
	locks map[int]control.IssueLockRecord
}

func newFlowIssueLockStore() *flowIssueLockStore {
	return &flowIssueLockStore{locks: make(map[int]control.IssueLockRecord)}
}

func (s *flowIssueLockStore) TryAcquire(issueNumber int, owner string, now time.Time, ttl time.Duration) (control.IssueLockRecord, bool, error) {
	if issueNumber <= 0 {
		return control.IssueLockRecord{}, false, fmt.Errorf("issue 编号无效: %d", issueNumber)
	}
	if strings.TrimSpace(owner) == "" {
		return control.IssueLockRecord{}, false, fmt.Errorf("owner 不能为空")
	}
	if ttl <= 0 {
		return control.IssueLockRecord{}, false, fmt.Errorf("ttl 必须大于 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.locks[issueNumber]
	if ok && record.Owner != "" && now.Before(record.ExpiresAt) {
		return record, false, nil
	}

	record = control.IssueLockRecord{
		IssueNumber: issueNumber,
		Owner:       owner,
		AcquiredAt:  now,
		HeartbeatAt: now,
		ExpiresAt:   now.Add(ttl),
		LastResult:  record.LastResult,
	}
	s.locks[issueNumber] = record
	return record, true, nil
}

func (s *flowIssueLockStore) Refresh(issueNumber int, owner string, now time.Time, ttl time.Duration) (control.IssueLockRecord, error) {
	if issueNumber <= 0 {
		return control.IssueLockRecord{}, fmt.Errorf("issue 编号无效: %d", issueNumber)
	}
	if strings.TrimSpace(owner) == "" {
		return control.IssueLockRecord{}, fmt.Errorf("owner 不能为空")
	}
	if ttl <= 0 {
		return control.IssueLockRecord{}, fmt.Errorf("ttl 必须大于 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.locks[issueNumber]
	if !ok || record.Owner == "" {
		return control.IssueLockRecord{}, fmt.Errorf("issue #%d 未持锁", issueNumber)
	}
	if record.Owner != owner {
		return control.IssueLockRecord{}, fmt.Errorf("issue #%d 锁 owner 不匹配: current=%s expect=%s", issueNumber, record.Owner, owner)
	}
	if !now.Before(record.ExpiresAt) {
		return control.IssueLockRecord{}, fmt.Errorf("issue #%d 锁已过期", issueNumber)
	}

	record.HeartbeatAt = now
	record.ExpiresAt = now.Add(ttl)
	s.locks[issueNumber] = record
	return record, nil
}

func (s *flowIssueLockStore) Release(issueNumber int, owner string, now time.Time, lastResult control.IssueLockResult) error {
	if issueNumber <= 0 {
		return nil
	}
	if strings.TrimSpace(owner) == "" {
		return fmt.Errorf("owner 不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.locks[issueNumber]
	if !ok {
		return nil
	}
	if record.Owner != "" && record.Owner != owner {
		return fmt.Errorf("issue #%d 锁 owner 不匹配: current=%s expect=%s", issueNumber, record.Owner, owner)
	}
	record.Owner = ""
	record.LastResult = lastResult
	record.HeartbeatAt = now
	record.ExpiresAt = now
	s.locks[issueNumber] = record
	return nil
}

type flowClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFlowClock(start time.Time) *flowClock {
	return &flowClock{now: start}
}

func (c *flowClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *flowClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func newDiscussFlowController(taskctl *control.TaskCtlClient, github control.GitHubOps, lockStore control.IssueLockStore, owner string, nowFn func() time.Time) *control.Controller {
	cfg := control.DefaultControlConfig()
	cfg.IssueLockStore = lockStore
	cfg.IssueLockTTL = discussFlowLockTTL
	cfg.IssueLockHeartbeatInterval = 0
	cfg.OwnerID = owner
	cfg.NowFn = nowFn

	return control.NewController(taskctl, nil, github, nil, cfg)
}

func newDiscussFlowTask(issueNumber int, inputHash string) control.Task {
	return control.Task{
		ID:      fmt.Sprintf("task-%d", issueNumber),
		Subject: fmt.Sprintf("issue %d", issueNumber),
		Metadata: map[string]string{
			"issue_num":  strconv.Itoa(issueNumber),
			"repo":       discussFlowDefaultRepo,
			"phase":      discussFlowDefaultPhase,
			"input_hash": inputHash,
		},
	}
}

func buildDiscussFlowIdempotencyKey(repo string, issueNum int, phase, inputHash string) string {
	payload := strings.Join([]string{
		strings.TrimSpace(repo),
		strconv.Itoa(issueNum),
		strings.TrimSpace(phase),
		strings.TrimSpace(inputHash),
	}, "\n")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func newDiscussFlowTaskCtlClient(t *testing.T, issueNumber int, inputHash string) (*control.TaskCtlClient, string) {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "taskctl.log")
	statePath := filepath.Join(dir, "state")
	binPath := filepath.Join(dir, "taskctl")
	idempotencyKey := buildDiscussFlowIdempotencyKey(
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
	list|ready)
		cat <<'JSON'
[{"id":"task-%d","subject":"issue %d","description":"issue %d","status":"pending","blocked_by":[],"metadata":{"issue_num":"%d","repo":"%s","phase":"%s","input_hash":"%s"}}]
JSON
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
	}, logPath
}

func countDiscussFlowTaskctlLogMatches(t *testing.T, logPath, target string) int {
	t.Helper()

	content, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		require.NoError(t, err)
	}

	count := 0
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, target) {
			count++
		}
	}
	return count
}

func captureDiscussFlowStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()

	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
	}()

	outputCh := make(chan string, 1)
	go func() {
		bytes, copyErr := io.ReadAll(r)
		if copyErr != nil {
			outputCh <- ""
			return
		}
		outputCh <- string(bytes)
	}()

	fn()
	require.NoError(t, w.Close())

	return <-outputCh
}

func TestDiscussFlow_SingleRunMultipleRoundsToPlanFinal(t *testing.T) {
	mockGH := newFlowGitHubMock()
	mockGH.issues[42] = &ghapi.Issue{
		Number: ghapi.Ptr(42),
		Title:  ghapi.Ptr("恢复讨论自动收敛"),
		Body:   ghapi.Ptr("needs-discussion 自动推进"),
	}
	mockGH.labels[42] = []string{string(state.StateNeedsDiscussion)}
	// 初始加入一条人工评论，模拟 issue_comment 事件触发。
	mockGH.comments[42] = []*ghapi.IssueComment{{
		ID:   ghapi.Ptr(int64(1)),
		Body: ghapi.Ptr("请开始自动多轮讨论并尽快收敛"),
	}}
	mockGH.nextID = 1

	mockAI := ai.NewMockProvider(
		"第一轮：补充限制。\n```json\n{\"should_finish\":false}\n```",
		"第二轮：已收敛。\n```json\n{\"should_finish\":true}\n```",
		`{"title":"最终方案","approach":"按共识执行","file_changes":[{"path":"automation/niuma/pkg/agent/orchestrator.go","action":"modify","description":"增加 discuss 多轮循环"}],"test_scenarios":[{"name":"自动收敛","input":"bot:needs-discussion","expected":"进入 bot:plan-final"}]}`,
	)

	orch := agent.NewOrchestrator(mockGH, mockAI, 42)
	err := orch.DoDiscuss(context.Background(), 5)
	require.NoError(t, err)

	// 单次 run 内两轮收敛，并进入 plan-final。
	labels, err := mockGH.ListLabels(context.Background(), 42)
	require.NoError(t, err)
	assert.Contains(t, labels, string(state.StatePlanFinal))
	assert.NotContains(t, labels, string(state.StateNeedsDiscussion))

	summaryMC := mockGH.markers[mockGH.markerKey(42, marker.TypeDiscussionSummary)]
	require.NotNil(t, summaryMC)
	assert.Equal(t, 2, summaryMC.Marker.Revision)

	finalMC := mockGH.markers[mockGH.markerKey(42, marker.TypePlanFinal)]
	require.NotNil(t, finalMC)
	assert.Contains(t, finalMC.Comment.GetBody(), "最终方案")
}

func TestDiscussFlow_Regression_RepeatTriggerOnlyOneEffectiveTransition(t *testing.T) {
	const issueNumber = 314
	const inputHash = "repeat-trigger"

	taskctl, logPath := newDiscussFlowTaskCtlClient(t, issueNumber, inputHash)
	mockGH := newControlFlowGitHubMock(issueNumber)
	clock := newFlowClock(time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC))
	ctrl := newDiscussFlowController(taskctl, mockGH, newFlowIssueLockStore(), "runner-a", clock.Now)
	task := newDiscussFlowTask(issueNumber, inputHash)

	output := captureDiscussFlowStdout(t, func() {
		for i := 0; i < 3; i++ {
			require.NoError(t, ctrl.ProcessIssue(context.Background(), task))
		}
	})

	assert.Equal(t, 1, mockGH.stateTransitionCount(issueNumber), "同一 issue 仅允许一次有效状态推进")
	assert.Equal(t, 1, mockGH.replaceCallCount(issueNumber), "重复触发必须在副作用前 no-op")
	assert.Equal(t, 1, countDiscussFlowTaskctlLogMatches(t, logPath, "--status in-progress"))
	assert.Contains(t, output, "[control][idempotency]")
	assert.Contains(t, output, "action=recorded")
	assert.Contains(t, output, "action=no-op")
}

func TestDiscussFlow_Regression_ConcurrentControlRunOnlyLockOwnerAdvances(t *testing.T) {
	const issueNumber = 315
	const inputHash = "concurrent-trigger"

	taskctl, logPath := newDiscussFlowTaskCtlClient(t, issueNumber, inputHash)
	mockGH := newControlFlowGitHubMock(issueNumber)
	store := newFlowIssueLockStore()
	clock := newFlowClock(time.Date(2026, 2, 18, 12, 30, 0, 0, time.UTC))

	ctrlA := newDiscussFlowController(taskctl, mockGH, store, "runner-a", clock.Now)
	ctrlB := newDiscussFlowController(taskctl, mockGH, store, "runner-b", clock.Now)

	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	mockGH.replaceLabelHook = func(_ int) {
		startedOnce.Do(func() {
			close(started)
		})
		<-release
	}

	output := captureDiscussFlowStdout(t, func() {
		firstErrCh := make(chan error, 1)
		go func() {
			firstErrCh <- ctrlA.Run(context.Background())
		}()

		select {
		case <-started:
		case <-time.After(discussFlowConcurrencyWaitMax):
			t.Fatal("等待首个 control run 进入副作用阶段超时")
		}

		secondErrCh := make(chan error, 1)
		go func() {
			secondErrCh <- ctrlB.Run(context.Background())
		}()

		select {
		case err := <-secondErrCh:
			require.NoError(t, err)
		case <-time.After(discussFlowConcurrencyWaitMax):
			t.Fatal("等待并发 control run 返回超时")
		}

		assert.Equal(t, 1, mockGH.stateTransitionCount(issueNumber), "并发抢锁场景只允许持锁实例推进")
		assert.Equal(t, 1, mockGH.replaceCallCount(issueNumber), "未持锁实例必须 no-op")
		close(release)
		require.NoError(t, <-firstErrCh)
	})

	assert.Equal(t, 1, countDiscussFlowTaskctlLogMatches(t, logPath, "--status in-progress"))
	assert.Contains(t, output, "[control][issue_lock]")
	assert.Contains(t, output, "status=skipped")
	assert.Contains(t, output, "reason=locked")
}

func TestDiscussFlow_Regression_LockExpiryRecoveryAfterTTLBuffer(t *testing.T) {
	const issueNumber = 316
	const inputHash = "ttl-recovery"

	taskctl, logPath := newDiscussFlowTaskCtlClient(t, issueNumber, inputHash)
	mockGH := newControlFlowGitHubMock(issueNumber)
	store := newFlowIssueLockStore()
	clock := newFlowClock(time.Date(2026, 2, 18, 13, 0, 0, 0, time.UTC))
	ctrl := newDiscussFlowController(taskctl, mockGH, store, "runner-recovery", clock.Now)
	task := newDiscussFlowTask(issueNumber, inputHash)

	_, acquired, err := store.TryAcquire(issueNumber, "runner-crashed", clock.Now(), discussFlowLockTTL)
	require.NoError(t, err)
	require.True(t, acquired)

	output := captureDiscussFlowStdout(t, func() {
		require.NoError(t, ctrl.ProcessIssue(context.Background(), task))
		assert.Equal(t, 0, mockGH.stateTransitionCount(issueNumber), "锁未过期前不得推进")

		clock.Advance(discussFlowLockTTL + discussFlowLockRecoveryBuffer)
		require.NoError(t, ctrl.ProcessIssue(context.Background(), task))
	})

	assert.Equal(t, 1, mockGH.stateTransitionCount(issueNumber), "TTL 过期后应恢复推进")
	assert.Equal(t, 1, mockGH.replaceCallCount(issueNumber))
	assert.Equal(t, 1, countDiscussFlowTaskctlLogMatches(t, logPath, "--status in-progress"))
	assert.Contains(t, output, "status=skipped")
	assert.Contains(t, output, "reason=locked")
	assert.Contains(t, output, "action=recorded")
}
