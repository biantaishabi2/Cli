use super::common;
use super::*;
use regex::Regex;
use std::collections::HashMap;

/// 使用 tree-sitter-elixir 解析源码并提取结构信息
pub fn extract(content: &str, path: &str) -> FileRecord {
    let tree = common::parse_tree(content, tree_sitter_elixir::LANGUAGE, "elixir");
    let root = tree.root_node();
    let source = content.as_bytes();

    let mut exports = Vec::new();
    let mut imports = Vec::new();
    let mut calls = Vec::new();
    let mut module_doc = None;
    let mut declarations: usize = 0;
    let mut side_effects = common::empty_side_effects();
    let mut pending_spec: Option<String> = None;

    let mut cursor = root.walk();
    extract_recursive(
        &mut cursor,
        source,
        &mut exports,
        &mut imports,
        &mut calls,
        &mut module_doc,
        &mut declarations,
        &mut side_effects,
        &mut pending_spec,
    );

    detect_side_effects(content, &mut side_effects);

    common::dedup_calls_by_callee(&mut calls);
    let relation_hints = detect_relation_hints(&imports, content);

    FileRecord {
        language: "elixir".into(),
        file_path: path.to_string(),
        module_doc,
        exports,
        imports,
        calls,
        local_call_targets: Vec::new(),
        relation_hints,
        side_effects,
        loc_lines: content.lines().count(),
        declarations,
    }
}

/// 在 tree-sitter-elixir 中，call 节点的子节点没有 named field "arguments"，
/// 但有一个 kind="arguments" 的子节点。此函数按 kind 查找。
fn find_child_by_kind<'a>(
    node: &tree_sitter::Node<'a>,
    target_kind: &str,
) -> Option<tree_sitter::Node<'a>> {
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
            // @moduledoc / @spec / @behaviour 等模块属性
            "unary_operator" => {
                if let Some(op) = node.child_by_field_name("operator") {
                    if common::node_text(op, source) == "@" {
                        if let Some(operand) = node.child_by_field_name("operand") {
                            handle_module_attribute(
                                operand,
                                source,
                                module_doc,
                                pending_spec,
                                imports,
                            );
                        }
                    }
                }
            }
            // call: def/defp/defmodule/alias/import/use
            "call" => {
                if let Some(target) = node.child_by_field_name("target") {
                    let target_text = common::node_text(target, source);
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
                                let spec = common::node_text(args, source)
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
                                let spec = common::node_text(args, source)
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
                                let spec = common::node_text(args, source)
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
                // alias/import/use/require 声明语境中的 dot 不是运行时调用，需过滤。
                if !is_declaration_dot_context(node, source) {
                    if let Some(left) = node.child_by_field_name("left") {
                        let module = common::node_text(left, source);
                        if !module.is_empty()
                            && module.chars().next().map_or(false, |c| c.is_uppercase())
                        {
                            calls.push(CallRecord {
                                callee: module,
                                line: node.start_position().row + 1,
                            });
                        }
                    }
                }
            }
            _ => {}
        }

        // 字符串类节点不递归子节点，防止文档字符串内的模块名被误识别为依赖
        let skip_children = matches!(kind, "string" | "quoted_content" | "charlist" | "sigil");
        if !skip_children && cursor.goto_first_child() {
            extract_recursive(
                cursor,
                source,
                exports,
                imports,
                calls,
                module_doc,
                declarations,
                side_effects,
                pending_spec,
            );
            cursor.goto_parent();
        }

        if !cursor.goto_next_sibling() {
            break;
        }
    }
}

/// 判断 dot 是否位于 alias/import/use/require 声明语境中。
fn is_declaration_dot_context(node: tree_sitter::Node, source: &[u8]) -> bool {
    let mut current = node;
    while let Some(parent) = current.parent() {
        if parent.kind() == "call" {
            if let Some(target) = parent.child_by_field_name("target") {
                let target_text = common::node_text(target, source);
                if matches!(target_text.as_str(), "alias" | "import" | "use" | "require") {
                    return true;
                }
            }
        }
        current = parent;
    }
    false
}

