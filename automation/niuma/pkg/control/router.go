package control

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RouteEvent 根据 workflow 与事件 payload 生成路由决策。
func RouteEvent(workflow, eventName string, payload []byte) (RouteDecision, error) {
	normalizedWorkflow := normalizeWorkflow(workflow)
	eventName = strings.TrimSpace(eventName)

	switch normalizedWorkflow {
	case "orchestrate":
		return routeOrchestrate(eventName, payload)
	case "plan":
		return routeIssueLabelWorkflow(normalizedWorkflow, eventName, payload, map[string]struct{}{"bot:fix": {}}, ActionPlan)
	case "implement":
		return routeIssueLabelWorkflow(normalizedWorkflow, eventName, payload, map[string]struct{}{"bot:plan-final": {}, "bot:plan-approved": {}}, ActionImplement)
	case "review":
		return routeIssueLabelWorkflow(normalizedWorkflow, eventName, payload, map[string]struct{}{"bot:pr-created": {}}, ActionReview)
	default:
		return NewFailDecision(normalizedWorkflow, eventName, ActionNone, ReasonFailUnsupportedWorkflow), nil
	}
}

func normalizeWorkflow(workflow string) string {
	w := strings.TrimSpace(strings.ToLower(workflow))
	w = strings.TrimSuffix(w, ".yml")
	w = strings.TrimSuffix(w, ".yaml")
	w = strings.TrimPrefix(w, "niuma-")
	w = strings.TrimSuffix(w, "-reusable")
	return w
}

func routeOrchestrate(eventName string, payload []byte) (RouteDecision, error) {
	wf := "orchestrate"
	switch eventName {
	case "issues":
		label, err := extractIssueLabel(payload)
		if err != nil {
			return NewFailDecision(wf, eventName, ActionOrchestrate, ReasonFailInvalidEventPayload), err
		}
		allowed := map[string]struct{}{
			"bot:orchestrate":   {},
			"bot:queued":        {},
			"bot:pr-reviewable": {},
			"bot:premerged":     {},
		}
		if _, ok := allowed[label]; !ok {
			return NewSkipDecision(wf, eventName, ActionNone, ReasonSkipLabelNotRoutable), nil
		}
		return NewRunDecision(wf, eventName, ActionOrchestrate, ReasonRunRoutable), nil
	case "repository_dispatch":
		action, source, err := extractDispatchActionSource(payload)
		if err != nil {
			return NewFailDecision(wf, eventName, ActionOrchestrate, ReasonFailInvalidEventPayload), err
		}
		if action != "niuma.task.completed" {
			return NewSkipDecision(wf, eventName, ActionNone, ReasonSkipDispatchActionNotAllowed), nil
		}
		if source != "close-after-integration-merge" {
			return NewSkipDecision(wf, eventName, ActionNone, ReasonSkipDispatchSourceNotAllowed), nil
		}
		return NewRunDecision(wf, eventName, ActionOrchestrate, ReasonRunRoutable), nil
	case "pull_request":
		action, merged, err := extractPullRequestClosedMerged(payload)
		if err != nil {
			return NewFailDecision(wf, eventName, ActionOrchestrate, ReasonFailInvalidEventPayload), err
		}
		if action != "closed" || !merged {
			return NewSkipDecision(wf, eventName, ActionNone, ReasonSkipStateNotApplicable), nil
		}
		return NewRunDecision(wf, eventName, ActionOrchestrate, ReasonRunRoutable), nil
	case "schedule":
		return NewRunDecision(wf, eventName, ActionOrchestrate, ReasonRunRoutable), nil
	default:
		return NewSkipDecision(wf, eventName, ActionNone, ReasonSkipEventNotSupported), nil
	}
}

func routeIssueLabelWorkflow(workflow, eventName string, payload []byte, allowed map[string]struct{}, action Action) (RouteDecision, error) {
	if eventName != "issues" {
		return NewSkipDecision(workflow, eventName, ActionNone, ReasonSkipEventNotSupported), nil
	}
	label, err := extractIssueLabel(payload)
	if err != nil {
		return NewFailDecision(workflow, eventName, action, ReasonFailInvalidEventPayload), err
	}
	if _, ok := allowed[label]; !ok {
		return NewSkipDecision(workflow, eventName, ActionNone, ReasonSkipLabelNotRoutable), nil
	}
	return NewRunDecision(workflow, eventName, action, ReasonRunRoutable), nil
}

func extractIssueLabel(payload []byte) (string, error) {
	var data struct {
		Label struct {
			Name string `json:"name"`
		} `json:"label"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", fmt.Errorf("解析 issues payload 失败: %w", err)
	}
	label := strings.TrimSpace(data.Label.Name)
	if label == "" {
		return "", fmt.Errorf("issues payload 缺少 label.name")
	}
	return label, nil
}

func extractDispatchActionSource(payload []byte) (string, string, error) {
	var data struct {
		Action        string `json:"action"`
		ClientPayload struct {
			EventSource string `json:"event_source"`
		} `json:"client_payload"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", "", fmt.Errorf("解析 repository_dispatch payload 失败: %w", err)
	}
	return strings.TrimSpace(data.Action), strings.TrimSpace(data.ClientPayload.EventSource), nil
}

func extractPullRequestClosedMerged(payload []byte) (string, bool, error) {
	var data struct {
		Action      string `json:"action"`
		PullRequest struct {
			Merged bool `json:"merged"`
		} `json:"pull_request"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", false, fmt.Errorf("解析 pull_request payload 失败: %w", err)
	}
	return strings.TrimSpace(data.Action), data.PullRequest.Merged, nil
}
