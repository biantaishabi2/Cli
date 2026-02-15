# V2 Validation Report (Research Draft)

This report summarizes structure metrics and scenario replay for V2 research matrix.

## Structure Complexity
- modules: 8
- directed_edges_possible: 56
- directed_edges_actual: 42
- directed_density_pct: 75.00%
- total_module_edge_weight: 4133
- bidirectional_pair_count: 20

### Top Bidirectional Pairs (by total edge weight)
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

## Candidate Status Aggregates
- keep_candidate: edges=13, total_edges=676, edge_weight_share=16.36%
- temporary_candidate: edges=10, total_edges=442, edge_weight_share=10.69%
- remove_candidate: edges=5, total_edges=26, edge_weight_share=0.63%

## Scenario Replay
### baseline_v1
- allow_count: 33, forbid_count: 1
- matched_edges_count: 33, matched_total_edges: 4094
- unexpected_edges_count: 9, unexpected_total_edges: 39
- forbidden_edges_count: 0, forbidden_total_edges: 0
- missing_edges_count: 0

### v2_balanced_allow_keep_and_temporary
- allow_count: 37, forbid_count: 6
- matched_edges_count: 37, matched_total_edges: 4107
- unexpected_edges_count: 0, unexpected_total_edges: 0
- forbidden_edges_count: 5, forbidden_total_edges: 26
- missing_edges_count: 0

### v2_strict_allow_keep_only
- allow_count: 27, forbid_count: 6
- matched_edges_count: 27, matched_total_edges: 3665
- unexpected_edges_count: 10, unexpected_total_edges: 442
- forbidden_edges_count: 5, forbidden_total_edges: 26
- missing_edges_count: 0

## Notes
- This is a measurement and hypothesis report; it is not the final architecture ruling.
- Final keep/remove decisions should follow semantic sampling review and remediation feasibility checks.
