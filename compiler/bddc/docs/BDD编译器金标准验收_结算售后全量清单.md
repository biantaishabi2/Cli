# 结算 + 售后：全量测试用例清单（Golden Master 附录）

> 更新时间：2026-02-09

本文档是 `docs/bdd/BDD编译器金标准验收.md` 的附录，用于把结算与售后模块的 **所有 ExUnit 测试用例**（`test "..."`）列为金标准清单，便于后续逐条分配 DSL 场景 ID 并迁移为可编译 DSL。

## 1. 统计

- 结算（`test/financial_settlement/**`）：341 条
- 售后（`test/aftersales/**`）：263 条
- 合计：604 条

BDD 标准口径（用于快速筛选）：
- `BDD(strict)`: 标题同时包含 `Given` + `When` + `Then`（三段齐全）
- `BDD(semantic)`: 不看标题字面，而是按“测试本质”近似判断（启发式），用于先圈定更接近验收/行为层的用例集合。
  - v1（边界分层）：`layer in {integration, event_handlers, events}` 或 `BDD(strict)`
  - v2（边界分层 + 常用前缀）：v1 或 标题前缀属于 `{IT-, EH-, SVC-}`（UT- 仍视为单测，不纳入）

- BDD(strict) 用例数：34
- BDD(semantic v1) 用例数：103
- BDD(semantic v2) 用例数：113

说明：

- `BDD(semantic)` 是“统计/筛选口径”，不是最终验收边界；它只能帮助我们把更像“行为验收”的测试先拉出来迁移。
- 若你要求“604 条全部都要迁移成 DSL”，那就不需要 `BDD(semantic)` 这层筛选，直接按清单逐条迁移即可。

## 2. 用例清单（604 条）

