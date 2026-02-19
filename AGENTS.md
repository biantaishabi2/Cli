# Agent Instructions

## Issue 驱动开发流程

本项目采用 **GitHub Issue 驱动开发**模式。

### 工作流

```
查看 issue → 分析需求 → 创建分支 → 开发 → 测试 → 提交 PR → 合并
```

### 常用命令

```bash
# 查看待处理 issue
gh issue list --state open

# 查看 issue 详情
gh issue view <number>

# 创建功能分支（二选一）
# 方式1：常规 - 从主分支切出
git checkout master && git checkout -b feat/<N>-<slug>
# 方式2：并行开发 - 使用 worktree
git worktree add ../Cli-feat-<N> -b feat/<N>-<slug> master

# 开发完成后提交
gh issue comment <number> --body "已实现，见 PR #X"

# 合并后清理 worktree
git worktree remove ../Cli-feat-<N>
```

### 并行开发（Worktree）

多个 issue 同时开发时，使用 `git worktree` 避免分支切换冲突：

- 每个 issue 一个独立工作目录，互不干扰
- 从 master 切出，不受其他未合并分支影响
- 共享同一个 git 仓库，commit/push 正常工作
- 完成合并后及时清理 worktree

### 会话结束检查清单

**工作未完成时**：
1. 创建 issue 记录剩余工作
2. 更新当前 issue 状态（添加进度评论）
3. `git push` 到远程分支

**工作完成时**：
1. 提交 PR 并关联 issue（`Closes #N`）
2. 确保 CI 通过
3. 合并到 master
4. `git pull --rebase && git push`

### 关键规则

- **每个功能对应一个 issue** - 便于追踪和讨论
- **Commit message 关联 issue** - `git commit -m "feat: xxx (#N)"`
- **PR 必须关联 issue** - PR body 中写 `Closes #N`
- **及时更新状态** - 避免 issue 长期处于 open 状态
- **bot 状态标签受控** - 禁止直接 `gh issue edit --add-label/--remove-label bot:*`
- **统一入口** - 迁移 `bot:*` 状态必须使用 `niuma state-label`

### Niuma 状态标签操作（机器人必读）

```bash
# 单 issue 流程
niuma state-label set --repo <owner/repo> --issue <num> --to bot:fix

# 多 issue DAG 入口
niuma state-label set --repo <owner/repo> --issue <num> --to bot:orchestrate
```

- 推荐安装 gh wrapper：`bash automation/niuma/scripts/install-gh-wrapper.sh`
- wrapper 会拦截直接改 `bot:*` 并提示改用 `niuma state-label`

## Git 提交规范

```
<type>: <description> (#<issue-number>)

[optional body]
```

Types:
- `feat`: 新功能
- `fix`: 修复 bug
- `docs`: 文档更新
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建/工具变动

## 项目特定

### 构建与测试

```bash
# 构建所有 Rust 工具
cargo build --release

# 运行测试
cargo test -p bcc

# 安装 bcc 到本地
./compiler/bcc/install.sh --link --rebuild
```

### 快速验证

```bash
# 使用 openclaw-arch 案例验证 arch 命令
cd compiler/bcc/examples/openclaw-arch
bcc arch validate \
  --target seed/v3.target-matrix.yaml \
  --actual artifacts/relation_matrix.actual.json \
  --out-dir /tmp/validate
```

## 参考

- Issue 列表: https://github.com/biantaishabi2/Cli/issues
- 项目 README: ./README.md
