# Architecture Debt Report

- generated_at: 2026-02-17T11:30:21Z
- source_summary: `/home/wangbo/document/Cli/compiler/bcc/examples/cross-project-arch-comparison/analysis/gong-live-20260217-192706/v1-validate/summary.json`

## Scenario Validation

| name | unexpected_edges_count | forbidden_edges_count | missing_edges_count |
|---|---:|---:|---:|
| v3_target | 4 | 0 | 4 |
| v3_transition | 0 | 0 | 4 |

## Gate Evaluation

| profile | metric | actual | limit | pass |
|---|---|---:|---:|---|
| target | unexpected_edges_count | 4 | 0 | fail |
| target | forbidden_edges_count | 0 | 0 | pass |
| target | forbidden_total_edges | 0 | 0 | pass |
| target | missing_edges_count | 4 | 0 | fail |
| target | directed_density_pct | 16.67 | 40 | pass |
| target | bidirectional_pair_count | 0 | 5 | pass |
| transition | unexpected_edges_count | 0 | 0 | pass |
| transition | forbidden_edges_count | 0 | 5 | pass |
| transition | forbidden_total_edges | 0 | 26 | pass |
| transition | missing_edges_count | 4 | 0 | fail |
| transition | directed_density_pct | 16.67 | 75 | pass |
| transition | bidirectional_pair_count | 0 | 20 | pass |
