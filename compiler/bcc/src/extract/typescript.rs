use std::collections::{HashMap, HashSet};
use std::path::PathBuf;
use std::path::Path;
use std::env;
use tree_sitter::Parser;
use super::*;

/// 使用 tree-sitter-typescript 解析源码并提取结构信息
pub fn extract(content: &str, path: &str, lang: &str) -> FileRecord {
    let mut parser = Parser::new();

    let language = if lang == "tsx" {
        tree_sitter_typescript::LANGUAGE_TSX
    } else {
        tree_sitter_typescript::LANGUAGE_TYPESCRIPT
    };
    parser.set_language(&language.into()).expect("failed to set typescript grammar");

    let tree = parser.parse(content, None).expect("failed to parse typescript source");
    let root = tree.root_node();
    let source = content.as_bytes();

    let mut exports = Vec::new();
    let mut imports = Vec::new();
    let mut calls = Vec::new();
    let mut local_call_symbols: HashSet<String> = HashSet::new();
    let mut import_aliases: HashMap<String, String> = HashMap::new();
    let mut declarations: usize = 0;

    let mut cursor = root.walk();
    extract_ts_recursive(
        &mut cursor,
        source,
        &mut exports,
        &mut imports,
        &mut import_aliases,
        &mut calls,
        &mut local_call_symbols,
        &mut declarations,
    );

    // 去重 calls
    let mut seen = std::collections::HashSet::new();
    calls.retain(|c| seen.insert(c.callee.clone()));
    calls.sort_by(|a, b| a.line.cmp(&b.line).then_with(|| a.callee.cmp(&b.callee)));
    local_call_symbols.clear();
    for call in &calls {
        local_call_symbols.insert(call.callee.clone());
    }

    let mut local_call_targets_set = HashSet::new();
    for symbol in local_call_symbols {
        if let Some(specifier) = import_aliases.get(&symbol) {
            if let Some(target) = resolve_local_import(path, specifier) {
                local_call_targets_set.insert(target);
            }
        }
    }
    let mut local_call_targets: Vec<String> = local_call_targets_set.into_iter().collect();
    local_call_targets.sort_unstable();

    let side_effects = SideEffects {
        has_async: content.contains("async ") || content.contains("await "),
        has_http: content.contains("fetch(") || content.contains("axios") || content.contains("ky("),
        has_genserver: false,
        has_file_io: content.contains("fs.") || content.contains("readFile") || content.contains("writeFile"),
        has_pubsub: content.contains("EventEmitter") || content.contains(".emit("),
    };

    FileRecord {
        language: lang.to_string(),
        file_path: path.to_string(),
        module_doc: None,
        exports,
        imports,
        calls,
        local_call_targets,
        side_effects,
        loc_lines: content.lines().count(),
        declarations,
    }
}

