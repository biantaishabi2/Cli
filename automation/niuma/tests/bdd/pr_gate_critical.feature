Feature: niuma PR gate 关键回归防假绿
  As a repository maintainer
  I want mergeability to depend on required critical regressions actually running and passing
  So smoke-only green or skipped critical regressions cannot bypass gate

  Scenario: 高风险文件变更且关键回归未运行
    Given PR 修改 dispatch/agent loop 等高风险文件
    And critical/full required jobs 未出现在 checks 列表中
    When 执行 niuma test gate
    Then gate 应失败
    And reason_code 应为 CRITICAL_REGRESSION_MISSING
    And missing_jobs 应包含缺失的关键回归任务

  Scenario: 低风险文档改动
    Given PR 仅修改文档或注释文件
    When 执行 niuma test gate
    Then gate 仅要求 smoke required jobs
    And 不应强制 full/critical required jobs

  Scenario: smoke 通过但 full 或 critical 跳过
    Given PR 命中高风险路径
    And smoke check 通过
    And full 或 critical required jobs 为 skipped/not_run
    When 执行 niuma test gate
    Then gate 应失败
    And reason_code 应为 INSUFFICIENT_COVERAGE_FOR_HIGH_RISK

  Scenario: 关键回归执行且通过
    Given PR 命中高风险路径
    And critical required jobs 全部执行且成功
    When 执行 niuma test gate
    Then gate 应通过
    And run_mode 应为 critical

  Scenario: 关键回归超时并重试后仍失败
    Given critical required jobs 返回 timeout
    And INFRA_RETRY_MAX 配置为 2
    When 执行 niuma test gate
    Then gate 应自动重试 2 次
    And 最终状态应阻塞
    And reason_code 应为 TIMEOUT_BLOCKED

  Scenario: 未定义 critical 清单时回退 full
    Given PR 命中高风险路径
    And 仓库中不存在 critical regressions 配置文件
    When 执行 niuma test gate
    Then gate 应回退为 full 模式
    And 日志应输出 critical 配置缺失告警
    And 质量基线不应低于 full required jobs
