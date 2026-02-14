# Git Bugfix → BDD 场景提取方案

> 子文档，隶属于 [技术设计文档-后端编译器.md](./技术设计文档-后端编译器.md) §18.2 M5 扩展。

## 一、背景与目标

FIT365 项目 6 年开发积累约 24,570 个 commit，其中约 **3,700 个 bugfix commit**（含"修复"~1,687 + "fix/bug"~2,377，有重叠）。项目无测试覆盖，但每个 bugfix 本身就是**生产环境验证过的边界条件**。

**目标**：从 git bugfix 历史中提取 bddc DSL 场景，为 Phoenix 迁移提供回归测试覆盖。

**工具载体**：所有能力收归 `bcc` 子命令，不散落为独立脚本。

## 二、原始素材分析

### 2.1 数据总量

| 来源 | commit 数 | 涉及后端逻辑 |
|------|----------|-------------|
| 含"修复" | ~1,687 | ~1,035 |
| 含"fix/bug"（不含修复） | ~2,377 | ~1,700（估算） |
| **合计（去重）** | **~3,700** | **~2,500** |

### 2.2 素材质量分级

| 级别 | 条件 | 数量 | BDD 转化率 | 说明 |
|------|------|------|----------|------|
| **A** | 改动 ≤10 行，涉及 Controller/Model/Trait | ~696 | 高（~80%） | 条件判断、类型转换、空值处理 |
| **B** | 改动 10-50 行 | ~245 | 中（~50%） | 需更多上下文 |
| **C** | 改动 >50 行 | ~94 | 低（~20%） | 重构混合修复，需人工判断 |

### 2.3 典型 Bug 类型

| Bug 类型 | 典型案例 | BDD 价值 |
|---------|---------|---------|
| 时区/时间 | UTC 存储但用本地时间查询（+8h） | 高 — 直接映射断言 |
| 空值/类型 | 日期为空传空字符串导致 DB 报错 | 高 — 边界条件 |
| 权限越权 | 订单任意访问、跨企业查数据 | 高 — 安全 BDD |
| 安全漏洞 | XSS 未转义、SSRF、URL 重定向 | 高 — 安全 BDD |
| 业务逻辑 | 盘点驳回未 commit 事务 | 高 — 流程 BDD |
| 分页/查询 | 分页参数缺失、SQL 范围不准 | 中 |
| 并发/一致性 | 导出数据不对、同步 bug | 中 |

### 2.4 Bug 密度 Top 20 控制器

| 排名 | 控制器 | 修复次数 | 模块 |
|------|--------|---------|------|
| 1 | OaactiveController | 42 | B.活动服务 |
| 2 | AsyncdownloadController | 41 | K.定时异步 |
| 3 | ShopexchangedataController | 37 | C.商城系统 |
| 4 | FirmclubController | 31 | B.活动服务 |
| 5 | FirmformController | 28 | J.客户定制 |
| 6 | SportsmeetsController | 28 | E.运动健康 |
| 7 | FirmoperateController | 28 | H.企业后台 |
| 8 | BsactiveenlistController | 28 | B.活动服务 |
| 9 | QuartzController | 27 | K.定时异步 |
| 10 | IatoolController | 21 | H.企业后台 |
| 11 | SportsmeetController | 20 | E.运动健康 |
| 12 | OauserController | 18 | H.企业后台 |
| 13 | Oascorerank4gccController | 18 | G.积分考核 |
| 14 | TradeunionController | 18 | H.企业后台 |
| 15 | OaapproveController | 16 | A.审批系统 |
| 16 | IauserController | 16 | H.企业后台 |
| 17 | JdvopController | 16 | C.商城系统 |
| 18 | OamomentController | 16 | H.企业后台 |
| 19 | OagoodsController | 15 | C.商城系统 |
| 20 | PropertyController | 13 | H.企业后台 |

## 三、工具架构

### 3.1 bcc 现有能力

```
bcc compile   — YAML 契约 → Elixir 模块骨架
bcc extract   — 源码 → FileRecord JSON（Elixir/TypeScript）
bcc trace     — 文档覆盖审计（status/report/seed）
```

### 3.2 新增能力

```
bcc extract（扩展）         — 新增 PHP 语言支持（tree-sitter-php）
bcc bugfix collect          — git log 扫描 → 分级清单 JSON
bcc bugfix context          — 清单 → diff + 函数上下文 JSON 包
bcc bugfix generate         — context 包 → bddc DSL 场景（调用本地 codex exec）
bcc bugfix organize         — 场景归类去重 → 最终 .dsl 文件
bcc bugfix pipeline         — 一键串联 collect → organize
```

