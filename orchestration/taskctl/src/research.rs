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
    /// 证据来源类型（化学键类比），影响聚合权重
    #[serde(default)]
    pub bond_type: BondType,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum EvidenceRelation {
    Supports,
    Conflicts,
}

/// 证据来源类型，对应论文中的三种化学键
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum BondType {
    /// 共价键：逻辑推导，A→B→C，因果链条（权重 1.0）
    Deduction,
    /// 氢键：回头校验，确认或否定之前的结论（权重 0.7）
    Verification,
    /// 范德华力：试探性猜测，弱关联（权重 0.3）
    Exploration,
}

impl Default for BondType {
    fn default() -> Self {
        BondType::Deduction
    }
}

impl BondType {
    pub fn weight(self) -> f64 {
        match self {
            BondType::Deduction => 1.0,
            BondType::Verification => 0.7,
            BondType::Exploration => 0.3,
        }
    }
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

/// 假设：先验 + 后验 + verdict
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResearchHypothesis {
    pub conclusion_id: String,
    pub prior: f64,
    pub posterior: f64,
    pub verdict: Verdict,
    pub evidence_count: usize,
}

/// 贝叶斯更新：用一条新 evidence 更新 hypothesis 的 posterior
pub fn bayesian_update(
    hypothesis: &mut ResearchHypothesis,
    evidence: &ResearchEvidence,
    options: &ReduceOptions,
) {
    let effective = evidence.confidence * evidence.bond_type.weight();
    let effective = effective.clamp(0.0, 1.0);
    let prior = hypothesis.posterior.clamp(0.01, 0.99);

    // 映射 effective 到 likelihood：supports 推高，conflicts 推低
    // supports: likelihood = 0.5 + effective/2（范围 0.5~1.0）
    // conflicts: likelihood = 0.5 - effective/2（范围 0.0~0.5）
    let likelihood = match evidence.relation {
        EvidenceRelation::Supports => (0.5 + effective / 2.0).clamp(0.01, 0.99),
        EvidenceRelation::Conflicts => (0.5 - effective / 2.0).clamp(0.01, 0.99),
    };

    // P(H|E) = P(H) × L / (P(H) × L + (1-P(H)) × (1-L))
    let posterior =
        (prior * likelihood) / (prior * likelihood + (1.0 - prior) * (1.0 - likelihood));
    hypothesis.posterior = posterior.clamp(0.0, 1.0);
    hypothesis.evidence_count += 1;
    hypothesis.verdict = classify_verdict(hypothesis.posterior, options);
}

/// 根据 posterior 判定 verdict
pub fn classify_verdict(posterior: f64, options: &ReduceOptions) -> Verdict {
    if posterior >= options.act_threshold {
        Verdict::Act
    } else if posterior <= options.investigate_threshold {
        Verdict::Investigate
    } else {
        Verdict::Investigate
    }
}

/// reduce 的配置参数
#[derive(Debug, Clone)]
pub struct ReduceOptions {
    /// posterior >= 此值 → Act（默认 0.7）
    pub act_threshold: f64,
    /// posterior <= 此值 → Investigate（默认 0.3）
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
    /// (evidence_id, effective_confidence = confidence × bond_weight)
    supports: Vec<(String, f64)>,
    conflicts: Vec<(String, f64)>,
    /// 所有证据的 bond_weight 之和，用于归一化
    total_weight: f64,
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

        let w = evidence.bond_type.weight();
        let effective = evidence.confidence * w;
        let bucket = grouped.entry(evidence.conclusion_id).or_default();
        bucket.total_weight += w;
        match evidence.relation {
            EvidenceRelation::Supports => bucket.supports.push((evidence.evidence_id, effective)),
            EvidenceRelation::Conflicts => bucket.conflicts.push((evidence.evidence_id, effective)),
        }
    }

    let mut conclusions = Vec::with_capacity(grouped.len());
    let mut diagnostics_conflicts = Vec::new();

