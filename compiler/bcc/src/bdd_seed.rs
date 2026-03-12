use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, HashMap};
use std::fs;
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Deserialize, Serialize, PartialEq, Eq)]
struct BddAuthContext {
    strategy: String,
    #[serde(default)]
    actor: BTreeMap<String, String>,
}

#[derive(Debug, Deserialize, Serialize)]
struct SeedContext {
    id: String,
    module: String,
    edge_class: String,
    source_file: String,
    #[serde(default)]
    source_summary: String,
    #[serde(default)]
    auth_context: Option<BddAuthContext>,
}

fn sanitize_id(s: &str) -> String {
    s.chars()
        .map(|c| {
            if c.is_ascii_alphanumeric() {
                c.to_ascii_lowercase()
            } else {
                '_'
            }
        })
        .collect::<String>()
        .trim_matches('_')
        .to_string()
}

#[derive(Debug, Default)]
struct SourceScanResult {
    entries: Vec<PathBuf>,
    skipped_non_yaml: usize,
}

fn read_source_entries(source: &Path) -> Result<SourceScanResult, String> {
    let mut out = Vec::new();
    let mut skipped_non_yaml = 0usize;
    let mut stack = vec![source.to_path_buf()];
    while let Some(dir) = stack.pop() {
        let rd =
            fs::read_dir(&dir).map_err(|e| format!("read_dir {} failed: {}", dir.display(), e))?;
        for ent in rd {
            let ent = ent.map_err(|e| format!("read_dir entry failed: {}", e))?;
            let p = ent.path();
            if p.is_dir() {
                stack.push(p);
                continue;
            }
            let Some(ext) = p.extension().and_then(|x| x.to_str()) else {
                skipped_non_yaml += 1;
                continue;
            };
            if ["yaml", "yml"].contains(&ext) {
                out.push(p);
            } else {
                skipped_non_yaml += 1;
            }
        }
    }
    out.sort();
    Ok(SourceScanResult {
        entries: out,
        skipped_non_yaml,
    })
}

fn extract_module_from_source(path: &Path, content: &str) -> String {
    if let Ok(v) = serde_yaml::from_str::<serde_yaml::Value>(content) {
        if let Some(m) = v.get("module").and_then(|x| x.as_str()) {
            return m.to_string();
        }
        if let Some(m) = v.get("module_id").and_then(|x| x.as_str()) {
            return m.to_string();
        }
    }

    path.parent()
        .and_then(|p| p.file_name())
        .and_then(|x| x.to_str())
        .map(|s| s.to_string())
        .unwrap_or_else(|| "unknown_module".to_string())
}

fn extract_auth_context_from_yaml(yaml_val: &serde_yaml::Value) -> Option<BddAuthContext> {
    yaml_val
        .get("auth_context")
        .cloned()
        .and_then(|value| serde_yaml::from_value::<BddAuthContext>(value).ok())
}

fn ensure_dir(path: &Path) -> Result<(), String> {
    fs::create_dir_all(path).map_err(|e| format!("create dir {} failed: {}", path.display(), e))
}

fn write_text(path: &Path, content: &str, force: bool) -> Result<(), String> {
    if path.exists() && !force {
        return Ok(());
    }
    if let Some(parent) = path.parent() {
        ensure_dir(parent)?;
    }
    fs::write(path, content).map_err(|e| format!("write {} failed: {}", path.display(), e))
}

fn load_existing_contexts(context_dir: &Path) -> HashMap<String, SeedContext> {
    let mut map = HashMap::new();
    let Ok(rd) = fs::read_dir(context_dir) else {
        return map;
    };
    for ent in rd.flatten() {
        let p = ent.path();
        if p.extension().and_then(|x| x.to_str()) != Some("json") {
            continue;
        }
        let Ok(raw) = fs::read_to_string(&p) else {
            continue;
        };
        let Ok(ctx) = serde_json::from_str::<SeedContext>(&raw) else {
            continue;
        };
        map.insert(ctx.id.clone(), ctx);
    }
    map
}

#[derive(Debug, Serialize)]
struct QualityIssue {
    id: String,
    file: String,
    reasons: Vec<String>,
}

