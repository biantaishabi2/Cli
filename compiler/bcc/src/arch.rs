//! 架构矩阵与门禁工具

use regex::Regex;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, BTreeSet, HashMap, HashSet};
use std::fs;
use std::path::{Path, PathBuf};

pub mod injection;
use injection::{
    append_classification_reason, classify_edge, CallType, RelationClassification, RelationHint,
};
// 子模块
pub mod score;

#[derive(Debug, Deserialize)]
#[allow(non_snake_case)]
struct AstSnapshot {
    #[serde(default)]
    source_count: usize,
    #[serde(default)]
    records: Vec<AstRecord>,
}

#[derive(Debug, Deserialize)]
#[allow(non_snake_case)]
struct AstRecord {
    sourcePath: String,
    #[serde(default)]
    localDependencies: Vec<String>,
    #[serde(default)]
    localCallTargets: Vec<String>,
    #[serde(default)]
    relationHints: Vec<AstRelationHint>,
}

#[derive(Debug, Clone, Deserialize)]
struct AstRelationHint {
    target: String,
    #[serde(default)]
    call_type_hint: String,
    #[serde(default)]
    via: String,
    #[serde(default)]
    confidence: f64,
    #[serde(default)]
    detector: String,
    #[serde(default)]
    reason: String,
}

/// 分层校验规则（可选，嵌入 SeedSpec）
#[derive(Debug, Deserialize)]
struct SeedLayerRules {
    #[serde(default)]
    layers: Vec<score::config::LayerDefinition>,
    #[serde(default)]
    forbidden_transitions: Vec<(String, String)>,
    #[serde(default)]
    allowed_transitions: Vec<(String, String)>,
}

#[derive(Debug, Deserialize)]
struct SeedSpec {
    version: Option<String>,
    source_of_truth: Option<String>,
    #[serde(default)]
    modules: Vec<SeedModule>,
    #[serde(default)]
    relations_expected: Vec<SeedRelation>,
    /// 分层校验规则（可选，未提供时使用默认规则）
    #[serde(default)]
    layer_rules: Option<SeedLayerRules>,
}

#[derive(Debug, Deserialize)]
struct SeedModule {
    module_id: String,
    display_name: Option<String>,
    layer: Option<String>,
    // TODO: 实现领域分析（core/support 模块比例统计）
    // See: https://github.com/biantaishabi2/Cli/issues/TODO
    domain_kind: Option<String>,
    #[serde(default)]
    parent: Option<String>,
    precedence: Option<i64>,
    path_rules: Option<PathRules>,
}

#[derive(Debug, Deserialize)]
struct PathRules {
    #[serde(default)]
    include: Vec<String>,
    #[serde(default)]
    exclude: Vec<String>,
}

