use serde::{Deserialize, Serialize};
use std::collections::{BTreeSet, HashMap};
use std::fs;
use std::path::{Path, PathBuf};

pub mod adapter;
pub mod common;
pub mod elixir;
pub mod php;
pub mod rust;
pub mod testing;
pub mod typescript;

#[derive(Debug, Serialize, Deserialize)]
pub struct FileRecord {
    pub language: String,
    pub file_path: String,
    pub module_doc: Option<String>,
    pub exports: Vec<ExportRecord>,
    pub imports: Vec<ImportRecord>,
    pub calls: Vec<CallRecord>,
    #[serde(rename = "localCallTargets", default)]
    pub local_call_targets: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub relation_hints: Vec<RelationHintRecord>,
    pub side_effects: SideEffects,
    /// 源码总行数
    pub loc_lines: usize,
    /// 声明总数（def/defp/class/function 等）
    pub declarations: usize,
    /// 类型注解信息（analyze 检测器使用）
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub type_annotations: Vec<TypeAnnotation>,
    /// 类型守卫信息（analyze 检测器使用）
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub type_guards: Vec<TypeGuard>,
    /// Schema 字段信息（analyze 检测器使用）
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub schema_fields: Vec<SchemaField>,
    /// 源码内容（内存流水线传递，不序列化到 JSON）
    #[serde(skip)]
    pub source_code: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TypeAnnotation {
    pub name: String,
    pub type_expr: String,
    pub line: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TypeGuard {
    pub function: String,
    pub guarded_type: String,
    pub line: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SchemaField {
    pub name: String,
    pub field_type: String,
    pub line: usize,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ExportRecord {
    pub name: String,
    pub kind: String,
    pub signature: Option<String>,
    pub line: usize,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ImportRecord {
    pub specifier: String,
    pub kind: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct CallRecord {
    pub callee: String,
    pub line: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RelationHintRecord {
    pub target: String,
    /// 关系类型提示：framework_injection | external_registration |
    /// supervisor_child | genserver_runtime_dep | callback_injection
    #[serde(default)]
    pub call_type_hint: String,
    #[serde(default)]
    pub via: String,
    #[serde(default)]
    pub confidence: f64,
    #[serde(default)]
    pub detector: String,
    #[serde(default)]
    pub reason: String,
}

/// 副作用分类标签——行为检测的分类维度，标注模块的外部交互类型
/// 通过全文关键词扫描得出，独立于 tree-sitter AST 提取
#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SideEffects {
    pub has_async: bool,
    pub has_http: bool,
    pub has_genserver: bool,
    pub has_file_io: bool,
    pub has_pubsub: bool,
}

pub fn run(path: &str, mode: &str, output: Option<&str>) {
    let content = match fs::read_to_string(path) {
        Ok(c) => c,
        Err(e) => {
            eprintln!("cannot read '{}': {}", path, e);
            std::process::exit(1);
        }
    };

    let lang = adapter::detect_language(path);
    let record = match adapter::get_adapter(&lang) {
        Some(lang_adapter) => lang_adapter.extract(&content, path),
        None => {
            eprintln!("unsupported language: {}", lang);
            std::process::exit(1);
        }
    };

    let result = match mode {
        "ast" => serde_json::to_string_pretty(&record).expect("JSON serialization failed"),
        "doc" => render_doc(&record),
        "yaml" => render_yaml(&record),
        other => {
            eprintln!("unsupported mode: {}", other);
            std::process::exit(1);
        }
    };

    match output {
        Some(path) => {
            if let Some(parent) = std::path::Path::new(path).parent() {
                fs::create_dir_all(parent).ok();
            }
            fs::write(path, &result).expect("failed to write output file");
            println!("Written: {}", path);
        }
        None => println!("{}", result),
    }
}

/// --mode doc: 输出 per-file 分析文档（设计文档模板格式）
fn render_doc(record: &FileRecord) -> String {
    let mut out = String::new();
    out.push_str(&format!("# {}\n\n", record.file_path));
    out.push_str(&format!("- **Language**: {}\n", record.language));
    out.push_str(&format!("- **Lines**: {}\n", record.loc_lines));
    out.push_str(&format!("- **Declarations**: {}\n", record.declarations));

    // ## 职责
    out.push_str("\n## 职责\n\n");
    if let Some(ref doc) = record.module_doc {
        out.push_str(&format!("{}\n", doc));
    } else {
        out.push_str("<!-- TODO: agent 补写 -->\n");
    }

    // ## 行为
    out.push_str("\n## 行为\n\n");
    if !record.exports.is_empty() {
        for ex in &record.exports {
            let sig = ex.signature.as_deref().unwrap_or("-");
            out.push_str(&format!(
                "- `{}` ({}, L{}) — {}\n",
                ex.name, ex.kind, ex.line, sig
            ));
        }
    } else {
        out.push_str("<!-- TODO: agent 补写 -->\n");
    }

    // ## 输入输出
    out.push_str("\n## 输入输出\n\n");
    if !record.exports.is_empty() {
        for ex in &record.exports {
            if let Some(ref sig) = ex.signature {
                out.push_str(&format!("- `{}`: {}\n", ex.name, sig));
            }
        }
    }
    if record.exports.iter().all(|e| e.signature.is_none()) {
        out.push_str("<!-- TODO: agent 补写 -->\n");
    }

    // ## 调用链位置
    out.push_str("\n## 调用链位置\n\n");
    if !record.calls.is_empty() || !record.imports.is_empty() {
        for im in &record.imports {
            out.push_str(&format!("- {} `{}`\n", im.kind, im.specifier));
        }
        for call in &record.calls {
            out.push_str(&format!("- 调用 `{}` (L{})\n", call.callee, call.line));
        }
    } else {
        out.push_str("<!-- TODO: agent 补写 -->\n");
    }

    // ## 状态与副作用
    out.push_str("\n## 状态与副作用\n\n");
    let se = &record.side_effects;
    let mut effects = Vec::new();
    if se.has_async {
        effects.push("async");
    }
    if se.has_http {
        effects.push("http");
    }
    if se.has_genserver {
        effects.push("genserver");
    }
    if se.has_file_io {
        effects.push("file_io");
    }
    if se.has_pubsub {
        effects.push("pubsub");
    }
    if !effects.is_empty() {
        out.push_str(&format!("{}\n", effects.join(", ")));
    } else {
        out.push_str("无检测到的副作用\n");
    }

    out
}

/// --mode yaml: 输出 YAML draft（可供 bcc compile 反向解析）
fn render_yaml(record: &FileRecord) -> String {
    let mut out = String::new();

    out.push_str("# [EXTRACTED — 需人工确认]\n");

    // 从 file_path 推断模块名
    let module_name = record
        .file_path
        .rsplit('/')
        .next()
        .unwrap_or(&record.file_path)
        .trim_end_matches(".ex")
        .trim_end_matches(".exs")
        .trim_end_matches(".ts")
        .trim_end_matches(".tsx");
    let pascal_name = to_pascal_case(module_name);

    out.push_str("module:\n");
    out.push_str(&format!("  name: {}\n", pascal_name));
    if let Some(ref doc) = record.module_doc {
        let first_line = doc.lines().next().unwrap_or(doc);
        out.push_str(&format!("  responsibility: \"{}\"\n", first_line));
    }

    out.push_str("\nports:\n");
    for ex in &record.exports {
        out.push_str(&format!("  - name: {}\n", ex.name));
        // 从 signature 推断 kind
        let kind = if ex.signature.as_ref().map_or(false, |s| s.contains("{:ok,")) {
            "query"
        } else {
            "command"
        };
        out.push_str(&format!("    kind: {}\n", kind));
    }

    if !record.calls.is_empty() {
        out.push_str("\nrelations:\n");
        for call in &record.calls {
            out.push_str(&format!("  - callee: {}\n", call.callee));
            out.push_str("    mode: sync\n");
        }
    }

    out
}

fn to_pascal_case(s: &str) -> String {
    s.split('_')
        .map(|w| {
            let mut chars = w.chars();
            match chars.next() {
                None => String::new(),
                Some(c) => c.to_uppercase().to_string() + &chars.as_str().to_lowercase(),
            }
        })
        .collect()
}

// ── batch extract ──

/// AstSnapshot 输出格式（与 arch matrix 期望的格式对齐）
#[derive(Debug, Serialize, Deserialize)]
pub struct AstSnapshot {
    pub source_count: usize,
    pub skipped_count: usize,
    pub records: Vec<AstSnapshotRecord>,
}

#[derive(Debug, Serialize, Deserialize)]
#[allow(non_snake_case)]
pub struct AstSnapshotRecord {
    pub sourcePath: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub localDependencies: Vec<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub localCallTargets: Vec<String>,
    #[serde(
        rename = "relationHints",
        default,
        skip_serializing_if = "Vec::is_empty"
    )]
    pub relation_hints: Vec<RelationHintRecord>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub calls: Vec<AstSnapshotCallRecord>,
    #[serde(default)]
    pub imports: Vec<AstSnapshotImportRecord>,
    pub exports_count: usize,
    pub imports_count: usize,
    pub loc_lines: usize,
    pub side_effects: SideEffects,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct AstSnapshotCallRecord {
    pub callee: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct AstSnapshotImportRecord {
    pub specifier: String,
    pub kind: String,
}

/// 需要排除的通用目录
const EXCLUDED_DIRS: &[&str] = &[
    "node_modules",
    ".git",
    "__tests__",
    "target",
    "_build",
    "deps",
    "dist",
    ".next",
    "vendor",
];

/// 规范化 batch 语言参数并校验支持集（大小写不敏感）。
pub fn normalize_batch_lang(lang: &str) -> Result<String, String> {
    let raw = lang.trim();
    let normalized = raw.to_ascii_lowercase();
    match normalized.as_str() {
        "elixir" | "typescript" | "rust" | "php" => Ok(normalized),
        _ => Err(format!("unsupported language: {}", raw)),
    }
}

/// 批量提取：递归遍历目录，按语言过滤，输出 AstSnapshot JSON
pub fn run_batch(dir: &str, lang: &str, output: &str) {
    let lang_norm = normalize_batch_lang(lang).unwrap_or_else(|err| {
        eprintln!("{}", err);
        std::process::exit(1);
    });

    let root = Path::new(dir);
    if !root.is_dir() {
        eprintln!("'{}' is not a directory", dir);
        std::process::exit(1);
    }

    let files = collect_files(root, &lang_norm);
    eprintln!("[batch] found {} files for lang={}", files.len(), lang_norm);

    let mut records = Vec::new();
    let mut skipped = 0usize;

    // Elixir 需要两遍：第一遍建模块名→路径映射
    let module_map = if lang_norm == "elixir" {
        build_elixir_module_map(root, &files)
    } else {
        HashMap::new()
    };

    // TypeScript monorepo：扫描 package.json 建立包名→入口路径映射
    let package_map = if lang_norm == "typescript" {
        build_monorepo_package_map(root)
    } else {
        HashMap::new()
    };

    for file_path in &files {
        let rel_path = file_path
            .strip_prefix(root)
            .unwrap_or(file_path)
            .to_string_lossy()
            .replace('\\', "/");

        let content = match fs::read_to_string(file_path) {
            Ok(c) => c,
            Err(e) => {
                eprintln!("[batch] WARN: cannot read '{}': {}", rel_path, e);
                skipped += 1;
                continue;
            }
        };

        let record = match extract_file(&content, file_path, &lang_norm) {
            Ok(r) => r,
            Err(e) => {
                eprintln!("[batch] WARN: extract failed for '{}': {}", rel_path, e);
                skipped += 1;
                continue;
            }
        };

        // 解析 localDependencies：将 import specifier 转为相对路径
        let mut local_deps = resolve_dependencies(
            &record,
            root,
            file_path,
            &lang_norm,
            &module_map,
            &package_map,
            &content,
        );
        // 过滤自引用（全文扫描会匹配到 defmodule 自身定义的模块名）
        local_deps.retain(|d| d != &rel_path);
        let relation_hints = resolve_relation_hints(
            &record,
            root,
            file_path,
            &lang_norm,
            &module_map,
            &package_map,
        );

        records.push(AstSnapshotRecord {
            sourcePath: rel_path,
            localDependencies: local_deps,
            localCallTargets: record.local_call_targets.clone(),
            relation_hints,
            calls: normalized_snapshot_calls(&record),
            imports: normalized_snapshot_imports(&record),
            exports_count: record.exports.len(),
            imports_count: record.imports.len(),
            loc_lines: record.loc_lines,
            side_effects: SideEffects {
                has_async: record.side_effects.has_async,
                has_http: record.side_effects.has_http,
                has_genserver: record.side_effects.has_genserver,
                has_file_io: record.side_effects.has_file_io,
                has_pubsub: record.side_effects.has_pubsub,
            },
        });
    }

    let snapshot = AstSnapshot {
        source_count: records.len(),
        skipped_count: skipped,
        records,
    };

    let json = serde_json::to_string_pretty(&snapshot).expect("JSON serialization failed");

    if let Some(parent) = Path::new(output).parent() {
        fs::create_dir_all(parent).ok();
    }
    fs::write(output, &json).expect("failed to write output file");
    eprintln!(
        "[batch] written {} records ({} skipped) → {}",
        snapshot.source_count, snapshot.skipped_count, output
    );
}

fn normalized_snapshot_calls(record: &FileRecord) -> Vec<AstSnapshotCallRecord> {
    let mut set = BTreeSet::new();
    for call in &record.calls {
        let callee = call.callee.trim();
        if !callee.is_empty() {
            set.insert(callee.to_string());
        }
    }

    set.into_iter()
        .map(|callee| AstSnapshotCallRecord { callee })
        .collect()
}

fn normalized_snapshot_imports(record: &FileRecord) -> Vec<AstSnapshotImportRecord> {
    let mut set = BTreeSet::new();
    for import in &record.imports {
        let kind = import.kind.trim();
        let specifier = normalize_spaces(import.specifier.trim());
        if !kind.is_empty() && !specifier.is_empty() {
            set.insert((kind.to_string(), specifier));
        }
    }

    set.into_iter()
        .map(|(kind, specifier)| AstSnapshotImportRecord { specifier, kind })
        .collect()
}

fn normalize_spaces(input: &str) -> String {
    input.split_whitespace().collect::<Vec<_>>().join(" ")
}

/// 对单个文件调用对应语言的 extract，返回 FileRecord 或错误
fn extract_file(content: &str, path: &Path, lang: &str) -> Result<FileRecord, String> {
    let path_str = path.to_string_lossy();
    let actual_lang = if lang == "typescript" && path_str.ends_with(".tsx") {
        "tsx"
    } else {
        lang
    };

    let lang_adapter = adapter::get_adapter(actual_lang)
        .ok_or_else(|| format!("unsupported language: {}", actual_lang))?;

    // 使用 catch_unwind 防止 tree-sitter panic 导致整个 batch 中断
    let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        lang_adapter.extract(content, &path_str)
    }));
    result.map_err(|_| "parser panic".to_string())
}

/// 递归收集目录中符合语言过滤条件的文件
fn collect_files(root: &Path, lang: &str) -> Vec<PathBuf> {
    let mut result = Vec::new();
    collect_files_recursive(root, lang, &mut result);
    result.sort();
    result
}

fn collect_files_recursive(dir: &Path, lang: &str, result: &mut Vec<PathBuf>) {
    let entries = match fs::read_dir(dir) {
        Ok(e) => e,
        Err(_) => return,
    };

    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            let dir_name = path
                .file_name()
                .map(|n| n.to_string_lossy().to_string())
                .unwrap_or_default();
            if EXCLUDED_DIRS.contains(&dir_name.as_str()) {
                continue;
            }
            // Elixir: 排除 test/ 目录
            if lang == "elixir" && dir_name == "test" {
                continue;
            }
            // Rust: 排除 tests/ 目录
            if lang == "rust" && dir_name == "tests" {
                continue;
            }
            // PHP: 排除 tests/ 目录
            if lang == "php" && dir_name == "tests" {
                continue;
            }
            collect_files_recursive(&path, lang, result);
        } else if should_include_file(&path, lang) {
            result.push(path);
        }
    }
}

/// 根据语言判断文件是否应被包含
fn should_include_file(path: &Path, lang: &str) -> bool {
    let name = path
        .file_name()
        .map(|n| n.to_string_lossy().to_string())
        .unwrap_or_default();

    match lang {
        "typescript" => {
            if name.ends_with(".d.ts") {
                return false;
            }
            if name.ends_with(".test.ts")
                || name.ends_with(".test.tsx")
                || name.ends_with(".spec.ts")
                || name.ends_with(".spec.tsx")
            {
                return false;
            }
            name.ends_with(".ts") || name.ends_with(".tsx")
        }
        "elixir" => {
            if name.ends_with("_test.exs") || name.ends_with(".exs") {
                return false;
            }
            name.ends_with(".ex")
        }
        "rust" => name.ends_with(".rs"),
        "php" => name.ends_with(".php"),
        _ => false,
    }
}

/// 将 FileRecord 的 imports 解析为相对于 root 的本地依赖路径
fn resolve_dependencies(
    record: &FileRecord,
    root: &Path,
    file_path: &Path,
    lang: &str,
    module_map: &HashMap<String, String>,
    package_map: &HashMap<String, String>,
    content: &str,
) -> Vec<String> {
    let mut deps = Vec::new();

    match lang {
        "typescript" => {
            let file_dir = file_path.parent().unwrap_or(root);
            for imp in &record.imports {
                if imp.specifier.starts_with('.') {
                    // 相对导入：解析为相对于 root 的路径
                    if let Some(resolved) = resolve_ts_import(file_dir, &imp.specifier, root) {
                        deps.push(resolved);
                    }
                } else {
                    // 非相对导入：查 monorepo 包名映射表
                    if let Some(resolved) = resolve_ts_package_import(&imp.specifier, package_map) {
                        deps.push(resolved);
                    }
                }
            }
        }
        "elixir" => {
            // 建立别名短名→完整模块名映射（用于解析 calls 中的短名）
            let mut alias_short_map: HashMap<String, String> = HashMap::new();

            // 从 alias/import/use/behaviour 解析
            for imp in &record.imports {
                let module_names = expand_elixir_specifier(&imp.specifier);
                for module_name in &module_names {
                    if let Some(rel_path) = module_map.get(module_name.as_str()) {
                        deps.push(rel_path.clone());
                    }
                    // 建立短名映射：Gong.Compaction.TokenEstimator → TokenEstimator
                    if let Some(short) = module_name.rsplit('.').next() {
                        alias_short_map.insert(short.to_string(), module_name.clone());
                    }
                }

                // 从 use 宏参数中提取嵌套的模块引用
                if imp.kind == "use" {
                    for nested_mod in extract_nested_module_refs(&imp.specifier) {
                        if let Some(rel_path) = module_map.get(nested_mod.as_str()) {
                            deps.push(rel_path.clone());
                        }
                    }
                }
            }

            // 从 calls（直接模块调用如 Gong.Tape.Store）解析
            for call in &record.calls {
                // 先尝试完整名匹配
                if let Some(rel_path) = module_map.get(&call.callee) {
                    deps.push(rel_path.clone());
                }
                // 再尝试别名短名映射
                else if let Some(full_name) = alias_short_map.get(&call.callee) {
                    if let Some(rel_path) = module_map.get(full_name.as_str()) {
                        deps.push(rel_path.clone());
                    }
                }
            }

            // 全文模块引用扫描：捕获 supervisor child specs、struct 引用、
            // 模块属性、函数参数中的模块名等 tree-sitter 无法抓到的引用
            // 先剥离注释、文档字符串、普通字符串，避免假依赖
            let cleaned_content = strip_elixir_non_code(content);
            let content_refs = scan_elixir_module_refs(&cleaned_content);
            for module_ref in &content_refs {
                if let Some(rel_path) = module_map.get(module_ref.as_str()) {
                    deps.push(rel_path.clone());
                }
            }
        }
        _ => {
            // PHP/Rust: 暂不解析本地依赖
        }
    }

    deps.sort();
    deps.dedup();
    deps
}

/// 将语言提取阶段产出的 relation_hints 归一化为可被 arch matrix 消费的目标路径。
fn resolve_relation_hints(
    record: &FileRecord,
    root: &Path,
    file_path: &Path,
    lang: &str,
    module_map: &HashMap<String, String>,
    package_map: &HashMap<String, String>,
) -> Vec<RelationHintRecord> {
    let mut hints = Vec::new();
    let file_dir = file_path.parent().unwrap_or(root);

    for hint in &record.relation_hints {
        let target = hint.target.trim();
        if target.is_empty() {
            continue;
        }

        let resolved_target = match lang {
            "typescript" => {
                if target.starts_with('.') {
                    resolve_ts_import(file_dir, target, root)
                } else {
                    resolve_ts_package_import(target, package_map)
                }
            }
            "elixir" => module_map.get(target).cloned(),
            _ => None,
        };

        let normalized = resolved_target.unwrap_or_else(|| target.replace('\\', "/"));
        hints.push(RelationHintRecord {
            target: normalized,
            call_type_hint: hint.call_type_hint.clone(),
            via: hint.via.clone(),
            confidence: hint.confidence,
            detector: hint.detector.clone(),
            reason: hint.reason.clone(),
        });
    }

    // 保持输出稳定，便于回归比较。
    hints.sort_by(|a, b| {
        a.target
            .cmp(&b.target)
            .then_with(|| a.call_type_hint.cmp(&b.call_type_hint))
            .then_with(|| a.via.cmp(&b.via))
            .then_with(|| {
                a.confidence
                    .partial_cmp(&b.confidence)
                    .unwrap_or(std::cmp::Ordering::Equal)
            })
    });
    hints.dedup_by(|a, b| {
        a.target == b.target
            && a.call_type_hint == b.call_type_hint
            && a.via == b.via
            && (a.confidence - b.confidence).abs() < f64::EPSILON
            && a.detector == b.detector
            && a.reason == b.reason
    });
    hints
}

/// 剥离 Elixir 源码中的非代码文本（注释、文档字符串、普通字符串字面量）
/// 返回清洗后的文本，用于 scan_elixir_module_refs 避免假依赖
fn strip_elixir_non_code(content: &str) -> String {
    let mut result = String::with_capacity(content.len());
    let mut in_heredoc = false;

    for line in content.lines() {
        if in_heredoc {
            // 在 @moduledoc/@doc 三引号块内，检查闭合 """
            if line.trim() == "\"\"\"" || line.trim().ends_with("\"\"\"") {
                in_heredoc = false;
            }
            result.push('\n');
            continue;
        }

        let trimmed = line.trim();

        // 检测 @moduledoc/@doc 三引号块开始
        if trimmed.starts_with("@moduledoc \"\"\"") || trimmed.starts_with("@doc \"\"\"") {
            // 开始行本身也是三引号块的一部分，如果同行闭合则不进入多行模式
            let after_triple = if trimmed.starts_with("@moduledoc") {
                &trimmed["@moduledoc \"\"\"".len()..]
            } else {
                &trimmed["@doc \"\"\"".len()..]
            };
            if after_triple.contains("\"\"\"") {
                // 同行闭合，跳过整行
                result.push('\n');
                continue;
            }
            in_heredoc = true;
            result.push('\n');
            continue;
        }

        // 检测单行 @moduledoc/@doc 字符串（非三引号）
        if (trimmed.starts_with("@moduledoc \"") || trimmed.starts_with("@doc \""))
            && !trimmed.starts_with("@moduledoc \"\"\"")
            && !trimmed.starts_with("@doc \"\"\"")
        {
            result.push('\n');
            continue;
        }

        // 检测 @moduledoc false
        if trimmed == "@moduledoc false" || trimmed == "@doc false" {
            result.push('\n');
            continue;
        }

        // 剥离行内 # 注释（排除字符串内的 #）
        let line_no_comment = strip_line_comment(line);

        // 剥离普通双引号字符串字面量内容
        let line_no_strings = strip_string_literals(&line_no_comment);

        result.push_str(&line_no_strings);
        result.push('\n');
    }

    result
}

/// 剥离行内 # 注释（简单实现：找到不在引号内的 #，截断到行尾）
fn strip_line_comment(line: &str) -> String {
    let mut in_string = false;
    let mut escaped = false;
    let bytes = line.as_bytes();

    for (i, &b) in bytes.iter().enumerate() {
        if escaped {
            escaped = false;
            continue;
        }
        if b == b'\\' && in_string {
            escaped = true;
            continue;
        }
        if b == b'"' {
            in_string = !in_string;
            continue;
        }
        if b == b'#' && !in_string {
            return line[..i].to_string();
        }
    }
    line.to_string()
}

/// 将双引号字符串字面量内容替换为空（保留引号本身）
fn strip_string_literals(line: &str) -> String {
    let mut result = String::with_capacity(line.len());
    let mut in_string = false;
    let mut escaped = false;

    for ch in line.chars() {
        if escaped {
            escaped = false;
            // 在字符串内，跳过转义字符
            if in_string {
                continue;
            }
            result.push(ch);
            continue;
        }
        if ch == '\\' && in_string {
            escaped = true;
            continue;
        }
        if ch == '"' {
            in_string = !in_string;
            result.push('"');
            continue;
        }
        if in_string {
            // 字符串内容，跳过
            continue;
        }
        result.push(ch);
    }
    result
}

/// 全文扫描 Elixir 源码中的模块引用（至少两段大写开头的 dot-separated 名称）
/// 捕获 tree-sitter dot-call 无法覆盖的场景：
/// - Supervisor child spec: `{Gong.Agent, opts}`
/// - 结构体引用: `%Gong.Tape.Entry{}`
/// - 模块属性: `@impl Gong.Extension`
/// - 函数参数: `apply(Gong.Prompt, :build, [])`
fn scan_elixir_module_refs(content: &str) -> Vec<String> {
    let mut refs = Vec::new();
    let mut seen = std::collections::HashSet::new();
    let bytes = content.as_bytes();
    let len = bytes.len();
    let mut i = 0;

    while i < len {
        // 找大写字母开头的位置
        if bytes[i].is_ascii_uppercase() {
            let start = i;
            // 收集连续的 字母/数字/下划线/点
            while i < len
                && (bytes[i].is_ascii_alphanumeric() || bytes[i] == b'_' || bytes[i] == b'.')
            {
                i += 1;
            }
            let raw = &content[start..i];
            let raw = raw.trim_end_matches('.');
            // 按 . 分段，保留从开头到最后一个大写开头段的部分
            // 这样 Gong.Tape.Store.get → Gong.Tape.Store
            let segments: Vec<&str> = raw.split('.').collect();
            let mut last_upper_idx = 0;
            for (idx, seg) in segments.iter().enumerate() {
                if seg.chars().next().map_or(false, |c| c.is_uppercase()) {
                    last_upper_idx = idx;
                } else {
                    break;
                }
            }
            if last_upper_idx >= 1 {
                // 至少两段大写开头
                let module_ref = segments[..=last_upper_idx].join(".");
                if !seen.contains(&module_ref) {
                    seen.insert(module_ref.clone());
                    refs.push(module_ref);
                }
            }
        } else {
            i += 1;
        }
    }
    refs
}

/// 解析 TypeScript 相对导入为相对于 root 的路径
fn resolve_ts_import(file_dir: &Path, specifier: &str, root: &Path) -> Option<String> {
    let base = file_dir.join(specifier);
    let base_text = base.to_string_lossy();
    let candidates = [
        base.clone(),
        PathBuf::from(format!("{}.ts", base_text)),
        PathBuf::from(format!("{}.tsx", base_text)),
        PathBuf::from(format!("{}.mts", base_text)),
        PathBuf::from(format!("{}.cts", base_text)),
        base.join("index.ts"),
        base.join("index.tsx"),
        base.join("index.mts"),
        base.join("index.cts"),
    ];

    // canonicalize root 以便 strip_prefix 正确工作
    let canon_root = root.canonicalize().ok()?;

    for candidate in &candidates {
        if candidate.exists() {
            // canonicalize 解析 .. 和 symlinks
            if let Ok(canon) = candidate.canonicalize() {
                if let Ok(rel) = canon.strip_prefix(&canon_root) {
                    return Some(rel.to_string_lossy().replace('\\', "/"));
                }
            }
        }
    }
    None
}

/// 建立 Elixir 模块名 → 相对路径映射表
fn build_elixir_module_map(root: &Path, files: &[PathBuf]) -> HashMap<String, String> {
    let mut map = HashMap::new();
    for file_path in files {
        let rel_path = file_path
            .strip_prefix(root)
            .unwrap_or(file_path)
            .to_string_lossy()
            .replace('\\', "/");

        // 从文件内容的 defmodule 提取模块名
        if let Ok(content) = fs::read_to_string(file_path) {
            if let Some(module_name) = extract_elixir_module_name(&content) {
                map.insert(module_name, rel_path.clone());
            }
        }

        // 也从文件路径推断模块名（如 lib/my_app/accounts/user.ex → MyApp.Accounts.User）
        if let Some(inferred) = elixir::infer_module_from_path(&rel_path) {
            map.insert(inferred, rel_path);
        }
    }
    map
}

/// 从 Elixir 源码中提取第一个 defmodule 的模块名
fn extract_elixir_module_name(content: &str) -> Option<String> {
    for line in content.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with("defmodule ") {
            // defmodule MyApp.Foo do
            let rest = trimmed.strip_prefix("defmodule ")?.trim();
            let module_name = rest.split_whitespace().next()?;
            return Some(module_name.to_string());
        }
    }
    None
}

