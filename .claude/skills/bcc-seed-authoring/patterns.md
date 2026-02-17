# 常见依赖注入模式识别

> 帮助识别代码中的依赖关系，避免 seed 漏定义

## Elixir 模式

### 模式 1：Jido 框架注入

```elixir
# 代码特征
use Jido.AI.ReActAgent,
  tools: [Gong.Tools.Bash, Gong.Tools.Write]

# Seed 定义
caller: AGENT
callee: TOOLS
injection_type: "framework"
```

### 模式 2：GenServer 依赖

```elixir
# 代码特征
defmodule Gong.Worker do
  use GenServer
  # 启动时依赖其他进程
end

# Seed 定义
caller: WORKER
callee: 依赖的模块
injection_type: "supervisor"
```

### 模式 3：外部库注册内部模块

```elixir
# 代码特征
ReqLLM.Providers.register(Gong.Providers.DeepSeek)
# ^ 外部库    ^ 内部模块

# Seed 定义
caller: INFRA
callee: PROVIDERS
rationale: "Application 启动时注册 Provider"
```

## TypeScript 模式

### 模式 1：构造函数注入

```typescript
// 代码特征
class Agent {
  constructor(private tools: ToolRegistry) {}
}

// Seed 定义
caller: AGENT
callee: TOOLS
injection_type: "constructor"
```

### 模式 2：装饰器注入

```typescript
// 代码特征
@Injectable()
class Agent {
  constructor(@Inject(ToolRegistry) tools) {}
}

// Seed 定义
caller: AGENT
callee: TOOLS
injection_type: "framework"
```

### 模式 3：直接导入（紧耦合）

```typescript
// 代码特征
import { bashTool } from './tools/bash';

// Seed 定义
caller: AGENT
callee: TOOLS
injection_type: "direct"
```

## Go 模式

### 模式 1：接口注入

```go
// 代码特征
type Agent struct {
  tools ToolRegistry  // 接口类型
}

// Seed 定义
caller: AGENT
callee: TOOLS
injection_type: "constructor"
```

### 模式 2：init 函数注册

```go
// 代码特征
func init() {
  registry.Register("provider", DeepSeekProvider)
}

// Seed 定义
caller: INFRA
callee: PROVIDERS
rationale: "init 函数自动注册"
```

## 通用模式

### 模式 1：启动时初始化

```
代码特征：
- Application.start()
- main()
- init()
- bootstrap()

Seed 定义：
- caller: INFRA / APP
- callee: 被初始化的模块
- rationale: "启动时初始化"
```

### 模式 2：插件/扩展加载

```
代码特征：
- loadExtensions()
- plugin.register()
- Extension.Loader.load_all()

Seed 定义：
- caller: 加载者
- callee: EXTENSIONS / PLUGINS
- rationale: "加载扩展/插件"
```

### 模式 3：配置驱动

```
代码特征：
- config.get('providers')
- 从配置文件读取模块列表

Seed 定义：
- caller: 配置使用者
- callee: 配置中的模块
- injection_type: "config"
```

## 识别技巧

### 技巧 1：grep 关键模式

```bash
# Elixir: 框架注入
grep -r "use.*Agent\|use.*Framework" lib --include="*.ex"

# TypeScript: 装饰器
grep -r "@Injectable\|@Inject" src --include="*.ts"

# Go: 接口定义
grep -r "type.*interface" --include="*.go"

# 通用：外部库调用内部模块
grep -r "register.*Gong\|register.*MyApp" lib --include="*.ex"
```

### 技巧 2：查看启动文件

```bash
# Elixir: application.ex
cat lib/my_app/application.ex

# TypeScript: main.ts/index.ts
cat src/main.ts

# Go: main.go
cat cmd/app/main.go
```

### 技巧 3：查看配置文件

```bash
# 配置文件中的模块列表
cat config/config.exs
cat src/config.ts
```