#[derive(Debug, Deserialize)]
struct SeedRelation {
    caller: String,
    callee: String,
    allowed: bool,
    #[serde(default)]
    rationale: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct Edge {
    caller: String,
    callee: String,
    #[serde(default)]
    rationale: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct TransitionEdge {
    caller: String,
    callee: String,
    #[serde(default)]
    owner: String,
    #[serde(default)]
    due: String,
    #[serde(default)]
    reason: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct BlockedEdge {
    caller: String,
    callee: String,
    #[serde(default)]
    owner: String,
    #[serde(default)]
    priority: String,
    #[serde(default)]
    reason: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct TargetContract {
    version: String,
    kind: String,
    intent: String,
    source_of_truth: String,
    notes: Vec<String>,
    allow_edges: Vec<Edge>,
    forbid_edges: Vec<Edge>,
}

#[derive(Debug, Serialize, Deserialize)]
struct TransitionContract {
    version: String,
    kind: String,
    base: String,
    intent: String,
    notes: Vec<String>,
    temporary_allow_edges: Vec<TransitionEdge>,
    blocked_edges: Vec<BlockedEdge>,
}

#[derive(Debug, Serialize, Deserialize)]
struct GateProfiles {
    transition: GateThreshold,
    target: GateThreshold,
}

#[derive(Debug, Serialize, Deserialize)]
struct GateThreshold {
    max_unexpected_edges_count: i64,
    max_forbidden_edges_count: i64,
    max_forbidden_total_edges: i64,
    max_missing_edges_count: i64,
    max_directed_density_pct: i64,
    max_bidirectional_pair_count: i64,
}

#[derive(Debug, Serialize, Deserialize)]
struct GateSpec {
    version: String,
    kind: String,
    intent: String,
    profiles: GateProfiles,
}

#[derive(Debug, Clone, Deserialize)]
struct RelationActual {
    caller: String,
    callee: String,
    #[serde(default)]
    import_edges: i64,
    #[serde(default)]
    call_edges: i64,
    #[serde(default)]
    total_edges: i64,
}

#[derive(Debug, Serialize)]
struct ValidateSummary {
    generated_at: String,
    outputs: Vec<String>,
    scenarios: BTreeMap<String, SummaryScenario>,
    #[serde(skip_serializing_if = "Option::is_none")]
    layer_violation_count: Option<i64>,
}

#[derive(Debug, Serialize)]
struct SummaryScenario {
    unexpected_edges_count: i64,
    forbidden_edges_count: i64,
    gate_pass: bool,
}

#[derive(Debug)]
struct EvalResult {
    name: String,
    allow_count: i64,
    forbid_count: i64,
    matched_edges_count: i64,
    matched_total_edges: i64,
    unexpected_edges_count: i64,
    unexpected_total_edges: i64,
    forbidden_edges_count: i64,
    forbidden_total_edges: i64,
    missing_edges_count: i64,
    unexpected_top: Vec<(String, i64)>,
    forbidden_top: Vec<(String, i64)>,
}

#[derive(Debug)]
struct StructureStat {
    modules_count: i64,
    directed_edges_possible: i64,
    directed_edges_actual: i64,
    directed_density_pct: f64,
    // TODO: 在 arch report 中输出模块依赖权重分析
    // See: https://github.com/biantaishabi2/Cli/issues/TODO
    total_module_edge_weight: i64,
    bidirectional_pair_count: i64,
    bidirectional_pairs_top: Vec<(String, i64, i64, i64)>,
}

#[derive(Debug)]
struct GateCheckRow {
    profile: String,
    metric: String,
    actual: String,
    limit: String,
    pass: bool,
}

#[derive(Debug, Deserialize)]
struct TraceModuleMap {
    #[serde(default)]
    entries: Vec<TraceModuleMapEntry>,
}

#[derive(Debug, Deserialize)]
struct TraceModuleMapEntry {
    path: String,
    module_id: String,
}

#[derive(Debug, Deserialize)]
struct ModuleRegistry {
    #[serde(default)]
    modules: Vec<RegistryModule>,
}

#[derive(Debug, Deserialize)]
struct RegistryModule {
    module_id: String,
    display_name: Option<String>,
}

#[derive(Debug, Serialize)]
struct BugfixModuleMap {
    mapping: BTreeMap<String, String>,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    module_names: BTreeMap<String, String>,
}

fn now_iso() -> String {
    use std::process::Command;
    Command::new("date")
        .args(["-u", "+%Y-%m-%dT%H:%M:%SZ"])
        .output()
        .ok()
        .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        .unwrap_or_else(|| "unknown".to_string())
}

fn edge_key(caller: &str, callee: &str) -> String {
    format!("{}->{}", caller, callee)
}

fn to_posix(p: &str) -> String {
    p.replace('\\', "/")
}

fn ensure_parent(path: &Path) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| format!("create dir failed: {}", e))?;
    }
    Ok(())
}

fn write_if_allowed(path: &Path, content: &str, force: bool) -> Result<(), String> {
    if path.exists() && !force {
        return Err(format!("output exists (use --force): {}", path.display()));
    }
    ensure_parent(path)?;
    fs::write(path, content).map_err(|e| format!("write {} failed: {}", path.display(), e))
}

fn glob_to_regex(glob: &str) -> Result<Regex, String> {
    let norm = to_posix(glob);
    let placeholder = "__DOUBLE_STAR__";
    let staged = norm.replace("**", placeholder);

    let mut escaped = String::new();
    for ch in staged.chars() {
        match ch {
            '.' | '+' | '?' | '^' | '$' | '(' | ')' | '|' | '[' | ']' | '{' | '}' | '\\' => {
                escaped.push('\\');
                escaped.push(ch);
            }
            '*' => escaped.push_str("[^/]*"),
            _ => escaped.push(ch),
        }
    }
    let re_s = format!("^{}$", escaped.replace(placeholder, ".*"));
    Regex::new(&re_s).map_err(|e| format!("invalid glob '{}': {}", glob, e))
}

fn map_files_to_modules(
    seed: &SeedSpec,
    ast: &AstSnapshot,
) -> Result<HashMap<String, String>, String> {
    let mut compiled = Vec::new();
    for m in &seed.modules {
        let rules = m
            .path_rules
            .as_ref()
            .ok_or_else(|| format!("module {} missing path_rules", m.module_id))?;
        if rules.include.is_empty() {
            return Err(format!("module {} missing path_rules.include", m.module_id));
        }

        let include = rules
            .include
            .iter()
            .map(|g| glob_to_regex(g))
            .collect::<Result<Vec<_>, _>>()?;
        let exclude = rules
            .exclude
            .iter()
            .map(|g| glob_to_regex(g))
            .collect::<Result<Vec<_>, _>>()?;

        compiled.push((
            m.module_id.clone(),
            m.precedence.unwrap_or(1000),
            include,
            exclude,
        ));
    }

    // 将路径匹配到模块的闭包
    let match_path =
        |path: &str, compiled: &[(String, i64, Vec<Regex>, Vec<Regex>)]| -> Option<String> {
            let mut candidates: Vec<(String, i64)> = Vec::new();
            for (module_id, precedence, includes, excludes) in compiled {
                let include_hit = includes.iter().any(|re| re.is_match(path));
                if !include_hit {
                    continue;
                }
                let exclude_hit = excludes.iter().any(|re| re.is_match(path));
                if exclude_hit {
                    continue;
                }
                candidates.push((module_id.clone(), *precedence));
            }
            if candidates.is_empty() {
                return None;
            }
            candidates.sort_by(|a, b| a.1.cmp(&b.1).then_with(|| a.0.cmp(&b.0)));
            Some(candidates[0].0.clone())
        };

    let mut out = HashMap::new();

    // 第一遍：通过 sourcePath 建立映射
    for rec in &ast.records {
        let path = to_posix(&rec.sourcePath);
        if let Some(module_id) = match_path(&path, &compiled) {
            out.insert(path, module_id);
        }
    }

    // 第二遍：遍历 localDependencies 和 localCallTargets，
    // 将依赖目标路径也通过 glob 规则匹配到模块（sourcePath 优先，不覆盖）
    for rec in &ast.records {
        for dep in rec
            .localDependencies
            .iter()
            .chain(rec.localCallTargets.iter())
        {
            let dep_path = to_posix(dep);
            if out.contains_key(&dep_path) {
                continue;
            }
            if let Some(module_id) = match_path(&dep_path, &compiled) {
                out.insert(dep_path, module_id);
            }
        }
    }

    Ok(out)
}

fn derive_actual_relations(
    ast: &AstSnapshot,
    file_to_module: &HashMap<String, String>,
) -> Vec<RelationActual> {
    let mut agg: HashMap<(String, String), RelationActual> = HashMap::new();

    for rec in &ast.records {
        let src_path = to_posix(&rec.sourcePath);
        let Some(src_module) = file_to_module.get(&src_path) else {
            continue;
        };

        for dep in &rec.localDependencies {
            let dep_path = to_posix(dep);
            let Some(dst_module) = file_to_module.get(&dep_path) else {
                continue;
            };
            if src_module == dst_module {
                continue;
            }
            let key = (src_module.clone(), dst_module.clone());
            let row = agg.entry(key).or_insert(RelationActual {
                caller: src_module.clone(),
                callee: dst_module.clone(),
                import_edges: 0,
                call_edges: 0,
                total_edges: 0,
            });
            row.import_edges += 1;
            row.total_edges += 1;
        }

        for dep in &rec.localCallTargets {
            let dep_path = to_posix(dep);
            let Some(dst_module) = file_to_module.get(&dep_path) else {
                continue;
            };
            if src_module == dst_module {
                continue;
            }
            let key = (src_module.clone(), dst_module.clone());
            let row = agg.entry(key).or_insert(RelationActual {
                caller: src_module.clone(),
                callee: dst_module.clone(),
                import_edges: 0,
                call_edges: 0,
                total_edges: 0,
            });
            row.call_edges += 1;
            row.total_edges += 1;
        }
    }

    let mut rows: Vec<RelationActual> = agg.into_values().collect();
    rows.sort_by(|a, b| edge_key(&a.caller, &a.callee).cmp(&edge_key(&b.caller, &b.callee)));
    rows
}

fn parse_call_type_hint(raw: &str) -> CallType {
    match raw.trim().to_ascii_lowercase().as_str() {
        "framework_injection" | "framework" => CallType::FrameworkInjection,
        "external_registration" | "external_lib" | "registration" => CallType::ExternalRegistration,
        _ => CallType::DirectCall,
    }
}

fn derive_relation_classification(
    ast: &AstSnapshot,
    file_to_module: &HashMap<String, String>,
    actual: &[RelationActual],
    detect_injection: bool,
) -> Vec<RelationClassification> {
    let mut hint_map: HashMap<String, Vec<RelationHint>> = HashMap::new();

    for rec in &ast.records {
        let src_path = to_posix(&rec.sourcePath);
        let Some(src_module) = file_to_module.get(&src_path) else {
            continue;
        };

        for hint in &rec.relationHints {
            let target_path = to_posix(&hint.target);
            let Some(dst_module) = file_to_module.get(&target_path) else {
                continue;
            };
            if src_module == dst_module {
                continue;
            }
            let key = edge_key(src_module, dst_module);
            hint_map.entry(key).or_default().push(RelationHint {
                target: target_path,
                call_type_hint: parse_call_type_hint(&hint.call_type_hint),
                via: hint.via.clone(),
                confidence: hint.confidence,
                detector: hint.detector.clone(),
                reason: hint.reason.clone(),
            });
        }
    }

    let mut rows = Vec::new();
    for edge in actual {
        let key = edge_key(&edge.caller, &edge.callee);
        let hints = hint_map.get(&key).cloned().unwrap_or_default();
        rows.push(classify_edge(
            &edge.caller,
            &edge.callee,
            &hints,
            detect_injection,
        ));
    }
    rows.sort_by(|a, b| edge_key(&a.caller, &a.callee).cmp(&edge_key(&b.caller, &b.callee)));
    rows
}

pub fn matrix(
    seed_file: &str,
    ast_file: &str,
    out_dir: &str,
    version: &str,
    emit: &str,
    detect_injection: bool,
    injection_patterns: Option<&str>,
    force: bool,
) {
    if let Err(e) = matrix_impl(
        seed_file,
        ast_file,
        out_dir,
        version,
        emit,
        detect_injection,
        injection_patterns,
        force,
    ) {
        eprintln!("{}", e);
        std::process::exit(1);
    }
}

fn matrix_impl(
    seed_file: &str,
    ast_file: &str,
    out_dir: &str,
    version: &str,
    emit: &str,
    detect_injection: bool,
    injection_patterns: Option<&str>,
    force: bool,
) -> Result<(), String> {
    let seed_raw =
        fs::read_to_string(seed_file).map_err(|e| format!("read seed_file failed: {}", e))?;
    let ast_raw =
        fs::read_to_string(ast_file).map_err(|e| format!("read ast_file failed: {}", e))?;

    let seed: SeedSpec =
        serde_yaml::from_str(&seed_raw).map_err(|e| format!("parse seed yaml failed: {}", e))?;
    let ast: AstSnapshot =
        serde_json::from_str(&ast_raw).map_err(|e| format!("parse ast json failed: {}", e))?;
    if injection_patterns.is_some() {
        eprintln!("warn: --injection-patterns is reserved for phase2 and currently ignored");
    }

    validate_parent_hierarchy(&seed)?;

    let file_to_module = map_files_to_modules(&seed, &ast)?;
    let actual = derive_actual_relations(&ast, &file_to_module);
    let classifications =
        derive_relation_classification(&ast, &file_to_module, &actual, detect_injection);
    let classification_map: HashMap<String, RelationClassification> = classifications
        .iter()
        .map(|row| (edge_key(&row.caller, &row.callee), row.clone()))
        .collect();

    let actual_keys: BTreeSet<String> = actual
        .iter()
        .map(|r| edge_key(&r.caller, &r.callee))
        .collect();

    let mut allow_edges: Vec<Edge> = Vec::new();
    let mut forbid_edges: Vec<Edge> = Vec::new();

    if seed.relations_expected.is_empty() {
        for k in &actual_keys {
            let (caller, callee) = k.split_once("->").unwrap_or(("", ""));
            let class_row = classification_map.get(k);
            let rationale = if let Some(row) = class_row {
                append_classification_reason("derived from actual relations", row, detect_injection)
            } else {
                "derived from actual relations".to_string()
            };
            allow_edges.push(Edge {
                caller: caller.to_string(),
                callee: callee.to_string(),
                rationale,
            });
        }
    } else {
        for rel in &seed.relations_expected {
            if rel.allowed {
                let key = edge_key(&rel.caller, &rel.callee);
                let rationale = if let Some(row) = classification_map.get(&key) {
                    append_classification_reason("from relations_expected", row, detect_injection)
                } else {
                    "from relations_expected".to_string()
                };
                allow_edges.push(Edge {
                    caller: rel.caller.clone(),
                    callee: rel.callee.clone(),
                    rationale,
                });
            } else {
                forbid_edges.push(Edge {
                    caller: rel.caller.clone(),
                    callee: rel.callee.clone(),
                    rationale: "from relations_expected".to_string(),
                });
            }
        }

        // 展开父模块边到子模块（继承）
        let children_map = build_children_map(&seed);
        if !children_map.is_empty() {
            // 收集显式定义的 key，用于避免继承覆盖显式规则
            let explicit_keys: HashSet<String> = allow_edges
                .iter()
                .chain(forbid_edges.iter())
                .map(|e| edge_key(&e.caller, &e.callee))
                .collect();

            // forbid 展开优先：先展开 forbid，这样 allow 展开时能看到 forbid 的 key
            let expanded_forbid =
                expand_edges_to_children(&forbid_edges, &children_map, &explicit_keys, true);
            forbid_edges.extend(expanded_forbid);

            // 更新 key 集合（含新 forbid）
            let all_keys: HashSet<String> = allow_edges
                .iter()
                .chain(forbid_edges.iter())
                .map(|e| edge_key(&e.caller, &e.callee))
                .collect();

            // allow 展开
            let expanded_allow =
                expand_edges_to_children(&allow_edges, &children_map, &all_keys, false);
            allow_edges.extend(expanded_allow);

            // 同父子模块间默认 allow（内聚性）
            let final_keys: HashSet<String> = allow_edges
                .iter()
                .chain(forbid_edges.iter())
                .map(|e| edge_key(&e.caller, &e.callee))
                .collect();
            let sibling_edges = sibling_default_allow_edges(&children_map, &final_keys);
            allow_edges.extend(sibling_edges);
        }
    }

    allow_edges.sort_by(|a, b| edge_key(&a.caller, &a.callee).cmp(&edge_key(&b.caller, &b.callee)));
    forbid_edges
        .sort_by(|a, b| edge_key(&a.caller, &a.callee).cmp(&edge_key(&b.caller, &b.callee)));

    let allow_set: HashSet<String> = allow_edges
        .iter()
        .map(|e| edge_key(&e.caller, &e.callee))
        .collect();
    let forbid_set: HashSet<String> = forbid_edges
        .iter()
        .map(|e| edge_key(&e.caller, &e.callee))
        .collect();

    let mut temporary_allow_edges = Vec::new();
    for key in &actual_keys {
        if allow_set.contains(key) || forbid_set.contains(key) {
            continue;
        }
        let (caller, callee) = key.split_once("->").unwrap_or(("", ""));
        let class_row = classification_map.get(key);
        let reason = if let Some(row) = class_row {
            append_classification_reason(
                "derived from actual but not in target",
                row,
                detect_injection,
            )
        } else {
            "derived from actual but not in target".to_string()
        };
        temporary_allow_edges.push(TransitionEdge {
            caller: caller.to_string(),
            callee: callee.to_string(),
            owner: "tbd-owner".to_string(),
            due: "2099-12-31".to_string(),
            reason,
        });
    }
    temporary_allow_edges
        .sort_by(|a, b| edge_key(&a.caller, &a.callee).cmp(&edge_key(&b.caller, &b.callee)));

    let mut blocked_edges = Vec::new();
    for edge in &forbid_edges {
        let key = edge_key(&edge.caller, &edge.callee);
        let base_reason = if edge.rationale.is_empty() {
            "forbidden by target".to_string()
        } else {
            edge.rationale.clone()
        };
        let reason = if let Some(row) = classification_map.get(&key) {
            append_classification_reason(&base_reason, row, detect_injection)
        } else {
            base_reason
        };
        blocked_edges.push(BlockedEdge {
            caller: edge.caller.clone(),
            callee: edge.callee.clone(),
            owner: "tbd-owner".to_string(),
            priority: "P1".to_string(),
            reason,
        });
    }

    let target = TargetContract {
        version: seed.version.clone().unwrap_or_else(|| "0.1.0".to_string()),
        kind: "target_contract".to_string(),
        intent: "Target architecture contract generated by bcc arch matrix".to_string(),
        source_of_truth: seed
            .source_of_truth
            .clone()
            .unwrap_or_else(|| "architecture_design".to_string()),
        notes: vec![
            "Generated by bcc arch matrix".to_string(),
            format!("mapped_files={}", file_to_module.len()),
            format!("source_count={}", ast.source_count.max(ast.records.len())),
        ],
        allow_edges,
        forbid_edges,
    };

    let transition = TransitionContract {
        version: target.version.clone(),
        kind: "transition_contract".to_string(),
        base: format!(
            "{}/{}.target-matrix.yaml",
            out_dir.trim_end_matches('/'),
            version
        ),
        intent: "Transition contract between current codebase and target".to_string(),
        notes: vec![
            "temporary_allow_edges are tolerated only during migration".to_string(),
            "blocked_edges are immediate cleanup targets".to_string(),
        ],
        temporary_allow_edges,
        blocked_edges,
    };

    let gates = GateSpec {
        version: target.version.clone(),
        kind: "verification_gates".to_string(),
        intent: "Objective acceptance gates for transition and final target".to_string(),
        profiles: GateProfiles {
            transition: GateThreshold {
                max_unexpected_edges_count: 0,
                max_forbidden_edges_count: 5,
                max_forbidden_total_edges: 26,
                max_missing_edges_count: 0,
                max_directed_density_pct: 75,
                max_bidirectional_pair_count: 20,
            },
            target: GateThreshold {
                max_unexpected_edges_count: 0,
                max_forbidden_edges_count: 0,
                max_forbidden_total_edges: 0,
                max_missing_edges_count: 0,
                max_directed_density_pct: 40,
                max_bidirectional_pair_count: 5,
            },
        },
    };

    let out_dir_path = PathBuf::from(out_dir);
    fs::create_dir_all(&out_dir_path).map_err(|e| format!("create out_dir failed: {}", e))?;

    let target_path = out_dir_path.join(format!("{}.target-matrix.yaml", version));
    let transition_path = out_dir_path.join(format!("{}.transition-matrix.yaml", version));
    let gates_path = out_dir_path.join(format!("{}.gates.yaml", version));
    let classification_path =
        out_dir_path.join(format!("{}.relation-classification.json", version));

    match emit {
        "all" => {
            write_if_allowed(
                &target_path,
                &serde_yaml::to_string(&target).map_err(|e| e.to_string())?,
                force,
            )?;
            write_if_allowed(
                &transition_path,
                &serde_yaml::to_string(&transition).map_err(|e| e.to_string())?,
                force,
            )?;
            write_if_allowed(
                &gates_path,
                &serde_yaml::to_string(&gates).map_err(|e| e.to_string())?,
                force,
            )?;
        }
        "target" => write_if_allowed(
            &target_path,
            &serde_yaml::to_string(&target).map_err(|e| e.to_string())?,
            force,
        )?,
        "transition" => write_if_allowed(
            &transition_path,
            &serde_yaml::to_string(&transition).map_err(|e| e.to_string())?,
            force,
        )?,
        "gates" => write_if_allowed(
            &gates_path,
            &serde_yaml::to_string(&gates).map_err(|e| e.to_string())?,
            force,
        )?,
        other => {
            return Err(format!(
                "invalid --emit '{}': expected all|target|transition|gates",
                other
            ))
        }
    }
    if detect_injection {
        let classification_json = serde_json::to_string_pretty(&classifications)
            .map_err(|e| format!("serialize relation classifications failed: {}", e))?;
        write_if_allowed(
            &classification_path,
            &format!("{}\n", classification_json),
            force,
        )?;
    }

    println!("matrix_written_out_dir={}", out_dir);
    println!("matrix_version={}", version);
    println!("mapped_files={}", file_to_module.len());
    println!("actual_edges={}", actual.len());
    if detect_injection {
        println!(
            "relation_classification_report={}",
            classification_path.to_string_lossy()
        );
    }

    Ok(())
}

fn evaluate_scenario(
    name: &str,
    allow_set: &HashSet<String>,
    forbid_set: &HashSet<String>,
    actual_rows: &[RelationActual],
) -> EvalResult {
    let actual_set: HashSet<String> = actual_rows
        .iter()
        .map(|e| edge_key(&e.caller, &e.callee))
        .collect();

    let mut matched_edges_count = 0;
    let mut matched_total_edges = 0;
    let mut unexpected_edges_count = 0;
    let mut unexpected_total_edges = 0;
    let mut forbidden_edges_count = 0;
    let mut forbidden_total_edges = 0;

    let mut unexpected_top = Vec::new();
    let mut forbidden_top = Vec::new();

    for edge in actual_rows {
        let k = edge_key(&edge.caller, &edge.callee);
        let weight = edge.total_edges;
        if forbid_set.contains(&k) {
            forbidden_edges_count += 1;
            forbidden_total_edges += weight;
            forbidden_top.push((k, weight));
        } else if allow_set.contains(&k) {
            matched_edges_count += 1;
            matched_total_edges += weight;
        } else {
            unexpected_edges_count += 1;
            unexpected_total_edges += weight;
            unexpected_top.push((k, weight));
        }
    }

    let mut missing_edges_count = 0;
    for k in allow_set {
        if !actual_set.contains(k) {
            missing_edges_count += 1;
        }
    }

    unexpected_top.sort_by(|a, b| b.1.cmp(&a.1));
    forbidden_top.sort_by(|a, b| b.1.cmp(&a.1));

    EvalResult {
        name: name.to_string(),
        allow_count: allow_set.len() as i64,
        forbid_count: forbid_set.len() as i64,
        matched_edges_count,
        matched_total_edges,
        unexpected_edges_count,
        unexpected_total_edges,
        forbidden_edges_count,
        forbidden_total_edges,
        missing_edges_count,
        unexpected_top,
        forbidden_top,
    }
}

fn compute_structure(actual_rows: &[RelationActual]) -> StructureStat {
    let modules: BTreeSet<String> = actual_rows
        .iter()
        .flat_map(|e| [e.caller.clone(), e.callee.clone()])
        .collect();
    let n = modules.len() as i64;
    let directed_edges_possible = n * (n - 1);
    let directed_edges_actual = actual_rows.len() as i64;
    let directed_density_pct = if directed_edges_possible == 0 {
        0.0
    } else {
        ((directed_edges_actual as f64) / (directed_edges_possible as f64) * 10000.0).round()
            / 100.0
    };

    let total_module_edge_weight = actual_rows.iter().map(|e| e.total_edges).sum();

    let mut map: HashMap<String, i64> = HashMap::new();
    for e in actual_rows {
        map.insert(edge_key(&e.caller, &e.callee), e.total_edges);
    }

    let mod_list: Vec<String> = modules.into_iter().collect();
    let mut bidirectional_pairs = Vec::new();
    for i in 0..mod_list.len() {
        for j in (i + 1)..mod_list.len() {
            let a = &mod_list[i];
            let b = &mod_list[j];
            let ab = *map.get(&edge_key(a, b)).unwrap_or(&0);
            let ba = *map.get(&edge_key(b, a)).unwrap_or(&0);
            if ab > 0 && ba > 0 {
                bidirectional_pairs.push((format!("{}<->{}", a, b), ab, ba, ab + ba));
            }
        }
    }
    bidirectional_pairs.sort_by(|x, y| y.3.cmp(&x.3));
    let bidirectional_pair_count = bidirectional_pairs.len() as i64;
    bidirectional_pairs.truncate(10);

    StructureStat {
        modules_count: n,
        directed_edges_possible,
        directed_edges_actual,
        directed_density_pct,
        total_module_edge_weight,
        bidirectional_pair_count,
        bidirectional_pairs_top: bidirectional_pairs,
    }
}

fn gate_check_rows(
    profile_name: &str,
    threshold: &GateThreshold,
    scenario: &EvalResult,
    structure: &StructureStat,
) -> (bool, Vec<GateCheckRow>) {
    let mut rows = Vec::new();
    rows.push(GateCheckRow {
        profile: profile_name.to_string(),
        metric: "unexpected_edges_count".to_string(),
        actual: scenario.unexpected_edges_count.to_string(),
        limit: threshold.max_unexpected_edges_count.to_string(),
        pass: scenario.unexpected_edges_count <= threshold.max_unexpected_edges_count,
    });
    rows.push(GateCheckRow {
        profile: profile_name.to_string(),
        metric: "forbidden_edges_count".to_string(),
        actual: scenario.forbidden_edges_count.to_string(),
        limit: threshold.max_forbidden_edges_count.to_string(),
        pass: scenario.forbidden_edges_count <= threshold.max_forbidden_edges_count,
    });
    rows.push(GateCheckRow {
        profile: profile_name.to_string(),
        metric: "forbidden_total_edges".to_string(),
        actual: scenario.forbidden_total_edges.to_string(),
        limit: threshold.max_forbidden_total_edges.to_string(),
        pass: scenario.forbidden_total_edges <= threshold.max_forbidden_total_edges,
    });
    rows.push(GateCheckRow {
        profile: profile_name.to_string(),
        metric: "missing_edges_count".to_string(),
        actual: scenario.missing_edges_count.to_string(),
        limit: threshold.max_missing_edges_count.to_string(),
        pass: scenario.missing_edges_count <= threshold.max_missing_edges_count,
    });

    rows.push(GateCheckRow {
        profile: profile_name.to_string(),
        metric: "directed_density_pct".to_string(),
        actual: format!("{:.2}", structure.directed_density_pct),
        limit: threshold.max_directed_density_pct.to_string(),
        pass: structure.directed_density_pct <= threshold.max_directed_density_pct as f64,
    });

    rows.push(GateCheckRow {
        profile: profile_name.to_string(),
        metric: "bidirectional_pair_count".to_string(),
        actual: structure.bidirectional_pair_count.to_string(),
        limit: threshold.max_bidirectional_pair_count.to_string(),
        pass: structure.bidirectional_pair_count <= threshold.max_bidirectional_pair_count,
    });

    let pass = rows.iter().all(|r| r.pass);
    (pass, rows)
}

fn to_tsv(headers: &[&str], rows: &[Vec<String>]) -> String {
    let mut out = String::new();
    out.push_str(&headers.join("\t"));
    out.push('\n');
    for row in rows {
        out.push_str(&row.join("\t"));
        out.push('\n');
    }
    out
}

/// 解析 edge_key "caller->callee" 为 (caller, callee)
fn parse_edge_key(key: &str) -> Option<(&str, &str)> {
    key.split_once("->")
}

/// 生成一个 bdd seed 可消费的 YAML source 文件
fn write_bdd_source_yaml(
    dir: &Path,
    caller: &str,
    callee: &str,
    edge_class: &str,
    weight: i64,
) -> Result<(), String> {
    let filename = format!("{}_{}.yaml", caller, callee);
    let content = format!(
        "module: {}\ncontract: {} -> {} arch violation\nedge_class: {}\nsource_file: arch-validate-export\nsource_summary: \"{} dependency {}->{}  (weight: {})\"\n",
        caller.to_ascii_uppercase(),
        caller,
        callee,
        edge_class,
        edge_class,
        caller,
        callee,
        weight
    );
    let path = dir.join(&filename);
    fs::write(&path, &content).map_err(|e| format!("write {} failed: {}", path.display(), e))
}

pub fn validate(
    target_path: &str,
    transition_path: &str,
    gates_path: &str,
    actual_path: &str,
    out_dir: &str,
    profile: &str,
    fail_on_gate: bool,
    fail_on_forbidden: bool,
    export_bdd_source: Option<&str>,
    smell_gate: Option<&str>,
    seed_file: Option<&str>,
    fail_on_layer_violation: bool,
) {
    let code = match validate_impl(
        target_path,
        transition_path,
        gates_path,
        actual_path,
        out_dir,
        profile,
        fail_on_gate,
        fail_on_forbidden,
        export_bdd_source,
        smell_gate,
        seed_file,
        fail_on_layer_violation,
    ) {
        Ok(code) => code,
        Err(e) => {
            eprintln!("{}", e);
            1
        }
    };
    if code != 0 {
        std::process::exit(code);
    }
}

fn validate_impl(
    target_path: &str,
    transition_path: &str,
    gates_path: &str,
    actual_path: &str,
    out_dir: &str,
    profile: &str,
    fail_on_gate: bool,
    fail_on_forbidden: bool,
    export_bdd_source: Option<&str>,
    smell_gate: Option<&str>,
    seed_file: Option<&str>,
    fail_on_layer_violation: bool,
) -> Result<i32, String> {
    let target_raw =
        fs::read_to_string(target_path).map_err(|e| format!("read target failed: {}", e))?;
    let transition_raw = fs::read_to_string(transition_path)
        .map_err(|e| format!("read transition failed: {}", e))?;
    let gates_raw =
        fs::read_to_string(gates_path).map_err(|e| format!("read gates failed: {}", e))?;
    let actual_raw =
        fs::read_to_string(actual_path).map_err(|e| format!("read actual failed: {}", e))?;

    let target: TargetContract =
        serde_yaml::from_str(&target_raw).map_err(|e| format!("parse target failed: {}", e))?;
    let transition: TransitionContract = serde_yaml::from_str(&transition_raw)
        .map_err(|e| format!("parse transition failed: {}", e))?;
    let gates: GateSpec =
        serde_yaml::from_str(&gates_raw).map_err(|e| format!("parse gates failed: {}", e))?;
    let actual: Vec<RelationActual> =
        serde_json::from_str(&actual_raw).map_err(|e| format!("parse actual failed: {}", e))?;

    let target_allow: HashSet<String> = target
        .allow_edges
        .iter()
        .map(|e| edge_key(&e.caller, &e.callee))
        .collect();
    let target_forbid: HashSet<String> = target
        .forbid_edges
        .iter()
        .map(|e| edge_key(&e.caller, &e.callee))
        .collect();

    let mut transition_allow = target_allow.clone();
    for e in &transition.temporary_allow_edges {
        transition_allow.insert(edge_key(&e.caller, &e.callee));
    }
    let mut transition_forbid = target_forbid.clone();
    for e in &transition.blocked_edges {
        transition_forbid.insert(edge_key(&e.caller, &e.callee));
    }

    let structure = compute_structure(&actual);
    let target_eval = evaluate_scenario("v3_target", &target_allow, &target_forbid, &actual);
    let transition_eval = evaluate_scenario(
        "v3_transition",
        &transition_allow,
        &transition_forbid,
        &actual,
    );

    let (target_pass, target_rows) =
        gate_check_rows("target", &gates.profiles.target, &target_eval, &structure);
    let (transition_pass, transition_rows) = gate_check_rows(
        "transition",
        &gates.profiles.transition,
        &transition_eval,
        &structure,
    );

    let out_dir_path = PathBuf::from(out_dir);
    fs::create_dir_all(&out_dir_path).map_err(|e| format!("create out_dir failed: {}", e))?;

    let scenario_rows = vec![
        vec![
            target_eval.name.clone(),
            target_eval.allow_count.to_string(),
            target_eval.forbid_count.to_string(),
            target_eval.matched_edges_count.to_string(),
            target_eval.matched_total_edges.to_string(),
            target_eval.unexpected_edges_count.to_string(),
            target_eval.unexpected_total_edges.to_string(),
            target_eval.forbidden_edges_count.to_string(),
            target_eval.forbidden_total_edges.to_string(),
            target_eval.missing_edges_count.to_string(),
        ],
        vec![
            transition_eval.name.clone(),
            transition_eval.allow_count.to_string(),
            transition_eval.forbid_count.to_string(),
            transition_eval.matched_edges_count.to_string(),
            transition_eval.matched_total_edges.to_string(),
            transition_eval.unexpected_edges_count.to_string(),
            transition_eval.unexpected_total_edges.to_string(),
            transition_eval.forbidden_edges_count.to_string(),
            transition_eval.forbidden_total_edges.to_string(),
            transition_eval.missing_edges_count.to_string(),
        ],
    ];
    let scenario_tsv = to_tsv(
        &[
            "name",
            "allow_count",
            "forbid_count",
            "matched_edges_count",
            "matched_total_edges",
            "unexpected_edges_count",
            "unexpected_total_edges",
            "forbidden_edges_count",
            "forbidden_total_edges",
            "missing_edges_count",
        ],
        &scenario_rows,
    );

    fs::write(out_dir_path.join("scenario-validation.tsv"), scenario_tsv)
        .map_err(|e| format!("write scenario-validation.tsv failed: {}", e))?;

    let mut gate_rows = Vec::new();
    for r in target_rows.iter().chain(transition_rows.iter()) {
        gate_rows.push(vec![
            r.profile.clone(),
            r.metric.clone(),
            r.actual.clone(),
            r.limit.clone(),
            if r.pass {
                "pass".to_string()
            } else {
                "fail".to_string()
            },
        ]);
    }
    let gate_tsv = to_tsv(
        &["profile", "metric", "actual", "limit", "pass"],
        &gate_rows,
    );
    fs::write(out_dir_path.join("gate-evaluation.tsv"), gate_tsv)
        .map_err(|e| format!("write gate-evaluation.tsv failed: {}", e))?;

    let mut report = String::new();
    report.push_str("# V3 Validation Report\n\n");
    report.push_str("## Structure Snapshot (actual code)\n");
    report.push_str(&format!("- modules: {}\n", structure.modules_count));
    report.push_str(&format!(
        "- directed_edges_possible: {}\n",
        structure.directed_edges_possible
    ));
    report.push_str(&format!(
        "- directed_edges_actual: {}\n",
        structure.directed_edges_actual
    ));
    report.push_str(&format!(
        "- directed_density_pct: {:.2}%\n",
        structure.directed_density_pct
    ));
    report.push_str(&format!(
        "- bidirectional_pair_count: {}\n",
        structure.bidirectional_pair_count
    ));
    report.push('\n');

    report.push_str("### Top Bidirectional Pairs\n");
    for (pair, ab, ba, total) in &structure.bidirectional_pairs_top {
        report.push_str(&format!("- {}: {} ({} + {})\n", pair, total, ab, ba));
    }
    report.push('\n');

    let write_eval = |r: &EvalResult, pass: bool, report: &mut String| {
        report.push_str(&format!("## {}\n", r.name));
        report.push_str(&format!("- allow_count: {}\n", r.allow_count));
        report.push_str(&format!("- forbid_count: {}\n", r.forbid_count));
        report.push_str(&format!(
            "- unexpected_edges_count: {}\n",
            r.unexpected_edges_count
        ));
        report.push_str(&format!(
            "- unexpected_total_edges: {}\n",
            r.unexpected_total_edges
        ));
        report.push_str(&format!(
            "- forbidden_edges_count: {}\n",
            r.forbidden_edges_count
        ));
        report.push_str(&format!(
            "- forbidden_total_edges: {}\n",
            r.forbidden_total_edges
        ));
        report.push_str(&format!(
            "- missing_edges_count: {}\n",
            r.missing_edges_count
        ));
        report.push_str(&format!(
            "- gate_result: {}\n",
            if pass { "PASS" } else { "FAIL" }
        ));
        report.push('\n');

        if !r.forbidden_top.is_empty() {
            report.push_str(&format!(
                "### Forbidden Top (showing top 20 of {})\n",
                r.forbidden_top.len()
            ));
            for (edge, c) in r.forbidden_top.iter().take(20) {
                report.push_str(&format!("- {}: {}\n", edge, c));
            }
            report.push('\n');
        }

        if !r.unexpected_top.is_empty() {
            report.push_str(&format!(
                "### Unexpected Top (showing top 20 of {})\n",
                r.unexpected_top.len()
            ));
            for (edge, c) in r.unexpected_top.iter().take(20) {
                report.push_str(&format!("- {}: {}\n", edge, c));
            }
            report.push('\n');
        }
    };

    write_eval(&target_eval, target_pass, &mut report);
    write_eval(&transition_eval, transition_pass, &mut report);

    report.push_str("## Gate Checks\n");
    for r in target_rows.iter().chain(transition_rows.iter()) {
        report.push_str(&format!(
            "- [{}] {}: actual={}, limit={} => {}\n",
            r.profile,
            r.metric,
            r.actual,
            r.limit,
            if r.pass { "PASS" } else { "FAIL" }
        ));
    }

    // === Layer violation 检查 ===
    let layer_violation_count: Option<i64> = if let Some(seed_path) = seed_file {
        let seed_raw =
            fs::read_to_string(seed_path).map_err(|e| format!("read seed_file failed: {}", e))?;
        let seed: SeedSpec = serde_yaml::from_str(&seed_raw)
            .map_err(|e| format!("parse seed yaml failed: {}", e))?;

        // 构建 module_id → layer 映射（支持 parent chain 继承：无显式 layer 时沿祖先链查找）
        let parent_map = build_parent_map(&seed);
        let explicit_layer_map: HashMap<String, String> = seed
            .modules
            .iter()
            .filter_map(|m| m.layer.as_ref().map(|l| (m.module_id.clone(), l.clone())))
            .collect();
        let layer_map: HashMap<String, String> = seed
            .modules
            .iter()
            .filter_map(|m| {
                if let Some(l) = m.layer.as_ref() {
                    Some((m.module_id.clone(), l.clone()))
                } else {
                    // 沿 parent chain 继承最近祖先的 layer
                    let ancestors = get_ancestors(&m.module_id, &parent_map);
                    ancestors
                        .iter()
                        .find_map(|a| explicit_layer_map.get(a))
                        .map(|l| (m.module_id.clone(), l.clone()))
                }
            })
            .collect();

        // 只有至少一个模块定义了 layer 才触发检查
        if layer_map.is_empty() {
            None
        } else {
            // 构建 layer rules
            let default_config = score::config::LayeringDimensionConfig::default();
            let layers = match seed.layer_rules {
                Some(ref rules) if !rules.layers.is_empty() => &rules.layers,
                _ => &default_config.layers,
            };
            let forbidden = match seed.layer_rules {
                Some(ref rules) => &rules.forbidden_transitions,
                None => &default_config.forbidden_transitions,
            };
            let allowed = match seed.layer_rules {
                Some(ref rules) if !rules.allowed_transitions.is_empty() => {
                    &rules.allowed_transitions
                }
                _ => &default_config.allowed_transitions,
            };

            struct LayerViolationRecord {
                caller: String,
                callee: String,
                caller_layer: String,
                callee_layer: String,
            }

            // 预计算 layer precedence map，避免循环内重复构建
            let layer_precedence: HashMap<&str, i32> = layers
                .iter()
                .map(|l| (l.name.as_str(), l.precedence))
                .collect();

            let mut violations = Vec::new();
            for edge in &actual {
                let caller_layer = layer_map.get(&edge.caller);
                let callee_layer = layer_map.get(&edge.callee);
                match (caller_layer, callee_layer) {
                    (Some(cl), Some(dl)) => {
                        if !score::check_layer_transition_with_precedence(
                            cl,
                            dl,
                            layer_precedence.get(cl.as_str()).copied(),
                            layer_precedence.get(dl.as_str()).copied(),
                            forbidden,
                            allowed,
                        ) {
                            violations.push(LayerViolationRecord {
                                caller: edge.caller.clone(),
                                callee: edge.callee.clone(),
                                caller_layer: cl.clone(),
                                callee_layer: dl.clone(),
                            });
                        }
                    }
                    (None, _) => {
                        eprintln!(
                            "[validate] skip layer check: {} has no layer defined",
                            edge.caller
                        );
                    }
                    (_, None) => {
                        eprintln!(
                            "[validate] skip layer check: {} has no layer defined",
                            edge.callee
                        );
                    }
                }
            }

            report.push_str("\n## Layer Violations\n");
            if violations.is_empty() {
                report.push_str("No layer violations detected.\n");
            } else {
                for v in &violations {
                    report.push_str(&format!(
                        "- {} ({}) → {} ({})\n",
                        v.caller, v.caller_layer, v.callee, v.callee_layer
                    ));
                }
            }

            Some(violations.len() as i64)
        }
    } else {
        None
    };

    fs::write(out_dir_path.join("v3-validation-report.md"), report)
        .map_err(|e| format!("write v3-validation-report.md failed: {}", e))?;

    let mut scenarios = BTreeMap::new();
    scenarios.insert(
        "v3_target".to_string(),
        SummaryScenario {
            unexpected_edges_count: target_eval.unexpected_edges_count,
            forbidden_edges_count: target_eval.forbidden_edges_count,
            gate_pass: target_pass,
        },
    );
    scenarios.insert(
        "v3_transition".to_string(),
        SummaryScenario {
            unexpected_edges_count: transition_eval.unexpected_edges_count,
            forbidden_edges_count: transition_eval.forbidden_edges_count,
            gate_pass: transition_pass,
        },
    );

    let summary = ValidateSummary {
        generated_at: now_iso(),
        outputs: vec![
            to_posix(
                &out_dir_path
                    .join("scenario-validation.tsv")
                    .to_string_lossy(),
            ),
            to_posix(&out_dir_path.join("gate-evaluation.tsv").to_string_lossy()),
            to_posix(
                &out_dir_path
                    .join("v3-validation-report.md")
                    .to_string_lossy(),
            ),
        ],
        scenarios,
        layer_violation_count,
    };

    fs::write(
        out_dir_path.join("summary.json"),
        format!(
            "{}\n",
            serde_json::to_string_pretty(&summary).map_err(|e| e.to_string())?
        ),
    )
    .map_err(|e| format!("write summary.json failed: {}", e))?;

    // 导出 bdd source YAML（如果指定了 --export-bdd-source）
    if let Some(bdd_dir) = export_bdd_source {
        let bdd_path = Path::new(bdd_dir);
        fs::create_dir_all(bdd_path).map_err(|e| format!("create bdd source dir failed: {}", e))?;

        let mut exported = 0usize;
        for (key, weight) in &transition_eval.forbidden_top {
            if let Some((caller, callee)) = parse_edge_key(key) {
                write_bdd_source_yaml(bdd_path, caller, callee, "blocked", *weight)?;
                exported += 1;
            } else {
                eprintln!("[validate] skip malformed edge key: {}", key);
            }
        }
        for (key, weight) in &transition_eval.unexpected_top {
            if let Some((caller, callee)) = parse_edge_key(key) {
                write_bdd_source_yaml(bdd_path, caller, callee, "temporary", *weight)?;
                exported += 1;
            } else {
                eprintln!("[validate] skip malformed edge key: {}", key);
            }
        }
        eprintln!(
            "[validate] exported {} bdd source files to {}",
            exported, bdd_dir
        );
    }

    let mut code = 0;
    let enforce_target = profile == "target" || profile == "both";
    let enforce_transition = profile == "transition" || profile == "both";

    if profile != "target" && profile != "transition" && profile != "both" {
        return Err(format!(
            "invalid --profile '{}': expected target|transition|both",
            profile
        ));
    }

    if fail_on_forbidden {
        if (enforce_target && target_eval.forbidden_edges_count > 0)
            || (enforce_transition && transition_eval.forbidden_edges_count > 0)
        {
            code = 2;
        }
    }

    if fail_on_gate {
        if (enforce_target && !target_pass) || (enforce_transition && !transition_pass) {
            code = 2;
        }
    }

    // smell-gate: 读取 SmellReport JSON，有 smell 则输出 CI 报告并设 gate 失败
    if let Some(smell_path) = smell_gate {
        let smell_raw = fs::read_to_string(smell_path)
            .map_err(|e| format!("read smell-gate file failed: {}", e))?;
        let smell_reports: Vec<crate::analyze::SmellReport> = serde_json::from_str(&smell_raw)
            .map_err(|e| format!("parse smell-gate json failed: {}", e))?;

        let mut all_smells: Vec<&crate::analyze::SmellRecord> = Vec::new();
        for report in &smell_reports {
            all_smells.extend(report.smells.iter());
        }

        if !all_smells.is_empty() {
            // 按 severity 排序：critical > warning > info
            let severity_order = |s: &str| -> u8 {
                match s {
                    "critical" => 0,
                    "warning" => 1,
                    "info" => 2,
                    _ => 3,
                }
            };
            all_smells.sort_by(|a, b| {
                severity_order(&a.severity)
                    .cmp(&severity_order(&b.severity))
                    .then_with(|| a.file.cmp(&b.file))
                    .then_with(|| a.line.cmp(&b.line))
            });

            eprintln!("\n=== Smell Gate Report ({} issue(s)) ===\n", all_smells.len());
            for smell in &all_smells {
                eprintln!(
                    "[{}] {}:{} - {} ({})",
                    smell.severity.to_uppercase(),
                    smell.file,
                    smell.line,
                    smell.message,
                    smell.rule
                );
                if !smell.offending_code.is_empty() {
                    eprintln!("  offending: {}", smell.offending_code);
                }
                if !smell.fix_hint.is_empty() {
                    eprintln!("  fix: {}", smell.fix_hint);
                }
                if !smell.code_snippet.is_empty() {
                    for snippet_line in smell.code_snippet.lines() {
                        eprintln!("  {}", snippet_line);
                    }
                }
            }

            // 统计
            let critical = all_smells.iter().filter(|s| s.severity == "critical").count();
            let warning = all_smells.iter().filter(|s| s.severity == "warning").count();
            let info = all_smells.iter().filter(|s| s.severity == "info").count();
            eprintln!(
                "\nTotal: {} (critical={}, warning={}, info={})",
                all_smells.len(),
                critical,
                warning,
                info
            );
            eprintln!("smell_gate=FAIL\n");

            // 保留已有的更高失败码（如 code=2 来自 forbidden/gate 失败）
            code = code.max(1);
        } else {
            eprintln!("[smell-gate] no smells found, gate passed");
        }
    }

    // layer violation gate
    if fail_on_layer_violation {
        if let Some(count) = layer_violation_count {
            if count > 0 {
                code = 2;
            }
        }
    }

    println!("validate_out_dir={}", out_dir);
    println!("target_gate_pass={}", target_pass);
    println!("transition_gate_pass={}", transition_pass);

    Ok(code)
}

pub fn export_mermaid(seed_file: &str, output: Option<&str>) {
    let seed_raw = match fs::read_to_string(seed_file) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("read seed_file failed: {}", e);
            std::process::exit(1);
        }
    };
    let seed: SeedSpec = match serde_yaml::from_str(&seed_raw) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("parse seed yaml failed: {}", e);
            std::process::exit(1);
        }
    };

    let children_map = build_children_map(&seed);
    let module_map: HashMap<&str, &SeedModule> = seed
        .modules
        .iter()
        .map(|m| (m.module_id.as_str(), m))
        .collect();

    // 构建 rationale 查找表：(caller, callee) → rationale
    let mut rationale_map: HashMap<(String, String), String> = HashMap::new();
    for rel in &seed.relations_expected {
        if let Some(ref r) = rel.rationale {
            rationale_map.insert((rel.caller.clone(), rel.callee.clone()), r.clone());
        }
    }

    // === 总览图 ===
    let overview = render_mermaid_overview(&seed, &children_map, &module_map, &rationale_map);

    // === 子模块详情图 ===
    const EDGE_THRESHOLD: usize = 15;
    let mut details: Vec<(String, String, usize)> = Vec::new(); // (label, mermaid, edge_count)
    for (parent_id, kids) in &children_map {
        let (mermaid, edge_count) = render_mermaid_detail(
            parent_id,
            kids,
            &seed,
            &children_map,
            &module_map,
            &rationale_map,
        );
        let parent_mod = module_map.get(parent_id.as_str());
        let label = parent_mod
            .and_then(|m| m.display_name.as_deref())
            .unwrap_or(parent_id)
            .to_string();
        details.push((label, mermaid, edge_count));
    }

    match output {
        None => {
            // 无 --output：输出到 stdout（保持兼容）
            println!("{}", overview);
            for (label, mermaid, _) in &details {
                println!();
                println!("%% === {} 子模块详情 ===", label);
                println!("{}", mermaid);
            }
        }
        Some(out_path) => {
            // 有 --output：自动生成 .md 文件，超阈值的拆到 -detail2.md
            let main_path = std::path::PathBuf::from(out_path);
            let stem = main_path.file_stem().unwrap_or_default().to_string_lossy();
            let parent_dir = main_path.parent().unwrap_or(std::path::Path::new("."));
            let overflow_name = format!("{}-detail2.md", stem);
            let overflow_path = parent_dir.join(&overflow_name);

            let mut main_parts: Vec<String> = Vec::new();
            let mut overflow_parts: Vec<String> = Vec::new();

            // 文件头
            main_parts.push(format!(
                "# 架构依赖图\n\n\
                 > 由 `bcc arch export-mermaid` 自动生成\n\n\
                 - 节点形状：`([圆角])` = core, `[方框]` = support, `[(圆柱)]` = generic\n\
                 - 边颜色：蓝色 = 允许依赖, 红色 = 禁止依赖\n\
                 - 边标注：中文说明依赖用途（来自 seed 的 rationale 字段）\n\
                 - 分组：按 layer 分 subgraph，子模块嵌套在父模块内（蓝色边框高亮）"
            ));

            // 总览图
            main_parts.push(format!(
                "## 总览图（父/顶层模块）\n\n```mermaid\n{}\n```",
                overview
            ));

            let mut overflow_labels: Vec<String> = Vec::new();

            for (label, mermaid, edge_count) in &details {
                if *edge_count <= EDGE_THRESHOLD {
                    main_parts.push(format!(
                        "## {} 子模块详情\n\n```mermaid\n{}\n```",
                        label, mermaid
                    ));
                } else {
                    overflow_labels.push(label.clone());
                    overflow_parts.push(format!(
                        "## {} 子模块详情\n\n```mermaid\n{}\n```",
                        label, mermaid
                    ));
                }
            }

            // 如果有 overflow，主文件加链接，生成 overflow 文件
            if !overflow_labels.is_empty() {
                let titles = overflow_labels.join(" / ");
                main_parts.push(format!(
                    "## {} 子模块详情\n\n> 这些图边数较多，单独存放：[{}](./{})",
                    titles, overflow_name, overflow_name
                ));

                let mut overflow_content = format!(
                    "# 架构依赖图 - {} 详情\n\n\
                     > 从 [{}](./{}) 拆分，这些图边数较多（>{} 条边），单独文件渲染\n",
                    titles,
                    main_path.file_name().unwrap_or_default().to_string_lossy(),
                    main_path.file_name().unwrap_or_default().to_string_lossy(),
                    EDGE_THRESHOLD
                );
                for part in &overflow_parts {
                    overflow_content.push('\n');
                    overflow_content.push_str(part);
                    overflow_content.push('\n');
                }

                if let Err(e) = fs::write(&overflow_path, &overflow_content) {
                    eprintln!("写入 overflow 文件失败: {}", e);
                } else {
                    eprintln!(
                        "拆分 {} 个详情图到 {}（边数 > {}）",
                        overflow_labels.len(),
                        overflow_path.display(),
                        EDGE_THRESHOLD
                    );
                }
            }

            let main_content = main_parts.join("\n\n");
            if let Err(e) = fs::write(&main_path, &main_content) {
                eprintln!("写入主文件失败: {}", e);
                std::process::exit(1);
            }
            eprintln!(
                "已生成 {}（{} 个详情图在主文件，{} 个拆分到 overflow）",
                main_path.display(),
                details.len() - overflow_labels.len(),
                overflow_labels.len()
            );
        }
    }
}