// ── TypeScript monorepo 包名解析 ──

/// 扫描目录下的 package.json 文件，建立 npm 包名 → 入口文件相对路径映射
/// 支持 packages/*/package.json 和根级 package.json
fn build_monorepo_package_map(root: &Path) -> HashMap<String, String> {
    let mut map = HashMap::new();
    // 递归扫描所有 package.json（排除 node_modules）
    collect_package_jsons(root, root, &mut map);
    map
}

fn collect_package_jsons(dir: &Path, root: &Path, map: &mut HashMap<String, String>) {
    let entries = match fs::read_dir(dir) {
        Ok(e) => e,
        Err(_) => return,
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            let name = path
                .file_name()
                .map(|n| n.to_string_lossy().to_string())
                .unwrap_or_default();
            if name == "node_modules" || name == ".git" || name == "dist" || name == ".next" {
                continue;
            }
            collect_package_jsons(&path, root, map);
        } else if path
            .file_name()
            .map(|n| n.to_string_lossy() == "package.json")
            .unwrap_or(false)
        {
            parse_package_json(&path, root, map);
        }
    }
}

/// 解析单个 package.json，提取包名和入口文件（main/exports）
fn parse_package_json(pkg_path: &Path, root: &Path, map: &mut HashMap<String, String>) {
    let content = match fs::read_to_string(pkg_path) {
        Ok(c) => c,
        Err(_) => return,
    };
    let json: serde_json::Value = match serde_json::from_str(&content) {
        Ok(v) => v,
        Err(_) => return,
    };

    let pkg_name = match json.get("name").and_then(|n| n.as_str()) {
        Some(n) => n.to_string(),
        None => return,
    };

    let pkg_dir = match pkg_path.parent() {
        Some(d) => d,
        None => return,
    };

    // 尝试从 exports 提取子路径映射
    // exports: { ".": { "import": "./src/index.ts" }, "./providers": { "import": "./src/providers/index.ts" } }
    if let Some(exports) = json.get("exports").and_then(|e| e.as_object()) {
        for (key, value) in exports {
            let entry_path = extract_export_entry(value);
            if let Some(entry) = entry_path {
                let abs_entry = pkg_dir.join(&entry);
                if let Some(resolved) = resolve_entry_path(&abs_entry, root) {
                    if key == "." {
                        map.insert(pkg_name.clone(), resolved);
                    } else {
                        // "./providers" → "@scope/pkg/providers"
                        let sub_path = key.strip_prefix("./").unwrap_or(key);
                        map.insert(format!("{}/{}", pkg_name, sub_path), resolved);
                    }
                }
            }
        }
    }

    // 如果 exports 没有 "." 入口，用 main/module 字段
    if !map.contains_key(&pkg_name) {
        let main_field = json
            .get("module")
            .or_else(|| json.get("main"))
            .and_then(|m| m.as_str());
        if let Some(main) = main_field {
            let abs_entry = pkg_dir.join(main);
            if let Some(resolved) = resolve_entry_path(&abs_entry, root) {
                map.insert(pkg_name.clone(), resolved);
            }
        } else {
            // 尝试默认入口 src/index.ts
            let default_entries = ["src/index.ts", "src/index.tsx", "index.ts"];
            for entry in &default_entries {
                let abs_entry = pkg_dir.join(entry);
                if abs_entry.exists() {
                    if let Some(resolved) = resolve_entry_path(&abs_entry, root) {
                        map.insert(pkg_name.clone(), resolved);
                        break;
                    }
                }
            }
        }
    }
}

