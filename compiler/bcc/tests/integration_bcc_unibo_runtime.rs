#[path = "gate_common.rs"]
mod gate_common;
#[path = "unibo_runtime_probe.rs"]
mod unibo_runtime_probe;

use gate_common::GateFinding;
use serde::Deserialize;
use serde_json::{json, Value};
use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::thread;
use std::time::{Instant, SystemTime, UNIX_EPOCH};

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

#[derive(Debug, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
struct RuntimeConsumableAction {
    action: String,
    graphql_kind: String,
    #[serde(default)]
    input: BTreeMap<String, String>,
    #[serde(default)]
    output: BTreeMap<String, String>,
}

#[derive(Debug, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
struct RuntimeConsumableContract {
    contract_key: String,
    actions: Vec<RuntimeConsumableAction>,
}

#[derive(Debug, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
struct RuntimeConsumableContractDocument {
    contracts: Vec<RuntimeConsumableContract>,
}

#[derive(Debug, Deserialize)]
struct ExpectedRuntimeActions {
    queries: Vec<String>,
    mutations: Vec<String>,
}

fn temp_dir(prefix: &str) -> PathBuf {
    let ts = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("clock")
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("{}_{}_{}", prefix, std::process::id(), ts));
    fs::create_dir_all(&dir).expect("create temp dir");
    dir
}

fn run_arch_generate(seed: &Path, output: &Path) {
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "generate",
            "--seed-file",
            &seed.to_string_lossy(),
            "--emit",
            "api-contract",
            "--emit-runtime-bridge",
            "true",
            "--conflict-strategy",
            "error-on-conflict",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run arch generate for runtime integration");
    assert!(status.success(), "arch generate should succeed");
}

fn load_runtime_contract(output: &Path) -> RuntimeConsumableContractDocument {
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
    serde_json::from_str(&contract_raw).expect("parse runtime contract json")
}

