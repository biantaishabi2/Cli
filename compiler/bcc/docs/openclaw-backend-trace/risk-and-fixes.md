# OpenClaw 后端风险清单与修复建议

## 说明
- 范围：基于本轮后端反推代码调研结果（`src/agents`、`src/gateway`、`src/infra`、`src/config`、`src/plugins`、`src/cron` 等）。
- 口径：按 `P0/P1/P2/P3` 给出风险优先级，包含问题、影响、触发条件、证据、修复建议、验收标准。
- 备注：以下为工程治理优先级建议，不代表当前线上必然已发生事故。

## 高风险（P0/P1）

### 1) 配置解析失败后回退空配置（Fail-open）
- 优先级：`P0`
- 问题：配置校验失败（`INVALID_CONFIG`）或读取异常时，直接返回 `{}`，系统继续以默认配置运行。
- 影响：关键安全/隔离参数可能被静默降级到默认值，出现“看似启动成功但实际策略未生效”。
- 触发条件：配置文件格式错误、字段非法、I/O 异常。
- 证据：`src/config/io.ts:328`，`src/config/io.ts:331`，`src/config/io.ts:332`
- 修复建议：
  - 区分“生产模式”和“开发模式”：生产模式下配置非法应 `hard-fail`（启动失败）。
  - 增加 `--allow-default-on-invalid-config` 显式开关，仅测试/本地可用。
  - 将失败原因写入结构化健康状态，避免只在日志中可见。
- 验收标准：
  - 非法配置在生产模式下返回非 0 退出码。
  - 启动健康检查接口可读到 `config_invalid` 状态与错误详情。

### 2) 插件安全扫描不阻断 + 远程包安装 + 动态执行链
- 优先级：`P1`
- 问题：插件扫描为告警模式（不阻断安装），并支持从 npm 拉包，运行时通过动态加载执行插件代码。
- 影响：供应链风险扩大；恶意或被投毒插件可进入运行面。
- 触发条件：安装第三方插件（尤其是远程 spec）且扫描未命中/误判。
- 证据：
  - `src/plugins/install.ts:197`（warn-only）
  - `src/plugins/install.ts:215`（扫描失败继续安装）
  - `src/plugins/install.ts:456`（npm spec 安装入口）
  - `src/plugins/loader.ts:299`（动态加载）
- 修复建议：
  - 增加 `security.enforcePluginScan=true` 时的阻断模式。
  - 对 npm 安装引入 allowlist（组织/包名前缀）与版本锁定策略。
  - 引入插件签名或校验摘要（至少记录并强校验 `sha256`）。
- 验收标准：
  - 扫描 `critical` 或扫描失败在阻断模式下必须安装失败。
  - 非 allowlist 的 npm spec 安装被拒绝并有明确错误码。

### 3) 会话存储写入存在静默跳过路径（潜在数据丢失）
- 优先级：`P1`
- 问题：部分 `ENOENT` 分支直接 `return`，导致调用方感知为“成功返回”但没有落盘。
- 影响：会话元数据可能丢失，表现为会话断档、上下文历史不连续。
- 触发条件：目录并发删除/重建、临时文件竞争、Windows 文件系统边界场景。
- 证据：`src/config/sessions/store.ts:511`，`src/config/sessions/store.ts:531`，`src/config/sessions/store.ts:544`
- 修复建议：
  - 将静默 `return` 改为明确错误上抛（带错误码，如 `SESSION_STORE_WRITE_DROPPED`）。
  - 在调用链增加有限重试（指数退避）和最终失败告警。
  - 增加写后校验（`stat + mtime`）确保真实落盘。
- 验收标准：
  - 人为制造 `ENOENT` 场景时，调用方可观测到失败事件和错误码。
  - 重试后落盘成功率与失败可观测性达到预期阈值。

### 4) 会话维护默认 `warn`，不执行裁剪/封顶
- 优先级：`P1`
- 问题：会话维护模式默认 `warn`，只告警不执行清理；长期运行下会话文件可能持续膨胀。
- 影响：磁盘占用增长、读写性能下降、恢复与迁移成本增加。
- 触发条件：高频对话、长期运行未手工维护。
- 证据：`src/config/sessions/store.ts:211`，`src/config/sessions/store.ts:266`，`src/config/sessions/store.ts:466`
- 修复建议：
  - 将默认模式调整为 `enforce`（至少在服务端部署模板中如此）。
  - 提供按环境的默认策略：开发 `warn`、生产 `enforce`。
  - 暴露会话存储大小与条目数监控指标。
- 验收标准：
  - 生产默认配置下会定期执行 prune/cap。
  - 会话文件大小增长在阈值内可控。

## 中风险（P2）

