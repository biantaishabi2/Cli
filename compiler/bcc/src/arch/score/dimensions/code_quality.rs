//! 代码质量维度 — 基于 SmellReport 评分
//!
//! 公式: 100 - (critical×20 + error×10 + warning×2 + info×0)，下限 0

use crate::analyze::{SmellRecord, SmellReport};
use crate::arch::score::context::ScoringContext;
use crate::arch::score::dimensions::ScoringDimension;
use crate::arch::score::models::*;
use std::fs;

/// 代码质量维度
pub struct CodeQualityDimension {
    weight: f64,
    blocking: bool,
    threshold: f64,
    smell_report_path: Option<String>,
}

impl CodeQualityDimension {
    pub fn new(weight: f64, blocking: bool, threshold: f64, smell_report_path: Option<String>) -> Self {
        Self {
            weight,
            blocking,
            threshold,
            smell_report_path,
        }
    }

    /// 从 SmellReport JSON 文件加载所有 smell
    /// 返回 Ok(smells) 或 Err(reason)；未指定路径时返回 Ok(空)
    fn load_smells(&self) -> Result<Vec<SmellRecord>, String> {
        let path = match &self.smell_report_path {
            Some(p) => p,
            None => return Ok(vec![]),
        };
        let content = fs::read_to_string(path)
            .map_err(|e| format!("cannot read {}: {}", path, e))?;
        let reports: Vec<SmellReport> = serde_json::from_str(&content)
            .map_err(|e| format!("cannot parse {}: {}", path, e))?;
        Ok(reports.into_iter().flat_map(|r| r.smells).collect())
    }
}

impl ScoringDimension for CodeQualityDimension {
    fn name(&self) -> &str {
        "code_quality"
    }

    fn display_name(&self) -> &str {
        "Code Quality"
    }

    fn weight(&self) -> f64 {
        self.weight
    }

    fn is_blocking(&self) -> bool {
        self.blocking
    }