### 3.3 整体流程

```
                    bcc 内部
┌──────────────────────────────────────────────────┐
│                                                  │
│  bugfix collect   bugfix context   bugfix generate  bugfix organize
│  ┌───────────┐   ┌────────────┐   ┌────────────┐  ┌────────────┐
│  │ git log   │   │ git diff   │   │ prompt构造  │  │ 归类去重   │
│  │ 扫描过滤  │──→│ + extract  │──→│ codex exec │─→│ → .dsl     │
│  │ 分级标签  │   │ PHP解析    │   │ → bddc DSL │  │ 覆盖率报告 │
│  └───────────┘   └────────────┘   └────────────┘  └─────┬──────┘
│                                                         │
└─────────────────────────────────────────────────────────┘
                                                          │
                    bddc 现有能力                           ▼
┌──────────────────────────────────────────────────┐
│  bddc compile → ExUnit 测试                       │
│  bddc lint    → 静态检查                          │
│  bddc check   → 运行时覆盖门禁                    │
└──────────────────────────────────────────────────┘
```

## 四、子命令详细设计

### 4.1 `bcc extract` 扩展：PHP 支持

新增 `extract/php.rs`，用 `tree-sitter-php` 解析 PHP 源码，输出 FileRecord JSON（结构与 Elixir 版一致）。

```
bcc extract app/controllers/outterajax/OaactiveController.php --mode ast
```

输出示例：
```json
{
  "language": "php",
  "file_path": "app/controllers/outterajax/OaactiveController.php",
  "module_doc": null,
  "exports": [
    {"name": "enrollAction", "kind": "method", "signature": "public function enrollAction()", "line": 45}
  ],
  "imports": [],
  "calls": [
    {"callee": "SysAssess", "line": 2006}
  ],
  "side_effects": {"hasAsync": false, "hasHttp": false},
  "loc_lines": 3200,
  "declarations": 67
}
```

PHP 特殊处理：
- 识别 `public function xxxAction()` 模式（Phalcon 约定）
- class 继承（`extends OutterController`）
- trait use（`use ValidateParamsTrait`）
- 静态方法调用（`SysAssess::P(...)`）作为 calls

**Cargo.toml 唯一新增依赖**：
```toml
tree-sitter-php = "0.24"
```

### 4.2 `bcc bugfix collect`

```
bcc bugfix collect /path/to/repo \
    --keywords "修复,fix,bug" \
    --output bugfix_inventory.json \
    --module-map /path/to/module_map.json
```

功能：
1. `git log --all --no-merges --grep=<keyword> --numstat --format` 扫描
2. 多关键字结果按 commit hash 去重
3. 过滤噪声（只保留改了 `.php` 且含 controllers/models/traits 的）
4. 按变更行数分级：A(≤10) / B(10-50) / C(>50)
5. 按 module_map 映射到业务模块
6. 自动打标签（匹配 commit message）

输出 `bugfix_inventory.json`：
```json
{
  "meta": {
    "repo": "/path/to/repo",
    "scanned_at": "2026-02-14T...",
    "total_commits": 24570,
    "bugfix_commits": 2500,
    "by_grade": {"A": 1500, "B": 600, "C": 400}
  },
  "commits": [
    {
      "hash": "014743023209",
      "message": "修复评论xss漏洞",
      "author": "liupan",
      "date": "2025-12-22",
      "grade": "A",
      "module": "H",
      "tags": ["security", "xss"],
      "changed_files": [
        {"path": "app/controllers/outterajax/OauserController.php", "add": 2, "del": 1, "kind": "controller"}
      ],
      "total_lines": 3
    }
  ]
}
```

**module_map.json schema**：
```json
{
  "mapping": {
    "outterajax/Oaactive": "B",
    "outterajax/Shop": "C",
    "admin/Property": "H"
  },
  "module_names": {
    "A": "审批系统",
    "B": "活动服务",
    "C": "商城系统"
  }
}
```

**自动标签规则**（内置，匹配 commit message）：
```
xss|漏洞|注入|安全|越权|任意访问  → security
分页|翻页|page|offset             → pagination
导出|导入|下载|export              → export
时间|日期|时区|UTC|timezone        → datetime
空|null|为空|类型|type             → null_safety
事务|commit|回滚|rollback         → transaction
并发|锁|重复|concurrent           → concurrency
```

### 4.3 `bcc bugfix context`

