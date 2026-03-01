//! 错误处理检测器（2 项）
//!
//! 对源码做 regex 模式匹配，检测吞掉异常和宽泛异常捕获。

use super::{extract_code_snippet, Detector, SmellRecord};
use regex::Regex;

/// 吞掉异常检测器
///
/// rescue/catch/except body 为空或只有 :ok/nil/pass/_ 等无意义处理
pub struct SwallowedErrorDetector;

impl Detector for SwallowedErrorDetector {
    fn name(&self) -> &str {
        "swallowed_error"
    }
    fn category(&self) -> &str {
        "error_handling"
    }

    fn detect(&self, source: &str, file_path: &str, lang: &str) -> Vec<SmellRecord> {
        let mut results = Vec::new();
        let lines: Vec<&str> = source.lines().collect();

        // 预编译正则，避免在循环中重复创建
        let elixir_trivial_re = Regex::new(r#"^_\s*->\s*(:ok|nil|:error)\s*$"#).unwrap();
        let js_empty_catch_re = Regex::new(r#"catch\s*\([^)]*\)\s*\{\s*\}"#).unwrap();
        // Rust: let _ = expr; 丢弃 Result（常见的静默吞掉错误模式）
        let rust_let_discard_re = Regex::new(r#"^\s*let\s+_\s*=\s*.+"#).unwrap();

        for (i, line) in lines.iter().enumerate() {
            let trimmed = line.trim();

            // Rust: let _ = expr; 丢弃返回值（静默吞掉错误）
            if lang == "rust" && rust_let_discard_re.is_match(line) {
                results.push(SmellRecord {
                    category: "error_handling".to_string(),
                    rule: "swallowed_error".to_string(),
                    severity: "critical".to_string(),
                    message: "Discarding Result/Error with `let _ =` silently swallows errors"
                        .to_string(),
                    file: file_path.to_string(),
                    line: i + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                    fix_hint:
                        "Handle the error explicitly or use .ok() with a comment explaining why"
                            .to_string(),
                    code_snippet: extract_code_snippet(source, i + 1, 2),
                    offending_code: line.trim().to_string(),
                    suggested_fix: "match expr { Ok(_) => {}, Err(e) => log::warn!(\"{}\", e) }"
                        .to_string(),
                    evidence: vec![],
                });
                continue;
            }

            // Python: except ... : \n pass/...
            // 跳过空行和注释行找到第一个实际语句来判断
            if lang == "python" && trimmed.starts_with("except") && trimmed.ends_with(':') {
                let mut next_idx = i + 1;
                while next_idx < lines.len() {
                    let t = lines[next_idx].trim();
                    if t.is_empty() || t.starts_with('#') {
                        next_idx += 1;
                    } else {
                        break;
                    }
                }
                if let Some(next) = lines.get(next_idx) {
                    let next_trimmed = next.trim();
                    if next_trimmed == "pass" || next_trimmed == "..." {
                        results.push(SmellRecord {
                            category: "error_handling".to_string(),
                            rule: "swallowed_error".to_string(),
                            severity: "critical".to_string(),
                            message: "Exception caught but swallowed (empty handler or pass)"
                                .to_string(),
                            file: file_path.to_string(),
                            line: i + 1,
                            source: "bcc".to_string(),
                            confidence: 0.9,
                            fix_hint: "Log the exception or re-raise it".to_string(),
                            code_snippet: extract_code_snippet(source, i + 1, 2),
                            offending_code: trimmed.to_string(),
                            suggested_fix: "except ValueError as e:\n    logger.error(e)"
                                .to_string(),
                            evidence: vec![],
                        });
                    }
                }
            }

            // Elixir: rescue _ -> :ok / rescue _ -> nil
            if lang == "elixir" && trimmed.starts_with("rescue") {
                let rescue_body = trimmed.trim_start_matches("rescue").trim();
                // 单行 rescue _ -> :ok
                if elixir_trivial_re.is_match(rescue_body) {
                    results.push(SmellRecord {
                        category: "error_handling".to_string(),
                        rule: "swallowed_error".to_string(),
                        severity: "critical".to_string(),
                        message: "Exception caught but swallowed (trivial return)".to_string(),
                        file: file_path.to_string(),
                        line: i + 1,
                        source: "bcc".to_string(),
                        confidence: 0.9,
                        fix_hint: "Log the exception or handle it meaningfully".to_string(),
                        code_snippet: extract_code_snippet(source, i + 1, 2),
                        offending_code: trimmed.to_string(),
                        suggested_fix: "rescue e -> Logger.error(Exception.message(e))".to_string(),
                        evidence: vec![],
                    });
                }
                // 多行 rescue\n _ -> :ok
                if rescue_body.is_empty() {
                    if let Some(next) = lines.get(i + 1) {
                        let next_trimmed = next.trim();
                        if elixir_trivial_re.is_match(next_trimmed) {
                            results.push(SmellRecord {
                                category: "error_handling".to_string(),
                                rule: "swallowed_error".to_string(),
                                severity: "critical".to_string(),
                                message: "Exception caught but swallowed (trivial return)"
                                    .to_string(),
                                file: file_path.to_string(),
                                line: i + 1,
                                source: "bcc".to_string(),
                                confidence: 0.9,
                                fix_hint: "Log the exception or handle it meaningfully".to_string(),
                                code_snippet: extract_code_snippet(source, i + 1, 2),
                                offending_code: trimmed.to_string(),
                                suggested_fix: "rescue e -> Logger.error(Exception.message(e))"
                                    .to_string(),
                                evidence: vec![],
                            });
                        }
                    }
                }
            }

            // TypeScript/JavaScript: catch (e) { } 或 catch (e) { /* empty */ }
            if (lang == "typescript" || lang == "javascript") && js_empty_catch_re.is_match(trimmed)
            {
                results.push(SmellRecord {
                    category: "error_handling".to_string(),
                    rule: "swallowed_error".to_string(),
                    severity: "critical".to_string(),
                    message: "Exception caught but swallowed (empty catch block)".to_string(),
                    file: file_path.to_string(),
                    line: i + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                    fix_hint: "Handle the error or add a comment explaining why it is ignored"
                        .to_string(),
                    code_snippet: extract_code_snippet(source, i + 1, 2),
                    offending_code: trimmed.to_string(),
                    suggested_fix: "catch (e) { console.error(e); }".to_string(),
                    evidence: vec![],
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
    fn name(&self) -> &str {
        "broad_catch"
    }
    fn category(&self) -> &str {
        "error_handling"
    }

    fn detect(&self, source: &str, file_path: &str, lang: &str) -> Vec<SmellRecord> {
        let mut results = Vec::new();

        // 预编译正则，避免在循环中重复创建
        let bare_except_re = Regex::new(r#"^\s*except\s*:\s*$"#).unwrap();
        let broad_except_re =
            Regex::new(r#"(?i)^\s*except\s+(Exception|BaseException)\b"#).unwrap();
        let elixir_rescue_inline_re = Regex::new(r#"^\s*rescue\s+_\s*->"#).unwrap();
        let elixir_rescue_alone_re = Regex::new(r#"^\s*rescue\s*$"#).unwrap();
        let elixir_wildcard_clause_re = Regex::new(r#"^\s*_\s*->"#).unwrap();
        let js_catch_no_param_re = Regex::new(r#"catch\s*\{"#).unwrap();

        let lines: Vec<&str> = source.lines().collect();

        for (i, line) in lines.iter().enumerate() {
            // Python: 裸 except: 或 except Exception
            if lang == "python" && bare_except_re.is_match(line) {
                results.push(SmellRecord {
                    category: "error_handling".to_string(),
                    rule: "broad_catch".to_string(),
                    severity: "warning".to_string(),
                    message: "Bare except clause catches all exceptions including SystemExit and KeyboardInterrupt".to_string(),
                    file: file_path.to_string(),
                    line: i + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                    fix_hint: "Catch specific exception types instead of using bare except".to_string(),
                    code_snippet: extract_code_snippet(source, i + 1, 2),
                    offending_code: line.trim().to_string(),
                    suggested_fix: "except ValueError as e:".to_string(),
                    evidence: vec![],
                });
            } else if lang == "python" && broad_except_re.is_match(line) {
                results.push(SmellRecord {
                    category: "error_handling".to_string(),
                    rule: "broad_catch".to_string(),
                    severity: "warning".to_string(),
                    message: "Overly broad exception catch (Exception/BaseException)".to_string(),
                    file: file_path.to_string(),
                    line: i + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                    fix_hint: "Catch specific exception types instead of Exception/BaseException"
                        .to_string(),
                    code_snippet: extract_code_snippet(source, i + 1, 2),
                    offending_code: line.trim().to_string(),
                    suggested_fix: "except (ValueError, KeyError) as e:".to_string(),
                    evidence: vec![],
                });
            }

            // Elixir: rescue _ -> (单行或多行)
            if lang == "elixir" && elixir_rescue_inline_re.is_match(line) {
                results.push(SmellRecord {
                    category: "error_handling".to_string(),
                    rule: "broad_catch".to_string(),
                    severity: "warning".to_string(),
                    message: "Broad rescue clause catches all exceptions".to_string(),
                    file: file_path.to_string(),
                    line: i + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                    fix_hint: "Rescue specific exception types instead of wildcard _".to_string(),
                    code_snippet: extract_code_snippet(source, i + 1, 2),
                    offending_code: line.trim().to_string(),
                    suggested_fix: "rescue e in ArgumentError -> handle(e)".to_string(),
                    evidence: vec![],
                });
            } else if lang == "elixir" && elixir_rescue_alone_re.is_match(line) {
                // 多行写法: rescue\n  _ -> ...
                if let Some(next) = lines.get(i + 1) {
                    if elixir_wildcard_clause_re.is_match(next) {
                        results.push(SmellRecord {
                            category: "error_handling".to_string(),
                            rule: "broad_catch".to_string(),
                            severity: "warning".to_string(),
                            message: "Broad rescue clause catches all exceptions".to_string(),
                            file: file_path.to_string(),
                            line: i + 1,
                            source: "bcc".to_string(),
                            confidence: 0.9,
                            fix_hint: "Rescue specific exception types instead of wildcard _"
                                .to_string(),
                            code_snippet: extract_code_snippet(source, i + 1, 2),
                            offending_code: line.trim().to_string(),
                            suggested_fix: "rescue e in ArgumentError -> handle(e)".to_string(),
                            evidence: vec![],
                        });
                    }
                }
            }

            // TypeScript/JavaScript: catch { } 无参数的 catch（省略异常变量）
            if (lang == "typescript" || lang == "javascript") && js_catch_no_param_re.is_match(line)
            {
                results.push(SmellRecord {
                    category: "error_handling".to_string(),
                    rule: "broad_catch".to_string(),
                    severity: "warning".to_string(),
                    message: "Catch block without exception parameter".to_string(),
                    file: file_path.to_string(),
                    line: i + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                    fix_hint: "Add an exception parameter to the catch block".to_string(),
                    code_snippet: extract_code_snippet(source, i + 1, 2),
                    offending_code: line.trim().to_string(),
                    suggested_fix: "catch (e) { handleError(e); }".to_string(),
                    evidence: vec![],
                });
            }
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
    fn broad_catch_detects_multiline_elixir_rescue() {
        let d = BroadCatchDetector;
        let source = "try do\n  risky()\nrescue\n  _ -> :ok\nend\n";
        let results = d.detect(source, "main.ex", "elixir");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].rule, "broad_catch");
    }

    #[test]
    fn broad_catch_ignores_specific_elixir_rescue() {
        let d = BroadCatchDetector;
        let source = "try do\n  risky()\nrescue e in ArgumentError -> handle(e)\nend\n";
        let results = d.detect(source, "main.ex", "elixir");
        assert!(results.is_empty());
    }

    #[test]
    fn broad_catch_detects_js_catch_no_param() {
        let d = BroadCatchDetector;
        let source = "try { risky() } catch { handleError() }";
        let results = d.detect(source, "main.ts", "typescript");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].rule, "broad_catch");
    }

    #[test]
    fn broad_catch_ignores_js_catch_with_param() {
        let d = BroadCatchDetector;
        let source = "try { risky() } catch (e) { handleError(e) }";
        let results = d.detect(source, "main.ts", "typescript");
        assert!(results.is_empty());
    }

    #[test]
    fn swallowed_error_detects_comment_then_pass() {
        let d = SwallowedErrorDetector;
        // except: 后第一行是注释，第二行是 pass → 应检出
        let source = "try:\n    risky()\nexcept:\n    # ignore error\n    pass\n";
        let results = d.detect(source, "main.py", "python");
        assert_eq!(
            results.len(),
            1,
            "Comment followed by pass should be detected as swallowed error"
        );
        assert_eq!(results[0].rule, "swallowed_error");
    }

    #[test]
    fn swallowed_error_no_false_positive_on_empty_line() {
        let d = SwallowedErrorDetector;
        // except: 后跟空行再跟有意义的处理代码 → 不应误报
        let source = "try:\n    risky()\nexcept:\n\n    log(e)\n";
        let results = d.detect(source, "main.py", "python");
        assert!(
            results.is_empty(),
            "Empty line after except should not trigger false positive, got: {:?}",
            results
        );
    }

    // --- Rust 支持测试 ---

    #[test]
    fn swallowed_error_detects_rust_let_discard() {
        let d = SwallowedErrorDetector;
        let source = "fn main() {\n    let _ = fs::remove_file(\"tmp.txt\");\n}\n";
        let results = d.detect(source, "main.rs", "rust");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].rule, "swallowed_error");
        assert_eq!(results[0].line, 2);
    }

    #[test]
    fn swallowed_error_ignores_rust_normal_let() {
        let d = SwallowedErrorDetector;
        let source = "fn main() {\n    let result = fs::read_to_string(\"f.txt\");\n}\n";
        let results = d.detect(source, "main.rs", "rust");
        assert!(results.is_empty());
    }

    #[test]
    fn broad_catch_not_applicable_for_rust() {
        let d = BroadCatchDetector;
        // Rust 无异常捕获，broad_catch 不适用
        let source =
            "fn main() {\n    match result {\n        Err(_) => {}\n        _ => {}\n    }\n}\n";
        let results = d.detect(source, "main.rs", "rust");
        assert!(results.is_empty());
    }
}
