use super::FileRecord;
use serde_json::{json, Value};
use std::collections::BTreeSet;
use std::path::{Path, PathBuf};

/// 提取调用名列表，便于测试断言。
pub fn call_names(record: &FileRecord) -> Vec<String> {
    record
        .calls
        .iter()
        .map(|call| call.callee.clone())
        .collect()
}

/// 提取导出名列表，便于测试断言。
pub fn export_names(record: &FileRecord) -> Vec<String> {
    record
        .exports
        .iter()
        .map(|export| export.name.clone())
        .collect()
}

/// 统一断言 imports 中存在指定 specifier+kind 组合。
pub fn assert_import_contains(record: &FileRecord, specifier: &str, kind: &str) {
    assert!(
        record
            .imports
            .iter()
            .any(|item| item.specifier == specifier && item.kind == kind),
        "imports should contain ({}, {}), actual: {:?}",
        specifier,
        kind,
        record
            .imports
            .iter()
            .map(|item| format!("{}:{}", item.kind, item.specifier))
            .collect::<Vec<_>>()
    );
}

/// 统一断言列表包含目标值，失败信息带上下文。
pub fn assert_contains(items: &[String], expected: &str, label: &str) {
    assert!(
        items.iter().any(|item| item == expected),
        "{} should contain '{}', actual: {:?}",
        label,
        expected,
        items
    );
}

/// 归一化 calls：按 callee 去重并排序，忽略顺序差异。
pub fn normalized_call_set(record: &FileRecord) -> BTreeSet<String> {
    record
        .calls
        .iter()
        .map(|call| call.callee.clone())
        .collect()
}

/// 归一化 imports：按 "kind:specifier" 去重并排序，统一空白。
pub fn normalized_import_set(record: &FileRecord) -> BTreeSet<String> {
    record
        .imports
        .iter()
        .map(|item| {
            format!(
                "{}:{}",
                item.kind.trim(),
                normalize_spaces(item.specifier.trim())
            )
        })
        .collect()
}

/// 断言两个记录的 calls/imports 关键字段集合一致。
pub fn assert_semantic_sets_equal(left: &FileRecord, right: &FileRecord, context: &str) {
    let left_calls = normalized_call_set(left);
    let right_calls = normalized_call_set(right);
    assert_eq!(
        left_calls, right_calls,
        "{} calls set mismatch, left={:?}, right={:?}",
        context, left_calls, right_calls
    );

    let left_imports = normalized_import_set(left);
    let right_imports = normalized_import_set(right);
    assert_eq!(
        left_imports, right_imports,
        "{} imports set mismatch, left={:?}, right={:?}",
        context, left_imports, right_imports
    );
}

/// 归一化 CLI `--mode ast` JSON 中的 calls 集合。
pub fn normalized_call_set_from_json(ast: &serde_json::Value) -> BTreeSet<String> {
    ast.get("calls")
        .and_then(|v| v.as_array())
        .map(|calls| {
            calls
                .iter()
                .filter_map(|call| call.get("callee").and_then(|c| c.as_str()))
                .map(|callee| callee.to_string())
                .collect()
        })
        .unwrap_or_default()
}

/// 归一化 CLI `--mode ast` JSON 中的 imports 集合。
pub fn normalized_import_set_from_json(ast: &serde_json::Value) -> BTreeSet<String> {
    ast.get("imports")
        .and_then(|v| v.as_array())
        .map(|imports| {
            imports
                .iter()
                .filter_map(|imp| {
                    let kind = imp.get("kind").and_then(|v| v.as_str())?;
                    let specifier = imp.get("specifier").and_then(|v| v.as_str())?;
                    Some(format!(
                        "{}:{}",
                        kind.trim(),
                        normalize_spaces(specifier.trim())
                    ))
                })
                .collect()
        })
        .unwrap_or_default()
}

/// 断言两个 CLI `--mode ast` JSON 的 calls/imports 集合一致。
pub fn assert_semantic_sets_equal_json(
    left: &serde_json::Value,
    right: &serde_json::Value,
    context: &str,
) {
    let left_calls = normalized_call_set_from_json(left);
    let right_calls = normalized_call_set_from_json(right);
    assert_eq!(
        left_calls, right_calls,
        "{} calls set mismatch, left={:?}, right={:?}",
        context, left_calls, right_calls
    );

    let left_imports = normalized_import_set_from_json(left);
    let right_imports = normalized_import_set_from_json(right);
    assert_eq!(
        left_imports, right_imports,
        "{} imports set mismatch, left={:?}, right={:?}",
        context, left_imports, right_imports
    );
}

