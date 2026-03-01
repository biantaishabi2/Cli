use std::fs;
use std::path::{Path, PathBuf};

/// 必须存在的章节列表
const REQUIRED_SECTIONS: &[&str] = &[
    "职责",
    "行为意图",
    "行为",
    "约束",
    "输入输出",
    "调用链位置",
    "状态与副作用",
    "异常处理",
    "与 PRD 需求映射",
    "溯源证据",
];

struct TraceResult {
    total_files: usize,
    with_docs: usize,
    missing_docs: Vec<String>,
    section_gaps: Vec<(String, Vec<String>)>,
}

/// 收集源文件列表（递归，排除测试）
fn collect_source_files(dir: &Path) -> Vec<PathBuf> {
    let mut files = Vec::new();
    if let Ok(entries) = fs::read_dir(dir) {
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_dir() {
                let name = path.file_name().unwrap_or_default().to_string_lossy();
                if name == "_build" || name == "target" || name == "node_modules" || name == "deps" || name == "__tests__" {
                    continue;
                }
                files.extend(collect_source_files(&path));
            } else {
                let ext = path.extension().unwrap_or_default().to_string_lossy();
                if matches!(ext.as_ref(), "ex" | "exs" | "go" | "rs" | "php" | "ts" | "tsx") {
                    // 排除测试文件
                    let name = path.file_name().unwrap_or_default().to_string_lossy();
                    if !name.contains("_test.") && !name.contains(".test.") && !name.contains("_spec.") {
                        files.push(path);
                    }
                }
            }
        }
    }
    files
}

/// 计算源文件相对路径
fn relative_path(file: &Path, base: &Path) -> String {
    file.strip_prefix(base).unwrap_or(file).to_string_lossy().to_string()
}

/// 对应的文档路径
fn doc_path_for(rel: &str, docs_dir: &Path) -> PathBuf {
    docs_dir.join(format!("{}.md", rel))
}

/// 检查文档中缺少的章节
fn check_sections(doc_path: &Path) -> Vec<String> {
    let content = match fs::read_to_string(doc_path) {
        Ok(c) => c,
        Err(_) => return REQUIRED_SECTIONS.iter().map(|s| s.to_string()).collect(),
    };

    let mut missing = Vec::new();
    for section in REQUIRED_SECTIONS {
        let heading = format!("## {}", section);
        if let Some(pos) = content.find(&heading) {
            // 检查章节是否有内容（heading 后面到下一个 ## 之间不全是空行）
            let after = &content[pos + heading.len()..];
            let next_heading = after.find("\n## ");
            let section_body = match next_heading {
                Some(idx) => &after[..idx],
                None => after,
            };
            let body_trimmed = section_body.trim();
            if body_trimmed.is_empty() {
                missing.push(format!("{} (空)", section));
            }
        } else {
            missing.push(section.to_string());
        }
    }
    missing
}

fn run_trace(source_dir: &str, docs_dir: &str) -> TraceResult {
    let source_path = Path::new(source_dir);
    let docs_path = Path::new(docs_dir);

    let files = collect_source_files(source_path);
    let total = files.len();
    let mut with_docs = 0;
    let mut missing_docs = Vec::new();
    let mut section_gaps = Vec::new();

    for file in &files {
        let rel = relative_path(file, source_path);
        let dp = doc_path_for(&rel, docs_path);
        if dp.exists() {
            with_docs += 1;
            let gaps = check_sections(&dp);
            if !gaps.is_empty() {
                section_gaps.push((rel, gaps));
            }
        } else {
            missing_docs.push(rel);
        }
    }

    TraceResult {
        total_files: total,
        with_docs,
        missing_docs,
        section_gaps,
    }
}

