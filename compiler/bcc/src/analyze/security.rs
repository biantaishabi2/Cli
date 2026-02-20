//! 安全模式检测器（5 项）
//!
//! 对源码做 regex 模式匹配，检测常见安全问题。

use super::{Detector, SmellRecord};
use regex::Regex;

/// 硬编码凭证检测器
///
/// 变量名含 key/secret/token/password + 赋值为字符串字面量
pub struct HardcodedCredentialDetector;

impl Detector for HardcodedCredentialDetector {
    fn name(&self) -> &str { "hardcoded_credential" }
    fn category(&self) -> &str { "security" }

    fn detect(&self, source: &str, file_path: &str, _lang: &str) -> Vec<SmellRecord> {
        // 匹配：变量名含敏感词 + 赋值为非空字符串字面量
        let re = Regex::new(
            r#"(?i)(api[_-]?key|secret[_-]?key|auth[_-]?token|password|access[_-]?token|private[_-]?key|client[_-]?secret)\s*[=:]\s*["'][^"']{8,}["']"#
        ).unwrap();

        let mut results = Vec::new();
        for (line_num, line) in source.lines().enumerate() {
            if re.is_match(line) {
                // 排除从环境变量读取的模式
                let env_re = Regex::new(r#"(?i)(environ|getenv|env\[|env\.get|ENV\[|System\.get_env|std::env)"#).unwrap();
                if env_re.is_match(line) {
                    continue;
                }
                results.push(SmellRecord {
                    category: "security".to_string(),
                    rule: "hardcoded_credential".to_string(),
                    severity: "warning".to_string(),
                    message: format!("Hardcoded credential detected"),
                    file: file_path.to_string(),
                    line: line_num + 1,
                    source: "bcc".to_string(),
                    confidence: 0.7,
                });
            }
        }
        results
    }
}

/// 注入风险检测器
///
/// 字符串拼接/插值出现在 execute()/query()/system() 参数中
pub struct InjectionRiskDetector;

impl Detector for InjectionRiskDetector {
    fn name(&self) -> &str { "injection_risk" }
    fn category(&self) -> &str { "security" }

    fn detect(&self, source: &str, file_path: &str, _lang: &str) -> Vec<SmellRecord> {
        // 匹配危险函数 + 字符串拼接（+）或 f-string 插值
        let re = Regex::new(
            r#"(?i)\b(execute|query|system|exec|eval|raw_query|run_query)\s*\([^)]*(\+|\.format\(|f["']|\$\{|#\{)"#
        ).unwrap();

        let mut results = Vec::new();
        for (line_num, line) in source.lines().enumerate() {
            if re.is_match(line) {
                results.push(SmellRecord {
                    category: "security".to_string(),
                    rule: "injection_risk".to_string(),
                    severity: "warning".to_string(),
                    message: format!("Potential injection risk: string concatenation/interpolation in dangerous function"),
                    file: file_path.to_string(),
                    line: line_num + 1,
                    source: "bcc".to_string(),
                    confidence: 0.7,
                });
            }
        }
        results
    }
}

/// 不安全反序列化检测器
///
/// pickle.load/eval/yaml.load 等黑名单函数调用
pub struct UnsafeDeserializationDetector;

impl Detector for UnsafeDeserializationDetector {
    fn name(&self) -> &str { "unsafe_deserialization" }
    fn category(&self) -> &str { "security" }

