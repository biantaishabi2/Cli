# BDD 编译器金标准验收（Golden Master）

> 更新时间：2026-02-09

这份文档的用途是：把“已经通过、真实可运行”的手写测试作为金标准（Golden Master），用于验收 BDD 编译器与指令集是否能表达并生成等价的可观测行为测试。

## 1. 验收方式（总原则）

- 金标准是“真实手写测试”。
- 编译器验收是：用 DSL 表达同一场景，编译生成的测试在同一环境下通过，且断言点只覆盖可观测行为（不测内部实现细节）。
- 本文档只列出金标准清单与映射占位；DSL 场景实现放在 `docs/bdd/*.dsl`。

### 1.1 “BDD 标准”口径（用于筛选与统计）

由于仓库内存在大量“非 BDD 风格”的单测/契约测试/值对象测试，为了可操作性，本项目将 BDD 标准分成两层口径：

- BDD(strict)：`test "..."` 的标题同时包含 `Given` + `When` + `Then`（三段齐全）。
  - 用途：一键从手写测试里抽取“明确 BDD 场景”的金标准清单，便于先迁移这一批。
- BDD(semantic)：即使标题不含 Given/When/Then，但测试结构遵循 Given/When/Then（或 AAA：Arrange/Act/Assert），且断言点只覆盖可观测行为。
  - 用途：后续扩展抽取规则或人工补充，逐步做到“结算+售后全量纳入（604 条）”。

当前自动抽取/统计规则以 **BDD(strict)** 为准；全量用例清单见附录：`docs/bdd/BDD编译器金标准验收_结算售后全量清单.md`。

补充：为了把“更接近系统边界的可观测行为用例”作为下一阶段验收门槛，本仓库额外提供一个可复现的 `BDD(semantic)=133` 子集清单：`docs/bdd/semantic_v2_133_list.tsv`（由 `scripts/bdd_gen_semantic_133_list.py` 生成）。

## 2. 范围分层（先做 Tier0）

- Tier0：结算线路 Route B + 售后退款链路。要求 100% 覆盖并可作为当前阶段的验收门槛。
- Tier1/Tier2：扩展候选。先列出清单，后续按指令集成熟度逐步纳入。

## 3. 金标准用例清单（含 DSL 映射占位）

### Tier0: 结算线路 Route B + 售后退款链路（编译器黄金验收范围）

#### test/financial_settlement/integration/settlement_route_b_flow_test.exs（15）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 129 | Given 当期 Normal 很小但跨期扣除很大 When 出单并审核通过 Then 负净额可持久化且流程正常 | BDD-SETTLEMENT-ROUTE-B-FLOW-001 | done |
| 2 | 279 | Given 订单项发生时间在 as_of 边界 When 出单 Then <= as_of 纳入 Normal 且 > as_of 不纳入 | BDD-SETTLEMENT-ROUTE-B-FLOW-002 | done |
| 3 | 369 | Given 订单不满足结算口径（状态不在 paid/shipped/completed）When 出单 Then 不生成 Normal 行 | BDD-SETTLEMENT-ROUTE-B-FLOW-003 | done |
| 4 | 421 | Given 已结算订单发生退款 When 下期出单 Then 生成 Adjustment 行并推进 refund_deductions 状态 | BDD-SETTLEMENT-ROUTE-B-ADJ-001 | done |
| 5 | 591 | Given 账期起止颠倒 When 出单 Then 返回 :invalid_params 且不落库 | BDD-SETTLEMENT-NEG-001 | done |
| 6 | 611 | Given 同一订单项发生多次部分退款 When 下期出单 Then 多条 Adjustment 可追溯且总额可解释 | BDD-SETTLEMENT-ROUTE-B-FLOW-004 | done |
| 7 | 761 | Given dev 中存在非 UUID supplier_id 的订单 When 按 UUID 供应商出单 Then 该订单被跳过且结算成功 | BDD-SETTLEMENT-ROUTE-B-FLOW-005 | done |
| 8 | 817 | Given 订单 supplier_id 非 UUID 但商品 supplier_id 为 UUID When 按 UUID 出单 Then 仍跳过（策略：严格以订单 supplier_id 为准） | BDD-SETTLEMENT-ROUTE-B-FLOW-006 | done |
| 9 | 893 | Given 出单前售后退款已完成 When 出单 Then 当期同一期吸收（不出 Normal 也不出 Adjustment） | BDD-SETTLEMENT-ROUTE-B-FLOW-007 | done |
| 10 | 990 | Given 出单前部分退货（按订单项数量）When 出单 Then Normal 行按剩余数量入账且不出 Adjustment | BDD-SETTLEMENT-ROUTE-B-AFTERSALES-001 | done |
| 11 | 1113 | Given 出单前部分退 + 结算锁定后再次退款 When 跨两期出单 Then 当期按剩余数量入账且下期生成 Adjustment 并推进扣除状态 | BDD-SETTLEMENT-ROUTE-B-FLOW-008 | done |
| 12 | 1299 | Given pending 扣除缺 origin When 出单 Then 返回 :adjustment_missing_origin 且不落库 | BDD-SETTLEMENT-ROUTE-B-FLOW-009 | done |
| 13 | 1345 | Given 超额冲减 When 下期出单 Then 返回 :deduction_exceeds_origin 且不落库 | BDD-SETTLEMENT-ROUTE-B-FLOW-010 | done |
| 14 | 1449 | Given 对账 diff != 0 When 执行结算 Then 阻断并返回 :reconciliation_diff_not_zero | BDD-SETTLEMENT-ROUTE-B-FLOW-011 | done |
| 15 | 1500 | Given Adjustment 转写临时失败 When 重试出单 Then 不留脏数据且最终成功 | BDD-SETTLEMENT-ROUTE-B-FLOW-012 | done |

