use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};
use std::collections::{BTreeMap, BTreeSet, VecDeque};
use std::fs;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};
use thiserror::Error;
use uuid::Uuid;

pub mod execute;
pub mod research;

pub use execute::{ExecuteDag, ExecuteEdge, ExecuteInput, ExecuteNode};
pub use research::{
    BondType, EvidenceRelation, ResearchConclusion, ResearchEvidence, ResearchGraph,
    ResearchHypothesis, ResearchInput, Verdict,
};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum ContractResult {
    Ok,
    Error,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, Default)]
pub struct Diagnostics {
    pub rules_hit: Vec<String>,
    pub conflicts: Vec<ConflictDiagnostic>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub warnings: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq, PartialOrd, Ord)]
pub struct ConflictDiagnostic {
    pub item_id: String,
    pub left: String,
    pub right: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct CoreErrorPayload {
    pub code: String,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cycle: Option<Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct CoreResponse {
    pub schema_version: String,
    pub result: ContractResult,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub graph: Option<ResearchGraph>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub dag: Option<ExecuteDag>,
    pub diagnostics: Diagnostics,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<CoreErrorPayload>,
}

impl CoreResponse {
    const SCHEMA_VERSION: &'static str = "1.0";

    pub fn ok_graph(graph: ResearchGraph, diagnostics: Diagnostics) -> Self {
        Self {
            schema_version: Self::SCHEMA_VERSION.to_string(),
            result: ContractResult::Ok,
            graph: Some(graph),
            dag: None,
            diagnostics,
            error: None,
        }
    }

    pub fn ok_dag(dag: ExecuteDag, diagnostics: Diagnostics) -> Self {
        Self {
            schema_version: Self::SCHEMA_VERSION.to_string(),
            result: ContractResult::Ok,
            graph: None,
            dag: Some(dag),
            diagnostics,
            error: None,
        }
    }

    pub fn from_error(error: CoreError) -> Self {
        Self {
            schema_version: Self::SCHEMA_VERSION.to_string(),
            result: ContractResult::Error,
            graph: None,
            dag: None,
            diagnostics: Diagnostics::default(),
            error: Some(error.to_payload()),
        }
    }
}

#[derive(Debug, Clone, Error, PartialEq, Eq)]
pub enum CoreError {
    #[error("invalid input: {0}")]
    InvalidInput(String),
    #[error("E1001 DAG_CYCLE_DETECTED: {0}")]
    DagCycleDetected(String, Vec<String>),
}

impl CoreError {
    pub fn invalid_input(message: String) -> Self {
        Self::InvalidInput(message)
    }

    pub fn dag_cycle_detected(cycle: Vec<String>) -> Self {
        let cycle_text = cycle.join("->");
        Self::DagCycleDetected(
            format!("cycle detected in dag compile: {cycle_text}"),
            cycle,
        )
    }

    pub fn code(&self) -> &'static str {
        match self {
            Self::InvalidInput(_) => "E0001",
            Self::DagCycleDetected(_, _) => "E1001",
        }
    }

    pub fn exit_code(&self) -> i32 {
        match self {
            Self::InvalidInput(_) => 2,
            Self::DagCycleDetected(_, _) => 2,
        }
    }

    pub fn to_payload(&self) -> CoreErrorPayload {
        match self {
            Self::InvalidInput(message) => CoreErrorPayload {
                code: self.code().to_string(),
                message: message.clone(),
                cycle: None,
            },
            Self::DagCycleDetected(message, cycle) => CoreErrorPayload {
                code: self.code().to_string(),
                message: message.clone(),
                cycle: Some(cycle.clone()),
            },
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum TaskStatus {
    Pending,
    InProgress,
    Completed,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct Task {
    pub id: String,
    pub subject: String,
    pub description: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub active_form: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub owner: Option<String>,
    #[serde(default)]
    pub metadata: Map<String, Value>,
    pub status: TaskStatus,
    #[serde(default)]
    pub blocks: BTreeSet<String>,
    #[serde(default)]
    pub blocked_by: BTreeSet<String>,
    pub created_at_epoch_ms: u128,
    pub updated_at_epoch_ms: u128,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TaskStore {
    #[serde(default = "default_version")]
    pub version: u32,
    #[serde(default)]
    pub tasks: BTreeMap<String, Task>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub research_hypotheses: BTreeMap<String, research::ResearchHypothesis>,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub research_evidences: BTreeMap<String, research::ResearchEvidence>,
}

impl Default for TaskStore {
    fn default() -> Self {
        Self {
            version: default_version(),
            tasks: BTreeMap::new(),
            research_hypotheses: BTreeMap::new(),
            research_evidences: BTreeMap::new(),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct DagNode {
    pub id: String,
    pub subject: String,
    pub status: TaskStatus,
    pub depth: usize,
    pub blocked_by: Vec<String>,
    pub blocks: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct DagEdge {
    pub from: String,
    pub to: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct DagGraph {
    pub topo_order: Vec<String>,
    pub layers: Vec<Vec<String>>,
    pub nodes: Vec<DagNode>,
    pub edges: Vec<DagEdge>,
}

#[derive(Debug, Clone)]
pub struct TaskCreate {
    pub subject: String,
    pub description: String,
    pub active_form: Option<String>,
    pub metadata: Map<String, Value>,
}

#[derive(Debug, Clone, Default)]
pub struct TaskUpdate {
    pub status: Option<UpdateStatus>,
    pub subject: Option<String>,
    pub description: Option<String>,
    pub active_form: Option<String>,
    pub owner: Option<String>,
    pub metadata: Option<Map<String, Value>>,
    pub add_blocks: Vec<String>,
    pub add_blocked_by: Vec<String>,
}

#[derive(Debug, Clone, Copy)]
pub enum UpdateStatus {
    Pending,
    InProgress,
    Completed,
    Deleted,
}

#[derive(Debug, Error)]
pub enum TaskError {
    #[error("task not found: {0}")]
    NotFound(String),
    #[error("invalid status transition: {from:?} -> {to:?}")]
    InvalidStatusTransition { from: TaskStatus, to: TaskStatus },
    #[error("blocked task cannot transition to {0:?}; unresolved dependencies: {1}")]
    BlockedTransition(TaskStatus, String),
    #[error("dependency target does not exist: {0}")]
    MissingDependency(String),
    #[error("self dependency is not allowed")]
    SelfDependency,
    #[error("cycle detected in dependency graph")]
    CycleDetected,
    #[error("store validation failed: {0}")]
    StoreValidation(String),
    #[error("io error: {0}")]
    Io(String),
    #[error("json error: {0}")]
    Json(String),
}

fn default_version() -> u32 {
    1
}

fn now_epoch_ms() -> u128 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_millis())
        .unwrap_or(0)
}

fn can_transition(from: TaskStatus, to: TaskStatus) -> bool {
    matches!(
        (from, to),
        (TaskStatus::Pending, TaskStatus::Pending)
            | (TaskStatus::Pending, TaskStatus::InProgress)
            | (TaskStatus::InProgress, TaskStatus::InProgress)
            | (TaskStatus::InProgress, TaskStatus::Completed)
            | (TaskStatus::Completed, TaskStatus::Completed)
    )
}

fn parse_update_status(s: UpdateStatus) -> Option<TaskStatus> {
    match s {
        UpdateStatus::Pending => Some(TaskStatus::Pending),
        UpdateStatus::InProgress => Some(TaskStatus::InProgress),
        UpdateStatus::Completed => Some(TaskStatus::Completed),
        UpdateStatus::Deleted => None,
    }
}

pub fn default_store_path() -> PathBuf {
    let home = std::env::var("HOME").unwrap_or_else(|_| ".".to_string());
    PathBuf::from(home)
        .join("cli")
        .join("taskctl")
        .join("tasks.json")
}

pub fn load_store(path: &Path) -> Result<TaskStore, TaskError> {
    if !path.exists() {
        return Ok(TaskStore::default());
    }
    let raw = fs::read_to_string(path).map_err(|e| TaskError::Io(e.to_string()))?;
    serde_json::from_str(&raw).map_err(|e| TaskError::Json(e.to_string()))
}

pub fn save_store(path: &Path, store: &TaskStore) -> Result<(), TaskError> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|e| TaskError::Io(e.to_string()))?;
    }
    let raw = serde_json::to_string_pretty(store).map_err(|e| TaskError::Json(e.to_string()))?;
    fs::write(path, raw).map_err(|e| TaskError::Io(e.to_string()))
}

pub fn create_task(store: &mut TaskStore, req: TaskCreate) -> Result<Task, TaskError> {
    let now = now_epoch_ms();
    let task = Task {
        id: Uuid::new_v4().to_string(),
        subject: req.subject,
        description: req.description,
        active_form: req.active_form,
        owner: None,
        metadata: req.metadata,
        status: TaskStatus::Pending,
        blocks: BTreeSet::new(),
        blocked_by: BTreeSet::new(),
        created_at_epoch_ms: now,
        updated_at_epoch_ms: now,
    };
    store.tasks.insert(task.id.clone(), task.clone());
    Ok(task)
}

pub fn get_task<'a>(store: &'a TaskStore, task_id: &str) -> Result<&'a Task, TaskError> {
    store
        .tasks
        .get(task_id)
        .ok_or_else(|| TaskError::NotFound(task_id.to_string()))
}

pub fn delete_task(store: &mut TaskStore, task_id: &str) -> Result<(), TaskError> {
    let removed = store
        .tasks
        .remove(task_id)
        .ok_or_else(|| TaskError::NotFound(task_id.to_string()))?;

    for blocked_id in &removed.blocks {
        if let Some(task) = store.tasks.get_mut(blocked_id) {
            task.blocked_by.remove(task_id);
            task.updated_at_epoch_ms = now_epoch_ms();
        }
    }

    for dep_id in &removed.blocked_by {
        if let Some(task) = store.tasks.get_mut(dep_id) {
            task.blocks.remove(task_id);
            task.updated_at_epoch_ms = now_epoch_ms();
        }
    }

    Ok(())
}

pub fn update_task(
    store: &mut TaskStore,
    task_id: &str,
    mut patch: TaskUpdate,
) -> Result<Option<Task>, TaskError> {
    if matches!(patch.status, Some(UpdateStatus::Deleted)) {
        delete_task(store, task_id)?;
        validate_store(store)?;
        return Ok(None);
    }

    if patch.add_blocks.iter().any(|id| id == task_id)
        || patch.add_blocked_by.iter().any(|id| id == task_id)
    {
        return Err(TaskError::SelfDependency);
    }

    for dep in patch.add_blocks.iter().chain(patch.add_blocked_by.iter()) {
        if !store.tasks.contains_key(dep) {
            return Err(TaskError::MissingDependency(dep.clone()));
        }
    }

    {
        let task = store
            .tasks
            .get_mut(task_id)
            .ok_or_else(|| TaskError::NotFound(task_id.to_string()))?;

        if let Some(subject) = patch.subject.take() {
            task.subject = subject;
        }
        if let Some(description) = patch.description.take() {
            task.description = description;
        }
        if let Some(active_form) = patch.active_form.take() {
            task.active_form = Some(active_form);
        }
        if let Some(owner) = patch.owner.take() {
            task.owner = Some(owner);
        }
        if let Some(metadata_patch) = patch.metadata.take() {
            for (k, v) in metadata_patch {
                if v.is_null() {
                    task.metadata.remove(&k);
                } else {
                    task.metadata.insert(k, v);
                }
            }
        }
    }

    for dep in &patch.add_blocked_by {
        {
            let task = store
                .tasks
                .get_mut(task_id)
                .ok_or_else(|| TaskError::NotFound(task_id.to_string()))?;
            task.blocked_by.insert(dep.clone());
            task.updated_at_epoch_ms = now_epoch_ms();
        }
        {
            let dep_task = store
                .tasks
                .get_mut(dep)
                .ok_or_else(|| TaskError::MissingDependency(dep.clone()))?;
            dep_task.blocks.insert(task_id.to_string());
            dep_task.updated_at_epoch_ms = now_epoch_ms();
        }
    }

    for blocked in &patch.add_blocks {
        {
            let task = store
                .tasks
                .get_mut(task_id)
                .ok_or_else(|| TaskError::NotFound(task_id.to_string()))?;
            task.blocks.insert(blocked.clone());
            task.updated_at_epoch_ms = now_epoch_ms();
        }
        {
            let blocked_task = store
                .tasks
                .get_mut(blocked)
                .ok_or_else(|| TaskError::MissingDependency(blocked.clone()))?;
            blocked_task.blocked_by.insert(task_id.to_string());
            blocked_task.updated_at_epoch_ms = now_epoch_ms();
        }
    }

    if let Some(target) = patch.status.and_then(parse_update_status) {
        let unresolved = unresolved_dependencies(store, task_id)?;
        let task = store
            .tasks
            .get_mut(task_id)
            .ok_or_else(|| TaskError::NotFound(task_id.to_string()))?;

        if !can_transition(task.status, target) {
            return Err(TaskError::InvalidStatusTransition {
                from: task.status,
                to: target,
            });
        }

        if matches!(target, TaskStatus::InProgress | TaskStatus::Completed)
            && !unresolved.is_empty()
        {
            return Err(TaskError::BlockedTransition(target, unresolved.join(",")));
        }

        task.status = target;
        task.updated_at_epoch_ms = now_epoch_ms();
    } else if let Some(task) = store.tasks.get_mut(task_id) {
        task.updated_at_epoch_ms = now_epoch_ms();
    }

    validate_store(store)?;

    let updated = store
        .tasks
        .get(task_id)
        .ok_or_else(|| TaskError::NotFound(task_id.to_string()))?
        .clone();
    Ok(Some(updated))
}

pub fn list_tasks(store: &TaskStore) -> Vec<Task> {
    store.tasks.values().cloned().collect()
}

pub fn ready_tasks(store: &TaskStore) -> Result<Vec<Task>, TaskError> {
    let mut out = Vec::new();
    for task in store.tasks.values() {
        if task.status != TaskStatus::Pending {
            continue;
        }
        let unresolved = unresolved_dependencies(store, &task.id)?;
        if unresolved.is_empty() {
            out.push(task.clone());
        }
    }
    Ok(out)
}

pub fn validate_store(store: &TaskStore) -> Result<(), TaskError> {
    for (task_id, task) in &store.tasks {
        for dep in &task.blocked_by {
            let dep_task = store.tasks.get(dep).ok_or_else(|| {
                TaskError::StoreValidation(format!("{task_id} blocked_by missing {dep}"))
            })?;
            if !dep_task.blocks.contains(task_id) {
                return Err(TaskError::StoreValidation(format!(
                    "{task_id}<->{dep} relation is not symmetric"
                )));
            }
        }
        for blocked in &task.blocks {
            let blocked_task = store.tasks.get(blocked).ok_or_else(|| {
                TaskError::StoreValidation(format!("{task_id} blocks missing {blocked}"))
            })?;
            if !blocked_task.blocked_by.contains(task_id) {
                return Err(TaskError::StoreValidation(format!(
                    "{task_id}<->{blocked} relation is not symmetric"
                )));
            }
        }
    }

    let _ = topo_order(store)?;

    Ok(())
}

pub fn dag_graph(store: &TaskStore) -> Result<DagGraph, TaskError> {
    validate_store(store)?;
    let topo = topo_order(store)?;

    let mut depth_map: BTreeMap<String, usize> = BTreeMap::new();
    for id in &topo {
        let task = store
            .tasks
            .get(id)
            .ok_or_else(|| TaskError::StoreValidation(format!("missing task in topo: {id}")))?;
        let mut depth = 0usize;
        for dep in &task.blocked_by {
            let parent_depth = *depth_map.get(dep).ok_or_else(|| {
                TaskError::StoreValidation(format!("missing parent depth for {dep}"))
            })?;
            depth = depth.max(parent_depth + 1);
        }
        depth_map.insert(id.clone(), depth);
    }

    let max_depth = depth_map.values().copied().max().unwrap_or(0);
    let mut layers = vec![Vec::new(); max_depth.saturating_add(1)];
    for id in &topo {
        let depth = *depth_map.get(id).unwrap_or(&0);
        if let Some(layer) = layers.get_mut(depth) {
            layer.push(id.clone());
        }
    }

    let mut nodes = Vec::new();
    for id in &topo {
        if let Some(task) = store.tasks.get(id) {
            nodes.push(DagNode {
                id: task.id.clone(),
                subject: task.subject.clone(),
                status: task.status,
                depth: *depth_map.get(id).unwrap_or(&0),
                blocked_by: task.blocked_by.iter().cloned().collect(),
                blocks: task.blocks.iter().cloned().collect(),
            });
        }
    }

    let mut edges = Vec::new();
    for task in store.tasks.values() {
        for dep in &task.blocked_by {
            edges.push(DagEdge {
                from: dep.clone(),
                to: task.id.clone(),
            });
        }
    }

    Ok(DagGraph {
        topo_order: topo,
        layers,
        nodes,
        edges,
    })
}

fn unresolved_dependencies(store: &TaskStore, task_id: &str) -> Result<Vec<String>, TaskError> {
    let task = store
        .tasks
        .get(task_id)
        .ok_or_else(|| TaskError::NotFound(task_id.to_string()))?;
    let mut unresolved = Vec::new();
    for dep_id in &task.blocked_by {
        let dep = store
            .tasks
            .get(dep_id)
            .ok_or_else(|| TaskError::MissingDependency(dep_id.clone()))?;
        if dep.status != TaskStatus::Completed
            && dep.metadata.get("integrated").and_then(|v| v.as_str()) != Some("true")
        {
            unresolved.push(dep_id.clone());
        }
    }
    Ok(unresolved)
}

fn topo_order(store: &TaskStore) -> Result<Vec<String>, TaskError> {
    let mut indegree: BTreeMap<String, usize> = BTreeMap::new();
    let mut outgoing: BTreeMap<String, Vec<String>> = BTreeMap::new();

    for id in store.tasks.keys() {
        indegree.insert(id.clone(), 0);
        outgoing.insert(id.clone(), Vec::new());
    }

    for (task_id, task) in &store.tasks {
        for dep in &task.blocked_by {
            if let Some(children) = outgoing.get_mut(dep) {
                children.push(task_id.clone());
            }
            if let Some(v) = indegree.get_mut(task_id) {
                *v += 1;
            }
        }
    }

    let mut queue: VecDeque<String> = indegree
        .iter()
        .filter(|(_, d)| **d == 0)
        .map(|(id, _)| id.clone())
        .collect();
    let mut visited = 0usize;
    let mut order = Vec::with_capacity(store.tasks.len());

    while let Some(node) = queue.pop_front() {
        visited += 1;
        order.push(node.clone());
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

    if visited != store.tasks.len() {
        return Err(TaskError::CycleDetected);
    }

    Ok(order)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashSet;
    use std::env;
    use std::fs;
    use std::path::PathBuf;

    fn mk_create(subject: &str) -> TaskCreate {
        TaskCreate {
            subject: subject.to_string(),
            description: format!("desc-{subject}"),
            active_form: None,
            metadata: Map::new(),
        }
    }

    #[test]
    fn create_defaults_to_pending() {
        let mut store = TaskStore::default();
        assert_eq!(store.version, 1);
        let task = create_task(&mut store, mk_create("run-tests")).expect("create failed");
        assert_eq!(task.status, TaskStatus::Pending);
        assert!(store.tasks.contains_key(&task.id));
    }

    #[test]
    fn add_dependency_and_ready_flow() {
        let mut store = TaskStore::default();
        let a = create_task(&mut store, mk_create("a")).expect("create a");
        let b = create_task(&mut store, mk_create("b")).expect("create b");

        update_task(
            &mut store,
            &b.id,
            TaskUpdate {
                add_blocked_by: vec![a.id.clone()],
                ..TaskUpdate::default()
            },
        )
        .expect("link dependency");

        let ready_before = ready_tasks(&store).expect("ready before");
        assert_eq!(ready_before.len(), 1);
        assert_eq!(ready_before[0].id, a.id);

        update_task(
            &mut store,
            &a.id,
            TaskUpdate {
                status: Some(UpdateStatus::InProgress),
                ..TaskUpdate::default()
            },
        )
        .expect("a in progress");
        update_task(
            &mut store,
            &a.id,
            TaskUpdate {
                status: Some(UpdateStatus::Completed),
                ..TaskUpdate::default()
            },
        )
        .expect("a completed");

        let ready_after = ready_tasks(&store).expect("ready after");
        assert_eq!(ready_after.len(), 1);
        assert_eq!(ready_after[0].id, b.id);
    }

    #[test]
    fn blocked_task_cannot_start() {
        let mut store = TaskStore::default();
        let a = create_task(&mut store, mk_create("a")).expect("create a");
        let b = create_task(&mut store, mk_create("b")).expect("create b");

        update_task(
            &mut store,
            &b.id,
            TaskUpdate {
                add_blocked_by: vec![a.id.clone()],
                ..TaskUpdate::default()
            },
        )
        .expect("link dependency");

        let err = update_task(
            &mut store,
            &b.id,
            TaskUpdate {
                status: Some(UpdateStatus::InProgress),
                ..TaskUpdate::default()
            },
        )
        .expect_err("should fail when dependency unresolved");

        assert!(matches!(
            err,
            TaskError::BlockedTransition(TaskStatus::InProgress, _)
        ));
    }

    #[test]
    fn dag_graph_contains_topo_layers_and_edges() {
        let mut store = TaskStore::default();
        let a = create_task(&mut store, mk_create("a")).expect("create a");
        let b = create_task(&mut store, mk_create("b")).expect("create b");
        let c = create_task(&mut store, mk_create("c")).expect("create c");

        update_task(
            &mut store,
            &b.id,
            TaskUpdate {
                add_blocked_by: vec![a.id.clone()],
                ..TaskUpdate::default()
            },
        )
        .expect("link b <- a");

        update_task(
            &mut store,
            &c.id,
            TaskUpdate {
                add_blocked_by: vec![b.id.clone()],
                ..TaskUpdate::default()
            },
        )
        .expect("link c <- b");

        let dag = dag_graph(&store).expect("build dag");
        assert_eq!(dag.layers.len(), 3);
        assert_eq!(dag.layers[0], vec![a.id.clone()]);
        assert_eq!(dag.layers[1], vec![b.id.clone()]);
        assert_eq!(dag.layers[2], vec![c.id.clone()]);
        assert_eq!(dag.topo_order.first(), Some(&a.id));
        assert_eq!(dag.topo_order.last(), Some(&c.id));
        assert!(dag.edges.iter().any(|e| e.from == a.id && e.to == b.id));
        assert!(dag.edges.iter().any(|e| e.from == b.id && e.to == c.id));
    }

    #[test]
    fn cycle_is_rejected() {
        let mut store = TaskStore::default();
        let a = create_task(&mut store, mk_create("a")).expect("create a");
        let b = create_task(&mut store, mk_create("b")).expect("create b");

        update_task(
            &mut store,
            &b.id,
            TaskUpdate {
                add_blocked_by: vec![a.id.clone()],
                ..TaskUpdate::default()
            },
        )
        .expect("link b <- a");

        let err = update_task(
            &mut store,
            &a.id,
            TaskUpdate {
                add_blocked_by: vec![b.id.clone()],
                ..TaskUpdate::default()
            },
        )
        .expect_err("should reject cycle");

        assert!(matches!(err, TaskError::CycleDetected));
    }

    #[test]
    fn metadata_merge_supports_null_delete() {
        let mut store = TaskStore::default();
        let mut meta = Map::new();
        meta.insert("priority".to_string(), Value::String("p1".to_string()));
        meta.insert("sprint".to_string(), Value::String("S1".to_string()));

        let created = create_task(
            &mut store,
            TaskCreate {
                subject: "task".to_string(),
                description: "desc".to_string(),
                active_form: None,
                metadata: meta,
            },
        )
        .expect("create");

        let mut patch = Map::new();
        patch.insert("priority".to_string(), Value::String("p0".to_string()));
        patch.insert("sprint".to_string(), Value::Null);

        let updated = update_task(
            &mut store,
            &created.id,
            TaskUpdate {
                metadata: Some(patch),
                ..TaskUpdate::default()
            },
        )
        .expect("update")
        .expect("not deleted");

        assert_eq!(
            updated.metadata.get("priority"),
            Some(&Value::String("p0".to_string()))
        );
        assert!(updated.metadata.get("sprint").is_none());
    }

    #[test]
    fn delete_cleans_reverse_edges() {
        let mut store = TaskStore::default();
        let a = create_task(&mut store, mk_create("a")).expect("create a");
        let b = create_task(&mut store, mk_create("b")).expect("create b");

        update_task(
            &mut store,
            &b.id,
            TaskUpdate {
                add_blocked_by: vec![a.id.clone()],
                ..TaskUpdate::default()
            },
        )
        .expect("link dependency");

        delete_task(&mut store, &a.id).expect("delete a");
        let b_task = get_task(&store, &b.id).expect("b exists");
        assert!(b_task.blocked_by.is_empty());
    }

    #[test]
    fn bdd_ready_task_becomes_progressable_only_when_unblocked() {
        // Given two tasks with a dependency: B blocked by A
        let mut store = TaskStore::default();
        let bootstrap = create_task(&mut store, mk_create("bootstrap infrastructure"))
            .expect("create bootstrap");
        let build_api = create_task(&mut store, mk_create("build api")).expect("create build api");
        update_task(
            &mut store,
            &build_api.id,
            TaskUpdate {
                add_blocked_by: vec![bootstrap.id.clone()],
                ..TaskUpdate::default()
            },
        )
        .expect("link build_api to bootstrap");

        // When: query ready tasks before any progress
        let ready_before = ready_tasks(&store).expect("ready tasks");
        assert_eq!(ready_before.len(), 1);
        assert!(ready_before.iter().any(|t| t.id == bootstrap.id));

        // Then build_api is observable as blocked, and bootstrap is the only executable one
        let blocked = update_task(
            &mut store,
            &build_api.id,
            TaskUpdate {
                status: Some(UpdateStatus::InProgress),
                ..TaskUpdate::default()
            },
        );
        assert!(
            matches!(blocked, Err(TaskError::BlockedTransition(_, _))),
            "expected build_api to be blocked before bootstrap completes"
        );

        // When: bootstrap reaches done
        update_task(
            &mut store,
            &bootstrap.id,
            TaskUpdate {
                status: Some(UpdateStatus::InProgress),
                ..TaskUpdate::default()
            },
        )
        .expect("start bootstrap");
        update_task(
            &mut store,
            &bootstrap.id,
            TaskUpdate {
                status: Some(UpdateStatus::Completed),
                ..TaskUpdate::default()
            },
        )
        .expect("complete bootstrap");

        // Then build_api becomes ready and can start (observable behavior contract)
        let ready_after = ready_tasks(&store).expect("ready tasks");
        assert!(ready_after.iter().any(|t| t.id == build_api.id));
        let now_running = update_task(
            &mut store,
            &build_api.id,
            TaskUpdate {
                status: Some(UpdateStatus::InProgress),
                ..TaskUpdate::default()
            },
        )
        .expect("start build_api");
        assert_eq!(
            now_running.expect("updated task").status,
            TaskStatus::InProgress
        );
    }

    #[test]
    fn bdd_multiple_ready_tasks_can_run_in_parallel() {
        // Given 3 independent tasks with no blockers
        let mut store = TaskStore::default();
        let mut ids = HashSet::new();
        for idx in 0..3 {
            let task = create_task(&mut store, mk_create(&format!("parallel-task-{idx}")))
                .expect("create task");
            ids.insert(task.id);
        }

        // When: query ready tasks
        let ready = ready_tasks(&store).expect("ready");

        // Then all three are ready and can be started by different agents
        let ready_ids: HashSet<_> = ready.iter().map(|t| t.id.clone()).collect();
        assert_eq!(ready_ids.len(), ids.len());
        assert_eq!(ready_ids, ids);
    }

    #[test]
    fn bdd_state_is_observable_from_store_file() {
        // Given: create dependency chain and write to disk
        let mut store = TaskStore::default();
        let mut meta = serde_json::Map::new();
        meta.insert(
            "priority".to_string(),
            serde_json::Value::String("P0".to_string()),
        );

        let design = create_task(
            &mut store,
            TaskCreate {
                subject: "Design API".to_string(),
                description: "define interfaces".to_string(),
                active_form: None,
                metadata: meta,
            },
        )
        .expect("create design");
        let implement =
            create_task(&mut store, mk_create("Implement API")).expect("create implement");
        update_task(
            &mut store,
            &implement.id,
            TaskUpdate {
                add_blocked_by: vec![design.id.clone()],
                ..TaskUpdate::default()
            },
        )
        .expect("link implement");

        let path = PathBuf::from(env::temp_dir()).join(format!(
            "taskctl-bdd-{}-{}.json",
            std::process::id(),
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap_or_else(|_| std::time::Duration::from_secs(0))
                .as_nanos()
        ));
        save_store(&path, &store).expect("save store");

        // When: loading the persisted store from disk
        let raw = fs::read_to_string(&path).expect("read persisted");
        let parsed: serde_json::Value = serde_json::from_str(&raw).expect("parse persisted");
        assert_eq!(parsed["version"], 1);
        assert_eq!(parsed["tasks"].as_object().expect("tasks obj").len(), 2);
        assert_eq!(
            parsed["tasks"][&design.id]["status"],
            serde_json::Value::String("pending".to_string())
        );
        assert_eq!(
            parsed["tasks"][&implement.id]["blocked_by"][0],
            design.id.clone()
        );

        let loaded = load_store(&path).expect("load");
        let ready = ready_tasks(&loaded).expect("ready");
        assert_eq!(ready.len(), 1);
        assert_eq!(ready[0].id, design.id);

        // Then observability is complete: file contains full intent plus concrete behavior output
        let patch = serde_json::Map::from_iter([
            (
                "status".to_string(),
                serde_json::Value::String("in_progress".to_string()),
            ),
            (
                "module".to_string(),
                serde_json::Value::String("api".to_string()),
            ),
        ]);
        let maybe_err = update_task(
            &mut store,
            &design.id,
            TaskUpdate {
                status: Some(UpdateStatus::InProgress),
                metadata: Some(patch),
                ..TaskUpdate::default()
            },
        )
        .expect("update metadata+status");
        let design = maybe_err.expect("updated design");
        assert_eq!(design.metadata["module"], "api");

        let _ = fs::remove_file(&path);
    }
}
