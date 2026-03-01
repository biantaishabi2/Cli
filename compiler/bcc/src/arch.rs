//! 架构矩阵与门禁工具

use chrono::Utc;
use regex::Regex;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, BTreeSet, HashMap, HashSet};
use std::fs;
use std::path::{Path, PathBuf};

use crate::arch_bridge_schema::{
    ContractSourceConfig, RuntimeBindingConfig, UniboActionContract, UniboApiContractDocument,
    UniboBoundaryContract, UniboContractProducer, UniboRuntimeBridgeConfig,
};

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
    /// 时序流程定义（可选，用于流程校验和 sequenceDiagram 导出）
    #[serde(default)]
    flows: Vec<SeedFlow>,
    /// 接口边界定义（可选，声明模块的公共 API 路径）
    #[serde(default)]
    boundaries: Vec<SeedBoundary>,
    /// 事件流定义（可选，声明事件的生产者/消费者关系）
    #[serde(default)]
    events: Vec<SeedEvent>,
}

/// 时序流程定义
#[derive(Debug, Deserialize)]
struct SeedFlow {
    name: String,
    steps: Vec<SeedFlowStep>,
}

/// 流程中的单步
#[derive(Debug, Deserialize)]
struct SeedFlowStep {
    from: String,
    to: String,
    #[serde(default)]
    action: Option<String>,
    /// 行为契约：输入字段 {field: type}
    #[serde(default)]
    input: HashMap<String, String>,
    /// 行为契约：输出字段 {field: type}
    #[serde(default)]
    output: HashMap<String, String>,
}

/// 流程校验结果
#[derive(Debug)]
struct FlowValidationResult {
    flow_name: String,
    missing_steps: Vec<(String, String, Option<String>)>, // (from, to, action)
    shortcuts: Vec<(String, String, String)>,              // (from, to, bypassed_module)
}

/// 接口边界定义：声明模块的公共 API 文件路径
#[derive(Debug, Deserialize)]
struct SeedBoundary {
    module_id: String,
    /// 公共 API 文件 glob 模式（如 "src/agents/index.*"）
    #[serde(default)]
    public_paths: Vec<String>,
    /// 行为契约：声明模块的公共 API 契约
    #[serde(default)]
    contracts: Vec<BoundaryContract>,
}

/// 边界契约：单个公共 API 的输入/输出声明
#[derive(Debug, Deserialize)]
struct BoundaryContract {
    name: String,
    /// 契约类型：command / query / crud（默认 command）
    #[serde(default = "default_contract_kind")]
    kind: String,
    #[serde(default)]
    input: HashMap<String, String>,
    #[serde(default)]
    output: HashMap<String, String>,
    /// 错误码列表（如 ["ORDER-CREATE-001"]）
    #[serde(default)]
    errors: Vec<String>,
    /// CRUD 字段声明（kind=crud 时使用，字段名→类型）
    #[serde(default)]
    fields: HashMap<String, String>,
}

fn default_contract_kind() -> String {
    "command".to_string()
}

const UNIBO_BRIDGE_VERSION: &str = "1.0.0";
const UNIBO_TARGET_RUNTIME_VERSION: &str = "1.x";
const UNIBO_COMPAT_VERSION: &str = "1";
const UNIBO_API_CONTRACT_FILE: &str = "unibo-api-contract.json";
const UNIBO_RUNTIME_BRIDGE_FILE: &str = "unibo-runtime-bridge.yaml";
const LEGACY_API_CONTRACT_SCHEMA_VERSION: &str = "1.0.0";
const LEGACY_API_CONTRACT_FILE: &str = "api-contract.json";
const GENERATE_EXIT_INVALID_ARGUMENT: i32 = 10;
const GENERATE_EXIT_CONTRACT_CONFLICT: i32 = 12;
const GENERATE_EXIT_OUTPUT_FAILED: i32 = 13;

#[derive(Debug, Serialize)]
struct LegacyApiContractProducer {
    name: String,
    version: String,
}

#[derive(Debug, Serialize)]
struct LegacyApiContractItem {
    module_id: String,
    name: String,
    kind: String,
    input: BTreeMap<String, String>,
    output: BTreeMap<String, String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    errors: Vec<String>,
    #[serde(skip_serializing_if = "BTreeMap::is_empty")]
    fields: BTreeMap<String, String>,
}

