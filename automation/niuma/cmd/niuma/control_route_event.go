package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/control"
	"github.com/spf13/cobra"
)

var controlRouteEventCmd = &cobra.Command{
	Use:   "route-event",
	Short: "根据 workflow/event 计算统一路由决策",
	RunE:  runControlRouteEvent,
}

var (
	flagRouteEventName string
	flagRouteEventPath string
	flagRouteWorkflow  string
)

func init() {
	controlCmd.AddCommand(controlRouteEventCmd)
	controlRouteEventCmd.Flags().StringVar(&flagRouteWorkflow, "workflow", "", "workflow 名称（例如 orchestrate/niuma-orchestrate.yml）")
	controlRouteEventCmd.Flags().StringVar(&flagRouteEventName, "event-name", "", "GitHub 事件名")
	controlRouteEventCmd.Flags().StringVar(&flagRouteEventPath, "event-path", "", "GitHub 事件 JSON 路径")
	controlRouteEventCmd.MarkFlagRequired("workflow")
	controlRouteEventCmd.MarkFlagRequired("event-name")
	controlRouteEventCmd.MarkFlagRequired("event-path")
}

func runControlRouteEvent(cmd *cobra.Command, args []string) error {
	payload, err := os.ReadFile(strings.TrimSpace(flagRouteEventPath))
	if err != nil {
		return withExitCode(1, fmt.Errorf("读取 event payload 失败: %w", err))
	}

	decision, routeErr := control.RouteEvent(flagRouteWorkflow, flagRouteEventName, payload)
	if routeErr != nil {
		decision = control.NewFailDecision(strings.TrimSpace(flagRouteWorkflow), strings.TrimSpace(flagRouteEventName), decision.Action, control.ReasonFailInvalidEventPayload)
	}
	decision.CorrelationID = resolveCorrelationID()
	if err := decision.Validate(); err != nil {
		return withExitCode(1, err)
	}

	audit := map[string]string{
		"ts":             time.Now().UTC().Format(time.RFC3339),
		"workflow":       decision.Workflow,
		"event_name":     decision.EventName,
		"action":         string(decision.Action),
		"decision":       string(decision.Decision),
		"reason":         string(decision.Reason),
		"correlation_id": decision.CorrelationID,
	}
	encoded, _ := json.Marshal(audit)
	fmt.Fprintf(os.Stderr, "[control.route-event] %s\n", string(encoded))

	fmt.Printf("decision=%s\n", decision.Decision)
	fmt.Printf("reason=%s\n", decision.Reason)
	fmt.Printf("action=%s\n", decision.Action)

	if routeErr != nil {
		return withExitCode(1, routeErr)
	}
	if decision.Decision == control.DecisionFail {
		return withExitCode(decision.ExitCode(), fmt.Errorf("route-event 判定失败: %s", decision.Reason))
	}
	return nil
}

func resolveCorrelationID() string {
	runID := strings.TrimSpace(os.Getenv("GITHUB_RUN_ID"))
	attempt := strings.TrimSpace(os.Getenv("GITHUB_RUN_ATTEMPT"))
	if runID != "" {
		if attempt == "" {
			attempt = "1"
		}
		return fmt.Sprintf("run-%s-attempt-%s", runID, attempt)
	}
	return fmt.Sprintf("pid-%d", os.Getpid())
}
