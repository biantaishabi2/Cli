package control

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/ai"
	"github.com/biantaishabi2/Cli/automation/niuma/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGitHubOps 用于 controller 测试的 GitHub mock
type mockGitHubOps struct {
	issues                    []IssueInfo
	issuesByLabel             map[string][]IssueInfo
	issuesByNumber            map[int]IssueInfo
	getIssueOverride          map[int]IssueInfo
	mergedPRs                 []int
	mergeError                map[int]error
	resolvePRMetadata         map[int]PRMetadata
	resolvePRMetadataErr      map[int]error
	resolvePRMetadataCall     []int
	resolvePRReviewStatus     map[int]PRReviewStatus
	resolvePRReviewStatusSeq  map[int][]PRReviewStatus
	resolvePRReviewStatusIdx  map[int]int
	resolvePRReviewStatusErr  map[int]error
	resolvePRReviewStatusCall []int
	replaceLabelCalls         []replaceLabelCall
	replaceLabelError         map[string]error
	replaceLabelPairError     map[string]error
	replaceLabelFails         map[string]int
	addLabelError             map[string]error
	addLabelFails             map[string]int
	updateIssueBodyCalls      []updateIssueBodyCall
	listCommentBodies         map[int][]string
	addIssueCommentCalls      []addIssueCommentCall
	closeIssueCalls           []int
	closeIssueError           map[int]error
	blockedBy                 map[int]map[int]struct{}
	listBlockedByErr          map[int]error
	addBlockedByErr           map[string]error
	removeBlockedByErr        map[string]error
	addBlockedByCalls         []blockedByCall
	removeBlockedByCalls      []blockedByCall
	listBlockedByCalls        int
	replaceLabelHook          func()
}

type replaceLabelCall struct {
	issueNumber int
	oldLabel    string
	newLabel    string
}

type updateIssueBodyCall struct {
	issueNumber int
	body        string
}

type addIssueCommentCall struct {
	issueNumber int
	body        string
}

type blockedByCall struct {
	issueNumber          int
	blockedByIssueNumber int
}

func newMockGitHubOps(issues ...IssueInfo) *mockGitHubOps {
	issuesByNumber := make(map[int]IssueInfo, len(issues))
	for _, issue := range issues {
		issuesByNumber[issue.Number] = issue
	}

	return &mockGitHubOps{
		issues:                   issues,
		issuesByLabel:            make(map[string][]IssueInfo),
		issuesByNumber:           issuesByNumber,
		getIssueOverride:         make(map[int]IssueInfo),
		mergeError:               make(map[int]error),
		resolvePRMetadata:        make(map[int]PRMetadata),
		resolvePRMetadataErr:     make(map[int]error),
		resolvePRReviewStatus:    make(map[int]PRReviewStatus),
		resolvePRReviewStatusSeq: make(map[int][]PRReviewStatus),
		resolvePRReviewStatusIdx: make(map[int]int),
		resolvePRReviewStatusErr: make(map[int]error),
		replaceLabelError:        make(map[string]error),
		replaceLabelPairError:    make(map[string]error),
		replaceLabelFails:        make(map[string]int),
		addLabelError:            make(map[string]error),
		addLabelFails:            make(map[string]int),
		listCommentBodies:        make(map[int][]string),
		closeIssueError:          make(map[int]error),
		blockedBy:                make(map[int]map[int]struct{}),
		listBlockedByErr:         make(map[int]error),
		addBlockedByErr:          make(map[string]error),
		removeBlockedByErr:       make(map[string]error),
	}
}

func (m *mockGitHubOps) ListIssuesWithLabel(_ context.Context, label string) ([]IssueInfo, error) {
	if issues, ok := m.issuesByLabel[label]; ok {
		return issues, nil
	}
	return m.issues, nil
}

func (m *mockGitHubOps) ListIssuesByState(_ context.Context, state string) ([]IssueInfo, error) {
	if state == "all" {
		result := make([]IssueInfo, 0, len(m.issuesByNumber))
		for _, issue := range m.issuesByNumber {
			result = append(result, issue)
		}
		return result, nil
	}

	result := make([]IssueInfo, 0, len(m.issuesByNumber))
	for _, issue := range m.issuesByNumber {
		if strings.EqualFold(issue.State, state) {
			result = append(result, issue)
		}
	}
	return result, nil
}

func (m *mockGitHubOps) GetIssue(_ context.Context, issueNumber int) (IssueInfo, error) {
	if issue, ok := m.getIssueOverride[issueNumber]; ok {
		return issue, nil
	}
	issue, ok := m.issuesByNumber[issueNumber]
	if !ok {
		return IssueInfo{}, fmt.Errorf("issue #%d not found", issueNumber)
	}
	return issue, nil
}

func (m *mockGitHubOps) UpdateIssueBody(_ context.Context, issueNumber int, body string) error {
	m.updateIssueBodyCalls = append(m.updateIssueBodyCalls, updateIssueBodyCall{
		issueNumber: issueNumber,
		body:        body,
	})
	issue, ok := m.issuesByNumber[issueNumber]
	if ok {
		issue.Body = body
		m.issuesByNumber[issueNumber] = issue
	}
	return nil
}

func (m *mockGitHubOps) ListCommentBodies(_ context.Context, issueNumber int) ([]string, error) {
	bodies := m.listCommentBodies[issueNumber]
	copied := make([]string, len(bodies))
	copy(copied, bodies)
	return copied, nil
}

func (m *mockGitHubOps) AddIssueComment(_ context.Context, issueNumber int, body string) error {
	m.addIssueCommentCalls = append(m.addIssueCommentCalls, addIssueCommentCall{
		issueNumber: issueNumber,
		body:        body,
	})
	m.listCommentBodies[issueNumber] = append(m.listCommentBodies[issueNumber], body)
	return nil
}

func (m *mockGitHubOps) CloseIssue(_ context.Context, issueNumber int) error {
	if err, ok := m.closeIssueError[issueNumber]; ok {
		return err
	}
	issue, ok := m.issuesByNumber[issueNumber]
	if ok {
		issue.State = "closed"
		m.issuesByNumber[issueNumber] = issue
	}
	m.closeIssueCalls = append(m.closeIssueCalls, issueNumber)
	return nil
}

func (m *mockGitHubOps) MergePR(_ context.Context, prNum int, _ string) error {
	if err, ok := m.mergeError[prNum]; ok {
		return err
	}
	m.mergedPRs = append(m.mergedPRs, prNum)
	return nil
}

func (m *mockGitHubOps) ResolvePRMetadata(_ context.Context, issueNumber int) (PRMetadata, error) {
	m.resolvePRMetadataCall = append(m.resolvePRMetadataCall, issueNumber)
	if err, ok := m.resolvePRMetadataErr[issueNumber]; ok {
		return PRMetadata{}, err
	}
	if metadata, ok := m.resolvePRMetadata[issueNumber]; ok {
		return metadata, nil
	}
	return PRMetadata{}, ErrPRMarkerNotFound
}

func (m *mockGitHubOps) ResolvePRReviewStatus(_ context.Context, issueNumber int) (PRReviewStatus, error) {
	m.resolvePRReviewStatusCall = append(m.resolvePRReviewStatusCall, issueNumber)
	if err, ok := m.resolvePRReviewStatusErr[issueNumber]; ok {
		return PRReviewStatus{}, err
	}
	if statuses, ok := m.resolvePRReviewStatusSeq[issueNumber]; ok && len(statuses) > 0 {
		idx := m.resolvePRReviewStatusIdx[issueNumber]
		if idx >= len(statuses) {
			idx = len(statuses) - 1
		}
		m.resolvePRReviewStatusIdx[issueNumber] = idx + 1
		return statuses[idx], nil
	}
	if status, ok := m.resolvePRReviewStatus[issueNumber]; ok {
		return status, nil
	}
	return PRReviewStatus{}, ErrPRMarkerNotFound
}

func (m *mockGitHubOps) ListLabels(_ context.Context, issueNumber int) ([]string, error) {
	issue, ok := m.issuesByNumber[issueNumber]
	if !ok {
		return nil, nil
	}
	labels := make([]string, len(issue.Labels))
	copy(labels, issue.Labels)
	return labels, nil
}

func (m *mockGitHubOps) AddLabel(_ context.Context, issueNumber int, label string) error {
	if remaining := m.addLabelFails[label]; remaining > 0 {
		m.addLabelFails[label] = remaining - 1
		return fmt.Errorf("add label %q temporary failed", label)
	}
	if err, ok := m.addLabelError[label]; ok {
		return err
	}
	issue := m.issuesByNumber[issueNumber]
	issue.Labels = append(issue.Labels, label)
	m.issuesByNumber[issueNumber] = issue
	return nil
}

func (m *mockGitHubOps) replaceLabelCore(issueNumber int, oldLabel, newLabel string, addWhenMissing bool) (bool, error) {
	m.replaceLabelCalls = append(m.replaceLabelCalls, replaceLabelCall{
		issueNumber: issueNumber,
		oldLabel:    oldLabel,
		newLabel:    newLabel,
	})
	if err, ok := m.replaceLabelPairError[fmt.Sprintf("%s=>%s", oldLabel, newLabel)]; ok {
		return false, err
	}

	if remaining := m.replaceLabelFails[newLabel]; remaining > 0 {
		m.replaceLabelFails[newLabel] = remaining - 1
		return false, fmt.Errorf("replace label %q temporary failed", newLabel)
	}
	if err, ok := m.replaceLabelError[newLabel]; ok {
		return false, err
	}

	issue, ok := m.issuesByNumber[issueNumber]
	if !ok {
		if m.replaceLabelHook != nil {
			m.replaceLabelHook()
		}
		return false, nil
	}
	found := false
	filtered := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		if label == oldLabel {
			found = true
			continue
		}
		if label == newLabel {
			continue
		}
		filtered = append(filtered, label)
	}
	if found || addWhenMissing {
		if newLabel != "" {
			filtered = append(filtered, newLabel)
		}
		issue.Labels = filtered
		m.issuesByNumber[issueNumber] = issue
	}
	if m.replaceLabelHook != nil {
		m.replaceLabelHook()
	}
	return found, nil
}

func (m *mockGitHubOps) ReplaceLabel(_ context.Context, issueNumber int, oldLabel, newLabel string) error {
	_, err := m.replaceLabelCore(issueNumber, oldLabel, newLabel, true)
	return err
}

func (m *mockGitHubOps) ReplaceLabelIfPresent(_ context.Context, issueNumber int, oldLabel, newLabel string) (bool, error) {
	return m.replaceLabelCore(issueNumber, oldLabel, newLabel, false)
}

func (m *mockGitHubOps) ReplaceLabels(_ context.Context, issueNumber int, labels []string) error {
	issue, ok := m.issuesByNumber[issueNumber]
	if !ok {
		issue = IssueInfo{Number: issueNumber}
	}
	oldBot := ""
	for _, label := range issue.Labels {
		if strings.HasPrefix(label, "bot:") {
			oldBot = label
			break
		}
	}
	next := make([]string, 0, len(labels))
	seen := make(map[string]struct{}, len(labels))
	newBot := ""
	for _, label := range labels {
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		next = append(next, label)
		if newBot == "" && strings.HasPrefix(label, "bot:") {
			newBot = label
		}
	}

	m.replaceLabelCalls = append(m.replaceLabelCalls, replaceLabelCall{
		issueNumber: issueNumber,
		oldLabel:    oldBot,
		newLabel:    newBot,
	})
	if err, ok := m.replaceLabelPairError[fmt.Sprintf("%s=>%s", oldBot, newBot)]; ok {
		return err
	}
	if remaining := m.replaceLabelFails[newBot]; remaining > 0 {
		m.replaceLabelFails[newBot] = remaining - 1
		return fmt.Errorf("replace label %q temporary failed", newBot)
	}
	if err, ok := m.replaceLabelError[newBot]; ok {
		return err
	}

	issue.Labels = next
	m.issuesByNumber[issueNumber] = issue
	if m.replaceLabelHook != nil {
		m.replaceLabelHook()
	}
	return nil
}

func (m *mockGitHubOps) ListIssueBlockedBy(_ context.Context, issueNumber int) ([]int, error) {
	m.listBlockedByCalls++
	if err, ok := m.listBlockedByErr[issueNumber]; ok {
		return nil, err
	}
	blockedSet := m.blockedBy[issueNumber]
	result := make([]int, 0, len(blockedSet))
	for dep := range blockedSet {
		result = append(result, dep)
	}
	return result, nil
}

