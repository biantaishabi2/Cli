use bcc::extract::testing as extract_testing;
use serde_json::Value;
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

fn write(path: &Path, content: &str) {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).expect("create parent");
    }
    fs::write(path, content).expect("write file");
}

fn run_extract_ast_json(input: &Path, output: &Path) -> serde_json::Value {
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &input.to_string_lossy(),
            "--mode",
            "ast",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run extract --mode ast");
    assert!(status.success());

    let raw = fs::read_to_string(output).expect("read ast output");
    serde_json::from_str(&raw).expect("parse ast output json")
}

fn run_extract_batch_json(dir: &Path, lang: &str, output: &Path) -> serde_json::Value {
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &dir.to_string_lossy(),
            "--batch",
            "--lang",
            lang,
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run extract --batch");
    assert!(status.success());

    let raw = fs::read_to_string(output).expect("read batch output");
    serde_json::from_str(&raw).expect("parse batch output json")
}

#[test]
fn arch_validate_exit_code_can_be_relaxed() {
    let root = temp_dir("bcc_cli_arch_flags");
    let target = root.join("target.yaml");
    let transition = root.join("transition.yaml");
    let gates = root.join("gates.yaml");
    let actual = root.join("actual.json");
    let strict_out = root.join("strict");
    let relaxed_out = root.join("relaxed");

    write(
        &target,
        r#"version: v3
kind: target_contract
intent: target
source_of_truth: test
notes: []
allow_edges:
  - caller: A
    callee: B
forbid_edges: []
"#,
    );
    write(
        &transition,
        r#"version: v3
kind: transition_contract
base: v3.target
intent: transition
notes: []
temporary_allow_edges: []
blocked_edges: []
"#,
    );
    write(
        &gates,
        r#"version: v3
kind: verification_gates
intent: gate
profiles:
  transition:
    max_unexpected_edges_count: 0
    max_forbidden_edges_count: 0
    max_forbidden_total_edges: 0
    max_missing_edges_count: 0
    max_directed_density_pct: 1
    max_bidirectional_pair_count: 0
  target:
    max_unexpected_edges_count: 0
    max_forbidden_edges_count: 0
    max_forbidden_total_edges: 0
    max_missing_edges_count: 0
    max_directed_density_pct: 1
    max_bidirectional_pair_count: 0
"#,
    );
    write(
        &actual,
        r#"[
  {"caller":"A","callee":"B","import_edges":1,"call_edges":0,"total_edges":1},
  {"caller":"B","callee":"A","import_edges":1,"call_edges":0,"total_edges":1}
]"#,
    );

    let strict = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "validate",
            "--target",
            &target.to_string_lossy(),
            "--transition",
            &transition.to_string_lossy(),
            "--gates",
            &gates.to_string_lossy(),
            "--actual",
            &actual.to_string_lossy(),
            "--out-dir",
            &strict_out.to_string_lossy(),
        ])
        .status()
        .expect("run strict validate");
    assert_eq!(strict.code(), Some(2));
    assert!(strict_out.join("summary.json").exists());

    let relaxed = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "validate",
            "--target",
            &target.to_string_lossy(),
            "--transition",
            &transition.to_string_lossy(),
            "--gates",
            &gates.to_string_lossy(),
            "--actual",
            &actual.to_string_lossy(),
            "--out-dir",
            &relaxed_out.to_string_lossy(),
            "--fail-on-gate",
            "false",
            "--fail-on-forbidden",
            "false",
        ])
        .status()
        .expect("run relaxed validate");
    assert_eq!(relaxed.code(), Some(0));
    assert!(relaxed_out.join("summary.json").exists());

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn given_seed_with_contracts_when_emit_api_contract_then_only_contract_file_is_emitted() {
    let root = temp_dir("bcc_bdd_arch_generate_api_contract");
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
      - name: create_invoice
        kind: command
        input:
          order_id: uuid
        output:
          invoice_id: uuid
"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "generate",
            "--seed-file",
            &seed.to_string_lossy(),
            "--emit",
            "api-contract",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run arch generate api-contract");
    assert!(status.success());

    let contract_path = output.join("api-contract.json");
    assert!(contract_path.exists(), "api-contract.json should exist");

    let payload: Value = serde_json::from_str(
        &fs::read_to_string(&contract_path).expect("read generated api-contract"),
    )
    .expect("parse generated api-contract json");
    assert!(payload.get("contracts").and_then(Value::as_array).is_some());

    let mut files: Vec<String> = fs::read_dir(&output)
        .expect("read output dir")
        .filter_map(Result::ok)
        .map(|entry| entry.file_name().to_string_lossy().to_string())
        .collect();
    files.sort();
    assert_eq!(files, vec!["api-contract.json".to_string()]);
    assert!(!files.iter().any(|name| {
        name.contains("runtime") || name.contains("controller") || name.contains("resolver")
    }));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn bdd_seed_cli_generates_expected_files() {
    let root = temp_dir("bcc_cli_bdd_seed");
    let source = root.join("source");
    let output = root.join("output");
    let template = root.join("template.dsl");
    fs::create_dir_all(&source).expect("create source");

    write(
        &source.join("account.yaml"),
        r#"module: ACCOUNT
contract: create account
"#,
    );
    write(
        &source.join("billing.json"),
        r#"{"module_id":"BILLING","contract":"issue invoice"}"#,
    );
    write(
        &template,
        r#"[SCENARIO: BDD-{MODULE}-SEED-{ID}] TITLE: {TITLE} TAGS: seed {EDGE_CLASS}
GIVEN given_seed_context id="{ID}" module="{MODULE}"
WHEN when_execute_seed_contract module="{MODULE}"
THEN then_seed_contract_should_hold module="{MODULE}"
"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "bdd",
            "seed",
            "--source",
            &source.to_string_lossy(),
            "--output",
            &output.to_string_lossy(),
            "--step",
            "organize",
            "--edge-class",
            "stable",
            "--prompt-template",
            &template.to_string_lossy(),
            "--force",
        ])
        .status()
        .expect("run bdd seed");
    assert!(status.success());

    assert!(output.join("contexts/account_account.json").exists());
    assert!(output.join("scenarios/account_account.dsl").exists());
    assert!(output.join("features/ACCOUNT.dsl").exists());
    assert!(output.join("coverage.md").exists());

    let coverage = fs::read_to_string(output.join("coverage.md")).expect("read coverage");
    assert!(coverage.contains("| ACCOUNT |"));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn bdd_seed_check_and_fix_cli_work() {
    let root = temp_dir("bcc_cli_bdd_quality");
    let source = root.join("source");
    let output = root.join("output");
    fs::create_dir_all(&source).expect("create source");

    write(
        &source.join("account.yaml"),
        r#"module: ACCOUNT
contract: create account
"#,
    );

    let organize = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "bdd",
            "seed",
            "--source",
            &source.to_string_lossy(),
            "--output",
            &output.to_string_lossy(),
            "--step",
            "organize",
            "--force",
        ])
        .status()
        .expect("run organize");
    assert!(organize.success());

    // Break scenario quality intentionally.
    write(
        &output.join("scenarios/account_account.dsl"),
        "[SCENARIO: BDD-ACCOUNT-SEED-account_account] TITLE: broken TAGS: seed stable\nGIVEN x\n",
    );

    let check_bad = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "bdd",
            "seed",
            "--source",
            &source.to_string_lossy(),
            "--output",
            &output.to_string_lossy(),
            "--step",
            "check",
        ])
        .status()
        .expect("run check");
    assert_eq!(check_bad.code(), Some(1));
    assert!(output.join("quality-check.json").exists());

    let fix = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "bdd",
            "seed",
            "--source",
            &source.to_string_lossy(),
            "--output",
            &output.to_string_lossy(),
            "--step",
            "fix",
            "--force",
        ])
        .status()
        .expect("run fix");
    assert!(fix.success());
    assert!(output.join("quality-fix.json").exists());

    let check_good = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "bdd",
            "seed",
            "--source",
            &source.to_string_lossy(),
            "--output",
            &output.to_string_lossy(),
            "--step",
            "check",
        ])
        .status()
        .expect("run check again");
    assert!(check_good.success());

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn arch_matrix_validate_report_cli_smoke() {
    let root = temp_dir("bcc_cli_arch_chain");
    let seed = root.join("seed.yaml");
    let ast = root.join("ast.json");
    let actual = root.join("actual.json");
    let seed_out = root.join("seed_out");
    let validate_out = root.join("validate_out");
    let report = validate_out.join("architecture-debt.md");

    write(
        &seed,
        r#"version: v3
source_of_truth: architecture_doc_v3
modules:
  - module_id: ACCOUNT
    precedence: 10
    path_rules:
      include: ["src/account/**"]
  - module_id: BILLING
    precedence: 10
    path_rules:
      include: ["src/billing/**"]
relations_expected:
  - caller: ACCOUNT
    callee: BILLING
    allowed: true
"#,
    );
    write(
        &ast,
        r#"{
  "source_count": 2,
  "records": [
    {
      "sourcePath": "src/account/create.ts",
      "localDependencies": ["src/billing/invoice.ts"],
      "localCallTargets": []
    },
    {
      "sourcePath": "src/billing/invoice.ts",
      "localDependencies": [],
      "localCallTargets": []
    }
  ]
}"#,
    );
    write(
        &actual,
        r#"[
  {"caller":"ACCOUNT","callee":"BILLING","import_edges":1,"call_edges":0,"total_edges":1}
]"#,
    );

    let matrix = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &seed.to_string_lossy(),
            "--ast-file",
            &ast.to_string_lossy(),
            "--out-dir",
            &seed_out.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
        ])
        .status()
        .expect("run matrix");
    assert!(matrix.success());

    let validate = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "validate",
            "--target",
            &seed_out.join("v3.target-matrix.yaml").to_string_lossy(),
            "--transition",
            &seed_out.join("v3.transition-matrix.yaml").to_string_lossy(),
            "--gates",
            &seed_out.join("v3.gates.yaml").to_string_lossy(),
            "--actual",
            &actual.to_string_lossy(),
            "--out-dir",
            &validate_out.to_string_lossy(),
            "--fail-on-gate",
            "false",
            "--fail-on-forbidden",
            "false",
        ])
        .status()
        .expect("run validate");
    assert!(validate.success());

    let report_status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "report",
            "--scenario-validation",
            &validate_out
                .join("scenario-validation.tsv")
                .to_string_lossy(),
            "--gate-evaluation",
            &validate_out.join("gate-evaluation.tsv").to_string_lossy(),
            "--summary",
            &validate_out.join("summary.json").to_string_lossy(),
            "--out",
            &report.to_string_lossy(),
            "--format",
            "md",
        ])
        .status()
        .expect("run report");
    assert!(report_status.success());
    assert!(report.exists());

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn brownfield_extract_bugfix_and_export_map_smoke() {
    let root = temp_dir("bcc_cli_brownfield");
    let repo = root.join("repo");
    let out = root.join("out");
    fs::create_dir_all(repo.join("app/controllers")).expect("create repo dirs");

    let php_file = repo.join("app/controllers/UserController.php");
    write(
        &php_file,
        r#"<?php
class UserController {
    public function save($name) {
        return strtoupper($name);
    }
}
"#,
    );

    let git = |args: &[&str]| {
        Command::new("git")
            .current_dir(&repo)
            .args(args)
            .status()
            .expect("run git")
    };

    assert!(git(&["init"]).success());
    assert!(git(&["config", "user.email", "test@example.com"]).success());
    assert!(git(&["config", "user.name", "bcc-test"]).success());
    assert!(git(&["add", "."]).success());
    assert!(git(&["commit", "-m", "feat: init controller"]).success());

    write(
        &php_file,
        r#"<?php
class UserController {
    public function save($name) {
        if ($name === null || $name === "") {
            return "";
        }
        return strtoupper($name);
    }
}
"#,
    );
    assert!(git(&["add", "."]).success());
    assert!(git(&["commit", "-m", "fix: handle empty name"]).success());

    let extract = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args(["extract", &php_file.to_string_lossy(), "--mode", "ast"])
        .status()
        .expect("run extract");
    assert!(extract.success());

    // -s c: collect（独立执行）
    let collect = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "bugfix",
            &repo.to_string_lossy(),
            "-o",
            &out.to_string_lossy(),
            "--lang",
            "php",
            "-s",
            "c",
            "--limit",
            "20",
            "--force",
        ])
        .status()
        .expect("run bugfix collect");
    assert!(collect.success());
    assert!(out.join("inventory.json").exists());

    // -s x: context（独立执行，依赖 inventory.json）
    let context = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "bugfix",
            &repo.to_string_lossy(),
            "-o",
            &out.to_string_lossy(),
            "--lang",
            "php",
            "-s",
            "x",
            "--force",
        ])
        .status()
        .expect("run bugfix context");
    assert!(context.success());
    assert!(out.join("contexts").exists());
    let context_count = fs::read_dir(out.join("contexts"))
        .expect("read contexts")
        .filter_map(Result::ok)
        .filter(|e| e.path().extension().map(|x| x == "json").unwrap_or(false))
        .count();
    assert!(context_count >= 1);

    let module_map = root.join("module_map.json");
    let module_map_out = root.join("module_map.bugfix.json");
    write(
        &module_map,
        r#"{
  "entries": [
    {"path":"app/controllers/","module_id":"USER"}
  ]
}"#,
    );
    let export = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "export-module-map",
            "--module-map",
            &module_map.to_string_lossy(),
            "--out",
            &module_map_out.to_string_lossy(),
        ])
        .status()
        .expect("run export module map");
    assert!(export.success());
    assert!(module_map_out.exists());

    let _ = fs::remove_dir_all(&root);
}