/// 处理 @moduledoc / @spec / @behaviour 等模块属性
fn handle_module_attribute(
    operand: tree_sitter::Node,
    source: &[u8],
    module_doc: &mut Option<String>,
    pending_spec: &mut Option<String>,
    imports: &mut Vec<ImportRecord>,
) {
    if operand.kind() != "call" {
        return;
    }

    if let Some(target) = operand.child_by_field_name("target") {
        let attr_name = common::node_text(target, source);
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
                    let spec_text = common::node_text(args, source)
                        .trim_matches(|c: char| c == '(' || c == ')')
                        .to_string();
                    *pending_spec = Some(spec_text);
                }
            }
            // @behaviour Module.Name — 记为 import
            "behaviour" | "behavior" => {
                if let Some(args) = find_child_by_kind(&operand, "arguments") {
                    let spec = common::node_text(args, source)
                        .trim_matches(|c: char| c == '(' || c == ')')
                        .trim()
                        .to_string();
                    if !spec.is_empty() {
                        imports.push(ImportRecord {
                            specifier: spec,
                            kind: "behaviour".into(),
                        });
                    }
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
                                        return Some((
                                            common::node_text(t, source),
                                            sub.start_position().row + 1,
                                        ));
                                    }
                                }
                                "identifier" => {
                                    return Some((
                                        common::node_text(sub, source),
                                        sub.start_position().row + 1,
                                    ));
                                }
                                _ => {}
                            }
                        }
                    }
                }
                "call" => {
                    if let Some(t) = child.child_by_field_name("target") {
                        return Some((
                            common::node_text(t, source),
                            child.start_position().row + 1,
                        ));
                    }
                }
                "identifier" => {
                    return Some((
                        common::node_text(child, source),
                        child.start_position().row + 1,
                    ));
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
                    let text = common::node_text(content, source);
                    if !text.trim().is_empty() {
                        return text.trim().to_string();
                    }
                }
                // 如果 quoted_content 为空，可能是单行字符串
                let text = common::node_text(child, source);
                let text = text.trim();
                if text.starts_with("\"\"\"") && text.ends_with("\"\"\"") {
                    return text[3..text.len() - 3].trim().to_string();
                } else if text.starts_with('"') && text.ends_with('"') {
                    return text[1..text.len() - 1].to_string();
                }
            }
        }
    }
    // 回退：直接用节点文本
    let text = common::node_text(node, source);
    let text = text
        .trim()
        .trim_matches(|c: char| c == '(' || c == ')')
        .trim();
    if text.starts_with("\"\"\"") && text.ends_with("\"\"\"") {
        text[3..text.len() - 3].trim().to_string()
    } else if text.starts_with('"') && text.ends_with('"') {
        text[1..text.len() - 1].to_string()
    } else {
        text.to_string()
    }
}

fn detect_relation_hints(imports: &[ImportRecord], content: &str) -> Vec<RelationHintRecord> {
    let mut hints = Vec::new();

    for imp in imports {
        if imp.kind != "use" {
            continue;
        }
        let via_module = extract_use_main_module(&imp.specifier);
        let via = if via_module.is_empty() {
            "use".to_string()
        } else {
            format!("use {}", via_module)
        };
        for target in extract_use_injected_modules(&imp.specifier) {
            hints.push(RelationHintRecord {
                target,
                call_type_hint: "framework_injection".to_string(),
                via: via.clone(),
                confidence: 0.92,
                detector: "elixir.use_macro".to_string(),
                reason: "use 宏参数中的模块注入".to_string(),
            });
        }
    }

    // ExternalLib(.Providers).register(InternalModule) -> external_registration
    // 使用 AST 节点提取，避免注释/字符串中的伪匹配误报。
    let alias_bindings = collect_alias_bindings(imports);
    for (lib_chain, target_module) in detect_external_register_calls(content) {
        if has_register_namespace_conflict(&lib_chain, &target_module, &alias_bindings) {
            continue;
        }
        hints.push(RelationHintRecord {
            target: target_module,
            call_type_hint: "external_registration".to_string(),
            via: format!("{}.register", lib_chain),
            confidence: 0.97,
            detector: "elixir.external_register".to_string(),
            reason: "外部库 register 调用内部模块".to_string(),
        });
    }

    // Detector 3: elixir.supervisor_children — children = [...] 中的模块引用
    for module_name in detect_supervisor_children(content) {
        hints.push(RelationHintRecord {
            target: module_name,
            call_type_hint: "supervisor_child".to_string(),
            via: "children list".to_string(),
            confidence: 0.90,
            detector: "elixir.supervisor_children".to_string(),
            reason: "Supervisor children 列表中的模块引用".to_string(),
        });
    }

    // Detector 4: elixir.genserver_call — handle_call/handle_cast 内的外部模块调用
    for (module_name, handler_kind) in detect_genserver_handler_calls(content) {
        hints.push(RelationHintRecord {
            target: module_name,
            call_type_hint: "genserver_runtime_dep".to_string(),
            via: handler_kind,
            confidence: 0.80,
            detector: "elixir.genserver_call".to_string(),
            reason: "GenServer handler 内的外部模块调用".to_string(),
        });
    }

    // Detector 5: elixir.callback_injection — Keyword.get(opts, :key) 模式
    for key_name in detect_keyword_get_injections(content) {
        hints.push(RelationHintRecord {
            target: key_name,
            call_type_hint: "callback_injection".to_string(),
            via: "Keyword.get opts".to_string(),
            confidence: 0.75,
            detector: "elixir.callback_injection".to_string(),
            reason: "Keyword.get opts 回调注入模式".to_string(),
        });
    }

    hints.sort_by(|a, b| {
        a.target
            .cmp(&b.target)
            .then_with(|| a.call_type_hint.cmp(&b.call_type_hint))
            .then_with(|| a.via.cmp(&b.via))
    });
    hints.dedup_by(|a, b| {
        a.target == b.target
            && a.call_type_hint == b.call_type_hint
            && a.via == b.via
            && a.detector == b.detector
    });
    hints
}

