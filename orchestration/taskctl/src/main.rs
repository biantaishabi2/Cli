use clap::{Parser, Subcommand, ValueEnum};
use serde_json::{Map, Value};
use std::collections::BTreeMap;
use std::path::PathBuf;
use taskctl::{
    DagGraph, TaskCreate, TaskStatus, TaskUpdate, UpdateStatus, create_task, dag_graph,
    default_store_path, delete_task, get_task, list_tasks, load_store, ready_tasks, save_store,
    update_task, validate_store,
};

const EXAMPLES: &str = r#"Examples:
  taskctl --store ./tasks.json create \
    --subject \"Run tests\" \
    --description \"Execute backend tests\" \
    --active-form \"Running tests\" \
    --metadata '{"priority":"P1","module":"quality"}'

  taskctl --store ./tasks.json update \
    --task-id <TASK_ID> \
    --owner qa@team \
    --add-blocked-by <DEP_ID_1>,<DEP_ID_2> \
    --status in-progress

  taskctl --store ./tasks.json dag
  taskctl --store ./tasks.json dag-ascii
  taskctl --store ./tasks.json validate
"#;

#[derive(Parser, Debug)]
#[command(name = "taskctl")]
#[command(about = "Task DAG CLI for human/agent collaboration")]
#[command(
    long_about = "Task DAG CLI for human/agent collaboration.\n\nUse this tool to create/update tasks, manage dependencies, compute ready tasks, and validate DAG constraints."
)]
#[command(after_help = EXAMPLES)]
struct Cli {
    #[arg(long, global = true, help = "Path to task store JSON file")]
    store: Option<PathBuf>,
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand, Debug)]
enum Commands {
    #[command(about = "Create a task in pending status")]
    Create {
        #[arg(long, help = "Task title in imperative style, e.g. 'Run tests'")]
        subject: String,
        #[arg(long, help = "Detailed context and acceptance criteria")]
        description: String,
        #[arg(long, help = "Progressive wording for spinner, e.g. 'Running tests'")]
        active_form: Option<String>,
        #[arg(long, help = "Metadata JSON object, e.g. '{\"priority\":\"P1\"}'")]
        metadata: Option<String>,
    },
    #[command(about = "Update task content, status, owner, metadata and dependencies")]
    Update {
        #[arg(long, help = "Task ID")]
        task_id: String,
        #[arg(
            long,
            help = "Status transition: pending -> in-progress -> completed, or deleted"
        )]
        status: Option<StatusArg>,
        #[arg(long, help = "Updated task title")]
        subject: Option<String>,
        #[arg(long, help = "Updated task description")]
        description: Option<String>,
        #[arg(long, help = "Updated active-form text")]
        active_form: Option<String>,
        #[arg(long, help = "Task owner, human or agent identity")]
        owner: Option<String>,
        #[arg(long, help = "Metadata JSON patch object; null value deletes key")]
        metadata: Option<String>,
        #[arg(long, value_delimiter = ',', help = "Task IDs blocked by this task")]
        add_blocks: Vec<String>,
        #[arg(
            long,
            value_delimiter = ',',
            help = "Dependency task IDs that block this task"
        )]
        add_blocked_by: Vec<String>,
    },
    #[command(about = "Delete task and clean reverse dependency links")]
    Delete {
        #[arg(long, help = "Task ID")]
        task_id: String,
    },
    #[command(about = "Get one task by ID")]
    Get {
        #[arg(long, help = "Task ID")]
        task_id: String,
    },
    #[command(about = "List tasks with optional status/owner filtering")]
    List {
        #[arg(long, help = "Filter by status")]
        status: Option<ListStatusArg>,
        #[arg(long, help = "Filter by owner")]
        owner: Option<String>,
    },
    #[command(about = "List pending tasks whose dependencies are fully completed")]
    Ready,
    #[command(about = "Validate graph integrity and acyclic DAG constraints")]
    Validate,
    #[command(
        about = "Generate DAG JSON (topo_order, layers, nodes, edges)",
        alias = "generate"
    )]
    Dag,
    #[command(about = "Render DAG in ASCII text for quick review", alias = "ascii")]
    DagAscii,
}

#[derive(ValueEnum, Clone, Copy, Debug)]
enum StatusArg {
    Pending,
    InProgress,
    Completed,
    Deleted,
}

#[derive(ValueEnum, Clone, Copy, Debug)]
enum ListStatusArg {
    Pending,
    InProgress,
    Completed,
}

impl From<StatusArg> for UpdateStatus {
    fn from(value: StatusArg) -> Self {
        match value {
            StatusArg::Pending => UpdateStatus::Pending,
            StatusArg::InProgress => UpdateStatus::InProgress,
            StatusArg::Completed => UpdateStatus::Completed,
            StatusArg::Deleted => UpdateStatus::Deleted,
        }
    }
}

impl From<ListStatusArg> for TaskStatus {
    fn from(value: ListStatusArg) -> Self {
        match value {
            ListStatusArg::Pending => TaskStatus::Pending,
            ListStatusArg::InProgress => TaskStatus::InProgress,
            ListStatusArg::Completed => TaskStatus::Completed,
        }
    }
}