```
bcc bugfix context bugfix_inventory.json \
    --repo /path/to/repo \
    --output bugfix_contexts/ \
    --grade A,B \
    --limit 100
```

功能：对清单中每个 commit 提取完整上下文包。

核心流程：
1. `git diff HASH^..HASH` 获取 raw diff
2. 解析 diff hunk header 定位改动行
3. `git show HASH:path` / `git show HASH^:path` 获取修复前后完整文件
4. 复用 `extract::php` 的 tree-sitter 解析，定位改动所在的函数
5. 提取函数体（before/after）

断点续传：已存在 `{hash}.json` 的跳过，`--force` 强制重做。

输出：每个 commit 一个 JSON 文件：
```json
{
  "hash": "014743023209",
  "message": "修复评论xss漏洞",
  "grade": "A",
  "module": "H",
  "tags": ["security", "xss"],
  "diffs": [
    {
      "file": "app/controllers/outterajax/OauserController.php",
      "raw_diff": "（原始diff文本）",
      "hunks": [
        {
          "function_name": "commentAction",
          "function_line": 640,
          "changed_lines": [653],
          "before_function": "（修复前该函数完整代码）",
          "after_function": "（修复后该函数完整代码）"
        }
      ]
    }
  ]
}
```

内部实现：`bugfix/context.rs` 里 `use crate::extract::php;` 复用函数定位能力。

### 4.4 `bcc bugfix generate`

```
bcc bugfix generate bugfix_contexts/ \
    --output bugfix_scenarios/ \
    --prompt-template prompts/bugfix_generate.txt
```

功能：调用本地 `codex exec`，将 context 包转为 bddc DSL 场景。

核心流程：
1. 扫描 `bugfix_contexts/` 下所有 JSON
2. 对每个 context JSON，读取 prompt 模板，拼接 context 内容
3. `std::process::Command::new("codex").args(["exec", "--full-auto", "-o", &out_path])` + stdin pipe prompt
4. 从输出中提取 DSL 块
5. 写入 `bugfix_scenarios/{module}/{hash}.dsl`

断点续传：已存在 `{hash}.dsl` 的跳过，`--force` 强制重做。

**prompt 模板**（默认从 `bcc/prompts/bugfix_generate.txt` 加载，`--prompt-template` 可覆盖）：
```
你是BDD测试专家。请将以下PHP bugfix记录转为bddc DSL场景。

## bddc DSL 语法
[SCENARIO: BDD-{MODULE}-BUGFIX-{HASH}] TITLE: {标题} TAGS: regression {tags}
GIVEN {前置条件指令} {参数}
WHEN {触发操作指令} {参数}
THEN {断言指令} {参数}

## 规则
1. diff中被替换的旧代码 = bug的根因，新代码 = 正确行为
2. GIVEN 描述触发bug的数据条件
3. WHEN 描述用户/系统操作
4. THEN 描述修复后的期望行为
5. 每个场景头部加注释：# Source: {hash} # Bug: {一句话根因}

## Bugfix记录
{context_json}
```

降级：若 `codex` 不可用，输出 prompt 文件供手动处理。

### 4.5 `bcc bugfix organize`

```
bcc bugfix organize bugfix_scenarios/ \
    --output bdd/features/ \
    --coverage-report bdd/coverage.md
```

功能：
1. 扫描所有 `.dsl` 文件
2. 按模块归类到目录
3. 同一 Controller 的场景合并到一个文件
4. 标记疑似重复（同函数 + 同标签），不自动删除，由人工决定
5. 生成覆盖率报告

覆盖率报告（`coverage.md`）：
```markdown
# BDD Bugfix 场景覆盖率

| 模块 | 控制器数 | 有场景覆盖 | 场景总数 | 覆盖率 |
|------|---------|-----------|---------|--------|
| A.审批系统 | 15 | 8 | 23 | 53% |
| B.活动服务 | 32 | 18 | 67 | 56% |
```

### 4.6 `bcc bugfix pipeline`

```
bcc bugfix pipeline /path/to/repo \
    --output bdd/ \
    --module-map module_map.json \
    --grade A,B
```

一键串联 collect → context → generate → organize，等价于依次执行四个子命令。

## 五、Cargo.toml 变更

```toml
[dependencies]
clap = { version = "4", features = ["derive"] }
serde = { version = "1", features = ["derive"] }
serde_yaml = "0.9"
serde_json = "1"
tree-sitter = "0.24"
tree-sitter-elixir = "0.3"
tree-sitter-typescript = "0.23"
tree-sitter-php = "0.24"              # 新增：PHP 解析
```

