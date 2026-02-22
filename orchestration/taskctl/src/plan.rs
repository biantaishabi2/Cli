use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, BTreeSet};

use crate::{CoreError, Diagnostics};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct PlanInput {
    pub root: String,
    pub nodes: Vec<PlanNode>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct PlanNode {
    pub node_id: String,
    pub node_type: PlanNodeType,
    pub score: f64,
    pub confidence: f64,
    #[serde(default)]
    pub children: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub evidence_id: Option<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum PlanNodeType {
    And,
    Or,
    Leaf,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct PlanDecision {
    pub root: String,
    pub selected_nodes: Vec<String>,
    pub selected_edges: Vec<PlanEdge>,
    pub total_score: f64,
    pub total_confidence: f64,
    pub trace: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct PlanEdge {
    pub from: String,
    pub to: String,
}

#[derive(Debug, Clone)]
struct Candidate {
    score: f64,
    confidence: f64,
    tie_evidence_id: String,
    tie_node_id: String,
    nodes: BTreeSet<String>,
    edges: BTreeSet<(String, String)>,
    trace: Vec<String>,
}

pub fn solve(input: PlanInput) -> Result<(PlanDecision, Diagnostics), CoreError> {
    let node_map = build_node_map(input.nodes)?;
    if !node_map.contains_key(&input.root) {
        return Err(CoreError::invalid_input(format!(
            "root node '{}' does not exist",
            input.root
        )));
    }

    let candidate = solve_node(&input.root, &node_map)?;

    let selected_nodes = candidate.nodes.into_iter().collect::<Vec<_>>();
    let selected_edges = candidate
        .edges
        .into_iter()
        .map(|(from, to)| PlanEdge { from, to })
        .collect::<Vec<_>>();

    Ok((
        PlanDecision {
            root: input.root,
            selected_nodes,
            selected_edges,
            total_score: candidate.score,
            total_confidence: candidate.confidence,
            trace: candidate.trace.clone(),
        },
        Diagnostics {
            rules_hit: vec![
                "plan.solve.and_or".to_string(),
                "plan.solve.tie_break".to_string(),
            ],
            conflicts: Vec::new(),
            warnings: candidate.trace,
        },
    ))
}

fn build_node_map(nodes: Vec<PlanNode>) -> Result<BTreeMap<String, PlanNode>, CoreError> {
    let mut node_map = BTreeMap::new();
    for node in nodes {
        if !(0.0..=1.0).contains(&node.confidence) {
            return Err(CoreError::invalid_input(format!(
                "node '{}' confidence must be within [0,1]",
                node.node_id
            )));
        }
        node_map.insert(node.node_id.clone(), node);
    }

    for node in node_map.values() {
        for child in &node.children {
            if !node_map.contains_key(child) {
                return Err(CoreError::invalid_input(format!(
                    "node '{}' references missing child '{}'",
                    node.node_id, child
                )));
            }
        }
    }

    Ok(node_map)
}

fn solve_node(
    node_id: &str,
    node_map: &BTreeMap<String, PlanNode>,
) -> Result<Candidate, CoreError> {
    let node = node_map
        .get(node_id)
        .ok_or_else(|| CoreError::invalid_input(format!("missing node '{node_id}'")))?;

    match node.node_type {
        PlanNodeType::Leaf => {
            let tie_evidence_id = node
                .evidence_id
                .clone()
                .unwrap_or_else(|| node.node_id.clone());
            let mut nodes = BTreeSet::new();
            nodes.insert(node.node_id.clone());
            Ok(Candidate {
                score: node.score,
                confidence: node.confidence,
                tie_evidence_id,
                tie_node_id: node.node_id.clone(),
                nodes,
                edges: BTreeSet::new(),
                trace: vec![format!("leaf:{}", node.node_id)],
            })
        }
        PlanNodeType::And => {
            if node.children.is_empty() {
                return Err(CoreError::invalid_input(format!(
                    "and node '{}' must have children",
                    node.node_id
                )));
            }

            let mut score = node.score;
            let mut confidence_sum = node.confidence;
            let mut confidence_count = 1usize;
            let mut tie_evidence_id = String::new();
            let mut nodes = BTreeSet::new();
            let mut edges = BTreeSet::new();
            let mut trace = vec![format!("and:{}", node.node_id)];

            nodes.insert(node.node_id.clone());

            for child in &node.children {
                let child_candidate = solve_node(child, node_map)?;
                score += child_candidate.score;
                confidence_sum += child_candidate.confidence;
                confidence_count += 1;
                if tie_evidence_id.is_empty() || child_candidate.tie_evidence_id < tie_evidence_id {
                    tie_evidence_id = child_candidate.tie_evidence_id.clone();
                }
                nodes.extend(child_candidate.nodes);
                edges.extend(child_candidate.edges);
                edges.insert((node.node_id.clone(), child.clone()));
                trace.extend(child_candidate.trace);
            }

            Ok(Candidate {
                score,
                confidence: confidence_sum / confidence_count as f64,
                tie_evidence_id,
                tie_node_id: node.node_id.clone(),
                nodes,
                edges,
                trace,
            })
        }
        PlanNodeType::Or => {
            if node.children.is_empty() {
                return Err(CoreError::invalid_input(format!(
                    "or node '{}' must have children",
                    node.node_id
                )));
            }

            let mut best: Option<(String, Candidate)> = None;

            for child in &node.children {
                let child_candidate = solve_node(child, node_map)?;
                match &best {
                    None => best = Some((child.clone(), child_candidate)),
                    Some((_, current)) => {
                        if is_better(&child_candidate, current) {
                            best = Some((child.clone(), child_candidate));
                        }
                    }
                }
            }

            let (chosen_child, chosen_candidate) = best.expect("or node has at least one child");
            let mut nodes = chosen_candidate.nodes;
            nodes.insert(node.node_id.clone());

            let mut edges = chosen_candidate.edges;
            edges.insert((node.node_id.clone(), chosen_child.clone()));

            let mut trace = vec![format!("or:{}->{}", node.node_id, chosen_child)];
            trace.extend(chosen_candidate.trace);

            Ok(Candidate {
                score: node.score + chosen_candidate.score,
                confidence: (node.confidence + chosen_candidate.confidence) / 2.0,
                tie_evidence_id: chosen_candidate.tie_evidence_id,
                tie_node_id: chosen_candidate.tie_node_id,
                nodes,
                edges,
                trace,
            })
        }
    }
}

fn is_better(candidate: &Candidate, current: &Candidate) -> bool {
    candidate.score > current.score
        || (candidate.score == current.score && candidate.confidence > current.confidence)
        || (candidate.score == current.score
            && candidate.confidence == current.confidence
            && candidate.tie_evidence_id < current.tie_evidence_id)
        || (candidate.score == current.score
            && candidate.confidence == current.confidence
            && candidate.tie_evidence_id == current.tie_evidence_id
            && candidate.tie_node_id < current.tie_node_id)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn solve_applies_tie_break_with_evidence_id() {
        let input = PlanInput {
            root: "root".to_string(),
            nodes: vec![
                PlanNode {
                    node_id: "root".to_string(),
                    node_type: PlanNodeType::Or,
                    score: 0.0,
                    confidence: 1.0,
                    children: vec!["b".to_string(), "a".to_string()],
                    evidence_id: None,
                },
                PlanNode {
                    node_id: "a".to_string(),
                    node_type: PlanNodeType::Leaf,
                    score: 0.8,
                    confidence: 0.9,
                    children: Vec::new(),
                    evidence_id: Some("ev-a".to_string()),
                },
                PlanNode {
                    node_id: "b".to_string(),
                    node_type: PlanNodeType::Leaf,
                    score: 0.8,
                    confidence: 0.9,
                    children: Vec::new(),
                    evidence_id: Some("ev-b".to_string()),
                },
            ],
        };

        let (decision, _) = solve(input).expect("solve");
        assert!(decision.selected_nodes.contains(&"a".to_string()));
        assert!(!decision.selected_nodes.contains(&"b".to_string()));
    }
}