fn extract_ts_recursive(
    cursor: &mut tree_sitter::TreeCursor,
    source: &[u8],
    exports: &mut Vec<ExportRecord>,
    imports: &mut Vec<ImportRecord>,
    import_aliases: &mut HashMap<String, String>,
    calls: &mut Vec<CallRecord>,
    local_call_symbols: &mut HashSet<String>,
    declarations: &mut usize,
) {
    loop {
        let node = cursor.node();
        let kind = node.kind();

        match kind {
            // export_statement 包裹的声明
            "export_statement" => {
                // 遍历子节点找 declaration
                let mut child_cursor = node.walk();
                if child_cursor.goto_first_child() {
                    loop {
                        let child = child_cursor.node();
                        match child.kind() {
                            "function_declaration" => {
                                *declarations += 1;
                                if let Some(name_node) = child.child_by_field_name("name") {
                                    exports.push(ExportRecord {
                                        name: node_text(name_node, source),
                                        kind: "function".into(),
                                        signature: None,
                                        line: child.start_position().row + 1,
                                    });
                                }
                            }
                            "class_declaration" => {
                                *declarations += 1;
                                if let Some(name_node) = child.child_by_field_name("name") {
                                    exports.push(ExportRecord {
                                        name: node_text(name_node, source),
                                        kind: "class".into(),
                                        signature: None,
                                        line: child.start_position().row + 1,
                                    });
                                }
                            }
                            "lexical_declaration" => {
                                // export const ...
                                *declarations += 1;
                                let mut decl_cursor = child.walk();
                                if decl_cursor.goto_first_child() {
                                    loop {
                                        let decl_child = decl_cursor.node();
                                        if decl_child.kind() == "variable_declarator" {
                                            if let Some(name_node) = decl_child.child_by_field_name("name") {
                                                exports.push(ExportRecord {
                                                    name: node_text(name_node, source),
                                                    kind: "const".into(),
                                                    signature: None,
                                                    line: decl_child.start_position().row + 1,
                                                });
                                            }
                                        }
                                        if !decl_cursor.goto_next_sibling() {
                                            break;
                                        }
                                    }
                                }
                            }
                            "interface_declaration" => {
                                *declarations += 1;
                                if let Some(name_node) = child.child_by_field_name("name") {
                                    exports.push(ExportRecord {
                                        name: node_text(name_node, source),
                                        kind: "interface".into(),
                                        signature: None,
                                        line: child.start_position().row + 1,
                                    });
                                }
                            }
                            "type_alias_declaration" => {
                                *declarations += 1;
                                if let Some(name_node) = child.child_by_field_name("name") {
                                    exports.push(ExportRecord {
                                        name: node_text(name_node, source),
                                        kind: "type".into(),
                                        signature: None,
                                        line: child.start_position().row + 1,
                                    });
                                }
                            }
                            "enum_declaration" => {
                                *declarations += 1;
                                if let Some(name_node) = child.child_by_field_name("name") {
                                    exports.push(ExportRecord {
                                        name: node_text(name_node, source),
                                        kind: "enum".into(),
                                        signature: None,
                                        line: child.start_position().row + 1,
                                    });
                                }
                            }
                            _ => {}
                        }
                        if !child_cursor.goto_next_sibling() {
                            break;
                        }
                    }
                }
            }
            // import_statement
            "import_statement" => {
                if let Some(source_node) = node.child_by_field_name("source") {
                    let spec = node_text(source_node, source)
                        .trim_matches(|c: char| c == '\'' || c == '"')
                        .to_string();
                    if !spec.is_empty() {
                        let import_text = node_text(node, source);
                        let aliases = extract_import_aliases(&import_text);
                        for alias in aliases {
                            import_aliases.insert(alias, spec.clone());
                        }
                        imports.push(ImportRecord {
                            specifier: spec,
                            kind: "import".into(),
                        });
                    }
                }
            }
            // 非导出的声明也计入 declarations
            "function_declaration" | "class_declaration" | "interface_declaration"
            | "type_alias_declaration" | "enum_declaration" => {
                // 只有不在 export_statement 内的才独立计数
                if node.parent().map_or(true, |p| p.kind() != "export_statement") {
                    *declarations += 1;
                }
            }
            "lexical_declaration" => {
                if node.parent().map_or(true, |p| p.kind() != "export_statement") {
                    *declarations += 1;
                }
            }
            // 模块调用检测: 以 call_expression 根调用符作为本地调用候选
            "call_expression" => {
                if let Some(func_node) = node.child_by_field_name("function") {
                    if let Some(callee) = extract_call_root(func_node, source) {
                        local_call_symbols.insert(callee.clone());
                        calls.push(CallRecord {
                            callee,
                            line: node.start_position().row + 1,
                        });
                    }
                }
                if !cursor.goto_next_sibling() {
                    break;
                }
                continue;
            }
            _ => {}
        }

        // 递归子节点（跳过已处理的 export_statement 内部）
        if kind != "export_statement" {
            if cursor.goto_first_child() {
                extract_ts_recursive(
                    cursor,
                    source,
                    exports,
                    imports,
                    import_aliases,
                    calls,
                    local_call_symbols,
                    declarations,
                );
                cursor.goto_parent();
            }
        }

        if !cursor.goto_next_sibling() {
            break;
        }
    }
}

fn node_text(node: tree_sitter::Node, source: &[u8]) -> String {
    node.utf8_text(source).unwrap_or("").to_string()
}

fn to_posix_path(path: &Path) -> String {
    path.to_string_lossy().replace('\\', "/")
}

fn resolve_local_import(source_path: &str, specifier: &str) -> Option<String> {
    if !specifier.starts_with('.') {
        return None;
    }

    let cwd = env::current_dir().ok()?;
    let source_file = Path::new(source_path);
    let source_dir = source_file.parent()?;
    let normalized_spec = if specifier.ends_with(".ts") || specifier.ends_with(".tsx") || specifier.ends_with(".cts") || specifier.ends_with(".mts") {
        specifier
            .trim_end_matches(".ts")
            .trim_end_matches(".tsx")
            .trim_end_matches(".cts")
            .trim_end_matches(".mts")
            .to_string()
    } else {
        specifier.to_string()
    };

    let base = source_dir.join(normalized_spec);
    let candidates = [
        PathBuf::from(&base),
        PathBuf::from(format!("{}{}", base.to_string_lossy(), ".ts")),
        PathBuf::from(format!("{}{}", base.to_string_lossy(), ".tsx")),
        PathBuf::from(format!("{}{}", base.to_string_lossy(), ".mts")),
        PathBuf::from(format!("{}{}", base.to_string_lossy(), ".cts")),
        base.join("index.ts"),
        base.join("index.tsx"),
        base.join("index.mts"),
        base.join("index.cts"),
    ];

    for candidate in candidates.iter() {
        if !candidate.exists() {
            continue;
        }

        let absolute = candidate.canonicalize().unwrap_or_else(|_| candidate.to_path_buf());
        let rel = absolute.strip_prefix(&cwd).ok()?;
        let rel_str = to_posix_path(rel);
        if rel_str.starts_with("src/") {
            return Some(rel_str);
        }
    }
    None
}