    fn calculate(&self, _ctx: &ScoringContext) -> DimensionResult {
        let smells = match self.load_smells() {
            Ok(s) => s,
            Err(reason) => {
                eprintln!("[code_quality] error: {}", reason);
                // 读取/解析失败 → 评分 0、不通过，防止门禁误放行
                let mut result = DimensionResult::new(0.0);
                result.passed = false;
                result.issues.push(Issue {
                    severity: Severity::Critical,
                    message: format!("Failed to load smell report: {}", reason),
                    metric: None,
                    actual: None,
                    threshold: None,
                    location: None,
                });
                return result;
            }
        };

        if smells.is_empty() && self.smell_report_path.is_none() {
            // 未指定 smell-report，返回满分（不影响总分）
            return DimensionResult::new(100.0);
        }

        let critical = smells.iter().filter(|s| s.severity == "critical").count();
        let error_count = smells.iter().filter(|s| s.severity == "error").count();
        let warning = smells.iter().filter(|s| s.severity == "warning").count();
        let info = smells.iter().filter(|s| s.severity == "info").count();

        let penalty = (critical as f64 * 20.0)
            + (error_count as f64 * 10.0)
            + (warning as f64 * 2.0);

        let score = (100.0 - penalty).max(0.0);
        // critical smell 存在时直接判定为不通过（blocking 语义）
        let passed = score >= self.threshold && critical == 0;

        let mut result = DimensionResult {
            score,
            passed,
            metrics: vec![
                Metric {
                    name: "critical_smells".to_string(),
                    display_name: Some("Critical Smells".to_string()),
                    value: critical as f64,
                    threshold: Some(0.0),
                    unit: String::new(),
                    passed: Some(critical == 0),
                },
                Metric {
                    name: "warning_smells".to_string(),
                    display_name: Some("Warning Smells".to_string()),
                    value: warning as f64,
                    threshold: None,
                    unit: String::new(),
                    passed: None,
                },
                Metric {
                    name: "total_smells".to_string(),
                    display_name: Some("Total Smells".to_string()),
                    value: smells.len() as f64,
                    threshold: None,
                    unit: String::new(),
                    passed: None,
                },
            ],
            issues: vec![],
            suggestions: vec![],
        };

        if critical > 0 {
            result.issues.push(Issue {
                severity: Severity::Critical,
                message: format!("{} critical code smell(s) detected", critical),
                metric: Some("critical_smells".to_string()),
                actual: Some(critical as f64),
                threshold: Some(0.0),
                location: None,
            });
            result
                .suggestions
                .push("Fix critical security/error handling issues first".to_string());
        }

        if warning > 0 {
            result.issues.push(Issue {
                severity: Severity::Warning,
                message: format!("{} warning-level code smell(s) detected", warning),
                metric: Some("warning_smells".to_string()),
                actual: Some(warning as f64),
                threshold: None,
                location: None,
            });
        }

        result
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write as _;

    fn make_smell_report_file(smells: Vec<SmellRecord>) -> tempfile::NamedTempFile {
        let reports = vec![SmellReport {
            file: "test.py".to_string(),
            smells,
            summary: crate::analyze::SmellSummary {
                total_smells: 0,
                by_severity: std::collections::HashMap::new(),
                by_category: std::collections::HashMap::new(),
            },
        }];
        let json = serde_json::to_string(&reports).unwrap();
        let mut f = tempfile::NamedTempFile::new().unwrap();
        f.write_all(json.as_bytes()).unwrap();
        f
    }

    fn make_smell(severity: &str) -> SmellRecord {
        SmellRecord {
            category: "security".to_string(),
            rule: "test".to_string(),
            severity: severity.to_string(),
            message: "test smell".to_string(),
            file: "test.py".to_string(),
            line: 1,
            source: "bcc".to_string(),
            confidence: 1.0,
            fix_hint: String::new(),
            code_snippet: String::new(),
            offending_code: String::new(),
            suggested_fix: String::new(),
            evidence: vec![],
        }
    }

    #[test]
    fn no_smell_report_returns_100() {
        let dim = CodeQualityDimension::new(0.10, true, 60.0, None);
        let ctx = ScoringContext::default();
        let result = dim.calculate(&ctx);
        assert_eq!(result.score, 100.0);
    }

    #[test]
    fn empty_smells_returns_100() {
        let f = make_smell_report_file(vec![]);
        let dim =
            CodeQualityDimension::new(0.10, true, 60.0, Some(f.path().to_str().unwrap().to_string()));
        let ctx = ScoringContext::default();
        let result = dim.calculate(&ctx);
        assert_eq!(result.score, 100.0);
    }

    #[test]
    fn critical_and_warning_scoring() {
        // 1 critical + 2 warning = 20 + 4 = 24 penalty → score 76
        let smells = vec![
            make_smell("critical"),
            make_smell("warning"),
            make_smell("warning"),
        ];
        let f = make_smell_report_file(smells);
        let dim =
            CodeQualityDimension::new(0.10, true, 60.0, Some(f.path().to_str().unwrap().to_string()));
        let ctx = ScoringContext::default();
        let result = dim.calculate(&ctx);
        assert_eq!(result.score, 76.0);
        assert!(!result.passed); // score >= 60 但有 critical，仍不通过
    }

    #[test]
    fn many_critical_floors_at_zero() {
        let smells = (0..10).map(|_| make_smell("critical")).collect();
        let f = make_smell_report_file(smells);
        let dim =
            CodeQualityDimension::new(0.10, true, 60.0, Some(f.path().to_str().unwrap().to_string()));
        let ctx = ScoringContext::default();
        let result = dim.calculate(&ctx);
        assert_eq!(result.score, 0.0);
        assert!(!result.passed);
    }

    #[test]
    fn file_not_found_fails_with_zero() {
        let dim = CodeQualityDimension::new(0.10, true, 60.0, Some("/nonexistent/path.json".to_string()));
        let ctx = ScoringContext::default();
        let result = dim.calculate(&ctx);
        assert_eq!(result.score, 0.0);
        assert!(!result.passed);
        assert!(!result.issues.is_empty(), "should have an error issue");
        assert!(
            result.issues[0].message.contains("Failed to load smell report"),
            "issue message should indicate load failure"
        );
    }

    #[test]
    fn invalid_json_fails_with_zero() {
        let mut f = tempfile::NamedTempFile::new().unwrap();
        f.write_all(b"not valid json{{{").unwrap();
        let dim =
            CodeQualityDimension::new(0.10, true, 60.0, Some(f.path().to_str().unwrap().to_string()));
        let ctx = ScoringContext::default();
        let result = dim.calculate(&ctx);
        assert_eq!(result.score, 0.0);
        assert!(!result.passed);
        assert!(!result.issues.is_empty(), "should have an error issue");
        assert!(
            result.issues[0].message.contains("Failed to load smell report"),
            "issue message should indicate parse failure"
        );
    }
}