/// 归一化测试路径：转为相对路径并统一为 POSIX 分隔符，避免平台差异。
pub fn normalize_test_path(path: &str) -> String {
    let raw = Path::new(path);
    let normalized_path = std::env::current_dir()
        .ok()
        .and_then(|cwd| raw.strip_prefix(&cwd).ok().map(PathBuf::from))
        .unwrap_or_else(|| raw.to_path_buf());
    normalize_path_separators(&normalized_path.to_string_lossy())
}

/// 仅保留 Golden 测试关注字段，并做稳定排序/去重，保证输出可复现。
pub fn normalize_core_fields(record: &FileRecord) -> Value {
    let mut calls: Vec<(String, usize)> = record
        .calls
        .iter()
        .map(|item| (normalize_spaces(item.callee.trim()), item.line))
        .collect();
    calls.sort_by(|a, b| a.0.cmp(&b.0).then_with(|| a.1.cmp(&b.1)));
    calls.dedup_by(|a, b| a.0 == b.0);

    let mut imports: Vec<(String, String)> = record
        .imports
        .iter()
        .map(|item| {
            (
                normalize_spaces(item.kind.trim()),
                normalize_spaces(item.specifier.trim()),
            )
        })
        .collect();
    imports.sort_by(|a, b| a.0.cmp(&b.0).then_with(|| a.1.cmp(&b.1)));
    imports.dedup();

    let mut exports: Vec<(String, String, Option<String>, usize)> = record
        .exports
        .iter()
        .map(|item| {
            (
                normalize_spaces(item.name.trim()),
                normalize_spaces(item.kind.trim()),
                item.signature
                    .as_ref()
                    .map(|sig| normalize_spaces(sig.trim())),
                item.line,
            )
        })
        .collect();
    exports.sort_by(|a, b| {
        a.0.cmp(&b.0)
            .then_with(|| a.1.cmp(&b.1))
            .then_with(|| a.2.cmp(&b.2))
            .then_with(|| a.3.cmp(&b.3))
    });
    exports.dedup();

    json!({
        "calls": calls
            .into_iter()
            .map(|(callee, line)| json!({ "callee": callee, "line": line }))
            .collect::<Vec<_>>(),
        "imports": imports
            .into_iter()
            .map(|(kind, specifier)| json!({ "kind": kind, "specifier": specifier }))
            .collect::<Vec<_>>(),
        "exports": exports
            .into_iter()
            .map(|(name, kind, signature, line)| json!({
                "name": name,
                "kind": kind,
                "signature": signature,
                "line": line
            }))
            .collect::<Vec<_>>(),
        "declarations": record.declarations,
        "loc_lines": record.loc_lines
    })
}

/// 统一 JSON pretty 序列化格式（末尾换行），便于审查 diff。
pub fn stable_pretty_json(value: &Value) -> String {
    let mut text = serde_json::to_string_pretty(value).expect("serialize JSON");
    text.push('\n');
    text
}

/// 生成字段级 diff 片段，定位 Golden 不一致的关键字段。
pub fn diff_core_fields(expected: &Value, actual: &Value, max_lines: usize) -> String {
    const FIELDS: &[&str] = &["calls", "imports", "exports", "declarations", "loc_lines"];
    let mut chunks = Vec::new();
    for field in FIELDS {
        let left = expected.get(field).unwrap_or(&Value::Null);
        let right = actual.get(field).unwrap_or(&Value::Null);
        if left == right {
            continue;
        }
        let left_text = truncate_lines(
            &serde_json::to_string_pretty(left).unwrap_or_else(|_| left.to_string()),
            max_lines,
        );
        let right_text = truncate_lines(
            &serde_json::to_string_pretty(right).unwrap_or_else(|_| right.to_string()),
            max_lines,
        );
        chunks.push(format!(
            "field `{}` mismatch\nexpected:\n{}\nactual:\n{}",
            field, left_text, right_text
        ));
    }

    if chunks.is_empty() {
        "no field-level diff".to_string()
    } else {
        chunks.join("\n\n")
    }
}

fn normalize_spaces(input: &str) -> String {
    input.split_whitespace().collect::<Vec<_>>().join(" ")
}

fn normalize_path_separators(input: &str) -> String {
    input
        .replace('\\', "/")
        .trim_start_matches("./")
        .to_string()
}

