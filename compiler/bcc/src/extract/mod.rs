use serde::Serialize;
use std::fs;

pub mod elixir;
pub mod typescript;
pub mod php;

#[derive(Debug, Serialize)]
pub struct FileRecord {
    pub language: String,
    pub file_path: String,
    pub module_doc: Option<String>,
    pub exports: Vec<ExportRecord>,
    pub imports: Vec<ImportRecord>,
    pub calls: Vec<CallRecord>,
    #[serde(rename = "localCallTargets", default)]
    pub local_call_targets: Vec<String>,
    pub side_effects: SideEffects,
    /// 源码总行数
    pub loc_lines: usize,
    /// 声明总数（def/defp/class/function 等）
    pub declarations: usize,
}

#[derive(Debug, Serialize)]
pub struct ExportRecord {
    pub name: String,
    pub kind: String,
    pub signature: Option<String>,
    pub line: usize,
}

#[derive(Debug, Serialize)]
pub struct ImportRecord {
    pub specifier: String,
    pub kind: String,
}

#[derive(Debug, Serialize)]
pub struct CallRecord {
    pub callee: String,
    pub line: usize,
}

/// 副作用分类标签——行为检测的分类维度，标注模块的外部交互类型
/// 通过全文关键词扫描得出，独立于 tree-sitter AST 提取
#[derive(Debug, Serialize)]
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

    let lang = detect_language(path);
    let record = match lang.as_str() {
        "elixir" => elixir::extract(&content, path),
        "typescript" | "tsx" => typescript::extract(&content, path, &lang),
        "php" => php::extract(&content, path),
        other => {
            eprintln!("unsupported language: {}", other);
            std::process::exit(1);
        }
    };

    let result = match mode {
        "ast" => {
            serde_json::to_string_pretty(&record).expect("JSON serialization failed")
        }
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

fn detect_language(path: &str) -> String {
    if path.ends_with(".ex") || path.ends_with(".exs") {
        "elixir".into()
    } else if path.ends_with(".tsx") {
        "tsx".into()
    } else if path.ends_with(".ts") {
        "typescript".into()
    } else if path.ends_with(".go") {
        "go".into()
    } else if path.ends_with(".rs") {
        "rust".into()
    } else if path.ends_with(".php") {
        "php".into()
    } else {
        "unknown".into()
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
            out.push_str(&format!("- `{}` ({}, L{}) — {}\n", ex.name, ex.kind, ex.line, sig));
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
    if se.has_async { effects.push("async"); }
    if se.has_http { effects.push("http"); }
    if se.has_genserver { effects.push("genserver"); }
    if se.has_file_io { effects.push("file_io"); }
    if se.has_pubsub { effects.push("pubsub"); }
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
    let module_name = record.file_path
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
