//! 结构性重复检测器：StructuralDuplicationDetector、BoilerplateSkeletonDetector
//!
//! MVP 仅支持 Python：对函数体 AST 做规范化后 blake3 hash，相同 hash 聚类。

use super::{SmellDetector, SmellRecord};
use crate::extract::FileRecord;
use std::collections::HashMap;
use tree_sitter::{Node, Tree};

/// 提取到的函数信息（供跨文件比对）
#[derive(Debug, Clone)]
pub struct FunctionInfo {
    pub file: String,
    pub name: String,
    pub line: usize,
    /// 规范化 AST 字符串（变量名/字符串/数字替换为占位符）
    pub normalized_ast: String,
    /// 规范化 AST 的 blake3 hash
    pub structural_hash: String,
    /// 骨架字符串（只保留节点类型，忽略所有叶子内容）
    pub skeleton: String,
    /// 骨架的 blake3 hash
    pub skeleton_hash: String,
}

/// StructuralDuplicationDetector — 单文件内的结构重复检测
pub struct StructuralDuplicationDetector;

impl SmellDetector for StructuralDuplicationDetector {
    fn category(&self) -> &str {
        "duplication"
    }

    fn detect(&self, record: &FileRecord, source: &str, tree: &Tree) -> Vec<SmellRecord> {
        if record.language != "python" {
            return vec![];
        }

        let functions = extract_functions(&record.file_path, source, tree, &record.language);
        detect_structural_duplication(&functions)
    }
}

/// BoilerplateSkeletonDetector — 单文件内的骨架重复检测
pub struct BoilerplateSkeletonDetector;

impl SmellDetector for BoilerplateSkeletonDetector {
    fn category(&self) -> &str {
        "duplication"
    }

    fn detect(&self, record: &FileRecord, source: &str, tree: &Tree) -> Vec<SmellRecord> {
        if record.language != "python" {
            return vec![];
        }

        let functions = extract_functions(&record.file_path, source, tree, &record.language);
        detect_boilerplate_skeleton(&functions)
    }
}

/// 从 AST 中提取所有函数的信息
pub fn extract_functions(
    file_path: &str,
    source: &str,
    tree: &Tree,
    language: &str,
) -> Vec<FunctionInfo> {
    if language != "python" {
        return vec![];
    }

    let mut functions = Vec::new();
    extract_functions_recursive(file_path, source, tree.root_node(), &mut functions);
    functions
}

fn extract_functions_recursive(
    file_path: &str,
    source: &str,
    node: Node,
    functions: &mut Vec<FunctionInfo>,
) {
    if node.kind() == "function_definition" {
        if let Some(body) = node.child_by_field_name("body") {
            let name = node
                .child_by_field_name("name")
                .map(|n| &source[n.byte_range()])
                .unwrap_or("<anonymous>");

            let normalized_ast = normalize_ast(body, source);
            let skeleton = build_skeleton(body);

            // 跳过过短的函数体（< 3 个语句）
            if body.named_child_count() < 2 {
                // 仍然递归子节点
                for i in 0..node.child_count() {
                    if let Some(child) = node.child(i) {
                        extract_functions_recursive(file_path, source, child, functions);
                    }
                }
                return;
            }

            let structural_hash = blake3::hash(normalized_ast.as_bytes()).to_hex().to_string();
            let skeleton_hash = blake3::hash(skeleton.as_bytes()).to_hex().to_string();

            functions.push(FunctionInfo {
                file: file_path.to_string(),
                name: name.to_string(),
                line: node.start_position().row + 1,
                normalized_ast,
                structural_hash,
                skeleton,
                skeleton_hash,
            });
        }
    }

    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            extract_functions_recursive(file_path, source, child, functions);
        }
    }
}

/// 规范化 AST：identifier → $VAR, string → $STR, number → $NUM
fn normalize_ast(node: Node, source: &str) -> String {
    let mut result = String::new();
    normalize_ast_recursive(node, source, &mut result);
    result
}

