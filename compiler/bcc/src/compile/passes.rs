use crate::compile::ast::QuotedAST;
use crate::spec::ModuleSpec;

/// 根据 --passes 参数应用 pass pipeline
pub fn apply_passes(mut ast: Vec<QuotedAST>, spec: &ModuleSpec, passes_str: &str) -> Vec<QuotedAST> {
    if passes_str == "none" {
        return ast;
    }

    let pass_names: Vec<&str> = passes_str.split(',').map(|s| s.trim()).collect();

    for pass_name in pass_names {
        ast = match pass_name {
            "envelope" => envelope_pass(ast, spec),
            "error_code" => error_code_pass(ast, spec),
            other => {
                eprintln!("[WARNING] unknown pass '{}', skipping", other);
                ast
            }
        };
    }

    ast
}

/// envelope_pass: 将函数返回值包装为 Envelope 结构
fn envelope_pass(ast: Vec<QuotedAST>, _spec: &ModuleSpec) -> Vec<QuotedAST> {
    ast.into_iter().map(|node| transform_node_envelope(node)).collect()
}

fn transform_node_envelope(node: QuotedAST) -> QuotedAST {
    match node {
        QuotedAST::Call { name, meta, args } if name == "defmodule" => {
            QuotedAST::Call {
                name,
                meta,
                args: args.into_iter().map(transform_node_envelope).collect(),
            }
        }
        QuotedAST::Block(items) => {
            let mut new_items = Vec::new();
            // 在 block 开头注入 Envelope 别名
            new_items.push(QuotedAST::Call {
                name: "alias".into(),
                meta: vec![],
                args: vec![QuotedAST::Aliases {
                    segments: vec!["Envelope".into()],
                }],
            });
            for item in items {
                new_items.push(transform_node_envelope(item));
            }
            QuotedAST::Block(new_items)
        }
        QuotedAST::Call { name, meta, args } if name == "def" => {
            // 替换返回值为 Envelope 包装
            let new_args: Vec<QuotedAST> = args.into_iter().map(|arg| {
                match arg {
                    QuotedAST::Tuple(items) if items.len() == 2 => {
                        // {:ok, value} → {:ok, %Envelope{data: value}}
                        QuotedAST::Tuple(vec![
                            items[0].clone(),
                            QuotedAST::Call {
                                name: "%Envelope{}".into(),
                                meta: vec![],
                                args: vec![items[1].clone()],
                            },
                        ])
                    }
                    other => other,
                }
            }).collect();
            QuotedAST::Call { name, meta, args: new_args }
        }
        other => other,
    }
}

/// error_code_pass: 注入 ErrorCode 结构
fn error_code_pass(ast: Vec<QuotedAST>, spec: &ModuleSpec) -> Vec<QuotedAST> {
    // 收集所有 error codes
    let mut all_codes: Vec<String> = Vec::new();
    for port in &spec.ports {
        for code in &port.errors {
            if !all_codes.contains(code) {
                all_codes.push(code.clone());
            }
        }
    }

    if all_codes.is_empty() {
        return ast;
    }

    ast.into_iter().map(|node| inject_error_codes(node, &all_codes)).collect()
}

fn inject_error_codes(node: QuotedAST, codes: &[String]) -> QuotedAST {
    match node {
        QuotedAST::Call { name, meta, args } if name == "defmodule" => {
            QuotedAST::Call {
                name,
                meta,
                args: args.into_iter().map(|a| inject_error_codes(a, codes)).collect(),
            }
        }
        QuotedAST::Block(items) => {
            let mut new_items = Vec::new();

            // 注入 ErrorCode 模块属性
            new_items.push(QuotedAST::Call {
                name: "@error_codes".into(),
                meta: vec![],
                args: vec![QuotedAST::List(
                    codes.iter().map(|c| QuotedAST::String(c.clone())).collect()
                )],
            });

            // 注入 ErrorCode 辅助函数
            new_items.push(QuotedAST::Call {
                name: "defp".into(),
                meta: vec![],
                args: vec![
                    QuotedAST::Call {
                        name: "error_code".into(),
                        meta: vec![],
                        args: vec![
                            QuotedAST::Call { name: "code".into(), meta: vec![], args: vec![] },
                            QuotedAST::Call { name: "message".into(), meta: vec![], args: vec![] },
                        ],
                    },
                    QuotedAST::Call {
                        name: "%ErrorCode{}".into(),
                        meta: vec![],
                        args: vec![
                            QuotedAST::String("code".into()),
                            QuotedAST::String("message".into()),
                        ],
                    },
                ],
            });

            for item in items {
                new_items.push(inject_error_codes(item, codes));
            }
            QuotedAST::Block(new_items)
        }
        other => other,
    }
}