/// 总览图：父/顶层模块之间的依赖，子模块嵌套在父模块 subgraph 内
/// 渲染总览图，返回 Mermaid 文本
fn render_mermaid_overview(
    seed: &SeedSpec,
    children_map: &HashMap<String, Vec<String>>,
    module_map: &HashMap<&str, &SeedModule>,
    _rationale_map: &HashMap<(String, String), String>,
) -> String {
    let mut out = String::new();

    // 按 layer 分组（只收集顶层模块）
    let mut layer_groups: BTreeMap<String, Vec<&SeedModule>> = BTreeMap::new();
    let mut no_layer: Vec<&SeedModule> = Vec::new();
    for m in &seed.modules {
        if m.parent.is_some() {
            continue;
        }
        if let Some(ref layer) = m.layer {
            layer_groups.entry(layer.clone()).or_default().push(m);
        } else {
            no_layer.push(m);
        }
    }

    out.push_str("graph TD\n");

    for (layer, modules) in &layer_groups {
        out.push_str(&format!("  subgraph {}[\"{}\"]\n", layer, layer));
        for m in modules {
            let label = m.display_name.as_deref().unwrap_or(&m.module_id);
            if let Some(kids) = children_map.get(&m.module_id) {
                out.push_str(&format!("    subgraph {}[\"{}\"]\n", m.module_id, label));
                for kid_id in kids {
                    if let Some(kid) = module_map.get(kid_id.as_str()) {
                        let kid_label = kid.display_name.as_deref().unwrap_or(&kid.module_id);
                        let shape = domain_kind_shape(
                            &kid.module_id,
                            kid_label,
                            kid.domain_kind.as_deref(),
                        );
                        out.push_str(&format!("      {}\n", shape));
                    }
                }
                out.push_str("    end\n");
            } else {
                let shape = domain_kind_shape(&m.module_id, label, m.domain_kind.as_deref());
                out.push_str(&format!("    {}\n", shape));
            }
        }
        out.push_str("  end\n");
    }

    for m in &no_layer {
        let label = m.display_name.as_deref().unwrap_or(&m.module_id);
        let shape = domain_kind_shape(&m.module_id, label, m.domain_kind.as_deref());
        out.push_str(&format!("  {}\n", shape));
    }

    for parent_id in children_map.keys() {
        out.push_str(&format!(
            "  style {} fill:#e8f4fd,stroke:#1a73e8,stroke-width:2px,stroke-dasharray:none\n",
            parent_id
        ));
    }

    out.push('\n');

    let child_ids: HashSet<&str> = children_map
        .values()
        .flat_map(|kids| kids.iter().map(|s| s.as_str()))
        .collect();
    let mut seen = HashSet::new();
    let mut edge_idx: usize = 0;
    let mut allow_indices: Vec<usize> = Vec::new();
    let mut forbid_indices: Vec<usize> = Vec::new();
    for rel in &seed.relations_expected {
        if child_ids.contains(rel.caller.as_str()) || child_ids.contains(rel.callee.as_str()) {
            continue;
        }
        let key = edge_key(&rel.caller, &rel.callee);
        if !seen.insert(key) {
            continue;
        }
        if rel.allowed {
            if let Some(ref r) = rel.rationale {
                out.push_str(&format!("  {} -->|\"{}\"| {}\n", rel.caller, r, rel.callee));
            } else {
                out.push_str(&format!("  {} --> {}\n", rel.caller, rel.callee));
            }
            allow_indices.push(edge_idx);
        } else {
            if let Some(ref r) = rel.rationale {
                out.push_str(&format!("  {} -.->|\"⛔ {}\"| {}\n", rel.caller, r, rel.callee));
            } else {
                out.push_str(&format!("  {} -.-x {}\n", rel.caller, rel.callee));
            }
            forbid_indices.push(edge_idx);
        }
        edge_idx += 1;
    }

    if !allow_indices.is_empty() {
        let indices: Vec<String> = allow_indices.iter().map(|i| i.to_string()).collect();
        out.push_str(&format!("  linkStyle {} stroke:#2196F3,stroke-width:2px\n", indices.join(",")));
    }
    if !forbid_indices.is_empty() {
        let indices: Vec<String> = forbid_indices.iter().map(|i| i.to_string()).collect();
        out.push_str(&format!("  linkStyle {} stroke:#F44336,stroke-width:2px,stroke-dasharray:5\n", indices.join(",")));
    }

    out
}

