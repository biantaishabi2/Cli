#[path = "gate_common.rs"]
mod gate_common;

use gate_common::{GateConfig, GateFinding, GateMode};
use serde::Deserialize;
use std::fs;
use std::path::PathBuf;

const WARN_THRESHOLD: i64 = 30;
const STRICT_THRESHOLD: i64 = 50;

#[derive(Debug, Deserialize)]
struct EvolutionFixture {
    scenario: String,
    level: String,
    rules: i64,
    max_depth: i64,
    branches: i64,
    #[serde(default)]
    description: String,
}

fn complexity(case: &EvolutionFixture) -> i64 {
    case.rules + case.max_depth * 3 + case.branches * 2
}

fn fixture_dir() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/evolution/l0_l3")
}

fn load_fixtures() -> Vec<EvolutionFixture> {
    let mut files: Vec<PathBuf> = fs::read_dir(fixture_dir())
        .expect("read evolution fixture dir")
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| path.extension().and_then(|ext| ext.to_str()) == Some("yaml"))
        .collect();
    files.sort();

    files
        .into_iter()
        .map(|path| {
            let raw = fs::read_to_string(&path).expect("read evolution fixture file");
            serde_yaml::from_str(&raw).expect("parse evolution fixture yaml")
        })
        .collect()
}

fn build_complexity_findings(cases: &[EvolutionFixture]) -> Vec<GateFinding> {
    let mut findings = Vec::new();

    for case in cases {
        let score = complexity(case);
        let message = format!(
            "scenario={} level={} complexity={} rules={} depth={} branches={} detail={}",
            case.scenario,
            case.level,
            score,
            case.rules,
            case.max_depth,
            case.branches,
            case.description
        );

        if score > STRICT_THRESHOLD {
            findings.push(GateFinding::new(
                format!("BREAKING:complexity_exceeds_strict:{}", case.scenario),
                "BREAKING",
                message,
            ));
            continue;
        }

        if score > WARN_THRESHOLD {
            findings.push(GateFinding::new(
                format!("DANGEROUS:complexity_exceeds_warn:{}", case.scenario),
                "DANGEROUS",
                message,
            ));
            continue;
        }

        findings.push(GateFinding::new(
            format!("SAFE:complexity_within_gate:{}", case.scenario),
            "SAFE",
            message,
        ));
    }

    findings
}

#[test]
fn l0_l3_min_chain_should_increase_complexity_step_by_step() {
    let cases = load_fixtures();
    assert_eq!(cases.len(), 4, "should load L0~L3 fixtures");

    let levels: Vec<&str> = cases.iter().map(|item| item.level.as_str()).collect();
    assert_eq!(levels, vec!["L0", "L1", "L2", "L3"]);

    let scores: Vec<i64> = cases.iter().map(complexity).collect();
    for window in scores.windows(2) {
        assert!(
            window[1] > window[0],
            "complexity should be monotonic increasing: {:?}",
            scores
        );
    }
}

#[test]
fn l0_l3_complexity_gate_should_support_warn_and_strict() {
    let cases = load_fixtures();
    let findings = build_complexity_findings(&cases);

    let warn_outcome = gate_common::evaluate_gate(
        "regression-l0-l3-evolution-warn",
        GateConfig {
            mode: GateMode::Warn,
            fail_on_dangerous: false,
        },
        findings.clone(),
        &[],
    );
    assert!(warn_outcome.passed, "warn mode should pass with warnings");

    let strict_outcome = gate_common::evaluate_gate(
        "regression-l0-l3-evolution-strict",
        GateConfig {
            mode: GateMode::Strict,
            fail_on_dangerous: true,
        },
        findings,
        &[],
    );

    assert!(
        !strict_outcome.passed,
        "strict mode should block over-threshold complexity"
    );
    assert!(strict_outcome
        .violations
        .iter()
        .any(|item| item.id == "BREAKING:complexity_exceeds_strict:l3_use_case"));
    assert!(strict_outcome
        .violations
        .iter()
        .any(|item| item.id == "DANGEROUS:complexity_exceeds_warn:l2_rules_combo"));
    assert!(
        strict_outcome.report_path.exists(),
        "report should be written"
    );
}
