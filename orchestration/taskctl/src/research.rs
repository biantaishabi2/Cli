use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

use crate::{ConflictDiagnostic, CoreError, Diagnostics};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResearchInput {
    pub evidences: Vec<ResearchEvidence>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResearchEvidence {
    pub evidence_id: String,
    pub conclusion_id: String,
    pub relation: EvidenceRelation,
    pub confidence: f64,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum EvidenceRelation {
    Supports,
    Conflicts,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResearchGraph {
    pub conclusions: Vec<ResearchConclusion>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResearchConclusion {
    pub conclusion_id: String,
    pub supports: Vec<String>,
    pub conflicts: Vec<String>,
    pub confidence: f64,
}

#[derive(Debug, Default)]
struct ConclusionAccumulator {
    supports: Vec<(String, f64)>,
    conflicts: Vec<(String, f64)>,
}

pub fn reduce(input: ResearchInput) -> Result<(ResearchGraph, Diagnostics), CoreError> {
    let mut grouped: BTreeMap<String, ConclusionAccumulator> = BTreeMap::new();

    for evidence in input.evidences {
        if !(0.0..=1.0).contains(&evidence.confidence) {
            return Err(CoreError::invalid_input(format!(
                "evidence '{}' confidence must be within [0,1]",
                evidence.evidence_id
            )));
        }

        let bucket = grouped.entry(evidence.conclusion_id).or_default();
        match evidence.relation {
            EvidenceRelation::Supports => bucket
                .supports
                .push((evidence.evidence_id, evidence.confidence)),
            EvidenceRelation::Conflicts => bucket
                .conflicts
                .push((evidence.evidence_id, evidence.confidence)),
        }
    }

    let mut conclusions = Vec::with_capacity(grouped.len());
    let mut diagnostics_conflicts = Vec::new();

    for (conclusion_id, mut acc) in grouped {
        acc.supports.sort_by(|a, b| a.0.cmp(&b.0));
        acc.conflicts.sort_by(|a, b| a.0.cmp(&b.0));

        let support_sum: f64 = acc.supports.iter().map(|(_, c)| c).sum();
        let conflict_sum: f64 = acc.conflicts.iter().map(|(_, c)| c).sum();
        let total_count = acc.supports.len() + acc.conflicts.len();
        let normalized = if total_count == 0 {
            0.5
        } else {
            let drift = (support_sum - conflict_sum) / (2.0 * total_count as f64);
            (0.5 + drift).clamp(0.0, 1.0)
        };

        for (support_id, _) in &acc.supports {
            for (conflict_id, _) in &acc.conflicts {
                diagnostics_conflicts.push(ConflictDiagnostic {
                    item_id: conclusion_id.clone(),
                    left: support_id.clone(),
                    right: conflict_id.clone(),
                });
            }
        }

        conclusions.push(ResearchConclusion {
            conclusion_id,
            supports: acc.supports.into_iter().map(|(id, _)| id).collect(),
            conflicts: acc.conflicts.into_iter().map(|(id, _)| id).collect(),
            confidence: normalized,
        });
    }

    diagnostics_conflicts
        .sort_by(|a, b| (&a.item_id, &a.left, &a.right).cmp(&(&b.item_id, &b.left, &b.right)));

    let mut rules_hit = vec!["research.reduce.supports_merge".to_string()];
    if !diagnostics_conflicts.is_empty() {
        rules_hit.push("research.reduce.conflict_detect".to_string());
    }

    Ok((
        ResearchGraph { conclusions },
        Diagnostics {
            rules_hit,
            conflicts: diagnostics_conflicts,
            warnings: Vec::new(),
        },
    ))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn reduce_merges_supports_and_conflicts_stably() {
        let input = ResearchInput {
            evidences: vec![
                ResearchEvidence {
                    evidence_id: "ev-2".to_string(),
                    conclusion_id: "c1".to_string(),
                    relation: EvidenceRelation::Supports,
                    confidence: 0.8,
                },
                ResearchEvidence {
                    evidence_id: "ev-1".to_string(),
                    conclusion_id: "c1".to_string(),
                    relation: EvidenceRelation::Supports,
                    confidence: 0.6,
                },
                ResearchEvidence {
                    evidence_id: "ev-3".to_string(),
                    conclusion_id: "c1".to_string(),
                    relation: EvidenceRelation::Conflicts,
                    confidence: 0.2,
                },
            ],
        };

        let (graph, diagnostics) = reduce(input).expect("reduce");
        assert_eq!(graph.conclusions.len(), 1);
        assert_eq!(graph.conclusions[0].supports, vec!["ev-1", "ev-2"]);
        assert_eq!(graph.conclusions[0].conflicts, vec!["ev-3"]);
        assert!(!diagnostics.conflicts.is_empty());
    }
}
