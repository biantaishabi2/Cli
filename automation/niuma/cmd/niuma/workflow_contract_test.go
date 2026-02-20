package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestWorkflowContract_EntrypointUsesReusableAndKeepsTriggers(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-orchestrate.yml")

	assert.Contains(t, content, "issues:")
	assert.Contains(t, content, "types: [labeled]")
	assert.Contains(t, content, "repository_dispatch:")
	assert.Contains(t, content, "types: [niuma.task.completed]")
	assert.Contains(t, content, "schedule:")
	assert.Contains(t, content, "github.event.label.name == 'bot:orchestrate'")
	assert.Contains(t, content, "github.event.label.name == 'bot:queued'")
	assert.Contains(t, content, "github.event.label.name == 'bot:pr-reviewable'")
	assert.Contains(t, content, "github.event.client_payload.event_source == 'close-after-integration-merge'")
	assert.Contains(t, content, "uses: ./.github/workflows/niuma-orchestrate-reusable.yml")
}

func TestWorkflowContract_ReusableWorkflowCallInputDefaults(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-orchestrate-reusable.yml")

	assert.Contains(t, content, "workflow_call:")
	assert.Contains(t, content, "repo:")
	assert.Contains(t, content, "required: true")
	assert.Contains(t, content, "repo_dir:")
	assert.Contains(t, content, "default: \".\"")
	assert.Contains(t, content, "build_niuma:")
	assert.Contains(t, content, "default: true")
	assert.Contains(t, content, "label_whitelist:")
	assert.Contains(t, content, "default: \"bot:orchestrate,bot:queued,bot:pr-reviewable\"")
	assert.Contains(t, content, "enable_dispatch_wakeup:")
	assert.Contains(t, content, "event_id:")
	assert.Contains(t, content, "default: \"\"")
	assert.Contains(t, content, "dedup_window_hours:")
	assert.Contains(t, content, "default: 24")
	assert.Contains(t, content, "concurrency_key:")
	assert.Contains(t, content, "default: \"niuma-orchestrate-${repo}\"")
}

func TestWorkflowContract_ReusableHasIdempotencyLoopGuardAndConcurrency(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-orchestrate-reusable.yml")

	assert.Contains(t, content, "concurrency:")
	assert.Contains(t, content, "cancel-in-progress: false")
	assert.Contains(t, content, "Idempotency Guard")
	assert.Contains(t, content, "/tmp/niuma-orchestrate-dedup")
	assert.Contains(t, content, "duplicate_event")
	assert.Contains(t, content, "Loop Guard")
	assert.Contains(t, content, "github-actions[bot]")
	assert.Contains(t, content, "event_id=\"run-${GITHUB_RUN_ID}-attempt-${GITHUB_RUN_ATTEMPT}\"")
}

func TestWorkflowContract_DispatchCompletedUsesNiumaBinary(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-dispatch-completed.yml")

	assert.Contains(t, content, "pull_request:")
	assert.Contains(t, content, "types: [closed]")
	assert.Contains(t, content, "github.event.pull_request.merged == true")
	assert.Contains(t, content, "startsWith(github.event.pull_request.head.ref, 'integration/')")
	assert.Contains(t, content, "control close-merged")
	assert.Contains(t, content, "--dispatch-wakeup")
}

func TestWorkflowContract_ImplementGateHasMaxRetriesValidation(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-implement-reusable.yml")

	assert.Contains(t, content, "GATE_MAX_RETRIES=2",
		"niuma-implement-reusable.yml 必须包含 GATE_MAX_RETRIES 默认值回退")
}

func TestWorkflowContract_IterateGateHasMaxRetriesValidation(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-iterate-reusable.yml")

	assert.Contains(t, content, "GATE_MAX_RETRIES=2",
		"niuma-iterate-reusable.yml 必须包含 GATE_MAX_RETRIES 默认值回退")
}

func TestWorkflowContract_ImplementGateUsesUnifiedCommand(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-implement-reusable.yml")

	assert.Contains(t, content, "\"$NIUMA_BIN\" gate run")
	assert.Contains(t, content, "GATE_MAX_RETRIES=2")
	assert.Contains(t, content, "--max-retries \"$GATE_MAX_RETRIES\"")
}

func TestWorkflowContract_IterateGateUsesUnifiedCommand(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-iterate-reusable.yml")

	assert.Equal(t, 2, strings.Count(content, "\"$NIUMA_BIN\" gate run"))
	assert.Equal(t, 2, strings.Count(content, "GATE_MAX_RETRIES=2"))
	assert.Equal(t, 2, strings.Count(content, "--max-retries \"$GATE_MAX_RETRIES\""))
}

func TestWorkflowContract_ReviewGateFailureTransitionsToNeedsFix(t *testing.T) {
	content := loadWorkflowFile(t, "niuma-review-reusable.yml")

	assert.Contains(t, content, "id: pr_gate")
	assert.Contains(t, content, "continue-on-error: true")
	assert.Contains(t, content, "steps.pr_gate.outcome == 'failure'")
	assert.Contains(t, content, "\"$NIUMA_BIN\" state-label set")
	assert.Contains(t, content, "--from bot:pr-created")
	assert.Contains(t, content, "--to bot:pr-needs-fix")
	assert.Contains(t, content, "inputs.skip_test_gate || steps.pr_gate.outcome == 'success'")
}

