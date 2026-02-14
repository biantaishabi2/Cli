use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, HashMap};
use std::fs;
use std::path::{Path, PathBuf};

#[derive(Debug, Deserialize, Serialize)]
struct SeedContext {
    id: String,
    module: String,
    edge_class: String,
    source_file: String,
    #[serde(default)]
    source_summary: String,
}

fn sanitize_id(s: &str) -> String {
    s.chars()
        .map(|c| if c.is_ascii_alphanumeric() { c.to_ascii_lowercase() } else { '_' })
        .collect::<String>()
        .trim_matches('_')
        .to_string()
}

fn read_source_entries(source: &Path) -> Result<Vec<PathBuf>, String> {
    let mut out = Vec::new();
    let mut stack = vec![source.to_path_buf()];
    while let Some(dir) = stack.pop() {
        let rd = fs::read_dir(&dir).map_err(|e| format!("read_dir {} failed: {}", dir.display(), e))?;
        for ent in rd {
            let ent = ent.map_err(|e| format!("read_dir entry failed: {}", e))?;
            let p = ent.path();
            if p.is_dir() {
                stack.push(p);
                continue;
            }
            let Some(ext) = p.extension().and_then(|x| x.to_str()) else { continue; };
            if ["yaml", "yml", "json"].contains(&ext) {
                out.push(p);
            }
        }
    }
    out.sort();
    Ok(out)
}

fn extract_module_from_source(path: &Path, content: &str) -> String {
    if path.extension().and_then(|x| x.to_str()) == Some("json") {
        if let Ok(v) = serde_json::from_str::<serde_json::Value>(content) {
            if let Some(m) = v.get("module").and_then(|x| x.as_str()) {
                return m.to_string();
            }
            if let Some(m) = v.get("module_id").and_then(|x| x.as_str()) {
                return m.to_string();
            }
        }
    }

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
    let Ok(rd) = fs::read_dir(context_dir) else { return map; };
    for ent in rd.flatten() {
        let p = ent.path();
        if p.extension().and_then(|x| x.to_str()) != Some("json") {
            continue;
        }
        let Ok(raw) = fs::read_to_string(&p) else { continue; };
        let Ok(ctx) = serde_json::from_str::<SeedContext>(&raw) else { continue; };
        map.insert(ctx.id.clone(), ctx);
    }
    map
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
    let mut files = read_source_entries(source_path)?;
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
        let stem = file
            .file_stem()
            .and_then(|x| x.to_str())
            .unwrap_or("seed");
        let id = sanitize_id(&format!("{}_{}", module, stem));

        let summary = raw.lines().take(8).collect::<Vec<_>>().join("\\n");
        let ctx = SeedContext {
            id: id.clone(),
            module,
            edge_class: edge_class.to_string(),
            source_file: rel,
            source_summary: summary,
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
            "[SCENARIO: BDD-{MODULE}-SEED-{ID}] TITLE: {TITLE} TAGS: seed {EDGE_CLASS}\nGIVEN given_seed_context id=\"{ID}\" module=\"{MODULE}\"\nWHEN when_execute_seed_contract module=\"{MODULE}\"\nTHEN then_seed_contract_should_hold module=\"{MODULE}\"\n".to_string()
        })
    } else {
        "[SCENARIO: BDD-{MODULE}-SEED-{ID}] TITLE: {TITLE} TAGS: seed {EDGE_CLASS}\nGIVEN given_seed_context id=\"{ID}\" module=\"{MODULE}\"\nWHEN when_execute_seed_contract module=\"{MODULE}\"\nTHEN then_seed_contract_should_hold module=\"{MODULE}\"\n".to_string()
    };

    for ctx in contexts.values() {
        let dsl = template
            .replace("{ID}", &ctx.id)
            .replace("{MODULE}", &ctx.module.to_ascii_uppercase())
            .replace("{EDGE_CLASS}", &ctx.edge_class)
            .replace("{TITLE}", &format!("{} contract seed scenario", ctx.module));
        let out_file = scenarios_dir.join(format!("{}.dsl", ctx.id));
        write_text(&out_file, &dsl, force)?;
    }

    Ok(())
}

fn run_organize(output: &str, coverage_report: Option<&str>, force: bool) -> Result<(), String> {
    let out_dir = Path::new(output);
    let scenarios_dir = out_dir.join("scenarios");
    let features_dir = out_dir.join("features");
    ensure_dir(&features_dir)?;

    let contexts = load_existing_contexts(&out_dir.join("contexts"));

    let rd = fs::read_dir(&scenarios_dir)
        .map_err(|e| format!("read scenarios dir {} failed: {}", scenarios_dir.display(), e))?;

    let mut by_module: BTreeMap<String, Vec<(String, String)>> = BTreeMap::new();
    for ent in rd {
        let ent = ent.map_err(|e| format!("scenario entry read failed: {}", e))?;
        let p = ent.path();
        if p.extension().and_then(|x| x.to_str()) != Some("dsl") {
            continue;
        }
        let stem = p.file_stem().and_then(|x| x.to_str()).unwrap_or("").to_string();
        let raw = fs::read_to_string(&p).map_err(|e| format!("read scenario {} failed: {}", p.display(), e))?;

        let module = contexts
            .get(&stem)
            .map(|c| c.module.clone())
            .unwrap_or_else(|| "unknown_module".to_string());

        by_module.entry(module).or_default().push((stem, raw));
    }

    let mut total_scenarios = 0usize;
    let mut lines = vec!["# BDD Seed Coverage".to_string(), "".to_string(), "| 模块 | 场景文件数 | 场景条数 |".to_string(), "|---|---:|---:|".to_string()];

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
        lines.push(format!("| {} | {} | {} |", module, items.len(), scenario_count));

        let out_file = features_dir.join(format!("{}.dsl", module));
        write_text(&out_file, &merged, force)?;
    }

    lines.push(format!("| **合计** | **{}** | **{}** |", by_module.values().map(|v| v.len()).sum::<usize>(), total_scenarios));
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
    let result = (|| {
        let _ = run_context(source, output, module_filter, edge_class, limit, force)?;
        if step == "context" {
            return Ok(());
        }

        run_generate(output, prompt_template, force)?;
        if step == "generate" {
            return Ok(());
        }

        run_organize(output, coverage_report, force)?;
        Ok::<(), String>(())
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

        let feature = fs::read_to_string(output.join("features/ACCOUNT.dsl")).expect("read feature");
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
"#,
        );
        write(
            &source.join("billing.yaml"),
            r#"module: BILLING
contract: billing contract
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
}
