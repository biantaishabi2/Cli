# V2 Sampling Evidence (Hypothesis-Only)

This file records evidence samples and hypothesis tags only; no final adjudication.

## plugin_tool_extension->channel_ingress
- status_candidate: `temporary_candidate`
- evidence_tag: `mixed`
- total_edges: 121, call_edges: 0, call_ratio: 0
- src_file_count: 13, dst_file_count: 101
- hypothesis_reason: suspected heavy barrel export/type coupling
- sample_src_files:
  - src/plugin-sdk/index.ts (60)
  - src/plugins/runtime/index.ts (37)
  - src/agents/tools/telegram-actions.ts (5)
- sample_dst_files:
  - src/discord/send.ts (4)
  - src/channels/plugins/types.ts (4)

## plugin_tool_extension->agent_runtime
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 108, call_edges: 10, call_ratio: 0.0926
- src_file_count: 35, dst_file_count: 56
- hypothesis_reason: tools need runtime context and agent schema/session state
- sample_src_files:
  - src/plugins/runtime/index.ts (19)
  - src/agents/tools/image-tool.ts (11)
  - src/agents/tools/session-status-tool.ts (10)
- sample_dst_files:
  - src/agents/agent-scope.ts (11)
  - src/agents/schema/typebox.ts (9)

## ops_client_entry->channel_ingress
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 96, call_edges: 10, call_ratio: 0.1042
- src_file_count: 43, dst_file_count: 21
- hypothesis_reason: ops and onboarding orchestrate channel accounts and capabilities
- sample_src_files:
  - src/cli/deps.ts (6)
  - src/commands/channels/capabilities.ts (6)
  - src/commands/agents.providers.ts (5)
- sample_dst_files:
  - src/channels/plugins/index.ts (32)
  - src/channels/plugins/types.ts (18)

## foundation_infra->channel_ingress
- status_candidate: `temporary_candidate`
- evidence_tag: `mixed`
- total_edges: 91, call_edges: 4, call_ratio: 0.044
- src_file_count: 40, dst_file_count: 31
- hypothesis_reason: outbound infra may include channel policy details
- sample_src_files:
  - src/infra/outbound/outbound-session.ts (14)
  - src/infra/outbound/deliver.ts (9)
  - src/config/sessions/metadata.ts (4)
- sample_dst_files:
  - src/channels/plugins/types.ts (27)
  - src/channels/plugins/index.ts (18)

## foundation_infra->agent_runtime
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 85, call_edges: 11, call_ratio: 0.1294
- src_file_count: 28, dst_file_count: 36
- hypothesis_reason: cron and heartbeat orchestration trigger runtime execution
- sample_src_files:
  - src/cron/isolated-agent/run.ts (22)
  - src/infra/heartbeat-runner.ts (10)
  - src/extensionAPI.ts (7)
- sample_dst_files:
  - src/agents/agent-scope.ts (10)
  - src/agents/model-selection.ts (8)

## ops_client_entry->platform_hosts
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 73, call_edges: 7, call_ratio: 0.0959
- src_file_count: 26, dst_file_count: 17
- hypothesis_reason: daemon and node-host lifecycle management from control plane
- sample_src_files:
  - src/cli/node-cli/daemon.ts (9)
  - src/cli/daemon-cli/status.print.ts (6)
  - src/commands/doctor-gateway-daemon-flow.ts (6)
- sample_dst_files:
  - src/daemon/service.ts (15)
  - src/daemon/constants.ts (10)

## agent_runtime->channel_ingress
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 73, call_edges: 5, call_ratio: 0.0685
- src_file_count: 32, dst_file_count: 24
- hypothesis_reason: runtime output flow couples with channel semantics
- sample_src_files:
  - src/auto-reply/reply/commands-allowlist.ts (12)
  - src/agents/channel-tools.ts (5)
  - src/tts/tts.ts (4)
