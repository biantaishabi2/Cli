#!/usr/bin/env elixir
# bcc_emit.exs — 将 Rust 侧 QuotedAST JSON 转为格式化 Elixir 源码
#
# 用法：
#   echo '<json>' | elixir scripts/bcc_emit.exs --stdin
#   elixir scripts/bcc_emit.exs --help

defmodule BCC.Emit do
  @moduledoc false

  def run(["--help"]) do
    IO.puts("bcc_emit.exs — convert QuotedAST JSON (stdin) to formatted Elixir source (stdout)")
    IO.puts("")
    IO.puts("Usage: echo '<json>' | elixir bcc_emit.exs --stdin")
  end

  def run(["--stdin"]) do
    stdin = IO.read(:stdio, :eof)

    case Jason.decode(stdin) do
      {:ok, json} ->
        quoted = json_to_quoted(json)
        source = Macro.to_string(quoted)

        case Code.format_string!(source) do
          formatted when is_binary(formatted) ->
            IO.write(formatted)
            IO.write("\n")

          iodata ->
            IO.write(IO.iodata_to_binary(iodata))
            IO.write("\n")
        end

      {:error, reason} ->
        IO.puts(:stderr, "JSON decode error: #{inspect(reason)}")
        System.halt(1)
    end
  end

  def run(_) do
    IO.puts(:stderr, "Usage: echo '<json>' | elixir bcc_emit.exs --stdin")
    System.halt(1)
  end

  # --- JSON → Elixir Quoted AST ---
  # Rust serde(rename_all = "snake_case") 输出小写 key

  # {:__block__, [], exprs}
  def json_to_quoted(%{"block" => items}) when is_list(items) do
    {:__block__, [], Enum.map(items, &json_to_quoted/1)}
  end

  # {:%, [], [module, map]} — 结构体字面量
  def json_to_quoted(%{"call" => %{"name" => "%", "meta" => _meta, "args" => args}}) do
    [module_ast, map_ast] = Enum.map(args, &json_to_quoted/1)
    {:%, [], [module_ast, map_ast]}
  end

  # {:%{}, [], kvs} — map 字面量
  def json_to_quoted(%{"call" => %{"name" => "%{}", "meta" => _meta, "args" => args}}) do
    kvs =
      args
      |> Enum.map(&json_to_quoted/1)
      |> Enum.flat_map(fn
        # 列表中的 keyword pair [{:key, val}, ...]
        items when is_list(items) -> items
        other -> [other]
      end)

    {:%{}, [], kvs}
  end

  # {name, meta, args} — 函数调用/定义
  def json_to_quoted(%{"call" => %{"name" => name, "meta" => meta, "args" => args}}) do
    atom_name = String.to_atom(name)
    elixir_meta = Enum.map(meta, fn [k, v] -> {String.to_atom(k), v} end)
    elixir_args = Enum.map(args, &json_to_quoted/1)
    {atom_name, elixir_meta, elixir_args}
  end

  # {:__aliases__, meta, segments} — 模块名
  def json_to_quoted(%{"aliases" => %{"segments" => segments}}) do
    {:__aliases__, [], Enum.map(segments, &String.to_atom/1)}
  end

  # 原子
  def json_to_quoted(%{"atom" => v}) do
    String.to_atom(v)
  end

  # 字符串
  def json_to_quoted(%{"string" => v}) do
    v
  end

  # 整数
  def json_to_quoted(%{"int" => v}) do
    v
  end

  # 列表
  def json_to_quoted(%{"list" => items}) when is_list(items) do
    Enum.map(items, &json_to_quoted/1)
  end

  # 元组（2 元素用 {} 字面量，其他用 {:{}, [], items}）
  def json_to_quoted(%{"tuple" => items}) when is_list(items) do
    elems = Enum.map(items, &json_to_quoted/1)

    case elems do
      [a, b] -> {a, b}
      _ -> {:{}, [], elems}
    end
  end

  # Map
  def json_to_quoted(%{"map" => pairs}) when is_list(pairs) do
    kvs =
      Enum.map(pairs, fn [k, v] ->
        {json_to_quoted(k), json_to_quoted(v)}
      end)

    {:%{}, [], kvs}
  end

  # 裸列表（顶层 JSON 数组）
  def json_to_quoted(items) when is_list(items) do
    quoted_items = Enum.map(items, &json_to_quoted/1)
    {:__block__, [], quoted_items}
  end

  # 原始值透传
  def json_to_quoted(v) when is_binary(v), do: v
  def json_to_quoted(v) when is_integer(v), do: v
  def json_to_quoted(v) when is_float(v), do: v
  def json_to_quoted(true), do: true
  def json_to_quoted(false), do: false
  def json_to_quoted(nil), do: nil
end

# 确保 Jason 可用（Mix.install 自动下载）
Mix.install([{:jason, "~> 1.4"}])

BCC.Emit.run(System.argv())
