use chrono::{DateTime, Duration, Utc};
use serde::Serialize;
use std::collections::BTreeMap;
use std::env;
use std::fs;
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum GateMode {
    Warn,
    Strict,
}

impl GateMode {
    fn as_str(self) -> &'static str {
        match self {
            Self::Warn => "warn",
            Self::Strict => "strict",
        }
    }
}

#[derive(Debug, Clone, Copy)]
pub struct GateConfig {
    pub mode: GateMode,
    pub fail_on_dangerous: bool,
}

impl GateConfig {
    pub fn from_env() -> Self {
        Self {
            mode: gate_mode_from_env(),
            fail_on_dangerous: gate_fail_on_dangerous_from_env(),
        }
    }
}

#[derive(Debug, Clone, Serialize)]
pub struct GateFinding {
    pub id: String,
    pub category: String,
    pub message: String,
}

impl GateFinding {
    pub fn new(
        id: impl Into<String>,
        category: impl Into<String>,
        message: impl Into<String>,
    ) -> Self {
        Self {
            id: id.into(),
            category: category.into(),
            message: message.into(),
        }
    }
}

#[derive(Debug, Clone)]
pub struct WhitelistEntry {
    pub id: String,
    pub reason: String,
    pub approved_by: String,
    pub expires_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize)]
pub struct GateViolation {
    pub id: String,
    pub category: String,
    pub message: String,
    pub status: String,
    pub reason: Option<String>,
    pub approved_by: Option<String>,
    pub expires_at: Option<String>,
}

#[derive(Debug, Serialize)]
struct GateSummary {
    total_findings: usize,
    breaking: usize,
    dangerous: usize,
    safe: usize,
    violations: usize,
    warnings: usize,
}

#[derive(Debug, Serialize)]
struct GateWhitelistHit {
    id: String,
    reason: String,
    approved_by: String,
    expires_at: String,
}

#[derive(Debug, Serialize)]
struct GateReport {
    gate_name: String,
    gate_mode: String,
    fail_on_dangerous: bool,
    generated_at: String,
    summary: GateSummary,
    diff: Vec<GateFinding>,
    violations: Vec<GateViolation>,
    warnings: Vec<String>,
    whitelist_hits: Vec<GateWhitelistHit>,
}

pub struct GateOutcome {
    pub passed: bool,
    pub report_path: PathBuf,
    pub violations: Vec<GateViolation>,
    pub warnings: Vec<String>,
}

pub fn gate_mode_from_env() -> GateMode {
    match env::var("GATE_MODE") {
        Ok(raw) if raw.eq_ignore_ascii_case("strict") => GateMode::Strict,
        _ => GateMode::Warn,
    }
}

pub fn gate_fail_on_dangerous_from_env() -> bool {
    match env::var("GATE_FAIL_ON_DANGEROUS") {
        Ok(raw) => {
            let normalized = raw.trim().to_ascii_lowercase();
            matches!(normalized.as_str(), "1" | "true" | "yes" | "on")
        }
        Err(_) => false,
    }
}

pub fn load_whitelist(path: &Path) -> Result<Vec<WhitelistEntry>, String> {
    let raw = fs::read_to_string(path)
        .map_err(|e| format!("read whitelist failed {}: {}", path.display(), e))?;

    let mut entries = Vec::new();
    let mut current: BTreeMap<String, String> = BTreeMap::new();

    let flush =
        |buf: &mut BTreeMap<String, String>, out: &mut Vec<WhitelistEntry>| -> Result<(), String> {
            if buf.is_empty() {
                return Ok(());
            }

            let id = buf
                .remove("id")
                .ok_or_else(|| "whitelist entry missing id".to_string())?;
            let reason = buf
                .remove("reason")
                .ok_or_else(|| format!("whitelist entry {} missing reason", id))?;
            let approved_by = buf
                .remove("approved_by")
                .ok_or_else(|| format!("whitelist entry {} missing approved_by", id))?;
            let expires_at_raw = buf
                .remove("expires_at")
                .ok_or_else(|| format!("whitelist entry {} missing expires_at", id))?;
            let expires_at = DateTime::parse_from_rfc3339(&expires_at_raw)
                .map_err(|e| format!("whitelist entry {} has invalid expires_at: {}", id, e))?
                .with_timezone(&Utc);

            out.push(WhitelistEntry {
                id,
                reason,
                approved_by,
                expires_at,
            });
            Ok(())
        };

    for line in raw.lines() {
        let trimmed = line.trim();
        if trimmed.is_empty() || trimmed.starts_with('#') {
            continue;
        }
        if trimmed == "[[entry]]" {
            flush(&mut current, &mut entries)?;
            continue;
        }

        let (key, value_raw) = trimmed
            .split_once('=')
            .ok_or_else(|| format!("invalid whitelist line: {}", trimmed))?;
        let key = key.trim();
        let value = decode_toml_string(value_raw.trim())
            .ok_or_else(|| format!("invalid whitelist value for key {}", key))?;
        current.insert(key.to_string(), value);
    }

    flush(&mut current, &mut entries)?;
    Ok(entries)
}

pub fn evaluate_gate(
    gate_name: &str,
    config: GateConfig,
    findings: Vec<GateFinding>,
    whitelist: &[WhitelistEntry],
) -> GateOutcome {
    evaluate_gate_with_now(gate_name, config, findings, whitelist, Utc::now())
}