fn parse_metadata(raw: Option<String>) -> Result<Map<String, Value>, String> {
    match raw {
        None => Ok(Map::new()),
        Some(s) => {
            let v: Value =
                serde_json::from_str(&s).map_err(|e| format!("invalid metadata json: {e}"))?;
            match v {
                Value::Object(map) => Ok(map),
                _ => Err("metadata must be a JSON object".to_string()),
            }
        }
    }
}

fn print_json<T: serde::Serialize>(value: &T) {
    match serde_json::to_string_pretty(value) {
        Ok(raw) => println!("{raw}"),
        Err(err) => {
            eprintln!("serialize error: {err}");
            std::process::exit(1);
        }
    }
}

fn task_label(id: &str, id_to_subject: &BTreeMap<String, String>) -> String {
    id_to_subject
        .get(id)
        .map_or_else(|| id.to_string(), std::clone::Clone::clone)
}

fn render_dag_ascii(dag: &DagGraph) -> String {
    let mut out = String::new();
    let mut id_to_subject = BTreeMap::new();
    for node in &dag.nodes {
        id_to_subject.insert(node.id.clone(), node.subject.clone());
    }

    out.push_str("DAG Layers\n");
    out.push_str("----------\n");
    for (idx, layer) in dag.layers.iter().enumerate() {
        let names = layer
            .iter()
            .map(|id| task_label(id, &id_to_subject))
            .collect::<Vec<_>>()
            .join(" | ");
        out.push_str(&format!("L{idx}: {names}\n"));
    }

    out.push_str("\nDAG Edges\n");
    out.push_str("---------\n");
    for edge in &dag.edges {
        let from = task_label(&edge.from, &id_to_subject);
        let to = task_label(&edge.to, &id_to_subject);
        out.push_str(&format!("{from} -> {to}\n"));
    }

    out
}

fn main() {
    let cli = Cli::parse();
    let store_path = cli.store.unwrap_or_else(default_store_path);

    if let Err(err) = run(cli.command, &store_path) {
        eprintln!("error: {err}");
        std::process::exit(1);
    }
}

fn run(command: Commands, store_path: &PathBuf) -> Result<(), String> {
    match command {
        Commands::Create {
            subject,
            description,
            active_form,
            metadata,
        } => {
            let mut store = load_store(store_path).map_err(|e| e.to_string())?;
            let meta = parse_metadata(metadata)?;
            let task = create_task(
                &mut store,
                TaskCreate {
                    subject,
                    description,
                    active_form,
                    metadata: meta,
                },
            )
            .map_err(|e| e.to_string())?;
            save_store(store_path, &store).map_err(|e| e.to_string())?;
            print_json(&task);
        }
        Commands::Update {
            task_id,
            status,
            subject,
            description,
            active_form,
            owner,
            metadata,
            add_blocks,
            add_blocked_by,
        } => {
            let mut store = load_store(store_path).map_err(|e| e.to_string())?;
            let metadata_patch = if metadata.is_some() {
                Some(parse_metadata(metadata)?)
            } else {
                None
            };
            let updated = update_task(
                &mut store,
                &task_id,
                TaskUpdate {
                    status: status.map(Into::into),
                    subject,
                    description,
                    active_form,
                    owner,
                    metadata: metadata_patch,
                    add_blocks,
                    add_blocked_by,
                },
            )
            .map_err(|e| e.to_string())?;
            save_store(store_path, &store).map_err(|e| e.to_string())?;
            match updated {
                Some(task) => print_json(&task),
                None => print_json(&serde_json::json!({"deleted": task_id})),
            }
        }
        Commands::Delete { task_id } => {
            let mut store = load_store(store_path).map_err(|e| e.to_string())?;
            delete_task(&mut store, &task_id).map_err(|e| e.to_string())?;
            save_store(store_path, &store).map_err(|e| e.to_string())?;
            print_json(&serde_json::json!({"deleted": task_id}));
        }
        Commands::Get { task_id } => {
            let store = load_store(store_path).map_err(|e| e.to_string())?;
            let task = get_task(&store, &task_id).map_err(|e| e.to_string())?;
            print_json(task);
        }
        Commands::List { status, owner } => {
            let store = load_store(store_path).map_err(|e| e.to_string())?;
            let mut tasks = list_tasks(&store);
            if let Some(want) = status.map(Into::into) {
                tasks.retain(|t| t.status == want);
            }
            if let Some(owner_value) = owner {
                tasks.retain(|t| t.owner.as_deref() == Some(owner_value.as_str()));
            }
            print_json(&tasks);
        }
        Commands::Ready => {
            let store = load_store(store_path).map_err(|e| e.to_string())?;
            let tasks = ready_tasks(&store).map_err(|e| e.to_string())?;
            print_json(&tasks);
        }
        Commands::Validate => {
            let store = load_store(store_path).map_err(|e| e.to_string())?;
            validate_store(&store).map_err(|e| e.to_string())?;
            print_json(&serde_json::json!({"ok": true}));
        }
        Commands::Dag => {
            let store = load_store(store_path).map_err(|e| e.to_string())?;
            let dag = dag_graph(&store).map_err(|e| e.to_string())?;
            print_json(&dag);
        }
        Commands::DagAscii => {
            let store = load_store(store_path).map_err(|e| e.to_string())?;
            let dag = dag_graph(&store).map_err(|e| e.to_string())?;
            print!("{}", render_dag_ascii(&dag));
        }
    }
    Ok(())
}
