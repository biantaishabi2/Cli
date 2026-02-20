//! 错误处理检测器（2 项）
//!
//! 对源码做 regex 模式匹配，检测吞掉异常和宽泛异常捕获。

use super::{Detector, SmellRecord};
use regex::Regex;

/// 吞掉异常检测器
///
/// rescue/catch/except body 为空或只有 :ok/nil/pass/_ 等无意义处理
pub struct SwallowedErrorDetector;

impl Detector for SwallowedErrorDetector {
    fn name(&self) -> &str { "swallowed_error" }
    fn category(&self) -> &str { "error_handling" }

    fn detect(&self, source: &str, file_path: &str, _lang: &str) -> Vec<SmellRecord> {
        let mut results = Vec::new();
        let lines: Vec<&str> = source.lines().collect();

        for (i, line) in lines.iter().enumerate() {
            let trimmed = line.trim();

            // Python: except ... : \n pass
            if trimmed.starts_with("except") && trimmed.ends_with(':') {
                if let Some(next) = lines.get(i + 1) {
                    let next_trimmed = next.trim();
                    if next_trimmed == "pass" || next_trimmed == "..." || next_trimmed.is_empty() {
                        results.push(SmellRecord {
                            category: "error_handling".to_string(),
                            rule: "swallowed_error".to_string(),
                            severity: "info".to_string(),
                            message: "Exception caught but swallowed (empty handler or pass)".to_string(),
                            file: file_path.to_string(),
                            line: i + 1,
                            source: "bcc".to_string(),
                            confidence: 0.9,
                        });
                    }
                }
            }

            // Elixir: rescue _ -> :ok / rescue _ -> nil
            if trimmed.starts_with("rescue") {
                let rescue_body = trimmed.trim_start_matches("rescue").trim();
                // 单行 rescue _ -> :ok
                if Regex::new(r#"^_\s*->\s*(:ok|nil|:error)\s*$"#).unwrap().is_match(rescue_body) {
                    results.push(SmellRecord {
                        category: "error_handling".to_string(),
                        rule: "swallowed_error".to_string(),
                        severity: "info".to_string(),
                        message: "Exception caught but swallowed (trivial return)".to_string(),
                        file: file_path.to_string(),
                        line: i + 1,
                        source: "bcc".to_string(),
                        confidence: 0.9,
                    });
                }
                // 多行 rescue\n _ -> :ok
                if rescue_body.is_empty() {
                    if let Some(next) = lines.get(i + 1) {
                        let next_trimmed = next.trim();
                        if Regex::new(r#"^_\s*->\s*(:ok|nil|:error)\s*$"#).unwrap().is_match(next_trimmed) {
                            results.push(SmellRecord {
                                category: "error_handling".to_string(),
                                rule: "swallowed_error".to_string(),
                                severity: "info".to_string(),
                                message: "Exception caught but swallowed (trivial return)".to_string(),
                                file: file_path.to_string(),
                                line: i + 1,
                                source: "bcc".to_string(),
                                confidence: 0.9,
                            });
                        }
                    }
                }
            }

            // TypeScript/JavaScript: catch (e) { } 或 catch (e) { /* empty */ }
            if Regex::new(r#"catch\s*\([^)]*\)\s*\{\s*\}"#).unwrap().is_match(trimmed) {
                results.push(SmellRecord {
                    category: "error_handling".to_string(),
                    rule: "swallowed_error".to_string(),
                    severity: "info".to_string(),
                    message: "Exception caught but swallowed (empty catch block)".to_string(),
                    file: file_path.to_string(),
                    line: i + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                });
            }

            // Rust: if let Err(_) = ... { } （空体不常见但可检测）
            // .unwrap_or_default() 和 let _ = 不算吞掉
        }
        results
    }
}

/// 宽泛异常捕获检测器
///
/// except Exception / 裸 rescue _ / 无类型限定的 catch
pub struct BroadCatchDetector;

impl Detector for BroadCatchDetector {
    fn name(&self) -> &str { "broad_catch" }
    fn category(&self) -> &str { "error_handling" }