    fn detect(&self, source: &str, file_path: &str, _lang: &str) -> Vec<SmellRecord> {
        // 黑名单模式：pickle.load(s)? / yaml.load(非safe) / marshal.load / eval(
        let re = Regex::new(
            r#"(?i)\b(pickle\.loads?\s*\(|yaml\.load\s*\(|marshal\.loads?\s*\(|shelve\.open\s*\(|jsonpickle\.decode\s*\()"#
        ).unwrap();

        // 安全版本排除
        let safe_re = Regex::new(r#"(?i)yaml\.safe_load"#).unwrap();

        let mut results = Vec::new();
        for (line_num, line) in source.lines().enumerate() {
            if re.is_match(line) && !safe_re.is_match(line) {
                results.push(SmellRecord {
                    category: "security".to_string(),
                    rule: "unsafe_deserialization".to_string(),
                    severity: "warning".to_string(),
                    message: format!("Unsafe deserialization detected"),
                    file: file_path.to_string(),
                    line: line_num + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                });
            }
        }
        results
    }
}

/// 弱加密检测器
///
/// md5/sha1/des/rc4 加密函数调用
pub struct WeakCryptoDetector;

impl Detector for WeakCryptoDetector {
    fn name(&self) -> &str { "weak_crypto" }
    fn category(&self) -> &str { "security" }

    fn detect(&self, source: &str, file_path: &str, _lang: &str) -> Vec<SmellRecord> {
        let re = Regex::new(
            r#"(?i)\b(hashlib\.md5|hashlib\.sha1|MD5\.new|SHA1\.new|DES\.new|RC4\.new|Digest::MD5|Digest::SHA1|md5\(|sha1\(|createHash\s*\(\s*['"](?:md5|sha1)['"])"#
        ).unwrap();

        let mut results = Vec::new();
        for (line_num, line) in source.lines().enumerate() {
            if re.is_match(line) {
                results.push(SmellRecord {
                    category: "security".to_string(),
                    rule: "weak_crypto".to_string(),
                    severity: "info".to_string(),
                    message: format!("Weak cryptographic algorithm detected (MD5/SHA1/DES/RC4)"),
                    file: file_path.to_string(),
                    line: line_num + 1,
                    source: "bcc".to_string(),
                    confidence: 0.9,
                });
            }
        }
        results
    }
}

/// 敏感数据日志泄露检测器
///
/// Logger/console.log 参数中插值含 password/token/secret 变量
pub struct SensitiveDataLogDetector;

impl Detector for SensitiveDataLogDetector {
    fn name(&self) -> &str { "sensitive_data_log" }
    fn category(&self) -> &str { "security" }

    fn detect(&self, source: &str, file_path: &str, _lang: &str) -> Vec<SmellRecord> {
        // 匹配日志函数调用
        let log_re = Regex::new(
            r#"(?i)\b(logger\.\w+|log\.\w+|console\.log|IO\.inspect|Logger\.\w+|logging\.\w+|print|println!?)\s*\("#
        ).unwrap();
        // 匹配敏感变量名出现在插值中
        let sensitive_re = Regex::new(
            r#"(?i)(password|passwd|secret|token|api_key|api_token|auth_token|access_token|private_key|credential)"#
        ).unwrap();

        let mut results = Vec::new();
        for (line_num, line) in source.lines().enumerate() {
            if log_re.is_match(line) && sensitive_re.is_match(line) {
                results.push(SmellRecord {
                    category: "security".to_string(),
                    rule: "sensitive_data_log".to_string(),
                    severity: "info".to_string(),
                    message: format!("Potentially sensitive data in log output"),
                    file: file_path.to_string(),
                    line: line_num + 1,
                    source: "bcc".to_string(),
                    confidence: 0.5,
                });
            }
        }
        results
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // --- HardcodedCredentialDetector ---

    #[test]
    fn hardcoded_credential_detects_api_key() {
        let d = HardcodedCredentialDetector;
        let source = r#"api_key = "sk-proj-abc123def456""#;
        let results = d.detect(source, "config.py", "python");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].rule, "hardcoded_credential");
        assert_eq!(results[0].line, 1);
    }

    #[test]
    fn hardcoded_credential_ignores_env_var() {
        let d = HardcodedCredentialDetector;
        let source = r#"config = os.environ.get("API_KEY")"#;
        let results = d.detect(source, "config.py", "python");
        assert!(results.is_empty());
    }