/// 从 exports 值中提取实际文件路径
fn extract_export_entry(value: &serde_json::Value) -> Option<String> {
    // 字符串：直接路径
    if let Some(s) = value.as_str() {
        return Some(s.to_string());
    }
    // 对象：优先 import → default → types
    if let Some(obj) = value.as_object() {
        for key in &["import", "default", "require"] {
            if let Some(v) = obj.get(*key) {
                if let Some(s) = v.as_str() {
                    return Some(s.to_string());
                }
                // 递归处理嵌套条件
                if let Some(resolved) = extract_export_entry(v) {
                    return Some(resolved);
                }
            }
        }
    }
    None
}

/// 将绝对路径解析为相对于 root 的路径（尝试多种扩展名）
fn resolve_entry_path(abs_path: &Path, root: &Path) -> Option<String> {
    let candidates = [
        abs_path.to_path_buf(),
        abs_path.with_extension("ts"),
        abs_path.with_extension("tsx"),
        abs_path.join("index.ts"),
        abs_path.join("index.tsx"),
    ];
    for candidate in &candidates {
        if candidate.exists() {
            if let Ok(rel) = candidate.strip_prefix(root) {
                return Some(rel.to_string_lossy().replace('\\', "/"));
            }
        }
    }
    None
}

/// 解析 TypeScript 非相对导入：查 monorepo 包名映射表
fn resolve_ts_package_import(
    specifier: &str,
    package_map: &HashMap<String, String>,
) -> Option<String> {
    // 精确匹配：@scope/pkg 或 @scope/pkg/sub
    if let Some(resolved) = package_map.get(specifier) {
        return Some(resolved.clone());
    }
    // 尝试最长前缀匹配：import "@scope/pkg/deep/path" → 匹配 "@scope/pkg"
    // 逐级剥离路径查找
    let mut path = specifier;
    while let Some(idx) = path.rfind('/') {
        path = &specifier[..idx];
        if let Some(resolved) = package_map.get(path) {
            return Some(resolved.clone());
        }
    }
    None
}

