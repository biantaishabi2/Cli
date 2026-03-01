use serde_json::Value;
use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, ExitStatus};
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

fn run_arch_generate(seed: &Path, emit: &str, output: &Path) -> ExitStatus {
    Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "generate",
            "--seed-file",
            &seed.to_string_lossy(),
            "--emit",
            emit,
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run arch generate")
}

fn sample_seed_with_contracts() -> &'static str {
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
"#
}

fn sample_seed_without_contracts() -> &'static str {
    r#"version: v1
modules:
  - module_id: billing
    precedence: 1
    path_rules:
      include: ["src/billing/**"]
relations_expected: []
"#
}

fn collect_dir_snapshot(root: &Path) -> BTreeMap<String, String> {
    fn visit(base: &Path, dir: &Path, out: &mut BTreeMap<String, String>) {
        let mut entries: Vec<_> = fs::read_dir(dir)
            .expect("read_dir")
            .map(|e| e.expect("dir entry"))
            .collect();
        entries.sort_by_key(|e| e.path());
        for entry in entries {
            let path = entry.path();
            if path.is_dir() {
                visit(base, &path, out);
                continue;
            }
            let rel = path
                .strip_prefix(base)
                .expect("strip prefix")
                .to_string_lossy()
                .to_string();
            let content = fs::read_to_string(&path).expect("read file content");
            out.insert(rel, content);
        }
    }

    let mut snapshot = BTreeMap::new();
    if root.exists() {
        visit(root, root, &mut snapshot);
    }
    snapshot
}

#[test]
fn arch_generate_emit_code_keeps_existing_output() {
    let root = temp_dir("bcc_arch_generate_code");
    let seed = root.join("seed.yaml");
    let output = root.join("code-out");
    write(&seed, sample_seed_with_contracts());

    let status = run_arch_generate(&seed, "code", &output);
    assert!(status.success(), "emit code should succeed");
    assert!(output.join("generate-commands.sh").exists());
    assert!(output.join("billing.ex").exists());

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn arch_generate_emit_api_contract_only_outputs_contract_file() {
    let root = temp_dir("bcc_arch_generate_api_contract");
    let seed = root.join("seed.yaml");
    let output = root.join("api-contract-out");
    write(&seed, sample_seed_with_contracts());

    let status = run_arch_generate(&seed, "api-contract", &output);
    assert!(status.success(), "emit api-contract should succeed");

    let contract_path = output.join("api-contract.json");
    assert!(contract_path.exists(), "api-contract.json should exist");
    assert!(!output.join("generate-commands.sh").exists());
    assert!(!output.join("billing.ex").exists());

    let payload: Value =
        serde_json::from_str(&fs::read_to_string(&contract_path).expect("read contract file"))
            .expect("parse contract json");
    assert_eq!(
        payload.get("contract_schema_version"),
        Some(&Value::String("1.0.0".to_string()))
    );
    assert_eq!(
        payload.get("seed_version"),
        Some(&Value::String("v1".to_string()))
    );
    assert!(
        payload
            .get("generated_at")
            .and_then(|v| v.as_str())
            .map(|v| v.contains('T'))
            .unwrap_or(false),
        "generated_at should be RFC3339"
    );
    let contracts = payload
        .get("contracts")
        .and_then(|v| v.as_array())
        .expect("contracts array");
    assert_eq!(contracts.len(), 2);
    assert!(contracts
        .iter()
        .all(|item| item.get("module_id") == Some(&Value::String("billing".to_string()))));

    let files = collect_dir_snapshot(&output);
    assert_eq!(
        files.len(),
        1,
        "api-contract output should contain only one file"
    );
    assert!(!files.keys().any(|name| {
        name.contains("runtime") || name.contains("controller") || name.contains("resolver")
    }));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn arch_generate_emit_api_contract_fails_without_contracts() {
    let root = temp_dir("bcc_arch_generate_api_contract_missing");
    let seed = root.join("seed.yaml");
    let output = root.join("api-contract-out");
    write(&seed, sample_seed_without_contracts());

    let status = run_arch_generate(&seed, "api-contract", &output);
    assert!(
        !status.success(),
        "emit api-contract should fail without contracts"
    );
    assert_eq!(status.code(), Some(1));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn arch_generate_emit_all_is_equivalent_to_emit_code() {
    let root = temp_dir("bcc_arch_generate_all_equivalent");
    let seed = root.join("seed.yaml");
    let code_output = root.join("code-out");
    let all_output = root.join("all-out");
    write(&seed, sample_seed_with_contracts());

    let code_status = run_arch_generate(&seed, "code", &code_output);
    let all_status = run_arch_generate(&seed, "all", &all_output);
    assert!(code_status.success(), "emit code should succeed");
    assert!(all_status.success(), "emit all should succeed");

    let code_snapshot = collect_dir_snapshot(&code_output);
    let all_snapshot = collect_dir_snapshot(&all_output);
    assert_eq!(code_snapshot, all_snapshot);

    let _ = fs::remove_dir_all(&root);
}