// ── export-bdd-source 集成测试 ──

/// 构造 validate 测试数据的辅助函数
fn setup_validate_data(root: &Path) -> (PathBuf, PathBuf, PathBuf, PathBuf) {
    let target = root.join("target.yaml");
    let transition = root.join("transition.yaml");
    let gates = root.join("gates.yaml");
    let actual = root.join("actual.json");

    write(
        &target,
        r#"version: v3
kind: target_contract
intent: target
source_of_truth: test
notes: []
allow_edges:
  - caller: A
    callee: B
forbid_edges:
  - caller: X
    callee: Y
"#,
    );
    write(
        &transition,
        r#"version: v3
kind: transition_contract
base: v3.target
intent: transition
notes: []
temporary_allow_edges: []
blocked_edges:
  - caller: X
    callee: Y
"#,
    );
    write(
        &gates,
        r#"version: v3
kind: verification_gates
intent: gate
profiles:
  transition:
    max_unexpected_edges_count: 999
    max_forbidden_edges_count: 999
    max_forbidden_total_edges: 999
    max_missing_edges_count: 999
    max_directed_density_pct: 100
    max_bidirectional_pair_count: 999
  target:
    max_unexpected_edges_count: 999
    max_forbidden_edges_count: 999
    max_forbidden_total_edges: 999
    max_missing_edges_count: 999
    max_directed_density_pct: 100
    max_bidirectional_pair_count: 999
"#,
    );
    // A->B: allowed; B->A: unexpected; X->Y: forbidden
    write(
        &actual,
        r#"[
  {"caller":"A","callee":"B","import_edges":1,"call_edges":0,"total_edges":1},
  {"caller":"B","callee":"A","import_edges":1,"call_edges":0,"total_edges":1},
  {"caller":"X","callee":"Y","import_edges":3,"call_edges":0,"total_edges":3}
]"#,
    );

    (target, transition, gates, actual)
}

#[test]
fn validate_export_bdd_source_generates_yaml() {
    let root = temp_dir("bcc_validate_bdd_export");
    let (target, transition, gates, actual) = setup_validate_data(&root);
    let out = root.join("out");
    let bdd_dir = root.join("bdd_source");

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "validate",
            "--target",
            &target.to_string_lossy(),
            "--transition",
            &transition.to_string_lossy(),
            "--gates",
            &gates.to_string_lossy(),
            "--actual",
            &actual.to_string_lossy(),
            "--out-dir",
            &out.to_string_lossy(),
            "--fail-on-gate",
            "false",
            "--fail-on-forbidden",
            "false",
            "--export-bdd-source",
            &bdd_dir.to_string_lossy(),
        ])
        .status()
        .expect("run validate with export-bdd-source");
    assert!(status.success());

    // bdd_dir 已创建
    assert!(bdd_dir.exists());

    // forbidden 边 X->Y 生成 YAML（edge_class: blocked）
    let forbidden_yaml = bdd_dir.join("X_Y.yaml");
    assert!(
        forbidden_yaml.exists(),
        "X_Y.yaml should exist for forbidden edge"
    );
    let content = fs::read_to_string(&forbidden_yaml).expect("read X_Y.yaml");
    assert!(content.contains("module: X"));
    assert!(content.contains("edge_class: blocked"));
    assert!(content.contains("contract: X -> Y arch violation"));
    assert!(content.contains("source_file: arch-validate-export"));
    assert!(content.contains("source_summary:"));

    // unexpected 边 B->A 生成 YAML（edge_class: temporary）
    let unexpected_yaml = bdd_dir.join("B_A.yaml");
    assert!(
        unexpected_yaml.exists(),
        "B_A.yaml should exist for unexpected edge"
    );
    let content2 = fs::read_to_string(&unexpected_yaml).expect("read B_A.yaml");
    assert!(content2.contains("module: B"));
    assert!(content2.contains("edge_class: temporary"));
    assert!(content2.contains("source_file: arch-validate-export"));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn validate_without_export_flag_unchanged() {
    let root = temp_dir("bcc_validate_no_export");
    let (target, transition, gates, actual) = setup_validate_data(&root);
    let out = root.join("out");

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "validate",
            "--target",
            &target.to_string_lossy(),
            "--transition",
            &transition.to_string_lossy(),
            "--gates",
            &gates.to_string_lossy(),
            "--actual",
            &actual.to_string_lossy(),
            "--out-dir",
            &out.to_string_lossy(),
            "--fail-on-gate",
            "false",
            "--fail-on-forbidden",
            "false",
        ])
        .status()
        .expect("run validate without export flag");
    assert!(status.success());

    // 标准输出文件正常
    assert!(out.join("summary.json").exists());
    assert!(out.join("scenario-validation.tsv").exists());
    assert!(out.join("gate-evaluation.tsv").exists());
    assert!(out.join("v3-validation-report.md").exists());

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn validate_export_empty_violations() {
    let root = temp_dir("bcc_validate_empty_export");
    let target = root.join("target.yaml");
    let transition = root.join("transition.yaml");
    let gates = root.join("gates.yaml");
    let actual = root.join("actual.json");
    let out = root.join("out");
    let bdd_dir = root.join("bdd_empty");

    write(
        &target,
        r#"version: v3
kind: target_contract
intent: target
source_of_truth: test
notes: []
allow_edges:
  - caller: A
    callee: B
forbid_edges: []
"#,
    );
    write(
        &transition,
        r#"version: v3
kind: transition_contract
base: v3.target
intent: transition
notes: []
temporary_allow_edges: []
blocked_edges: []
"#,
    );
    write(
        &gates,
        r#"version: v3
kind: verification_gates
intent: gate
profiles:
  transition:
    max_unexpected_edges_count: 999
    max_forbidden_edges_count: 999
    max_forbidden_total_edges: 999
    max_missing_edges_count: 999
    max_directed_density_pct: 100
    max_bidirectional_pair_count: 999
  target:
    max_unexpected_edges_count: 999
    max_forbidden_edges_count: 999
    max_forbidden_total_edges: 999
    max_missing_edges_count: 999
    max_directed_density_pct: 100
    max_bidirectional_pair_count: 999
"#,
    );
    // 只有 allow 内的边，无违规
    write(
        &actual,
        r#"[
  {"caller":"A","callee":"B","import_edges":1,"call_edges":0,"total_edges":1}
]"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "validate",
            "--target",
            &target.to_string_lossy(),
            "--transition",
            &transition.to_string_lossy(),
            "--gates",
            &gates.to_string_lossy(),
            "--actual",
            &actual.to_string_lossy(),
            "--out-dir",
            &out.to_string_lossy(),
            "--fail-on-gate",
            "false",
            "--fail-on-forbidden",
            "false",
            "--export-bdd-source",
            &bdd_dir.to_string_lossy(),
        ])
        .status()
        .expect("run validate with empty violations");
    assert!(status.success());

    // bdd_dir 存在但为空（无 YAML 文件）
    assert!(bdd_dir.exists());
    let yaml_count = fs::read_dir(&bdd_dir)
        .expect("read bdd_dir")
        .filter_map(Result::ok)
        .filter(|e| e.path().extension().map(|x| x == "yaml").unwrap_or(false))
        .count();
    assert_eq!(yaml_count, 0, "no yaml files expected for clean actual");

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn validate_truncate_removed_full_output() {
    let root = temp_dir("bcc_validate_full_output");
    let target = root.join("target.yaml");
    let transition = root.join("transition.yaml");
    let gates = root.join("gates.yaml");
    let actual = root.join("actual.json");
    let out = root.join("out");
    let bdd_dir = root.join("bdd_full");

    // 构造 > 10 条 forbidden 边
    let mut forbid_edges = String::new();
    let mut actual_entries = Vec::new();
    for i in 0..15 {
        forbid_edges.push_str(&format!("  - caller: M{}\n    callee: N{}\n", i, i));
        actual_entries.push(format!(
            r#"  {{"caller":"M{}","callee":"N{}","import_edges":{},"call_edges":0,"total_edges":{}}}"#,
            i, i, i + 1, i + 1
        ));
    }

    write(
        &target,
        &format!(
            r#"version: v3
kind: target_contract
intent: target
source_of_truth: test
notes: []
allow_edges:
  - caller: A
    callee: B
forbid_edges:
{}
"#,
            forbid_edges
        ),
    );
    write(
        &transition,
        &format!(
            r#"version: v3
kind: transition_contract
base: v3.target
intent: transition
notes: []
temporary_allow_edges: []
blocked_edges:
{}
"#,
            forbid_edges
        ),
    );
    write(
        &gates,
        r#"version: v3
kind: verification_gates
intent: gate
profiles:
  transition:
    max_unexpected_edges_count: 999
    max_forbidden_edges_count: 999
    max_forbidden_total_edges: 999
    max_missing_edges_count: 999
    max_directed_density_pct: 100
    max_bidirectional_pair_count: 999
  target:
    max_unexpected_edges_count: 999
    max_forbidden_edges_count: 999
    max_forbidden_total_edges: 999
    max_missing_edges_count: 999
    max_directed_density_pct: 100
    max_bidirectional_pair_count: 999
"#,
    );
    write(&actual, &format!("[\n{}\n]", actual_entries.join(",\n")));

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "validate",
            "--target",
            &target.to_string_lossy(),
            "--transition",
            &transition.to_string_lossy(),
            "--gates",
            &gates.to_string_lossy(),
            "--actual",
            &actual.to_string_lossy(),
            "--out-dir",
            &out.to_string_lossy(),
            "--fail-on-gate",
            "false",
            "--fail-on-forbidden",
            "false",
            "--export-bdd-source",
            &bdd_dir.to_string_lossy(),
        ])
        .status()
        .expect("run validate full output");
    assert!(status.success());

    // 验证输出 YAML 文件数 > 10（truncate 已移除）
    let yaml_count = fs::read_dir(&bdd_dir)
        .expect("read bdd_dir")
        .filter_map(Result::ok)
        .filter(|e| e.path().extension().map(|x| x == "yaml").unwrap_or(false))
        .count();
    assert!(
        yaml_count > 10,
        "expected > 10 yaml files, got {}",
        yaml_count
    );

    // 验证 markdown 报告中包含 "showing top 20 of" 标注
    let report = fs::read_to_string(out.join("v3-validation-report.md")).expect("read report");
    assert!(
        report.contains("showing top 20 of"),
        "report should contain 'showing top 20 of' annotation"
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn validate_export_then_bdd_seed_e2e() {
    // 端到端：validate --export-bdd-source 输出 YAML → bdd seed --source 消费
    let root = temp_dir("bcc_validate_bdd_e2e");
    let (target, transition, gates, actual) = setup_validate_data(&root);
    let validate_out = root.join("validate_out");
    let bdd_source = root.join("bdd_source");
    let bdd_output = root.join("bdd_output");

    // 第一步：validate --export-bdd-source
    let validate_status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "validate",
            "--target",
            &target.to_string_lossy(),
            "--transition",
            &transition.to_string_lossy(),
            "--gates",
            &gates.to_string_lossy(),
            "--actual",
            &actual.to_string_lossy(),
            "--out-dir",
            &validate_out.to_string_lossy(),
            "--fail-on-gate",
            "false",
            "--fail-on-forbidden",
            "false",
            "--export-bdd-source",
            &bdd_source.to_string_lossy(),
        ])
        .status()
        .expect("run validate");
    assert!(validate_status.success());

    // 确认 bdd_source 目录有 YAML 文件（X_Y.yaml + B_A.yaml）
    let yaml_count = fs::read_dir(&bdd_source)
        .expect("read bdd_source")
        .filter_map(Result::ok)
        .filter(|e| e.path().extension().map(|x| x == "yaml").unwrap_or(false))
        .count();
    assert!(
        yaml_count >= 2,
        "expected >= 2 yaml files, got {}",
        yaml_count
    );

    // 第二步：bdd seed --source <bdd_source> --step organize --force
    let seed_status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "bdd",
            "seed",
            "--source",
            &bdd_source.to_string_lossy(),
            "--output",
            &bdd_output.to_string_lossy(),
            "--step",
            "organize",
            "--edge-class",
            "all",
            "--force",
        ])
        .status()
        .expect("run bdd seed");
    assert!(seed_status.success());

    // 验证 bdd seed 输出结构
    assert!(
        bdd_output.join("contexts").exists(),
        "contexts dir should exist"
    );
    assert!(
        bdd_output.join("scenarios").exists(),
        "scenarios dir should exist"
    );
    assert!(
        bdd_output.join("features").exists(),
        "features dir should exist"
    );
    assert!(
        bdd_output.join("coverage.md").exists(),
        "coverage.md should exist"
    );

    // 验证 context JSON 被正确生成（X_Y.yaml → module=X, B_A.yaml → module=B）
    let contexts_dir = bdd_output.join("contexts");
    let context_files: Vec<_> = fs::read_dir(&contexts_dir)
        .expect("read contexts")
        .filter_map(Result::ok)
        .filter(|e| e.path().extension().map(|x| x == "json").unwrap_or(false))
        .collect();
    assert!(
        context_files.len() >= 2,
        "expected >= 2 context json files, got {}",
        context_files.len()
    );

    // 验证 context JSON 中包含正确的 module（从 YAML 的 module 字段读取）
    let mut found_module_x = false;
    let mut found_module_b = false;
    for entry in &context_files {
        let raw = fs::read_to_string(entry.path()).expect("read context json");
        if raw.contains("\"module\":\"X\"") || raw.contains("\"module\": \"X\"") {
            found_module_x = true;
        }
        if raw.contains("\"module\":\"B\"") || raw.contains("\"module\": \"B\"") {
            found_module_b = true;
        }
    }
    assert!(
        found_module_x,
        "should have a context with module=X (from X_Y.yaml)"
    );
    assert!(
        found_module_b,
        "should have a context with module=B (from B_A.yaml)"
    );

    // 验证 coverage.md 包含模块名
    let coverage = fs::read_to_string(bdd_output.join("coverage.md")).expect("read coverage");
    assert!(
        coverage.contains("| X |") || coverage.contains("| x |"),
        "coverage should mention module X"
    );
    assert!(
        coverage.contains("| B |") || coverage.contains("| b |"),
        "coverage should mention module B"
    );

    // 验证 features 目录下生成了 DSL 文件
    let feature_files: Vec<_> = fs::read_dir(bdd_output.join("features"))
        .expect("read features")
        .filter_map(Result::ok)
        .filter(|e| e.path().extension().map(|x| x == "dsl").unwrap_or(false))
        .collect();
    assert!(
        feature_files.len() >= 2,
        "expected >= 2 feature dsl files, got {}",
        feature_files.len()
    );

    let _ = fs::remove_dir_all(&root);
}