func (m *mockGitHubOps) AddIssueBlockedBy(_ context.Context, issueNumber int, blockedByIssueNumber int) error {
	m.addBlockedByCalls = append(m.addBlockedByCalls, blockedByCall{
		issueNumber:          issueNumber,
		blockedByIssueNumber: blockedByIssueNumber,
	})
	key := fmt.Sprintf("%d->%d", blockedByIssueNumber, issueNumber)
	if err, ok := m.addBlockedByErr[key]; ok {
		return err
	}
	if _, ok := m.blockedBy[issueNumber]; !ok {
		m.blockedBy[issueNumber] = make(map[int]struct{})
	}
	m.blockedBy[issueNumber][blockedByIssueNumber] = struct{}{}
	return nil
}

func (m *mockGitHubOps) RemoveIssueBlockedBy(_ context.Context, issueNumber int, blockedByIssueNumber int) error {
	m.removeBlockedByCalls = append(m.removeBlockedByCalls, blockedByCall{
		issueNumber:          issueNumber,
		blockedByIssueNumber: blockedByIssueNumber,
	})
	key := fmt.Sprintf("%d->%d", blockedByIssueNumber, issueNumber)
	if err, ok := m.removeBlockedByErr[key]; ok {
		return err
	}
	if set, ok := m.blockedBy[issueNumber]; ok {
		delete(set, blockedByIssueNumber)
	}
	return nil
}

// mockTaskCtlClient 用于 controller 测试的 taskctl mock
type mockTaskCtlClient struct {
	tasks                  []Task
	nextID                 int
	createCallCount        int
	readyList              []Task
	readyCallCount         int
	readyHook              func([]Task) error
	failBlockedByForTaskID map[string]error
}

func newMockTaskCtlClient() *mockTaskCtlClient {
	return &mockTaskCtlClient{
		failBlockedByForTaskID: make(map[string]error),
	}
}

func (m *mockTaskCtlClient) create(subject, desc string, meta map[string]string) (*Task, error) {
	m.createCallCount++
	issueNum, hasIssue := parseIssueNum(meta)
	if hasIssue {
		for i := range m.tasks {
			if m.tasks[i].IssueNum() == issueNum && isTaskActiveStatus(m.tasks[i].Status) {
				task := m.tasks[i]
				return &task, nil
			}
		}
	}

	m.nextID++
	task := Task{
		ID:       fmt.Sprintf("task-%d", m.nextID),
		Subject:  subject,
		Desc:     desc,
		Status:   TaskStatusPending,
		Metadata: meta,
	}
	m.tasks = append(m.tasks, task)
	return &task, nil
}

func (m *mockTaskCtlClient) update(taskID string, opts UpdateOpts) error {
	if opts.BlockedBy != nil {
		if err, ok := m.failBlockedByForTaskID[taskID]; ok {
			return err
		}
	}
	for i := range m.tasks {
		if m.tasks[i].ID == taskID {
			if opts.Status != nil {
				m.tasks[i].Status = *opts.Status
			}
			if opts.BlockedBy != nil {
				m.tasks[i].BlockedBy = *opts.BlockedBy
			}
			if opts.Metadata != nil {
				if m.tasks[i].Metadata == nil {
					m.tasks[i].Metadata = make(map[string]string)
				}
				for k, v := range *opts.Metadata {
					m.tasks[i].Metadata[k] = v
				}
			}
			return nil
		}
	}
	return fmt.Errorf("task %s not found", taskID)
}

func (m *mockTaskCtlClient) list(_ string) ([]Task, error) {
	return m.tasks, nil
}

func (m *mockTaskCtlClient) ready() ([]Task, error) {
	m.readyCallCount++
	if m.readyHook != nil {
		if err := m.readyHook(m.tasks); err != nil {
			return nil, err
		}
	}
	if m.readyList != nil {
		return m.readyList, nil
	}
	var ready []Task
	for _, t := range m.tasks {
		if t.Status == TaskStatusPending && len(t.BlockedBy) == 0 {
			ready = append(ready, t)
		}
	}
	return ready, nil
}

func newScriptTaskCtlClient(t *testing.T, listJSON, dagJSON string) *TaskCtlClient {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "taskctl")
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -e
cmd="$1"
case "$cmd" in
  list)
    cat <<'JSON'
%s
JSON
    ;;
  dag)
    cat <<'JSON'
%s
JSON
    ;;
  ready)
    echo '[]'
    ;;
  create)
    cat <<'JSON'
{"id":"task-created","subject":"created","description":"created","status":"pending","metadata":{"issue_num":"999"}}
JSON
    ;;
  update|get)
    echo '{}'
    ;;
  *)
    echo '[]'
    ;;
esac
`, listJSON, dagJSON)
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	return &TaskCtlClient{
		BinPath:   bin,
		StorePath: filepath.Join(tmp, "tasks.json"),
	}
}

// inMemController 使用内存 mock 创建 Controller（绕过真实 taskctl 二进制）
type inMemController struct {
	*Controller
	mockTaskCtl *mockTaskCtlClient
	mockGitHub  *mockGitHubOps
	mockAI      *ai.MockProvider
}

type heartbeatFailIssueLockStore struct {
	base              IssueLockStore
	mu                sync.Mutex
	refreshCount      int
	lastReleaseResult IssueLockResult
}

func newHeartbeatFailIssueLockStore() *heartbeatFailIssueLockStore {
	return &heartbeatFailIssueLockStore{
		base: newInMemoryIssueLockStore(),
	}
}

func (s *heartbeatFailIssueLockStore) TryAcquire(issueNumber int, owner string, now time.Time, ttl time.Duration) (IssueLockRecord, bool, error) {
	return s.base.TryAcquire(issueNumber, owner, now, ttl)
}

func (s *heartbeatFailIssueLockStore) Refresh(issueNumber int, owner string, now time.Time, ttl time.Duration) (IssueLockRecord, error) {
	s.mu.Lock()
	s.refreshCount++
	s.mu.Unlock()
	return IssueLockRecord{}, fmt.Errorf("inject refresh failure")
}

func (s *heartbeatFailIssueLockStore) Release(issueNumber int, owner string, now time.Time, lastResult IssueLockResult) error {
	s.mu.Lock()
	s.lastReleaseResult = lastResult
	s.mu.Unlock()
	return s.base.Release(issueNumber, owner, now, lastResult)
}

func (s *heartbeatFailIssueLockStore) RefreshCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshCount
}

func (s *heartbeatFailIssueLockStore) LastReleaseResult() IssueLockResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastReleaseResult
}

func newInMemController(issues []IssueInfo, aiResp string) *inMemController {
	mockTC := newMockTaskCtlClient()
	mockGH := newMockGitHubOps(issues...)

	var provider ai.Provider
	var mockAI *ai.MockProvider
	if aiResp != "" {
		mockAI = ai.NewMockProvider(aiResp)
		provider = mockAI
	}

	analyzer := NewDependencyAnalyzer(provider)

	// 我们需要包装 mockTaskCtlClient 使其被 Controller 使用
	// 但 Controller 使用 *TaskCtlClient，所以我们直接测试核心逻辑
	ctrl := &Controller{
		analyzer: analyzer,
		github:   mockGH,
		cfg:      DefaultControlConfig(),
	}

	return &inMemController{
		Controller:  ctrl,
		mockTaskCtl: mockTC,
		mockGitHub:  mockGH,
		mockAI:      mockAI,
	}
}

// RunInMem 模拟 Run 逻辑但使用内存 mock
func (c *inMemController) RunInMem(ctx context.Context) error {
	issues, _, err := c.collectAutomationIssues(ctx)
	if err != nil {
		return err
	}

	// 找新 issue（仅以 active task 去重）
	existing := make(map[int]bool)
	for _, t := range c.mockTaskCtl.tasks {
		if n := t.IssueNum(); n > 0 && isTaskActiveStatus(t.Status) {
			existing[n] = true
		}
	}

	var newIssues []IssueInfo
	for _, issue := range issues {
		if !existing[issue.Number] {
			newIssues = append(newIssues, issue)
		}
	}

	// 分析依赖：显式 depends-on 优先，AI 仅补全未声明依赖。
	analysis := c.buildDependencyAnalysis(ctx, issues)

	// 创建 task
	issueToTask := make(map[int]string)
	for _, t := range c.mockTaskCtl.tasks {
		if n := t.IssueNum(); n > 0 {
			issueToTask[n] = t.ID
		}
	}
	for _, issue := range newIssues {
		meta := map[string]string{"issue_num": strconv.Itoa(issue.Number)}
		task, err := c.mockTaskCtl.create(issue.Title, "", meta)
		if err != nil {
			continue
		}
		issueToTask[issue.Number] = task.ID
		if hasLabel(issue.Labels, string(state.StateOrchestrate)) {
			_ = state.TransitionBotState(ctx, c.mockGitHub, issue.Number, state.StateOrchestrate, state.StateQueued)
		}
	}

	// 先落盘 blocked_by，再决定是否推进 ready。
	blockedByPersisted := true
	for issueNum, deps := range analysis.Dependencies {
		taskID, ok := issueToTask[issueNum]
		if !ok {
			blockedByPersisted = false
			continue
		}
		var blockedBy []string
		for _, dep := range deps {
			depTaskID, ok := issueToTask[dep]
			if !ok {
				blockedByPersisted = false
				continue
			}
			blockedBy = append(blockedBy, depTaskID)
		}
		if len(blockedBy) > 0 {
			if err := c.mockTaskCtl.update(taskID, UpdateOpts{BlockedBy: &blockedBy}); err != nil {
				blockedByPersisted = false
			}
		} else if len(deps) > 0 {
			blockedByPersisted = false
		}
	}

	// 推进 ready tasks
	if !blockedByPersisted {
		return nil
	}
	readyTasks, err := c.mockTaskCtl.ready()
	if err != nil {
		return err
	}
	for _, task := range readyTasks {
		status := TaskStatusInProgress
		if err := c.mockTaskCtl.update(task.ID, UpdateOpts{Status: &status}); err != nil {
			return err
		}
		if issueNum := task.IssueNum(); issueNum > 0 {
			_ = state.TransitionBotState(ctx, c.mockGitHub, issueNum, state.StateQueued, state.StateFixRequested)
		}
	}

	return nil
}

func newIssueLockTestController(owner string, store IssueLockStore, ttl, heartbeat time.Duration, nowFn func() time.Time) *Controller {
	if store == nil {
		store = newInMemoryIssueLockStore()
	}
	if owner == "" {
		owner = "test-owner"
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if heartbeat <= 0 {
		heartbeat = 100 * time.Second
	}
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Controller{
		issueLocks:         store,
		issueLockTTL:       ttl,
		issueLockHeartbeat: heartbeat,
		nowFn:              nowFn,
		ownerID:            owner,
	}
}

func newTaskCtlLoggingClient(t *testing.T) (*TaskCtlClient, string) {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "taskctl.log")
	binPath := filepath.Join(dir, "taskctl")
	script := fmt.Sprintf("#!/usr/bin/env bash\nset -e\nprintf '%%s\\n' \"$*\" >> %q\necho '{}'\n", logPath)
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	return &TaskCtlClient{
		BinPath:   binPath,
		StorePath: filepath.Join(dir, "tasks.json"),
	}, logPath
}

func newTaskCtlStatefulIdempotencyClient(t *testing.T, idempotencyKey string) (*TaskCtlClient, string) {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "taskctl.log")
	statePath := filepath.Join(dir, "state")
	binPath := filepath.Join(dir, "taskctl")
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
	get)
		if [[ "$state" == "recorded" ]]; then
			cat <<'JSON'
{"id":"task-314","subject":"issue 314","description":"issue 314","status":"pending","blocked_by":[],"metadata":{"issue_num":"314","repo":"biantaishabi2/Cli","phase":"fix","input_hash":"abc","idempotency.key.fix":"%s"}}
JSON
		else
			cat <<'JSON'
{"id":"task-314","subject":"issue 314","description":"issue 314","status":"pending","blocked_by":[],"metadata":{"issue_num":"314","repo":"biantaishabi2/Cli","phase":"fix","input_hash":"abc"}}
JSON
		fi
		;;
	update)
		if [[ "$*" == *"idempotency.key.fix"* ]]; then
			printf 'recorded' > %q
		fi
		echo '{}'
		;;
	*)
		echo '{}'
		;;
esac
`, logPath, statePath, statePath, idempotencyKey, statePath)
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	return &TaskCtlClient{
		BinPath:   binPath,
		StorePath: filepath.Join(dir, "tasks.json"),
	}, logPath
}