pub fn evaluate_gate_with_now(
    gate_name: &str,
    config: GateConfig,
    findings: Vec<GateFinding>,
    whitelist: &[WhitelistEntry],
    now: DateTime<Utc>,
) -> GateOutcome {
    let whitelist_map: BTreeMap<&str, &WhitelistEntry> = whitelist
        .iter()
        .map(|entry| (entry.id.as_str(), entry))
        .collect();

    let mut violations = Vec::new();
    let mut warnings = Vec::new();
    let mut whitelist_hits = Vec::new();

    for finding in &findings {
        let category = finding.category.to_ascii_uppercase();
        if category == "SAFE" {
            continue;
        }

        if let Some(entry) = whitelist_map.get(finding.id.as_str()) {
            if entry.expires_at < now {
                violations.push(GateViolation {
                    id: finding.id.clone(),
                    category,
                    message: finding.message.clone(),
                    status: "expired_whitelist".to_string(),
                    reason: Some(entry.reason.clone()),
                    approved_by: Some(entry.approved_by.clone()),
                    expires_at: Some(entry.expires_at.to_rfc3339()),
                });
                continue;
            }

            if entry.expires_at <= now + Duration::days(7) {
                warnings.push(format!(
                    "whitelist {} will expire soon at {}",
                    finding.id,
                    entry.expires_at.to_rfc3339()
                ));
            }

            whitelist_hits.push(GateWhitelistHit {
                id: finding.id.clone(),
                reason: entry.reason.clone(),
                approved_by: entry.approved_by.clone(),
                expires_at: entry.expires_at.to_rfc3339(),
            });
            continue;
        }

        if category == "BREAKING" || category == "DANGEROUS" {
            violations.push(GateViolation {
                id: finding.id.clone(),
                category,
                message: finding.message.clone(),
                status: "unwhitelisted".to_string(),
                reason: None,
                approved_by: None,
                expires_at: None,
            });
        }
    }

    let strict_failed = violations.iter().any(|violation| {
        if violation.status == "expired_whitelist" {
            return true;
        }
        if violation.category == "BREAKING" {
            return true;
        }
        violation.category == "DANGEROUS" && config.fail_on_dangerous
    });

    let passed = match config.mode {
        GateMode::Warn => true,
        GateMode::Strict => !strict_failed,
    };

    let summary = GateSummary {
        total_findings: findings.len(),
        breaking: findings
            .iter()
            .filter(|finding| finding.category.eq_ignore_ascii_case("BREAKING"))
            .count(),
        dangerous: findings
            .iter()
            .filter(|finding| finding.category.eq_ignore_ascii_case("DANGEROUS"))
            .count(),
        safe: findings
            .iter()
            .filter(|finding| finding.category.eq_ignore_ascii_case("SAFE"))
            .count(),
        violations: violations.len(),
        warnings: warnings.len(),
    };

    let report = GateReport {
        gate_name: gate_name.to_string(),
        gate_mode: config.mode.as_str().to_string(),
        fail_on_dangerous: config.fail_on_dangerous,
        generated_at: now.to_rfc3339(),
        summary,
        diff: findings,
        violations: violations.clone(),
        warnings: warnings.clone(),
        whitelist_hits,
    };

    let report_path = build_report_path(gate_name);
    let report_json = serde_json::to_string_pretty(&report).expect("serialize gate report");
    fs::create_dir_all(
        report_path
            .parent()
            .expect("gate report path should have parent"),
    )
    .expect("create gate report dir");
    fs::write(&report_path, format!("{}\n", report_json)).expect("write gate report");

    println!(
        "[gate:{}] mode={} findings={} violations={} passed={} report={}",
        gate_name,
        config.mode.as_str(),
        report.diff.len(),
        report.violations.len(),
        passed,
        report_path.display()
    );

    GateOutcome {
        passed,
        report_path,
        violations,
        warnings,
    }
}

fn decode_toml_string(value: &str) -> Option<String> {
    if value.is_empty() {
        return None;
    }
    let trimmed = value.trim();
    if trimmed.len() < 2 || !trimmed.starts_with('"') || !trimmed.ends_with('"') {
        return None;
    }

    let inner = &trimmed[1..trimmed.len() - 1];
    Some(inner.replace("\\\"", "\"").replace("\\\\", "\\"))
}

fn build_report_path(gate_name: &str) -> PathBuf {
    let file_name = format!("{}.json", sanitize_file_name(gate_name));
    gate_reports_dir().join(file_name)
}

fn sanitize_file_name(name: &str) -> String {
    let mut out = String::with_capacity(name.len());
    for ch in name.chars() {
        if ch.is_ascii_alphanumeric() || ch == '-' || ch == '_' {
            out.push(ch);
        } else {
            out.push('-');
        }
    }
    if out.is_empty() {
        "gate".to_string()
    } else {
        out
    }
}

fn gate_reports_dir() -> PathBuf {
    if let Ok(dir) = env::var("CARGO_TARGET_DIR") {
        return PathBuf::from(dir).join("gate-reports");
    }

    // 统一落到 workspace target，便于 CI 归档。
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("..")
        .join("target")
        .join("gate-reports")
}
