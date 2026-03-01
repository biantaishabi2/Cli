#[path = "gate_common.rs"]
mod gate_common;

use gate_common::{GateConfig, GateFinding, GateMode};
use serde::Deserialize;
use serde_json::Value;
use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::thread;
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ManifestDocument {
    #[serde(default)]
    contracts: Vec<ManifestContract>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ManifestContract {
    contract_key: String,
    #[serde(default)]
    graphql_kind: String,
    #[serde(default)]
    fields: BTreeMap<String, String>,
    #[serde(default)]
    actions: Vec<ManifestAction>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ManifestAction {
    #[serde(default)]
    action_key: String,
    #[serde(default)]
    action: String,
    #[serde(default)]
    graphql_kind: String,
    #[serde(default)]
    input: BTreeMap<String, String>,
    #[serde(default)]
    output: BTreeMap<String, String>,
}

#[derive(Debug, Clone, Copy)]
enum FieldRole {
    ContractField,
    Input,
    Output,
}

#[derive(Debug)]
struct TypeShape {
    base: String,
    required: bool,
    enum_values: Option<BTreeSet<String>>,
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

fn parse_manifest_from_path(path: &Path) -> ManifestDocument {
    let raw = fs::read_to_string(path).expect("read manifest");
    parse_manifest_from_raw(&raw)
}

fn parse_manifest_from_raw(raw: &str) -> ManifestDocument {
    let mut value: Value = serde_json::from_str(raw).expect("parse manifest json");

    // 归一化 generatedAt，避免时间戳噪声影响 drift 判定。
    if let Some(obj) = value.as_object_mut() {
        if obj.contains_key("generatedAt") {
            obj.insert(
                "generatedAt".to_string(),
                Value::String("<normalized-generated-at>".to_string()),
            );
        }
    }

    serde_json::from_value(value).expect("deserialize manifest")
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
        .expect("run arch generate for manifest drift");
    assert!(status.success(), "arch generate should succeed");
}

fn action_key(action: &ManifestAction) -> String {
    if !action.action_key.is_empty() {
        action.action_key.clone()
    } else {
        action.action.clone()
    }
}

fn parse_type_shape(raw: &str) -> TypeShape {
    let mut token = raw.trim().to_string();
    let required = if token.ends_with('!') {
        token.pop();
        true
    } else if token.ends_with('?') {
        token.pop();
        false
    } else {
        false
    };

    let base = token.trim().to_string();
    let enum_values = base
        .strip_prefix("enum[")
        .and_then(|inner| inner.strip_suffix(']'))
        .map(|inner| {
            inner
                .split('|')
                .map(str::trim)
                .filter(|item| !item.is_empty())
                .map(str::to_string)
                .collect::<BTreeSet<_>>()
        });

    TypeShape {
        base,
        required,
        enum_values,
    }
}

fn diff_type_map(
    findings: &mut Vec<GateFinding>,
    scope_prefix: &str,
    role: FieldRole,
    baseline: &BTreeMap<String, String>,
    current: &BTreeMap<String, String>,
) {
    for (field, old_ty) in baseline {
        let scope = format!("{}.{}", scope_prefix, field);
        let Some(new_ty) = current.get(field) else {
            findings.push(GateFinding::new(
                format!("BREAKING:field_removed:{}", scope),
                "BREAKING",
                format!("removed field {}", scope),
            ));
            continue;
        };

        if old_ty == new_ty {
            continue;
        }

        let old_shape = parse_type_shape(old_ty);
        let new_shape = parse_type_shape(new_ty);

        if old_shape.base != new_shape.base {
            findings.push(GateFinding::new(
                format!("BREAKING:type_changed:{}", scope),
                "BREAKING",
                format!("type changed at {} from {} to {}", scope, old_ty, new_ty),
            ));
        }

        if let (Some(old_enum), Some(new_enum)) = (&old_shape.enum_values, &new_shape.enum_values) {
            for removed in old_enum.difference(new_enum) {
                findings.push(GateFinding::new(
                    format!("DANGEROUS:enum_value_removed:{}:{}", scope, removed),
                    "DANGEROUS",
                    format!("enum value {} removed at {}", removed, scope),
                ));
            }
        }

        if !old_shape.required && new_shape.required {
            let (id_prefix, category, message) = match role {
                FieldRole::Input => (
                    "DANGEROUS:input_required",
                    "DANGEROUS",
                    format!("input field became required at {}", scope),
                ),
                FieldRole::ContractField | FieldRole::Output => (
                    "BREAKING:field_non_null",
                    "BREAKING",
                    format!("field became non-null at {}", scope),
                ),
            };
            findings.push(GateFinding::new(
                format!("{}:{}", id_prefix, scope),
                category,
                message,
            ));
        }
    }

    for (field, new_ty) in current {
        if baseline.contains_key(field) {
            continue;
        }
        let scope = format!("{}.{}", scope_prefix, field);
        let new_shape = parse_type_shape(new_ty);
        match role {
            FieldRole::Input if new_shape.required => findings.push(GateFinding::new(
                format!("DANGEROUS:required_argument_added:{}", scope),
                "DANGEROUS",
                format!("required input argument added at {}", scope),
            )),
            _ => findings.push(GateFinding::new(
                format!("SAFE:field_added:{}", scope),
                "SAFE",
                format!("field added at {}", scope),
            )),
        }
    }
}

fn diff_manifests(baseline: &ManifestDocument, current: &ManifestDocument) -> Vec<GateFinding> {
    let mut findings = Vec::new();

    let baseline_map: BTreeMap<&str, &ManifestContract> = baseline
        .contracts
        .iter()
        .map(|contract| (contract.contract_key.as_str(), contract))
        .collect();
    let current_map: BTreeMap<&str, &ManifestContract> = current
        .contracts
        .iter()
        .map(|contract| (contract.contract_key.as_str(), contract))
        .collect();

    for (contract_key, baseline_contract) in &baseline_map {
        let Some(current_contract) = current_map.get(contract_key) else {
            findings.push(GateFinding::new(
                format!("BREAKING:contract_removed:{}", contract_key),
                "BREAKING",
                format!("contract {} removed", contract_key),
            ));
            continue;
        };

        if !baseline_contract.graphql_kind.is_empty()
            && !current_contract.graphql_kind.is_empty()
            && baseline_contract.graphql_kind != current_contract.graphql_kind
        {
            findings.push(GateFinding::new(
                format!("BREAKING:contract_graphql_kind_changed:{}", contract_key),
                "BREAKING",
                format!(
                    "contract {} graphql kind changed from {} to {}",
                    contract_key, baseline_contract.graphql_kind, current_contract.graphql_kind
                ),
            ));
        }

        diff_type_map(
            &mut findings,
            &format!("{}.fields", contract_key),
            FieldRole::ContractField,
            &baseline_contract.fields,
            &current_contract.fields,
        );

        let baseline_actions: BTreeMap<String, &ManifestAction> = baseline_contract
            .actions
            .iter()
            .map(|action| (action_key(action), action))
            .collect();
        let current_actions: BTreeMap<String, &ManifestAction> = current_contract
            .actions
            .iter()
            .map(|action| (action_key(action), action))
            .collect();

        for (name, baseline_action) in &baseline_actions {
            let Some(current_action) = current_actions.get(name) else {
                findings.push(GateFinding::new(
                    format!("BREAKING:action_removed:{}:{}", contract_key, name),
                    "BREAKING",
                    format!("action {}.{} removed", contract_key, name),
                ));
                continue;
            };

            if !baseline_action.graphql_kind.is_empty()
                && !current_action.graphql_kind.is_empty()
                && baseline_action.graphql_kind != current_action.graphql_kind
            {
                findings.push(GateFinding::new(
                    format!(
                        "BREAKING:action_graphql_kind_changed:{}:{}",
                        contract_key, name
                    ),
                    "BREAKING",
                    format!(
                        "action {}.{} graphql kind changed from {} to {}",
                        contract_key,
                        name,
                        baseline_action.graphql_kind,
                        current_action.graphql_kind
                    ),
                ));
            }

            diff_type_map(
                &mut findings,
                &format!("{}.{}.input", contract_key, name),
                FieldRole::Input,
                &baseline_action.input,
                &current_action.input,
            );
            diff_type_map(
                &mut findings,
                &format!("{}.{}.output", contract_key, name),
                FieldRole::Output,
                &baseline_action.output,
                &current_action.output,
            );
        }

        for name in current_actions.keys() {
            if baseline_actions.contains_key(name) {
                continue;
            }
            findings.push(GateFinding::new(
                format!("SAFE:action_added:{}:{}", contract_key, name),
                "SAFE",
                format!("action {}.{} added", contract_key, name),
            ));
        }
    }

    for contract_key in current_map.keys() {
        if baseline_map.contains_key(contract_key) {
            continue;
        }
        findings.push(GateFinding::new(
            format!("SAFE:contract_added:{}", contract_key),
            "SAFE",
            format!("contract {} added", contract_key),
        ));
    }

    findings
}

fn bcc_root() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
}

#[test]
fn manifest_drift_on_openclaw_baseline_should_pass_strict_gate() {
    let root = bcc_root();
    let baseline_path = root.join("tests/fixtures/contracts/baseline_manifest.json");
    let whitelist_path = root.join("tests/fixtures/contracts/whitelist.toml");
    let seed_path = root.join("examples/openclaw-arch/bridge.seed.yaml");

    let out = temp_dir("bcc_manifest_drift_openclaw");
    run_arch_generate(&seed_path, &out);
    let generated_path = out.join("unibo-api-contract.json");

    let baseline = parse_manifest_from_path(&baseline_path);
    let generated = parse_manifest_from_path(&generated_path);
    let findings = diff_manifests(&baseline, &generated);
    let whitelist = gate_common::load_whitelist(&whitelist_path).expect("load whitelist");

    let outcome = gate_common::evaluate_gate(
        "contract-manifest-drift-openclaw",
        GateConfig::from_env(),
        findings,
        &whitelist,
    );

    assert!(
        outcome.passed,
        "strict mode should pass without drift violations: {:?}",
        outcome
            .violations
            .iter()
            .map(|item| (&item.id, &item.status))
            .collect::<Vec<_>>()
    );
    assert!(outcome.report_path.exists(), "report should be written");

    let _ = fs::remove_dir_all(&out);
}

#[test]
fn manifest_drift_warn_and_strict_should_follow_whitelist_policy() {
    let baseline = parse_manifest_from_raw(
        r#"{
  "contracts": [
    {
      "contractKey": "billing.invoice",
      "graphqlKind": "mutation",
      "fields": {
        "invoice_id": "uuid?",
        "status": "enum[draft|paid]"
      },
      "actions": [
        {
          "actionKey": "update",
          "action": "update",
          "graphqlKind": "mutation",
          "input": {
            "trace_id": "string?",
            "status": "enum[draft|paid]"
          },
          "output": {
            "invoice_id": "uuid?"
          }
        },
        {
          "actionKey": "archive",
          "action": "archive",
          "graphqlKind": "mutation",
          "input": {
            "id": "uuid"
          },
          "output": {
            "archived": "boolean"
          }
        }
      ]
    },
    {
      "contractKey": "billing.legacy",
      "graphqlKind": "query",
      "actions": [
        {
          "actionKey": "read",
          "action": "read",
          "graphqlKind": "query",
          "input": {},
          "output": {
            "ok": "boolean"
          }
        }
      ]
    }
  ]
}"#,
    );

    let generated = parse_manifest_from_raw(
        r#"{
  "contracts": [
    {
      "contractKey": "billing.invoice",
      "graphqlKind": "mutation",
      "fields": {
        "invoice_id": "uuid?",
        "status": "enum[draft|paid]"
      },
      "actions": [
        {
          "actionKey": "update",
          "action": "update",
          "graphqlKind": "mutation",
          "input": {
            "trace_id": "string!",
            "status": "enum[draft]"
          },
          "output": {
            "invoice_id": "uuid!"
          }
        }
      ]
    }
  ]
}"#,
    );

    let findings = diff_manifests(&baseline, &generated);

    assert!(findings
        .iter()
        .any(|item| item.id == "BREAKING:action_removed:billing.invoice:archive"));
    assert!(findings
        .iter()
        .any(|item| item.id == "BREAKING:contract_removed:billing.legacy"));
    assert!(findings
        .iter()
        .any(|item| item.id == "DANGEROUS:input_required:billing.invoice.update.input.trace_id"));
    assert!(findings
        .iter()
        .any(|item| item.id == "BREAKING:field_non_null:billing.invoice.update.output.invoice_id"));

    let whitelist =
        gate_common::load_whitelist(&bcc_root().join("tests/fixtures/contracts/whitelist.toml"))
            .expect("load whitelist");

    let warn_outcome = gate_common::evaluate_gate(
        "contract-manifest-drift-warn",
        GateConfig {
            mode: GateMode::Warn,
            fail_on_dangerous: false,
        },
        findings.clone(),
        &whitelist,
    );
    assert!(warn_outcome.passed, "warn mode should never block");

    let strict_outcome = gate_common::evaluate_gate(
        "contract-manifest-drift-strict",
        GateConfig {
            mode: GateMode::Strict,
            fail_on_dangerous: true,
        },
        findings,
        &whitelist,
    );

    assert!(
        !strict_outcome.passed,
        "strict mode should fail on unwhitelisted/expired drift"
    );
    assert!(strict_outcome
        .violations
        .iter()
        .any(|item| item.id == "BREAKING:contract_removed:billing.legacy"
            && item.status == "expired_whitelist"));
    assert!(strict_outcome.violations.iter().any(|item| item.id
        == "BREAKING:field_non_null:billing.invoice.update.output.invoice_id"
        && item.status == "unwhitelisted"));
    assert!(strict_outcome.violations.iter().any(|item| item.id
        == "DANGEROUS:input_required:billing.invoice.update.input.trace_id"
        && item.status == "unwhitelisted"));
    assert!(
        strict_outcome.report_path.exists(),
        "report should be written"
    );
}

