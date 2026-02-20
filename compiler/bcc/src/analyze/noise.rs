//! 噪音代码检测器：CommentDensityDetector、LeftoverBoilerplateDetector、DeadCodeDetector

use super::{SmellDetector, SmellRecord};
use crate::extract::FileRecord;
use tree_sitter::{Node, Tree};

// ── CommentDensityDetector ──

/// 注释密度检测：注释行/代码行比超阈值 + 废话注释检测
pub struct CommentDensityDetector {
    threshold: f64,
}

impl CommentDensityDetector {
    pub fn new(threshold: f64) -> Self {
        Self { threshold }
    }
}

impl SmellDetector for CommentDensityDetector {
    fn category(&self) -> &str {
        "noise"
    }

    fn detect(&self, record: &FileRecord, source: &str, tree: &Tree) -> Vec<SmellRecord> {
        let mut smells = Vec::new();
        let lines: Vec<&str> = source.lines().collect();

        // 收集所有 comment 节点
        let mut comment_nodes = Vec::new();
        collect_comments(tree.root_node(), &mut comment_nodes);

        // 按函数粒度分析注释密度
        let functions = collect_function_ranges(tree.root_node(), &record.language, source);
        for (func_name, start_line, end_line) in &functions {
            let func_lines = end_line.saturating_sub(*start_line) + 1;
            if func_lines < 3 {
                continue;
            }

            let func_comment_lines: usize = comment_nodes
                .iter()
                .filter(|(line, _)| *line >= *start_line && *line <= *end_line)
                .map(|(start, end)| end - start + 1)
                .sum();

            let code_lines = func_lines.saturating_sub(func_comment_lines);
            if code_lines == 0 {
                continue;
            }

            let ratio = func_comment_lines as f64 / code_lines as f64;
            if ratio > self.threshold {
                smells.push(SmellRecord {
                    category: "noise".to_string(),
                    rule: "excessive_comments".to_string(),
                    severity: "warning".to_string(),
                    message: format!(
                        "函数 '{}' 注释/代码比 {:.2} 超过阈值 {:.2}",
                        func_name, ratio, self.threshold
                    ),
                    file: record.file_path.clone(),
                    line: *start_line + 1,
                    source: "bcc".to_string(),
                    confidence: (0.5 + ratio.min(1.0) * 0.5).min(1.0),
                });
            }
        }

        // 废话注释检测：注释内容与下一行代码编辑距离过近
        // 构建注释行集合，用于跳过非代码行
        let comment_line_set: std::collections::HashSet<usize> = comment_nodes
            .iter()
            .flat_map(|(start, end)| *start..=*end)
            .collect();

        for (comment_start_line, comment_end_line) in &comment_nodes {
            // 寻找下一行代码（跳过空行和注释行）
            let mut next_code_line = *comment_end_line + 1;
            while next_code_line < lines.len() {
                let trimmed = lines[next_code_line].trim();
                if !trimmed.is_empty() && !comment_line_set.contains(&next_code_line) {
                    break;
                }
                next_code_line += 1;
            }
            if next_code_line >= lines.len() {
                continue;
            }

            let comment_text = extract_comment_text(
                &lines[*comment_start_line..*comment_end_line + 1],
                &record.language,
            );
            let next_line = lines[next_code_line].trim();

            if comment_text.is_empty() || next_line.is_empty() {
                continue;
            }

            // 规范化比较：去掉标点符号和多余空格
            let normalized_comment = normalize_for_comparison(&comment_text);
            let normalized_code = normalize_for_comparison(next_line);

            if normalized_comment.is_empty() || normalized_code.is_empty() {
                continue;
            }

            let distance = strsim::normalized_levenshtein(&normalized_comment, &normalized_code);
            // distance > 0.7 表示很相似（normalized_levenshtein 返回相似度 0-1）
            if distance > 0.7 {
                smells.push(SmellRecord {
                    category: "noise".to_string(),
                    rule: "trivial_comment".to_string(),
                    severity: "info".to_string(),
                    message: format!(
                        "注释与代码高度相似（相似度 {:.2}），可能是废话注释",
                        distance
                    ),
                    file: record.file_path.clone(),
                    line: *comment_start_line + 1,
                    source: "bcc".to_string(),
                    confidence: distance.min(1.0),
                });
            }
        }

        smells
    }
}

