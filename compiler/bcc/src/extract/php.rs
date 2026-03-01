use super::common;
use super::*;

/// 使用 tree-sitter-php 解析 PHP 源码并提取结构信息
pub fn extract(content: &str, path: &str) -> FileRecord {
    let tree = common::parse_tree(content, tree_sitter_php::LANGUAGE_PHP, "php");
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

    // 提取 class 属性 → SchemaField
    let mut schema_fields = Vec::new();
    extract_php_class_properties(root, source, &mut schema_fields);

    FileRecord {
        language: "php".into(),
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
        type_annotations: vec![],
        type_guards: vec![],
        schema_fields,
        source_code: Some(content.to_string()),
    }
}

/// 从 PHP class 中提取属性 → SchemaField
fn extract_php_class_properties(
    root: tree_sitter::Node,
    source: &[u8],
    schema_fields: &mut Vec<SchemaField>,
) {
    let mut stack = vec![root];
    while let Some(node) = stack.pop() {
        if node.kind() == "property_declaration" {
            // property_declaration 包含 visibility + type? + property_element
            let mut prop_type = String::new();
            for i in 0..node.child_count() {
                if let Some(child) = node.child(i) {
                    // 类型标注（如 ?string, int 等）
                    if matches!(
                        child.kind(),
                        "union_type"
                            | "named_type"
                            | "primitive_type"
                            | "nullable_type"
                            | "optional_type"
                    ) {
                        prop_type = common::node_text(child, source);
                    }
                    if child.kind() == "property_element" {
                        // 提取属性名
                        if let Some(name_node) = child.child_by_field_name("name") {
                            let name = common::node_text(name_node, source)
                                .trim_start_matches('$')
                                .to_string();
                            // 每个 property_element 独立检查是否有默认值
                            // tree-sitter-php 可能用 default_value field 或 "=" 后跟值
                            // tree-sitter-php 中 property_element 的默认值可能通过 value/default_value field 或 "=" 子节点表示
                            let has_default = child.child_by_field_name("value").is_some()
                                || child.child_by_field_name("default_value").is_some()
                                || (0..child.child_count())
                                    .any(|k| child.child(k).map_or(false, |c| c.kind() == "="));
                            let is_nullable = prop_type.starts_with('?');
                            if !name.is_empty() {
                                schema_fields.push(SchemaField {
                                    name,
                                    field_type: prop_type.clone(),
                                    line: child.start_position().row + 1,
                                    required: !has_default && !is_nullable,
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

            // 文件级 namespace use：use A\B; use C\D as E;
            "namespace_use_declaration" => {
                extract_namespace_use_declaration(node, source, imports);
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
                        let args = extract_php_call_args(node, source);
                        calls.push(CallRecord { callee, line, args });
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
                let class_node = node.child_by_field_name("class").or_else(|| {
                    // tree-sitter-php 不同版本下 object_creation_expression 的 field 可能不一致，
                    // 兜底按子节点 kind 提取类名节点。
                    for i in 0..node.child_count() {
                        if let Some(child) = node.child(i) {
                            let kind = child.kind();
                            if kind == "name" || kind == "qualified_name" {
                                return Some(child);
                            }
                        }
                    }
                    None
                });

                if let Some(class_node) = class_node {
                    let callee = common::node_text(class_node, source);
                    if callee.chars().next().map_or(false, |c| c.is_uppercase()) {
                        let line = node.start_position().row + 1;
                        let args = extract_php_call_args(node, source);
                        calls.push(CallRecord { callee, line, args });
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

/// 提取调用参数的文本列表（用于 detect_unnecessary_default 等规则匹配）
fn extract_php_call_args(call_node: tree_sitter::Node, source: &[u8]) -> Vec<String> {
    let mut args = Vec::new();
    if let Some(args_node) = call_node.child_by_field_name("arguments") {
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

fn extract_namespace_use_declaration(
    node: tree_sitter::Node,
    source: &[u8],
    imports: &mut Vec<ImportRecord>,
) {
    let mut prefix: Option<String> = None;
    let mut clauses: Vec<tree_sitter::Node> = Vec::new();

    for i in 0..node.child_count() {
        let Some(child) = node.child(i) else { continue };
        if !child.is_named() {
            continue;
        }

        match child.kind() {
            "namespace_name" => {
                let p = common::node_text(child, source);
                if !p.is_empty() {
                    prefix = Some(p);
                }
            }
            "namespace_use_clause" => clauses.push(child),
            "namespace_use_group" => {
                for j in 0..child.child_count() {
                    let Some(grand_child) = child.child(j) else {
                        continue;
                    };
                    if grand_child.is_named() && grand_child.kind() == "namespace_use_clause" {
                        clauses.push(grand_child);
                    }
                }
            }
            _ => {}
        }
    }

    for clause in clauses {
        if let Some(specifier) = format_namespace_use_clause(clause, source, prefix.as_deref()) {
            imports.push(ImportRecord {
                specifier,
                kind: "use".into(),
            });
        }
    }
}

fn format_namespace_use_clause(
    clause: tree_sitter::Node,
    source: &[u8],
    prefix: Option<&str>,
) -> Option<String> {
    let mut target: Option<String> = None;
    for i in 0..clause.child_count() {
        let Some(child) = clause.child(i) else {
            continue;
        };
        if !child.is_named() {
            continue;
        }
        if child.kind() == "name" || child.kind() == "qualified_name" {
            target = Some(common::node_text(child, source));
            break;
        }
    }

    let mut full = target?;
    if let Some(base) = prefix {
        if !base.is_empty() {
            full = format!("{}\\{}", base.trim_matches('\\'), full.trim_matches('\\'));
        }
    }
    full = full.trim().trim_matches('\\').to_string();
    if full.is_empty() {
        return None;
    }

    let alias = clause
        .child_by_field_name("alias")
        .map(|n| common::node_text(n, source))
        .unwrap_or_default()
        .trim()
        .to_string();

    if alias.is_empty() {
        Some(full)
    } else {
        Some(format!("{} as {}", full, alias))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extract::testing;

    #[test]
    fn extracts_public_method_and_ignores_private_method() {
        let source = r#"<?php
class UserController {
    public function save($name) { return $name; }
    private function hidden() { return null; }
}
"#;

        let record = extract(source, "app/controllers/UserController.php");
        let exports = testing::export_names(&record);
        testing::assert_contains(&exports, "save", "exports");
        assert!(!exports.iter().any(|name| name == "hidden"));
        assert_eq!(record.module_doc.as_deref(), Some("UserController"));
    }

    #[test]
    fn extracts_extends_and_trait_imports() {
        let source = r#"<?php
class UserController extends BaseController {
    use LoggerTrait;
    public function save($name) { return $name; }
}
"#;

        let record = extract(source, "app/controllers/UserController.php");
        testing::assert_import_contains(&record, "BaseController", "extends");
        testing::assert_import_contains(&record, "LoggerTrait", "trait");
    }

    #[test]
    fn extracts_static_and_object_creation_calls_with_dedup() {
        let source = r#"<?php
class UserController {
    public function save($id) {
        Service::run($id);
        Service::run($id);
        $worker = new Worker();
    }
}
"#;

        let record = extract(source, "app/controllers/UserController.php");
        let calls = testing::call_names(&record);
        testing::assert_contains(&calls, "Service", "calls");
        testing::assert_contains(&calls, "Worker", "calls");
        assert_eq!(
            calls.iter().filter(|callee| *callee == "Service").count(),
            1
        );
    }

    #[test]
    fn extracts_namespace_use_and_keeps_extends_trait_and_new_behavior() {
        let source = r#"<?php
use A\B;
use C\D as E;

class X extends Base {
    use LoggerTrait;

    public function f() {
        $v = new E();
        return $v;
    }
}
"#;

        let record = extract(source, "app/controllers/X.php");
        testing::assert_import_contains(&record, "A\\B", "use");
        testing::assert_import_contains(&record, "C\\D as E", "use");
        testing::assert_import_contains(&record, "Base", "extends");
        testing::assert_import_contains(&record, "LoggerTrait", "trait");

        let calls = testing::call_names(&record);
        testing::assert_contains(&calls, "E", "calls");
    }

    #[test]
    fn multi_property_element_has_default_independent() {
        // 同一 property_declaration 中多个 property_element，
        // 每个的 has_default 应独立判定
        let source = r#"<?php
class Config {
    public string $a = 'default', $b;
}
"#;
        let record = extract(source, "app/Config.php");
        let a = record
            .schema_fields
            .iter()
            .find(|f| f.name == "a")
            .expect("should find $a");
        let b = record
            .schema_fields
            .iter()
            .find(|f| f.name == "b")
            .expect("should find $b");
        // $a 有默认值 → required=false
        assert!(!a.required, "$a has default, should not be required");
        // $b 无默认值 → required=true
        assert!(b.required, "$b has no default, should be required");
    }
}