/// 子模块详情图：展示父模块内部子模块 + 对外依赖
/// 返回 (mermaid_text, edge_count)
fn render_mermaid_detail(
    parent_id: &str,
    kids: &[String],
    seed: &SeedSpec,
    children_map: &HashMap<String, Vec<String>>,
    module_map: &HashMap<&str, &SeedModule>,
    _rationale_map: &HashMap<(String, String), String>,
) -> (String, usize) {
    let parent_mod = match module_map.get(parent_id) {
        Some(m) => m,
        None => return (String::new(), 0),
    };
    let _parent_label = parent_mod
        .display_name
        .as_deref()
        .unwrap_or(parent_id);

    let mut out = String::new();
    out.push_str("graph LR\n");

    let kid_set: HashSet<&str> = kids.iter().map(|s| s.as_str()).collect();

    // 构建"子模块→其父模块"映射
    let mut child_to_parent: HashMap<String, String> = HashMap::new();
    for (pid, child_ids) in children_map {
        for cid in child_ids {
            child_to_parent.insert(cid.clone(), pid.clone());
        }
    }

    // 折叠外部子模块为其父模块（本 parent 的子模块保持不变）
    let collapse_external = |id: &str| -> String {
        if kid_set.contains(id) || id == parent_id {
            return id.to_string();
        }
        if let Some(pid) = child_to_parent.get(id) {
            pid.clone()
        } else {
            id.to_string()
        }
    };

    // 边结构：(from, to, rationale, is_allowed)
    // from/to 中本 parent 的子模块保持原 ID，外部折叠为父/顶层
    struct DetailEdge {
        from: String,
        to: String,
        rationale: String,
        allowed: bool,
    }

    let mut edges: Vec<DetailEdge> = Vec::new();
    let mut seen_keys: HashSet<String> = HashSet::new();

    // 第一遍：收集子模块级别边，记录已覆盖的 (方向, 外部模块) 对
    // 用于后续去除冗余的父级边
    let mut kid_out_covered: HashSet<String> = HashSet::new(); // 子模块→外部 已有边
    let mut kid_in_covered: HashSet<String> = HashSet::new();  // 外部→子模块 已有边

    for rel in &seed.relations_expected {
        let caller_is_kid = kid_set.contains(rel.caller.as_str());
        let callee_is_kid = kid_set.contains(rel.callee.as_str());

        if caller_is_kid && !callee_is_kid {
            let ext = collapse_external(&rel.callee);
            kid_out_covered.insert(ext);
        }
        if !caller_is_kid && callee_is_kid {
            let ext = collapse_external(&rel.caller);
            kid_in_covered.insert(ext);
        }
    }

    // 第二遍：构建边
    for rel in &seed.relations_expected {
        let caller_is_kid = kid_set.contains(rel.caller.as_str());
        let callee_is_kid = kid_set.contains(rel.callee.as_str());
        let caller_is_parent = rel.caller == parent_id;
        let callee_is_parent = rel.callee == parent_id;
        let caller_is_mine = caller_is_kid || caller_is_parent;
        let callee_is_mine = callee_is_kid || callee_is_parent;

        let caller_collapsed = collapse_external(&rel.caller);
        let callee_collapsed = collapse_external(&rel.callee);
        let caller_collapsed_mine = caller_collapsed == parent_id
            || kid_set.contains(caller_collapsed.as_str());
        let callee_collapsed_mine = callee_collapsed == parent_id
            || kid_set.contains(callee_collapsed.as_str());

        if !caller_is_mine && !callee_is_mine
            && !caller_collapsed_mine && !callee_collapsed_mine
        {
            continue;
        }

        // 内部边（两端都是本 parent 的子模块）
        if caller_is_kid && callee_is_kid {
            let key = format!("{}→{}", rel.caller, rel.callee);
            if seen_keys.insert(key) {
                edges.push(DetailEdge {
                    from: rel.caller.clone(),
                    to: rel.callee.clone(),
                    rationale: rel.rationale.as_deref().unwrap_or("").to_string(),
                    allowed: rel.allowed,
                });
            }
            continue;
        }

        // 外向边：本 parent 的子模块 → 外部
        if caller_is_kid {
            let ext = collapse_external(&rel.callee);
            let key = format!("{}→{}", rel.caller, ext);
            if seen_keys.insert(key) {
                edges.push(DetailEdge {
                    from: rel.caller.clone(),
                    to: ext,
                    rationale: rel.rationale.as_deref().unwrap_or("").to_string(),
                    allowed: rel.allowed,
                });
            }
            continue;
        }

        // 内向边：外部 → 本 parent 的子模块
        if callee_is_kid {
            let ext = collapse_external(&rel.caller);
            let key = format!("{}→{}", ext, rel.callee);
            if seen_keys.insert(key) {
                edges.push(DetailEdge {
                    from: ext,
                    to: rel.callee.clone(),
                    rationale: rel.rationale.as_deref().unwrap_or("").to_string(),
                    allowed: rel.allowed,
                });
            }
            continue;
        }

        // 父模块级别边 — 如果子模块级别已覆盖同方向同外部模块，跳过（除非是禁止边）
        let from = if caller_is_parent {
            parent_id.to_string()
        } else {
            caller_collapsed.clone()
        };
        let to = if callee_is_parent {
            parent_id.to_string()
        } else {
            callee_collapsed.clone()
        };
        if from == to {
            continue;
        }

        // 允许边：子模块级别已精确覆盖则跳过
        if rel.allowed {
            if from == parent_id && kid_out_covered.contains(&to) {
                continue;
            }
            if to == parent_id && kid_in_covered.contains(&from) {
                continue;
            }
        }

        let key = format!("{}→{}", from, to);
        if seen_keys.insert(key) {
            edges.push(DetailEdge {
                from,
                to,
                rationale: rel.rationale.as_deref().unwrap_or("").to_string(),
                allowed: rel.allowed,
            });
        }
    }

    // 输出子模块节点（subgraph 包裹）
    out.push_str(&format!("  subgraph {}[\"{}\"]\n", parent_id, _parent_label));
    for kid_id in kids {
        if let Some(kid) = module_map.get(kid_id.as_str()) {
            let kid_label = kid.display_name.as_deref().unwrap_or(&kid.module_id);
            let shape = domain_kind_shape(&kid.module_id, kid_label, kid.domain_kind.as_deref());
            out.push_str(&format!("    {}\n", shape));
        }
    }
    out.push_str("  end\n");
    out.push_str(&format!(
        "  style {} fill:#e8f4fd,stroke:#1a73e8,stroke-width:2px,stroke-dasharray:none\n",
        parent_id
    ));

    // 收集外部模块 ID
    let mut external_ids: BTreeSet<String> = BTreeSet::new();
    for e in &edges {
        if !kid_set.contains(e.from.as_str()) && e.from != parent_id {
            external_ids.insert(e.from.clone());
        }
        if !kid_set.contains(e.to.as_str()) && e.to != parent_id {
            external_ids.insert(e.to.clone());
        }
    }

    // 输出外部模块节点
    for ext_id in &external_ids {
        if let Some(ext) = module_map.get(ext_id.as_str()) {
            let ext_label = ext.display_name.as_deref().unwrap_or(&ext.module_id);
            let shape = domain_kind_shape(ext_id, ext_label, ext.domain_kind.as_deref());
            out.push_str(&format!("  {}\n", shape));
        } else {
            out.push_str(&format!("  {}[\"{}\"]\n", ext_id, ext_id));
        }
    }

    out.push('\n');

    let edge_count = edges.len();
    let mut edge_idx: usize = 0;
    let mut allow_indices: Vec<usize> = Vec::new();
    let mut forbid_indices: Vec<usize> = Vec::new();

    for e in &edges {
        if e.allowed {
            if !e.rationale.is_empty() {
                out.push_str(&format!("  {} -->|\"{}\"| {}\n", e.from, e.rationale, e.to));
            } else {
                out.push_str(&format!("  {} --> {}\n", e.from, e.to));
            }
            allow_indices.push(edge_idx);
        } else {
            if !e.rationale.is_empty() {
                out.push_str(&format!("  {} -.->|\"⛔ {}\"| {}\n", e.from, e.rationale, e.to));
            } else {
                out.push_str(&format!("  {} -.-x {}\n", e.from, e.to));
            }
            forbid_indices.push(edge_idx);
        }
        edge_idx += 1;
    }

    if !allow_indices.is_empty() {
        let indices: Vec<String> = allow_indices.iter().map(|i| i.to_string()).collect();
        out.push_str(&format!("  linkStyle {} stroke:#2196F3,stroke-width:2px\n", indices.join(",")));
    }
    if !forbid_indices.is_empty() {
        let indices: Vec<String> = forbid_indices.iter().map(|i| i.to_string()).collect();
        out.push_str(&format!("  linkStyle {} stroke:#F44336,stroke-width:2px,stroke-dasharray:5\n", indices.join(",")));
    }

    (out, edge_count)
}

