//go:build smoke

package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const smokeTestRepo = "biantaishabi2/Cli-niuma-test"

type smokeCase struct {
	name       string
	workflow   string
	event      string
	trigger    func(t *testing.T, issueNum int)
	cleanup    func(t *testing.T, issueNum int)
	expectJobs []string // 期望被触发（非 skipped）的 job 名称前缀
	skipJobs   []string // 期望被 skipped 的 job 名称前缀
}

func TestWorkflowSmoke_EventJobCoverage(t *testing.T) {
	requireGhCLI(t)

	cases := []smokeCase{
		{
			name:       "discuss/workflow_dispatch",
			workflow:   "niuma-discuss.yml",
			event:      "workflow_dispatch",
			expectJobs: []string{"call-discuss-from-dispatch"},
			skipJobs:   []string{"call-discuss-from-label", "call-discuss-from-comment"},
			trigger: func(t *testing.T, issueNum int) {
				ghExec(t, "workflow", "run", "niuma-discuss.yml",
					"--repo", smokeTestRepo,
					"-f", fmt.Sprintf("issue=%d", issueNum))
			},
		},
		{
			name:       "discuss/issues_labeled",
			workflow:   "niuma-discuss.yml",
			event:      "issues",
			expectJobs: []string{"call-discuss-from-label"},
			skipJobs:   []string{"call-discuss-from-dispatch", "call-discuss-from-comment"},
			trigger: func(t *testing.T, issueNum int) {
				addLabelViaAPI(t, issueNum, "bot:needs-discussion")
			},
			cleanup: func(t *testing.T, issueNum int) {
				removeLabelViaAPI(t, issueNum, "bot:needs-discussion")
			},
		},
		{
			name:       "discuss/issue_comment",
			workflow:   "niuma-discuss.yml",
			event:      "issue_comment",
			expectJobs: []string{"call-discuss-from-comment"},
			skipJobs:   []string{"call-discuss-from-dispatch", "call-discuss-from-label"},
			trigger: func(t *testing.T, issueNum int) {
				ghExec(t, "issue", "comment", fmt.Sprint(issueNum),
					"--repo", smokeTestRepo,
					"--body", fmt.Sprintf("<!-- smoke-test %d -->", time.Now().UnixNano()))
			},
		},
		{
			name:       "plan/issues_labeled",
			workflow:   "niuma-plan.yml",
			event:      "issues",
			expectJobs: []string{"plan-draft"},
			trigger: func(t *testing.T, issueNum int) {
				addLabelViaAPI(t, issueNum, "bot:fix")
			},
			cleanup: func(t *testing.T, issueNum int) {
				removeLabelViaAPI(t, issueNum, "bot:fix")
			},
		},
		{
			name:       "implement/issues_labeled",
			workflow:   "niuma-implement.yml",
			event:      "issues",
			expectJobs: []string{"implement"},
			trigger: func(t *testing.T, issueNum int) {
				addLabelViaAPI(t, issueNum, "bot:plan-final")
			},
			cleanup: func(t *testing.T, issueNum int) {
				removeLabelViaAPI(t, issueNum, "bot:plan-final")
			},
		},
		{
			name:       "review/issues_labeled",
			workflow:   "niuma-review.yml",
			event:      "issues",
			expectJobs: []string{"review"},
			trigger: func(t *testing.T, issueNum int) {
				addLabelViaAPI(t, issueNum, "bot:pr-created")
			},
			cleanup: func(t *testing.T, issueNum int) {
				removeLabelViaAPI(t, issueNum, "bot:pr-created")
			},
		},
		{
			name:       "iterate/issues_labeled",
			workflow:   "niuma-iterate.yml",
			event:      "issues",
			expectJobs: []string{"call-iterate-from-review"},
			skipJobs:   []string{"call-iterate-from-human"},
			trigger: func(t *testing.T, issueNum int) {
				addLabelViaAPI(t, issueNum, "bot:pr-needs-fix")
			},
			cleanup: func(t *testing.T, issueNum int) {
				removeLabelViaAPI(t, issueNum, "bot:pr-needs-fix")
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runSmokeCase(t, tc)
		})
	}
}

// ─── 单个用例执行 ───

func runSmokeCase(t *testing.T, tc smokeCase) {
	issueNum := createSmokeIssue(t)
	t.Cleanup(func() { closeSmokeIssue(t, issueNum) })

	beforeTrigger := time.Now().UTC()
	time.Sleep(1 * time.Second)

	t.Logf("触发 %s (issue #%d)", tc.name, issueNum)
	tc.trigger(t, issueNum)

	if tc.cleanup != nil {
		t.Cleanup(func() { tc.cleanup(t, issueNum) })
	}

	runID := pollForRun(t, tc.workflow, tc.event, beforeTrigger, 90*time.Second)
	t.Logf("找到 run %d", runID)
	t.Cleanup(func() {
		_ = ghExecIgnoreErr("run", "cancel", fmt.Sprint(runID), "--repo", smokeTestRepo)
	})

	jobs := pollForJobs(t, runID, 60*time.Second)
	t.Logf("jobs: %v", formatJobs(jobs))

	assertExpectedJobs(t, jobs, tc.expectJobs, tc.skipJobs)
}

// ─── 校验 ───

type ghJob struct {
	Name       string `json:"name"`
	Conclusion string `json:"conclusion"`
	Status     string `json:"status"`
}