#[test]
fn manifest_drift_parallel_evaluation_should_be_stable() {
    let baseline = parse_manifest_from_raw(
        r#"{
  "contracts": [
    {
      "contractKey": "billing.invoice",
      "graphqlKind": "mutation",
      "fields": {
        "invoice_id": "uuid?",
        "status": "enum[draft|paid]"
      },
      "actions": [
        {
          "actionKey": "update",
          "action": "update",
          "graphqlKind": "mutation",
          "input": {
            "trace_id": "string?",
            "status": "enum[draft|paid]"
          },
          "output": {
            "invoice_id": "uuid?"
          }
        }
      ]
    }
  ]
}"#,
    );
    let generated = parse_manifest_from_raw(
        r#"{
  "contracts": [
    {
      "contractKey": "billing.invoice",
      "graphqlKind": "mutation",
      "fields": {
        "invoice_id": "uuid?",
        "status": "enum[draft|paid]"
      },
      "actions": [
        {
          "actionKey": "update",
          "action": "update",
          "graphqlKind": "mutation",
          "input": {
            "trace_id": "string!",
            "status": "enum[draft]"
          },
          "output": {
            "invoice_id": "uuid!"
          }
        }
      ]
    }
  ]
}"#,
    );
    let whitelist =
        gate_common::load_whitelist(&bcc_root().join("tests/fixtures/contracts/whitelist.toml"))
            .expect("load whitelist");

    let mut handles = Vec::new();
    for idx in 0..6 {
        let baseline = baseline.clone();
        let generated = generated.clone();
        let whitelist = whitelist.clone();
        handles.push(thread::spawn(move || {
            let findings = diff_manifests(&baseline, &generated);
            let outcome = gate_common::evaluate_gate(
                &format!("contract-manifest-drift-parallel-{}", idx),
                GateConfig::from_env(),
                findings,
                &whitelist,
            );
            assert!(outcome.report_path.exists(), "report should be written");
        }));
    }

    for handle in handles {
        handle.join().expect("join manifest drift workers");
    }
}