func newTaskCtlGetFailClient(t *testing.T) (*TaskCtlClient, string) {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "taskctl.log")
	binPath := filepath.Join(dir, "taskctl")
	script := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
cmd="${1:-}"
if [[ -n "$cmd" ]]; then
	shift
fi
printf '%%s %%s\n' "$cmd" "$*" >> %q
if [[ "$cmd" == "get" ]]; then
	echo "get failed" >&2
	exit 1
fi
echo '{}'
`, logPath)
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	return &TaskCtlClient{
		BinPath:   binPath,
		StorePath: filepath.Join(dir, "tasks.json"),
	}, logPath
}

func countTaskctlLogMatches(t *testing.T, logPath, target string) int {
	t.Helper()

	content, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		require.NoError(t, err)
	}

	count := 0
	for _, line := range splitNonEmpty(string(content)) {
		if strings.Contains(line, target) {
			count++
		}
	}
	return count
}

func captureControllerStdout(t *testing.T, fn func()) string {
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
		data, copyErr := io.ReadAll(r)
		if copyErr != nil {
			outputCh <- ""
			return
		}
		outputCh <- string(data)
	}()

	fn()
	require.NoError(t, w.Close())

	return <-outputCh
}

func TestController_WithIssueLock_MutualExclusion(t *testing.T) {
	store := newInMemoryIssueLockStore()
	ctrl := newIssueLockTestController("owner-a", store, 5*time.Minute, 100*time.Second, time.Now)
	issue := IssueInfo{Number: 315}

	started := make(chan struct{})
	done := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		errCh <- ctrl.withIssueLock(context.Background(), issue, func(context.Context) error {
			close(started)
			<-done
			return nil
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("等待首个持锁流程超时")
	}

	secondExecuted := false
	output := captureControllerStdout(t, func() {
		err := ctrl.withIssueLock(context.Background(), issue, func(context.Context) error {
			secondExecuted = true
			return nil
		})
		require.NoError(t, err)
	})
	assert.False(t, secondExecuted)
	assert.Contains(t, output, "[control][issue_lock]")
	assert.Contains(t, output, "status=skipped")
	assert.Contains(t, output, "reason=locked")

	close(done)
	require.NoError(t, <-errCh)
}

func TestController_WithIssueLock_ReleasesAfterCompletion(t *testing.T) {
	ctrl := newIssueLockTestController("owner-a", nil, 5*time.Minute, 100*time.Second, time.Now)
	issue := IssueInfo{Number: 315}

	runCount := 0
	err := ctrl.withIssueLock(context.Background(), issue, func(context.Context) error {
		runCount++
		return nil
	})
	require.NoError(t, err)

	err = ctrl.withIssueLock(context.Background(), issue, func(context.Context) error {
		runCount++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, runCount)
}

func TestController_IssueLock_TTLExpiryRecovery(t *testing.T) {
	store := newInMemoryIssueLockStore()
	ttl := 5 * time.Minute

	var mu sync.Mutex
	now := time.Date(2026, 2, 18, 0, 0, 0, 0, time.UTC)
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	advance := func(d time.Duration) {
		mu.Lock()
		now = now.Add(d)
		mu.Unlock()
	}

	ctrlA := newIssueLockTestController("owner-a", store, ttl, 100*time.Second, nowFn)
	ctrlB := newIssueLockTestController("owner-b", store, ttl, 100*time.Second, nowFn)
	issue := IssueInfo{Number: 315}

	acquired, _, err := ctrlA.tryAcquireIssueLock(issue)
	require.NoError(t, err)
	require.True(t, acquired)

	acquired, _, err = ctrlB.tryAcquireIssueLock(issue)
	require.NoError(t, err)
	assert.False(t, acquired)

	advance(ttl + time.Second)

	acquired, _, err = ctrlB.tryAcquireIssueLock(issue)
	require.NoError(t, err)
	assert.True(t, acquired)
}

func TestController_WithIssueLock_HeartbeatRefreshFailureReturnsError(t *testing.T) {
	store := newHeartbeatFailIssueLockStore()
	ctrl := newIssueLockTestController("owner-a", store, 5*time.Minute, 10*time.Millisecond, time.Now)
	issue := IssueInfo{Number: 315}

	err := ctrl.withIssueLock(context.Background(), issue, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "锁心跳刷新失败")
	assert.GreaterOrEqual(t, store.RefreshCount(), 1)
	assert.Equal(t, IssueLockResultFailed, store.LastReleaseResult())
}

func TestInMemoryIssueLockStore_ValidationAndOwnerMismatch(t *testing.T) {
	now := time.Date(2026, 2, 18, 0, 0, 0, 0, time.UTC)
	store := newInMemoryIssueLockStore()

	_, _, err := store.TryAcquire(0, "owner-a", now, time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "issue 编号无效")

	_, _, err = store.TryAcquire(1, "", now, time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "owner 不能为空")

	_, _, err = store.TryAcquire(1, "owner-a", now, 0)
	require.Error(t, err)
	assert.ErrorContains(t, err, "ttl 必须大于 0")

	_, err = store.Refresh(0, "owner-a", now, time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "issue 编号无效")

	_, err = store.Refresh(1, "", now, time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "owner 不能为空")

	_, err = store.Refresh(1, "owner-a", now, 0)
	require.Error(t, err)
	assert.ErrorContains(t, err, "ttl 必须大于 0")

	_, err = store.Refresh(1, "owner-a", now, time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "未持锁")

	_, acquired, err := store.TryAcquire(1, "owner-a", now, time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)

	_, err = store.Refresh(1, "owner-b", now.Add(10*time.Second), time.Minute)
	require.Error(t, err)
	assert.ErrorContains(t, err, "owner 不匹配")

	err = store.Release(1, "", now, IssueLockResultFailed)
	require.Error(t, err)
	assert.ErrorContains(t, err, "owner 不能为空")

	err = store.Release(1, "owner-b", now, IssueLockResultFailed)
	require.Error(t, err)
	assert.ErrorContains(t, err, "owner 不匹配")
}

func TestController_ProcessIssue_IdempotencyHitSkipsSideEffects(t *testing.T) {
	taskctl, logPath := newTaskCtlLoggingClient(t)
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		taskctl: taskctl,
		github:  mockGH,
	}

	repo := "biantaishabi2/Cli"
	phase := "fix"
	inputHash := "abc"
	idempotencyKey := buildIssueIdempotencyKey(repo, 314, phase, inputHash)
	task := Task{
		ID:      "task-314",
		Subject: "issue 314",
		Metadata: map[string]string{
			"issue_num":          "314",
			metaKeyTaskRepo:      repo,
			metaKeyTaskPhase:     phase,
			metaKeyTaskInputHash: inputHash,
			phaseScopedMetadataKey(metadataKeyIdempotencyKeyPrefix, phase): idempotencyKey,
		},
	}

	output := captureControllerStdout(t, func() {
		err := ctrl.ProcessIssue(context.Background(), task)
		require.NoError(t, err)
	})
	assert.Len(t, mockGH.replaceLabelCalls, 0)
	assert.Equal(t, 0, countTaskctlLogMatches(t, logPath, "--status in-progress"))
	assert.Equal(t, 0, countTaskctlLogMatches(t, logPath, "idempotency.key."+phase))
	assert.Contains(t, output, "[control][idempotency]")
	assert.Contains(t, output, "action=no-op")
}

func TestController_ProcessIssue_IdempotencyGetFailureStopsSideEffects(t *testing.T) {
	taskctl, logPath := newTaskCtlGetFailClient(t)
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		taskctl: taskctl,
		github:  mockGH,
	}

	task := Task{
		ID:      "task-314",
		Subject: "issue 314",
		Metadata: map[string]string{
			"issue_num":          "314",
			metaKeyTaskRepo:      "biantaishabi2/Cli",
			metaKeyTaskPhase:     "fix",
			metaKeyTaskInputHash: "abc",
		},
	}

	err := ctrl.ProcessIssue(context.Background(), task)
	require.Error(t, err)
	assert.ErrorContains(t, err, "读取任务最新 metadata 失败")
	assert.Len(t, mockGH.replaceLabelCalls, 0)
	assert.Equal(t, 0, countTaskctlLogMatches(t, logPath, "--status in-progress"))
	assert.Equal(t, 0, countTaskctlLogMatches(t, logPath, "idempotency.key.fix"))
	assert.Equal(t, 1, countTaskctlLogMatches(t, logPath, "get --task-id task-314"))
}

func TestController_ProcessIssue_IdempotencyInputHashChangedAllowsReprocess(t *testing.T) {
	taskctl, logPath := newTaskCtlLoggingClient(t)
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		taskctl: taskctl,
		github:  mockGH,
	}

	task := Task{
		ID:      "task-314",
		Subject: "issue 314",
		Metadata: map[string]string{
			"issue_num":          "314",
			metaKeyTaskRepo:      "biantaishabi2/Cli",
			metaKeyTaskPhase:     "fix",
			metaKeyTaskInputHash: "abc",
		},
	}

	err := ctrl.ProcessIssue(context.Background(), task)
	require.NoError(t, err)

	task.Metadata[metaKeyTaskInputHash] = "def"
	err = ctrl.ProcessIssue(context.Background(), task)
	require.NoError(t, err)

	assert.Len(t, mockGH.replaceLabelCalls, 1)
	assert.Equal(t, 2, countTaskctlLogMatches(t, logPath, "--status in-progress"))
	assert.Equal(
		t,
		buildIssueIdempotencyKey("biantaishabi2/Cli", 314, "fix", "def"),
		task.Metadata[phaseScopedMetadataKey(metadataKeyIdempotencyKeyPrefix, "fix")],
	)
}

func TestController_ProcessIssue_IdempotencyBackfillsLegacyMetadataAndNoopsOnRepeat(t *testing.T) {
	taskctl, logPath := newTaskCtlLoggingClient(t)
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		taskctl: taskctl,
		github:  mockGH,
		nowFn: func() time.Time {
			return time.Date(2026, 2, 18, 12, 0, 0, 0, time.UTC)
		},
	}

	task := Task{
		ID:      "task-314",
		Subject: "issue 314",
		Metadata: map[string]string{
			"issue_num":          "314",
			metaKeyTaskRepo:      "biantaishabi2/Cli",
			metaKeyTaskPhase:     "fix",
			metaKeyTaskInputHash: "abc",
		},
	}

	err := ctrl.ProcessIssue(context.Background(), task)
	require.NoError(t, err)
	err = ctrl.ProcessIssue(context.Background(), task)
	require.NoError(t, err)

	idempotencyKeyMeta := phaseScopedMetadataKey(metadataKeyIdempotencyKeyPrefix, "fix")
	idempotencyHashMeta := phaseScopedMetadataKey(metadataKeyIdempotencyInputHashPrefix, "fix")
	idempotencyTimeMeta := phaseScopedMetadataKey(metadataKeyIdempotencyTimestampPrefix, "fix")
	assert.Equal(t, buildIssueIdempotencyKey("biantaishabi2/Cli", 314, "fix", "abc"), task.Metadata[idempotencyKeyMeta])
	assert.Equal(t, "abc", task.Metadata[idempotencyHashMeta])
	assert.Equal(t, "2026-02-18T12:00:00Z", task.Metadata[idempotencyTimeMeta])

	assert.Len(t, mockGH.replaceLabelCalls, 1)
	assert.Equal(t, 1, countTaskctlLogMatches(t, logPath, "--status in-progress"))
	assert.Equal(t, 1, countTaskctlLogMatches(t, logPath, "idempotency.key.fix"))
}

func TestController_ProcessIssue_IdempotencyConcurrentDuplicateUsesLatestSnapshot(t *testing.T) {
	repo := "biantaishabi2/Cli"
	phase := "fix"
	inputHash := "abc"
	idempotencyKey := buildIssueIdempotencyKey(repo, 314, phase, inputHash)
	taskctl, logPath := newTaskCtlStatefulIdempotencyClient(t, idempotencyKey)
	mockGH := newMockGitHubOps()
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	mockGH.replaceLabelHook = func() {
		startedOnce.Do(func() {
			close(started)
		})
		<-release
	}
	ctrl := &Controller{
		taskctl:            taskctl,
		github:             mockGH,
		issueLocks:         newInMemoryIssueLockStore(),
		issueLockTTL:       5 * time.Minute,
		issueLockHeartbeat: 0,
		ownerID:            "test-owner",
	}
	task := Task{
		ID:      "task-314",
		Subject: "issue 314",
		Metadata: map[string]string{
			"issue_num":          "314",
			metaKeyTaskRepo:      repo,
			metaKeyTaskPhase:     phase,
			metaKeyTaskInputHash: inputHash,
		},
	}
	staleTask := Task{
		ID:      "task-314",
		Subject: "issue 314",
		Metadata: map[string]string{
			"issue_num":          "314",
			metaKeyTaskRepo:      repo,
			metaKeyTaskPhase:     phase,
			metaKeyTaskInputHash: inputHash,
		},
	}

	firstErrCh := make(chan error, 1)
	go func() {
		firstErrCh <- ctrl.ProcessIssue(context.Background(), task)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("等待首个流程进入副作用阶段超时")
	}

	secondErrCh := make(chan error, 1)
	go func() {
		secondErrCh <- ctrl.ProcessIssue(context.Background(), staleTask)
	}()

	select {
	case err := <-secondErrCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("等待并发重复触发返回超时")
	}

	assert.Len(t, mockGH.replaceLabelCalls, 1)
	close(release)
	require.NoError(t, <-firstErrCh)

	err := ctrl.ProcessIssue(context.Background(), staleTask)
	require.NoError(t, err)

	assert.Len(t, mockGH.replaceLabelCalls, 1)
	assert.Equal(t, 1, countTaskctlLogMatches(t, logPath, "--status in-progress"))
	assert.Equal(t, 1, countTaskctlLogMatches(t, logPath, "idempotency.key.fix"))
	assert.Equal(t, 2, countTaskctlLogMatches(t, logPath, "get --task-id task-314"))
}

func TestController_ProcessIssue_RepeatedWakeupLockThenIdempotencyNoop(t *testing.T) {
	repo := "biantaishabi2/Cli"
	phase := "fix"
	inputHash := "abc"
	idempotencyKey := buildIssueIdempotencyKey(repo, 314, phase, inputHash)
	taskctl, logPath := newTaskCtlStatefulIdempotencyClient(t, idempotencyKey)
	mockGH := newMockGitHubOps()
	store := newInMemoryIssueLockStore()
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	mockGH.replaceLabelHook = func() {
		startedOnce.Do(func() {
			close(started)
		})
		<-release
	}

	ctrlA := &Controller{
		taskctl:            taskctl,
		github:             mockGH,
		issueLocks:         store,
		issueLockTTL:       5 * time.Minute,
		issueLockHeartbeat: 0,
		ownerID:            "runner-a",
	}
	ctrlB := &Controller{
		taskctl:            taskctl,
		github:             mockGH,
		issueLocks:         store,
		issueLockTTL:       5 * time.Minute,
		issueLockHeartbeat: 0,
		ownerID:            "runner-b",
	}

	task := Task{
		ID:      "task-314",
		Subject: "issue 314",
		Metadata: map[string]string{
			"issue_num":          "314",
			metaKeyTaskRepo:      repo,
			metaKeyTaskPhase:     phase,
			metaKeyTaskInputHash: inputHash,
		},
	}

	firstErrCh := make(chan error, 1)
	go func() {
		firstErrCh <- ctrlA.ProcessIssue(context.Background(), task)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("等待首个唤醒进入副作用阶段超时")
	}

	lockOutput := captureControllerStdout(t, func() {
		err := ctrlB.ProcessIssue(context.Background(), task)
		require.NoError(t, err)
	})
	assert.Contains(t, lockOutput, "[control][issue_lock]")
	assert.Contains(t, lockOutput, "status=skipped")
	assert.Contains(t, lockOutput, "reason=locked")

	close(release)
	require.NoError(t, <-firstErrCh)

	idempotencyOutput := captureControllerStdout(t, func() {
		err := ctrlB.ProcessIssue(context.Background(), task)
		require.NoError(t, err)
	})
	assert.Contains(t, idempotencyOutput, "[control][idempotency]")
	assert.Contains(t, idempotencyOutput, "action=no-op")

	assert.Len(t, mockGH.replaceLabelCalls, 1)
	assert.Equal(t, 1, countTaskctlLogMatches(t, logPath, "--status in-progress"))
	assert.Equal(t, 1, countTaskctlLogMatches(t, logPath, "idempotency.key.fix"))
}

func TestController_ProcessIssue_IssueLockTTLRecoveryAfterSkip(t *testing.T) {
	taskctl, logPath := newTaskCtlLoggingClient(t)
	mockGH := newMockGitHubOps()
	store := newInMemoryIssueLockStore()
	ttl := 30 * time.Second

	var mu sync.Mutex
	now := time.Date(2026, 2, 18, 0, 0, 0, 0, time.UTC)
	nowFn := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	advance := func(d time.Duration) {
		mu.Lock()
		now = now.Add(d)
		mu.Unlock()
	}

	holder := &Controller{
		issueLocks:         store,
		issueLockTTL:       ttl,
		issueLockHeartbeat: 0,
		nowFn:              nowFn,
		ownerID:            "owner-a",
	}
	acquired, _, err := holder.tryAcquireIssueLock(IssueInfo{Number: 314})
	require.NoError(t, err)
	require.True(t, acquired)

	ctrl := &Controller{
		taskctl:            taskctl,
		github:             mockGH,
		issueLocks:         store,
		issueLockTTL:       ttl,
		issueLockHeartbeat: 0,
		nowFn:              nowFn,
		ownerID:            "owner-b",
	}
	task := Task{
		ID:      "task-314",
		Subject: "issue 314",
		Metadata: map[string]string{
			"issue_num":          "314",
			metaKeyTaskRepo:      "biantaishabi2/Cli",
			metaKeyTaskPhase:     "fix",
			metaKeyTaskInputHash: "abc",
		},
	}

	firstOutput := captureControllerStdout(t, func() {
		require.NoError(t, ctrl.ProcessIssue(context.Background(), task))
	})
	assert.Len(t, mockGH.replaceLabelCalls, 0)
	assert.Equal(t, 0, countTaskctlLogMatches(t, logPath, "--status in-progress"))
	assert.Contains(t, firstOutput, "status=skipped")
	assert.Contains(t, firstOutput, "reason=locked")

	advance(ttl + 5*time.Second)

	secondOutput := captureControllerStdout(t, func() {
		require.NoError(t, ctrl.ProcessIssue(context.Background(), task))
	})
	assert.Len(t, mockGH.replaceLabelCalls, 1)
	assert.Equal(t, 1, countTaskctlLogMatches(t, logPath, "--status in-progress"))
	assert.Contains(t, secondOutput, "[control][idempotency]")
	assert.Contains(t, secondOutput, "action=recorded")
}
func TestController_NewIssueDiscovery(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth fix", Body: "fix auth"},
		{Number: 41, Title: "Payment fix", Body: "fix payment"},
		{Number: 42, Title: "Test fix", Body: "fix tests"},
	}

	ctrl := newInMemController(issues, "")
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	assert.Len(t, ctrl.mockTaskCtl.tasks, 3)
	for _, task := range ctrl.mockTaskCtl.tasks {
		assert.NotEmpty(t, task.ID)
		assert.Equal(t, TaskStatusInProgress, task.Status) // 无依赖，应被推进
	}
}

func TestController_DependencySetup(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Payment", Body: "fix payment"},
		{Number: 42, Title: "Tests", Body: "depends-on: #40"},
	}

	ctrl := newInMemController(issues, "")
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	// 找到 issue 42 的 task
	var task42 *Task
	for i := range ctrl.mockTaskCtl.tasks {
		if ctrl.mockTaskCtl.tasks[i].IssueNum() == 42 {
			task42 = &ctrl.mockTaskCtl.tasks[i]
		}
	}
	require.NotNil(t, task42)
	assert.Len(t, task42.BlockedBy, 1)
	// 42 应该仍为 pending（因为有依赖）
	assert.Equal(t, TaskStatusPending, task42.Status)
}

func TestController_Idempotent(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
	}

	ctrl := newInMemController(issues, "")

	// 第一次 Run
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)
	assert.Len(t, ctrl.mockTaskCtl.tasks, 1)

	// 第二次 Run（不应创建新 task）
	err = ctrl.RunInMem(context.Background())
	require.NoError(t, err)
	assert.Len(t, ctrl.mockTaskCtl.tasks, 1)
	assert.Equal(t, 1, ctrl.mockTaskCtl.createCallCount)
}

func TestController_IntakeOnlyOrchestrateIssues(t *testing.T) {
	issues := []IssueInfo{
		{Number: 101, Title: "entry", Labels: []string{"bot:orchestrate"}},
		{Number: 102, Title: "single", Labels: []string{"bot:implementing"}},
	}
	ctrl := newInMemController(issues, "")
	ctrl.mockGitHub.issuesByLabel[string(state.StateOrchestrate)] = []IssueInfo{issues[0]}

	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	require.Len(t, ctrl.mockTaskCtl.tasks, 1)
	assert.Equal(t, 101, ctrl.mockTaskCtl.tasks[0].IssueNum())
}

func TestController_ReadyProgressDoesNotDowngradeImplementing(t *testing.T) {
	ctrl := newInMemController([]IssueInfo{
		{Number: 312, Title: "polluted", Labels: []string{"bot:implementing"}},
	}, "")
	ctrl.mockTaskCtl.tasks = []Task{
		{
			ID:       "task-312",
			Status:   TaskStatusPending,
			Metadata: map[string]string{"issue_num": "312"},
		},
	}

	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	labels, err := ctrl.mockGitHub.ListLabels(context.Background(), 312)
	require.NoError(t, err)
	assert.Equal(t, []string{"bot:implementing"}, labels)
	assert.Equal(t, TaskStatusInProgress, ctrl.mockTaskCtl.tasks[0].Status)
}

func TestController_ReadyTaskAdvance(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Tests", Body: "depends-on: #40"},
	}

	ctrl := newInMemController(issues, "")
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	// 40 应被推进（无依赖），41 应仍为 pending
	for _, task := range ctrl.mockTaskCtl.tasks {
		switch task.IssueNum() {
		case 40:
			assert.Equal(t, TaskStatusInProgress, task.Status)
		case 41:
			assert.Equal(t, TaskStatusPending, task.Status)
		}
	}
}

func TestController_ManualDependsOnWinsAndAIFillsUndeclared(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Payment", Body: "fix payment"},
		{Number: 42, Title: "Tests", Body: "depends-on: #40"},
	}
	aiResp := `{"dependencies":{"41":[40],"42":[41]},"potential_conflicts":[]}`

	ctrl := newInMemController(issues, aiResp)
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	taskByIssue := make(map[int]Task)
	taskIDByIssue := make(map[int]string)
	for _, task := range ctrl.mockTaskCtl.tasks {
		taskByIssue[task.IssueNum()] = task
		taskIDByIssue[task.IssueNum()] = task.ID
	}

	require.Contains(t, taskByIssue, 42)
	assert.Equal(t, []string{taskIDByIssue[40]}, taskByIssue[42].BlockedBy)
	assert.Equal(t, TaskStatusPending, taskByIssue[42].Status)
	assert.Equal(t, []string{taskIDByIssue[40]}, taskByIssue[41].BlockedBy)
}

func TestController_SubIssueWithParentOnlyUsesUnifiedAnalyzePath(t *testing.T) {
	issues := []IssueInfo{
		{Number: 210, Title: "parent", Body: "parent root"},
		{Number: 214, Title: "sub", Body: "parent: #210"},
	}
	aiResp := `{"dependencies":{"214":[210]},"potential_conflicts":[]}`

	ctrl := newInMemController(issues, aiResp)
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	require.NotNil(t, ctrl.mockAI)
	assert.Equal(t, 1, ctrl.mockAI.CallCount())
	assert.Contains(t, ctrl.mockAI.LastPrompt(), "#214")

	taskByIssue := make(map[int]Task)
	taskIDByIssue := make(map[int]string)
	for _, task := range ctrl.mockTaskCtl.tasks {
		taskByIssue[task.IssueNum()] = task
		taskIDByIssue[task.IssueNum()] = task.ID
	}

	assert.Equal(t, TaskStatusInProgress, taskByIssue[210].Status)
	assert.Equal(t, TaskStatusPending, taskByIssue[214].Status)
	assert.Equal(t, []string{taskIDByIssue[210]}, taskByIssue[214].BlockedBy)
}

func TestController_SubIssueParentDoesNotOverrideDependsOn(t *testing.T) {
	issues := []IssueInfo{
		{Number: 210, Title: "parent", Body: "parent root"},
		{Number: 300, Title: "base", Body: "base impl"},
		{Number: 214, Title: "sub", Body: "parent: #210\ndepends-on: #300"},
	}
	aiResp := `{"dependencies":{"214":[210]},"potential_conflicts":[]}`

	ctrl := newInMemController(issues, aiResp)
	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	taskByIssue := make(map[int]Task)
	taskIDByIssue := make(map[int]string)
	for _, task := range ctrl.mockTaskCtl.tasks {
		taskByIssue[task.IssueNum()] = task
		taskIDByIssue[task.IssueNum()] = task.ID
	}

	assert.Equal(t, []string{taskIDByIssue[300]}, taskByIssue[214].BlockedBy)
}

func TestController_ReadyWaitsForBlockedByPersistence(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Tests", Body: "depends-on: #40"},
	}

	ctrl := newInMemController(issues, "")
	ctrl.mockTaskCtl.readyHook = func(tasks []Task) error {
		for _, task := range tasks {
			if task.IssueNum() == 41 && len(task.BlockedBy) == 0 {
				return fmt.Errorf("ready called before blocked_by persisted for issue #41")
			}
		}
		return nil
	}

	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, ctrl.mockTaskCtl.readyCallCount)
}

func TestController_SkipReadyWhenBlockedByPersistFails(t *testing.T) {
	issues := []IssueInfo{
		{Number: 40, Title: "Auth", Body: "fix auth"},
		{Number: 41, Title: "Tests", Body: "depends-on: #40"},
	}

	ctrl := newInMemController(issues, "")
	ctrl.mockTaskCtl.failBlockedByForTaskID["task-2"] = fmt.Errorf("write blocked_by failed")

	err := ctrl.RunInMem(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, ctrl.mockTaskCtl.readyCallCount)
	for _, task := range ctrl.mockTaskCtl.tasks {
		assert.Equal(t, TaskStatusPending, task.Status)
	}
}

func TestController_AdvanceExistingTasksWithoutNewOrchestrateIssues(t *testing.T) {
	ctrl := newInMemController(nil, "")

	_, err := ctrl.mockTaskCtl.create("Existing queued task", "", map[string]string{"issue_num": "999"})
	require.NoError(t, err)

	err = ctrl.RunInMem(context.Background())
	require.NoError(t, err)

	require.Len(t, ctrl.mockTaskCtl.tasks, 1)
	assert.Equal(t, TaskStatusInProgress, ctrl.mockTaskCtl.tasks[0].Status)
}

func TestFormatStatus(t *testing.T) {
	status := &ControlStatus{
		Dag: &DagGraph{},
		Tasks: []Task{
			{ID: "t1", Subject: "Auth fix", Status: TaskStatusCompleted, Metadata: map[string]string{"issue_num": "40", "pr_num": "10"}},
			{ID: "t2", Subject: "Payment fix", Status: TaskStatusInProgress, Metadata: map[string]string{"issue_num": "41"}},
		},
	}

	output := FormatStatus(status)
	assert.Contains(t, output, "#40")
	assert.Contains(t, output, "#41")
	assert.Contains(t, output, "completed")
	assert.Contains(t, output, string(TaskStatusInProgress))
}

func TestBuildEscalationMetadata_FirstWrite(t *testing.T) {
	outcome := MergeOutcome{
		Status:          MergeStatusEscalated,
		ExecutedAt:      "2026-02-17T10:00:00Z",
		ExecutorVersion: "integration-merge-executor/v1",
		Conflict: &ConflictSummary{
			Files:           []string{"docs/b.md", "docs/a.md"},
			TotalHunkCount:  3,
			Reason:          "核心文件语义冲突",
			SuggestedAction: "请人工处理",
		},
	}

	update := buildEscalationMetadata(nil, outcome)
	assert.Equal(t, string(MergeStatusEscalated), update[metaKeyIntegrationMergeStatus])
	assert.Equal(t, "integration-merge-executor/v1", update[metaKeyIntegrationExecutorVersion])
	assert.Equal(t, "2026-02-17T10:00:00Z", update[metaKeyIntegrationMergeExecutedAt])
	assert.Equal(t, "docs/a.md,docs/b.md", update[metaKeyIntegrationConflictFiles])
	assert.Equal(t, "3", update[metaKeyIntegrationConflictTotalHunks])
	assert.NotEmpty(t, update[metaKeyIntegrationConflictSummary])
	assert.NotEmpty(t, update[metaKeyIntegrationConflictRecordedAt])
}

func TestBuildEscalationMetadata_IdempotentRetry(t *testing.T) {
	outcome := MergeOutcome{
		Status:          MergeStatusEscalated,
		ExecutedAt:      "2026-02-17T10:00:00Z",
		ExecutorVersion: "integration-merge-executor/v1",
		Conflict: &ConflictSummary{
			Files:           []string{"docs/a.md"},
			TotalHunkCount:  1,
			Reason:          "复杂冲突",
			SuggestedAction: "人工处理",
		},
	}

	first := buildEscalationMetadata(nil, outcome)
	existing := make(map[string]string, len(first))
	for k, v := range first {
		existing[k] = v
	}

	second := buildEscalationMetadata(existing, outcome)
	assert.Empty(t, second)
}

func TestController_EscalateIntegrationConflict_LabelIdempotentRetry(t *testing.T) {
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   "/bin/true",
			StorePath: t.TempDir() + "/tasks.json",
		},
	}

	outcome := MergeOutcome{
		Status:          MergeStatusEscalated,
		SourceBranch:    "feat/41-core-b",
		ExecutedAt:      "2026-02-17T10:00:00Z",
		ExecutorVersion: "integration-merge-executor/v1",
		Conflict: &ConflictSummary{
			Files:           []string{"pkg/service.go"},
			TotalHunkCount:  1,
			Reason:          "复杂冲突",
			SuggestedAction: "人工处理",
		},
	}

	firstTask := Task{
		ID:       "task-1",
		Metadata: map[string]string{"issue_num": "41"},
	}
	ctrl.escalateIntegrationConflict(context.Background(), firstTask, outcome)
	labels, err := mockGH.ListLabels(context.Background(), 41)
	require.NoError(t, err)
	assert.Contains(t, labels, integrationConflictLabel)
	assert.Contains(t, labels, needsHumanLabel)

	retryTask := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                           "41",
			metaKeyIntegrationMergeStatus:         string(MergeStatusEscalated),
			metaKeyIntegrationMergeExecutedAt:     outcome.ExecutedAt,
			metaKeyIntegrationExecutorVersion:     outcome.ExecutorVersion,
			metaKeyIntegrationConflictRecordedAt:  "2026-02-17T10:00:01Z",
			metaKeyIntegrationConflictLabelSynced: "true",
		},
	}
	ctrl.escalateIntegrationConflict(context.Background(), retryTask, outcome)
	labels, err = mockGH.ListLabels(context.Background(), 41)
	require.NoError(t, err)
	assert.Equal(t, 1, countLabel(labels, integrationConflictLabel))
	assert.Equal(t, 1, countLabel(labels, needsHumanLabel))
}

func TestShouldRetryEscalationLabels(t *testing.T) {
	assert.True(t, shouldRetryEscalationLabels(map[string]string{
		metaKeyIntegrationMergeStatus: string(MergeStatusEscalated),
	}))
	assert.False(t, shouldRetryEscalationLabels(map[string]string{
		metaKeyIntegrationMergeStatus:         string(MergeStatusEscalated),
		metaKeyIntegrationConflictLabelSynced: "true",
	}))
	assert.False(t, shouldRetryEscalationLabels(map[string]string{
		metaKeyIntegrationMergeStatus: string(MergeStatusMerged),
	}))
}

func TestMergeOutcomeFromEscalatedTask_FromMetadata(t *testing.T) {
	task := Task{
		Metadata: map[string]string{
			"issue_num":                           "41",
			"pr_num":                              "410",
			"branch":                              "feat/41-core",
			metaKeyIntegrationMergeStatus:         string(MergeStatusEscalated),
			metaKeyIntegrationMergeExecutedAt:     "2026-02-17T10:00:00Z",
			metaKeyIntegrationExecutorVersion:     "integration-merge-executor/v1",
			metaKeyIntegrationConflictSummary:     `{"files":["pkg/service.go"],"total_hunk_count":2,"reason":"复杂冲突","suggested_action":"人工处理"}`,
			metaKeyIntegrationConflictLabelSynced: "false",
		},
	}

	outcome := mergeOutcomeFromEscalatedTask(task, "integration/main")
	assert.Equal(t, MergeStatusEscalated, outcome.Status)
	assert.Equal(t, "integration/main", outcome.IntegrationBranch)
	assert.Equal(t, "feat/41-core", outcome.SourceBranch)
	assert.Equal(t, 41, outcome.IssueNum)
	assert.Equal(t, 410, outcome.PRNum)
	require.NotNil(t, outcome.Conflict)
	assert.Equal(t, []string{"pkg/service.go"}, outcome.Conflict.Files)
	assert.Equal(t, 2, outcome.Conflict.TotalHunkCount)
	assert.Equal(t, "复杂冲突", outcome.Conflict.Reason)
}

func TestController_EscalateIntegrationConflict_LabelCanRetryAfterFailure(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.addLabelFails[integrationConflictLabel] = 1
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   "/bin/true",
			StorePath: t.TempDir() + "/tasks.json",
		},
	}

	task := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                   "41",
			metaKeyIntegrationMergeStatus: string(MergeStatusEscalated),
		},
	}
	outcome := MergeOutcome{
		Status:       MergeStatusEscalated,
		SourceBranch: "feat/41-core-b",
		Conflict: &ConflictSummary{
			Files:           []string{"pkg/service.go"},
			TotalHunkCount:  1,
			Reason:          "复杂冲突",
			SuggestedAction: "人工处理",
		},
	}

	ctrl.escalateIntegrationConflict(context.Background(), task, outcome)
	labels, err := mockGH.ListLabels(context.Background(), 41)
	require.NoError(t, err)
	assert.NotContains(t, labels, integrationConflictLabel)
	assert.NotContains(t, labels, needsHumanLabel)

	ctrl.escalateIntegrationConflict(context.Background(), task, outcome)
	labels, err = mockGH.ListLabels(context.Background(), 41)
	require.NoError(t, err)
	assert.Contains(t, labels, integrationConflictLabel)
	assert.Contains(t, labels, needsHumanLabel)
}

func TestHandleIntegrationGateFailure_TriggersRetryLabelUnderLimit(t *testing.T) {
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   "/bin/true",
			StorePath: t.TempDir() + "/tasks.json",
		},
		cfg: &ControlConfig{
			IntegrationGateMaxRetries: 2,
			RepoDir:                   t.TempDir(),
		},
	}

	task := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                         "41",
			metaKeyIntegrationGateRetryCount:    "0",
			metaKeyIntegrationGateAttemptKey:    "old",
			metaKeyIntegrationGateStatus:        integrationGateStatusPassed,
			metaKeyIntegrationGateLastError:     "",
			metaKeyIntegrationGateLastCheckedAt: "2026-02-17T10:00:00Z",
		},
	}

	err := ctrl.handleIntegrationGateFailure(context.Background(), task, "attempt-1", errors.New("gate failed once"))
	require.NoError(t, err)

	labels, err := mockGH.ListLabels(context.Background(), 41)
	require.NoError(t, err)
	assert.Contains(t, labels, "bot:pr-needs-fix")
	assert.NotContains(t, labels, integrationGateFailLabel)
}

func TestHandleIntegrationGateFailure_EscalatesWhenExceeded(t *testing.T) {
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   "/bin/true",
			StorePath: t.TempDir() + "/tasks.json",
		},
		cfg: &ControlConfig{
			IntegrationGateMaxRetries: 2,
			RepoDir:                   t.TempDir(),
		},
	}

	task := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                      "41",
			metaKeyIntegrationGateRetryCount: "2",
			metaKeyIntegrationGateAttemptKey: "old-attempt",
			metaKeyIntegrationGateStatus:     integrationGateStatusRetrying,
		},
	}

	err := ctrl.handleIntegrationGateFailure(context.Background(), task, "attempt-3", errors.New("gate failed third time"))
	require.NoError(t, err)
	labels, labelsErr := mockGH.ListLabels(context.Background(), 41)
	require.NoError(t, labelsErr)
	assert.Contains(t, labels, needsHumanLabel)
	assert.Contains(t, labels, integrationGateFailLabel)
}

func TestHandleIntegrationGateFailure_DuplicateAttemptNoop(t *testing.T) {
	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   "/bin/true",
			StorePath: t.TempDir() + "/tasks.json",
		},
		cfg: &ControlConfig{
			IntegrationGateMaxRetries: 2,
			RepoDir:                   t.TempDir(),
		},
	}

	task := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                      "41",
			metaKeyIntegrationGateRetryCount: "1",
			metaKeyIntegrationGateAttemptKey: "attempt-same",
			metaKeyIntegrationGateStatus:     integrationGateStatusRetrying,
		},
	}

	err := ctrl.handleIntegrationGateFailure(context.Background(), task, "attempt-same", errors.New("duplicate fail"))
	require.NoError(t, err)
	assert.Len(t, mockGH.replaceLabelCalls, 0)
}

func TestBuildIntegrationGateAttemptKey_IgnoresIntegrationHeadChanges(t *testing.T) {
	dir := setupGitRepo(t)
	createBranch(t, dir, "feat/41-gate", "feature.txt", "feature v1\n")

	runGit(t, dir, "checkout", "master")
	runGit(t, dir, "checkout", "-b", "integration/main")
	runGit(t, dir, "merge", "--no-ff", "feat/41-gate", "-m", "merge feat/41-gate")
	runGit(t, dir, "checkout", "master")

	ctrl := &Controller{
		cfg: &ControlConfig{
			RepoDir: dir,
		},
	}
	outcome := MergeOutcome{
		IntegrationBranch: "integration/main",
		SourceBranch:      "feat/41-gate",
	}
	firstKey, err := ctrl.buildIntegrationGateAttemptKey(outcome)
	require.NoError(t, err)

	createBranch(t, dir, "feat/99-other", "unrelated.txt", "other change\n")
	runGit(t, dir, "checkout", "integration/main")
	runGit(t, dir, "merge", "--no-ff", "feat/99-other", "-m", "merge feat/99-other")
	runGit(t, dir, "checkout", "master")

	secondKey, err := ctrl.buildIntegrationGateAttemptKey(outcome)
	require.NoError(t, err)
	assert.Equal(t, firstKey, secondKey)
}

func TestRunIntegrationGateAndDecide_FirstFailThenPass_MarksIntegrated(t *testing.T) {
	dir := setupGitRepo(t)

	runGit(t, dir, "checkout", "-b", "feat/41-gate")
	niumaDir := filepath.Join(dir, "automation", "niuma")
	require.NoError(t, os.MkdirAll(filepath.Join(niumaDir, "gate"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(niumaDir, "go.mod"), []byte("module example.com/niuma\n\ngo 1.20\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(niumaDir, "gate", "gate_test.go"), []byte(`package gate