#### test/financial_settlement/integration/settlement_route_b_recon_block_test.exs（1）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 73 | Given settlement_lines 与结算头净额不一致 When execute_settlement Then 返回 :reconciliation_diff_not_zero | BDD-SETTLEMENT-ROUTE-B-RECON-001 | done |

#### test/financial_settlement/integration/settlement_route_b_after_sales_e2e_test.exs（1）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 107 | Given 售后通过应用服务提交并完成退款 When 当期出单 Then Normal 行按剩余数量入账且不出 Adjustment | BDD-SETTLEMENT-ROUTE-B-AFTERSALES-001 | done |

#### test/financial_settlement/event_handlers/refund_deduction_handler_test.exs（4）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 23 | EH-001: Handler 启动成功 | BDD-FIN-REFUND-DEDUCTION-HANDLER-001 | done |
| 2 | 53 | EH-002: 收到非事件消息不崩溃 | BDD-FIN-REFUND-DEDUCTION-HANDLER-002 | done |
| 3 | 62 | EH-003: 收到非 RefundSuccess 事件忽略 | BDD-FIN-REFUND-DEDUCTION-HANDLER-003 | done |
| 4 | 70 | EH-004: Given 订单已结算 When 投递 RefundSuccess 事件 Then 创建扣除记录且 refund_id 幂等 | BDD-FIN-REFUND-DEDUCTION-HANDLER-004 | done |

#### test/aftersales/infrastructure/event_handlers/payment_refund_event_handler_test.exs（3）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 43 | Given 售后明细已落库 When refund_success Then 回写refund_info并标记明细completed | BDD-AFTERSALES-REFUND-001 | done |
| 2 | 139 | Given 事件直接携带 order_id When refund_success Then 仍能回写refund_info并标记明细completed | BDD-AFTERSALES-REFUND-002 | done |
| 3 | 201 | Given 同一订单存在多张可退款售后单 When refund_success Then 不回写（避免歧义） | BDD-AFTERSALES-REFUND-003 | done |

#### test/aftersales/infrastructure/event_handlers/refund_trigger_handler_test.exs（4）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 53 | 售后审批通过时自动触发退款 | BDD-AFTERSALES-REFUND-TRIGGER-001 | done |
| 2 | 113 | 自动审批时也触发退款 | BDD-AFTERSALES-REFUND-TRIGGER-002 | done |
| 3 | 166 | 无支付单的售后审批不触发退款 | BDD-AFTERSALES-REFUND-TRIGGER-003 | done |
| 4 | 208 | 拒绝的售后不触发退款 | BDD-AFTERSALES-REFUND-TRIGGER-004 | done |

> 小计：28 条

### Tier1: 售后端到端/用例级测试（扩展验收候选）

#### test/aftersales/integration/after_sales_flow_test.exs（6）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 49 | complete return flow from submission to refund |  | TODO |
| 2 | 153 | complete exchange flow |  | TODO |
| 3 | 240 | supplier rejects after-sales request |  | TODO |
| 4 | 294 | refund only flow without return |  | TODO |
| 5 | 363 | after-sales timeout auto processing |  | TODO |
| 6 | 423 | concurrent after-sales request handling |  | TODO |

