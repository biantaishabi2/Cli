//! JSON 格式输出

use super::*;
use serde_json::json;

/// 格式化为 JSON
pub fn format(score: &ArchitectureScore, _verbose: bool) -> Result<String, String> {
    let json_value = json!({
        "version": "1.0",
        "generated_at": chrono::Utc::now().to_rfc3339(),
        "summary": {
            "total_score": score.total,
            "grade": format!("{:?}", score.grade),
            "passed": score.passed,
            "mode": score.mode,
            "threshold": 60.0,
            "issue_counts": {
                "critical": score.summary.critical_issues,
                "error": score.summary.error_issues,
                "warning": score.summary.warning_issues,
                "info": score.summary.info_issues,
                "total": score.summary.total_issues
            }
        },
        "dimensions": score.dimensions.iter().map(|d| json!({
            "name": d.name,
            "display_name": d.display_name,
            "weight": d.weight,
            "score": d.score,
            "contribution": d.contribution,
            "passed": d.passed,
            "is_blocking": d.is_blocking,
            "metrics": d.metrics,
            "issues": d.issues.iter().map(|i| json!({
                "severity": format!("{:?}", i.severity).to_lowercase(),
                "message": i.message,
                "metric": i.metric,
                "actual": i.actual,
                "threshold": i.threshold,
                "location": i.location
            })).collect::<Vec<_>>(),
            "suggestions": d.suggestions
        })).collect::<Vec<_>>(),
        "recommendations": score.recommendations.iter().map(|r| json!({
            "priority": format!("{:?}", r.priority).to_lowercase(),
            "category": r.category,
            "message": r.message,
            "affected_modules": r.affected_modules
        })).collect::<Vec<_>>()
    });

    serde_json::to_string_pretty(&json_value)
        .map_err(|e| format!("serialize json failed: {}", e))
}

/// 格式化为对比 JSON
pub fn format_comparison(results: &[(String, ArchitectureScore)]) -> Result<String, String> {
    let json_value = json!({
        "version": "1.0",
        "generated_at": chrono::Utc::now().to_rfc3339(),
        "comparison": results.iter().map(|(label, score)| json!({
            "version": label,
            "total_score": score.total,
            "grade": format!("{:?}", score.grade),
            "passed": score.passed,
            "dimensions": score.dimensions.iter().map(|d| json!({
                "name": d.name,
                "score": d.score,
                "passed": d.passed
            })).collect::<Vec<_>>()
        })).collect::<Vec<_>>()
    });

    serde_json::to_string_pretty(&json_value)
        .map_err(|e| format!("serialize json failed: {}", e))
}