    for (conclusion_id, mut acc) in grouped {
        acc.supports.sort_by(|a, b| a.0.cmp(&b.0));
        acc.conflicts.sort_by(|a, b| a.0.cmp(&b.0));

        // effective values 已经是 confidence × bond_weight
        let support_sum: f64 = acc.supports.iter().map(|(_, c)| c).sum();
        let conflict_sum: f64 = acc.conflicts.iter().map(|(_, c)| c).sum();
        let normalized = if acc.total_weight == 0.0 {
            0.5
        } else {
            let drift = (support_sum - conflict_sum) / (2.0 * acc.total_weight);
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
            .then_with(|| {
                a.confidence
                    .partial_cmp(&b.confidence)
                    .unwrap_or(std::cmp::Ordering::Equal)
            })
            .then_with(|| a.conclusion_id.cmp(&b.conclusion_id))
    });

    let summary = ResearchSummary {
        total: conclusions.len(),
        act_count: conclusions
            .iter()
            .filter(|c| c.verdict == Verdict::Act)
            .count(),
        investigate_count: conclusions
            .iter()
            .filter(|c| c.verdict == Verdict::Investigate)
            .count(),
        contested_count: conclusions
            .iter()
            .filter(|c| c.verdict == Verdict::Contested)
            .count(),
    };

    let mut rules_hit = vec!["research.reduce.supports_merge".to_string()];
    if !diagnostics_conflicts.is_empty() {
        rules_hit.push("research.reduce.conflict_detect".to_string());
    }
    rules_hit.push("research.reduce.verdict_classify".to_string());

    Ok((
        ResearchGraph {
            conclusions,
            summary,
        },
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

    fn ev(id: &str, conclusion: &str, rel: EvidenceRelation, conf: f64) -> ResearchEvidence {
        ev_bond(id, conclusion, rel, conf, BondType::Deduction)
    }

    fn ev_bond(
        id: &str,
        conclusion: &str,
        rel: EvidenceRelation,
        conf: f64,
        bond: BondType,
    ) -> ResearchEvidence {
        ResearchEvidence {
            evidence_id: id.to_string(),
            conclusion_id: conclusion.to_string(),
            relation: rel,
            confidence: conf,
            bond_type: bond,
        }
    }

    #[test]
    fn reduce_merges_supports_and_conflicts_stably() {
        let input = ResearchInput {
            evidences: vec![
                ev("ev-2", "c1", EvidenceRelation::Supports, 0.8),
                ev("ev-1", "c1", EvidenceRelation::Supports, 0.6),
                ev("ev-3", "c1", EvidenceRelation::Conflicts, 0.2),
            ],
        };

        let (graph, diagnostics) = reduce(input).expect("reduce");
        assert_eq!(graph.conclusions.len(), 1);
        assert_eq!(graph.conclusions[0].supports, vec!["ev-1", "ev-2"]);
        assert_eq!(graph.conclusions[0].conflicts, vec!["ev-3"]);
        assert_eq!(graph.conclusions[0].verdict, Verdict::Contested);
        assert!(!diagnostics.conflicts.is_empty());
        assert_eq!(graph.summary.contested_count, 1);
    }

    #[test]
    fn verdict_act_when_high_confidence_no_conflict() {
        let input = ResearchInput {
            evidences: vec![
                ev("e1", "c1", EvidenceRelation::Supports, 0.9),
                ev("e2", "c1", EvidenceRelation::Supports, 0.8),
            ],
        };
        let (graph, _) = reduce(input).expect("reduce");
        assert_eq!(graph.conclusions[0].verdict, Verdict::Act);
        assert_eq!(graph.summary.act_count, 1);
    }

    #[test]
    fn verdict_investigate_when_low_confidence() {
        let input = ResearchInput {
            evidences: vec![ev("e1", "c1", EvidenceRelation::Supports, 0.1)],
        };
        let (graph, _) = reduce(input).expect("reduce");
        assert_eq!(graph.conclusions[0].verdict, Verdict::Investigate);
        assert_eq!(graph.summary.investigate_count, 1);
    }

    #[test]
    fn custom_thresholds_change_verdict() {
        // deduction weight=1.0, confidence=0.3 → effective=0.3
        // drift = 0.3 / (2 × 1.0) = 0.15 → normalized = 0.65
        let input = ResearchInput {
            evidences: vec![ev("e1", "c1", EvidenceRelation::Supports, 0.3)],
        };
        let (graph, _) = reduce(input.clone()).expect("reduce default");
        assert_eq!(graph.conclusions[0].verdict, Verdict::Investigate);

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
                ev("e1", "c1", EvidenceRelation::Supports, 0.9),
                ev("e2", "c2", EvidenceRelation::Supports, 0.8),
                ev("e3", "c2", EvidenceRelation::Conflicts, 0.3),
                ev("e4", "c3", EvidenceRelation::Supports, 0.1),
            ],
        };
        let (graph, _) = reduce(input).expect("reduce");
        assert_eq!(graph.conclusions.len(), 3);
        assert_eq!(graph.conclusions[0].conclusion_id, "c2");
        assert_eq!(graph.conclusions[0].verdict, Verdict::Contested);
        assert_eq!(graph.conclusions[1].conclusion_id, "c3");
        assert_eq!(graph.conclusions[1].verdict, Verdict::Investigate);
        assert_eq!(graph.conclusions[2].conclusion_id, "c1");
        assert_eq!(graph.conclusions[2].verdict, Verdict::Act);
    }