- sample_dst_files:
  - src/channels/plugins/index.ts (14)
  - src/channels/dock.ts (13)

## channel_ingress->gateway_control_plane
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 72, call_edges: 1, call_ratio: 0.0139
- src_file_count: 44, dst_file_count: 8
- hypothesis_reason: inbound channel events require routing/session resolution
- sample_src_files:
  - src/slack/monitor/message-handler/prepare.ts (4)
  - src/telegram/bot-message-context.ts (4)
  - src/discord/monitor/agent-components.ts (3)
- sample_dst_files:
  - src/routing/session-key.ts (29)
  - src/routing/resolve-route.ts (18)

## agent_runtime->gateway_control_plane
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 44, call_edges: 8, call_ratio: 0.1818
- src_file_count: 30, dst_file_count: 8
- hypothesis_reason: runtime reads/writes session routing information
- sample_src_files:
  - src/auto-reply/reply/directive-handling.impl.ts (4)
  - src/auto-reply/reply/directive-handling.persist.ts (3)
  - src/agents/agent-scope.ts (2)
- sample_dst_files:
  - src/routing/session-key.ts (22)
  - src/gateway/call.ts (5)

## foundation_infra->ops_client_entry
- status_candidate: `temporary_candidate`
- evidence_tag: `mixed`
- total_edges: 44, call_edges: 11, call_ratio: 0.25
- src_file_count: 23, dst_file_count: 17
- hypothesis_reason: infra reuses CLI presentation helpers
- sample_src_files:
  - src/config/sessions/store.ts (4)
  - src/index.ts (4)
  - src/infra/tailscale.ts (3)
- sample_dst_files:
  - src/cli/command-format.ts (9)
  - src/cli/parse-duration.ts (8)

## channel_ingress->ops_client_entry
- status_candidate: `temporary_candidate`
- evidence_tag: `mixed`
- total_edges: 41, call_edges: 0, call_ratio: 0
- src_file_count: 23, dst_file_count: 12
- hypothesis_reason: suspected ui/format utility leakage
- sample_src_files:
  - src/channel-web.ts (6)
  - src/channels/plugins/onboarding/signal.ts (5)
  - src/channels/plugins/onboarding/whatsapp.ts (4)
- sample_dst_files:
  - src/web/media.ts (9)
  - src/wizard/prompts.ts (8)

## plugin_tool_extension->gateway_control_plane
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 40, call_edges: 4, call_ratio: 0.1
- src_file_count: 22, dst_file_count: 13
- hypothesis_reason: tools interact with session and routing control contracts
- sample_src_files:
  - src/agents/tools/session-status-tool.ts (4)
  - src/agents/tools/message-tool.ts (3)
  - src/agents/tools/sessions-helpers.ts (3)
- sample_dst_files:
  - src/routing/session-key.ts (16)
  - src/gateway/call.ts (9)

## foundation_infra->gateway_control_plane
- status_candidate: `temporary_candidate`
- evidence_tag: `mixed`
- total_edges: 40, call_edges: 7, call_ratio: 0.175
- src_file_count: 28, dst_file_count: 11
- hypothesis_reason: shared session-key utility may be misplaced
- sample_src_files:
  - src/security/audit.ts (4)
  - src/infra/outbound/outbound-session.ts (3)
  - src/config/sessions/main-session.ts (2)
- sample_dst_files:
  - src/routing/session-key.ts (24)
  - src/sessions/transcript-events.ts (2)

## gateway_control_plane->ops_client_entry
- status_candidate: `temporary_candidate`
- evidence_tag: `mixed`
- total_edges: 37, call_edges: 1, call_ratio: 0.027
- src_file_count: 24, dst_file_count: 12
- hypothesis_reason: control plane and ops entry boundary may be over-coupled
- sample_src_files:
  - src/gateway/server.impl.ts (4)
  - src/gateway/openresponses-http.ts (3)
  - src/gateway/server-methods/types.ts (3)