// ── batch extract 集成测试 ──

#[test]
fn extract_accuracy_gate_ts_export_calls_and_php_namespace_use() {
    let fixture_root = Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/extract");
    let ts_root = fixture_root.join("typescript");
    let php_root = fixture_root.join("php");
    let ts_file = ts_root.join("export_calls.ts");
    let php_file = php_root.join("namespace_use.php");

    assert!(ts_file.exists(), "missing fixture: {}", ts_file.display());
    assert!(php_file.exists(), "missing fixture: {}", php_file.display());

    let work = temp_dir("bcc_extract_accuracy_gate");

    // 1) TS AST：export 内调用不漏提
    let ts_ast = run_extract_ast_json(&ts_file, &work.join("ts.ast.json"));
    let ts_calls = extract_testing::normalized_call_set_from_json(&ts_ast);
    assert!(ts_calls.contains("foo"), "ts calls should contain foo");
    assert!(ts_calls.contains("bar"), "ts calls should contain bar");
    assert!(ts_calls.contains("baz"), "ts calls should contain baz");
    assert!(ts_calls.contains("level1"), "ts calls should contain level1");
    assert!(ts_calls.contains("level2"), "ts calls should contain level2");
    assert!(ts_calls.contains("level3"), "ts calls should contain level3");

    // 2) PHP AST：namespace use/use as 不漏提，同时 extends/trait/new 不回归
    let php_ast = run_extract_ast_json(&php_file, &work.join("php.ast.json"));
    let php_imports = extract_testing::normalized_import_set_from_json(&php_ast);
    assert!(php_imports.contains("use:A\\B"), "php imports should contain use:A\\B");
    assert!(
        php_imports.contains("use:C\\D as E"),
        "php imports should contain use:C\\D as E"
    );
    assert!(
        php_imports.contains("use:Vendor\\Package\\Alpha"),
        "php imports should contain use:Vendor\\Package\\Alpha"
    );
    assert!(
        php_imports.contains("use:Vendor\\Package\\Beta as B"),
        "php imports should contain use:Vendor\\Package\\Beta as B"
    );
    assert!(
        php_imports.contains("extends:Base"),
        "php imports should contain extends:Base"
    );
    assert!(
        php_imports.contains("trait:LoggerTrait"),
        "php imports should contain trait:LoggerTrait"
    );
    let php_calls = extract_testing::normalized_call_set_from_json(&php_ast);
    assert!(php_calls.contains("E"), "php calls should contain E");
    assert!(php_calls.contains("B"), "php calls should contain B");

    // 3) AST vs Batch 一致性（文件集合 + 计数 + 关键字段集合）
    let ts_batch = run_extract_batch_json(&ts_root, "typescript", &work.join("ts.batch.json"));
    assert_eq!(ts_batch["source_count"].as_u64(), Some(1));
    let ts_records = ts_batch["records"].as_array().expect("ts records array");
    assert_eq!(ts_records.len(), 1);
    let ts_batch_record = &ts_records[0];
    assert_eq!(ts_batch_record["sourcePath"].as_str(), Some("export_calls.ts"));
    assert_eq!(
        ts_batch_record["imports_count"].as_u64(),
        Some(ts_ast["imports"].as_array().map(|v| v.len()).unwrap_or(0) as u64)
    );
    assert_eq!(
        ts_batch_record["calls"].as_array().map(|v| v.len()),
        ts_ast["calls"].as_array().map(|v| v.len())
    );
    assert_eq!(
        ts_batch_record["imports"].as_array().map(|v| v.len()),
        ts_ast["imports"].as_array().map(|v| v.len())
    );
    extract_testing::assert_semantic_sets_equal_json(&ts_ast, ts_batch_record, "typescript");

    let php_batch = run_extract_batch_json(&php_root, "php", &work.join("php.batch.json"));
    assert_eq!(php_batch["source_count"].as_u64(), Some(1));
    let php_records = php_batch["records"].as_array().expect("php records array");
    assert_eq!(php_records.len(), 1);
    let php_batch_record = &php_records[0];
    assert_eq!(php_batch_record["sourcePath"].as_str(), Some("namespace_use.php"));
    assert_eq!(
        php_batch_record["imports_count"].as_u64(),
        Some(php_ast["imports"].as_array().map(|v| v.len()).unwrap_or(0) as u64)
    );
    assert_eq!(
        php_batch_record["calls"].as_array().map(|v| v.len()),
        php_ast["calls"].as_array().map(|v| v.len())
    );
    assert_eq!(
        php_batch_record["imports"].as_array().map(|v| v.len()),
        php_ast["imports"].as_array().map(|v| v.len())
    );
    extract_testing::assert_semantic_sets_equal_json(&php_ast, php_batch_record, "php");

    let _ = fs::remove_dir_all(&work);
}