// ── LeftoverBoilerplateDetector ──

/// 残留样板检测：TODO/FIXME/HACK 注释、pass 空函数体、未使用 import
pub struct LeftoverBoilerplateDetector;

impl SmellDetector for LeftoverBoilerplateDetector {
    fn category(&self) -> &str {
        "noise"
    }

    fn detect(&self, record: &FileRecord, source: &str, tree: &Tree) -> Vec<SmellRecord> {
        let mut smells = Vec::new();
        let lines: Vec<&str> = source.lines().collect();

        // 1. TODO/FIXME/HACK 注释
        let todo_re = regex::Regex::new(r"(?i)\b(TODO|FIXME|HACK)\b").unwrap();
        let mut comment_nodes = Vec::new();
        collect_comments(tree.root_node(), &mut comment_nodes);
        for (start_line, end_line) in &comment_nodes {
            for line_idx in *start_line..=*end_line {
                if line_idx < lines.len() {
                    if let Some(m) = todo_re.find(lines[line_idx]) {
                        smells.push(SmellRecord {
                            category: "noise".to_string(),
                            rule: "leftover_boilerplate".to_string(),
                            severity: "info".to_string(),
                            message: format!(
                                "残留 {} 注释: {}",
                                m.as_str().to_uppercase(),
                                lines[line_idx].trim()
                            ),
                            file: record.file_path.clone(),
                            line: line_idx + 1,
                            source: "bcc".to_string(),
                            confidence: 0.9,
                        });
                    }
                }
            }
        }

        // 2. 空函数体检测（Python: pass / raise NotImplementedError）
        if record.language == "python" {
            detect_empty_python_functions(tree.root_node(), source, record, &mut smells);
        }

        // 3. 未使用的 import
        detect_unused_imports(record, &mut smells);

        smells
    }
}

/// 检测 Python 空函数体（pass 或 raise NotImplementedError）
fn detect_empty_python_functions(
    node: Node,
    source: &str,
    record: &FileRecord,
    smells: &mut Vec<SmellRecord>,
) {
    if node.kind() == "function_definition" {
        if let Some(body) = node.child_by_field_name("body") {
            // 仅当函数体只有一条语句时才判定为空函数体
            if body.named_child_count() == 1 {
                if let Some(stmt) = body.named_child(0) {
                    let stmt_text = source[stmt.byte_range()].trim();
                    let is_empty_body = match stmt.kind() {
                        "pass_statement" => true,
                        "expression_statement" => stmt_text == "...",
                        "raise_statement" => stmt_text.starts_with("raise NotImplementedError"),
                        _ => false,
                    };
                    if is_empty_body {
                        let func_name = node
                            .child_by_field_name("name")
                            .map(|n| &source[n.byte_range()])
                            .unwrap_or("<anonymous>");
                        smells.push(SmellRecord {
                            category: "noise".to_string(),
                            rule: "empty_function_body".to_string(),
                            severity: "warning".to_string(),
                            message: format!("函数 '{}' 体为空（{}）", func_name, stmt_text),
                            file: record.file_path.clone(),
                            line: node.start_position().row + 1,
                            source: "bcc".to_string(),
                            confidence: 0.9,
                        });
                    }
                }
            }
        }
    }

    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            detect_empty_python_functions(child, source, record, smells);
        }
    }
}