不引入 `reqwest`/`ureq`——AI 调用通过 `std::process::Command` 调本地 `codex exec`。

## 六、源码目录结构

```
compiler/bcc/src/
├── main.rs              # CLI 入口，新增 Bugfix 子命令
├── spec.rs
├── compile/
│   ├── mod.rs
│   ├── ast.rs
│   ├── emit.rs
│   └── passes.rs
├── extract/
│   ├── mod.rs
│   ├── elixir.rs
│   ├── typescript.rs
│   └── php.rs           # 新增：PHP tree-sitter 解析
├── trace.rs
├── bugfix/              # 新增：整个模块
│   ├── mod.rs           # bugfix 子命令分发
│   ├── collect.rs       # git log 扫描、分级、标签
│   ├── context.rs       # diff 提取 + 函数上下文（复用 extract::php）
│   ├── generate.rs      # codex exec 调用、prompt 构造、DSL 提取
│   ├── organize.rs      # 归类、疑似去重标记、覆盖率报告
│   └── pipeline.rs      # 一键串联
└── prompts/
    └── bugfix_generate.txt  # prompt 模板（外置，可迭代无需重编译）
```

## 七、CLI 接口设计（main.rs 扩展）

```rust
#[derive(Subcommand)]
enum Commands {
    Compile { ... },       // 现有
    Extract { ... },       // 现有（扩展 PHP）
    Trace { ... },         // 现有
    /// Mine BDD scenarios from git bugfix history
    Bugfix {
        #[command(subcommand)]
        action: BugfixAction,
    },
}

#[derive(Subcommand)]
enum BugfixAction {
    /// Scan git history for bugfix commits, classify and tag
    Collect {
        /// Git repo path
        repo: String,
        #[arg(short, long)]
        output: String,
        #[arg(long, default_value = "修复,fix,bug")]
        keywords: String,
        #[arg(long)]
        module_map: Option<String>,
    },
    /// Extract diff + function context for each bugfix
    Context {
        /// bugfix_inventory.json path
        inventory: String,
        #[arg(long)]
        repo: String,
        #[arg(short, long)]
        output: String,
        #[arg(long, default_value = "A,B,C")]
        grade: String,
        #[arg(long)]
        limit: Option<usize>,
        #[arg(long)]
        force: bool,
    },
    /// Generate bddc DSL scenarios from contexts via local codex
    Generate {
        /// bugfix_contexts/ directory
        contexts_dir: String,
        #[arg(short, long)]
        output: String,
        #[arg(long)]
        prompt_template: Option<String>,
        #[arg(long)]
        force: bool,
    },
    /// Organize scenarios into module directories, mark duplicates
    Organize {
        /// bugfix_scenarios/ directory
        scenarios_dir: String,
        #[arg(short, long)]
        output: String,
        #[arg(long)]
        coverage_report: Option<String>,
    },
    /// Run full pipeline: collect → context → generate → organize
    Pipeline {
        /// Git repo path
        repo: String,
        #[arg(short, long)]
        output: String,
        #[arg(long)]
        module_map: Option<String>,
        #[arg(long, default_value = "A,B")]
        grade: String,
        #[arg(long)]
        prompt_template: Option<String>,
    },
}
```

## 八、执行计划

### Phase 1：基础能力（P1 + P2 并行）
1. `extract/php.rs` — PHP tree-sitter 解析，输出 FileRecord
2. `bugfix/collect.rs` — git log 扫描分级
3. 跑全量 collect，验证清单质量

### Phase 2：上下文提取（P3）
4. `bugfix/context.rs` — diff + 函数上下文
5. 对 OaactiveController 42 个 bugfix 跑 context，验证输出

### Phase 3：生成 + 整理（P4 + P5）
6. `bugfix/generate.rs` — codex exec 生成 bddc DSL
7. `bugfix/organize.rs` — 归类 + 疑似重复标记
8. 小批量验证：42 个 → bddc compile → ExUnit

### Phase 4：全量 + 门禁
9. `bugfix/pipeline.rs` — 一键串联
10. 全量 A 级（~1500），B 级（~600）
11. bddc lint + check 门禁接入

### 5 人分工