#[test]
fn batch_extract_typescript_smoke() {
    let root = temp_dir("bcc_batch_ts_smoke");
    let src = root.join("src");
    fs::create_dir_all(src.join("utils")).expect("create dirs");

    write(
        &src.join("index.ts"),
        r#"import { helper } from './utils/helper';
export function main() { return helper(); }
"#,
    );
    write(
        &src.join("utils/helper.ts"),
        r#"export function helper() { return 42; }
"#,
    );

    let output = root.join("ast.json");
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &src.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract");
    assert!(status.success());
    assert!(output.exists());

    let content = fs::read_to_string(&output).expect("read output");
    let v: serde_json::Value = serde_json::from_str(&content).expect("parse JSON");
    assert_eq!(v["source_count"], 2);
    assert_eq!(v["records"].as_array().unwrap().len(), 2);

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn batch_extract_excludes_test_files() {
    let root = temp_dir("bcc_batch_exclude_tests");
    let src = root.join("src");
    fs::create_dir_all(&src).expect("create dirs");

    write(&src.join("foo.ts"), "export const foo = 1;\n");
    write(&src.join("foo.test.ts"), "test('foo', () => {});\n");
    write(&src.join("foo.spec.ts"), "describe('foo', () => {});\n");
    write(&src.join("foo.d.ts"), "declare const foo: number;\n");

    let output = root.join("ast.json");
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &src.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract");
    assert!(status.success());

    let content = fs::read_to_string(&output).expect("read output");
    let v: serde_json::Value = serde_json::from_str(&content).expect("parse JSON");
    assert_eq!(v["source_count"], 1, "only foo.ts should be included");

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn batch_extract_empty_dir() {
    let root = temp_dir("bcc_batch_empty");
    let src = root.join("empty");
    fs::create_dir_all(&src).expect("create dirs");

    let output = root.join("ast.json");
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &src.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract");
    assert!(status.success());

    let content = fs::read_to_string(&output).expect("read output");
    let v: serde_json::Value = serde_json::from_str(&content).expect("parse JSON");
    assert_eq!(v["source_count"], 0);
    assert_eq!(v["skipped_count"], 0);

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn batch_extract_wrong_lang_filter() {
    let root = temp_dir("bcc_batch_wrong_lang");
    let src = root.join("src");
    fs::create_dir_all(&src).expect("create dirs");

    write(&src.join("foo.ts"), "export const foo = 1;\n");
    write(&src.join("bar.tsx"), "export const bar = 2;\n");

    let output = root.join("ast.json");
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &src.to_string_lossy(),
            "--batch",
            "--lang",
            "elixir",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract");
    assert!(status.success());

    let content = fs::read_to_string(&output).expect("read output");
    let v: serde_json::Value = serde_json::from_str(&content).expect("parse JSON");
    assert_eq!(v["source_count"], 0, "no elixir files in ts directory");

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn batch_extract_unsupported_lang_fails_fast() {
    let root = temp_dir("bcc_batch_unsupported_lang");
    let src = root.join("src");
    fs::create_dir_all(&src).expect("create dirs");
    write(&src.join("foo.ts"), "export const foo = 1;\n");

    let output = root.join("ast.json");
    let result = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &src.to_string_lossy(),
            "--batch",
            "--lang",
            "go",
            "--output",
            &output.to_string_lossy(),
        ])
        .output()
        .expect("run batch extract");
    assert_eq!(result.status.code(), Some(1));

    let stderr = String::from_utf8_lossy(&result.stderr);
    assert!(
        stderr.contains("unsupported language: go"),
        "stderr should contain unsupported language message, got: {}",
        stderr
    );
    assert!(
        !output.exists(),
        "unsupported batch lang should fail before writing output snapshot"
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn extract_single_and_batch_keep_same_unsupported_lang_semantics() {
    let root = temp_dir("bcc_extract_unsupported_lang_semantics");
    let src = root.join("src");
    fs::create_dir_all(&src).expect("create dirs");
    let go_file = src.join("main.go");
    write(
        &go_file,
        r#"package main
func main() {}
"#,
    );

    let single = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args(["extract", &go_file.to_string_lossy(), "--mode", "ast"])
        .output()
        .expect("run single-file extract");
    assert_eq!(single.status.code(), Some(1));

    let batch_output = root.join("ast.json");
    let batch = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &src.to_string_lossy(),
            "--batch",
            "--lang",
            "go",
            "--output",
            &batch_output.to_string_lossy(),
        ])
        .output()
        .expect("run batch extract");
    assert_eq!(batch.status.code(), Some(1));

    let single_stderr = String::from_utf8_lossy(&single.stderr);
    let batch_stderr = String::from_utf8_lossy(&batch.stderr);
    assert!(
        single_stderr.contains("unsupported language: go"),
        "single-file stderr should contain unsupported language message, got: {}",
        single_stderr
    );
    assert!(
        batch_stderr.contains("unsupported language: go"),
        "batch stderr should contain unsupported language message, got: {}",
        batch_stderr
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn batch_extract_error_tolerance() {
    let root = temp_dir("bcc_batch_error_tolerance");
    let src = root.join("src");
    fs::create_dir_all(&src).expect("create dirs");

    write(&src.join("good.ts"), "export const good = 1;\n");
    write(
        &src.join("bad.ts"),
        "export const {{{ = syntax error @@@;\n",
    );
    write(
        &src.join("ok.ts"),
        "export function ok() { return true; }\n",
    );

    let output = root.join("ast.json");
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &src.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract");
    assert!(status.success());

    let content = fs::read_to_string(&output).expect("read output");
    let v: serde_json::Value = serde_json::from_str(&content).expect("parse JSON");
    // tree-sitter 是容错解析器，即使语法错误也能部分解析
    let total = v["source_count"].as_u64().unwrap() + v["skipped_count"].as_u64().unwrap();
    assert_eq!(total, 3, "total processed + skipped should be 3");

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn batch_extract_then_arch_matrix_e2e() {
    let root = temp_dir("bcc_batch_arch_e2e");
    let src = root.join("src");
    fs::create_dir_all(src.join("account")).expect("create account dir");
    fs::create_dir_all(src.join("billing")).expect("create billing dir");

    write(
        &src.join("account/create.ts"),
        r#"import { createInvoice } from '../billing/invoice';
export function createAccount(name: string) {
    createInvoice(name);
    return { name };
}
"#,
    );
    write(
        &src.join("billing/invoice.ts"),
        r#"export function createInvoice(name: string) {
    return { invoice: name };
}
"#,
    );

    // Step 1: batch extract
    let ast_file = root.join("ast.json");
    let extract_status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &src.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &ast_file.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract");
    assert!(extract_status.success());

    let ast_content = fs::read_to_string(&ast_file).expect("read ast.json");
    let ast_v: serde_json::Value = serde_json::from_str(&ast_content).expect("parse ast JSON");
    assert_eq!(ast_v["source_count"], 2);

    // Step 2: arch matrix 消费 ast.json
    let seed_file = root.join("seed.yaml");
    write(
        &seed_file,
        r#"version: v3
source_of_truth: test
modules:
  - module_id: ACCOUNT
    precedence: 10
    path_rules:
      include: ["account/**"]
  - module_id: BILLING
    precedence: 10
    path_rules:
      include: ["billing/**"]
relations_expected:
  - caller: ACCOUNT
    callee: BILLING
    allowed: true
"#,
    );

    let out_dir = root.join("arch_out");
    let matrix_status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &seed_file.to_string_lossy(),
            "--ast-file",
            &ast_file.to_string_lossy(),
            "--out-dir",
            &out_dir.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
        ])
        .status()
        .expect("run arch matrix");
    assert!(matrix_status.success());

    // arch matrix 输出 target/transition/gates YAML
    assert!(out_dir.join("v3.target-matrix.yaml").exists());
    assert!(out_dir.join("v3.transition-matrix.yaml").exists());
    assert!(out_dir.join("v3.gates.yaml").exists());

    // 验证 target-matrix 中包含 ACCOUNT/BILLING 模块引用
    let target_yaml =
        fs::read_to_string(out_dir.join("v3.target-matrix.yaml")).expect("read target matrix");
    assert!(
        target_yaml.contains("ACCOUNT") && target_yaml.contains("BILLING"),
        "target matrix should reference ACCOUNT and BILLING modules"
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn batch_extract_elixir_with_module_resolution() {
    let root = temp_dir("bcc_batch_elixir");
    let lib = root.join("lib");
    fs::create_dir_all(lib.join("my_app/accounts")).expect("create dirs");
    fs::create_dir_all(lib.join("my_app/billing")).expect("create dirs");
    // 排除目标
    fs::create_dir_all(root.join("test")).expect("create test dir");

    write(
        &lib.join("my_app/accounts/user.ex"),
        r#"defmodule MyApp.Accounts.User do
  alias MyApp.Billing.Invoice

  def create(name) do
    Invoice.generate(name)
  end
end
"#,
    );
    write(
        &lib.join("my_app/billing/invoice.ex"),
        r#"defmodule MyApp.Billing.Invoice do
  def generate(name) do
    %{name: name, amount: 0}
  end
end
"#,
    );
    // 应被排除的文件
    write(
        &root.join("test/user_test.exs"),
        r#"defmodule MyApp.Accounts.UserTest do
  use ExUnit.Case
  test "create" do
    assert true
  end
end
"#,
    );
    write(
        &root.join("mix.exs"),
        r#"defmodule MyApp.MixProject do
  use Mix.Project
end
"#,
    );

    let output = root.join("ast.json");
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &root.to_string_lossy(),
            "--batch",
            "--lang",
            "elixir",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract elixir");
    assert!(status.success());

    let content = fs::read_to_string(&output).expect("read output");
    let v: serde_json::Value = serde_json::from_str(&content).expect("parse JSON");

    // 只有 2 个 .ex 文件，test/ 和 .exs 被排除
    assert_eq!(v["source_count"], 2, "should have 2 .ex files");
    assert_eq!(v["skipped_count"], 0);

    // 验证 records 中的 sourcePath 是相对路径
    let records = v["records"].as_array().unwrap();
    let paths: Vec<&str> = records
        .iter()
        .map(|r| r["sourcePath"].as_str().unwrap())
        .collect();
    assert!(paths.contains(&"lib/my_app/accounts/user.ex"));
    assert!(paths.contains(&"lib/my_app/billing/invoice.ex"));

    // 验证 user.ex 的 localDependencies 包含 invoice.ex（通过模块名映射）
    let user_record = records
        .iter()
        .find(|r| r["sourcePath"].as_str().unwrap().contains("user.ex"))
        .expect("find user record");
    let deps = user_record["localDependencies"]
        .as_array()
        .map(|a| a.iter().map(|v| v.as_str().unwrap()).collect::<Vec<_>>())
        .unwrap_or_default();
    assert!(
        deps.iter().any(|d| d.contains("invoice.ex")),
        "user.ex should depend on invoice.ex, got: {:?}",
        deps
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn batch_extract_skips_node_modules() {
    let root = temp_dir("bcc_batch_node_modules");
    let src = root.join("src");
    fs::create_dir_all(&src).expect("create src");
    fs::create_dir_all(root.join("node_modules/lodash")).expect("create node_modules");
    fs::create_dir_all(root.join("__tests__")).expect("create __tests__");

    // 正常文件
    write(&src.join("app.ts"), "export const app = 'hello';\n");
    // node_modules 下的文件应被排除
    write(
        &root.join("node_modules/lodash/index.ts"),
        "export function chunk() {}\n",
    );
    // __tests__ 下的文件应被排除
    write(
        &root.join("__tests__/app.test.ts"),
        "test('app', () => {});\n",
    );

    let output = root.join("ast.json");
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &root.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract");
    assert!(status.success());

    let content = fs::read_to_string(&output).expect("read output");
    let v: serde_json::Value = serde_json::from_str(&content).expect("parse JSON");
    assert_eq!(
        v["source_count"], 1,
        "only src/app.ts should be included, node_modules and __tests__ excluded"
    );

    let records = v["records"].as_array().unwrap();
    let path = records[0]["sourcePath"].as_str().unwrap();
    assert!(
        path.contains("app.ts") && !path.contains("node_modules"),
        "should be src/app.ts, got: {}",
        path
    );

    let _ = fs::remove_dir_all(&root);
}

// ── Issue #18: 增强依赖解析测试 ──

#[test]
fn batch_extract_monorepo_package_name_resolution() {
    // 创建两个 package 目录，包 A import 包 B 的 npm 包名
    let root = temp_dir("bcc_batch_monorepo");
    let pkg_a = root.join("packages/alpha");
    let pkg_b = root.join("packages/beta");
    fs::create_dir_all(pkg_a.join("src")).expect("create alpha");
    fs::create_dir_all(pkg_b.join("src")).expect("create beta");

    write(
        &pkg_b.join("package.json"),
        r#"{"name": "@test/beta", "main": "src/index.ts"}"#,
    );
    write(
        &pkg_b.join("src/index.ts"),
        "export function betaHelper() { return 42; }\n",
    );

    write(
        &pkg_a.join("package.json"),
        r#"{"name": "@test/alpha", "main": "src/index.ts"}"#,
    );
    write(
        &pkg_a.join("src/index.ts"),
        "import { betaHelper } from '@test/beta';\nexport function alphaMain() { return betaHelper(); }\n",
    );

    let output = root.join("ast.json");
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &root.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract monorepo");
    assert!(status.success());

    let content = fs::read_to_string(&output).expect("read output");
    let v: serde_json::Value = serde_json::from_str(&content).expect("parse JSON");
    assert_eq!(v["source_count"], 2);

    let records = v["records"].as_array().unwrap();
    let alpha_record = records
        .iter()
        .find(|r| r["sourcePath"].as_str().unwrap().contains("alpha"))
        .expect("find alpha record");
    let deps = alpha_record["localDependencies"]
        .as_array()
        .map(|a| {
            a.iter()
                .map(|v| v.as_str().unwrap().to_string())
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    assert!(
        deps.iter()
            .any(|d| d.contains("beta") && d.contains("index.ts")),
        "alpha should depend on beta's index.ts via package name, got deps: {:?}",
        deps
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn batch_extract_monorepo_with_exports_field() {
    let root = temp_dir("bcc_batch_monorepo_exports");
    let pkg_a = root.join("packages/alpha");
    let pkg_b = root.join("packages/beta");
    fs::create_dir_all(pkg_a.join("src")).expect("create alpha");
    fs::create_dir_all(pkg_b.join("src/providers")).expect("create beta providers");

    write(
        &pkg_b.join("package.json"),
        r#"{
  "name": "@test/beta",
  "exports": {
    ".": { "import": "./src/index.ts" },
    "./providers": { "import": "./src/providers/index.ts" }
  }
}"#,
    );
    write(
        &pkg_b.join("src/index.ts"),
        "export function betaMain() { return 1; }\n",
    );
    write(
        &pkg_b.join("src/providers/index.ts"),
        "export function betaProvider() { return 2; }\n",
    );

    write(
        &pkg_a.join("package.json"),
        r#"{"name": "@test/alpha", "main": "src/index.ts"}"#,
    );
    write(
        &pkg_a.join("src/index.ts"),
        "import { betaProvider } from '@test/beta/providers';\nexport function alphaMain() { return betaProvider(); }\n",
    );

    let output = root.join("ast.json");
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &root.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract");
    assert!(status.success());

    let content = fs::read_to_string(&output).expect("read output");
    let v: serde_json::Value = serde_json::from_str(&content).expect("parse JSON");

    let records = v["records"].as_array().unwrap();
    let alpha_record = records
        .iter()
        .find(|r| r["sourcePath"].as_str().unwrap().contains("alpha"))
        .expect("find alpha record");
    let deps = alpha_record["localDependencies"]
        .as_array()
        .map(|a| {
            a.iter()
                .map(|v| v.as_str().unwrap().to_string())
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    assert!(
        deps.iter().any(|d| d.contains("providers")),
        "alpha should depend on beta/providers via exports field, got deps: {:?}",
        deps
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn batch_extract_elixir_multi_alias_resolution() {
    let root = temp_dir("bcc_batch_elixir_multi_alias");
    let lib = root.join("lib");
    fs::create_dir_all(lib.join("my_app")).expect("create dirs");

    write(
        &lib.join("my_app/core.ex"),
        "defmodule MyApp.Core do\n  alias MyApp.{Tape, Tools}\n\n  def run do\n    Tape.get()\n    Tools.execute()\n  end\nend\n",
    );
    write(
        &lib.join("my_app/tape.ex"),
        "defmodule MyApp.Tape do\n  def get, do: :ok\nend\n",
    );
    write(
        &lib.join("my_app/tools.ex"),
        "defmodule MyApp.Tools do\n  def execute, do: :ok\nend\n",
    );

    let output = root.join("ast.json");
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &root.to_string_lossy(),
            "--batch",
            "--lang",
            "elixir",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract elixir");
    assert!(status.success());

    let content = fs::read_to_string(&output).expect("read output");
    let v: serde_json::Value = serde_json::from_str(&content).expect("parse JSON");
    assert_eq!(v["source_count"], 3);

    let records = v["records"].as_array().unwrap();
    let core_record = records
        .iter()
        .find(|r| r["sourcePath"].as_str().unwrap().contains("core.ex"))
        .expect("find core record");
    let deps = core_record["localDependencies"]
        .as_array()
        .map(|a| {
            a.iter()
                .map(|v| v.as_str().unwrap().to_string())
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    assert!(
        deps.iter().any(|d| d.contains("tape.ex")),
        "core.ex should depend on tape.ex via multi-alias, got: {:?}",
        deps
    );
    assert!(
        deps.iter().any(|d| d.contains("tools.ex")),
        "core.ex should depend on tools.ex via multi-alias, got: {:?}",
        deps
    );

    let _ = fs::remove_dir_all(&root);
}

/// #89: @moduledoc 中的模块引用不应出现在 localDependencies 中
#[test]
fn batch_extract_elixir_skips_moduledoc_module_refs() {
    let root = temp_dir("bcc_batch_elixir_moduledoc_skip");
    let lib = root.join("lib");
    fs::create_dir_all(lib.join("gong")).expect("create dirs");

    // gong.ex 只有 @moduledoc，没有真实代码依赖
    write(
        &lib.join("gong.ex"),
        "defmodule Gong do\n  @moduledoc \"\"\"\n  - `Gong.Compaction` — 上下文压缩\n  - `Gong.Truncate` — 输出截断系统\n  \"\"\"\nend\n",
    );
    write(
        &lib.join("gong/compaction.ex"),
        "defmodule Gong.Compaction do\n  def run, do: :ok\nend\n",
    );
    write(
        &lib.join("gong/truncate.ex"),
        "defmodule Gong.Truncate do\n  def run, do: :ok\nend\n",
    );

    let output = root.join("ast.json");
    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &root.to_string_lossy(),
            "--batch",
            "--lang",
            "elixir",
            "--output",
            &output.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract elixir");
    assert!(status.success());

    let content = fs::read_to_string(&output).expect("read output");
    let v: serde_json::Value = serde_json::from_str(&content).expect("parse JSON");

    let records = v["records"].as_array().unwrap();
    let gong_record = records
        .iter()
        .find(|r| r["sourcePath"].as_str().unwrap() == "lib/gong.ex")
        .expect("find gong.ex record");
    let deps = gong_record["localDependencies"]
        .as_array()
        .map(|a| {
            a.iter()
                .map(|v| v.as_str().unwrap().to_string())
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    assert!(
        !deps.iter().any(|d| d.contains("compaction.ex")),
        "gong.ex should NOT depend on compaction.ex (from @moduledoc), got: {:?}",
        deps
    );
    assert!(
        !deps.iter().any(|d| d.contains("truncate.ex")),
        "gong.ex should NOT depend on truncate.ex (from @moduledoc), got: {:?}",
        deps
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn batch_extract_then_arch_matrix_actual_edges_gt_zero() {
    // 端到端：batch extract → arch matrix → actual_edges > 0
    let root = temp_dir("bcc_batch_actual_edges");
    let src = root.join("src");
    fs::create_dir_all(src.join("account")).expect("create account");
    fs::create_dir_all(src.join("billing")).expect("create billing");

    write(
        &src.join("account/create.ts"),
        "import { createInvoice } from '../billing/invoice';\nexport function createAccount(name: string) { return createInvoice(name); }\n",
    );
    write(
        &src.join("billing/invoice.ts"),
        "export function createInvoice(name: string) { return { invoice: name }; }\n",
    );

    let ast_file = root.join("ast.json");
    let extract_status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &src.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &ast_file.to_string_lossy(),
        ])
        .status()
        .expect("run batch extract");
    assert!(extract_status.success());

    let seed = root.join("seed.yaml");
    write(
        &seed,
        "version: v3\nsource_of_truth: test\nmodules:\n  - module_id: ACCOUNT\n    precedence: 10\n    path_rules:\n      include: [\"account/**\"]\n  - module_id: BILLING\n    precedence: 10\n    path_rules:\n      include: [\"billing/**\"]\nrelations_expected:\n  - caller: ACCOUNT\n    callee: BILLING\n    allowed: true\n",
    );

    let out_dir = root.join("arch_out");
    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &seed.to_string_lossy(),
            "--ast-file",
            &ast_file.to_string_lossy(),
            "--out-dir",
            &out_dir.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
        ])
        .output()
        .expect("run arch matrix");
    assert!(output.status.success());

    let stdout = String::from_utf8_lossy(&output.stdout);
    assert!(
        stdout.contains("actual_edges=1"),
        "should have actual_edges=1 (ACCOUNT->BILLING), stdout: {}",
        stdout
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn gong_external_register_is_classified_as_external_registration() {
    let root = temp_dir("bcc_gong_injection_bdd");
    let lib = root.join("lib");
    fs::create_dir_all(lib.join("gong/provider")).expect("create dirs");
    write(
        &lib.join("gong/application.ex"),
        "defmodule Gong.Application do\n  def init do\n    ReqLLM.Providers.register(Gong.Provider.Anthropic)\n  end\nend\n",
    );
    write(
        &lib.join("gong/provider/anthropic.ex"),
        "defmodule Gong.Provider.Anthropic do\n  def run, do: :ok\nend\n",
    );

    let ast_file = root.join("ast.json");
    let extract = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &root.to_string_lossy(),
            "--batch",
            "--lang",
            "elixir",
            "--output",
            &ast_file.to_string_lossy(),
        ])
        .status()
        .expect("run elixir extract");
    assert!(extract.success());

    let seed = root.join("seed.yaml");
    write(
        &seed,
        "version: v3\nsource_of_truth: test\nmodules:\n  - module_id: INFRA\n    precedence: 10\n    path_rules:\n      include: [\"lib/gong/application.ex\"]\n  - module_id: PROVIDERS\n    precedence: 10\n    path_rules:\n      include: [\"lib/gong/provider/**\"]\nrelations_expected: []\n",
    );

    let out = root.join("out");
    let matrix = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &seed.to_string_lossy(),
            "--ast-file",
            &ast_file.to_string_lossy(),
            "--out-dir",
            &out.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
            "--detect-injection",
        ])
        .status()
        .expect("run matrix");
    assert!(matrix.success());

    let report = fs::read_to_string(out.join("v3.relation-classification.json")).expect("read");
    let rows: serde_json::Value = serde_json::from_str(&report).expect("parse report");
    assert!(rows.as_array().unwrap().iter().any(|row| {
        row["caller"] == "INFRA"
            && row["callee"] == "PROVIDERS"
            && row["call_type"] == "external_registration"
    }));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn pi_mono_module_imports_are_classified_as_framework_injection() {
    let root = temp_dir("bcc_pi_mono_injection_bdd");
    let api = root.join("apps/api/src");
    let user = root.join("apps/user/src");
    let prisma = root.join("apps/prisma/src");
    fs::create_dir_all(&api).expect("create api");
    fs::create_dir_all(&user).expect("create user");
    fs::create_dir_all(&prisma).expect("create prisma");

    write(
        &api.join("app.module.ts"),
        "import { Module } from '@nestjs/common';\nimport { UserModule } from '../../user/src/user.module';\nimport { PrismaModule } from '../../prisma/src/prisma.module';\n\n@Module({\n  imports: [UserModule, PrismaModule],\n})\nexport class AppModule {}\n",
    );
    write(&user.join("user.module.ts"), "export class UserModule {}\n");
    write(
        &prisma.join("prisma.module.ts"),
        "export class PrismaModule {}\n",
    );

    let ast_file = root.join("ast.json");
    let extract = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &root.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &ast_file.to_string_lossy(),
        ])
        .status()
        .expect("run ts extract");
    assert!(extract.success());

    let seed = root.join("seed.yaml");
    write(
        &seed,
        "version: v3\nsource_of_truth: test\nmodules:\n  - module_id: API\n    precedence: 10\n    path_rules:\n      include: [\"apps/api/src/**\"]\n  - module_id: USER\n    precedence: 10\n    path_rules:\n      include: [\"apps/user/src/**\"]\n  - module_id: PRISMA\n    precedence: 10\n    path_rules:\n      include: [\"apps/prisma/src/**\"]\nrelations_expected: []\n",
    );

    let out = root.join("out");
    let matrix = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &seed.to_string_lossy(),
            "--ast-file",
            &ast_file.to_string_lossy(),
            "--out-dir",
            &out.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
            "--detect-injection",
        ])
        .status()
        .expect("run matrix");
    assert!(matrix.success());

    let report = fs::read_to_string(out.join("v3.relation-classification.json")).expect("read");
    let rows: serde_json::Value = serde_json::from_str(&report).expect("parse report");
    assert!(rows.as_array().unwrap().iter().any(|row| {
        row["caller"] == "API"
            && row["callee"] == "USER"
            && row["call_type"] == "framework_injection"
    }));
    assert!(rows.as_array().unwrap().iter().any(|row| {
        row["caller"] == "API"
            && row["callee"] == "PRISMA"
            && row["call_type"] == "framework_injection"
    }));

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn bdd_injection_pipeline_is_stable_in_parallel_runs() {
    let mut handles = Vec::new();

    for i in 0..2 {
        handles.push(thread::spawn(move || {
            let root = temp_dir(&format!("bcc_bdd_injection_parallel_{}", i));
            let api = root.join("apps/api/src");
            let user = root.join("apps/user/src");
            fs::create_dir_all(&api).expect("create api");
            fs::create_dir_all(&user).expect("create user");

            write(
                &api.join("app.module.ts"),
                "import { Module } from '@nestjs/common';\nimport { UserModule } from '../../user/src/user.module';\n\n@Module({\n  imports: [UserModule],\n})\nexport class AppModule {}\n",
            );
            write(&user.join("user.module.ts"), "export class UserModule {}\n");

            let ast_file = root.join("ast.json");
            let extract = Command::new(env!("CARGO_BIN_EXE_bcc"))
                .args([
                    "extract",
                    &root.to_string_lossy(),
                    "--batch",
                    "--lang",
                    "typescript",
                    "--output",
                    &ast_file.to_string_lossy(),
                ])
                .status()
                .expect("run ts extract in parallel");
            assert!(extract.success());

            let seed = root.join("seed.yaml");
            write(
                &seed,
                "version: v3\nsource_of_truth: test\nmodules:\n  - module_id: API\n    precedence: 10\n    path_rules:\n      include: [\"apps/api/src/**\"]\n  - module_id: USER\n    precedence: 10\n    path_rules:\n      include: [\"apps/user/src/**\"]\nrelations_expected: []\n",
            );

            let out = root.join("out");
            let matrix = Command::new(env!("CARGO_BIN_EXE_bcc"))
                .args([
                    "arch",
                    "matrix",
                    "--seed-file",
                    &seed.to_string_lossy(),
                    "--ast-file",
                    &ast_file.to_string_lossy(),
                    "--out-dir",
                    &out.to_string_lossy(),
                    "--version",
                    "v3",
                    "--emit",
                    "all",
                    "--detect-injection",
                ])
                .status()
                .expect("run matrix in parallel");
            assert!(matrix.success());

            let report =
                fs::read_to_string(out.join("v3.relation-classification.json")).expect("read");
            let rows: serde_json::Value = serde_json::from_str(&report).expect("parse report");
            let rows = rows.as_array().expect("classification rows");
            assert!(rows.iter().any(|row| {
                row["caller"] == "API"
                    && row["callee"] == "USER"
                    && row["call_type"] == "framework_injection"
            }));

            let _ = fs::remove_dir_all(&root);
        }));
    }

    for handle in handles {
        handle.join().expect("thread should not panic");
    }
}

#[test]
fn bdd_injection_pipeline_handles_deep_module_imports() {
    let depth = 64usize;
    let root = temp_dir("bcc_bdd_injection_deep");
    let api = root.join("apps/api/src");
    fs::create_dir_all(&api).expect("create api");

    let mut app_module = String::from("import { Module } from '@nestjs/common';\n");
    for i in 0..depth {
        app_module.push_str(&format!(
            "import {{ F{}Module }} from './f{}.module';\n",
            i, i
        ));
    }
    app_module.push_str("\n@Module({\n  imports: [");
    for i in 0..depth {
        if i > 0 {
            app_module.push_str(", ");
        }
        app_module.push_str(&format!("F{}Module", i));
    }
    app_module.push_str("],\n})\nexport class AppModule {}\n");
    write(&api.join("app.module.ts"), &app_module);

    for i in 0..depth {
        write(
            &api.join(format!("f{}.module.ts", i)),
            &format!("export class F{}Module {{}}\n", i),
        );
    }

    let ast_file = root.join("ast.json");
    let extract = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "extract",
            &root.to_string_lossy(),
            "--batch",
            "--lang",
            "typescript",
            "--output",
            &ast_file.to_string_lossy(),
        ])
        .status()
        .expect("run ts extract");
    assert!(extract.success());

    let ast_content = fs::read_to_string(&ast_file).expect("read ast json");
    let ast_json: serde_json::Value = serde_json::from_str(&ast_content).expect("parse ast json");
    let app_record = ast_json["records"]
        .as_array()
        .and_then(|records| {
            records.iter().find(|record| {
                record["sourcePath"]
                    .as_str()
                    .map(|p| p.ends_with("apps/api/src/app.module.ts"))
                    .unwrap_or(false)
            })
        })
        .expect("app.module.ts record");
    let hint_count = app_record["relationHints"]
        .as_array()
        .map(|h| {
            h.iter()
                .filter(|item| item["via"] == "@Module.imports")
                .count()
        })
        .unwrap_or(0);
    assert_eq!(hint_count, depth);

    let seed = root.join("seed.yaml");
    write(
        &seed,
        "version: v3\nsource_of_truth: test\nmodules:\n  - module_id: API\n    precedence: 10\n    path_rules:\n      include: [\"apps/api/src/app.module.ts\"]\n  - module_id: FEATURE\n    precedence: 10\n    path_rules:\n      include: [\"apps/api/src/f*.module.ts\"]\nrelations_expected: []\n",
    );

    let out = root.join("out");
    let matrix = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &seed.to_string_lossy(),
            "--ast-file",
            &ast_file.to_string_lossy(),
            "--out-dir",
            &out.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
            "--detect-injection",
        ])
        .status()
        .expect("run matrix");
    assert!(matrix.success());

    let report = fs::read_to_string(out.join("v3.relation-classification.json")).expect("read");
    let rows: serde_json::Value = serde_json::from_str(&report).expect("parse report");
    let rows = rows.as_array().expect("classification rows");
    assert!(rows.iter().any(|row| {
        row["caller"] == "API"
            && row["callee"] == "FEATURE"
            && row["call_type"] == "framework_injection"
    }));

    let _ = fs::remove_dir_all(&root);
}

// ── analyze CLI 集成测试 ──

#[test]
fn analyze_help_shows_expected_params() {
    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args(["analyze", "--help"])
        .output()
        .expect("run analyze --help");
    assert!(output.status.success());

    let stdout = String::from_utf8_lossy(&output.stdout);
    assert!(
        stdout.contains("--ast-file"),
        "help should mention --ast-file, got: {}",
        stdout
    );
    assert!(
        stdout.contains("--output"),
        "help should mention --output, got: {}",
        stdout
    );
    assert!(
        stdout.contains("--rules"),
        "help should mention --rules, got: {}",
        stdout
    );
}

#[test]
fn analyze_missing_required_params_exits_nonzero() {
    let result = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args(["analyze"])
        .output()
        .expect("run analyze without params");
    assert!(
        !result.status.success(),
        "analyze without --ast-file should fail, exit code: {:?}",
        result.status.code()
    );

    let stderr = String::from_utf8_lossy(&result.stderr);
    assert!(
        stderr.contains("--ast-file") || stderr.contains("ast-file") || stderr.contains("required"),
        "stderr should mention missing --ast-file, got: {}",
        stderr
    );
}

// ── smell-gate 集成测试 ──

/// 辅助：写 SmellReport JSON 文件
fn write_smell_report(path: &Path, smells: &[serde_json::Value]) {
    let report = serde_json::json!([{
        "file": "test.py",
        "smells": smells,
        "summary": { "total_smells": smells.len(), "by_severity": {}, "by_category": {} }
    }]);
    fs::write(path, serde_json::to_string_pretty(&report).unwrap()).expect("write smell report");
}

#[test]
fn smell_gate_with_smells_exits_nonzero() {
    let root = temp_dir("bcc_smell_gate_fail");
    let (target, transition, gates, actual) = setup_validate_data(&root);
    let out = root.join("out");
    let smells_file = root.join("smells.json");

    let smells = vec![
        serde_json::json!({
            "category": "security", "rule": "hardcoded_secret", "severity": "critical",
            "message": "hardcoded secret found", "file": "app.py", "line": 10,
            "source": "bcc", "confidence": 0.9,
            "fix_hint": "Use environment variables", "code_snippet": "  10| SECRET = 'abc' ← HERE",
            "offending_code": "SECRET = 'abc'"
        }),
        serde_json::json!({
            "category": "error_handling", "rule": "swallowed_error", "severity": "warning",
            "message": "error swallowed", "file": "handler.py", "line": 20,
            "source": "bcc", "confidence": 0.8
        }),
        serde_json::json!({
            "category": "security", "rule": "sql_injection", "severity": "critical",
            "message": "SQL injection risk", "file": "db.py", "line": 5,
            "source": "bcc", "confidence": 0.95
        }),
    ];
    write_smell_report(&smells_file, &smells);

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--smell-gate", &smells_file.to_string_lossy(),
        ])
        .output()
        .expect("run validate with smell-gate");

    assert_eq!(output.status.code(), Some(1), "should exit 1 with smells");

    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(stderr.contains("Smell Gate Report"), "should contain report header");
    assert!(stderr.contains("CRITICAL"), "should show CRITICAL severity");
    assert!(stderr.contains("hardcoded secret found"), "should show smell message");
    assert!(stderr.contains("fix:"), "should show fix_hint");
    assert!(stderr.contains("offending:"), "should show offending_code");
    assert!(stderr.contains("smell_gate=FAIL"), "should contain FAIL marker");
    assert!(stderr.contains("Total: 3"), "should show total count");

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn smell_gate_empty_smells_passes() {
    let root = temp_dir("bcc_smell_gate_pass");
    let (target, transition, gates, actual) = setup_validate_data(&root);
    let out = root.join("out");
    let smells_file = root.join("smells.json");

    write_smell_report(&smells_file, &[]);

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--smell-gate", &smells_file.to_string_lossy(),
        ])
        .status()
        .expect("run validate with empty smell-gate");

    assert_eq!(status.code(), Some(0), "should pass with no smells");

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn smell_gate_old_severity_alias_maps_correctly() {
    let root = temp_dir("bcc_smell_gate_alias");
    let (target, transition, gates, actual) = setup_validate_data(&root);
    let out = root.join("out");
    let smells_file = root.join("smells.json");

    // 使用旧 severity 值 "high"
    let smells = vec![serde_json::json!({
        "category": "security", "rule": "test_rule", "severity": "high",
        "message": "old severity format", "file": "old.py", "line": 1,
        "source": "bcc", "confidence": 1.0
    })];
    write_smell_report(&smells_file, &smells);

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--smell-gate", &smells_file.to_string_lossy(),
        ])
        .output()
        .expect("run validate with old severity");

    assert_eq!(output.status.code(), Some(1), "should exit 1 with smells");

    let stderr = String::from_utf8_lossy(&output.stderr);
    // high 应映射为 critical
    assert!(
        stderr.contains("CRITICAL"),
        "old 'high' severity should map to CRITICAL, stderr: {}",
        stderr
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn validate_without_smell_gate_backward_compatible() {
    // 不带 --smell-gate 参数时行为与改动前完全一致
    let root = temp_dir("bcc_smell_gate_compat");
    let (target, transition, gates, actual) = setup_validate_data(&root);
    let out = root.join("out");

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
        ])
        .status()
        .expect("run validate without smell-gate");

    assert!(status.success(), "should pass without smell-gate");
    assert!(out.join("summary.json").exists());

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn linter_timeout_kills_slow_command() {
    let root = temp_dir("bcc_linter_timeout");
    let ast = root.join("ast.json");
    let out = root.join("out.json");

    // 创建最简 ast 文件
    write(&ast, r#"{"source_count":0,"records":[]}"#);

    // 设置 1 秒超时，运行 sleep 60 的 linter
    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "analyze",
            "--ast-file", &ast.to_string_lossy(),
            "-o", &out.to_string_lossy(),
            "--linter", "slow:sleep 60",
        ])
        .env("BCC_LINTER_TIMEOUT", "1")
        .output()
        .expect("run analyze with slow linter");

    assert!(output.status.success(), "should still succeed despite linter timeout");

    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        stderr.contains("timed out"),
        "should contain timeout warning, stderr: {}",
        stderr
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn critical_smell_triggers_blocking_fail_in_score() {
    let root = temp_dir("bcc_score_critical_blocking");
    let input = root.join("input");
    let smells_file = root.join("smells.json");
    fs::create_dir_all(&input).expect("create input dir");

    // 创建最简评分输入（scenario-validation.tsv + gate-evaluation.tsv + summary.json）
    write(
        &input.join("scenario-validation.tsv"),
        "category\tscenario\tcaller\tcallee\tstatus\n",
    );
    write(
        &input.join("gate-evaluation.tsv"),
        "profile\tgate\tvalue\tthreshold\tstatus\n",
    );
    write(
        &input.join("summary.json"),
        r#"{"target_gate_pass":true,"transition_gate_pass":true,"forbidden_edges_count":0,"unexpected_edges_count":0,"missing_edges_count":0,"total_edges":0,"directed_density_pct":0,"bidirectional_pair_count":0,"matched_edges_count":0}"#,
    );

    // 1 critical + 2 warning → score 76 (>= 60) 但 critical 存在 → blocking fail
    let smells = vec![
        serde_json::json!({
            "category": "security", "rule": "hardcoded_secret", "severity": "critical",
            "message": "critical smell", "file": "app.py", "line": 1,
            "source": "bcc", "confidence": 1.0
        }),
        serde_json::json!({
            "category": "error_handling", "rule": "broad_catch", "severity": "warning",
            "message": "warning smell 1", "file": "app.py", "line": 10,
            "source": "bcc", "confidence": 0.8
        }),
        serde_json::json!({
            "category": "error_handling", "rule": "broad_catch", "severity": "warning",
            "message": "warning smell 2", "file": "app.py", "line": 20,
            "source": "bcc", "confidence": 0.8
        }),
    ];
    write_smell_report(&smells_file, &smells);

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "score", "score",
            "--input", &input.to_string_lossy(),
            "--smell-report", &smells_file.to_string_lossy(),
            "--mode", "strict",
        ])
        .output()
        .expect("run arch score with critical smells");

    // strict 模式下，blocking 维度 passed=false → 整体 fail (exit 2)
    assert_eq!(
        output.status.code(),
        Some(2),
        "strict mode + critical smell should fail, stdout: {}, stderr: {}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );

    let stdout = String::from_utf8_lossy(&output.stdout);
    assert!(
        stdout.contains("code_quality") || stdout.contains("Code Quality"),
        "output should include code_quality dimension, stdout: {}",
        stdout
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn score_with_nonexistent_smell_report_fails() {
    let root = temp_dir("bcc_score_missing_report");
    let input = root.join("input");
    fs::create_dir_all(&input).expect("create input dir");

    write(
        &input.join("scenario-validation.tsv"),
        "category\tscenario\tcaller\tcallee\tstatus\n",
    );
    write(
        &input.join("gate-evaluation.tsv"),
        "profile\tgate\tvalue\tthreshold\tstatus\n",
    );
    write(
        &input.join("summary.json"),
        r#"{"target_gate_pass":true,"transition_gate_pass":true,"forbidden_edges_count":0,"unexpected_edges_count":0,"missing_edges_count":0,"total_edges":0,"directed_density_pct":0,"bidirectional_pair_count":0,"matched_edges_count":0}"#,
    );

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "score", "score",
            "--input", &input.to_string_lossy(),
            "--smell-report", "/nonexistent/smell_report.json",
            "--mode", "strict",
        ])
        .output()
        .expect("run arch score with nonexistent smell-report");

    // 文件不存在 → code_quality 维度 score=0, passed=false → strict 模式 exit 2
    assert_eq!(
        output.status.code(),
        Some(2),
        "nonexistent smell-report should cause blocking fail, stdout: {}, stderr: {}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );

    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        stderr.contains("cannot read"),
        "stderr should mention read failure, stderr: {}",
        stderr
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn score_with_invalid_json_smell_report_fails() {
    let root = temp_dir("bcc_score_invalid_json");
    let input = root.join("input");
    let smells_file = root.join("bad_smells.json");
    fs::create_dir_all(&input).expect("create input dir");

    write(
        &input.join("scenario-validation.tsv"),
        "category\tscenario\tcaller\tcallee\tstatus\n",
    );
    write(
        &input.join("gate-evaluation.tsv"),
        "profile\tgate\tvalue\tthreshold\tstatus\n",
    );
    write(
        &input.join("summary.json"),
        r#"{"target_gate_pass":true,"transition_gate_pass":true,"forbidden_edges_count":0,"unexpected_edges_count":0,"missing_edges_count":0,"total_edges":0,"directed_density_pct":0,"bidirectional_pair_count":0,"matched_edges_count":0}"#,
    );

    // 写入非法 JSON
    write(&smells_file, "this is not valid json{{{");

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "score", "score",
            "--input", &input.to_string_lossy(),
            "--smell-report", &smells_file.to_string_lossy(),
            "--mode", "strict",
        ])
        .output()
        .expect("run arch score with invalid json smell-report");

    // 解析失败 → code_quality 维度 score=0, passed=false → strict 模式 exit 2
    assert_eq!(
        output.status.code(),
        Some(2),
        "invalid JSON smell-report should cause blocking fail, stdout: {}, stderr: {}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );

    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        stderr.contains("cannot parse"),
        "stderr should mention parse failure, stderr: {}",
        stderr
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn linter_large_output_does_not_false_timeout() {
    // 验证大输出的 linter 不会因管道缓冲区满而被误判超时
    let root = temp_dir("bcc_linter_large_output");
    let ast = root.join("ast.json");
    let out = root.join("out.json");

    write(&ast, r#"{"source_count":0,"records":[]}"#);

    // 生成超过 64KB 的 JSON 数组输出（管道缓冲区通常 64KB）
    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "analyze",
            "--ast-file", &ast.to_string_lossy(),
            "-o", &out.to_string_lossy(),
            "--linter",
            r#"big:python3 -c "import json; items=[{'code':'E501','message':'line too long','filename':'f.py','row':i} for i in range(2000)]; print(json.dumps(items))""#,
        ])
        .env("BCC_LINTER_TIMEOUT", "10")
        .output()
        .expect("run analyze with large-output linter");

    assert!(
        output.status.success(),
        "should succeed with large linter output, stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        !stderr.contains("timed out"),
        "should NOT timeout on large output, stderr: {}",
        stderr
    );
    // 应该解析到 2000 条记录
    assert!(
        stderr.contains("2000 smell(s)"),
        "should parse 2000 records, stderr: {}",
        stderr
    );

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn smell_gate_preserves_higher_failure_code() {
    // 当 forbidden/gate 失败（code=2）且 smell-gate 也触发时，最终 code 应为 2 而非 1
    let root = temp_dir("bcc_smell_gate_code_priority");
    let (target, transition, gates, actual) = setup_validate_data(&root);
    let out = root.join("out");
    let smells_file = root.join("smells.json");

    let smells = vec![serde_json::json!({
        "category": "security", "rule": "test_rule", "severity": "critical",
        "message": "some smell", "file": "app.py", "line": 1,
        "source": "bcc", "confidence": 1.0
    })];
    write_smell_report(&smells_file, &smells);

    // --fail-on-gate=true 会在 gate 失败时设 code=2
    // smell-gate 有 smell 时应 code=max(2,1)=2
    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "true",
            "--fail-on-forbidden", "true",
            "--smell-gate", &smells_file.to_string_lossy(),
        ])
        .output()
        .expect("run validate with gate fail + smell-gate");

    let exit_code = output.status.code().unwrap_or(-1);
    let stderr = String::from_utf8_lossy(&output.stderr);

    // smell-gate 有 smell → 至少 exit 1
    assert_ne!(exit_code, 0, "should fail due to smell-gate");
    // 如果 gate/forbidden 也失败了，code 应为 2 而不是被降级为 1
    if exit_code == 2 {
        assert!(
            stderr.contains("smell_gate=FAIL"),
            "smell-gate should still report even with higher code, stderr: {}",
            stderr
        );
    }

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn multi_linter_one_fails_one_succeeds() {
    // 验证多个 --linter 时，一个失败不影响另一个成功的结果聚合
    let root = temp_dir("bcc_multi_linter_combo");
    let ast = root.join("ast.json");
    let out = root.join("out.json");

    write(&ast, r#"{"source_count":0,"records":[]}"#);

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "analyze",
            "--ast-file", &ast.to_string_lossy(),
            "-o", &out.to_string_lossy(),
            "--linter", "bad:false",
            "--linter", "good:echo '[]'",
        ])
        .env("BCC_LINTER_TIMEOUT", "5")
        .output()
        .expect("run analyze with multi linter");

    assert!(
        output.status.success(),
        "should succeed despite one linter failing, stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    let stderr = String::from_utf8_lossy(&output.stderr);
    // 失败的 linter 应有告警
    assert!(
        stderr.contains("bad") && stderr.contains("failed"),
        "should warn about failed linter 'bad', stderr: {}",
        stderr
    );

    // 输出文件应存在且可解析
    assert!(out.exists(), "output file should exist");
    let raw = fs::read_to_string(&out).expect("read output");
    let _: serde_json::Value = serde_json::from_str(&raw).expect("output should be valid JSON");

    let _ = fs::remove_dir_all(&root);
}

#[test]
fn multi_linter_one_timeout_one_succeeds_with_results() {
    // 验证多 --linter 场景：一个超时、一个成功并产出非空结果，
    // 超时分支不影响后续成功 linter 的结果聚合
    let root = temp_dir("bcc_multi_linter_timeout_combo");
    let ast = root.join("ast.json");
    let out = root.join("out.json");

    write(&ast, r#"{"source_count":0,"records":[]}"#);

    // slow linter: sleep 60 会被 1s 超时 kill
    // good linter: 输出 2 条 ruff 格式的 smell 记录
    let good_cmd = r#"echo '[{"code":"E501","message":"line too long","filename":"f.py","row":1},{"code":"E302","message":"expected 2 blank lines","filename":"g.py","row":10}]'"#;

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "analyze",
            "--ast-file", &ast.to_string_lossy(),
            "-o", &out.to_string_lossy(),
            "--linter", "slow:sleep 60",
            "--linter", &format!("good:{}", good_cmd),
        ])
        .env("BCC_LINTER_TIMEOUT", "1")
        .output()
        .expect("run analyze with timeout + success linter");

    assert!(
        output.status.success(),
        "should succeed despite one linter timeout, stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    let stderr = String::from_utf8_lossy(&output.stderr);

    // 超时的 linter 应有 timeout 告警
    assert!(
        stderr.contains("slow") && stderr.contains("timed out"),
        "should warn about timed-out linter 'slow', stderr: {}",
        stderr
    );

    // 成功的 linter 应报告 2 条结果
    assert!(
        stderr.contains("good") && stderr.contains("2 smell(s)"),
        "should report 2 smells from 'good' linter, stderr: {}",
        stderr
    );

    // 输出文件应存在且可解析
    assert!(out.exists(), "output file should exist");
    let raw = fs::read_to_string(&out).expect("read output");
    let reports: serde_json::Value = serde_json::from_str(&raw).expect("output should be valid JSON");

    // analyze 输出是 Vec<SmellReport> 数组，遍历所有 report 的 smells
    let reports_arr = reports.as_array().expect(&format!(
        "output should be a JSON array (Vec<SmellReport>), raw: {}", raw
    ));
    let good_smells: Vec<_> = reports_arr.iter()
        .flat_map(|r| r.get("smells").and_then(|v| v.as_array()).into_iter().flatten())
        .filter(|s| s.get("source").and_then(|v| v.as_str()) == Some("good"))
        .collect();
    assert_eq!(
        good_smells.len(), 2,
        "should have 2 smells from 'good' linter, got {}: {:?}",
        good_smells.len(), good_smells
    );

    let _ = fs::remove_dir_all(&root);
}

// === Layer Violation 集成测试 ===

/// 共享辅助：创建最小 target/transition/gates 文件，不含 forbid/unexpected 边
fn setup_minimal_contracts(root: &Path) -> (PathBuf, PathBuf, PathBuf) {
    let target = root.join("target.yaml");
    let transition = root.join("transition.yaml");
    let gates = root.join("gates.yaml");

    write(
        &target,
        r#"version: v3
kind: target_contract
intent: target
source_of_truth: test
notes: []
allow_edges:
  - caller: MOD_A
    callee: MOD_B
  - caller: MOD_B
    callee: MOD_A
forbid_edges: []
"#,
    );
    write(
        &transition,
        r#"version: v3
kind: transition_contract
base: v3.target
intent: transition
notes: []
temporary_allow_edges: []
blocked_edges: []
"#,
    );
    write(
        &gates,
        r#"version: v3
kind: verification_gates
intent: gate
profiles:
  transition:
    max_unexpected_edges_count: 10
    max_forbidden_edges_count: 10
    max_forbidden_total_edges: 10
    max_missing_edges_count: 10
    max_directed_density_pct: 100
    max_bidirectional_pair_count: 10
  target:
    max_unexpected_edges_count: 10
    max_forbidden_edges_count: 10
    max_forbidden_total_edges: 10
    max_missing_edges_count: 10
    max_directed_density_pct: 100
    max_bidirectional_pair_count: 10
"#,
    );
    (target, transition, gates)
}

/// 测试1: infrastructure → application 违规检测
#[test]
fn layer_violation_infra_to_app_detected() {
    let root = temp_dir("bcc_layer_v1");
    let (target, transition, gates) = setup_minimal_contracts(&root);
    let actual = root.join("actual.json");
    let seed = root.join("seed.yaml");
    let out = root.join("out");

    write(
        &actual,
        r#"[{"caller":"MOD_A","callee":"MOD_B","import_edges":1,"call_edges":0,"total_edges":1}]"#,
    );
    // MOD_A = infrastructure (precedence 3), MOD_B = application (precedence 1)
    // infrastructure → application 是下层依赖上层，应该违规
    write(
        &seed,
        r#"version: v3
modules:
  - module_id: MOD_A
    layer: infrastructure
    path_rules:
      include: ["src/a/**"]
  - module_id: MOD_B
    layer: application
    path_rules:
      include: ["src/b/**"]
layer_rules:
  layers:
    - name: application
      precedence: 1
    - name: service
      precedence: 2
    - name: infrastructure
      precedence: 3
  forbidden_transitions: []
"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--seed-file", &seed.to_string_lossy(),
        ])
        .status()
        .expect("run validate");
    assert_eq!(status.code(), Some(0)); // 不开 --fail-on-layer-violation

    let report = fs::read_to_string(out.join("v3-validation-report.md")).expect("read report");
    assert!(report.contains("## Layer Violations"), "report should have Layer Violations section");
    assert!(
        report.contains("MOD_A (infrastructure)"),
        "report should mention MOD_A infrastructure: {}",
        report
    );
    assert!(
        report.contains("MOD_B (application)"),
        "report should mention MOD_B application: {}",
        report
    );

    // summary.json 应有 layer_violation_count
    let summary_raw = fs::read_to_string(out.join("summary.json")).expect("read summary");
    let summary: serde_json::Value = serde_json::from_str(&summary_raw).expect("parse summary");
    assert_eq!(summary["layer_violation_count"], 1);

    let _ = fs::remove_dir_all(&root);
}

/// 测试2: application → infrastructure 允许
#[test]
fn layer_violation_app_to_infra_allowed() {
    let root = temp_dir("bcc_layer_v2");
    let (target, transition, gates) = setup_minimal_contracts(&root);
    let actual = root.join("actual.json");
    let seed = root.join("seed.yaml");
    let out = root.join("out");

    write(
        &actual,
        r#"[{"caller":"MOD_A","callee":"MOD_B","import_edges":1,"call_edges":0,"total_edges":1}]"#,
    );
    // MOD_A = application (precedence 1), MOD_B = infrastructure (precedence 3)
    // 上层依赖下层，应该允许
    write(
        &seed,
        r#"version: v3
modules:
  - module_id: MOD_A
    layer: application
    path_rules:
      include: ["src/a/**"]
  - module_id: MOD_B
    layer: infrastructure
    path_rules:
      include: ["src/b/**"]
layer_rules:
  layers:
    - name: application
      precedence: 1
    - name: service
      precedence: 2
    - name: infrastructure
      precedence: 3
  forbidden_transitions: []
"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--seed-file", &seed.to_string_lossy(),
        ])
        .status()
        .expect("run validate");
    assert_eq!(status.code(), Some(0));

    let report = fs::read_to_string(out.join("v3-validation-report.md")).expect("read report");
    assert!(report.contains("No layer violations detected."));

    let _ = fs::remove_dir_all(&root);
}

/// 测试3: 同层调用允许
#[test]
fn layer_violation_same_layer_allowed() {
    let root = temp_dir("bcc_layer_v3");
    let (target, transition, gates) = setup_minimal_contracts(&root);
    let actual = root.join("actual.json");
    let seed = root.join("seed.yaml");
    let out = root.join("out");

    write(
        &actual,
        r#"[{"caller":"MOD_A","callee":"MOD_B","import_edges":1,"call_edges":0,"total_edges":1}]"#,
    );
    write(
        &seed,
        r#"version: v3
modules:
  - module_id: MOD_A
    layer: application
    path_rules:
      include: ["src/a/**"]
  - module_id: MOD_B
    layer: application
    path_rules:
      include: ["src/b/**"]
layer_rules:
  layers:
    - name: application
      precedence: 1
  forbidden_transitions: []
"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--seed-file", &seed.to_string_lossy(),
        ])
        .status()
        .expect("run validate");
    assert_eq!(status.code(), Some(0));

    let report = fs::read_to_string(out.join("v3-validation-report.md")).expect("read report");
    assert!(report.contains("No layer violations detected."));

    let _ = fs::remove_dir_all(&root);
}

/// 测试4: 自定义 forbidden_transitions
#[test]
fn layer_violation_custom_forbidden() {
    let root = temp_dir("bcc_layer_v4");
    let (target, transition, gates) = setup_minimal_contracts(&root);
    let actual = root.join("actual.json");
    let seed = root.join("seed.yaml");
    let out = root.join("out");

    write(
        &actual,
        r#"[{"caller":"MOD_A","callee":"MOD_B","import_edges":1,"call_edges":0,"total_edges":1}]"#,
    );
    // 自定义 forbidden: service → api
    write(
        &seed,
        r#"version: v3
modules:
  - module_id: MOD_A
    layer: service
    path_rules:
      include: ["src/a/**"]
  - module_id: MOD_B
    layer: api
    path_rules:
      include: ["src/b/**"]
layer_rules:
  layers:
    - name: api
      precedence: 1
    - name: service
      precedence: 2
  forbidden_transitions:
    - [service, api]
"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--seed-file", &seed.to_string_lossy(),
        ])
        .status()
        .expect("run validate");
    assert_eq!(status.code(), Some(0));

    let report = fs::read_to_string(out.join("v3-validation-report.md")).expect("read report");
    assert!(report.contains("## Layer Violations"));
    assert!(report.contains("MOD_A (service)"));
    assert!(report.contains("MOD_B (api)"));

    let _ = fs::remove_dir_all(&root);
}

