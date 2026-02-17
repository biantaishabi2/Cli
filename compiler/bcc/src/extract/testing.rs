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
