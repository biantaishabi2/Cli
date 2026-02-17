# V3 Validation Report

## Structure Snapshot (actual code)
- modules: 7
- directed_edges_possible: 42
- directed_edges_actual: 7
- directed_density_pct: 16.67%
- bidirectional_pair_count: 0

### Top Bidirectional Pairs

## v3_target
- allow_count: 7
- forbid_count: 0
- unexpected_edges_count: 0
- unexpected_total_edges: 0
- forbidden_edges_count: 0
- forbidden_total_edges: 0
- missing_edges_count: 0
- gate_result: PASS

## v3_transition
- allow_count: 7
- forbid_count: 0
- unexpected_edges_count: 0
- unexpected_total_edges: 0
- forbidden_edges_count: 0
- forbidden_total_edges: 0
- missing_edges_count: 0
- gate_result: PASS

## Gate Checks
- [target] unexpected_edges_count: actual=0, limit=0 => PASS
- [target] forbidden_edges_count: actual=0, limit=0 => PASS
- [target] forbidden_total_edges: actual=0, limit=0 => PASS
- [target] missing_edges_count: actual=0, limit=0 => PASS
- [target] directed_density_pct: actual=16.67, limit=40 => PASS
- [target] bidirectional_pair_count: actual=0, limit=5 => PASS
- [transition] unexpected_edges_count: actual=0, limit=0 => PASS
- [transition] forbidden_edges_count: actual=0, limit=5 => PASS
- [transition] forbidden_total_edges: actual=0, limit=26 => PASS
- [transition] missing_edges_count: actual=0, limit=0 => PASS
- [transition] directed_density_pct: actual=16.67, limit=75 => PASS
- [transition] bidirectional_pair_count: actual=0, limit=20 => PASS