// ─── YAML 结构体定义 ───

type workflowFile struct {
	On   interface{}            `yaml:"on"`   // map 或 list
	Jobs map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	If    string                 `yaml:"if"`
	Uses  string                 `yaml:"uses"`
	With  map[string]interface{} `yaml:"with"`
	Needs interface{}            `yaml:"needs"` // string 或 []string
}

type workflowCallInput struct {
	Type     string      `yaml:"type"`
	Required bool        `yaml:"required"`
	Default  interface{} `yaml:"default"`
}

// entrypoint → reusable 配对表
var workflowPairs = []struct {
	entrypoint string
	reusable   string
}{
	{"niuma-orchestrate.yml", "niuma-orchestrate-reusable.yml"},
	{"niuma-implement.yml", "niuma-implement-reusable.yml"},
	{"niuma-iterate.yml", "niuma-iterate-reusable.yml"},
	{"niuma-discuss.yml", "niuma-discuss-reusable.yml"},
	{"niuma-plan.yml", "niuma-plan-reusable.yml"},
	{"niuma-review.yml", "niuma-review-reusable.yml"},
}

// ─── 结构化集成校验测试 ───

// TestWorkflowIntegration_ReusableParamTypeMatch 校验 caller with 参数类型与 reusable inputs 类型兼容
func TestWorkflowIntegration_ReusableParamTypeMatch(t *testing.T) {
	for _, pair := range workflowPairs {
		t.Run(pair.entrypoint+"→"+pair.reusable, func(t *testing.T) {
			entryWf := parseWorkflow(t, pair.entrypoint)
			reusableInputs := parseReusableInputs(t, pair.reusable)

			for jobName, job := range entryWf.Jobs {
				if job.Uses == "" || job.With == nil {
					continue
				}
				for paramKey, paramVal := range job.With {
					inputDef, ok := reusableInputs[paramKey]
					if !ok {
						continue // NoExtraParams 测试覆盖此场景
					}
					// 校验类型兼容性：GitHub Actions 表达式 ${{ }} 始终为 string，
					// 但对于 number/boolean 类型，非表达式的字面量应匹配
					valStr := fmt.Sprintf("%v", paramVal)
					if strings.Contains(valStr, "${{") {
						continue // 表达式由 GitHub Actions 运行时处理类型转换
					}
					switch inputDef.Type {
					case "boolean":
						assert.Conditionf(t, func() bool {
							return valStr == "true" || valStr == "false"
						}, "job %s.with.%s 值 %q 与 reusable input 类型 boolean 不兼容",
							jobName, paramKey, valStr)
					case "number":
						assert.Conditionf(t, func() bool {
							for _, c := range valStr {
								if (c < '0' || c > '9') && c != '-' && c != '.' {
									return false
								}
							}
							return len(valStr) > 0
						}, "job %s.with.%s 值 %q 与 reusable input 类型 number 不兼容",
							jobName, paramKey, valStr)
					}
				}
			}
		})
	}
}

// TestWorkflowIntegration_RequiredInputsCovered 校验 reusable 中 required=true 的 input 在 caller with 中存在
func TestWorkflowIntegration_RequiredInputsCovered(t *testing.T) {
	for _, pair := range workflowPairs {
		t.Run(pair.entrypoint+"→"+pair.reusable, func(t *testing.T) {
			entryWf := parseWorkflow(t, pair.entrypoint)
			reusableInputs := parseReusableInputs(t, pair.reusable)

			// 收集所有 required input 名称
			var requiredKeys []string
			for key, input := range reusableInputs {
				if input.Required {
					requiredKeys = append(requiredKeys, key)
				}
			}

			for jobName, job := range entryWf.Jobs {
				if job.Uses == "" {
					continue // 跳过非 reusable-call 的 job
				}
				for _, rk := range requiredKeys {
					assert.Containsf(t, job.With, rk,
						"job %s 缺少 required input %q", jobName, rk)
				}
			}
		})
	}
}

// TestWorkflowIntegration_NoExtraParams 校验 caller with 中的 key 必须在 reusable inputs 中有定义
func TestWorkflowIntegration_NoExtraParams(t *testing.T) {
	for _, pair := range workflowPairs {
		t.Run(pair.entrypoint+"→"+pair.reusable, func(t *testing.T) {
			entryWf := parseWorkflow(t, pair.entrypoint)
			reusableInputs := parseReusableInputs(t, pair.reusable)

			for jobName, job := range entryWf.Jobs {
				if job.Uses == "" || job.With == nil {
					continue
				}
				for paramKey := range job.With {
					assert.Containsf(t, reusableInputs, paramKey,
						"job %s.with.%s 在 reusable %s 的 inputs 中未定义（拼写错误或过时参数？）",
						jobName, paramKey, pair.reusable)
				}
			}
		})
	}
}