- sample_dst_files:
  - src/cli/deps.ts (13)
  - src/commands/agent.ts (6)

## ops_client_entry->plugin_tool_extension
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 36, call_edges: 5, call_ratio: 0.1389
- src_file_count: 11, dst_file_count: 19
- hypothesis_reason: plugin install/update/status via operational entrypoints
- sample_src_files:
  - src/cli/hooks-cli.ts (7)
  - src/cli/plugins-cli.ts (7)
  - src/commands/onboarding/plugin-install.ts (7)
- sample_dst_files:
  - src/plugins/loader.ts (5)
  - src/plugins/enable.ts (4)

## plugin_tool_extension->ops_client_entry
- status_candidate: `temporary_candidate`
- evidence_tag: `mixed`
- total_edges: 32, call_edges: 2, call_ratio: 0.0625
- src_file_count: 12, dst_file_count: 23
- hypothesis_reason: plugin runtime may depend on CLI/web helpers
- sample_src_files:
  - src/plugins/runtime/index.ts (7)
  - src/plugin-sdk/index.ts (5)
  - src/agents/tools/browser-tool.ts (4)
- sample_dst_files:
  - src/web/media.ts (3)
  - src/cli/parse-duration.ts (3)

## agent_runtime->ops_client_entry
- status_candidate: `temporary_candidate`
- evidence_tag: `mixed`
- total_edges: 27, call_edges: 4, call_ratio: 0.1481
- src_file_count: 20, dst_file_count: 9
- hypothesis_reason: runtime may depend on terminal/CLI formatting helpers
- sample_src_files:
  - src/agents/sandbox/browser.ts (3)
  - src/agents/pi-embedded-runner/run/images.ts (2)
  - src/agents/pi-extensions/context-pruning/settings.ts (2)
- sample_dst_files:
  - src/cli/command-format.ts (10)
  - src/cli/parse-duration.ts (5)

## channel_ingress->plugin_tool_extension
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 24, call_edges: 1, call_ratio: 0.0417
- src_file_count: 14, dst_file_count: 12
- hypothesis_reason: channel action handlers invoke tool/plugin capabilities
- sample_src_files:
  - src/channels/plugins/actions/discord/handle-action.ts (3)
  - src/channels/plugins/catalog.ts (3)
  - src/channels/plugins/actions/discord/handle-action.guild-admin.ts (2)
- sample_dst_files:
  - src/agents/tools/common.ts (7)
  - src/plugins/runtime.ts (5)

## gateway_control_plane->plugin_tool_extension
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 21, call_edges: 2, call_ratio: 0.0952
- src_file_count: 12, dst_file_count: 10
- hypothesis_reason: gateway startup/invoke path integrates plugin services
- sample_src_files:
  - src/gateway/server-startup.ts (6)
  - src/gateway/test-helpers.mocks.ts (3)
  - src/gateway/server-close.ts (2)
- sample_dst_files:
  - src/plugins/runtime.ts (3)
  - src/hooks/gmail-watcher.ts (3)

## platform_hosts->ops_client_entry
- status_candidate: `remove_candidate`
- evidence_tag: `accidental_likely`
- total_edges: 8, call_edges: 1, call_ratio: 0.125
- src_file_count: 5, dst_file_count: 5
- hypothesis_reason: host->CLI/browser reverse dependency likely accidental
- sample_src_files:
  - src/node-host/runner.ts (3)
  - src/daemon/systemd-hints.ts (2)
  - src/daemon/launchd.ts (1)
- sample_dst_files:
  - src/terminal/theme.ts (3)
  - src/cli/command-format.ts (2)

## external_dependencies->foundation_infra
- status_candidate: `temporary_candidate`
- evidence_tag: `mixed`
- total_edges: 7, call_edges: 2, call_ratio: 0.2857
- src_file_count: 3, dst_file_count: 5
- hypothesis_reason: provider currently touches config/log storage helpers
- sample_src_files:
  - src/providers/github-copilot-auth.ts (3)
  - src/providers/github-copilot-token.ts (3)
  - src/providers/github-copilot-models.ts (1)
