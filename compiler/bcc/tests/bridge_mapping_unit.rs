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

fn write(path: &Path, content: &str) {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).expect("create parent");
    }
    fs::write(path, content).expect("write file");
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

fn read_contract(output: &Path) -> Value {
    let path = output.join("unibo-api-contract.json");
    let raw = fs::read_to_string(&path).expect("read unibo-api-contract.json");
    serde_json::from_str(&raw).expect("parse unibo api contract")
}

#[test]
fn bridge_mapping_query_command_crud_should_be_correct_and_sorted() {
    let root = temp_dir("bcc_bridge_mapping_kind");
    let seed = root.join("seed.yaml");
    let output = root.join("out");

    write(
        &seed,
        r#"version: v1
modules:
  - module_id: billing
    precedence: 1
    path_rules:
      include: ["src/billing/**"]
  - module_id: account
    precedence: 2
    path_rules:
      include: ["src/account/**"]
relations_expected: []
boundaries:
  - module_id: billing
    contracts:
      - name: invoice
        kind: crud
        fields:
          amount: decimal
          buyer_id: uuid
      - name: create_invoice
        kind: command
        input:
          order_id: uuid
        output:
          invoice_id: uuid
  - module_id: account
    contracts:
      - name: list_users
        kind: query
        input:
          page: integer
        output:
          users: array
"#,
    );

    let status = run_arch_generate(&seed, &output, false, "error-on-conflict");
    assert!(status.success(), "emit api-contract should succeed");

    let payload = read_contract(&output);
    assert_eq!(
        payload.get("bridgeVersion"),
        Some(&Value::String("1.0.0".to_string()))
    );

    let contracts = payload
        .get("contracts")
        .and_then(Value::as_array)
        .expect("contracts array");
    let keys: Vec<String> = contracts
        .iter()
        .filter_map(|v| {
            v.get("contractKey")
                .and_then(Value::as_str)
                .map(str::to_string)
        })
        .collect();
    assert_eq!(
        keys,
        vec![
            "account.list_users".to_string(),
            "billing.create_invoice".to_string(),
            "billing.invoice".to_string()
        ]
    );

    let query = contracts
        .iter()
        .find(|item| {
            item.get("contractKey") == Some(&Value::String("account.list_users".to_string()))
        })
        .expect("query contract");
    assert_eq!(
        query.get("graphqlKind"),
        Some(&Value::String("query".to_string()))
    );
    assert_eq!(
        query.pointer("/actions/0/graphqlKind"),
        Some(&Value::String("query".to_string()))
    );

    let command = contracts
        .iter()
        .find(|item| {
            item.get("contractKey") == Some(&Value::String("billing.create_invoice".to_string()))
        })
        .expect("command contract");
    assert_eq!(
        command.get("graphqlKind"),
        Some(&Value::String("mutation".to_string()))
    );
    assert_eq!(
        command.pointer("/actions/0/actionKey"),
        Some(&Value::String("create_invoice".to_string()))
    );

    let crud = contracts
        .iter()
        .find(|item| item.get("contractKey") == Some(&Value::String("billing.invoice".to_string())))
        .expect("crud contract");
    assert_eq!(
        crud.get("graphqlKind"),
        Some(&Value::String("mixed".to_string()))
    );
    let action_keys: Vec<String> = crud
        .get("actions")
        .and_then(Value::as_array)
        .expect("crud actions")
        .iter()
        .filter_map(|item| {
            item.get("actionKey")
                .and_then(Value::as_str)
                .map(str::to_string)
        })
        .collect();
    assert_eq!(
        action_keys,
        vec![
            "create".to_string(),
            "delete".to_string(),
            "list".to_string(),
            "read".to_string(),
            "update".to_string()
        ]
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn bridge_contract_name_conflict_should_fail_by_default() {
    let root = temp_dir("bcc_bridge_conflict_error");
    let seed = root.join("seed.yaml");
    let output = root.join("out");

    write(
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
      - name: Create Invoice
        kind: command
        input:
          order_id: uuid
      - name: create_invoice
        kind: command
        input:
          order_id: uuid
"#,
    );

    let status = run_arch_generate(&seed, &output, false, "error-on-conflict");
    assert!(!status.success(), "default conflict strategy should fail");
    assert_eq!(status.code(), Some(12));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn bridge_contract_name_conflict_dedupe_should_keep_latest() {
    let root = temp_dir("bcc_bridge_conflict_dedupe");
    let seed = root.join("seed.yaml");
    let output = root.join("out");

    write(
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
      - name: Create Invoice
        kind: command
        input:
          legacy_order_id: uuid
      - name: create_invoice
        kind: command
        input:
          order_id: uuid
"#,
    );

    let status = run_arch_generate(&seed, &output, false, "dedupe");
    assert!(status.success(), "dedupe strategy should succeed");

    let payload = read_contract(&output);
    let contracts = payload
        .get("contracts")
        .and_then(Value::as_array)
        .expect("contracts array");
    assert_eq!(contracts.len(), 1, "conflict contracts should be deduped");
    assert_eq!(
        contracts[0].get("contractKey"),
        Some(&Value::String("billing.create_invoice".to_string()))
    );
    assert_eq!(
        contracts[0].get("name"),
        Some(&Value::String("create_invoice".to_string()))
    );
    assert_eq!(
        contracts[0].pointer("/actions/0/input/order_id"),
        Some(&Value::String("uuid".to_string()))
    );

    let _ = fs::remove_dir_all(&root);
}
