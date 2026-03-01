//! 外部 linter 集成
//!
//! 执行外部 linter 命令，解析输出为统一 SmellRecord 格式。

use super::SmellRecord;
use std::process::Command;
use std::time::{Duration, Instant};

/// Linter 配置（从 "name:command" 格式解析）
pub struct LinterConfig {
    pub name: String,
    pub command: String,
}

impl LinterConfig {
    /// 从 "name:command" 格式解析
    pub fn parse(spec: &str) -> Option<Self> {
        let (name, command) = spec.split_once(':')?;
        let name = name.trim().to_string();
        let command = command.trim().to_string();
        if name.is_empty() || command.is_empty() {
            return None;
        }
        Some(Self { name, command })
    }
}

/// 获取超时时长（从 BCC_LINTER_TIMEOUT 环境变量读取，默认 60 秒）
fn linter_timeout() -> Duration {
    let secs = std::env::var("BCC_LINTER_TIMEOUT")
        .ok()
        .and_then(|v| v.parse::<u64>().ok())
        .unwrap_or(60);
    Duration::from_secs(secs)
}

/// 执行单个外部 linter，返回解析后的 SmellRecord 列表
fn run_linter(config: &LinterConfig) -> Vec<SmellRecord> {
    let timeout = linter_timeout();

    // 用 sh -c 执行命令
    let mut child = match Command::new("sh")
        .args(["-c", &config.command])
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped())
        .spawn()
    {
        Ok(c) => c,
        Err(e) => {
            eprintln!(
                "[linter] warn: linter '{}' failed to start: {}",
                config.name, e
            );
            return vec![];
        }
    };

    // 先取出 stdout/stderr handle，在独立线程中并发读取，
    // 避免管道缓冲区满导致子进程写端阻塞而被误判超时
    let child_stdout = child.stdout.take();
    let child_stderr = child.stderr.take();

    let stdout_thread = std::thread::spawn(move || {
        use std::io::Read;
        let mut buf = Vec::new();
        if let Some(mut out) = child_stdout {
            let _ = out.read_to_end(&mut buf);
        }
        buf
    });

    let stderr_thread = std::thread::spawn(move || {
        use std::io::Read;
        let mut buf = Vec::new();
        if let Some(mut err) = child_stderr {
            let _ = err.read_to_end(&mut buf);
        }
        buf
    });

    // 超时轮询等待子进程完成
    let start = Instant::now();
    let poll_interval = Duration::from_millis(100);
    loop {
        match child.try_wait() {
            Ok(Some(_status)) => break,
            Ok(None) => {
                if start.elapsed() >= timeout {
                    let _ = child.kill();
                    let _ = child.wait();
                    eprintln!(
                        "[linter] warn: linter '{}' timed out after {}s",
                        config.name,
                        timeout.as_secs()
                    );
                    return vec![];
                }
                std::thread::sleep(poll_interval);
            }
            Err(e) => {
                eprintln!("[linter] warn: linter '{}' failed: {}", config.name, e);
                return vec![];
            }
        }
    }

    let status = match child.wait() {
        Ok(s) => s,
        Err(e) => {
            eprintln!("[linter] warn: linter '{}' failed: {}", config.name, e);
            return vec![];
        }
    };

    // 命令失败时 warn 并返回空
    if !status.success() {
        eprintln!(
            "[linter] warn: linter '{}' failed with exit code {:?}",
            config.name,
            status.code()
        );
        return vec![];
    }

    let stdout_bytes = stdout_thread.join().unwrap_or_default();
    let stdout = String::from_utf8_lossy(&stdout_bytes);

    // 尝试按不同格式解析
    let mut records = parse_ruff_json(&stdout, &config.name);
    if records.is_empty() {
        records = parse_credo_json(&stdout, &config.name);
    }

    // 非空输出但两种解析器都返回空：warn
    if records.is_empty() && !stdout.trim().is_empty() {
        eprintln!(
            "[linter] warn: linter '{}' produced output but no results could be parsed",
            config.name
        );
    }

    records
}

/// 解析 ruff JSON 输出格式
/// ruff check --output-format json 输出格式:
/// [{"code":"E501","message":"...","filename":"...","row":1,...}]
pub fn parse_ruff_json(stdout: &str, source_name: &str) -> Vec<SmellRecord> {
    let arr: Vec<serde_json::Value> = match serde_json::from_str(stdout) {
        Ok(v) => v,
        Err(_) => return vec![],
    };

    arr.iter()
        .filter_map(|item| {
            let code = item.get("code")?.as_str()?;
            let message = item.get("message")?.as_str()?;
            let filename = item.get("filename")?.as_str()?;
            let row = item.get("row").and_then(|v| v.as_u64()).unwrap_or(0) as usize;

            Some(SmellRecord {
                category: "lint".to_string(),
                rule: code.to_string(),
                severity: "warning".to_string(),
                message: message.to_string(),
                file: filename.to_string(),
                line: row,
                source: source_name.to_string(),
                confidence: 1.0,
                fix_hint: String::new(),
                code_snippet: String::new(),
                offending_code: String::new(),
                suggested_fix: String::new(),
                evidence: vec![],
            })
        })
        .collect()
}

