//! 输出格式化模块

use super::models::{ArchitectureScore, Grade, Priority, Severity};

mod json;
mod markdown;
mod text;

pub use json::{format as format_json, format_comparison as format_comparison_json};
pub use markdown::{format as format_markdown, format_comparison as format_comparison_markdown};
pub use text::format as format_text;

/// 格式化进度条
pub fn format_progress_bar(score: f64, width: usize) -> String {
    let filled = ((score / 100.0) * width as f64).round() as usize;
    let empty = width - filled;
    format!(
        "[{}{}]",
        "█".repeat(filled),
        "░".repeat(empty)
    )
}

/// 格式化状态图标
pub fn format_status(passed: bool) -> &'static str {
    if passed { "✅" } else { "❌" }
}

/// 格式化严重级别图标
pub fn format_severity(severity: Severity) -> &'static str {
    severity.emoji()
}

/// 格式化优先级
pub fn format_priority(priority: Priority) -> String {
    match priority {
        Priority::High => "🔴 High".to_string(),
        Priority::Medium => "🟡 Medium".to_string(),
        Priority::Low => "🟢 Low".to_string(),
    }
}

/// 格式化等级
pub fn format_grade(grade: Grade) -> String {
    format!("{} {}", grade.emoji(), grade.display_name())
}

/// 截断字符串
pub fn truncate(s: &str, max_len: usize) -> String {
    if s.len() <= max_len {
        s.to_string()
    } else {
        format!("{}...", &s[..max_len - 3])
    }
}

/// 格式化数字为百分比
pub fn format_percent(value: f64) -> String {
    format!("{:.1}%", value)
}

/// 格式化数字为固定小数位
pub fn format_decimal(value: f64, decimals: usize) -> String {
    format!("{:.1$}", value, decimals)
}