#[derive(Debug, Serialize)]
struct QualityReport {
    total_files: usize,
    invalid_files: usize,
    invalid: Vec<QualityIssue>,
}

fn run_context(
    source: &str,
    output: &str,
    module_filter: Option<&str>,
    edge_class: &str,
    limit: Option<usize>,
    force: bool,
) -> Result<Vec<SeedContext>, String> {
    let source_path = Path::new(source);
    if !source_path.exists() {
        return Err(format!("source path not found: {}", source));
    }

    let out_dir = Path::new(output);
    let context_dir = out_dir.join("contexts");
    ensure_dir(&context_dir)?;

    let module_set: Option<std::collections::HashSet<String>> = module_filter.map(|m| {
        m.split(',')
            .map(|x| x.trim().to_string())
            .filter(|x| !x.is_empty())
            .collect::<std::collections::HashSet<_>>()
    });

    let mut contexts = Vec::new();
    let scan = read_source_entries(source_path)?;
    if scan.entries.is_empty() {
        return Err(format!(
            "no YAML source files found under: {}",
            source_path.display()
        ));
    }
    // 显式提示被忽略的非 YAML 文件数量，便于排查输入目录污染。
    if scan.skipped_non_yaml > 0 {
        eprintln!(
            "[bdd seed] skipped_non_yaml_count={} (source={})",
            scan.skipped_non_yaml,
            source_path.display()
        );
    }
    let mut files = scan.entries;
    if let Some(max) = limit {
        files.truncate(max);
    }

    for file in files {
        let raw = fs::read_to_string(&file)
            .map_err(|e| format!("read source {} failed: {}", file.display(), e))?;

        let module = extract_module_from_source(&file, &raw);
        if let Some(ref set) = module_set {
            if !set.contains(&module) {
                continue;
            }
        }

        let rel = file
            .strip_prefix(source_path)
            .unwrap_or(&file)
            .to_string_lossy()
            .to_string();
        let stem = file.file_stem().and_then(|x| x.to_str()).unwrap_or("seed");
        let id = sanitize_id(&format!("{}_{}", module, stem));

        // 从 YAML 中提取结构化字段，避免截断污染
        let yaml_val: serde_yaml::Value = serde_yaml::from_str(&raw)
            .unwrap_or_else(|_| serde_yaml::Value::Mapping(Default::default()));
        let summary = yaml_val
            .get("source_summary")
            .and_then(|x| x.as_str())
            .unwrap_or("")
            .to_string();
        let auth_context = extract_auth_context_from_yaml(&yaml_val);
        let file_edge_class = yaml_val
            .get("edge_class")
            .and_then(|x| x.as_str())
            .unwrap_or(edge_class)
            .to_string();

        let ctx = SeedContext {
            id: id.clone(),
            module,
            edge_class: file_edge_class,
            source_file: rel,
            source_summary: summary,
            auth_context,
        };

        let out_file = context_dir.join(format!("{}.json", id));
        write_text(
            &out_file,
            &format!(
                "{}\n",
                serde_json::to_string_pretty(&ctx).map_err(|e| e.to_string())?
            ),
            force,
        )?;
        contexts.push(ctx);
    }

    Ok(contexts)
}

fn run_generate(output: &str, prompt_template: Option<&str>, force: bool) -> Result<(), String> {
    let out_dir = Path::new(output);
    let context_dir = out_dir.join("contexts");
    let scenarios_dir = out_dir.join("scenarios");
    ensure_dir(&scenarios_dir)?;

    let contexts = load_existing_contexts(&context_dir);
    if contexts.is_empty() {
        return Err("no contexts found; run context step first".to_string());
    }

    let template = if let Some(path) = prompt_template {
        fs::read_to_string(path).unwrap_or_else(|_| {
            "[SCENARIO: BDD-{MODULE}-SEED-{ID}] TITLE: {TITLE} TAGS: seed {EDGE_CLASS}\n{GIVEN_SEED_CONTEXT_LINE}\nWHEN when_execute_seed_contract module=\"{MODULE}\"\nTHEN then_seed_contract_should_hold module=\"{MODULE}\"\n".to_string()
        })
    } else {
        "[SCENARIO: BDD-{MODULE}-SEED-{ID}] TITLE: {TITLE} TAGS: seed {EDGE_CLASS}\n{GIVEN_SEED_CONTEXT_LINE}\nWHEN when_execute_seed_contract module=\"{MODULE}\"\nTHEN then_seed_contract_should_hold module=\"{MODULE}\"\n".to_string()
    };

    for ctx in contexts.values() {
        let dsl = render_scenario_template(&template, ctx);
        let out_file = scenarios_dir.join(format!("{}.dsl", ctx.id));
        write_text(&out_file, &dsl, force)?;
    }

    Ok(())
}