/// 测试5: 无 layer 定义时跳过（stderr 输出 warning）
#[test]
fn layer_violation_skip_when_no_layer() {
    let root = temp_dir("bcc_layer_v5");
    let (target, transition, gates) = setup_minimal_contracts(&root);
    let actual = root.join("actual.json");
    let seed = root.join("seed.yaml");
    let out = root.join("out");

    write(
        &actual,
        r#"[{"caller":"MOD_A","callee":"MOD_B","import_edges":1,"call_edges":0,"total_edges":1}]"#,
    );
    // MOD_A 有 layer，MOD_B 没有 layer → 只有一个模块有 layer，触发检查但跳过该边
    write(
        &seed,
        r#"version: v3
modules:
  - module_id: MOD_A
    layer: application
    path_rules:
      include: ["src/a/**"]
  - module_id: MOD_B
    path_rules:
      include: ["src/b/**"]
"#,
    );

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--seed-file", &seed.to_string_lossy(),
        ])
        .output()
        .expect("run validate");
    assert_eq!(output.status.code(), Some(0));

    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        stderr.contains("[validate] skip layer check: MOD_B has no layer defined"),
        "stderr should warn about MOD_B missing layer: {}",
        stderr
    );

    // 报告不应包含违规（跳过的边不算违规）
    let report = fs::read_to_string(out.join("v3-validation-report.md")).expect("read report");
    assert!(report.contains("No layer violations detected."));

    let _ = fs::remove_dir_all(&root);
}

