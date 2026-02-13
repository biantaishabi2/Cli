defmodule BDDCompiler.Config do
  @moduledoc false

  # Minimal TOML reader for bddc.
  #
  # We intentionally support only the subset we need:
  # - comments (#...)
  # - sections: [global], [compile], [lint], [check], [domain.autowire], [runtime.caps.sync]
  # - key = "string"
  # - key = true/false
  # - key = ["a", "b"] (string array)

  @type t :: %{optional(String.t()) => map()}

  @default_path ".bddc.toml"

  @spec load(String.t()) :: t()
  def load(project_root) when is_binary(project_root) do
    path = Path.join(project_root, @default_path)

    if File.regular?(path) do
      path
      |> File.read!()
      |> parse()
    else
      %{}
    end
  end

  @spec get(t(), String.t(), String.t()) :: term()
  def get(cfg, section, key) when is_map(cfg) and is_binary(section) and is_binary(key) do
    global = Map.get(cfg, "global", %{})
    local = Map.get(cfg, section, %{})
    Map.get(local, key, Map.get(global, key))
  end

  @spec get_local(t(), String.t(), String.t()) :: term()
  def get_local(cfg, section, key) when is_map(cfg) and is_binary(section) and is_binary(key) do
    cfg
    |> Map.get(section, %{})
    |> Map.get(key)
  end

  @spec parse(String.t()) :: t()
  def parse(text) when is_binary(text) do
    lines = String.split(text, "\n")
    do_parse(lines, "global", %{})
  end

  defp do_parse([], _section, acc), do: acc

  defp do_parse([line0 | rest], section, acc) do
    line =
      line0
      |> String.trim()
      |> strip_comment()
      |> String.trim()

    cond do
      line == "" ->
        do_parse(rest, section, acc)

      String.starts_with?(line, "[") and String.ends_with?(line, "]") ->
        sec =
          line
          |> String.trim_leading("[")
          |> String.trim_trailing("]")
          |> String.trim()

        do_parse(rest, sec, Map.put_new(acc, sec, %{}))

      true ->
        case String.split(line, "=", parts: 2) do
          [k0, v0] ->
            key = String.trim(k0)
            value = parse_value(String.trim(v0))
            sec_map = Map.get(acc, section, %{})
            acc = Map.put(acc, section, Map.put(sec_map, key, value))
            do_parse(rest, section, acc)

          _ ->
            # ignore malformed lines (keep parser minimal)
            do_parse(rest, section, acc)
        end
    end
  end

  defp strip_comment(line) do
    case String.split(line, "#", parts: 2) do
      [head, _] -> head
      [head] -> head
    end
  end

  defp parse_value("true"), do: true
  defp parse_value("false"), do: false

  defp parse_value(value) when is_binary(value) do
    cond do
      String.starts_with?(value, "\"") and String.ends_with?(value, "\"") ->
        value |> String.trim_leading("\"") |> String.trim_trailing("\"")

      String.starts_with?(value, "[") and String.ends_with?(value, "]") ->
        inner = value |> String.trim_leading("[") |> String.trim_trailing("]") |> String.trim()

        if inner == "" do
          []
        else
          inner
          |> String.split(",", trim: true)
          |> Enum.map(&String.trim/1)
          |> Enum.map(fn item ->
            item
            |> String.trim()
            |> String.trim_leading("\"")
            |> String.trim_trailing("\"")
          end)
        end

      true ->
        # fallback: raw string
        value
    end
  end
end