fn load_runtime_visible_actions(
    payload: &RuntimeConsumableContractDocument,
) -> (BTreeSet<String>, BTreeSet<String>) {
    let mut queries = BTreeSet::new();
    let mut mutations = BTreeSet::new();
    for contract in &payload.contracts {
        for action in &contract.actions {
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

fn runtime_probe_or_skip(test_name: &str) -> Option<unibo_runtime_probe::RuntimeProbe> {
    match unibo_runtime_probe::RuntimeProbe::resolve() {
        unibo_runtime_probe::RuntimeProbeAvailability::Available(probe) => Some(probe),
        unibo_runtime_probe::RuntimeProbeAvailability::Missing { message, strict } => {
            if strict {
                panic!("[{}] {}", test_name, message);
            }
            eprintln!("[{}] skip runtime execution: {}", test_name, message);
            None
        }
    }
}

#[test]
fn bcc_unibo_runtime_bridge_smoke_should_keep_runtime_reuse_contract() {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let seed = root.join("examples/unibo-bridge-smoke/bridge.seed.yaml");
    let expected_actions_path =
        root.join("examples/unibo-bridge-smoke/expected-runtime-actions.json");
    assert!(seed.exists(), "smoke seed should exist");
    assert!(
        expected_actions_path.exists(),
        "smoke expected actions should exist"
    );

    let out = temp_dir("bcc_unibo_runtime_smoke");
    let mut findings = Vec::new();

    let generate_started = Instant::now();
    run_arch_generate(&seed, &out);
    let generate_elapsed = generate_started.elapsed().as_millis();
    println!(
        "[runtime-smoke] request=generate_bridge_artifacts duration_ms={} result=PASS summary=artifacts_ready",
        generate_elapsed
    );
    findings.push(GateFinding::new(
        "SAFE:runtime_smoke:generate_bridge_artifacts",
        "SAFE",
        format!(
            "request=generate_bridge_artifacts duration_ms={} result=PASS",
            generate_elapsed
        ),
    ));

    let expected: ExpectedRuntimeActions = serde_json::from_str(
        &fs::read_to_string(expected_actions_path).expect("read expected actions"),
    )
    .expect("parse expected actions");
    let ExpectedRuntimeActions {
        queries: expected_queries_raw,
        mutations: expected_mutations_raw,
    } = expected;

    let load_started = Instant::now();
    let payload = load_runtime_contract(&out);
    let (queries, mutations) = load_runtime_visible_actions(&payload);
    let load_elapsed = load_started.elapsed().as_millis();
    let runtime_contract_path = out.join("unibo-api-contract.json");

    let expected_queries: BTreeSet<String> = expected_queries_raw.into_iter().collect();
    assert_eq!(
        queries, expected_queries,
        "query contract should be runtime-visible"
    );
    let expected_mutations: BTreeSet<String> = expected_mutations_raw.into_iter().collect();
    assert_eq!(
        mutations, expected_mutations,
        "mutation contract should be runtime-visible"
    );

    if let Some(runtime_probe) = runtime_probe_or_skip("integration-runtime-smoke") {
        let query_started = Instant::now();
        let query_output = runtime_probe
            .execute_action(
                &runtime_contract_path,
                "query",
                "account.list_users:list_users",
                &json!({"page": 1}),
            )
            .expect("query action should execute via runtime probe");
        assert!(
            query_output.get("users").is_some_and(Value::is_array),
            "query output should include users array"
        );
        let query_elapsed = query_started.elapsed().as_millis();
        println!(
            "[runtime-smoke] request=runtime_query_execute duration_ms={} result=PASS summary=action=account.list_users:list_users",
            query_elapsed
        );
        findings.push(GateFinding::new(
            "SAFE:runtime_smoke:runtime_query_execute",
            "SAFE",
            format!(
                "request=runtime_query_execute duration_ms={} result=PASS action=account.list_users:list_users",
                query_elapsed
            ),
        ));

        let mutation_started = Instant::now();
        let mutation_output = runtime_probe
            .execute_action(
                &runtime_contract_path,
                "mutation",
                "billing.create_invoice:create_invoice",
                &json!({"order_id": "11111111-1111-1111-1111-111111111111"}),
            )
            .expect("mutation action should execute via runtime probe");
        assert!(
            mutation_output
                .get("invoice_id")
                .is_some_and(Value::is_string),
            "mutation output should include invoice_id"
        );
        let mutation_elapsed = mutation_started.elapsed().as_millis();
        println!(
            "[runtime-smoke] request=runtime_mutation_execute duration_ms={} result=PASS summary=action=billing.create_invoice:create_invoice",
            mutation_elapsed
        );
        findings.push(GateFinding::new(
            "SAFE:runtime_smoke:runtime_mutation_execute",
            "SAFE",
            format!(
                "request=runtime_mutation_execute duration_ms={} result=PASS action=billing.create_invoice:create_invoice",
                mutation_elapsed
            ),
        ));
    } else {
        findings.push(GateFinding::new(
            "DANGEROUS:runtime_smoke:runtime_probe_unavailable",
            "DANGEROUS",
            "runtime probe unavailable, skip real query/mutation execution in warn mode",
        ));
    }

    println!(
        "[runtime-smoke] request=load_runtime_contract duration_ms={} result=PASS summary=query+mutation_executed",
        load_elapsed
    );

    let mut output_files: Vec<String> = fs::read_dir(&out)
        .expect("read output dir")
        .filter_map(Result::ok)
        .map(|entry| entry.file_name().to_string_lossy().to_string())
        .collect();
    output_files.sort();
    assert_eq!(
        output_files,
        vec![
            "api-contract.json".to_string(),
            "unibo-api-contract.json".to_string(),
            "unibo-runtime-bridge.yaml".to_string(),
        ],
        "bridge smoke should only output contract + runtime bridge artifacts"
    );

    assert!(
        !output_files
            .iter()
            .any(|name| name.contains("controller") || name.contains("resolver")),
        "bridge smoke should not generate BCC custom runtime/controller/resolver files"
    );

    let outcome = gate_common::evaluate_gate(
        "integration-bcc-unibo-runtime",
        gate_common::GateConfig::from_env(),
        findings,
        &[],
    );
    assert!(outcome.passed, "integration gate should pass");
    assert!(outcome.report_path.exists(), "report should be written");

    let _ = fs::remove_dir_all(&out);
}

#[test]
fn bcc_unibo_runtime_bridge_should_reject_null_and_malformed_inputs() {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let seed = root.join("examples/unibo-bridge-smoke/bridge.seed.yaml");
    let out = temp_dir("bcc_unibo_runtime_invalid_inputs");

    run_arch_generate(&seed, &out);
    let runtime_contract_path = out.join("unibo-api-contract.json");
    let Some(runtime_probe) = runtime_probe_or_skip("integration-runtime-invalid-inputs") else {
        let _ = fs::remove_dir_all(&out);
        return;
    };

    let null_input = runtime_probe
        .execute_action(
            &runtime_contract_path,
            "query",
            "account.list_users:list_users",
            &Value::Null,
        )
        .expect_err("null input should be rejected");

    let malformed_type = runtime_probe
        .execute_action(
            &runtime_contract_path,
            "query",
            "account.list_users:list_users",
            &json!({"page": "not-integer"}),
        )
        .expect_err("malformed input type should be rejected");

    let missing_required = runtime_probe
        .execute_action(
            &runtime_contract_path,
            "mutation",
            "billing.create_invoice:create_invoice",
            &json!({}),
        )
        .expect_err("missing required field should be rejected");

    let unknown_action = runtime_probe
        .execute_action(
            &runtime_contract_path,
            "mutation",
            "billing.invoice:unknown_action",
            &json!({"id": "11111111-1111-1111-1111-111111111111"}),
        )
        .expect_err("unknown action should be rejected");
    assert!(
        !null_input.is_empty()
            && !malformed_type.is_empty()
            && !missing_required.is_empty()
            && !unknown_action.is_empty(),
        "runtime should return explicit error message for invalid inputs"
    );

    let _ = fs::remove_dir_all(&out);
}

#[test]
fn bcc_unibo_runtime_bridge_should_handle_same_resource_concurrent_writes() {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let seed = root.join("examples/unibo-bridge-smoke/bridge.seed.yaml");
    let out = temp_dir("bcc_unibo_runtime_concurrent_writes");

    run_arch_generate(&seed, &out);
    let runtime_contract_path = out.join("unibo-api-contract.json");
    let Some(runtime_probe) = runtime_probe_or_skip("integration-runtime-concurrent-writes") else {
        let _ = fs::remove_dir_all(&out);
        return;
    };
    let invoice_id = "33333333-3333-3333-3333-333333333333";

    let mut handles = Vec::new();
    for idx in 0..8 {
        let runtime_probe = runtime_probe.clone();
        let runtime_contract_path = runtime_contract_path.clone();
        handles.push(thread::spawn(move || {
            runtime_probe.execute_action(
                &runtime_contract_path,
                "mutation",
                "billing.invoice:update",
                &json!({
                    "id": invoice_id,
                    "amount": idx as f64 + 1.0,
                    "buyer_id": "11111111-1111-1111-1111-111111111111"
                }),
            )
        }));
    }

    for handle in handles {
        let output = handle
            .join()
            .expect("join concurrent writer")
            .expect("concurrent mutation should succeed");
        assert!(
            output.get("amount").is_some_and(Value::is_number),
            "mutation output should keep typed amount"
        );
    }

    let read_back = runtime_probe
        .execute_action(
            &runtime_contract_path,
            "query",
            "billing.invoice:read",
            &json!({ "id": invoice_id }),
        )
        .expect("read after concurrent mutation should succeed via runtime probe");
    assert!(
        read_back.get("amount").is_some_and(Value::is_number),
        "read output should include amount after concurrent writes"
    );
    assert!(
        read_back.get("buyer_id").is_some_and(Value::is_string),
        "read output should include buyer_id after concurrent writes"
    );

    let _ = fs::remove_dir_all(&out);
}
