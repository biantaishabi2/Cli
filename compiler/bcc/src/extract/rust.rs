use super::common;
use super::*;

/// 使用 tree-sitter-rust 解析源码并提取结构信息
pub fn extract(content: &str, path: &str) -> FileRecord {
    let tree = common::parse_tree(content, tree_sitter_rust::LANGUAGE, "rust");
    let root = tree.root_node();
    let source = content.as_bytes();

    let mut exports = Vec::new();
    let mut imports = Vec::new();
    let mut calls = Vec::new();
    let mut declarations: usize = 0;

    let mut cursor = root.walk();
    extract_recursive(
        &mut cursor,
        source,
        &mut exports,
        &mut imports,
        &mut calls,
        &mut declarations,
    );

    common::dedup_sort_calls(&mut calls);

    // 副作用检测（关键词扫描）
    let side_effects = detect_side_effects(content);

    // 提取模块文档（首行 //! 或 /*! 注释）
    let module_doc = extract_module_doc(content);

    // 提取 TypeAnnotation 和 TypeGuard
    let mut type_annotations = Vec::new();
    let mut type_guards = Vec::new();
    extract_rust_type_info(root, source, &mut type_annotations, &mut type_guards);

    // 提取 struct 字段 → SchemaField
    let mut schema_fields = Vec::new();
    extract_rust_struct_fields(root, source, &mut schema_fields);

    FileRecord {
        language: "rust".into(),
        file_path: path.to_string(),
        module_doc,
        exports,
        imports,
        calls,
        local_call_targets: Vec::new(),
        relation_hints: Vec::new(),
        side_effects,
        loc_lines: content.lines().count(),
        declarations,
        type_annotations,
        type_guards,
        schema_fields,
    }
}

