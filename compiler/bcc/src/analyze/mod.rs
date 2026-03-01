//! bcc analyze — AST 气味检测
//!
//! 读取 extract 产出的 FileRecord JSON，遍历每个文件运行检测器生成 SmellReport。
//! 检测器分两类：
//! - `Detector`（正则/文本匹配）：security、error_handling
//! - `SmellDetector`（tree-sitter AST）：noise、duplication

pub mod duplication;
pub mod error_handling;
pub mod lang;
pub mod linter;
pub mod noise;
pub mod security;

use crate::extract::FileRecord;
use serde::{Deserialize, Deserializer, Serialize};
use std::collections::HashMap;
use std::fs;
use std::path::Path;

// === 类型兼容映射表 ===

/// 检查 annotation 类型和 guard 类型是否兼容（同一类型的不同表示）
fn type_compatible(ann_type: &str, guard_type: &str, lang: &str) -> bool {
    let ann = ann_type.trim();
    let guard = guard_type.trim();

    // 完全相同
    if ann == guard {
        return true;
    }

    match lang {
        "elixir" => {
            matches!(
                (ann, guard),
                ("map()", "map()") | ("integer()", "integer()") | ("binary()", "binary()")
                | ("atom()", "atom()") | ("list()", "list()") | ("float()", "float()")
                | ("number()", "number()") | ("boolean()", "boolean()")
                | ("tuple()", "tuple()") | ("pid()", "pid()")
                | ("reference()", "reference()") | ("function()", "function()")
                | ("bitstring()", "bitstring()")
            )
        }
        "typescript" | "tsx" => {
            matches!(
                (ann, guard),
                ("string", "string") | ("number", "number") | ("boolean", "boolean")
                | ("bigint", "bigint") | ("symbol", "symbol") | ("object", "object")
                | ("undefined", "undefined")
            ) || {
                if guard == "Array" {
                    ann.ends_with("[]") || ann.starts_with("Array")
                } else {
                    false
                }
            }
        }
        "rust" => {
            if guard == "Some"
                && (ann.starts_with("Option") || ann.starts_with("&Option"))
            {
                return true;
            }
            if (guard == "Ok" || guard == "Err")
                && (ann.starts_with("Result") || ann.starts_with("&Result"))
            {
                return true;
            }
            matches!(
                (ann, guard),
                ("&str", "str") | ("String", "String") | ("Vec", "Vec")
            )
        }
        _ => false,
    }
}

/// 检测冗余类型检查：同函数内 TypeAnnotation × TypeGuard 交叉匹配
fn detect_redundant_type_check(record: &FileRecord) -> Vec<SmellRecord> {
    let mut smells = Vec::new();
    let lang = record.language.as_str();

    for guard in &record.type_guards {
        for ann in &record.type_annotations {
            if ann.func_name == guard.function
                && ann.param == guard.var
                && type_compatible(&ann.type_expr, &guard.guarded_type, lang)
            {
                let (severity, confidence, extra_msg) = match lang {
                    "elixir" => (
                        "warning",
                        0.7,
                        " (guard may serve as pattern dispatch)",
                    ),
                    _ => ("critical", 0.9, ""),
                };
                smells.push(SmellRecord {
                    category: "defensive".to_string(),
                    rule: "redundant_type_check".to_string(),
                    severity: severity.to_string(),
                    message: format!(
                        "Parameter '{}' in '{}' is annotated as '{}' but also checked with '{}'{}",
                        guard.var, guard.function, ann.type_expr, guard.guarded_type, extra_msg
                    ),
                    file: record.file_path.clone(),
                    line: guard.line,
                    source: "bcc".to_string(),
                    confidence,
                    fix_hint: String::new(),
                    code_snippet: String::new(),
                    offending_code: String::new(),
                    suggested_fix: String::new(),
                    evidence: vec![],
                });
            }
        }
    }
    smells
}

/// 检测不必要的默认值：required SchemaField 使用 .get(key, default)
fn detect_unnecessary_default(record: &FileRecord) -> Vec<SmellRecord> {
    let mut smells = Vec::new();

    let required_fields: Vec<&str> = record
        .schema_fields
        .iter()
        .filter(|f| f.required)
        .map(|f| f.name.as_str())
        .collect();

    if required_fields.is_empty() {
        return smells;
    }

    let mut seen: std::collections::HashSet<(usize, String)> = std::collections::HashSet::new();

    for call in &record.calls {
        if !(call.callee.ends_with(".get") || call.callee == "get") {
            continue;
        }
        if call.args.len() < 2 {
            continue;
        }
        for field_name in &required_fields {
            if call.args[0] == *field_name {
                let key = (call.line, field_name.to_string());
                if seen.contains(&key) {
                    continue;
                }
                seen.insert(key);
                smells.push(SmellRecord {
                    category: "defensive".to_string(),
                    rule: "unnecessary_default".to_string(),
                    severity: "warning".to_string(),
                    message: format!(
                        "Field '{}' is required but accessed with default value",
                        field_name
                    ),
                    file: record.file_path.clone(),
                    line: call.line,
                    source: "bcc".to_string(),
                    confidence: 0.8,
                    fix_hint: String::new(),
                    code_snippet: String::new(),
                    offending_code: String::new(),
                    suggested_fix: String::new(),
                    evidence: vec![],
                });
            }
        }
    }
    smells
}