/// 解析 Credo JSON 输出格式
/// mix credo --format json 输出格式:
/// {"issues":[{"category":"...","message":"...","filename":"...","line_no":1,...}]}
pub fn parse_credo_json(stdout: &str, source_name: &str) -> Vec<SmellRecord> {
    let val: serde_json::Value = match serde_json::from_str(stdout) {
        Ok(v) => v,
        Err(_) => return vec![],
    };

    let issues = match val.get("issues").and_then(|v| v.as_array()) {
        Some(arr) => arr,
        None => return vec![],
    };

    issues
        .iter()
        .filter_map(|item| {
            let category = item.get("category")?.as_str().unwrap_or("lint");
            let message = item.get("message")?.as_str()?;
            let filename = item.get("filename")?.as_str()?;
            let line_no = item.get("line_no").and_then(|v| v.as_u64()).unwrap_or(0) as usize;
            let priority = item.get("priority").and_then(|v| v.as_i64()).unwrap_or(0);

            let severity = match priority {
                p if p >= 20 => "critical",
                p if p >= 10 => "warning",
                _ => "info",
            };

            Some(SmellRecord {
                category: category.to_string(),
                rule: format!("credo_{}", category.to_lowercase()),
                severity: severity.to_string(),
                message: message.to_string(),
                file: filename.to_string(),
                line: line_no,
                source: source_name.to_string(),
                confidence: 1.0,
                fix_hint: String::new(),
                code_snippet: String::new(),
                offending_code: String::new(),
                suggested_fix: String::new(),
                evidence: vec![],
            })
        })
        .collect()
}

/// 批量执行所有 linter，合并结果
pub fn run_linters(specs: &[String]) -> Vec<SmellRecord> {
    let mut all = Vec::new();
    for spec in specs {
        match LinterConfig::parse(spec) {
            Some(config) => {
                let records = run_linter(&config);
                eprintln!(
                    "[linter] '{}' returned {} smell(s)",
                    config.name,
                    records.len()
                );
                all.extend(records);
            }
            None => {
                eprintln!(
                    "[linter] warn: invalid linter spec '{}' (expected 'name:command')",
                    spec
                );
            }
        }
    }
    all
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_linter_config() {
        let c = LinterConfig::parse("credo:mix credo --format json").unwrap();
        assert_eq!(c.name, "credo");
        assert_eq!(c.command, "mix credo --format json");
    }

    #[test]
    fn parse_linter_config_invalid() {
        assert!(LinterConfig::parse("nocolon").is_none());
        assert!(LinterConfig::parse(":empty_name").is_none());
        assert!(LinterConfig::parse("empty_cmd:").is_none());
    }

    #[test]
    fn parse_ruff_json_empty_array() {
        let records = parse_ruff_json("[]", "test");
        assert!(records.is_empty());
    }

    #[test]
    fn parse_ruff_json_valid() {
        let json = r#"[{"code":"E501","message":"Line too long","filename":"foo.py","row":42}]"#;
        let records = parse_ruff_json(json, "ruff");
        assert_eq!(records.len(), 1);
        assert_eq!(records[0].rule, "E501");
        assert_eq!(records[0].source, "ruff");
        assert_eq!(records[0].line, 42);
    }

    #[test]
    fn parse_credo_json_valid() {
        let json = r#"{"issues":[{"category":"readability","message":"Module is too long","filename":"lib/foo.ex","line_no":1,"priority":10}]}"#;
        let records = parse_credo_json(json, "credo");
        assert_eq!(records.len(), 1);
        assert_eq!(records[0].source, "credo");
        assert_eq!(records[0].severity, "warning");
    }

    #[test]
    fn parse_invalid_json_returns_empty() {
        let records = parse_ruff_json("not json", "test");
        assert!(records.is_empty());
        let records = parse_credo_json("not json", "test");
        assert!(records.is_empty());
    }

    #[test]
    fn run_linters_with_echo_empty_array() {
        let specs = vec!["mock:echo '[]'".to_string()];
        let records = run_linters(&specs);
        assert!(records.is_empty());
    }

    #[test]
    fn run_linters_with_failing_command() {
        let specs = vec!["bad:false".to_string()];
        let records = run_linters(&specs);
        assert!(records.is_empty());
    }
}
