# Trace2Contract 产物说明（映射实验）

本目录保存“架构规则 + AST 事实”模块映射实验的机器产物。

统一入口：
- 实验说明与复现步骤：`../../trace2contract-pilot-repro.md`

文件说明：
- `input-lock.json`：输入快照锁（git commit、seed/ast/source 文件哈希）
- `module_registry.json`：由 seed 编译后的模块规则
- `module_map.json`：文件到模块的全量映射结果
- `unmapped_queue.json`：未映射或冲突文件队列
- `relation_matrix.expected.json`：期望模块关系
- `relation_matrix.actual.json`：AST 聚合出的实际模块关系
- `relation_matrix.diff.json`：关系差异（missing/unexpected/forbidden）
- `summary.md`：本次运行摘要