/// 单个文件的气味检测报告
#[derive(Debug, Serialize, Deserialize)]
pub struct SmellReport {
    pub file: String,
    pub smells: Vec<SmellRecord>,
    pub summary: SmellSummary,
}

/// 单条气味记录
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct SmellRecord {
    /// 检测类别：defensive / duplication / security / noise / error_handling
    pub category: String,
    /// 规则标识：redundant_type_check / hardcoded_credential / ...
    pub rule: String,
    /// 严重级别：critical / warning / info（兼容旧值 high→critical, medium→warning, low→info）
    #[serde(deserialize_with = "deserialize_severity")]
    pub severity: String,
    /// 人类可读描述
    pub message: String,
    /// 所在文件路径
    pub file: String,
    /// 所在行号
    pub line: usize,
    /// 检测来源："bcc" / "credo" / "clippy"
    pub source: String,
    /// 置信度 0.0 - 1.0
    pub confidence: f64,
    /// 修复提示
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub fix_hint: String,
    /// 代码片段（含上下文行 + ← HERE 标记）
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub code_snippet: String,
    /// 问题代码行
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub offending_code: String,
    /// 建议修复代码
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub suggested_fix: String,
    /// 证据链
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub evidence: Vec<SmellEvidence>,
}

/// 气味证据
#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct SmellEvidence {
    /// 证据类型：regex_match / ast_pattern / linter_output
    pub kind: String,
    /// 证据描述
    pub detail: String,
}

/// severity 反序列化：将旧值 high→critical, medium→warning, low→info 映射到新值
fn deserialize_severity<'de, D>(deserializer: D) -> Result<String, D::Error>
where
    D: Deserializer<'de>,
{
    let s = String::deserialize(deserializer)?;
    Ok(normalize_severity(&s))
}

/// 将 severity 旧值映射到新值
pub fn normalize_severity(s: &str) -> String {
    match s {
        "high" => "critical".to_string(),
        "medium" => "warning".to_string(),
        "low" => "info".to_string(),
        other => other.to_string(),
    }
}

/// 从源码中提取代码片段（目标行 + 前后 context_lines 行），在目标行末尾加 ← HERE 标记
pub fn extract_code_snippet(source: &str, line: usize, context_lines: usize) -> String {
    let lines: Vec<&str> = source.lines().collect();
    if line == 0 || line > lines.len() {
        return String::new();
    }
    let idx = line - 1;
    let start = idx.saturating_sub(context_lines);
    let end = (idx + context_lines + 1).min(lines.len());
    let mut snippet = String::new();
    for i in start..end {
        let line_num = i + 1;
        if i == idx {
            snippet.push_str(&format!("{:>4} | {}  ← HERE\n", line_num, lines[i]));
        } else {
            snippet.push_str(&format!("{:>4} | {}\n", line_num, lines[i]));
        }
    }
    snippet
}

/// 汇总统计
#[derive(Debug, Serialize, Deserialize)]
pub struct SmellSummary {
    pub total_smells: usize,
    pub by_severity: HashMap<String, usize>,
    pub by_category: HashMap<String, usize>,
}

/// 正则/文本匹配检测器 trait（security, error_handling）
pub trait Detector {
    /// 检测器名称（用于日志）
    fn name(&self) -> &str;
    /// 检测器所属类别（security / error_handling）
    fn category(&self) -> &str;
    /// 对源码运行检测，返回发现的 SmellRecord 列表
    fn detect(&self, source: &str, file_path: &str, lang: &str) -> Vec<SmellRecord>;
}

/// tree-sitter AST 检测器 trait（noise, duplication）
pub trait SmellDetector {
    /// 检测器覆盖的 category（用于 rules 过滤）
    fn category(&self) -> &str;

    /// 对单个文件执行检测，返回发现的 smell 列表
    fn detect(
        &self,
        record: &FileRecord,
        source: &str,
        tree: &tree_sitter::Tree,
    ) -> Vec<SmellRecord>;
}

