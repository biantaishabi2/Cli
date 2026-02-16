# CLI 接口规范（统一版）

## 全局参数

```bash
--repo <name>     # 指定仓库（所有命令必需）
--db <path>       # 自定义数据库路径（可选，默认 ~/.bcc/index/<repo>.db）
```

## 命令规范

### build

```bash
bcc graph-index build \
  --repo nanobot \
  --input ast.json \
  --commit abc123
```

### query（精确查询）

```bash
# 统一用 --id，不用 positional
bcc graph-index query --repo nanobot --id "order.php#create#42"
bcc graph-index query --repo nanobot --name "create"
bcc graph-index query --repo nanobot --module "order"
```

### search（图遍历搜索）

```bash
bcc graph-index search \
  --repo nanobot \
  --id "order.php#create#42" \
  --depth 2 \
  --include callers,callees,same-file,same-module
```

### validate-arch

```bash
bcc graph-index validate-arch \
  --repo nanobot \
  --target target-matrix.yaml \
  --output violations.json
```

## 错误码规范

| 退出码 | 错误类型 | 说明 |
|-------|---------|------|
| 0 | Success | 成功 |
| 1 | RepoNotFound | 仓库不存在 |
| 2 | SqliteCorrupted | 数据库损坏 |
| 3 | InheritanceBroken | 继承链断裂（父类未索引）|
| 4 | CircularInheritance | 循环继承 |
| 5 | DepthLimitExceeded | 搜索深度超限 |
| 6 | ArchViolation | 架构违规（validate-arch 发现）|
| 10 | InvalidArgs | 参数错误 |