| 人员 | 负责 | 依赖 | 可立即开始 |
|------|------|------|-----------|
| P1 | `extract/php.rs` + fixtures + BDD 场景 | 无 | 是 |
| P2 | `bugfix/mod.rs` + `collect.rs` + BDD 场景 | 无 | 是 |
| P3 | `bugfix/context.rs` + BDD 场景 | P1（可 mock 先启动） | 是 |
| P4 | `bugfix/generate.rs` + prompt 模板 + BDD 场景 | context JSON schema | 是 |
| P5 | `bugfix/organize.rs` + `pipeline.rs` + 集成测试 | P2/P3/P4（可 mock） | 是 |

关键路径：P1 → P3 → P4 → P5

## 九、示例：端到端演示

```bash
# 1. 扫描全量 bugfix
bcc bugfix collect /home/wangbo/document/upfit \
    -o /tmp/bdd/inventory.json \
    --module-map scripts/module_map.json

# 2. 提取 A 级上下文
bcc bugfix context /tmp/bdd/inventory.json \
    --repo /home/wangbo/document/upfit \
    -o /tmp/bdd/contexts/ \
    --grade A

# 3. 用 codex 生成 DSL
bcc bugfix generate /tmp/bdd/contexts/ \
    -o /tmp/bdd/scenarios/

# 4. 整理输出
bcc bugfix organize /tmp/bdd/scenarios/ \
    -o bdd/features/ \
    --coverage-report bdd/coverage.md

# 5. 或一键搞定
bcc bugfix pipeline /home/wangbo/document/upfit \
    -o bdd/ \
    --module-map scripts/module_map.json \
    --grade A,B

# 6. 用 bddc 编译成 ExUnit 测试
bddc compile --in bdd/features/ --out test/bdd_generated/
bddc lint --in bdd/features/
```

## 十、示例：从 diff 到 bddc DSL

### 示例 1：时区 Bug（A 级）

**Commit**: `a2c4538418` — 修复获取跑步步数

**Diff**:
```diff
- and `inserted_at` between ? and ?
+ and DATE_ADD(`inserted_at`, INTERVAL 8 HOUR) between ? and ?
```

**生成的 bddc DSL**:
```
# Source: a2c4538418
# Bug: inserted_at 存储 UTC，查询用北京时间导致跨日统计错误
[SCENARIO: BDD-E-BUGFIX-a2c453] TITLE: 查询今日跑步里程应正确处理UTC时区偏移 TAGS: regression datetime sports
GIVEN given_running_record user_id=$user_id activity_id=$activity_id inserted_at="2025-11-30T16:30:00Z" distance=3.5
WHEN query_today_distance user_id=$user_id activity_id=$activity_id query_time="2025-12-01T10:00:00+08:00"
THEN assert_distance expected=3.5
```

### 示例 2：安全漏洞（A 级）

**Commit**: `014743023209` — 修复评论 xss 漏洞

**Diff**:
```diff
- $rtn["content"] = $content;
+ $rtn["content"] = !empty($content) ? htmlspecialchars($content, ENT_QUOTES, 'UTF-8') : $content;
```

**生成的 bddc DSL**:
```
# Source: 014743023209
# Bug: 圈子评论内容未做 HTML 转义，存在存储型 XSS
[SCENARIO: BDD-H-BUGFIX-014743] TITLE: 发表圈子评论应对内容进行XSS防护 TAGS: regression security xss
GIVEN given_circle_post post_id=$post_id
WHEN submit_comment post_id=$post_id content="<script>alert('xss')</script>"
THEN assert_comment_escaped content_not_contains="<script>"
```

### 示例 3：事务遗漏（B 级）

**Commit**: `387031738f` — 修复盘点驳回

**Diff**:
```diff
- return;
+ DB::connection()->commit();
+ return $this->successWithMessage('操作成功');
```

**生成的 bddc DSL**:
```
# Source: 387031738f
# Bug: 盘点审批驳回未提交事务，数据库状态未持久化
[SCENARIO: BDD-H-BUGFIX-387031] TITLE: 盘点审批驳回应正确提交事务 TAGS: regression transaction asset
GIVEN given_inventory_apply apply_id=$apply_id status="pending"
WHEN reject_inventory_apply apply_id=$apply_id
THEN assert_apply_status apply_id=$apply_id expected_status="no_pass"
THEN assert_response status="success" message="操作成功"
```

## 十一、预期产出

| 指标 | 估算值 |
|------|--------|
| 有效 BDD 场景数 | 800-1,200 |
| 覆盖控制器数 | ~150（Top bug 控制器全覆盖） |
| 安全相关场景 | 30-50 |
| 模块覆盖 | 23 个模块全覆盖 |
| bddc 可编译率 | A 级 ~80%，B 级 ~50%（需人工调整指令集） |
