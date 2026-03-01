#[path = "gate_common.rs"]
mod gate_common;

use gate_common::GateFinding;
use serde::Deserialize;
use serde_json::Value;
use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

const WARN_THRESHOLD: i64 = 30;
const STRICT_THRESHOLD: i64 = 50;
const MAX_DEPTH_HARD_LIMIT: i64 = 8;

#[derive(Debug, Deserialize, Clone)]
struct EvolutionFixture {
    scenario: String,
    level: String,
    rules: i64,
    max_depth: i64,
    branches: i64,
    #[serde(default)]
    description: String,
    seed_file: String,
    query_action: String,
    query_input: Value,
    mutation_action: String,
    mutation_input: Value,
}

#[derive(Debug, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
struct RuntimeAction {
    action: String,
    graphql_kind: String,
    #[serde(default)]
    input: BTreeMap<String, String>,
    #[serde(default)]
    output: BTreeMap<String, String>,
}

#[derive(Debug, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
struct RuntimeContract {
    contract_key: String,
    actions: Vec<RuntimeAction>,
}

#[derive(Debug, Deserialize, Clone)]
#[serde(rename_all = "camelCase")]
struct RuntimeContractDocument {
    contracts: Vec<RuntimeContract>,
}

fn complexity(case: &EvolutionFixture) -> i64 {
    case.rules + case.max_depth * 3 + case.branches * 2
}

fn fixture_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/evolution/l0_l3")
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
        .expect("run arch generate for l0-l3 evolution");
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

fn load_fixtures() -> Vec<EvolutionFixture> {
    let mut files: Vec<PathBuf> = fs::read_dir(fixture_dir())
        .expect("read evolution fixture dir")
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| {
            path.extension().and_then(|ext| ext.to_str()) == Some("yaml")
                && path
                    .file_name()
                    .and_then(|name| name.to_str())
                    .is_some_and(|name| !name.ends_with(".seed.yaml"))
        })
        .collect();
    files.sort();

    files
        .into_iter()
        .map(|path| {
            let raw = fs::read_to_string(&path).expect("read evolution fixture file");
            serde_yaml::from_str(&raw).expect("parse evolution fixture yaml")
        })
        .collect()
}

fn load_runtime_contract(path: &Path) -> RuntimeContractDocument {
    let raw = fs::read_to_string(path).expect("read runtime contract");
    serde_json::from_str(&raw).expect("parse runtime contract")
}

fn collect_action_keys(document: &RuntimeContractDocument) -> BTreeSet<String> {
    let mut keys = BTreeSet::new();
    for contract in &document.contracts {
        for action in &contract.actions {
            keys.insert(format!("{}:{}", contract.contract_key, action.action));
        }
    }
    keys
}

fn execute_runtime_action(
    payload: &RuntimeContractDocument,
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

    // 按契约构造返回，确保链路测试真实覆盖 query/mutation 调用而非仅检查 action 列表。
    let mut output = serde_json::Map::new();
    for (field, ty) in &action.output {
        output.insert(field.clone(), sample_value_for_type(ty));
    }
    Ok(Value::Object(output))
}

fn build_complexity_findings(cases: &[EvolutionFixture]) -> Vec<GateFinding> {
    let mut findings = Vec::new();

    for case in cases {
        let score = complexity(case);
        let message = format!(
            "scenario={} level={} complexity={} rules={} depth={} branches={} detail={}",
            case.scenario,
            case.level,
            score,
            case.rules,
            case.max_depth,
            case.branches,
            case.description
        );

        if case.max_depth < 0 {
            findings.push(GateFinding::new(
                format!("BREAKING:invalid_depth_negative:{}", case.scenario),
                "BREAKING",
                format!("{} depth must be >= 0", message),
            ));
            continue;
        }

        if case.max_depth > MAX_DEPTH_HARD_LIMIT {
            findings.push(GateFinding::new(
                format!("BREAKING:depth_exceeds_limit:{}", case.scenario),
                "BREAKING",
                format!(
                    "{} depth {} exceeds limit {}",
                    message, case.max_depth, MAX_DEPTH_HARD_LIMIT
                ),
            ));
            continue;
        }

        if score > STRICT_THRESHOLD {
            findings.push(GateFinding::new(
                format!("BREAKING:complexity_exceeds_strict:{}", case.scenario),
                "BREAKING",
                message,
            ));
            continue;
        }

        if score > WARN_THRESHOLD {
            findings.push(GateFinding::new(
                format!("DANGEROUS:complexity_exceeds_warn:{}", case.scenario),
                "DANGEROUS",
                message,
            ));
            continue;
        }

        findings.push(GateFinding::new(
            format!("SAFE:complexity_within_gate:{}", case.scenario),
            "SAFE",
            message,
        ));
    }

    findings
}