import "testing"

func TestIntegrationGate(t *testing.T) {
	t.Fatalf("gate failed")
}
`), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add failing integration gate test")

	runGit(t, dir, "checkout", "-b", "integration/main")
	runGit(t, dir, "checkout", "master")

	binDir := t.TempDir()
	logPath := filepath.Join(binDir, "taskctl.log")
	binPath := filepath.Join(binDir, "taskctl")
	script := fmt.Sprintf("#!/usr/bin/env bash\nprintf '%%s\\n' \"$*\" >> %q\nexit 0\n", logPath)
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	mockGH := newMockGitHubOps()
	ctrl := &Controller{
		github: mockGH,
		taskctl: &TaskCtlClient{
			BinPath:   binPath,
			StorePath: filepath.Join(dir, ".niuma", "tasks.json"),
		},
		cfg: &ControlConfig{
			IntegrationGateMaxRetries: 2,
			RepoDir:                   dir,
		},
	}

	outcome := MergeOutcome{
		Status:            MergeStatusMerged,
		IntegrationBranch: "integration/main",
		SourceBranch:      "feat/41-gate",
		IssueNum:          41,
		PRNum:             410,
		ExecutorVersion:   "integration-merge-executor/v1",
		ExecutedAt:        "2026-02-17T10:00:00Z",
	}

	firstTask := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num": "41",
		},
	}
	integrated, err := ctrl.runIntegrationGateAndDecide(context.Background(), firstTask, outcome)
	require.NoError(t, err)
	assert.False(t, integrated)
	labels, err := mockGH.ListLabels(context.Background(), 41)
	require.NoError(t, err)
	assert.Contains(t, labels, "bot:pr-needs-fix")

	firstAttemptKey, err := ctrl.buildIntegrationGateAttemptKey(outcome)
	require.NoError(t, err)

	runGit(t, dir, "checkout", "feat/41-gate")
	require.NoError(t, os.WriteFile(filepath.Join(niumaDir, "gate", "gate_test.go"), []byte(`package gate