/// 未使用 import 检测：import 的 specifier 不在 calls 和 local_call_targets 中出现
fn detect_unused_imports(record: &FileRecord, smells: &mut Vec<SmellRecord>) {
    // Elixir 的 use/behaviour 类型 import 有隐式副作用，跳过
    let skip_kinds = ["use", "behaviour"];

    let call_names: std::collections::HashSet<&str> = record
        .calls
        .iter()
        .map(|c| c.callee.as_str())
        .chain(record.local_call_targets.iter().map(|s| s.as_str()))
        .collect();

    for imp in &record.imports {
        if skip_kinds.contains(&imp.kind.as_str()) {
            continue;
        }

        let specifier = imp.specifier.trim();
        if specifier.is_empty() {
            continue;
        }

        // 提取最后一段名字作为使用名（如 os.path → path, Foo.Bar → Bar）
        let short_name = specifier
            .rsplit('.')
            .next()
            .unwrap_or(specifier)
            .trim();

        // 交叉比对 imports specifier 与 calls + local_call_targets
        let used = call_names.contains(specifier)
            || call_names.contains(short_name)
            || call_names.iter().any(|c| c.contains(specifier) || c.contains(short_name));

        if !used {
            smells.push(SmellRecord {
                category: "noise".to_string(),
                rule: "unused_import".to_string(),
                severity: "info".to_string(),
                message: format!("import '{}' 可能未使用", specifier),
                file: record.file_path.clone(),
                line: 1,
                source: "bcc".to_string(),
                confidence: 0.6,
            });
        }
    }
}

// ── DeadCodeDetector ──

/// 死代码检测：被注释掉的代码、if False 分支、return 后不可达代码
pub struct DeadCodeDetector;

impl SmellDetector for DeadCodeDetector {
    fn category(&self) -> &str {
        "noise"
    }

    fn detect(&self, record: &FileRecord, source: &str, tree: &Tree) -> Vec<SmellRecord> {
        let mut smells = Vec::new();
        let lines: Vec<&str> = source.lines().collect();

        // 1. 被注释掉的代码块（连续 3+ 行注释，内容可 parse 为合法代码）
        detect_commented_out_code(&lines, record, &record.language, &mut smells);

        // 2. if False / if 0 死分支
        detect_dead_branches(tree.root_node(), source, record, &mut smells);

        // 3. return/raise 后不可达代码
        detect_unreachable_code(tree.root_node(), source, record, &mut smells);

        smells
    }
}

/// 检测被注释掉的代码块
fn detect_commented_out_code(
    lines: &[&str],
    record: &FileRecord,
    language: &str,
    smells: &mut Vec<SmellRecord>,
) {
    // 只对 Python 做精确检测（需要 tree-sitter-python parse 验证）
    if language != "python" {
        return;
    }

    let comment_prefix = "#";
    let mut i = 0;
    while i < lines.len() {
        // 找连续注释块
        let start = i;
        while i < lines.len() && lines[i].trim().starts_with(comment_prefix) {
            i += 1;
        }
        let block_len = i - start;
        if block_len >= 3 {
            // 提取注释内容（去掉 # 前缀）
            let code_text: String = (start..i)
                .map(|idx| {
                    let trimmed = lines[idx].trim();
                    trimmed.strip_prefix(comment_prefix).unwrap_or(trimmed).trim_start()
                })
                .collect::<Vec<_>>()
                .join("\n");

            // 尝试用 tree-sitter-python parse
            if try_parse_as_code(&code_text, "python") {
                smells.push(SmellRecord {
                    category: "noise".to_string(),
                    rule: "commented_out_code".to_string(),
                    severity: "warning".to_string(),
                    message: format!(
                        "连续 {} 行注释内容为合法代码，可能是被注释掉的代码",
                        block_len
                    ),
                    file: record.file_path.clone(),
                    line: start + 1,
                    source: "bcc".to_string(),
                    confidence: 0.7,
                });
            }
        }
        if i == start {
            i += 1;
        }
    }
}

/// 尝试将文本作为代码 parse，成功且无 ERROR 节点返回 true
fn try_parse_as_code(text: &str, language: &str) -> bool {
    let lang = match language {
        "python" => tree_sitter_python::LANGUAGE.into(),
        _ => return false,
    };
    let mut parser = tree_sitter::Parser::new();
    if parser.set_language(&lang).is_err() {
        return false;
    }
    match parser.parse(text, None) {
        Some(tree) => !has_error_node(tree.root_node()),
        None => false,
    }
}

