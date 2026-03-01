use serde::Deserialize;
use serde_json::Value;
use std::collections::BTreeSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::thread;
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

#[derive(Debug, Deserialize)]
struct RuntimeBridgeRuntime {
    package: String,
    mode: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RuntimeBridgeContractSource {
    path: String,
    format: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RuntimeBridgeDocument {
    runtime: RuntimeBridgeRuntime,
    contract_source: RuntimeBridgeContractSource,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RuntimeConsumableAction {
    action: String,
    graphql_kind: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RuntimeConsumableContract {
    contract_key: String,
    actions: Vec<RuntimeConsumableAction>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RuntimeConsumableContractDocument {
    contracts: Vec<RuntimeConsumableContract>,
}

fn assert_bridge_artifacts_parseable(output: &Path) {
    let unibo_raw =
        fs::read_to_string(output.join("unibo-api-contract.json")).expect("read unibo contract");
    let _: Value = serde_json::from_str(&unibo_raw).expect("parse unibo contract");

    let legacy_raw =
        fs::read_to_string(output.join("api-contract.json")).expect("read legacy contract");
    let _: Value = serde_json::from_str(&legacy_raw).expect("parse legacy contract");

    let bridge_raw =
        fs::read_to_string(output.join("unibo-runtime-bridge.yaml")).expect("read runtime bridge");
    let _: RuntimeBridgeDocument = serde_yaml::from_str(&bridge_raw).expect("parse runtime bridge");
}

fn load_runtime_visible_actions(output: &Path) -> (BTreeSet<String>, BTreeSet<String>) {
    let bridge_raw = fs::read_to_string(output.join("unibo-runtime-bridge.yaml"))
        .expect("read runtime bridge config");
    let bridge: RuntimeBridgeDocument =
        serde_yaml::from_str(&bridge_raw).expect("parse runtime bridge config");
    assert_eq!(bridge.runtime.package, "unibo_graphql_runtime");
    assert_eq!(bridge.runtime.mode, "reuse");
    assert_eq!(bridge.contract_source.format, "json");

    let relative_path = bridge.contract_source.path.trim_start_matches("./");
    let contract_path = output.join(relative_path);
    let contract_raw = fs::read_to_string(&contract_path).expect("read runtime contract source");
    let payload: RuntimeConsumableContractDocument =
        serde_json::from_str(&contract_raw).expect("parse runtime contract json");

    let mut queries = BTreeSet::new();
    let mut mutations = BTreeSet::new();
    for contract in payload.contracts {
        for action in contract.actions {
            let normalized = format!("{}:{}", contract.contract_key, action.action);
            match action.graphql_kind.as_str() {
                "query" => {
                    queries.insert(normalized);
                }
                "mutation" => {
                    mutations.insert(normalized);
                }
                other => panic!("unexpected graphql kind for runtime visibility: {}", other),
            }
        }
    }
    (queries, mutations)
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
    assert!(out.join("api-contract.json").exists());
    assert!(!out.join("unibo-runtime-bridge.yaml").exists());

    let mut names: Vec<String> = fs::read_dir(&out)
        .expect("read output dir")
        .filter_map(Result::ok)
        .map(|entry| entry.file_name().to_string_lossy().to_string())
        .collect();
    names.sort();
    assert_eq!(
        names,
        vec![
            "api-contract.json".to_string(),
            "unibo-api-contract.json".to_string()
        ]
    );

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

    assert_bridge_artifacts_parseable(&out);
    let (queries, mutations) = load_runtime_visible_actions(&out);
    let expected_queries: BTreeSet<String> = [
        "account.list_users:list_users",
        "billing.invoice:list",
        "billing.invoice:read",
    ]
    .into_iter()
    .map(str::to_string)
    .collect();
    let expected_mutations: BTreeSet<String> = [
        "billing.create_invoice:create_invoice",
        "billing.invoice:create",
        "billing.invoice:delete",
        "billing.invoice:update",
    ]
    .into_iter()
    .map(str::to_string)
    .collect();
    assert_eq!(queries, expected_queries);
    assert_eq!(mutations, expected_mutations);

    let _ = fs::remove_dir_all(&out);
}

#[test]
fn runtime_bridge_should_bootstrap_runtime_loader_from_contract_source() {
    let root = temp_dir("bcc_cli_bridge_runtime_loader");
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

    let status = run_arch_generate(&seed, &out, true, "error-on-conflict");
    assert!(status.success(), "bridge generation should succeed");

    let (queries, mutations) = load_runtime_visible_actions(&out);
    assert!(
        queries.is_empty(),
        "command-only seed should not expose query"
    );
    assert!(mutations.contains("billing.create_invoice:create_invoice"));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn bridge_parallel_writes_to_same_output_should_keep_artifacts_parseable() {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let seed = manifest_dir.join("examples/openclaw-arch/bridge.seed.yaml");
    let out = temp_dir("bcc_cli_bridge_parallel_write");
    fs::create_dir_all(&out).expect("create output dir");

    let mut handles = Vec::new();
    for _ in 0..4 {
        let seed = seed.clone();
        let out = out.clone();
        handles.push(thread::spawn(move || {
            run_arch_generate(&seed, &out, true, "error-on-conflict")
        }));
    }

    for handle in handles {
        let status = handle.join().expect("join parallel bridge generation");
        assert!(
            status.success(),
            "parallel bridge generation should succeed"
        );
    }

    for _ in 0..3 {
        assert_bridge_artifacts_parseable(&out);
        let (queries, mutations) = load_runtime_visible_actions(&out);
        assert!(!queries.is_empty(), "runtime queries should not be empty");
        assert!(
            !mutations.is_empty(),
            "runtime mutations should not be empty"
        );
    }

    let _ = fs::remove_dir_all(&out);
}