// ── Elixir alias/import/use specifier 解析 ──

/// 展开 Elixir import specifier 为模块名列表
/// 处理：
/// - 简单模块名：`Gong.Tape.Store` → [`Gong.Tape.Store`]
/// - multi-alias：`Gong.Tape.{Entry, Index}` → [`Gong.Tape.Entry`, `Gong.Tape.Index`]
/// - 带 as: 子句：`Gong.Tape.Store, as: TS` → [`Gong.Tape.Store`]
/// - 带 only/except：`Gong.Tape.Store, only: [get: 1]` → [`Gong.Tape.Store`]
pub fn expand_elixir_specifier(specifier: &str) -> Vec<String> {
    let trimmed = specifier.trim();

    // 检查是否包含 multi-alias 语法 {A, B}
    if let Some(brace_start) = trimmed.find('{') {
        if let Some(brace_end) = trimmed.find('}') {
            let prefix = trimmed[..brace_start].trim().trim_end_matches('.');
            let inner = &trimmed[brace_start + 1..brace_end];
            return inner
                .split(',')
                .map(|s| {
                    let name = s.trim();
                    // 子元素可能也有 as: 子句，去掉
                    let name = strip_keyword_options(name);
                    format!("{}.{}", prefix, name)
                })
                .filter(|s| !s.ends_with('.'))
                .collect();
        }
    }

    // 简单模块名，可能带 as:/only:/except: 选项
    let module_name = strip_keyword_options(trimmed);
    if module_name.is_empty() {
        return Vec::new();
    }
    vec![module_name.to_string()]
}