fn extract_import_aliases(import_text: &str) -> Vec<String> {
    let mut aliases = Vec::new();
    let text = import_text.trim();
    if !text.starts_with("import") {
        return aliases;
    }
    let (binding_part, _) = match text.split_once("from") {
        Some((left, right)) => (left.trim(), right.trim()),
        None => return aliases,
    };
    let import_bindings = binding_part.strip_prefix("import").unwrap_or("").trim();
    if import_bindings.is_empty() {
        return aliases;
    }

    for segment in split_top_level_commas(import_bindings) {
        let seg = segment.trim();
        if seg.is_empty() {
            continue;
        }
        if seg.starts_with("{") {
            if seg.contains('}') {
                let list = seg.trim_start_matches('{').trim_end_matches('}');
                for item in list.split(',') {
                    let name = extract_named_binding_alias(item.trim());
                    if let Some(a) = name {
                        aliases.push(a);
                    }
                }
            }
            continue;
        }

        if seg.starts_with("* as ") {
            if let Some(alias) = seg.split_whitespace().nth(2) {
                aliases.push(alias.to_string());
            }
            continue;
        }

        if seg.contains(" as ") {
            if let Some(alias) = seg.split(" as ").last() {
                aliases.push(alias.trim().to_string());
            }
            continue;
        }

        let alias = seg.split_whitespace().next().unwrap_or("").to_string();
        if !alias.is_empty() && alias != "{" && alias != "}" {
            aliases.push(alias);
        }
    }
    aliases
}

fn split_top_level_commas(input: &str) -> Vec<&str> {
    let mut items = Vec::new();
    let mut start = 0usize;
    let mut depth = 0i32;
    let chars: Vec<char> = input.chars().collect();

    for (i, ch) in chars.iter().enumerate() {
        match ch {
            '{' => depth += 1,
            '}' => depth = depth.saturating_sub(1),
            ',' if depth == 0 => {
                items.push(input[start..i].trim());
                start = i + 1;
            }
            _ => {}
        }
    }
    if start <= input.len() {
        items.push(input[start..].trim());
    }
    items
}

fn extract_named_binding_alias(item: &str) -> Option<String> {
    if item.is_empty() {
        return None;
    }
    if item.contains(" as ") {
        item.split(" as ").nth(1).map(|a| a.trim().to_string())
    } else {
        Some(item.to_string())
    }
}

fn extract_call_root(node: tree_sitter::Node, source: &[u8]) -> Option<String> {
    match node.kind() {
        "identifier" => Some(node_text(node, source)),
        "member_expression" => {
            let mut current = node;
            loop {
                match current.child_by_field_name("object") {
                    Some(obj) if obj.kind() == "identifier" => return Some(node_text(obj, source)),
                    Some(obj) if obj.kind() == "member_expression" => current = obj,
                    Some(_) => return None,
                    None => return None,
                }
            }
        }
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use std::env;

    #[test]
    fn extracts_direct_and_member_calls() {
        let source = r#"
import { formatName } from './util';
import * as Helper from './helper';

function run() {
  formatName('x');
  Helper.log('y');
  return Foo.bar.baz();
}
"#;

        let record = extract(source, "src/sample.ts", "typescript");
        let calls: Vec<_> = record.calls.into_iter().map(|c| c.callee).collect();

        assert_eq!(calls, vec!["formatName".to_string(), "Helper".to_string(), "Foo".to_string()]);
    }

    #[test]
    fn derives_local_call_targets_from_import_aliases() {
        let cwd = env::current_dir().expect("cwd");
        let fixture_dir = cwd.join("src/tmp-ts-call-targets-test");
        let src_file = fixture_dir.join("main.ts");
        let util_file = fixture_dir.join("util.ts");

        fs::create_dir_all(&fixture_dir).expect("create fixture dir");
        fs::write(&util_file, "export function formatName() { return 'x'; }\n").expect("write util");

        let source = r#"
import { formatName } from './util';

function run() {
  return formatName();
}
"#;
        fs::write(&src_file, source).expect("write src");

        let record = extract(
            source,
            src_file.to_string_lossy().as_ref(),
            "typescript",
        );
        assert_eq!(
            record.local_call_targets,
            vec![String::from("src/tmp-ts-call-targets-test/util.ts")]
        );

        fs::remove_file(&src_file).expect("remove src");
        fs::remove_file(&util_file).expect("remove util");
        fs::remove_dir_all(&fixture_dir).expect("remove fixture dir");
    }

    #[test]
    fn ignores_non_call_expression_nodes() {
        let source = "const n = 1; const s = value ? 'x' : 'y';";
        let record = extract(source, "src/sample.ts", "typescript");

        assert!(record.calls.is_empty());
    }
}