| # | 模块 | 分层 | 文件 | 行号 | 用例标题 | BDD(strict) | DSL 场景 ID | 迁移状态 |
| - | - | - | - | - | - | - | - | - |
| 1 | 售后 | application | `test/aftersales/application/after_sales_application_service_test.exs` | 87 | submits after sales request |  | BDD-GM-AS-APPLICATION-0001 | TODO |
| 2 | 售后 | application | `test/aftersales/application/after_sales_application_service_test.exs` | 112 | gets after sales order details |  | BDD-GM-AS-APPLICATION-0002 | TODO |
| 3 | 售后 | application | `test/aftersales/application/after_sales_application_service_test.exs` | 132 | gets user after sales orders |  | BDD-GM-AS-APPLICATION-0003 | TODO |
| 4 | 售后 | application | `test/aftersales/application/after_sales_application_service_test.exs` | 156 | gets supplier after sales orders |  | BDD-GM-AS-APPLICATION-0004 | TODO |
| 5 | 售后 | application | `test/aftersales/application/after_sales_application_service_test.exs` | 180 | cancels after sales request |  | BDD-GM-AS-APPLICATION-0005 | TODO |
| 6 | 售后 | application | `test/aftersales/application/after_sales_application_service_test.exs` | 197 | handles non-existent after sales order |  | BDD-GM-AS-APPLICATION-0006 | TODO |
| 7 | 售后 | application | `test/aftersales/application/after_sales_application_service_test.exs` | 209 | validates submit command |  | BDD-GM-AS-APPLICATION-0007 | TODO |
| 8 | 售后 | application | `test/aftersales/application/after_sales_application_service_test.exs` | 229 | normalizes submit errors to list for API stability |  | BDD-GM-AS-APPLICATION-0008 | TODO |
| 9 | 售后 | application | `test/aftersales/application/after_sales_application_service_test.exs` | 248 | transforms domain object to DTO |  | BDD-GM-AS-APPLICATION-0009 | TODO |
| 10 | 售后 | application | `test/aftersales/application/after_sales_application_service_test.exs` | 274 | handles pagination correctly |  | BDD-GM-AS-APPLICATION-0010 | TODO |
| 11 | 售后 | application | `test/aftersales/application/after_sales_application_service_test.exs` | 294 | gets statistics for supplier |  | BDD-GM-AS-APPLICATION-0011 | TODO |
| 12 | 售后 | application | `test/aftersales/application/after_sales_processing_application_service_test.exs` | 84 | supplier approves after sales request |  | BDD-GM-AS-APPLICATION-0012 | TODO |
| 13 | 售后 | application | `test/aftersales/application/after_sales_processing_application_service_test.exs` | 104 | supplier rejects after sales request |  | BDD-GM-AS-APPLICATION-0013 | TODO |
| 14 | 售后 | application | `test/aftersales/application/after_sales_processing_application_service_test.exs` | 123 | updates logistics information |  | BDD-GM-AS-APPLICATION-0014 | TODO |
| 15 | 售后 | application | `test/aftersales/application/after_sales_processing_application_service_test.exs` | 142 | confirms receipt |  | BDD-GM-AS-APPLICATION-0015 | TODO |
| 16 | 售后 | application | `test/aftersales/application/after_sales_processing_application_service_test.exs` | 159 | processes refund |  | BDD-GM-AS-APPLICATION-0016 | TODO |
| 17 | 售后 | application | `test/aftersales/application/after_sales_processing_application_service_test.exs` | 180 | completes after sales processing |  | BDD-GM-AS-APPLICATION-0017 | TODO |
| 18 | 售后 | application | `test/aftersales/application/after_sales_processing_application_service_test.exs` | 194 | validates supplier authority on approval |  | BDD-GM-AS-APPLICATION-0018 | TODO |
| 19 | 售后 | application | `test/aftersales/application/after_sales_processing_application_service_test.exs` | 212 | handles non-existent order |  | BDD-GM-AS-APPLICATION-0019 | TODO |
| 20 | 售后 | application | `test/aftersales/application/after_sales_processing_application_service_test.exs` | 228 | validates refund amount |  | BDD-GM-AS-APPLICATION-0020 | TODO |
| 21 | 售后 | application | `test/aftersales/application/after_sales_processing_application_service_test.exs` | 247 | handles inspection failure |  | BDD-GM-AS-APPLICATION-0021 | TODO |
| 22 | 售后 | application | `test/aftersales/application/after_sales_processing_application_service_test.exs` | 265 | tracks logistics status updates |  | BDD-GM-AS-APPLICATION-0022 | TODO |
| 23 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 15 | validates required fields |  | BDD-GM-AS-APPLICATION-0023 | TODO |
| 24 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 26 | validates after sales type |  | BDD-GM-AS-APPLICATION-0024 | TODO |
| 25 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 40 | validates reason category |  | BDD-GM-AS-APPLICATION-0025 | TODO |
| 26 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 54 | accepts valid command |  | BDD-GM-AS-APPLICATION-0026 | TODO |
| 27 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 71 | validates required fields |  | BDD-GM-AS-APPLICATION-0027 | TODO |
| 28 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 77 | accepts valid command |  | BDD-GM-AS-APPLICATION-0028 | TODO |
| 29 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 88 | validates required fields |  | BDD-GM-AS-APPLICATION-0029 | TODO |
| 30 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 95 | accepts valid command |  | BDD-GM-AS-APPLICATION-0030 | TODO |
| 31 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 106 | validates required fields |  | BDD-GM-AS-APPLICATION-0031 | TODO |
| 32 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 113 | accepts valid command |  | BDD-GM-AS-APPLICATION-0032 | TODO |
| 33 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 124 | validates required fields |  | BDD-GM-AS-APPLICATION-0033 | TODO |
| 34 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 131 | validates logistics status |  | BDD-GM-AS-APPLICATION-0034 | TODO |
| 35 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 143 | accepts valid command |  | BDD-GM-AS-APPLICATION-0035 | TODO |
| 36 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 158 | validates required fields |  | BDD-GM-AS-APPLICATION-0036 | TODO |
| 37 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 165 | validates refund amount |  | BDD-GM-AS-APPLICATION-0037 | TODO |
| 38 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 177 | validates refund method |  | BDD-GM-AS-APPLICATION-0038 | TODO |
| 39 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 190 | accepts valid command |  | BDD-GM-AS-APPLICATION-0039 | TODO |
| 40 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 205 | validates required fields |  | BDD-GM-AS-APPLICATION-0040 | TODO |
| 41 | 售后 | application | `test/aftersales/application/commands/after_sales_commands_test.exs` | 211 | accepts valid command |  | BDD-GM-AS-APPLICATION-0041 | TODO |
| 42 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 124 | submits complaint |  | BDD-GM-AS-APPLICATION-0042 | TODO |
| 43 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 148 | supplier responds to complaint |  | BDD-GM-AS-APPLICATION-0043 | TODO |
| 44 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 167 | escalates complaint |  | BDD-GM-AS-APPLICATION-0044 | TODO |
| 45 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 185 | platform processes complaint |  | BDD-GM-AS-APPLICATION-0045 | TODO |
| 46 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 205 | gets user complaints |  | BDD-GM-AS-APPLICATION-0046 | TODO |
| 47 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 222 | gets supplier complaints |  | BDD-GM-AS-APPLICATION-0047 | TODO |
| 48 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 242 | gets complaint details |  | BDD-GM-AS-APPLICATION-0048 | TODO |
| 49 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 259 | handles non-existent complaint |  | BDD-GM-AS-APPLICATION-0049 | TODO |
| 50 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 271 | validates complaint content |  | BDD-GM-AS-APPLICATION-0050 | TODO |
| 51 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 291 | validates complaint type |  | BDD-GM-AS-APPLICATION-0051 | TODO |
| 52 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 312 | transforms domain object to DTO |  | BDD-GM-AS-APPLICATION-0052 | TODO |
| 53 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 338 | filters complaints by date range |  | BDD-GM-AS-APPLICATION-0053 | TODO |
| 54 | 售后 | application | `test/aftersales/application/complaint_application_service_test.exs` | 357 | gets complaint statistics |  | BDD-GM-AS-APPLICATION-0054 | TODO |
| 55 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 9 | creates valid after sales order |  | BDD-GM-AS-DOMAIN-0001 | TODO |
| 56 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 32 | fails to create after sales order without supplier_id |  | BDD-GM-AS-DOMAIN-0002 | TODO |
| 57 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 46 | fails to create after sales order with invalid type |  | BDD-GM-AS-DOMAIN-0003 | TODO |
| 58 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 61 | assigns after sales order to supplier |  | BDD-GM-AS-DOMAIN-0004 | TODO |
| 59 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 75 | supplier approves after sales order |  | BDD-GM-AS-DOMAIN-0005 | TODO |
| 60 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 92 | supplier rejects after sales order |  | BDD-GM-AS-DOMAIN-0006 | TODO |
| 61 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 109 | starts processing after sales order |  | BDD-GM-AS-DOMAIN-0007 | TODO |
| 62 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 122 | fails invalid status transition |  | BDD-GM-AS-DOMAIN-0008 | TODO |
| 63 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 134 | adds after sales item |  | BDD-GM-AS-DOMAIN-0009 | TODO |
| 64 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 161 | updates processing details |  | BDD-GM-AS-DOMAIN-0010 | TODO |
| 65 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 181 | checks supplier processing authority |  | BDD-GM-AS-DOMAIN-0011 | TODO |
| 66 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 192 | calculates refund amount |  | BDD-GM-AS-DOMAIN-0012 | TODO |
| 67 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 208 | updates processing details with logistics info |  | BDD-GM-AS-DOMAIN-0013 | TODO |
| 68 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 232 | checks if order can be cancelled |  | BDD-GM-AS-DOMAIN-0014 | TODO |
| 69 | 售后 | domain | `test/aftersales/domain/entities/after_sales_order_test.exs` | 243 | generates after sales order processed event |  | BDD-GM-AS-DOMAIN-0015 | TODO |
| 70 | 售后 | domain | `test/aftersales/domain/entities/complaint_test.exs` | 8 | creates complaint |  | BDD-GM-AS-DOMAIN-0016 | TODO |
| 71 | 售后 | domain | `test/aftersales/domain/entities/complaint_test.exs` | 30 | assigns complaint to supplier |  | BDD-GM-AS-DOMAIN-0017 | TODO |
| 72 | 售后 | domain | `test/aftersales/domain/entities/complaint_test.exs` | 44 | supplier responds to complaint |  | BDD-GM-AS-DOMAIN-0018 | TODO |
| 73 | 售后 | domain | `test/aftersales/domain/entities/complaint_test.exs` | 61 | escalates complaint to platform |  | BDD-GM-AS-DOMAIN-0019 | TODO |
| 74 | 售后 | domain | `test/aftersales/domain/entities/complaint_test.exs` | 78 | platform arbitrates complaint |  | BDD-GM-AS-DOMAIN-0020 | TODO |
| 75 | 售后 | domain | `test/aftersales/domain/entities/complaint_test.exs` | 100 | closes complaint |  | BDD-GM-AS-DOMAIN-0021 | TODO |
| 76 | 售后 | domain | `test/aftersales/domain/entities/complaint_test.exs` | 117 | cannot escalate pending complaint |  | BDD-GM-AS-DOMAIN-0022 | TODO |
| 77 | 售后 | domain | `test/aftersales/domain/entities/complaint_test.exs` | 128 | cannot arbitrate non-platform-arbitration complaint |  | BDD-GM-AS-DOMAIN-0023 | TODO |
| 78 | 售后 | domain | `test/aftersales/domain/entities/complaint_test.exs` | 139 | cannot close unresolved complaint |  | BDD-GM-AS-DOMAIN-0024 | TODO |
| 79 | 售后 | domain | `test/aftersales/domain/entities/complaint_test.exs` | 150 | validates complaint type |  | BDD-GM-AS-DOMAIN-0025 | TODO |
| 80 | 售后 | domain | `test/aftersales/domain/entities/complaint_test.exs` | 175 | requires content for complaint |  | BDD-GM-AS-DOMAIN-0026 | TODO |
| 81 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 7 | creates valid policy rule |  | BDD-GM-AS-DOMAIN-0027 | TODO |
| 82 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 29 | fails to create rule without required fields |  | BDD-GM-AS-DOMAIN-0028 | TODO |
| 83 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 42 | validates rule type |  | BDD-GM-AS-DOMAIN-0029 | TODO |
| 84 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 54 | validates time limit |  | BDD-GM-AS-DOMAIN-0030 | TODO |
| 85 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 67 | validates time limit from |  | BDD-GM-AS-DOMAIN-0031 | TODO |
| 86 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 80 | creates rule with all valid rule types |  | BDD-GM-AS-DOMAIN-0032 | TODO |
| 87 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 95 | updates rule |  | BDD-GM-AS-DOMAIN-0033 | TODO |
| 88 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 118 | activates rule |  | BDD-GM-AS-DOMAIN-0034 | TODO |
| 89 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 134 | deactivates rule |  | BDD-GM-AS-DOMAIN-0035 | TODO |
| 90 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 150 | checks if rule is applicable |  | BDD-GM-AS-DOMAIN-0036 | TODO |
| 91 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 182 | inactive rule is not applicable |  | BDD-GM-AS-DOMAIN-0037 | TODO |
| 92 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 196 | checks auto approve capability |  | BDD-GM-AS-DOMAIN-0038 | TODO |
| 93 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 233 | disabled auto approve |  | BDD-GM-AS-DOMAIN-0039 | TODO |
| 94 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 247 | gets time limit in hours |  | BDD-GM-AS-DOMAIN-0040 | TODO |
| 95 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 260 | gets valid rule types |  | BDD-GM-AS-DOMAIN-0041 | TODO |
| 96 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 268 | gets valid time limit from options |  | BDD-GM-AS-DOMAIN-0042 | TODO |
| 97 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 275 | handles decimal amounts correctly |  | BDD-GM-AS-DOMAIN-0043 | TODO |
| 98 | 售后 | domain | `test/aftersales/domain/entities/policy_rule_test.exs` | 292 | creates rule with complex conditions |  | BDD-GM-AS-DOMAIN-0044 | TODO |
| 99 | 售后 | domain | `test/aftersales/domain/entities/supplier_after_sales_policy_test.exs` | 9 | creates valid supplier after sales policy |  | BDD-GM-AS-DOMAIN-0045 | TODO |
| 100 | 售后 | domain | `test/aftersales/domain/entities/supplier_after_sales_policy_test.exs` | 38 | fails to create policy without supplier_id |  | BDD-GM-AS-DOMAIN-0046 | TODO |
| 101 | 售后 | domain | `test/aftersales/domain/entities/supplier_after_sales_policy_test.exs` | 51 | adds policy rule |  | BDD-GM-AS-DOMAIN-0047 | TODO |
| 102 | 售后 | domain | `test/aftersales/domain/entities/supplier_after_sales_policy_test.exs` | 77 | checks policy applicability within time limit |  | BDD-GM-AS-DOMAIN-0048 | TODO |
| 103 | 售后 | domain | `test/aftersales/domain/entities/supplier_after_sales_policy_test.exs` | 96 | checks policy applicability exceeds time limit |  | BDD-GM-AS-DOMAIN-0049 | TODO |
| 104 | 售后 | domain | `test/aftersales/domain/entities/supplier_after_sales_policy_test.exs` | 115 | activates policy |  | BDD-GM-AS-DOMAIN-0050 | TODO |
| 105 | 售后 | domain | `test/aftersales/domain/entities/supplier_after_sales_policy_test.exs` | 127 | deactivates policy |  | BDD-GM-AS-DOMAIN-0051 | TODO |
| 106 | 售后 | domain | `test/aftersales/domain/entities/supplier_after_sales_policy_test.exs` | 141 | updates policy rules |  | BDD-GM-AS-DOMAIN-0052 | TODO |
| 107 | 售后 | domain | `test/aftersales/domain/entities/supplier_after_sales_policy_test.exs` | 169 | checks multiple rules applicability |  | BDD-GM-AS-DOMAIN-0053 | TODO |
| 108 | 售后 | domain | `test/aftersales/domain/entities/supplier_after_sales_policy_test.exs` | 203 | gets rule by type |  | BDD-GM-AS-DOMAIN-0054 | TODO |
| 109 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 19 | creates AfterSalesOrderCreated event |  | BDD-GM-AS-DOMAIN-0055 | TODO |
| 110 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 41 | creates AfterSalesOrderStatusChanged event |  | BDD-GM-AS-DOMAIN-0056 | TODO |
| 111 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 62 | creates AfterSalesOrderProcessed event |  | BDD-GM-AS-DOMAIN-0057 | TODO |
| 112 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 81 | includes metadata in events |  | BDD-GM-AS-DOMAIN-0058 | TODO |
| 113 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 100 | creates ComplaintCreated event |  | BDD-GM-AS-DOMAIN-0059 | TODO |
| 114 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 120 | creates ComplaintResolved event |  | BDD-GM-AS-DOMAIN-0060 | TODO |
| 115 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 137 | includes order info in complaint events when available |  | BDD-GM-AS-DOMAIN-0061 | TODO |
| 116 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 156 | creates SupplierPolicyUpdated event |  | BDD-GM-AS-DOMAIN-0062 | TODO |
| 117 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 175 | includes rule details in policy update event |  | BDD-GM-AS-DOMAIN-0063 | TODO |
| 118 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 200 | serializes event to JSON |  | BDD-GM-AS-DOMAIN-0064 | TODO |
| 119 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 219 | deserializes event from JSON |  | BDD-GM-AS-DOMAIN-0065 | TODO |
| 120 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 242 | validates required fields in AfterSalesOrderCreated |  | BDD-GM-AS-DOMAIN-0066 | TODO |
| 121 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 256 | validates event type consistency |  | BDD-GM-AS-DOMAIN-0067 | TODO |
| 122 | 售后 | domain | `test/aftersales/domain/events/domain_events_test.exs` | 268 | tracks event causality chain |  | BDD-GM-AS-DOMAIN-0068 | TODO |
| 123 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 88 | creates supplier policy |  | BDD-GM-AS-DOMAIN-0069 | TODO |
| 124 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 115 | updates policy rules |  | BDD-GM-AS-DOMAIN-0070 | TODO |
| 125 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 142 | checks policy compliance |  | BDD-GM-AS-DOMAIN-0071 | TODO |
| 126 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 165 | gets applicable rules |  | BDD-GM-AS-DOMAIN-0072 | TODO |
| 127 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 184 | validates policy rules |  | BDD-GM-AS-DOMAIN-0073 | TODO |
| 128 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 202 | validates rule type |  | BDD-GM-AS-DOMAIN-0074 | TODO |
| 129 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 225 | validates time limit |  | BDD-GM-AS-DOMAIN-0075 | TODO |
| 130 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 249 | checks compliance with expired time limit |  | BDD-GM-AS-DOMAIN-0076 | TODO |
| 131 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 272 | handles supplier without policy |  | BDD-GM-AS-DOMAIN-0077 | TODO |
| 132 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 292 | handles non-existent policy update |  | BDD-GM-AS-DOMAIN-0078 | TODO |
| 133 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 310 | activates and deactivates policy |  | BDD-GM-AS-DOMAIN-0079 | TODO |
| 134 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 332 | gets all rule types for supplier |  | BDD-GM-AS-DOMAIN-0080 | TODO |
| 135 | 售后 | domain | `test/aftersales/domain/services/after_sales_policy_service_test.exs` | 342 | validates duplicate rule types |  | BDD-GM-AS-DOMAIN-0081 | TODO |
| 136 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 61 | creates after sales order |  | BDD-GM-AS-DOMAIN-0082 | TODO |
| 137 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 98 | Given items 缺少必填字段 When create_after_sales_request Then 返回校验错误列表 | Y | BDD-GM-AS-DOMAIN-0083 | TODO |
| 138 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 132 | Given 同一请求内 order_item_id 重复 When create_after_sales_request Then 返回重复错误 | Y | BDD-GM-AS-DOMAIN-0084 | TODO |
| 139 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 171 | Given type=return 且 items 为空 When create_after_sales_request Then 返回 items 不能为空 | Y | BDD-GM-AS-DOMAIN-0085 | TODO |
| 140 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 191 | Given type=exchange 且未提供 items When create_after_sales_request Then 仍可创建（兼容） | Y | BDD-GM-AS-DOMAIN-0086 | TODO |
| 141 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 212 | supplier approves after sales request |  | BDD-GM-AS-DOMAIN-0087 | TODO |
| 142 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 234 | supplier rejects after sales request |  | BDD-GM-AS-DOMAIN-0088 | TODO |
| 143 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 256 | unauthorized supplier cannot process order |  | BDD-GM-AS-DOMAIN-0089 | TODO |
| 144 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 276 | checks policy applicability |  | BDD-GM-AS-DOMAIN-0090 | TODO |
| 145 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 297 | calculates refund amount |  | BDD-GM-AS-DOMAIN-0091 | TODO |
| 146 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 317 | completes after sales processing |  | BDD-GM-AS-DOMAIN-0092 | TODO |
| 147 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 336 | cannot process non-existent order |  | BDD-GM-AS-DOMAIN-0093 | TODO |
| 148 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 353 | validates supplier authority |  | BDD-GM-AS-DOMAIN-0094 | TODO |
| 149 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 373 | cannot complete already completed order |  | BDD-GM-AS-DOMAIN-0095 | TODO |
| 150 | 售后 | domain | `test/aftersales/domain/services/after_sales_service_test.exs` | 388 | checks policy with expired time limit |  | BDD-GM-AS-DOMAIN-0096 | TODO |
| 151 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 41 | creates complaint |  | BDD-GM-AS-DOMAIN-0097 | TODO |
| 152 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 66 | supplier responds to complaint |  | BDD-GM-AS-DOMAIN-0098 | TODO |
| 153 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 86 | escalates complaint to platform |  | BDD-GM-AS-DOMAIN-0099 | TODO |
| 154 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 105 | platform arbitrates complaint |  | BDD-GM-AS-DOMAIN-0100 | TODO |
| 155 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 128 | resolves complaint |  | BDD-GM-AS-DOMAIN-0101 | TODO |
| 156 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 146 | validates complaint content |  | BDD-GM-AS-DOMAIN-0102 | TODO |
| 157 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 166 | validates complaint type |  | BDD-GM-AS-DOMAIN-0103 | TODO |
| 158 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 185 | handles non-existent complaint |  | BDD-GM-AS-DOMAIN-0104 | TODO |
| 159 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 202 | validates supplier authority for response |  | BDD-GM-AS-DOMAIN-0105 | TODO |
| 160 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 234 | validates escalation conditions |  | BDD-GM-AS-DOMAIN-0106 | TODO |
| 161 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 262 | validates arbitration conditions |  | BDD-GM-AS-DOMAIN-0107 | TODO |
| 162 | 售后 | domain | `test/aftersales/domain/services/complaint_service_test.exs` | 290 | validates compensation amount |  | BDD-GM-AS-DOMAIN-0108 | TODO |
| 163 | 售后 | domain | `test/aftersales/domain/value_objects/after_sales_reason_test.exs` | 11 | 创建有效的售后原因 |  | BDD-GM-AS-DOMAIN-0109 | TODO |
| 164 | 售后 | domain | `test/aftersales/domain/value_objects/after_sales_reason_test.exs` | 24 | 缺少必要字段时返回错误 |  | BDD-GM-AS-DOMAIN-0110 | TODO |
| 165 | 售后 | domain | `test/aftersales/domain/value_objects/after_sales_reason_test.exs` | 34 | 无效的category时返回错误 |  | BDD-GM-AS-DOMAIN-0111 | TODO |
| 166 | 售后 | domain | `test/aftersales/domain/value_objects/after_sales_reason_test.exs` | 44 | 所有有效的category都能正常工作 |  | BDD-GM-AS-DOMAIN-0112 | TODO |
| 167 | 售后 | domain | `test/aftersales/domain/value_objects/after_sales_reason_test.exs` | 66 | 空的evidence_images列表默认为空数组 |  | BDD-GM-AS-DOMAIN-0113 | TODO |
| 168 | 售后 | domain | `test/aftersales/domain/value_objects/after_sales_reason_test.exs` | 78 | 验证描述长度限制 |  | BDD-GM-AS-DOMAIN-0114 | TODO |
| 169 | 售后 | domain | `test/aftersales/domain/value_objects/after_sales_reason_test.exs` | 91 | 验证证据图片数量限制 |  | BDD-GM-AS-DOMAIN-0115 | TODO |
| 170 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 7 | creates valid logistics info |  | BDD-GM-AS-DOMAIN-0116 | TODO |
| 171 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 21 | creates empty logistics info |  | BDD-GM-AS-DOMAIN-0117 | TODO |
| 172 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 29 | updates logistics status |  | BDD-GM-AS-DOMAIN-0118 | TODO |
| 173 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 37 | sets actual_delivery when status becomes delivered |  | BDD-GM-AS-DOMAIN-0119 | TODO |
| 174 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 47 | updates shipping info |  | BDD-GM-AS-DOMAIN-0120 | TODO |
| 175 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 57 | validates status transitions |  | BDD-GM-AS-DOMAIN-0121 | TODO |
| 176 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 64 | checks if delivered |  | BDD-GM-AS-DOMAIN-0122 | TODO |
| 177 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 72 | checks if trackable |  | BDD-GM-AS-DOMAIN-0123 | TODO |
| 178 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 82 | validates tracking number length |  | BDD-GM-AS-DOMAIN-0124 | TODO |
| 179 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 88 | validates company name |  | BDD-GM-AS-DOMAIN-0125 | TODO |
| 180 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 98 | validates status |  | BDD-GM-AS-DOMAIN-0126 | TODO |
| 181 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 109 | converts to map |  | BDD-GM-AS-DOMAIN-0127 | TODO |
| 182 | 售后 | domain | `test/aftersales/domain/value_objects/logistics_info_test.exs` | 124 | creates from map |  | BDD-GM-AS-DOMAIN-0128 | TODO |
| 183 | 售后 | domain | `test/aftersales/domain/value_objects/processing_note_test.exs` | 8 | creates processing note |  | BDD-GM-AS-DOMAIN-0129 | TODO |
| 184 | 售后 | domain | `test/aftersales/domain/value_objects/processing_note_test.exs` | 26 | fails to create note with empty content |  | BDD-GM-AS-DOMAIN-0130 | TODO |
| 185 | 售后 | domain | `test/aftersales/domain/value_objects/processing_note_test.exs` | 38 | validates note types |  | BDD-GM-AS-DOMAIN-0131 | TODO |
| 186 | 售后 | domain | `test/aftersales/domain/value_objects/processing_note_test.exs` | 52 | fails to create note with invalid type |  | BDD-GM-AS-DOMAIN-0132 | TODO |
| 187 | 售后 | domain | `test/aftersales/domain/value_objects/processing_note_test.exs` | 64 | requires created_by |  | BDD-GM-AS-DOMAIN-0133 | TODO |
| 188 | 售后 | domain | `test/aftersales/domain/value_objects/processing_note_test.exs` | 76 | immutability of processing note |  | BDD-GM-AS-DOMAIN-0134 | TODO |
| 189 | 售后 | domain | `test/aftersales/domain/value_objects/processing_note_test.exs` | 105 | trims whitespace from note content |  | BDD-GM-AS-DOMAIN-0135 | TODO |
| 190 | 售后 | domain | `test/aftersales/domain/value_objects/processing_note_test.exs` | 118 | allows multiline notes |  | BDD-GM-AS-DOMAIN-0136 | TODO |
| 191 | 售后 | domain | `test/aftersales/domain/value_objects/processing_note_test.exs` | 136 | enforces maximum note length |  | BDD-GM-AS-DOMAIN-0137 | TODO |
| 192 | 售后 | domain | `test/aftersales/domain/value_objects/processing_note_test.exs` | 151 | formats note for display |  | BDD-GM-AS-DOMAIN-0138 | TODO |
| 193 | 售后 | domain | `test/aftersales/domain/value_objects/processing_note_test.exs` | 167 | creates note with custom timestamp |  | BDD-GM-AS-DOMAIN-0139 | TODO |
| 194 | 售后 | domain | `test/aftersales/domain/value_objects/refund_info_test.exs` | 8 | creates valid refund info |  | BDD-GM-AS-DOMAIN-0140 | TODO |
| 195 | 售后 | domain | `test/aftersales/domain/value_objects/refund_info_test.exs` | 26 | fails to create refund info with negative amount |  | BDD-GM-AS-DOMAIN-0141 | TODO |
| 196 | 售后 | domain | `test/aftersales/domain/value_objects/refund_info_test.exs` | 39 | compares refund info for equality |  | BDD-GM-AS-DOMAIN-0142 | TODO |
| 197 | 售后 | domain | `test/aftersales/domain/value_objects/refund_info_test.exs` | 57 | validates refund methods |  | BDD-GM-AS-DOMAIN-0143 | TODO |
| 198 | 售后 | domain | `test/aftersales/domain/value_objects/refund_info_test.exs` | 71 | fails to create refund info with invalid method |  | BDD-GM-AS-DOMAIN-0144 | TODO |
| 199 | 售后 | domain | `test/aftersales/domain/value_objects/refund_info_test.exs` | 83 | requires transaction_id |  | BDD-GM-AS-DOMAIN-0145 | TODO |
| 200 | 售后 | domain | `test/aftersales/domain/value_objects/refund_info_test.exs` | 95 | immutability of refund info |  | BDD-GM-AS-DOMAIN-0146 | TODO |
| 201 | 售后 | domain | `test/aftersales/domain/value_objects/refund_info_test.exs` | 114 | tracks refund status changes |  | BDD-GM-AS-DOMAIN-0147 | TODO |
| 202 | 售后 | domain | `test/aftersales/domain/value_objects/refund_info_test.exs` | 138 | validates amount precision |  | BDD-GM-AS-DOMAIN-0148 | TODO |
| 203 | 售后 | domain | `test/aftersales/domain/value_objects/refund_info_test.exs` | 160 | zero amount is invalid |  | BDD-GM-AS-DOMAIN-0149 | TODO |
| 204 | 售后 | domain | `test/aftersales/domain/value_objects/supplier_response_test.exs` | 8 | creates valid supplier response |  | BDD-GM-AS-DOMAIN-0150 | TODO |
| 205 | 售后 | domain | `test/aftersales/domain/value_objects/supplier_response_test.exs` | 27 | fails to create response without supplier_id |  | BDD-GM-AS-DOMAIN-0151 | TODO |
| 206 | 售后 | domain | `test/aftersales/domain/value_objects/supplier_response_test.exs` | 39 | fails to create response with invalid type |  | BDD-GM-AS-DOMAIN-0152 | TODO |
| 207 | 售后 | domain | `test/aftersales/domain/value_objects/supplier_response_test.exs` | 52 | compares supplier responses for equality |  | BDD-GM-AS-DOMAIN-0153 | TODO |
| 208 | 售后 | domain | `test/aftersales/domain/value_objects/supplier_response_test.exs` | 70 | validates response types |  | BDD-GM-AS-DOMAIN-0154 | TODO |
| 209 | 售后 | domain | `test/aftersales/domain/value_objects/supplier_response_test.exs` | 84 | requires response content |  | BDD-GM-AS-DOMAIN-0155 | TODO |
| 210 | 售后 | domain | `test/aftersales/domain/value_objects/supplier_response_test.exs` | 96 | immutability of supplier response |  | BDD-GM-AS-DOMAIN-0156 | TODO |
| 211 | 售后 | domain | `test/aftersales/domain/value_objects/supplier_response_test.exs` | 115 | sets handled_at automatically if not provided |  | BDD-GM-AS-DOMAIN-0157 | TODO |
| 212 | 售后 | infrastructure | `test/aftersales/infrastructure/event_handlers/payment_refund_event_handler_test.exs` | 43 | Given 售后明细已落库 When refund_success Then 回写refund_info并标记明细completed | Y | BDD-AFTERSALES-REFUND-001 | done |
| 213 | 售后 | infrastructure | `test/aftersales/infrastructure/event_handlers/payment_refund_event_handler_test.exs` | 139 | Given 事件直接携带 order_id When refund_success Then 仍能回写refund_info并标记明细completed | Y | BDD-AFTERSALES-REFUND-002 | done |
| 214 | 售后 | infrastructure | `test/aftersales/infrastructure/event_handlers/payment_refund_event_handler_test.exs` | 201 | Given 同一订单存在多张可退款售后单 When refund_success Then 不回写（避免歧义） | Y | BDD-AFTERSALES-REFUND-003 | done |
| 215 | 售后 | infrastructure | `test/aftersales/infrastructure/event_handlers/refund_trigger_handler_test.exs` | 53 | 售后审批通过时自动触发退款 |  | BDD-AFTERSALES-REFUND-TRIGGER-001 | done |
| 216 | 售后 | infrastructure | `test/aftersales/infrastructure/event_handlers/refund_trigger_handler_test.exs` | 113 | 自动审批时也触发退款 |  | BDD-AFTERSALES-REFUND-TRIGGER-002 | done |
| 217 | 售后 | infrastructure | `test/aftersales/infrastructure/event_handlers/refund_trigger_handler_test.exs` | 166 | 无支付单的售后审批不触发退款 |  | BDD-AFTERSALES-REFUND-TRIGGER-003 | done |
| 218 | 售后 | infrastructure | `test/aftersales/infrastructure/event_handlers/refund_trigger_handler_test.exs` | 208 | 拒绝的售后不触发退款 |  | BDD-AFTERSALES-REFUND-TRIGGER-004 | done |
| 219 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/after_sales_order_repository_test.exs` | 11 | saves and finds after sales order |  | BDD-GM-AS-INFRASTRUCTURE-0001 | TODO |
| 220 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/after_sales_order_repository_test.exs` | 35 | returns error when order not found |  | BDD-GM-AS-INFRASTRUCTURE-0002 | TODO |
| 221 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/after_sales_order_repository_test.exs` | 41 | finds orders by supplier |  | BDD-GM-AS-INFRASTRUCTURE-0003 | TODO |
| 222 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/after_sales_order_repository_test.exs` | 87 | updates existing order |  | BDD-GM-AS-INFRASTRUCTURE-0004 | TODO |
| 223 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/after_sales_order_repository_test.exs` | 114 | finds timeout orders |  | BDD-GM-AS-INFRASTRUCTURE-0005 | TODO |
| 224 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/after_sales_order_repository_test.exs` | 157 | finds orders by status |  | BDD-GM-AS-INFRASTRUCTURE-0006 | TODO |
| 225 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/after_sales_order_repository_test.exs` | 184 | saves after_sales_items and loads them back (order item granularity) |  | BDD-GM-AS-INFRASTRUCTURE-0007 | TODO |
| 226 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/after_sales_order_repository_test.exs` | 228 | saving same after_sales_order twice does not create duplicate after_sales_items (idempotent) |  | BDD-GM-AS-INFRASTRUCTURE-0008 | TODO |
| 227 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/after_sales_order_repository_test.exs` | 265 | cumulative after_sales_quantity cannot exceed ordered_quantity (blocks over-refund) |  | BDD-GM-AS-INFRASTRUCTURE-0009 | TODO |
| 228 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/complaint_repository_test.exs` | 8 | saves and finds complaint |  | BDD-GM-AS-INFRASTRUCTURE-0010 | TODO |
| 229 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/complaint_repository_test.exs` | 32 | returns error when complaint not found |  | BDD-GM-AS-INFRASTRUCTURE-0011 | TODO |
| 230 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/complaint_repository_test.exs` | 37 | finds complaints by after sales order |  | BDD-GM-AS-INFRASTRUCTURE-0012 | TODO |
| 231 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/complaint_repository_test.exs` | 71 | updates existing complaint |  | BDD-GM-AS-INFRASTRUCTURE-0013 | TODO |
| 232 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/complaint_repository_test.exs` | 97 | finds complaints by status |  | BDD-GM-AS-INFRASTRUCTURE-0014 | TODO |
| 233 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/complaint_repository_test.exs` | 123 | finds complaints by supplier |  | BDD-GM-AS-INFRASTRUCTURE-0015 | TODO |
| 234 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 15 | saves and retrieves after sales order by id |  | BDD-GM-AS-INFRASTRUCTURE-0016 | TODO |
| 235 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 42 | finds after sales orders by supplier |  | BDD-GM-AS-INFRASTRUCTURE-0017 | TODO |
| 236 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 112 | finds after sales orders by supplier and status |  | BDD-GM-AS-INFRASTRUCTURE-0018 | TODO |
| 237 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 153 | updates after sales order |  | BDD-GM-AS-INFRASTRUCTURE-0019 | TODO |
| 238 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 194 | handles non-existent after sales order |  | BDD-GM-AS-INFRASTRUCTURE-0020 | TODO |
| 239 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 198 | searches after sales orders with pagination |  | BDD-GM-AS-INFRASTRUCTURE-0021 | TODO |
| 240 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 249 | saves and finds active supplier policy |  | BDD-GM-AS-INFRASTRUCTURE-0022 | TODO |
| 241 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 287 | returns error when no active policy found |  | BDD-GM-AS-INFRASTRUCTURE-0023 | TODO |
| 242 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 292 | finds only active policy when multiple exist |  | BDD-GM-AS-INFRASTRUCTURE-0024 | TODO |
| 243 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 350 | supports multi-dimensional complaint queries |  | BDD-GM-AS-INFRASTRUCTURE-0025 | TODO |
| 244 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 384 | searches complaints with complex criteria |  | BDD-GM-AS-INFRASTRUCTURE-0026 | TODO |
| 245 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 431 | repository supports all required query operations |  | BDD-GM-AS-INFRASTRUCTURE-0027 | TODO |
| 246 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 481 | ensures supplier data isolation |  | BDD-GM-AS-INFRASTRUCTURE-0028 | TODO |
| 247 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/repository_contract_test.exs` | 529 | handles repository errors properly |  | BDD-GM-AS-INFRASTRUCTURE-0029 | TODO |
| 248 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/supplier_after_sales_policy_repository_test.exs` | 8 | saves and finds active supplier policy |  | BDD-GM-AS-INFRASTRUCTURE-0030 | TODO |
| 249 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/supplier_after_sales_policy_repository_test.exs` | 40 | returns error when no active policy found |  | BDD-GM-AS-INFRASTRUCTURE-0031 | TODO |
| 250 | 售后 | infrastructure | `test/aftersales/infrastructure/repositories/supplier_after_sales_policy_repository_test.exs` | 45 | finds only active policy when multiple exist |  | BDD-GM-AS-INFRASTRUCTURE-0032 | TODO |
| 251 | 售后 | integration | `test/aftersales/integration/after_sales_flow_test.exs` | 49 | complete return flow from submission to refund |  | BDD-GM-AS-INTEGRATION-0001 | TODO |
| 252 | 售后 | integration | `test/aftersales/integration/after_sales_flow_test.exs` | 153 | complete exchange flow |  | BDD-GM-AS-INTEGRATION-0002 | TODO |
| 253 | 售后 | integration | `test/aftersales/integration/after_sales_flow_test.exs` | 240 | supplier rejects after-sales request |  | BDD-GM-AS-INTEGRATION-0003 | TODO |
| 254 | 售后 | integration | `test/aftersales/integration/after_sales_flow_test.exs` | 294 | refund only flow without return |  | BDD-GM-AS-INTEGRATION-0004 | TODO |
| 255 | 售后 | integration | `test/aftersales/integration/after_sales_flow_test.exs` | 363 | after-sales timeout auto processing |  | BDD-GM-AS-INTEGRATION-0005 | TODO |
| 256 | 售后 | integration | `test/aftersales/integration/after_sales_flow_test.exs` | 423 | concurrent after-sales request handling |  | BDD-GM-AS-INTEGRATION-0006 | TODO |
| 257 | 售后 | integration | `test/aftersales/integration/complaint_flow_test.exs` | 25 | complete complaint flow resolved by supplier |  | BDD-GM-AS-INTEGRATION-0007 | TODO |
| 258 | 售后 | integration | `test/aftersales/integration/complaint_flow_test.exs` | 96 | complaint escalated to platform arbitration |  | BDD-GM-AS-INTEGRATION-0008 | TODO |
| 259 | 售后 | integration | `test/aftersales/integration/complaint_flow_test.exs` | 185 | handles different complaint types |  | BDD-GM-AS-INTEGRATION-0009 | TODO |
| 260 | 售后 | integration | `test/aftersales/integration/complaint_flow_test.exs` | 226 | handles complaint timeout without supplier response |  | BDD-GM-AS-INTEGRATION-0010 | TODO |
| 261 | 售后 | integration | `test/aftersales/integration/complaint_flow_test.exs` | 279 | batch complaint query and statistics |  | BDD-GM-AS-INTEGRATION-0011 | TODO |
| 262 | 售后 | integration | `test/aftersales/integration/complaint_flow_test.exs` | 357 | complaint related to after-sales order |  | BDD-GM-AS-INTEGRATION-0012 | TODO |
| 263 | 售后 | integration | `test/aftersales/integration/complaint_flow_test.exs` | 420 | concurrent complaint processing |  | BDD-GM-AS-INTEGRATION-0013 | TODO |
| 264 | 结算 | application | `test/financial_settlement/application/reconciliation_application_service_test.exs` | 101 | 成功触发新的对账 |  | BDD-GM-FS-APPLICATION-0001 | TODO |
| 265 | 结算 | application | `test/financial_settlement/application/reconciliation_application_service_test.exs` | 141 | 批次号重复时返回错误 |  | BDD-GM-FS-APPLICATION-0002 | TODO |
| 266 | 结算 | application | `test/financial_settlement/application/reconciliation_application_service_test.exs` | 167 | 查询存在的对账批次 |  | BDD-GM-FS-APPLICATION-0003 | TODO |
| 267 | 结算 | application | `test/financial_settlement/application/reconciliation_application_service_test.exs` | 185 | 查询不存在的对账批次 |  | BDD-GM-FS-APPLICATION-0004 | TODO |
| 268 | 结算 | application | `test/financial_settlement/application/reconciliation_application_service_test.exs` | 200 | 按条件查询对账列表 |  | BDD-GM-FS-APPLICATION-0005 | TODO |
| 269 | 结算 | application | `test/financial_settlement/application/reconciliation_application_service_test.exs` | 236 | 按状态过滤 |  | BDD-GM-FS-APPLICATION-0006 | TODO |
| 270 | 结算 | application | `test/financial_settlement/application/reconciliation_application_service_test.exs` | 259 | 获取完整的对账详情 |  | BDD-GM-FS-APPLICATION-0007 | TODO |
| 271 | 结算 | application | `test/financial_settlement/application/reconciliation_application_service_test.exs` | 289 | 处理对账差异 |  | BDD-GM-FS-APPLICATION-0008 | TODO |
| 272 | 结算 | application | `test/financial_settlement/application/reconciliation_application_service_test.exs` | 320 | 导出对账报告 |  | BDD-GM-FS-APPLICATION-0009 | TODO |
| 273 | 结算 | application | `test/financial_settlement/application/reconciliation_application_service_test.exs` | 346 | 设置自动对账计划 |  | BDD-GM-FS-APPLICATION-0010 | TODO |
| 274 | 结算 | application | `test/financial_settlement/application/reconciliation_application_service_test.exs` | 374 | 获取对账统计数据 |  | BDD-GM-FS-APPLICATION-0011 | TODO |
| 275 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 223 | 成功创建新的结算单 |  | BDD-GM-FS-APPLICATION-0012 | TODO |
| 276 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 249 | 创建结算单时自动计算利润 |  | BDD-GM-FS-APPLICATION-0013 | TODO |
| 277 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 297 | 供应商不存在时创建失败 |  | BDD-GM-FS-APPLICATION-0014 | TODO |
| 278 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 320 | 成功审核结算单 |  | BDD-GM-FS-APPLICATION-0015 | TODO |
| 279 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 355 | 拒绝结算单 |  | BDD-GM-FS-APPLICATION-0016 | TODO |
| 280 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 386 | 成功执行结算支付 |  | BDD-GM-FS-APPLICATION-0017 | TODO |
| 281 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 426 | 确认结算成功 |  | BDD-GM-FS-APPLICATION-0018 | TODO |
| 282 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 464 | 按供应商查询结算单列表 |  | BDD-GM-FS-APPLICATION-0019 | TODO |
| 283 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 499 | 按状态过滤结算单 |  | BDD-GM-FS-APPLICATION-0020 | TODO |
| 284 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 520 | 获取完整的结算单详情 |  | BDD-GM-FS-APPLICATION-0021 | TODO |
| 285 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 549 | 查询结算单时返回利润数据 |  | BDD-GM-FS-APPLICATION-0022 | TODO |
| 286 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 588 | 获取结算统计数据 |  | BDD-GM-FS-APPLICATION-0023 | TODO |
| 287 | 结算 | application | `test/financial_settlement/application/settlement_application_service_test.exs` | 625 | 批量审核结算单 |  | BDD-GM-FS-APPLICATION-0024 | TODO |
| 288 | 结算 | application | `test/financial_settlement/application/settlement_with_bank_account_test.exs` | 46 | 供应商有银行账户时，结算单使用供应商的银行账户 |  | BDD-GM-FS-APPLICATION-0025 | TODO |
| 289 | 结算 | application | `test/financial_settlement/application/settlement_with_bank_account_test.exs` | 75 | 供应商没有银行账户时，结算单使用默认值 |  | BDD-GM-FS-APPLICATION-0026 | TODO |
| 290 | 结算 | application | `test/financial_settlement/application/settlement_with_bank_account_test.exs` | 98 | 使用供应商的佣金率 |  | BDD-GM-FS-APPLICATION-0027 | TODO |
| 291 | 结算 | application | `test/financial_settlement/application/settlement_with_bank_account_test.exs` | 120 | 供应商不存在时返回错误 |  | BDD-GM-FS-APPLICATION-0028 | TODO |
| 292 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 11 | 创建有效的财务报表成功 |  | BDD-GM-FS-DOMAIN-0001 | TODO |
| 293 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 35 | 创建缺少必要字段的报表返回错误 |  | BDD-GM-FS-DOMAIN-0002 | TODO |
| 294 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 46 | 创建无效报表类型返回错误 |  | BDD-GM-FS-DOMAIN-0003 | TODO |
| 295 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 60 | 添加报表章节 |  | BDD-GM-FS-DOMAIN-0004 | TODO |
| 296 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 78 | 计算报表数据 |  | BDD-GM-FS-DOMAIN-0005 | TODO |
| 297 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 89 | 已发布的报表不能添加章节 |  | BDD-GM-FS-DOMAIN-0006 | TODO |
| 298 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 100 | 审核通过报表 |  | BDD-GM-FS-DOMAIN-0007 | TODO |
| 299 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 111 | 审核拒绝报表 |  | BDD-GM-FS-DOMAIN-0008 | TODO |
| 300 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 122 | 只有待审核状态可以审核 |  | BDD-GM-FS-DOMAIN-0009 | TODO |
| 301 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 131 | 发布已审核的报表 |  | BDD-GM-FS-DOMAIN-0010 | TODO |
| 302 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 142 | 只有已审核状态可以发布 |  | BDD-GM-FS-DOMAIN-0011 | TODO |
| 303 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 151 | 归档已发布的报表 |  | BDD-GM-FS-DOMAIN-0012 | TODO |
| 304 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 159 | 只有已发布状态可以归档 |  | BDD-GM-FS-DOMAIN-0013 | TODO |
| 305 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 168 | 导出报表数据为指定格式 |  | BDD-GM-FS-DOMAIN-0014 | TODO |
| 306 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 178 | 支持多种导出格式 |  | BDD-GM-FS-DOMAIN-0015 | TODO |
| 307 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 194 | 生成中的报表不能导出 |  | BDD-GM-FS-DOMAIN-0016 | TODO |
| 308 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 202 | 报表编号必须唯一 |  | BDD-GM-FS-DOMAIN-0017 | TODO |
| 309 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 218 | 报表周期结束日期不能早于开始日期 |  | BDD-GM-FS-DOMAIN-0018 | TODO |
| 310 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 233 | UT-FS-FR-002: 生成利润分析报表 |  | BDD-GM-FS-DOMAIN-0019 | TODO |
| 311 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 266 | UT-FS-FR-003: 按供应商分析利润 |  | BDD-GM-FS-DOMAIN-0020 | TODO |
| 312 | 结算 | domain | `test/financial_settlement/domain/financial_report_test.exs` | 306 | 计算利润率指标 |  | BDD-GM-FS-DOMAIN-0021 | TODO |
| 313 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 10 | UT-FS-R-001: 创建有效对账批次返回待处理状态 |  | BDD-GM-FS-DOMAIN-0022 | TODO |
| 314 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 39 | 创建缺少必要字段的对账批次返回错误 |  | BDD-GM-FS-DOMAIN-0023 | TODO |
| 315 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 51 | 创建无效对账类型返回错误 |  | BDD-GM-FS-DOMAIN-0024 | TODO |
| 316 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 63 | 创建未来日期的对账批次返回错误 |  | BDD-GM-FS-DOMAIN-0025 | TODO |
| 317 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 77 | UT-FS-R-003: 待处理状态可以转换到处理中 |  | BDD-GM-FS-DOMAIN-0026 | TODO |
| 318 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 87 | 已完成状态不能再次开始处理 |  | BDD-GM-FS-DOMAIN-0027 | TODO |
| 319 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 94 | 处理中状态不能再次开始处理 |  | BDD-GM-FS-DOMAIN-0028 | TODO |
| 320 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 101 | UT-FS-R-004: 处理中状态可以转换到已完成 |  | BDD-GM-FS-DOMAIN-0029 | TODO |
| 321 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 125 | UT-FS-R-007: 处理中状态可以转换到失败 |  | BDD-GM-FS-DOMAIN-0030 | TODO |
| 322 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 137 | 待处理状态不能直接完成 |  | BDD-GM-FS-DOMAIN-0031 | TODO |
| 323 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 148 | UT-FS-R-005: 标记差异更新统计信息 |  | BDD-GM-FS-DOMAIN-0032 | TODO |
| 324 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 169 | 只能在处理中状态标记差异 |  | BDD-GM-FS-DOMAIN-0033 | TODO |
| 325 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 182 | UT-FS-R-006: 解决差异更新统计信息 |  | BDD-GM-FS-DOMAIN-0034 | TODO |
| 326 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 222 | 更新统计信息 |  | BDD-GM-FS-DOMAIN-0035 | TODO |
| 327 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 244 | 统计数据必须一致 |  | BDD-GM-FS-DOMAIN-0036 | TODO |
| 328 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 261 | 对账批次号必须唯一 |  | BDD-GM-FS-DOMAIN-0037 | TODO |
| 329 | 结算 | domain | `test/financial_settlement/domain/reconciliation_test.exs` | 278 | 完成的对账批次不能修改 |  | BDD-GM-FS-DOMAIN-0038 | TODO |
| 330 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 32 | RD-001: 有效数据创建成功 |  | BDD-GM-FS-DOMAIN-0039 | TODO |
| 331 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 42 | RD-002: 扣除金额计算正确 |  | BDD-GM-FS-DOMAIN-0040 | TODO |
| 332 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 55 | RD-003: 退款金额计算正确 |  | BDD-GM-FS-DOMAIN-0041 | TODO |
| 333 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 68 | RD-004: 缺少 supplier_id 返回错误 |  | BDD-GM-FS-DOMAIN-0042 | TODO |
| 334 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 73 | RD-005: 空 items 列表返回错误 |  | BDD-GM-FS-DOMAIN-0043 | TODO |
| 335 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 78 | 缺少 order_id 返回错误 |  | BDD-GM-FS-DOMAIN-0044 | TODO |
| 336 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 83 | 缺少 refund_id 返回错误 |  | BDD-GM-FS-DOMAIN-0045 | TODO |
| 337 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 88 | 支持已创建的 RefundDeductionItem |  | BDD-GM-FS-DOMAIN-0046 | TODO |
| 338 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 98 | RD-006: pending → deducted 成功 |  | BDD-GM-FS-DOMAIN-0047 | TODO |
| 339 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 108 | RD-008: deducted → deducted 返回错误 |  | BDD-GM-FS-DOMAIN-0048 | TODO |
| 340 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 115 | RD-009: cancelled → deducted 返回错误 |  | BDD-GM-FS-DOMAIN-0049 | TODO |
| 341 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 124 | RD-007: pending → cancelled 成功 |  | BDD-GM-FS-DOMAIN-0050 | TODO |
| 342 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 132 | RD-010: deducted → cancelled 返回错误 |  | BDD-GM-FS-DOMAIN-0051 | TODO |
| 343 | 结算 | domain | `test/financial_settlement/domain/refund_deduction_test.exs` | 139 | 已取消的不能再取消 |  | BDD-GM-FS-DOMAIN-0052 | TODO |
| 344 | 结算 | domain | `test/financial_settlement/domain/settlement_detail_test.exs` | 9 | 创建有效结算明细成功 |  | BDD-GM-FS-DOMAIN-0053 | TODO |
| 345 | 结算 | domain | `test/financial_settlement/domain/settlement_detail_test.exs` | 29 | 创建缺少必要字段的结算明细返回错误 |  | BDD-GM-FS-DOMAIN-0054 | TODO |
| 346 | 结算 | domain | `test/financial_settlement/domain/settlement_detail_test.exs` | 38 | 包含服务费的结算明细 |  | BDD-GM-FS-DOMAIN-0055 | TODO |
| 347 | 结算 | domain | `test/financial_settlement/domain/settlement_detail_test.exs` | 55 | 包含成本字段的结算明细（用于报表分析） |  | BDD-GM-FS-DOMAIN-0056 | TODO |
| 348 | 结算 | domain | `test/financial_settlement/domain/settlement_detail_test.exs` | 71 | 退款状态的结算明细 |  | BDD-GM-FS-DOMAIN-0057 | TODO |
| 349 | 结算 | domain | `test/financial_settlement/domain/settlement_detail_test.exs` | 89 | 计算净额 - 基本场景 |  | BDD-GM-FS-DOMAIN-0058 | TODO |
| 350 | 结算 | domain | `test/financial_settlement/domain/settlement_detail_test.exs` | 98 | 计算净额 - 高佣金场景 |  | BDD-GM-FS-DOMAIN-0059 | TODO |
| 351 | 结算 | domain | `test/financial_settlement/domain/settlement_detail_test.exs` | 109 | 验证成本不能为负数 |  | BDD-GM-FS-DOMAIN-0060 | TODO |
| 352 | 结算 | domain | `test/financial_settlement/domain/settlement_detail_test.exs` | 124 | 验证佣金不能超过销售额（正常订单） |  | BDD-GM-FS-DOMAIN-0061 | TODO |
| 353 | 结算 | domain | `test/financial_settlement/domain/settlement_detail_test.exs` | 138 | 退款订单允许佣金为0 |  | BDD-GM-FS-DOMAIN-0062 | TODO |
| 354 | 结算 | domain | `test/financial_settlement/domain/settlement_detail_test.exs` | 151 | 验证净额计算的一致性 |  | BDD-GM-FS-DOMAIN-0063 | TODO |
| 355 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 10 | UT-FS-S-001: 创建有效结算单成功 |  | BDD-GM-FS-DOMAIN-0064 | TODO |
| 356 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 49 | 创建缺少必要字段的结算单返回错误 |  | BDD-GM-FS-DOMAIN-0065 | TODO |
| 357 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 60 | 创建无效商户类型返回错误 |  | BDD-GM-FS-DOMAIN-0066 | TODO |
| 358 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 83 | UT-FS-S-002: 计算结算金额 |  | BDD-GM-FS-DOMAIN-0067 | TODO |
| 359 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 123 | 包含退款的结算金额计算 |  | BDD-GM-FS-DOMAIN-0068 | TODO |
| 360 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 171 | 基于明细重新计算结算单总额 |  | BDD-GM-FS-DOMAIN-0069 | TODO |
| 361 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 206 | 结算明细重新计算 |  | BDD-GM-FS-DOMAIN-0070 | TODO |
| 362 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 241 | UT-FS-S-003: 提交结算审核 |  | BDD-GM-FS-DOMAIN-0071 | TODO |
| 363 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 249 | UT-FS-S-004: 审核通过结算单 |  | BDD-GM-FS-DOMAIN-0072 | TODO |
| 364 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 260 | UT-FS-S-005: 审核拒绝结算单 |  | BDD-GM-FS-DOMAIN-0073 | TODO |
| 365 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 272 | UT-FS-S-006: 执行结算 |  | BDD-GM-FS-DOMAIN-0074 | TODO |
| 366 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 280 | UT-FS-S-007: 确认结算完成 |  | BDD-GM-FS-DOMAIN-0075 | TODO |
| 367 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 290 | UT-FS-S-008: 取消结算单 |  | BDD-GM-FS-DOMAIN-0076 | TODO |
| 368 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 300 | 审核中状态可以取消 |  | BDD-GM-FS-DOMAIN-0077 | TODO |
| 369 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 307 | 已结算状态不能取消 |  | BDD-GM-FS-DOMAIN-0078 | TODO |
| 370 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 314 | 已审核状态不能直接结算完成 |  | BDD-GM-FS-DOMAIN-0079 | TODO |
| 371 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 323 | 添加结算项 |  | BDD-GM-FS-DOMAIN-0080 | TODO |
| 372 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 339 | 批量添加结算项 |  | BDD-GM-FS-DOMAIN-0081 | TODO |
| 373 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 358 | 已审核的结算单不能添加结算项 |  | BDD-GM-FS-DOMAIN-0082 | TODO |
| 374 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 369 | 结算单号必须唯一 |  | BDD-GM-FS-DOMAIN-0083 | TODO |
| 375 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 394 | 结算周期结束日期不能早于开始日期 |  | BDD-GM-FS-DOMAIN-0084 | TODO |
| 376 | 结算 | domain | `test/financial_settlement/domain/settlement_test.exs` | 417 | 实际结算金额不能超过应结算金额 |  | BDD-GM-FS-DOMAIN-0085 | TODO |
| 377 | 结算 | event_handlers | `test/financial_settlement/event_handlers/refund_deduction_handler_test.exs` | 23 | EH-001: Handler 启动成功 |  | BDD-FIN-REFUND-DEDUCTION-HANDLER-001 | done |
| 378 | 结算 | event_handlers | `test/financial_settlement/event_handlers/refund_deduction_handler_test.exs` | 53 | EH-002: 收到非事件消息不崩溃 |  | BDD-FIN-REFUND-DEDUCTION-HANDLER-002 | done |
| 379 | 结算 | event_handlers | `test/financial_settlement/event_handlers/refund_deduction_handler_test.exs` | 62 | EH-003: 收到非 RefundSuccess 事件忽略 |  | BDD-FIN-REFUND-DEDUCTION-HANDLER-003 | done |
| 380 | 结算 | event_handlers | `test/financial_settlement/event_handlers/refund_deduction_handler_test.exs` | 70 | EH-004: Given 订单已结算 When 投递 RefundSuccess 事件 Then 创建扣除记录且 refund_id 幂等 | Y | BDD-FIN-REFUND-DEDUCTION-HANDLER-004 | done |
| 381 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 24 | 发布对账启动事件 |  | BDD-GM-FS-EVENTS-0001 | TODO |
| 382 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 44 | 发布对账完成事件 |  | BDD-GM-FS-EVENTS-0002 | TODO |
| 383 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 64 | 发布对账失败事件 |  | BDD-GM-FS-EVENTS-0003 | TODO |
| 384 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 82 | 发布结算单创建事件 |  | BDD-GM-FS-EVENTS-0004 | TODO |
| 385 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 108 | 发布结算单提交事件 |  | BDD-GM-FS-EVENTS-0005 | TODO |
| 386 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 125 | 发布结算单审批事件 |  | BDD-GM-FS-EVENTS-0006 | TODO |
| 387 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 143 | 发布结算单拒绝事件 |  | BDD-GM-FS-EVENTS-0007 | TODO |
| 388 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 161 | 发布结算支付事件 |  | BDD-GM-FS-EVENTS-0008 | TODO |
| 389 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 183 | 发布报表生成事件 |  | BDD-GM-FS-EVENTS-0009 | TODO |
| 390 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 203 | 发布报表审核事件 |  | BDD-GM-FS-EVENTS-0010 | TODO |
| 391 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 219 | 发布报表发布事件 |  | BDD-GM-FS-EVENTS-0011 | TODO |
| 392 | 结算 | events | `test/financial_settlement/events/event_publishing_test.exs` | 235 | 批量发布多个事件 |  | BDD-GM-FS-EVENTS-0012 | TODO |
| 393 | 结算 | infrastructure | `test/financial_settlement/infrastructure/adapters/supplier_query_adapter_test.exs` | 7 | 处理供应商不存在的情况 |  | BDD-GM-FS-INFRASTRUCTURE-0001 | TODO |
| 394 | 结算 | infrastructure | `test/financial_settlement/infrastructure/adapters/supplier_query_adapter_test.exs` | 14 | 返回供应商基本信息结构 |  | BDD-GM-FS-INFRASTRUCTURE-0002 | TODO |
| 395 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/refund_deduction_repository_test.exs` | 38 | REPO-001: 保存并读取数据完整一致 |  | BDD-GM-FS-INFRASTRUCTURE-0003 | TODO |
| 396 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/refund_deduction_repository_test.exs` | 51 | REPO-007: 查询不存在的ID返回错误 |  | BDD-GM-FS-INFRASTRUCTURE-0004 | TODO |
| 397 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/refund_deduction_repository_test.exs` | 55 | REPO-008: items JSONB 存取完整 |  | BDD-GM-FS-INFRASTRUCTURE-0005 | TODO |
| 398 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/refund_deduction_repository_test.exs` | 78 | REPO-009: refund_id 存在时返回 true |  | BDD-GM-FS-INFRASTRUCTURE-0006 | TODO |
| 399 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/refund_deduction_repository_test.exs` | 85 | REPO-010: refund_id 不存在时返回 false |  | BDD-GM-FS-INFRASTRUCTURE-0007 | TODO |
| 400 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/refund_deduction_repository_test.exs` | 91 | REPO-002: 只返回 pending 状态的记录 |  | BDD-GM-FS-INFRASTRUCTURE-0008 | TODO |
| 401 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/refund_deduction_repository_test.exs` | 115 | REPO-005: 正确统计待扣除总额 |  | BDD-GM-FS-INFRASTRUCTURE-0009 | TODO |
| 402 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/refund_deduction_repository_test.exs` | 133 | 无记录时返回 0 |  | BDD-GM-FS-INFRASTRUCTURE-0010 | TODO |
| 403 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/refund_deduction_repository_test.exs` | 140 | REPO-006: 批量标记为已扣除 |  | BDD-GM-FS-INFRASTRUCTURE-0011 | TODO |
| 404 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/refund_deduction_repository_test.exs` | 164 | 只更新 pending 状态的记录 |  | BDD-GM-FS-INFRASTRUCTURE-0012 | TODO |
| 405 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/refund_deduction_repository_test.exs` | 184 | 相同 refund_id 插入返回错误 |  | BDD-GM-FS-INFRASTRUCTURE-0013 | TODO |
| 406 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/settlement_line_repository_test.exs` | 41 | Given Adjustment 行缺少 reason_code When 写入结算行 Then 返回错误且不落库 | Y | BDD-GM-FS-INFRASTRUCTURE-0014 | TODO |
| 407 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/settlement_line_repository_test.exs` | 61 | Given 已存在 Normal 行 When 重复写入相同 order_item_id Then 返回错误且只保留一条 | Y | BDD-GM-FS-INFRASTRUCTURE-0015 | TODO |
| 408 | 结算 | infrastructure | `test/financial_settlement/infrastructure/repositories/settlement_line_repository_test.exs` | 95 | Given 已存在 RETURN Adjustment 行 When 重复写入相同 refund_deduction_id Then 返回错误且只保留一条 | Y | BDD-GM-FS-INFRASTRUCTURE-0016 | TODO |
| 409 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 116 | IT-FS-RPT-001: 生成月度财务汇总报表 |  | BDD-GM-FS-INTEGRATION-0001 | TODO |
| 410 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 145 | IT-FS-RPT-002: 生成供应商对账单报表 |  | BDD-GM-FS-INTEGRATION-0002 | TODO |
| 411 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 168 | IT-FS-RPT-003: 生成渠道对账汇总报表 |  | BDD-GM-FS-INTEGRATION-0003 | TODO |
| 412 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 193 | IT-FS-RPT-004: 审核通过并发布报表 |  | BDD-GM-FS-INTEGRATION-0004 | TODO |
| 413 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 230 | IT-FS-RPT-005: 批量审核多个报表 |  | BDD-GM-FS-INTEGRATION-0005 | TODO |
| 414 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 265 | IT-FS-RPT-006: 按类型查询报表列表 |  | BDD-GM-FS-INTEGRATION-0006 | TODO |
| 415 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 296 | IT-FS-RPT-007: 按时间范围查询报表 |  | BDD-GM-FS-INTEGRATION-0007 | TODO |
| 416 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 314 | IT-FS-RPT-008: 导出报表为PDF格式 |  | BDD-GM-FS-INTEGRATION-0008 | TODO |
| 417 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 350 | IT-FS-RPT-009: 报表生成发布领域事件 |  | BDD-GM-FS-INTEGRATION-0009 | TODO |
| 418 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 379 | IT-FS-RPT-010: 处理报表完整生命周期事件链 |  | BDD-GM-FS-INTEGRATION-0010 | TODO |
| 419 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 428 | IT-FS-RPT-011: 生成跨多供应商的综合报表 |  | BDD-GM-FS-INTEGRATION-0011 | TODO |
| 420 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 478 | IT-FS-RPT-012: 生成异常数据报表 |  | BDD-GM-FS-INTEGRATION-0012 | TODO |
| 421 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 525 | IT-FS-RPT-013: 验证报表访问权限控制 |  | BDD-GM-FS-INTEGRATION-0013 | TODO |
| 422 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 568 | IT-FS-RPT-014: 大数据量报表生成性能测试 |  | BDD-GM-FS-INTEGRATION-0014 | TODO |
| 423 | 结算 | integration | `test/financial_settlement/integration/financial_report_flow_test.exs` | 640 | IT-FS-RPT-015: 并发生成多个报表 |  | BDD-GM-FS-INTEGRATION-0015 | TODO |
| 424 | 结算 | integration | `test/financial_settlement/integration/reconciliation_flow_test.exs` | 83 | IT-FS-REC-001: 创建并执行日对账批次 - 完全匹配场景 |  | BDD-GM-FS-INTEGRATION-0016 | TODO |
| 425 | 结算 | integration | `test/financial_settlement/integration/reconciliation_flow_test.exs` | 110 | IT-FS-REC-002: 识别并处理对账差异 - 金额不匹配 |  | BDD-GM-FS-INTEGRATION-0017 | TODO |
| 426 | 结算 | integration | `test/financial_settlement/integration/reconciliation_flow_test.exs` | 131 | IT-FS-REC-003: 查询对账批次状态和进度 |  | BDD-GM-FS-INTEGRATION-0018 | TODO |
| 427 | 结算 | integration | `test/financial_settlement/integration/reconciliation_flow_test.exs` | 154 | IT-FS-REC-004: 分页查询对账批次列表 |  | BDD-GM-FS-INTEGRATION-0019 | TODO |
| 428 | 结算 | integration | `test/financial_settlement/integration/reconciliation_flow_test.exs` | 183 | IT-FS-REC-005: 生成并导出对账报告 |  | BDD-GM-FS-INTEGRATION-0020 | TODO |
| 429 | 结算 | integration | `test/financial_settlement/integration/reconciliation_flow_test.exs` | 201 | IT-FS-REC-006: 处理重复创建对账批次 |  | BDD-GM-FS-INTEGRATION-0021 | TODO |
| 430 | 结算 | integration | `test/financial_settlement/integration/reconciliation_flow_test.exs` | 231 | IT-FS-REC-007: 自动修复小额差异 |  | BDD-GM-FS-INTEGRATION-0022 | TODO |
| 431 | 结算 | integration | `test/financial_settlement/integration/reconciliation_flow_test.exs` | 277 | IT-FS-REC-008: 识别单边账交易 |  | BDD-GM-FS-INTEGRATION-0023 | TODO |
| 432 | 结算 | integration | `test/financial_settlement/integration/reconciliation_flow_test.exs` | 318 | IT-FS-REC-009: 对账流程发布领域事件 |  | BDD-GM-FS-INTEGRATION-0024 | TODO |
| 433 | 结算 | integration | `test/financial_settlement/integration/reconciliation_flow_test.exs` | 338 | IT-FS-REC-010: 并发创建对账批次 |  | BDD-GM-FS-INTEGRATION-0025 | TODO |
| 434 | 结算 | integration | `test/financial_settlement/integration/reconciliation_flow_test.exs` | 389 | IT-FS-REC-011: 大批量交易对账性能测试 |  | BDD-GM-FS-INTEGRATION-0026 | TODO |
| 435 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 245 | 成功创建供应商月度结算单 |  | BDD-GM-FS-INTEGRATION-0027 | TODO |
| 436 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 280 | 供应商不存在时创建失败 |  | BDD-GM-FS-INTEGRATION-0028 | TODO |
| 437 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 292 | 重复创建同期结算单失败 |  | BDD-GM-FS-INTEGRATION-0029 | TODO |
| 438 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 308 | 已作废 cancelled 的结算单允许重出同账期 |  | BDD-GM-FS-INTEGRATION-0030 | TODO |
| 439 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 331 | 审核通过即写入 locked_at/locked_by，且冻结后禁止删除 settlement_lines |  | BDD-GM-FS-INTEGRATION-0031 | TODO |
| 440 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 396 | Given 结算单已冻结 When 尝试修改金额/期间/as_of Then 保存返回 :settlement_locked（但允许推进状态） | Y | BDD-GM-FS-INTEGRATION-0032 | TODO |
| 441 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 437 | 审核通过后回写 orders.settled_at/settlement_id（以 Normal 快照行的订单为准） |  | BDD-GM-FS-INTEGRATION-0033 | TODO |
| 442 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 501 | Given 结算单被拒绝 When 审核 reject Then 不回写订单已结算标记 | Y | BDD-GM-FS-INTEGRATION-0034 | TODO |
| 443 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 587 | 成功审批结算单 |  | BDD-GM-FS-INTEGRATION-0035 | TODO |
| 444 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 615 | 拒绝结算单 |  | BDD-GM-FS-INTEGRATION-0036 | TODO |
| 445 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 643 | 已审批的结算单不能重复审批 |  | BDD-GM-FS-INTEGRATION-0037 | TODO |
| 446 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 693 | 成功执行支付 |  | BDD-GM-FS-INTEGRATION-0038 | TODO |
| 447 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 720 | 未审批的结算单不能支付 |  | BDD-GM-FS-INTEGRATION-0039 | TODO |
| 448 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 796 | 支付失败后可以重试 |  | BDD-GM-FS-INTEGRATION-0040 | TODO |
| 449 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 822 | 并发创建多个供应商的结算单 |  | BDD-GM-FS-INTEGRATION-0041 | TODO |
| 450 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 858 | 同一供应商同账期只允许创建一张结算单（DB 唯一约束） |  | BDD-GM-FS-INTEGRATION-0042 | TODO |
| 451 | 结算 | integration | `test/financial_settlement/integration/settlement_flow_test.exs` | 919 | 批量创建大量结算单的性能 |  | BDD-GM-FS-INTEGRATION-0043 | TODO |
| 452 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_after_sales_e2e_test.exs` | 107 | Given 售后通过应用服务提交并完成退款 When 当期出单 Then Normal 行按剩余数量入账且不出 Adjustment | Y | BDD-SETTLEMENT-ROUTE-B-AFTERSALES-001 | done |
| 453 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 129 | Given 当期 Normal 很小但跨期扣除很大 When 出单并审核通过 Then 负净额可持久化且流程正常 | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-001 | done |
| 454 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 279 | Given 订单项发生时间在 as_of 边界 When 出单 Then <= as_of 纳入 Normal 且 > as_of 不纳入 | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-002 | done |
| 455 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 369 | Given 订单不满足结算口径（状态不在 paid/shipped/completed）When 出单 Then 不生成 Normal 行 | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-003 | done |
| 456 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 421 | Given 已结算订单发生退款 When 下期出单 Then 生成 Adjustment 行并推进 refund_deductions 状态 | Y | BDD-SETTLEMENT-ROUTE-B-ADJ-001 | done |
| 457 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 591 | Given 账期起止颠倒 When 出单 Then 返回 :invalid_params 且不落库 | Y | BDD-SETTLEMENT-NEG-001 | done |
| 458 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 611 | Given 同一订单项发生多次部分退款 When 下期出单 Then 多条 Adjustment 可追溯且总额可解释 | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-004 | done |
| 459 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 761 | Given dev 中存在非 UUID supplier_id 的订单 When 按 UUID 供应商出单 Then 该订单被跳过且结算成功 | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-005 | done |
| 460 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 817 | Given 订单 supplier_id 非 UUID 但商品 supplier_id 为 UUID When 按 UUID 出单 Then 仍跳过（策略：严格以订单 supplier_id 为准） | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-006 | done |
| 461 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 893 | Given 出单前售后退款已完成 When 出单 Then 当期同一期吸收（不出 Normal 也不出 Adjustment） | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-007 | done |
| 462 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 990 | Given 出单前部分退货（按订单项数量）When 出单 Then Normal 行按剩余数量入账且不出 Adjustment | Y | BDD-SETTLEMENT-ROUTE-B-AFTERSALES-001 | done |
| 463 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 1113 | Given 出单前部分退 + 结算锁定后再次退款 When 跨两期出单 Then 当期按剩余数量入账且下期生成 Adjustment 并推进扣除状态 | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-008 | done |
| 464 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 1299 | Given pending 扣除缺 origin When 出单 Then 返回 :adjustment_missing_origin 且不落库 | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-009 | done |
| 465 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 1345 | Given 超额冲减 When 下期出单 Then 返回 :deduction_exceeds_origin 且不落库 | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-010 | done |
| 466 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 1449 | Given 对账 diff != 0 When 执行结算 Then 阻断并返回 :reconciliation_diff_not_zero | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-011 | done |
| 467 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_flow_test.exs` | 1500 | Given Adjustment 转写临时失败 When 重试出单 Then 不留脏数据且最终成功 | Y | BDD-SETTLEMENT-ROUTE-B-FLOW-012 | done |
| 468 | 结算 | integration | `test/financial_settlement/integration/settlement_route_b_recon_block_test.exs` | 73 | Given settlement_lines 与结算头净额不一致 When execute_settlement Then 返回 :reconciliation_diff_not_zero | Y | BDD-SETTLEMENT-ROUTE-B-RECON-001 | done |
| 469 | 结算 | services | `test/financial_settlement/services/profit_calculation_service_test.exs` | 7 | UT-FS-PS-001: 计算单笔订单利润 - A档价格 |  | BDD-GM-FS-SERVICES-0001 | TODO |
| 470 | 结算 | services | `test/financial_settlement/services/profit_calculation_service_test.exs` | 28 | UT-FS-PS-002: 批量计算多笔订单利润 |  | BDD-GM-FS-SERVICES-0002 | TODO |
| 471 | 结算 | services | `test/financial_settlement/services/profit_calculation_service_test.exs` | 62 | 计算包含多个商品的订单利润 |  | BDD-GM-FS-SERVICES-0003 | TODO |
| 472 | 结算 | services | `test/financial_settlement/services/profit_calculation_service_test.exs` | 90 | UT-FS-PS-003: 计算供应商周期利润汇总 |  | BDD-GM-FS-SERVICES-0004 | TODO |
| 473 | 结算 | services | `test/financial_settlement/services/profit_calculation_service_test.exs` | 124 | UT-FS-PS-004: 计算商品类别利润 |  | BDD-GM-FS-SERVICES-0005 | TODO |
| 474 | 结算 | services | `test/financial_settlement/services/profit_calculation_service_test.exs` | 137 | UT-FS-PS-005: 处理退款对利润的影响 |  | BDD-GM-FS-SERVICES-0006 | TODO |
| 475 | 结算 | services | `test/financial_settlement/services/profit_calculation_service_test.exs` | 157 | 全额退款导致负利润 |  | BDD-GM-FS-SERVICES-0007 | TODO |
| 476 | 结算 | services | `test/financial_settlement/services/profit_calculation_service_test.exs` | 179 | UT-FS-PS-006: 计算净利润率 |  | BDD-GM-FS-SERVICES-0008 | TODO |
| 477 | 结算 | services | `test/financial_settlement/services/profit_calculation_service_test.exs` | 198 | 佣金和服务费超过毛利润导致净亏损 |  | BDD-GM-FS-SERVICES-0009 | TODO |
| 478 | 结算 | services | `test/financial_settlement/services/profit_calculation_service_test.exs` | 216 | 获取单个商品成本 |  | BDD-GM-FS-SERVICES-0010 | TODO |
| 479 | 结算 | services | `test/financial_settlement/services/profit_calculation_service_test.exs` | 227 | 获取不存在的商品返回错误 |  | BDD-GM-FS-SERVICES-0011 | TODO |
| 480 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 46 | 从支付渠道获取交易数据 |  | BDD-GM-FS-SERVICES-0012 | TODO |
| 481 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 78 | 获取我方订单数据 |  | BDD-GM-FS-SERVICES-0013 | TODO |
| 482 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 110 | 成功匹配所有交易 |  | BDD-GM-FS-SERVICES-0014 | TODO |
| 483 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 149 | 识别金额不匹配的交易 |  | BDD-GM-FS-SERVICES-0015 | TODO |
| 484 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 182 | 识别单边账 |  | BDD-GM-FS-SERVICES-0016 | TODO |
| 485 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 218 | 从匹配结果中识别差异 |  | BDD-GM-FS-SERVICES-0017 | TODO |
| 486 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 255 | 完整的对账流程 |  | BDD-GM-FS-SERVICES-0018 | TODO |
| 487 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 276 | 对账失败的处理 |  | BDD-GM-FS-SERVICES-0019 | TODO |
| 488 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 296 | 生成成功的对账报告 |  | BDD-GM-FS-SERVICES-0020 | TODO |
| 489 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 308 | 包含差异详情的报告 |  | BDD-GM-FS-SERVICES-0021 | TODO |
| 490 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 344 | 重试失败的对账批次 |  | BDD-GM-FS-SERVICES-0022 | TODO |
| 491 | 结算 | services | `test/financial_settlement/services/reconciliation_service_test.exs` | 362 | 非失败状态不能重试 |  | BDD-GM-FS-SERVICES-0023 | TODO |
| 492 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 39 | SVC-001: 正确计算待扣除总额 |  | BDD-GM-FS-SERVICES-0024 | TODO |
| 493 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 57 | SVC-002: 无记录时返回0 |  | BDD-GM-FS-SERVICES-0025 | TODO |
| 494 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 67 | Given 订单未结算 When 退款成功调用 create_deduction_if_needed Then 返回 :not_needed 且不创建扣除记录 | Y | BDD-GM-FS-SERVICES-0026 | TODO |
| 495 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 123 | Given 订单已结算 When 退款成功调用 create_deduction_if_needed Then 创建扣除记录且 refund_id 幂等 | Y | BDD-GM-FS-SERVICES-0027 | TODO |
| 496 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 207 | SVC-003: 批量执行扣除 |  | BDD-GM-FS-SERVICES-0028 | TODO |
| 497 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 227 | SVC-004: 无待扣除记录时返回0 |  | BDD-GM-FS-SERVICES-0029 | TODO |
| 498 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 236 | SVC-005: 只扣除该供应商的记录 |  | BDD-GM-FS-SERVICES-0030 | TODO |
| 499 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 260 | SVC-010: 只返回 pending 的记录 |  | BDD-GM-FS-SERVICES-0031 | TODO |
| 500 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 279 | SVC-006: 计算单个商品扣除金额 |  | BDD-GM-FS-SERVICES-0032 | TODO |
| 501 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 288 | SVC-007: 计算多个商品扣除金额 |  | BDD-GM-FS-SERVICES-0033 | TODO |
| 502 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 299 | SVC-008: 支持字符串键 |  | BDD-GM-FS-SERVICES-0034 | TODO |
| 503 | 结算 | services | `test/financial_settlement/services/refund_deduction_service_test.exs` | 308 | SVC-009: 空列表返回0 |  | BDD-GM-FS-SERVICES-0035 | TODO |
| 504 | 结算 | services | `test/financial_settlement/services/settlement_recon_bridge_test.exs` | 36 | Given 结算单存在 Normal/Adjustment 行 When 查询对账汇总 Then 返回 computed_net 且 diff 可用于验收 | Y | BDD-GM-FS-SERVICES-0036 | TODO |
| 505 | 结算 | services | `test/financial_settlement/services/settlement_recon_bridge_test.exs` | 78 | Given settlement_id 不存在 When 查询对账汇总 Then 返回 :not_found | Y | BDD-GM-FS-SERVICES-0037 | TODO |
| 506 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 25 | 根据订单计算结算金额 |  | BDD-GM-FS-SERVICES-0038 | TODO |
| 507 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 76 | 扣除退款金额 |  | BDD-GM-FS-SERVICES-0039 | TODO |
| 508 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 104 | 验证最小结算金额 |  | BDD-GM-FS-SERVICES-0040 | TODO |
| 509 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 123 | 验证最大结算金额 |  | BDD-GM-FS-SERVICES-0041 | TODO |
| 510 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 142 | 验证通过 |  | BDD-GM-FS-SERVICES-0042 | TODO |
| 511 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 163 | 应用百分比佣金 |  | BDD-GM-FS-SERVICES-0043 | TODO |
| 512 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 184 | 应用阶梯佣金 |  | BDD-GM-FS-SERVICES-0044 | TODO |
| 513 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 210 | 创建新的结算单 |  | BDD-GM-FS-SERVICES-0045 | TODO |
| 514 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 239 | 结算单号必须唯一 |  | BDD-GM-FS-SERVICES-0046 | TODO |
| 515 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 247 | 提交审核 |  | BDD-GM-FS-SERVICES-0047 | TODO |
| 516 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 255 | 审核通过 |  | BDD-GM-FS-SERVICES-0048 | TODO |
| 517 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 270 | 审核拒绝 |  | BDD-GM-FS-SERVICES-0049 | TODO |
| 518 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 287 | 执行银行转账结算 |  | BDD-GM-FS-SERVICES-0050 | TODO |
| 519 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 309 | 处理结算执行失败 |  | BDD-GM-FS-SERVICES-0051 | TODO |
| 520 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 328 | 确认结算完成 |  | BDD-GM-FS-SERVICES-0052 | TODO |
| 521 | 结算 | services | `test/financial_settlement/services/settlement_service_test.exs` | 345 | 实际金额与应结算金额不符 |  | BDD-GM-FS-SERVICES-0053 | TODO |
| 522 | 结算 | unit | `test/financial_settlement/unit/supplier_bank_account_integration_test.exs` | 73 | 使用供应商银行账户创建结算单 |  | BDD-GM-FS-UNIT-0001 | TODO |
| 523 | 结算 | unit | `test/financial_settlement/unit/supplier_bank_account_integration_test.exs` | 108 | 供应商无银行账户时使用默认值 |  | BDD-GM-FS-UNIT-0002 | TODO |
| 524 | 结算 | unit | `test/financial_settlement/unit/supplier_bank_account_integration_test.exs` | 136 | 适配器正确转换供应商信息 |  | BDD-GM-FS-UNIT-0003 | TODO |
| 525 | 结算 | unit | `test/financial_settlement/unit/supplier_bank_account_integration_test.exs` | 150 | 适配器处理无银行账户的供应商 |  | BDD-GM-FS-UNIT-0004 | TODO |
| 526 | 结算 | unit | `test/financial_settlement/unit/supplier_bank_account_integration_test.exs` | 161 | 适配器处理供应商不存在的情况 |  | BDD-GM-FS-UNIT-0005 | TODO |
| 527 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 7 | 创建有效的对公账户 |  | BDD-GM-FS-VALUE_OBJECTS-0001 | TODO |
| 528 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 29 | 创建有效的对私账户 |  | BDD-GM-FS-VALUE_OBJECTS-0002 | TODO |
| 529 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 45 | 缺少必要字段返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0003 | TODO |
| 530 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 56 | 无效的账户类型返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0004 | TODO |
| 531 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 70 | 验证有效的账户信息 |  | BDD-GM-FS-VALUE_OBJECTS-0005 | TODO |
| 532 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 76 | 账户号码格式验证 |  | BDD-GM-FS-VALUE_OBJECTS-0006 | TODO |
| 533 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 86 | 银行代码格式验证 |  | BDD-GM-FS-VALUE_OBJECTS-0007 | TODO |
| 534 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 92 | 货币代码验证 |  | BDD-GM-FS-VALUE_OBJECTS-0008 | TODO |
| 535 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 100 | 对账户号码进行脱敏处理 |  | BDD-GM-FS-VALUE_OBJECTS-0009 | TODO |
| 536 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 113 | 短账号的脱敏处理 |  | BDD-GM-FS-VALUE_OBJECTS-0010 | TODO |
| 537 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 132 | 相同账户判断 |  | BDD-GM-FS-VALUE_OBJECTS-0011 | TODO |
| 538 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 139 | 不同账户判断 |  | BDD-GM-FS-VALUE_OBJECTS-0012 | TODO |
| 539 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 154 | 同一银行不同账号判断 |  | BDD-GM-FS-VALUE_OBJECTS-0013 | TODO |
| 540 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 166 | 格式化账户信息为字符串 |  | BDD-GM-FS-VALUE_OBJECTS-0014 | TODO |
| 541 | 结算 | value_objects | `test/financial_settlement/value_objects/bank_account_test.exs` | 174 | 格式化没有支行的账户 |  | BDD-GM-FS-VALUE_OBJECTS-0015 | TODO |
| 542 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 8 | 创建有效的销售佣金 |  | BDD-GM-FS-VALUE_OBJECTS-0016 | TODO |
| 543 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 23 | 创建有效的推广佣金 |  | BDD-GM-FS-VALUE_OBJECTS-0017 | TODO |
| 544 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 36 | 创建固定金额佣金 |  | BDD-GM-FS-VALUE_OBJECTS-0018 | TODO |
| 545 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 49 | 缺少必要字段返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0019 | TODO |
| 546 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 60 | 无效的佣金类型返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0020 | TODO |
| 547 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 72 | 根据费率计算佣金 |  | BDD-GM-FS-VALUE_OBJECTS-0021 | TODO |
| 548 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 81 | 处理零费率 |  | BDD-GM-FS-VALUE_OBJECTS-0022 | TODO |
| 549 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 90 | 重新计算已有佣金 |  | BDD-GM-FS-VALUE_OBJECTS-0023 | TODO |
| 550 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 107 | 验证有效的佣金 |  | BDD-GM-FS-VALUE_OBJECTS-0024 | TODO |
| 551 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 118 | 负数基础金额无效 |  | BDD-GM-FS-VALUE_OBJECTS-0025 | TODO |
| 552 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 128 | 负数费率无效 |  | BDD-GM-FS-VALUE_OBJECTS-0026 | TODO |
| 553 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 138 | 费率超过100%无效 |  | BDD-GM-FS-VALUE_OBJECTS-0027 | TODO |
| 554 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 151 | 汇总多个佣金 |  | BDD-GM-FS-VALUE_OBJECTS-0028 | TODO |
| 555 | 结算 | value_objects | `test/financial_settlement/value_objects/commission_test.exs` | 163 | 按类型分组汇总 |  | BDD-GM-FS-VALUE_OBJECTS-0029 | TODO |
| 556 | 结算 | value_objects | `test/financial_settlement/value_objects/reconciliation_period_test.exs` | 7 | 创建有效的日周期 |  | BDD-GM-FS-VALUE_OBJECTS-0030 | TODO |
| 557 | 结算 | value_objects | `test/financial_settlement/value_objects/reconciliation_period_test.exs` | 18 | 创建有效的月周期 |  | BDD-GM-FS-VALUE_OBJECTS-0031 | TODO |
| 558 | 结算 | value_objects | `test/financial_settlement/value_objects/reconciliation_period_test.exs` | 29 | 结束日期早于开始日期返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0032 | TODO |
| 559 | 结算 | value_objects | `test/financial_settlement/value_objects/reconciliation_period_test.exs` | 36 | 无效的周期类型返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0033 | TODO |
| 560 | 结算 | value_objects | `test/financial_settlement/value_objects/reconciliation_period_test.exs` | 46 | UT-FS-V-005: 判断日期是否在对账周期内 |  | BDD-GM-FS-VALUE_OBJECTS-0034 | TODO |
| 561 | 结算 | value_objects | `test/financial_settlement/value_objects/reconciliation_period_test.exs` | 63 | 获取周期天数 |  | BDD-GM-FS-VALUE_OBJECTS-0035 | TODO |
| 562 | 结算 | value_objects | `test/financial_settlement/value_objects/reconciliation_period_test.exs` | 74 | UT-FS-V-006: 获取下一对账周期 |  | BDD-GM-FS-VALUE_OBJECTS-0036 | TODO |
| 563 | 结算 | value_objects | `test/financial_settlement/value_objects/reconciliation_period_test.exs` | 95 | 获取上一对账周期 |  | BDD-GM-FS-VALUE_OBJECTS-0037 | TODO |
| 564 | 结算 | value_objects | `test/financial_settlement/value_objects/reconciliation_period_test.exs` | 111 | 日周期必须是同一天 |  | BDD-GM-FS-VALUE_OBJECTS-0038 | TODO |
| 565 | 结算 | value_objects | `test/financial_settlement/value_objects/reconciliation_period_test.exs` | 116 | 周周期必须是7天 |  | BDD-GM-FS-VALUE_OBJECTS-0039 | TODO |
| 566 | 结算 | value_objects | `test/financial_settlement/value_objects/reconciliation_period_test.exs` | 121 | 月周期必须是完整月份 |  | BDD-GM-FS-VALUE_OBJECTS-0040 | TODO |
| 567 | 结算 | value_objects | `test/financial_settlement/value_objects/refund_deduction_item_test.exs` | 7 | VO-001: 有效数据创建成功 |  | BDD-GM-FS-VALUE_OBJECTS-0041 | TODO |
| 568 | 结算 | value_objects | `test/financial_settlement/value_objects/refund_deduction_item_test.exs` | 24 | VO-002: 数量为0时返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0042 | TODO |
| 569 | 结算 | value_objects | `test/financial_settlement/value_objects/refund_deduction_item_test.exs` | 36 | VO-003: 负数价格时返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0043 | TODO |
| 570 | 结算 | value_objects | `test/financial_settlement/value_objects/refund_deduction_item_test.exs` | 48 | VO-004: 供货价大于售价时允许创建（可能是促销） |  | BDD-GM-FS-VALUE_OBJECTS-0044 | TODO |
| 571 | 结算 | value_objects | `test/financial_settlement/value_objects/refund_deduction_item_test.exs` | 62 | VO-005: 金额计算精度 |  | BDD-GM-FS-VALUE_OBJECTS-0045 | TODO |
| 572 | 结算 | value_objects | `test/financial_settlement/value_objects/refund_deduction_item_test.exs` | 75 | 数量为nil时返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0046 | TODO |
| 573 | 结算 | value_objects | `test/financial_settlement/value_objects/refund_deduction_item_test.exs` | 87 | 支持数字类型的价格 |  | BDD-GM-FS-VALUE_OBJECTS-0047 | TODO |
| 574 | 结算 | value_objects | `test/financial_settlement/value_objects/report_detail_test.exs` | 7 | 创建有效的报表明细成功 |  | BDD-GM-FS-VALUE_OBJECTS-0048 | TODO |
| 575 | 结算 | value_objects | `test/financial_settlement/value_objects/report_detail_test.exs` | 35 | 使用默认值创建明细 |  | BDD-GM-FS-VALUE_OBJECTS-0049 | TODO |
| 576 | 结算 | value_objects | `test/financial_settlement/value_objects/report_detail_test.exs` | 52 | 创建退款类型明细允许负金额 |  | BDD-GM-FS-VALUE_OBJECTS-0050 | TODO |
| 577 | 结算 | value_objects | `test/financial_settlement/value_objects/report_detail_test.exs` | 64 | 非退款类型不允许负金额 |  | BDD-GM-FS-VALUE_OBJECTS-0051 | TODO |
| 578 | 结算 | value_objects | `test/financial_settlement/value_objects/report_detail_test.exs` | 75 | 无效的项目类型返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0052 | TODO |
| 579 | 结算 | value_objects | `test/financial_settlement/value_objects/report_detail_test.exs` | 88 | 按项目类型分组明细 |  | BDD-GM-FS-VALUE_OBJECTS-0053 | TODO |
| 580 | 结算 | value_objects | `test/financial_settlement/value_objects/report_detail_test.exs` | 134 | 汇总多个明细的金额 |  | BDD-GM-FS-VALUE_OBJECTS-0054 | TODO |
| 581 | 结算 | value_objects | `test/financial_settlement/value_objects/report_detail_test.exs` | 169 | 汇总空列表返回零值 |  | BDD-GM-FS-VALUE_OBJECTS-0055 | TODO |
| 582 | 结算 | value_objects | `test/financial_settlement/value_objects/report_summary_test.exs` | 7 | 创建包含利润数据的报表汇总成功 |  | BDD-GM-FS-VALUE_OBJECTS-0056 | TODO |
| 583 | 结算 | value_objects | `test/financial_settlement/value_objects/report_summary_test.exs` | 33 | 使用默认值创建汇总 |  | BDD-GM-FS-VALUE_OBJECTS-0057 | TODO |
| 584 | 结算 | value_objects | `test/financial_settlement/value_objects/report_summary_test.exs` | 51 | 验证负数交易笔数返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0058 | TODO |
| 585 | 结算 | value_objects | `test/financial_settlement/value_objects/report_summary_test.exs` | 64 | 计算毛利率 |  | BDD-GM-FS-VALUE_OBJECTS-0059 | TODO |
| 586 | 结算 | value_objects | `test/financial_settlement/value_objects/report_summary_test.exs` | 76 | 计算净利率 |  | BDD-GM-FS-VALUE_OBJECTS-0060 | TODO |
| 587 | 结算 | value_objects | `test/financial_settlement/value_objects/report_summary_test.exs` | 87 | 销售额为0时利润率为0 |  | BDD-GM-FS-VALUE_OBJECTS-0061 | TODO |
| 588 | 结算 | value_objects | `test/financial_settlement/value_objects/report_summary_test.exs` | 101 | 合并多个汇总数据 |  | BDD-GM-FS-VALUE_OBJECTS-0062 | TODO |
| 589 | 结算 | value_objects | `test/financial_settlement/value_objects/report_summary_test.exs` | 147 | 合并空列表返回空汇总 |  | BDD-GM-FS-VALUE_OBJECTS-0063 | TODO |
| 590 | 结算 | value_objects | `test/financial_settlement/value_objects/report_summary_test.exs` | 158 | 创建空汇总 |  | BDD-GM-FS-VALUE_OBJECTS-0064 | TODO |
| 591 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 8 | 创建T+N结算规则 |  | BDD-GM-FS-VALUE_OBJECTS-0065 | TODO |
| 592 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 24 | 创建固定日期结算规则 |  | BDD-GM-FS-VALUE_OBJECTS-0066 | TODO |
| 593 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 38 | 创建百分比佣金规则 |  | BDD-GM-FS-VALUE_OBJECTS-0067 | TODO |
| 594 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 50 | 创建阶梯佣金规则 |  | BDD-GM-FS-VALUE_OBJECTS-0068 | TODO |
| 595 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 66 | 缺少必要字段返回错误 |  | BDD-GM-FS-VALUE_OBJECTS-0069 | TODO |
| 596 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 77 | UT-FS-V-003: 计算T+1结算日期 |  | BDD-GM-FS-VALUE_OBJECTS-0070 | TODO |
| 597 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 90 | UT-FS-V-004: 计算T+7结算日期 |  | BDD-GM-FS-VALUE_OBJECTS-0071 | TODO |
| 598 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 103 | 固定日期结算 - 选择下一个结算日 |  | BDD-GM-FS-VALUE_OBJECTS-0072 | TODO |
| 599 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 128 | UT-FS-V-007: 计算固定金额佣金 |  | BDD-GM-FS-VALUE_OBJECTS-0073 | TODO |
| 600 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 143 | UT-FS-V-008: 计算百分比佣金 |  | BDD-GM-FS-VALUE_OBJECTS-0074 | TODO |
| 601 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 164 | 计算阶梯佣金 |  | BDD-GM-FS-VALUE_OBJECTS-0075 | TODO |
| 602 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 196 | 最小金额验证 |  | BDD-GM-FS-VALUE_OBJECTS-0076 | TODO |
| 603 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 215 | 最大金额验证 |  | BDD-GM-FS-VALUE_OBJECTS-0077 | TODO |
| 604 | 结算 | value_objects | `test/financial_settlement/value_objects/settlement_rule_test.exs` | 236 | 根据优先级排序规则 |  | BDD-GM-FS-VALUE_OBJECTS-0078 | TODO |