fn render_scenario_template(template: &str, ctx: &SeedContext) -> String {
    let given_line = render_given_seed_context_line(ctx);
    let module = ctx.module.to_ascii_uppercase();
    let has_given_placeholder = template.contains("{GIVEN_SEED_CONTEXT_LINE}");
    let rendered = template
        .replace("{GIVEN_SEED_CONTEXT_LINE}", &given_line)
        .replace("{ID}", &ctx.id)
        .replace("{MODULE}", &module)
        .replace("{EDGE_CLASS}", &ctx.edge_class)
        .replace("{TITLE}", &format!("{} contract seed scenario", ctx.module));
    let legacy_given_line = format!(
        "GIVEN given_seed_context id=\"{}\" module=\"{}\"",
        ctx.id, module
    );

    if !has_given_placeholder && rendered.contains(&legacy_given_line) {
        rendered.replace(&legacy_given_line, &given_line)
    } else {
        rendered
    }
}

fn render_given_seed_context_line(ctx: &SeedContext) -> String {
    let parts = vec![
        format!("id=\"{}\"", escape_dsl_value(&ctx.id)),
        format!(
            "module=\"{}\"",
            escape_dsl_value(&ctx.module.to_ascii_uppercase())
        ),
    ];

    format!("GIVEN given_seed_context {}", parts.join(" "))
}

fn escape_dsl_value(value: &str) -> String {
    value.replace('\\', "\\\\").replace('"', "\\\"")
}

fn scenario_quality_reasons(raw: &str) -> Vec<String> {
    let mut reasons = Vec::new();
    if !raw.contains("[SCENARIO:") {
        reasons.push("missing [SCENARIO:] header".to_string());
    }
    if !raw.lines().any(|l| l.starts_with("GIVEN ")) {
        reasons.push("missing GIVEN step".to_string());
    }
    if !raw.lines().any(|l| l.starts_with("WHEN ")) {
        reasons.push("missing WHEN step".to_string());
    }
    if !raw.lines().any(|l| l.starts_with("THEN ")) {
        reasons.push("missing THEN step".to_string());
    }
    reasons
}

fn run_check(output: &str) -> Result<QualityReport, String> {
    let out_dir = Path::new(output);
    let scenarios_dir = out_dir.join("scenarios");
    let rd = fs::read_dir(&scenarios_dir).map_err(|e| {
        format!(
            "read scenarios dir {} failed: {}",
            scenarios_dir.display(),
            e
        )
    })?;

    let mut total_files = 0usize;
    let mut invalid: Vec<QualityIssue> = Vec::new();

    for ent in rd {
        let ent = ent.map_err(|e| format!("scenario entry read failed: {}", e))?;
        let p = ent.path();
        if p.extension().and_then(|x| x.to_str()) != Some("dsl") {
            continue;
        }
        total_files += 1;
        let raw = fs::read_to_string(&p)
            .map_err(|e| format!("read scenario {} failed: {}", p.display(), e))?;
        let reasons = scenario_quality_reasons(&raw);
        if !reasons.is_empty() {
            invalid.push(QualityIssue {
                id: p
                    .file_stem()
                    .and_then(|x| x.to_str())
                    .unwrap_or("")
                    .to_string(),
                file: p.to_string_lossy().to_string(),
                reasons,
            });
        }
    }

    let report = QualityReport {
        total_files,
        invalid_files: invalid.len(),
        invalid,
    };

    let report_path = out_dir.join("quality-check.json");
    write_text(
        &report_path,
        &(serde_json::to_string_pretty(&report).map_err(|e| e.to_string())? + "\n"),
        true,
    )?;
    Ok(report)
}