    #[test]
    fn bond_type_weights_affect_confidence() {
        // 场景：1 条 deduction(0.8) vs 3 条 exploration(0.8)
        // deduction: effective=0.8×1.0=0.8, total_weight=1.0
        //   drift = 0.8/(2×1.0) = 0.4 → confidence = 0.9
        let input_deduction = ResearchInput {
            evidences: vec![ev_bond(
                "e1",
                "c1",
                EvidenceRelation::Supports,
                0.8,
                BondType::Deduction,
            )],
        };
        let (g1, _) = reduce(input_deduction).expect("deduction");

        // 3× exploration: effective=0.8×0.3=0.24 each, total_weight=0.9
        //   drift = 0.72/(2×0.9) = 0.4 → confidence = 0.9
        // 相同 confidence，但这是 3 条弱证据才达到 1 条强证据的效果
        let input_exploration = ResearchInput {
            evidences: vec![
                ev_bond(
                    "e1",
                    "c1",
                    EvidenceRelation::Supports,
                    0.8,
                    BondType::Exploration,
                ),
                ev_bond(
                    "e2",
                    "c1",
                    EvidenceRelation::Supports,
                    0.8,
                    BondType::Exploration,
                ),
                ev_bond(
                    "e3",
                    "c1",
                    EvidenceRelation::Supports,
                    0.8,
                    BondType::Exploration,
                ),
            ],
        };
        let (g2, _) = reduce(input_exploration).expect("exploration");

        // 两者 confidence 应该相同（0.9），说明 3 条弱 = 1 条强
        assert!((g1.conclusions[0].confidence - g2.conclusions[0].confidence).abs() < 0.01);
    }

    #[test]
    fn exploration_cannot_overwhelm_deduction() {
        // 1 条 deduction supports(0.9) + 5 条 exploration conflicts(0.9)
        // deduction: effective = 0.9×1.0 = 0.9, weight = 1.0
        // 5× exploration conflicts: effective = 0.9×0.3 = 0.27 each, total = 1.35, weight = 1.5
        // total_weight = 1.0 + 1.5 = 2.5
        // drift = (0.9 - 1.35) / (2 × 2.5) = -0.45/5.0 = -0.09
        // confidence = 0.5 - 0.09 = 0.41
        // 虽然 5 条反对，但因为是 exploration，只拉到 0.41，不会一边倒
        let input = ResearchInput {
            evidences: vec![
                ev_bond(
                    "strong",
                    "c1",
                    EvidenceRelation::Supports,
                    0.9,
                    BondType::Deduction,
                ),
                ev_bond(
                    "weak1",
                    "c1",
                    EvidenceRelation::Conflicts,
                    0.9,
                    BondType::Exploration,
                ),
                ev_bond(
                    "weak2",
                    "c1",
                    EvidenceRelation::Conflicts,
                    0.9,
                    BondType::Exploration,
                ),
                ev_bond(
                    "weak3",
                    "c1",
                    EvidenceRelation::Conflicts,
                    0.9,
                    BondType::Exploration,
                ),
                ev_bond(
                    "weak4",
                    "c1",
                    EvidenceRelation::Conflicts,
                    0.9,
                    BondType::Exploration,
                ),
                ev_bond(
                    "weak5",
                    "c1",
                    EvidenceRelation::Conflicts,
                    0.9,
                    BondType::Exploration,
                ),
            ],
        };
        let (graph, _) = reduce(input).expect("reduce");
        // contested（有双方），但 confidence 不会低于 0.3
        assert_eq!(graph.conclusions[0].verdict, Verdict::Contested);
        assert!(graph.conclusions[0].confidence > 0.35);
    }