/// 查找边的 rationale：继承边回溯到父模块的原始 rationale
fn find_edge_rationale(
    edge: &Edge,
    parent_id: &str,
    rationale_map: &HashMap<(String, String), String>,
) -> String {
    // 直接匹配
    if let Some(r) = rationale_map.get(&(edge.caller.clone(), edge.callee.clone())) {
        return r.clone();
    }
    // 继承边：caller 是子模块 → 回溯到父模块
    if let Some(r) = rationale_map.get(&(parent_id.to_string(), edge.callee.clone())) {
        return r.clone();
    }
    // callee 是子模块 → 回溯
    if let Some(r) = rationale_map.get(&(edge.caller.clone(), parent_id.to_string())) {
        return r.clone();
    }
    // 兄弟模块
    if edge.rationale.contains("sibling") {
        return "兄弟模块".to_string();
    }
    String::new()
}

/// 根据 domain_kind 返回不同 Mermaid 节点形状
/// core=圆角stadium, support=方框, generic=圆柱cylindrical
fn domain_kind_shape(id: &str, label: &str, domain_kind: Option<&str>) -> String {
    match domain_kind {
        Some("core") => format!("{}([\"{}\"])", id, label),
        Some("generic") => format!("{}[(\"{}\")]", id, label),
        _ => format!("{}[\"{}\"]", id, label),
    }
}

pub fn export_module_map(
    module_map_path: &str,
    module_registry_path: Option<&str>,
    out_path: &str,
    mapping_mode: &str,
    include_module_names: bool,
) {
    if let Err(e) = export_module_map_impl(
        module_map_path,
        module_registry_path,
        out_path,
        mapping_mode,
        include_module_names,
    ) {
        eprintln!("{}", e);
        std::process::exit(1);
    }
}

fn export_module_map_impl(
    module_map_path: &str,
    module_registry_path: Option<&str>,
    out_path: &str,
    mapping_mode: &str,
    include_module_names: bool,
) -> Result<(), String> {
    let raw = fs::read_to_string(module_map_path)
        .map_err(|e| format!("read module_map failed: {}", e))?;
    let tmm: TraceModuleMap =
        serde_json::from_str(&raw).map_err(|e| format!("parse module_map failed: {}", e))?;

    if mapping_mode != "file" && mapping_mode != "dir" {
        return Err(format!(
            "invalid --mapping-mode '{}': expected file|dir",
            mapping_mode
        ));
    }

    let mut mapping: BTreeMap<String, String> = BTreeMap::new();
    for row in &tmm.entries {
        let key = if mapping_mode == "file" {
            to_posix(&row.path)
        } else {
            let p = Path::new(&row.path);
            let parent = p
                .parent()
                .map(|p| p.to_string_lossy().to_string())
                .unwrap_or_else(|| row.path.clone());
            to_posix(&parent)
        };

        mapping.entry(key).or_insert_with(|| row.module_id.clone());
    }

    let mut module_names = BTreeMap::new();
    if include_module_names {
        if let Some(reg_path) = module_registry_path {
            let reg_raw = fs::read_to_string(reg_path)
                .map_err(|e| format!("read module_registry failed: {}", e))?;
            // 支持 json 或 yaml
            let registry: ModuleRegistry =
                if reg_path.ends_with(".yaml") || reg_path.ends_with(".yml") {
                    serde_yaml::from_str(&reg_raw)
                        .map_err(|e| format!("parse module_registry yaml failed: {}", e))?
                } else {
                    serde_json::from_str(&reg_raw)
                        .map_err(|e| format!("parse module_registry json failed: {}", e))?
                };
            for m in registry.modules {
                module_names.insert(m.module_id.clone(), m.display_name.unwrap_or(m.module_id));
            }
        } else {
            let ids: BTreeSet<String> = mapping.values().cloned().collect();
            for id in ids {
                module_names.insert(id.clone(), id);
            }
        }
    }

    let out = BugfixModuleMap {
        mapping,
        module_names,
    };
    let out_abs = Path::new(out_path);
    ensure_parent(out_abs)?;
    fs::write(
        out_abs,
        format!(
            "{}\n",
            serde_json::to_string_pretty(&out).map_err(|e| e.to_string())?
        ),
    )
    .map_err(|e| format!("write out failed: {}", e))?;

    println!("module_map_exported={}", out_path);
    Ok(())
}

