package control

import "fmt"

// Decision 表示 route-event 的执行决策。
type Decision string

const (
	DecisionRun  Decision = "run"
	DecisionSkip Decision = "skip"
	DecisionFail Decision = "fail"
)

// Action 表示 workflow 对应的业务动作。
type Action string

const (
	ActionOrchestrate Action = "orchestrate"
	ActionPlan        Action = "plan"
	ActionImplement   Action = "implement"
	ActionReview      Action = "review"
	ActionIterate     Action = "iterate"
	ActionDiscuss     Action = "discuss"
	ActionNone        Action = "none"
)

// Reason 表示稳定的 machine-readable 决策原因。
type Reason string

const (
	ReasonRunRoutable                  Reason = "run_routable"
	ReasonSkipLabelNotRoutable         Reason = "skip_label_not_routable"
	ReasonSkipDispatchSourceNotAllowed Reason = "skip_dispatch_source_not_allowed"
	ReasonSkipDispatchActionNotAllowed Reason = "skip_dispatch_action_not_allowed"
	ReasonSkipStateNotApplicable       Reason = "skip_state_not_applicable"
	ReasonSkipEventNotSupported        Reason = "skip_event_not_supported"
	ReasonFailInvalidEventPayload      Reason = "fail_invalid_event_payload"
	ReasonFailUnsupportedWorkflow      Reason = "fail_unsupported_workflow"
	ReasonFailInternalError            Reason = "fail_internal_error"
)

// RouteDecision 为 control 路由结果。
type RouteDecision struct {
	Workflow      string   `json:"workflow"`
	EventName     string   `json:"event_name"`
	Decision      Decision `json:"decision"`
	Reason        Reason   `json:"reason"`
	Action        Action   `json:"action"`
	CorrelationID string   `json:"correlation_id,omitempty"`
}

func NewRunDecision(workflow, eventName string, action Action, reason Reason) RouteDecision {
	return RouteDecision{Workflow: workflow, EventName: eventName, Decision: DecisionRun, Reason: reason, Action: action}
}

func NewSkipDecision(workflow, eventName string, action Action, reason Reason) RouteDecision {
	return RouteDecision{Workflow: workflow, EventName: eventName, Decision: DecisionSkip, Reason: reason, Action: action}
}

func NewFailDecision(workflow, eventName string, action Action, reason Reason) RouteDecision {
	return RouteDecision{Workflow: workflow, EventName: eventName, Decision: DecisionFail, Reason: reason, Action: action}
}

// ExitCode 返回该决策建议的退出码。
func (d RouteDecision) ExitCode() int {
	if d.Decision == DecisionFail {
		return 1
	}
	return 0
}

// Validate 校验决策字段。
func (d RouteDecision) Validate() error {
	if d.Decision == "" {
		return fmt.Errorf("decision 不能为空")
	}
	if d.Reason == "" {
		return fmt.Errorf("reason 不能为空")
	}
	if d.Action == "" {
		return fmt.Errorf("action 不能为空")
	}
	return nil
}