/// 检测 `children = [...]` 赋值中的模块引用（Supervisor child specs）
/// 提取列表中的模块名和 tuple 首元素
fn detect_supervisor_children(content: &str) -> Vec<String> {
    let tree = common::parse_tree(content, tree_sitter_elixir::LANGUAGE, "elixir");
    let root = tree.root_node();
    let source = content.as_bytes();
    let mut modules = Vec::new();
    collect_supervisor_children(root, source, &mut modules);
    modules.sort();
    modules.dedup();
    modules
}

fn collect_supervisor_children(
    node: tree_sitter::Node,
    source: &[u8],
    modules: &mut Vec<String>,
) {
    // 寻找 match 操作符 `children = [...]`
    // tree-sitter-elixir: binary_operator[operator="=", left=identifier("children"), right=list]
    if node.kind() == "binary_operator" {
        if let Some(op) = node.child_by_field_name("operator") {
            if common::node_text(op, source) == "=" {
                if let Some(left) = node.child_by_field_name("left") {
                    let left_text = common::node_text(left, source);
                    if left_text == "children" {
                        if let Some(right) = node.child_by_field_name("right") {
                            if right.kind() == "list" {
                                extract_modules_from_list(right, source, modules);
                                return;
                            }
                        }
                    }
                }
            }
        }
    }

    // 跳过字符串/注释节点
    let skip = matches!(
        node.kind(),
        "string" | "quoted_content" | "charlist" | "sigil" | "comment"
    );
    if skip {
        return;
    }

    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            collect_supervisor_children(child, source, modules);
        }
    }
}

/// 从 list 节点提取模块名：裸模块（`MyApp.Worker`）和 tuple 首元素（`{Registry, ...}`）
fn extract_modules_from_list(
    list_node: tree_sitter::Node,
    source: &[u8],
    modules: &mut Vec<String>,
) {
    for i in 0..list_node.child_count() {
        if let Some(child) = list_node.child(i) {
            match child.kind() {
                // 裸模块引用：MyApp.Worker
                "dot" | "alias" => {
                    let text = common::node_text(child, source).trim().to_string();
                    if is_module_chain(&text, false) {
                        modules.push(text);
                    }
                }
                // tuple: {Registry, keys: :unique, name: ...}
                "tuple" => {
                    // tuple 首元素是模块名
                    if let Some(first) = first_non_paren_child(&child) {
                        let text = common::node_text(first, source).trim().to_string();
                        if is_module_chain(&text, false) {
                            modules.push(text);
                        }
                    }
                }
                _ => {
                    // 可能是单个 alias 节点
                    let text = common::node_text(child, source).trim().to_string();
                    if is_module_chain(&text, false) && text.contains('.') {
                        modules.push(text);
                    }
                }
            }
        }
    }
}

/// 获取节点的第一个非标点符号子节点
fn first_non_paren_child(node: &tree_sitter::Node) -> Option<tree_sitter::Node> {
    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            let kind = child.kind();
            if kind != "{" && kind != "}" && kind != "," && kind != "[" && kind != "]" {
                return Some(child);
            }
        }
    }
    None
}

/// 检测 handle_call/handle_cast 函数体内的外部模块调用（Module.function 形式）
/// 返回 (模块名, handler类型) 对
fn detect_genserver_handler_calls(content: &str) -> Vec<(String, String)> {
    let tree = common::parse_tree(content, tree_sitter_elixir::LANGUAGE, "elixir");
    let root = tree.root_node();
    let source = content.as_bytes();
    let mut calls = Vec::new();
    collect_genserver_handler_calls(root, source, &mut calls);
    calls.sort();
    calls.dedup();
    calls
}

fn collect_genserver_handler_calls(
    node: tree_sitter::Node,
    source: &[u8],
    calls: &mut Vec<(String, String)>,
) {
    // 找 def handle_call / def handle_cast
    if node.kind() == "call" {
        if let Some(target) = node.child_by_field_name("target") {
            let target_text = common::node_text(target, source);
            if target_text == "def" {
                if let Some(handler_kind) = extract_handler_kind(&node, source) {
                    // 在函数体（do_block）中查找 Module.func 调用
                    if let Some(do_block) = find_child_by_kind(&node, "do_block") {
                        collect_dot_calls_in_body(do_block, source, &handler_kind, calls);
                    }
                    return;
                }
            }
        }
    }

    let skip = matches!(
        node.kind(),
        "string" | "quoted_content" | "charlist" | "sigil" | "comment"
    );
    if skip {
        return;
    }

    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            collect_genserver_handler_calls(child, source, calls);
        }
    }
}

