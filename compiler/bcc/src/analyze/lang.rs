//! 语言节点映射配置
//!
//! 为 tree-sitter 检测器提供语言无关的节点类型查询，
//! 避免在每个检测器中硬编码 Python 节点类型名。

use tree_sitter::Node;

/// 语言配置：映射各语言 tree-sitter 节点类型到通用概念
pub struct LangConfig {
    /// tree-sitter Language 构造器
    pub ts_language: fn() -> tree_sitter::Language,
    /// 代码块节点类型（Python: block, Rust: block, TS: statement_block）
    pub block_kinds: &'static [&'static str],
    /// return 语句节点类型
    pub return_kinds: &'static [&'static str],
    /// raise/throw 语句节点类型
    pub raise_kinds: &'static [&'static str],
    /// if 语句节点类型
    pub if_kind: &'static str,
    /// if 条件的 field 名（大多数语言是 "condition"）
    pub condition_field: &'static str,
    /// 假值字面量文本
    pub false_literals: &'static [&'static str],
    /// normalize_ast 中视为标识符的节点类型
    pub identifier_kinds: &'static [&'static str],
    /// normalize_ast 中视为字符串的节点类型
    pub string_kinds: &'static [&'static str],
    /// normalize_ast 中视为数字的节点类型
    pub number_kinds: &'static [&'static str],
    /// build_skeleton 中视为原子的复合节点类型（不展开子节点）
    pub atomic_kinds: &'static [&'static str],
    /// 注释前缀（用于去除注释标记提取纯文本）
    pub comment_prefix: &'static str,
    /// 空函数体判定：body 节点中最大允许的 named child 数（超过则非空）
    /// Python 空函数有 `pass` (1 named child)，Rust 空函数有 0 个
    pub empty_body_max_children: usize,
    /// 空函数体中允许的"空语句"类型（Python: expression_statement(string=docstring), Rust: 无）
    pub empty_body_allowed_kinds: &'static [&'static str],
}

/// 函数节点识别结果
pub struct FuncMatch<'a> {
    /// 函数名
    pub name: String,
    /// 函数体节点
    pub body: Node<'a>,
}

/// 获取指定语言的配置，不支持的语言返回 None
pub fn lang_config(language: &str) -> Option<&'static LangConfig> {
    match language {
        "python" => Some(&PYTHON),
        "elixir" => Some(&ELIXIR),
        "rust" => Some(&RUST),
        "typescript" | "tsx" => Some(&TYPESCRIPT),
        _ => None,
    }
}

/// 判断节点是否为函数定义，并提取函数名和函数体
pub fn match_function<'a>(node: Node<'a>, language: &str, source: &str) -> Option<FuncMatch<'a>> {
    match language {
        "python" => {
            if node.kind() == "function_definition" {
                let name = node
                    .child_by_field_name("name")
                    .map(|n| source[n.byte_range()].to_string())
                    .unwrap_or_else(|| "<anonymous>".to_string());
                let body = node.child_by_field_name("body")?;
                Some(FuncMatch { name, body })
            } else {
                None
            }
        }
        "elixir" => {
            // Elixir: def/defp 是 call 节点，函数名是第二个 child
            if node.kind() == "call" {
                let callee = node.child(0)?;
                if callee.kind() == "identifier" {
                    let callee_name = &source[callee.byte_range()];
                    if matches!(callee_name, "def" | "defp" | "defmacro" | "defmacrop") {
                        // 函数名：第二个 child（可能是 call 或 identifier）
                        let name_node = node.child(1)?;
                        let name = if name_node.kind() == "call" {
                            // def foo(args) — foo 是 call 的第一个 child
                            name_node
                                .child(0)
                                .map(|n| source[n.byte_range()].to_string())
                                .unwrap_or_else(|| "<anonymous>".to_string())
                        } else {
                            source[name_node.byte_range()].to_string()
                        };
                        // 函数体：do...end block
                        let body = find_child_by_kind(node, "do_block")?;
                        return Some(FuncMatch { name, body });
                    }
                }
            }
            None
        }
        "rust" => {
            if node.kind() == "function_item" {
                let name = node
                    .child_by_field_name("name")
                    .map(|n| source[n.byte_range()].to_string())
                    .unwrap_or_else(|| "<anonymous>".to_string());
                let body = node.child_by_field_name("body")?;
                Some(FuncMatch { name, body })
            } else {
                None
            }
        }
        "typescript" | "tsx" => {
            if matches!(
                node.kind(),
                "function_declaration" | "method_definition" | "arrow_function"
            ) {
                let name = node
                    .child_by_field_name("name")
                    .map(|n| source[n.byte_range()].to_string())
                    .unwrap_or_else(|| "<anonymous>".to_string());
                let body = node.child_by_field_name("body")?;
                Some(FuncMatch { name, body })
            } else {
                None
            }
        }
        _ => None,
    }
}