/// 从 use 宏参数文本中提取嵌套的模块引用
/// 例如 `Jido.AI.ReActAgent, tools: [Gong.Tools.Read, Gong.Tools.Write]`
/// → 提取 [`Gong.Tools.Read`, `Gong.Tools.Write`]
fn extract_nested_module_refs(specifier: &str) -> Vec<String> {
    let mut refs = Vec::new();
    // 用正则匹配大写开头的 dot-separated 标识符（至少两段，排除主模块名）
    // 模式：大写字母开头，后跟字母数字，然后 .大写字母开头 重复一次以上
    let mut i = 0;
    let bytes = specifier.as_bytes();
    let len = bytes.len();

    // 跳过第一个模块名（use 的目标模块）
    let first_comma = specifier.find(',').unwrap_or(len);

    while i < len {
        // 跳过第一个逗号之前的内容
        if i < first_comma {
            i += 1;
            continue;
        }
        // 找大写字母开头的位置
        if bytes[i].is_ascii_uppercase() {
            let start = i;
            // 收集 模块名段: 大写字母开头 + 字母数字下划线，用 . 连接
            while i < len
                && (bytes[i].is_ascii_alphanumeric() || bytes[i] == b'_' || bytes[i] == b'.')
            {
                i += 1;
            }
            let candidate = &specifier[start..i];
            // 至少两段（Foo.Bar）才是模块引用，排除单段如 String
            if candidate.contains('.')
                && candidate.chars().next().map_or(false, |c| c.is_uppercase())
            {
                refs.push(candidate.to_string());
            }
        } else {
            i += 1;
        }
    }
    refs
}