fn run_fix(output: &str, prompt_template: Option<&str>, force: bool) -> Result<(), String> {
    let out_dir = Path::new(output);
    let contexts = load_existing_contexts(&out_dir.join("contexts"));
    if contexts.is_empty() {
        return Err("no contexts found; run context step first".to_string());
    }

    let template = if let Some(path) = prompt_template {
        fs::read_to_string(path).unwrap_or_else(|_| {
            "[SCENARIO: BDD-{MODULE}-SEED-{ID}] TITLE: {TITLE} TAGS: seed {EDGE_CLASS}\n{GIVEN_SEED_CONTEXT_LINE}\nWHEN when_execute_seed_contract module=\"{MODULE}\"\nTHEN then_seed_contract_should_hold module=\"{MODULE}\"\n".to_string()
        })
    } else {
        "[SCENARIO: BDD-{MODULE}-SEED-{ID}] TITLE: {TITLE} TAGS: seed {EDGE_CLASS}\n{GIVEN_SEED_CONTEXT_LINE}\nWHEN when_execute_seed_contract module=\"{MODULE}\"\nTHEN then_seed_contract_should_hold module=\"{MODULE}\"\n".to_string()
    };

    let first_check = run_check(output)?;
    let mut fixed = 0usize;
    for issue in &first_check.invalid {
        let Some(ctx) = contexts.get(&issue.id) else {
            continue;
        };
        let dsl = render_scenario_template(&template, ctx);
        let out_file = out_dir.join("scenarios").join(format!("{}.dsl", issue.id));
        write_text(&out_file, &dsl, force)?;
        fixed += 1;
    }

    run_organize(output, None, true)?;
    let second_check = run_check(output)?;

    let fix_summary = serde_json::json!({
        "fixed_files": fixed,
        "remaining_invalid_files": second_check.invalid_files,
    });
    write_text(
        &out_dir.join("quality-fix.json"),
        &(serde_json::to_string_pretty(&fix_summary).map_err(|e| e.to_string())? + "\n"),
        true,
    )?;

    if second_check.invalid_files > 0 {
        return Err(format!(
            "quality fix incomplete: remaining_invalid_files={}",
            second_check.invalid_files
        ));
    }
    Ok(())
}

fn run_organize(output: &str, coverage_report: Option<&str>, force: bool) -> Result<(), String> {
    let out_dir = Path::new(output);
    let scenarios_dir = out_dir.join("scenarios");
    let features_dir = out_dir.join("features");
    ensure_dir(&features_dir)?;

    let contexts = load_existing_contexts(&out_dir.join("contexts"));

    let rd = fs::read_dir(&scenarios_dir).map_err(|e| {
        format!(
            "read scenarios dir {} failed: {}",
            scenarios_dir.display(),
            e
        )
    })?;

    let mut by_module: BTreeMap<String, Vec<(String, String)>> = BTreeMap::new();
    for ent in rd {
        let ent = ent.map_err(|e| format!("scenario entry read failed: {}", e))?;
        let p = ent.path();
        if p.extension().and_then(|x| x.to_str()) != Some("dsl") {
            continue;
        }
        let stem = p
            .file_stem()
            .and_then(|x| x.to_str())
            .unwrap_or("")
            .to_string();
        let raw = fs::read_to_string(&p)
            .map_err(|e| format!("read scenario {} failed: {}", p.display(), e))?;

        let module = contexts
            .get(&stem)
            .map(|c| c.module.clone())
            .unwrap_or_else(|| "unknown_module".to_string());

        by_module.entry(module).or_default().push((stem, raw));
    }

    let mut total_scenarios = 0usize;
    let mut lines = vec![
        "# BDD Seed Coverage".to_string(),
        "".to_string(),
        "| 模块 | 场景文件数 | 场景条数 |".to_string(),
        "|---|---:|---:|".to_string(),
    ];

    for (module, items) in &by_module {
        let mut merged = String::new();
        merged.push_str("# Generated by: bcc bdd seed --step organize\n\n");

        let mut scenario_count = 0usize;
        for (_id, dsl) in items {
            merged.push_str(dsl);
            if !dsl.ends_with('\n') {
                merged.push('\n');
            }
            merged.push('\n');
            scenario_count += dsl.matches("[SCENARIO:").count();
        }

        total_scenarios += scenario_count;
        lines.push(format!(
            "| {} | {} | {} |",
            module,
            items.len(),
            scenario_count
        ));

        let out_file = features_dir.join(format!("{}.dsl", module));
        write_text(&out_file, &merged, force)?;
    }

    lines.push(format!(
        "| **合计** | **{}** | **{}** |",
        by_module.values().map(|v| v.len()).sum::<usize>(),
        total_scenarios
    ));
    let report_path = coverage_report
        .map(PathBuf::from)
        .unwrap_or_else(|| out_dir.join("coverage.md"));
    write_text(&report_path, &(lines.join("\n") + "\n"), true)?;

    Ok(())
}

