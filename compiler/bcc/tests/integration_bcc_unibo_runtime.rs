#[path = "gate_common.rs"]
mod gate_common;

use gate_common::GateFinding;
use serde::Deserialize;
use std::collections::BTreeSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
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
    let (queries, mutations) = load_runtime_visible_actions(&out);
    let load_elapsed = load_started.elapsed().as_millis();

    let query_started = Instant::now();
    let expected_queries: BTreeSet<String> = expected_queries_raw.into_iter().collect();
    assert_eq!(
        queries, expected_queries,
        "query contract should be runtime-visible"
    );
    let query_elapsed = query_started.elapsed().as_millis();
    println!(
        "[runtime-smoke] request=query_actions_check duration_ms={} result=PASS summary=query_count={}",
        query_elapsed,
        queries.len()
    );
    findings.push(GateFinding::new(
        "SAFE:runtime_smoke:query_actions_check",
        "SAFE",
        format!(
            "request=query_actions_check duration_ms={} result=PASS summary=query_count={}",
            query_elapsed,
            queries.len()
        ),
    ));

    let mutation_started = Instant::now();
    let expected_mutations: BTreeSet<String> = expected_mutations_raw.into_iter().collect();
    assert_eq!(
        mutations, expected_mutations,
        "mutation contract should be runtime-visible"
    );
    let mutation_elapsed = mutation_started.elapsed().as_millis();
    println!(
        "[runtime-smoke] request=mutation_actions_check duration_ms={} result=PASS summary=mutation_count={}",
        mutation_elapsed,
        mutations.len()
    );
    findings.push(GateFinding::new(
        "SAFE:runtime_smoke:mutation_actions_check",
        "SAFE",
        format!(
            "request=mutation_actions_check duration_ms={} result=PASS summary=mutation_count={}",
            mutation_elapsed,
            mutations.len()
        ),
    ));

    println!(
        "[runtime-smoke] request=load_runtime_contract duration_ms={} result=PASS summary=query+mutation_loaded",
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