#### test/aftersales/application/after_sales_processing_application_service_test.exs（11）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 84 | supplier approves after sales request |  | TODO |
| 2 | 104 | supplier rejects after sales request |  | TODO |
| 3 | 123 | updates logistics information |  | TODO |
| 4 | 142 | confirms receipt |  | TODO |
| 5 | 159 | processes refund |  | TODO |
| 6 | 180 | completes after sales processing |  | TODO |
| 7 | 194 | validates supplier authority on approval |  | TODO |
| 8 | 212 | handles non-existent order |  | TODO |
| 9 | 228 | validates refund amount |  | TODO |
| 10 | 247 | handles inspection failure |  | TODO |
| 11 | 265 | tracks logistics status updates |  | TODO |

#### test/aftersales/application/after_sales_application_service_test.exs（11）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 87 | submits after sales request |  | TODO |
| 2 | 112 | gets after sales order details |  | TODO |
| 3 | 132 | gets user after sales orders |  | TODO |
| 4 | 156 | gets supplier after sales orders |  | TODO |
| 5 | 180 | cancels after sales request |  | TODO |
| 6 | 197 | handles non-existent after sales order |  | TODO |
| 7 | 209 | validates submit command |  | TODO |
| 8 | 229 | normalizes submit errors to list for API stability |  | TODO |
| 9 | 248 | transforms domain object to DTO |  | TODO |
| 10 | 274 | handles pagination correctly |  | TODO |
| 11 | 294 | gets statistics for supplier |  | TODO |

> 小计：28 条

### Tier2: 结算通用集成测试（扩展验收候选）

#### test/financial_settlement/integration/settlement_flow_test.exs（17）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 245 | 成功创建供应商月度结算单 |  | TODO |
| 2 | 280 | 供应商不存在时创建失败 |  | TODO |
| 3 | 292 | 重复创建同期结算单失败 |  | TODO |
| 4 | 308 | 已作废 cancelled 的结算单允许重出同账期 |  | TODO |
| 5 | 331 | 审核通过即写入 locked_at/locked_by，且冻结后禁止删除 settlement_lines |  | TODO |
| 6 | 396 | Given 结算单已冻结 When 尝试修改金额/期间/as_of Then 保存返回 :settlement_locked（但允许推进状态） |  | TODO |
| 7 | 437 | 审核通过后回写 orders.settled_at/settlement_id（以 Normal 快照行的订单为准） |  | TODO |
| 8 | 501 | Given 结算单被拒绝 When 审核 reject Then 不回写订单已结算标记 |  | TODO |
| 9 | 587 | 成功审批结算单 |  | TODO |
| 10 | 615 | 拒绝结算单 |  | TODO |
| 11 | 643 | 已审批的结算单不能重复审批 |  | TODO |
| 12 | 693 | 成功执行支付 |  | TODO |
| 13 | 720 | 未审批的结算单不能支付 |  | TODO |
| 14 | 796 | 支付失败后可以重试 |  | TODO |
| 15 | 822 | 并发创建多个供应商的结算单 |  | TODO |
| 16 | 858 | 同一供应商同账期只允许创建一张结算单（DB 唯一约束） |  | TODO |
| 17 | 919 | 批量创建大量结算单的性能 |  | TODO |

#### test/financial_settlement/integration/reconciliation_flow_test.exs（11）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 83 | IT-FS-REC-001: 创建并执行日对账批次 - 完全匹配场景 |  | TODO |
| 2 | 110 | IT-FS-REC-002: 识别并处理对账差异 - 金额不匹配 |  | TODO |
| 3 | 131 | IT-FS-REC-003: 查询对账批次状态和进度 |  | TODO |
| 4 | 154 | IT-FS-REC-004: 分页查询对账批次列表 |  | TODO |
| 5 | 183 | IT-FS-REC-005: 生成并导出对账报告 |  | TODO |
| 6 | 201 | IT-FS-REC-006: 处理重复创建对账批次 |  | TODO |
| 7 | 231 | IT-FS-REC-007: 自动修复小额差异 |  | TODO |
| 8 | 277 | IT-FS-REC-008: 识别单边账交易 |  | TODO |
| 9 | 318 | IT-FS-REC-009: 对账流程发布领域事件 |  | TODO |
| 10 | 338 | IT-FS-REC-010: 并发创建对账批次 |  | TODO |
| 11 | 389 | IT-FS-REC-011: 大批量交易对账性能测试 |  | TODO |