- sample_dst_files:
  - src/config/logging.ts (2)
  - src/infra/json-file.ts (2)

## foundation_infra->plugin_tool_extension
- status_candidate: `remove_candidate`
- evidence_tag: `accidental_likely`
- total_edges: 7, call_edges: 1, call_ratio: 0.1429
- src_file_count: 4, dst_file_count: 6
- hypothesis_reason: infra->plugin reverse dependency likely layer breach
- sample_src_files:
  - src/config/validation.ts (3)
  - src/infra/outbound/message-action-runner.ts (2)
  - src/test-utils/channel-plugins.ts (1)
- sample_dst_files:
  - src/agents/tools/common.ts (2)
  - src/plugins/config-state.ts (1)

## external_dependencies->ops_client_entry
- status_candidate: `remove_candidate`
- evidence_tag: `accidental_likely`
- total_edges: 5, call_edges: 1, call_ratio: 0.2
- src_file_count: 2, dst_file_count: 4
- hypothesis_reason: provider->CLI reverse dependency likely accidental
- sample_src_files:
  - src/providers/github-copilot-auth.ts (3)
  - src/providers/qwen-portal-oauth.ts (2)
- sample_dst_files:
  - src/cli/command-format.ts (2)
  - src/commands/models/shared.ts (1)

## platform_hosts->agent_runtime
- status_candidate: `remove_candidate`
- evidence_tag: `accidental_likely`
- total_edges: 4, call_edges: 0, call_ratio: 0
- src_file_count: 3, dst_file_count: 2
- hypothesis_reason: host->runtime reverse dependency should go through protocol ports
- sample_src_files:
  - src/node-host/runner.ts (2)
  - src/canvas-host/a2ui.ts (1)
  - src/canvas-host/server.ts (1)
- sample_dst_files:
  - src/media/mime.ts (3)
  - src/agents/agent-scope.ts (1)

## ops_client_entry->external_dependencies
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 2, call_edges: 0, call_ratio: 0
- src_file_count: 2, dst_file_count: 1
- hypothesis_reason: auth/login flows initiated by operational commands
- sample_src_files:
  - src/commands/auth-choice.apply.github-copilot.ts (1)
  - src/commands/models.ts (1)
- sample_dst_files:
  - src/providers/github-copilot-auth.ts (2)

## agent_runtime->external_dependencies
- status_candidate: `keep_candidate`
- evidence_tag: `essential_likely`
- total_edges: 2, call_edges: 0, call_ratio: 0
- src_file_count: 2, dst_file_count: 2
- hypothesis_reason: provider auth/model handshake at runtime
- sample_src_files:
  - src/agents/auth-profiles/oauth.ts (1)
  - src/agents/models-config.providers.ts (1)
- sample_dst_files:
  - src/providers/qwen-portal-oauth.ts (1)
  - src/providers/github-copilot-token.ts (1)

## foundation_infra->platform_hosts
- status_candidate: `temporary_candidate`
- evidence_tag: `mixed`
- total_edges: 2, call_edges: 1, call_ratio: 0.5
- src_file_count: 1, dst_file_count: 1
- hypothesis_reason: restart and host constants coupling may be interfaceable
- sample_src_files:
  - src/infra/restart.ts (2)
- sample_dst_files:
  - src/daemon/constants.ts (2)

## external_dependencies->agent_runtime
- status_candidate: `remove_candidate`
- evidence_tag: `accidental_likely`
- total_edges: 2, call_edges: 1, call_ratio: 0.5
- src_file_count: 1, dst_file_count: 1
- hypothesis_reason: reverse provider->runtime dependency should be constrained by ports
- sample_src_files:
  - src/providers/github-copilot-auth.ts (2)
- sample_dst_files:
  - src/agents/auth-profiles.ts (2)

