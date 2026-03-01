use crate::compile::ast::QuotedAST;
use crate::spec::ModuleSpec;

/// 根据 --passes 参数应用 pass pipeline
pub fn apply_passes(
    mut ast: Vec<QuotedAST>,
    spec: &ModuleSpec,
    passes_str: &str,
) -> Vec<QuotedAST> {
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
    ast.into_iter()
        .map(|node| transform_node_envelope(node))
        .collect()
}

fn transform_node_envelope(node: QuotedAST) -> QuotedAST {
    match node {
        QuotedAST::Call { name, meta, args } if name == "defmodule" => QuotedAST::Call {
            name,
            meta,
            args: args.into_iter().map(transform_node_envelope).collect(),
        },
        // 透传 List/Tuple，递归到内部 Block
        QuotedAST::List(items) => {
            QuotedAST::List(items.into_iter().map(transform_node_envelope).collect())
        }
        QuotedAST::Tuple(items) => {
            QuotedAST::Tuple(items.into_iter().map(transform_node_envelope).collect())
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
            // def 的 args: [call_head, [do: body]]
            // 需要递归进入 [do: body] 中的 body 部分来包装返回值
            let new_args: Vec<QuotedAST> = args
                .into_iter()
                .map(|arg| wrap_envelope_in_do_block(arg))
                .collect();
            QuotedAST::Call {
                name,
                meta,
                args: new_args,
            }
        }
        other => other,
    }
}

/// 递归进入 [do: body] 结构，找到返回值并包装为 Envelope
fn wrap_envelope_in_do_block(node: QuotedAST) -> QuotedAST {
    match node {
        // [do: body] → keyword list
        QuotedAST::List(items) => {
            QuotedAST::List(items.into_iter().map(wrap_envelope_in_do_block).collect())
        }
        // {:do, body} → 包装 body 中的返回 tuple
        QuotedAST::Tuple(items) if items.len() == 2 => {
            if let QuotedAST::Atom(ref key) = items[0] {
                if key == "do" {
                    return QuotedAST::Tuple(vec![
                        items[0].clone(),
                        wrap_return_tuple(items[1].clone()),
                    ]);
                }
            }
            QuotedAST::Tuple(items)
        }
        other => other,
    }
}

/// 将 {:ok, value} 包装为 {:ok, %Envelope{data: value}}
fn wrap_return_tuple(node: QuotedAST) -> QuotedAST {
    match node {
        QuotedAST::Tuple(items) if items.len() == 2 => {
            // {:ok, value} → {:ok, %Envelope{data: value}}
            if let QuotedAST::Atom(ref key) = items[0] {
                if key == "ok" {
                    return QuotedAST::Tuple(vec![
                        items[0].clone(),
                        QuotedAST::Call {
                            name: "%".into(),
                            meta: vec![],
                            args: vec![
                                QuotedAST::Aliases {
                                    segments: vec!["Envelope".into()],
                                },
                                QuotedAST::Call {
                                    name: "%{}".into(),
                                    meta: vec![],
                                    args: vec![QuotedAST::List(vec![QuotedAST::Tuple(vec![
                                        QuotedAST::Atom("data".into()),
                                        items[1].clone(),
                                    ])])],
                                },
                            ],
                        },
                    ]);
                }
            }
            QuotedAST::Tuple(items)
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

    ast.into_iter()
        .map(|node| inject_error_codes(node, &all_codes))
        .collect()
}

fn inject_error_codes(node: QuotedAST, codes: &[String]) -> QuotedAST {
    match node {
        QuotedAST::Call { name, meta, args } if name == "defmodule" => QuotedAST::Call {
            name,
            meta,
            args: args
                .into_iter()
                .map(|a| inject_error_codes(a, codes))
                .collect(),
        },
        // 透传 List/Tuple，递归到内部 Block
        QuotedAST::List(items) => QuotedAST::List(
            items
                .into_iter()
                .map(|a| inject_error_codes(a, codes))
                .collect(),
        ),
        QuotedAST::Tuple(items) => QuotedAST::Tuple(
            items
                .into_iter()
                .map(|a| inject_error_codes(a, codes))
                .collect(),
        ),
        QuotedAST::Block(items) => {
            let mut new_items = Vec::new();

            // 注入 ErrorCode 模块属性
            new_items.push(QuotedAST::Call {
                name: "@error_codes".into(),
                meta: vec![],
                args: vec![QuotedAST::List(
                    codes.iter().map(|c| QuotedAST::String(c.clone())).collect(),
                )],
            });

            // 注入 ErrorCode 辅助函数: defp error_code(code, message), do: %ErrorCode{...}
            new_items.push(QuotedAST::Call {
                name: "defp".into(),
                meta: vec![],
                args: vec![
                    QuotedAST::Call {
                        name: "error_code".into(),
                        meta: vec![],
                        args: vec![
                            QuotedAST::Call {
                                name: "code".into(),
                                meta: vec![],
                                args: vec![],
                            },
                            QuotedAST::Call {
                                name: "message".into(),
                                meta: vec![],
                                args: vec![],
                            },
                        ],
                    },
                    QuotedAST::List(vec![QuotedAST::Tuple(vec![
                        QuotedAST::Atom("do".into()),
                        // %ErrorCode{code: code, message: message}
                        QuotedAST::Call {
                            name: "%".into(),
                            meta: vec![],
                            args: vec![
                                QuotedAST::Aliases {
                                    segments: vec!["ErrorCode".into()],
                                },
                                QuotedAST::Call {
                                    name: "%{}".into(),
                                    meta: vec![],
                                    args: vec![QuotedAST::List(vec![
                                        QuotedAST::Tuple(vec![
                                            QuotedAST::Atom("code".into()),
                                            QuotedAST::Call {
                                                name: "code".into(),
                                                meta: vec![],
                                                args: vec![],
                                            },
                                        ]),
                                        QuotedAST::Tuple(vec![
                                            QuotedAST::Atom("message".into()),
                                            QuotedAST::Call {
                                                name: "message".into(),
                                                meta: vec![],
                                                args: vec![],
                                            },
                                        ]),
                                    ])],
                                },
                            ],
                        },
                    ])]),
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