import "testing"

func TestIntegrationGate(t *testing.T) {}
`), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "fix integration gate test")
	runGit(t, dir, "checkout", "integration/main")
	runGit(t, dir, "merge", "--ff-only", "feat/41-gate")
	runGit(t, dir, "checkout", "master")

	secondAttemptKey, err := ctrl.buildIntegrationGateAttemptKey(outcome)
	require.NoError(t, err)
	assert.NotEqual(t, firstAttemptKey, secondAttemptKey)

	retryTask := Task{
		ID: "task-1",
		Metadata: map[string]string{
			"issue_num":                      "41",
			metaKeyIntegrationGateStatus:     integrationGateStatusRetrying,
			metaKeyIntegrationGateRetryCount: "1",
			metaKeyIntegrationGateAttemptKey: firstAttemptKey,
		},
	}
	integrated, err = ctrl.runIntegrationGateAndDecide(context.Background(), retryTask, outcome)
	require.NoError(t, err)
	assert.True(t, integrated)

	logContent, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(string(logContent)), `"integrated":"true"`)
}

func TestCollectAutomationIssues_OnlyOrchestrateEntry(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.issuesByLabel["bot:orchestrate"] = []IssueInfo{
		{Number: 214, Title: "entry task", State: "open", Labels: []string{"bot:orchestrate"}},
	}
	mockGH.issuesByLabel["bot:implementing"] = []IssueInfo{
		{Number: 215, Title: "single issue", State: "open", Labels: []string{"bot:implementing"}},
	}

	ctrl := &Controller{github: mockGH}
	issues, orchestrateCount, err := ctrl.collectAutomationIssues(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, orchestrateCount)
	require.Len(t, issues, 1)
	assert.Equal(t, 214, issues[0].Number)
}

func TestSyncPRReviewableMetadata_SyncsMetadataBeforeIntegration(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.resolvePRMetadata[321] = PRMetadata{
		PRNum:  123,
		Branch: "feat/321-fix",
	}

	taskctlClient, logPath := newRecordingTaskCtlClient(t)
	ctrl := &Controller{
		github:  mockGH,
		taskctl: taskctlClient,
	}

	tasks := []Task{
		{
			ID:       "task-321",
			Metadata: map[string]string{"issue_num": "321"},
		},
	}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.syncPRReviewableMetadata(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)

	assert.Equal(t, "123", tasks[0].Metadata["pr_num"])
	assert.Equal(t, "feat/321-fix", tasks[0].Metadata["branch"])
	assert.Equal(t, "main", tasks[0].Metadata["meta_issue_slug"])
	assert.Equal(t, []int{321}, mockGH.resolvePRMetadataCall)

	logContent, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(logContent), "--task-id task-321")
	assert.Contains(t, string(logContent), `"pr_num":"123"`)
	assert.Contains(t, string(logContent), `"branch":"feat/321-fix"`)
}

func TestSyncPRReviewableMetadata_IdempotentNoop(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.resolvePRMetadata[321] = PRMetadata{
		PRNum:  123,
		Branch: "feat/321-fix",
	}

	taskctlClient, logPath := newRecordingTaskCtlClient(t)
	ctrl := &Controller{
		github:  mockGH,
		taskctl: taskctlClient,
	}

	tasks := []Task{
		{
			ID: "task-321",
			Metadata: map[string]string{
				"issue_num":       "321",
				"pr_num":          "123",
				"branch":          "feat/321-fix",
				"meta_issue_slug": "main",
			},
		},
	}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.syncPRReviewableMetadata(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)
	assert.Equal(t, []int{321}, mockGH.resolvePRMetadataCall)

	logContent, err := os.ReadFile(logPath)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(string(logContent)))
}

func TestSyncPRReviewableMetadata_SkippableErrorsDoNotBlock(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.resolvePRMetadataErr[321] = ErrPRMarkerNotFound
	mockGH.resolvePRMetadata[322] = PRMetadata{
		PRNum:  124,
		Branch: "feat/322-fix",
	}

	taskctlClient, logPath := newRecordingTaskCtlClient(t)
	ctrl := &Controller{
		github:  mockGH,
		taskctl: taskctlClient,
	}

	tasks := []Task{
		{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}},
		{ID: "task-322", Metadata: map[string]string{"issue_num": "322"}},
	}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
		322: {Number: 322, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.syncPRReviewableMetadata(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)
	assert.Equal(t, "124", tasks[1].Metadata["pr_num"])

	logContent, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(logContent), "--task-id task-322")
	assert.NotContains(t, string(logContent), "--task-id task-321")
}

func TestSyncPRReviewableMetadata_APIFailureReturnsError(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.resolvePRMetadataErr[321] = errors.New("github api unavailable")

	taskctlClient, _ := newRecordingTaskCtlClient(t)
	ctrl := &Controller{
		github:  mockGH,
		taskctl: taskctlClient,
	}

	tasks := []Task{
		{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}},
	}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.syncPRReviewableMetadata(context.Background(), tasks, issueByNumber)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github api unavailable")
}

func TestSyncPRReviewableMetadata_PersistFailureReturnsError(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.resolvePRMetadata[321] = PRMetadata{
		PRNum:  123,
		Branch: "feat/321-fix",
	}

	ctrl := &Controller{
		github:  mockGH,
		taskctl: newFailingTaskCtlClient(t),
	}

	tasks := []Task{
		{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}},
	}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.syncPRReviewableMetadata(context.Background(), tasks, issueByNumber)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "持久化 PR 元数据失败")
}

func TestReconcilePRReviewableConflicts_ConflictRollbackAndCommentDedup(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Body:   "conflict body\n\n<!-- PR_CONFLICT_RETRY:1 -->",
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.resolvePRReviewStatus[321] = PRReviewStatus{
		PRNum:            123,
		HeadSHA:          "abc123",
		Mergeable:        PRMergeableConflicting,
		MergeStateStatus: "DIRTY",
	}

	ctrl := &Controller{
		github: mockGH,
		cfg: &ControlConfig{
			PRConflictRetryThreshold:  3,
			PRConflictUnknownBackoffs: []time.Duration{time.Millisecond},
		},
	}
	tasks := []Task{
		{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}},
	}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)

	assert.Equal(t, 2, parsePRConflictRetryCount(mockGH.issuesByNumber[321].Body))
	assert.Greater(t, countReplaceLabelByIssueAndTarget(mockGH.replaceLabelCalls, 321, "bot:pr-needs-fix"), 0)
	require.Len(t, mockGH.addIssueCommentCalls, 1)
	assert.Contains(t, mockGH.addIssueCommentCalls[0].body, "<!-- BOT:CONFLICT_DETECTED sha:abc123 -->")

	err = ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)
	assert.Len(t, mockGH.addIssueCommentCalls, 1)
}

func TestReconcilePRReviewableConflicts_MergeableNoopAndResetRetry(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Body:   "ok body\n\n<!-- PR_CONFLICT_RETRY:2 -->",
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.resolvePRReviewStatus[321] = PRReviewStatus{
		PRNum:            123,
		HeadSHA:          "def456",
		Mergeable:        PRMergeableMergeable,
		MergeStateStatus: "CLEAN",
	}

	ctrl := &Controller{
		github: mockGH,
		cfg: &ControlConfig{
			PRConflictRetryThreshold:  3,
			PRConflictUnknownBackoffs: []time.Duration{time.Millisecond},
		},
	}
	tasks := []Task{
		{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}},
	}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)

	assert.Equal(t, 0, parsePRConflictRetryCount(mockGH.issuesByNumber[321].Body))
	assert.Empty(t, mockGH.replaceLabelCalls)
	assert.Empty(t, mockGH.addIssueCommentCalls)
}

func TestReconcilePRReviewableConflicts_UnknownRetriesThenNoop(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Body:   "unknown body\n\n<!-- PR_CONFLICT_RETRY:2 -->",
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.resolvePRReviewStatusSeq[321] = []PRReviewStatus{
		{PRNum: 123, HeadSHA: "sha-unknown", Mergeable: PRMergeableUnknown, MergeStateStatus: "UNKNOWN"},
		{PRNum: 123, HeadSHA: "sha-unknown", Mergeable: PRMergeableUnknown, MergeStateStatus: "UNKNOWN"},
		{PRNum: 123, HeadSHA: "sha-unknown", Mergeable: PRMergeableUnknown, MergeStateStatus: "UNKNOWN"},
		{PRNum: 123, HeadSHA: "sha-unknown", Mergeable: PRMergeableUnknown, MergeStateStatus: "UNKNOWN"},
	}

	ctrl := &Controller{
		github: mockGH,
		cfg: &ControlConfig{
			PRConflictRetryThreshold:  3,
			PRConflictUnknownBackoffs: []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond},
		},
	}
	tasks := []Task{
		{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}},
	}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)

	assert.Len(t, mockGH.resolvePRReviewStatusCall, 4)
	assert.Equal(t, 2, parsePRConflictRetryCount(mockGH.issuesByNumber[321].Body))
	assert.Empty(t, mockGH.replaceLabelCalls)
	assert.Empty(t, mockGH.addIssueCommentCalls)
}

func TestReconcilePRReviewableConflicts_ExceedThresholdEscalatesNeedsHuman(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Body:   "escalate body\n\n<!-- PR_CONFLICT_RETRY:3 -->",
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.resolvePRReviewStatus[321] = PRReviewStatus{
		PRNum:            123,
		HeadSHA:          "sha-escalate",
		Mergeable:        PRMergeableConflicting,
		MergeStateStatus: "BLOCKED",
	}

	ctrl := &Controller{
		github: mockGH,
		cfg: &ControlConfig{
			PRConflictRetryThreshold:  3,
			PRConflictUnknownBackoffs: []time.Duration{time.Millisecond},
		},
	}
	tasks := []Task{
		{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}},
	}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)

	assert.Equal(t, 4, parsePRConflictRetryCount(mockGH.issuesByNumber[321].Body))
	assert.Greater(t, countReplaceLabelByIssueAndTarget(mockGH.replaceLabelCalls, 321, needsHumanLabel), 0)
	assert.Equal(t, 0, countReplaceLabelByIssueAndTarget(mockGH.replaceLabelCalls, 321, "bot:pr-needs-fix"))
	require.Len(t, mockGH.addIssueCommentCalls, 2)
	assert.Contains(t, mockGH.addIssueCommentCalls[1].body, "BOT:CONFLICT_ESCALATED")
}

func TestReconcilePRReviewableConflicts_LabelChangedSkipsTransition(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Body:   "race body\n\n<!-- PR_CONFLICT_RETRY:1 -->",
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.getIssueOverride[321] = IssueInfo{
		Number: 321,
		Body:   "race body\n\n<!-- PR_CONFLICT_RETRY:1 -->",
		Labels: []string{"bot:pr-needs-fix"},
	}
	mockGH.resolvePRReviewStatus[321] = PRReviewStatus{
		PRNum:            123,
		HeadSHA:          "sha-race",
		Mergeable:        PRMergeableConflicting,
		MergeStateStatus: "DIRTY",
	}

	ctrl := &Controller{
		github: mockGH,
		cfg: &ControlConfig{
			PRConflictRetryThreshold:  3,
			PRConflictUnknownBackoffs: []time.Duration{time.Millisecond},
		},
	}
	tasks := []Task{
		{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}},
	}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)

	assert.Empty(t, mockGH.replaceLabelCalls)
	assert.Empty(t, mockGH.updateIssueBodyCalls)
	assert.Empty(t, mockGH.addIssueCommentCalls)
}

func TestReconcilePRReviewableConflicts_LabelSyncFailedDoesNotPersistRetry(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Body:   "label-fail body\n\n<!-- PR_CONFLICT_RETRY:1 -->",
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.resolvePRReviewStatus[321] = PRReviewStatus{
		PRNum:            123,
		HeadSHA:          "sha-label-fail",
		Mergeable:        PRMergeableConflicting,
		MergeStateStatus: "DIRTY",
	}
	mockGH.replaceLabelError["bot:pr-needs-fix"] = errors.New("replace failed")

	ctrl := &Controller{
		github: mockGH,
		cfg: &ControlConfig{
			PRConflictRetryThreshold:  3,
			PRConflictUnknownBackoffs: []time.Duration{time.Millisecond},
		},
	}
	tasks := []Task{
		{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}},
	}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "状态回退失败")
	assert.Equal(t, 1, parsePRConflictRetryCount(mockGH.issuesByNumber[321].Body))
	assert.Empty(t, mockGH.updateIssueBodyCalls)
}

func TestReconcilePRReviewableConflicts_NoConflictFilesNoop(t *testing.T) {
	dir := setupGitRepo(t)
	taskctlClient, logPath := newRecordingTaskCtlClient(t)

	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Body:   "clean body\n\n<!-- PR_CONFLICT_RETRY:1 -->",
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.resolvePRReviewStatus[321] = PRReviewStatus{
		PRNum:            123,
		HeadSHA:          "sha-no-conflict",
		Mergeable:        PRMergeableConflicting,
		MergeStateStatus: "DIRTY",
	}

	ctrl := &Controller{
		github:  mockGH,
		taskctl: taskctlClient,
		cfg: &ControlConfig{
			RepoDir:                   dir,
			PRConflictEnableAI:        true,
			PRConflictAIMaxAttempts:   2,
			PRConflictRetryThreshold:  3,
			PRConflictUnknownBackoffs: []time.Duration{time.Millisecond},
		},
	}

	tasks := []Task{{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}}}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)
	assert.Empty(t, mockGH.replaceLabelCalls)
	assert.Empty(t, mockGH.addIssueCommentCalls)
	assert.Empty(t, mockGH.updateIssueBodyCalls)

	rawLog, readErr := os.ReadFile(logPath)
	if os.IsNotExist(readErr) {
		return
	}
	require.NoError(t, readErr)
	assert.Empty(t, strings.TrimSpace(string(rawLog)))
}

func TestReconcilePRReviewableConflicts_RuleLayerSuccess(t *testing.T) {
	dir := setupPRConflictImportRepo(t)
	taskctlClient, logPath := newRecordingTaskCtlClient(t)

	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Body:   "rule body",
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.resolvePRReviewStatus[321] = PRReviewStatus{
		PRNum:            123,
		HeadSHA:          "sha-rule-success",
		Mergeable:        PRMergeableConflicting,
		MergeStateStatus: "DIRTY",
	}

	ctrl := &Controller{
		github:  mockGH,
		taskctl: taskctlClient,
		cfg: &ControlConfig{
			RepoDir:                   dir,
			PRConflictEnableAI:        true,
			PRConflictAIMaxAttempts:   2,
			PRConflictRetryThreshold:  3,
			PRConflictUnknownBackoffs: []time.Duration{time.Millisecond},
		},
	}
	tasks := []Task{{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}}}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)

	unmerged := runGitOutput(t, dir, "diff", "--name-only", "--diff-filter=U")
	assert.Empty(t, unmerged)
	assert.Empty(t, mockGH.replaceLabelCalls)

	rawLog, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	logText := string(rawLog)
	assert.Contains(t, logText, metaKeyConflictResolutionLayer)
	assert.Contains(t, logText, conflictResolutionLayerRule)
	assert.Contains(t, logText, metaKeyConflictResolutionAttempts)
	assert.Contains(t, logText, "0")
}

func TestReconcilePRReviewableConflicts_AILayerSuccess(t *testing.T) {
	dir, conflictFile := setupPRConflictTestHelperRepo(t)
	taskctlClient, logPath := newRecordingTaskCtlClient(t)
	provider := ai.NewMockProvider(fmt.Sprintf(
		`{"edits":[{"path":"%s","content":"package pkg\n\nfunc helperValue() string {\n\treturn \"merged\"\n}\n"}]}`,
		conflictFile,
	))

	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Body:   "ai body",
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.resolvePRReviewStatus[321] = PRReviewStatus{
		PRNum:            123,
		HeadSHA:          "sha-ai-success",
		Mergeable:        PRMergeableConflicting,
		MergeStateStatus: "DIRTY",
	}

	ctrl := &Controller{
		github:   mockGH,
		taskctl:  taskctlClient,
		analyzer: NewDependencyAnalyzer(provider),
		cfg: &ControlConfig{
			RepoDir:                   dir,
			PRConflictEnableAI:        true,
			PRConflictAIMaxAttempts:   2,
			PRConflictRetryThreshold:  3,
			PRConflictUnknownBackoffs: []time.Duration{time.Millisecond},
		},
	}
	tasks := []Task{{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}}}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)

	unmerged := runGitOutput(t, dir, "diff", "--name-only", "--diff-filter=U")
	assert.Empty(t, unmerged)
	assert.Empty(t, mockGH.replaceLabelCalls)
	assert.GreaterOrEqual(t, len(mockGH.addIssueCommentCalls), 2)

	rawLog, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	logText := string(rawLog)
	assert.Contains(t, logText, metaKeyConflictResolutionLayer)
	assert.Contains(t, logText, conflictResolutionLayerAI)
	assert.Contains(t, logText, metaKeyConflictResolutionAttempts)
	assert.Contains(t, logText, "1")
}

func TestReconcilePRReviewableConflicts_AIExhaustedEscalatesHuman(t *testing.T) {
	dir, conflictFile := setupPRConflictTestHelperRepo(t)
	taskctlClient, logPath := newRecordingTaskCtlClient(t)
	provider := ai.NewMockProvider(fmt.Sprintf(
		`{"edits":[{"path":"%s","content":"package pkg\n\nfunc helperValue() string {\n\treturn missingSymbol1\n}\n"}]}`,
		conflictFile,
	), fmt.Sprintf(
		`{"edits":[{"path":"%s","content":"package pkg\n\nfunc helperValue() string {\n\treturn missingSymbol2\n}\n"}]}`,
		conflictFile,
	))

	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Body:   "ai exhausted body",
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.resolvePRReviewStatus[321] = PRReviewStatus{
		PRNum:            123,
		HeadSHA:          "sha-ai-exhausted",
		Mergeable:        PRMergeableConflicting,
		MergeStateStatus: "DIRTY",
	}

	ctrl := &Controller{
		github:   mockGH,
		taskctl:  taskctlClient,
		analyzer: NewDependencyAnalyzer(provider),
		cfg: &ControlConfig{
			RepoDir:                   dir,
			PRConflictEnableAI:        true,
			PRConflictAIMaxAttempts:   2,
			PRConflictRetryThreshold:  3,
			PRConflictUnknownBackoffs: []time.Duration{time.Millisecond},
		},
	}
	tasks := []Task{{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}}}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)

	labels, labelsErr := mockGH.ListLabels(context.Background(), 321)
	require.NoError(t, labelsErr)
	assert.Contains(t, labels, needsHumanLabel)
	assert.Equal(t, 2, provider.CallCount())

	resolvedContent, readErr := os.ReadFile(filepath.Join(dir, conflictFile))
	require.NoError(t, readErr)
	assert.Contains(t, string(resolvedContent), "<<<<<<<")

	rawLog, logErr := os.ReadFile(logPath)
	require.NoError(t, logErr)
	logText := string(rawLog)
	assert.Contains(t, logText, metaKeyConflictResolutionLayer)
	assert.Contains(t, logText, conflictResolutionLayerHuman)
	assert.Contains(t, logText, metaKeyConflictResolutionAttempts)
	assert.Contains(t, logText, "2")
	assert.Contains(t, logText, "质量门禁失败")
	assert.Contains(t, logText, metaKeyConflictResolutionLastFailedAt)
}

func TestReconcilePRReviewableConflicts_AIDisabledEscalatesHuman(t *testing.T) {
	dir, _ := setupPRConflictTestHelperRepo(t)
	taskctlClient, logPath := newRecordingTaskCtlClient(t)

	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Body:   "ai disabled body",
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.resolvePRReviewStatus[321] = PRReviewStatus{
		PRNum:            123,
		HeadSHA:          "sha-ai-disabled",
		Mergeable:        PRMergeableConflicting,
		MergeStateStatus: "DIRTY",
	}

	ctrl := &Controller{
		github:  mockGH,
		taskctl: taskctlClient,
		cfg: &ControlConfig{
			RepoDir:                   dir,
			PRConflictEnableAI:        false,
			PRConflictAIMaxAttempts:   2,
			PRConflictRetryThreshold:  3,
			PRConflictUnknownBackoffs: []time.Duration{time.Millisecond},
		},
	}
	tasks := []Task{{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}}}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}

	err := ctrl.reconcilePRReviewableConflicts(context.Background(), tasks, issueByNumber)
	require.NoError(t, err)

	labels, labelsErr := mockGH.ListLabels(context.Background(), 321)
	require.NoError(t, labelsErr)
	assert.Contains(t, labels, needsHumanLabel)

	rawLog, logErr := os.ReadFile(logPath)
	require.NoError(t, logErr)
	logText := string(rawLog)
	assert.Contains(t, logText, metaKeyConflictResolutionLayer)
	assert.Contains(t, logText, conflictResolutionLayerHuman)
	assert.Contains(t, logText, metaKeyConflictResolutionAttempts)
	assert.Contains(t, logText, "0")
	assert.Contains(t, logText, "AI 层已禁用")
}

func TestNewController_PRConflictAIMaxAttemptsDefaultsWhenNonPositive(t *testing.T) {
	cfgZero := &ControlConfig{
		RepoDir:                 ".",
		PRConflictEnableAI:      true,
		PRConflictAIMaxAttempts: 0,
	}
	ctrlZero := NewController(nil, nil, nil, nil, cfgZero)
	assert.Equal(t, prConflictAIDefaultMaxAttempts, cfgZero.PRConflictAIMaxAttempts)
	assert.Equal(t, prConflictAIDefaultMaxAttempts, ctrlZero.prConflictAIMaxAttempts())

	cfgNegative := &ControlConfig{
		RepoDir:                 ".",
		PRConflictEnableAI:      true,
		PRConflictAIMaxAttempts: -3,
	}
	ctrlNegative := NewController(nil, nil, nil, nil, cfgNegative)
	assert.Equal(t, prConflictAIDefaultMaxAttempts, cfgNegative.PRConflictAIMaxAttempts)
	assert.Equal(t, prConflictAIDefaultMaxAttempts, ctrlNegative.prConflictAIMaxAttempts())
}

func TestShouldEnqueueIntegrationMergeTask_UsesLatestIssueLabel(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 321,
			Labels: []string{"bot:pr-reviewable"},
		},
	)
	mockGH.getIssueOverride[321] = IssueInfo{
		Number: 321,
		Labels: []string{"bot:pr-needs-fix"},
	}

	ctrl := &Controller{github: mockGH}
	issueByNumber := map[int]IssueInfo{
		321: {Number: 321, Labels: []string{"bot:pr-reviewable"}},
	}
	task := Task{ID: "task-321", Metadata: map[string]string{"issue_num": "321"}}

	allowed := ctrl.shouldEnqueueIntegrationMergeTask(context.Background(), task, issueByNumber)
	assert.False(t, allowed)
	assert.Equal(t, []string{"bot:pr-needs-fix"}, issueByNumber[321].Labels)
}

func TestEnsureIssueCommentWithMarker_DedupByMarker(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.listCommentBodies[321] = []string{
		"已存在评论\n\n<!-- BOT:CONFLICT_DETECTED sha:abc123 -->",
	}
	ctrl := &Controller{github: mockGH}

	err := ctrl.ensureIssueCommentWithMarker(context.Background(), 321, "<!-- BOT:CONFLICT_DETECTED sha:abc123 -->", "new body")
	require.NoError(t, err)
	assert.Empty(t, mockGH.addIssueCommentCalls)
}

func TestFinalizeIntegratedIssues_ClosesSubAndParent(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{Number: 210, Title: "parent", State: "open"},
		IssueInfo{Number: 214, Title: "sub-214", Body: "parent: #210", State: "open", Labels: []string{"bot:pr-reviewable"}},
		IssueInfo{Number: 215, Title: "sub-215", Body: "parent: #210", State: "closed"},
	)

	ctrl := &Controller{github: mockGH}
	err := ctrl.FinalizeIntegratedIssues(context.Background(), []int{214})
	require.NoError(t, err)

	assert.Contains(t, mockGH.closeIssueCalls, 214)
	assert.Contains(t, mockGH.closeIssueCalls, 210)
	labels214, err := mockGH.ListLabels(context.Background(), 214)
	require.NoError(t, err)
	assert.Contains(t, labels214, botDoneLabel)
	labels210, err := mockGH.ListLabels(context.Background(), 210)
	require.NoError(t, err)
	assert.Contains(t, labels210, botDoneLabel)
}

func TestFinalizeIntegratedIssues_ParentRemainsOpenWhenSubNotAllClosed(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{Number: 210, Title: "parent", State: "open"},
		IssueInfo{Number: 214, Title: "sub-214", Body: "parent: #210", State: "open"},
		IssueInfo{Number: 215, Title: "sub-215", Body: "parent: #210", State: "open"},
	)

	ctrl := &Controller{github: mockGH}
	err := ctrl.FinalizeIntegratedIssues(context.Background(), []int{214})
	require.NoError(t, err)

	assert.Contains(t, mockGH.closeIssueCalls, 214)
	assert.NotContains(t, mockGH.closeIssueCalls, 210)
}

func TestFinalizeIntegratedIssues_ClosedIssueIsIdempotent(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{Number: 210, Title: "parent", State: "open"},
		IssueInfo{Number: 214, Title: "sub-214", Body: "parent: #210", State: "closed"},
		IssueInfo{Number: 215, Title: "sub-215", Body: "parent: #210", State: "open"},
	)

	ctrl := &Controller{github: mockGH}
	err := ctrl.FinalizeIntegratedIssues(context.Background(), []int{214})
	require.NoError(t, err)

	assert.NotContains(t, mockGH.closeIssueCalls, 214)
	assert.NotContains(t, mockGH.closeIssueCalls, 210)
}

func TestSyncIssueStateLabel_SkipsSelfReplacement(t *testing.T) {
	mockGH := newMockGitHubOps()
	mockGH.replaceLabelPairError["bot:fix=>bot:fix"] = errors.New("self replacement should not happen")
	mockGH.issuesByNumber[41] = IssueInfo{Number: 41}
	ctrl := &Controller{github: mockGH}

	err := ctrl.syncIssueStateLabel(context.Background(), 41, "bot:fix")
	require.NoError(t, err)
	labels, err := mockGH.ListLabels(context.Background(), 41)
	require.NoError(t, err)
	assert.Contains(t, labels, "bot:fix")
	for _, call := range mockGH.replaceLabelCalls {
		assert.NotEqual(t, call.oldLabel, call.newLabel)
	}
}

func TestSyncIssueStateLabel_AutoHealMultiStateAndDedupComment(t *testing.T) {
	mockGH := newMockGitHubOps(
		IssueInfo{
			Number: 41,
			Labels: []string{string(state.StateFixRequested), string(state.StateNeedsDiscussion)},
		},
	)
	ctrl := &Controller{github: mockGH}

	err := ctrl.syncIssueStateLabel(context.Background(), 41, string(state.StateNeedsDiscussion))
	require.NoError(t, err)
	labels, err := mockGH.ListLabels(context.Background(), 41)
	require.NoError(t, err)
	assert.Equal(t, []string{string(state.StateNeedsDiscussion)}, labels)
	require.Len(t, mockGH.addIssueCommentCalls, 1)
	assert.Contains(t, mockGH.addIssueCommentCalls[0].body, "状态自愈")

	err = ctrl.syncIssueStateLabel(context.Background(), 41, string(state.StateNeedsDiscussion))
	require.NoError(t, err)
	assert.Len(t, mockGH.addIssueCommentCalls, 1)
}

func setupPRConflictImportRepo(t *testing.T) string {
	t.Helper()

	dir := setupGitRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/prconflict\n\ngo 1.22\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "env.go"), []byte(`package pkg

