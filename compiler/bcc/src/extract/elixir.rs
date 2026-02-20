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
    let mut type_annotations = Vec::new();
    let mut type_guards = Vec::new();
    let mut schema_fields = Vec::new();

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
        &mut type_annotations,
        &mut type_guards,
        &mut schema_fields,
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
        type_annotations,
        type_guards,
        schema_fields,
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
    type_annotations: &mut Vec<TypeAnnotation>,
    type_guards: &mut Vec<TypeGuard>,
    schema_fields: &mut Vec<SchemaField>,
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
                            let spec_text = pending_spec.take();
                            if let Some((name, line)) = extract_def_name(&node, source) {
                                // 从 @spec 提取参数类型标注 → TypeAnnotation
                                if let Some(ref spec) = spec_text {
                                    let params = parse_spec_params(spec);
                                    let param_names = extract_def_param_names(&node, source);
                                    for (i, type_expr) in params.iter().enumerate() {
                                        let param = param_names
                                            .get(i)
                                            .cloned()
                                            .unwrap_or_else(|| format!("arg{}", i));
                                        type_annotations.push(TypeAnnotation {
                                            name: format!("@spec {}", name),
                                            type_expr: type_expr.clone(),
                                            line,
                                            func_name: name.clone(),
                                            param,
                                        });
                                    }
                                }
                                // 从 when 子句提取 guard → TypeGuard
                                let guards = extract_when_guards(&node, source, &name);
                                type_guards.extend(guards);

                                exports.push(ExportRecord {
                                    name,
                                    kind: "function".into(),
                                    signature: spec_text,
                                    line,
                                });
                            }
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
                        // Ecto schema / embedded_schema：提取 field 定义 → SchemaField
                        "schema" | "embedded_schema" => {
                            extract_ecto_schema_fields(&node, source, schema_fields);
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
                            // dot 节点的 parent 是 call 节点，从 call 节点提取参数
                            let args = node
                                .parent()
                                .map(|p| extract_elixir_call_args(p, source))
                                .unwrap_or_default();
                            calls.push(CallRecord {
                                callee: module,
                                line: node.start_position().row + 1,
                                args,
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
                type_annotations,
                type_guards,
                schema_fields,
            );
            cursor.goto_parent();
        }

        if !cursor.goto_next_sibling() {
            break;
        }
    }
}

