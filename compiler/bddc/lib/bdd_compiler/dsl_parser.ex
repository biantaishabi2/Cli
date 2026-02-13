defmodule BDDCompiler.DslParser do
  @moduledoc """
  解析行式 BDD DSL，输出可用于代码生成的 AST。

  设计目标：
  - DSL 可读（接近验收文档）
  - 解析简单（不依赖外部 parser 依赖）
  - 输出稳定（便于编译到 ExUnit）
  """

  @type meta :: %{
          optional(:file) => String.t() | nil,
          line: pos_integer(),
          raw: String.t()
        }

  @type expr ::
          {:var, atom()}
          | {:call, :uuid | :now | :run_id | :dec | :dt | :date, [String.t()]}
          | {:int, integer()}
          | {:bool, boolean()}
          | {:str, String.t()}

  @type step ::
          {:let, atom(), expr(), meta()}
          | {:given | :when | :then, atom(), map(), meta()}

  @type scenario :: %{
          id: String.t(),
          title: String.t(),
          tags: [atom()],
          steps: [step()],
          meta: meta()
        }

  @spec parse_file!(String.t()) :: [scenario()]
  def parse_file!(path) when is_binary(path) do
    content = File.read!(path)
    parse_string!(content, file: path)
  end

  @spec parse_string!(String.t()) :: [scenario()]
  def parse_string!(content, opts \\ []) when is_binary(content) do
    file = Keyword.get(opts, :file)
    line_offset = Keyword.get(opts, :line_offset, 0)

    content
    |> String.split("\n")
    |> Enum.with_index(1)
    |> Enum.reduce({[], nil}, fn {raw, line_no}, {acc, current} ->
      line = String.trim(raw)
      abs_line = line_no + line_offset

      cond do
        line == "" or String.starts_with?(line, "#") ->
          {acc, current}

        String.starts_with?(line, "[SCENARIO:") ->
          scenario = parse_scenario_header!(line, abs_line, raw, file)

          if current do
            {[current | acc], scenario}
          else
            {acc, scenario}
          end

        is_nil(current) ->
          raise ArgumentError, "DSL 语法错误：在第 #{abs_line} 行出现步骤但未声明 SCENARIO：#{line} (file=#{file})"

        true ->
          step = parse_step!(line, abs_line, raw, file)
          {acc, %{current | steps: current.steps ++ [step]}}
      end
    end)
    |> then(fn {acc, current} ->
      scenarios =
        case current do
          nil -> acc
          _ -> [current | acc]
        end

      Enum.reverse(scenarios)
    end)
  end

  defp parse_scenario_header!(line, line_no, raw, file) do
    # 格式：
    # [SCENARIO: ID] TITLE: xxx TAGS: a b c
    case Regex.run(~r/^\[SCENARIO:\s*([^\]]+)\]\s*(.*)$/, line) do
      [_, id, rest] ->
        {title, tags} = parse_header_rest(rest)

        %{
          id: String.trim(id),
          title: title,
          tags: tags,
          steps: [],
          meta: %{file: file, line: line_no, raw: raw}
        }

      _ ->
        raise ArgumentError, "DSL SCENARIO 头解析失败（第 #{line_no} 行）：#{line}"
    end
  end

  defp parse_header_rest(rest) do
    rest = String.trim(rest)

    title =
      case Regex.run(~r/TITLE:\s*([^T]+?)(?:\s+TAGS:|$)/, rest) do
        [_, t] -> String.trim(t)
        _ -> ""
      end

    tags =
      case Regex.run(~r/TAGS:\s*(.*)$/, rest) do
        [_, t] ->
          t
          |> String.trim()
          |> String.split(~r/\s+/, trim: true)
          |> Enum.map(&String.to_atom/1)

        _ ->
          []
      end

    {title, tags}
  end

  defp parse_step!("LET " <> rest, line_no, raw, file) do
    case String.split(rest, "=", parts: 2) do
      [name, value] ->
        {:let, String.trim(name) |> String.to_atom(), parse_expr!(String.trim(value), line_no),
         %{file: file, line: line_no, raw: raw}}

      _ ->
        raise ArgumentError, "DSL LET 语法错误（第 #{line_no} 行）：#{rest}"
    end
  end

  defp parse_step!(line, line_no, raw, file) do
    tokens = split_tokens(line)

    case tokens do
      [kind, step_name | kvs] when kind in ["GIVEN", "WHEN", "THEN"] ->
        kind = String.downcase(kind) |> String.to_atom()
        step_name = String.to_atom(step_name)
        args = parse_kvs!(kvs, line_no)
        {kind, step_name, args, %{file: file, line: line_no, raw: raw}}

      _ ->
        raise ArgumentError, "DSL 步骤语法错误（第 #{line_no} 行）：#{line}"
    end
  end

  defp parse_kvs!(kvs, line_no) do
    Enum.reduce(kvs, %{}, fn kv, acc ->
      case String.split(kv, "=", parts: 2) do
        [k, v] ->
          Map.put(acc, String.to_atom(k), parse_expr!(String.trim(v), line_no))

        _ ->
          raise ArgumentError, "DSL key=value 语法错误（第 #{line_no} 行）：#{kv}"
      end
    end)
  end

  defp parse_expr!("$" <> var, _line_no) do
    {:var, String.to_atom(var)}
  end

  defp parse_expr!("true", _line_no), do: {:bool, true}
  defp parse_expr!("false", _line_no), do: {:bool, false}

  defp parse_expr!("uuid()", _line_no), do: {:call, :uuid, []}
  defp parse_expr!("now()", _line_no), do: {:call, :now, []}
  defp parse_expr!("run_id()", _line_no), do: {:call, :run_id, []}

  defp parse_expr!("dec(" <> rest, line_no) do
    case Regex.run(~r/^dec\(\"(.*)\"\)$/, "dec(" <> rest) do
      [_, s] -> {:call, :dec, [s]}
      _ -> raise ArgumentError, "DSL dec(...) 语法错误（第 #{line_no} 行）：dec(#{rest}"
    end
  end

  defp parse_expr!("dt(" <> rest, line_no) do
    case Regex.run(~r/^dt\(\"(.*)\"\)$/, "dt(" <> rest) do
      [_, s] -> {:call, :dt, [s]}
      _ -> raise ArgumentError, "DSL dt(...) 语法错误（第 #{line_no} 行）：dt(#{rest}"
    end
  end

  defp parse_expr!("date(" <> rest, line_no) do
    case Regex.run(~r/^date\(\"(.*)\"\)$/, "date(" <> rest) do
      [_, s] -> {:call, :date, [s]}
      _ -> raise ArgumentError, "DSL date(...) 语法错误（第 #{line_no} 行）：date(#{rest}"
    end
  end

  defp parse_expr!(<<"\"", _::binary>> = s, line_no) do
    case Regex.run(~r/^\"(.*)\"$/, s) do
      [_, inner] -> {:str, inner}
      _ -> raise ArgumentError, "DSL 字符串语法错误（第 #{line_no} 行）：#{s}"
    end
  end

  defp parse_expr!(s, _line_no) do
    s = String.trim(s)

    cond do
      Regex.match?(~r/^-?\d+$/, s) ->
        {:int, String.to_integer(s)}

      true ->
        {:str, s}
    end
  end

  # 简单 tokenizer：按空格拆分，但保留双引号内容与括号内容不拆分。
  defp split_tokens(line) do
    line
    |> String.to_charlist()
    |> do_split_tokens([], [], :normal, 0)
    |> Enum.reverse()
  end

  defp do_split_tokens([], current, acc, _mode, _paren_depth) do
    token = current |> Enum.reverse() |> to_string() |> String.trim()
    acc = if token == "", do: acc, else: [token | acc]
    acc
  end

  defp do_split_tokens([c | rest], current, acc, mode, paren_depth) do
    cond do
      mode == :quote and c == ?" ->
        do_split_tokens(rest, [c | current], acc, :normal, paren_depth)

      mode == :quote ->
        do_split_tokens(rest, [c | current], acc, :quote, paren_depth)

      c == ?" ->
        do_split_tokens(rest, [c | current], acc, :quote, paren_depth)

      c == ?( ->
        do_split_tokens(rest, [c | current], acc, :normal, paren_depth + 1)

      c == ?) and paren_depth > 0 ->
        do_split_tokens(rest, [c | current], acc, :normal, paren_depth - 1)

      c in [?\s, ?\t] and paren_depth == 0 ->
        token = current |> Enum.reverse() |> to_string() |> String.trim()
        acc = if token == "", do: acc, else: [token | acc]
        do_split_tokens(rest, [], acc, :normal, paren_depth)

      true ->
        do_split_tokens(rest, [c | current], acc, mode, paren_depth)
    end
  end
end