// TestWorkflowIntegration_EventJobCoverage 校验 entrypoint 的每种 event 至少有一个 job 能响应
func TestWorkflowIntegration_EventJobCoverage(t *testing.T) {
	// event 名称到 if 条件中隐式匹配模式的映射
	// 例如 github.event.label.name 隐含 issues 事件
	eventImplicitPatterns := map[string][]string{
		"issues":              {"github.event.label", "github.event.issue", "github.event_name == 'issues'"},
		"issue_comment":       {"github.event.comment", "github.event_name == 'issue_comment'"},
		"pull_request":        {"github.event.pull_request", "github.event_name == 'pull_request'"},
		"pull_request_review": {"github.event.review", "github.event_name == 'pull_request_review'"},
		"repository_dispatch": {"github.event.client_payload", "github.event.action", "github.event_name == 'repository_dispatch'"},
		"schedule":            {"github.event_name == 'schedule'"},
		"workflow_dispatch":   {"github.event_name == 'workflow_dispatch'"},
	}

	for _, pair := range workflowPairs {
		t.Run(pair.entrypoint, func(t *testing.T) {
			wf := parseWorkflow(t, pair.entrypoint)
			events := extractEventNames(wf.On)
			if len(events) == 0 {
				t.Skip("无法解析 on: triggers")
			}

			for _, event := range events {
				covered := false
				for _, job := range wf.Jobs {
					ifCond := job.If
					if ifCond == "" {
						// 无 if 条件的 job 响应所有 event
						covered = true
						break
					}
					// 直接匹配事件名称
					if strings.Contains(ifCond, event) {
						covered = true
						break
					}
					// 通过隐式模式匹配（如 github.event.label 暗示 issues 事件）
					if patterns, ok := eventImplicitPatterns[event]; ok {
						for _, pat := range patterns {
							if strings.Contains(ifCond, pat) {
								covered = true
								break
							}
						}
					}
					if covered {
						break
					}
				}
				assert.Truef(t, covered,
					"event %q 在 %s 中没有任何 job 能响应（所有 job 的 if 条件均不匹配）",
					event, pair.entrypoint)
			}
		})
	}
}

// TestWorkflowIntegration_NeedsDepsExist 校验 job.needs 引用的 job 名称必须存在于同一 workflow
func TestWorkflowIntegration_NeedsDepsExist(t *testing.T) {
	// 校验所有 workflow（entrypoint + reusable）
	allFiles := make(map[string]bool)
	for _, pair := range workflowPairs {
		allFiles[pair.entrypoint] = true
		allFiles[pair.reusable] = true
	}

	for file := range allFiles {
		t.Run(file, func(t *testing.T) {
			wf := parseWorkflow(t, file)
			jobNames := make(map[string]bool)
			for name := range wf.Jobs {
				jobNames[name] = true
			}

			for jobName, job := range wf.Jobs {
				for _, dep := range parseNeeds(job.Needs) {
					assert.Truef(t, jobNames[dep],
						"job %s.needs 引用了不存在的 job %q", jobName, dep)
				}
			}
		})
	}
}

// ─── 辅助函数 ───

// parseWorkflow 解析 workflow YAML 文件为结构体
func parseWorkflow(t *testing.T, workflowName string) workflowFile {
	t.Helper()
	raw := loadWorkflowFile(t, workflowName)
	var wf workflowFile
	require.NoError(t, yaml.Unmarshal([]byte(raw), &wf),
		"解析 %s 失败", workflowName)
	return wf
}

// parseReusableInputs 从 reusable workflow 中提取 workflow_call.inputs 定义
func parseReusableInputs(t *testing.T, workflowName string) map[string]workflowCallInput {
	t.Helper()
	raw := loadWorkflowFile(t, workflowName)

	// 完整解析 on 字段以获取 workflow_call.inputs
	var full struct {
		On struct {
			WorkflowCall struct {
				Inputs map[string]workflowCallInput `yaml:"inputs"`
			} `yaml:"workflow_call"`
		} `yaml:"on"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(raw), &full),
		"解析 %s workflow_call inputs 失败", workflowName)
	require.NotNil(t, full.On.WorkflowCall.Inputs,
		"%s 缺少 on.workflow_call.inputs", workflowName)
	return full.On.WorkflowCall.Inputs
}

// extractEventNames 从 on 字段提取事件名称列表
func extractEventNames(on interface{}) []string {
	var events []string
	switch v := on.(type) {
	case map[string]interface{}:
		for key := range v {
			events = append(events, key)
		}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				events = append(events, s)
			}
		}
	}
	return events
}

// parseNeeds 将 needs 字段（string 或 []string）解析为字符串切片
func parseNeeds(needs interface{}) []string {
	if needs == nil {
		return nil
	}
	switch v := needs.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case []interface{}:
		var result []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func loadWorkflowFile(t *testing.T, workflowName string) string {
	t.Helper()

	workflowPath := findWorkflowPath(t, workflowName)
	raw, err := os.ReadFile(workflowPath)
	require.NoError(t, err)
	return string(raw)
}

func findWorkflowPath(t *testing.T, workflowName string) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, ".github", "workflows", workflowName)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	t.Fatalf("未找到 .github/workflows/%s", workflowName)
	return ""
}
