defmodule BDDCompiler.MarkdownExtractor do
  @moduledoc """
  从 Markdown 文档中抽取可编译的 BDD DSL block。

  约定：
  - 使用 fenced code block，语言标识为 `bdd` / `dsl` / `bdd-dsl`：
      ```bdd
      [SCENARIO: ...]
      ...
      ```

  产物：
  - 返回每个 block 的起始行号（DSL 第一行在 Markdown 中的行号）与内容。
  """

  alias BDDCompiler.CompileError

  @type block :: %{
          index: pos_integer(),
          start_line: pos_integer(),
          content: String.t()
        }

  @spec extract_blocks!(String.t()) :: [block()]
  def extract_blocks!(md_path) when is_binary(md_path) do
    content = File.read!(md_path)
    extract_blocks_from_string!(content, md_path)
  end

  @spec extract_blocks_from_string!(String.t(), String.t()) :: [block()]
  def extract_blocks_from_string!(content, md_path) when is_binary(content) and is_binary(md_path) do
    lines = String.split(content, "\n")

    {blocks, state} =
      lines
      |> Enum.with_index(1)
      |> Enum.reduce({[], :outside}, fn {line, line_no}, {acc, state} ->
        case state do
          :outside ->
            if fence_start?(line) do
              {acc, {:inside, line_no + 1, []}}
            else
              {acc, :outside}
            end

          {:inside, start_line, buf} ->
            if fence_end?(line) do
              content = buf |> Enum.reverse() |> Enum.join("\n")
              block = %{index: length(acc) + 1, start_line: start_line, content: content}
              {[block | acc], :outside}
            else
              {acc, {:inside, start_line, [line | buf]}}
            end
        end
      end)

    case state do
      :outside ->
        Enum.reverse(blocks)

      {:inside, start_line, _buf} ->
        raise CompileError,
          message: "Markdown DSL block 未闭合（缺少结束 ```）",
          file: md_path,
          line: start_line - 1,
          raw: nil
    end
  end

  defp fence_start?(line) when is_binary(line) do
    Regex.match?(~r/^\s*```(bdd|dsl|bdd-dsl)\s*$/i, String.trim_trailing(line))
  end

  defp fence_end?(line) when is_binary(line) do
    Regex.match?(~r/^\s*```\s*$/, String.trim_trailing(line))
  end
end

