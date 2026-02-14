use tree_sitter::Parser;
use super::*;

/// 使用 tree-sitter-elixir 解析源码并提取结构信息
pub fn extract(content: &str, path: &str) -> FileRecord {
    let mut parser = Parser::new();
    let language = tree_sitter_elixir::LANGUAGE;
    parser.set_language(&language.into()).expect("failed to set elixir grammar");

    let tree = parser.parse(content, None).expect("failed to parse elixir source");
    let root = tree.root_node();
    let source = content.as_bytes();

    let mut exports = Vec::new();
    let mut imports = Vec::new();
    let mut calls = Vec::new();
    let mut module_doc = None;
    let mut declarations: usize = 0;
    let mut side_effects = SideEffects {
        has_async: false,
        has_http: false,
        has_genserver: false,
        has_file_io: false,
        has_pubsub: false,
    };
    let mut pending_spec: Option<String> = None;

    let mut cursor = root.walk();
    extract_recursive(&mut cursor, source, &mut exports, &mut imports, &mut calls,
                      &mut module_doc, &mut declarations, &mut side_effects, &mut pending_spec);

    detect_side_effects(content, &mut side_effects);

    // 去重 calls
    let mut seen = std::collections::HashSet::new();
    calls.retain(|c| seen.insert(c.callee.clone()));

    FileRecord {
        language: "elixir".into(),
        file_path: path.to_string(),
        module_doc,
        exports,
        imports,
        calls,
        side_effects,
        loc_lines: content.lines().count(),
        declarations,
    }
}

/// 在 tree-sitter-elixir 中，call 节点的子节点没有 named field "arguments"，
/// 但有一个 kind="arguments" 的子节点。此函数按 kind 查找。
fn find_child_by_kind<'a>(node: &tree_sitter::Node<'a>, target_kind: &str) -> Option<tree_sitter::Node<'a>> {
    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            if child.kind() == target_kind {
                return Some(child);
            }
        }
    }
    None
}


fn extract_recursive(
    cursor: &mut tree_sitter::TreeCursor,
    source: &[u8],
    exports: &mut Vec<ExportRecord>,
    imports: &mut Vec<ImportRecord>,
    calls: &mut Vec<CallRecord>,
    module_doc: &mut Option<String>,
    declarations: &mut usize,
    side_effects: &mut SideEffects,
    pending_spec: &mut Option<String>,
) {
    loop {
        let node = cursor.node();
        let kind = node.kind();

        match kind {
            // @moduledoc / @spec 等模块属性
            "unary_operator" => {
                if let Some(op) = node.child_by_field_name("operator") {
                    if node_text(op, source) == "@" {
                        if let Some(operand) = node.child_by_field_name("operand") {
                            handle_module_attribute(operand, source, module_doc, pending_spec);
                        }
                    }
                }
            }
            // call: def/defp/defmodule/alias/import/use
            "call" => {
                if let Some(target) = node.child_by_field_name("target") {
                    let target_text = node_text(target, source);
                    match target_text.as_str() {
                        "def" => {
                            *declarations += 1;
                            if let Some((name, line)) = extract_def_name(&node, source) {
                                exports.push(ExportRecord {
                                    name,
                                    kind: "function".into(),
                                    signature: pending_spec.take(),
                                    line,
                                });
                            }
                            *pending_spec = None;
                        }
                        "defp" => {
                            *declarations += 1;
                            *pending_spec = None;
                        }
                        "defmacro" | "defmacrop" | "defguard" | "defguardp" | "defdelegate" => {
                            *declarations += 1;
                            // defmacro/defguard 不计入 exports（设计约定）
                            *pending_spec = None;
                        }
                        "defmodule" => {
                            *declarations += 1;
                        }
                        "alias" => {
                            if let Some(args) = find_child_by_kind(&node, "arguments") {
                                let spec = node_text(args, source)
                                    .trim_matches(|c: char| c == '(' || c == ')')
                                    .to_string();
                                if !spec.is_empty() {
                                    imports.push(ImportRecord {
                                        specifier: spec,
                                        kind: "alias".into(),
                                    });
                                }
                            }
                        }
                        "import" => {
                            if let Some(args) = find_child_by_kind(&node, "arguments") {
                                let spec = node_text(args, source)
                                    .trim_matches(|c: char| c == '(' || c == ')')
                                    .to_string();
                                if !spec.is_empty() {
                                    imports.push(ImportRecord {
                                        specifier: spec,
                                        kind: "import".into(),
                                    });
                                }
                            }
                        }
                        "use" => {
                            if let Some(args) = find_child_by_kind(&node, "arguments") {
                                let spec = node_text(args, source)
                                    .trim_matches(|c: char| c == '(' || c == ')')
                                    .to_string();
                                if !spec.is_empty() {
                                    imports.push(ImportRecord {
                                        specifier: spec,
                                        kind: "use".into(),
                                    });
                                }
                            }
                        }
                        _ => {}
                    }
                }
            }
            // dot 调用: Module.func(...)
            "dot" => {
                if let Some(left) = node.child_by_field_name("left") {
                    let module = node_text(left, source);
                    if !module.is_empty() && module.chars().next().map_or(false, |c| c.is_uppercase()) {
                        calls.push(CallRecord {
                            callee: module,
                            line: node.start_position().row + 1,
                        });
                    }
                }
            }
            _ => {}
        }

        // 递归子节点
        if cursor.goto_first_child() {
            extract_recursive(cursor, source, exports, imports, calls,
                            module_doc, declarations, side_effects, pending_spec);
            cursor.goto_parent();
        }

        if !cursor.goto_next_sibling() {
            break;
        }
    }
}