fn extract_recursive(
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
            // pub fn / fn
            "function_item" => {
                *declarations += 1;
                let is_pub = has_visibility(&node);
                if let Some(name_node) = node.child_by_field_name("name") {
                    let name = common::node_text(name_node, source);
                    let signature = build_fn_signature(&node, source);
                    if is_pub {
                        exports.push(ExportRecord {
                            name,
                            kind: "function".into(),
                            signature: Some(signature),
                            line: node.start_position().row + 1,
                        });
                    }
                }
            }
            // pub struct
            "struct_item" => {
                *declarations += 1;
                let is_pub = has_visibility(&node);
                if is_pub {
                    if let Some(name_node) = node.child_by_field_name("name") {
                        exports.push(ExportRecord {
                            name: common::node_text(name_node, source),
                            kind: "struct".into(),
                            signature: None,
                            line: node.start_position().row + 1,
                        });
                    }
                }
            }
            // pub enum
            "enum_item" => {
                *declarations += 1;
                let is_pub = has_visibility(&node);
                if is_pub {
                    if let Some(name_node) = node.child_by_field_name("name") {
                        exports.push(ExportRecord {
                            name: common::node_text(name_node, source),
                            kind: "enum".into(),
                            signature: None,
                            line: node.start_position().row + 1,
                        });
                    }
                }
            }
            // pub trait
            "trait_item" => {
                *declarations += 1;
                let is_pub = has_visibility(&node);
                if is_pub {
                    if let Some(name_node) = node.child_by_field_name("name") {
                        exports.push(ExportRecord {
                            name: common::node_text(name_node, source),
                            kind: "trait".into(),
                            signature: None,
                            line: node.start_position().row + 1,
                        });
                    }
                }
            }
            // impl Foo / impl Trait for Foo
            "impl_item" => {
                *declarations += 1;
                let impl_text = common::node_text(node, source);
                // 取 impl 行的摘要（到 { 为止）
                let summary = impl_text
                    .lines()
                    .next()
                    .unwrap_or("")
                    .trim_end_matches('{')
                    .trim()
                    .to_string();
                exports.push(ExportRecord {
                    name: summary.clone(),
                    kind: "impl".into(),
                    signature: Some(summary),
                    line: node.start_position().row + 1,
                });
                // 递归进入 impl 块提取内部方法
                if cursor.goto_first_child() {
                    extract_recursive(cursor, source, exports, imports, calls, declarations);
                    cursor.goto_parent();
                }
                if !cursor.goto_next_sibling() {
                    break;
                }
                continue;
            }
            // use 声明
            "use_declaration" => {
                let use_text = common::node_text(node, source);
                // 去掉 "use " 前缀和尾部 ";"
                let specifier = use_text
                    .strip_prefix("use ")
                    .unwrap_or(&use_text)
                    .trim_end_matches(';')
                    .trim()
                    .to_string();
                if !specifier.is_empty() {
                    imports.push(ImportRecord {
                        specifier,
                        kind: "use".into(),
                    });
                }
            }
            // 函数调用: foo(), module::func(), obj.method()
            "call_expression" => {
                if let Some(func_node) = node.child_by_field_name("function") {
                    let callee = extract_call_name(func_node, source);
                    if !callee.is_empty() {
                        calls.push(CallRecord {
                            callee,
                            line: node.start_position().row + 1,
                            args: vec![],
                        });
                    }
                }
            }
            // 宏调用: println!(), vec![], format!()
            "macro_invocation" => {
                if let Some(macro_node) = node.child(0) {
                    let macro_name = common::node_text(macro_node, source);
                    if !macro_name.is_empty() {
                        calls.push(CallRecord {
                            callee: format!("{}!", macro_name),
                            line: node.start_position().row + 1,
                            args: vec![],
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

        // 递归子节点（impl_item 已在分支内手动处理，macro_invocation 直接跳过）
        if kind != "impl_item" {
            if cursor.goto_first_child() {
                extract_recursive(cursor, source, exports, imports, calls, declarations);
                cursor.goto_parent();
            }
        }

        if !cursor.goto_next_sibling() {
            break;
        }
    }
}

/// 检查节点是否有 pub 可见性修饰符
fn has_visibility(node: &tree_sitter::Node) -> bool {
    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            if child.kind() == "visibility_modifier" {
                return true;
            }
        }
    }
    false
}

/// 从 function_item 构建函数签名：fn foo(a: i32, b: &str) -> Result<T, E>
fn build_fn_signature(node: &tree_sitter::Node, source: &[u8]) -> String {
    let name = node
        .child_by_field_name("name")
        .map(|n| common::node_text(n, source))
        .unwrap_or_default();

    let params = node
        .child_by_field_name("parameters")
        .map(|n| common::node_text(n, source))
        .unwrap_or_else(|| "()".into());

    let ret = node
        .child_by_field_name("return_type")
        .map(|n| {
            let text = common::node_text(n, source);
            // return_type 节点包含 "-> Type"
            if text.starts_with("->") {
                format!(" {}", text.trim())
            } else {
                format!(" -> {}", text.trim())
            }
        })
        .unwrap_or_default();

    format!("fn {}{}{}", name, params, ret)
}

/// 提取调用名：identifier → "foo", scoped_identifier → "module::func",
/// field_expression → "method"（稳定方法名）
fn extract_call_name(node: tree_sitter::Node, source: &[u8]) -> String {
    let raw_node = extract_raw_call_name(node);
    raw_node
        .and_then(|n| normalize_call_name(n, source))
        .unwrap_or_default()
}

/// 先抽取可规范化的 callee 节点，避免直接回退为表达式文本。
fn extract_raw_call_name(node: tree_sitter::Node) -> Option<tree_sitter::Node> {
    match node.kind() {
        // generic_function 包裹真实函数节点（如 foo::<T>()）
        "generic_function" => node.child_by_field_name("function"),
        _ => Some(node),
    }
}

/// 规范化调用名为稳定标识符：
/// identifier/scoped_identifier 原样，field_expression 取 field（方法名）。
fn normalize_call_name(node: tree_sitter::Node, source: &[u8]) -> Option<String> {
    let name = match node.kind() {
        "identifier" | "scoped_identifier" => common::node_text(node, source),
        "field_expression" => node
            .child_by_field_name("field")
            .map(|field| common::node_text(field, source))
            .unwrap_or_default(),
        _ => String::new(),
    };
    if name.is_empty() {
        None
    } else {
        Some(name)
    }
}

/// 提取 //! 模块文档注释
fn extract_module_doc(content: &str) -> Option<String> {
    let mut doc_lines = Vec::new();
    for line in content.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with("//!") {
            doc_lines.push(trimmed.trim_start_matches("//!").trim());
        } else if trimmed.is_empty() && doc_lines.is_empty() {
            continue;
        } else if !doc_lines.is_empty() {
            break;
        } else {
            break;
        }
    }
    if doc_lines.is_empty() {
        None
    } else {
        Some(doc_lines.join("\n"))
    }
}

/// 基于全文关键词扫描的副作用分类标签
fn detect_side_effects(content: &str) -> SideEffects {
    SideEffects {
        has_async: content.contains("async ")
            || content.contains(".await")
            || content.contains("tokio::"),
        has_http: content.contains("reqwest")
            || content.contains("hyper::")
            || content.contains("http::"),
        has_genserver: false,
        has_file_io: content.contains("std::fs")
            || content.contains("File::open")
            || content.contains("File::create")
            || content.contains("fs::read")
            || content.contains("fs::write"),
        has_pubsub: content.contains("channel")
            || content.contains("mpsc::")
            || content.contains("broadcast::"),
    }
}

// === SchemaField 提取（struct 字段） ===

/// 从 Rust struct 定义中提取字段 → SchemaField
fn extract_rust_struct_fields(
    root: tree_sitter::Node,
    source: &[u8],
    schema_fields: &mut Vec<SchemaField>,
) {
    let mut stack = vec![root];
    while let Some(node) = stack.pop() {
        if node.kind() == "struct_item" {
            if let Some(body) = node.child_by_field_name("body") {
                for i in 0..body.child_count() {
                    if let Some(field) = body.child(i) {
                        if field.kind() == "field_declaration" {
                            let mut field_name = String::new();
                            let mut field_type = String::new();
                            if let Some(name_node) = field.child_by_field_name("name") {
                                field_name = common::node_text(name_node, source);
                            }
                            if let Some(type_node) = field.child_by_field_name("type") {
                                field_type = common::node_text(type_node, source);
                            }
                            if !field_name.is_empty() {
                                // Option<T> 字段视为 optional，其他为 required
                                let required = !field_type.starts_with("Option");
                                schema_fields.push(SchemaField {
                                    name: field_name,
                                    field_type,
                                    line: field.start_position().row + 1,
                                    required,
                                });
                            }
                        }
                    }
                }
            }
        }
        for i in 0..node.child_count() {
            if let Some(child) = node.child(i) {
                stack.push(child);
            }
        }
    }
}

// === TypeAnnotation / TypeGuard 提取 ===

/// 从 Rust AST 中提取函数参数类型标注和类型守卫模式
fn extract_rust_type_info(
    root: tree_sitter::Node,
    source: &[u8],
    type_annotations: &mut Vec<TypeAnnotation>,
    type_guards: &mut Vec<TypeGuard>,
) {
    let mut stack = vec![root];
    while let Some(node) = stack.pop() {
        if node.kind() == "function_item" {
            let func_name = node
                .child_by_field_name("name")
                .map(|n| common::node_text(n, source))
                .unwrap_or_default();
            // 提取参数类型标注
            if let Some(params) = node.child_by_field_name("parameters") {
                extract_rust_param_annotations(params, source, &func_name, type_annotations);
            }
            // 提取函数体中的类型守卫（if let, matches!）
            if let Some(body) = node.child_by_field_name("body") {
                extract_rust_type_guards(body, source, &func_name, type_guards);
            }
        }
        // 压栈子节点
        let mut idx = node.child_count();
        while idx > 0 {
            idx -= 1;
            if let Some(child) = node.child(idx) {
                // 跳过已处理的函数体（避免嵌套函数重复归属）
                if node.kind() == "function_item" && child.kind() == "block" {
                    continue;
                }
                stack.push(child);
            }
        }
    }
}

/// 提取 Rust 函数参数的类型标注
fn extract_rust_param_annotations(
    params: tree_sitter::Node,
    source: &[u8],
    func_name: &str,
    type_annotations: &mut Vec<TypeAnnotation>,
) {
    for i in 0..params.child_count() {
        let Some(child) = params.child(i) else { continue };
        if child.kind() != "parameter" {
            continue;
        }
        let param_name = child
            .child_by_field_name("pattern")
            .map(|n| common::node_text(n, source))
            .unwrap_or_default();
        if param_name.is_empty() {
            continue;
        }
        if let Some(type_node) = child.child_by_field_name("type") {
            let type_text = common::node_text(type_node, source);
            if !type_text.is_empty() {
                type_annotations.push(TypeAnnotation {
                    name: format!("param:{}", param_name),
                    type_expr: type_text,
                    line: child.start_position().row + 1,
                    func_name: func_name.to_string(),
                    param: param_name,
                });
            }
        }
    }
}

/// 从 Rust 函数体中提取类型守卫模式（if let Some/Err, matches!）
fn extract_rust_type_guards(
    body: tree_sitter::Node,
    source: &[u8],
    func_name: &str,
    type_guards: &mut Vec<TypeGuard>,
) {
    let mut stack = vec![body];
    while let Some(node) = stack.pop() {
        // 跳过嵌套函数
        if node.kind() == "function_item" {
            continue;
        }
        // if let Some(x) = expr / if let Err(e) = expr
        if node.kind() == "if_expression" || node.kind() == "if_let_expression" {
            let node_text = common::node_text(node, source);
            if node_text.starts_with("if let Some(") {
                if let Some(var) = extract_if_let_var(&node_text, "Some") {
                    type_guards.push(TypeGuard {
                        function: func_name.to_string(),
                        guarded_type: "Some".to_string(),
                        line: node.start_position().row + 1,
                        var,
                    });
                }
            } else if node_text.starts_with("if let Err(") {
                if let Some(var) = extract_if_let_var(&node_text, "Err") {
                    type_guards.push(TypeGuard {
                        function: func_name.to_string(),
                        guarded_type: "Err".to_string(),
                        line: node.start_position().row + 1,
                        var,
                    });
                }
            } else if node_text.starts_with("if let Ok(") {
                if let Some(var) = extract_if_let_var(&node_text, "Ok") {
                    type_guards.push(TypeGuard {
                        function: func_name.to_string(),
                        guarded_type: "Ok".to_string(),
                        line: node.start_position().row + 1,
                        var,
                    });
                }
            }
        }
        // downcast_ref::<Type>() 调用
        if node.kind() == "call_expression" {
            let call_text = common::node_text(node, source);
            if let Some(guard) = parse_downcast_ref(&call_text, func_name, node.start_position().row + 1) {
                type_guards.push(guard);
            }
        }
        // matches! 宏
        if node.kind() == "macro_invocation" {
            let macro_text = common::node_text(node, source);
            if macro_text.starts_with("matches!") {
                // matches!(x, Some(_)) → var=x, guarded_type=Some
                if let Some((var, guarded)) = parse_matches_macro(&macro_text) {
                    type_guards.push(TypeGuard {
                        function: func_name.to_string(),
                        guarded_type: guarded,
                        line: node.start_position().row + 1,
                        var,
                    });
                }
            }
        }
        let mut idx = node.child_count();
        while idx > 0 {
            idx -= 1;
            if let Some(child) = node.child(idx) {
                stack.push(child);
            }
        }
    }
}

/// 从 "if let Some(x) = expr" 提取被检查的表达式变量名（expr 而非 x）
fn extract_if_let_var(text: &str, variant: &str) -> Option<String> {
    // "if let Some(x) = expr {" → 提取 "expr"（被检查的变量）
    let pattern = format!("if let {}(", variant);
    let rest = text.strip_prefix(&pattern)?;
    let close = rest.find(')')?;
    let after_close = rest[close + 1..].trim();
    // 跳过 "= "
    let after_eq = after_close.strip_prefix('=')?.trim();
    // 提取变量名（到空格或 { 为止）
    let var = after_eq
        .split(|c: char| c.is_whitespace() || c == '{')
        .next()?
        .trim()
        .to_string();
    if var.is_empty() || var == "_" {
        return None;
    }
    Some(var)
}

/// 从 "matches!(x, Some(_))" 提取变量名和守卫类型
fn parse_matches_macro(text: &str) -> Option<(String, String)> {
    let inner = text.strip_prefix("matches!")?.trim();
    let inner = inner.strip_prefix('(')?.strip_suffix(')')?;
    let comma_pos = inner.find(',')?;
    let var = inner[..comma_pos].trim().to_string();
    let pattern = inner[comma_pos + 1..].trim();
    // 提取模式的第一个标识符（如 Some, Err, Ok）
    let guarded = pattern
        .split(|c: char| !c.is_alphanumeric() && c != '_')
        .next()
        .unwrap_or("")
        .to_string();
    if var.is_empty() || guarded.is_empty() {
        return None;
    }
    Some((var, guarded))
}

/// 解析 expr.downcast_ref::<Type>() 形式的类型守卫
fn parse_downcast_ref(text: &str, func_name: &str, line: usize) -> Option<TypeGuard> {
    // 匹配 var.downcast_ref::<Type>()
    let idx = text.find(".downcast_ref::<")?;
    let var = text[..idx].trim().to_string();
    let rest = &text[idx + ".downcast_ref::<".len()..];
    let end = rest.find('>')?;
    let guarded_type = rest[..end].trim().to_string();
    if var.is_empty() || guarded_type.is_empty() {
        return None;
    }
    Some(TypeGuard {
        function: func_name.to_string(),
        guarded_type,
        line,
        var,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extract::testing;

    #[test]
    fn extracts_pub_functions_and_structs() {
        let source = r#"
use std::collections::HashMap;

pub struct Config {
    name: String,
}

pub fn run(config: &Config) -> Result<(), String> {
    println!("running");
    Ok(())
}

        fn private_helper() {}
"#;
        let record = extract(source, "src/lib.rs");
        let export_names = testing::export_names(&record);
        testing::assert_contains(&export_names, "Config", "exports");
        testing::assert_contains(&export_names, "run", "exports");
        // private_helper 不应出现在 exports
        assert!(!export_names.iter().any(|name| name == "private_helper"));

        // 验证 imports
        assert_eq!(record.imports.len(), 1);
        assert_eq!(record.imports[0].specifier, "std::collections::HashMap");

        // 验证 calls 包含 println! 宏
        let call_names = testing::call_names(&record);
        testing::assert_contains(&call_names, "println!", "calls");

        // 验证签名
        let run_export = record.exports.iter().find(|e| e.name == "run").unwrap();
        assert!(run_export
            .signature
            .as_ref()
            .unwrap()
            .contains("-> Result<(), String>"));
    }

    #[test]
    fn extracts_impl_and_trait() {
        let source = r#"
pub trait Handler {
    fn handle(&self);
}

pub struct Server;

impl Handler for Server {
    fn handle(&self) {
        self.process();
    }
}

impl Server {
    pub fn new() -> Self { Server }
    fn process(&self) {}
}
"#;
        let record = extract(source, "src/server.rs");
        let kinds: Vec<_> = record.exports.iter().map(|e| e.kind.as_str()).collect();
        assert!(kinds.contains(&"trait"), "should export trait");
        assert!(kinds.contains(&"struct"), "should export struct");
        assert!(kinds.contains(&"impl"), "should export impl blocks");
    }

    #[test]
    fn extracts_enum_and_use() {
        let source = r#"
use serde::{Serialize, Deserialize};
use std::fmt;

pub enum Status {
    Active,
    Inactive,
}

impl fmt::Display for Status {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}
"#;
        let record = extract(source, "src/model.rs");
        assert_eq!(record.imports.len(), 2);

        let enum_exports: Vec<_> = record.exports.iter().filter(|e| e.kind == "enum").collect();
        assert_eq!(enum_exports.len(), 1);
        assert_eq!(enum_exports[0].name, "Status");
    }

    #[test]
    fn detects_side_effects() {
        let source = r#"
use tokio::fs;
use reqwest::Client;

pub async fn fetch_data() -> Result<String, Box<dyn std::error::Error>> {
    let client = Client::new();
    let body = client.get("http://example.com").send().await?.text().await?;
    fs::write("output.txt", &body).await?;
    Ok(body)
}
"#;
        let record = extract(source, "src/fetcher.rs");
        assert!(record.side_effects.has_async, "should detect async");
        assert!(record.side_effects.has_http, "should detect http (reqwest)");
        assert!(!record.side_effects.has_genserver, "rust has no genserver");
    }

    #[test]
    fn extracts_module_doc() {
        let source = r#"//! 这个模块负责处理用户认证
//! 支持 JWT 和 session 两种方式

use crate::auth;

pub fn login() {}
"#;
        let record = extract(source, "src/auth.rs");
        assert!(record.module_doc.is_some());
        assert!(record.module_doc.unwrap().contains("用户认证"));
    }

    #[test]
    fn normalizes_index_then_method_call_to_method_name() {
        let source = r#"
pub fn parse(args: Vec<String>) {
    let _ = args[0].as_str();
}
"#;
        let record = extract(source, "src/lib.rs");
        let call_names = testing::call_names(&record);
        testing::assert_contains(&call_names, "as_str", "calls");
        assert!(
            !call_names.iter().any(|name| name == "args[0]"),
            "should not keep expression noise in calls: {:?}",
            call_names
        );
    }

    #[test]
    fn keeps_chain_method_names_without_expression_noise() {
        let source = r#"
pub fn parse(rest: Vec<String>) {
    let _ = rest.get(0).and_then(|x| Some(x));
}
"#;
        let record = extract(source, "src/lib.rs");
        let call_names = testing::call_names(&record);
        testing::assert_contains(&call_names, "get", "calls");
        testing::assert_contains(&call_names, "and_then", "calls");
        assert!(
            !call_names.iter().any(|name| name.contains("rest.get(0)")),
            "should not keep chain expression as callee: {:?}",
            call_names
        );
    }

    #[test]
    fn parse_matches_macro_complex_patterns() {
        // 基本用法
        let result = parse_matches_macro("matches!(x, Some(_))");
        assert_eq!(result, Some(("x".into(), "Some".into())));

        // 嵌套模式：matches!(val, Ok(Some(_)))
        let result = parse_matches_macro("matches!(val, Ok(Some(_)))");
        assert_eq!(result, Some(("val".into(), "Ok".into())));

        // 空变量名
        assert!(parse_matches_macro("matches!(, Some(_))").is_none());

        // 缺少逗号
        assert!(parse_matches_macro("matches!(x Some(_))").is_none());
    }

    #[test]
    fn parse_downcast_ref_edge_cases() {
        // 基本用法
        let result = parse_downcast_ref("e.downcast_ref::<String>()", "handle_err", 10);
        assert!(result.is_some());
        let guard = result.unwrap();
        assert_eq!(guard.var, "e");
        assert_eq!(guard.guarded_type, "String");
        assert_eq!(guard.function, "handle_err");

        // 泛型参数中带路径
        let result = parse_downcast_ref("err.downcast_ref::<std::io::Error>()", "f", 1);
        assert!(result.is_some());
        assert_eq!(result.unwrap().guarded_type, "std::io::Error");

        // 空变量
        let result = parse_downcast_ref(".downcast_ref::<Foo>()", "f", 1);
        assert!(result.is_none());

        // 不包含 downcast_ref
        let result = parse_downcast_ref("e.as_ref()", "f", 1);
        assert!(result.is_none());
    }
}
