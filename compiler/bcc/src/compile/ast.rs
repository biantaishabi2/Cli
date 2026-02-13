use serde::Serialize;
use crate::spec::ModuleSpec;

/// 映射 Elixir quoted AST 的三元组结构
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum QuotedAST {
    /// {atom, meta, args} — Elixir AST 基本节点
    Call {
        name: String,
        meta: Vec<(String, serde_json::Value)>,
        args: Vec<QuotedAST>,
    },
    /// {:__aliases__, meta, segments} — 模块名
    Aliases { segments: Vec<String> },
    Atom(String),
    String(String),
    Int(i64),
    List(Vec<QuotedAST>),
    Tuple(Vec<QuotedAST>),
    Block(Vec<QuotedAST>),
}

/// 构建裸骨架 AST：defmodule + @moduledoc + 每个 port 一个 def
pub fn build_skeleton(spec: &ModuleSpec) -> Vec<QuotedAST> {
    let mut body = Vec::new();

    // @moduledoc
    if let Some(ref resp) = spec.module.responsibility {
        body.push(QuotedAST::Call {
            name: "@moduledoc".into(),
            meta: vec![],
            args: vec![QuotedAST::String(resp.clone())],
        });
    }

    // 每个 port 生成 def + @spec
    for port in &spec.ports {
        let kind = port.kind.as_deref().unwrap_or("command");

        // 收集输入参数名
        let param_names: Vec<String> = port.input.as_ref()
            .map(|m| m.keys().cloned().collect())
            .unwrap_or_default();

        // @spec 注解
        let spec_params: Vec<String> = port.input.as_ref()
            .map(|m| m.values().map(|t| yaml_type_to_elixir(t)).collect())
            .unwrap_or_default();
        let spec_return = match kind {
            "query" => {
                let out_types: Vec<String> = port.output.as_ref()
                    .map(|m| m.values().map(|t| yaml_type_to_elixir(t)).collect())
                    .unwrap_or_else(|| vec!["any()".into()]);
                format!("{{:ok, {}}}", out_types.join(", "))
            }
            _ => "{:ok, map()} | {:error, term()}".into(),
        };
        let spec_str = format!(
            "@spec {}({}) :: {}",
            port.name,
            spec_params.join(", "),
            spec_return
        );
        body.push(QuotedAST::Call {
            name: "@spec".into(),
            meta: vec![],
            args: vec![QuotedAST::String(spec_str)],
        });

        // def 函数体
        let params: Vec<QuotedAST> = param_names.iter()
            .map(|n| QuotedAST::Call {
                name: n.clone(),
                meta: vec![],
                args: vec![],
            })
            .collect();

        let return_val = match kind {
            "query" => QuotedAST::Tuple(vec![
                QuotedAST::Atom("ok".into()),
                QuotedAST::Atom("nil".into()),
            ]),
            _ => QuotedAST::Tuple(vec![
                QuotedAST::Atom("ok".into()),
                QuotedAST::Atom("nil".into()),
            ]),
        };

        body.push(QuotedAST::Call {
            name: "def".into(),
            meta: vec![],
            args: vec![
                QuotedAST::Call {
                    name: port.name.clone(),
                    meta: vec![],
                    args: params,
                },
                return_val,
            ],
        });
    }

    // defmodule 包裹
    vec![QuotedAST::Call {
        name: "defmodule".into(),
        meta: vec![],
        args: vec![
            QuotedAST::Aliases { segments: vec![spec.module.name.clone()] },
            QuotedAST::Block(body),
        ],
    }]
}

fn yaml_type_to_elixir(t: &str) -> String {
    match t {
        "uuid" => "String.t()".into(),
        "string" => "String.t()".into(),
        "integer" => "integer()".into(),
        "boolean" => "boolean()".into(),
        "map" => "map()".into(),
        "list" => "list()".into(),
        other => format!("{}()", other),
    }
}
