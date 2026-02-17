use super::common;
use super::*;

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

    FileRecord {
        language: "elixir".into(),
        file_path: path.to_string(),
        module_doc,
        exports,
        imports,
        calls,
        local_call_targets: Vec::new(),
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
