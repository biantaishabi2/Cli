# BCC — Backend Compiler

YAML 驱动的 Elixir 骨架生成器，含源码结构提取、文档覆盖审计和 git bugfix BDD 场景挖掘。

## 安装

```bash
# 首次安装（编译 + symlink 到 ~/.local/bin）
./compiler/bcc/install.sh --link --rebuild

# 后续更新（代码改动后重新编译）
./compiler/bcc/install.sh --rebuild
```

需要 Rust 工具链（`cargo`）。安装后确保 `~/.local/bin` 在 `PATH` 中。

## 子命令

```
bcc compile   YAML 契约 → Elixir 模块骨架
bcc extract   源码 → FileRecord JSON（Elixir/TypeScript/PHP）
bcc trace     文档覆盖审计（status/report/seed）
bcc bugfix    git bugfix 历史 → bddc DSL 场景
```

### bcc compile

```bash
bcc compile contract.yaml -o output/
bcc compile contract.yaml --dry-run        # 只校验不生成
bcc compile contract.yaml --emit-ast       # 输出 AST JSON
```

### bcc extract

```bash
bcc extract lib/shop/order/cart.ex --mode ast    # JSON 结构
bcc extract lib/shop/order/cart.ex --mode doc    # 分析文档
bcc extract lib/shop/order/cart.ex --mode yaml   # YAML draft
bcc extract app/controllers/Foo.php --mode ast   # PHP 支持
```

### bcc trace

```bash
bcc trace status lib/ docs/           # 覆盖率概览
bcc trace report lib/ docs/ --output report/  # 生成报告
bcc trace seed lib/ docs/ --write     # 补充缺失文档模板
```

### bcc bugfix

从 git bugfix 历史中提取 BDD 场景，四步流水线：

```
collect(c)  → git log 扫描、分级(A/B/C)、自动打标签
context(x)  → diff + 函数体 before/after 提取
generate(g) → codex exec 生成 bddc DSL 场景
organize(o) → 按模块归类、重复检测、覆盖率报告
```

```bash
bcc bugfix /path/to/repo -o output/                    # 全量执行
bcc bugfix /path/to/repo -o output/ -s c               # 只扫描
bcc bugfix /path/to/repo -o output/ -s x               # 扫描 + 上下文
bcc bugfix /path/to/repo -o output/ -s g --limit 20    # 前 20 个到生成
bcc bugfix /path/to/repo -o output/ --lang elixir      # Elixir 项目
bcc bugfix /path/to/repo -o output/ --lang typescript   # TypeScript 项目
```

## 支持语言

| 语言 | extract | bugfix | tree-sitter |
|------|---------|--------|-------------|
| Elixir | .ex .exs | .ex .exs | tree-sitter-elixir 0.3 |
| TypeScript | .ts .tsx | .ts .tsx | tree-sitter-typescript 0.23 |
| PHP | .php | .php | tree-sitter-php 0.24 |

## 测试

```bash
cd compiler/bcc
cargo test          # 30 个单测
cargo run -- extract fixtures/sample_controller.php --mode ast  # PHP 端到端
```

## 里程碑

| 阶段 | 状态 | 说明 |
|------|------|------|
| M4 核心 | ✅ | compile/extract/trace 三命令闭环，51 场景通过 |
| M5 扩展 | ✅ | +bugfix 四步流水线 + PHP extract + 多语言，69 场景通过 |
| M6 推广 | 待启动 | CI 集成 + 升级/回滚手册 |

## 文档

- [技术设计文档](docs/技术设计文档-后端编译器.md) — 完整设计、BDD 场景、里程碑
- [BDD 场景提取方案](docs/BDD场景提取方案.md) — bugfix 子命令详细设计
- [架构闭环迁移计划](docs/架构闭环迁移计划.md) — TS 过渡到 Rust 的闭环落地路径与待实现清单
