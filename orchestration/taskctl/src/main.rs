use clap::{Parser, Subcommand, ValueEnum};
use serde::de::DeserializeOwned;
use serde_json::{Map, Value};
use std::collections::BTreeMap;
use std::fs;
use std::io::Read;
use std::path::PathBuf;
use taskctl::{
    CoreError, CoreResponse, DagGraph, ExecuteInput, PlanInput, ResearchInput, TaskCreate,
    TaskStatus, TaskUpdate, UpdateStatus, create_task, dag_graph, default_store_path, delete_task,
    execute, get_task, list_tasks, load_store, plan, ready_tasks, research, save_store,
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

  taskctl research reduce --input ./research.json
  taskctl plan solve --input ./plan.json
  taskctl execute compile --input ./execute.json
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
    #[command(about = "Research graph computing commands")]
    Research {
        #[command(subcommand)]
        command: ResearchCommands,
    },
    #[command(about = "Plan graph computing commands")]
    Plan {
        #[command(subcommand)]
        command: PlanCommands,
    },
    #[command(about = "Execute graph computing commands")]
    Execute {
        #[command(subcommand)]
        command: ExecuteCommands,
    },
}

#[derive(Subcommand, Debug)]
enum ResearchCommands {
    #[command(about = "Reduce research evidences into aggregated graph")]
    Reduce {
        #[arg(long, help = "JSON input file path; defaults to STDIN")]
        input: Option<PathBuf>,
    },
}

#[derive(Subcommand, Debug)]
enum PlanCommands {
    #[command(about = "Solve AND-OR plan graph")]
    Solve {
        #[arg(long, help = "JSON input file path; defaults to STDIN")]
        input: Option<PathBuf>,
    },
}

#[derive(Subcommand, Debug)]
enum ExecuteCommands {
    #[command(about = "Compile plan decision into executable DAG")]
    Compile {
        #[arg(long, help = "JSON input file path; defaults to STDIN")]
        input: Option<PathBuf>,
    },
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

fn parse_json_input<T: DeserializeOwned>(input: Option<PathBuf>) -> Result<T, CoreError> {
    let raw = match input {
        Some(path) => fs::read_to_string(path)
            .map_err(|e| CoreError::invalid_input(format!("failed to read input file: {e}")))?,
        None => {
            let mut raw = String::new();
            std::io::stdin()
                .read_to_string(&mut raw)
                .map_err(|e| CoreError::invalid_input(format!("failed to read stdin: {e}")))?;
            raw
        }
    };

    serde_json::from_str::<T>(&raw)
        .map_err(|e| CoreError::invalid_input(format!("invalid input json: {e}")))
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
    let code = run(cli.command, &store_path);
    std::process::exit(code);
}

fn run(command: Commands, store_path: &PathBuf) -> i32 {
    match command {
        Commands::Research { command } => {
            let result = match command {
                ResearchCommands::Reduce { input } => {
                    let input: Result<ResearchInput, CoreError> = parse_json_input(input);
                    input.and_then(research::reduce)
                }
            };
            return print_core_result(
                result.map(|(graph, diagnostics)| CoreResponse::ok_graph(graph, diagnostics)),
            );
        }
        Commands::Plan { command } => {
            let result = match command {
                PlanCommands::Solve { input } => {
                    let input: Result<PlanInput, CoreError> = parse_json_input(input);
                    input.and_then(plan::solve)
                }
            };
            return print_core_result(
                result.map(|(decision, diagnostics)| CoreResponse::ok_plan(decision, diagnostics)),
            );
        }
        Commands::Execute { command } => {
            let result = match command {
                ExecuteCommands::Compile { input } => {
                    let input: Result<ExecuteInput, CoreError> = parse_json_input(input);
                    input.and_then(execute::compile)
                }
            };
            return print_core_result(
                result.map(|(dag, diagnostics)| CoreResponse::ok_dag(dag, diagnostics)),
            );
        }
        other => {
            if let Err(err) = run_legacy(other, store_path) {
                eprintln!("error: {err}");
                return 1;
            }
            0
        }
    }
}

fn print_core_result(result: Result<CoreResponse, CoreError>) -> i32 {
    match result {
        Ok(response) => {
            print_json(&response);
            0
        }
        Err(err) => {
            let response = CoreResponse::from_error(err.clone());
            print_json(&response);
            err.exit_code()
        }
    }
}

fn run_legacy(command: Commands, store_path: &PathBuf) -> Result<(), String> {
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
        Commands::Research { .. } | Commands::Plan { .. } | Commands::Execute { .. } => {
            unreachable!("core commands are handled before legacy dispatcher")
        }
    }
    Ok(())
}
