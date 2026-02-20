use super::common;
use super::*;
use regex::Regex;
use std::collections::{HashMap, HashSet};
use std::env;
use std::path::Path;
use std::path::PathBuf;

/// 使用 tree-sitter-typescript 解析源码并提取结构信息
pub fn extract(content: &str, path: &str, lang: &str) -> FileRecord {
    let language = if lang == "tsx" {
        tree_sitter_typescript::LANGUAGE_TSX
    } else {
        tree_sitter_typescript::LANGUAGE_TYPESCRIPT
    };
    let tree = common::parse_tree(content, language, "typescript");
    let root = tree.root_node();
    let source = content.as_bytes();

    let mut exports = Vec::new();
    let mut imports = Vec::new();
    let mut calls = Vec::new();
    let mut local_call_symbols: HashSet<String> = HashSet::new();
    let mut import_aliases: HashMap<String, String> = HashMap::new();
    let mut import_alias_sources: HashMap<String, HashSet<String>> = HashMap::new();
    let mut declarations: usize = 0;

    let mut cursor = root.walk();
    extract_ts_recursive(
        &mut cursor,
        source,
        &mut exports,
        &mut imports,
        &mut import_aliases,
        &mut import_alias_sources,
        &mut calls,
        &mut local_call_symbols,
        &mut declarations,
    );

    common::dedup_sort_calls(&mut calls);
    local_call_symbols.clear();
    for call in &calls {
        local_call_symbols.insert(call.callee.clone());
    }

    // 收集本地函数定义（用于本地函数调用）
    let mut local_functions: HashSet<String> = HashSet::new();
    for export in &exports {
        local_functions.insert(export.name.clone());
    }

    // 从 declarations 中提取更多本地函数名（简化处理）
    // 实际应该从 AST 中提取所有函数定义

    let mut local_call_targets_set = HashSet::new();
    for symbol in &local_call_symbols {
        // 1. 检查是否是 import 的函数
        if let Some(specifier) = import_aliases.get(symbol) {
            if let Some(target) = resolve_local_import(path, specifier) {
                local_call_targets_set.insert(target);
            }
        }
        // 2. 检查是否是本地函数（同一文件内的函数）
        else if local_functions.contains(symbol) {
            // 本地函数调用，目标就是当前文件
            local_call_targets_set.insert(path.to_string());
        }
        // 3. 尝试解析为相对路径导入
        else if symbol.contains('/') || symbol.starts_with('.') {
            if let Some(target) = resolve_local_import(path, symbol) {
                local_call_targets_set.insert(target);
            }
        }
    }
    let mut local_call_targets: Vec<String> = local_call_targets_set.into_iter().collect();
    local_call_targets.sort_unstable();
    let relation_hints = detect_relation_hints(root, source, &import_alias_sources);

    let side_effects = SideEffects {
        has_async: content.contains("async ") || content.contains("await "),
        has_http: content.contains("fetch(")
            || content.contains("axios")
            || content.contains("ky("),
        has_genserver: false,
        has_file_io: content.contains("fs.")
            || content.contains("readFile")
            || content.contains("writeFile"),
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
        relation_hints,
        side_effects,
        loc_lines: content.lines().count(),
        declarations,
        type_annotations: vec![],
        type_guards: vec![],
        schema_fields: vec![],
    }
}

