// cmd/niuma/gate_label.go
// gate label 子命令：根据配置校验触发标签是否在允许列表中
package main

import (
	"fmt"
	"strings"

	"github.com/biantaishabi2/Cli/automation/niuma/pkg/config"
	"github.com/spf13/cobra"
)

var gateLabelCmd = &cobra.Command{
	Use:   "label",
	Short: "校验触发标签是否在配置的允许列表中",
	RunE:  runGateLabel,
}

var (
	flagGateLabelLabel string
	flagGateLabelType  string
)

func init() {
	gateCmd.AddCommand(gateLabelCmd)
	gateLabelCmd.Flags().StringVar(&flagGateLabelLabel, "label", "", "待校验的标签名")
	gateLabelCmd.Flags().StringVar(&flagGateLabelType, "type", "", "门禁类型（目前仅支持 implement）")
	gateLabelCmd.MarkFlagRequired("label")
	gateLabelCmd.MarkFlagRequired("type")
}

func runGateLabel(cmd *cobra.Command, args []string) error {
	if flagGateLabelType != "implement" {
		return fmt.Errorf("不支持的 gate 类型: %q（目前仅支持 implement）", flagGateLabelType)
	}

	configDir := "."
	if flagRepoDir != "" {
		configDir = flagRepoDir
	}
	cfg := config.LoadWithDefaults(configDir)

	triggerLabels := cfg.Workflow.GetImplementTriggerLabels()

	for _, allowed := range triggerLabels {
		if allowed == flagGateLabelLabel {
			fmt.Printf("Gate passed: label %s is allowed for %s\n", flagGateLabelLabel, flagGateLabelType)
			return nil
		}
	}

	fmt.Printf("Gate skipped: label %s is not in %s trigger list [%s]\n",
		flagGateLabelLabel, flagGateLabelType, strings.Join(triggerLabels, ", "))
	return withExitCode(78, fmt.Errorf("label %s not in trigger list", flagGateLabelLabel))
}