fn truncate_lines(input: &str, max_lines: usize) -> String {
    if max_lines == 0 {
        return String::new();
    }

    let mut lines = input.lines();
    let mut picked = Vec::new();
    for _ in 0..max_lines {
        let Some(line) = lines.next() else { break };
        picked.push(line.to_string());
    }

    if lines.next().is_some() {
        picked.push(format!(
            "... ({} lines omitted)",
            input.lines().count() - max_lines
        ));
    }
    picked.join("\n")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extract::{CallRecord, ExportRecord, ImportRecord, SideEffects};

    fn sample_record(calls: &[&str], exports: &[&str]) -> FileRecord {
        FileRecord {
            language: "elixir".to_string(),
            file_path: "lib/demo.ex".to_string(),
            module_doc: None,
            exports: exports
                .iter()
                .enumerate()
                .map(|(idx, name)| ExportRecord {
                    name: (*name).to_string(),
                    kind: "function".to_string(),
                    signature: None,
                    line: idx + 1,
                })
                .collect(),
            imports: vec![ImportRecord {
                specifier: "Demo.Helper".to_string(),
                kind: "alias".to_string(),
            }],
            calls: calls
                .iter()
                .enumerate()
                .map(|(idx, name)| CallRecord {
                    callee: (*name).to_string(),
                    line: idx + 1,
                })
                .collect(),
            local_call_targets: Vec::new(),
            relation_hints: Vec::new(),
            side_effects: SideEffects {
                has_async: false,
                has_http: false,
                has_genserver: false,
                has_file_io: false,
                has_pubsub: false,
            },
            loc_lines: 1,
            type_annotations: vec![],
            type_guards: vec![],
            schema_fields: vec![],
            declarations: 0,
        }
    }

    #[test]
    fn call_and_export_names_handle_partial_input() {
        let record = sample_record(&["Demo.Worker.run"], &[]);
        assert_eq!(call_names(&record), vec!["Demo.Worker.run".to_string()]);
        assert!(export_names(&record).is_empty());
    }

    #[test]
    fn assert_contains_accepts_single_item_input() {
        let items = vec!["Demo.Worker.run".to_string()];
        assert_contains(&items, "Demo.Worker.run", "calls");
    }

    #[test]
    fn assert_contains_panics_on_missing_item_boundary() {
        let items = vec!["Demo.Worker.run".to_string()];
        let result = std::panic::catch_unwind(|| {
            assert_contains(&items, "Demo.Worker.stop", "calls");
        });
        assert!(result.is_err());
    }

    #[test]
    fn assert_import_contains_accepts_expected_import() {
        let record = sample_record(&[], &[]);
        assert_import_contains(&record, "Demo.Helper", "alias");
    }

    #[test]
    fn normalized_sets_ignore_order_and_duplicates() {
        let left = sample_record(&["B.run", "A.run", "A.run"], &[]);
        let right = sample_record(&["A.run", "B.run"], &[]);
        assert_eq!(normalized_call_set(&left), normalized_call_set(&right));
    }

    #[test]
    fn normalize_core_fields_sorts_and_dedups() {
        let mut record = sample_record(&["B.run", "A.run", "A.run"], &["Run"]);
        record.imports.push(ImportRecord {
            specifier: " Demo.Helper ".to_string(),
            kind: " alias ".to_string(),
        });
        record.imports.push(ImportRecord {
            specifier: "Demo.Helper".to_string(),
            kind: "alias".to_string(),
        });

        let value = normalize_core_fields(&record);
        let calls = value
            .get("calls")
            .and_then(|v| v.as_array())
            .expect("calls array");
        assert_eq!(calls.len(), 2);
        assert_eq!(calls[0]["callee"], "A.run");
        assert_eq!(calls[1]["callee"], "B.run");

        let imports = value
            .get("imports")
            .and_then(|v| v.as_array())
            .expect("imports array");
        assert_eq!(imports.len(), 1);
        assert_eq!(imports[0]["kind"], "alias");
        assert_eq!(imports[0]["specifier"], "Demo.Helper");
    }

    #[test]
    fn diff_core_fields_reports_mismatched_field() {
        let expected = json!({
            "calls": [{"callee":"A.run","line":1}],
            "imports": [],
            "exports": [],
            "declarations": 1,
            "loc_lines": 3
        });
        let actual = json!({
            "calls": [{"callee":"B.run","line":1}],
            "imports": [],
            "exports": [],
            "declarations": 1,
            "loc_lines": 3
        });

        let diff = diff_core_fields(&expected, &actual, 20);
        assert!(diff.contains("field `calls` mismatch"));
        assert!(diff.contains("A.run"));
        assert!(diff.contains("B.run"));
    }

    #[test]
    fn normalize_test_path_handles_absolute_and_relative() {
        let abs = std::env::current_dir()
            .unwrap()
            .join("compiler/bcc/src/lib.rs");
        let normalized = normalize_test_path(abs.to_string_lossy().as_ref());
        assert!(normalized.ends_with("compiler/bcc/src/lib.rs"));
        assert!(!normalized.contains('\\'));
    }
}
