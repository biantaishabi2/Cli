//! bcc analyze — AST 气味检测骨架
//!
//! 读取 extract 产出的 FileRecord JSON，遍历每个文件生成 SmellReport。
//! 当前为骨架实现，检测器列表为空，后续 sub-issue 逐步添加规则。

use crate::extract::FileRecord;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs;

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
            // Elixir @spec 类型 ↔ is_* guard 类型
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
            // TypeScript 参数类型 ↔ typeof 结果
            matches!(
                (ann, guard),
                ("string", "string") | ("number", "number") | ("boolean", "boolean")
                | ("bigint", "bigint") | ("symbol", "symbol") | ("object", "object")
                | ("undefined", "undefined")
            ) || {
                // Array 相关：string[] / number[] / Array<T> ↔ Array
                if guard == "Array" {
                    ann.ends_with("[]") || ann.starts_with("Array")
                } else {
                    false
                }
            }
        }
        "rust" => {
            // Rust 类型匹配（预留）
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
        // 查找同函数、同变量的类型标注
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
                    _ => ("error", 0.9, ""),
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

    // 检查 calls 中是否有 .get(field_name, default) 模式
    for call in &record.calls {
        for field_name in &required_fields {
            // 匹配 callee 包含 .get 且行附近有字段名的模式
            // 简化实现：检查 callee 是否是 get/fetch 相关
            if (call.callee.ends_with(".get") || call.callee == "get")
                && call.callee.contains(field_name)
            {
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
#[derive(Debug, Serialize, Deserialize)]
pub struct SmellRecord {
    /// 检测类别：defensive / duplication / security / noise / error_handling
    pub category: String,
    /// 规则标识：redundant_type_check / hardcoded_credential / ...
    pub rule: String,
    /// 严重级别：error / warning / info
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

/// analyze 命令入口
///
/// - ast_file: extract 输出的 JSON 文件路径（Vec<FileRecord> 或单个 FileRecord）
/// - output: SmellReport JSON 输出路径
/// - rules: 可选的逗号分隔规则类别过滤（骨架阶段仅做解析，不影响输出）
pub fn run(ast_file: &str, output: &str, rules: Option<String>) -> Result<(), Box<dyn std::error::Error>> {
    // 解析 rules 参数（骨架阶段仅记录，不过滤）
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

    // 对每个 FileRecord 运行检测器
    let reports: Vec<SmellReport> = records
        .iter()
        .map(|record| {
            let mut smells = Vec::new();

            // 第 1 层：同函数内 TypeAnnotation × TypeGuard 交叉匹配
            smells.extend(detect_redundant_type_check(record));

            // 第 3 层：SchemaField(required) × .get() default 匹配
            smells.extend(detect_unnecessary_default(record));

            // 如有规则过滤，应用之
            if let Some(ref filters) = rule_filter {
                smells.retain(|s| filters.iter().any(|f| s.category == *f || s.rule == *f));
            }

            // 统计
            let mut by_severity: HashMap<String, usize> = HashMap::new();
            let mut by_category: HashMap<String, usize> = HashMap::new();
            for smell in &smells {
                *by_severity.entry(smell.severity.clone()).or_insert(0) += 1;
                *by_category.entry(smell.category.clone()).or_insert(0) += 1;
            }
            let total_smells = smells.len();

            SmellReport {
                file: record.file_path.clone(),
                smells,
                summary: SmellSummary {
                    total_smells,
                    by_severity,
                    by_category,
                },
            }
        })
        .collect();

    // 写出结果
    let json = serde_json::to_string_pretty(&reports)?;
    if let Some(parent) = std::path::Path::new(output).parent() {
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
    use std::io::Write;

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
        let ast_path = dir.path().join("valid.json");
        let out_path = dir.path().join("smells.json");

        // 最小 FileRecord JSON
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
        let ast_path = dir.path().join("single.json");
        let out_path = dir.path().join("smells.json");

        // extract 默认输出的单个 FileRecord（非数组）
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
    fn detect_redundant_type_check_elixir() {
        let dir = tempfile::tempdir().unwrap();
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
        assert_eq!(smell.severity, "warning");
        assert!((smell.confidence - 0.7).abs() < f64::EPSILON);
        assert!(smell.message.contains("pattern dispatch"));
    }

    #[test]
    fn detect_redundant_type_check_typescript() {
        let dir = tempfile::tempdir().unwrap();
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
        assert_eq!(smell.severity, "error");
        assert!((smell.confidence - 0.9).abs() < f64::EPSILON);
    }

    #[test]
    fn no_annotation_no_false_positive() {
        let dir = tempfile::tempdir().unwrap();
        let ast_path = dir.path().join("no_ann.json");
        let out_path = dir.path().join("smells.json");

        // guard 存在但没有对应的 type_annotation → 不应报
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
        let ast_path = dir.path().join("diff_type.json");
        let out_path = dir.path().join("smells.json");

        // annotation=number, guard=string → 类型不兼容，不应报
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

        // 过滤只保留 security 类别 → 不应有 defensive 的结果
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
    fn old_format_ast_backward_compatible() {
        let dir = tempfile::tempdir().unwrap();
        let ast_path = dir.path().join("old.json");
        let out_path = dir.path().join("smells.json");

        // 旧格式：不含 type_annotations/type_guards/schema_fields
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
}
