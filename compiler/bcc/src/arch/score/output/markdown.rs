//! Markdown 格式输出

use super::*;
use std::fmt::Write;

/// 格式化为 Markdown
pub fn format(score: &ArchitectureScore, _verbose: bool) -> Result<String, String> {
    let mut output = String::new();

    // 标题
    writeln!(output, "# Architecture Health Score Report").unwrap();
    writeln!(output).unwrap();

    // 元信息
    writeln!(output, "**Generated:** {}" , chrono::Utc::now().to_rfc3339()).unwrap();
    writeln!(output, "**Overall Score:** {:.1}/100 ({})", score.total, format_grade(score.grade)).unwrap();
    writeln!(output, "**Status:** {}", if score.passed { "✅ PASSED" } else { "❌ FAILED" }).unwrap();
    writeln!(output, "**Mode:** {}", score.mode).unwrap();
    writeln!(output).unwrap();

    // 汇总
    writeln!(output, "## Summary").unwrap();
    writeln!(output).unwrap();
    writeln!(output, "| Metric | Value |").unwrap();
    writeln!(output, "|--------|-------|").unwrap();
    writeln!(output, "| Total Score | {:.1}/100 |", score.total).unwrap();
    writeln!(output, "| Grade | {} |", format_grade(score.grade)).unwrap();
    writeln!(output, "| Passed | {} |", if score.passed { "✅" } else { "❌" }).unwrap();
    writeln!(output, "| Critical Issues | {} |", score.summary.critical_issues).unwrap();
    writeln!(output, "| Errors | {} |", score.summary.error_issues).unwrap();
    writeln!(output, "| Warnings | {} |", score.summary.warning_issues).unwrap();
    writeln!(output, "| Info | {} |", score.summary.info_issues).unwrap();
    writeln!(output).unwrap();

    // 维度细分
    writeln!(output, "## Dimension Breakdown").unwrap();
    writeln!(output).unwrap();
    writeln!(output, "| Dimension | Score | Weight | Contribution | Status |").unwrap();
    writeln!(output, "|-----------|-------|--------|--------------|--------|").unwrap();

    for dim in &score.dimensions {
        let status = if dim.passed {
            "✅"
        } else if dim.is_blocking {
            "❌ BLOCKING"
        } else {
            "⚠️"
        };
        writeln!(
            output,
            "| {} | {:.1} | {:.0}% | {:.1} | {} |",
            dim.display_name, dim.score, dim.weight * 100.0, dim.contribution, status
        ).unwrap();
    }
    writeln!(output).unwrap();

    // 指标详情
    writeln!(output, "## Metrics Detail").unwrap();
    writeln!(output).unwrap();

    for dim in &score.dimensions {
        if !dim.metrics.is_empty() {
            writeln!(output, "### {}", dim.display_name).unwrap();
            writeln!(output).unwrap();
            writeln!(output, "| Metric | Value | Threshold | Status |").unwrap();
            writeln!(output, "|--------|-------|-----------|--------|").unwrap();

            for metric in &dim.metrics {
                let value_str = if metric.unit == "%" {
                    format!("{:.1}%", metric.value)
                } else {
                    format!("{:.0} {}", metric.value, metric.unit)
                };

                let threshold_str = metric.threshold.map_or("-".to_string(), |t| {
                    if metric.unit == "%" {
                        format!("{:.1}%", t)
                    } else {
                        format!("{:.0}", t)
                    }
                });

                let status_str = metric.passed.map_or("-".to_string(), |p| {
                    if p { "✅".to_string() } else { "❌".to_string() }
                });

                writeln!(
                    output,
                    "| {} | {} | {} | {} |",
                    metric.display_name.as_ref().unwrap_or(&metric.name),
                    value_str,
                    threshold_str,
                    status_str
                ).unwrap();
            }
            writeln!(output).unwrap();
        }
    }

    // 问题
    let all_issues: Vec<_> = score.dimensions.iter()
        .flat_map(|d| d.issues.iter().map(move |i| (d, i)))
        .collect();

    if !all_issues.is_empty() {
        writeln!(output, "## Issues").unwrap();
        writeln!(output).unwrap();

        for (dim, issue) in all_issues {
            writeln!(
                output,
                "- **{}** [{}] {}: {}",
                format_severity(issue.severity),
                dim.display_name,
                issue.metric.as_ref().unwrap_or(&"general".to_string()),
                issue.message
            ).unwrap();

            if let Some(actual) = issue.actual {
                if let Some(threshold) = issue.threshold {
                    writeln!(output, "  - Actual: {:.1}, Threshold: {:.1}", actual, threshold).unwrap();
                }
            }

            if let Some(location) = &issue.location {
                writeln!(output, "  - Location: `{}`", location).unwrap();
            }
        }
        writeln!(output).unwrap();
    }

    // 建议
    if !score.recommendations.is_empty() {
        writeln!(output, "## Recommendations").unwrap();
        writeln!(output).unwrap();

        for (i, rec) in score.recommendations.iter().enumerate() {
            writeln!(output, "{}. **{}** - {}", i + 1, rec.category, rec.message).unwrap();
            writeln!(output, "   - Priority: {}", format_priority(rec.priority)).unwrap();
            if !rec.affected_modules.is_empty() {
                writeln!(output, "   - Affected modules: {}", rec.affected_modules.join(", ")).unwrap();
            }
            writeln!(output).unwrap();
        }
    }

    // 维度建议
    let dims_with_suggestions: Vec<_> = score.dimensions.iter()
        .filter(|d| !d.suggestions.is_empty())
        .collect();

    if !dims_with_suggestions.is_empty() {
        writeln!(output, "## Suggestions by Dimension").unwrap();
        writeln!(output).unwrap();

        for dim in dims_with_suggestions {
            writeln!(output, "### {}", dim.display_name).unwrap();
            writeln!(output).unwrap();
            for suggestion in &dim.suggestions {
                writeln!(output, "- {}", suggestion).unwrap();
            }
            writeln!(output).unwrap();
        }
    }

    Ok(output)
}

