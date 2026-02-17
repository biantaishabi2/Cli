use super::FileRecord;
use std::collections::BTreeSet;

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
    record.calls.iter().map(|call| call.callee.clone()).collect()
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

fn normalize_spaces(input: &str) -> String {
    input.split_whitespace().collect::<Vec<_>>().join(" ")
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
}