#[test]
fn manifest_drift_parallel_shared_report_path_should_be_safe() {
    let baseline = parse_manifest_from_raw(
        r#"{
  "contracts": [
    {
      "contractKey": "billing.invoice",
      "graphqlKind": "mutation",
      "fields": {
        "invoice_id": "uuid?",
        "status": "enum[draft|paid]"
      },
      "actions": [
        {
          "actionKey": "update",
          "action": "update",
          "graphqlKind": "mutation",
          "input": {
            "trace_id": "string?",
            "status": "enum[draft|paid]"
          },
          "output": {
            "invoice_id": "uuid?"
          }
        }
      ]
    }
  ]
}"#,
    );
    let generated = parse_manifest_from_raw(
        r#"{
  "contracts": [
    {
      "contractKey": "billing.invoice",
      "graphqlKind": "mutation",
      "fields": {
        "invoice_id": "uuid?",
        "status": "enum[draft|paid]"
      },
      "actions": [
        {
          "actionKey": "update",
          "action": "update",
          "graphqlKind": "mutation",
          "input": {
            "trace_id": "string!",
            "status": "enum[draft]"
          },
          "output": {
            "invoice_id": "uuid!"
          }
        }
      ]
    }
  ]
}"#,
    );
    let whitelist =
        gate_common::load_whitelist(&bcc_root().join("tests/fixtures/contracts/whitelist.toml"))
            .expect("load whitelist");

    let gate_name = "contract-manifest-drift-parallel-shared-report";
    let mut handles = Vec::new();
    for _ in 0..6 {
        let baseline = baseline.clone();
        let generated = generated.clone();
        let whitelist = whitelist.clone();
        let gate_name = gate_name.to_string();
        handles.push(thread::spawn(move || {
            let findings = diff_manifests(&baseline, &generated);
            let outcome = gate_common::evaluate_gate(
                &gate_name,
                GateConfig::from_env(),
                findings,
                &whitelist,
            );
            assert!(outcome.report_path.exists(), "report should be written");
            outcome.report_path
        }));
    }

    let mut report_path = None;
    for handle in handles {
        let path = handle.join().expect("join shared report workers");
        report_path = Some(path);
    }

    let report_path = report_path.expect("shared report path should exist");
    let report_raw = fs::read_to_string(&report_path).expect("read shared report output");
    let report_json: Value = serde_json::from_str(&report_raw).expect("parse shared report json");
    assert_eq!(
        report_json
            .get("gate_name")
            .and_then(Value::as_str)
            .unwrap_or_default(),
        gate_name,
        "shared report should keep gate metadata"
    );
}

#[test]
fn manifest_drift_should_reject_malformed_manifest_input() {
    let malformed_json = std::panic::catch_unwind(|| parse_manifest_from_raw("{not-json"));
    assert!(malformed_json.is_err(), "invalid json should panic");

    let invalid_shape =
        std::panic::catch_unwind(|| parse_manifest_from_raw(r#"{"contracts":"invalid"}"#));
    assert!(
        invalid_shape.is_err(),
        "invalid manifest shape should panic"
    );
}