/// 判断文件路径是否为测试文件（排除测试文件以减少误报）
fn is_test_path(path: &str) -> bool {
    let p = path.to_lowercase();
    p.contains("/test/")
        || p.contains("/tests/")
        || p.contains("/spec/")
        || p.contains("_test.")
        || p.contains(".test.")
        || p.contains("_spec.")
        || p.contains(".spec.")
        || p.starts_with("test/")
        || p.starts_with("tests/")
}

/// 获取语言对应的 tree-sitter Language
fn get_language(lang: &str) -> Option<tree_sitter::Language> {
    match lang {
        "python" => Some(tree_sitter_python::LANGUAGE.into()),
        "elixir" => Some(tree_sitter_elixir::LANGUAGE.into()),
        "typescript" => Some(tree_sitter_typescript::LANGUAGE_TYPESCRIPT.into()),
        "tsx" => Some(tree_sitter_typescript::LANGUAGE_TSX.into()),
        "rust" => Some(tree_sitter_rust::LANGUAGE.into()),
        "php" => Some(tree_sitter_php::LANGUAGE_PHP.into()),
        _ => None,
    }
}

/// 构建正则/文本匹配检测器（security, error_handling）
fn build_regex_detectors() -> Vec<Box<dyn Detector>> {
    vec![
        // 安全检测器
        Box::new(security::HardcodedCredentialDetector),
        Box::new(security::InjectionRiskDetector),
        Box::new(security::UnsafeDeserializationDetector),
        Box::new(security::WeakCryptoDetector),
        Box::new(security::SensitiveDataLogDetector),
        // 错误处理检测器
        Box::new(error_handling::SwallowedErrorDetector),
        Box::new(error_handling::BroadCatchDetector),
    ]
}

/// 构建 tree-sitter AST 检测器（noise，不含 duplication——duplication 仅在跨文件阶段执行）
fn build_ts_detectors() -> Vec<Box<dyn SmellDetector>> {
    vec![
        Box::new(noise::CommentDensityDetector::new(0.5)),
        Box::new(noise::LeftoverBoilerplateDetector),
        Box::new(noise::DeadCodeDetector),
    ]
}