/// 从 def 调用中提取 handler 类型（handle_call 或 handle_cast），如果匹配则返回
fn extract_handler_kind(def_node: &tree_sitter::Node, source: &[u8]) -> Option<String> {
    // def 的 arguments 中包含 handle_call/handle_cast 调用节点
    if let Some(args) = find_child_by_kind(def_node, "arguments") {
        for i in 0..args.child_count() {
            if let Some(child) = args.child(i) {
                let text = common::node_text(child, source);
                if text.starts_with("handle_call") {
                    return Some("handle_call".to_string());
                }
                if text.starts_with("handle_cast") {
                    return Some("handle_cast".to_string());
                }
            }
        }
    }
    None
}

/// 在函数体中递归查找 Module.func 形式的 dot 调用
fn collect_dot_calls_in_body(
    node: tree_sitter::Node,
    source: &[u8],
    handler_kind: &str,
    calls: &mut Vec<(String, String)>,
) {
    if node.kind() == "dot" {
        if let Some(left) = node.child_by_field_name("left") {
            let module = common::node_text(left, source);
            if !module.is_empty()
                && module.chars().next().map_or(false, |c| c.is_uppercase())
                && module.contains('.')
            {
                calls.push((module, handler_kind.to_string()));
            }
        }
    }

    let skip = matches!(
        node.kind(),
        "string" | "quoted_content" | "charlist" | "sigil" | "comment"
    );
    if skip {
        return;
    }

    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            collect_dot_calls_in_body(child, source, handler_kind, calls);
        }
    }
}

/// 检测 Keyword.get(opts, :key) 模式，提取 key 名
fn detect_keyword_get_injections(content: &str) -> Vec<String> {
    let tree = common::parse_tree(content, tree_sitter_elixir::LANGUAGE, "elixir");
    let root = tree.root_node();
    let source = content.as_bytes();
    let mut keys = Vec::new();
    collect_keyword_get_injections(root, source, &mut keys);
    keys.sort();
    keys.dedup();
    keys
}

fn collect_keyword_get_injections(
    node: tree_sitter::Node,
    source: &[u8],
    keys: &mut Vec<String>,
) {
    // 匹配 Keyword.get(..., :key) 或 Keyword.get(..., :key, default) 调用
    if node.kind() == "call" {
        if let Some(target) = node.child_by_field_name("target") {
            let target_text = common::node_text(target, source);
            if target_text == "Keyword.get" || target_text == "Keyword.fetch" || target_text == "Keyword.fetch!" || target_text == "Keyword.get_lazy" {
                if let Some(args) = find_child_by_kind(&node, "arguments") {
                    // 从参数中找 :atom 形式的 key
                    let args_text = common::node_text(args, source);
                    if let Some(key) = extract_keyword_atom_key(&args_text) {
                        keys.push(key);
                    }
                }
            }
        }
    }

    let skip = matches!(
        node.kind(),
        "string" | "quoted_content" | "charlist" | "sigil" | "comment"
    );
    if skip {
        return;
    }

    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            collect_keyword_get_injections(child, source, keys);
        }
    }
}

/// 从 Keyword.get 的参数文本中提取 :atom key
/// 例如 `(opts, :llm_backend)` → `llm_backend`
fn extract_keyword_atom_key(args_text: &str) -> Option<String> {
    // 找第二个参数中的 :atom
    let re = Regex::new(r",\s*:([a-z_][a-z0-9_]*)").ok()?;
    re.captures(args_text)
        .and_then(|cap| cap.get(1).map(|m| m.as_str().to_string()))
}

fn detect_external_register_calls(content: &str) -> Vec<(String, String)> {
    let tree = common::parse_tree(content, tree_sitter_elixir::LANGUAGE, "elixir");
    let root = tree.root_node();
    let source = content.as_bytes();
    let mut calls = Vec::new();
    collect_external_register_calls(root, source, &mut calls);
    calls
}

fn collect_external_register_calls(
    node: tree_sitter::Node,
    source: &[u8],
    calls: &mut Vec<(String, String)>,
) {
    if node.kind() == "call" {
        if let Some(hit) = parse_external_register_call(node, source) {
            calls.push(hit);
        }
    }

    let skip_children = matches!(
        node.kind(),
        "string" | "quoted_content" | "charlist" | "sigil" | "comment"
    );
    if skip_children {
        return;
    }

    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            collect_external_register_calls(child, source, calls);
        }
    }
}

fn parse_external_register_call(
    call_node: tree_sitter::Node,
    source: &[u8],
) -> Option<(String, String)> {
    let target = call_node.child_by_field_name("target")?;
    let target_text = common::node_text(target, source);
    let lib_chain = target_text.strip_suffix(".register")?.trim();
    if !is_module_chain(lib_chain, false) {
        return None;
    }

    let args = find_child_by_kind(&call_node, "arguments")?;
    let args_text = common::node_text(args, source);
    let target_module = extract_first_module_argument(&args_text)?;
    if !is_module_chain(&target_module, true) {
        return None;
    }

    Some((lib_chain.to_string(), target_module))
}