pub fn run(
    source: &str,
    output: &str,
    step: &str,
    module_filter: Option<&str>,
    edge_class: &str,
    limit: Option<usize>,
    prompt_template: Option<&str>,
    coverage_report: Option<&str>,
    force: bool,
) {
    let result = (|| match step {
        "context" => {
            let _ = run_context(source, output, module_filter, edge_class, limit, force)?;
            Ok(())
        }
        "generate" => {
            let _ = run_context(source, output, module_filter, edge_class, limit, force)?;
            run_generate(output, prompt_template, force)?;
            Ok(())
        }
        "organize" => {
            let _ = run_context(source, output, module_filter, edge_class, limit, force)?;
            run_generate(output, prompt_template, force)?;
            run_organize(output, coverage_report, force)?;
            Ok(())
        }
        "check" => {
            let report = run_check(output)?;
            if report.invalid_files > 0 {
                return Err(format!(
                    "quality check failed: invalid_files={}",
                    report.invalid_files
                ));
            }
            Ok(())
        }
        "fix" => {
            run_fix(output, prompt_template, true)?;
            Ok(())
        }
        other => Err(format!(
            "invalid --step '{}': expected context|generate|organize|check|fix",
            other
        )),
    })();

    if let Err(e) = result {
        eprintln!("{}", e);
        std::process::exit(1);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
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
    fn bdd_seed_pipeline_outputs_expected_structure() {
        let root = temp_dir("bcc_bdd_seed");
        let source = root.join("source");
        let output = root.join("output");
        let template = root.join("template.dsl");
        fs::create_dir_all(&source).expect("create source");

        write(
            &source.join("account.yaml"),
            r#"module: ACCOUNT
contract: create account
edge_class: action_contract
source_summary: "GIVEN 实体 ACCOUNT 的动作 create 可执行 / WHEN 执行动作 create / THEN 账户应被创建"
"#,
        );
        write(
            &source.join("billing.yml"),
            r#"module: BILLING
contract: issue invoice
edge_class: integration_contract
source_summary: "GIVEN 实体 BILLING 声明 integration invoice / WHEN 触发外部调用 / THEN 应返回发票编号"
"#,
        );
        write(
            &template,
            r#"[SCENARIO: BDD-{MODULE}-SEED-{ID}] TITLE: {TITLE} TAGS: seed {EDGE_CLASS}
GIVEN given_seed_context id="{ID}" module="{MODULE}"
WHEN when_execute_seed_contract module="{MODULE}"
THEN then_seed_contract_should_hold module="{MODULE}"
"#,
        );

        let contexts = run_context(
            &source.to_string_lossy(),
            &output.to_string_lossy(),
            None,
            "stable",
            None,
            true,
        )
        .expect("context");
        assert_eq!(contexts.len(), 2);
        assert!(contexts.iter().any(|c| c.module == "BILLING"));
        // 验证 source_summary 从 YAML 字段提取，非截断
        let account_ctx = contexts.iter().find(|c| c.module == "ACCOUNT").unwrap();
        assert!(account_ctx.source_summary.contains("账户应被创建"));
        assert!(!account_ctx.source_summary.starts_with("module:"));
        // 验证 edge_class 从 YAML 字段提取，非 CLI 参数
        assert_eq!(account_ctx.edge_class, "action_contract");
        let billing_ctx = contexts.iter().find(|c| c.module == "BILLING").unwrap();
        assert_eq!(billing_ctx.edge_class, "integration_contract");

        run_generate(
            &output.to_string_lossy(),
            Some(&template.to_string_lossy()),
            true,
        )
        .expect("generate");
        run_organize(&output.to_string_lossy(), None, true).expect("organize");

        assert!(output.join("contexts/account_account.json").exists());
        assert!(output.join("contexts/billing_billing.json").exists());
        assert!(output.join("scenarios/account_account.dsl").exists());
        assert!(output.join("scenarios/billing_billing.dsl").exists());
        assert!(output.join("features/ACCOUNT.dsl").exists());
        assert!(output.join("features/BILLING.dsl").exists());
        assert!(output.join("coverage.md").exists());

        let feature =
            fs::read_to_string(output.join("features/ACCOUNT.dsl")).expect("read feature");
        assert!(feature.contains("[SCENARIO: BDD-ACCOUNT-SEED-account_account]"));
        let coverage = fs::read_to_string(output.join("coverage.md")).expect("read coverage");
        assert!(coverage.contains("| ACCOUNT |"));
        assert!(coverage.contains("| BILLING |"));

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn bdd_seed_context_module_filter_works() {
        let root = temp_dir("bcc_bdd_filter");
        let source = root.join("source");
        let output = root.join("output");
        fs::create_dir_all(&source).expect("create source");

        write(
            &source.join("account.yaml"),
            r#"module: ACCOUNT
contract: account contract
edge_class: action_contract
source_summary: "GIVEN account / WHEN create / THEN ok"
"#,
        );
        write(
            &source.join("billing.yaml"),
            r#"module: BILLING
contract: billing contract
edge_class: action_contract
source_summary: "GIVEN billing / WHEN invoice / THEN ok"
"#,
        );

        let contexts = run_context(
            &source.to_string_lossy(),
            &output.to_string_lossy(),
            Some("ACCOUNT"),
            "all",
            None,
            true,
        )
        .expect("context filter");

        assert_eq!(contexts.len(), 1);
        assert_eq!(contexts[0].module, "ACCOUNT");
        assert!(output.join("contexts/account_account.json").exists());
        assert!(!output.join("contexts/billing_billing.json").exists());

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn bdd_seed_generate_requires_contexts() {
        let root = temp_dir("bcc_bdd_no_context");
        let output = root.join("output");
        fs::create_dir_all(&output).expect("create output");

        let err = run_generate(&output.to_string_lossy(), None, true).expect_err("should fail");
        assert!(err.contains("no contexts found"));

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn bdd_seed_check_and_fix_work() {
        let root = temp_dir("bcc_bdd_check_fix");
        let source = root.join("source");
        let output = root.join("output");
        fs::create_dir_all(&source).expect("create source");

        write(
            &source.join("account.yaml"),
            r#"module: ACCOUNT
contract: create account
"#,
        );

        let _ = run_context(
            &source.to_string_lossy(),
            &output.to_string_lossy(),
            None,
            "stable",
            None,
            true,
        )
        .expect("context");
        run_generate(&output.to_string_lossy(), None, true).expect("generate");

        // Break scenario intentionally.
        write(
            &output.join("scenarios/account_account.dsl"),
            "[SCENARIO: BDD-ACCOUNT-SEED-account_account] TITLE: broken TAGS: seed stable\nGIVEN x\n",
        );

        let bad = run_check(&output.to_string_lossy()).expect("check");
        assert_eq!(bad.invalid_files, 1);

        run_fix(&output.to_string_lossy(), None, true).expect("fix");
        let good = run_check(&output.to_string_lossy()).expect("check after fix");
        assert_eq!(good.invalid_files, 0);
        assert!(output.join("quality-fix.json").exists());

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn bdd_seed_context_ignores_non_yaml_files() {
        let root = temp_dir("bcc_bdd_ignore_non_yaml");
        let source = root.join("source");
        let output = root.join("output");
        fs::create_dir_all(&source).expect("create source");

        write(
            &source.join("account.yaml"),
            r#"module: ACCOUNT
contract: account contract
"#,
        );
        write(
            &source.join("summary.json"),
            r#"{"module":"SOURCE","contract":"should be ignored"}"#,
        );
        write(&source.join("README.md"), "# ignored");

        let contexts = run_context(
            &source.to_string_lossy(),
            &output.to_string_lossy(),
            None,
            "all",
            None,
            true,
        )
        .expect("context");

        assert_eq!(contexts.len(), 1);
        assert_eq!(contexts[0].module, "ACCOUNT");
        assert!(output.join("contexts/account_account.json").exists());
        assert!(!output.join("contexts/source_summary.json").exists());

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn bdd_seed_context_fallback_when_no_summary_field() {
        let root = temp_dir("bcc_bdd_no_summary");
        let source = root.join("source");
        let output = root.join("output");
        fs::create_dir_all(&source).expect("create source");

        // YAML 无 source_summary 和 edge_class 字段
        write(
            &source.join("legacy.yaml"),
            r#"module: LEGACY
contract: legacy contract
"#,
        );

        let contexts = run_context(
            &source.to_string_lossy(),
            &output.to_string_lossy(),
            None,
            "all",
            None,
            true,
        )
        .expect("context");

        assert_eq!(contexts.len(), 1);
        // 缺少 source_summary 时降级为空字符串
        assert_eq!(contexts[0].source_summary, "");
        // 缺少 edge_class 时降级为 CLI 参数值
        assert_eq!(contexts[0].edge_class, "all");

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn bdd_seed_context_extracts_auth_context_and_keeps_given_line_stable() {
        let root = temp_dir("bcc_bdd_auth_context");
        let source = root.join("source");
        let output = root.join("output");
        fs::create_dir_all(&source).expect("create source");

        write(
            &source.join("protected_read.yaml"),
            r#"module: ACCOUNT
contract: account read contract
edge_class: action_contract
source_summary: "GIVEN account / WHEN read / THEN ok"
auth_context:
  strategy: admin_actor
  actor:
    role: admin
    party_id: party-1
"#,
        );

        let contexts = run_context(
            &source.to_string_lossy(),
            &output.to_string_lossy(),
            None,
            "all",
            None,
            true,
        )
        .expect("context");

        assert_eq!(contexts.len(), 1);
        let auth = contexts[0].auth_context.as_ref().expect("auth context");
        assert_eq!(auth.strategy, "admin_actor");
        assert_eq!(auth.actor.get("role").map(String::as_str), Some("admin"));
        assert_eq!(
            auth.actor.get("party_id").map(String::as_str),
            Some("party-1")
        );

        run_generate(&output.to_string_lossy(), None, true).expect("generate");
        let scenario = fs::read_to_string(output.join("scenarios/account_protected_read.dsl"))
            .expect("read scenario");
        assert!(scenario.contains("GIVEN given_seed_context"));
        assert!(scenario.contains("id=\"account_protected_read\""));
        assert!(scenario.contains("module=\"ACCOUNT\""));
        assert!(!scenario.contains("auth_strategy="));
        assert!(!scenario.contains("actor_role="));

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn bdd_seed_context_errors_when_no_yaml_sources() {
        let root = temp_dir("bcc_bdd_no_yaml");
        let source = root.join("source");
        let output = root.join("output");
        fs::create_dir_all(&source).expect("create source");

        write(
            &source.join("summary.json"),
            r#"{"module":"SOURCE","contract":"not yaml"}"#,
        );
        write(&source.join("notes.md"), "not yaml");

        let err = run_context(
            &source.to_string_lossy(),
            &output.to_string_lossy(),
            None,
            "all",
            None,
            true,
        )
        .expect_err("should fail without yaml");

        assert!(err.contains("no YAML source files found"));

        let _ = fs::remove_dir_all(&root);
    }
}
