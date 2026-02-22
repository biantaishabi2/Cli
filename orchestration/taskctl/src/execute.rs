use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, BTreeSet, VecDeque};

use crate::{CoreError, Diagnostics};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ExecuteInput {
    pub nodes: Vec<ExecuteNode>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ExecuteNode {
    pub node_id: String,
    #[serde(default)]
    pub depends_on: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ExecuteDag {
    pub topo_order: Vec<String>,
    pub layers: Vec<Vec<String>>,
    pub nodes: Vec<ExecuteNode>,
    pub edges: Vec<ExecuteEdge>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ExecuteEdge {
    pub from: String,
    pub to: String,
}

pub fn compile(input: ExecuteInput) -> Result<(ExecuteDag, Diagnostics), CoreError> {
    let mut node_map: BTreeMap<String, BTreeSet<String>> = BTreeMap::new();

    for node in input.nodes {
        if node_map.contains_key(&node.node_id) {
            return Err(CoreError::invalid_input(format!(
                "duplicate node_id '{}'",
                node.node_id
            )));
        }
        let deps = node.depends_on.into_iter().collect::<BTreeSet<_>>();
        node_map.insert(node.node_id, deps);
    }

    for (node_id, deps) in &node_map {
        for dep in deps {
            if !node_map.contains_key(dep) {
                return Err(CoreError::invalid_input(format!(
                    "node '{}' depends on missing node '{}'",
                    node_id, dep
                )));
            }
        }
    }

    if let Some(cycle) = detect_cycle(&node_map) {
        return Err(CoreError::dag_cycle_detected(cycle));
    }

    let (topo_order, layers) = topo_and_layers(&node_map)?;

    let nodes = node_map
        .iter()
        .map(|(node_id, deps)| ExecuteNode {
            node_id: node_id.clone(),
            depends_on: deps.iter().cloned().collect(),
        })
        .collect::<Vec<_>>();

    let mut edges = Vec::new();
    for (node_id, deps) in &node_map {
        for dep in deps {
            edges.push(ExecuteEdge {
                from: dep.clone(),
                to: node_id.clone(),
            });
        }
    }

    Ok((
        ExecuteDag {
            topo_order,
            layers,
            nodes,
            edges,
        },
        Diagnostics {
            rules_hit: vec![
                "execute.compile.dag_build".to_string(),
                "execute.compile.cycle_check".to_string(),
            ],
            conflicts: Vec::new(),
            warnings: Vec::new(),
        },
    ))
}

fn topo_and_layers(
    node_map: &BTreeMap<String, BTreeSet<String>>,
) -> Result<(Vec<String>, Vec<Vec<String>>), CoreError> {
    let mut indegree: BTreeMap<String, usize> = BTreeMap::new();
    let mut outgoing: BTreeMap<String, BTreeSet<String>> = BTreeMap::new();

    for node_id in node_map.keys() {
        indegree.insert(node_id.clone(), 0);
        outgoing.insert(node_id.clone(), BTreeSet::new());
    }

    for (node_id, deps) in node_map {
        indegree.insert(node_id.clone(), deps.len());
        for dep in deps {
            outgoing
                .entry(dep.clone())
                .or_default()
                .insert(node_id.clone());
        }
    }

    let mut queue = VecDeque::new();
    for (node_id, deg) in &indegree {
        if *deg == 0 {
            queue.push_back(node_id.clone());
        }
    }

    let mut topo_order = Vec::new();
    let mut layers = Vec::new();

    while !queue.is_empty() {
        let mut current_layer = Vec::new();
        let layer_size = queue.len();
        for _ in 0..layer_size {
            let node = queue.pop_front().expect("queue not empty");
            topo_order.push(node.clone());
            current_layer.push(node.clone());

            if let Some(children) = outgoing.get(&node) {
                for child in children {
                    if let Some(v) = indegree.get_mut(child) {
                        *v -= 1;
                        if *v == 0 {
                            queue.push_back(child.clone());
                        }
                    }
                }
            }
        }
        layers.push(current_layer);
    }

    if topo_order.len() != node_map.len() {
        return Err(CoreError::invalid_input(
            "topological ordering failed after cycle check".to_string(),
        ));
    }

    Ok((topo_order, layers))
}

fn detect_cycle(node_map: &BTreeMap<String, BTreeSet<String>>) -> Option<Vec<String>> {
    #[derive(Clone, Copy, PartialEq, Eq)]
    enum VisitState {
        NotVisited,
        Visiting,
        Visited,
    }

    fn dfs(
        node: &str,
        node_map: &BTreeMap<String, BTreeSet<String>>,
        states: &mut BTreeMap<String, VisitState>,
        stack: &mut Vec<String>,
    ) -> Option<Vec<String>> {
        states.insert(node.to_string(), VisitState::Visiting);
        stack.push(node.to_string());

        if let Some(deps) = node_map.get(node) {
            for dep in deps {
                match states.get(dep).copied().unwrap_or(VisitState::NotVisited) {
                    VisitState::NotVisited => {
                        if let Some(cycle) = dfs(dep, node_map, states, stack) {
                            return Some(cycle);
                        }
                    }
                    VisitState::Visiting => {
                        if let Some(pos) = stack.iter().position(|n| n == dep) {
                            let mut cycle = stack[pos..].to_vec();
                            cycle.push(dep.clone());
                            return Some(cycle);
                        }
                    }
                    VisitState::Visited => {}
                }
            }
        }

        stack.pop();
        states.insert(node.to_string(), VisitState::Visited);
        None
    }

    let mut states = node_map
        .keys()
        .map(|k| (k.clone(), VisitState::NotVisited))
        .collect::<BTreeMap<_, _>>();
    let mut stack = Vec::new();

    for node in node_map.keys() {
        if states.get(node).copied() == Some(VisitState::NotVisited) {
            if let Some(cycle) = dfs(node, node_map, &mut states, &mut stack) {
                return Some(cycle);
            }
        }
    }

    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn compile_detects_cycle_path() {
        let input = ExecuteInput {
            nodes: vec![
                ExecuteNode {
                    node_id: "A".to_string(),
                    depends_on: vec!["B".to_string()],
                },
                ExecuteNode {
                    node_id: "B".to_string(),
                    depends_on: vec!["A".to_string()],
                },
            ],
        };

        let err = compile(input).expect_err("expect cycle");
        assert_eq!(err.code(), "E1001");
        assert_eq!(err.exit_code(), 2);
    }
}