/// 处理 @moduledoc / @spec 等模块属性
fn handle_module_attribute(
    operand: tree_sitter::Node,
    source: &[u8],
    module_doc: &mut Option<String>,
    pending_spec: &mut Option<String>,
) {
    if operand.kind() != "call" {
        return;
    }

    if let Some(target) = operand.child_by_field_name("target") {
        let attr_name = node_text(target, source);
        match attr_name.as_str() {
            "moduledoc" => {
                // 用 find_child_by_kind 替代 child_by_field_name
                if let Some(args) = find_child_by_kind(&operand, "arguments") {
                    let doc_text = extract_string_content(args, source);
                    if !doc_text.is_empty() {
                        *module_doc = Some(doc_text);
                    }
                }
            }
            "spec" => {
                if let Some(args) = find_child_by_kind(&operand, "arguments") {
                    let spec_text = node_text(args, source)
                        .trim_matches(|c: char| c == '(' || c == ')')
                        .to_string();
                    *pending_spec = Some(spec_text);
                }
            }
            _ => {}
        }
    }
}

/// 从 def/defp 调用中提取函数名和行号
fn extract_def_name(call_node: &tree_sitter::Node, source: &[u8]) -> Option<(String, usize)> {
    // tree-sitter-elixir 中 def 的结构:
    // call[target=identifier("def"), arguments(...), do_block(...)]
    // arguments 不是 named field，需要按 kind 查找

    // 遍历子节点，跳过 target，找 arguments 或直接的 call/identifier
    for i in 0..call_node.child_count() {
        if let Some(child) = call_node.child(i) {
            let field = call_node.field_name_for_child(i as u32).unwrap_or("");
            if field == "target" {
                continue;
            }
            match child.kind() {
                "arguments" => {
                    // arguments 内含函数签名 call 节点
                    for j in 0..child.child_count() {
                        if let Some(sub) = child.child(j) {
                            match sub.kind() {
                                "call" => {
                                    if let Some(t) = sub.child_by_field_name("target") {
                                        return Some((node_text(t, source), sub.start_position().row + 1));
                                    }
                                }
                                "identifier" => {
                                    return Some((node_text(sub, source), sub.start_position().row + 1));
                                }
                                _ => {}
                            }
                        }
                    }
                }
                "call" => {
                    if let Some(t) = child.child_by_field_name("target") {
                        return Some((node_text(t, source), child.start_position().row + 1));
                    }
                }
                "identifier" => {
                    return Some((node_text(child, source), child.start_position().row + 1));
                }
                _ => {}
            }
        }
    }
    None
}

/// 提取字符串节点内容（去掉引号/三引号）
fn extract_string_content(node: tree_sitter::Node, source: &[u8]) -> String {
    // 优先遍历子节点找 string 类型
    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            if child.kind() == "string" {
                // string 节点的子节点: quoted_start, quoted_content, quoted_end
                if let Some(content) = find_child_by_kind(&child, "quoted_content") {
                    let text = node_text(content, source);
                    if !text.trim().is_empty() {
                        return text.trim().to_string();
                    }
                }
                // 如果 quoted_content 为空，可能是单行字符串
                let text = node_text(child, source);
                let text = text.trim();
                if text.starts_with("\"\"\"") && text.ends_with("\"\"\"") {
                    return text[3..text.len()-3].trim().to_string();
                } else if text.starts_with('"') && text.ends_with('"') {
                    return text[1..text.len()-1].to_string();
                }
            }
        }
    }
    // 回退：直接用节点文本
    let text = node_text(node, source);
    let text = text.trim().trim_matches(|c: char| c == '(' || c == ')').trim();
    if text.starts_with("\"\"\"") && text.ends_with("\"\"\"") {
        text[3..text.len()-3].trim().to_string()
    } else if text.starts_with('"') && text.ends_with('"') {
        text[1..text.len()-1].to_string()
    } else {
        text.to_string()
    }
}

fn node_text(node: tree_sitter::Node, source: &[u8]) -> String {
    node.utf8_text(source).unwrap_or("").to_string()
}

/// 基于全文关键词扫描的副作用分类标签（行为检测的分类维度）
/// 独立于 tree-sitter 提取，用 contains() 捕获 use GenServer 等非 dot-call 模式
fn detect_side_effects(content: &str, se: &mut SideEffects) {
    if content.contains("Task.async") || content.contains("Task.start") {
        se.has_async = true;
    }
    if content.contains("HTTPoison") || content.contains("Req.") || content.contains("Tesla.") || content.contains("Finch.") {
        se.has_http = true;
    }
    if content.contains("GenServer.") || content.contains("use GenServer") {
        se.has_genserver = true;
    }
    if content.contains("File.") || content.contains("IO.") {
        se.has_file_io = true;
    }
    if content.contains("PubSub.") || content.contains("Phoenix.PubSub") {
        se.has_pubsub = true;
    }
}