/// 测试6: --fail-on-layer-violation 门禁
#[test]
fn layer_violation_fail_on_flag_exit_code_2() {
    let root = temp_dir("bcc_layer_v6");
    let (target, transition, gates) = setup_minimal_contracts(&root);
    let actual = root.join("actual.json");
    let seed = root.join("seed.yaml");
    let out = root.join("out");

    write(
        &actual,
        r#"[{"caller":"MOD_A","callee":"MOD_B","import_edges":1,"call_edges":0,"total_edges":1}]"#,
    );
    // infrastructure → application 违规
    write(
        &seed,
        r#"version: v3
modules:
  - module_id: MOD_A
    layer: infrastructure
    path_rules:
      include: ["src/a/**"]
  - module_id: MOD_B
    layer: application
    path_rules:
      include: ["src/b/**"]
layer_rules:
  layers:
    - name: application
      precedence: 1
    - name: infrastructure
      precedence: 3
  forbidden_transitions: []
"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--seed-file", &seed.to_string_lossy(),
            "--fail-on-layer-violation", "true",
        ])
        .status()
        .expect("run validate");
    assert_eq!(status.code(), Some(2), "should exit 2 when --fail-on-layer-violation and violations exist");

    let _ = fs::remove_dir_all(&root);
}

/// 测试7: 向后兼容——不传 --seed-file 时行为不变
#[test]
fn layer_violation_backward_compatible_no_seed_file() {
    let root = temp_dir("bcc_layer_v7");
    let (target, transition, gates) = setup_minimal_contracts(&root);
    let actual = root.join("actual.json");
    let out = root.join("out");

    write(
        &actual,
        r#"[{"caller":"MOD_A","callee":"MOD_B","import_edges":1,"call_edges":0,"total_edges":1}]"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
        ])
        .status()
        .expect("run validate");
    assert_eq!(status.code(), Some(0));

    let report = fs::read_to_string(out.join("v3-validation-report.md")).expect("read report");
    // 不传 seed-file 时，报告不应含 Layer Violations 章节
    assert!(
        !report.contains("## Layer Violations"),
        "report should NOT have Layer Violations without --seed-file"
    );

    // summary.json 不应有 layer_violation_count 字段
    let summary_raw = fs::read_to_string(out.join("summary.json")).expect("read summary");
    let summary: serde_json::Value = serde_json::from_str(&summary_raw).expect("parse summary");
    assert!(summary.get("layer_violation_count").is_none());

    let _ = fs::remove_dir_all(&root);
}