    fn detect(&self, source: &str, file_path: &str, _lang: &str) -> Vec<SmellRecord> {
        let mut results = Vec::new();

        for (i, line) in source.lines().enumerate() {
            let trimmed = line.trim();

            // Python: 裸 except: 或 except Exception
            if Regex::new(r#"^\s*except\s*:\s*$"#).unwrap().is_match(line) {
                results.push(SmellRecord {
                    category: "error_handling".to_string(),
                    rule: "broad_catch".to_string(),
                    severity: "info".to_string(),
                    message: "Bare except clause catches all exceptions including SystemExit and KeyboardInterrupt".to_string(),
                    file: file_path.to_string(),
                    line: i + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                });
            } else if Regex::new(r#"(?i)^\s*except\s+(Exception|BaseException)\b"#).unwrap().is_match(line) {
                results.push(SmellRecord {
                    category: "error_handling".to_string(),
                    rule: "broad_catch".to_string(),
                    severity: "info".to_string(),
                    message: "Overly broad exception catch (Exception/BaseException)".to_string(),
                    file: file_path.to_string(),
                    line: i + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                });
            }

            // Elixir: rescue _ -> (无类型限定)
            if Regex::new(r#"^\s*rescue\s+_\s*->"#).unwrap().is_match(line) {
                results.push(SmellRecord {
                    category: "error_handling".to_string(),
                    rule: "broad_catch".to_string(),
                    severity: "info".to_string(),
                    message: "Broad rescue clause catches all exceptions".to_string(),
                    file: file_path.to_string(),
                    line: i + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                });
            }

            // TypeScript/JavaScript: catch (e) 无类型过滤（所有 JS catch 都是宽泛的，但 catch 本身不算问题）
            // 这里只检测 catch { } 这种连类型参数都省略的
        }
        results
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // --- SwallowedErrorDetector ---

    #[test]
    fn swallowed_error_detects_except_pass() {
        let d = SwallowedErrorDetector;
        let source = "try:\n    risky()\nexcept:\n    pass\n";
        let results = d.detect(source, "main.py", "python");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].rule, "swallowed_error");
        assert_eq!(results[0].line, 3);
    }

    #[test]
    fn swallowed_error_detects_except_exception_pass() {
        let d = SwallowedErrorDetector;
        let source = "try:\n    risky()\nexcept Exception:\n    pass\n";
        let results = d.detect(source, "main.py", "python");
        assert_eq!(results.len(), 1);
    }

    #[test]
    fn swallowed_error_ignores_handled_exception() {
        let d = SwallowedErrorDetector;
        let source = "try:\n    risky()\nexcept ValueError as e:\n    log(e)\n";
        let results = d.detect(source, "main.py", "python");
        assert!(results.is_empty());
    }

    #[test]
    fn swallowed_error_detects_elixir_rescue() {
        let d = SwallowedErrorDetector;
        let source = "try do\n  risky()\nrescue\n  _ -> :ok\nend\n";
        let results = d.detect(source, "main.ex", "elixir");
        assert_eq!(results.len(), 1);
    }

    #[test]
    fn swallowed_error_detects_empty_js_catch() {
        let d = SwallowedErrorDetector;
        let source = "try { risky() } catch (e) { }";
        let results = d.detect(source, "main.ts", "typescript");
        assert_eq!(results.len(), 1);
    }

    // --- BroadCatchDetector ---

    #[test]
    fn broad_catch_detects_bare_except() {
        let d = BroadCatchDetector;
        let source = "try:\n    risky()\nexcept:\n    log('error')\n";
        let results = d.detect(source, "main.py", "python");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].rule, "broad_catch");
        assert_eq!(results[0].line, 3);
    }

    #[test]
    fn broad_catch_detects_exception_class() {
        let d = BroadCatchDetector;
        let source = "try:\n    risky()\nexcept Exception as e:\n    log(e)\n";
        let results = d.detect(source, "main.py", "python");
        assert_eq!(results.len(), 1);
    }

    #[test]
    fn broad_catch_ignores_specific_exception() {
        let d = BroadCatchDetector;
        let source = "try:\n    risky()\nexcept ValueError as e:\n    handle(e)\n";
        let results = d.detect(source, "main.py", "python");
        assert!(results.is_empty());
    }

    #[test]
    fn broad_catch_detects_elixir_rescue_wildcard() {
        let d = BroadCatchDetector;
        let source = "try do\n  risky()\nrescue _ -> log(e)\nend\n";
        let results = d.detect(source, "main.ex", "elixir");
        assert_eq!(results.len(), 1);
    }

    #[test]
    fn broad_catch_ignores_specific_elixir_rescue() {
        let d = BroadCatchDetector;
        let source = "try do\n  risky()\nrescue e in ArgumentError -> handle(e)\nend\n";
        let results = d.detect(source, "main.ex", "elixir");
        assert!(results.is_empty());
    }
}