#### test/financial_settlement/integration/financial_report_flow_test.exs（15）

| # | 行号 | 金标准用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - |
| 1 | 116 | IT-FS-RPT-001: 生成月度财务汇总报表 | BDD-GM-FS-INTEGRATION-0001 | done（deps=mock） |
| 2 | 145 | IT-FS-RPT-002: 生成供应商对账单报表 | BDD-GM-FS-INTEGRATION-0002 | done（deps=mock） |
| 3 | 168 | IT-FS-RPT-003: 生成渠道对账汇总报表 | BDD-GM-FS-INTEGRATION-0003 | done（deps=mock） |
| 4 | 193 | IT-FS-RPT-004: 审核通过并发布报表 | BDD-GM-FS-INTEGRATION-0004 | done（deps=mock） |
| 5 | 230 | IT-FS-RPT-005: 批量审核多个报表 | BDD-GM-FS-INTEGRATION-0005 | done（deps=mock） |
| 6 | 265 | IT-FS-RPT-006: 按类型查询报表列表 | BDD-GM-FS-INTEGRATION-0006 | done（deps=mock） |
| 7 | 296 | IT-FS-RPT-007: 按时间范围查询报表 | BDD-GM-FS-INTEGRATION-0007 | done（deps=mock） |
| 8 | 314 | IT-FS-RPT-008: 导出报表为PDF格式 | BDD-GM-FS-INTEGRATION-0008 | done（deps=mock） |
| 9 | 350 | IT-FS-RPT-009: 报表生成发布领域事件 | BDD-GM-FS-INTEGRATION-0009 | done（deps=mock） |
| 10 | 379 | IT-FS-RPT-010: 处理报表完整生命周期事件链 | BDD-GM-FS-INTEGRATION-0010 | done（deps=mock） |
| 11 | 428 | IT-FS-RPT-011: 生成跨多供应商的综合报表 | BDD-GM-FS-INTEGRATION-0011 | done（deps=mock） |
| 12 | 478 | IT-FS-RPT-012: 生成异常数据报表 | BDD-GM-FS-INTEGRATION-0012 | done（deps=mock） |
| 13 | 525 | IT-FS-RPT-013: 验证报表访问权限控制 | BDD-GM-FS-INTEGRATION-0013 | done（deps=mock） |
| 14 | 568 | IT-FS-RPT-014: 大数据量报表生成性能测试 | BDD-GM-FS-INTEGRATION-0014 | done（deps=mock） |
| 15 | 640 | IT-FS-RPT-015: 并发生成多个报表 | BDD-GM-FS-INTEGRATION-0015 | done（deps=mock） |

补充：为了验证“同一业务场景用真实边界驱动”的能力，FinancialReport 额外提供 `deps=real` 的金标准用例（生产 `FinancialReportApplicationService`）：

| # | 用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - |
| 1 | REAL-001 生产应用服务生成报表（pending_review） | BDD-GM-FS-INTEGRATION-REALRPT-0001 | done（deps=real） |
| 2 | REAL-002 生产应用服务审核通过报表 | BDD-GM-FS-INTEGRATION-REALRPT-0002 | done（deps=real） |
| 3 | REAL-003 生产应用服务发布报表 | BDD-GM-FS-INTEGRATION-REALRPT-0003 | done（deps=real） |

> 小计：43 条

## 4. 完整性检查（用于回答“是不是只有这么点？”）

下面是“相关目录下测试文件数量”的客观统计，避免漏掉：

| 目录范围 | 测试文件数 |
| - | - |
| `financial_settlement/integration` | 6 |
| `financial_settlement/services` | 5 |
| `aftersales/**` | 25 |

说明：本金标准清单目前聚焦“结算线路/售后退款链路”的可观测行为验收，因此优先列 Tier0/Tier1/Tier2。若你希望把所有 value_objects/domain/services 的单测也纳入编译器验收，需要先决定编译器的目标边界是否覆盖到这些层级（通常不建议）。

## 5. 全量纳入：已有“BDD 风格”手写测试清单（标题含 Given/When/Then）

为了满足“把所有结算+售后的 BDD 用例都拉进金标准”的要求，本节通过脚本从以下目录抽取：

- `test/financial_settlement/**`
- `test/aftersales/**`