fn extract_ts_recursive(
    cursor: &mut tree_sitter::TreeCursor,
    source: &[u8],
    exports: &mut Vec<ExportRecord>,
    imports: &mut Vec<ImportRecord>,
    import_aliases: &mut HashMap<String, String>,
    import_alias_sources: &mut HashMap<String, HashSet<String>>,
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
                                        name: common::node_text(name_node, source),
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
                                        name: common::node_text(name_node, source),
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
                                            if let Some(name_node) =
                                                decl_child.child_by_field_name("name")
                                            {
                                                exports.push(ExportRecord {
                                                    name: common::node_text(name_node, source),
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
                                        name: common::node_text(name_node, source),
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
                                        name: common::node_text(name_node, source),
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
                                        name: common::node_text(name_node, source),
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

                // 修复：export 子树内同样需要提取 call_expression（函数体、类方法、对象方法等）。
                collect_ts_calls_in_subtree(node, source, calls, local_call_symbols);
            }
            // import_statement
            "import_statement" => {
                if let Some(source_node) = node.child_by_field_name("source") {
                    let spec = common::node_text(source_node, source)
                        .trim_matches(|c: char| c == '\'' || c == '"')
                        .to_string();
                    if !spec.is_empty() {
                        let import_text = common::node_text(node, source);
                        let aliases = extract_import_aliases(&import_text);
                        for alias in aliases {
                            import_aliases.insert(alias.clone(), spec.clone());
                            import_alias_sources
                                .entry(alias)
                                .or_default()
                                .insert(spec.clone());
                        }
                        imports.push(ImportRecord {
                            specifier: spec,
                            kind: "import".into(),
                        });
                    }
                }
            }
            // 非导出的声明也计入 declarations
            "function_declaration"
            | "class_declaration"
            | "interface_declaration"
            | "type_alias_declaration"
            | "enum_declaration" => {
                // 只有不在 export_statement 内的才独立计数
                if node
                    .parent()
                    .map_or(true, |p| p.kind() != "export_statement")
                {
                    *declarations += 1;
                }
            }
            "lexical_declaration" => {
                if node
                    .parent()
                    .map_or(true, |p| p.kind() != "export_statement")
                {
                    *declarations += 1;
                }
            }
            // 模块调用检测: 以 call_expression 根调用符作为本地调用候选
            "call_expression" => {
                if let Some(func_node) = node.child_by_field_name("function") {
                    // 提取各种调用类型
                    let callees = extract_call_targets(func_node, source);
                    for callee in callees {
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
                    import_alias_sources,
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

fn collect_ts_calls_in_subtree(
    node: tree_sitter::Node,
    source: &[u8],
    calls: &mut Vec<CallRecord>,
    local_call_symbols: &mut HashSet<String>,
) {
    if node.kind() == "call_expression" {
        if let Some(func_node) = node.child_by_field_name("function") {
            let callees = extract_call_targets(func_node, source);
            for callee in callees {
                local_call_symbols.insert(callee.clone());
                calls.push(CallRecord {
                    callee,
                    line: node.start_position().row + 1,
                });
            }
        }
    }

    let mut cursor = node.walk();
    if cursor.goto_first_child() {
        loop {
            collect_ts_calls_in_subtree(cursor.node(), source, calls, local_call_symbols);
            if !cursor.goto_next_sibling() {
                break;
            }
        }
    }
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
    let normalized_spec = if specifier.ends_with(".ts")
        || specifier.ends_with(".tsx")
        || specifier.ends_with(".cts")
        || specifier.ends_with(".mts")
    {
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

        let absolute = candidate
            .canonicalize()
            .unwrap_or_else(|_| candidate.to_path_buf());
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

/// 提取所有调用目标（包括方法调用、属性访问等）
fn extract_call_targets(node: tree_sitter::Node, source: &[u8]) -> Vec<String> {
    let mut targets = Vec::new();

    match node.kind() {
        // 简单标识符调用: foo()
        "identifier" => {
            if let Some(name) = node.utf8_text(source).ok() {
                targets.push(name.to_string());
            }
        }
        // 属性访问调用: obj.method() 或 this.method()
        "member_expression" => {
            // 提取完整的属性链: obj.method 或 this.method
            if let Some(text) = extract_member_chain(node, source) {
                targets.push(text);
            }
            // 同时提取方法名本身
            if let Some(prop) = node.child_by_field_name("property") {
                if let Some(name) = prop.utf8_text(source).ok() {
                    targets.push(name.to_string());
                }
            }
        }
        // 带可选链的调用: obj?.method()
        "call_expression" => {
            // 递归处理嵌套调用
            if let Some(func) = node.child_by_field_name("function") {
                targets.extend(extract_call_targets(func, source));
            }
        }
        // 其他情况使用原来的提取逻辑
        _ => {
            if let Some(callee) = extract_call_root(node, source) {
                targets.push(callee);
            }
        }
    }

    targets
}

/// 提取属性访问链: obj.prop1.prop2 -> "obj.prop1.prop2"
fn extract_member_chain(node: tree_sitter::Node, source: &[u8]) -> Option<String> {
    let mut parts = Vec::new();
    let mut current = node;

    loop {
        match current.kind() {
            "member_expression" => {
                if let Some(prop) = current.child_by_field_name("property") {
                    if let Ok(name) = prop.utf8_text(source) {
                        parts.push(name.to_string());
                    }
                }
                if let Some(obj) = current.child_by_field_name("object") {
                    current = obj;
                } else {
                    break;
                }
            }
            "identifier" | "this" | "super" => {
                if let Ok(name) = current.utf8_text(source) {
                    parts.push(name.to_string());
                }
                break;
            }
            _ => break,
        }
    }

    if parts.is_empty() {
        None
    } else {
        parts.reverse();
        Some(parts.join("."))
    }
}

fn extract_call_root(node: tree_sitter::Node, source: &[u8]) -> Option<String> {
    match node.kind() {
        "identifier" => Some(common::node_text(node, source)),
        "member_expression" => {
            let mut current = node;
            loop {
                match current.child_by_field_name("object") {
                    Some(obj) if obj.kind() == "identifier" => {
                        return Some(common::node_text(obj, source))
                    }
                    Some(obj) if obj.kind() == "member_expression" => current = obj,
                    Some(_) => return None,
                    None => return None,
                }
            }
        }
        _ => None,
    }
}

fn detect_relation_hints(
    root: tree_sitter::Node,
    source: &[u8],
    import_alias_sources: &HashMap<String, HashSet<String>>,
) -> Vec<RelationHintRecord> {
    let mut hints = Vec::new();
    let sanitized_content = mask_non_code_regions_for_hint_detection(root, source);
    let module_re = Regex::new(r"(?s)@Module\s*\(\s*\{(.*?)\}\s*\)").expect("valid module regex");
    let module_field_re = Regex::new(r"(imports|providers|controllers)\s*:\s*\[([^\]]*)\]")
        .expect("valid module field regex");
    let symbol_re =
        Regex::new(r"[A-Za-z_][A-Za-z0-9_]*").expect("valid ts symbol extraction regex");

    for cap in module_re.captures_iter(&sanitized_content) {
        let Some(body) = cap.get(1).map(|m| m.as_str()) else {
            continue;
        };
        for field_cap in module_field_re.captures_iter(body) {
            let Some(field) = field_cap.get(1).map(|m| m.as_str()) else {
                continue;
            };
            let Some(items) = field_cap.get(2).map(|m| m.as_str()) else {
                continue;
            };
            let confidence = match field {
                "imports" | "providers" => 0.95,
                "controllers" => 0.90,
                _ => 0.85,
            };
            for symbol_cap in symbol_re.captures_iter(items) {
                let Some(symbol_match) = symbol_cap.get(0) else {
                    continue;
                };
                let symbol = symbol_match.as_str();
                let Some(specifier) = resolve_unique_hint_target(symbol, import_alias_sources)
                else {
                    continue;
                };
                hints.push(RelationHintRecord {
                    target: specifier,
                    call_type_hint: "framework_injection".to_string(),
                    via: format!("@Module.{}", field),
                    confidence,
                    detector: "typescript.nest.module".to_string(),
                    reason: "NestJS @Module 注入关系".to_string(),
                });
            }
        }
    }

    let class_re = Regex::new(
        r"(?s)@(Injectable|Controller)\s*(?:\([^)]*\))?\s*(?:export\s+)?class\s+[A-Za-z_][A-Za-z0-9_]*\s*\{(.*?)\}",
    )
    .expect("valid nest class regex");
    let ctor_re = Regex::new(r"constructor\s*\(([^)]*)\)").expect("valid constructor regex");
    let type_re = Regex::new(r":\s*([A-Za-z_][A-Za-z0-9_]*)").expect("valid type regex");

    for class_cap in class_re.captures_iter(&sanitized_content) {
        let Some(decorator) = class_cap.get(1).map(|m| m.as_str()) else {
            continue;
        };
        let Some(class_body) = class_cap.get(2).map(|m| m.as_str()) else {
            continue;
        };
        let Some(ctor_cap) = ctor_re.captures(class_body) else {
            continue;
        };
        let Some(params) = ctor_cap.get(1).map(|m| m.as_str()) else {
            continue;
        };

        for ty_cap in type_re.captures_iter(params) {
            let Some(symbol) = ty_cap.get(1).map(|m| m.as_str()) else {
                continue;
            };
            let Some(specifier) = resolve_unique_hint_target(symbol, import_alias_sources) else {
                continue;
            };
            hints.push(RelationHintRecord {
                target: specifier,
                call_type_hint: "framework_injection".to_string(),
                via: format!("@{} constructor", decorator),
                confidence: 0.86,
                detector: "typescript.nest.constructor".to_string(),
                reason: "NestJS 装饰器构造注入".to_string(),
            });
        }
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

fn mask_non_code_regions_for_hint_detection(root: tree_sitter::Node, source: &[u8]) -> String {
    let mut masked = source.to_vec();
    let mut stack = vec![root];

    while let Some(node) = stack.pop() {
        let should_mask = matches!(
            node.kind(),
            "comment" | "string" | "template_string" | "jsx_text"
        );
        if should_mask {
            let start = node.start_byte();
            let end = node.end_byte();
            if start < end && end <= masked.len() {
                for byte in &mut masked[start..end] {
                    if *byte != b'\n' && *byte != b'\r' {
                        *byte = b' ';
                    }
                }
            }
            continue;
        }

        let mut idx = node.child_count();
        while idx > 0 {
            idx -= 1;
            if let Some(child) = node.child(idx) {
                stack.push(child);
            }
        }
    }

    String::from_utf8_lossy(&masked).into_owned()
}

fn resolve_unique_hint_target(
    symbol: &str,
    import_alias_sources: &HashMap<String, HashSet<String>>,
) -> Option<String> {
    let candidates = import_alias_sources.get(symbol)?;
    if candidates.len() == 1 {
        return candidates.iter().next().cloned();
    }

    let mut internal = candidates
        .iter()
        .filter(|spec| is_internal_specifier(spec))
        .cloned()
        .collect::<Vec<_>>();
    internal.sort_unstable();
    internal.dedup();
    if internal.len() == 1 {
        return internal.into_iter().next();
    }

    None
}

fn is_internal_specifier(specifier: &str) -> bool {
    let spec = specifier.trim();
    spec.starts_with("./")
        || spec.starts_with("../")
        || spec.starts_with('/')
        || spec.starts_with("@app/")
        || spec.starts_with("@/")
        || spec.starts_with("~/")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extract::testing;
    use std::env;
    use std::fs;

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
        let calls = testing::call_names(&record);

        // 增强后的解析器现在能捕获更多调用类型
        assert_eq!(
            calls,
            vec![
                "formatName".to_string(),
                "Helper.log".to_string(),
                "log".to_string(),
                "Foo.bar.baz".to_string(),
                "baz".to_string(),
            ]
        );
    }

    #[test]
    fn derives_local_call_targets_from_import_aliases() {
        let cwd = env::current_dir().expect("cwd");
        let fixture_dir = cwd.join("src/tmp-ts-call-targets-test");
        let src_file = fixture_dir.join("main.ts");
        let util_file = fixture_dir.join("util.ts");

        fs::create_dir_all(&fixture_dir).expect("create fixture dir");
        fs::write(&util_file, "export function formatName() { return 'x'; }\n")
            .expect("write util");

        let source = r#"
import { formatName } from './util';

function run() {
  return formatName();
}
"#;
        fs::write(&src_file, source).expect("write src");

        let record = extract(source, src_file.to_string_lossy().as_ref(), "typescript");
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

    #[test]
    fn extracts_calls_inside_export_function_and_class_methods() {
        let source = r#"
export function run() {
  foo();
}

export class A {
  m() {
    this.bar();
    baz();
  }
}
"#;

        let record = extract(source, "src/export_calls.ts", "typescript");
        let calls = testing::call_names(&record);

        testing::assert_contains(&calls, "foo", "calls");
        testing::assert_contains(&calls, "bar", "calls");
        testing::assert_contains(&calls, "baz", "calls");

        assert_eq!(calls.iter().filter(|callee| *callee == "bar").count(), 1);
    }

    #[test]
    fn detects_nest_module_relation_hints() {
        let source = r#"
import { Module } from '@nestjs/common';
import { UserModule } from '@app/user';
import { PrismaModule } from '@app/prisma';

@Module({
  imports: [UserModule, PrismaModule],
})
export class AppModule {}
"#;
        let record = extract(source, "apps/api/src/app.module.ts", "typescript");
        let targets: Vec<String> = record
            .relation_hints
            .iter()
            .map(|h| h.target.clone())
            .collect();
        assert!(targets.iter().any(|t| t == "@app/user"));
        assert!(targets.iter().any(|t| t == "@app/prisma"));
        assert!(record
            .relation_hints
            .iter()
            .all(|h| h.call_type_hint == "framework_injection"));
    }

    #[test]
    fn detects_injectable_constructor_relation_hints() {
        let source = r#"
import { Injectable } from '@nestjs/common';
import { UserService } from '@app/user';

@Injectable()
export class AppService {
  constructor(private readonly userService: UserService) {}
}
"#;
        let record = extract(source, "apps/api/src/app.service.ts", "typescript");
        assert!(record.relation_hints.iter().any(|h| {
            h.target == "@app/user"
                && h.call_type_hint == "framework_injection"
                && h.via.contains("@Injectable")
        }));
    }

    #[test]
    fn ignores_nest_decorators_in_comments_and_strings() {
        let source = r#"
import { Module, Injectable } from '@nestjs/common';
import { UserModule } from '@app/user';
import { UserService } from '@app/user';

// @Module({ imports: [UserModule] })
const txt = "@Injectable() constructor(userService: UserService) {}";

export class AppModule {}
"#;
        let record = extract(source, "apps/api/src/app.module.ts", "typescript");
        assert!(record.relation_hints.is_empty());
    }
}
