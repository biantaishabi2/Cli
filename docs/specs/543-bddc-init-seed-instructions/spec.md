# Specification: bddc init 补齐 seed 三件套内置指令

## Overview

修复 `bddc init` 生成脚手架遗漏 seed 三件套的问题，确保新项目可直接消费 `compile-project` 生成的 seed BDD DSL。

## Workflow Type

bugfix

## Task Scope

**In scope**：
- 补齐 `bddc init` 生成的 `instructions_v1.exs` 默认 spec
- 补齐默认 common instructions 的 `@caps` 与 `run!/5` 分支
- 补齐 runtime `__caps_sync_fixture__` 的 seed 指令扫描面
- 新增覆盖 init 模板、`runtime.caps.sync`、seed DSL smoke 的回归测试

**Out of scope**：
- 重做 seed 指令的完整业务语义
- 调整 `compile-project` 的 seed DSL 生成策略

## Success Criteria

- `bddc init` 生成的默认模板包含完整 7 个标准内置指令
- 新 init 项目执行 `runtime.caps.sync` 时能导出 seed 三件套 capability
- 新 init 项目执行 `bddc check` 可通过最小 seed DSL smoke，不再报 `未知指令`

## Dev Environment

| 配置 | 值 |
|------|-----|
| 端口 | 不适用 |
| Worktree | `../Cli-feat-543` |

启动命令：

```bash
cd ../Cli-feat-543
```

## Files to Modify

- `compiler/bddc/lib/bdd_compiler/cli.ex`
- `compiler/bddc/test/cli_init_and_config_test.exs`

## Files to Reference

- `compiler/bcc/src/bdd_seed.rs`
- `compiler/bddc/test/linter_test.exs`
- `compiler/bddc/test/fixtures/mock_project/test/support/bdd/instructions_v1.ex`

## QA Acceptance Criteria

- TC-543-01: init 后模板包含 seed 三件套 spec 与 runtime fixture
- TC-543-02: `runtime.caps.sync` 导出的 caps 文件包含 seed 三件套
- TC-543-03: 默认 init 模板可通过最小 seed DSL smoke

## Test Setup

- 在 `compiler/bddc` 下重新构建 escript
- 使用临时目录创建纯文件项目验证 `init/check/runtime.caps.sync`

## Test Cases

- TC-543-01: 执行 `bddc init --project-root <tmp> --namespace Demo --force`，验证生成的指令模板与 runtime fixture 都包含 seed 三件套
- TC-543-02: 对 init 后项目执行 `bddc runtime.caps.sync --project-root <tmp>`，验证 `runtime_caps_v1.exs` 出现 3 个 seed capability
- TC-543-03: 在 init 后项目写入最小 seed DSL，执行 `bddc check --project-root <tmp> --no-fail-on-warn`，验证退出码为 0 且输出不含未知指令

## Step-by-step Validation

0. **补齐 init 模板（已完成）**
   - 做什么：修改 `cli.ex` 的默认 spec、common instructions、runtime fixture
   - 验证：读取生成文件内容，确认 seed 三件套都出现

1. **补齐回归测试（已完成）**
   - 做什么：增强 `cli_init_and_config_test.exs`
   - 验证：测试覆盖模板内容、caps 导出、seed smoke

2. **运行测试验证（已完成）**
   - 做什么：执行 `mix test compiler/bddc/test/cli_init_and_config_test.exs` 或等价命令
   - 验证：相关测试全部通过

## Notes

seed 三件套已经被 `compile-project` 和 linter 当成标准指令，问题在于 `bddc init` 模板陈旧。

## Progress

- [x] 已创建 worktree `feat/543-bddc-init-seed-instructions`
- [x] 已补齐 init 模板与 runtime fixture
- [x] 已补齐并通过回归测试

## Next

- 视需要提交 commit / 推送分支 / 创建 PR
