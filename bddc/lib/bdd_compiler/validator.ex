defmodule BDDCompiler.Validator do
  @moduledoc """
  对 `BDDCompiler.DslParser` 的 AST 做编译期校验：

  - 未知指令/缺参/多余参数
  - 变量引用必须已定义（LET 或前序指令输出）
  - 参数类型匹配（uuid/datetime/decimal/int/string）
  - 指令自定义规则（例如 exactly_one_of）
  """

  alias BDDCompiler.{CompileError, InstructionSet}

  @type env :: %{optional(atom()) => InstructionSet.value_type()}

  @spec validate!(String.t(), [BDDCompiler.DslParser.scenario()], InstructionSet.t()) :: :ok
  def validate!(dsl_path, scenarios, %InstructionSet{} = instruction_set)
      when is_binary(dsl_path) and is_list(scenarios) do
    Enum.each(scenarios, fn scenario ->
      validate_scenario!(dsl_path, scenario, instruction_set)
    end)

    :ok
  end

  defp validate_scenario!(dsl_path, scenario, instruction_set) do
    version = scenario_version(scenario)
    scope = scenario_scope(scenario)
    allow_domain_boundary? = scenario_allow_domain_boundary?(scenario)
    steps = Map.fetch!(scenario, :steps)

    {_, _clock_frozen?, _when_count, then_count} =
      steps
      |> Enum.reduce({%{}, false, 0, 0}, fn step, {env, clock_frozen?, when_count, then_count} ->
        {env2, clock_frozen2} =
          validate_step!(dsl_path, step, env, clock_frozen?, version, scope, allow_domain_boundary?, instruction_set)

        {when_count2, then_count2} =
          case step do
            {:when, _name, _args, _meta} -> {when_count + 1, then_count}
            {:then, _name, _args, _meta} -> {when_count, then_count + 1}
            _ -> {when_count, then_count}
          end

        {env2, clock_frozen2, when_count2, then_count2}
      end)

    if then_count < 1 do
      meta = Map.get(scenario, :meta, %{})

      raise CompileError,
        message: "场景结构不完整：每个场景至少需要 1 个 THEN",
        file: meta[:file] || dsl_path,
        line: meta[:line],
        raw: meta[:raw]
    end
    :ok
  end

  defp scenario_version(%{tags: tags}) when is_list(tags) do
    cond do
      :bdd_v2 in tags -> :v2
      :bdd_v1 in tags -> :v1
      true -> :v1
    end
  end

  defp scenario_version(_), do: :v1

  defp scenario_scope(%{tags: tags}) when is_list(tags) do
    cond do
      :unit in tags -> :unit
      :e2e in tags -> :e2e
      true -> :integration
    end
  end

  defp scenario_scope(_), do: :integration

  defp scenario_allow_domain_boundary?(%{tags: tags}) when is_list(tags) do
    :allow_domain_boundary in tags
  end

  defp scenario_allow_domain_boundary?(_), do: false

  defp validate_step!(
         _dsl_path,
         {:let, name, expr, meta},
         env,
         clock_frozen?,
         _version,
         _scope,
         _allow_domain_boundary?,
         _instruction_set
       ) do
    if Map.has_key?(env, name) do
      raise CompileError,
        message: "LET 重复定义变量：#{inspect(name)}",
        file: meta[:file],
        line: meta[:line],
        raw: meta[:raw]
    end

    type = expr_type!(expr, env, meta, clock_frozen?)
    {Map.put(env, name, type), clock_frozen?}
  end

  defp validate_step!(
         _dsl_path,
         {kind, instruction, args, meta},
         env,
         clock_frozen?,
         version,
         scope,
         allow_domain_boundary?,
         %InstructionSet{} = instruction_set
       )
       when kind in [:given, :when, :then] and is_atom(instruction) and is_map(args) do
    spec =
      case InstructionSet.fetch(instruction_set, instruction, version) do
        {:ok, s} ->
          s

        :error ->
          supported = InstructionSet.supported_versions(instruction_set, instruction)

          if supported != [] do
            wanted =
              case supported do
                [:v2] -> "bdd_v2"
                _ -> Enum.map_join(supported, ", ", &("bdd_" <> Atom.to_string(&1)))
              end

            raise CompileError,
              message: "指令 #{inspect(instruction)} 仅在 #{wanted} 可用，需要迁移或为场景声明对应版本",
              file: meta[:file],
              line: meta[:line],
              raw: meta[:raw],
              instruction: instruction
          else
            raise CompileError,
              message: "未知指令：#{inspect(instruction)}",
              file: meta[:file],
              line: meta[:line],
              raw: meta[:raw],
              instruction: instruction
          end
      end

    if spec.kind != kind do
      raise CompileError,
        message: "指令类型不匹配：期望 #{inspect(spec.kind)}，实际 #{inspect(kind)}",
        file: meta[:file],
        line: meta[:line],
        raw: meta[:raw],
        instruction: instruction
    end

    if not (scope in (spec.scopes || [])) do
      raise CompileError,
        message:
          "指令不允许在当前测试层级使用：scope=#{inspect(scope)} instruction=#{inspect(instruction)} allowed_scopes=#{inspect(spec.scopes)}",
        file: meta[:file],
        line: meta[:line],
        raw: meta[:raw],
        instruction: instruction
    end

    if Map.get(spec, :boundary) == :domain and scope != :unit and not allow_domain_boundary? do
      raise CompileError,
        message: "domain 边界指令仅允许在 unit 场景中使用（或显式允许）",
        file: meta[:file],
        line: meta[:line],
        raw: meta[:raw],
        instruction: instruction
    end

    validate_args!(spec, args, env, meta, clock_frozen?)

    outputs =
      spec.outputs
      |> Enum.reduce(env, fn {out_name, out_type}, acc ->
        Map.put(acc, out_name, out_type)
      end)

    clock_frozen2 =
      case instruction do
        :clock_freeze -> true
        _ -> clock_frozen?
      end

    {outputs, clock_frozen2}
  end

  defp validate_step!(
         _dsl_path,
         other,
         _env,
         _clock_frozen?,
         _version,
         _scope,
         _allow_domain_boundary?,
         _instruction_set
       ) do
    raise CompileError, message: "无法识别的 DSL 步骤结构：#{inspect(other)}"
  end

  defp validate_args!(spec, args, env, meta, clock_frozen?) do
    expected_args = spec.args

    unknown =
      Map.keys(args)
      |> Enum.reject(&Map.has_key?(expected_args, &1))

    if unknown != [] do
      raise CompileError,
        message: "指令参数不允许：#{Enum.map_join(unknown, ", ", &inspect/1)}",
        file: meta[:file],
        line: meta[:line],
        raw: meta[:raw],
        instruction: spec.name
    end

    Enum.each(expected_args, fn {arg_name, arg_spec} ->
      if arg_spec.required? and not Map.has_key?(args, arg_name) do
        raise CompileError,
          message: "缺少必填参数：#{inspect(arg_name)}",
          file: meta[:file],
          line: meta[:line],
          raw: meta[:raw],
          instruction: spec.name
      end
    end)

    Enum.each(args, fn {arg_name, expr} ->
      expected = expected_args[arg_name].type
      actual = expr_type!(expr, env, meta, clock_frozen?)

      coerced? =
        case {expected, actual, expr} do
          # 兼容早期 DSL：允许用 "YYYY-MM-DD" 作为 :date（运行期会 to_date!）
          {:date, :string, {:str, s}} -> Regex.match?(~r/^\d{4}-\d{2}-\d{2}$/, s)
          _ -> false
        end

      if expected != actual and not coerced? do
        raise CompileError,
          message: "参数类型不匹配：#{inspect(arg_name)} 期望 #{inspect(expected)}，实际 #{inspect(actual)}",
          file: meta[:file],
          line: meta[:line],
          raw: meta[:raw],
          instruction: spec.name
      end

      allowed = expected_args[arg_name].allowed

      if is_list(allowed) do
        val =
          case expr do
            {:str, s} -> s
            _ -> nil
          end

        if is_nil(val) or val not in allowed do
          raise CompileError,
            message:
              "参数取值不合法：#{inspect(arg_name)} 只能是 #{Enum.map_join(allowed, ", ", &inspect/1)}，实际为 #{inspect(val)}",
            file: meta[:file],
            line: meta[:line],
            raw: meta[:raw],
            instruction: spec.name
        end
      end
    end)

    Enum.each(spec.rules, fn
      {:exactly_one_of, keys} ->
        present = Enum.filter(keys, &Map.has_key?(args, &1))

        if length(present) != 1 do
          raise CompileError,
            message: "参数约束失败：必须且只能提供其中一个 #{Enum.map_join(keys, ", ", &inspect/1)}",
            file: meta[:file],
            line: meta[:line],
            raw: meta[:raw],
            instruction: spec.name
        end
    end)
  end

  defp expr_type!({:call, :uuid, []}, _env, _meta, _clock_frozen?), do: :uuid
  defp expr_type!({:call, :dec, [_s]}, _env, _meta, _clock_frozen?), do: :decimal
  defp expr_type!({:call, :dt, [_s]}, _env, _meta, _clock_frozen?), do: :datetime
  defp expr_type!({:call, :date, [_s]}, _env, _meta, _clock_frozen?), do: :date
  defp expr_type!({:call, :run_id, []}, _env, _meta, _clock_frozen?), do: :string
  defp expr_type!({:int, _}, _env, _meta, _clock_frozen?), do: :int
  defp expr_type!({:bool, _}, _env, _meta, _clock_frozen?), do: :bool
  defp expr_type!({:str, _}, _env, _meta, _clock_frozen?), do: :string

  defp expr_type!({:call, :now, []}, _env, meta, clock_frozen?) do
    if not clock_frozen? do
      raise CompileError,
        message: "使用 now() 前必须先执行 clock_freeze（时间必须可复现）",
        file: meta[:file],
        line: meta[:line],
        raw: meta[:raw]
    end

    :datetime
  end

  defp expr_type!({:var, name}, env, meta, _clock_frozen?) when is_atom(name) do
    case Map.fetch(env, name) do
      {:ok, t} ->
        t

      :error ->
        raise CompileError,
          message: "引用了未定义变量：#{inspect(name)}",
          file: meta[:file],
          line: meta[:line],
          raw: meta[:raw]
    end
  end

  defp expr_type!(other, _env, meta, _clock_frozen?) do
    raise CompileError,
      message: "不支持的表达式：#{inspect(other)}",
      file: meta[:file],
      line: meta[:line],
      raw: meta[:raw]
  end
end
