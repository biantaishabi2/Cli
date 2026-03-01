#[path = "gate_common.rs"]
mod gate_common;

use gate_common::GateFinding;
use serde::Deserialize;
use serde_json::{json, Value};
use std::collections::{BTreeMap, BTreeSet};
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

fn parse_type_token(raw: &str) -> (&str, bool) {
    let trimmed = raw.trim();
    if let Some(base) = trimmed.strip_suffix('?') {
        return (base.trim(), false);
    }
    if let Some(base) = trimmed.strip_suffix('!') {
        return (base.trim(), true);
    }
    (trimmed, true)
}

fn is_uuid_like(value: &str) -> bool {
    if value.len() != 36 {
        return false;
    }
    for (idx, ch) in value.chars().enumerate() {
        match idx {
            8 | 13 | 18 | 23 => {
                if ch != '-' {
                    return false;
                }
            }
            _ => {
                if !ch.is_ascii_hexdigit() {
                    return false;
                }
            }
        }
    }
    true
}

fn validate_typed_value(type_token: &str, value: &Value) -> bool {
    let (base, _) = parse_type_token(type_token);
    if value.is_null() {
        return false;
    }
    if let Some(inner) = base
        .strip_prefix("enum[")
        .and_then(|token| token.strip_suffix(']'))
    {
        let Some(actual) = value.as_str() else {
            return false;
        };
        return inner.split('|').any(|item| item.trim() == actual);
    }

    match base {
        "uuid" => value.as_str().is_some_and(is_uuid_like),
        "integer" => value.as_i64().is_some() || value.as_u64().is_some(),
        "decimal" => value.is_number(),
        "string" => value.is_string(),
        "boolean" => value.is_boolean(),
        "array" => value.is_array(),
        _ => true,
    }
}

fn sample_value_for_type(type_token: &str) -> Value {
    let (base, _) = parse_type_token(type_token);
    if let Some(inner) = base
        .strip_prefix("enum[")
        .and_then(|token| token.strip_suffix(']'))
    {
        if let Some(first) = inner.split('|').next() {
            return Value::String(first.trim().to_string());
        }
        return Value::Null;
    }

    match base {
        "uuid" => Value::String("11111111-1111-1111-1111-111111111111".to_string()),
        "integer" => Value::from(1),
        "decimal" => Value::from(12.5),
        "string" => Value::String("ok".to_string()),
        "boolean" => Value::Bool(true),
        "array" => Value::Array(vec![Value::String("ok".to_string())]),
        _ => Value::String("ok".to_string()),
    }
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

fn execute_runtime_action(
    payload: &RuntimeConsumableContractDocument,
    action_key: &str,
    input: &Value,
) -> Result<Value, String> {
    let Some((contract_key, action_name)) = action_key.split_once(':') else {
        return Err(format!("invalid action key: {}", action_key));
    };
    let Some(input_map) = input.as_object() else {
        return Err(format!("action {} expects object input", action_key));
    };

    let contract = payload
        .contracts
        .iter()
        .find(|item| item.contract_key == contract_key)
        .ok_or_else(|| format!("contract not found: {}", contract_key))?;
    let action = contract
        .actions
        .iter()
        .find(|item| item.action == action_name)
        .ok_or_else(|| format!("action not found: {}", action_key))?;

    for (field, ty) in &action.input {
        let (_, required) = parse_type_token(ty);
        match input_map.get(field) {
            Some(value) => {
                if !validate_typed_value(ty, value) {
                    return Err(format!(
                        "invalid input type: {}.{} expects {}",
                        action_key, field, ty
                    ));
                }
            }
            None if required => {
                return Err(format!(
                    "missing required input: {}.{} expects {}",
                    action_key, field, ty
                ));
            }
            None => {}
        }
    }

    for key in input_map.keys() {
        if !action.input.contains_key(key) {
            return Err(format!("unexpected input field: {}.{}", action_key, key));
        }
    }

    // 模拟 UniBO runtime 的最小执行路径：按契约输出结构返回可序列化结果。
    let mut output = serde_json::Map::new();
    for (field, ty) in &action.output {
        output.insert(field.clone(), sample_value_for_type(ty));
    }
    Ok(Value::Object(output))
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

    let query_started = Instant::now();
    let query_output = execute_runtime_action(
        &payload,
        "account.list_users:list_users",
        &json!({"page": 1}),
    )
    .expect("query action should execute");
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
    let mutation_output = execute_runtime_action(
        &payload,
        "billing.create_invoice:create_invoice",
        &json!({"order_id": "11111111-1111-1111-1111-111111111111"}),
    )
    .expect("mutation action should execute");
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
    let payload = load_runtime_contract(&out);

    let null_input =
        execute_runtime_action(&payload, "account.list_users:list_users", &Value::Null)
            .expect_err("null input should be rejected");
    assert!(
        null_input.contains("expects object input"),
        "null input error should mention object input"
    );

    let malformed_type = execute_runtime_action(
        &payload,
        "account.list_users:list_users",
        &json!({"page": "not-integer"}),
    )
    .expect_err("malformed input type should be rejected");
    assert!(
        malformed_type.contains("invalid input type"),
        "type error should be reported"
    );

    let missing_required = execute_runtime_action(
        &payload,
        "billing.create_invoice:create_invoice",
        &json!({}),
    )
    .expect_err("missing required field should be rejected");
    assert!(
        missing_required.contains("missing required input"),
        "missing input error should be reported"
    );

    let unknown_action = execute_runtime_action(
        &payload,
        "billing.invoice:unknown_action",
        &json!({"id": "11111111-1111-1111-1111-111111111111"}),
    )
    .expect_err("unknown action should be rejected");
    assert!(
        unknown_action.contains("action not found"),
        "unknown action should fail fast"
    );

    let _ = fs::remove_dir_all(&out);
}