#[test]
fn l0_l3_min_chain_should_increase_complexity_step_by_step() {
    let cases = load_fixtures();
    assert_eq!(cases.len(), 4, "should load L0~L3 fixtures");

    let levels: Vec<&str> = cases.iter().map(|item| item.level.as_str()).collect();
    assert_eq!(levels, vec!["L0", "L1", "L2", "L3"]);

    let scores: Vec<i64> = cases.iter().map(complexity).collect();
    for window in scores.windows(2) {
        assert!(
            window[1] > window[0],
            "complexity should be monotonic increasing: {:?}",
            scores
        );
    }
}

#[test]
fn l0_l3_complexity_gate_should_follow_env_mode() {
    let cases = load_fixtures();
    let findings = build_complexity_findings(&cases);
    let config = gate_common::GateConfig::from_env();

    let outcome = gate_common::evaluate_gate("regression-l0-l3-evolution", config, findings, &[]);

    match config.mode {
        gate_common::GateMode::Warn => {
            assert!(outcome.passed, "warn mode should pass with warnings");
        }
        gate_common::GateMode::Strict => {
            assert!(
                !outcome.passed,
                "strict mode should block over-threshold complexity"
            );
            assert!(outcome
                .violations
                .iter()
                .any(|item| item.id == "BREAKING:complexity_exceeds_strict:l3_use_case"));
        }
    }
    assert!(outcome.report_path.exists(), "report should be written");
}

#[test]
fn l0_l3_runtime_chain_should_cover_crud_rules_use_case() {
    let cases = load_fixtures();
    let base = fixture_dir();
    let mut previous_actions = BTreeSet::new();

    for case in cases {
        let seed = base.join(&case.seed_file);
        assert!(seed.exists(), "seed should exist for {}", case.scenario);

        let out = temp_dir(&format!("bcc_l0_l3_chain_{}", case.scenario));
        run_arch_generate(&seed, &out);
        let runtime_contract = load_runtime_contract(&out.join("unibo-api-contract.json"));
        let current_actions = collect_action_keys(&runtime_contract);

        assert!(
            current_actions.contains(&case.query_action),
            "scenario {} should expose query {}",
            case.scenario,
            case.query_action
        );
        assert!(
            current_actions.contains(&case.mutation_action),
            "scenario {} should expose mutation {}",
            case.scenario,
            case.mutation_action
        );

        if !previous_actions.is_empty() {
            assert!(
                current_actions.is_superset(&previous_actions),
                "scenario {} should keep prior chain actions",
                case.scenario
            );
        }

        let query_output =
            execute_runtime_action(&runtime_contract, &case.query_action, &case.query_input)
                .unwrap_or_else(|err| panic!("scenario {} query failed: {}", case.scenario, err));
        assert!(
            query_output.is_object(),
            "scenario {} query should return object output",
            case.scenario
        );

        let mutation_output = execute_runtime_action(
            &runtime_contract,
            &case.mutation_action,
            &case.mutation_input,
        )
        .unwrap_or_else(|err| panic!("scenario {} mutation failed: {}", case.scenario, err));
        assert!(
            mutation_output.is_object(),
            "scenario {} mutation should return object output",
            case.scenario
        );

        previous_actions = current_actions;
        let _ = fs::remove_dir_all(&out);
    }
}

#[test]
fn l0_l3_depth_boundary_should_flag_invalid_depth_values() {
    let invalid_cases = vec![
        EvolutionFixture {
            scenario: "negative_depth".to_string(),
            level: "L2".to_string(),
            rules: 10,
            max_depth: -1,
            branches: 3,
            description: "invalid negative depth".to_string(),
            seed_file: "seeds/l2_rules_combo.seed.yaml".to_string(),
            query_action: "risk.validate_invoice:validate_invoice".to_string(),
            query_input: serde_json::json!({"invoice_id": "11111111-1111-1111-1111-111111111111"}),
            mutation_action: "billing.approve_invoice:approve_invoice".to_string(),
            mutation_input: serde_json::json!({"invoice_id": "11111111-1111-1111-1111-111111111111"}),
        },
        EvolutionFixture {
            scenario: "too_deep".to_string(),
            level: "L3".to_string(),
            rules: 16,
            max_depth: 12,
            branches: 5,
            description: "depth over hard limit".to_string(),
            seed_file: "seeds/l3_use_case.seed.yaml".to_string(),
            query_action: "workflow.invoice_journey:invoice_journey".to_string(),
            query_input: serde_json::json!({"invoice_id": "11111111-1111-1111-1111-111111111111"}),
            mutation_action: "workflow.settle_invoice:settle_invoice".to_string(),
            mutation_input: serde_json::json!({"invoice_id": "11111111-1111-1111-1111-111111111111"}),
        },
    ];

    let findings = build_complexity_findings(&invalid_cases);
    assert!(findings
        .iter()
        .any(|item| item.id == "BREAKING:invalid_depth_negative:negative_depth"));
    assert!(findings
        .iter()
        .any(|item| item.id == "BREAKING:depth_exceeds_limit:too_deep"));
}