/// 格式化为对比 Markdown
pub fn format_comparison(results: &[(String, ArchitectureScore)]) -> Result<String, String> {
    let mut output = String::new();

    writeln!(output, "# Architecture Score Comparison").unwrap();
    writeln!(output).unwrap();
    writeln!(output, "**Generated:** {}", chrono::Utc::now().to_rfc3339()).unwrap();
    writeln!(output).unwrap();

    // 总分对比
    writeln!(output, "## Overall Score Comparison").unwrap();
    writeln!(output).unwrap();
    writeln!(output, "| Version | Score | Grade | Status |").unwrap();
    writeln!(output, "|---------|-------|-------|--------|").unwrap();

    for (label, score) in results {
        writeln!(
            output,
            "| {} | {:.1} | {} | {} |",
            label,
            score.total,
            format_grade(score.grade),
            if score.passed { "✅" } else { "❌" }
        ).unwrap();
    }
    writeln!(output).unwrap();

    // 维度对比
    writeln!(output, "## Dimension Score Comparison").unwrap();
    writeln!(output).unwrap();

    // 获取所有维度名称
    let dim_names: Vec<_> = results[0].1.dimensions.iter()
        .map(|d| d.name.clone())
        .collect();

    // 表头
    write!(output, "| Dimension |").unwrap();
    for (label, _) in results {
        write!(output, " {} |", label).unwrap();
    }
    writeln!(output).unwrap();

    // 分隔线
    write!(output, "|-----------|").unwrap();
    for _ in results {
        write!(output, "-------|").unwrap();
    }
    writeln!(output).unwrap();

    // 数据行
    for dim_name in &dim_names {
        let dim_display = results[0].1.dimensions.iter()
            .find(|d| &d.name == dim_name)
            .map(|d| d.display_name.clone())
            .unwrap_or_else(|| dim_name.clone());

        write!(output, "| {} |", dim_display).unwrap();

        for (_, score) in results {
            let dim = score.dimensions.iter()
                .find(|d| &d.name == dim_name)
                .map(|d| format!("{:.1} {}", d.score, if d.passed { "✅" } else { "❌" }))
                .unwrap_or_else(|| "-".to_string());
            write!(output, " {} |", dim).unwrap();
        }
        writeln!(output).unwrap();
    }

    writeln!(output).unwrap();

    // 趋势分析
    if results.len() >= 2 {
        writeln!(output, "## Trend Analysis").unwrap();
        writeln!(output).unwrap();

        let first = &results[0].1;
        let last = &results[results.len() - 1].1;
        let change = last.total - first.total;

        if change > 0.0 {
            writeln!(output, "📈 **Improved** by {:.1} points from {} to {}", change, results[0].0, results[results.len() - 1].0).unwrap();
        } else if change < 0.0 {
            writeln!(output, "📉 **Declined** by {:.1} points from {} to {}", change.abs(), results[0].0, results[results.len() - 1].0).unwrap();
        } else {
            writeln!(output, "➡️ **Stable** - no significant change").unwrap();
        }
        writeln!(output).unwrap();
    }

    Ok(output)
}