    #[test]
    fn verification_conflicts_amplify_doubt() {
        let input = ResearchInput {
            evidences: vec![
                ev_bond(
                    "d1",
                    "c1",
                    EvidenceRelation::Supports,
                    0.8,
                    BondType::Deduction,
                ),
                ev_bond(
                    "d2",
                    "c1",
                    EvidenceRelation::Supports,
                    0.8,
                    BondType::Deduction,
                ),
                ev_bond(
                    "v1",
                    "c1",
                    EvidenceRelation::Conflicts,
                    0.8,
                    BondType::Verification,
                ),
            ],
        };
        let (graph, _) = reduce(input).expect("reduce");
        assert_eq!(graph.conclusions[0].verdict, Verdict::Contested);
        assert!(graph.conclusions[0].confidence < 0.7);
    }

    // ========= 贝叶斯增量更新测试 =========

    fn mk_hyp(prior: f64) -> ResearchHypothesis {
        let options = ReduceOptions::default();
        ResearchHypothesis {
            conclusion_id: "h1".to_string(),
            prior,
            posterior: prior,
            verdict: classify_verdict(prior, &options),
            evidence_count: 0,
        }
    }

    #[test]
    fn bayesian_supports_increases_posterior() {
        let mut hyp = mk_hyp(0.5);
        let e = ev("e1", "h1", EvidenceRelation::Supports, 0.9);
        let options = ReduceOptions::default();
        bayesian_update(&mut hyp, &e, &options);
        // 强支持证据应该大幅提升 posterior
        assert!(hyp.posterior > 0.8);
        assert_eq!(hyp.evidence_count, 1);
    }

    #[test]
    fn bayesian_conflicts_decreases_posterior() {
        let mut hyp = mk_hyp(0.5);
        let e = ev("e1", "h1", EvidenceRelation::Conflicts, 0.9);
        let options = ReduceOptions::default();
        bayesian_update(&mut hyp, &e, &options);
        // 强反对证据应该大幅降低 posterior
        assert!(hyp.posterior < 0.2);
        assert_eq!(hyp.evidence_count, 1);
    }

    #[test]
    fn bayesian_incremental_converges() {
        // 从 0.5 开始，逐步加 supports → posterior 逐步上升
        let mut hyp = mk_hyp(0.5);
        let options = ReduceOptions::default();

        let e1 = ev("e1", "h1", EvidenceRelation::Supports, 0.7);
        bayesian_update(&mut hyp, &e1, &options);
        let p1 = hyp.posterior;
        assert!(p1 > 0.5); // 上升了

        let e2 = ev("e2", "h1", EvidenceRelation::Supports, 0.7);
        bayesian_update(&mut hyp, &e2, &options);
        let p2 = hyp.posterior;
        assert!(p2 > p1); // 继续上升

        let e3 = ev("e3", "h1", EvidenceRelation::Supports, 0.7);
        bayesian_update(&mut hyp, &e3, &options);
        let p3 = hyp.posterior;
        assert!(p3 > p2); // 继续上升
        assert!(p3 > 0.7); // 3 条中等支持 → 应该够 act 了
        assert_eq!(hyp.verdict, Verdict::Act);
    }

    #[test]
    fn bayesian_exploration_weak_impact() {
        // exploration 的 supports 对 posterior 影响很小
        let mut hyp = mk_hyp(0.5);
        let options = ReduceOptions::default();
        let e = ev_bond(
            "e1",
            "h1",
            EvidenceRelation::Supports,
            0.8,
            BondType::Exploration,
        );
        bayesian_update(&mut hyp, &e, &options);
        // exploration weight=0.3, effective=0.24 → likelihood=0.62 → posterior≈0.62
        assert!(hyp.posterior > 0.5);
        assert!(hyp.posterior < 0.65); // 弱证据拉升有限
    }

    #[test]
    fn bayesian_high_prior_plus_conflicts_drops() {
        // 先验很高(0.9)，一条强 conflicts 能拉下来
        let mut hyp = mk_hyp(0.9);
        let options = ReduceOptions::default();
        let e = ev("e1", "h1", EvidenceRelation::Conflicts, 0.9);
        bayesian_update(&mut hyp, &e, &options);
        // 0.9 被强反对拉下来
        assert!(hyp.posterior < 0.5);
    }
}
