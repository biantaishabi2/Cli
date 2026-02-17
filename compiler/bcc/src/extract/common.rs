use std::collections::HashSet;
use tree_sitter::{Node, Parser, Tree};

use super::{CallRecord, SideEffects};

/// 统一 Parser 初始化与语法树解析逻辑。
pub fn parse_tree<T>(content: &str, language: T, lang_name: &str) -> Tree
where
    T: Into<tree_sitter::Language>,
{
    let mut parser = Parser::new();
    parser
        .set_language(&language.into())
        .unwrap_or_else(|_| panic!("failed to set {} grammar", lang_name));
    parser
        .parse(content, None)
        .unwrap_or_else(|| panic!("failed to parse {} source", lang_name))
}

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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn dedup_calls_handles_empty_input() {
        let mut calls = Vec::new();
        dedup_calls_by_callee(&mut calls);
        assert!(calls.is_empty());
    }

    #[test]
    fn parse_tree_parses_minimal_rust_source() {
        let tree = parse_tree("fn main() {}", tree_sitter_rust::LANGUAGE, "rust");
        assert!(!tree.root_node().is_error());
    }

    #[test]
    fn dedup_sort_calls_handles_deep_call_chain_boundary() {
        let mut calls = Vec::new();
        // 构造较深调用链长度，验证在边界规模下仍可稳定去重并排序。
        for depth in (0..2048).rev() {
            calls.push(CallRecord {
                callee: format!("Mod{}.run", depth % 64),
                line: depth + 1,
            });
        }

        dedup_sort_calls(&mut calls);

        assert_eq!(calls.len(), 64);
        assert!(calls.windows(2).all(|w| w[0].line <= w[1].line));
    }

    #[test]
    fn empty_side_effects_defaults_to_all_false() {
        let se = empty_side_effects();
        assert!(!se.has_async);
        assert!(!se.has_http);
        assert!(!se.has_genserver);
        assert!(!se.has_file_io);
        assert!(!se.has_pubsub);
    }
}