    #[test]
    fn hardcoded_credential_detects_password() {
        let d = HardcodedCredentialDetector;
        let source = r#"password = "super_secret_password_123""#;
        let results = d.detect(source, "settings.py", "python");
        assert_eq!(results.len(), 1);
    }

    #[test]
    fn hardcoded_credential_ignores_short_values() {
        let d = HardcodedCredentialDetector;
        // 短于 8 字符的值不匹配（可能是占位符）
        let source = r#"api_key = "test""#;
        let results = d.detect(source, "config.py", "python");
        assert!(results.is_empty());
    }

    // --- InjectionRiskDetector ---

    #[test]
    fn injection_risk_detects_concat() {
        let d = InjectionRiskDetector;
        let source = r#"cursor.execute("SELECT * WHERE id=" + user_id)"#;
        let results = d.detect(source, "db.py", "python");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].rule, "injection_risk");
    }

    #[test]
    fn injection_risk_ignores_parameterized() {
        let d = InjectionRiskDetector;
        let source = r#"cursor.execute("SELECT * WHERE id=%s", (user_id,))"#;
        let results = d.detect(source, "db.py", "python");
        assert!(results.is_empty());
    }

    #[test]
    fn injection_risk_detects_fstring() {
        let d = InjectionRiskDetector;
        let source = r#"cursor.execute(f"SELECT * WHERE id={user_id}")"#;
        let results = d.detect(source, "db.py", "python");
        assert_eq!(results.len(), 1);
    }

    // --- UnsafeDeserializationDetector ---

    #[test]
    fn unsafe_deser_detects_pickle_load() {
        let d = UnsafeDeserializationDetector;
        let source = "data = pickle.load(f)";
        let results = d.detect(source, "utils.py", "python");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].rule, "unsafe_deserialization");
    }

    #[test]
    fn unsafe_deser_ignores_safe_load() {
        let d = UnsafeDeserializationDetector;
        let source = "data = yaml.safe_load(f)";
        let results = d.detect(source, "config.py", "python");
        assert!(results.is_empty());
    }

    #[test]
    fn unsafe_deser_detects_yaml_load() {
        let d = UnsafeDeserializationDetector;
        let source = "data = yaml.load(f)";
        let results = d.detect(source, "config.py", "python");
        assert_eq!(results.len(), 1);
    }

    // --- WeakCryptoDetector ---

    #[test]
    fn weak_crypto_detects_md5() {
        let d = WeakCryptoDetector;
        let source = "h = hashlib.md5(data)";
        let results = d.detect(source, "hash.py", "python");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].rule, "weak_crypto");
    }

    #[test]
    fn weak_crypto_ignores_sha256() {
        let d = WeakCryptoDetector;
        let source = "h = hashlib.sha256(data)";
        let results = d.detect(source, "hash.py", "python");
        assert!(results.is_empty());
    }

    #[test]
    fn weak_crypto_detects_sha1() {
        let d = WeakCryptoDetector;
        let source = "h = hashlib.sha1(data)";
        let results = d.detect(source, "hash.py", "python");
        assert_eq!(results.len(), 1);
    }

    // --- SensitiveDataLogDetector ---

    #[test]
    fn sensitive_log_detects_token() {
        let d = SensitiveDataLogDetector;
        let source = r#"logger.info(f"token={api_token}")"#;
        let results = d.detect(source, "auth.py", "python");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].rule, "sensitive_data_log");
    }

    #[test]
    fn sensitive_log_ignores_normal_log() {
        let d = SensitiveDataLogDetector;
        let source = r#"logger.info(f"user={username}")"#;
        let results = d.detect(source, "auth.py", "python");
        assert!(results.is_empty());
    }

    #[test]
    fn sensitive_log_detects_password_in_console() {
        let d = SensitiveDataLogDetector;
        let source = r#"console.log("password=" + password)"#;
        let results = d.detect(source, "auth.ts", "typescript");
        assert_eq!(results.len(), 1);
    }
}