fn extract_first_module_argument(args_text: &str) -> Option<String> {
    let module_arg_re =
        Regex::new(r"^\s*\(?\s*([A-Z][A-Za-z0-9_]*(?:\.[A-Z][A-Za-z0-9_]*)+)").ok()?;
    module_arg_re
        .captures(args_text)
        .and_then(|cap| cap.get(1).map(|m| m.as_str().to_string()))
}

fn collect_alias_bindings(imports: &[ImportRecord]) -> HashMap<String, Vec<String>> {
    let mut out: HashMap<String, Vec<String>> = HashMap::new();
    for imp in imports {
        if imp.kind != "alias" {
            continue;
        }
        let Some((alias, full_module)) = parse_alias_binding(&imp.specifier) else {
            continue;
        };
        let entry = out.entry(alias).or_default();
        if !entry.iter().any(|existing| existing == &full_module) {
            entry.push(full_module);
        }
    }
    out
}

fn parse_alias_binding(specifier: &str) -> Option<(String, String)> {
    let trimmed = specifier.trim();
    if trimmed.is_empty() || trimmed.starts_with('{') {
        return None;
    }

    let mut parts = trimmed.splitn(2, ',');
    let module_path = parts.next()?.trim();
    if !is_module_chain(module_path, false) {
        return None;
    }

    let alias_name = if let Some(rest) = parts.next() {
        let rest = rest.trim();
        if let Some(idx) = rest.find("as:") {
            let alias_raw = rest[idx + 3..].trim();
            let alias = alias_raw
                .chars()
                .take_while(|ch| ch.is_ascii_alphanumeric() || *ch == '_')
                .collect::<String>();
            if alias.is_empty() {
                module_path
                    .rsplit('.')
                    .next()
                    .map(|s| s.to_string())
                    .unwrap_or_default()
            } else {
                alias
            }
        } else {
            module_path
                .rsplit('.')
                .next()
                .map(|s| s.to_string())
                .unwrap_or_default()
        }
    } else {
        module_path
            .rsplit('.')
            .next()
            .map(|s| s.to_string())
            .unwrap_or_default()
    };

    if alias_name.is_empty() {
        return None;
    }
    Some((alias_name, module_path.to_string()))
}

fn first_module_segment(module_path: &str) -> Option<&str> {
    module_path.split('.').next().filter(|seg| !seg.is_empty())
}

fn is_module_chain(value: &str, require_nested: bool) -> bool {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        return false;
    }
    let mut segments = trimmed.split('.');
    let mut count = 0usize;
    for seg in segments.by_ref() {
        if seg.is_empty() {
            return false;
        }
        let mut chars = seg.chars();
        let Some(first) = chars.next() else {
            return false;
        };
        if !first.is_ascii_uppercase() {
            return false;
        }
        if !chars.all(|ch| ch.is_ascii_alphanumeric() || ch == '_') {
            return false;
        }
        count += 1;
    }
    if require_nested {
        count >= 2
    } else {
        count >= 1
    }
}

fn has_register_namespace_conflict(
    lib_chain: &str,
    target_module: &str,
    alias_bindings: &HashMap<String, Vec<String>>,
) -> bool {
    let Some(lib_root) = first_module_segment(lib_chain) else {
        return true;
    };
    let Some(target_root) = first_module_segment(target_module) else {
        return true;
    };

    // 同根命名空间无法确定是外部注册还是内部模块调用，保守降级 direct_call。
    if lib_root == target_root {
        return true;
    }

    let Some(resolved) = alias_bindings.get(lib_root) else {
        return false;
    };
    if resolved.len() != 1 {
        return true;
    }

    let Some(resolved_root) = first_module_segment(&resolved[0]) else {
        return true;
    };
    resolved_root == target_root
}

fn extract_use_main_module(specifier: &str) -> String {
    let trimmed = specifier.trim();
    let end = trimmed
        .find(',')
        .unwrap_or(trimmed.len())
        .min(trimmed.find('\n').unwrap_or(trimmed.len()));
    trimmed[..end].trim().to_string()
}

fn extract_use_injected_modules(specifier: &str) -> Vec<String> {
    let mut refs = Vec::new();
    let bytes = specifier.as_bytes();
    let len = bytes.len();
    let mut idx = specifier.find(',').unwrap_or(len);

    while idx < len {
        if bytes[idx].is_ascii_uppercase() {
            let start = idx;
            while idx < len
                && (bytes[idx].is_ascii_alphanumeric() || bytes[idx] == b'_' || bytes[idx] == b'.')
            {
                idx += 1;
            }
            let candidate = specifier[start..idx].trim_end_matches('.');
            if candidate.split('.').count() >= 2 {
                refs.push(candidate.to_string());
            }
            continue;
        }
        idx += 1;
    }

    refs.sort();
    refs.dedup();
    refs
}

