# BDD 场景提取方案 v2 — 模块驱动全量扫描

> 基于 v1 验证结果修订。v1 关键词过滤仅覆盖 11% 后端 commit，v2 改为按文件路径全量扫描。

## 一、v1 的问题

v1 用 `git log --grep="修复,fix,bug"` + `--all` 扫描，两个缺陷：

1. **关键词只覆盖 11%**：主分支 13,775 个 controller commit 中，message 含"修复/fix/bug"的只有 1,518 个。剩下 89% 的 message 是"修改xxx"、ticket号、客户名——其中大量是真正的 bugfix。

2. **`--all` 扫全部 5,126 个分支**：实际只需看主分支 `fit365/2.0.0`。bugfix 分支 54% 已合并到主分支，剩余是客户长期分叉，不是简单 bugfix。

## 二、数据全貌

```
仓库:  FIT365 PHP (Phalcon), 2020-01 ~ 2026-02, 42 位作者
总量:  77,777 commits (5,126 个分支)
主分支: fit365/2.0.0, 33,681 commits (其中 24,563 个非 merge)
```

### 主分支后端文件变更分布

| 文件类型 | 涉及 commit 数 | 占比 | 提取 |
|---------|--------------|------|------|
| controller | 13,775 | 56% | 是 |
| model | 2,659 | 10% | 是 |
| plugin | 992 | 4% | 是 |
| component | 773 | 3% | 是 |
| trait | 465 | 1% | 是 |
| listener | 196 | 0.8% | 是 |
| view (.phtml) | 11,116 | 45% | 否 — 前端模板 |
| vendor | 65 | 0.2% | 否 — 第三方库 |

提取范围合计：**~16,200 个** 后端变更 commit。

### 控制器目录分布

| 目录 | commit 数 | 文件数 |
|------|----------|--------|
| root | 9,554 | 255 |
| outterajax | 2,107 | 45 |
| service | 1,594 | 66 |
| sysadmin | 1,041 | 38 |
| openadmin | 1,013 | 57 |
| innerajax | 792 | 24 |

### 修改最频繁的控制器 (Top 15)

| 控制器 | commit 数 | 控制器 | commit 数 |
|--------|----------|--------|----------|
| Asyncdownload | 526 | Oascorerank4gcc | 278 |
| Oauser | 425 | Jdvop | 257 |
| Year2022 | 372 | Fesco | 242 |
| Oaactive | 351 | Demos | 239 |
| Hhj | 350 | Firmoperate | 238 |
| Ctg | 326 | Firmform | 237 |
| Wcuser | 304 | Cofco | 234 |
| Quartz | 286 | | |

## 三、v2 改动（两处）

### 改动 1: collect 支持按文件路径扫描

现有 collect 只有 `git log --grep` 模式。新增 `--path` 参数：

```
git log <branch> --no-merges --numstat -- <path>
```

传了 `--path` 就按文件路径扫描，得到该路径下的**全部** commit；不传则保持 v1 关键词模式。

### 改动 2: `--all` 改为 `--branch`

新增 `--branch` 参数（默认当前分支），替换 collect 中的 `--all`。

### 不需要单独的 classify 步骤

非 bugfix 的 commit 在 generate 步骤自然过滤——prompt 里加一句 "如果不是 bugfix 则输出 SKIP"，AI 看 diff 内容判定。不用多调一次 AI。

## 四、改动后的命令行

```diff
  bcc bugfix <REPO> [OPTIONS]

  OPTIONS:
+   -b, --branch <REF>          目标分支 [默认: 当前分支]
+       --path <PATH>           按文件路径扫描（传了则不按关键字匹配）
    -o, --output <DIR>          输出目录
    -s, --step <STEP>           collect(c) / context(x) / generate(g) / organize(o)
        --grade <GRADES>        [默认: A,B]
        --keywords <KW>         [默认: "修复,fix,bug"] (仅无 --path 时生效)
        --module-map <FILE>     模块映射 JSON（可选，用于模块名解析）
        --prompt-template <FILE>
        --limit <N>
        --force
```

四步流水线不变：collect → context → generate → organize。

### 用法

```bash
# v1 模式（向下兼容，关键词扫描）
bcc bugfix . -o output/ --keywords "修复,fix,bug,错误,报错"

# v2 模式（文件路径扫描）
bcc bugfix . -o output/ \
  --path app/controllers/ \
  --branch origin/fit365/2.0.0

# v2 + module_map（可选，用于模块名解析）
bcc bugfix . -o output/ \
  --path app/controllers/ \
  --branch origin/fit365/2.0.0 \
  --module-map module_map.json

# 分步 + 限量
bcc bugfix . -o output/ --path app/controllers/ -s c         # 只扫描
bcc bugfix . -o output/ -s g --limit 100                      # 生成前100个
bcc bugfix . -o output/ -s g                                  # 继续跑剩余的
```

## 五、generate 输出：测试规格书 JSON

generate 调用 LLM 将 diff 上下文翻译为**结构化的业务语义描述**（测试规格书），而不是直接生成 DSL。
这是因为 bcc 站在 PHP 侧，只能说清"要测什么"（业务层），"怎么测"（实现层）由下游 bddc 在 Elixir 项目中决定。

输出格式：`specs/<hash>.json`

```json
{
  "source_commit": "hash",
  "bug_summary": "一句话 bug 根因",
  "module": "从文件路径推导的模块名",
  "action": "被修改的函数/方法名",
  "fix_summary": "修复做了什么",
  "test_type": "regression|boundary|null_safety|security|concurrency",
  "test_spec": {
    "preconditions": [{"what": "条件", "involves": "db|redis|...", "key_params": ["参数"]}],
    "trigger": {"type": "controller_action", "target": "类名", "method": "方法名"},
    "assertions": [{"what": "断言", "type": "return_value|db_state|...", "expected": "值"}]
  },
  "wrong_behavior": "修复前的错误行为",
  "correct_behavior": "修复后的正确行为"
}
```

非 bugfix 的 commit 输出 `{"skip": true, "reason": "feature|refactor|config"}`，由 organize 步骤过滤。

## 六、module_map.json

module_map 是可选的，用于将文件路径映射到业务模块名。不传时用 `module_from_filename()` 自动推导（去后缀 + CamelCase→snake_case）。

```json
{
  "mapping": {
    "outterajax/Oaactive": "B",
    "outterajax/Oauser": "H",
    "ShopController": "C"
  },
  "module_names": {
    "A": "审批系统",
    "B": "活动服务",
    "C": "商城系统"
  }
}
```

## 七、执行策略

按模块分批跑，优先高频模块：

| 优先级 | 模块 | 估算 commit 数 |
|--------|------|---------------|
| P0 | H.企业后台 | ~4,000 |
| P1 | B.活动/C.商城/J.定制 | ~1,000 各 |
| P2 | E.运动/K.异步/A.审批 | ~600-800 各 |
| P3 | 其余 16 个模块 | ~6,000 |

每个模块：scan → context → generate → organize → bddc autochain（指令设计 + DSL + 编译 + 测试）。

## 八、预期产出

| 指标 | v1 | v2 (实测) |
|------|-----|-----|
| 扫描 commit 数 | ~2,500 | 13,775 |
| 分级 A/B/C | — | 8,334 / 3,424 / 2,017 |
| 覆盖模块数 | ~150 | 400 |
| 测试规格书数(估) | 800-1,200 | 2,500-4,000 |
