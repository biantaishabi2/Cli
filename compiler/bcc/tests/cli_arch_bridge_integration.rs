use serde_json::Value;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

fn temp_dir(prefix: &str) -> PathBuf {
    let ts = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("clock")
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("{}_{}_{}", prefix, std::process::id(), ts));
    fs::create_dir_all(&dir).expect("create temp dir");
    dir
}

fn run_arch_generate(
    seed: &Path,
    output: &Path,
    emit_runtime_bridge: bool,
    conflict_strategy: &str,
) -> std::process::ExitStatus {
    Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "generate",
            "--seed-file",
            &seed.to_string_lossy(),
            "--emit",
            "api-contract",
            "--emit-runtime-bridge",
            if emit_runtime_bridge { "true" } else { "false" },
            "--conflict-strategy",
            conflict_strategy,
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run arch generate")
}

fn sanitize_generated_at(mut payload: Value) -> Value {
    if let Some(obj) = payload.as_object_mut() {
        if obj.contains_key("generatedAt") {
            obj.insert(
                "generatedAt".to_string(),
                Value::String("<normalized-generated-at>".to_string()),
            );
        }
    }
    payload
}

#[test]
fn arch_generate_emit_api_contract_without_runtime_bridge_only_outputs_contract() {
    let root = temp_dir("bcc_cli_bridge_only_contract");
    let seed = root.join("seed.yaml");
    let out = root.join("out");

    fs::write(
        &seed,
        r#"version: v1
modules:
  - module_id: billing
    precedence: 1
    path_rules:
      include: ["src/billing/**"]
relations_expected: []
boundaries:
  - module_id: billing
    contracts:
      - name: create_invoice
        kind: command
        input:
          order_id: uuid
        output:
          invoice_id: uuid
"#,
    )
    .expect("write seed");

    let status = run_arch_generate(&seed, &out, false, "error-on-conflict");
    assert!(status.success());

    assert!(out.join("unibo-api-contract.json").exists());
    assert!(!out.join("unibo-runtime-bridge.yaml").exists());

    let mut names: Vec<String> = fs::read_dir(&out)
        .expect("read output dir")
        .filter_map(Result::ok)
        .map(|entry| entry.file_name().to_string_lossy().to_string())
        .collect();
    names.sort();
    assert_eq!(names, vec!["unibo-api-contract.json".to_string()]);

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn openclaw_bridge_example_should_match_expected_artifacts() {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let seed = manifest_dir.join("examples/openclaw-arch/bridge.seed.yaml");
    let expected_contract =
        manifest_dir.join("examples/openclaw-arch/expected/unibo-api-contract.json");
    let expected_bridge =
        manifest_dir.join("examples/openclaw-arch/expected/unibo-runtime-bridge.yaml");

    assert!(seed.exists(), "bridge seed should exist");
    assert!(expected_contract.exists(), "expected contract should exist");
    assert!(expected_bridge.exists(), "expected bridge should exist");

    let out = temp_dir("bcc_cli_bridge_openclaw");
    let status = run_arch_generate(&seed, &out, true, "error-on-conflict");
    assert!(
        status.success(),
        "openclaw bridge generation should succeed"
    );

    let actual_contract: Value = serde_json::from_str(
        &fs::read_to_string(out.join("unibo-api-contract.json")).expect("read actual contract"),
    )
    .expect("parse actual contract");
    let expected_contract_payload: Value = serde_json::from_str(
        &fs::read_to_string(expected_contract).expect("read expected contract"),
    )
    .expect("parse expected contract");

    assert_eq!(
        sanitize_generated_at(actual_contract),
        sanitize_generated_at(expected_contract_payload),
        "openclaw unibo-api-contract.json should match expected"
    );

    let actual_bridge =
        fs::read_to_string(out.join("unibo-runtime-bridge.yaml")).expect("read actual bridge");
    let expected_bridge_raw = fs::read_to_string(expected_bridge).expect("read expected bridge");
    assert_eq!(
        actual_bridge, expected_bridge_raw,
        "openclaw unibo-runtime-bridge.yaml should match expected"
    );

    let _ = fs::remove_dir_all(&out);
}