/// 从文件路径推断 Elixir 模块名
/// 例如 lib/my_app/accounts/user.ex → MyApp.Accounts.User
pub fn infer_module_from_path(rel_path: &str) -> Option<String> {
    let path = rel_path.strip_suffix(".ex")?;
    // 去掉 lib/ 前缀
    let path = path.strip_prefix("lib/").unwrap_or(path);
    let parts: Vec<&str> = path.split('/').collect();
    if parts.is_empty() {
        return None;
    }
    let module_name = parts
        .iter()
        .map(|part| {
            part.split('_')
                .map(|w| {
                    let mut chars = w.chars();
                    match chars.next() {
                        None => String::new(),
                        Some(c) => c.to_uppercase().to_string() + chars.as_str(),
                    }
                })
                .collect::<Vec<_>>()
                .join("")
        })
        .collect::<Vec<_>>()
        .join(".");
    Some(module_name)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn infer_module_from_lib_path() {
        assert_eq!(
            infer_module_from_path("lib/my_app/accounts/user.ex"),
            Some("MyApp.Accounts.User".to_string())
        );
    }

    #[test]
    fn infer_module_from_nested_path() {
        assert_eq!(
            infer_module_from_path("lib/gong_web/live/dashboard_live.ex"),
            Some("GongWeb.Live.DashboardLive".to_string())
        );
    }

    #[test]
    fn infer_module_without_lib_prefix() {
        // 直接传相对路径（没有 lib/ 前缀）
        assert_eq!(
            infer_module_from_path("accounts/user.ex"),
            Some("Accounts.User".to_string())
        );
    }

    #[test]
    fn infer_module_returns_none_for_non_ex() {
        assert_eq!(infer_module_from_path("lib/mix.exs"), None);
        assert_eq!(infer_module_from_path("lib/config.exs"), None);
    }

    #[test]
    fn extract_behaviour_as_import() {
        let source = r#"
defmodule MyApp.Worker do
  @behaviour GenServer
  @behaviour Gong.Extension

  def init(state), do: {:ok, state}
end
"#;
        let record = extract(source, "lib/my_app/worker.ex");
        // @behaviour 应被记为 import
        let behaviour_imports: Vec<_> = record
            .imports
            .iter()
            .filter(|i| i.kind == "behaviour")
            .collect();
        assert_eq!(behaviour_imports.len(), 2);
        assert!(behaviour_imports.iter().any(|i| i.specifier == "GenServer"));
        assert!(behaviour_imports
            .iter()
            .any(|i| i.specifier == "Gong.Extension"));
    }

    #[test]
    fn test_moduledoc_module_ref_not_extracted_as_call() {
        let source = r#"
defmodule Gong do
  @moduledoc """
  - `Gong.Compaction` — 上下文压缩
  - `Gong.Truncate` — 输出截断系统
  """
end
"#;
        let record = extract(source, "lib/gong.ex");
        // @moduledoc 中提到的模块不应出现在 calls 中
        assert!(
            record.calls.is_empty(),
            "expected no calls, got: {:?}",
            record.calls
        );
    }

    #[test]
    fn test_real_call_still_extracted() {
        let source = r#"
defmodule MyApp.Worker do
  @moduledoc "Worker that calls Fake.Module in docs"

  def run do
    Result.ok()
    Phoenix.PubSub.broadcast(topic, msg)
  end
end
"#;
        let record = extract(source, "lib/my_app/worker.ex");
        let callees: Vec<&str> = record.calls.iter().map(|c| c.callee.as_str()).collect();
        assert!(
            callees.contains(&"Result"),
            "should contain Result, got: {:?}",
            callees
        );
        assert!(
            callees.contains(&"Phoenix.PubSub"),
            "should contain Phoenix.PubSub, got: {:?}",
            callees
        );
        // 文档字符串中的 Fake.Module 不应被提取
        assert!(
            !callees.contains(&"Fake"),
            "should not contain Fake from docstring, got: {:?}",
            callees
        );
    }

    #[test]
    fn test_doc_string_module_ref_not_extracted() {
        let source = r#"
defmodule MyApp.Foo do
  @doc "See `MyApp.Bar` for details"
  def hello, do: :world
end
"#;
        let record = extract(source, "lib/my_app/foo.ex");
        let callees: Vec<&str> = record.calls.iter().map(|c| c.callee.as_str()).collect();
        assert!(
            !callees.contains(&"MyApp"),
            "should not contain MyApp.Bar from @doc, got: {:?}",
            callees
        );
    }

    #[test]
    fn extract_use_with_keyword_args() {
        let source = r#"
defmodule Test do
  use Jido.AI.ReActAgent,
    tools: [Gong.Tools.Read, Gong.Tools.Write],
    model: "deepseek"

  def run, do: :ok
end
"#;
        let record = extract(source, "lib/test.ex");
        // use 应该被记为 import
        let use_imports: Vec<_> = record.imports.iter().filter(|i| i.kind == "use").collect();
        assert!(!use_imports.is_empty());
        // specifier 应该包含完整的 use 参数文本
        let spec = &use_imports[0].specifier;
        assert!(spec.contains("Jido.AI.ReActAgent"));
    }

    #[test]
    fn alias_declaration_dot_not_extracted_as_call() {
        let source = r#"defmodule Demo do
  alias Foo.{Bar, Baz}
  def run, do: :ok
end
"#;
        let record = extract(source, "lib/demo.ex");
        assert!(
            record.calls.is_empty(),
            "alias declaration should not create calls, got: {:?}",
            record.calls
        );
    }

    #[test]
    fn alias_and_real_call_only_tracks_runtime_call() {
        let source = r#"defmodule Demo do
  alias Foo.Bar
  def run, do: Foo.Bar.execute()
end
"#;
        let record = extract(source, "lib/demo.ex");
        let call_lines: Vec<usize> = record.calls.iter().map(|call| call.line).collect();
        assert!(
            !record.calls.is_empty(),
            "real runtime call should be extracted, got empty calls"
        );
        assert!(
            !call_lines.contains(&2),
            "alias declaration line should not emit calls, got lines: {:?}",
            call_lines
        );
    }

    #[test]
    fn import_use_require_declaration_dot_not_extracted_as_call() {
        let cases = vec![
            ("import", "import Foo.Bar"),
            ("use", "use Foo.Bar"),
            ("require", "require Foo.Bar"),
        ];

        for (name, declaration) in cases {
            let source = format!(
                "defmodule Demo do\n  {}\n  def run, do: :ok\nend\n",
                declaration
            );
            let record = extract(&source, "lib/demo.ex");
            assert!(
                record.calls.is_empty(),
                "{} declaration should not create calls, got: {:?}",
                name,
                record.calls
            );
        }
    }

    #[test]
    fn detect_use_macro_relation_hints() {
        let source = r#"
defmodule Gong.Agent do
  use Jido.AI.ReActAgent, tools: [Gong.Tools.Read, Gong.Tools.Write]
end
"#;
        let record = extract(source, "lib/gong/agent.ex");
        let targets: Vec<String> = record
            .relation_hints
            .iter()
            .map(|h| h.target.clone())
            .collect();
        assert!(targets.iter().any(|t| t == "Gong.Tools.Read"));
        assert!(targets.iter().any(|t| t == "Gong.Tools.Write"));
        assert!(record
            .relation_hints
            .iter()
            .all(|h| h.call_type_hint == "framework_injection"));
    }

    #[test]
    fn detect_external_register_relation_hint() {
        let source = r#"
defmodule Gong.Infra.ReqLlm do
  def init do
    ReqLLM.Providers.register(Gong.Provider.Anthropic)
  end
end
"#;
        let record = extract(source, "lib/gong/infra/req_llm.ex");
        let hit = record
            .relation_hints
            .iter()
            .find(|h| h.target == "Gong.Provider.Anthropic")
            .expect("external register hint should exist");
        assert_eq!(hit.call_type_hint, "external_registration");
        assert!(hit.via.contains("ReqLLM.Providers.register"));
    }

    #[test]
    fn ignore_external_register_text_inside_comment_and_string() {
        let source = r#"
defmodule Gong.Infra.ReqLlm do
  # ReqLLM.Providers.register(Gong.Provider.Anthropic)
  @doc "ReqLLM.Providers.register(Gong.Provider.Anthropic)"
  def init, do: :ok
end
"#;
        let record = extract(source, "lib/gong/infra/req_llm.ex");
        assert!(record
            .relation_hints
            .iter()
            .all(|h| h.call_type_hint != "external_registration"));
    }

    #[test]
    fn ambiguous_same_namespace_register_is_downgraded() {
        let source = r#"
defmodule Gong.Infra.Registry do
  def init do
    Gong.Providers.register(Gong.Provider.Anthropic)
  end
end
"#;
        let record = extract(source, "lib/gong/infra/registry.ex");
        assert!(record
            .relation_hints
            .iter()
            .all(|h| h.call_type_hint != "external_registration"));
    }

    #[test]
    fn detect_supervisor_children_standard() {
        let source = r#"
defmodule MyApp.Application do
  def start(_type, _args) do
    children = [
      {Registry, keys: :unique, name: MyApp.Registry},
      {DynamicSupervisor, name: MyApp.DynSup},
      MyApp.Worker
    ]
    Supervisor.start_link(children, strategy: :one_for_one)
  end
end
"#;
        let record = extract(source, "lib/my_app/application.ex");
        let supervisor_hints: Vec<_> = record
            .relation_hints
            .iter()
            .filter(|h| h.call_type_hint == "supervisor_child")
            .collect();
        let targets: Vec<&str> = supervisor_hints.iter().map(|h| h.target.as_str()).collect();
        assert!(targets.contains(&"Registry"), "should contain Registry, got: {:?}", targets);
        assert!(targets.contains(&"DynamicSupervisor"), "should contain DynamicSupervisor, got: {:?}", targets);
        assert!(targets.contains(&"MyApp.Worker"), "should contain MyApp.Worker, got: {:?}", targets);
        assert!(supervisor_hints.iter().all(|h| (h.confidence - 0.90).abs() < f64::EPSILON));
    }

    #[test]
    fn detect_supervisor_children_empty_list() {
        let source = r#"
defmodule MyApp.Application do
  def start(_type, _args) do
    children = []
    Supervisor.start_link(children, strategy: :one_for_one)
  end
end
"#;
        let record = extract(source, "lib/my_app/application.ex");
        assert!(record
            .relation_hints
            .iter()
            .all(|h| h.call_type_hint != "supervisor_child"));
    }

    #[test]
    fn detect_genserver_handle_call_deps() {
        let source = r#"
defmodule MyApp.Session do
  use GenServer

  def handle_call({:prompt, msg}, _from, state) do
    result = MyApp.Agent.process(msg)
    stream = MyApp.Stream.emit(result)
    {:reply, stream, state}
  end
end
"#;
        let record = extract(source, "lib/my_app/session.ex");
        let gs_hints: Vec<_> = record
            .relation_hints
            .iter()
            .filter(|h| h.call_type_hint == "genserver_runtime_dep")
            .collect();
        let targets: Vec<&str> = gs_hints.iter().map(|h| h.target.as_str()).collect();
        assert!(targets.contains(&"MyApp.Agent"), "should contain MyApp.Agent, got: {:?}", targets);
        assert!(targets.contains(&"MyApp.Stream"), "should contain MyApp.Stream, got: {:?}", targets);
        assert!(gs_hints.iter().all(|h| h.via == "handle_call"));
    }

    #[test]
    fn detect_genserver_handle_cast_deps() {
        let source = r#"
defmodule MyApp.Worker do
  use GenServer

  def handle_cast({:async_op, data}, state) do
    MyApp.Worker.process(data)
    {:noreply, state}
  end
end
"#;
        let record = extract(source, "lib/my_app/worker.ex");
        let gs_hints: Vec<_> = record
            .relation_hints
            .iter()
            .filter(|h| h.call_type_hint == "genserver_runtime_dep")
            .collect();
        assert_eq!(gs_hints.len(), 1);
        assert_eq!(gs_hints[0].target, "MyApp.Worker");
        assert_eq!(gs_hints[0].via, "handle_cast");
    }

    #[test]
    fn detect_keyword_get_injection() {
        let source = r#"
defmodule MyApp.Session do
  def start_link(opts) do
    llm_backend = Keyword.get(opts, :llm_backend)
    compaction = Keyword.get(opts, :compaction_strategy)
    GenServer.start_link(__MODULE__, %{llm: llm_backend, comp: compaction})
  end
end
"#;
        let record = extract(source, "lib/my_app/session.ex");
        let cb_hints: Vec<_> = record
            .relation_hints
            .iter()
            .filter(|h| h.call_type_hint == "callback_injection")
            .collect();
        let targets: Vec<&str> = cb_hints.iter().map(|h| h.target.as_str()).collect();
        assert!(targets.contains(&"llm_backend"), "should contain llm_backend, got: {:?}", targets);
        assert!(targets.contains(&"compaction_strategy"), "should contain compaction_strategy, got: {:?}", targets);
        assert!(cb_hints.iter().all(|h| (h.confidence - 0.75).abs() < f64::EPSILON));
    }

    #[test]
    fn detect_all_five_detectors_coexist() {
        let source = r#"
defmodule MyApp.FullExample do
  use Jido.AI.ReActAgent, tools: [MyApp.Tools.Read]

  def start(_type, _args) do
    ReqLLM.Providers.register(MyApp.Provider.Anthropic)

    children = [
      {Registry, keys: :unique, name: MyApp.Registry},
      MyApp.Worker
    ]

    llm = Keyword.get(opts, :llm_backend)
    Supervisor.start_link(children, strategy: :one_for_one)
  end

  def handle_call(:get, _from, state) do
    result = MyApp.Agent.process(state)
    {:reply, result, state}
  end
end
"#;
        let record = extract(source, "lib/my_app/full_example.ex");
        let types: Vec<&str> = record
            .relation_hints
            .iter()
            .map(|h| h.call_type_hint.as_str())
            .collect();
        assert!(types.contains(&"framework_injection"), "missing framework_injection");
        assert!(types.contains(&"external_registration"), "missing external_registration");
        assert!(types.contains(&"supervisor_child"), "missing supervisor_child");
        assert!(types.contains(&"genserver_runtime_dep"), "missing genserver_runtime_dep");
        assert!(types.contains(&"callback_injection"), "missing callback_injection");
    }
}

/// 基于全文关键词扫描的副作用分类标签（行为检测的分类维度）
/// 独立于 tree-sitter 提取，用 contains() 捕获 use GenServer 等非 dot-call 模式
fn detect_side_effects(content: &str, se: &mut SideEffects) {
    if content.contains("Task.async") || content.contains("Task.start") {
        se.has_async = true;
    }
    if content.contains("HTTPoison")
        || content.contains("Req.")
        || content.contains("Tesla.")
        || content.contains("Finch.")
    {
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
