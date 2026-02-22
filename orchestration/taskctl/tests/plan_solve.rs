use serde_json::Value;
use std::path::{Path, PathBuf};
use std::process::Command;
use taskctl::{plan, PlanInput};

fn fixture_path(name: &str) -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests")
        .join("fixtures")
        .join("plan")
        .join(name)
}

#[test]
fn plan_tie_break_fixture_is_deterministic() {
    let bin = env!("CARGO_BIN_EXE_taskctl");
    let fixture = fixture_path("plan_tie_break.json");

    let output = Command::new(bin)
        .args([
            "plan",
            "solve",
            "--input",
            fixture.to_str().expect("fixture path utf8"),
        ])
        .output()
        .expect("run taskctl");

    assert_eq!(output.status.code(), Some(0));
    let stdout = String::from_utf8(output.stdout).expect("stdout utf8");
    let json: Value = serde_json::from_str(&stdout).expect("json");

    assert_eq!(json["result"], Value::String("ok".to_string()));
    let selected_nodes = json["plan_decision"]["selected_nodes"]
        .as_array()
        .expect("selected_nodes array");
    assert!(selected_nodes.iter().any(|v| v == "path-a"));
    assert!(!selected_nodes.iter().any(|v| v == "path-b"));
}

#[test]
fn plan_decision_trace_is_diagnostic() {
    let input = serde_json::from_str::<PlanInput>(
        r#"{
            "root":"g",
            "nodes":[
                {"node_id":"g","node_type":"or","score":0.0,"confidence":1.0,"children":["b","a"]},
                {"node_id":"a","node_type":"leaf","score":0.5,"confidence":0.8,"evidence_id":"ev-a"},
                {"node_id":"b","node_type":"leaf","score":0.5,"confidence":0.8,"evidence_id":"ev-b"}
            ]
        }"#,
    )
    .expect("plan json");

    let (decision, diagnostics) = plan::solve(input).expect("solve");
    assert!(decision.trace.iter().any(|t| t.contains("or:g->a")));
    // trace 不再重复写入 warnings，warnings 应为空
    assert!(diagnostics.warnings.is_empty());
}

#[test]
fn plan_empty_nodes_returns_e0001() {
    let input = PlanInput {
        root: "missing".to_string(),
        nodes: Vec::new(),
    };

    let err = plan::solve(input).expect_err("expected invalid input");
    assert_eq!(err.code(), "E0001");
}

#[test]
fn plan_rejects_duplicate_node_id() {
    let input = serde_json::from_str::<PlanInput>(
        r#"{
            "root":"root",
            "nodes":[
                {"node_id":"root","node_type":"or","score":0.0,"confidence":1.0,"children":["a"]},
                {"node_id":"a","node_type":"leaf","score":0.5,"confidence":0.8},
                {"node_id":"a","node_type":"leaf","score":0.6,"confidence":0.7}
            ]
        }"#,
    )
    .expect("plan json");

    let err = plan::solve(input).expect_err("expected duplicate id error");
    assert_eq!(err.code(), "E0001");
}

#[test]
fn plan_rejects_leaf_with_children() {
    let input = serde_json::from_str::<PlanInput>(
        r#"{
            "root":"leaf-root",
            "nodes":[
                {"node_id":"leaf-root","node_type":"leaf","score":0.2,"confidence":0.8,"children":["x"]},
                {"node_id":"x","node_type":"leaf","score":0.1,"confidence":0.7}
            ]
        }"#,
    )
    .expect("plan json");

    let err = plan::solve(input).expect_err("expected invalid leaf");
    assert_eq!(err.code(), "E0001");
}

#[test]
fn plan_cycle_returns_e0001_instead_of_recursive_overflow() {
    let input = serde_json::from_str::<PlanInput>(
        r#"{
            "root":"a",
            "nodes":[
                {"node_id":"a","node_type":"and","score":0.0,"confidence":1.0,"children":["b"]},
                {"node_id":"b","node_type":"and","score":0.0,"confidence":1.0,"children":["a"]}
            ]
        }"#,
    )
    .expect("plan json");

    let err = plan::solve(input).expect_err("expected cycle error");
    assert_eq!(err.code(), "E0001");
}

#[test]
fn plan_and_node_confidence_averages_only_children() {
    let input = serde_json::from_str::<PlanInput>(
        r#"{
            "root":"and_root",
            "nodes":[
                {"node_id":"and_root","node_type":"and","score":0.0,"confidence":0.0,"children":["c1","c2"]},
                {"node_id":"c1","node_type":"leaf","score":1.0,"confidence":1.0},
                {"node_id":"c2","node_type":"leaf","score":1.0,"confidence":0.6}
            ]
        }"#,
    )
    .expect("plan json");

    let (decision, _) = plan::solve(input).expect("solve");
    // AND confidence = average of children only: (1.0 + 0.6) / 2 = 0.8
    // NOT (0.0 + 1.0 + 0.6) / 3 = 0.533
    assert!((decision.total_confidence - 0.8).abs() < 1e-9);
    assert_eq!(decision.total_score, 2.0); // 0.0 + 1.0 + 1.0
}

#[test]
fn plan_deep_chain_solves_without_panic() {
    let depth = 256usize;
    let mut nodes = Vec::new();
    for i in 0..depth {
        let id = format!("n{i}");
        let children = if i + 1 < depth {
            vec![format!("n{}", i + 1)]
        } else {
            Vec::new()
        };
        let node_type = if children.is_empty() { "leaf" } else { "and" };
        nodes.push(serde_json::json!({
            "node_id": id,
            "node_type": node_type,
            "score": 0.1,
            "confidence": 0.9,
            "children": children
        }));
    }

    let input = PlanInput {
        root: "n0".to_string(),
        nodes: nodes
            .into_iter()
            .map(|v| serde_json::from_value(v).expect("node json"))
            .collect(),
    };

    let (decision, _) = plan::solve(input).expect("solve deep chain");
    assert!(decision.selected_nodes.contains(&"n0".to_string()));
    assert!(decision.selected_nodes.contains(&format!("n{}", depth - 1)));
}