/// analyze 命令入口
///
/// - ast_file: extract 输出的 JSON 文件路径（Vec<FileRecord> 或单个 FileRecord）
/// - output: SmellReport JSON 输出路径
/// - rules: 可选的逗号分隔规则类别过滤（如 security,error_handling,noise,duplication,defensive）
pub fn run(ast_file: &str, output: &str, rules: Option<String>, linters: Vec<String>) -> Result<(), Box<dyn std::error::Error>> {
    // 解析 rules 参数
    let rule_filter: Option<Vec<String>> = rules.map(|r| {
        r.split(',')
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
            .collect()
    });

    // 读取 AST JSON
    let content = fs::read_to_string(ast_file)
        .map_err(|e| format!("cannot read '{}': {}", ast_file, e))?;

    // 支持三种输入格式：Vec<FileRecord>、单个 FileRecord、{ source_count, records: [...] } wrapper
    let records: Vec<FileRecord> = serde_json::from_str::<Vec<FileRecord>>(&content)
        .or_else(|_| serde_json::from_str::<FileRecord>(&content).map(|r| vec![r]))
        .or_else(|_| {
            #[derive(Deserialize)]
            struct AstBatch { records: Vec<FileRecord> }
            serde_json::from_str::<AstBatch>(&content).map(|b| b.records)
        })
        .map_err(|e| format!("failed to parse '{}': {}", ast_file, e))?;

    // 构建两类检测器，按 rules 过滤
    let all_regex = build_regex_detectors();
    let regex_detectors: Vec<&dyn Detector> = all_regex
        .iter()
        .filter(|d| match &rule_filter {
            Some(filters) => filters.iter().any(|f| f == d.category()),
            None => true,
        })
        .map(|d| d.as_ref())
        .collect();

    let all_ts = build_ts_detectors();
    let ts_detectors: Vec<&dyn SmellDetector> = all_ts
        .iter()
        .filter(|d| match &rule_filter {
            Some(filters) => filters.iter().any(|f| f == d.category()),
            None => true,
        })
        .map(|d| d.as_ref())
        .collect();

    // defensive 检测器是否启用
    let defensive_enabled = rule_filter
        .as_ref()
        .map_or(true, |f| f.iter().any(|c| c == "defensive"));

    // 推断 AST JSON 所在目录，用于解析相对路径
    let ast_dir = Path::new(ast_file)
        .parent()
        .unwrap_or_else(|| Path::new("."));

    // 收集源码读取失败的文件，最终作为错误返回
    let mut read_failures: Vec<String> = Vec::new();

    // 一次 parse：为每个文件读取源码并 parse，共享给单文件和跨文件检测器
    let parsed_files: Vec<Option<(String, Option<tree_sitter::Tree>)>> = records
        .iter()
        .map(|record| {
            let file_path = &record.file_path;

            // 跳过测试文件
            if is_test_path(file_path) {
                return None;
            }

            // 读取源码：优先 source_code 字段，其次绝对路径，再次相对于 AST 文件目录
            let source_code = match &record.source_code {
                Some(code) if !code.is_empty() => code.clone(),
                _ => match fs::read_to_string(file_path)
                    .or_else(|_| fs::read_to_string(ast_dir.join(file_path)))
                {
                    Ok(s) if !s.is_empty() => s,
                    Ok(_) => return None, // 空文件
                    Err(e) => {
                        read_failures.push(format!("{}: {}", file_path, e));
                        return None;
                    }
                },
            };

            let tree = get_language(&record.language).and_then(|lang| {
                let mut parser = tree_sitter::Parser::new();
                parser.set_language(&lang).ok()?;
                parser.parse(&source_code, None)
            });
            Some((source_code, tree))
        })
        .collect();

    // 单文件检测（同时运行三类检测器）
    let mut reports: Vec<SmellReport> = records
        .iter()
        .zip(parsed_files.iter())
        .map(|(record, parsed)| {
            let file_path = &record.file_path;
            let lang = &record.language;

            // defensive 检测器先跑（基于 FileRecord 数据，不需要源码）
            let mut smells = Vec::new();
            if defensive_enabled {
                smells.extend(detect_redundant_type_check(record));
                smells.extend(detect_unnecessary_default(record));
            }

            let (source_code, tree) = match parsed {
                Some((src, t)) => (src.as_str(), t),
                None => {
                    if smells.is_empty() {
                        return SmellReport {
                            file: file_path.clone(),
                            smells: vec![],
                            summary: SmellSummary {
                                total_smells: 0,
                                by_severity: HashMap::new(),
                                by_category: HashMap::new(),
                            },
                        };
                    } else {
                        // defensive smells 存在但没有源码，直接用 defensive 结果
                        let total_smells = smells.len();
                        let mut by_severity: HashMap<String, usize> = HashMap::new();
                        let mut by_category: HashMap<String, usize> = HashMap::new();
                        for s in &smells {
                            *by_severity.entry(s.severity.clone()).or_insert(0) += 1;
                            *by_category.entry(s.category.clone()).or_insert(0) += 1;
                        }
                        return SmellReport {
                            file: file_path.clone(),
                            smells,
                            summary: SmellSummary {
                                total_smells,
                                by_severity,
                                by_category,
                            },
                        };
                    }
                }
            };

            // 正则/文本匹配检测器
            for detector in &regex_detectors {
                smells.extend(detector.detect(source_code, file_path, lang));
            }

            // tree-sitter AST 检测器
            if let Some(ref tree) = tree {
                for detector in &ts_detectors {
                    smells.extend(detector.detect(record, source_code, tree));
                }
            }

            // 构建汇总
            let total_smells = smells.len();
            let mut by_severity: HashMap<String, usize> = HashMap::new();
            let mut by_category: HashMap<String, usize> = HashMap::new();
            for s in &smells {
                *by_severity.entry(s.severity.clone()).or_insert(0) += 1;
                *by_category.entry(s.category.clone()).or_insert(0) += 1;
            }

            SmellReport {
                file: file_path.clone(),
                smells,
                summary: SmellSummary {
                    total_smells,
                    by_severity,
                    by_category,
                },
            }
        })
        .collect();

    // 跨文件重复检测（复用已 parse 的数据）
    let cross_smells = run_cross_file_detectors(&records, &parsed_files, &rule_filter);
    if !cross_smells.is_empty() {
        let report_map: HashMap<String, usize> = reports
            .iter()
            .enumerate()
            .map(|(i, r)| (r.file.clone(), i))
            .collect();
        for smell in cross_smells {
            if let Some(&idx) = report_map.get(&smell.file) {
                reports[idx].smells.push(smell);
            }
        }
        // 重新计算受影响 report 的 summary
        for report in &mut reports {
            let total = report.smells.len();
            if total != report.summary.total_smells {
                let mut by_severity: HashMap<String, usize> = HashMap::new();
                let mut by_category: HashMap<String, usize> = HashMap::new();
                for smell in &report.smells {
                    *by_severity.entry(smell.severity.clone()).or_insert(0) += 1;
                    *by_category.entry(smell.category.clone()).or_insert(0) += 1;
                }
                report.summary = SmellSummary {
                    total_smells: total,
                    by_severity,
                    by_category,
                };
            }
        }
    }

    // 源码读取失败视为错误，避免假阴性
    if !read_failures.is_empty() {
        return Err(format!(
            "failed to read {} source file(s):\n  {}",
            read_failures.len(),
            read_failures.join("\n  ")
        )
        .into());
    }

    // 合并外部 linter 结果
    let mut reports = reports;
    if !linters.is_empty() {
        let external_smells = linter::run_linters(&linters);
        if !external_smells.is_empty() {
            // 按文件分组合并到对应的 SmellReport
            for smell in external_smells {
                if let Some(report) = reports.iter_mut().find(|r| r.file == smell.file) {
                    *report.summary.by_severity.entry(smell.severity.clone()).or_insert(0) += 1;
                    *report.summary.by_category.entry(smell.category.clone()).or_insert(0) += 1;
                    report.summary.total_smells += 1;
                    report.smells.push(smell);
                } else {
                    // 外部 linter 报告了不在 AST 中的文件，创建新 report
                    let mut by_severity = HashMap::new();
                    by_severity.insert(smell.severity.clone(), 1);
                    let mut by_category = HashMap::new();
                    by_category.insert(smell.category.clone(), 1);
                    let file = smell.file.clone();
                    reports.push(SmellReport {
                        file,
                        smells: vec![smell],
                        summary: SmellSummary {
                            total_smells: 1,
                            by_severity,
                            by_category,
                        },
                    });
                }
            }
        }
    }

    // 写出结果
    let json = serde_json::to_string_pretty(&reports)?;
    if let Some(parent) = Path::new(output).parent() {
        fs::create_dir_all(parent).ok();
    }
    fs::write(output, &json)?;
    eprintln!(
        "[analyze] {} files processed, {} total smells → {}",
        reports.len(),
        reports.iter().map(|r| r.summary.total_smells).sum::<usize>(),
        output
    );

    Ok(())
}