#[derive(Debug, Serialize)]
struct LegacyApiContractDocument {
    contract_schema_version: String,
    producer: LegacyApiContractProducer,
    seed_version: String,
    generated_at: String,
    contracts: Vec<LegacyApiContractItem>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ConflictStrategy {
    ErrorOnConflict,
    Dedupe,
}

impl ConflictStrategy {
    fn parse(raw: &str) -> Result<Self, String> {
        match raw.trim().to_ascii_lowercase().as_str() {
            "error-on-conflict" => Ok(Self::ErrorOnConflict),
            "dedupe" => Ok(Self::Dedupe),
            _ => Err(format!(
                "unknown conflict strategy: {} (expected error-on-conflict|dedupe)",
                raw
            )),
        }
    }
}

/// 契约复杂度分类
#[derive(Debug, Clone, PartialEq, Eq)]
enum ContractComplexity {
    /// 简单 CRUD，可委托 mix phx.gen
    Crud,
    /// 简单命令，单模块骨架 + BDD
    SimpleCommand,
    /// 流程参与者，骨架 + flow BDD
    FlowParticipant,
    /// 事件参与者，骨架 + 事件 BDD
    EventParticipant,
    /// 复杂编排，骨架 + 完整 BDD
    ComplexOrchestration,
}

/// 根据 seed 上下文自动判定 contract 复杂度
fn classify_contract(
    contract: &BoundaryContract,
    module_id: &str,
    seed: &SeedSpec,
) -> ContractComplexity {
    // kind=crud 直接判定
    if contract.kind == "crud" {
        return ContractComplexity::Crud;
    }

    let in_flow = seed.flows.iter().any(|f| {
        f.steps
            .iter()
            .any(|s| s.from == module_id || s.to == module_id)
    });
    let in_event = seed.events.iter().any(|e| {
        e.producers.iter().any(|p| p == module_id)
            || e.consumers.iter().any(|c| c == module_id)
    });
    let out_edges = seed
        .relations_expected
        .iter()
        .filter(|r| r.caller == module_id && r.allowed)
        .count();

    match (contract.kind.as_str(), in_flow, in_event, out_edges) {
        ("query", false, false, 0..=1) => ContractComplexity::Crud,
        ("command", false, false, 0..=1) => ContractComplexity::SimpleCommand,
        (_, true, _, _) => ContractComplexity::FlowParticipant,
        (_, _, true, _) => ContractComplexity::EventParticipant,
        (_, _, _, 2..) => ContractComplexity::ComplexOrchestration,
        _ => ContractComplexity::SimpleCommand,
    }
}

/// 边界校验结果
#[derive(Debug)]
struct BoundaryValidationResult {
    module_id: String,
    total_external_refs: usize,
    leaked_refs: Vec<BoundaryLeak>,
}

/// 单条 API 泄漏
#[derive(Debug)]
struct BoundaryLeak {
    source_file: String,
    source_module: String,
    target_file: String,
    target_module: String,
}

/// 事件流定义：声明事件的生产者/消费者关系
#[derive(Debug, Deserialize)]
struct SeedEvent {
    name: String,
    /// 事件生产者（module_id 列表）
    #[serde(default)]
    producers: Vec<String>,
    /// 事件消费者（module_id 列表）
    #[serde(default)]
    consumers: Vec<String>,
    /// 行为契约：事件载荷 {field: type}
    #[serde(default)]
    payload: HashMap<String, String>,
}

/// 事件校验结果
#[derive(Debug)]
struct EventValidationResult {
    event_name: String,
    orphan: bool,
    ghost_consumers: Vec<String>,
    missing_edges: Vec<(String, String)>,
    reverse_deps: Vec<(String, String)>,
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

/// 校验流程定义：检查每个 step 是否有对应的实际依赖边，以及是否存在跳过中间模块的捷径
fn validate_flows(
    flows: &[SeedFlow],
    actual_edges: &HashSet<(String, String)>,
) -> Vec<FlowValidationResult> {
    let mut results = Vec::new();
    for flow in flows {
        let mut missing_steps = Vec::new();
        let mut shortcuts = Vec::new();

        // 检查每个 step 是否在 actual edges 中存在
        for step in &flow.steps {
            if !actual_edges.contains(&(step.from.clone(), step.to.clone())) {
                missing_steps.push((
                    step.from.clone(),
                    step.to.clone(),
                    step.action.clone(),
                ));
            }
        }

        // 捷径检测：flow 定义 A→B→C，但 actual 存在 A→C 直接边
        // 遍历所有非相邻 step 对，检查是否存在跳跃边
        let nodes: Vec<&str> = {
            let mut ns = Vec::new();
            for step in &flow.steps {
                if ns.is_empty() || *ns.last().unwrap() != step.from.as_str() {
                    ns.push(step.from.as_str());
                }
                ns.push(step.to.as_str());
            }
            ns
        };
        // 对于序列 [A, B, C, D]，检查 A→C, A→D, B→D 等跳跃边
        for i in 0..nodes.len() {
            for j in (i + 2)..nodes.len() {
                let from = nodes[i].to_string();
                let to = nodes[j].to_string();
                if actual_edges.contains(&(from.clone(), to.clone())) {
                    // 收集被跳过的中间模块
                    let bypassed: Vec<&str> = nodes[i + 1..j].to_vec();
                    let bypassed_str = bypassed.join("→");
                    shortcuts.push((from, to, bypassed_str));
                }
            }
        }

        results.push(FlowValidationResult {
            flow_name: flow.name.clone(),
            missing_steps,
            shortcuts,
        });
    }
    results
}

/// 渲染 Mermaid sequenceDiagram
fn render_mermaid_flows(flows: &[SeedFlow], module_map: &HashMap<&str, &SeedModule>) -> String {
    let mut out = String::new();
    for (i, flow) in flows.iter().enumerate() {
        if i > 0 {
            out.push('\n');
        }
        out.push_str(&format!("%% 流程: {}\n", flow.name));
        out.push_str("sequenceDiagram\n");

        // 收集参与者（保持出现顺序）
        let mut participants: Vec<&str> = Vec::new();
        let mut seen: HashSet<&str> = HashSet::new();
        for step in &flow.steps {
            if seen.insert(step.from.as_str()) {
                participants.push(step.from.as_str());
            }
            if seen.insert(step.to.as_str()) {
                participants.push(step.to.as_str());
            }
        }

        // 输出 participant 声明
        for pid in &participants {
            let display = module_map
                .get(pid)
                .and_then(|m| m.display_name.as_deref())
                .unwrap_or(pid);
            out.push_str(&format!("    participant {} as {}\n", pid, display));
        }

        // 输出步骤
        for step in &flow.steps {
            let action = step.action.as_deref().unwrap_or(" ");
            out.push_str(&format!(
                "    {}->>{}:{}\n",
                step.from, step.to, action
            ));
        }
    }
    out
}

/// 校验接口边界定义
///
/// 两层校验：
/// 1. seed 级：检查 boundary 中的 module_id 是否存在于 seed modules
/// 2. 文件级（需要 ast_snapshot + file_to_module）：检测外部模块是否绕过公共 API 直接引用内部文件
fn validate_boundaries(
    boundaries: &[SeedBoundary],
    module_ids: &HashSet<&str>,
    file_to_module: Option<&HashMap<String, String>>,
    ast: Option<&AstSnapshot>,
) -> Vec<BoundaryValidationResult> {
    let mut results = Vec::new();

    for boundary in boundaries {
        // seed 级：检查 module_id 是否存在
        if !module_ids.contains(boundary.module_id.as_str()) {
            eprintln!(
                "[boundary] 警告: boundary 引用了不存在的 module_id '{}'",
                boundary.module_id
            );
            continue;
        }

        // 编译 public_paths 为 regex
        let public_regexes: Vec<Regex> = boundary
            .public_paths
            .iter()
            .filter_map(|g| match glob_to_regex(g) {
                Ok(r) => Some(r),
                Err(e) => {
                    eprintln!("[boundary] glob 编译失败: {}", e);
                    None
                }
            })
            .collect();

        // 文件级泄漏检测
        let mut total_external_refs = 0usize;
        let mut leaked_refs = Vec::new();

        if let (Some(ftm), Some(ast_snap)) = (file_to_module, ast) {
            for record in &ast_snap.records {
                let src_mod = match ftm.get(&record.sourcePath) {
                    Some(m) => m,
                    None => continue,
                };
                // 只关注「外部模块引用当前 boundary 模块」的情况
                if src_mod == &boundary.module_id {
                    continue;
                }

                for dep in &record.localDependencies {
                    let dep_mod = match ftm.get(dep.as_str()) {
                        Some(m) => m,
                        None => continue,
                    };
                    if dep_mod != &boundary.module_id {
                        continue;
                    }

                    total_external_refs += 1;

                    // 检查目标文件是否匹配 public_paths
                    let dep_posix = to_posix(dep);
                    let is_public = public_regexes.iter().any(|r| r.is_match(&dep_posix));
                    if !is_public {
                        leaked_refs.push(BoundaryLeak {
                            source_file: record.sourcePath.clone(),
                            source_module: src_mod.clone(),
                            target_file: dep.clone(),
                            target_module: boundary.module_id.clone(),
                        });
                    }
                }
            }
        }

        results.push(BoundaryValidationResult {
            module_id: boundary.module_id.clone(),
            total_external_refs,
            leaked_refs,
        });
    }

    results
}

/// 渲染接口边界 Mermaid 图
fn render_mermaid_boundaries(
    boundaries: &[SeedBoundary],
    module_map: &HashMap<&str, &SeedModule>,
    validation_results: &[BoundaryValidationResult],
) -> String {
    let mut out = String::from("graph LR\n");

    for (i, boundary) in boundaries.iter().enumerate() {
        let display = module_map
            .get(boundary.module_id.as_str())
            .and_then(|m| m.display_name.as_deref())
            .unwrap_or(&boundary.module_id);

        let sg_id = format!("bd_{}", boundary.module_id);
        out.push_str(&format!(
            "  subgraph {}[\"{}\"]\n",
            sg_id, display
        ));
        let pub_id = format!("bd_{}_pub", boundary.module_id);
        let int_id = format!("bd_{}_int", boundary.module_id);
        out.push_str(&format!(
            "    {}[\"📤 公共 API\"]\n",
            pub_id
        ));
        out.push_str(&format!(
            "    {}[\"🔒 内部实现\"]\n",
            int_id
        ));
        out.push_str("  end\n");

        // 如果有校验结果，渲染泄漏边
        if let Some(result) = validation_results.get(i) {
            // 收集泄漏来源模块（去重）
            let leak_sources: BTreeSet<&str> = result
                .leaked_refs
                .iter()
                .map(|l| l.source_module.as_str())
                .collect();

            for src_mod in &leak_sources {
                let src_display = module_map
                    .get(src_mod)
                    .and_then(|m| m.display_name.as_deref())
                    .unwrap_or(src_mod);
                let src_node = format!("leak_src_{}", src_mod);
                out.push_str(&format!("  {}[\"{}\"]\n", src_node, src_display));
                out.push_str(&format!(
                    "  {} -..->|\"⚠️ 泄漏\"| {}\n",
                    src_node, int_id
                ));
            }

            // 合规引用（如果有外部引用且不全是泄漏）
            if result.total_external_refs > result.leaked_refs.len() {
                let ok_node = format!("bd_{}_ok", boundary.module_id);
                out.push_str(&format!("  {}[\"合规调用方\"]\n", ok_node));
                out.push_str(&format!(
                    "  {} -->|\"✅ 合规\"| {}\n",
                    ok_node, pub_id
                ));
            }
        }
    }

    out
}

/// 校验事件流定义
fn validate_events(
    events: &[SeedEvent],
    module_ids: &HashSet<&str>,
    actual_edges: &HashSet<(String, String)>,
) -> Vec<EventValidationResult> {
    let mut results = Vec::new();

    for event in events {
        let orphan = event.consumers.is_empty();

        let ghost_consumers: Vec<String> = event
            .consumers
            .iter()
            .filter(|c| !module_ids.contains(c.as_str()))
            .cloned()
            .collect();

        let mut missing_edges = Vec::new();
        let mut reverse_deps = Vec::new();

        for producer in &event.producers {
            for consumer in &event.consumers {
                // 检查 producer → consumer 是否在 actual edges 中
                if !actual_edges.contains(&(producer.clone(), consumer.clone())) {
                    missing_edges.push((producer.clone(), consumer.clone()));
                }
                // 检查 consumer → producer 反向依赖
                if actual_edges.contains(&(consumer.clone(), producer.clone())) {
                    reverse_deps.push((consumer.clone(), producer.clone()));
                }
            }
        }

        results.push(EventValidationResult {
            event_name: event.name.clone(),
            orphan,
            ghost_consumers,
            missing_edges,
            reverse_deps,
        });
    }

    results
}

/// 渲染事件流 Mermaid 图
fn render_mermaid_events(
    events: &[SeedEvent],
    module_map: &HashMap<&str, &SeedModule>,
) -> String {
    let mut out = String::from("graph LR\n");

    for event in events {
        // 事件节点用菱形样式
        let evt_id = format!("evt_{}", event.name.replace(' ', "_").replace(|c: char| !c.is_alphanumeric() && c != '_', ""));
        out.push_str(&format!(
            "  {}{{\"{}\"}}\n",
            evt_id, event.name
        ));

        // 生产者 → 事件
        for producer in &event.producers {
            let display = module_map
                .get(producer.as_str())
                .and_then(|m| m.display_name.as_deref())
                .unwrap_or(producer);
            out.push_str(&format!(
                "  {}[\"{}\"] -->|publish| {}\n",
                producer, display, evt_id
            ));
        }

        // 事件 → 消费者
        for consumer in &event.consumers {
            let display = module_map
                .get(consumer.as_str())
                .and_then(|m| m.display_name.as_deref())
                .unwrap_or(consumer);
            out.push_str(&format!(
                "  {} -->|subscribe| {}[\"{}\"] \n",
                evt_id, consumer, display
            ));
        }
    }

    out
}

/// 流程数据连续性问题
#[derive(Debug)]
struct FlowDataIssue {
    flow_name: String,
    step_index: usize,
    from: String,
    to: String,
    missing_field: String,
}

/// 校验流程步骤间的数据连续性：step[i].output 是否覆盖 step[i+1].input
fn validate_flow_data_continuity(flows: &[SeedFlow]) -> Vec<FlowDataIssue> {
    let mut issues = Vec::new();
    for flow in flows {
        for i in 0..flow.steps.len().saturating_sub(1) {
            let curr = &flow.steps[i];
            let next = &flow.steps[i + 1];
            // 只在两边都声明了 input/output 时才校验
            if curr.output.is_empty() || next.input.is_empty() {
                continue;
            }
            for field in next.input.keys() {
                if !curr.output.contains_key(field) {
                    issues.push(FlowDataIssue {
                        flow_name: flow.name.clone(),
                        step_index: i,
                        from: curr.to.clone(),
                        to: next.to.clone(),
                        missing_field: field.clone(),
                    });
                }
            }
        }
    }
    issues
}

/// 格式化 HashMap<String, String> 为 "field1:type1, field2:type2" 形式
fn format_fields(fields: &HashMap<String, String>) -> String {
    let mut pairs: Vec<String> = fields
        .iter()
        .map(|(k, v)| format!("{}:{}", k, v))
        .collect();
    pairs.sort();
    pairs.join(", ")
}

/// 将 seed 字段类型映射为 Phoenix/Ecto 字段类型
fn map_field_type_to_ecto(ty: &str) -> &str {
    match ty {
        "string" | "text" => "string",
        "integer" | "int" => "integer",
        "float" | "double" => "float",
        "decimal" | "money" => "decimal",
        "boolean" | "bool" => "boolean",
        "date" => "date",
        "time" => "time",
        "datetime" | "timestamp" => "utc_datetime",
        "uuid" => "uuid",
        "map" | "json" | "object" => "map",
        "list" | "array" => "array",
        "binary" | "blob" => "binary",
        _ => "string",
    }
}

/// 从 module_id 推导 Phoenix context/schema 名称
/// 例：order_service → (Orders, Order, orders)
fn derive_phoenix_names(module_id: &str, contract_name: &str) -> (String, String, String) {
    // schema 名称：contract name 首字母大写，单数
    let schema = to_pascal_case(contract_name);
    // context 名称：schema + "s" 首字母大写
    let context = format!("{}s", schema);
    // 表名：module_id 作为前缀的复数小写
    let table = format!("{}s", contract_name.to_lowercase());
    let _ = module_id; // 预留 module_id 用于更智能的推导
    (context, schema, table)
}

fn to_pascal_case(s: &str) -> String {
    s.split('_')
        .map(|w| {
            let mut c = w.chars();
            match c.next() {
                None => String::new(),
                Some(f) => f.to_uppercase().to_string() + &c.as_str().to_lowercase(),
            }
        })
        .collect()
}

/// 从 CRUD contract 生成 mix phx.gen.context 命令
fn generate_crud_mix_command(
    module_id: &str,
    contract: &BoundaryContract,
) -> String {
    let (context, schema, table) = derive_phoenix_names(module_id, &contract.name);

    // 优先用 fields，没有则从 input+output 合并
    let fields: BTreeMap<String, String> = if !contract.fields.is_empty() {
        contract.fields.iter().map(|(k, v)| (k.clone(), v.clone())).collect()
    } else {
        let mut merged = BTreeMap::new();
        for (k, v) in &contract.input {
            merged.insert(k.clone(), v.clone());
        }
        for (k, v) in &contract.output {
            merged.insert(k.clone(), v.clone());
        }
        merged
    };

    let field_args: Vec<String> = fields
        .iter()
        .filter(|(k, _)| *k != "id" && !k.ends_with("_at"))
        .map(|(k, v)| {
            // uuid 且以 _id 结尾 → references
            if v == "uuid" && k.ends_with("_id") {
                let ref_table = k.trim_end_matches("_id");
                format!("{}:references:{}s", k, ref_table)
            } else {
                format!("{}:{}", k, map_field_type_to_ecto(v))
            }
        })
        .collect();

    format!(
        "mix phx.gen.context {} {} {} {}",
        context,
        schema,
        table,
        field_args.join(" ")
    )
}

/// 将 seed 类型映射为 Elixir typespec
fn seed_type_to_elixir_typespec(t: &str) -> &str {
    match t {
        "string" | "text" | "uuid" => "String.t()",
        "integer" | "int" => "integer()",
        "float" | "double" | "decimal" | "money" => "float()",
        "boolean" | "bool" => "boolean()",
        "map" | "json" | "object" => "map()",
        "list" | "array" => "list()",
        "timestamp" | "datetime" => "DateTime.t()",
        _ => "any()",
    }
}

/// 从 boundary contracts 生成 Elixir 模块骨架源码
fn generate_elixir_skeleton(
    module_id: &str,
    contracts: &[&BoundaryContract],
    complexity: &ContractComplexity,
) -> String {
    let module_name = to_pascal_case(module_id);
    let mut out = String::new();

    out.push_str(&format!("defmodule MyApp.{} do\n", module_name));
    out.push_str(&format!(
        "  @moduledoc \"\"\"\n  {} 模块骨架（{:?}）\n\n  由 bcc arch generate 自动生成\n  \"\"\"\n\n",
        module_name, complexity
    ));

    for contract in contracts {
        let kind = contract.kind.as_str();

        // 参数列表
        let param_names: Vec<&String> = contract.input.keys().collect::<Vec<_>>();
        let mut sorted_params = param_names.clone();
        sorted_params.sort();

        // @spec
        let spec_params: Vec<String> = sorted_params
            .iter()
            .map(|k| {
                let t = contract.input.get(*k).map(|s| s.as_str()).unwrap_or("any");
                seed_type_to_elixir_typespec(t).to_string()
            })
            .collect();
        let return_type = match kind {
            "query" => "{:ok, map()} | {:error, :not_found}",
            _ => "{:ok, map()} | {:error, term()}",
        };
        out.push_str(&format!(
            "  @spec {}({}) :: {}\n",
            contract.name,
            spec_params.join(", "),
            return_type
        ));

        // def
        let param_str = sorted_params
            .iter()
            .map(|s| s.as_str())
            .collect::<Vec<_>>()
            .join(", ");
        out.push_str(&format!("  def {}({}) do\n", contract.name, param_str));
        out.push_str("    # TODO: 实现业务逻辑\n");
        out.push_str("    {:ok, nil}\n");
        out.push_str("  end\n\n");
    }

    let out = out.trim_end().to_string();
    format!("{}\nend\n", out)
}

/// 从 seed 生成所有代码产出（CRUD 命令 + Elixir 骨架）
fn generate_from_seed(seed: &SeedSpec) -> Vec<(String, String, ContractComplexity, String)> {
    // 返回 (module_id, contract_name, complexity, output)
    let mut results = Vec::new();

    // 先按 module_id 分组非 CRUD contracts，合并生成一个模块文件
    let mut skeleton_groups: HashMap<String, (ContractComplexity, Vec<&BoundaryContract>)> =
        HashMap::new();

    for boundary in &seed.boundaries {
        for contract in &boundary.contracts {
            let complexity = classify_contract(contract, &boundary.module_id, seed);
            match complexity {
                ContractComplexity::Crud => {
                    let output = generate_crud_mix_command(&boundary.module_id, contract);
                    results.push((
                        boundary.module_id.clone(),
                        contract.name.clone(),
                        complexity,
                        output,
                    ));
                }
                _ => {
                    let entry = skeleton_groups
                        .entry(boundary.module_id.clone())
                        .or_insert_with(|| (complexity.clone(), Vec::new()));
                    entry.1.push(contract);
                }
            }
        }
    }

    // 为每个非 CRUD 模块生成合并的骨架
    let mut sorted_modules: Vec<_> = skeleton_groups.into_iter().collect();
    sorted_modules.sort_by(|a, b| a.0.cmp(&b.0));
    for (module_id, (complexity, contracts)) in sorted_modules {
        let skeleton = generate_elixir_skeleton(&module_id, &contracts, &complexity);
        let names: Vec<&str> = contracts.iter().map(|c| c.name.as_str()).collect();
        results.push((
            module_id,
            names.join("+"),
            complexity,
            skeleton,
        ));
    }

    results
}

fn normalize_key(raw: &str) -> String {
    let mut normalized = String::new();
    let mut last_is_sep = false;
    for ch in raw.chars() {
        if ch.is_ascii_alphanumeric() {
            normalized.push(ch.to_ascii_lowercase());
            last_is_sep = false;
        } else if !last_is_sep {
            normalized.push('_');
            last_is_sep = true;
        }
    }
    let trimmed = normalized.trim_matches('_').to_string();
    if trimmed.is_empty() {
        "unnamed".to_string()
    } else {
        trimmed
    }
}

fn normalize_contract_kind(raw: &str) -> String {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        default_contract_kind()
    } else {
        trimmed.to_ascii_lowercase()
    }
}

fn to_sorted_map(raw: &HashMap<String, String>) -> BTreeMap<String, String> {
    raw.iter().map(|(k, v)| (k.clone(), v.clone())).collect()
}

fn resolve_contract_fields(contract: &BoundaryContract) -> BTreeMap<String, String> {
    if !contract.fields.is_empty() {
        return to_sorted_map(&contract.fields);
    }
    let mut merged = BTreeMap::new();
    for (k, v) in &contract.input {
        merged.insert(k.clone(), v.clone());
    }
    for (k, v) in &contract.output {
        merged.insert(k.clone(), v.clone());
    }
    merged
}

fn build_action(
    action: &str,
    graphql_kind: &str,
    input: BTreeMap<String, String>,
    output: BTreeMap<String, String>,
    errors: &[String],
) -> UniboActionContract {
    UniboActionContract {
        action_key: normalize_key(action),
        action: action.to_string(),
        graphql_kind: graphql_kind.to_string(),
        input,
        output,
        errors: errors.to_vec(),
    }
}

fn build_unibo_contract_item(
    module_id: &str,
    contract: &BoundaryContract,
) -> Result<UniboBoundaryContract, String> {
    let source_kind = normalize_contract_kind(&contract.kind);
    let fields = resolve_contract_fields(contract);
    let contract_key = format!(
        "{}.{}",
        normalize_key(module_id),
        normalize_key(&contract.name)
    );
    let mut actions = match source_kind.as_str() {
        "query" => vec![build_action(
            &contract.name,
            "query",
            to_sorted_map(&contract.input),
            to_sorted_map(&contract.output),
            &contract.errors,
        )],
        "command" => vec![build_action(
            &contract.name,
            "mutation",
            to_sorted_map(&contract.input),
            to_sorted_map(&contract.output),
            &contract.errors,
        )],
        "crud" => {
            let mut read_delete_input = BTreeMap::new();
            read_delete_input.insert("id".to_string(), "uuid".to_string());

            let mut update_input = fields.clone();
            update_input.insert("id".to_string(), "uuid".to_string());

            let mut delete_output = BTreeMap::new();
            delete_output.insert("deleted".to_string(), "boolean".to_string());

            let mut list_output = BTreeMap::new();
            list_output.insert("items".to_string(), "array".to_string());

            vec![
                build_action(
                    "create",
                    "mutation",
                    fields.clone(),
                    fields.clone(),
                    &contract.errors,
                ),
                build_action(
                    "read",
                    "query",
                    read_delete_input.clone(),
                    fields.clone(),
                    &contract.errors,
                ),
                build_action(
                    "update",
                    "mutation",
                    update_input,
                    fields.clone(),
                    &contract.errors,
                ),
                build_action(
                    "delete",
                    "mutation",
                    read_delete_input,
                    delete_output,
                    &contract.errors,
                ),
                build_action(
                    "list",
                    "query",
                    BTreeMap::new(),
                    list_output,
                    &contract.errors,
                ),
            ]
        }
        other => {
            return Err(format!(
                "unsupported contract kind `{}` for {}.{} (expected query|command|crud)",
                other, module_id, contract.name
            ));
        }
    };
    actions.sort_by(|a, b| a.action_key.cmp(&b.action_key));

    Ok(UniboBoundaryContract {
        contract_key,
        module_id: module_id.to_string(),
        name: contract.name.clone(),
        source_kind: source_kind.clone(),
        graphql_kind: match source_kind.as_str() {
            "query" => "query".to_string(),
            "command" => "mutation".to_string(),
            _ => "mixed".to_string(),
        },
        fields,
        actions,
    })
}

#[derive(Debug)]
enum UniboContractBuildError {
    Conflict(String),
    Other(String),
}

fn build_unibo_api_contract_document(
    seed: &SeedSpec,
    strategy: ConflictStrategy,
) -> Result<UniboApiContractDocument, UniboContractBuildError> {
    let mut contracts_by_key: BTreeMap<String, UniboBoundaryContract> = BTreeMap::new();
    let mut conflicts: BTreeMap<String, Vec<String>> = BTreeMap::new();

    for boundary in &seed.boundaries {
        for contract in &boundary.contracts {
            let item = build_unibo_contract_item(&boundary.module_id, contract)
                .map_err(UniboContractBuildError::Other)?;
            let source = format!("{}.{}", boundary.module_id, contract.name);
            if let Some(prev) = contracts_by_key.get(&item.contract_key) {
                match strategy {
                    ConflictStrategy::ErrorOnConflict => {
                        let entry = conflicts.entry(item.contract_key.clone()).or_default();
                        if entry.is_empty() {
                            entry.push(format!("{}.{}", prev.module_id, prev.name));
                        }
                        entry.push(source);
                    }
                    ConflictStrategy::Dedupe => {
                        eprintln!(
                            "[generate] warning: contract key conflict `{}`，保留后者 {}（覆盖 {}.{}）",
                            item.contract_key, source, prev.module_id, prev.name
                        );
                        contracts_by_key.insert(item.contract_key.clone(), item);
                    }
                }
            } else {
                contracts_by_key.insert(item.contract_key.clone(), item);
            }
        }
    }

    if !conflicts.is_empty() {
        let mut detail = Vec::new();
        for (key, mut sources) in conflicts {
            sources.sort();
            sources.dedup();
            detail.push(format!("{} => {}", key, sources.join(" | ")));
        }
        return Err(UniboContractBuildError::Conflict(format!(
            "contract key conflicts detected: {}",
            detail.join("; ")
        )));
    }

    if contracts_by_key.is_empty() {
        return Err(UniboContractBuildError::Other(
            "seed 缺少 contracts：--emit api-contract 需要 boundaries[].contracts".to_string(),
        ));
    }

    Ok(UniboApiContractDocument {
        bridge_version: UNIBO_BRIDGE_VERSION.to_string(),
        target_runtime_version: UNIBO_TARGET_RUNTIME_VERSION.to_string(),
        compat_version: UNIBO_COMPAT_VERSION.to_string(),
        producer: UniboContractProducer {
            name: env!("CARGO_PKG_NAME").to_string(),
            version: env!("CARGO_PKG_VERSION").to_string(),
        },
        seed_version: seed
            .version
            .clone()
            .unwrap_or_else(|| "unspecified".to_string()),
        generated_at: Utc::now().to_rfc3339(),
        contracts: contracts_by_key.into_values().collect(),
    })
}

fn build_legacy_api_contract_document(seed: &SeedSpec) -> Result<LegacyApiContractDocument, String> {
    let mut contracts = Vec::new();
    for boundary in &seed.boundaries {
        for contract in &boundary.contracts {
            contracts.push(LegacyApiContractItem {
                module_id: boundary.module_id.clone(),
                name: contract.name.clone(),
                kind: normalize_contract_kind(&contract.kind),
                input: to_sorted_map(&contract.input),
                output: to_sorted_map(&contract.output),
                errors: contract.errors.clone(),
                fields: to_sorted_map(&contract.fields),
            });
        }
    }

    if contracts.is_empty() {
        return Err(
            "seed 缺少 contracts：--emit api-contract 需要 boundaries[].contracts".to_string(),
        );
    }

    contracts.sort_by(|a, b| {
        a.module_id
            .cmp(&b.module_id)
            .then_with(|| a.name.cmp(&b.name))
    });

    Ok(LegacyApiContractDocument {
        contract_schema_version: LEGACY_API_CONTRACT_SCHEMA_VERSION.to_string(),
        producer: LegacyApiContractProducer {
            name: env!("CARGO_PKG_NAME").to_string(),
            version: env!("CARGO_PKG_VERSION").to_string(),
        },
        seed_version: seed
            .version
            .clone()
            .unwrap_or_else(|| "unspecified".to_string()),
        generated_at: Utc::now().to_rfc3339(),
        contracts,
    })
}

fn build_unibo_runtime_bridge_config() -> UniboRuntimeBridgeConfig {
    UniboRuntimeBridgeConfig {
        bridge_version: UNIBO_BRIDGE_VERSION.to_string(),
        target_runtime_version: UNIBO_TARGET_RUNTIME_VERSION.to_string(),
        compat_version: UNIBO_COMPAT_VERSION.to_string(),
        runtime: RuntimeBindingConfig {
            package: "unibo_graphql_runtime".to_string(),
            mode: "reuse".to_string(),
        },
        contract_source: ContractSourceConfig {
            path: format!("./{}", UNIBO_API_CONTRACT_FILE),
            format: "json".to_string(),
        },
    }
}

fn write_unibo_api_contract_document(
    document: &UniboApiContractDocument,
    output_dir: Option<&str>,
) -> Result<(), String> {
    let payload = serde_json::to_string_pretty(document)
        .map_err(|e| format!("serialize api-contract failed: {}", e))?;
    match output_dir {
        Some(dir) => {
            let out_path = Path::new(dir);
            fs::create_dir_all(out_path).map_err(|e| format!("create output dir failed: {}", e))?;
            let file_path = out_path.join(UNIBO_API_CONTRACT_FILE);
            fs::write(&file_path, format!("{}\n", payload))
                .map_err(|e| format!("write {} failed: {}", file_path.display(), e))?;
            eprintln!(
                "[generate] 已输出 UniBO API Contract 到 {}（contracts: {}）",
                file_path.display(),
                document.contracts.len()
            );
        }
        None => {
            eprintln!(
                "[generate] 未指定 --output，保留 stdout 给兼容层；UniBO 契约请使用 --output 获取 {}",
                UNIBO_API_CONTRACT_FILE
            );
            eprintln!("{}", payload);
        }
    }
    Ok(())
}

fn write_legacy_api_contract_document(
    document: &LegacyApiContractDocument,
    output_dir: Option<&str>,
) -> Result<(), String> {
    let payload = serde_json::to_string_pretty(document)
        .map_err(|e| format!("serialize legacy api-contract failed: {}", e))?;
    match output_dir {
        Some(dir) => {
            let out_path = Path::new(dir);
            fs::create_dir_all(out_path).map_err(|e| format!("create output dir failed: {}", e))?;
            let file_path = out_path.join(LEGACY_API_CONTRACT_FILE);
            fs::write(&file_path, format!("{}\n", payload))
                .map_err(|e| format!("write {} failed: {}", file_path.display(), e))?;
            eprintln!(
                "[generate] 已输出兼容层 API Contract 到 {}（contracts: {}）",
                file_path.display(),
                document.contracts.len()
            );
        }
        None => {
            println!("{}", payload);
        }
    }
    Ok(())
}

fn write_unibo_runtime_bridge_config(
    config: &UniboRuntimeBridgeConfig,
    output_dir: Option<&str>,
) -> Result<(), String> {
    let payload = serde_yaml::to_string(config)
        .map_err(|e| format!("serialize runtime-bridge failed: {}", e))?;
    match output_dir {
        Some(dir) => {
            let out_path = Path::new(dir);
            fs::create_dir_all(out_path).map_err(|e| format!("create output dir failed: {}", e))?;
            let file_path = out_path.join(UNIBO_RUNTIME_BRIDGE_FILE);
            fs::write(&file_path, payload)
                .map_err(|e| format!("write {} failed: {}", file_path.display(), e))?;
            eprintln!(
                "[generate] 已输出 UniBO runtime bridge 配置到 {}",
                file_path.display()
            );
        }
        None => {
            eprintln!(
                "[generate] 未指定 --output，runtime bridge 改为输出到 stderr 以保持 stdout JSON 可机读"
            );
            eprintln!("{}", payload);
        }
    }
    Ok(())
}

/// 公共入口：从 seed 生成代码
pub fn generate(
    seed_file: &str,
    emit: &str,
    output_dir: Option<&str>,
    emit_runtime_bridge: bool,
    conflict_strategy: &str,
) {
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

    // 按 emit 模式输出
    match emit {
        "code" | "all" => {
            if emit == "all" {
                eprintln!("[generate] emit=all 当前与 emit=code 等价（兼容保留）");
            }
            let results = generate_from_seed(&seed);
            if results.is_empty() {
                eprintln!("[generate] seed 中没有 boundary contracts，无产出");
                return;
            }
            let crud_count = results
                .iter()
                .filter(|(_, _, c, _)| matches!(c, ContractComplexity::Crud))
                .count();
            let other_count = results.len() - crud_count;

            if let Some(dir) = output_dir {
                let out_path = Path::new(dir);
                fs::create_dir_all(out_path).unwrap_or_else(|e| {
                    eprintln!("create output dir failed: {}", e);
                    std::process::exit(1);
                });

                // CRUD 命令写到 generate-commands.sh
                let mut crud_lines = Vec::new();
                for (module_id, name, complexity, output) in &results {
                    if matches!(complexity, ContractComplexity::Crud) {
                        crud_lines.push(format!("# [CRUD] {}.{}", module_id, name));
                        crud_lines.push(output.clone());
                        crud_lines.push(String::new());
                    }
                }
                if !crud_lines.is_empty() {
                    let cmd_path = out_path.join("generate-commands.sh");
                    if let Err(e) = fs::write(&cmd_path, crud_lines.join("\n")) {
                        eprintln!("write commands failed: {}", e);
                    }
                }

                // 非 CRUD 骨架写到单独 .ex 文件
                for (module_id, _name, complexity, output) in &results {
                    if !matches!(complexity, ContractComplexity::Crud) {
                        let file_name = format!("{}.ex", module_id);
                        let file_path = out_path.join(&file_name);
                        if let Err(e) = fs::write(&file_path, output) {
                            eprintln!("write {} failed: {}", file_name, e);
                        }
                    }
                }

                eprintln!(
                    "[generate] 已输出到 {}（CRUD: {}, 骨架: {}）",
                    out_path.display(),
                    crud_count,
                    other_count
                );
            } else {
                // stdout 模式：混合输出所有内容
                for (module_id, name, complexity, output) in &results {
                    match complexity {
                        ContractComplexity::Crud => {
                            println!("# [CRUD] {}.{}", module_id, name);
                        }
                        _ => {
                            println!("# [{:?}] {}.{}", complexity, module_id, name);
                        }
                    }
                    println!("{}\n", output);
                }
                eprintln!("[generate] CRUD: {}, 骨架: {}", crud_count, other_count);
            }
        }
        "api-contract" => {
            let strategy = match ConflictStrategy::parse(conflict_strategy) {
                Ok(strategy) => strategy,
                Err(e) => {
                    eprintln!("[generate] {}", e);
                    std::process::exit(GENERATE_EXIT_INVALID_ARGUMENT);
                }
            };
            let document = match build_unibo_api_contract_document(&seed, strategy) {
                Ok(doc) => doc,
                Err(UniboContractBuildError::Conflict(e)) => {
                    eprintln!("[generate] {}", e);
                    std::process::exit(GENERATE_EXIT_CONTRACT_CONFLICT);
                }
                Err(UniboContractBuildError::Other(e)) => {
                    eprintln!("[generate] {}", e);
                    std::process::exit(1);
                }
            };
            let legacy_document = match build_legacy_api_contract_document(&seed) {
                Ok(doc) => doc,
                Err(e) => {
                    eprintln!("[generate] {}", e);
                    std::process::exit(1);
                }
            };
            if let Err(e) = write_unibo_api_contract_document(&document, output_dir) {
                eprintln!("[generate] {}", e);
                std::process::exit(GENERATE_EXIT_OUTPUT_FAILED);
            }
            if let Err(e) = write_legacy_api_contract_document(&legacy_document, output_dir) {
                eprintln!("[generate] {}", e);
                std::process::exit(GENERATE_EXIT_OUTPUT_FAILED);
            }
            if emit_runtime_bridge {
                let runtime_bridge = build_unibo_runtime_bridge_config();
                if let Err(e) = write_unibo_runtime_bridge_config(&runtime_bridge, output_dir) {
                    eprintln!("[generate] {}", e);
                    std::process::exit(GENERATE_EXIT_OUTPUT_FAILED);
                }
            } else {
                eprintln!("[generate] emit-runtime-bridge=false，仅输出契约桥接产物");
            }
        }
        other => {
            eprintln!("[generate] 未知 emit 模式: {}", other);
            eprintln!("[generate] 可选值: code | api-contract | all");
            std::process::exit(GENERATE_EXIT_INVALID_ARGUMENT);
        }
    }
}

/// 从 seed 契约声明导出 BDD source YAML 文件
fn export_bdd_sources_from_contracts(
    dir: &Path,
    seed: &SeedSpec,
) -> Result<(usize, usize, usize), String> {
    fs::create_dir_all(dir).map_err(|e| format!("create dir failed: {}", e))?;

    let mut flow_count = 0;
    let mut boundary_count = 0;
    let mut event_count = 0;

    // 1. flow steps 契约
    for flow in &seed.flows {
        for (i, step) in flow.steps.iter().enumerate() {
            if step.input.is_empty() && step.output.is_empty() {
                continue;
            }
            let action = step.action.as_deref().unwrap_or("处理");
            let input_str = format_fields(&step.input);
            let output_str = format_fields(&step.output);
            let summary = format!(
                "GIVEN 模块{}接收来自{}的请求，输入: {{{}}} / WHEN 执行{} / THEN 应输出: {{{}}}",
                step.to, step.from, input_str, action, output_str
            );
            let safe_flow = flow.name.replace(' ', "_").replace('/', "_");
            let filename = format!("flow_{}_{}_{}_{}.yaml", safe_flow, i, step.from, step.to);
            let content = format!(
                "module: {}\ncontract: {} -> {} flow step contract\nedge_class: flow_contract\nsource_file: seed-contract-export\nsource_summary: \"{}\"\n",
                step.to.to_ascii_uppercase(),
                step.from,
                step.to,
                summary
            );
            let path = dir.join(&filename);
            fs::write(&path, &content)
                .map_err(|e| format!("write {} failed: {}", path.display(), e))?;
            flow_count += 1;
        }
    }

    // 2. boundary contracts
    for boundary in &seed.boundaries {
        for contract in &boundary.contracts {
            let input_str = format_fields(&contract.input);
            let output_str = format_fields(&contract.output);
            let summary = format!(
                "GIVEN 外部调用模块{}的接口{}，输入: {{{}}} / WHEN 执行调用 / THEN 返回: {{{}}}",
                boundary.module_id, contract.name, input_str, output_str
            );
            let safe_name = contract.name.replace(' ', "_").replace('/', "_");
            let filename = format!("boundary_{}_{}.yaml", boundary.module_id, safe_name);
            let content = format!(
                "module: {}\ncontract: {} boundary contract ({})\nedge_class: boundary_contract\nsource_file: seed-contract-export\nsource_summary: \"{}\"\n",
                boundary.module_id.to_ascii_uppercase(),
                boundary.module_id,
                contract.name,
                summary
            );
            let path = dir.join(&filename);
            fs::write(&path, &content)
                .map_err(|e| format!("write {} failed: {}", path.display(), e))?;
            boundary_count += 1;
        }
    }

    // 3. event payload contracts
    for event in &seed.events {
        if event.payload.is_empty() {
            continue;
        }
        let payload_str = format_fields(&event.payload);
        let producer_str = event.producers.join(", ");
        for consumer in &event.consumers {
            let summary = format!(
                "GIVEN 事件{}由{}发布，payload: {{{}}} / WHEN 消费者{}接收 / THEN 应解析: {{{}}}",
                event.name, producer_str, payload_str, consumer, payload_str
            );
            let safe_event = event.name.replace(' ', "_").replace('/', "_");
            let filename = format!("event_{}_{}.yaml", safe_event, consumer);
            let content = format!(
                "module: {}\ncontract: event {} consumer contract\nedge_class: event_contract\nsource_file: seed-contract-export\nsource_summary: \"{}\"\n",
                consumer.to_ascii_uppercase(),
                event.name,
                summary
            );
            let path = dir.join(&filename);
            fs::write(&path, &content)
                .map_err(|e| format!("write {} failed: {}", path.display(), e))?;
            event_count += 1;
        }
    }

    Ok((flow_count, boundary_count, event_count))
}

pub fn export_mermaid(seed_file: &str, ast_file: Option<&str>, output: Option<&str>, export_bdd_source: Option<&str>) {
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

    // === 行为契约校验与 BDD source 导出 ===
    if let Some(bdd_dir) = export_bdd_source {
        let bdd_path = Path::new(bdd_dir);

        // 数据连续性校验
        let data_issues = validate_flow_data_continuity(&seed.flows);
        if !data_issues.is_empty() {
            eprintln!("[contract] 流程数据连续性警告:");
            for issue in &data_issues {
                eprintln!(
                    "  流程「{}」step[{}] {}→{}: 缺失字段 {}",
                    issue.flow_name, issue.step_index, issue.from, issue.to, issue.missing_field
                );
            }
        }

        // 导出 BDD source 文件
        match export_bdd_sources_from_contracts(bdd_path, &seed) {
            Ok((fc, bc, ec)) => {
                let total = fc + bc + ec;
                eprintln!(
                    "[contract] 已导出 {} 个 BDD source 文件到 {}（flow: {}, boundary: {}, event: {}）",
                    total,
                    bdd_dir,
                    fc,
                    bc,
                    ec
                );
            }
            Err(e) => {
                eprintln!("[contract] BDD source 导出失败: {}", e);
            }
        }
    }

    // === 流程校验与 sequenceDiagram ===
    let flows_mermaid = if !seed.flows.is_empty() {
        // 从 relations_expected 中构建 actual edges 集合（allowed=true 的边）
        let actual_edges: HashSet<(String, String)> = seed
            .relations_expected
            .iter()
            .filter(|r| r.allowed)
            .map(|r| (r.caller.clone(), r.callee.clone()))
            .collect();

        // 校验
        let results = validate_flows(&seed.flows, &actual_edges);
        for result in &results {
            if !result.missing_steps.is_empty() || !result.shortcuts.is_empty() {
                eprintln!("[flow] 流程「{}」校验结果:", result.flow_name);
                for (from, to, action) in &result.missing_steps {
                    let act = action.as_deref().unwrap_or("");
                    eprintln!("  缺失步骤: {} → {} {}", from, to, act);
                }
                for (from, to, bypassed) in &result.shortcuts {
                    eprintln!(
                        "  捷径: {} → {} 跳过 {}",
                        from, to, bypassed
                    );
                }
            } else {
                eprintln!("[flow] 流程「{}」校验通过", result.flow_name);
            }
        }

        // 渲染 sequenceDiagram
        Some(render_mermaid_flows(&seed.flows, &module_map))
    } else {
        None
    };

    // === 接口边界校验与 Mermaid ===
    let module_ids: HashSet<&str> = seed.modules.iter().map(|m| m.module_id.as_str()).collect();

    let boundaries_mermaid = if !seed.boundaries.is_empty() {
        // 加载 ast 和 file_to_module（如果提供了 ast_file）
        let (ast_snap, file_to_mod) = if let Some(af) = ast_file {
            match fs::read_to_string(af) {
                Ok(raw) => match serde_json::from_str::<AstSnapshot>(&raw) {
                    Ok(ast) => {
                        let ftm = match map_files_to_modules(&seed, &ast) {
                            Ok(m) => m,
                            Err(e) => {
                                eprintln!("[boundary] file_to_module 映射失败: {}", e);
                                HashMap::new()
                            }
                        };
                        (Some(ast), Some(ftm))
                    }
                    Err(e) => {
                        eprintln!("[boundary] 解析 ast_file 失败: {}", e);
                        (None, None)
                    }
                },
                Err(e) => {
                    eprintln!("[boundary] 读取 ast_file 失败: {}", e);
                    (None, None)
                }
            }
        } else {
            (None, None)
        };

        let results = validate_boundaries(
            &seed.boundaries,
            &module_ids,
            file_to_mod.as_ref(),
            ast_snap.as_ref(),
        );

        // 输出校验结果到 stderr
        for result in &results {
            if result.leaked_refs.is_empty() {
                eprintln!(
                    "[boundary] 模块「{}」: {} 个外部引用，无泄漏",
                    result.module_id, result.total_external_refs
                );
            } else {
                eprintln!(
                    "[boundary] 模块「{}」: {} 个外部引用，{} 个泄漏:",
                    result.module_id, result.total_external_refs, result.leaked_refs.len()
                );
                for leak in &result.leaked_refs {
                    eprintln!(
                        "  {} ({}) → {} ({})",
                        leak.source_file, leak.source_module,
                        leak.target_file, leak.target_module
                    );
                }
            }
        }

        Some(render_mermaid_boundaries(
            &seed.boundaries,
            &module_map,
            &results,
        ))
    } else {
        None
    };

    // === 事件流校验与 Mermaid ===
    let events_mermaid = if !seed.events.is_empty() {
        let actual_edges: HashSet<(String, String)> = seed
            .relations_expected
            .iter()
            .filter(|r| r.allowed)
            .map(|r| (r.caller.clone(), r.callee.clone()))
            .collect();

        let results = validate_events(&seed.events, &module_ids, &actual_edges);

        // 输出校验结果到 stderr
        for result in &results {
            let mut issues = Vec::new();
            if result.orphan {
                issues.push("孤儿事件(无消费者)".to_string());
            }
            if !result.ghost_consumers.is_empty() {
                issues.push(format!(
                    "幽灵消费者: {}",
                    result.ghost_consumers.join(", ")
                ));
            }
            if !result.missing_edges.is_empty() {
                let edges: Vec<String> = result
                    .missing_edges
                    .iter()
                    .map(|(p, c)| format!("{}→{}", p, c))
                    .collect();
                issues.push(format!("缺失依赖边: {}", edges.join(", ")));
            }
            if !result.reverse_deps.is_empty() {
                let edges: Vec<String> = result
                    .reverse_deps
                    .iter()
                    .map(|(c, p)| format!("{}→{}", c, p))
                    .collect();
                issues.push(format!("反向依赖: {}", edges.join(", ")));
            }

            if issues.is_empty() {
                eprintln!("[event] 事件「{}」校验通过", result.event_name);
            } else {
                eprintln!(
                    "[event] 事件「{}」: {}",
                    result.event_name,
                    issues.join("; ")
                );
            }
        }

        Some(render_mermaid_events(&seed.events, &module_map))
    } else {
        None
    };

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
            if let Some(ref fm) = flows_mermaid {
                println!("{}", fm);
            }
            if let Some(ref bm) = boundaries_mermaid {
                println!();
                println!("%% === 接口边界图 ===");
                println!("{}", bm);
            }
            if let Some(ref em) = events_mermaid {
                println!();
                println!("%% === 事件流图 ===");
                println!("{}", em);
            }
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

            // 时序流程图（在总览图之前）
            if let Some(ref fm) = flows_mermaid {
                main_parts.push(format!(
                    "## 时序流程图\n\n```mermaid\n{}\n```",
                    fm.trim()
                ));
            }

            // 接口边界图
            if let Some(ref bm) = boundaries_mermaid {
                main_parts.push(format!(
                    "## 接口边界图\n\n```mermaid\n{}\n```",
                    bm.trim()
                ));
            }

            // 事件流图
            if let Some(ref em) = events_mermaid {
                main_parts.push(format!(
                    "## 事件流图\n\n```mermaid\n{}\n```",
                    em.trim()
                ));
            }

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
            flows: vec![],
            boundaries: vec![],
            events: vec![],
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
            flows: vec![],
            boundaries: vec![],
            events: vec![],
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
            flows: vec![],
            boundaries: vec![],
            events: vec![],
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
            flows: vec![],
            boundaries: vec![],
            events: vec![],
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
            flows: vec![],
            boundaries: vec![],
            events: vec![],
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
            flows: vec![],
            boundaries: vec![],
            events: vec![],
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
            flows: vec![],
            boundaries: vec![],
            events: vec![],
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
            flows: vec![],
            boundaries: vec![],
            events: vec![],
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
            flows: vec![],
            boundaries: vec![],
            events: vec![],
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
            flows: vec![],
            boundaries: vec![],
            events: vec![],
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

    #[test]
    fn flows_parse_from_yaml() {
        let yaml = r#"
version: "1.0"
modules:
  - module_id: A
    display_name: 模块A
  - module_id: B
    display_name: 模块B
  - module_id: C
    display_name: 模块C
relations_expected: []
flows:
  - name: "测试流程"
    steps:
      - from: A
        to: B
        action: "请求"
      - from: B
        to: C
"#;
        let seed: SeedSpec = serde_yaml::from_str(yaml).expect("parse");
        assert_eq!(seed.flows.len(), 1);
        assert_eq!(seed.flows[0].name, "测试流程");
        assert_eq!(seed.flows[0].steps.len(), 2);
        assert_eq!(seed.flows[0].steps[0].action, Some("请求".to_string()));
        assert_eq!(seed.flows[0].steps[1].action, None);
    }

    #[test]
    fn flows_absent_defaults_to_empty() {
        let yaml = r#"
version: "1.0"
modules: []
relations_expected: []
"#;
        let seed: SeedSpec = serde_yaml::from_str(yaml).expect("parse");
        assert!(seed.flows.is_empty());
    }

    #[test]
    fn validate_flows_detects_missing_steps() {
        let flows = vec![SeedFlow {
            name: "f1".into(),
            steps: vec![
                SeedFlowStep { from: "A".into(), to: "B".into(), action: Some("act1".into()), input: HashMap::new(), output: HashMap::new() },
                SeedFlowStep { from: "B".into(), to: "C".into(), action: None, input: HashMap::new(), output: HashMap::new() },
            ],
        }];
        // actual 只有 A→B，缺少 B→C
        let mut actual = HashSet::new();
        actual.insert(("A".into(), "B".into()));

        let results = validate_flows(&flows, &actual);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].missing_steps.len(), 1);
        assert_eq!(results[0].missing_steps[0].0, "B");
        assert_eq!(results[0].missing_steps[0].1, "C");
    }

    #[test]
    fn validate_flows_detects_shortcuts() {
        let flows = vec![SeedFlow {
            name: "f1".into(),
            steps: vec![
                SeedFlowStep { from: "A".into(), to: "B".into(), action: None, input: HashMap::new(), output: HashMap::new() },
                SeedFlowStep { from: "B".into(), to: "C".into(), action: None, input: HashMap::new(), output: HashMap::new() },
            ],
        }];
        // actual 有 A→B, B→C, 还有跳跃边 A→C
        let mut actual = HashSet::new();
        actual.insert(("A".into(), "B".into()));
        actual.insert(("B".into(), "C".into()));
        actual.insert(("A".into(), "C".into()));

        let results = validate_flows(&flows, &actual);
        assert_eq!(results.len(), 1);
        assert!(results[0].missing_steps.is_empty());
        assert_eq!(results[0].shortcuts.len(), 1);
        assert_eq!(results[0].shortcuts[0].0, "A");
        assert_eq!(results[0].shortcuts[0].1, "C");
        assert_eq!(results[0].shortcuts[0].2, "B");
    }

    #[test]
    fn validate_flows_all_pass() {
        let flows = vec![SeedFlow {
            name: "happy".into(),
            steps: vec![
                SeedFlowStep { from: "A".into(), to: "B".into(), action: None, input: HashMap::new(), output: HashMap::new() },
                SeedFlowStep { from: "B".into(), to: "C".into(), action: None, input: HashMap::new(), output: HashMap::new() },
            ],
        }];
        let mut actual = HashSet::new();
        actual.insert(("A".into(), "B".into()));
        actual.insert(("B".into(), "C".into()));

        let results = validate_flows(&flows, &actual);
        assert!(results[0].missing_steps.is_empty());
        assert!(results[0].shortcuts.is_empty());
    }

    #[test]
    fn render_mermaid_flows_basic() {
        let flows = vec![SeedFlow {
            name: "测试流程".into(),
            steps: vec![
                SeedFlowStep { from: "A".into(), to: "B".into(), action: Some("请求".into()), input: HashMap::new(), output: HashMap::new() },
                SeedFlowStep { from: "B".into(), to: "C".into(), action: None, input: HashMap::new(), output: HashMap::new() },
            ],
        }];
        let mod_a = SeedModule {
            module_id: "A".into(),
            display_name: Some("模块A".into()),
            layer: None, domain_kind: None, parent: None,
            precedence: None, path_rules: None,
        };
        let mod_b = SeedModule {
            module_id: "B".into(),
            display_name: Some("模块B".into()),
            layer: None, domain_kind: None, parent: None,
            precedence: None, path_rules: None,
        };
        let mod_c = SeedModule {
            module_id: "C".into(),
            display_name: None,
            layer: None, domain_kind: None, parent: None,
            precedence: None, path_rules: None,
        };
        let module_map: HashMap<&str, &SeedModule> = [
            ("A", &mod_a), ("B", &mod_b), ("C", &mod_c),
        ].into_iter().collect();

        let result = render_mermaid_flows(&flows, &module_map);
        assert!(result.contains("sequenceDiagram"));
        assert!(result.contains("participant A as 模块A"));
        assert!(result.contains("participant B as 模块B"));
        assert!(result.contains("participant C as C")); // 无 display_name 时用 module_id
        assert!(result.contains("A->>B:请求"));
        assert!(result.contains("B->>C: ")); // 无 action 时用空格
    }

    // ==================== boundary 测试 ====================

    #[test]
    fn validate_boundaries_seed_level_unknown_module() {
        let boundaries = vec![SeedBoundary {
            module_id: "nonexistent".into(),
            public_paths: vec!["src/api/**".into()],
            contracts: vec![],
        }];
        let module_ids: HashSet<&str> = ["mod_a", "mod_b"].into_iter().collect();

        let results = validate_boundaries(&boundaries, &module_ids, None, None);
        // 不存在的 module_id 被跳过
        assert!(results.is_empty());
    }

    #[test]
    fn validate_boundaries_seed_level_valid() {
        let boundaries = vec![SeedBoundary {
            module_id: "mod_a".into(),
            public_paths: vec!["src/api/**".into()],
            contracts: vec![],
        }];
        let module_ids: HashSet<&str> = ["mod_a", "mod_b"].into_iter().collect();

        let results = validate_boundaries(&boundaries, &module_ids, None, None);
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].module_id, "mod_a");
        assert_eq!(results[0].total_external_refs, 0);
        assert!(results[0].leaked_refs.is_empty());
    }

    #[test]
    fn validate_boundaries_file_level_leak_detection() {
        let boundaries = vec![SeedBoundary {
            module_id: "mod_a".into(),
            public_paths: vec!["src/mod_a/api/**".into()],
            contracts: vec![],
        }];
        let module_ids: HashSet<&str> = ["mod_a", "mod_b"].into_iter().collect();

        // 模拟 ast: mod_b 的文件引用了 mod_a 的内部文件
        let ast = AstSnapshot {
            source_count: 2,
            records: vec![AstRecord {
                sourcePath: "src/mod_b/handler.ts".into(),
                localDependencies: vec![
                    "src/mod_a/api/index.ts".into(),      // 合规
                    "src/mod_a/internal/helper.ts".into(), // 泄漏
                ],
                localCallTargets: vec![],
                relationHints: vec![],
            }],
        };
        let mut file_to_module = HashMap::new();
        file_to_module.insert("src/mod_b/handler.ts".into(), "mod_b".into());
        file_to_module.insert("src/mod_a/api/index.ts".into(), "mod_a".into());
        file_to_module.insert("src/mod_a/internal/helper.ts".into(), "mod_a".into());

        let results = validate_boundaries(
            &boundaries,
            &module_ids,
            Some(&file_to_module),
            Some(&ast),
        );

        assert_eq!(results.len(), 1);
        assert_eq!(results[0].total_external_refs, 2);
        assert_eq!(results[0].leaked_refs.len(), 1);
        assert_eq!(
            results[0].leaked_refs[0].target_file,
            "src/mod_a/internal/helper.ts"
        );
        assert_eq!(results[0].leaked_refs[0].source_module, "mod_b");
    }

    #[test]
    fn render_mermaid_boundaries_basic() {
        let boundaries = vec![SeedBoundary {
            module_id: "mod_a".into(),
            public_paths: vec!["src/api/**".into()],
            contracts: vec![],
        }];
        let mod_a = SeedModule {
            module_id: "mod_a".into(),
            display_name: Some("模块A".into()),
            layer: None,
            domain_kind: None,
            parent: None,
            precedence: None,
            path_rules: None,
        };
        let module_map: HashMap<&str, &SeedModule> = [("mod_a", &mod_a)].into_iter().collect();
        let results = vec![BoundaryValidationResult {
            module_id: "mod_a".into(),
            total_external_refs: 0,
            leaked_refs: vec![],
        }];

        let mermaid = render_mermaid_boundaries(&boundaries, &module_map, &results);
        assert!(mermaid.contains("graph LR"));
        assert!(mermaid.contains("模块A"));
        assert!(mermaid.contains("公共 API"));
        assert!(mermaid.contains("内部实现"));
    }

    // ==================== events 测试 ====================

    #[test]
    fn validate_events_orphan() {
        let events = vec![SeedEvent {
            name: "evt1".into(),
            producers: vec!["A".into()],
            consumers: vec![],
            payload: HashMap::new(),
        }];
        let module_ids: HashSet<&str> = ["A", "B"].into_iter().collect();
        let actual = HashSet::new();

        let results = validate_events(&events, &module_ids, &actual);
        assert_eq!(results.len(), 1);
        assert!(results[0].orphan);
    }

    #[test]
    fn validate_events_ghost_consumer() {
        let events = vec![SeedEvent {
            name: "evt1".into(),
            producers: vec!["A".into()],
            consumers: vec!["ghost_mod".into()],
            payload: HashMap::new(),
        }];
        let module_ids: HashSet<&str> = ["A", "B"].into_iter().collect();
        let actual = HashSet::new();

        let results = validate_events(&events, &module_ids, &actual);
        assert_eq!(results[0].ghost_consumers, vec!["ghost_mod"]);
    }

    #[test]
    fn validate_events_missing_and_reverse_edges() {
        let events = vec![SeedEvent {
            name: "evt1".into(),
            producers: vec!["A".into()],
            consumers: vec!["B".into()],
            payload: HashMap::new(),
        }];
        let module_ids: HashSet<&str> = ["A", "B"].into_iter().collect();
        // actual 只有 B→A（反向），缺少 A→B
        let mut actual = HashSet::new();
        actual.insert(("B".into(), "A".into()));

        let results = validate_events(&events, &module_ids, &actual);
        assert_eq!(results[0].missing_edges.len(), 1);
        assert_eq!(results[0].missing_edges[0], ("A".into(), "B".into()));
        assert_eq!(results[0].reverse_deps.len(), 1);
        assert_eq!(results[0].reverse_deps[0], ("B".into(), "A".into()));
    }

    #[test]
    fn validate_events_all_pass() {
        let events = vec![SeedEvent {
            name: "evt1".into(),
            producers: vec!["A".into()],
            consumers: vec!["B".into()],
            payload: HashMap::new(),
        }];
        let module_ids: HashSet<&str> = ["A", "B"].into_iter().collect();
        let mut actual = HashSet::new();
        actual.insert(("A".into(), "B".into()));

        let results = validate_events(&events, &module_ids, &actual);
        assert!(!results[0].orphan);
        assert!(results[0].ghost_consumers.is_empty());
        assert!(results[0].missing_edges.is_empty());
        assert!(results[0].reverse_deps.is_empty());
    }

    #[test]
    fn render_mermaid_events_basic() {
        let events = vec![SeedEvent {
            name: "消息完成".into(),
            producers: vec!["A".into()],
            consumers: vec!["B".into(), "C".into()],
            payload: HashMap::new(),
        }];
        let mod_a = SeedModule {
            module_id: "A".into(),
            display_name: Some("模块A".into()),
            layer: None,
            domain_kind: None,
            parent: None,
            precedence: None,
            path_rules: None,
        };
        let mod_b = SeedModule {
            module_id: "B".into(),
            display_name: Some("模块B".into()),
            layer: None,
            domain_kind: None,
            parent: None,
            precedence: None,
            path_rules: None,
        };
        let mod_c = SeedModule {
            module_id: "C".into(),
            display_name: None,
            layer: None,
            domain_kind: None,
            parent: None,
            precedence: None,
            path_rules: None,
        };
        let module_map: HashMap<&str, &SeedModule> =
            [("A", &mod_a), ("B", &mod_b), ("C", &mod_c)]
                .into_iter()
                .collect();

        let result = render_mermaid_events(&events, &module_map);
        assert!(result.contains("graph LR"));
        assert!(result.contains("publish"));
        assert!(result.contains("subscribe"));
        assert!(result.contains("消息完成"));
    }

    #[test]
    fn seed_spec_deserializes_boundaries_and_events() {
        let yaml = r#"
version: "1.0"
modules:
  - module_id: mod_a
    path_rules:
      include: ["src/a/**"]
relations_expected: []
boundaries:
  - module_id: mod_a
    public_paths:
      - "src/a/api/**"
events:
  - name: "test_event"
    producers:
      - mod_a
    consumers: []
"#;
        let seed: SeedSpec = serde_yaml::from_str(yaml).expect("parse");
        assert_eq!(seed.boundaries.len(), 1);
        assert_eq!(seed.boundaries[0].module_id, "mod_a");
        assert_eq!(seed.boundaries[0].public_paths, vec!["src/a/api/**"]);
        assert_eq!(seed.events.len(), 1);
        assert_eq!(seed.events[0].name, "test_event");
        assert_eq!(seed.events[0].producers, vec!["mod_a"]);
    }

    // === 行为契约相关测试 ===

    #[test]
    fn contract_fields_parse_from_yaml() {
        let yaml = r#"
version: v1
modules:
  - module_id: A
    precedence: 1
    path_rules:
      include: ["src/a/**"]
  - module_id: B
    precedence: 2
    path_rules:
      include: ["src/b/**"]
relations_expected: []
flows:
  - name: "test flow"
    steps:
      - from: A
        to: B
        action: "process"
        input:
          user_id: string
          amount: number
        output:
          result: boolean
boundaries:
  - module_id: A
    public_paths: ["src/a/api/**"]
    contracts:
      - name: "doSomething"
        input:
          param1: string
        output:
          result: json
events:
  - name: "test_event"
    producers: [A]
    consumers: [B]
    payload:
      event_id: string
      data: json
"#;
        let seed: SeedSpec = serde_yaml::from_str(yaml).expect("parse");
        // flow step 契约
        let step = &seed.flows[0].steps[0];
        assert_eq!(step.input.get("user_id"), Some(&"string".to_string()));
        assert_eq!(step.output.get("result"), Some(&"boolean".to_string()));
        // boundary 契约
        assert_eq!(seed.boundaries[0].contracts.len(), 1);
        assert_eq!(seed.boundaries[0].contracts[0].name, "doSomething");
        assert_eq!(
            seed.boundaries[0].contracts[0].input.get("param1"),
            Some(&"string".to_string())
        );
        // event payload
        assert_eq!(
            seed.events[0].payload.get("event_id"),
            Some(&"string".to_string())
        );
    }

    #[test]
    fn contract_fields_default_for_old_seed() {
        // 老格式没有新字段，应正常解析
        let yaml = r#"
version: v1
modules:
  - module_id: A
    precedence: 1
    path_rules:
      include: ["src/a/**"]
relations_expected: []
flows:
  - name: "old flow"
    steps:
      - from: A
        to: A
        action: "loop"
boundaries:
  - module_id: A
    public_paths: ["src/a/**"]
events:
  - name: "old event"
    producers: [A]
    consumers: [A]
"#;
        let seed: SeedSpec = serde_yaml::from_str(yaml).expect("parse");
        assert!(seed.flows[0].steps[0].input.is_empty());
        assert!(seed.flows[0].steps[0].output.is_empty());
        assert!(seed.boundaries[0].contracts.is_empty());
        assert!(seed.events[0].payload.is_empty());
    }

    #[test]
    fn flow_data_continuity_detects_missing_field() {
        let flows = vec![SeedFlow {
            name: "test".to_string(),
            steps: vec![
                SeedFlowStep {
                    from: "A".to_string(),
                    to: "B".to_string(),
                    action: Some("step1".to_string()),
                    input: HashMap::new(),
                    output: {
                        let mut m = HashMap::new();
                        m.insert("field_a".to_string(), "string".to_string());
                        m
                    },
                },
                SeedFlowStep {
                    from: "B".to_string(),
                    to: "C".to_string(),
                    action: Some("step2".to_string()),
                    input: {
                        let mut m = HashMap::new();
                        m.insert("field_a".to_string(), "string".to_string());
                        m.insert("field_b".to_string(), "number".to_string());
                        m
                    },
                    output: HashMap::new(),
                },
            ],
        }];
        let issues = validate_flow_data_continuity(&flows);
        assert_eq!(issues.len(), 1);
        assert_eq!(issues[0].missing_field, "field_b");
        assert_eq!(issues[0].from, "B");
        assert_eq!(issues[0].to, "C");
    }

    #[test]
    fn flow_data_continuity_passes_when_covered() {
        let flows = vec![SeedFlow {
            name: "ok".to_string(),
            steps: vec![
                SeedFlowStep {
                    from: "A".to_string(),
                    to: "B".to_string(),
                    action: None,
                    input: HashMap::new(),
                    output: {
                        let mut m = HashMap::new();
                        m.insert("x".to_string(), "string".to_string());
                        m.insert("y".to_string(), "number".to_string());
                        m
                    },
                },
                SeedFlowStep {
                    from: "B".to_string(),
                    to: "C".to_string(),
                    action: None,
                    input: {
                        let mut m = HashMap::new();
                        m.insert("x".to_string(), "string".to_string());
                        m
                    },
                    output: HashMap::new(),
                },
            ],
        }];
        let issues = validate_flow_data_continuity(&flows);
        assert!(issues.is_empty());
    }

    #[test]
    fn flow_data_continuity_skips_empty_fields() {
        // 当 step 没有声明 input/output 时跳过校验
        let flows = vec![SeedFlow {
            name: "skip".to_string(),
            steps: vec![
                SeedFlowStep {
                    from: "A".to_string(),
                    to: "B".to_string(),
                    action: None,
                    input: HashMap::new(),
                    output: HashMap::new(),
                },
                SeedFlowStep {
                    from: "B".to_string(),
                    to: "C".to_string(),
                    action: None,
                    input: {
                        let mut m = HashMap::new();
                        m.insert("z".to_string(), "any".to_string());
                        m
                    },
                    output: HashMap::new(),
                },
            ],
        }];
        let issues = validate_flow_data_continuity(&flows);
        assert!(issues.is_empty());
    }

    #[test]
    fn export_bdd_sources_generates_files() {
        let dir = temp_dir("bcc_contract_bdd");
        let seed = SeedSpec {
            version: None,
            source_of_truth: None,
            modules: vec![],
            relations_expected: vec![],
            layer_rules: None,
            flows: vec![SeedFlow {
                name: "msg flow".to_string(),
                steps: vec![SeedFlowStep {
                    from: "A".to_string(),
                    to: "B".to_string(),
                    action: Some("route".to_string()),
                    input: {
                        let mut m = HashMap::new();
                        m.insert("msg".to_string(), "string".to_string());
                        m
                    },
                    output: {
                        let mut m = HashMap::new();
                        m.insert("target".to_string(), "string".to_string());
                        m
                    },
                }],
            }],
            boundaries: vec![SeedBoundary {
                module_id: "X".to_string(),
                public_paths: vec![],
                contracts: vec![BoundaryContract {
                    name: "callX".to_string(),
                    kind: "command".to_string(),
                    input: {
                        let mut m = HashMap::new();
                        m.insert("id".to_string(), "string".to_string());
                        m
                    },
                    output: {
                        let mut m = HashMap::new();
                        m.insert("ok".to_string(), "bool".to_string());
                        m
                    },
                    errors: vec![],
                    fields: HashMap::new(),
                }],
            }],
            events: vec![SeedEvent {
                name: "evt1".to_string(),
                producers: vec!["P".to_string()],
                consumers: vec!["C1".to_string(), "C2".to_string()],
                payload: {
                    let mut m = HashMap::new();
                    m.insert("data".to_string(), "json".to_string());
                    m
                },
            }],
        };

        let (fc, bc, ec) = export_bdd_sources_from_contracts(&dir, &seed).expect("export ok");
        assert_eq!(fc, 1); // 1 flow step
        assert_eq!(bc, 1); // 1 boundary contract
        assert_eq!(ec, 2); // 2 event consumers

        // 检查生成的文件存在且内容正确
        let flow_files: Vec<_> = fs::read_dir(&dir)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.file_name().to_string_lossy().starts_with("flow_"))
            .collect();
        assert_eq!(flow_files.len(), 1);
        let flow_content = fs::read_to_string(flow_files[0].path()).unwrap();
        assert!(flow_content.contains("edge_class: flow_contract"));
        assert!(flow_content.contains("GIVEN"));
        assert!(flow_content.contains("WHEN"));
        assert!(flow_content.contains("THEN"));

        let boundary_files: Vec<_> = fs::read_dir(&dir)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.file_name().to_string_lossy().starts_with("boundary_"))
            .collect();
        assert_eq!(boundary_files.len(), 1);
        let boundary_content = fs::read_to_string(boundary_files[0].path()).unwrap();
        assert!(boundary_content.contains("edge_class: boundary_contract"));

        let event_files: Vec<_> = fs::read_dir(&dir)
            .unwrap()
            .filter_map(|e| e.ok())
            .filter(|e| e.file_name().to_string_lossy().starts_with("event_"))
            .collect();
        assert_eq!(event_files.len(), 2);

        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn export_bdd_sources_skips_empty_contracts() {
        let dir = temp_dir("bcc_contract_empty");
        let seed = SeedSpec {
            version: None,
            source_of_truth: None,
            modules: vec![],
            relations_expected: vec![],
            layer_rules: None,
            flows: vec![SeedFlow {
                name: "no contract".to_string(),
                steps: vec![SeedFlowStep {
                    from: "A".to_string(),
                    to: "B".to_string(),
                    action: None,
                    input: HashMap::new(),
                    output: HashMap::new(),
                }],
            }],
            boundaries: vec![SeedBoundary {
                module_id: "X".to_string(),
                public_paths: vec![],
                contracts: vec![],
            }],
            events: vec![SeedEvent {
                name: "no payload".to_string(),
                producers: vec!["P".to_string()],
                consumers: vec!["C".to_string()],
                payload: HashMap::new(),
            }],
        };

        let (fc, bc, ec) = export_bdd_sources_from_contracts(&dir, &seed).expect("export ok");
        assert_eq!(fc, 0);
        assert_eq!(bc, 0);
        assert_eq!(ec, 0);

        let _ = fs::remove_dir_all(&dir);
    }

    // === classify_contract 分流测试 ===

    fn make_seed_for_classify(
        flows: Vec<SeedFlow>,
        events: Vec<SeedEvent>,
        relations: Vec<SeedRelation>,
    ) -> SeedSpec {
        SeedSpec {
            version: None,
            source_of_truth: None,
            modules: vec![
                SeedModule {
                    module_id: "mod_a".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: None,
                    precedence: None,
                    path_rules: None,
                },
                SeedModule {
                    module_id: "mod_b".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: None,
                    precedence: None,
                    path_rules: None,
                },
                SeedModule {
                    module_id: "mod_c".to_string(),
                    display_name: None,
                    layer: None,
                    domain_kind: None,
                    parent: None,
                    precedence: None,
                    path_rules: None,
                },
            ],
            relations_expected: relations,
            layer_rules: None,
            flows,
            boundaries: vec![],
            events,
        }
    }

    fn make_contract(kind: &str) -> BoundaryContract {
        BoundaryContract {
            name: "test".to_string(),
            kind: kind.to_string(),
            input: HashMap::new(),
            output: HashMap::new(),
            errors: vec![],
            fields: HashMap::new(),
        }
    }

    #[test]
    fn classify_contract_crud_by_kind() {
        let seed = make_seed_for_classify(vec![], vec![], vec![]);
        let contract = make_contract("crud");
        assert_eq!(
            classify_contract(&contract, "mod_a", &seed),
            ContractComplexity::Crud
        );
    }

    #[test]
    fn classify_contract_query_no_flow_is_crud() {
        let seed = make_seed_for_classify(vec![], vec![], vec![]);
        let contract = make_contract("query");
        assert_eq!(
            classify_contract(&contract, "mod_a", &seed),
            ContractComplexity::Crud
        );
    }

    #[test]
    fn classify_contract_command_no_flow_is_simple() {
        let seed = make_seed_for_classify(vec![], vec![], vec![]);
        let contract = make_contract("command");
        assert_eq!(
            classify_contract(&contract, "mod_a", &seed),
            ContractComplexity::SimpleCommand
        );
    }

    #[test]
    fn classify_contract_in_flow_is_flow_participant() {
        let seed = make_seed_for_classify(
            vec![SeedFlow {
                name: "f1".to_string(),
                steps: vec![SeedFlowStep {
                    from: "mod_a".to_string(),
                    to: "mod_b".to_string(),
                    action: None,
                    input: HashMap::new(),
                    output: HashMap::new(),
                }],
            }],
            vec![],
            vec![],
        );
        let contract = make_contract("command");
        assert_eq!(
            classify_contract(&contract, "mod_a", &seed),
            ContractComplexity::FlowParticipant
        );
    }

    #[test]
    fn classify_contract_in_event_is_event_participant() {
        let seed = make_seed_for_classify(
            vec![],
            vec![SeedEvent {
                name: "evt".to_string(),
                producers: vec!["mod_a".to_string()],
                consumers: vec!["mod_b".to_string()],
                payload: HashMap::new(),
            }],
            vec![],
        );
        let contract = make_contract("command");
        assert_eq!(
            classify_contract(&contract, "mod_a", &seed),
            ContractComplexity::EventParticipant
        );
    }

    #[test]
    fn classify_contract_many_out_edges_is_complex() {
        let seed = make_seed_for_classify(
            vec![],
            vec![],
            vec![
                SeedRelation {
                    caller: "mod_a".to_string(),
                    callee: "mod_b".to_string(),
                    allowed: true,
                    rationale: None,
                },
                SeedRelation {
                    caller: "mod_a".to_string(),
                    callee: "mod_c".to_string(),
                    allowed: true,
                    rationale: None,
                },
            ],
        );
        let contract = make_contract("command");
        assert_eq!(
            classify_contract(&contract, "mod_a", &seed),
            ContractComplexity::ComplexOrchestration
        );
    }

    #[test]
    fn classify_contract_flow_trumps_out_edges() {
        // 同时在 flow 中且有多条出边，flow 优先
        let seed = make_seed_for_classify(
            vec![SeedFlow {
                name: "f".to_string(),
                steps: vec![SeedFlowStep {
                    from: "mod_a".to_string(),
                    to: "mod_b".to_string(),
                    action: None,
                    input: HashMap::new(),
                    output: HashMap::new(),
                }],
            }],
            vec![],
            vec![
                SeedRelation {
                    caller: "mod_a".to_string(),
                    callee: "mod_b".to_string(),
                    allowed: true,
                    rationale: None,
                },
                SeedRelation {
                    caller: "mod_a".to_string(),
                    callee: "mod_c".to_string(),
                    allowed: true,
                    rationale: None,
                },
            ],
        );
        let contract = make_contract("command");
        assert_eq!(
            classify_contract(&contract, "mod_a", &seed),
            ContractComplexity::FlowParticipant
        );
    }

    #[test]
    fn contract_kind_errors_fields_parse_from_yaml() {
        let yaml = r#"
version: v1
modules:
  - module_id: A
    precedence: 1
    path_rules:
      include: ["src/a/**"]
relations_expected: []
boundaries:
  - module_id: A
    public_paths: ["src/a/api/**"]
    contracts:
      - name: "create_order"
        kind: command
        input:
          user_id: uuid
        output:
          order_id: uuid
        errors:
          - "ORDER-001"
          - "ORDER-002"
      - name: "order"
        kind: crud
        fields:
          status: string
          total: decimal
"#;
        let seed: SeedSpec = serde_yaml::from_str(yaml).expect("parse");
        let c0 = &seed.boundaries[0].contracts[0];
        assert_eq!(c0.kind, "command");
        assert_eq!(c0.errors, vec!["ORDER-001", "ORDER-002"]);

        let c1 = &seed.boundaries[0].contracts[1];
        assert_eq!(c1.kind, "crud");
        assert_eq!(c1.fields.get("status"), Some(&"string".to_string()));
        assert_eq!(c1.fields.get("total"), Some(&"decimal".to_string()));
    }

    #[test]
    fn contract_kind_defaults_to_command() {
        let yaml = r#"
version: v1
modules: []
relations_expected: []
boundaries:
  - module_id: X
    contracts:
      - name: "no_kind"
        input: { a: string }
"#;
        let seed: SeedSpec = serde_yaml::from_str(yaml).expect("parse");
        assert_eq!(seed.boundaries[0].contracts[0].kind, "command");
        assert!(seed.boundaries[0].contracts[0].errors.is_empty());
        assert!(seed.boundaries[0].contracts[0].fields.is_empty());
    }

    // === CRUD mix 命令生成测试 ===

    #[test]
    fn crud_mix_command_from_fields() {
        let contract = BoundaryContract {
            name: "order".to_string(),
            kind: "crud".to_string(),
            input: HashMap::new(),
            output: HashMap::new(),
            errors: vec![],
            fields: {
                let mut m = HashMap::new();
                m.insert("status".to_string(), "string".to_string());
                m.insert("total".to_string(), "decimal".to_string());
                m
            },
        };
        let cmd = generate_crud_mix_command("order_service", &contract);
        assert!(cmd.starts_with("mix phx.gen.context"));
        assert!(cmd.contains("Orders"));
        assert!(cmd.contains("Order"));
        assert!(cmd.contains("orders"));
        assert!(cmd.contains("status:string"));
        assert!(cmd.contains("total:decimal"));
    }

    #[test]
    fn crud_mix_command_uuid_ref() {
        let contract = BoundaryContract {
            name: "order".to_string(),
            kind: "crud".to_string(),
            input: HashMap::new(),
            output: HashMap::new(),
            errors: vec![],
            fields: {
                let mut m = HashMap::new();
                m.insert("user_id".to_string(), "uuid".to_string());
                m.insert("amount".to_string(), "decimal".to_string());
                m
            },
        };
        let cmd = generate_crud_mix_command("shop", &contract);
        assert!(cmd.contains("user_id:references:users"));
        assert!(cmd.contains("amount:decimal"));
    }

    #[test]
    fn crud_mix_command_fallback_to_input_output() {
        // 没有 fields 时从 input+output 合并
        let contract = BoundaryContract {
            name: "product".to_string(),
            kind: "crud".to_string(),
            input: {
                let mut m = HashMap::new();
                m.insert("name".to_string(), "string".to_string());
                m
            },
            output: {
                let mut m = HashMap::new();
                m.insert("price".to_string(), "decimal".to_string());
                m
            },
            errors: vec![],
            fields: HashMap::new(),
        };
        let cmd = generate_crud_mix_command("catalog", &contract);
        assert!(cmd.contains("name:string"));
        assert!(cmd.contains("price:decimal"));
    }

    #[test]
    fn crud_mix_type_mapping() {
        assert_eq!(map_field_type_to_ecto("string"), "string");
        assert_eq!(map_field_type_to_ecto("integer"), "integer");
        assert_eq!(map_field_type_to_ecto("int"), "integer");
        assert_eq!(map_field_type_to_ecto("boolean"), "boolean");
        assert_eq!(map_field_type_to_ecto("bool"), "boolean");
        assert_eq!(map_field_type_to_ecto("decimal"), "decimal");
        assert_eq!(map_field_type_to_ecto("money"), "decimal");
        assert_eq!(map_field_type_to_ecto("datetime"), "utc_datetime");
        assert_eq!(map_field_type_to_ecto("timestamp"), "utc_datetime");
        assert_eq!(map_field_type_to_ecto("uuid"), "uuid");
        assert_eq!(map_field_type_to_ecto("json"), "map");
        assert_eq!(map_field_type_to_ecto("map"), "map");
        assert_eq!(map_field_type_to_ecto("unknown_type"), "string");
    }

    #[test]
    fn to_pascal_case_basic() {
        assert_eq!(to_pascal_case("order"), "Order");
        assert_eq!(to_pascal_case("order_item"), "OrderItem");
        assert_eq!(to_pascal_case("user"), "User");
    }

    #[test]
    fn generate_from_seed_classifies_and_generates() {
        let yaml = r#"
version: v1
modules:
  - module_id: shop
    precedence: 1
    path_rules:
      include: ["src/shop/**"]
relations_expected: []
boundaries:
  - module_id: shop
    contracts:
      - name: product
        kind: crud
        fields:
          name: string
          price: decimal
      - name: create_order
        kind: command
        input:
          user_id: uuid
        output:
          order_id: uuid
"#;
        let seed: SeedSpec = serde_yaml::from_str(yaml).expect("parse");
        let results = generate_from_seed(&seed);
        assert_eq!(results.len(), 2);

        // product → CRUD
        assert_eq!(results[0].2, ContractComplexity::Crud);
        assert!(results[0].3.starts_with("mix phx.gen.context"));

        // create_order → SimpleCommand（无 flow/event，出边 0），生成 Elixir 骨架
        assert_eq!(results[1].2, ContractComplexity::SimpleCommand);
        assert!(results[1].3.contains("defmodule MyApp.Shop do"));
        assert!(results[1].3.contains("def create_order("));
        assert!(results[1].3.contains("@spec create_order("));
    }

    #[test]
    fn generate_elixir_skeleton_basic() {
        let c1 = BoundaryContract {
            name: "process_message".to_string(),
            kind: "command".to_string(),
            input: [("user_id".to_string(), "uuid".to_string()), ("content".to_string(), "string".to_string())].into(),
            output: [("message_id".to_string(), "uuid".to_string())].into(),
            errors: vec!["invalid_content".to_string()],
            fields: HashMap::new(),
        };
        let skeleton = generate_elixir_skeleton("agent_runtime", &[&c1], &ContractComplexity::SimpleCommand);
        assert!(skeleton.contains("defmodule MyApp.AgentRuntime do"));
        assert!(skeleton.contains("@moduledoc"));
        assert!(skeleton.contains("@spec process_message("));
        assert!(skeleton.contains("String.t()"));  // uuid → String.t()
        assert!(skeleton.contains("def process_message(content, user_id)"));
        assert!(skeleton.contains("{:ok, nil}"));
        assert!(skeleton.contains("end\n"));
    }

    #[test]
    fn generate_elixir_skeleton_multiple_contracts() {
        let c1 = BoundaryContract {
            name: "send".to_string(),
            kind: "command".to_string(),
            input: [("msg".to_string(), "string".to_string())].into(),
            output: HashMap::new(),
            errors: vec![],
            fields: HashMap::new(),
        };
        let c2 = BoundaryContract {
            name: "query_status".to_string(),
            kind: "query".to_string(),
            input: [("id".to_string(), "integer".to_string())].into(),
            output: [("status".to_string(), "string".to_string())].into(),
            errors: vec![],
            fields: HashMap::new(),
        };
        let skeleton = generate_elixir_skeleton("channel_ingress", &[&c1, &c2], &ContractComplexity::ComplexOrchestration);
        // 两个 def 都在
        assert!(skeleton.contains("def send(msg)"));
        assert!(skeleton.contains("def query_status(id)"));
        // query 返回类型
        assert!(skeleton.contains("{:ok, map()} | {:error, :not_found}"));
        // command 返回类型
        assert!(skeleton.contains("{:ok, map()} | {:error, term()}"));
    }

    #[test]
    fn generate_elixir_skeleton_no_params() {
        let c = BoundaryContract {
            name: "health_check".to_string(),
            kind: "query".to_string(),
            input: HashMap::new(),
            output: [("ok".to_string(), "boolean".to_string())].into(),
            errors: vec![],
            fields: HashMap::new(),
        };
        let skeleton = generate_elixir_skeleton("gateway", &[&c], &ContractComplexity::SimpleCommand);
        assert!(skeleton.contains("def health_check()"));
        assert!(skeleton.contains("@spec health_check()"));
    }

    #[test]
    fn seed_type_to_elixir_typespec_coverage() {
        assert_eq!(seed_type_to_elixir_typespec("string"), "String.t()");
        assert_eq!(seed_type_to_elixir_typespec("integer"), "integer()");
        assert_eq!(seed_type_to_elixir_typespec("boolean"), "boolean()");
        assert_eq!(seed_type_to_elixir_typespec("map"), "map()");
        assert_eq!(seed_type_to_elixir_typespec("list"), "list()");
        assert_eq!(seed_type_to_elixir_typespec("timestamp"), "DateTime.t()");
        assert_eq!(seed_type_to_elixir_typespec("xyz"), "any()");
    }

    #[test]
    fn generate_from_seed_groups_non_crud_per_module() {
        let yaml = r#"
version: v1
modules:
  - module_id: gateway
    precedence: 1
    path_rules:
      include: ["src/gw/**"]
relations_expected: []
boundaries:
  - module_id: gateway
    contracts:
      - name: route_request
        kind: command
        input:
          path: string
        output:
          handler: string
      - name: dispatch
        kind: command
        input:
          target: string
        output:
          result: string
"#;
        let seed: SeedSpec = serde_yaml::from_str(yaml).expect("parse");
        let results = generate_from_seed(&seed);
        // 两个 command（SimpleCommand）→ 合并成一个模块骨架
        assert_eq!(results.len(), 1);
        assert!(results[0].3.contains("defmodule MyApp.Gateway do"));
        assert!(results[0].3.contains("def route_request("));
        assert!(results[0].3.contains("def dispatch("));
    }
}
