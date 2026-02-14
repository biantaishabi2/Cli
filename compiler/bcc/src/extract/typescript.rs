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
    let mut declarations: usize = 0;

    let mut cursor = root.walk();
    extract_ts_recursive(&mut cursor, source, &mut exports, &mut imports, &mut calls, &mut declarations);

    // 去重 calls
    let mut seen = std::collections::HashSet::new();
    calls.retain(|c| seen.insert(c.callee.clone()));

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
    calls: &mut Vec<CallRecord>,
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
            // 模块调用检测: member_expression 如 ModuleName.method()
            "member_expression" => {
                if let Some(obj) = node.child_by_field_name("object") {
                    let obj_text = node_text(obj, source);
                    if obj_text.chars().next().map_or(false, |c| c.is_uppercase()) {
                        calls.push(CallRecord {
                            callee: obj_text,
                            line: node.start_position().row + 1,
                        });
                    }
                }
            }
            _ => {}
        }

        // 递归子节点（跳过已处理的 export_statement 内部）
        if kind != "export_statement" {
            if cursor.goto_first_child() {
                extract_ts_recursive(cursor, source, exports, imports, calls, declarations);
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
