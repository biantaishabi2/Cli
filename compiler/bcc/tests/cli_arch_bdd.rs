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

    let bugfix = Command::new(env!("CARGO_BIN_EXE_bcc"))
        .args([
            "bugfix",
            &repo.to_string_lossy(),
            "-o",
            &out.to_string_lossy(),
            "--lang",
            "php",
            "-s",
            "x",
            "--limit",
            "20",
            "--force",
        ])
        .status()
        .expect("run bugfix context");
    assert!(bugfix.success());
    assert!(out.join("inventory.json").exists());
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