/// 递归检查 AST 中是否存在 ERROR 节点
fn has_error_node(node: Node) -> bool {
    if node.is_error() || node.is_missing() {
        return true;
    }
    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            if has_error_node(child) {
                return true;
            }
        }
    }
    false
}

/// 检测 if False / if 0 死分支（Python）
fn detect_dead_branches(
    node: Node,
    source: &str,
    record: &FileRecord,
    smells: &mut Vec<SmellRecord>,
) {
    if record.language != "python" {
        return;
    }
    detect_dead_branches_recursive(node, source, record, smells);
}

fn detect_dead_branches_recursive(
    node: Node,
    source: &str,
    record: &FileRecord,
    smells: &mut Vec<SmellRecord>,
) {
    if node.kind() == "if_statement" {
        if let Some(condition) = node.child_by_field_name("condition") {
            let cond_text = &source[condition.byte_range()];
            let trimmed = cond_text.trim();
            if trimmed == "False" || trimmed == "0" || trimmed == "false" {
                smells.push(SmellRecord {
                    category: "noise".to_string(),
                    rule: "dead_branch".to_string(),
                    severity: "warning".to_string(),
                    message: format!("死分支: if {} 永远为假", trimmed),
                    file: record.file_path.clone(),
                    line: node.start_position().row + 1,
                    source: "bcc".to_string(),
                    confidence: 0.95,
                });
            }
        }
    }

    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            detect_dead_branches_recursive(child, source, record, smells);
        }
    }
}

/// 检测 return/raise 后不可达代码（Python）
fn detect_unreachable_code(
    node: Node,
    source: &str,
    record: &FileRecord,
    smells: &mut Vec<SmellRecord>,
) {
    if record.language != "python" {
        return;
    }
    detect_unreachable_recursive(node, source, record, smells);
}

fn detect_unreachable_recursive(
    node: Node,
    source: &str,
    record: &FileRecord,
    smells: &mut Vec<SmellRecord>,
) {
    // 在 block 节点中查找 return_statement / raise_statement 后面还有语句
    if node.kind() == "block" {
        let mut found_return = false;
        let mut return_line = 0;
        for i in 0..node.named_child_count() {
            if let Some(child) = node.named_child(i) {
                if found_return {
                    smells.push(SmellRecord {
                        category: "noise".to_string(),
                        rule: "unreachable_code".to_string(),
                        severity: "warning".to_string(),
                        message: format!(
                            "return/raise 后的不可达代码（return 在第 {} 行）",
                            return_line
                        ),
                        file: record.file_path.clone(),
                        line: child.start_position().row + 1,
                        source: "bcc".to_string(),
                        confidence: 0.9,
                    });
                    break;
                }
                if child.kind() == "return_statement" || child.kind() == "raise_statement" {
                    found_return = true;
                    return_line = child.start_position().row + 1;
                }
            }
        }
    }

    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            detect_unreachable_recursive(child, source, record, smells);
        }
    }
}

// ── 辅助函数 ──

/// 递归收集所有 comment 节点的行范围 (start_line, end_line)
fn collect_comments(node: Node, comments: &mut Vec<(usize, usize)>) {
    if node.kind() == "comment" || node.kind() == "line_comment" || node.kind() == "block_comment"
    {
        comments.push((node.start_position().row, node.end_position().row));
        return;
    }
    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            collect_comments(child, comments);
        }
    }
}

/// 收集函数范围：(函数名, start_line, end_line)
fn collect_function_ranges(node: Node, language: &str, source: &str) -> Vec<(String, usize, usize)> {
    let mut functions = Vec::new();
    collect_function_ranges_recursive(node, language, source, &mut functions);
    functions
}

