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
    pub summary: ResearchSummary,
}

/// 按 verdict 分类的统计摘要
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResearchSummary {
    pub total: usize,
    pub act_count: usize,
    pub investigate_count: usize,
    pub contested_count: usize,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum Verdict {
    /// confidence >= act_threshold 且无冲突 → 可以行动
    Act,
    /// confidence < investigate_threshold → 证据不足，继续调研
    Investigate,
    /// 同时有 supports 和 conflicts → 有争议需人工判断
    Contested,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResearchConclusion {
    pub conclusion_id: String,
    pub supports: Vec<String>,
    pub conflicts: Vec<String>,
    pub confidence: f64,
    pub verdict: Verdict,
}

/// reduce 的配置参数
#[derive(Debug, Clone)]
pub struct ReduceOptions {
    /// confidence >= 此值且无冲突 → Act（默认 0.7）
    pub act_threshold: f64,
    /// confidence < 此值 → Investigate（默认 0.3）
    pub investigate_threshold: f64,
}

impl Default for ReduceOptions {
    fn default() -> Self {
        Self {
            act_threshold: 0.7,
            investigate_threshold: 0.3,
        }
    }
}

#[derive(Debug, Default)]
struct ConclusionAccumulator {
    supports: Vec<(String, f64)>,
    conflicts: Vec<(String, f64)>,
}

pub fn reduce(input: ResearchInput) -> Result<(ResearchGraph, Diagnostics), CoreError> {
    reduce_with_options(input, ReduceOptions::default())
}

pub fn reduce_with_options(
    input: ResearchInput,
    options: ReduceOptions,
) -> Result<(ResearchGraph, Diagnostics), CoreError> {
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

        let has_both_sides = !acc.supports.is_empty() && !acc.conflicts.is_empty();
        let verdict = if has_both_sides {
            Verdict::Contested
        } else if normalized >= options.act_threshold {
            Verdict::Act
        } else if normalized < options.investigate_threshold {
            Verdict::Investigate
        } else {
            // 中间地带：证据方向一致但还不够强
            Verdict::Investigate
        };

        conclusions.push(ResearchConclusion {
            conclusion_id,
            supports: acc.supports.into_iter().map(|(id, _)| id).collect(),
            conflicts: acc.conflicts.into_iter().map(|(id, _)| id).collect(),
            confidence: normalized,
            verdict,
        });
    }

    diagnostics_conflicts
        .sort_by(|a, b| (&a.item_id, &a.left, &a.right).cmp(&(&b.item_id, &b.left, &b.right)));

    // 排序：contested 最前（需人关注），然后 investigate，最后 act
    conclusions.sort_by(|a, b| {
        fn verdict_order(v: &Verdict) -> u8 {
            match v {
                Verdict::Contested => 0,
                Verdict::Investigate => 1,
                Verdict::Act => 2,
            }
        }
        verdict_order(&a.verdict)
            .cmp(&verdict_order(&b.verdict))
            .then_with(|| a.confidence.partial_cmp(&b.confidence).unwrap_or(std::cmp::Ordering::Equal))
            .then_with(|| a.conclusion_id.cmp(&b.conclusion_id))
    });

    let summary = ResearchSummary {
        total: conclusions.len(),
        act_count: conclusions.iter().filter(|c| c.verdict == Verdict::Act).count(),
        investigate_count: conclusions.iter().filter(|c| c.verdict == Verdict::Investigate).count(),
        contested_count: conclusions.iter().filter(|c| c.verdict == Verdict::Contested).count(),
    };

    let mut rules_hit = vec!["research.reduce.supports_merge".to_string()];
    if !diagnostics_conflicts.is_empty() {
        rules_hit.push("research.reduce.conflict_detect".to_string());
    }
    rules_hit.push("research.reduce.verdict_classify".to_string());

    Ok((
        ResearchGraph { conclusions, summary },
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
        // 有 supports 又有 conflicts → contested
        assert_eq!(graph.conclusions[0].verdict, Verdict::Contested);
        assert!(!diagnostics.conflicts.is_empty());
        assert_eq!(graph.summary.contested_count, 1);
    }

    #[test]
    fn verdict_act_when_high_confidence_no_conflict() {
        let input = ResearchInput {
            evidences: vec![
                ResearchEvidence {
                    evidence_id: "e1".to_string(),
                    conclusion_id: "c1".to_string(),
                    relation: EvidenceRelation::Supports,
                    confidence: 0.9,
                },
                ResearchEvidence {
                    evidence_id: "e2".to_string(),
                    conclusion_id: "c1".to_string(),
                    relation: EvidenceRelation::Supports,
                    confidence: 0.8,
                },
            ],
        };
        let (graph, _) = reduce(input).expect("reduce");
        assert_eq!(graph.conclusions[0].verdict, Verdict::Act);
        assert_eq!(graph.summary.act_count, 1);
    }

    #[test]
    fn verdict_investigate_when_low_confidence() {
        let input = ResearchInput {
            evidences: vec![ResearchEvidence {
                evidence_id: "e1".to_string(),
                conclusion_id: "c1".to_string(),
                relation: EvidenceRelation::Supports,
                confidence: 0.1,
            }],
        };
        let (graph, _) = reduce(input).expect("reduce");
        assert_eq!(graph.conclusions[0].verdict, Verdict::Investigate);
        assert_eq!(graph.summary.investigate_count, 1);
    }

    #[test]
    fn custom_thresholds_change_verdict() {
        // 单条 supports confidence=0.3 → drift=0.3/2=0.15 → normalized=0.65
        let input = ResearchInput {
            evidences: vec![ResearchEvidence {
                evidence_id: "e1".to_string(),
                conclusion_id: "c1".to_string(),
                relation: EvidenceRelation::Supports,
                confidence: 0.3,
            }],
        };
        // 默认阈值: act=0.7 → 0.65 < 0.7 → investigate
        let (graph, _) = reduce(input.clone()).expect("reduce default");
        assert_eq!(graph.conclusions[0].verdict, Verdict::Investigate);

        // 降低 act 阈值到 0.6 → 0.65 >= 0.6 → act
        let options = ReduceOptions {
            act_threshold: 0.6,
            investigate_threshold: 0.3,
        };
        let (graph, _) = reduce_with_options(input, options).expect("reduce custom");
        assert_eq!(graph.conclusions[0].verdict, Verdict::Act);
    }

    #[test]
    fn sorting_contested_first_then_investigate_then_act() {
        let input = ResearchInput {
            evidences: vec![
                // c1: 高置信度纯支持 → act
                ResearchEvidence {
                    evidence_id: "e1".to_string(),
                    conclusion_id: "c1".to_string(),
                    relation: EvidenceRelation::Supports,
                    confidence: 0.9,
                },
                // c2: 有冲突 → contested
                ResearchEvidence {
                    evidence_id: "e2".to_string(),
                    conclusion_id: "c2".to_string(),
                    relation: EvidenceRelation::Supports,
                    confidence: 0.8,
                },
                ResearchEvidence {
                    evidence_id: "e3".to_string(),
                    conclusion_id: "c2".to_string(),
                    relation: EvidenceRelation::Conflicts,
                    confidence: 0.3,
                },
                // c3: 低置信度 → investigate
                ResearchEvidence {
                    evidence_id: "e4".to_string(),
                    conclusion_id: "c3".to_string(),
                    relation: EvidenceRelation::Supports,
                    confidence: 0.1,
                },
            ],
        };
        let (graph, _) = reduce(input).expect("reduce");
        assert_eq!(graph.conclusions.len(), 3);
        // contested 排第一
        assert_eq!(graph.conclusions[0].conclusion_id, "c2");
        assert_eq!(graph.conclusions[0].verdict, Verdict::Contested);
        // investigate 排第二
        assert_eq!(graph.conclusions[1].conclusion_id, "c3");
        assert_eq!(graph.conclusions[1].verdict, Verdict::Investigate);
        // act 排最后
        assert_eq!(graph.conclusions[2].conclusion_id, "c1");
        assert_eq!(graph.conclusions[2].verdict, Verdict::Act);
    }
}
