use serde::Serialize;
use std::fs;

#[derive(Debug, Serialize)]
pub struct FileRecord {
    pub language: String,
    pub file_path: String,
    pub module_doc: Option<String>,
    pub exports: Vec<ExportRecord>,
    pub imports: Vec<ImportRecord>,
    pub calls: Vec<CallRecord>,
    pub side_effects: SideEffects,
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

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SideEffects {
    pub has_async: bool,
    pub has_http: bool,
    pub has_genserver: bool,
    pub has_file_io: bool,
    pub has_pubsub: bool,
}

pub fn run(path: &str, mode: &str) {
    let content = match fs::read_to_string(path) {
        Ok(c) => c,
        Err(e) => {
            eprintln!("cannot read '{}': {}", path, e);
            std::process::exit(1);
        }
    };

    let lang = detect_language(path);
    let record = match lang.as_str() {
        "elixir" => extract_elixir(&content, path),
        "typescript" => extract_typescript(&content, path),
        other => {
            eprintln!("unsupported language: {}", other);
            std::process::exit(1);
        }
    };

    match mode {
        "ast" => {
            let json = serde_json::to_string_pretty(&record).expect("JSON serialization failed");
            println!("{}", json);
        }
        other => {
            eprintln!("unsupported mode: {}", other);
            std::process::exit(1);
        }
    }
}

fn detect_language(path: &str) -> String {
    if path.ends_with(".ex") || path.ends_with(".exs") {
        "elixir".into()
    } else if path.ends_with(".ts") || path.ends_with(".tsx") {
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

fn extract_elixir(content: &str, path: &str) -> FileRecord {
    let mut exports = Vec::new();
    let mut imports = Vec::new();
    let mut calls = Vec::new();
    let mut module_doc = None;
    let mut side_effects = SideEffects {
        has_async: false,
        has_http: false,
        has_genserver: false,
        has_file_io: false,
        has_pubsub: false,
    };

    // 当前正在收集的 @spec（关联到下一个 def）
    let mut pending_spec: Option<String> = None;

    // 收集 @moduledoc（简化：取三引号之间的内容）
    let mut in_moduledoc = false;
    let mut moduledoc_buf = String::new();

    for (line_num, line) in content.lines().enumerate() {
        let trimmed = line.trim();

        // @moduledoc 收集
        if trimmed.starts_with("@moduledoc") {
            if trimmed.contains("\"\"\"") {
                in_moduledoc = true;
                moduledoc_buf.clear();
                continue;
            }
        }
        if in_moduledoc {
            if trimmed == "\"\"\"" {
                module_doc = Some(moduledoc_buf.trim().to_string());
                in_moduledoc = false;
                continue;
            }
            moduledoc_buf.push_str(trimmed);
            moduledoc_buf.push('\n');
            continue;
        }

        // @spec 收集
        if trimmed.starts_with("@spec ") {
            pending_spec = Some(trimmed[6..].to_string());
            continue;
        }

        // def（公开函数）
        if trimmed.starts_with("def ") && !trimmed.starts_with("defp ") && !trimmed.starts_with("defmodule") {
            if let Some(name) = extract_fn_name(trimmed, "def ") {
                exports.push(ExportRecord {
                    name,
                    kind: "function".into(),
                    signature: pending_spec.take(),
                    line: line_num + 1,
                });
            }
            pending_spec = None;
            continue;
        }

        // defp 不导出，只清除 pending_spec
        if trimmed.starts_with("defp ") {
            pending_spec = None;
            continue;
        }

        // alias
        if trimmed.starts_with("alias ") {
            let module = trimmed[6..].trim_end_matches(|c: char| !c.is_alphanumeric() && c != '.');
            imports.push(ImportRecord {
                specifier: module.to_string(),
                kind: "alias".into(),
            });
        }

        // 模块调用检测 Module.func(
        for word in trimmed.split(|c: char| !c.is_alphanumeric() && c != '.' && c != '_') {
            if word.contains('.') {
                let parts: Vec<&str> = word.split('.').collect();
                if parts.len() >= 2 && parts[0].chars().next().map_or(false, |c| c.is_uppercase()) {
                    let callee = parts[0].to_string();
                    // 避免重复
                    if !calls.iter().any(|c: &CallRecord| c.callee == callee) {
                        calls.push(CallRecord {
                            callee,
                            line: line_num + 1,
                        });
                    }
                }
            }
        }

        // 副作用检测
        if trimmed.contains("Task.async") || trimmed.contains("Task.start") {
            side_effects.has_async = true;
        }
        if trimmed.contains("HTTPoison") || trimmed.contains("Req.") || trimmed.contains("Tesla.") {
            side_effects.has_http = true;
        }
        if trimmed.contains("GenServer.") {
            side_effects.has_genserver = true;
        }
        if trimmed.contains("File.") || trimmed.contains("IO.") {
            side_effects.has_file_io = true;
        }
        if trimmed.contains("PubSub.") {
            side_effects.has_pubsub = true;
        }
    }

    FileRecord {
        language: "elixir".into(),
        file_path: path.to_string(),
        module_doc,
        exports,
        imports,
        calls,
        side_effects,
    }
}

fn extract_typescript(content: &str, path: &str) -> FileRecord {
    let mut exports = Vec::new();
    let mut imports = Vec::new();
    let mut calls = Vec::new();
    let side_effects = SideEffects {
        has_async: content.contains("async ") || content.contains("await "),
        has_http: content.contains("fetch(") || content.contains("axios"),
        has_genserver: false,
        has_file_io: content.contains("fs.") || content.contains("readFile") || content.contains("writeFile"),
        has_pubsub: content.contains("EventEmitter") || content.contains(".emit("),
    };

    for (line_num, line) in content.lines().enumerate() {
        let trimmed = line.trim();

        // export function
        if trimmed.starts_with("export function ") || trimmed.starts_with("export async function ") {
            let name_start = if trimmed.starts_with("export async") {
                "export async function ".len()
            } else {
                "export function ".len()
            };
            if let Some(name) = extract_until_paren(&trimmed[name_start..]) {
                exports.push(ExportRecord {
                    name,
                    kind: "function".into(),
                    signature: None,
                    line: line_num + 1,
                });
            }
        }

        // export class
        if trimmed.starts_with("export class ") {
            if let Some(name) = extract_until_space(&trimmed["export class ".len()..]) {
                exports.push(ExportRecord {
                    name,
                    kind: "class".into(),
                    signature: None,
                    line: line_num + 1,
                });
            }
        }

        // export const
        if trimmed.starts_with("export const ") {
            if let Some(name) = extract_until_space_or_eq(&trimmed["export const ".len()..]) {
                exports.push(ExportRecord {
                    name,
                    kind: "const".into(),
                    signature: None,
                    line: line_num + 1,
                });
            }
        }

        // import from
        if trimmed.starts_with("import ") {
            if let Some(spec) = extract_import_specifier(trimmed) {
                imports.push(ImportRecord {
                    specifier: spec,
                    kind: "import".into(),
                });
            }
        }
    }

    FileRecord {
        language: "typescript".into(),
        file_path: path.to_string(),
        module_doc: None,
        exports,
        imports,
        calls,
        side_effects,
    }
}

fn extract_fn_name(line: &str, prefix: &str) -> Option<String> {
    let after = line.strip_prefix(prefix)?;
    let name: String = after.chars().take_while(|c| c.is_alphanumeric() || *c == '_').collect();
    if name.is_empty() { None } else { Some(name) }
}

fn extract_until_paren(s: &str) -> Option<String> {
    let name: String = s.chars().take_while(|c| *c != '(').collect();
    let name = name.trim().to_string();
    if name.is_empty() { None } else { Some(name) }
}

fn extract_until_space(s: &str) -> Option<String> {
    let name: String = s.chars().take_while(|c| !c.is_whitespace() && *c != '{').collect();
    if name.is_empty() { None } else { Some(name) }
}

fn extract_until_space_or_eq(s: &str) -> Option<String> {
    let name: String = s.chars().take_while(|c| !c.is_whitespace() && *c != '=' && *c != ':').collect();
    if name.is_empty() { None } else { Some(name) }
}

fn extract_import_specifier(line: &str) -> Option<String> {
    // import { x } from './path'
    // import x from 'path'
    let from_idx = line.find(" from ")?;
    let after_from = &line[from_idx + 6..];
    let spec: String = after_from.trim().trim_matches(|c| c == '\'' || c == '"' || c == ';').to_string();
    if spec.is_empty() { None } else { Some(spec) }
}
