use super::common;
use super::*;
use tree_sitter::Parser;

/// 使用 tree-sitter-php 解析 PHP 源码并提取结构信息
pub fn extract(content: &str, path: &str) -> FileRecord {
    let mut parser = Parser::new();
    let language = tree_sitter_php::LANGUAGE_PHP;
    parser
        .set_language(&language.into())
        .expect("failed to set php grammar");

    let tree = parser
        .parse(content, None)
        .expect("failed to parse php source");
    let root = tree.root_node();
    let source = content.as_bytes();

    let mut exports = Vec::new();
    let mut imports = Vec::new();
    let mut calls = Vec::new();
    let mut declarations: usize = 0;
    let mut class_name: Option<String> = None;

    let mut cursor = root.walk();
    extract_php_recursive(
        &mut cursor,
        source,
        &mut exports,
        &mut imports,
        &mut calls,
        &mut declarations,
        &mut class_name,
    );

    common::dedup_calls_by_callee(&mut calls);

    // PHP 副作用检测（关键词扫描）
    let side_effects = SideEffects {
        has_async: content.contains("dispatch(") || content.contains("Queue::"),
        has_http: content.contains("curl_")
            || content.contains("file_get_contents(")
            || content.contains("Guzzle")
            || content.contains("Http::"),
        has_genserver: false,
        has_file_io: content.contains("fopen(")
            || content.contains("file_put_contents(")
            || content.contains("fwrite(")
            || content.contains("unlink("),
        has_pubsub: content.contains("event(") || content.contains("Event::"),
    };

    // module_doc: 用 class 名标识
    let module_doc = class_name.clone();

    FileRecord {
        language: "php".into(),
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

fn extract_php_recursive(
    cursor: &mut tree_sitter::TreeCursor,
    source: &[u8],
    exports: &mut Vec<ExportRecord>,
    imports: &mut Vec<ImportRecord>,
    calls: &mut Vec<CallRecord>,
    declarations: &mut usize,
    class_name: &mut Option<String>,
) {
    loop {
        let node = cursor.node();
        let kind = node.kind();

        match kind {
            // class 声明：提取类名、继承、trait
            "class_declaration" => {
                *declarations += 1;
                // 提取类名
                if let Some(name_node) = node.child_by_field_name("name") {
                    let name = common::node_text(name_node, source);
                    *class_name = Some(name.clone());

                    // 提取 extends（base_clause 是子节点类型，非命名字段）
                    for i in 0..node.child_count() {
                        if let Some(child) = node.child(i) {
                            if child.kind() == "base_clause" {
                                // base_clause 内部有 name 节点
                                for j in 0..child.child_count() {
                                    if let Some(name_child) = child.child(j) {
                                        if name_child.kind() == "name" {
                                            let parent_name = common::node_text(name_child, source);
                                            if !parent_name.is_empty() {
                                                imports.push(ImportRecord {
                                                    specifier: parent_name,
                                                    kind: "extends".into(),
                                                });
                                            }
                                        }
                                    }
                                }
                            }
                        }
                    }
                }

                // 递归进入 class body
                if cursor.goto_first_child() {
                    extract_php_recursive(
                        cursor,
                        source,
                        exports,
                        imports,
                        calls,
                        declarations,
                        class_name,
                    );
                    cursor.goto_parent();
                }
            }

            // use trait 语句
            "use_declaration" => {
                // use Trait1, Trait2;
                let text = common::node_text(node, source);
                let text = text
                    .trim_start_matches("use")
                    .trim()
                    .trim_end_matches(';')
                    .trim();
                for trait_name in text.split(',') {
                    let t = trait_name.trim();
                    if !t.is_empty() && t.chars().next().map_or(false, |c| c.is_uppercase()) {
                        imports.push(ImportRecord {
                            specifier: t.to_string(),
                            kind: "trait".into(),
                        });
                    }
                }
            }

            // method 声明
            "method_declaration" => {
                *declarations += 1;
                let visibility = get_method_visibility(&node, source);
                if let Some(name_node) = node.child_by_field_name("name") {
                    let name = common::node_text(name_node, source);
                    let line = node.start_position().row + 1;

                    // 只有 public 方法进 exports
                    if visibility == "public" {
                        // 构造 signature
                        let params = node
                            .child_by_field_name("parameters")
                            .map(|p| common::node_text(p, source))
                            .unwrap_or_default();
                        let signature = format!("public function {}{}", name, params);

                        exports.push(ExportRecord {
                            name,
                            kind: "method".into(),
                            signature: Some(signature),
                            line,
                        });
                    }
                }

                // 递归进入 method body 提取 calls
                if cursor.goto_first_child() {
                    extract_php_recursive(
                        cursor,
                        source,
                        exports,
                        imports,
                        calls,
                        declarations,
                        class_name,
                    );
                    cursor.goto_parent();
                }
            }

            // 静态方法调用: Class::method(...)
            "scoped_call_expression" => {
                if let Some(scope) = node.child_by_field_name("scope") {
                    let callee = common::node_text(scope, source);
                    // 过滤 $this、self、parent、static
                    if !callee.starts_with('$')
                        && callee != "self"
                        && callee != "parent"
                        && callee != "static"
                    {
                        let line = node.start_position().row + 1;
                        calls.push(CallRecord { callee, line });
                    }
                }
                // 继续递归
                if cursor.goto_first_child() {
                    extract_php_recursive(
                        cursor,
                        source,
                        exports,
                        imports,
                        calls,
                        declarations,
                        class_name,
                    );
                    cursor.goto_parent();
                }
            }

            // 普通函数调用（非方法调用）— 可能是全局函数或 new Class
            "object_creation_expression" => {
                // new SomeClass(...)
                if let Some(class_node) = node.child_by_field_name("class") {
                    let callee = common::node_text(class_node, source);
                    if callee.chars().next().map_or(false, |c| c.is_uppercase()) {
                        let line = node.start_position().row + 1;
                        calls.push(CallRecord { callee, line });
                    }
                }
                if cursor.goto_first_child() {
                    extract_php_recursive(
                        cursor,
                        source,
                        exports,
                        imports,
                        calls,
                        declarations,
                        class_name,
                    );
                    cursor.goto_parent();
                }
            }

            // 其他节点：递归遍历子节点
            _ => {
                if cursor.goto_first_child() {
                    extract_php_recursive(
                        cursor,
                        source,
                        exports,
                        imports,
                        calls,
                        declarations,
                        class_name,
                    );
                    cursor.goto_parent();
                }
            }
        }

        if !cursor.goto_next_sibling() {
            break;
        }
    }
}

/// 获取方法的可见性修饰符
fn get_method_visibility(node: &tree_sitter::Node, source: &[u8]) -> String {
    // method_declaration 的子节点中查找 visibility_modifier
    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            if child.kind() == "visibility_modifier" {
                return common::node_text(child, source).to_lowercase();
            }
        }
    }
    // PHP 默认 public
    "public".into()
}
