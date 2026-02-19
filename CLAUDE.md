# Cli 项目开发指南

## 项目结构

```
Cli/
├── compiler/bcc/          # BCC 编译器（主要开发目标）
│   ├── src/
│   │   ├── main.rs        # CLI 入口（clap 命令定义）
│   │   ├── bugfix.rs      # bugfix 流水线（collect/context/generate/organize）
│   │   ├── arch.rs        # 架构矩阵与门禁
│   │   ├── bdd_seed.rs    # BDD 场景种子生成
│   │   ├── compile/       # YAML → Elixir 编译
│   │   ├── extract/       # 源码结构提取（php/elixir/typescript/rust）
│   │   ├── spec.rs        # 规格定义
│   │   └── trace.rs       # 文档覆盖率审计
│   ├── tests/
│   │   ├── bugfix_bdd.rs  # bugfix 集成测试（BDD 风格）
│   │   └── cli_arch_bdd.rs # arch/bdd CLI 冒烟测试
│   └── Cargo.toml
└── orchestration/taskctl/  # 任务编排（独立 crate）
```

## 构建与测试

```bash
# 编译检查
cargo check -p bcc

# 运行全部测试
cargo test -p bcc

# 只运行 bugfix 相关测试
cargo test -p bcc bugfix

# 只运行集成测试
cargo test -p bcc --test bugfix_bdd

# 构建 release
cargo build --release -p bcc
```

## 开发流程（遵循全局 Issue 驱动规范）

1. `gh issue create` 创建 issue（描述背景、目标、变更范围）
2. issue 或 plan 中写明测试场景（输入、预期输出、边界条件），不能只写"包含测试"
3. 创建分支（二选一）：
   - 常规：`git checkout master && git checkout -b feat/<N>-<slug>`
   - 并行开发：`git worktree add ../Cli-feat-<N> -b feat/<N>-<slug> master`
4. 实现代码 + 测试
5. `cargo check && cargo test` 全过
6. 提交（带 `#N`）→ 推送 → `gh pr create`
7. 合并到 master
8. 如用 worktree，合并后清理：`git worktree remove ../Cli-feat-<N>`

## Niuma 状态标签操作（机器人必读）

- 禁止直接执行：`gh issue edit --add-label/--remove-label bot:*`
- 统一使用受控入口：`niuma state-label`

```bash
# 单 issue 流程
niuma state-label set --repo <owner/repo> --issue <num> --to bot:fix

# 多 issue DAG 入口
niuma state-label set --repo <owner/repo> --issue <num> --to bot:orchestrate
```

- 建议先安装 gh wrapper：`bash automation/niuma/scripts/install-gh-wrapper.sh`

## 测试规范

- **单元测试**：写在对应源文件的 `#[cfg(test)] mod tests` 中（如 `bugfix.rs` 底部）
- **集成测试**：写在 `tests/bugfix_bdd.rs`，使用真实 git 仓库做端到端验证
- 集成测试通过 `env!("CARGO_BIN_EXE_bcc")` 调用编译好的二进制
- 降级模式测试设置 `BCC_FORCE_PROMPT_MODE=1` 避免依赖外部 codex

## bugfix 流水线

四步流水线：`collect(c)` → `context(x)` → `generate(g)` → `organize(o)`

```bash
# 全量执行
bcc bugfix <repo> -o <output> --lang rust

# 单步执行
bcc bugfix <repo> -o <output> --lang rust -s c    # 只 collect
bcc bugfix <repo> -o <output> --lang rust -s x    # 只 context
bcc bugfix <repo> -o <output> --lang rust -s g    # 只 generate

# 常用选项
--force          # 强制重做
--grade A,B,C    # 分级过滤（默认 A,B）
--limit N        # 限制处理数量
--issue N        # 手动指定关联的 GitHub Issue
--branch main    # 指定扫描分支
```

## 注意事项

- `bugfix.rs` 的 `run()` 函数签名变更时，需要同步更新 `main.rs` 的调用和 `mod tests` 中的直接调用
- `BugfixCommit` 新增字段时，用 `#[serde(skip_serializing_if = "Option::is_none")]` 保持向后兼容
- `parse_git_log` 中构造 `BugfixCommit` 需要初始化所有字段
