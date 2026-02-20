//! bcc analyze — AST 气味检测
//!
//! 读取 extract 产出的 FileRecord JSON，遍历每个文件运行检测器生成 SmellReport。

pub mod error_handling;
pub mod security;

use crate::extract::FileRecord;
use serde::{Deserialize, Serialize};
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
                        "medium",
                        0.7,
                        " (guard may serve as pattern dispatch)",
                    ),
                    _ => ("high", 0.9, ""),
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
                    severity: "medium".to_string(),
                    message: format!(
                        "Field '{}' is required but accessed with default value",
                        field_name
                    ),
                    file: record.file_path.clone(),
                    line: call.line,
                    source: "bcc".to_string(),
                    confidence: 0.8,
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
    /// 严重级别：high / medium / low
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
}

/// 汇总统计
#[derive(Debug, Serialize, Deserialize)]
pub struct SmellSummary {
    pub total_smells: usize,
    pub by_severity: HashMap<String, usize>,
    pub by_category: HashMap<String, usize>,
}

/// 检测器 trait：每个检测器对源码做独立的模式匹配
pub trait Detector {
    /// 检测器名称（用于日志）
    fn name(&self) -> &str;
    /// 检测器所属类别（security / error_handling）
    fn category(&self) -> &str;
    /// 对源码运行检测，返回发现的 SmellRecord 列表
    fn detect(&self, source: &str, file_path: &str, lang: &str) -> Vec<SmellRecord>;
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

/// 构建所有检测器的注册表
fn build_detectors() -> Vec<Box<dyn Detector>> {
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

/// analyze 命令入口
///
/// - ast_file: extract 输出的 JSON 文件路径（Vec<FileRecord> 或单个 FileRecord）
/// - output: SmellReport JSON 输出路径
/// - rules: 可选的逗号分隔规则类别过滤（如 security,error_handling,defensive）
pub fn run(ast_file: &str, output: &str, rules: Option<String>) -> Result<(), Box<dyn std::error::Error>> {
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

    // 支持 Vec<FileRecord> 和单个 FileRecord 两种输入格式
    let records: Vec<FileRecord> = serde_json::from_str::<Vec<FileRecord>>(&content)
        .or_else(|_| {
            serde_json::from_str::<FileRecord>(&content).map(|r| vec![r])
        })
        .map_err(|e| format!("failed to parse '{}': {}", ast_file, e))?;

    // 构建检测器，按 rules 过滤
    let all_detectors = build_detectors();
    let detectors: Vec<&dyn Detector> = all_detectors
        .iter()
        .filter(|d| {
            match &rule_filter {
                Some(filters) => filters.iter().any(|f| f == d.category()),
                None => true,
            }
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

    let reports: Vec<SmellReport> = records
        .iter()
        .map(|record| {
            let file_path = &record.file_path;
            let lang = &record.language;

            // 跳过测试文件
            if is_test_path(file_path) {
                return SmellReport {
                    file: file_path.clone(),
                    smells: vec![],
                    summary: SmellSummary {
                        total_smells: 0,
                        by_severity: HashMap::new(),
                        by_category: HashMap::new(),
                    },
                };
            }

            let mut smells = Vec::new();

            // defensive 检测器（基于 FileRecord 数据，不需要源码）
            if defensive_enabled {
                smells.extend(detect_redundant_type_check(record));
                smells.extend(detect_unnecessary_default(record));
            }

            // 尝试从磁盘读取源码（先尝试绝对路径，再尝试相对于 AST 文件目录）
            let source = match fs::read_to_string(file_path)
                .or_else(|_| fs::read_to_string(ast_dir.join(file_path)))
            {
                Ok(s) if !s.is_empty() => Some(s),
                Ok(_) => None, // 空文件
                Err(e) => {
                    // 仅当有需要源码的检测器时才报错
                    if !detectors.is_empty() {
                        read_failures.push(format!("{}: {}", file_path, e));
                    }
                    None
                }
            };

            // 源码匹配检测器（security, error_handling）
            if let Some(ref source) = source {
                for detector in &detectors {
                    smells.extend(detector.detect(source, file_path, lang));
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

    // 源码读取失败视为错误，避免假阴性
    if !read_failures.is_empty() {
        return Err(format!(
            "failed to read {} source file(s):\n  {}",
            read_failures.len(),
            read_failures.join("\n  ")
        ).into());
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

        run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            Some("defensive".to_string()),
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

        fs::write(&src_path, "api_key = \"sk-proj-abc123\"\ntry:\n    risky()\nexcept:\n    pass\n").unwrap();

        let ast_json = format!(r#"[{{
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
        }}]"#, src_path.to_str().unwrap());
        fs::write(&ast_path, &ast_json).unwrap();

        // 只筛选 security 类别
        run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            Some("security".to_string()),
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

        let ast_json = format!(r#"[{{
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
        }}]"#, src_path.to_str().unwrap());
        fs::write(&ast_path, &ast_json).unwrap();

        let result = run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            None,
        );
        assert!(result.is_ok(), "Empty source file should not cause failure: {:?}", result.err());

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
        );
        assert!(result.is_err(), "Should return error when source file is unreadable");
        let err_msg = result.unwrap_err().to_string();
        assert!(err_msg.contains("failed to read"), "Error should mention read failure: {}", err_msg);
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

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None).unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(result.len(), 1);
        assert_eq!(result[0].smells.len(), 1);
        let smell = &result[0].smells[0];
        assert_eq!(smell.rule, "redundant_type_check");
        assert_eq!(smell.severity, "medium");
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

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None).unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(result[0].smells.len(), 1);
        let smell = &result[0].smells[0];
        assert_eq!(smell.rule, "redundant_type_check");
        assert_eq!(smell.severity, "high");
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

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None).unwrap();

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

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None).unwrap();

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

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None).unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(result[0].smells.len(), 1);
        let smell = &result[0].smells[0];
        assert_eq!(smell.rule, "unnecessary_default");
        assert_eq!(smell.severity, "medium");
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

        run(ast_path.to_str().unwrap(), out_path.to_str().unwrap(), None).unwrap();

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
