//! bcc analyze — AST 气味检测
//!
//! 读取 extract 产出的 FileRecord JSON，遍历每个文件生成 SmellReport。
//! 检测器通过 SmellDetector trait 注册，支持 rules 参数按 category 过滤。

pub mod duplication;
pub mod noise;

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
#[derive(Debug, Clone, Serialize, Deserialize)]
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

/// 检测器 trait：每个检测器实现此接口
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

/// 构建所有已注册的检测器
fn build_detectors() -> Vec<Box<dyn SmellDetector>> {
    vec![
        Box::new(noise::CommentDensityDetector::new(0.5)),
        Box::new(noise::LeftoverBoilerplateDetector),
        Box::new(noise::DeadCodeDetector),
        Box::new(duplication::StructuralDuplicationDetector),
        Box::new(duplication::BoilerplateSkeletonDetector),
    ]
}

/// analyze 命令入口
///
/// - ast_file: extract 输出的 JSON 文件路径（Vec<FileRecord> 或单个 FileRecord）
/// - output: SmellReport JSON 输出路径
/// - rules: 可选的逗号分隔规则类别过滤
pub fn run(
    ast_file: &str,
    output: &str,
    rules: Option<String>,
) -> Result<(), Box<dyn std::error::Error>> {
    let rule_filter: Option<Vec<String>> = rules.map(|r| {
        r.split(',')
            .map(|s| s.trim().to_string())
            .filter(|s| !s.is_empty())
            .collect()
    });

    let content = fs::read_to_string(ast_file)
        .map_err(|e| format!("cannot read '{}': {}", ast_file, e))?;

    let records: Vec<FileRecord> = serde_json::from_str::<Vec<FileRecord>>(&content)
        .or_else(|_| serde_json::from_str::<FileRecord>(&content).map(|r| vec![r]))
        .map_err(|e| format!("failed to parse '{}': {}", ast_file, e))?;

    let all_detectors = build_detectors();

    // 按 rules 过滤检测器
    let detectors: Vec<&dyn SmellDetector> = all_detectors
        .iter()
        .filter(|d| match &rule_filter {
            Some(filters) => filters.iter().any(|f| f == d.category()),
            None => true,
        })
        .map(|d| d.as_ref())
        .collect();

    let mut reports: Vec<SmellReport> = records
        .iter()
        .map(|record| {
            // 读取源码：优先从 source_code 字段，否则从 file_path 读取
            let source_code = match &record.source_code {
                Some(code) => code.clone(),
                None => match fs::read_to_string(&record.file_path) {
                    Ok(code) => code,
                    Err(e) => {
                        eprintln!(
                            "[analyze] warning: cannot read source '{}': {}, skipping detectors",
                            record.file_path, e
                        );
                        return SmellReport {
                            file: record.file_path.clone(),
                            summary: SmellSummary {
                                total_smells: 0,
                                by_severity: HashMap::new(),
                                by_category: HashMap::new(),
                            },
                            smells: Vec::new(),
                        };
                    }
                },
            };

            // 尝试 tree-sitter parse（可能失败，如不支持的语言）
            let tree = get_language(&record.language).and_then(|lang| {
                let mut parser = tree_sitter::Parser::new();
                parser.set_language(&lang).ok()?;
                parser.parse(&source_code, None)
            });

            let mut smells = Vec::new();
            if let Some(ref tree) = tree {
                for detector in &detectors {
                    let mut found = detector.detect(record, &source_code, tree);
                    smells.append(&mut found);
                }
            }

            // 构建 summary
            let mut by_severity: HashMap<String, usize> = HashMap::new();
            let mut by_category: HashMap<String, usize> = HashMap::new();
            for smell in &smells {
                *by_severity.entry(smell.severity.clone()).or_insert(0) += 1;
                *by_category.entry(smell.category.clone()).or_insert(0) += 1;
            }

            SmellReport {
                file: record.file_path.clone(),
                summary: SmellSummary {
                    total_smells: smells.len(),
                    by_severity,
                    by_category,
                },
                smells,
            }
        })
        .collect();

    // 跨文件重复检测
    let cross_smells = run_cross_file_detectors(&records, &rule_filter);
    if !cross_smells.is_empty() {
        // 将跨文件 smell 分配到对应文件的 report 中
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

/// 提供给 duplication 检测器的跨文件检测入口
/// 在所有单文件检测完成后，对整个 records 集合做跨文件重复检测
pub fn run_cross_file_detectors(
    records: &[FileRecord],
    rule_filter: &Option<Vec<String>>,
) -> Vec<SmellRecord> {
    let mut smells = Vec::new();

    // StructuralDuplicationDetector 和 BoilerplateSkeletonDetector 需要跨文件比对
    if rule_filter
        .as_ref()
        .map_or(true, |f| f.iter().any(|c| c == "duplication"))
    {
        let mut all_functions = Vec::new();
        for record in records {
            let source_code = match &record.source_code {
                Some(code) => code.clone(),
                None => match fs::read_to_string(&record.file_path) {
                    Ok(code) => code,
                    Err(e) => {
                        eprintln!(
                            "[analyze] warning: cannot read source '{}': {}, skipping cross-file detection",
                            record.file_path, e
                        );
                        continue;
                    }
                },
            };

            if let Some(lang) = get_language(&record.language) {
                let mut parser = tree_sitter::Parser::new();
                if parser.set_language(&lang).is_ok() {
                    if let Some(tree) = parser.parse(&source_code, None) {
                        let funcs = duplication::extract_functions(
                            &record.file_path,
                            &source_code,
                            &tree,
                            &record.language,
                        );
                        all_functions.extend(funcs);
                    }
                }
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

        // 过滤到 defensive 类别，noise/duplication 检测器不应运行
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
