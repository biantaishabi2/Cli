use serde_json::Value;
use std::path::{Path, PathBuf};
use std::process::Command;
use taskctl::{ExecuteInput, execute};

fn fixture_path(name: &str) -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests")
        .join("fixtures")
        .join("execute")
        .join(name)
}

#[test]
fn execute_compile_success_outputs_dag() {
    let input = ExecuteInput {
        nodes: vec![
            taskctl::ExecuteNode {
                node_id: "A".to_string(),
                depends_on: vec![],
            },
            taskctl::ExecuteNode {
                node_id: "B".to_string(),
                depends_on: vec!["A".to_string()],
            },
            taskctl::ExecuteNode {
                node_id: "C".to_string(),
                depends_on: vec!["B".to_string()],
            },
        ],
    };

    let (dag, diagnostics) = execute::compile(input).expect("compile success");
    assert_eq!(dag.topo_order, vec!["A", "B", "C"]);
    assert!(diagnostics.conflicts.is_empty());
}

#[test]
fn execute_cycle_error_returns_e1001_and_cycle_path() {
    let bin = env!("CARGO_BIN_EXE_taskctl");
    let fixture = fixture_path("execute_cycle.json");

    let output = Command::new(bin)
        .args([
            "execute",
            "compile",
            "--input",
            fixture.to_str().expect("fixture path utf8"),
        ])
        .output()
        .expect("run taskctl");

    assert_eq!(output.status.code(), Some(2));
    let stdout = String::from_utf8(output.stdout).expect("stdout utf8");
    let json: Value = serde_json::from_str(&stdout).expect("json");

    assert_eq!(json["result"], Value::String("error".to_string()));
    assert_eq!(json["error"]["code"], Value::String("E1001".to_string()));
    assert_eq!(json["error"]["cycle"], serde_json::json!(["A", "B", "A"]));
}