pub fn report(
    scenario_validation: &str,
    gate_evaluation: &str,
    summary: &str,
    out: &str,
    top: usize,
    format: &str,
) {
    if let Err(e) = report_impl(
        scenario_validation,
        gate_evaluation,
        summary,
        out,
        top,
        format,
    ) {
        eprintln!("{}", e);
        std::process::exit(1);
    }
}

fn parse_tsv(raw: &str) -> Vec<HashMap<String, String>> {
    let lines: Vec<&str> = raw.lines().filter(|l| !l.trim().is_empty()).collect();
    if lines.is_empty() {
        return Vec::new();
    }
    let headers: Vec<String> = lines[0].split('\t').map(|s| s.to_string()).collect();
    let mut out = Vec::new();
    for line in lines.iter().skip(1) {
        let cols: Vec<&str> = line.split('\t').collect();
        let mut row = HashMap::new();
        for (i, h) in headers.iter().enumerate() {
            row.insert(h.clone(), cols.get(i).unwrap_or(&"").to_string());
        }
        out.push(row);
    }
    out
}

fn report_impl(
    scenario_validation: &str,
    gate_evaluation: &str,
    summary: &str,
    out: &str,
    top: usize,
    format: &str,
) -> Result<(), String> {
    let scenario_raw = fs::read_to_string(scenario_validation)
        .map_err(|e| format!("read scenario-validation failed: {}", e))?;
    let gate_raw = fs::read_to_string(gate_evaluation)
        .map_err(|e| format!("read gate-evaluation failed: {}", e))?;
    let summary_raw =
        fs::read_to_string(summary).map_err(|e| format!("read summary failed: {}", e))?;

    let scenario_rows = parse_tsv(&scenario_raw);
    let gate_rows = parse_tsv(&gate_raw);
    let summary_v: Value =
        serde_json::from_str(&summary_raw).map_err(|e| format!("parse summary failed: {}", e))?;

    if format != "md" && format != "json" {
        return Err(format!("invalid --format '{}': expected md|json", format));
    }

    let out_path = Path::new(out);
    ensure_parent(out_path)?;

    if format == "json" {
        let payload = serde_json::json!({
            "generated_at": now_iso(),
            "summary": summary_v,
            "scenario_rows": scenario_rows,
            "gate_rows": gate_rows,
            "top": top,
        });
        fs::write(
            out_path,
            format!(
                "{}\n",
                serde_json::to_string_pretty(&payload).map_err(|e| e.to_string())?
            ),
        )
        .map_err(|e| format!("write report json failed: {}", e))?;
    } else {
        let mut md = String::new();
        md.push_str("# Architecture Debt Report\n\n");
        md.push_str(&format!("- generated_at: {}\n", now_iso()));
        md.push_str(&format!("- source_summary: `{}`\n\n", summary));

        md.push_str("## Scenario Validation\n\n");
        md.push_str(
            "| name | unexpected_edges_count | forbidden_edges_count | missing_edges_count |\n",
        );
        md.push_str("|---|---:|---:|---:|\n");
        for row in scenario_rows.iter().take(top) {
            md.push_str(&format!(
                "| {} | {} | {} | {} |\n",
                row.get("name").cloned().unwrap_or_default(),
                row.get("unexpected_edges_count")
                    .cloned()
                    .unwrap_or_default(),
                row.get("forbidden_edges_count")
                    .cloned()
                    .unwrap_or_default(),
                row.get("missing_edges_count").cloned().unwrap_or_default()
            ));
        }
        md.push('\n');

        md.push_str("## Gate Evaluation\n\n");
        md.push_str("| profile | metric | actual | limit | pass |\n");
        md.push_str("|---|---|---:|---:|---|\n");
        for row in gate_rows.iter().take(top) {
            md.push_str(&format!(
                "| {} | {} | {} | {} | {} |\n",
                row.get("profile").cloned().unwrap_or_default(),
                row.get("metric").cloned().unwrap_or_default(),
                row.get("actual").cloned().unwrap_or_default(),
                row.get("limit").cloned().unwrap_or_default(),
                row.get("pass").cloned().unwrap_or_default(),
            ));
        }

        fs::write(out_path, md).map_err(|e| format!("write report md failed: {}", e))?;
    }

    println!("arch_report_written={}", out);
    Ok(())
}

/// 校验 seed 中 parent 层级的合法性：无效引用和循环引用
pub(crate) fn validate_parent_hierarchy(seed: &SeedSpec) -> Result<(), String> {
    let known_ids: HashSet<&str> = seed.modules.iter().map(|m| m.module_id.as_str()).collect();

    // 检查无效 parent 引用
    for m in &seed.modules {
        if let Some(ref p) = m.parent {
            if !known_ids.contains(p.as_str()) {
                return Err(format!(
                    "module '{}' has invalid parent '{}': not found in seed modules",
                    m.module_id, p
                ));
            }
        }
    }

    // 检查循环引用
    let parent_map: HashMap<&str, &str> = seed
        .modules
        .iter()
        .filter_map(|m| m.parent.as_ref().map(|p| (m.module_id.as_str(), p.as_str())))
        .collect();

    for m in &seed.modules {
        if m.parent.is_some() {
            let mut visited = HashSet::new();
            let mut current = m.module_id.as_str();
            visited.insert(current);
            while let Some(&next) = parent_map.get(current) {
                if !visited.insert(next) {
                    return Err(format!(
                        "circular parent reference detected involving module '{}'",
                        next
                    ));
                }
                current = next;
            }
        }
    }

    Ok(())
}

/// 构建 module_id → parent 映射
pub(crate) fn build_parent_map(seed: &SeedSpec) -> HashMap<String, Option<String>> {
    seed.modules
        .iter()
        .map(|m| (m.module_id.clone(), m.parent.clone()))
        .collect()
}

/// 沿 parent 链向上遍历，收集祖先列表（从近到远排序）
pub(crate) fn get_ancestors(
    module_id: &str,
    parent_map: &HashMap<String, Option<String>>,
) -> Vec<String> {
    let mut ancestors = Vec::new();
    let mut current = module_id;
    while let Some(Some(ref parent)) = parent_map.get(current) {
        ancestors.push(parent.clone());
        current = parent.as_str();
    }
    ancestors
}

/// 构建 parent → children 映射
fn build_children_map(seed: &SeedSpec) -> HashMap<String, Vec<String>> {
    let mut children: HashMap<String, Vec<String>> = HashMap::new();
    for m in &seed.modules {
        if let Some(ref parent) = m.parent {
            children
                .entry(parent.clone())
                .or_default()
                .push(m.module_id.clone());
        }
    }
    children
}

/// 展开 relations_expected 到子模块：父模块的 allow/forbid 边自动继承给子模块
fn expand_edges_to_children(
    edges: &[Edge],
    children_map: &HashMap<String, Vec<String>>,
    existing_keys: &HashSet<String>,
    is_forbid: bool,
) -> Vec<Edge> {
    let mut expanded = Vec::new();
    for edge in edges {
        let callers = {
            let mut v = vec![edge.caller.clone()];
            if let Some(kids) = children_map.get(&edge.caller) {
                v.extend(kids.iter().cloned());
            }
            v
        };
        let callees = {
            let mut v = vec![edge.callee.clone()];
            if let Some(kids) = children_map.get(&edge.callee) {
                v.extend(kids.iter().cloned());
            }
            v
        };
        for c in &callers {
            for e in &callees {
                if c == &edge.caller && e == &edge.callee {
                    continue; // 原始边已存在，跳过
                }
                let key = edge_key(c, e);
                if existing_keys.contains(&key) {
                    continue; // 已有显式定义，不覆盖
                }
                let inherit_note = if is_forbid {
                    format!(
                        "inherited forbid from {}→{}",
                        edge.caller, edge.callee
                    )
                } else {
                    format!(
                        "inherited from {}→{}",
                        edge.caller, edge.callee
                    )
                };
                expanded.push(Edge {
                    caller: c.clone(),
                    callee: e.clone(),
                    rationale: inherit_note,
                });
            }
        }
    }
    expanded
}