func assertExpectedJobs(t *testing.T, jobs []ghJob, expectPrefixes, skipPrefixes []string) {
	t.Helper()

	for _, prefix := range expectPrefixes {
		found := false
		for _, j := range jobs {
			if strings.HasPrefix(j.Name, prefix) {
				found = true
				if j.Conclusion == "skipped" {
					t.Errorf("期望 job %q 被触发，但实际 skipped（if 条件未匹配该 event）", j.Name)
				} else {
					t.Logf("✓ job %q 已触发 (status=%s, conclusion=%s)", j.Name, j.Status, j.Conclusion)
				}
				break
			}
		}
		if !found {
			t.Errorf("期望 job %q 存在但未出现在 jobs 列表中（被 GitHub Actions 静默丢弃）", prefix)
		}
	}

	for _, prefix := range skipPrefixes {
		for _, j := range jobs {
			if strings.HasPrefix(j.Name, prefix) {
				if j.Conclusion != "skipped" && j.Conclusion != "" {
					t.Errorf("期望 job %q 被 skipped，但实际 conclusion=%s（错误触发）", j.Name, j.Conclusion)
				}
				break
			}
		}
	}
}

// ─── GitHub API 操作（绕过 gh wrapper 对 bot:* label 的拦截）───

func addLabelViaAPI(t *testing.T, issueNum int, label string) {
	t.Helper()
	endpoint := fmt.Sprintf("repos/%s/issues/%d/labels", smokeTestRepo, issueNum)
	ghExec(t, "api", endpoint, "--method", "POST", "-f", fmt.Sprintf("labels[]=%s", label))
}

func removeLabelViaAPI(t *testing.T, issueNum int, label string) {
	t.Helper()
	endpoint := fmt.Sprintf("repos/%s/issues/%d/labels/%s", smokeTestRepo, issueNum, label)
	_ = ghExecIgnoreErr("api", endpoint, "--method", "DELETE")
}

// ─── 辅助 ───

func requireGhCLI(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI 不可用，跳过冒烟测试")
	}
	out := ghExec(t, "auth", "status", "--hostname", "github.com")
	if strings.Contains(out, "not logged") {
		t.Skip("gh 未登录，跳过冒烟测试")
	}
}

func createSmokeIssue(t *testing.T) int {
	t.Helper()
	ts := time.Now().Format("20060102-150405")
	title := fmt.Sprintf("[smoke] workflow job coverage %s", ts)
	out := ghExec(t, "issue", "create",
		"--repo", smokeTestRepo,
		"--title", title,
		"--body", "自动冒烟测试 issue，测试完成后自动关闭。")

	parts := strings.Split(strings.TrimSpace(out), "/")
	numStr := parts[len(parts)-1]
	var num int
	fmt.Sscanf(numStr, "%d", &num)
	if num == 0 {
		t.Fatalf("无法从 gh issue create 输出中提取 issue 号: %s", out)
	}
	t.Logf("创建 smoke issue #%d", num)
	return num
}

func closeSmokeIssue(t *testing.T, issueNum int) {
	t.Helper()
	_ = ghExecIgnoreErr("issue", "close", fmt.Sprint(issueNum),
		"--repo", smokeTestRepo,
		"--comment", "smoke test 完成，自动关闭")
}

func pollForRun(t *testing.T, workflow, event string, after time.Time, timeout time.Duration) int64 {
	t.Helper()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		out := ghExec(t, "run", "list",
			"--repo", smokeTestRepo,
			"--workflow", workflow,
			"--limit", "5",
			"--json", "databaseId,event,createdAt,status")

		var runs []struct {
			DatabaseId int64  `json:"databaseId"`
			Event      string `json:"event"`
			CreatedAt  string `json:"createdAt"`
		}
		if err := json.Unmarshal([]byte(out), &runs); err == nil {
			for _, r := range runs {
				created, _ := time.Parse(time.RFC3339, r.CreatedAt)
				if r.Event == event && created.After(after) {
					return r.DatabaseId
				}
			}
		}
		time.Sleep(3 * time.Second)
	}

	t.Fatalf("超时 %v 未找到 workflow=%s event=%s 的 run", timeout, workflow, event)
	return 0
}

func pollForJobs(t *testing.T, runID int64, timeout time.Duration) []ghJob {
	t.Helper()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		out := ghExec(t, "run", "view", fmt.Sprint(runID),
			"--repo", smokeTestRepo,
			"--json", "jobs")

		var result struct {
			Jobs []ghJob `json:"jobs"`
		}
		if err := json.Unmarshal([]byte(out), &result); err == nil && len(result.Jobs) > 0 {
			return result.Jobs
		}
		time.Sleep(3 * time.Second)
	}

	t.Fatalf("超时 %v 未获取到 run %d 的 jobs", timeout, runID)
	return nil
}

func formatJobs(jobs []ghJob) string {
	var parts []string
	for _, j := range jobs {
		parts = append(parts, fmt.Sprintf("%s(%s)", j.Name, j.Conclusion))
	}
	return strings.Join(parts, ", ")
}

func ghExec(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("gh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gh %s 失败: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func ghExecIgnoreErr(args ...string) string {
	cmd := exec.Command("gh", args...)
	out, _ := cmd.CombinedOutput()
	return string(out)
}