fn normalize_ast_recursive(node: Node, source: &str, result: &mut String) {
    match node.kind() {
        "identifier" => {
            result.push_str("$VAR");
        }
        "string" | "concatenated_string" => {
            result.push_str("$STR");
        }
        "integer" | "float" => {
            result.push_str("$NUM");
        }
        _ => {
            if node.child_count() == 0 {
                // 叶子节点保留原文
                result.push_str(&source[node.byte_range()]);
            } else {
                result.push('(');
                result.push_str(node.kind());
                for i in 0..node.child_count() {
                    if let Some(child) = node.child(i) {
                        result.push(' ');
                        normalize_ast_recursive(child, source, result);
                    }
                }
                result.push(')');
            }
        }
    }
}

/// 构建骨架：只保留节点类型，忽略所有叶子节点内容
fn build_skeleton(node: Node) -> String {
    let mut result = String::new();
    build_skeleton_recursive(node, &mut result);
    result
}

fn build_skeleton_recursive(node: Node, result: &mut String) {
    // 叶子节点和"原子"复合节点只保留类型，不展开子节点。
    // 这样骨架只反映语句级控制流，不区分参数个数、字符串内容等细节。
    const ATOMIC_KINDS: &[&str] = &[
        "argument_list", "parameters", "string", "concatenated_string",
        "tuple", "list", "dictionary", "set",
    ];
    if node.child_count() == 0 || ATOMIC_KINDS.contains(&node.kind()) {
        result.push_str(node.kind());
    } else {
        result.push('(');
        result.push_str(node.kind());
        for i in 0..node.child_count() {
            if let Some(child) = node.child(i) {
                result.push(' ');
                build_skeleton_recursive(child, result);
            }
        }
        result.push(')');
    }
}

/// 检测结构重复：相同 structural_hash 的函数聚类
pub fn detect_structural_duplication(functions: &[FunctionInfo]) -> Vec<SmellRecord> {
    let mut groups: HashMap<&str, Vec<&FunctionInfo>> = HashMap::new();
    for func in functions {
        groups
            .entry(&func.structural_hash)
            .or_default()
            .push(func);
    }

    let mut smells = Vec::new();
    for (_hash, group) in &groups {
        if group.len() < 2 {
            continue;
        }

        // 精确比对规范化字符串确认非 hash 碰撞
        let first_ast = &group[0].normalized_ast;
        let confirmed: Vec<&&FunctionInfo> = group
            .iter()
            .filter(|f| &f.normalized_ast == first_ast)
            .collect();

        if confirmed.len() < 2 {
            continue;
        }

        let names: Vec<String> = confirmed
            .iter()
            .map(|f| format!("{}:{} ({})", f.file, f.line, f.name))
            .collect();

        for func in &confirmed {
            smells.push(SmellRecord {
                category: "duplication".to_string(),
                rule: "structural_duplication".to_string(),
                severity: "medium".to_string(),
                message: format!(
                    "函数 '{}' 与其他 {} 个函数结构相同（仅变量名/字面量不同），建议提取公共逻辑: {}",
                    func.name,
                    confirmed.len() - 1,
                    names.join(", ")
                ),
                file: func.file.clone(),
                line: func.line,
                source: "bcc".to_string(),
                confidence: 0.85,
            });
        }
    }

    smells
}

