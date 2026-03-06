defmodule BDDCompiler.InstructionSet do
  @moduledoc """
  指令集（纯数据）加载与查询。

  约定：
  - `--instructions` / `--instructions-v2` 接受 `.exs` 文件路径
  - `.exs` 文件的返回值必须是一个 map：`%{instruction_atom => instruction_spec_map}`

  说明：
  - v2 如果只提供 delta（只包含 v2-only 指令），查询时会自动回退到 v1（即 v2 = v1 + delta）。
  - 这里不负责运行期执行；运行期由业务项目提供（生成测试会调用 runtime module）。
  """

  alias BDDCompiler.CompileError

  @type value_type :: :uuid | :datetime | :date | :decimal | :int | :string | :bool
  @type version :: :v1 | :v2

  @type arg_spec :: %{
          type: value_type(),
          required?: boolean(),
          allowed: [String.t()] | nil
        }

  @type rule ::
          {:exactly_one_of, [atom()]}

  @type instruction_spec :: %{
          name: atom(),
          kind: :given | :when | :then,
          args: %{optional(atom()) => arg_spec()},
          outputs: %{optional(atom()) => value_type()},
          rules: [rule()],
          scopes: [:unit | :integration | :e2e],
          boundary: :http | :service | :event | :domain | :db | :external_io | :test_runtime | nil,
          async?: boolean(),
          eventually?: boolean(),
          assert_class: :A | :B | :C | :D | :weak | :error | nil
        }

  @type t :: %__MODULE__{
          v1: %{optional(atom()) => instruction_spec()},
          v2: %{optional(atom()) => instruction_spec()}
        }

  defstruct v1: %{}, v2: %{}

  @doc """
  从 JSON manifest 文件加载指令集（仅 v1）。

  JSON 格式为 unibo 编译器输出的 instruction_manifest.json：
  `{"total": N, "entries": [...]}`

  每个 entry 包含 name, kind, args, outputs, scopes, boundary 等字段，
  缺失字段使用默认值。
  """
  @spec load_json!(String.t()) :: t()
  def load_json!(path) when is_binary(path) do
    content = File.read!(path)
    data = BDDCompiler.JsonParser.parse!(content)

    entries = Map.get(data, "entries", [])

    specs =
      Enum.into(entries, %{}, fn entry ->
        name = entry |> Map.fetch!("name") |> String.to_atom()
        kind = entry |> Map.fetch!("kind") |> String.to_atom()

        args = convert_args(Map.get(entry, "args", %{}))
        outputs = convert_outputs(Map.get(entry, "outputs", %{}))

        scopes =
          entry
          |> Map.get("scopes", ["integration", "e2e"])
          |> Enum.map(&String.to_atom/1)

        boundary =
          case Map.get(entry, "boundary") do
            nil -> :service
            b when is_binary(b) -> String.to_atom(b)
          end

        spec = %{
          name: name,
          kind: kind,
          args: args,
          outputs: outputs,
          scopes: scopes,
          boundary: boundary
        }

        {name, spec}
      end)

    v1 = normalize_specs(specs)
    %__MODULE__{v1: v1, v2: %{}}
  rescue
    e ->
      raise CompileError,
        message: "JSON 指令集文件加载失败：#{Exception.message(e)}",
        file: path,
        line: nil,
        raw: nil
  end

  @spec load!([String.t()], [String.t()]) :: t()
  def load!(v1_paths, v2_paths) when is_list(v1_paths) and is_list(v2_paths) do
    v1 =
      v1_paths
      |> Enum.flat_map(&read_specs_file!/1)
      |> merge_specs!("v1")
      |> normalize_specs()

    v2 =
      v2_paths
      |> Enum.flat_map(&read_specs_file!/1)
      |> merge_specs!("v2")
      |> normalize_specs()

    %__MODULE__{v1: v1, v2: v2}
  end

  @spec fetch(t(), atom(), version()) :: {:ok, instruction_spec()} | :error
  def fetch(%__MODULE__{} = set, name, version \\ :v1) when is_atom(name) and version in [:v1, :v2] do
    case version do
      :v1 ->
        Map.fetch(set.v1, name)

      :v2 ->
        case Map.fetch(set.v2, name) do
          {:ok, _} = ok -> ok
          :error -> Map.fetch(set.v1, name)
        end
    end
  end

  @spec supported_versions(t(), atom()) :: [version()]
  def supported_versions(%__MODULE__{} = set, name) when is_atom(name) do
    [:v1, :v2]
    |> Enum.filter(fn v -> match?({:ok, _}, fetch(set, name, v)) end)
  end

  # JSON args 转换：{"name": {"type": "string", "required": true}} -> %{name: %{type: :string, required?: true}}
  defp convert_args(nil), do: %{}
  defp convert_args(args) when is_map(args) do
    Enum.into(args, %{}, fn {k, v} ->
      arg_spec = %{
        type: v |> Map.get("type", "string") |> String.to_atom(),
        required?: Map.get(v, "required", false)
      }

      arg_spec =
        case Map.get(v, "allowed") do
          nil -> arg_spec
          list when is_list(list) -> Map.put(arg_spec, :allowed, list)
        end

      {String.to_atom(k), arg_spec}
    end)
  end

  # JSON outputs 转换：{"id": {"type": "uuid"}} -> %{id: :uuid}
  defp convert_outputs(nil), do: %{}
  defp convert_outputs(outputs) when is_map(outputs) do
    Enum.into(outputs, %{}, fn {k, v} ->
      type =
        case v do
          %{"type" => t} -> String.to_atom(t)
          t when is_binary(t) -> String.to_atom(t)
        end

      {String.to_atom(k), type}
    end)
  end

  defp read_specs_file!(path) when is_binary(path) do
    {term, _binding} = Code.eval_file(path)

    cond do
      is_map(term) ->
        [term]

      is_list(term) and Enum.all?(term, &is_map/1) ->
        term

      true ->
        raise CompileError,
          message: "指令集文件返回值必须是 map（或 map 列表），实际为：#{inspect(term)}",
          file: path,
          line: nil,
          raw: nil
    end
  rescue
    e ->
      raise CompileError,
        message: "指令集文件加载失败：#{Exception.message(e)}",
        file: path,
        line: nil,
        raw: nil
  end

  defp merge_specs!(maps, label) when is_list(maps) and is_binary(label) do
    Enum.reduce(maps, %{}, fn m, acc ->
      collisions =
        acc
        |> Map.keys()
        |> MapSet.new()
        |> MapSet.intersection(MapSet.new(Map.keys(m)))
        |> MapSet.to_list()
        |> Enum.sort()

      if collisions != [] do
        raise ArgumentError,
              "BDD 指令集(#{label})合并冲突: " <> Enum.map_join(collisions, ", ", &inspect/1)
      end

      Map.merge(acc, m)
    end)
  end

  defp normalize_specs(specs) when is_map(specs) do
    Enum.into(specs, %{}, fn {k, spec0} ->
      spec =
        spec0
        |> Map.put_new(:name, k)
        |> Map.put_new(:args, %{})
        |> Map.put_new(:outputs, %{})
        |> Map.put_new(:rules, [])
        |> Map.put_new(:async?, false)
        |> Map.put_new(:eventually?, false)
        |> Map.put_new(:assert_class, nil)
        |> Map.put(:scopes, Map.get(spec0, :scopes) || default_scopes(spec0))

      {k, spec}
    end)
  end

  defp default_scopes(%{name: name}) when name in [:clock_freeze, :clock_travel, :assert_noop],
    do: [:unit, :integration, :e2e]

  defp default_scopes(_), do: [:integration, :e2e]
end

