use serde_json::Value;
use std::path::{Path, PathBuf};
use std::process::Command;
use taskctl::{PlanInput, plan};

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
    assert!(diagnostics.warnings.iter().any(|t| t.contains("or:g->a")));
}
