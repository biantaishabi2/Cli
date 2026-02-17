use std::collections::HashSet;
use tree_sitter::Node;

use super::{CallRecord, SideEffects};

/// 统一读取 AST 节点文本，避免各语言实现重复样板代码。
pub fn node_text(node: Node, source: &[u8]) -> String {
    node.utf8_text(source).unwrap_or("").to_string()
}

/// 按 callee 去重并按首次出现顺序保留。
pub fn dedup_calls_by_callee(calls: &mut Vec<CallRecord>) {
    let mut seen = HashSet::new();
    calls.retain(|call| seen.insert(call.callee.clone()));
}

/// 按 callee 去重后再稳定排序，适合测试和快照输出。
pub fn dedup_sort_calls(calls: &mut Vec<CallRecord>) {
    dedup_calls_by_callee(calls);
    calls.sort_by(|a, b| a.line.cmp(&b.line).then_with(|| a.callee.cmp(&b.callee)));
}

/// 初始化空副作用标签。
pub fn empty_side_effects() -> SideEffects {
    SideEffects {
        has_async: false,
        has_http: false,
        has_genserver: false,
        has_file_io: false,
        has_pubsub: false,
    }
}
