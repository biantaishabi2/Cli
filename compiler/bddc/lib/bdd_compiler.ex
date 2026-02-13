defmodule BDDCompiler do
  @moduledoc """
  BDD DSL 编译器（独立 CLI）。

  目标：
  - 解析 DSL（`.dsl` + Markdown fenced code block）
  - 编译期校验（指令签名/参数类型/变量引用/结构约束）
  - 生成 ExUnit 测试源码（交由业务项目提供运行期指令实现）
  - 提供 lint 门禁能力（暴露不稳定/不规范的测试写法）

  入口请用 `BDDCompiler.CLI`（`escript`）。
  """
end