抽取规则（当前版本）：仅纳入标题同时包含 `Given` + `When` + `Then` 的 `test "..."` 用例。

结果：

- BDD 风格手写用例总数：34
- 已在本文档 Tier0 映射并完成 DSL 迁移：21（`done`）
- 仍未纳入映射/迁移：13（`TODO`）

> 说明：
> - 这里的“全量纳入”是“全量 BDD 风格手写测试”（不是全量 604 条结算+售后测试）。
> - 若后续仓库里存在 BDD 风格但标题不含 Given/When/Then（例如用例体内使用 Given/When/Then 注释或 helper），需要升级抽取规则或人工补充。
>
> 全量（604 条）用例清单请看附录：`docs/bdd/BDD编译器金标准验收_结算售后全量清单.md`。

| # | 文件 | 行号 | 用例标题 | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - | - |
| 1 | `test/aftersales/domain/services/after_sales_service_test.exs` | 98 | Given items 缺少必填字段 When create_after_sales_request Then 返回校验错误列表 |  | TODO |
| 2 | `test/aftersales/domain/services/after_sales_service_test.exs` | 132 | Given 同一请求内 order_item_id 重复 When create_after_sales_request Then 返回重复错误 |  | TODO |
| 3 | `test/aftersales/domain/services/after_sales_service_test.exs` | 171 | Given type=return 且 items 为空 When create_after_sales_request Then 返回 items 不能为空 |  | TODO |
| 4 | `test/aftersales/domain/services/after_sales_service_test.exs` | 191 | Given type=exchange 且未提供 items When create_after_sales_request Then 仍可创建（兼容） |  | TODO |
| 5 | `test/aftersales/infrastructure/event_handlers/payment_refund_event_handler_test.exs` | 43 | Given 售后明细已落库 When refund_success Then 回写refund_info并标记明细completed | BDD-AFTERSALES-REFUND-001 | done |
| 6 | `test/aftersales/infrastructure/event_handlers/payment_refund_event_handler_test.exs` | 139 | Given 事件直接携带 order_id When refund_success Then 仍能回写refund_info并标记明细completed | BDD-AFTERSALES-REFUND-002 | done |
| 7 | `test/aftersales/infrastructure/event_handlers/payment_refund_event_handler_test.exs` | 201 | Given 同一订单存在多张可退款售后单 When refund_success Then 不回写（避免歧义） | BDD-AFTERSALES-REFUND-003 | done |
| 8 | `test/financial_settlement/event_handlers/refund_deduction_handler_test.exs` | 70 | EH-004: Given 订单已结算 When 投递 RefundSuccess 事件 Then 创建扣除记录且 refund_id 幂等 | BDD-FIN-REFUND-DEDUCTION-HANDLER-004 | done |
| 9 | `test/financial_settlement/infrastructure/repositories/settlement_line_repository_test.exs` | 41 | Given Adjustment 行缺少 reason_code When 写入结算行 Then 返回错误且不落库 |  | TODO |
| 10 | `test/financial_settlement/infrastructure/repositories/settlement_line_repository_test.exs` | 61 | Given 已存在 Normal 行 When 重复写入相同 order_item_id Then 返回错误且只保留一条 |  | TODO |
| 11 | `test/financial_settlement/infrastructure/repositories/settlement_line_repository_test.exs` | 95 | Given 已存在 RETURN Adjustment 行 When 重复写入相同 refund_deduction_id Then 返回错误且只保留一条 |  | TODO |
| 12 | `test/financial_settlement/integration/settlement_flow_test.exs` | 396 | Given 结算单已冻结 When 尝试修改金额/期间/as_of Then 保存返回 :settlement_locked（但允许推进状态） |  | TODO |
| 13 | `test/financial_settlement/integration/settlement_flow_test.exs` | 501 | Given 结算单被拒绝 When 审核 reject Then 不回写订单已结算标记 |  | TODO |
| 14 | `test/financial_settlement/integration/settlement_route_b_after_sales_e2e_test.exs` | 107 | Given 售后通过应用服务提交并完成退款 When 当期出单 Then Normal 行按剩余数量入账且不出 Adjustment | BDD-SETTLEMENT-ROUTE-B-AFTERSALES-001 | done |
| 15 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 129 | Given 当期 Normal 很小但跨期扣除很大 When 出单并审核通过 Then 负净额可持久化且流程正常 | BDD-SETTLEMENT-ROUTE-B-FLOW-001 | done |
| 16 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 279 | Given 订单项发生时间在 as_of 边界 When 出单 Then <= as_of 纳入 Normal 且 > as_of 不纳入 | BDD-SETTLEMENT-ROUTE-B-FLOW-002 | done |
| 17 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 369 | Given 订单不满足结算口径（状态不在 paid/shipped/completed）When 出单 Then 不生成 Normal 行 | BDD-SETTLEMENT-ROUTE-B-FLOW-003 | done |
| 18 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 421 | Given 已结算订单发生退款 When 下期出单 Then 生成 Adjustment 行并推进 refund_deductions 状态 | BDD-SETTLEMENT-ROUTE-B-ADJ-001 | done |
| 19 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 591 | Given 账期起止颠倒 When 出单 Then 返回 :invalid_params 且不落库 | BDD-SETTLEMENT-NEG-001 | done |
| 20 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 611 | Given 同一订单项发生多次部分退款 When 下期出单 Then 多条 Adjustment 可追溯且总额可解释 | BDD-SETTLEMENT-ROUTE-B-FLOW-004 | done |
| 21 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 761 | Given dev 中存在非 UUID supplier_id 的订单 When 按 UUID 供应商出单 Then 该订单被跳过且结算成功 | BDD-SETTLEMENT-ROUTE-B-FLOW-005 | done |
| 22 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 817 | Given 订单 supplier_id 非 UUID 但商品 supplier_id 为 UUID When 按 UUID 出单 Then 仍跳过（策略：严格以订单 supplier_id 为准） | BDD-SETTLEMENT-ROUTE-B-FLOW-006 | done |
| 23 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 893 | Given 出单前售后退款已完成 When 出单 Then 当期同一期吸收（不出 Normal 也不出 Adjustment） | BDD-SETTLEMENT-ROUTE-B-FLOW-007 | done |
| 24 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 990 | Given 出单前部分退货（按订单项数量）When 出单 Then Normal 行按剩余数量入账且不出 Adjustment | BDD-SETTLEMENT-ROUTE-B-AFTERSALES-001 | done |
| 25 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 1113 | Given 出单前部分退 + 结算锁定后再次退款 When 跨两期出单 Then 当期按剩余数量入账且下期生成 Adjustment 并推进扣除状态 | BDD-SETTLEMENT-ROUTE-B-FLOW-008 | done |
| 26 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 1299 | Given pending 扣除缺 origin When 出单 Then 返回 :adjustment_missing_origin 且不落库 | BDD-SETTLEMENT-ROUTE-B-FLOW-009 | done |
| 27 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 1345 | Given 超额冲减 When 下期出单 Then 返回 :deduction_exceeds_origin 且不落库 | BDD-SETTLEMENT-ROUTE-B-FLOW-010 | done |
| 28 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 1449 | Given 对账 diff != 0 When 执行结算 Then 阻断并返回 :reconciliation_diff_not_zero | BDD-SETTLEMENT-ROUTE-B-FLOW-011 | done |
| 29 | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 1500 | Given Adjustment 转写临时失败 When 重试出单 Then 不留脏数据且最终成功 | BDD-SETTLEMENT-ROUTE-B-FLOW-012 | done |
| 30 | `test/financial_settlement/integration/settlement_route_b_recon_block_test.exs` | 73 | Given settlement_lines 与结算头净额不一致 When execute_settlement Then 返回 :reconciliation_diff_not_zero | BDD-SETTLEMENT-ROUTE-B-RECON-001 | done |
| 31 | `test/financial_settlement/services/refund_deduction_service_test.exs` | 67 | Given 订单未结算 When 退款成功调用 create_deduction_if_needed Then 返回 :not_needed 且不创建扣除记录 |  | TODO |
| 32 | `test/financial_settlement/services/refund_deduction_service_test.exs` | 123 | Given 订单已结算 When 退款成功调用 create_deduction_if_needed Then 创建扣除记录且 refund_id 幂等 |  | TODO |
| 33 | `test/financial_settlement/services/settlement_recon_bridge_test.exs` | 36 | Given 结算单存在 Normal/Adjustment 行 When 查询对账汇总 Then 返回 computed_net 且 diff 可用于验收 |  | TODO |
| 34 | `test/financial_settlement/services/settlement_recon_bridge_test.exs` | 78 | Given settlement_id 不存在 When 查询对账汇总 Then 返回 :not_found |  | TODO |