/// 检测骨架重复：骨架相同但规范化 hash 不同
pub fn detect_boilerplate_skeleton(functions: &[FunctionInfo]) -> Vec<SmellRecord> {
    let mut skeleton_groups: HashMap<&str, Vec<&FunctionInfo>> = HashMap::new();
    for func in functions {
        skeleton_groups
            .entry(&func.skeleton_hash)
            .or_default()
            .push(func);
    }

    let mut smells = Vec::new();
    for (_hash, group) in &skeleton_groups {
        if group.len() < 2 {
            continue;
        }

        // 精确比对骨架字符串
        let first_skeleton = &group[0].skeleton;
        let confirmed: Vec<&&FunctionInfo> = group
            .iter()
            .filter(|f| &f.skeleton == first_skeleton)
            .collect();

        if confirmed.len() < 2 {
            continue;
        }

        // 检查是否已经被 structural_duplication 覆盖（所有 structural_hash 相同则跳过）
        let structural_hashes: std::collections::HashSet<&str> = confirmed
            .iter()
            .map(|f| f.structural_hash.as_str())
            .collect();
        if structural_hashes.len() == 1 {
            // 所有函数 structural hash 相同，已被 structural_duplication 覆盖
            continue;
        }

        let names: Vec<String> = confirmed
            .iter()
            .map(|f| format!("{}:{} ({})", f.file, f.line, f.name))
            .collect();

        for func in &confirmed {
            smells.push(SmellRecord {
                category: "duplication".to_string(),
                rule: "boilerplate_skeleton".to_string(),
                severity: "medium".to_string(),
                message: format!(
                    "函数 '{}' 与其他 {} 个函数骨架结构相同（叶子节点不同），可能可以泛化: {}",
                    func.name,
                    confirmed.len() - 1,
                    names.join(", ")
                ),
                file: func.file.clone(),
                line: func.line,
                source: "bcc".to_string(),
                confidence: 0.7,
            });
        }
    }

    smells
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extract::SideEffects;

    fn make_record(language: &str, file_path: &str) -> FileRecord {
        FileRecord {
            language: language.to_string(),
            file_path: file_path.to_string(),
            module_doc: None,
            exports: vec![],
            imports: vec![],
            calls: vec![],
            local_call_targets: vec![],
            relation_hints: vec![],
            side_effects: SideEffects {
                has_async: false,
                has_http: false,
                has_genserver: false,
                has_file_io: false,
                has_pubsub: false,
            },
            loc_lines: 0,
            declarations: 0,
            type_annotations: vec![],
            type_guards: vec![],
            schema_fields: vec![],
            source_code: None,
        }
    }

    fn parse_python(source: &str) -> Tree {
        let mut parser = tree_sitter::Parser::new();
        let lang: tree_sitter::Language = tree_sitter_python::LANGUAGE.into();
        parser.set_language(&lang).unwrap();
        parser.parse(source, None).unwrap()
    }

    #[test]
    fn structural_duplication_detected() {
        // 两个函数结构完全相同，只有变量名不同
        let source = r#"
def process_user(user):
    result = validate(user)
    if result:
        save(result)
    return result

def process_order(order):
    result = validate(order)
    if result:
        save(result)
    return result
"#;
        let tree = parse_python(source);
        let record = make_record("python", "test.py");
        let detector = StructuralDuplicationDetector;
        let smells = detector.detect(&record, source, &tree);
        assert!(
            smells.iter().any(|s| s.rule == "structural_duplication"),
            "应检出 structural_duplication: {:?}",
            smells
        );
    }

    #[test]
    fn boilerplate_skeleton_detected() {
        // 两个函数骨架相同但 normalized AST 不同：
        // 调用参数个数不同（process(item) vs transform(item, extra)），
        // normalized_ast 区分参数数量 → structural_hash 不同，
        // 但 build_skeleton 将 argument_list 视为原子节点不展开 → skeleton_hash 相同
        let source = r#"
def fetch_users(db):
    items = db.query("users")
    for item in items:
        process(item)
    return items

def fetch_orders(db):
    items = db.execute("orders")
    for item in items:
        transform(item, "extra")
    return items
"#;
        let tree = parse_python(source);
        let record = make_record("python", "test.py");
        let detector = BoilerplateSkeletonDetector;
        let smells = detector.detect(&record, source, &tree);
        assert!(
            smells.iter().any(|s| s.rule == "boilerplate_skeleton"),
            "应检出 boilerplate_skeleton: {:?}",
            smells
        );
    }

    #[test]
    fn no_duplication_for_different_functions() {
        let source = r#"
def add(a, b):
    return a + b

def multiply(a, b):
    result = a * b
    print(result)
    return result
"#;
        let tree = parse_python(source);
        let record = make_record("python", "test.py");

        let detector1 = StructuralDuplicationDetector;
        let smells1 = detector1.detect(&record, source, &tree);
        assert!(
            !smells1.iter().any(|s| s.rule == "structural_duplication"),
            "不应检出 structural_duplication: {:?}",
            smells1
        );

        let detector2 = BoilerplateSkeletonDetector;
        let smells2 = detector2.detect(&record, source, &tree);
        assert!(
            !smells2.iter().any(|s| s.rule == "boilerplate_skeleton"),
            "不应检出 boilerplate_skeleton: {:?}",
            smells2
        );
    }

    #[test]
    fn non_python_skipped() {
        let source = "def foo(): pass\ndef bar(): pass\n";
        let tree = parse_python(source);
        let record = make_record("rust", "test.rs");
        let detector = StructuralDuplicationDetector;
        let smells = detector.detect(&record, source, &tree);
        assert!(smells.is_empty(), "非 Python 应跳过");
    }
}
