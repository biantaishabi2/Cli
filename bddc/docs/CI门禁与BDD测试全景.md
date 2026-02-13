# CI 门禁与 BDD 测试全景

> 最后更新: 2026-02-12

## 概述

BDD 测试是两个项目（Shop / ZCPG）的 **唯一 CI 测试入口**。老的 `mix test` 不纳入 CI，所有进入 CI 的测试都必须走 BDD 体系。

## 测试规模

| 项目 | BDD 文件数 | 场景数 | 类型 |
|------|-----------|--------|------|
| Shop | 33 DSL | 278 | DSL 编译生成 ExUnit |
| ZCPG | 32 手写 | 159 | 手写 `*_bdd_test.exs` |
| **合计** | **65** | **437** | |

## Shop 门禁链路

入口: `./scripts/pre_deploy_check.sh`

```
step1: bdd_gate.sh --strict-factories
       ├── bddc check (编译 + lint + 运行时覆盖校验；在 bdd_gate.sh 内默认带 --skip-annotations-check/--skip-bdd-test)
       └── bddc factories.check (数据工厂门禁)
step2: MIX_ENV=test mix compile
step3: MIX_ENV=test mix test test/bdd_generated/ --include integration
step4: 可选 extra test
```

### 参数

| 参数 | 说明 |
|------|------|
| `--fast` | 非严格模式（跳过 strict-factories） |
| `--bdd-only` | 仅执行静态门禁（跳过 compile 和测试） |
| `--skip-bdd` | 跳过静态门禁（调试用） |
| `--skip-compile` | 跳过编译检查（调试用） |
| `--skip-bdd-test` | 跳过 BDD 集成测试（调试用） |
| `--extra-test CMD` | 额外测试命令 |

### Shop BDD DSL 分布

| 域 | DSL 文件数 | 场景数 |
|----|-----------|--------|
| 结算 (settlement) | 10 | ~80 |
| 库存 (inventory) | 3 | 44 |
| 定价 (pricing) | 1 | 16 |
| 赊销/退款 (credit/refund) | 4 | ~20 |
| 对账 (reconciliation) | 3 | ~25 |
| 财务报表 (financial report) | 2 | ~18 |
| 售后 (aftersales) | 3 | ~13 |
| 投诉 (complaint) | 1 | 7 |
| 数据工厂 | 2 | 17 |
| 代记账评估 (agent eval) | 2 | 4 |
| HTTP 冒烟 | 1 | 1 |

## ZCPG 门禁链路

入口: `./scripts/pre_deploy_check.sh`

```
step1: MIX_ENV=test mix compile
step2: find test -name "*_bdd_test.exs" | mix test
step3: 可选 extra test
```

### 参数

| 参数 | 说明 |
|------|------|
| `--skip-compile` | 跳过编译检查 |
| `--skip-bdd-test` | 跳过 BDD 集成测试 |
| `--extra-test CMD` | 额外测试命令 |

### ZCPG BDD 测试分布

| 域 | 文件数 | 说明 |
|----|--------|------|
| accounting (代记账核心) | 12 | 凭证生成、待处理项、工单同步、勾稽、流水线等 |
| workers (后台任务) | 10 | 银行同步、凭证生成、合同到期、SLA巡检等 |
| integrations (外部集成) | 4 | 银行、钉钉、Win17、发票同步 |
| legacy_coin_billing | 3 | 对账快照、客户映射、巡检通知 |
| mix_tasks (CLI) | 2 | 银行连续性检查、凭证代理 |
| controllers (API) | 1 | 待处理项 API |

## 新增 BDD 测试流程

### Shop (DSL 模式)

1. 写 DSL: `docs/bdd/xxx.dsl`
2. 补注册: `bddc registry.upsert ...`
3. 补运行时: `test/support/bdd/xxx_runtime.ex` + `instructions_v1.ex` 分发
4. 编译: `bddc compile ...`
5. 跑测试: `mix test test/bdd_generated/xxx_generated_test.exs --include integration`
6. 自动进 CI（`pre_deploy_check.sh` 会扫 `test/bdd_generated/` 全目录）

### ZCPG (手写模式)

1. 写测试: `test/zcpg/xxx/*_bdd_test.exs`
2. 跑测试: `mix test test/zcpg/xxx/*_bdd_test.exs`
3. 自动进 CI（`pre_deploy_check.sh` 会 `find` 所有 `*_bdd_test.exs`）

## 配置存储说明

### 企业定价配置 (Shop)

定价配置存储在 `enterprises.extra_info` JSON 字段中，不单独建表/字段：

```json
{
  "price_tier": "custom_rate",
  "custom_rate": "0.88",
  "pricing_script_id": "uuid-xxx"
}
```

支持四级定价: A档 / B档 / 自定义系数 / Lua脚本。
