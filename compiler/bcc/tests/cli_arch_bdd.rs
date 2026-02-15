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
    assert!(forbidden_yaml.exists(), "X_Y.yaml should exist for forbidden edge");
    let content = fs::read_to_string(&forbidden_yaml).expect("read X_Y.yaml");
    assert!(content.contains("module: X"));
    assert!(content.contains("edge_class: blocked"));
    assert!(content.contains("contract: X -> Y arch violation"));
    assert!(content.contains("source_file: arch-validate-export"));
    assert!(content.contains("source_summary:"));

    // unexpected 边 B->A 生成 YAML（edge_class: temporary）
    let unexpected_yaml = bdd_dir.join("B_A.yaml");
    assert!(unexpected_yaml.exists(), "B_A.yaml should exist for unexpected edge");
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
        .filter(|e| {
            e.path()
                .extension()
                .map(|x| x == "yaml")
                .unwrap_or(false)
        })
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
        forbid_edges.push_str(&format!(
            "  - caller: M{}\n    callee: N{}\n",
            i, i
        ));
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
    write(
        &actual,
        &format!("[\n{}\n]", actual_entries.join(",\n")),
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
        .expect("run validate full output");
    assert!(status.success());

    // 验证输出 YAML 文件数 > 10（truncate 已移除）
    let yaml_count = fs::read_dir(&bdd_dir)
        .expect("read bdd_dir")
        .filter_map(Result::ok)
        .filter(|e| {
            e.path()
                .extension()
                .map(|x| x == "yaml")
                .unwrap_or(false)
        })
        .count();
    assert!(
        yaml_count > 10,
        "expected > 10 yaml files, got {}",
        yaml_count
    );

    // 验证 markdown 报告中包含 "showing top 20 of" 标注
    let report = fs::read_to_string(out.join("v3-validation-report.md"))
        .expect("read report");
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
        .filter(|e| {
            e.path()
                .extension()
                .map(|x| x == "yaml")
                .unwrap_or(false)
        })
        .count();
    assert!(yaml_count >= 2, "expected >= 2 yaml files, got {}", yaml_count);

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
    assert!(bdd_output.join("contexts").exists(), "contexts dir should exist");
    assert!(bdd_output.join("scenarios").exists(), "scenarios dir should exist");
    assert!(bdd_output.join("features").exists(), "features dir should exist");
    assert!(bdd_output.join("coverage.md").exists(), "coverage.md should exist");

    // 验证 context JSON 被正确生成（X_Y.yaml → module=X, B_A.yaml → module=B）
    let contexts_dir = bdd_output.join("contexts");
    let context_files: Vec<_> = fs::read_dir(&contexts_dir)
        .expect("read contexts")
        .filter_map(Result::ok)
        .filter(|e| {
            e.path()
                .extension()
                .map(|x| x == "json")
                .unwrap_or(false)
        })
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
    assert!(found_module_x, "should have a context with module=X (from X_Y.yaml)");
    assert!(found_module_b, "should have a context with module=B (from B_A.yaml)");

    // 验证 coverage.md 包含模块名
    let coverage = fs::read_to_string(bdd_output.join("coverage.md")).expect("read coverage");
    assert!(coverage.contains("| X |") || coverage.contains("| x |"),
        "coverage should mention module X");
    assert!(coverage.contains("| B |") || coverage.contains("| b |"),
        "coverage should mention module B");

    // 验证 features 目录下生成了 DSL 文件
    let feature_files: Vec<_> = fs::read_dir(bdd_output.join("features"))
        .expect("read features")
        .filter_map(Result::ok)
        .filter(|e| {
            e.path()
                .extension()
                .map(|x| x == "dsl")
                .unwrap_or(false)
        })
        .collect();
    assert!(
        feature_files.len() >= 2,
        "expected >= 2 feature dsl files, got {}",
        feature_files.len()
    );

    let _ = fs::remove_dir_all(&root);
}
