#[cfg(test)]
use super::FileRecord;

/// 提取调用名列表，便于测试断言。
#[cfg(test)]
pub fn call_names(record: &FileRecord) -> Vec<String> {
    record
        .calls
        .iter()
        .map(|call| call.callee.clone())
        .collect()
}

/// 提取导出名列表，便于测试断言。
#[cfg(test)]
pub fn export_names(record: &FileRecord) -> Vec<String> {
    record
        .exports
        .iter()
        .map(|export| export.name.clone())
        .collect()
}

/// 统一断言 imports 中存在指定 specifier+kind 组合。
#[cfg(test)]
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
#[cfg(test)]
pub fn assert_contains(items: &[String], expected: &str, label: &str) {
    assert!(
        items.iter().any(|item| item == expected),
        "{} should contain '{}', actual: {:?}",
        label,
        expected,
        items
    );
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
}
