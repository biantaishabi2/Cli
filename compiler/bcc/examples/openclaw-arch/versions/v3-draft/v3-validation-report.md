# V3 Validation Report

## Structure Snapshot (actual code)
- modules: 8
- directed_edges_possible: 56
- directed_edges_actual: 42
- directed_density_pct: 75%
- bidirectional_pair_count: 20

### Top Bidirectional Pairs
- foundation_infra<->ops_client_entry: 865 (44 + 821)
- agent_runtime<->foundation_infra: 647 (562 + 85)
- channel_ingress<->foundation_infra: 489 (398 + 91)
- foundation_infra<->gateway_control_plane: 334 (40 + 294)
- agent_runtime<->ops_client_entry: 281 (27 + 254)
- agent_runtime<->channel_ingress: 264 (73 + 191)
- foundation_infra<->plugin_tool_extension: 177 (7 + 170)
- agent_runtime<->plugin_tool_extension: 151 (43 + 108)
- channel_ingress<->plugin_tool_extension: 145 (24 + 121)
- gateway_control_plane<->ops_client_entry: 142 (37 + 105)

## v3_target
- allow_count: 27
- forbid_count: 6
- unexpected_edges_count: 10
- unexpected_total_edges: 442
- forbidden_edges_count: 5
- forbidden_total_edges: 26
- missing_edges_count: 0
- gate_result: FAIL

### Forbidden Top
- platform_hosts->ops_client_entry: 8
- foundation_infra->plugin_tool_extension: 7
- external_dependencies->ops_client_entry: 5
- platform_hosts->agent_runtime: 4
- external_dependencies->agent_runtime: 2

### Unexpected Top
- plugin_tool_extension->channel_ingress: 121
- foundation_infra->channel_ingress: 91
- foundation_infra->ops_client_entry: 44
- channel_ingress->ops_client_entry: 41
- foundation_infra->gateway_control_plane: 40
- gateway_control_plane->ops_client_entry: 37
- plugin_tool_extension->ops_client_entry: 32
- agent_runtime->ops_client_entry: 27
- external_dependencies->foundation_infra: 7
- foundation_infra->platform_hosts: 2

## v3_transition
- allow_count: 37
- forbid_count: 6
- unexpected_edges_count: 0
- unexpected_total_edges: 0
- forbidden_edges_count: 5
- forbidden_total_edges: 26
- missing_edges_count: 0
- gate_result: PASS

### Forbidden Top
- platform_hosts->ops_client_entry: 8
- foundation_infra->plugin_tool_extension: 7
- external_dependencies->ops_client_entry: 5
- platform_hosts->agent_runtime: 4
- external_dependencies->agent_runtime: 2

## Gate Checks
- [target] unexpected_edges_count: actual=10, limit=0 => FAIL
- [target] forbidden_edges_count: actual=5, limit=0 => FAIL
- [target] forbidden_total_edges: actual=26, limit=0 => FAIL
- [target] missing_edges_count: actual=0, limit=0 => PASS
- [target] directed_density_pct: actual=75, limit=40 => FAIL
- [target] bidirectional_pair_count: actual=20, limit=5 => FAIL
- [transition] unexpected_edges_count: actual=0, limit=0 => PASS
- [transition] forbidden_edges_count: actual=5, limit=5 => PASS
- [transition] forbidden_total_edges: actual=26, limit=26 => PASS
- [transition] missing_edges_count: actual=0, limit=0 => PASS
- [transition] directed_density_pct: actual=75, limit=75 => PASS
- [transition] bidirectional_pair_count: actual=20, limit=20 => PASS