/// 跨文件重复检测入口（复用已 parse 的数据）
pub fn run_cross_file_detectors(
    records: &[FileRecord],
    parsed_files: &[Option<(String, Option<tree_sitter::Tree>)>],
    rule_filter: &Option<Vec<String>>,
) -> Vec<SmellRecord> {
    let mut smells = Vec::new();

    if rule_filter
        .as_ref()
        .map_or(true, |f| f.iter().any(|c| c == "duplication"))
    {
        let mut all_functions = Vec::new();
        for (record, parsed) in records.iter().zip(parsed_files.iter()) {
            if let Some((source_code, Some(tree))) = parsed {
                let funcs = duplication::extract_functions(
                    &record.file_path,
                    source_code,
                    tree,
                    &record.language,
                );
                all_functions.extend(funcs);
            }
        }
        smells.extend(duplication::detect_structural_duplication(&all_functions));
        smells.extend(duplication::detect_boilerplate_skeleton(&all_functions));
    }

    smells
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn empty_ast_produces_empty_reports() {
        let dir = tempfile::tempdir().unwrap();
        let ast_path = dir.path().join("empty.json");
        let out_path = dir.path().join("smells.json");

        fs::write(&ast_path, "[]").unwrap();

        run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            None,
            vec![],
        )
        .unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert!(result.is_empty());
    }

    #[test]
    fn valid_ast_with_rules_filter() {
        let dir = tempfile::tempdir().unwrap();
        let src_path = dir.path().join("lib");
        fs::create_dir_all(&src_path).unwrap();
        fs::write(src_path.join("foo.ex"), "defmodule Foo do\n  def bar, do: :ok\nend\n").unwrap();

        let ast_path = dir.path().join("valid.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = r#"[{
            "language": "elixir",
            "file_path": "lib/foo.ex",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [],
            "side_effects": {
                "hasAsync": false,
                "hasHttp": false,
                "hasGenserver": false,
                "hasFileIo": false,
                "hasPubsub": false
            },
            "loc_lines": 10,
            "declarations": 2
        }]"#;
        fs::write(&ast_path, ast_json).unwrap();

        // 过滤到 defensive 类别，所有检测器都不匹配
        run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            Some("defensive".to_string()),
            vec![],
        )
        .unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(result.len(), 1);
        assert_eq!(result[0].file, "lib/foo.ex");
        assert!(result[0].smells.is_empty());
        assert_eq!(result[0].summary.total_smells, 0);
    }

    #[test]
    fn single_file_record_input() {
        let dir = tempfile::tempdir().unwrap();
        let src_path = dir.path().join("lib");
        fs::create_dir_all(&src_path).unwrap();
        fs::write(src_path.join("bar.ex"), "defmodule Bar do\nend\n").unwrap();

        let ast_path = dir.path().join("single.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = r#"{
            "language": "elixir",
            "file_path": "lib/bar.ex",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [],
            "side_effects": {
                "hasAsync": false,
                "hasHttp": false,
                "hasGenserver": false,
                "hasFileIo": false,
                "hasPubsub": false
            },
            "loc_lines": 20,
            "declarations": 3
        }"#;
        fs::write(&ast_path, ast_json).unwrap();

        run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            None,
            vec![],
        )
        .unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(result.len(), 1);
        assert_eq!(result[0].file, "lib/bar.ex");
    }

    #[test]
    fn old_format_ast_backward_compatible() {
        let dir = tempfile::tempdir().unwrap();
        let src_path = dir.path().join("src");
        fs::create_dir_all(&src_path).unwrap();
        fs::write(src_path.join("main.rs"), "fn main() {}\n").unwrap();

        let ast_path = dir.path().join("old.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = r#"[{
            "language": "rust",
            "file_path": "src/main.rs",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [],
            "side_effects": {
                "hasAsync": false,
                "hasHttp": false,
                "hasGenserver": false,
                "hasFileIo": false,
                "hasPubsub": false
            },
            "loc_lines": 5,
            "declarations": 1
        }]"#;
        fs::write(&ast_path, ast_json).unwrap();

        // 不应 panic，新字段默认为空 Vec
        run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            None,
            vec![],
        )
        .unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(result.len(), 1);
    }

    #[test]
    fn is_test_path_excludes_test_files() {
        assert!(is_test_path("test/foo_test.py"));
        assert!(is_test_path("tests/unit/bar.py"));
        assert!(is_test_path("src/foo_test.rs"));
        assert!(is_test_path("lib/foo.spec.ts"));
        assert!(!is_test_path("lib/foo.ex"));
        assert!(!is_test_path("src/main.rs"));
    }

    #[test]
    fn rules_filter_selects_category() {
        let dir = tempfile::tempdir().unwrap();
        let src_path = dir.path().join("vuln.py");
        let ast_path = dir.path().join("ast.json");
        let out_path = dir.path().join("smells.json");

        fs::write(
            &src_path,
            "api_key = \"sk-proj-abc123\"\ntry:\n    risky()\nexcept:\n    pass\n",
        )
        .unwrap();

        let ast_json = format!(
            r#"[{{
            "language": "python",
            "file_path": "{}",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [],
            "side_effects": {{
                "hasAsync": false,
                "hasHttp": false,
                "hasGenserver": false,
                "hasFileIo": false,
                "hasPubsub": false
            }},
            "loc_lines": 5,
            "declarations": 0
        }}]"#,
            src_path.to_str().unwrap()
        );
        fs::write(&ast_path, &ast_json).unwrap();

        // 只筛选 security 类别
        run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            Some("security".to_string()),
            vec![],
        )
        .unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(result.len(), 1);
        for smell in &result[0].smells {
            assert_eq!(smell.category, "security");
        }
    }

    #[test]
    fn empty_source_file_succeeds() {
        let dir = tempfile::tempdir().unwrap();
        let src_path = dir.path().join("empty.py");
        fs::write(&src_path, "").unwrap();

        let ast_path = dir.path().join("ast.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = format!(
            r#"[{{
            "language": "python",
            "file_path": "{}",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [],
            "side_effects": {{
                "hasAsync": false,
                "hasHttp": false,
                "hasGenserver": false,
                "hasFileIo": false,
                "hasPubsub": false
            }},
            "loc_lines": 0,
            "declarations": 0
        }}]"#,
            src_path.to_str().unwrap()
        );
        fs::write(&ast_path, &ast_json).unwrap();

        let result = run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            None,
            vec![],
        );
        assert!(
            result.is_ok(),
            "Empty source file should not cause failure: {:?}",
            result.err()
        );

        let reports: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(reports.len(), 1);
        assert!(reports[0].smells.is_empty());
    }

    #[test]
    fn unreadable_source_returns_error() {
        let dir = tempfile::tempdir().unwrap();
        let ast_path = dir.path().join("missing_src.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = r#"[{
            "language": "python",
            "file_path": "/nonexistent/path/missing.py",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [],
            "side_effects": {
                "hasAsync": false,
                "hasHttp": false,
                "hasGenserver": false,
                "hasFileIo": false,
                "hasPubsub": false
            },
            "loc_lines": 5,
            "declarations": 0
        }]"#;
        fs::write(&ast_path, ast_json).unwrap();

        let result = run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            None,
            vec![],
        );
        assert!(
            result.is_err(),
            "Should return error when source file is unreadable"
        );
        let err_msg = result.unwrap_err().to_string();
        assert!(
            err_msg.contains("failed to read"),
            "Error should mention read failure: {}",
            err_msg
        );
    }

    #[test]
    fn detect_redundant_type_check_elixir() {
        let dir = tempfile::tempdir().unwrap();
        fs::create_dir_all(dir.path().join("lib")).unwrap();
        fs::write(dir.path().join("lib/foo.ex"), "defmodule Foo do\nend\n").unwrap();
        let ast_path = dir.path().join("elixir_guard.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = r#"[{
            "language": "elixir",
            "file_path": "lib/foo.ex",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [],
            "side_effects": { "hasAsync": false, "hasHttp": false, "hasGenserver": false, "hasFileIo": false, "hasPubsub": false },
            "loc_lines": 10,
            "declarations": 1,
            "type_annotations": [
                { "name": "@spec process", "type_expr": "map()", "line": 2, "func_name": "process", "param": "data" }
            ],
            "type_guards": [
                { "function": "process", "guarded_type": "map()", "line": 3, "var": "data" }
            ]
        }]"#;
        fs::write(&ast_path, ast_json).unwrap();

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None, vec![]).unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(result.len(), 1);
        assert_eq!(result[0].smells.len(), 1);
        let smell = &result[0].smells[0];
        assert_eq!(smell.rule, "redundant_type_check");
        assert_eq!(smell.severity, "warning");
        assert!((smell.confidence - 0.7).abs() < f64::EPSILON);
        assert!(smell.message.contains("pattern dispatch"));
    }

    #[test]
    fn detect_redundant_type_check_typescript() {
        let dir = tempfile::tempdir().unwrap();
        fs::create_dir_all(dir.path().join("src")).unwrap();
        fs::write(dir.path().join("src/foo.ts"), "// dummy\n").unwrap();
        let ast_path = dir.path().join("ts_guard.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = r#"[{
            "language": "typescript",
            "file_path": "src/foo.ts",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [],
            "side_effects": { "hasAsync": false, "hasHttp": false, "hasGenserver": false, "hasFileIo": false, "hasPubsub": false },
            "loc_lines": 10,
            "declarations": 1,
            "type_annotations": [
                { "name": "param:x", "type_expr": "string", "line": 1, "func_name": "foo", "param": "x" }
            ],
            "type_guards": [
                { "function": "foo", "guarded_type": "string", "line": 2, "var": "x" }
            ]
        }]"#;
        fs::write(&ast_path, ast_json).unwrap();

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None, vec![]).unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(result[0].smells.len(), 1);
        let smell = &result[0].smells[0];
        assert_eq!(smell.rule, "redundant_type_check");
        assert_eq!(smell.severity, "critical");
        assert!((smell.confidence - 0.9).abs() < f64::EPSILON);
    }

    #[test]
    fn no_annotation_no_false_positive() {
        let dir = tempfile::tempdir().unwrap();
        fs::create_dir_all(dir.path().join("src")).unwrap();
        fs::write(dir.path().join("src/foo.ts"), "// dummy\n").unwrap();
        let ast_path = dir.path().join("no_ann.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = r#"[{
            "language": "typescript",
            "file_path": "src/foo.ts",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [],
            "side_effects": { "hasAsync": false, "hasHttp": false, "hasGenserver": false, "hasFileIo": false, "hasPubsub": false },
            "loc_lines": 10,
            "declarations": 1,
            "type_annotations": [],
            "type_guards": [
                { "function": "foo", "guarded_type": "string", "line": 2, "var": "x" }
            ]
        }]"#;
        fs::write(&ast_path, ast_json).unwrap();

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None, vec![]).unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert!(result[0].smells.is_empty());
    }

    #[test]
    fn different_type_no_false_positive() {
        let dir = tempfile::tempdir().unwrap();
        fs::create_dir_all(dir.path().join("src")).unwrap();
        fs::write(dir.path().join("src/foo.ts"), "// dummy\n").unwrap();
        let ast_path = dir.path().join("diff_type.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = r#"[{
            "language": "typescript",
            "file_path": "src/foo.ts",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [],
            "side_effects": { "hasAsync": false, "hasHttp": false, "hasGenserver": false, "hasFileIo": false, "hasPubsub": false },
            "loc_lines": 10,
            "declarations": 1,
            "type_annotations": [
                { "name": "param:x", "type_expr": "number", "line": 1, "func_name": "foo", "param": "x" }
            ],
            "type_guards": [
                { "function": "foo", "guarded_type": "string", "line": 2, "var": "x" }
            ]
        }]"#;
        fs::write(&ast_path, ast_json).unwrap();

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None, vec![]).unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert!(result[0].smells.is_empty());
    }

    #[test]
    fn rule_filter_works() {
        let dir = tempfile::tempdir().unwrap();
        fs::create_dir_all(dir.path().join("src")).unwrap();
        fs::write(dir.path().join("src/foo.ts"), "// dummy\n").unwrap();
        let ast_path = dir.path().join("filter.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = r#"[{
            "language": "typescript",
            "file_path": "src/foo.ts",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [],
            "side_effects": { "hasAsync": false, "hasHttp": false, "hasGenserver": false, "hasFileIo": false, "hasPubsub": false },
            "loc_lines": 10,
            "declarations": 1,
            "type_annotations": [
                { "name": "param:x", "type_expr": "string", "line": 1, "func_name": "foo", "param": "x" }
            ],
            "type_guards": [
                { "function": "foo", "guarded_type": "string", "line": 2, "var": "x" }
            ]
        }]"#;
        fs::write(&ast_path, ast_json).unwrap();

        // 过滤只保留 security → defensive 的结果不应出现
        run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            Some("security".to_string()),
            vec![],
        )
        .unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert!(result[0].smells.is_empty());
    }

    #[test]
    fn detect_unnecessary_default_required_field() {
        let dir = tempfile::tempdir().unwrap();
        fs::create_dir_all(dir.path().join("src")).unwrap();
        fs::write(dir.path().join("src/user.ts"), "// dummy\n").unwrap();
        let ast_path = dir.path().join("required.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = r#"[{
            "language": "typescript",
            "file_path": "src/user.ts",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [
                { "callee": "data.get", "line": 10, "args": ["email", ""] }
            ],
            "side_effects": { "hasAsync": false, "hasHttp": false, "hasGenserver": false, "hasFileIo": false, "hasPubsub": false },
            "loc_lines": 20,
            "declarations": 1,
            "schema_fields": [
                { "name": "email", "field_type": "string", "line": 3, "required": true }
            ]
        }]"#;
        fs::write(&ast_path, ast_json).unwrap();

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None, vec![]).unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(result[0].smells.len(), 1);
        let smell = &result[0].smells[0];
        assert_eq!(smell.rule, "unnecessary_default");
        assert_eq!(smell.severity, "warning");
        assert!((smell.confidence - 0.8).abs() < f64::EPSILON);
        assert!(smell.message.contains("email"));
    }

    #[test]
    fn detect_unnecessary_default_optional_field_no_report() {
        let dir = tempfile::tempdir().unwrap();
        fs::create_dir_all(dir.path().join("src")).unwrap();
        fs::write(dir.path().join("src/user.ts"), "// dummy\n").unwrap();
        let ast_path = dir.path().join("optional.json");
        let out_path = dir.path().join("smells.json");

        let ast_json = r#"[{
            "language": "typescript",
            "file_path": "src/user.ts",
            "module_doc": null,
            "exports": [],
            "imports": [],
            "calls": [
                { "callee": "data.get", "line": 10, "args": ["nickname", ""] }
            ],
            "side_effects": { "hasAsync": false, "hasHttp": false, "hasGenserver": false, "hasFileIo": false, "hasPubsub": false },
            "loc_lines": 20,
            "declarations": 1,
            "schema_fields": [
                { "name": "nickname", "field_type": "string", "line": 5, "required": false }
            ]
        }]"#;
        fs::write(&ast_path, ast_json).unwrap();

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None, vec![]).unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert!(result[0].smells.is_empty());
    }

    #[test]
    fn detect_unnecessary_default_e2e_typescript_no_duplicate() {
        let ts_src = r#"
interface UserSchema {
    email: string;
    nickname?: string;
}

function process(data: Map<string, string>) {
    const email = data.get("email", "");
    const nick = data.get("nickname", "anon");
}
"#;
        let record = crate::extract::typescript::extract(ts_src, "test.ts", "typescript");

        assert!(
            record.schema_fields.iter().any(|f| f.name == "email" && f.required),
            "should extract required field 'email'"
        );
        assert!(
            record.schema_fields.iter().any(|f| f.name == "nickname" && !f.required),
            "should extract optional field 'nickname'"
        );

        let get_calls: Vec<_> = record.calls.iter().filter(|c| c.callee.ends_with(".get") || c.callee == "get").collect();
        assert!(!get_calls.is_empty(), "should have .get calls extracted");

        let smells = detect_unnecessary_default(&record);

        let email_smells: Vec<_> = smells.iter().filter(|s| s.message.contains("email")).collect();
        assert_eq!(email_smells.len(), 1, "required field 'email' should produce exactly 1 smell, got {}", email_smells.len());

        let nick_smells: Vec<_> = smells.iter().filter(|s| s.message.contains("nickname")).collect();
        assert_eq!(nick_smells.len(), 0, "optional field 'nickname' should produce 0 smells");
    }
}