import "os"

func Env() string {
	return os.Getenv("X")
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "env_test.go"), []byte(`package pkg

import "testing"

func TestEnv(t *testing.T) {
	_ = Env()
}
`), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add go module")

	runGit(t, dir, "checkout", "-b", "feat/import-conflict")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "env.go"), []byte(`package pkg

import (
	_ "fmt"
	"os"
)

func Env() string {
	return os.Getenv("X")
}
`), 0o644))
	runGit(t, dir, "add", "pkg/env.go")
	runGit(t, dir, "commit", "-m", "feat adds fmt import")

	runGit(t, dir, "checkout", "master")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "env.go"), []byte(`package pkg

import (
	"os"
	_ "strings"
)

func Env() string {
	return os.Getenv("X")
}
`), 0o644))
	runGit(t, dir, "add", "pkg/env.go")
	runGit(t, dir, "commit", "-m", "master adds strings import")

	runGitWithExpectedFailure(t, dir, "merge", "--no-ff", "feat/import-conflict")
	return dir
}

func setupPRConflictTestHelperRepo(t *testing.T) (string, string) {
	t.Helper()

	dir := setupGitRepo(t)
	conflictFile := filepath.ToSlash(filepath.Join("pkg", "helper_test.go"))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/prconflict\n\ngo 1.22\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "helper_test.go"), []byte(`package pkg

func helperValue() string {
	return "base"
}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pkg", "helper_value_test.go"), []byte(`package pkg

import "testing"

func TestHelperValue(t *testing.T) {
	_ = helperValue()
}
`), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add helper test")

	runGit(t, dir, "checkout", "-b", "feat/test-helper-conflict")
	require.NoError(t, os.WriteFile(filepath.Join(dir, conflictFile), []byte(`package pkg

func helperValue() string {
	return "feature"
}
`), 0o644))
	runGit(t, dir, "add", conflictFile)
	runGit(t, dir, "commit", "-m", "feature helper change")

	runGit(t, dir, "checkout", "master")
	require.NoError(t, os.WriteFile(filepath.Join(dir, conflictFile), []byte(`package pkg

func helperValue() int {
	return 1
}
`), 0o644))
	runGit(t, dir, "add", conflictFile)
	runGit(t, dir, "commit", "-m", "master helper change")

	runGitWithExpectedFailure(t, dir, "merge", "--no-ff", "feat/test-helper-conflict")
	return dir, conflictFile
}

func runGitWithExpectedFailure(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.Error(t, err, "git %v should fail", args)
	require.Contains(t, string(out), "CONFLICT", "git %v output=%s", args, string(out))
}

func countReplaceLabelByTarget(calls []replaceLabelCall, label string) int {
	count := 0
	for _, call := range calls {
		if call.newLabel == label {
			count++
		}
	}
	return count
}

func countReplaceLabelByIssueAndTarget(calls []replaceLabelCall, issueNum int, label string) int {
	count := 0
	for _, call := range calls {
		if call.issueNumber == issueNum && call.newLabel == label {
			count++
		}
	}
	return count
}

func countLabel(labels []string, target string) int {
	count := 0
	for _, label := range labels {
		if label == target {
			count++
		}
	}
	return count
}

func newRecordingTaskCtlClient(t *testing.T) (*TaskCtlClient, string) {
	t.Helper()

	binDir := t.TempDir()
	logPath := filepath.Join(binDir, "taskctl.log")
	binPath := filepath.Join(binDir, "taskctl")
	script := fmt.Sprintf("#!/usr/bin/env bash\nif [ \"$1\" = \"update\" ]; then\n  printf '%%s\\n' \"$*\" >> %q\nfi\nexit 0\n", logPath)
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	return &TaskCtlClient{
		BinPath:   binPath,
		StorePath: filepath.Join(binDir, "tasks.json"),
	}, logPath
}

func newFailingTaskCtlClient(t *testing.T) *TaskCtlClient {
	t.Helper()

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "taskctl")
	script := "#!/usr/bin/env bash\nexit 1\n"
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0o755))

	return &TaskCtlClient{
		BinPath:   binPath,
		StorePath: filepath.Join(binDir, "tasks.json"),
	}
}

func findLineIndexContaining(lines []string, target string) int {
	for idx, line := range lines {
		if strings.Contains(line, target) {
			return idx
		}
	}
	return -1
}
