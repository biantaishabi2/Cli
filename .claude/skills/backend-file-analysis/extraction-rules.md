# 语言提取规则参考

agent 在分析源码时，按语言使用对应的提取规则。

## Elixir (.ex / .exs)

| 提取项 | 匹配模式 | 说明 |
|--------|---------|------|
| 公开函数 | `def func_name(args)` | 忽略 `defp`（私有） |
| 类型签名 | `@spec func_name(type) :: return_type` | 直接作为输入输出 |
| 错误模式 | `{:error, :reason}` / `{:error, %ErrorStruct{}}` | 提取为异常处理来源 |
| 模块调用 | `ModuleName.func()` / `alias ModuleName` | 下游依赖 |
| 模块文档 | `@moduledoc """..."""` | 填充职责章节 |
| 副作用 | `GenServer.call/cast`、`Task.async`、`HTTPoison/Req`、`File.*`、`PubSub.broadcast` | 填充副作用章节 |
| 上游搜索 | `grep -r "alias.*ModuleName\|ModuleName\." --include="*.ex"` | 填充上游引用 |

## Go (.go)

| 提取项 | 匹配模式 | 说明 |
|--------|---------|------|
| 公开函数 | `func FuncName(` （大写开头） | 小写开头是私有 |
| 类型签名 | 函数参数和返回值类型 | Go 是静态类型，签名即文档 |
| 错误模式 | `return ..., err` / `return ..., fmt.Errorf(...)` | 多返回值最后一个是 error |
| 模块调用 | `import "pkg/path"` + `pkg.Func()` | 下游依赖 |
| 模块文档 | `// Package pkgname ...` 注释 | 填充职责章节 |
| 副作用 | `go func()`、`http.Get/Post`、`os.Open/Create`、`chan <-` | 填充副作用章节 |
| 上游搜索 | `grep -r "import.*\"pkg/path\"" --include="*.go"` | 填充上游引用 |

## Rust (.rs)

| 提取项 | 匹配模式 | 说明 |
|--------|---------|------|
| 公开函数 | `pub fn func_name(` / `pub async fn` | 无 `pub` 的是私有 |
| 类型签名 | `-> ReturnType` / `-> Result<T, E>` | Rust 是静态类型 |
| 错误模式 | `Result::Err(...)` / `?` 操作符 / `panic!` | 提取为异常处理来源 |
| 模块调用 | `use crate::module::*` / `mod::func()` | 下游依赖 |
| 模块文档 | `//! module-level doc comment` | 填充职责章节 |
| 副作用 | `tokio::spawn`、`reqwest::get`、`std::fs::*`、`async fn` | 填充副作用章节 |
| 上游搜索 | `grep -r "use crate::module_name\|mod module_name" --include="*.rs"` | 填充上游引用 |

## PHP (.php)

| 提取项 | 匹配模式 | 说明 |
|--------|---------|------|
| 公开函数 | `public function methodName(` | `private`/`protected` 忽略 |
| 类型签名 | PHPDoc `@param type $name` / `@return type` / PHP 8 类型声明 | 填充输入输出 |
| 错误模式 | `throw new Exception(...)` / `return false` / `return null` | 提取为异常处理来源 |
| 模块调用 | `use ClassName` / `ClassName::method()` / `new ClassName()` | 下游依赖 |
| 模块文档 | `/** ... */` 类级注释 | 填充职责章节 |
| 副作用 | `curl_*`、`file_get_contents/fwrite`、`$redis->*`、`event()` | 填充副作用章节 |
| 上游搜索 | `grep -r "use.*ClassName\|ClassName::" --include="*.php"` | 填充上游引用 |

## TypeScript / JavaScript (.ts / .tsx / .js)

| 提取项 | 匹配模式 | 说明 |
|--------|---------|------|
| 公开函数 | `export function/class/const` | 无 export 的是内部 |
| 类型签名 | TS 类型标注 `: Type` / JSDoc `@param/@returns` | 填充输入输出 |
| 错误模式 | `throw new Error(...)` / `Promise.reject()` / `catch (e)` | 提取为异常处理来源 |
| 模块调用 | `import { x } from './path'` / `require('./path')` | 下游依赖 |
| 模块文档 | JSDoc `/** ... */` 模块级 / 文件顶部注释 | 填充职责章节 |
| 副作用 | `fetch()`、`fs.*`、`child_process.*`、`async/await`、`EventEmitter` | 填充副作用章节 |
| 上游搜索 | `grep -r "from.*['\"].*module_path['\"]" --include="*.ts"` | 填充上游引用 |

## 通用规则

- 测试文件排除：`*.test.*`、`*.spec.*`、`__tests__/`、`test/`（看语言惯例）
- 生成文件排除：`*_generated.*`、`*.gen.*`
- 第三方依赖排除：`node_modules/`、`vendor/`、`deps/`、`_build/`、`target/`
