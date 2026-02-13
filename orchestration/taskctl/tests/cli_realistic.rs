use serde_json::Value;
use std::collections::BTreeSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

fn fixture_path(name: &str) -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests")
        .join("fixtures")
        .join(name)
}

fn temp_store_path(suffix: &str) -> PathBuf {
    let ts = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_nanos())
        .unwrap_or(0);
    std::env::temp_dir().join(format!("taskctl-it-{}-{suffix}-{ts}.json", std::process::id()))
}

fn run_cli(args: &[String]) -> String {
    let bin = env!("CARGO_BIN_EXE_taskctl");
    let output = Command::new(bin)
        .args(args)
        .output()
        .expect("failed to execute taskctl");

    if !output.status.success() {
        panic!(
            "taskctl failed\nargs: {:?}\nstatus: {:?}\nstderr: {}",
            args,
            output.status.code(),
            String::from_utf8_lossy(&output.stderr)
        );
    }

    String::from_utf8(output.stdout).expect("stdout is not utf8")
}

#[test]
fn realistic_fixture_generates_expected_dag() {
    let store = fixture_path("realistic_tasks.json");
    let expected_dag = fixture_path("realistic_dag.json");

    let validate = run_cli(&[
        "--store".into(),
        store.display().to_string(),
        "validate".into(),
    ]);
    let validate_json: Value = serde_json::from_str(&validate).expect("validate json");
    assert_eq!(validate_json["ok"], Value::Bool(true));

    let dag_raw = run_cli(&[
        "--store".into(),
        store.display().to_string(),
        "dag".into(),
    ]);
    let generate_raw = run_cli(&[
        "--store".into(),
        store.display().to_string(),
        "generate".into(),
    ]);

    let dag_json: Value = serde_json::from_str(&dag_raw).expect("dag json");
    let generate_json: Value = serde_json::from_str(&generate_raw).expect("generate json");
    let expected_json: Value =
        serde_json::from_str(&fs::read_to_string(expected_dag).expect("read expected dag"))
            .expect("expected dag json");

    assert_eq!(dag_json, generate_json);
    assert_eq!(dag_json, expected_json);

    assert_eq!(dag_json["nodes"].as_array().map_or(0, |a| a.len()), 13);
    assert_eq!(dag_json["edges"].as_array().map_or(0, |a| a.len()), 18);
    assert_eq!(dag_json["layers"].as_array().map_or(0, |a| a.len()), 7);

    let ready_raw = run_cli(&[
        "--store".into(),
        store.display().to_string(),
        "ready".into(),
    ]);
    let ready_json: Value = serde_json::from_str(&ready_raw).expect("ready json");
    let subjects: BTreeSet<String> = ready_json
        .as_array()
        .expect("ready array")
        .iter()
        .filter_map(|item| item.get("subject").and_then(Value::as_str))
        .map(ToString::to_string)
        .collect();

    let expected: BTreeSet<String> = [
        "Generate BDD Scenario Pack".to_string(),
        "Implement Application Service Orchestrator".to_string(),
    ]
    .into_iter()
    .collect();

    assert_eq!(subjects, expected);
}

#[test]
fn create_update_validate_and_ascii_flow() {
    let store = temp_store_path("flow");

    let a_raw = run_cli(&[
        "--store".into(),
        store.display().to_string(),
        "create".into(),
        "--subject".into(),
        "Design contract".into(),
        "--description".into(),
        "Prepare API contract and examples".into(),
        "--active-form".into(),
        "Designing contract".into(),
        "--metadata".into(),
        "{\"priority\":\"P0\",\"module\":\"contract\"}".into(),
    ]);
    let b_raw = run_cli(&[
        "--store".into(),
        store.display().to_string(),
        "create".into(),
        "--subject".into(),
        "Implement service".into(),
        "--description".into(),
        "Implement application service".into(),
        "--active-form".into(),
        "Implementing service".into(),
        "--metadata".into(),
        "{\"priority\":\"P1\",\"module\":\"application\"}".into(),
    ]);

    let a = serde_json::from_str::<Value>(&a_raw).expect("create a json");
    let b = serde_json::from_str::<Value>(&b_raw).expect("create b json");
    let a_id = a["id"].as_str().expect("a id").to_string();
    let b_id = b["id"].as_str().expect("b id").to_string();

    run_cli(&[
        "--store".into(),
        store.display().to_string(),
        "update".into(),
        "--task-id".into(),
        b_id.clone(),
        "--add-blocked-by".into(),
        a_id.clone(),
        "--owner".into(),
        "app@team".into(),
    ]);

    run_cli(&[
        "--store".into(),
        store.display().to_string(),
        "update".into(),
        "--task-id".into(),
        a_id.clone(),
        "--status".into(),
        "in-progress".into(),
    ]);
    run_cli(&[
        "--store".into(),
        store.display().to_string(),
        "update".into(),
        "--task-id".into(),
        a_id,
        "--status".into(),
        "completed".into(),
    ]);

    let dag_ascii = run_cli(&[
        "--store".into(),
        store.display().to_string(),
        "dag-ascii".into(),
    ]);
    assert!(dag_ascii.contains("DAG Layers"));
    assert!(dag_ascii.contains("Design contract -> Implement service"));

    let validate = run_cli(&[
        "--store".into(),
        store.display().to_string(),
        "validate".into(),
    ]);
    let validate_json: Value = serde_json::from_str(&validate).expect("validate json");
    assert_eq!(validate_json["ok"], Value::Bool(true));

    let _ = fs::remove_file(store);
}