### 5) Hook 执行默认吞错（错误记录后继续）
- 优先级：`P2`
- 问题：`catchErrors` 默认开启，插件 Hook 失败通常只记录日志，不中断主流程。
- 影响：功能“部分失效但流程继续”，容易形成隐蔽错误。
- 触发条件：插件 Hook 抛异常。
- 证据：`src/plugins/hooks.ts:95`，`src/plugins/hooks.ts:118`，`src/plugins/hooks.ts:163`
- 修复建议：
  - 对关键 Hook（鉴权、策略注入）提供 fail-closed 选项。
  - 增加 hook 级别策略：`ignore/log/fail`。
- 验收标准：
  - 关键 Hook 失败时可配置为阻断请求并返回可识别错误码。

### 6) 同步 Hook 误返回 Promise 时结果被忽略
- 优先级：`P2`
- 问题：同步 Hook 若误写成异步，当前仅警告并忽略结果。
- 影响：插件作者误用时产生“代码执行了但结果不生效”的行为偏差。
- 触发条件：`tool_result_persist` Hook 返回 Promise。
- 证据：`src/plugins/hooks.ts:344`，`src/plugins/hooks.ts:350`，`src/plugins/hooks.ts:352`
- 修复建议：
  - 在注册阶段做签名校验，禁止 sync hook 注册 async handler。
  - 在严格模式下将该类错误升级为失败。
- 验收标准：
  - CI 或启动期即可拦截不合法 Hook 定义。

### 7) 会话锁与清理流程多处 best-effort，异常可见性不足
- 优先级：`P2`
- 问题：锁文件写入/清理、备份清理等路径存在多处 `catch(() => undefined)` 或空 catch。
- 影响：并发异常时可恢复性依赖运气，排障成本高。
- 触发条件：高并发写、崩溃恢复、临时 I/O 故障。
- 证据：
  - `src/config/sessions/store.ts:433`
  - `src/config/sessions/store.ts:608`
  - `src/config/sessions/store.ts:644`
  - `src/config/sessions/store.ts:655`
- 修复建议：
  - 保留 best-effort 行为，但必须计入结构化告警指标（计数+采样日志）。
  - 将锁超时错误纳入统一错误分类并上报。
- 验收标准：
  - 压测下可观察到锁冲突/清理失败指标，而非仅静默处理。

## 低风险（P3）

### 8) 配置文件备份与回收失败多为静默处理
- 优先级：`P3`
- 问题：配置写入过程中的备份/临时文件清理多为 best-effort，失败时可观测性偏弱。
- 影响：遗留临时文件、备份不完整，影响长期运维整洁性。
- 触发条件：Windows/跨盘/权限边界场景。
- 证据：`src/config/io.ts:526`，`src/config/io.ts:538`，`src/config/io.ts:541`
- 修复建议：
  - 对清理失败增加低频告警（防日志风暴）。
  - 提供 `openclaw doctor` 的“残留文件清理”子项。
- 验收标准：
  - 人工构造失败场景时，能看到告警并可通过工具清理残留。

### 9) 插件发现面覆盖 workspace/global，需强化默认最小化原则
- 优先级：`P3`
- 问题：插件发现默认会扫描工作区和全局目录，部署边界不清晰时可能加载到非预期插件。
- 影响：运行行为不确定性增加，尤其在共享主机或多项目环境。
- 触发条件：目录中存在同名/遗留插件。
- 证据：`src/plugins/discovery.ts:328`，`src/plugins/discovery.ts:343`，`src/plugins/discovery.ts:352`
- 修复建议：
  - 增加默认“显式声明来源”模式（仅加载配置中声明的路径）。
  - 启动时打印最终加载源清单并支持 `--strict-plugin-origin`。
- 验收标准：
  - 严格模式下未声明来源不会被扫描/加载。

### 10) 文档语义对齐口径与覆盖统计混淆
- 优先级：`P3`
- 问题：后端反推已完成 832 文档，但在“关键导出/函数”语义层仍有 35 处差异；若直接用于生成器驱动，可能误导“可自动用证据”决策。
- 影响：生成脚本或复用方按“已对齐文档”做自动化校验时，会出现误报和漏检。
- 触发条件：以文档条目作为直接接口契约输入做自动化采集。
- 证据：
  - `docs/backend-trace/artifacts/symbol-alignment-mismatch.tsv`
  - `docs/backend-trace/symbol-alignment-gap.md`
- 修复建议：
  - 首先区分两类差异：代码真实不一致 vs `export` 语法抽取噪音。
  - 将语义清单中的“需要修订”与“抽取器误报”分离，按优先级处理。
  - 复跑语义对齐脚本并写入复检闭环记录。
- 验收标准：
  - 语义清单复盘后，明确标注 0 条“阻断级”文档失真（或给出待修订项清单）。

## 建议落地顺序
1. P0/P1：先处理配置 fail-open、插件供应链阻断、会话写入静默丢失。
2. P2：补强 hook 与锁流程的可观测性和失败策略。
3. P3：做运维体验与默认最小权限收敛。

## 关联文档
- `docs/backend-trace/manual-tracking.md`
- `docs/backend-trace/coverage-matrix.md`
- `docs/backend-trace/coverage-supplement.md`
- `docs/backend-trace/artifacts/prd-coverage-judgement.tsv`