/// 生成同父子模块间的默认 allow 边（内聚性）
fn sibling_default_allow_edges(
    children_map: &HashMap<String, Vec<String>>,
    existing_keys: &HashSet<String>,
) -> Vec<Edge> {
    let mut edges = Vec::new();
    for (parent, kids) in children_map {
        for i in 0..kids.len() {
            for j in 0..kids.len() {
                if i == j {
                    continue;
                }
                let key = edge_key(&kids[i], &kids[j]);
                if !existing_keys.contains(&key) {
                    edges.push(Edge {
                        caller: kids[i].clone(),
                        callee: kids[j].clone(),
                        rationale: format!("sibling modules under {}", parent),
                    });
                }
            }
        }
    }
    edges
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
        fs::write(path, content).expect("write fixture");
    }

    #[test]
    fn arch_end_to_end_outputs_are_generated() {
        let root = temp_dir("bcc_arch_e2e");
        let seed = root.join("seed.yaml");
        let ast = root.join("ast.json");
        let actual = root.join("actual.json");
        let matrix_out = root.join("seed_out");
        let validate_out = root.join("validate_out");
        let report_out = validate_out.join("architecture-debt.md");
        let trace_module_map = root.join("module_map.json");
        let module_registry = root.join("module_registry.yaml");
        let bugfix_module_map = root.join("module_map.bugfix.json");

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
  - module_id: NOTIFY
    precedence: 10
    path_rules:
      include: ["src/notify/**"]
relations_expected:
  - caller: ACCOUNT
    callee: BILLING
    allowed: true
  - caller: ACCOUNT
    callee: NOTIFY
    allowed: true
"#,
        );
        write(
            &ast,
            r#"{
  "source_count": 3,
  "records": [
    {
      "sourcePath": "src/account/create.ts",
      "localDependencies": ["src/billing/invoice.ts", "src/notify/send.ts"],
      "localCallTargets": []
    },
    {
      "sourcePath": "src/billing/invoice.ts",
      "localDependencies": [],
      "localCallTargets": []
    },
    {
      "sourcePath": "src/notify/send.ts",
      "localDependencies": [],
      "localCallTargets": []
    }
  ]
}"#,
        );
        write(
            &actual,
            r#"[
  {"caller":"ACCOUNT","callee":"BILLING","import_edges":1,"call_edges":0,"total_edges":1},
  {"caller":"ACCOUNT","callee":"NOTIFY","import_edges":1,"call_edges":0,"total_edges":1}
]"#,
        );
        write(
            &trace_module_map,
            r#"{
  "entries": [
    {"path":"src/account/","module_id":"ACCOUNT"},
    {"path":"src/billing/","module_id":"BILLING"},
    {"path":"src/notify/","module_id":"NOTIFY"}
  ]
}"#,
        );
        write(
            &module_registry,
            r#"modules:
  - module_id: ACCOUNT
    display_name: 账户域
  - module_id: BILLING
    display_name: 计费域
  - module_id: NOTIFY
    display_name: 通知域
"#,
        );

        matrix_impl(
            &seed.to_string_lossy(),
            &ast.to_string_lossy(),
            &matrix_out.to_string_lossy(),
            "v3",
            "all",
            false,
            None,
            true,
        )
        .expect("matrix ok");

        assert!(matrix_out.join("v3.target-matrix.yaml").exists());
        assert!(matrix_out.join("v3.transition-matrix.yaml").exists());
        assert!(matrix_out.join("v3.gates.yaml").exists());

        let code = validate_impl(
            &matrix_out.join("v3.target-matrix.yaml").to_string_lossy(),
            &matrix_out
                .join("v3.transition-matrix.yaml")
                .to_string_lossy(),
            &matrix_out.join("v3.gates.yaml").to_string_lossy(),
            &actual.to_string_lossy(),
            &validate_out.to_string_lossy(),
            "both",
            true,
            true,
            None,
            None,
            None,
            false,
        )
        .expect("validate ok");
        assert_eq!(code, 0);

        assert!(validate_out.join("scenario-validation.tsv").exists());
        assert!(validate_out.join("gate-evaluation.tsv").exists());
        assert!(validate_out.join("summary.json").exists());
        assert!(validate_out.join("v3-validation-report.md").exists());

        report_impl(
            &validate_out
                .join("scenario-validation.tsv")
                .to_string_lossy(),
            &validate_out.join("gate-evaluation.tsv").to_string_lossy(),
            &validate_out.join("summary.json").to_string_lossy(),
            &report_out.to_string_lossy(),
            20,
            "md",
        )
        .expect("report ok");
        let report_text = fs::read_to_string(&report_out).expect("read report");
        assert!(report_text.contains("Architecture Debt Report"));

        export_module_map_impl(
            &trace_module_map.to_string_lossy(),
            Some(&module_registry.to_string_lossy()),
            &bugfix_module_map.to_string_lossy(),
            "file",
            true,
        )
        .expect("export map ok");
        let map_text = fs::read_to_string(&bugfix_module_map).expect("read module_map");
        assert!(map_text.contains("\"mapping\""));
        assert!(map_text.contains("\"module_names\""));

        let _ = fs::remove_dir_all(&root);
    }

    #[test]
    fn validate_fail_flags_control_exit_code() {
        let root = temp_dir("bcc_arch_flags");
        let target = root.join("target.yaml");
        let transition = root.join("transition.yaml");
        let gates = root.join("gates.yaml");
        let actual = root.join("actual.json");
        let strict_out = root.join("strict_out");
        let report_out = root.join("report_out");

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
        // B->A is unexpected, so gate should fail.
        write(
            &actual,
            r#"[
  {"caller":"A","callee":"B","import_edges":1,"call_edges":0,"total_edges":1},
  {"caller":"B","callee":"A","import_edges":1,"call_edges":0,"total_edges":1}
]"#,
        );

        let strict_code = validate_impl(
            &target.to_string_lossy(),
            &transition.to_string_lossy(),
            &gates.to_string_lossy(),
            &actual.to_string_lossy(),
            &strict_out.to_string_lossy(),
            "both",
            true,
            true,
            None,
            None,
            None,
            false,
        )
        .expect("strict validate");
        assert_eq!(strict_code, 2);

        let report_code = validate_impl(
            &target.to_string_lossy(),
            &transition.to_string_lossy(),
            &gates.to_string_lossy(),
            &actual.to_string_lossy(),
            &report_out.to_string_lossy(),
            "both",
            false,
            false,
            None,
            None,
            None,
            false,
        )
        .expect("report validate");
        assert_eq!(report_code, 0);

        let _ = fs::remove_dir_all(&root);
    }

    /// dep-path ≠ source-path 时，localDependencies 中的路径也能被 glob 匹配到正确模块
    #[test]
    fn map_files_includes_dep_paths_not_in_source_paths() {
        let seed = SeedSpec {
            version: None,
            source_of_truth: None,
            modules: vec![
                SeedModule {
                    module_id: "TOOLS".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: None,
                    precedence: Some(10),
                    path_rules: Some(PathRules {
                        include: vec!["gong/tools/**".to_string()],
                        exclude: vec![],
                    }),
                },
                SeedModule {
                    module_id: "COMPACTION".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: None,
                    precedence: Some(10),
                    path_rules: Some(PathRules {
                        include: vec!["gong/truncate.ex".to_string()],
                        exclude: vec![],
                    }),
                },
            ],
            relations_expected: vec![],
            layer_rules: None,
        };

        // sourcePath 是 gong/tools/truncate.ex，但 localDependencies 中引用 gong/truncate.ex
        let ast = AstSnapshot {
            source_count: 1,
            records: vec![AstRecord {
                sourcePath: "gong/tools/truncate.ex".to_string(),
                localDependencies: vec!["gong/truncate.ex".to_string()],
                localCallTargets: vec![],
                relationHints: vec![],
            }],
        };

        let ftm = map_files_to_modules(&seed, &ast).expect("map ok");

        // sourcePath 应映射到 TOOLS
        assert_eq!(
            ftm.get("gong/tools/truncate.ex"),
            Some(&"TOOLS".to_string())
        );
        // dep 路径 gong/truncate.ex 应映射到 COMPACTION
        assert_eq!(ftm.get("gong/truncate.ex"), Some(&"COMPACTION".to_string()));

        // derive_actual_relations 应检测到 TOOLS → COMPACTION 边
        let rels = derive_actual_relations(&ast, &ftm);
        assert_eq!(rels.len(), 1);
        assert_eq!(rels[0].caller, "TOOLS");
        assert_eq!(rels[0].callee, "COMPACTION");
        assert_eq!(rels[0].import_edges, 1);
    }

    /// sourcePath 已有映射时，dep 路径不应覆盖
    #[test]
    fn dep_path_does_not_override_source_path() {
        let seed = SeedSpec {
            version: None,
            source_of_truth: None,
            modules: vec![
                SeedModule {
                    module_id: "MOD_A".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: None,
                    precedence: Some(10),
                    path_rules: Some(PathRules {
                        include: vec!["src/a/**".to_string()],
                        exclude: vec![],
                    }),
                },
                SeedModule {
                    module_id: "MOD_B".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: None,
                    precedence: Some(10),
                    path_rules: Some(PathRules {
                        include: vec!["src/b/**".to_string()],
                        exclude: vec![],
                    }),
                },
            ],
            relations_expected: vec![],
            layer_rules: None,
        };

        let ast = AstSnapshot {
            source_count: 2,
            records: vec![
                AstRecord {
                    sourcePath: "src/a/foo.ts".to_string(),
                    localDependencies: vec!["src/b/bar.ts".to_string()],
                    localCallTargets: vec![],
                    relationHints: vec![],
                },
                AstRecord {
                    sourcePath: "src/b/bar.ts".to_string(),
                    localDependencies: vec![],
                    localCallTargets: vec![],
                    relationHints: vec![],
                },
            ],
        };

        let ftm = map_files_to_modules(&seed, &ast).expect("map ok");
        // src/b/bar.ts 既是 sourcePath 又是 dep，应保持 sourcePath 的映射
        assert_eq!(ftm.get("src/b/bar.ts"), Some(&"MOD_B".to_string()));
    }

    // === parent 层级相关测试 ===

    /// 基本层级解析：build_parent_map 和 get_ancestors 正确工作
    #[test]
    fn parent_hierarchy_basic() {
        let seed = SeedSpec {
            version: None,
            source_of_truth: None,
            modules: vec![
                SeedModule {
                    module_id: "AGENT".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: None,
                    precedence: None,
                    path_rules: None,
                },
                SeedModule {
                    module_id: "AGENT_HISTORY".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: Some("AGENT".to_string()),
                    precedence: None,
                    path_rules: None,
                },
                SeedModule {
                    module_id: "AGENT_LOOP".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: Some("AGENT".to_string()),
                    precedence: None,
                    path_rules: None,
                },
            ],
            relations_expected: vec![],
            layer_rules: None,
        };

        let pm = build_parent_map(&seed);
        assert_eq!(pm.get("AGENT"), Some(&None));
        assert_eq!(
            pm.get("AGENT_HISTORY"),
            Some(&Some("AGENT".to_string()))
        );
        assert_eq!(
            pm.get("AGENT_LOOP"),
            Some(&Some("AGENT".to_string()))
        );

        let ancestors = get_ancestors("AGENT_HISTORY", &pm);
        assert_eq!(ancestors, vec!["AGENT"]);

        let ancestors_root = get_ancestors("AGENT", &pm);
        assert!(ancestors_root.is_empty());
    }

    /// 无效 parent 引用被检测
    #[test]
    fn parent_invalid_reference() {
        let seed = SeedSpec {
            version: None,
            source_of_truth: None,
            modules: vec![SeedModule {
                module_id: "X".to_string(),
                display_name: None,
                layer: None,
                domain_kind: None,
                parent: Some("NONEXISTENT".to_string()),
                precedence: None,
                path_rules: None,
            }],
            relations_expected: vec![],
            layer_rules: None,
        };

        let result = validate_parent_hierarchy(&seed);
        assert!(result.is_err());
        let msg = result.unwrap_err();
        assert!(msg.contains("X"));
        assert!(msg.contains("NONEXISTENT"));
    }

    /// 循环引用被检测
    #[test]
    fn parent_circular_reference() {
        let seed = SeedSpec {
            version: None,
            source_of_truth: None,
            modules: vec![
                SeedModule {
                    module_id: "A".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: Some("B".to_string()),
                    precedence: None,
                    path_rules: None,
                },
                SeedModule {
                    module_id: "B".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: Some("A".to_string()),
                    precedence: None,
                    path_rules: None,
                },
            ],
            relations_expected: vec![],
            layer_rules: None,
        };

        let result = validate_parent_hierarchy(&seed);
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("circular"));
    }

    /// 多层嵌套：get_ancestors 返回从近到远的祖先列表
    #[test]
    fn parent_multi_level_ancestors() {
        let seed = SeedSpec {
            version: None,
            source_of_truth: None,
            modules: vec![
                SeedModule {
                    module_id: "A".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: None,
                    precedence: None,
                    path_rules: None,
                },
                SeedModule {
                    module_id: "B".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: Some("A".to_string()),
                    precedence: None,
                    path_rules: None,
                },
                SeedModule {
                    module_id: "C".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: Some("B".to_string()),
                    precedence: None,
                    path_rules: None,
                },
            ],
            relations_expected: vec![],
            layer_rules: None,
        };

        let pm = build_parent_map(&seed);
        let ancestors = get_ancestors("C", &pm);
        assert_eq!(ancestors, vec!["B", "A"]);
    }

    /// 向后兼容：无 parent 字段的 seed 正常通过校验
    #[test]
    fn parent_backward_compatible() {
        let seed = SeedSpec {
            version: None,
            source_of_truth: None,
            modules: vec![
                SeedModule {
                    module_id: "MOD1".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: None,
                    precedence: None,
                    path_rules: None,
                },
                SeedModule {
                    module_id: "MOD2".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: None,
                    precedence: None,
                    path_rules: None,
                },
            ],
            relations_expected: vec![],
            layer_rules: None,
        };

        let result = validate_parent_hierarchy(&seed);
        assert!(result.is_ok());
    }

    /// 深度 parent 链：10 层嵌套正确解析且通过校验
    #[test]
    fn parent_deep_chain() {
        // 构建 L0 -> L1 -> L2 -> ... -> L9 的 10 层链
        let modules: Vec<SeedModule> = (0..10)
            .map(|i| SeedModule {
                module_id: format!("L{}", i),
                display_name: None,
                layer: None,
                domain_kind: None,
                parent: if i == 0 { None } else { Some(format!("L{}", i - 1)) },
                precedence: None,
                path_rules: None,
            })
            .collect();

        let seed = SeedSpec {
            version: None,
            source_of_truth: None,
            modules,
            relations_expected: vec![],
            layer_rules: None,
        };

        // 校验通过
        assert!(validate_parent_hierarchy(&seed).is_ok());

        // 最深节点的祖先链正确（从近到远）
        let pm = build_parent_map(&seed);
        let ancestors = get_ancestors("L9", &pm);
        assert_eq!(ancestors.len(), 9);
        assert_eq!(ancestors[0], "L8");
        assert_eq!(ancestors[8], "L0");
    }

    // === #429 边继承测试 ===

    #[test]
    fn test_child_inherits_parent_allow_edge() {
        // 场景 1：子模块继承父模块的 allow 边
        // AGENT→SESSION allowed, AGENT_HISTORY.parent=AGENT
        // → AGENT_HISTORY→SESSION 应被自动展开为 allow
        let seed = SeedSpec {
            version: Some("test".to_string()),
            source_of_truth: None,
            modules: vec![
                SeedModule {
                    module_id: "AGENT".to_string(),
                    display_name: None,
                    domain_kind: None,
                    layer: None,
                    precedence: None,
                    parent: None,
                    path_rules: Some(PathRules {
                        include: vec!["src/agent/**".to_string()],
                        exclude: vec![],
                    }),
                },
                SeedModule {
                    module_id: "AGENT_HISTORY".to_string(),
                    display_name: None,
                    domain_kind: None,
                    layer: None,
                    precedence: None,
                    parent: Some("AGENT".to_string()),
                    path_rules: Some(PathRules {
                        include: vec!["src/agent/history/**".to_string()],
                        exclude: vec![],
                    }),
                },
                SeedModule {
                    module_id: "SESSION".to_string(),
                    display_name: None,
                    domain_kind: None,
                    layer: None,
                    precedence: None,
                    parent: None,
                    path_rules: Some(PathRules {
                        include: vec!["src/session/**".to_string()],
                        exclude: vec![],
                    }),
                },
            ],
            relations_expected: vec![SeedRelation {
                caller: "AGENT".to_string(),
                callee: "SESSION".to_string(),
                allowed: true,
                rationale: None,
            }],
            layer_rules: None,
        };

        let children_map = build_children_map(&seed);
        assert_eq!(children_map.get("AGENT").unwrap(), &vec!["AGENT_HISTORY".to_string()]);

        let allow_edges = vec![Edge {
            caller: "AGENT".to_string(),
            callee: "SESSION".to_string(),
            rationale: "from relations_expected".to_string(),
        }];
        let existing: HashSet<String> = allow_edges.iter().map(|e| edge_key(&e.caller, &e.callee)).collect();
        let expanded =
            expand_edges_to_children(&allow_edges, &children_map, &existing, false);

        // AGENT_HISTORY→SESSION 应被展开
        let keys: HashSet<String> = expanded.iter().map(|e| edge_key(&e.caller, &e.callee)).collect();
        assert!(keys.contains(&edge_key("AGENT_HISTORY", "SESSION")));

        // rationale 标注继承来源
        let ah_edge = expanded.iter().find(|e| e.caller == "AGENT_HISTORY" && e.callee == "SESSION").unwrap();
        assert!(ah_edge.rationale.contains("inherited from AGENT→SESSION"));
    }

    #[test]
    fn test_explicit_forbid_overrides_inherited_allow() {
        // 场景 2：显式 forbid 优先于继承的 allow
        // AGENT→SESSION allowed, 但显式 AGENT_HISTORY→SESSION forbidden
        // → AGENT_HISTORY→SESSION 保持 forbidden
        let allow_edges = vec![Edge {
            caller: "AGENT".to_string(),
            callee: "SESSION".to_string(),
            rationale: "from relations_expected".to_string(),
        }];
        let forbid_edges = vec![Edge {
            caller: "AGENT_HISTORY".to_string(),
            callee: "SESSION".to_string(),
            rationale: "from relations_expected".to_string(),
        }];

        let mut children_map = HashMap::new();
        children_map.insert("AGENT".to_string(), vec!["AGENT_HISTORY".to_string()]);

        // 先展开 forbid
        let explicit_keys: HashSet<String> = allow_edges
            .iter()
            .chain(forbid_edges.iter())
            .map(|e| edge_key(&e.caller, &e.callee))
            .collect();
        let expanded_forbid = expand_edges_to_children(&forbid_edges, &children_map, &explicit_keys, true);

        // 更新 key 集合
        let mut all_keys = explicit_keys;
        for e in &expanded_forbid {
            all_keys.insert(edge_key(&e.caller, &e.callee));
        }

        // 展开 allow —— AGENT_HISTORY→SESSION 已被显式 forbid，不应出现在 allow 展开中
        let expanded_allow = expand_edges_to_children(&allow_edges, &children_map, &all_keys, false);
        let allow_keys: HashSet<String> = expanded_allow.iter().map(|e| edge_key(&e.caller, &e.callee)).collect();
        assert!(!allow_keys.contains(&edge_key("AGENT_HISTORY", "SESSION")));
    }

    #[test]
    fn test_sibling_modules_default_allowed() {
        // 场景 3：同父子模块间默认 allowed
        // AGENT_HISTORY 和 AGENT_LOOP 都是 AGENT 的子模块
        // → AGENT_HISTORY↔AGENT_LOOP 默认 allow
        let mut children_map = HashMap::new();
        children_map.insert(
            "AGENT".to_string(),
            vec!["AGENT_HISTORY".to_string(), "AGENT_LOOP".to_string()],
        );
        let existing = HashSet::new();
        let sibling_edges = sibling_default_allow_edges(&children_map, &existing);

        let keys: HashSet<String> = sibling_edges.iter().map(|e| edge_key(&e.caller, &e.callee)).collect();
        assert!(keys.contains(&edge_key("AGENT_HISTORY", "AGENT_LOOP")));
        assert!(keys.contains(&edge_key("AGENT_LOOP", "AGENT_HISTORY")));

        // rationale 标注
        let e = sibling_edges.iter().find(|e| e.caller == "AGENT_HISTORY").unwrap();
        assert!(e.rationale.contains("sibling modules under AGENT"));
    }

    #[test]
    fn test_no_parent_behavior_unchanged() {
        // 场景 4：无 parent 时行为不变
        // 所有模块无 parent → children_map 为空 → 不展开
        let seed = SeedSpec {
            version: Some("test".to_string()),
            source_of_truth: None,
            modules: vec![
                SeedModule {
                    module_id: "A".to_string(),
                    display_name: None,
                    domain_kind: None,
                    layer: None,
                    precedence: None,
                    parent: None,
                    path_rules: None,
                },
                SeedModule {
                    module_id: "B".to_string(),
                    display_name: None,
                    domain_kind: None,
                    layer: None,
                    precedence: None,
                    parent: None,
                    path_rules: None,
                },
            ],
            relations_expected: vec![],
            layer_rules: None,
        };

        let children_map = build_children_map(&seed);
        assert!(children_map.is_empty());
    }

    #[test]
    fn test_multi_level_inheritance() {
        // 场景 5：多层嵌套继承
        // C.parent=B, B.parent=A, A→X allowed
        // → B→X 和 C→X 都应被展开
        let mut children_map = HashMap::new();
        children_map.insert("A".to_string(), vec!["B".to_string()]);
        children_map.insert("B".to_string(), vec!["C".to_string()]);

        let allow_edges = vec![Edge {
            caller: "A".to_string(),
            callee: "X".to_string(),
            rationale: "from relations_expected".to_string(),
        }];
        let existing: HashSet<String> = allow_edges.iter().map(|e| edge_key(&e.caller, &e.callee)).collect();

        // 第一轮展开：A→X → B→X
        let expanded1 = expand_edges_to_children(&allow_edges, &children_map, &existing, false);
        let mut all_allow = allow_edges.clone();
        all_allow.extend(expanded1);

        // 第二轮展开（B→X → C→X）
        let keys2: HashSet<String> = all_allow.iter().map(|e| edge_key(&e.caller, &e.callee)).collect();
        let expanded2 = expand_edges_to_children(&all_allow, &children_map, &keys2, false);
        all_allow.extend(expanded2);

        let final_keys: HashSet<String> = all_allow.iter().map(|e| edge_key(&e.caller, &e.callee)).collect();
        assert!(final_keys.contains(&edge_key("A", "X")));
        assert!(final_keys.contains(&edge_key("B", "X")));
        assert!(final_keys.contains(&edge_key("C", "X")));
    }

    /// 全流程端到端集成测试：parent 层级校验 + layer violation
    ///
    /// 验证 parent hierarchy 校验已集成到 matrix pipeline：
    /// 1. 合法 parent 层级 → matrix_impl 成功生成 contract
    /// 2. 非法 parent 引用 → matrix_impl 返回错误（校验拦截）
    /// 3. validate_impl 端到端检测 infrastructure→application layer violation
    ///
    /// 注：当前 parent 的作用是"层级校验 + parent_map 元数据"，
    /// "子模块继承父模块 allow_edges"属于后续迭代（#429 scope 外）。
    #[test]
    fn test_arch_parent_layer_integration() {
        let root = temp_dir("bcc_parent_layer_integration");
        let seed_path = root.join("seed.yaml");
        let ast_path = root.join("ast.json");
        let matrix_out = root.join("matrix_out");
        let validate_out = root.join("validate_out");

        // 构造 3 层模块层级：GRANDCHILD → CHILD → PARENT
        // PARENT 和 CHILD 在 application 层，GRANDCHILD 在 infrastructure 层
        // forbidden_transitions: infrastructure → application
        write(
            &seed_path,
            r#"version: v3
source_of_truth: test_integration
modules:
  - module_id: PARENT
    display_name: Parent Module
    layer: application
    precedence: 10
    path_rules:
      include: ["src/parent/**"]
  - module_id: CHILD
    display_name: Child Module
    layer: application
    parent: PARENT
    precedence: 20
    path_rules:
      include: ["src/child/**"]
  - module_id: GRANDCHILD
    display_name: Grandchild Module
    layer: infrastructure
    parent: CHILD
    precedence: 30
    path_rules:
      include: ["src/grandchild/**"]
relations_expected:
  - caller: GRANDCHILD
    callee: CHILD
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

        // AST: infrastructure 模块 (GRANDCHILD) 调用 application 模块 (CHILD)
        write(
            &ast_path,
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
      "sourcePath": "src/grandchild/repo.ts",
      "localDependencies": ["src/child/service.ts"],
      "localCallTargets": []
    }
  ]
}"#,
        );

        // Step 1: matrix_impl 应成功生成 contract
        matrix_impl(
            &seed_path.to_string_lossy(),
            &ast_path.to_string_lossy(),
            &matrix_out.to_string_lossy(),
            "v3",
            "all",
            false,
            None,
            true,
        )
        .expect("matrix_impl should succeed with parent hierarchy");

        // Step 1a: 验证 matrix 输出的 target contract 包含 seed 中声明的 allow edge
        let target_raw =
            fs::read_to_string(matrix_out.join("v3.target-matrix.yaml")).expect("read target");
        let target: TargetContract =
            serde_yaml::from_str(&target_raw).expect("parse target contract");
        let has_gc_child_edge = target.allow_edges.iter().any(|e| {
            e.caller == "GRANDCHILD" && e.callee == "CHILD"
        });
        assert!(
            has_gc_child_edge,
            "target contract should contain allow edge GRANDCHILD→CHILD"
        );

        // Step 1b: 验证 parent hierarchy 校验已集成到 matrix pipeline
        // 构造含无效 parent 引用的 seed → matrix_impl 必须拒绝
        // 这证明 validate_parent_hierarchy 是 matrix pipeline 的一环，
        // 若被移除此断言立即失败
        let bad_seed_path = root.join("bad_seed.yaml");
        write(
            &bad_seed_path,
            r#"version: v3
source_of_truth: test_integration
modules:
  - module_id: MOD_A
    display_name: Module A
    precedence: 10
    path_rules:
      include: ["src/a/**"]
  - module_id: MOD_B
    display_name: Module B
    parent: NONEXISTENT_PARENT
    precedence: 20
    path_rules:
      include: ["src/b/**"]
relations_expected: []
"#,
        );
        let bad_matrix_out = root.join("bad_matrix_out");
        let bad_result = matrix_impl(
            &bad_seed_path.to_string_lossy(),
            &ast_path.to_string_lossy(),
            &bad_matrix_out.to_string_lossy(),
            "v3",
            "all",
            false,
            None,
            true,
        );
        assert!(
            bad_result.is_err(),
            "matrix_impl should reject seed with invalid parent reference"
        );
        let err_msg = bad_result.unwrap_err();
        assert!(
            err_msg.contains("NONEXISTENT_PARENT"),
            "error should mention the invalid parent 'NONEXISTENT_PARENT', got: {}",
            err_msg
        );

        // Step 1c: 验证循环 parent 引用也被 matrix pipeline 拒绝
        let cycle_seed_path = root.join("cycle_seed.yaml");
        write(
            &cycle_seed_path,
            r#"version: v3
source_of_truth: test_integration
modules:
  - module_id: MOD_X
    display_name: Module X
    parent: MOD_Y
    precedence: 10
    path_rules:
      include: ["src/x/**"]
  - module_id: MOD_Y
    display_name: Module Y
    parent: MOD_X
    precedence: 20
    path_rules:
      include: ["src/y/**"]
relations_expected: []
"#,
        );
        let cycle_matrix_out = root.join("cycle_matrix_out");
        let cycle_result = matrix_impl(
            &cycle_seed_path.to_string_lossy(),
            &ast_path.to_string_lossy(),
            &cycle_matrix_out.to_string_lossy(),
            "v3",
            "all",
            false,
            None,
            true,
        );
        assert!(
            cycle_result.is_err(),
            "matrix_impl should reject seed with circular parent references"
        );

        // 构造 actual relations JSON（infrastructure→application 边）
        let actual_path = root.join("actual.json");
        write(
            &actual_path,
            r#"[
  {"caller":"GRANDCHILD","callee":"CHILD","import_edges":1,"call_edges":0,"total_edges":1}
]"#,
        );

        // Step 2: validate_impl 应检测 layer violation 并返回 exit code 2
        let code = validate_impl(
            &matrix_out.join("v3.target-matrix.yaml").to_string_lossy(),
            &matrix_out
                .join("v3.transition-matrix.yaml")
                .to_string_lossy(),
            &matrix_out.join("v3.gates.yaml").to_string_lossy(),
            &actual_path.to_string_lossy(),
            &validate_out.to_string_lossy(),
            "both",
            false,
            false,
            None,
            None,
            Some(&seed_path.to_string_lossy()),
            true, // fail_on_layer_violation
        )
        .expect("validate_impl should succeed");
        assert_eq!(code, 2, "exit code should be 2 due to layer violation");

        // Step 3: 验证 v3-validation-report.md 包含 Layer Violations 章节
        let report =
            fs::read_to_string(validate_out.join("v3-validation-report.md")).expect("read report");
        assert!(
            report.contains("Layer Violations"),
            "report should contain Layer Violations section"
        );
        assert!(
            report.contains("GRANDCHILD"),
            "report should mention GRANDCHILD in violation"
        );

        // Step 4: 验证 summary.json 中 layer_violation_count > 0
        let summary_raw =
            fs::read_to_string(validate_out.join("summary.json")).expect("read summary");
        let summary: serde_json::Value =
            serde_json::from_str(&summary_raw).expect("parse summary");
        let lvc = summary["layer_violation_count"].as_i64().unwrap_or(0);
        assert!(
            lvc > 0,
            "summary.json layer_violation_count should be > 0, got {}",
            lvc
        );

        // === Step 5: parent 继承语义端到端验证 ===
        // 构造场景：INHERITOR 无显式 layer，其 parent INFRA_BASE 有 layer: infrastructure
        // INHERITOR 调用 APP_MOD (layer: application)，仅当 parent 继承生效时才触发 violation
        // 若 parent 继承逻辑失效，INHERITOR 无 layer → 跳过检查 → 无 violation → 断言失败
        let inherit_seed_path = root.join("inherit_seed.yaml");
        write(
            &inherit_seed_path,
            r#"version: v3
source_of_truth: test_inherit
modules:
  - module_id: APP_MOD
    display_name: Application Module
    layer: application
    precedence: 10
    path_rules:
      include: ["src/app/**"]
  - module_id: INFRA_BASE
    display_name: Infrastructure Base
    layer: infrastructure
    precedence: 20
    path_rules:
      include: ["src/infra_base/**"]
  - module_id: INHERITOR
    display_name: Inheritor (no explicit layer)
    parent: INFRA_BASE
    precedence: 30
    path_rules:
      include: ["src/inheritor/**"]
relations_expected:
  - caller: INHERITOR
    callee: APP_MOD
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
        let inherit_ast_path = root.join("inherit_ast.json");
        write(
            &inherit_ast_path,
            r#"{
  "source_count": 3,
  "records": [
    {"sourcePath": "src/app/handler.ts", "localDependencies": [], "localCallTargets": []},
    {"sourcePath": "src/infra_base/base.ts", "localDependencies": [], "localCallTargets": []},
    {"sourcePath": "src/inheritor/impl.ts", "localDependencies": ["src/app/handler.ts"], "localCallTargets": []}
  ]
}"#,
        );
        let inherit_matrix_out = root.join("inherit_matrix_out");
        matrix_impl(
            &inherit_seed_path.to_string_lossy(),
            &inherit_ast_path.to_string_lossy(),
            &inherit_matrix_out.to_string_lossy(),
            "v3",
            "all",
            false,
            None,
            true,
        )
        .expect("matrix_impl should succeed with inherited layer");

        let inherit_actual_path = root.join("inherit_actual.json");
        write(
            &inherit_actual_path,
            r#"[
  {"caller":"INHERITOR","callee":"APP_MOD","import_edges":1,"call_edges":0,"total_edges":1}
]"#,
        );
        let inherit_validate_out = root.join("inherit_validate_out");
        let inherit_code = validate_impl(
            &inherit_matrix_out
                .join("v3.target-matrix.yaml")
                .to_string_lossy(),
            &inherit_matrix_out
                .join("v3.transition-matrix.yaml")
                .to_string_lossy(),
            &inherit_matrix_out
                .join("v3.gates.yaml")
                .to_string_lossy(),
            &inherit_actual_path.to_string_lossy(),
            &inherit_validate_out.to_string_lossy(),
            "both",
            false,
            false,
            None,
            None,
            Some(&inherit_seed_path.to_string_lossy()),
            true,
        )
        .expect("validate_impl should succeed");
        // INHERITOR 无显式 layer，通过 parent INFRA_BASE 继承 infrastructure
        // infrastructure → application 是 forbidden，因此应检测到 violation（exit code 2）
        assert_eq!(
            inherit_code, 2,
            "exit code should be 2: INHERITOR inherits infrastructure layer from parent INFRA_BASE, \
             calling APP_MOD (application) violates forbidden_transitions"
        );
        let inherit_report = fs::read_to_string(
            inherit_validate_out.join("v3-validation-report.md"),
        )
        .expect("read inherit report");
        assert!(
            inherit_report.contains("INHERITOR"),
            "report should mention INHERITOR in layer violation (inherited layer from parent)"
        );
        let inherit_summary_raw =
            fs::read_to_string(inherit_validate_out.join("summary.json")).expect("read summary");
        let inherit_summary: serde_json::Value =
            serde_json::from_str(&inherit_summary_raw).expect("parse summary");
        assert!(
            inherit_summary["layer_violation_count"].as_i64().unwrap_or(0) > 0,
            "layer_violation_count should be > 0 for inherited layer violation"
        );

        let _ = fs::remove_dir_all(&root);
    }

    /// 验证真实 gong seed-v3.yaml 的结构：parent 层级和 layer_rules 格式
    #[test]
    fn test_gong_seed_v3_structure() {
        let seed_path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("examples/cross-project-arch-comparison/projects/gong/seed-v3.yaml");
        let content = fs::read_to_string(&seed_path)
            .expect("should read gong seed-v3.yaml");
        let seed: SeedSpec = serde_yaml::from_str(&content)
            .expect("gong seed-v3.yaml should parse as valid SeedSpec");

        // 验证 AGENT_CORE.parent = AGENT
        let agent_core = seed.modules.iter().find(|m| m.module_id == "AGENT_CORE")
            .expect("AGENT_CORE module should exist");
        assert_eq!(
            agent_core.parent.as_deref(),
            Some("AGENT"),
            "AGENT_CORE.parent should be AGENT"
        );

        // 验证 AGENT_LOOP.parent = AGENT
        let agent_loop = seed.modules.iter().find(|m| m.module_id == "AGENT_LOOP")
            .expect("AGENT_LOOP module should exist");
        assert_eq!(
            agent_loop.parent.as_deref(),
            Some("AGENT"),
            "AGENT_LOOP.parent should be AGENT"
        );

        // 验证 layer_rules 存在且格式正确
        let layer_rules = seed.layer_rules.as_ref()
            .expect("layer_rules should be present");
        assert!(
            !layer_rules.layers.is_empty(),
            "layer_rules.layers should not be empty"
        );
        assert!(
            !layer_rules.forbidden_transitions.is_empty(),
            "layer_rules.forbidden_transitions should not be empty"
        );

        // 验证 parent 层级无循环引用
        validate_parent_hierarchy(&seed)
            .expect("gong seed-v3.yaml parent hierarchy should be valid");

        // 验证 path_rules 不使用 ** 通配符（避免错误匹配 BDD 文件）
        for m in &[agent_core, agent_loop] {
            if let Some(ref pr) = m.path_rules {
                for rule in &pr.include {
                    assert!(
                        !rule.starts_with("**/"),
                        "module {} path_rule '{}' should not use ** prefix to avoid matching BDD files",
                        m.module_id, rule
                    );
                }
            }
        }
    }
}