fn collect_function_ranges_recursive(
    node: Node,
    language: &str,
    source: &str,
    functions: &mut Vec<(String, usize, usize)>,
) {
    let is_func = match language {
        "python" => node.kind() == "function_definition",
        "elixir" => node.kind() == "call" && {
            node.child(0)
                .map(|c| {
                    c.kind() == "identifier" && {
                        let name = &source[c.byte_range()];
                        matches!(name, "def" | "defp" | "defmacro" | "defmacrop")
                    }
                })
                .unwrap_or(false)
        },
        "rust" => node.kind() == "function_item",
        "typescript" | "tsx" => {
            node.kind() == "function_declaration"
                || node.kind() == "arrow_function"
                || node.kind() == "method_definition"
        }
        _ => false,
    };

    if is_func {
        let name = node
            .child_by_field_name("name")
            .map(|n| source[n.byte_range()].to_string())
            .unwrap_or_else(|| "<anonymous>".to_string());
        functions.push((name, node.start_position().row, node.end_position().row));
    }

    for i in 0..node.child_count() {
        if let Some(child) = node.child(i) {
            collect_function_ranges_recursive(child, language, source, functions);
        }
    }
}

/// 从注释行中提取纯文本（去掉注释前缀符号）
fn extract_comment_text(comment_lines: &[&str], language: &str) -> String {
    comment_lines
        .iter()
        .map(|line| {
            let trimmed = line.trim();
            match language {
                "python" => trimmed.strip_prefix('#').unwrap_or(trimmed).trim(),
                "elixir" => trimmed.strip_prefix('#').unwrap_or(trimmed).trim(),
                "rust" => trimmed
                    .strip_prefix("///")
                    .or_else(|| trimmed.strip_prefix("//"))
                    .unwrap_or(trimmed)
                    .trim(),
                _ => trimmed
                    .strip_prefix("//")
                    .or_else(|| trimmed.strip_prefix('#'))
                    .unwrap_or(trimmed)
                    .trim(),
            }
        })
        .collect::<Vec<_>>()
        .join(" ")
}

