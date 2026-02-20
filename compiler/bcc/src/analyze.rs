//! bcc analyze — AST 气味检测骨架
//!
//! 读取 extract 产出的 FileRecord JSON，遍历每个文件生成 SmellReport。
//! 当前为骨架实现，检测器列表为空，后续 sub-issue 逐步添加规则。

use crate::extract::FileRecord;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs;

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
    let _rule_filter: Option<Vec<String>> = rules.map(|r| {
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

    // 骨架：为每个 FileRecord 生成空的 SmellReport
    let reports: Vec<SmellReport> = records
        .iter()
        .map(|record| SmellReport {
            file: record.file_path.clone(),
            smells: vec![],
            summary: SmellSummary {
                total_smells: 0,
                by_severity: HashMap::new(),
                by_category: HashMap::new(),
            },
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
