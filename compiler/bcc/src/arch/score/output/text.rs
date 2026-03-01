//! 文本格式输出

use super::*;
use std::fmt::Write;

/// 格式化为文本
pub fn format(score: &ArchitectureScore, verbose: bool) -> Result<String, String> {
    let mut output = String::new();

    // 标题
    writeln!(output, "{}", "=".repeat(80)).unwrap();
    writeln!(output, "{:^80}", "Architecture Health Score").unwrap();
    writeln!(output, "{}", "=".repeat(80)).unwrap();
    writeln!(output).unwrap();

    // 总体得分
    writeln!(output, "Overall Score: {:.1}/100 ({})", score.total, format_grade(score.grade)).unwrap();
    writeln!(output, "Status: {} {}", format_status(score.passed), if score.passed { "PASSED" } else { "FAILED" }).unwrap();
    writeln!(output, "Mode: {}", score.mode).unwrap();
    writeln!(output).unwrap();

    // 维度细分
    writeln!(output, "{}", "-".repeat(80)).unwrap();
    writeln!(output, "{:^80}", "Dimension Breakdown").unwrap();
    writeln!(output, "{}", "-".repeat(80)).unwrap();
    writeln!(output).unwrap();

    for dim in &score.dimensions {
        let progress = format_progress_bar(dim.score, 10);
        let blocking_marker = if dim.is_blocking { " [BLOCKING]" } else { "" };
        writeln!(
            output,
            "{} {:15} {:6.1}/100 {} weight: {:>3.0}% contribution: {:>5.1}{}",
            format_status(dim.passed),
            dim.display_name,
            dim.score,
            progress,
            dim.weight * 100.0,
            dim.contribution,
            blocking_marker
        ).unwrap();
    }
    writeln!(output).unwrap();

    // 问题汇总
    writeln!(output, "{}", "-".repeat(80)).unwrap();
    writeln!(output, "{:^80}", "Issues Summary").unwrap();
    writeln!(output, "{}", "-".repeat(80)).unwrap();
    writeln!(output).unwrap();

    writeln!(
        output,
        "Critical: {} | Error: {} | Warning: {} | Info: {}",
        score.summary.critical_issues,
        score.summary.error_issues,
        score.summary.warning_issues,
        score.summary.info_issues
    ).unwrap();
    writeln!(output).unwrap();

    // 详细问题（如果 verbose 或有失败）
    if verbose || !score.passed {
        for dim in &score.dimensions {
            if !dim.issues.is_empty() {
                writeln!(output, "[{}] {} ({:.1}/100):", if dim.passed { "✓" } else { "✗" }, dim.display_name, dim.score).unwrap();
                for issue in &dim.issues {
                    writeln!(
                        output,
                        "  {} {}",
                        format_severity(issue.severity),
                        issue.message
                    ).unwrap();
                    if let Some(location) = &issue.location {
                        writeln!(output, "     Location: {}", location).unwrap();
                    }
                }
                writeln!(output).unwrap();
            }
        }
    }

    // 建议
    if !score.recommendations.is_empty() {
        writeln!(output, "{}", "-".repeat(80)).unwrap();
        writeln!(output, "{:^80}", "Recommendations").unwrap();
        writeln!(output, "{}", "-".repeat(80)).unwrap();
        writeln!(output).unwrap();

        for (i, rec) in score.recommendations.iter().enumerate() {
            writeln!(output, "{}. {}", i + 1, format_priority(rec.priority)).unwrap();
            writeln!(output, "   Category: {}", rec.category).unwrap();
            writeln!(output, "   Message: {}", rec.message).unwrap();
            if !rec.affected_modules.is_empty() {
                writeln!(output, "   Affected: {}", rec.affected_modules.join(", ")).unwrap();
            }
            writeln!(output).unwrap();
        }
    }

    // 页脚
    writeln!(output, "{}", "=".repeat(80)).unwrap();

    Ok(output)
}