/// 提取调用参数的文本列表（用于 detect_unnecessary_default 等规则匹配）
/// Elixir 的 dot 节点位于 call 节点内部，arguments 是 call 的子节点。
fn extract_elixir_call_args(call_node: tree_sitter::Node, source: &[u8]) -> Vec<String> {
    let mut args = Vec::new();
    if let Some(args_node) = find_child_by_kind(&call_node, "arguments") {
        for i in 0..args_node.child_count() {
            if let Some(arg) = args_node.child(i) {
                // 跳过括号和逗号
                if matches!(arg.kind(), "(" | ")" | ",") {
                    continue;
                }
                let text = common::node_text(arg, source);
                // 去掉引号
                let cleaned = text.trim_matches(|c: char| c == '\'' || c == '"');
                if !cleaned.is_empty() {
                    args.push(cleaned.to_string());
                }
            }
        }
    }
    args
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
                    // arguments 内含函数签名 call 节点，
                    // 或 binary_operator[when]（def foo(x) when is_map(x)）
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
                                "binary_operator" => {
                                    // when 子句：左侧是函数签名 call
                                    if let Some(left) = sub.child_by_field_name("left") {
                                        if left.kind() == "call" {
                                            if let Some(t) = left.child_by_field_name("target") {
                                                return Some((
                                                    common::node_text(t, source),
                                                    left.start_position().row + 1,
                                                ));
                                            }
                                        }
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

// === @spec 参数类型解析 + when guard 提取 ===

/// 从 @spec 文本中提取参数类型列表
/// 例如 `foo(map(), integer()) :: term()` → ["map()", "integer()"]
/// 例如 `bar(binary()) :: atom()` → ["binary()"]
fn parse_spec_params(spec_text: &str) -> Vec<String> {
    // 找到第一个 ( 和对应的 )
    let Some(paren_start) = spec_text.find('(') else {
        return vec![];
    };
    let mut depth = 0;
    let mut paren_end = None;
    for (i, ch) in spec_text[paren_start..].char_indices() {
        match ch {
            '(' => depth += 1,
            ')' => {
                depth -= 1;
                if depth == 0 {
                    paren_end = Some(paren_start + i);
                    break;
                }
            }
            _ => {}
        }
    }
    let Some(paren_end) = paren_end else {
        return vec![];
    };
    let params_text = &spec_text[paren_start + 1..paren_end];
    if params_text.trim().is_empty() {
        return vec![];
    }
    // 按顶层逗号分割（跳过括号内的逗号）
    let mut params = Vec::new();
    let mut current = String::new();
    let mut depth = 0i32;
    for ch in params_text.chars() {
        match ch {
            '(' | '[' | '{' => {
                depth += 1;
                current.push(ch);
            }
            ')' | ']' | '}' => {
                depth -= 1;
                current.push(ch);
            }
            ',' if depth == 0 => {
                let trimmed = current.trim().to_string();
                if !trimmed.is_empty() {
                    params.push(trimmed);
                }
                current.clear();
            }
            _ => current.push(ch),
        }
    }
    let trimmed = current.trim().to_string();
    if !trimmed.is_empty() {
        params.push(trimmed);
    }
    params
}

/// 从 def 节点提取参数名列表
/// 例如 `def foo(data, count)` → ["data", "count"]
fn extract_def_param_names(call_node: &tree_sitter::Node, source: &[u8]) -> Vec<String> {
    let mut names = Vec::new();
    // 找 def 的 arguments 子节点
    let Some(args) = find_child_by_kind(call_node, "arguments") else {
        return names;
    };
    // arguments 内有一个 call 节点（函数签名）
    for i in 0..args.child_count() {
        let Some(child) = args.child(i) else { continue };
        match child.kind() {
            "call" => {
                // call 的 arguments 包含函数参数
                if let Some(fn_args) = find_child_by_kind(&child, "arguments") {
                    collect_param_names(fn_args, source, &mut names);
                }
            }
            "binary_operator" => {
                // def foo(data) when is_map(data) — binary_operator with 'when'
                if let Some(left) = child.child_by_field_name("left") {
                    if left.kind() == "call" {
                        if let Some(fn_args) = find_child_by_kind(&left, "arguments") {
                            collect_param_names(fn_args, source, &mut names);
                        }
                    }
                }
            }
            _ => {}
        }
    }
    names
}

/// 从参数列表节点中收集参数名
fn collect_param_names(args_node: tree_sitter::Node, source: &[u8], names: &mut Vec<String>) {
    for i in 0..args_node.child_count() {
        let Some(child) = args_node.child(i) else { continue };
        let kind = child.kind();
        if kind == "identifier" {
            let text = common::node_text(child, source);
            if !text.starts_with('_') {
                names.push(text);
            } else {
                // _var 形式，保留变量名但去掉前缀下划线
                names.push(text.trim_start_matches('_').to_string());
            }
        } else if kind == "unary_operator" {
            // 匹配模式如 %{} = data 或 pin 操作
            // 尝试提取内部标识符
            let text = common::node_text(child, source);
            // 简单情况：取最后的标识符
            if let Some(last) = text.split_whitespace().last() {
                let clean = last.trim_start_matches('_');
                if !clean.is_empty() && clean.chars().all(|c| c.is_alphanumeric() || c == '_') {
                    names.push(clean.to_string());
                }
            }
        }
    }
}

/// 从 Ecto schema/embedded_schema 的 do 块中提取 field 定义
fn extract_ecto_schema_fields(
    schema_call: &tree_sitter::Node,
    source: &[u8],
    schema_fields: &mut Vec<SchemaField>,
) {
    // 查找 do_block 子节点
    let mut do_block = None;
    for i in 0..schema_call.child_count() {
        if let Some(child) = schema_call.child(i) {
            if child.kind() == "do_block" {
                do_block = Some(child);
                break;
            }
        }
    }
    let Some(block) = do_block else { return };

    // 遍历 do_block 内的节点，查找 field 调用
    let mut stack = vec![block];
    while let Some(node) = stack.pop() {
        if node.kind() == "call" {
            if let Some(target) = node.child_by_field_name("target") {
                let target_text = common::node_text(target, source);
                if target_text == "field" {
                    if let Some(args) = find_child_by_kind(&node, "arguments") {
                        let mut field_name = String::new();
                        let mut field_type = String::new();
                        let mut has_default = false;
                        let mut arg_idx = 0;
                        for j in 0..args.child_count() {
                            if let Some(arg) = args.child(j) {
                                // 跳过分隔符
                                if arg.kind() == "," { continue; }
                                let text = common::node_text(arg, source);
                                if arg_idx == 0 {
                                    // 第一个参数：字段名（atom）
                                    field_name = text.trim_start_matches(':').to_string();
                                } else if arg_idx == 1 {
                                    // 第二个参数：类型
                                    field_type = text.trim_start_matches(':').to_string();
                                } else {
                                    // 后续参数：keyword args（检查 default）
                                    if text.contains("default") {
                                        has_default = true;
                                    }
                                }
                                arg_idx += 1;
                            }
                        }
                        if !field_name.is_empty() {
                            schema_fields.push(SchemaField {
                                name: field_name,
                                field_type,
                                line: node.start_position().row + 1,
                                // Ecto field 默认 required，有 default 值则视为 optional
                                required: !has_default,
                            });
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

/// Elixir guard 函数名 → 类型映射
fn guard_to_type(guard_func: &str) -> Option<&'static str> {
    match guard_func {
        "is_map" => Some("map()"),
        "is_integer" => Some("integer()"),
        "is_binary" => Some("binary()"),
        "is_atom" => Some("atom()"),
        "is_list" => Some("list()"),
        "is_float" => Some("float()"),
        "is_number" => Some("number()"),
        "is_boolean" => Some("boolean()"),
        "is_tuple" => Some("tuple()"),
        "is_pid" => Some("pid()"),
        "is_reference" => Some("reference()"),
        "is_function" => Some("function()"),
        "is_bitstring" => Some("bitstring()"),
        "is_nil" => Some("nil"),
        _ => None,
    }
}

/// 从 def 节点提取 when guard 信息
/// 例如 `def foo(data) when is_map(data)` → TypeGuard { function: "foo", var: "data", guarded_type: "map()", line }
fn extract_when_guards(
    call_node: &tree_sitter::Node,
    source: &[u8],
    func_name: &str,
) -> Vec<TypeGuard> {
    let mut guards = Vec::new();
    let Some(args) = find_child_by_kind(call_node, "arguments") else {
        return guards;
    };
    // 查找 binary_operator[operator=when]
    for i in 0..args.child_count() {
        let Some(child) = args.child(i) else { continue };
        if child.kind() == "binary_operator" {
            if let Some(op) = child.child_by_field_name("operator") {
                if common::node_text(op, source) == "when" {
                    if let Some(right) = child.child_by_field_name("right") {
                        collect_guard_calls(right, source, func_name, &mut guards);
                    }
                }
            }
        }
    }
    guards
}

/// 递归收集 guard 表达式中的 is_* 调用
fn collect_guard_calls(
    node: tree_sitter::Node,
    source: &[u8],
    func_name: &str,
    guards: &mut Vec<TypeGuard>,
) {
    if node.kind() == "call" {
        if let Some(target) = node.child_by_field_name("target") {
            let target_text = common::node_text(target, source);
            if let Some(guarded_type) = guard_to_type(&target_text) {
                // 提取 guard 调用的第一个参数（变量名）
                if let Some(args) = find_child_by_kind(&node, "arguments") {
                    for j in 0..args.child_count() {
                        if let Some(arg) = args.child(j) {
                            if arg.kind() == "identifier" {
                                let var = common::node_text(arg, source);
                                guards.push(TypeGuard {
                                    function: func_name.to_string(),
                                    guarded_type: guarded_type.to_string(),
                                    line: node.start_position().row + 1,
                                    var,
                                });
                                break;
                            }
                        }
                    }
                }
            }
        }
    }
    // 递归处理 and/or 组合 guard（binary_operator with 'and'/'or'）
    // 注意：不跳过 call 子节点，因为组合 guard 如 `when is_integer(x) and is_binary(y)` 中，
    // binary_operator[and] 的子节点直接是 call 节点，跳过会导致漏提取。
    // 已提取的 call 节点在递归时不会重复匹配（guard_to_type 只匹配 is_* 函数）。
    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            collect_guard_calls(child, source, func_name, guards);
        }
    }
}

// === Visitor pattern: 统一 AST 遍历，消除重复 parse/skip 逻辑 ===

struct DetectContext<'a> {
    imports: &'a [ImportRecord],
    alias_bindings: HashMap<String, Vec<String>>,
}

trait HintDetector {
    fn detect(&self, node: tree_sitter::Node, source: &[u8], ctx: &DetectContext) -> Option<Vec<RelationHintRecord>>;
}

/// 统一 AST 遍历：skip 逻辑只写一份，每个节点调用所有 detector
fn walk_tree(
    node: tree_sitter::Node,
    source: &[u8],
    detectors: &[Box<dyn HintDetector>],
    ctx: &DetectContext,
    hints: &mut Vec<RelationHintRecord>,
) {
    if matches!(node.kind(), "string" | "quoted_content" | "charlist" | "sigil" | "comment") {
        return;
    }
    for detector in detectors {
        if let Some(results) = detector.detect(node, source, ctx) {
            hints.extend(results);
        }
    }
    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            walk_tree(child, source, detectors, ctx, hints);
        }
    }
}

struct SupervisorChildrenDetector;

impl HintDetector for SupervisorChildrenDetector {
    fn detect(&self, node: tree_sitter::Node, source: &[u8], _ctx: &DetectContext) -> Option<Vec<RelationHintRecord>> {
        if node.kind() != "binary_operator" { return None; }
        let op = node.child_by_field_name("operator")?;
        if common::node_text(op, source) != "=" { return None; }
        let left = node.child_by_field_name("left")?;
        if common::node_text(left, source) != "children" { return None; }
        let right = node.child_by_field_name("right")?;
        if right.kind() != "list" { return None; }
        let mut modules = Vec::new();
        extract_modules_from_list(right, source, &mut modules);
        modules.sort();
        modules.dedup();
        Some(modules.into_iter().map(|m| RelationHintRecord {
            target: m, call_type_hint: "supervisor_child".into(),
            via: "children list".into(), confidence: 0.90,
            detector: "elixir.supervisor_children".into(),
            reason: "Supervisor children 列表中的模块引用".into(),
        }).collect())
    }
}

struct GenServerCallDetector;

impl HintDetector for GenServerCallDetector {
    /// 匹配 dot 节点（Module.func），检查是否在 handle_call/handle_cast do_block 内
    fn detect(&self, node: tree_sitter::Node, source: &[u8], _ctx: &DetectContext) -> Option<Vec<RelationHintRecord>> {
        if node.kind() != "dot" { return None; }
        let left = node.child_by_field_name("left")?;
        let module = common::node_text(left, source);
        if module.is_empty() || !module.starts_with(|c: char| c.is_uppercase()) { return None; }
        let handler_kind = find_enclosing_handler(node, source)?;
        Some(vec![RelationHintRecord {
            target: module, call_type_hint: "genserver_runtime_dep".into(),
            via: handler_kind, confidence: 0.80,
            detector: "elixir.genserver_call".into(),
            reason: "GenServer handler 内的外部模块调用".into(),
        }])
    }
}

struct KeywordGetDetector;

impl HintDetector for KeywordGetDetector {
    fn detect(&self, node: tree_sitter::Node, source: &[u8], _ctx: &DetectContext) -> Option<Vec<RelationHintRecord>> {
        if node.kind() != "call" { return None; }
        let target = node.child_by_field_name("target")?;
        let target_text = common::node_text(target, source);
        if !matches!(target_text.as_str(), "Keyword.get" | "Keyword.fetch" | "Keyword.fetch!" | "Keyword.get_lazy") {
            return None;
        }
        let args = find_child_by_kind(&node, "arguments")?;
        let key = extract_keyword_atom_key(&common::node_text(args, source))?;
        Some(vec![RelationHintRecord {
            target: key.clone(), call_type_hint: "callback_injection".into(),
            via: format!("Keyword.get :{}", key), confidence: 0.75,
            detector: "elixir.callback_injection".into(),
            reason: "Keyword.get opts 回调注入模式".into(),
        }])
    }
}

struct ExternalRegisterDetector;

impl HintDetector for ExternalRegisterDetector {
    fn detect(&self, node: tree_sitter::Node, source: &[u8], ctx: &DetectContext) -> Option<Vec<RelationHintRecord>> {
        if node.kind() != "call" { return None; }
        let (lib_chain, target_module) = parse_external_register_call(node, source)?;
        if has_register_namespace_conflict(&lib_chain, &target_module, &ctx.alias_bindings) {
            return None;
        }
        Some(vec![RelationHintRecord {
            target: target_module, call_type_hint: "external_registration".into(),
            via: format!("{}.register", lib_chain), confidence: 0.97,
            detector: "elixir.external_register".into(),
            reason: "外部库 register 调用内部模块".into(),
        }])
    }
}

fn detect_relation_hints(imports: &[ImportRecord], content: &str) -> Vec<RelationHintRecord> {
    let mut hints = Vec::new();

    // UseMacro detector: 基于 imports 列表，不需要 AST 遍历
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

    // AST-based detectors: 一次 parse，一次遍历
    let tree = common::parse_tree(content, tree_sitter_elixir::LANGUAGE, "elixir");
    let source = content.as_bytes();
    let ctx = DetectContext {
        imports,
        alias_bindings: collect_alias_bindings(imports),
    };

    let detectors: Vec<Box<dyn HintDetector>> = vec![
        Box::new(SupervisorChildrenDetector),
        Box::new(GenServerCallDetector),
        Box::new(KeywordGetDetector),
        Box::new(ExternalRegisterDetector),
    ];

    walk_tree(tree.root_node(), source, &detectors, &ctx, &mut hints);

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
fn first_non_paren_child<'a>(node: &'a tree_sitter::Node<'a>) -> Option<tree_sitter::Node<'a>> {
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

/// 向上查找 dot 节点是否在 handle_call/handle_cast 的 do_block 内
fn find_enclosing_handler(node: tree_sitter::Node, source: &[u8]) -> Option<String> {
    let mut current = node.parent();
    let mut inside_do_block = false;
    while let Some(n) = current {
        if n.kind() == "do_block" { inside_do_block = true; }
        if n.kind() == "call" && inside_do_block {
            if let Some(target) = n.child_by_field_name("target") {
                if common::node_text(target, source) == "def" {
                    return extract_handler_kind(&n, source);
                }
            }
        }
        current = n.parent();
    }
    None
}

/// 从 def 调用中提取 handler 类型（handle_call 或 handle_cast），如果匹配则返回
fn extract_handler_kind(def_node: &tree_sitter::Node, source: &[u8]) -> Option<String> {
    let args = find_child_by_kind(def_node, "arguments")?;
    for i in 0..args.child_count() {
        if let Some(child) = args.child(i) {
            let text = common::node_text(child, source);
            if text.starts_with("handle_call") { return Some("handle_call".into()); }
            if text.starts_with("handle_cast") { return Some("handle_cast".into()); }
        }
    }
    None
}

/// 从 Keyword.get 的参数文本中提取第二个参数位置的 :atom key
/// 例如 `(opts, :llm_backend)` → `llm_backend`
/// 只匹配第一个逗号后紧跟的 :atom，避免把第三个参数（默认值）误识别为 key
fn extract_keyword_atom_key(args_text: &str) -> Option<String> {
    // 从开头匹配：( + 第一个参数 + 第一个逗号 + :atom
    let re = Regex::new(r"^\s*\(?\s*[^,]+,\s*:([a-z_][a-z0-9_]*)").ok()?;
    re.captures(args_text)
        .and_then(|cap| cap.get(1).map(|m| m.as_str().to_string()))
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
    if trimmed.is_empty() || trimmed.starts_with('{') { return None; }
    let mut parts = trimmed.splitn(2, ',');
    let module_path = parts.next()?.trim();
    if !is_module_chain(module_path, false) { return None; }
    let default_alias = module_path.rsplit('.').next().unwrap_or("").to_string();
    let alias_name = match parts.next() {
        Some(rest) => match rest.trim().find("as:") {
            Some(idx) => {
                let alias: String = rest.trim()[idx + 3..].trim()
                    .chars().take_while(|ch| ch.is_ascii_alphanumeric() || *ch == '_').collect();
                if alias.is_empty() { default_alias } else { alias }
            }
            None => default_alias,
        },
        None => default_alias,
    };
    if alias_name.is_empty() { return None; }
    Some((alias_name, module_path.to_string()))
}

fn first_module_segment(module_path: &str) -> Option<&str> {
    module_path.split('.').next().filter(|seg| !seg.is_empty())
}

fn is_module_chain(value: &str, require_nested: bool) -> bool {
    let trimmed = value.trim();
    if trimmed.is_empty() { return false; }
    let segments: Vec<&str> = trimmed.split('.').collect();
    if require_nested && segments.len() < 2 { return false; }
    segments.iter().all(|seg| {
        let mut chars = seg.chars();
        chars.next().map_or(false, |c| c.is_ascii_uppercase())
            && chars.all(|ch| ch.is_ascii_alphanumeric() || ch == '_')
    })
}

fn has_register_namespace_conflict(
    lib_chain: &str, target_module: &str, alias_bindings: &HashMap<String, Vec<String>>,
) -> bool {
    let Some(lib_root) = first_module_segment(lib_chain) else { return true; };
    let Some(target_root) = first_module_segment(target_module) else { return true; };
    // 同根命名空间无法确定是外部注册还是内部模块调用，保守降级 direct_call
    if lib_root == target_root { return true; }
    let Some(resolved) = alias_bindings.get(lib_root) else { return false; };
    if resolved.len() != 1 { return true; }
    first_module_segment(&resolved[0]).map_or(true, |r| r == target_root)
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
        // via 字段应包含 key 名
        assert!(cb_hints.iter().any(|h| h.via.contains("llm_backend")));
        assert!(cb_hints.iter().any(|h| h.via.contains("compaction_strategy")));
    }

    #[test]
    fn detect_keyword_get_ignores_default_value_atom() {
        // Keyword.get(opts, key, :fallback) 的第三个参数不应被误识别为 key
        let source = r#"
defmodule MyApp.Session do
  def start_link(opts) do
    val = Keyword.get(opts, key, :fallback)
  end
end
"#;
        let record = extract(source, "lib/my_app/session.ex");
        let cb_hints: Vec<_> = record
            .relation_hints
            .iter()
            .filter(|h| h.call_type_hint == "callback_injection")
            .collect();
        // key 是变量而非 :atom，不应产生 hint；:fallback 是默认值，也不应产生 hint
        assert!(cb_hints.is_empty(), "should not extract default value atom as key, got: {:?}", cb_hints);
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

    #[test]
    fn walk_tree_skips_string_and_comment_for_all_detectors() {
        // 验证统一 skip 逻辑：字符串/注释/moduledoc 内的模式不应产生误报
        let source = r#"
defmodule MyApp.FalsePositive do
  @moduledoc """
  children = [MyApp.InDoc.Worker]
  Keyword.get(opts, :should_not_match)
  Lib.register(MyApp.InDoc.Target)
  def handle_call(:x, _f, s), do: MyApp.InDoc.Agent.call()
  """

  # children = [MyApp.InComment.Worker]
  # Keyword.get(opts, :should_not_match)

  def run do
    msg = "children = [MyApp.InString.Worker]"
    msg2 = "Keyword.get(opts, :in_string)"
    :ok
  end
end
"#;
        let record = extract(source, "lib/my_app/false_positive.ex");
        for hint in &record.relation_hints {
            assert!(
                !hint.target.contains("InDoc")
                    && !hint.target.contains("InComment")
                    && !hint.target.contains("InString")
                    && hint.target != "should_not_match"
                    && hint.target != "in_string",
                "skip logic failed: unexpected hint {:?}",
                hint
            );
        }
    }

    #[test]
    fn genserver_handle_call_nested_if_case_deep_do_block() {
        // 验证 handle_call 内嵌套 if/case 且包含多个 Module.func 调用时全部被提取
        let source = r#"
defmodule MyApp.Handler do
  use GenServer

  def handle_call({:process, data}, _from, state) do
    if data.valid? do
      result = MyApp.Validator.check(data)
      case result do
        :ok ->
          MyApp.Notifier.send(data)
          {:reply, :ok, state}
        :error ->
          MyApp.ErrorHandler.log(data)
          {:reply, :error, state}
      end
    else
      {:reply, :invalid, state}
    end
  end
end
"#;
        let record = extract(source, "lib/my_app/handler.ex");
        let gs_hints: Vec<_> = record
            .relation_hints
            .iter()
            .filter(|h| h.call_type_hint == "genserver_runtime_dep")
            .collect();
        let targets: Vec<&str> = gs_hints.iter().map(|h| h.target.as_str()).collect();
        assert!(targets.contains(&"MyApp.Validator"), "missing MyApp.Validator in nested if, got: {:?}", targets);
        assert!(targets.contains(&"MyApp.Notifier"), "missing MyApp.Notifier in nested case, got: {:?}", targets);
        assert!(targets.contains(&"MyApp.ErrorHandler"), "missing MyApp.ErrorHandler in nested case, got: {:?}", targets);
        assert!(gs_hints.iter().all(|h| h.via == "handle_call"));
    }

    // === TypeAnnotation / TypeGuard 提取测试 ===

    #[test]
    fn extract_spec_type_annotations() {
        let source = r#"
defmodule MyApp.Foo do
  @spec process(map()) :: term()
  def process(data) do
    data
  end
end
"#;
        let record = extract(source, "lib/my_app/foo.ex");
        assert_eq!(record.type_annotations.len(), 1);
        let ann = &record.type_annotations[0];
        assert_eq!(ann.func_name, "process");
        assert_eq!(ann.param, "data");
        assert_eq!(ann.type_expr, "map()");
    }

    #[test]
    fn extract_spec_multiple_params() {
        let source = r#"
defmodule MyApp.Foo do
  @spec convert(integer(), binary()) :: atom()
  def convert(num, text) do
    :ok
  end
end
"#;
        let record = extract(source, "lib/my_app/foo.ex");
        assert_eq!(record.type_annotations.len(), 2);
        assert_eq!(record.type_annotations[0].param, "num");
        assert_eq!(record.type_annotations[0].type_expr, "integer()");
        assert_eq!(record.type_annotations[1].param, "text");
        assert_eq!(record.type_annotations[1].type_expr, "binary()");
    }

    #[test]
    fn extract_when_guard_type_guard() {
        let source = r#"
defmodule MyApp.Foo do
  @spec process(map()) :: term()
  def process(data) when is_map(data) do
    data
  end
end
"#;
        let record = extract(source, "lib/my_app/foo.ex");
        assert_eq!(record.type_guards.len(), 1);
        let guard = &record.type_guards[0];
        assert_eq!(guard.function, "process");
        assert_eq!(guard.var, "data");
        assert_eq!(guard.guarded_type, "map()");
    }

    #[test]
    fn extract_multiple_when_guards() {
        let source = r#"
defmodule MyApp.Foo do
  def check(x, y) when is_integer(x) and is_binary(y) do
    :ok
  end
end
"#;
        let record = extract(source, "lib/my_app/foo.ex");
        assert_eq!(record.type_guards.len(), 2, "got: {:?}", record.type_guards);
        let vars: Vec<&str> = record.type_guards.iter().map(|g| g.var.as_str()).collect();
        assert!(vars.contains(&"x"));
        assert!(vars.contains(&"y"));
    }

    #[test]
    fn no_spec_no_guard_no_extraction() {
        let source = r#"
defmodule MyApp.Foo do
  def run do
    :ok
  end
end
"#;
        let record = extract(source, "lib/my_app/foo.ex");
        assert!(record.type_annotations.is_empty());
        assert!(record.type_guards.is_empty());
    }

    #[test]
    fn parse_spec_params_basic() {
        assert_eq!(parse_spec_params("foo(map()) :: term()"), vec!["map()"]);
        assert_eq!(
            parse_spec_params("bar(integer(), binary()) :: atom()"),
            vec!["integer()", "binary()"]
        );
        assert!(parse_spec_params("baz() :: :ok").is_empty());
    }

    #[test]
    fn parse_spec_params_nested_generics() {
        // 嵌套泛型：list(map()) 应作为单个参数
        assert_eq!(
            parse_spec_params("foo(list(map()), integer()) :: term()"),
            vec!["list(map())", "integer()"]
        );
        // 深层嵌套
        assert_eq!(
            parse_spec_params("foo({atom(), list(tuple())}) :: :ok"),
            vec!["{atom(), list(tuple())}"]
        );
    }

    #[test]
    fn parse_spec_params_no_parens() {
        // 缺少括号的非法 spec 不应 panic
        assert!(parse_spec_params("foo :: term()").is_empty());
    }

    #[test]
    fn parse_spec_params_unmatched_parens() {
        // 不匹配的括号应安全返回空
        assert!(parse_spec_params("foo(map(").is_empty());
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