/// 辅助：按 kind 查找子节点
fn find_child_by_kind<'a>(node: Node<'a>, kind: &str) -> Option<Node<'a>> {
    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            if child.kind() == kind {
                return Some(child);
            }
        }
    }
    None
}

// === 各语言配置 ===

static PYTHON: LangConfig = LangConfig {
    ts_language: || tree_sitter_python::LANGUAGE.into(),
    block_kinds: &["block"],
    return_kinds: &["return_statement"],
    raise_kinds: &["raise_statement"],
    if_kind: "if_statement",
    condition_field: "condition",
    false_literals: &["False", "0", "false"],
    identifier_kinds: &["identifier"],
    string_kinds: &["string", "concatenated_string"],
    number_kinds: &["integer", "float"],
    atomic_kinds: &[
        "argument_list", "parameters", "string", "concatenated_string",
        "tuple", "list", "dictionary", "set",
    ],
    comment_prefix: "#",
    empty_body_max_children: 1,
    empty_body_allowed_kinds: &["expression_statement", "pass_statement"],
};

static ELIXIR: LangConfig = LangConfig {
    ts_language: || tree_sitter_elixir::LANGUAGE.into(),
    block_kinds: &["do_block", "body"],
    // Elixir 没有显式 return 关键字（极少使用）
    return_kinds: &[],
    raise_kinds: &[], // raise 是普通 call 节点，无法单靠 kind 区分，跳过 unreachable 检测
    if_kind: "call", // if 也是 call 节点
    condition_field: "",
    false_literals: &["false", "nil"],
    identifier_kinds: &["identifier", "atom"],
    string_kinds: &["string", "charlist"],
    number_kinds: &["integer", "float"],
    atomic_kinds: &[
        "arguments", "string", "charlist", "tuple", "list", "map",
        "sigil", "atom",
    ],
    comment_prefix: "#",
    empty_body_max_children: 0,
    empty_body_allowed_kinds: &[],
};

static RUST: LangConfig = LangConfig {
    ts_language: || tree_sitter_rust::LANGUAGE.into(),
    block_kinds: &["block"],
    return_kinds: &["return_expression"],
    raise_kinds: &[], // panic!/todo! 等由 rustc 自身警告，bcc 不重复检测
    if_kind: "if_expression",
    condition_field: "condition",
    false_literals: &["false"],
    identifier_kinds: &["identifier"],
    string_kinds: &["string_literal", "raw_string_literal"],
    number_kinds: &["integer_literal", "float_literal"],
    atomic_kinds: &[
        "arguments", "parameters", "string_literal", "raw_string_literal",
        "tuple_expression", "array_expression", "struct_expression",
    ],
    comment_prefix: "//",
    empty_body_max_children: 1,
    empty_body_allowed_kinds: &["expression_statement"],
};

static TYPESCRIPT: LangConfig = LangConfig {
    ts_language: || tree_sitter_typescript::LANGUAGE_TYPESCRIPT.into(),
    block_kinds: &["statement_block"],
    return_kinds: &["return_statement"],
    raise_kinds: &["throw_statement"],
    if_kind: "if_statement",
    condition_field: "condition",
    false_literals: &["false", "0"],
    identifier_kinds: &["identifier"],
    string_kinds: &["string", "template_string"],
    number_kinds: &["number"],
    atomic_kinds: &[
        "arguments", "formal_parameters", "string", "template_string",
        "array", "object",
    ],
    comment_prefix: "//",
    empty_body_max_children: 0,
    empty_body_allowed_kinds: &[],
};