pub fn status(source_dir: &str, docs_dir: &str) {
    let result = run_trace(source_dir, docs_dir);

    println!("总文件: {}", result.total_files);
    println!("已有文档: {}", result.with_docs);
    println!("缺口: {}", result.missing_docs.len());

    if !result.missing_docs.is_empty() {
        println!("\n缺少文档的文件:");
        for f in &result.missing_docs {
            println!("  - {}", f);
        }
    }

    if !result.section_gaps.is_empty() {
        println!("\n章节缺口:");
        for (file, gaps) in &result.section_gaps {
            println!("  {}:", file);
            for gap in gaps {
                println!("    - {}", gap);
            }
        }
    }
}

pub fn report(source_dir: &str, docs_dir: &str, output: &str) {
    let result = run_trace(source_dir, docs_dir);
    let output_path = Path::new(output);
    fs::create_dir_all(output_path).expect("failed to create output directory");

    // 生成审计报告 markdown
    let mut report = String::new();
    report.push_str("# 后端文档覆盖审计报告\n\n");
    report.push_str(&format!("- 总文件: {}\n", result.total_files));
    report.push_str(&format!("- 已有文档: {}\n", result.with_docs));
    report.push_str(&format!("- 缺口: {}\n\n", result.missing_docs.len()));

    if !result.missing_docs.is_empty() {
        report.push_str("## 缺少文档的文件\n\n");
        for f in &result.missing_docs {
            report.push_str(&format!("- {}\n", f));
        }
        report.push('\n');
    }

    if !result.section_gaps.is_empty() {
        report.push_str("## 章节缺口\n\n");
        for (file, gaps) in &result.section_gaps {
            report.push_str(&format!("### {}\n", file));
            for gap in gaps {
                report.push_str(&format!("- {}\n", gap));
            }
            report.push('\n');
        }
    }

    let report_file = output_path.join("backend-docs-audit.md");
    fs::write(&report_file, &report).expect("failed to write report");

    // 生成 TSV 缺口清单
    let mut tsv = String::from("file\tmissing_section\n");
    for f in &result.missing_docs {
        tsv.push_str(&format!("{}\t(no doc)\n", f));
    }
    for (file, gaps) in &result.section_gaps {
        for gap in gaps {
            tsv.push_str(&format!("{}\t{}\n", file, gap));
        }
    }

    let tsv_file = output_path.join("backend-docs-audit-missing.tsv");
    fs::write(&tsv_file, &tsv).expect("failed to write TSV");

    println!("Report: {}", report_file.display());
    println!("Missing: {}", tsv_file.display());
}

pub fn seed(source_dir: &str, docs_dir: &str, write: bool, max: Option<usize>) {
    let source_path = Path::new(source_dir);
    let docs_path = Path::new(docs_dir);

    let files = collect_source_files(source_path);
    let mut created = 0;
    let limit = max.unwrap_or(usize::MAX);

    for file in &files {
        if created >= limit {
            break;
        }
        let rel = relative_path(file, source_path);
        let dp = doc_path_for(&rel, docs_path);
        if !dp.exists() {
            if write {
                if let Some(parent) = dp.parent() {
                    fs::create_dir_all(parent).ok();
                }
                let template = generate_template(&rel);
                fs::write(&dp, &template).expect("failed to write template");
                println!("Created: {}", dp.display());
            } else {
                println!("Would create: {}", dp.display());
            }
            created += 1;
        }
    }

    if created == 0 {
        println!("No missing docs to seed.");
    } else {
        println!("Created {} template(s).", created);
    }
}

fn generate_template(rel_path: &str) -> String {
    format!(
        r#"# {}

## 职责
- （待补充）

## 行为意图
- （待补充）

## 行为
- （待补充）

## 约束
- （待补充）

## 输入输出
- （待补充）

## 调用链位置

### 本地依赖（下游）
- （待补充）

### 上游引用
- （待补充）

## 状态与副作用
- （待补充）

## 异常处理
- （待补充）

## 与 PRD 需求映射
- （待补充）

## 溯源证据
- source: {}
- 证据时间：（待补充）
- 关键代码定位：（待补充）
"#,
        rel_path, rel_path
    )
}
