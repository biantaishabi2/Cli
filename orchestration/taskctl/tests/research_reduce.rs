use serde_json::Value;
use std::path::{Path, PathBuf};
use std::process::Command;
use taskctl::{
    research, BondType, ContractResult, CoreResponse, EvidenceRelation, ResearchEvidence,
    ResearchInput,
};

fn fixture_path(name: &str) -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests")
        .join("fixtures")
        .join("research")
        .join(name)
}

#[test]
fn research_success_fixture_outputs_graph() {
    let bin = env!("CARGO_BIN_EXE_taskctl");
    let fixture = fixture_path("research_success.json");

    let output = Command::new(bin)
        .args([
            "research",
            "reduce",
            "--input",
            fixture.to_str().expect("fixture path utf8"),
        ])
        .output()
        .expect("run taskctl");

    assert_eq!(output.status.code(), Some(0));
    let stdout = String::from_utf8(output.stdout).expect("stdout utf8");
    let json: Value = serde_json::from_str(&stdout).expect("json");

    assert_eq!(json["schema_version"], Value::String("1.0".to_string()));
    assert_eq!(json["result"], Value::String("ok".to_string()));
    assert!(json["graph"].is_object());
    assert!(json["diagnostics"]["conflicts"]
        .as_array()
        .is_some_and(|arr| arr.is_empty()));
}

#[test]
fn research_conflict_produces_diagnostics() {
    let input = ResearchInput {
        evidences: vec![
            ResearchEvidence {
                evidence_id: "ev-support".to_string(),
                conclusion_id: "c1".to_string(),
                relation: EvidenceRelation::Supports,
                confidence: 0.9,
                bond_type: BondType::Deduction,
            },
            ResearchEvidence {
                evidence_id: "ev-conflict".to_string(),
                conclusion_id: "c1".to_string(),
                relation: EvidenceRelation::Conflicts,
                confidence: 0.7,
                bond_type: BondType::Deduction,
            },
        ],
    };

    let (graph, diagnostics) = research::reduce(input).expect("reduce");
    let response = CoreResponse::ok_graph(graph, diagnostics);

    assert_eq!(response.result, ContractResult::Ok);
    assert_eq!(response.schema_version, "1.0");
    assert!(response
        .diagnostics
        .conflicts
        .iter()
        .any(|c| c.left == "ev-support" && c.right == "ev-conflict"));
}

#[test]
fn research_empty_input_returns_empty_graph() {
    let input = ResearchInput {
        evidences: Vec::new(),
    };

    let (graph, diagnostics) = research::reduce(input).expect("reduce empty");
    assert!(graph.conclusions.is_empty());
    assert!(diagnostics.conflicts.is_empty());
}
