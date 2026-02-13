defmodule BDDCompiler.CompileError do
  @moduledoc """
  BDD DSL 编译期错误（硬失败）。
  """

  defexception [:message, :file, :line, :raw, :instruction]

  @impl true
  def exception(opts) do
    file = Keyword.get(opts, :file)
    line = Keyword.get(opts, :line)
    raw = Keyword.get(opts, :raw)
    instruction = Keyword.get(opts, :instruction)
    msg = Keyword.fetch!(opts, :message)

    suffix =
      [
        if(file, do: "file=#{file}", else: nil),
        if(line, do: "line=#{line}", else: nil),
        if(instruction, do: "instruction=#{instruction}", else: nil),
        if(raw, do: "raw=#{String.trim(raw)}", else: nil)
      ]
      |> Enum.reject(&is_nil/1)
      |> case do
        [] -> ""
        parts -> " (" <> Enum.join(parts, ", ") <> ")"
      end

    %__MODULE__{
      message: msg <> suffix,
      file: file,
      line: line,
      raw: raw,
      instruction: instruction
    }
  end
end