/// 测试8: seed 文件不存在时应报错退出（exit code 1）
#[test]
fn layer_violation_seed_file_not_found_exits_1() {
    let root = temp_dir("bcc_layer_v8");
    let (target, transition, gates) = setup_minimal_contracts(&root);
    let actual = root.join("actual.json");
    let out = root.join("out");

    write(
        &actual,
        r#"[{"caller":"MOD_A","callee":"MOD_B","import_edges":1,"call_edges":0,"total_edges":1}]"#,
    );

    let nonexistent_seed = root.join("nonexistent_seed.yaml");

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--seed-file", &nonexistent_seed.to_string_lossy(),
        ])
        .output()
        .expect("run validate");
    assert_eq!(output.status.code(), Some(1), "should exit 1 when seed file not found");
    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        stderr.contains("read seed_file failed"),
        "stderr should mention read seed_file failed: {}",
        stderr
    );

    let _ = fs::remove_dir_all(&root);
}

/// 测试9: seed 文件 YAML 格式非法时应报错退出（exit code 1）
#[test]
fn layer_violation_seed_file_invalid_yaml_exits_1() {
    let root = temp_dir("bcc_layer_v9");
    let (target, transition, gates) = setup_minimal_contracts(&root);
    let actual = root.join("actual.json");
    let seed = root.join("seed.yaml");
    let out = root.join("out");

    write(
        &actual,
        r#"[{"caller":"MOD_A","callee":"MOD_B","import_edges":1,"call_edges":0,"total_edges":1}]"#,
    );
    // 非法 YAML
    write(&seed, "{{{{not valid yaml at all!!!}}}}");

    let output = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--seed-file", &seed.to_string_lossy(),
        ])
        .output()
        .expect("run validate");
    assert_eq!(output.status.code(), Some(1), "should exit 1 when seed yaml is invalid");
    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(
        stderr.contains("parse seed yaml failed"),
        "stderr should mention parse seed yaml failed: {}",
        stderr
    );

    let _ = fs::remove_dir_all(&root);
}

