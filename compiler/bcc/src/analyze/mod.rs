//! bcc analyze — AST 气味检测
//!
//! 读取 extract 产出的 FileRecord JSON，遍历每个文件运行检测器生成 SmellReport。

pub mod error_handling;
pub mod linter;
pub mod security;

use crate::extract::FileRecord;
use serde::{Deserialize, Deserializer, Serialize};
use std::collections::HashMap;
use std::fs;
use std::path::Path;

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
/// - rules: 可选的逗号分隔规则类别过滤（如 security,error_handling）
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

            // 尝试从磁盘读取源码（先尝试绝对路径，再尝试相对于 AST 文件目录）
            let source = match fs::read_to_string(file_path)
                .or_else(|_| fs::read_to_string(ast_dir.join(file_path)))
            {
                Ok(s) if !s.is_empty() => s,
                Ok(_) => {
                    // 空文件是合法的，直接返回空报告（不视为读取失败）
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
                Err(e) => {
                    read_failures.push(format!("{}: {}", file_path, e));
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
            };

            // 遍历所有检测器收集结果
            let mut smells = Vec::new();
            for detector in &detectors {
                smells.extend(detector.detect(&source, file_path, lang));
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
        let src_file = src_path.join("foo.ex");
        fs::write(&src_file, "defmodule Foo do\n  def bar, do: :ok\nend\n").unwrap();

        let ast_path = dir.path().join("valid.json");
        let out_path = dir.path().join("smells.json");

        // 最小 FileRecord JSON，file_path 使用相对于 AST 文件的路径
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

        // 源码含安全和错误处理两种问题
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
            vec![],
        )
        .unwrap();

        let result: Vec<SmellReport> =
            serde_json::from_str(&fs::read_to_string(&out_path).unwrap()).unwrap();
        assert_eq!(result.len(), 1);
        // 应该只有 security 类别的 smell，不含 error_handling
        for smell in &result[0].smells {
            assert_eq!(smell.category, "security");
        }
    }

    #[test]
    fn empty_source_file_succeeds() {
        let dir = tempfile::tempdir().unwrap();
        let src_path = dir.path().join("empty.py");
        fs::write(&src_path, "").unwrap(); // 合法的空文件

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

        // 空文件不应导致失败
        let result = run(
            ast_path.to_str().unwrap(),
            out_path.to_str().unwrap(),
            None,
            vec![],
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

        // file_path 指向不存在的源码文件
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
        assert!(result.is_err(), "Should return error when source file is unreadable");
        let err_msg = result.unwrap_err().to_string();
        assert!(err_msg.contains("failed to read"), "Error should mention read failure: {}", err_msg);
    }
}