/// 从 specifier 中提取模块名，去掉关键字选项和换行
/// 处理 `Gong.Tape.Store, as: TS` → `Gong.Tape.Store`
/// 处理 `Jido.AI.ReActAgent,\n  name: "gong"` → `Jido.AI.ReActAgent`
fn strip_keyword_options(s: &str) -> &str {
    let trimmed = s.trim();
    // 找第一个逗号或换行，取前面部分
    let end = trimmed
        .find(',')
        .unwrap_or(trimmed.len())
        .min(trimmed.find('\n').unwrap_or(trimmed.len()));
    trimmed[..end].trim()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn normalize_batch_lang_accepts_supported_values_case_insensitive() {
        assert_eq!(
            normalize_batch_lang("TyPeScRiPt"),
            Ok("typescript".to_string())
        );
        assert_eq!(normalize_batch_lang("RUST"), Ok("rust".to_string()));
    }

    #[test]
    fn normalize_batch_lang_rejects_unsupported_value() {
        assert_eq!(
            normalize_batch_lang("go"),
            Err("unsupported language: go".to_string())
        );
    }

    #[test]
    fn expand_simple_module() {
        assert_eq!(
            expand_elixir_specifier("Gong.Tape.Store"),
            vec!["Gong.Tape.Store"]
        );
    }

    #[test]
    fn expand_multi_alias() {
        let mut result = expand_elixir_specifier("Gong.Tape.{Entry, Index, FileStore}");
        result.sort();
        assert_eq!(
            result,
            vec!["Gong.Tape.Entry", "Gong.Tape.FileStore", "Gong.Tape.Index"]
        );
    }

    #[test]
    fn expand_with_as_clause() {
        assert_eq!(
            expand_elixir_specifier("Gong.Tape.Store, as: TS"),
            vec!["Gong.Tape.Store"]
        );
    }

    #[test]
    fn expand_with_only_clause() {
        assert_eq!(
            expand_elixir_specifier("Gong.Tape.Store, only: [get: 1, put: 2]"),
            vec!["Gong.Tape.Store"]
        );
    }

    #[test]
    fn expand_multi_alias_with_dot_prefix() {
        // alias Gong.{Tape, Tools} — 有时 prefix 带点，有时不带
        let mut result = expand_elixir_specifier("Gong.{Tape, Tools}");
        result.sort();
        assert_eq!(result, vec!["Gong.Tape", "Gong.Tools"]);
    }

    #[test]
    fn resolve_ts_package_exact_match() {
        let mut map = HashMap::new();
        map.insert(
            "@mariozechner/pi-ai".to_string(),
            "ai/src/index.ts".to_string(),
        );
        assert_eq!(
            resolve_ts_package_import("@mariozechner/pi-ai", &map),
            Some("ai/src/index.ts".to_string())
        );
    }

    #[test]
    fn resolve_ts_package_prefix_match() {
        let mut map = HashMap::new();
        map.insert(
            "@mariozechner/pi-ai".to_string(),
            "ai/src/index.ts".to_string(),
        );
        // import "@mariozechner/pi-ai/providers/openai" 应匹配包名
        assert_eq!(
            resolve_ts_package_import("@mariozechner/pi-ai/providers/openai", &map),
            Some("ai/src/index.ts".to_string())
        );
    }

    #[test]
    fn resolve_ts_package_no_match() {
        let map = HashMap::new();
        assert_eq!(resolve_ts_package_import("lodash", &map), None);
    }

    #[test]
    fn strip_keyword_options_works() {
        assert_eq!(strip_keyword_options("Foo.Bar, as: FB"), "Foo.Bar");
        assert_eq!(strip_keyword_options("Foo.Bar, only: [a: 1]"), "Foo.Bar");
        assert_eq!(strip_keyword_options("Foo.Bar, except: [b: 2]"), "Foo.Bar");
        assert_eq!(strip_keyword_options("Foo.Bar"), "Foo.Bar");
    }

    #[test]
    fn extract_nested_module_refs_from_use_args() {
        // use Jido.AI.ReActAgent, tools: [Gong.Tools.Read, Gong.Tools.Write]
        let spec =
            "Jido.AI.ReActAgent, tools: [Gong.Tools.Read, Gong.Tools.Write], model: \"deepseek\"";
        let mut refs = extract_nested_module_refs(spec);
        refs.sort();
        assert_eq!(refs, vec!["Gong.Tools.Read", "Gong.Tools.Write"]);
    }

    #[test]
    fn extract_nested_module_refs_no_nested() {
        // 没有嵌套模块引用
        let spec = "GenServer";
        let refs = extract_nested_module_refs(spec);
        assert!(refs.is_empty());
    }

    #[test]
    fn extract_nested_module_refs_single_word_excluded() {
        // 单段名（如 String, Integer）不应被提取
        let spec = "Foo.Bar, validate: String";
        let refs = extract_nested_module_refs(spec);
        assert!(refs.is_empty());
    }

    #[test]
    fn scan_elixir_module_refs_basic() {
        let content = r#"
defmodule MyApp.Worker do
  use GenServer
  alias Gong.Tape.Store

  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts)
  end

  def init(state) do
    children = [
      {Gong.Agent, []},
      {Gong.ProviderRegistry, []}
    ]
    Supervisor.init(children, strategy: :one_for_one)
  end

  def handle_call(:get, _from, state) do
    entry = %Gong.Tape.Entry{content: "hello"}
    {:reply, entry, state}
  end
end
"#;
        let refs = scan_elixir_module_refs(content);
        // 应捕获 supervisor child specs 和 struct 引用
        assert!(refs.contains(&"Gong.Agent".to_string()));
        assert!(refs.contains(&"Gong.ProviderRegistry".to_string()));
        assert!(refs.contains(&"Gong.Tape.Entry".to_string()));
        assert!(refs.contains(&"Gong.Tape.Store".to_string()));
        // GenServer 是单段名（大写但无点），不应被提取
        assert!(!refs.iter().any(|r| r == "GenServer"));
    }

    #[test]
    fn scan_elixir_module_refs_truncates_at_lowercase() {
        // Gong.Tape.store 中 store 小写开头 → 截断为 Gong.Tape
        let content = "result = Gong.Tape.store.get()";
        let refs = scan_elixir_module_refs(content);
        assert!(refs.contains(&"Gong.Tape".to_string()));
        assert!(!refs.iter().any(|r| r.contains("store")));
    }

    #[test]
    fn scan_elixir_module_refs_method_call() {
        // Gong.Tape.Store.get() → 截断到 Gong.Tape.Store（get 小写）
        let content = "Gong.Tape.Store.get()";
        let refs = scan_elixir_module_refs(content);
        assert!(refs.contains(&"Gong.Tape.Store".to_string()));
    }

    // ── strip_elixir_non_code 测试 ──

    #[test]
    fn strip_moduledoc_heredoc() {
        // 纯文档模块（Gong 复现场景）
        let content = r#"defmodule Gong do
  @moduledoc """
  - `Gong.Compaction` — 上下文压缩
  - `Gong.Truncate` — 输出截断系统
  """
end"#;
        let cleaned = strip_elixir_non_code(content);
        let refs = scan_elixir_module_refs(&cleaned);
        assert!(
            !refs.iter().any(|r| r == "Gong.Compaction"),
            "不应从 @moduledoc 中提取 Gong.Compaction"
        );
        assert!(
            !refs.iter().any(|r| r == "Gong.Truncate"),
            "不应从 @moduledoc 中提取 Gong.Truncate"
        );
    }

    #[test]
    fn strip_comment_module_refs() {
        let content = "# 使用 Foo.Bar 处理数据\ndefmodule Baz do\nend";
        let cleaned = strip_elixir_non_code(content);
        let refs = scan_elixir_module_refs(&cleaned);
        assert!(
            !refs.iter().any(|r| r == "Foo.Bar"),
            "不应从注释中提取 Foo.Bar"
        );
    }

    #[test]
    fn strip_string_literal_module_refs() {
        let content = r#"x = "调用 Foo.Bar.run()""#;
        let cleaned = strip_elixir_non_code(content);
        let refs = scan_elixir_module_refs(&cleaned);
        assert!(
            !refs.iter().any(|r| r == "Foo.Bar"),
            "不应从字符串字面量中提取 Foo.Bar"
        );
    }

    #[test]
    fn strip_preserves_real_code_deps() {
        let content = "defmodule A do\n  def run, do: B.Worker.start()\nend";
        let cleaned = strip_elixir_non_code(content);
        let refs = scan_elixir_module_refs(&cleaned);
        assert!(
            refs.contains(&"B.Worker".to_string()),
            "应保留真实代码依赖 B.Worker"
        );
    }

    #[test]
    fn strip_mixed_scenario() {
        // 混合场景：@doc 字符串 + 注释 + 真实代码
        let content = r#"defmodule Mix do
  @doc "See Mix.Helper"
  # Mix.Utils 辅助
  def run, do: Mix.Real.call()
end"#;
        let cleaned = strip_elixir_non_code(content);
        let refs = scan_elixir_module_refs(&cleaned);
        assert!(
            refs.contains(&"Mix.Real".to_string()),
            "应保留真实依赖 Mix.Real"
        );
        assert!(
            !refs.iter().any(|r| r == "Mix.Helper"),
            "不应从 @doc 字符串中提取 Mix.Helper"
        );
        assert!(
            !refs.iter().any(|r| r == "Mix.Utils"),
            "不应从注释中提取 Mix.Utils"
        );
    }

    #[test]
    fn strip_doc_false() {
        let content = "@moduledoc false\nFoo.Bar.baz()";
        let cleaned = strip_elixir_non_code(content);
        let refs = scan_elixir_module_refs(&cleaned);
        assert!(refs.contains(&"Foo.Bar".to_string()));
    }

    #[test]
    fn strip_single_line_doc_string() {
        let content = r#"@doc "Returns Foo.Bar result"
def run, do: Baz.Qux.call()"#;
        let cleaned = strip_elixir_non_code(content);
        let refs = scan_elixir_module_refs(&cleaned);
        assert!(
            !refs.iter().any(|r| r == "Foo.Bar"),
            "不应从单行 @doc 中提取"
        );
        assert!(refs.contains(&"Baz.Qux".to_string()), "应保留真实依赖");
    }
}