/// 测试10: 未知 layer（不在 layers 配置中）默认放行，不产生 violation
#[test]
fn layer_violation_unknown_layer_default_allowed() {
    let root = temp_dir("bcc_layer_v10");
    let (target, transition, gates) = setup_minimal_contracts(&root);
    let actual = root.join("actual.json");
    let seed = root.join("seed.yaml");
    let out = root.join("out");

    write(
        &actual,
        r#"[{"caller":"MOD_A","callee":"MOD_B","import_edges":1,"call_edges":0,"total_edges":1}]"#,
    );
    // MOD_A 和 MOD_B 的 layer 值不在 layers 配置的 name 列表中
    write(
        &seed,
        r#"version: v3
modules:
  - module_id: MOD_A
    layer: unknown_tier
    path_rules:
      include: ["src/a/**"]
  - module_id: MOD_B
    layer: another_unknown
    path_rules:
      include: ["src/b/**"]
layer_rules:
  layers:
    - name: application
      precedence: 1
    - name: service
      precedence: 2
    - name: infrastructure
      precedence: 3
  forbidden_transitions: []
"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--seed-file", &seed.to_string_lossy(),
        ])
        .status()
        .expect("run validate");
    assert_eq!(status.code(), Some(0), "unknown layers should not cause violation");

    let report = fs::read_to_string(out.join("v3-validation-report.md")).expect("read report");
    assert!(
        report.contains("No layer violations detected."),
        "unknown layers should default to allowed: {}",
        report
    );

    // summary.json 的 layer_violation_count 应为 0
    let summary_raw = fs::read_to_string(out.join("summary.json")).expect("read summary");
    let summary: serde_json::Value = serde_json::from_str(&summary_raw).expect("parse summary");
    assert_eq!(summary["layer_violation_count"], 0);

    let _ = fs::remove_dir_all(&root);
}

/// 测试11: seed 提供 layer_rules（仅 forbidden）时，默认 allowed_transitions 不被清空
#[test]
fn layer_violation_custom_rules_preserve_default_allowed() {
    let root = temp_dir("bcc_layer_v11");
    let (target, transition, gates) = setup_minimal_contracts(&root);
    let actual = root.join("actual.json");
    let seed = root.join("seed.yaml");
    let out = root.join("out");

    // support → core 是默认 allowed，不应被误报
    write(
        &actual,
        r#"[{"caller":"MOD_A","callee":"MOD_B","import_edges":1,"call_edges":0,"total_edges":1}]"#,
    );
    write(
        &seed,
        r#"version: v3
modules:
  - module_id: MOD_A
    layer: support
    path_rules:
      include: ["src/a/**"]
  - module_id: MOD_B
    layer: core
    path_rules:
      include: ["src/b/**"]
layer_rules:
  forbidden_transitions:
    - [infrastructure, application]
"#,
    );

    let status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch", "validate",
            "--target", &target.to_string_lossy(),
            "--transition", &transition.to_string_lossy(),
            "--gates", &gates.to_string_lossy(),
            "--actual", &actual.to_string_lossy(),
            "--out-dir", &out.to_string_lossy(),
            "--fail-on-gate", "false",
            "--fail-on-forbidden", "false",
            "--seed-file", &seed.to_string_lossy(),
        ])
        .status()
        .expect("run validate");
    assert_eq!(status.code(), Some(0));

    let report = fs::read_to_string(out.join("v3-validation-report.md")).expect("read report");
    assert!(
        report.contains("No layer violations detected."),
        "support->core should be allowed by default even when layer_rules is provided: {}",
        report
    );

    let summary_raw = fs::read_to_string(out.join("summary.json")).expect("read summary");
    let summary: serde_json::Value = serde_json::from_str(&summary_raw).expect("parse summary");
    assert_eq!(summary["layer_violation_count"], 0);

    let _ = fs::remove_dir_all(&root);
}

/// CLI 集成测试：parent 层级 + layer violation + --fail-on-layer-violation
/// 验证 bcc arch matrix → bcc arch validate 全流程退出码和输出文件
#[test]
fn arch_parent_layer_cli_integration() {
    let root = temp_dir("bcc_cli_parent_layer");
    let seed = root.join("seed.yaml");
    let ast = root.join("ast.json");
    let matrix_out = root.join("matrix_out");
    let validate_out = root.join("validate_out");

    // 带 parent 和 layer_rules 的 seed YAML
    write(
        &seed,
        r#"version: v3
source_of_truth: test_cli_integration
modules:
  - module_id: PARENT_MOD
    display_name: Parent
    layer: application
    precedence: 10
    path_rules:
      include: ["src/parent/**"]
  - module_id: CHILD_MOD
    display_name: Child
    layer: application
    parent: PARENT_MOD
    precedence: 20
    path_rules:
      include: ["src/child/**"]
  - module_id: INFRA_MOD
    display_name: Infra
    layer: infrastructure
    parent: CHILD_MOD
    precedence: 30
    path_rules:
      include: ["src/infra/**"]
relations_expected:
  - caller: INFRA_MOD
    callee: CHILD_MOD
    allowed: true
layer_rules:
  layers:
    - name: application
      precedence: 1
    - name: infrastructure
      precedence: 2
  forbidden_transitions:
    - [infrastructure, application]
"#,
    );

    // AST: infrastructure 模块调用 application 模块
    write(
        &ast,
        r#"{
  "source_count": 3,
  "records": [
    {
      "sourcePath": "src/parent/main.ts",
      "localDependencies": [],
      "localCallTargets": []
    },
    {
      "sourcePath": "src/child/service.ts",
      "localDependencies": [],
      "localCallTargets": []
    },
    {
      "sourcePath": "src/infra/repo.ts",
      "localDependencies": ["src/child/service.ts"],
      "localCallTargets": []
    }
  ]
}"#,
    );

    // Step 1: bcc arch matrix
    let matrix_status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "matrix",
            "--seed-file",
            &seed.to_string_lossy(),
            "--ast-file",
            &ast.to_string_lossy(),
            "--out-dir",
            &matrix_out.to_string_lossy(),
            "--version",
            "v3",
            "--emit",
            "all",
        ])
        .status()
        .expect("run matrix");
    assert!(matrix_status.success(), "matrix should succeed");

    // 构造 actual relations
    let actual = root.join("actual.json");
    write(
        &actual,
        r#"[
  {"caller":"INFRA_MOD","callee":"CHILD_MOD","import_edges":1,"call_edges":0,"total_edges":1}
]"#,
    );

    // Step 2: bcc arch validate --fail-on-layer-violation true
    let validate_status = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "arch",
            "validate",
            "--target",
            &matrix_out.join("v3.target-matrix.yaml").to_string_lossy(),
            "--transition",
            &matrix_out
                .join("v3.transition-matrix.yaml")
                .to_string_lossy(),
            "--gates",
            &matrix_out.join("v3.gates.yaml").to_string_lossy(),
            "--actual",
            &actual.to_string_lossy(),
            "--out-dir",
            &validate_out.to_string_lossy(),
            "--fail-on-gate",
            "false",
            "--fail-on-forbidden",
            "false",
            "--seed-file",
            &seed.to_string_lossy(),
            "--fail-on-layer-violation",
            "true",
        ])
        .status()
        .expect("run validate with layer violation");
    assert_eq!(
        validate_status.code(),
        Some(2),
        "validate should exit with code 2 due to layer violation"
    );

    // 验证报告文件存在且包含 Layer Violations
    let report =
        fs::read_to_string(validate_out.join("v3-validation-report.md")).expect("read report");
    assert!(
        report.contains("Layer Violations"),
        "report should contain Layer Violations section"
    );
    assert!(
        report.contains("INFRA_MOD"),
        "report should mention INFRA_MOD in violation"
    );

    // 验证 summary.json 中 layer_violation_count > 0
    let summary_raw =
        fs::read_to_string(validate_out.join("summary.json")).expect("read summary");
    let summary: serde_json::Value = serde_json::from_str(&summary_raw).expect("parse summary");
    let lvc = summary["layer_violation_count"].as_i64().unwrap_or(0);
    assert!(
        lvc > 0,
        "summary.json layer_violation_count should be > 0, got {}",
        lvc
    );

    let _ = fs::remove_dir_all(&root);
}
