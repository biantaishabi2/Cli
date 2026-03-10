defmodule BDDCompiler.Linter do
  @moduledoc """
  DSL 静态检查（lint）：不生成测试、不修改文件，只输出可操作建议。

  目标：先以 warning 形式暴露反模式，成熟后可逐步升级为编译期 hard fail。
  """

  alias BDDCompiler.{DslParser, InstructionSet, JsonParser, MarkdownExtractor}

  defmodule Warning do
    @moduledoc false
    defstruct [:rule, :message, :file, :line, :raw, :scenario_id, :tags]
  end

  defmodule SeedContract do
    @moduledoc false
    defstruct edge_class: nil,
              expected_facts: [],
              action_path: [],
              branch_hints: [],
              declared_error_codes: [],
              business_post_conditions: []
  end

  @type warning :: %Warning{}

  @spec lint_dir(String.t(), InstructionSet.t()) :: [warning()]
  def lint_dir(dir, %InstructionSet{} = instruction_set) when is_binary(dir) do
    seed_contracts = load_seed_contracts(dir)

    dsl_paths = discover_dsl_paths(dir)

    # 默认仅在目录结构为 docs/bdd 时，lint 同级 docs_root 下的 Markdown 内嵌 DSL（文档即测试）。
    docs_root =
      if Path.basename(dir) == "bdd" and Path.basename(Path.dirname(dir)) == "docs" do
        Path.dirname(dir)
      else
        nil
      end

    md_paths =
      if is_binary(docs_root) do
        docs_root
        |> Path.join("**/*.md")
        |> Path.wildcard()
        |> Enum.sort()
      else
        []
      end

    {warnings, scenarios} =
      lint_sources_with_scenarios(dsl_paths, md_paths, instruction_set, seed_contracts)

    warnings ++ lint_negative_coverage(dir, scenarios)
  end

  @spec lint_paths([String.t()], InstructionSet.t()) :: [warning()]
  def lint_paths(paths, %InstructionSet{} = instruction_set) when is_list(paths) do
    paths
    |> Enum.flat_map(fn path ->
      scenarios = DslParser.parse_file!(path)
      seed_contracts = load_seed_contracts(Path.dirname(path))
      Enum.flat_map(scenarios, &lint_scenario(&1, instruction_set, seed_contracts))
    end)
  end

  defp lint_sources_with_scenarios(dsl_paths, md_paths, instruction_set, seed_contracts) do
    from_dsl_scenarios =
      dsl_paths
      |> Enum.flat_map(&DslParser.parse_file!/1)

    from_md_scenarios =
      md_paths
      |> Enum.flat_map(fn md_path ->
        MarkdownExtractor.extract_blocks!(md_path)
        |> Enum.flat_map(fn b ->
          DslParser.parse_string!(b.content, file: md_path, line_offset: b.start_line - 1)
        end)
      end)

    scenarios = from_dsl_scenarios ++ from_md_scenarios
    warnings = Enum.flat_map(scenarios, &lint_scenario(&1, instruction_set, seed_contracts))
    {warnings, scenarios}
  end

  defp lint_scenario(%{id: id, steps: steps, tags: tags}, instruction_set, seed_contracts) do
    version =
      cond do
        :bdd_v2 in tags -> :v2
        true -> :v1
      end

    seed_contract = lookup_seed_contract(seed_contracts, id)
    warns = []
    warns = warns ++ lint_weak_assertions(id, tags, steps, version, instruction_set, seed_contract)
    warns = warns ++ lint_strict_evidence(id, tags, steps, version, instruction_set)
    warns = warns ++ lint_over_interaction_assertion(id, tags, steps, version, instruction_set)
    warns = warns ++ lint_sleep_anti_pattern(id, tags, steps)
    warns = warns ++ lint_order_dependence(id, tags, steps)
    warns = warns ++ lint_impl_detail_reference(id, tags, steps)
    warns = warns ++ lint_unique_risk(id, tags, steps)
    warns = warns ++ lint_pure_snapshot(id, tags, steps)
    warns ++ lint_async_without_eventually(id, tags, steps, version, instruction_set)
  end

  # 规则（严格证据）：当场景带 TAGS: strict_evidence 时，必须包含至少 1 个“强断言”（A/B/C/D/error）。
  # 目的：避免只有 assert_noop/弱断言导致假阳性与不可诊断。
  defp lint_strict_evidence(scenario_id, tags, steps, version, instruction_set) do
    if :strict_evidence not in tags do
      []
    else
      strong_then? =
        Enum.any?(steps, fn
          {:then, name, _args, _meta} ->
            case InstructionSet.fetch(instruction_set, name, version) do
              {:ok, spec} -> spec.assert_class in [:A, :B, :C, :D, :error]
              :error -> false
            end

          _ ->
            false
        end)

      if strong_then? do
        []
      else
        meta =
          steps
          |> Enum.find_value(fn
            {:then, _name, _args, meta} -> meta
            _ -> nil
          end) || %{}

        [
          %Warning{
            rule: :insufficient_evidence,
            scenario_id: scenario_id,
            tags: tags,
            file: meta[:file],
            line: meta[:line],
            raw: meta[:raw],
            message:
              "strict_evidence 场景必须包含至少 1 个强断言（A/B/C/D/error），避免只有弱断言导致假阳性与不可诊断。"
          }
        ]
      end
    end
  end

  # 规则：如果一个场景的 THEN 全是“弱断言”（或只有 assert_noop），提示补齐关键业务事实断言。
  defp lint_weak_assertions(scenario_id, tags, steps, version, instruction_set, seed_contract) do
    then_steps =
      Enum.filter(steps, fn
        {:then, _name, _args, _meta} -> true
        _ -> false
      end)

    then_specs =
      Enum.map(then_steps, fn {:then, name, _args, _meta} ->
        case InstructionSet.fetch(instruction_set, name, version) do
          {:ok, spec} -> spec
          :error -> %{name: name, assert_class: nil}
        end
      end)

    weak? = fn spec -> spec.assert_class in [:weak, nil] end

    if then_steps != [] and Enum.all?(then_specs, weak?) and
         not strong_seed_contract_assertion?(then_steps, seed_contract) and
         not exempt_seed_contract_weak_assertion?(then_steps, seed_contract) do
      meta = elem(List.first(then_steps), 3)

      [
        %Warning{
          rule: :weak_assertion,
          scenario_id: scenario_id,
          tags: tags,
          file: meta[:file],
          line: meta[:line],
          raw: meta[:raw],
          message: "场景的 THEN 只有弱断言（如 assert_noop）。建议补充关键业务事实断言（A/B/C/D）。"
        }
      ]
    else
      []
    end
  end

  defp load_seed_contracts(dir) when is_binary(dir) do
    context_dir = discover_context_dir(dir)

    if is_binary(context_dir) and File.dir?(context_dir) do
      context_dir
      |> Path.join("*.json")
      |> Path.wildcard()
      |> Enum.reduce(%{}, fn path, acc ->
        case load_seed_contract(path) do
          {id, %SeedContract{} = contract} -> Map.put(acc, id, contract)
          _ -> acc
        end
      end)
    else
      %{}
    end
  end

  defp discover_dsl_paths(dir) when is_binary(dir) do
    root_paths =
      dir
      |> Path.join("*.dsl")
      |> Path.wildcard()

    feature_paths =
      dir
      |> Path.join("features/**/*.dsl")
      |> Path.wildcard()

    (root_paths ++ feature_paths)
    |> Enum.uniq()
    |> Enum.sort()
  end

  defp discover_context_dir(dir) when is_binary(dir) do
    direct = Path.join(dir, "contexts")

    cond do
      File.dir?(direct) ->
        direct

      Path.basename(dir) in ["features", "scenarios"] ->
        sibling = Path.join(Path.dirname(dir), "contexts")
        if File.dir?(sibling), do: sibling, else: nil

      true ->
        nil
    end
  end

  defp load_seed_contract(path) when is_binary(path) do
    with {:ok, raw} <- File.read(path),
         data when is_map(data) <- JsonParser.parse!(raw),
         id when is_binary(id) and id != "" <- Map.get(data, "id") do
      {id,
       %SeedContract{
         edge_class: Map.get(data, "edge_class"),
         expected_facts: string_list(Map.get(data, "expected_facts")),
         action_path: string_list(Map.get(data, "action_path")),
         branch_hints: string_list(Map.get(data, "branch_hints")),
         declared_error_codes: string_list(Map.get(data, "declared_error_codes")),
         business_post_conditions: string_list(Map.get(data, "business_post_conditions"))
       }}
    else
      _ -> nil
    end
  rescue
    _ -> nil
  end

  defp string_list(values) when is_list(values) do
    Enum.filter(values, &is_binary/1)
  end

  defp string_list(_), do: []

  defp strong_seed_contract_assertion?(then_steps, %SeedContract{} = seed_contract) do
    has_seed_then? =
      Enum.any?(then_steps, fn
        {:then, :then_seed_contract_should_hold, _args, _meta} -> true
        _ -> false
      end)

    has_seed_then? and seed_contract_sufficient?(seed_contract)
  end

  defp strong_seed_contract_assertion?(_then_steps, _), do: false

  defp exempt_seed_contract_weak_assertion?(then_steps, %SeedContract{} = seed_contract) do
    has_seed_then? =
      Enum.any?(then_steps, fn
        {:then, :then_seed_contract_should_hold, _args, _meta} -> true
        _ -> false
      end)

    has_seed_then? and seed_contract_expected_weak?(seed_contract)
  end

  defp exempt_seed_contract_weak_assertion?(_then_steps, _), do: false

  defp seed_contract_expected_weak?(%SeedContract{edge_class: "workflow_contract"} = contract) do
    length(contract.action_path) == 1 and contract.branch_hints == []
  end

  defp seed_contract_expected_weak?(%SeedContract{}), do: false

  defp seed_contract_sufficient?(%SeedContract{edge_class: "workflow_contract"} = contract) do
    length(contract.action_path) >= 2 or contract.branch_hints != []
  end

  defp seed_contract_sufficient?(%SeedContract{} = contract) do
    contract.expected_facts != [] or contract.action_path != [] or contract.branch_hints != [] or
      contract.declared_error_codes != [] or contract.business_post_conditions != []
  end

  defp lookup_seed_contract(seed_contracts, scenario_id)
       when is_map(seed_contracts) and is_binary(scenario_id) do
    Map.get(seed_contracts, scenario_id) ||
      case String.split(scenario_id, "-SEED-", parts: 2) do
        [_prefix, bare_id] -> Map.get(seed_contracts, bare_id)
        _ -> nil
      end
  end

  # 规则（可选严格模式）：避免在非 e2e 场景使用“交互断言”（mock/spy/called 等），这类断言更容易锁死内部协作细节。
  #
  # 启用方式：场景带 TAGS: strict_interaction
  #
  # 例外：
  # - e2e 层允许（更接近系统边界）；
  # - 显式带 TAGS: allow_interaction_assert 时允许（用于过渡或确有必要的场景）。
  defp lint_over_interaction_assertion(scenario_id, tags, steps, version, instruction_set) do
    if :strict_interaction not in tags do
      []
    else
      scope =
        cond do
          :e2e in tags -> :e2e
          :unit in tags -> :unit
          true -> :integration
        end

      allowed? = scope == :e2e or :allow_interaction_assert in tags

      cond do
        allowed? ->
          []

        has_semantic_interaction_assert?(steps, instruction_set, version) ->
          # 已使用语义化 D 类交互断言（external_io），不再基于 raw 误伤。
          []

        true ->
          then_hit =
            Enum.find(steps, fn
              {:then, _name, _args, meta} ->
                raw = to_string(meta[:raw] || "")
                Regex.match?(~r/\b(mock|spy|stub|called|invoked)\b/i, raw)

              _ ->
                false
            end)

          case then_hit do
            {:then, _name, _args, meta} ->
              [
                %Warning{
                  rule: :over_interaction_assertion,
                  scenario_id: scenario_id,
                  tags: tags,
                  file: meta[:file],
                  line: meta[:line],
                  raw: meta[:raw],
                  message:
                    "检测到可能的交互断言（mock/spy/called）。建议优先断言可观测业务事实（A/B/C）；如确需交互断言，请使用 external_io 语义的 D 类断言，或改为 e2e/显式加 TAGS: allow_interaction_assert。"
                }
              ]

            _ ->
              []
          end
      end
    end
  end

  defp has_semantic_interaction_assert?(steps, instruction_set, version) do
    Enum.any?(steps, fn
      {:then, name, _args, _meta} ->
        case InstructionSet.fetch(instruction_set, name, version) do
          {:ok, spec} ->
            spec.assert_class == :D and Map.get(spec, :boundary) == :external_io

          :error ->
            false
        end

      _ ->
        false
    end)
  end

  # 规则：禁止/提示“sleep”式等待（应使用 eventually）。
  defp lint_sleep_anti_pattern(scenario_id, tags, steps) do
    steps
    |> Enum.flat_map(fn
      {_kind, _name, _args, meta} ->
        raw = to_string(meta[:raw] || "")

        if String.contains?(String.downcase(raw), "sleep") do
          [
            %Warning{
              rule: :sleep_anti_pattern,
              scenario_id: scenario_id,
              tags: tags,
              file: meta[:file],
              line: meta[:line],
              raw: meta[:raw],
              message: "检测到 sleep 相关写法。建议用 assert_eventually/eventually 风格断言替代固定等待。"
            }
          ]
        else
          []
        end

      _ ->
        []
    end)
  end

  # 规则：提示可能的顺序依赖（index/first/last）。
  defp lint_order_dependence(scenario_id, tags, steps) do
    steps
    |> Enum.flat_map(fn
      {_kind, _name, _args, meta} ->
        raw = to_string(meta[:raw] || "")

        hit? =
          Regex.match?(~r/(^|\s)index=/i, raw) or
            Regex.match?(~r/(^|\s)first\b/i, raw) or
            Regex.match?(~r/(^|\s)last\b/i, raw)

        if hit? do
          [
            %Warning{
              rule: :order_dependence,
              scenario_id: scenario_id,
              tags: tags,
              file: meta[:file],
              line: meta[:line],
              raw: meta[:raw],
              message: "检测到可能的顺序依赖写法（index/first/last）。建议断言显式排序规则或改为集合断言。"
            }
          ]
        else
          []
        end

      _ ->
        []
    end)
  end

  # 规则：出现 async act（例如事件广播）但缺少 eventually 风格断言时，提示补齐稳定断言。
  defp lint_async_without_eventually(scenario_id, tags, steps, version, instruction_set) do
    async_act? =
      Enum.any?(steps, fn
        {:when, name, _args, _meta} ->
          case InstructionSet.fetch(instruction_set, name, version) do
            {:ok, spec} -> spec.async? == true
            :error -> false
          end

        _ ->
          false
      end)

    has_eventually_assert? =
      Enum.any?(steps, fn
        {:then, name, _args, _meta} ->
          case InstructionSet.fetch(instruction_set, name, version) do
            {:ok, spec} -> spec.eventually? == true
            :error -> false
          end

        _ ->
          false
      end)

    if async_act? and not has_eventually_assert? do
      meta =
        steps
        |> Enum.find_value(fn
          {:when, name, _args, meta} ->
            case InstructionSet.fetch(instruction_set, name, version) do
              {:ok, spec} when spec.async? == true -> meta
              _ -> nil
            end

          _ ->
            nil
        end)

      [
        %Warning{
          rule: :async_without_eventually,
          scenario_id: scenario_id,
          tags: tags,
          file: meta[:file],
          line: meta[:line],
          raw: meta[:raw],
          message: "检测到 async act，但缺少 eventually 风格断言。建议补齐以避免偶发失败。"
        }
      ]
    else
      []
    end
  end

  # 规则：提示“实现细节泄露”的迹象（Repo/Ecto/Schema/Changeset 等）。
  # 这类写法通常意味着 Then 在断言内部实现而非可观测业务事实。
  defp lint_impl_detail_reference(scenario_id, tags, steps) do
    patterns = [
      ~r/\bRepo\b/,
      ~r/\bEcto\b/,
      ~r/\bSchema\b/,
      ~r/\bChangeset\b/,
      ~r/\binsert_all\b/i,
      ~r/\bupdate_all\b/i,
      ~r/\bfrom\(/,
      ~r/\btransaction\b/i,
      ~r/\brollback\b/i,
      ~r/\bSQL\b/i
    ]

    steps
    |> Enum.flat_map(fn
      {_kind, _name, _args, meta} ->
        raw = to_string(meta[:raw] || "")

        if Enum.any?(patterns, &Regex.match?(&1, raw)) do
          [
            %Warning{
              rule: :impl_detail_reference,
              scenario_id: scenario_id,
              tags: tags,
              file: meta[:file],
              line: meta[:line],
              raw: meta[:raw],
              message: "检测到可能的实现细节引用（Repo/Ecto/Schema/Changeset/SQL 等）。建议改为断言可观测业务事实（A/B/C/D）。"
            }
          ]
        else
          []
        end

      _ ->
        []
    end)
  end

  # 规则：负向/边界场景覆盖（suite 级别）。
  # 以最小启发式判断 negative：
  # - 出现 THEN assert_last_error
  # - 或出现 WHEN try_*（预期失败分支）
  defp lint_negative_coverage(dir, scenarios) when is_binary(dir) and is_list(scenarios) do
    scoped =
      Enum.filter(scenarios, fn s ->
        tags = Map.get(s, :tags, []) || []
        :integration in tags or :e2e in tags
      end)

    if scoped == [] do
      []
    else
      negative? = fn s ->
        Enum.any?(s.steps, fn
          {:then, :assert_last_error, _args, _meta} -> true
          {:when, name, _args, _meta} when is_atom(name) -> String.starts_with?(Atom.to_string(name), "try_")
          _ -> false
        end)
      end

      negative_count = Enum.count(scoped, negative?)

      if negative_count < 1 do
        [
          %Warning{
            rule: :insufficient_negative_cases,
            scenario_id: "(suite)",
            tags: [],
            file: dir,
            line: nil,
            raw: nil,
            message:
              "检测到 integration/e2e 场景集缺少负向/边界场景。建议至少添加 1 个（例如 invalid_params/invalid_status/duplicate 等）。"
          }
        ]
      else
        []
      end
    end
  end

  # 规则：禁止“纯 snapshot”（默认仅在 TAGS 含 strict_snapshot 时启用）。
  defp lint_pure_snapshot(scenario_id, tags, steps) do
    if :strict_snapshot not in tags do
      []
    else
      then_names =
        steps
        |> Enum.filter(fn
          {:then, _name, _args, _meta} -> true
          _ -> false
        end)
        |> Enum.map(fn {:then, name, _args, _meta} -> name end)

      if :assert_http_snapshot in then_names and Enum.uniq(then_names) == [:assert_http_snapshot] do
        meta =
          steps
          |> Enum.find_value(fn
            {:then, :assert_http_snapshot, _args, meta} -> meta
            _ -> nil
          end)

        [
          %Warning{
            rule: :pure_snapshot,
            scenario_id: scenario_id,
            tags: tags,
            file: meta[:file],
            line: meta[:line],
            raw: meta[:raw],
            message: "检测到纯 snapshot 断言。建议至少补充 1 条关键显式断言（例如 status/body 的关键字段）。"
          }
        ]
      else
        []
      end
    end
  end

  # 规则：唯一性风险提示（默认仅在 TAGS 含 strict_unique 时启用）。
  # 仅做最小启发式：对可能触发唯一约束的 string 字段（如 product_id）如果是纯常量且未包含 run_id 痕迹，提示改用 run_id() 拼接。
  defp lint_unique_risk(scenario_id, tags, steps) do
    if :strict_unique not in tags do
      []
    else
      watched = [
        {:given, :create_product, :product_id},
        {:given, :create_order_item, :product_id}
      ]

      steps
      |> Enum.flat_map(fn
        {kind, name, args, meta} when is_map(args) ->
          hit = Enum.find(watched, fn {k, n, _a} -> k == kind and n == name end)

          case hit do
            nil ->
              []

            {_k, _n, arg_key} ->
              expr = Map.get(args, arg_key)

              case expr do
                {:str, s} ->
                  has_run_id? =
                    String.contains?(s, "bdd_run_") or String.contains?(s, "run_id") or String.contains?(s, "#" <> "{")

                  if has_run_id? do
                    []
                  else
                    [
                      %Warning{
                        rule: :unique_risk,
                        scenario_id: scenario_id,
                        tags: tags,
                        file: meta[:file],
                        line: meta[:line],
                        raw: meta[:raw],
                        message:
                          "检测到可能的唯一性风险：#{name}.#{arg_key} 使用常量字符串 #{inspect(s)}。建议用 run_id() 拼接生成唯一值，避免 DataCase 不清库导致冲突/串场。"
                      }
                    ]
                  end

                {:var, _} ->
                  []

                _ ->
                  []
              end
          end

        _ ->
          []
      end)
    end
  end
end