/// 规范化文本用于比较（去标点、转小写、去多余空格）
fn normalize_for_comparison(text: &str) -> String {
    text.chars()
        .filter(|c| c.is_alphanumeric() || c.is_whitespace())
        .collect::<String>()
        .to_lowercase()
        .split_whitespace()
        .collect::<Vec<_>>()
        .join(" ")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extract::{SideEffects, ImportRecord};

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
    fn excessive_comments_detected() {
        let source = r#"
def heavy_comments():
    # comment line 1
    # comment line 2
    # comment line 3
    # comment line 4
    # comment line 5
    # comment line 6
    x = 1
    y = 2
    return x + y
"#;
        let tree = parse_python(source);
        let record = make_record("python", "test.py");
        let detector = CommentDensityDetector::new(0.5);
        let smells = detector.detect(&record, source, &tree);
        assert!(
            smells.iter().any(|s| s.rule == "excessive_comments"),
            "应检出 excessive_comments: {:?}",
            smells
        );
    }

    #[test]
    fn normal_comments_not_flagged() {
        let source = r#"
def normal():
    # 这是一个合理的注释
    x = 1
    y = 2
    z = 3
    a = 4
    b = 5
    c = 6
    d = 7
    return x + y + z + a + b + c + d
"#;
        let tree = parse_python(source);
        let record = make_record("python", "test.py");
        let detector = CommentDensityDetector::new(0.5);
        let smells = detector.detect(&record, source, &tree);
        assert!(
            !smells.iter().any(|s| s.rule == "excessive_comments"),
            "不应检出 excessive_comments: {:?}",
            smells
        );
    }

    #[test]
    fn trivial_comment_detected() {
        let source = r#"
# increment x
x += 1
"#;
        let tree = parse_python(source);
        let record = make_record("python", "test.py");
        let detector = CommentDensityDetector::new(0.5);
        let smells = detector.detect(&record, source, &tree);
        assert!(
            smells.iter().any(|s| s.rule == "trivial_comment"),
            "应检出 trivial_comment: {:?}",
            smells
        );
    }

    #[test]
    fn todo_comment_detected() {
        let source = r#"
# TODO: fix this later
x = 1
"#;
        let tree = parse_python(source);
        let record = make_record("python", "test.py");
        let detector = LeftoverBoilerplateDetector;
        let smells = detector.detect(&record, source, &tree);
        assert!(
            smells.iter().any(|s| s.rule == "leftover_boilerplate"),
            "应检出 leftover_boilerplate: {:?}",
            smells
        );
    }

    #[test]
    fn empty_function_body_detected() {
        let source = r#"
def foo():
    pass

def bar():
    raise NotImplementedError("not implemented")
"#;
        let tree = parse_python(source);
        let record = make_record("python", "test.py");
        let detector = LeftoverBoilerplateDetector;
        let smells = detector.detect(&record, source, &tree);
        let empty_bodies: Vec<_> = smells
            .iter()
            .filter(|s| s.rule == "empty_function_body")
            .collect();
        assert!(
            empty_bodies.len() >= 1,
            "应检出至少 1 个 empty_function_body: {:?}",
            smells
        );
    }

    #[test]
    fn unused_import_detected() {
        let source = "import os\nx = 1\n";
        let tree = parse_python(source);
        let mut record = make_record("python", "test.py");
        record.imports.push(ImportRecord {
            specifier: "os".to_string(),
            kind: "import".to_string(),
        });
        let detector = LeftoverBoilerplateDetector;
        let smells = detector.detect(&record, source, &tree);
        assert!(
            smells.iter().any(|s| s.rule == "unused_import"),
            "应检出 unused_import: {:?}",
            smells
        );
    }

    #[test]
    fn elixir_use_behaviour_not_flagged_as_unused() {
        // Elixir 的 use/behaviour import 有隐式副作用，不应被标记为 unused_import
        let source = "use Phoenix.LiveView\n@behaviour GenServer\n";
        let tree = {
            let mut parser = tree_sitter::Parser::new();
            let lang: tree_sitter::Language = tree_sitter_elixir::LANGUAGE.into();
            parser.set_language(&lang).unwrap();
            parser.parse(source, None).unwrap()
        };
        let mut record = make_record("elixir", "lib/foo.ex");
        record.imports.push(ImportRecord {
            specifier: "Phoenix.LiveView".to_string(),
            kind: "use".to_string(),
        });
        record.imports.push(ImportRecord {
            specifier: "GenServer".to_string(),
            kind: "behaviour".to_string(),
        });
        let detector = LeftoverBoilerplateDetector;
        let smells = detector.detect(&record, source, &tree);
        assert!(
            !smells.iter().any(|s| s.rule == "unused_import"),
            "Elixir use/behaviour 不应被标记为 unused_import: {:?}",
            smells
        );
    }

    #[test]
    fn commented_out_code_detected() {
        let source = r#"
x = 1
# def old_function():
#     x = 1
#     return x + 2
y = 2
"#;
        let tree = parse_python(source);
        let record = make_record("python", "test.py");
        let detector = DeadCodeDetector;
        let smells = detector.detect(&record, source, &tree);
        assert!(
            smells.iter().any(|s| s.rule == "commented_out_code"),
            "应检出 commented_out_code: {:?}",
            smells
        );
    }

    #[test]
    fn dead_branch_detected() {
        let source = r#"
if False:
    do_something()
x = 1
"#;
        let tree = parse_python(source);
        let record = make_record("python", "test.py");
        let detector = DeadCodeDetector;
        let smells = detector.detect(&record, source, &tree);
        assert!(
            smells.iter().any(|s| s.rule == "dead_branch"),
            "应检出 dead_branch: {:?}",
            smells
        );
    }

    #[test]
    fn unreachable_code_detected() {
        let source = r#"
def foo():
    return 1
    x = 2
"#;
        let tree = parse_python(source);
        let record = make_record("python", "test.py");
        let detector = DeadCodeDetector;
        let smells = detector.detect(&record, source, &tree);
        assert!(
            smells.iter().any(|s| s.rule == "unreachable_code"),
            "应检出 unreachable_code: {:?}",
            smells
        );
    }
}
