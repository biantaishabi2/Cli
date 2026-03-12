defmodule BDDCompiler.CLI do
  @moduledoc false

  alias BDDCompiler.{Compiler, CompileError, Config, DslParser, InstructionSet, Linter, MarkdownExtractor}

  def main(argv) when is_list(argv) do
    case argv do
      [] ->
        print_help(:global)
        System.halt(1)

      ["-h"] ->
        print_help(:global)

      ["--help"] ->
        print_help(:global)

      ["help"] ->
        print_help(:global)

      ["help", cmd] ->
        print_help(cmd)

      [cmd | rest] ->
        rest =
          if cmd in ["compile", "lint", "check"] do
            normalize_bool_value_args(rest)
          else
            rest
          end

        if wants_help?(rest) do
          print_help(cmd)
        else
          dispatch(cmd, rest)
        end
    end
  end

  # 兼容常见写法：`--flag true|false`。
  # OptionParser 对 :boolean 开关默认只支持 `--flag` / `--no-flag`，
  # 这里把 `--flag false` 规范化为 `--no-flag`，把 `--flag true` 规范化为 `--flag`。
  defp normalize_bool_value_args(argv) when is_list(argv) do
    argv
    |> Enum.reduce({[], nil}, fn item, {acc, prev} ->
      cond do
        prev == nil ->
          {acc, item}

        item in ["true", "false"] and is_binary(prev) and String.starts_with?(prev, "--") and prev not in ["--help"] ->
          {acc ++ normalize_one_bool_pair(prev, item), nil}

        true ->
          {acc ++ [prev], item}
      end
    end)
    |> then(fn {acc, prev} ->
      if prev == nil, do: acc, else: acc ++ [prev]
    end)
  end

  defp normalize_one_bool_pair(flag, "true") when is_binary(flag), do: [flag]

  defp normalize_one_bool_pair(flag, "false") when is_binary(flag) do
    if String.starts_with?(flag, "--no-") do
      [flag]
    else
      ["--no-" <> String.replace_prefix(flag, "--", "")]
    end
  end

  defp dispatch("compile", args), do: cmd_compile(args)
  defp dispatch("lint", args), do: cmd_lint(args)
  defp dispatch("check", args), do: cmd_check(args)
  defp dispatch("init", args), do: cmd_init(args)
  defp dispatch("annotations.check", args), do: cmd_mix_task("bdd.annotations.check", args)
  defp dispatch("registry.scaffold", args), do: cmd_registry_scaffold(args)
  defp dispatch("registry.upsert", args), do: cmd_registry_upsert(args)
  defp dispatch("instructions.docs", args), do: cmd_mix_task("bdd.instructions_docs", args)
  defp dispatch("factories.scaffold", args), do: cmd_factories_scaffold(args)
  defp dispatch("factories.upsert", args), do: cmd_factories_upsert(args)
  defp dispatch("factories.check", args), do: cmd_factories_check(args)
  defp dispatch("runtime.caps.sync", args), do: cmd_runtime_caps_sync(args)
  defp dispatch("domain.autowire", args), do: cmd_domain_autowire(args)
  defp dispatch("contract.check", args), do: cmd_mix_task("bdd.contract.check", args)
  defp dispatch("fuzz", args), do: cmd_mix_task("bdd.fuzz", args)
  defp dispatch("mutation.report", args), do: cmd_mix_task("bdd.mutation.report", args)
  defp dispatch("mutation.run", args), do: cmd_mix_task("bdd.mutation.run", args)

  defp dispatch(cmd, _args) do
    IO.puts(:stderr, "未知命令: #{cmd}\n")
    print_help(:global)
    System.halt(1)
  end

  defp wants_help?(rest) when is_list(rest) do
    Enum.any?(rest, &(&1 in ["-h", "--help"]))
  end

  defp cmd_compile(args) do
    {opts, _rest, invalid} = parse_common_args(args, [:in, :out, :docs_root, :instructions_v2])

    if invalid != [] do
      IO.puts(:stderr, "参数解析失败: #{inspect(invalid)}")
      System.halt(1)
    end

    project_root = Keyword.get(opts, :project_root, File.cwd!())
    opts = apply_config_defaults(opts, "compile", project_root)
    instruction_set = load_instruction_set!(opts, "compile")

    in_dir = opts |> Keyword.get(:in, "docs/bdd") |> expand_path(project_root)
    out_dir = opts |> Keyword.get(:out, "test/bdd_generated") |> expand_path(project_root)
    docs_root = opts |> Keyword.get(:docs_root) |> expand_optional_path(project_root)

    target = parse_target(Keyword.get(opts, :target, "elixir"))

    # 对于 Go target，--go-module 优先，否则回退到 --module-prefix
    module_prefix =
      if target == :go do
        go_mod = Keyword.get(opts, :go_module)
        fallback = Keyword.get(opts, :module_prefix)

        cond do
          is_binary(go_mod) and go_mod != "" -> go_mod
          is_binary(fallback) and fallback != "" -> fallback
          true ->
            IO.puts(:stderr, "[warn] --target go 未指定 --go-module，将使用 GoEmitter 默认值")
            nil
        end
      else
        Keyword.get(opts, :module_prefix, "BDD.Generated")
      end

    compile_opts =
      [
        docs_root: docs_root,
        target: target,
        runtime_module: Keyword.get(opts, :runtime_module, "BDD.Instructions.V1"),
        test_case: Keyword.get(opts, :test_case, "ExUnit.Case"),
        module_prefix: module_prefix
      ]
      |> Enum.reject(fn {_k, v} -> is_nil(v) end)

    paths = Compiler.compile_dir!(in_dir, out_dir, instruction_set, compile_opts)
    IO.puts("BDD 编译完成：生成 #{length(paths)} 个测试文件到 #{out_dir}")
  rescue
    e in [CompileError, ArgumentError] ->
      IO.puts(:stderr, Exception.message(e))
      System.halt(1)
  end

  defp cmd_lint(args) do
    {opts, _rest, invalid} = parse_common_args(args, [:in, :fail_on_warn, :docs_root, :instructions_v2])

    if invalid != [] do
      IO.puts(:stderr, "参数解析失败: #{inspect(invalid)}")
      System.halt(1)
    end

    project_root = Keyword.get(opts, :project_root, File.cwd!())
    opts = apply_config_defaults(opts, "lint", project_root)
    instruction_set = load_instruction_set!(opts, "lint")
    in_dir = opts |> Keyword.get(:in, "docs/bdd") |> expand_path(project_root)
    fail_on_warn? = Keyword.get(opts, :fail_on_warn, false)
    fail_tags = Keyword.get(opts, :fail_tags)
    fail_globs = Keyword.get(opts, :fail_globs)

    warnings = Linter.lint_dir(in_dir, instruction_set)
    print_warnings(warnings)

    if should_fail_on_warnings?(warnings, fail_on_warn?, fail_tags, fail_globs) do
      System.halt(2)
    end
  rescue
    e in [CompileError, ArgumentError] ->
      IO.puts(:stderr, Exception.message(e))
      System.halt(1)
  end

  defp cmd_check(args) do
    {opts, _rest, invalid} = parse_common_args(args, [:in, :out, :fail_on_warn, :docs_root, :instructions_v2])

    if invalid != [] do
      IO.puts(:stderr, "参数解析失败: #{inspect(invalid)}")
      System.halt(1)
    end

    project_root = Keyword.get(opts, :project_root, File.cwd!())
    opts = apply_config_defaults(opts, "check", project_root)
    instruction_set = load_instruction_set!(opts, "check")
    in_dir = opts |> Keyword.get(:in, "docs/bdd") |> expand_path(project_root)
    out_dir = opts |> Keyword.get(:out, "test/bdd_generated") |> expand_path(project_root)
    docs_root = opts |> Keyword.get(:docs_root) |> expand_optional_path(project_root)
    runtime_module = Keyword.get(opts, :runtime_module, "BDD.Instructions.V1")
    runtime_caps_file = opts |> Keyword.get(:runtime_caps_file) |> expand_optional_path(project_root)
    fail_on_warn? = Keyword.get(opts, :fail_on_warn, true)
    fail_tags = Keyword.get(opts, :fail_tags)
    fail_globs = Keyword.get(opts, :fail_globs)
    exclude_flaky? = Keyword.get(opts, :exclude_flaky, false)
    rerun_failures = Keyword.get(opts, :rerun_failures, 0) || 0
    skip_annotations_check? = Keyword.get(opts, :skip_annotations_check, false)
    skip_bdd_test? = Keyword.get(opts, :skip_bdd_test, false)

    unless skip_annotations_check? do
      run_mix_task_if_exists!("bdd.annotations.check", [], project_root)
    end

    :ok = validate_runtime_coverage!(project_root, in_dir, docs_root, runtime_module, runtime_caps_file)

    compile_opts =
      [
        docs_root: docs_root,
        runtime_module: runtime_module,
        test_case: Keyword.get(opts, :test_case, "ExUnit.Case"),
        module_prefix: Keyword.get(opts, :module_prefix, "BDD.Generated")
      ]
      |> Enum.reject(fn {_k, v} -> is_nil(v) end)

    paths = Compiler.compile_dir!(in_dir, out_dir, instruction_set, compile_opts)
    IO.puts("compiled_out=#{out_dir} count=#{length(paths)}")
    warnings = Linter.lint_dir(in_dir, instruction_set)
    print_warnings(warnings)

    if should_fail_on_warnings?(warnings, fail_on_warn?, fail_tags, fail_globs) do
      System.halt(2)
    end

    unless skip_bdd_test? do
      if File.regular?(Path.join(project_root, "mix.exs")) do
        run_generated_tests!(project_root, out_dir, exclude_flaky?: exclude_flaky?, rerun_failures: rerun_failures)
      else
        IO.puts("[check] skip mix test: mix.exs not found (use in a Mix project to run generated tests)")
      end
    end
  rescue
    e in [CompileError, ArgumentError] ->
      IO.puts(:stderr, Exception.message(e))
      System.halt(1)
  end

  defp should_fail_on_warnings?(warnings, fail_on_warn?, fail_tags, fail_globs)
       when is_list(warnings) and is_boolean(fail_on_warn?) do
    if not fail_on_warn? do
      false
    else
      if warnings == [] do
        false
      else
        tag_set =
          if is_binary(fail_tags) and String.trim(fail_tags) != "" do
            fail_tags
            |> String.split(",", trim: true)
            |> Enum.map(&String.trim/1)
            |> Enum.reject(&(&1 == ""))
            |> Enum.map(&String.to_atom/1)
            |> MapSet.new()
          else
            MapSet.new()
          end

        glob_patterns =
          if is_binary(fail_globs) and String.trim(fail_globs) != "" do
            fail_globs
            |> String.split(",", trim: true)
            |> Enum.map(&String.trim/1)
            |> Enum.reject(&(&1 == ""))
          else
            []
          end

        # 没有指定 tags/globs 时：等价于旧行为（任何 warning 都失败）。
        if MapSet.size(tag_set) == 0 and glob_patterns == [] do
          true
        else
          Enum.any?(warnings, fn w ->
            file = to_string(Map.get(w, :file, "") || "")
            scenario_tags = Map.get(w, :tags, []) || []

            hit_tag? =
              MapSet.size(tag_set) > 0 and Enum.any?(scenario_tags, &MapSet.member?(tag_set, &1))

            hit_glob? =
              if glob_patterns == [] do
                false
              else
                Enum.any?(glob_patterns, fn pat ->
                  pat != "" and glob_match?(pat, file)
                end)
              end

            hit_tag? or hit_glob?
          end)
        end
      end
    end
  end

  defp glob_match?(pattern, path) when is_binary(pattern) and is_binary(path) do
    pat = Path.expand(pattern)
    p = Path.expand(path)

    # 支持 * ? ** 三类通配：
    # - ** 匹配任意路径片段（含 /）
    # - * 匹配单层片段（不含 /）
    # - ? 匹配单字符（不含 /）
    regex =
      pat
      |> Regex.escape()
      # 先处理 **/ 这种形态：允许匹配“0 个或多个目录层级”
      |> String.replace("\\*\\*/", "___GLOBSTAR_SLASH___")
      |> String.replace("\\*\\*", "___GLOBSTAR___")
      |> String.replace("\\*", "[^/]*")
      |> String.replace("\\?", "[^/]")
      |> String.replace("___GLOBSTAR_SLASH___", "(?:.*/)?")
      |> String.replace("___GLOBSTAR___", ".*")
      |> then(&("^" <> &1 <> "$"))

    Regex.match?(Regex.compile!(regex), p)
  end

  defp run_mix_task_if_exists!(task, args, project_root)
       when is_binary(task) and is_list(args) and is_binary(project_root) do
    {out, status} =
      System.cmd("mix", [task | args],
        cd: project_root,
        stderr_to_stdout: true
      )

    if status == 0 do
      IO.write(out)
      :ok
    else
      if String.contains?(out, "could not be found") do
        IO.puts("[check] skip #{task}: mix task not found (use --skip-annotations-check to silence)")
        :ok
      else
        IO.write(out)
        System.halt(status)
      end
    end
  end

  defp run_generated_tests!(project_root, out_dir, opts)
       when is_binary(project_root) and is_binary(out_dir) and is_list(opts) do
    exclude_flaky? = Keyword.get(opts, :exclude_flaky?, false)
    rerun_failures = Keyword.get(opts, :rerun_failures, 0) || 0

    base_args =
      ["test", out_dir] ++
        if(exclude_flaky?, do: ["--exclude", "flaky"], else: [])

    env = [{"MIX_ENV", "test"}]

    case run_mix_cmd(project_root, base_args, env) do
      :ok ->
        :ok

      :error when is_integer(rerun_failures) and rerun_failures > 0 ->
        rerun_failed_tests!(project_root, out_dir, rerun_failures, exclude_flaky?, env)

      :error ->
        System.halt(1)
    end
  end

  defp rerun_failed_tests!(project_root, out_dir, times, exclude_flaky?, env)
       when is_integer(times) and times > 0 do
    args =
      ["test", out_dir, "--failed"] ++
        if(exclude_flaky?, do: ["--exclude", "flaky"], else: [])

    Enum.reduce_while(1..times, :error, fn _i, _acc ->
      case run_mix_cmd(project_root, args, env) do
        :ok -> {:halt, :ok}
        :error -> {:cont, :error}
      end
    end)
    |> case do
      :ok -> :ok
      :error -> System.halt(1)
    end
  end

  defp run_mix_cmd(project_root, args, env) when is_binary(project_root) and is_list(args) and is_list(env) do
    {_out, status} =
      System.cmd("mix", args,
        cd: project_root,
        env: env,
        into: IO.stream(:stdio, :line),
        stderr_to_stdout: true
      )

    if status == 0, do: :ok, else: :error
  end

  defp validate_runtime_coverage!(
         project_root,
         in_dir,
         docs_root_opt,
         runtime_module,
         runtime_caps_file
       ) do
    used = collect_used_instructions!(in_dir, docs_root_opt)
    caps = load_runtime_capabilities!(project_root, runtime_module, runtime_caps_file)

    used_by_name =
      Enum.reduce(used, %{}, fn item, acc ->
        Map.put_new(acc, item.name, item)
      end)

    missing =
      used_by_name
      |> Map.keys()
      |> Enum.reject(&MapSet.member?(caps, &1))
      |> Enum.sort()

    if missing != [] do
      details =
        Enum.map_join(missing, "\n", fn name ->
          m = Map.fetch!(used_by_name, name)
          "- #{inspect(name)} @ #{m.file}:#{m.line} (scenario=#{m.scenario_id})"
        end)

      raise CompileError,
        message:
          "runtime 覆盖校验失败：以下 DSL 指令未被 #{runtime_module} 实现 capabilities/0 覆盖\n" <>
            details,
        file: nil,
        line: nil,
        raw: nil
    end

    :ok
  end

  defp collect_used_instructions!(in_dir, docs_root_opt) do
    docs_root =
      cond do
        is_binary(docs_root_opt) and docs_root_opt != "" -> docs_root_opt
        Path.basename(in_dir) == "bdd" and Path.basename(Path.dirname(in_dir)) == "docs" -> Path.dirname(in_dir)
        true -> nil
      end

    dsl_paths = collect_dsl_paths(in_dir)

    dsl_items =
      Enum.flat_map(dsl_paths, fn path ->
        DslParser.parse_file!(path)
        |> scenarios_to_instruction_items()
      end)

    md_items =
      if is_binary(docs_root) do
        docs_root
        |> Path.join("**/*.md")
        |> Path.wildcard()
        |> Enum.sort()
        |> Enum.flat_map(fn md_path ->
          MarkdownExtractor.extract_blocks!(md_path)
          |> Enum.flat_map(fn b ->
            DslParser.parse_string!(b.content, file: md_path, line_offset: b.start_line - 1)
            |> scenarios_to_instruction_items()
          end)
        end)
      else
        []
      end

    dsl_items ++ md_items
  end

  defp scenarios_to_instruction_items(scenarios) when is_list(scenarios) do
    Enum.flat_map(scenarios, fn s ->
      scenario_id = Map.get(s, :id, "(unknown)")

      Enum.flat_map(Map.get(s, :steps, []), fn
        {kind, name, _args, meta} when kind in [:given, :when, :then] ->
          [
            %{
              name: name,
              kind: kind,
              scenario_id: scenario_id,
              file: meta[:file] || "(unknown)",
              line: meta[:line] || 0
            }
          ]

        _ ->
          []
      end)
    end)
  end

  # organize 目录下优先读 features 聚合 DSL，避免 scenarios 明细导致重复场景。
  defp collect_dsl_paths(in_dir) do
    feature_paths =
      in_dir
      |> Path.join("features/**/*.dsl")
      |> Path.wildcard()
      |> Enum.sort()

    if feature_paths != [] do
      feature_paths
    else
      in_dir
      |> Path.join("**/*.dsl")
      |> Path.wildcard()
      |> Enum.sort()
    end
  end

  defp load_runtime_capabilities!(_project_root, _runtime_module, runtime_caps_file)
       when is_binary(runtime_caps_file) and runtime_caps_file != "" do
    {caps0, _binding} = Code.eval_file(runtime_caps_file)

    caps =
      cond do
        is_list(caps0) -> caps0
        match?(%MapSet{}, caps0) -> MapSet.to_list(caps0)
        is_map(caps0) -> Map.keys(caps0)
        true -> raise "runtime caps file must return list|MapSet|map, got: #{inspect(caps0)}"
      end

    caps
    |> Enum.map(fn
      x when is_atom(x) -> x
      x when is_binary(x) -> String.to_atom(x)
      x -> raise "runtime caps item must be atom|string, got: #{inspect(x)}"
    end)
    |> MapSet.new()
  rescue
    e in CompileError ->
      reraise e, __STACKTRACE__

    e ->
      raise CompileError,
        message:
          "runtime 覆盖校验失败（--runtime-caps-file=#{runtime_caps_file}）：#{Exception.message(e)}",
        file: nil,
        line: nil,
        raw: nil
  end

  defp load_runtime_capabilities!(project_root, runtime_module, _runtime_caps_file) do
    eval = """
    mod = String.to_atom("Elixir." <> "#{runtime_module}")
    unless Code.ensure_loaded?(mod) do
      suffix = mod |> Module.split() |> List.last() |> Macro.underscore()
      explicit = Path.join(["test", "support", "bdd", "instructions_" <> suffix <> ".ex"])
      wildcard = Path.wildcard("test/support/**/*instructions*\#{suffix}.ex")
      candidates =
        ([explicit] ++ wildcard)
        |> Enum.uniq()
        |> Enum.filter(&File.exists?/1)
      if candidates != [] do
        Code.require_file(List.first(candidates))
      end
    end
    unless Code.ensure_loaded?(mod) do
      raise "runtime module not loaded: \#{inspect(mod)}"
    end
    unless function_exported?(mod, :capabilities, 0) do
      raise "runtime module must implement capabilities/0: \#{inspect(mod)}"
    end
    caps0 = mod.capabilities()
    caps1 =
      cond do
        is_list(caps0) -> caps0
        match?(%MapSet{}, caps0) -> MapSet.to_list(caps0)
        is_map(caps0) -> Map.keys(caps0)
        true -> raise "capabilities/0 must return list|MapSet|map, got: \#{inspect(caps0)}"
      end
    caps =
      Enum.map(caps1, fn
        x when is_atom(x) -> x
        x when is_binary(x) -> String.to_atom(x)
        x -> raise "capability item must be atom|string, got: \#{inspect(x)}"
      end)
      |> Enum.sort()
    IO.puts("BDD_RUNTIME_CAPS=" <> inspect(caps, limit: :infinity))
    """

    {out, status} =
      System.cmd("mix", ["run", "--no-start", "-e", eval],
        cd: project_root,
        stderr_to_stdout: true,
        env: [{"MIX_ENV", resolve_mix_env(:runtime)}]
      )

    if status != 0 do
      raise CompileError,
        message:
          "runtime 覆盖校验失败（--project-root=#{project_root}, --runtime-module=#{runtime_module}）：\n#{out}",
        file: nil,
        line: nil,
        raw: nil
    end

    caps_raw = extract_marker_path(out, "BDD_RUNTIME_CAPS=")

    if not is_binary(caps_raw) or String.trim(caps_raw) == "" do
      raise CompileError,
        message: "runtime 覆盖校验失败：未解析到 capabilities 输出",
        file: nil,
        line: nil,
        raw: nil
    end

    {caps, _binding} = Code.eval_string(caps_raw)

    caps
    |> List.wrap()
    |> MapSet.new()
  rescue
    e in CompileError ->
      reraise e, __STACKTRACE__

    e ->
      raise CompileError,
        message:
          "runtime 覆盖校验失败（--project-root=#{project_root}, --runtime-module=#{runtime_module}）：#{Exception.message(e)}",
        file: nil,
        line: nil,
        raw: nil
  end

  defp load_instruction_set!(opts, cfg_section) do
    project_root = Keyword.get(opts, :project_root, File.cwd!())
    registry_module = Keyword.get(opts, :registry_module)
    json_path = Keyword.get(opts, :instructions_json)
    v1_paths = Keyword.get_values(opts, :instructions) |> Enum.map(&expand_path(&1, project_root))
    v2_paths = Keyword.get_values(opts, :instructions_v2) |> Enum.map(&expand_path(&1, project_root))

    # --instructions-json 和 --instructions 互斥
    if json_path != nil and v1_paths != [] do
      raise CompileError,
        message: "--instructions-json 和 --instructions 不能同时使用",
        file: nil,
        line: nil,
        raw: nil
    end

    cond do
      json_path != nil ->
        InstructionSet.load_json!(expand_path(json_path, project_root))

      v1_paths != [] ->
        InstructionSet.load!(v1_paths, v2_paths)

      is_binary(registry_module) and registry_module != "" ->
        {auto_v1_paths, auto_v2_paths} = export_registry_to_temp_files!(project_root, registry_module)
        InstructionSet.load!(auto_v1_paths, auto_v2_paths)

      true ->
        cfg = Config.load(project_root)

        cfg_v1 =
          cfg
          |> Config.get(cfg_section, "instructions")
          |> List.wrap()
          |> Enum.reject(&is_nil/1)
          |> Enum.map(&expand_path(&1, project_root))

        cfg_v2 =
          cfg
          |> Config.get(cfg_section, "instructions_v2")
          |> List.wrap()
          |> Enum.reject(&is_nil/1)
          |> Enum.map(&expand_path(&1, project_root))

        cfg_registry = Config.get(cfg, cfg_section, "registry_module")

        cond do
          cfg_v1 != [] ->
            InstructionSet.load!(cfg_v1, cfg_v2)

          is_binary(cfg_registry) and cfg_registry != "" ->
            {auto_v1_paths, auto_v2_paths} = export_registry_to_temp_files!(project_root, cfg_registry)
            InstructionSet.load!(auto_v1_paths, auto_v2_paths)

          true ->
            raise CompileError,
              message:
                "必须提供指令来源：优先 --instructions；或提供 --registry-module 并配合 --project-root 自动装载；或在 .bddc.toml 配置 instructions/registry_module",
              file: nil,
              line: nil,
              raw: nil
        end
    end
  end

  defp apply_config_defaults(opts, section, project_root) when is_list(opts) and is_binary(section) do
    cfg = Config.load(project_root)

    maybe_put_from_cfg(opts, cfg, section, :registry_module, "registry_module")
    |> maybe_put_from_cfg(cfg, section, :runtime_module, "runtime_module")
    |> maybe_put_from_cfg(cfg, section, :runtime_caps_file, "runtime_caps_file")
    |> maybe_put_from_cfg(cfg, section, :out_meta, "out_meta")
    |> maybe_put_from_cfg(cfg, section, :docs_root, "docs_root")
    |> maybe_put_from_cfg(cfg, section, :in, "in")
    |> maybe_put_from_cfg(cfg, section, :out, "out")
    |> maybe_put_from_cfg(cfg, section, :test_case, "test_case")
    |> maybe_put_from_cfg(cfg, section, :module_prefix, "module_prefix")
    |> maybe_put_from_cfg(cfg, section, :target, "target")
    |> maybe_put_from_cfg(cfg, section, :go_module, "go_module")
    |> maybe_put_runtime_sources_from_cfg(cfg, section, project_root)
    |> maybe_put_instructions_from_cfg(cfg, section, project_root)
  end

  defp maybe_put_from_cfg(opts, cfg, section, opt_key, cfg_key) do
    if Keyword.has_key?(opts, opt_key) do
      opts
    else
      case Config.get(cfg, section, cfg_key) do
        nil -> opts
        "" -> opts
        v -> Keyword.put(opts, opt_key, v)
      end
    end
  end

  defp maybe_put_instructions_from_cfg(opts, cfg, section, project_root) do
    skip_cfg_instructions? =
      Keyword.has_key?(opts, :registry_module) or Keyword.get_values(opts, :instructions) != []

    opts =
      if skip_cfg_instructions? do
        opts
      else
        cfg_v1 = Config.get(cfg, section, "instructions")

        v1_list =
          cfg_v1
          |> List.wrap()
          |> Enum.reject(&is_nil/1)
          |> Enum.map(&expand_path(&1, project_root))

        Enum.reduce(v1_list, opts, fn path, acc -> Keyword.put(acc, :instructions, path) end)
      end

    if skip_cfg_instructions? or Keyword.get_values(opts, :instructions_v2) != [] do
      opts
    else
      cfg_v2 = Config.get(cfg, section, "instructions_v2")

      v2_list =
        cfg_v2
        |> List.wrap()
        |> Enum.reject(&is_nil/1)
        |> Enum.map(&expand_path(&1, project_root))
      Enum.reduce(v2_list, opts, fn path, acc -> Keyword.put(acc, :instructions_v2, path) end)
    end
  end

  defp maybe_put_runtime_sources_from_cfg(opts, cfg, section, project_root) do
    if Keyword.get_values(opts, :runtime_source) == [] do
      cfg_sources = Config.get(cfg, section, "runtime_source")

      list =
        cfg_sources
        |> List.wrap()
        |> Enum.reject(&is_nil/1)
        |> Enum.map(&expand_path(&1, project_root))

      Enum.reduce(list, opts, fn path, acc -> Keyword.put(acc, :runtime_source, path) end)
    else
      opts
    end
  end

  # runtime.caps.sync 的 opts 不应该被 [global] 覆盖（尤其是 out/in/out 等 key 会冲突）。
  # 所以这里提供 local-only 版本，避免把 `compile/check` 的 out 目录错误套到 runtime caps 文件路径上。
  defp maybe_put_from_cfg_local(opts, cfg, section, opt_key, cfg_key) do
    if Keyword.has_key?(opts, opt_key) do
      opts
    else
      case Config.get_local(cfg, section, cfg_key) do
        nil -> opts
        "" -> opts
        v -> Keyword.put(opts, opt_key, v)
      end
    end
  end

  defp maybe_put_runtime_sources_from_cfg_local(opts, cfg, section, project_root) do
    if Keyword.get_values(opts, :runtime_source) == [] do
      cfg_sources = Config.get_local(cfg, section, "runtime_source")

      list =
        cfg_sources
        |> List.wrap()
        |> Enum.reject(&is_nil/1)
        |> Enum.map(&expand_path(&1, project_root))

      Enum.reduce(list, opts, fn path, acc -> Keyword.put(acc, :runtime_source, path) end)
    else
      opts
    end
  end

  defp expand_path(path, project_root) when is_binary(path) and is_binary(project_root) do
    if Path.type(path) == :absolute, do: path, else: Path.expand(path, project_root)
  end

  defp expand_optional_path(nil, _project_root), do: nil
  defp expand_optional_path(path, project_root), do: expand_path(path, project_root)

  # 解析 --target 参数，默认 :elixir
  defp parse_target("elixir"), do: :elixir
  defp parse_target("go"), do: :go

  defp parse_target(other) do
    IO.puts(:stderr, "不支持的 target: #{other}（可选值: elixir, go）")
    System.halt(1)
  end

  defp parse_common_args(args, extra_keys) do
    switches =
      Enum.uniq([
        {:in, :string},
        {:out, :string},
        {:instructions, :keep},
        {:instructions_json, :string},
        {:instructions_v2, :keep},
        {:project_root, :string},
        {:registry_module, :string},
        {:docs_root, :string},
        {:runtime_module, :string},
        {:runtime_caps_file, :string},
        {:test_case, :string},
        {:module_prefix, :string},
        {:fail_on_warn, :boolean},
        {:fail_tags, :string},
        {:fail_globs, :string},
        {:exclude_flaky, :boolean},
        {:rerun_failures, :integer},
        {:skip_annotations_check, :boolean},
        {:skip_bdd_test, :boolean},
        {:target, :string},
        {:go_module, :string}
      ])

    {opts, rest, invalid} = OptionParser.parse(args, switches: switches)

    allowed =
      MapSet.new([
        :in,
        :out,
        :instructions,
        :instructions_json,
        :instructions_v2,
        :project_root,
        :registry_module,
        :docs_root,
        :runtime_module,
        :runtime_caps_file,
        :test_case,
        :module_prefix,
        :fail_on_warn,
        :fail_tags,
        :fail_globs,
        :exclude_flaky,
        :rerun_failures,
        :skip_annotations_check,
        :skip_bdd_test,
        :target,
        :go_module
      ])

    required =
      MapSet.new([:instructions] ++ extra_keys)

    _ = {allowed, required, rest}
    {opts, rest, invalid}
  end

  defp export_registry_to_temp_files!(project_root, registry_module) do
    eval = """
    mod = String.to_atom("Elixir." <> "#{registry_module}")
    unless Code.ensure_loaded?(mod) do
      raise \"registry module not loaded: \#{inspect(mod)}\"
    end
    v1 = mod.all(:v1) |> Enum.into(%{}, fn s -> {s.name, s} end)
    v2 = mod.all(:v2) |> Enum.into(%{}, fn s -> {s.name, s} end)
    v1_path = "/tmp/bdd_compiler_registry_v1_\#{System.os_time(:microsecond)}.exs"
    v2_path = "/tmp/bdd_compiler_registry_v2_\#{System.os_time(:microsecond)}.exs"
    File.write!(v1_path, inspect(v1, pretty: true, limit: :infinity))
    File.write!(v2_path, inspect(v2, pretty: true, limit: :infinity))
    IO.puts("BDD_REGISTRY_V1_PATH=" <> v1_path)
    IO.puts("BDD_REGISTRY_V2_PATH=" <> v2_path)
    """

    {out, status} =
      System.cmd("mix", ["run", "--no-start", "-e", eval],
        cd: project_root,
        stderr_to_stdout: true
      )

    if status != 0 do
      raise CompileError,
        message:
          "自动装载 registry 失败（--project-root=#{project_root}, --registry-module=#{registry_module}）：\n#{out}",
        file: nil,
        line: nil,
        raw: nil
    end

    v1_path = extract_marker_path(out, "BDD_REGISTRY_V1_PATH=")
    v2_path = extract_marker_path(out, "BDD_REGISTRY_V2_PATH=")

    cond do
      not is_binary(v1_path) or v1_path == "" or not File.exists?(v1_path) ->
        raise CompileError,
          message: "自动装载 registry 失败：未生成 v1 临时指令集文件",
          file: nil,
          line: nil,
          raw: nil

      not is_binary(v2_path) or v2_path == "" or not File.exists?(v2_path) ->
        raise CompileError,
          message: "自动装载 registry 失败：未生成 v2 临时指令集文件",
          file: nil,
          line: nil,
          raw: nil

      true ->
        {[v1_path], [v2_path]}
    end
  rescue
    e in CompileError ->
      reraise e, __STACKTRACE__

    e ->
      raise CompileError,
        message:
          "自动装载 registry 失败（--project-root=#{project_root}, --registry-module=#{registry_module}）：#{Exception.message(e)}",
        file: nil,
        line: nil,
        raw: nil
  end

  defp extract_marker_path(out, marker) when is_binary(out) and is_binary(marker) do
    out
    |> String.split("\n")
    |> Enum.find_value(fn line ->
      if String.starts_with?(line, marker) do
        String.replace_prefix(line, marker, "")
      else
        nil
      end
    end)
  end

  defp cmd_mix_task(task, argv) when is_binary(task) and is_list(argv) do
    {project_root, passthrough} = pop_project_root(argv)
    run_mix_task!(task, passthrough, project_root)
  rescue
    e ->
      IO.puts(:stderr, "运行 mix 任务失败: #{Exception.message(e)}")
      System.halt(1)
  end

  defp cmd_init(args) do
    {opts, _rest, invalid} =
      OptionParser.parse(args,
        strict: [
          project_root: :string,
          namespace: :string,
          force: :boolean,
          dry_run: :boolean
        ]
      )

    if invalid != [] do
      IO.puts(:stderr, "参数解析失败: #{inspect(invalid)}")
      System.halt(1)
    end

    project_root = Keyword.get(opts, :project_root, File.cwd!())
    force? = Keyword.get(opts, :force, false)
    dry_run? = Keyword.get(opts, :dry_run, false)

    namespace =
      Keyword.get(opts, :namespace) ||
        guess_namespace(project_root) ||
        "MyApp"

    runtime_module = "#{namespace}.BDD.Instructions.V1"
    module_prefix = "#{namespace}.BDD.Generated"

    plan = init_plan(project_root, namespace, runtime_module, module_prefix)

    if dry_run? do
      IO.puts("[init] dry-run project_root=#{project_root} namespace=#{namespace}")

      Enum.each(plan, fn %{path: path, action: action} ->
        IO.puts("[init] #{action} path=#{path}")
      end)

      System.halt(0)
    end

    Enum.each(plan, fn %{path: path, content: content, action: action} ->
      cond do
        File.exists?(path) and not force? ->
          IO.puts(:stderr, "[init] skip_existing path=#{path} (use --force to overwrite)")

        true ->
          File.mkdir_p!(Path.dirname(path))
          File.write!(path, content)
          IO.puts("[init] #{action} path=#{path}")
      end
    end)

    IO.puts("[init] done")
  rescue
    e ->
      IO.puts(:stderr, "[init] failed: #{Exception.message(e)}")
      System.halt(1)
  end

  defp guess_namespace(project_root) do
    mix_path = Path.join(project_root, "mix.exs")

    if File.regular?(mix_path) do
      mix_path
      |> File.read!()
      |> then(fn src ->
        case Regex.run(~r/defmodule\s+([A-Za-z0-9_]+)\.MixProject\s+do/, src) do
          [_, mod] -> mod
          _ -> nil
        end
      end)
    else
      nil
    end
  end

  defp init_plan(project_root, namespace, runtime_module, module_prefix) do
    toml = init_toml(namespace, runtime_module, module_prefix)

    [
      %{
        action: "write",
        path: Path.join(project_root, ".bddc.toml"),
        content: toml
      },
      %{
        action: "write",
        path: Path.join([project_root, "priv", "bdd", "instructions_v1.exs"]),
        content: init_instructions_v1_exs()
      },
      %{
        action: "write",
        path: Path.join([project_root, "test", "support", "bdd", "bddc_runtime.ex"]),
        content: init_runtime_macro_ex(namespace)
      },
      %{
        action: "write",
        path: Path.join([project_root, "test", "support", "bdd", "common_instructions.ex"]),
        content: init_common_instructions_ex(namespace)
      },
      %{
        action: "write",
        path: Path.join([project_root, "test", "support", "bdd", "instructions_v1.ex"]),
        content: init_runtime_dispatcher_ex(runtime_module, namespace)
      },
      %{
        action: "write",
        path: Path.join([project_root, "docs", "bdd", "hello.dsl"]),
        content: init_hello_dsl()
      },
      %{
        action: "write",
        path: Path.join([project_root, "scripts", "bdd_gate.sh"]),
        content: init_bdd_gate_sh(runtime_module)
      }
    ]
  end

  defp init_toml(namespace, runtime_module, module_prefix) do
    """
    [global]
    namespace = "#{namespace}"
    instructions = ["priv/bdd/instructions_v1.exs"]
    in = "docs/bdd"
    out = "test/bdd_generated"
    docs_root = "docs"
    runtime_module = "#{runtime_module}"
    runtime_caps_file = "docs/bdd/_generated/runtime_caps_v1.exs"
    test_case = "ExUnit.Case"
    module_prefix = "#{module_prefix}"

    [runtime.caps.sync]
    out = "docs/bdd/_generated/runtime_caps_v1.exs"
    """
  end

  defp init_instructions_v1_exs do
    """
    # bddc instructions spec (v1)
    #
    # 说明：
    # - 该文件必须返回一个 map：%{instruction_atom => spec_map}
    # - 你可以手工维护，也可以预留 GENERATED 区域由 bddc registry.upsert 写入。

    %{
      # BEGIN BDDC GENERATED
      # END BDDC GENERATED

      create_temp_dir: %{
        kind: :given,
        args: %{key: %{type: :string, required?: true, allowed: nil}},
        outputs: %{path: :string},
        rules: [],
        scopes: [:integration, :e2e],
        boundary: :test_runtime,
        async?: false,
        eventually?: false,
        assert_class: nil
      },
      create_temp_file: %{
        kind: :given,
        args: %{
          dir: %{type: :string, required?: true, allowed: nil},
          filename: %{type: :string, required?: true, allowed: nil},
          content: %{type: :string, required?: false, allowed: nil}
        },
        outputs: %{path: :string},
        rules: [],
        scopes: [:integration, :e2e],
        boundary: :test_runtime,
        async?: false,
        eventually?: false,
        assert_class: nil
      },
      noop: %{
        kind: :when,
        args: %{},
        outputs: %{},
        rules: [],
        scopes: [:unit, :integration, :e2e],
        boundary: :test_runtime,
        async?: false,
        eventually?: false,
        assert_class: nil
      },
      assert_noop: %{
        kind: :then,
        args: %{},
        outputs: %{},
        rules: [],
        scopes: [:unit, :integration, :e2e],
        boundary: :test_runtime,
        async?: false,
        eventually?: false,
        assert_class: :weak
      }
    }
    """
  end

  defp init_runtime_macro_ex(namespace) do
    """
    defmodule #{namespace}.BDDC.Runtime do
      @moduledoc false

      defmacro __using__(opts) do
        common_mod = Keyword.fetch!(opts, :common_module)

        quote do
          @common_module unquote(common_mod)

          def capabilities do
            MapSet.new() |> MapSet.union(@common_module.capabilities())
          end

          # For runtime.caps.sync: keep an explicit, machine-readable pattern surface.
          # When you add new instructions, append a clause here.
          def __caps_sync_fixture__(tuple) do
            case tuple do
              {:given, :create_temp_dir} -> :ok
              {:given, :create_temp_file} -> :ok
              {:when, :noop} -> :ok
              {:then, :assert_noop} -> :ok
            end
          end

          def new_run_id do
            Base.encode16(:crypto.strong_rand_bytes(8), case: :lower)
          end

          def get!(ctx, key, meta) when is_map(ctx) and is_atom(key) do
            case Map.fetch(ctx, key) do
              {:ok, v} -> v
              :error -> raise \"missing ctx var: \#{inspect(key)} meta=\#{inspect(meta)}\"
            end
          end

          def run_step!(ctx, kind, name, args, meta, _dsl_line)
              when is_map(ctx) and kind in [:given, :when, :then] and is_atom(name) and is_map(args) do
            @common_module.run!(ctx, kind, name, args, meta)
          end
        end
      end
    end
    """
  end

  defp init_common_instructions_ex(namespace) do
    """
    defmodule #{namespace}.BDD.CommonInstructions do
      @moduledoc false

      @caps MapSet.new([:create_temp_dir, :create_temp_file, :noop, :assert_noop])
      def capabilities, do: @caps

      def run!(ctx, :given, :create_temp_dir, %{key: key}, _meta) when is_binary(key) do
        base = System.tmp_dir!()
        path = Path.join(base, \"bddc_\" <> key <> \"_\" <> (ctx[:run_id] || \"no_run\"))
        File.mkdir_p!(path)
        Map.merge(ctx, %{path: path})
      end

      def run!(ctx, :given, :create_temp_file, %{dir: dir, filename: filename} = args, _meta)
          when is_binary(dir) and is_binary(filename) do
        content = Map.get(args, :content, \"\")
        path = Path.join(dir, filename)
        File.mkdir_p!(Path.dirname(path))
        File.write!(path, content)
        Map.merge(ctx, %{path: path})
      end

      def run!(ctx, :then, :assert_noop, %{}, _meta) do
        ctx
      end

      def run!(ctx, :when, :noop, %{}, _meta) do
        ctx
      end

      def run!(_ctx, kind, name, _args, meta) do
        raise \"unimplemented instruction: \#{inspect({kind, name})} meta=\#{inspect(meta)}\"
      end
    end
    """
  end

  defp init_runtime_dispatcher_ex(runtime_module, namespace) do
    """
    defmodule #{runtime_module} do
      @moduledoc false

      use #{namespace}.BDDC.Runtime, common_module: #{namespace}.BDD.CommonInstructions

      # For runtime.caps.sync: keep an explicit, machine-readable pattern surface.
      # When you add new instructions, append a clause here.
      def __caps_sync_fixture__(tuple) do
        case tuple do
          {:given, :create_temp_dir} -> :ok
          {:given, :create_temp_file} -> :ok
          {:when, :noop} -> :ok
          {:then, :assert_noop} -> :ok
        end
      end
    end
    """
  end

  defp init_hello_dsl do
    """
    [SCENARIO: HELLO-001] TITLE: hello world TAGS: integration
    GIVEN create_temp_dir key=\"hello\"
    WHEN noop
    THEN assert_noop
    """
  end

  defp init_bdd_gate_sh(runtime_module) do
    """
    #!/usr/bin/env bash
    set -euo pipefail

    bddc runtime.caps.sync \\
      --project-root . \\
      --runtime-module #{runtime_module} \\
      --out docs/bdd/_generated/runtime_caps_v1.exs

    bddc check \\
      --project-root . \\
      --runtime-module #{runtime_module} \\
      --runtime-caps-file docs/bdd/_generated/runtime_caps_v1.exs
    """
  end

  defp cmd_registry_scaffold(args) do
    {project_root, passthrough} = pop_project_root(args)

    {opts, _rest, invalid} =
      OptionParser.parse(passthrough,
        strict: [
          standalone: :boolean,
          src: :string,
          out: :string,
          module: :string,
          functions: :string,
          prefix: :string,
          kind: :string,
          version: :string
        ]
      )

    if invalid != [] do
      IO.puts(:stderr, "参数解析失败: #{inspect(invalid)}")
      System.halt(1)
    end

    if Keyword.get(opts, :standalone, false) do
      cmd_registry_scaffold_standalone(project_root, opts)
    else
      # Keep backward compatibility: try mix task first; if missing, fallback to standalone.
      {out, status} =
        System.cmd("mix", ["bdd.registry.scaffold" | passthrough],
          cd: project_root,
          stderr_to_stdout: true
        )

      if status == 0 do
        IO.write(out)
      else
        if String.contains?(out, "could not be found") do
          cmd_registry_scaffold_standalone(project_root, opts)
        else
          IO.write(out)
          System.halt(status)
        end
      end
    end
  end

  defp cmd_registry_scaffold_standalone(project_root, opts) do
    src = Keyword.get(opts, :src, "lib")

    out_path =
      opts
      |> Keyword.get(:out, Path.join(["priv", "bdd", "_generated", "instructions_v1_scaffold.exs"]))
      |> expand_path(project_root)

    specs =
      project_root
      |> Path.join(Path.join(src, "**/*.ex"))
      |> Path.wildcard()
      |> Enum.sort()
      |> Enum.flat_map(&extract_bdd_instruction_attrs_from_file!/1)
      |> Enum.reduce(%{}, fn spec, acc ->
        name = Map.fetch!(spec, :name)

        if Map.has_key?(acc, name) do
          raise ArgumentError, "registry.scaffold 冲突: #{inspect(name)}"
        end

        Map.put(acc, name, Map.delete(spec, :name))
      end)

    File.mkdir_p!(Path.dirname(out_path))
    File.write!(out_path, inspect(specs, pretty: true, limit: :infinity))
    IO.puts("generated=#{out_path} count=#{map_size(specs)} mode=standalone")
  end

  defp extract_bdd_instruction_attrs_from_file!(path) do
    src = File.read!(path)

    {:ok, ast} =
      case Code.string_to_quoted(src, file: path) do
        {:ok, quoted} -> {:ok, quoted}
        {:error, {line, error, token}} -> raise ArgumentError, "#{path}:#{line} #{error} #{inspect(token)}"
      end

    {_ast, specs} =
      Macro.prewalk(ast, [], fn
        {:@, _meta, [{:bdd_instruction, _m2, [value_ast]}]} = node, acc ->
          spec = literal_to_term!(value_ast)
          {node, [spec | acc]}

        node, acc ->
          {node, acc}
      end)

    specs
    |> Enum.reverse()
    |> Enum.map(fn spec ->
      # normalize minimal: require name/kind/args; other fields optional
      spec
      |> Map.new(fn {k, v} when is_atom(k) -> {k, v} end)
      |> Map.put_new(:args, %{})
      |> Map.put_new(:outputs, %{})
      |> Map.put_new(:rules, [])
      |> Map.put_new(:scopes, [:integration, :e2e])
      |> Map.put_new(:boundary, :service)
      |> Map.put_new(:async?, false)
      |> Map.put_new(:eventually?, false)
      |> Map.put_new(:assert_class, nil)
    end)
  end

  defp literal_to_term!(ast) do
    case ast do
      x when is_atom(x) or is_binary(x) or is_integer(x) or is_float(x) or is_boolean(x) or is_nil(x) ->
        x

      {:%{}, _m, kvs} ->
        Map.new(kvs, fn {k, v} -> {literal_to_term!(k), literal_to_term!(v)} end)

      {:{}, _m, items} ->
        items |> Enum.map(&literal_to_term!/1) |> List.to_tuple()

      list when is_list(list) ->
        Enum.map(list, &literal_to_term!/1)

      _ ->
        raise ArgumentError, "仅支持字面量注解（atom/string/number/bool/nil/list/map/tuple），收到: #{Macro.to_string(ast)}"
    end
  end

  defp cmd_registry_upsert(args) do
    {project_root, passthrough} = pop_project_root(args)

    {opts, _rest, invalid} =
      OptionParser.parse(passthrough,
        strict: [
          standalone: :boolean,
          scaffold: :string,
          target: :string,
          module: :string,
          functions: :string,
          prefix: :string,
          kind: :string,
          version: :string
        ]
      )

    if invalid != [] do
      IO.puts(:stderr, "参数解析失败: #{inspect(invalid)}")
      System.halt(1)
    end

    if Keyword.get(opts, :standalone, false) do
      cmd_registry_upsert_standalone(project_root, opts)
    else
      {out, status} =
        System.cmd("mix", ["bdd.registry.upsert" | passthrough],
          cd: project_root,
          stderr_to_stdout: true
        )

      if status == 0 do
        IO.write(out)
      else
        if String.contains?(out, "could not be found") do
          cmd_registry_upsert_standalone(project_root, opts)
        else
          IO.write(out)
          System.halt(status)
        end
      end
    end
  end

  defp cmd_registry_upsert_standalone(project_root, opts) do
    scaffold_path =
      opts
      |> Keyword.get(:scaffold, Path.join(["priv", "bdd", "_generated", "instructions_v1_scaffold.exs"]))
      |> expand_path(project_root)

    target_path =
      opts
      |> Keyword.get(:target, Path.join(["priv", "bdd", "instructions_v1.exs"]))
      |> expand_path(project_root)

    {scaffold, _binding} = Code.eval_file(scaffold_path)

    unless is_map(scaffold) do
      raise ArgumentError, "scaffold 文件必须返回 map: #{scaffold_path}"
    end

    target = File.read!(target_path)

    start_marker = "# BEGIN BDDC GENERATED"
    end_marker = "# END BDDC GENERATED"

    unless String.contains?(target, start_marker) and String.contains?(target, end_marker) do
      raise ArgumentError, "target 文件缺少 GENERATED 标记：#{target_path}"
    end

    generated_block =
      scaffold
      |> Enum.sort_by(fn {k, _v} -> Atom.to_string(k) end)
      |> Enum.map(fn {k, v} ->
        key = Atom.to_string(k)
        "  #{key}: " <> inspect(v, pretty: true, limit: :infinity) <> ","
      end)
      |> Enum.join("\n")

    replaced =
      Regex.replace(
        ~r/#{Regex.escape(start_marker)}.*?#{Regex.escape(end_marker)}/s,
        target,
        "#{start_marker}\n#{generated_block}\n  #{end_marker}"
      )

    File.write!(target_path, replaced)
    IO.puts("upserted=#{map_size(scaffold)} target=#{target_path} mode=standalone")
  end

  defp run_mix_task!(task, passthrough, project_root)
       when is_binary(task) and is_list(passthrough) and is_binary(project_root) do
    {_out, status} =
      System.cmd("mix", [task | passthrough],
        cd: project_root,
        into: IO.stream(:stdio, :line),
        stderr_to_stdout: true
      )

    if status != 0 do
      System.halt(status)
    end
  end

  defp cmd_factories_scaffold(args) do
    {opts, _rest, invalid} =
      OptionParser.parse(args,
        strict: [project_root: :string, scope: :string, out: :string]
      )

    if invalid != [] do
      IO.puts(:stderr, "参数解析失败: #{inspect(invalid)}")
      System.halt(1)
    end

    project_root = Keyword.get(opts, :project_root, File.cwd!())
    scope_opt = Keyword.get(opts, :scope)
    out_opt = Keyword.get(opts, :out)

    eval = """
    Mix.Task.run("app.start")
    scope = Shop.BDD.Factories.Scope.load!(#{inspect(scope_opt)})
    out_dir = #{inspect(out_opt)} || scope.default_out_dir
    files = Shop.BDD.Factories.Generator.generate_files!(scope.schemas)
    Enum.each(files, fn %{path: rel, content: content} ->
      path = Path.join(out_dir, Path.basename(rel))
      File.mkdir_p!(Path.dirname(path))
      File.write!(path, content)
    end)
    IO.puts("generated=" <> Integer.to_string(length(files)) <> " out_dir=" <> to_string(out_dir))
    """

    run_mix_eval!(project_root, eval)
  end

  defp cmd_factories_upsert(args) do
    {opts, _rest, invalid} =
      OptionParser.parse(args,
        strict: [project_root: :string, scope: :string]
      )

    if invalid != [] do
      IO.puts(:stderr, "参数解析失败: #{inspect(invalid)}")
      System.halt(1)
    end

    project_root = Keyword.get(opts, :project_root, File.cwd!())
    scope_opt = Keyword.get(opts, :scope)

    eval = """
    Mix.Task.run("app.start")
    scope = Shop.BDD.Factories.Scope.load!(#{inspect(scope_opt)})
    out_dir = scope.default_out_dir
    files = Shop.BDD.Factories.Generator.generate_files!(scope.schemas)
    Enum.each(files, fn %{path: rel, content: content} ->
      path = Path.join(out_dir, Path.basename(rel))
      File.mkdir_p!(Path.dirname(path))
      File.write!(path, content)
    end)
    IO.puts("upserted=" <> Integer.to_string(length(files)) <> " out_dir=" <> to_string(out_dir))
    """

    run_mix_eval!(project_root, eval)
  end

  defp cmd_factories_check(args) do
    {opts, _rest, invalid} =
      OptionParser.parse(args,
        strict: [project_root: :string, paths: :keep]
      )

    if invalid != [] do
      IO.puts(:stderr, "参数解析失败: #{inspect(invalid)}")
      System.halt(1)
    end

    project_root = Keyword.get(opts, :project_root, File.cwd!())

    provided_paths = Keyword.get_values(opts, :paths)

    paths =
      case provided_paths do
        [] -> ["test/support/bdd/factories_generated", "test/support/bdd/semantic_givens"]
        list -> list
      end

    paths
    |> Enum.map(&Path.expand(&1, project_root))
    |> check_factory_paths!()

    IO.puts("ok paths=" <> inspect(paths))
  end

  defp cmd_runtime_caps_sync(args) do
    {opts, _rest, invalid} =
      OptionParser.parse(args,
        strict: [
          project_root: :string,
          runtime_module: :string,
          runtime_source: :keep,
          out: :string,
          out_meta: :string
        ]
      )

    if invalid != [] do
      IO.puts(:stderr, "参数解析失败: #{inspect(invalid)}")
      System.halt(1)
    end

    project_root = Keyword.get(opts, :project_root, File.cwd!())
    opts = apply_runtime_caps_sync_config(opts, project_root)
    runtime_module = Keyword.get(opts, :runtime_module, "BDD.Instructions.V1")
    suffix = runtime_module_suffix(runtime_module)

    runtime_sources =
      case Keyword.get_values(opts, :runtime_source) do
        [] -> find_runtime_source_files!(project_root, suffix)
        list -> Enum.map(list, &expand_path(&1, project_root))
      end

    out_path =
      opts
      |> Keyword.get(:out, Path.join(["docs", "bdd", "_generated", "runtime_caps_#{suffix}.exs"]))
      |> expand_path(project_root)

    {caps, meta} = extract_runtime_caps_multi!(runtime_sources)
    File.mkdir_p!(Path.dirname(out_path))
    File.write!(out_path, inspect(caps, pretty: true, limit: :infinity))

    out_meta_path =
      opts
      |> Keyword.get(:out_meta)
      |> expand_optional_path(project_root)

    if is_binary(out_meta_path) and out_meta_path != "" do
      File.mkdir_p!(Path.dirname(out_meta_path))
      File.write!(out_meta_path, inspect(meta, pretty: true, limit: :infinity))
    end

    IO.puts("generated=#{out_path} count=#{length(caps)} sources=#{inspect(runtime_sources)} out_meta=#{inspect(out_meta_path)}")
  rescue
    e in [CompileError, ArgumentError] ->
      IO.puts(:stderr, Exception.message(e))
      System.halt(1)
  end

  defp apply_runtime_caps_sync_config(opts, project_root) when is_list(opts) and is_binary(project_root) do
    cfg = Config.load(project_root)
    section = "runtime.caps.sync"

    opts
    |> maybe_put_from_cfg_local(cfg, section, :runtime_module, "runtime_module")
    |> maybe_put_runtime_sources_from_cfg_local(cfg, section, project_root)
    |> maybe_put_from_cfg_local(cfg, section, :out, "out")
    |> maybe_put_from_cfg_local(cfg, section, :out_meta, "out_meta")
  end

  defp cmd_domain_autowire(args) do
    {opts, _rest, invalid} =
      OptionParser.parse(args,
        strict: [
          project_root: :string,
          module: :string,
          functions: :string,
          prefix: :string,
          kind: :string,
          version: :string,
          registry_module: :string,
          runtime_module: :string,
          runtime_caps_file: :string,
          docs_root: :string,
          in: :string,
          out: :string,
          fail_on_warn: :string,
          strict: :string,
          strict_factories: :string,
          sync_runtime_caps: :boolean,
          dry_run: :boolean,
          skip_annotations_check: :boolean,
          skip_registry: :boolean,
          skip_docs: :boolean,
          skip_factories_check: :boolean
        ]
      )

    if invalid != [] do
      IO.puts(:stderr, "参数解析失败: #{inspect(invalid)}")
      System.halt(1)
    end

    project_root = Keyword.get(opts, :project_root, File.cwd!())
    opts = apply_config_defaults(opts, "domain.autowire", project_root)
    skip_annotations_check? = Keyword.get(opts, :skip_annotations_check, false)
    skip_registry? = Keyword.get(opts, :skip_registry, false)
    skip_docs? = Keyword.get(opts, :skip_docs, false)
    dry_run? = Keyword.get(opts, :dry_run, false)
    module = Keyword.get(opts, :module)
    functions = Keyword.get(opts, :functions)

    strict? = resolve_strict!(opts)
    strict_factories? = resolve_strict_factories!(opts)
    skip_factories_check? = Keyword.get(opts, :skip_factories_check, false)
    sync_runtime_caps? = Keyword.get(opts, :sync_runtime_caps, false)

    check_args =
      build_check_args(opts, project_root)
      |> maybe_put_fail_on_warn(opts, strict?)

    factories_check_args = build_factories_check_args(project_root, strict_factories?)

    if dry_run? do
      print_domain_autowire_dry_run(
        skip_annotations_check?,
        skip_registry?,
        skip_docs?,
        skip_factories_check?,
        check_args,
        factories_check_args
      )

      System.halt(0)
    end

    if not skip_annotations_check? do
      IO.puts("[autowire] step=annotations.check")
      run_mix_task!("bdd.annotations.check", [], project_root)
    end

    if not skip_registry? do
      if is_nil(module) or module == "" do
        IO.puts(:stderr, "参数缺失: --module（或使用 --skip-registry）")
        System.halt(1)
      end

      if is_nil(functions) or functions == "" do
        IO.puts(:stderr, "参数缺失: --functions（或使用 --skip-registry）")
        System.halt(1)
      end

      registry_args = build_registry_args(opts)

      IO.puts("[autowire] step=registry.scaffold")
      cmd_registry_scaffold(["--project-root", project_root | registry_args])

      IO.puts("[autowire] step=registry.upsert")
      cmd_registry_upsert(["--project-root", project_root | registry_args])
    end

    if not skip_docs? do
      IO.puts("[autowire] step=instructions.docs")
      run_mix_task!("bdd.instructions_docs", [], project_root)
    end

    if sync_runtime_caps? do
      runtime_module = Keyword.get(opts, :runtime_module, "Shop.BDD.Instructions.V1")

      IO.puts("[autowire] step=runtime.caps.sync")

      cmd_runtime_caps_sync([
        "--project-root",
        project_root,
        "--runtime-module",
        runtime_module,
        "--out",
        default_runtime_caps_path(project_root, runtime_module, Keyword.get(opts, :runtime_caps_file))
      ])
    end

    IO.puts("[autowire] step=check")
    cmd_check(check_args)

    if not skip_factories_check? do
      IO.puts("[autowire] step=factories.check")
      cmd_factories_check(factories_check_args)
    end

    print_acceptance_report_template(project_root, check_args)
  end

  defp build_registry_args(opts) do
    []
    |> put_opt("--module", Keyword.get(opts, :module))
    |> put_opt("--functions", Keyword.get(opts, :functions))
    |> put_opt("--prefix", Keyword.get(opts, :prefix))
    |> put_opt("--kind", Keyword.get(opts, :kind))
    |> put_opt("--version", Keyword.get(opts, :version))
  end

  defp build_check_args(opts, project_root) do
    []
    |> put_opt("--project-root", project_root)
    |> put_opt("--registry-module", Keyword.get(opts, :registry_module))
    |> put_opt("--runtime-module", Keyword.get(opts, :runtime_module))
    |> put_opt("--runtime-caps-file", Keyword.get(opts, :runtime_caps_file))
    |> put_opt("--docs-root", Keyword.get(opts, :docs_root))
    |> put_opt("--in", Keyword.get(opts, :in))
    |> put_opt("--out", Keyword.get(opts, :out))
  end

  defp default_runtime_caps_path(project_root, runtime_module, nil) do
    suffix = runtime_module_suffix(runtime_module)
    Path.expand(Path.join(["docs", "bdd", "_generated", "runtime_caps_#{suffix}.exs"]), project_root)
  end

  defp default_runtime_caps_path(project_root, _runtime_module, path) when is_binary(path) do
    expand_path(path, project_root)
  end

  defp maybe_put_fail_on_warn(args, opts, strict?) do
    case Keyword.fetch(opts, :fail_on_warn) do
      {:ok, "true"} -> args ++ ["--fail-on-warn"]
      {:ok, "false"} -> args ++ ["--no-fail-on-warn"]
      {:ok, value} ->
        IO.puts(:stderr, "参数解析失败: --fail-on-warn 仅支持 true/false，收到 #{inspect(value)}")
        System.halt(1)

      :error -> if(strict?, do: args ++ ["--fail-on-warn"], else: args ++ ["--no-fail-on-warn"])
    end
  end

  defp resolve_strict!(opts) do
    case Keyword.fetch(opts, :strict) do
      {:ok, "true"} -> true
      {:ok, "false"} -> false
      {:ok, value} ->
        IO.puts(:stderr, "参数解析失败: --strict 仅支持 true/false，收到 #{inspect(value)}")
        System.halt(1)

      :error -> true
    end
  end

  defp resolve_strict_factories!(opts) do
    case Keyword.fetch(opts, :strict_factories) do
      {:ok, "true"} -> true
      {:ok, "false"} -> false
      {:ok, value} ->
        IO.puts(:stderr, "参数解析失败: --strict-factories 仅支持 true/false，收到 #{inspect(value)}")
        System.halt(1)

      :error -> false
    end
  end

  defp build_factories_check_args(project_root, strict_factories?) do
    args = ["--project-root", project_root, "--paths", "test/support/bdd/factories_generated"]

    if strict_factories? do
      args ++ ["--paths", "test/support/bdd/semantic_givens"]
    else
      args
    end
  end

  defp runtime_module_suffix(runtime_module) when is_binary(runtime_module) do
    runtime_module
    |> String.split(".")
    |> List.last()
    |> Macro.underscore()
  end

  defp find_runtime_source_files!(project_root, suffix) when is_binary(project_root) and is_binary(suffix) do
    explicit = Path.join([project_root, "test", "support", "bdd", "instructions_#{suffix}.ex"])

    wildcard =
      Path.wildcard(Path.join([project_root, "test", "support", "**", "*instructions*#{suffix}.ex"]))

    candidates =
      ([explicit] ++ wildcard)
      |> Enum.uniq()
      |> Enum.filter(&File.exists?/1)

    case candidates do
      [] ->
        raise ArgumentError,
              "未找到 runtime 源文件，请传 --runtime-source，默认查找 test/support/**/instructions*#{suffix}.ex"

      list ->
        Enum.sort(list)
    end
  end

  defp extract_runtime_caps_multi!(runtime_sources) when is_list(runtime_sources) do
    entries =
      runtime_sources
      |> Enum.flat_map(fn path ->
        extract_runtime_entries!(path)
      end)
      |> Enum.uniq()
      |> Enum.sort()

    validate_runtime_caps_entries!(entries)

    caps =
      entries
      |> Enum.map(fn {_kind, name, _file, _line} -> name end)
      |> Enum.uniq()
      |> Enum.sort()

    meta =
      entries
      |> Enum.group_by(fn {_kind, name, _file, _line} -> name end, fn {kind, _name, file, line} ->
        %{kind: kind, file: file, line: line}
      end)

    {caps, meta}
  end

  defp extract_runtime_entries!(runtime_source) when is_binary(runtime_source) do
    source = File.read!(runtime_source)

    ast =
      case Code.string_to_quoted(source, file: runtime_source) do
        {:ok, quoted} ->
          quoted

        {:error, {line, error, token}} ->
          raise ArgumentError,
                "runtime 源文件语法解析失败: #{runtime_source}:#{line} #{error} #{inspect(token)}"
      end

    ast
    |> runtime_caps_entries_from_ast()
    |> Enum.map(fn {kind, name, line} -> {kind, name, runtime_source, line} end)
  end

  defp runtime_caps_entries_from_ast(ast) do
    {_ast, entries} =
      Macro.prewalk(ast, [], fn
        {:def, meta, [{name, _m2, args_ast}, _body]} = node, acc when name in [:run!, :run_step!] ->
          line = meta[:line] || 0
          extracted = runtime_entries_from_fun_args(args_ast, line)
          {node, extracted ++ acc}

        {:->, meta, [patterns, _body]} = node, acc ->
          line = meta[:line] || 0

          extracted =
            patterns
            |> List.wrap()
            |> Enum.flat_map(&runtime_entries_from_pattern(&1, line))

          {node, extracted ++ acc}

        node, acc ->
          {node, acc}
      end)

    entries
  end

  defp runtime_entries_from_fun_args(args_ast, line) when is_list(args_ast) do
    args_ast
    |> Enum.with_index()
    |> Enum.flat_map(fn {arg, idx} ->
      case arg do
        kind when kind in [:given, :when, :then] ->
          next = Enum.at(args_ast, idx + 1)

          case next do
            name when is_atom(name) -> [{kind, name, line}]
            _ -> []
          end

        _ ->
          []
      end
    end)
    |> Enum.uniq()
  end

  defp runtime_entries_from_fun_args(_args_ast, _line), do: []

  defp runtime_entries_from_pattern(pattern, default_line) do
    {_pattern, entries} =
      Macro.prewalk(pattern, [], fn
        {kind, name} = node, acc when kind in [:given, :when, :then] ->
          {node, [{kind, name, default_line} | acc]}

        {:{}, meta, [kind, name]} = node, acc when kind in [:given, :when, :then] ->
          line = meta[:line] || default_line || 0
          {node, [{kind, name, line} | acc]}

        node, acc ->
          {node, acc}
      end)

    entries
  end

  defp validate_runtime_caps_entries!([]) do
    raise ArgumentError, "未提取到任何 runtime 指令：请检查 {:given|:when|:then, :instruction} -> 子句"
  end

  defp validate_runtime_caps_entries!(entries) do
    invalid_name =
      Enum.find(entries, fn {_kind, name, _file, _line} ->
        not runtime_instruction_name_valid?(name)
      end)

    if invalid_name do
      {kind, name, file, line} = invalid_name

      raise ArgumentError,
            "runtime 指令名非法: #{inspect(name)} (kind=#{kind}) at #{file}:#{line}，要求小写 snake_case atom"
    end

    conflict =
      entries
      |> Enum.group_by(fn {_kind, name, _file, _line} -> name end, fn {kind, _name, _file, _line} -> kind end)
      |> Enum.find(fn {_name, kinds} -> kinds |> Enum.uniq() |> length() > 1 end)

    if conflict do
      {name, kinds} = conflict
      uniq_kinds = kinds |> Enum.uniq() |> Enum.sort()

      raise ArgumentError,
            "runtime 指令冲突: #{inspect(name)} 同时出现在多种 step 类型 #{inspect(uniq_kinds)}，请保持单一类型"
    end
  end

  defp runtime_instruction_name_valid?(name) when is_atom(name) do
    name
    |> Atom.to_string()
    |> String.match?(~r/^[a-z][a-z0-9_]*$/)
  end

  defp runtime_instruction_name_valid?(_name), do: false

  defp print_domain_autowire_dry_run(
         skip_annotations_check?,
         skip_registry?,
         skip_docs?,
         skip_factories_check?,
         check_args,
         factories_check_args
       ) do
    IO.puts("[dry-run] command=domain.autowire")

    if not skip_annotations_check? do
      IO.puts("[dry-run] step=annotations.check")
    end

    if not skip_registry? do
      IO.puts("[dry-run] step=registry.scaffold")
      IO.puts("[dry-run] step=registry.upsert")
    end

    if not skip_docs? do
      IO.puts("[dry-run] step=instructions.docs")
    end

    IO.puts("[dry-run] step=check")
    IO.puts("[dry-run] check_args=#{Enum.join(check_args, " ")}")

    if not skip_factories_check? do
      IO.puts("[dry-run] step=factories.check")
      IO.puts("[dry-run] factories_check_args=#{Enum.join(factories_check_args, " ")}")
    end
  end

  defp print_acceptance_report_template(project_root, check_args) do
    date = Date.utc_today() |> Date.to_iso8601()
    check_cmd = "bdd_compiler check " <> Enum.join(check_args, " ")

    IO.puts("""

    [autowire] acceptance-report-template
    # BDD 链路验收报告（#{date}）

    ## 项目信息
    - project_root: #{project_root}
    - command: bdd_compiler domain.autowire

    ## 执行命令
    ```bash
    #{check_cmd}
    ```

    ## 结果
    - check: 通过
    - lint: 参考上方输出
    - generated tests: 参考上方输出

    ## 结论
    - 该域 BDD 链路已完成自动接线并通过门禁验收。
    """)
  end

  defp put_opt(args, _flag, nil), do: args
  defp put_opt(args, _flag, ""), do: args
  defp put_opt(args, flag, value), do: args ++ [flag, to_string(value)]

  defp run_mix_eval!(project_root, eval_code) when is_binary(project_root) and is_binary(eval_code) do
    {_out, status} =
      System.cmd("mix", ["run", "--no-start", "-e", eval_code],
        cd: project_root,
        into: IO.stream(:stdio, :line),
        stderr_to_stdout: true,
        env: [{"MIX_ENV", resolve_mix_env(:factories)}]
      )

    if status != 0 do
      System.halt(status)
    end
  rescue
    e ->
      IO.puts(:stderr, "运行 factories 子命令失败: #{Exception.message(e)}")
      System.halt(1)
  end

  defp pop_project_root(argv) when is_list(argv) do
    do_pop_project_root(argv, File.cwd!(), [])
  end

  defp do_pop_project_root([], root, passthrough) do
    {root, Enum.reverse(passthrough)}
  end

  defp do_pop_project_root(["--project-root", value | rest], _root, passthrough) do
    do_pop_project_root(rest, value, passthrough)
  end

  defp do_pop_project_root(["--project_root", value | rest], _root, passthrough) do
    do_pop_project_root(rest, value, passthrough)
  end

  defp do_pop_project_root(["--project-root"], _root, _passthrough) do
    IO.puts(:stderr, "参数解析失败: --project-root 缺少路径值")
    System.halt(1)
  end

  defp do_pop_project_root(["--project_root"], _root, _passthrough) do
    IO.puts(:stderr, "参数解析失败: --project_root 缺少路径值")
    System.halt(1)
  end

  defp do_pop_project_root([arg | rest], root, passthrough) when is_binary(arg) do
    cond do
      String.starts_with?(arg, "--project-root=") ->
        value = String.replace_prefix(arg, "--project-root=", "")

        if value == "" do
          IO.puts(:stderr, "参数解析失败: --project-root= 缺少路径值")
          System.halt(1)
        end

        do_pop_project_root(rest, value, passthrough)

      String.starts_with?(arg, "--project_root=") ->
        value = String.replace_prefix(arg, "--project_root=", "")

        if value == "" do
          IO.puts(:stderr, "参数解析失败: --project_root= 缺少路径值")
          System.halt(1)
        end

        do_pop_project_root(rest, value, passthrough)

      true ->
        do_pop_project_root(rest, root, [arg | passthrough])
    end
  end

  defp resolve_mix_env(:runtime) do
    System.get_env("BDD_COMPILER_RUNTIME_MIX_ENV") ||
      System.get_env("BDD_COMPILER_MIX_ENV") ||
      "test"
  end

  defp resolve_mix_env(:factories) do
    System.get_env("BDD_COMPILER_FACTORIES_MIX_ENV") ||
      System.get_env("BDD_COMPILER_MIX_ENV") ||
      "test"
  end

  defp check_factory_paths!(paths) when is_list(paths) do
    forbidden = ["NaiveDateTime.utc_now", "DateTime.utc_now", ":rand.uniform", "System.unique_integer"]

    paths
    |> Enum.flat_map(&expand_factory_files!/1)
    |> Enum.uniq()
    |> Enum.sort()
    |> Enum.each(&check_factory_file!(&1, forbidden))

    :ok
  end

  defp expand_factory_files!(path) when is_binary(path) do
    cond do
      File.dir?(path) ->
        Path.wildcard(Path.join(path, "**/*.ex"))

      File.regular?(path) ->
        [path]

      true ->
        raise ArgumentError, "数据工厂门禁失败：路径不存在或不可访问 path=#{inspect(path)}"
    end
  end

  defp check_factory_file!(path, forbidden) when is_binary(path) do
    path
    |> File.read!()
    |> String.split("\n")
    |> Enum.with_index(1)
    |> Enum.each(fn {line, line_no} ->
      Enum.each(forbidden, fn pat ->
        if String.contains?(line, pat) do
          raise ArgumentError, "数据工厂门禁失败：发现禁用模式 #{inspect(pat)} at #{path}:#{line_no}"
        end
      end)
    end)
  end

  defp print_warnings([]) do
    IO.puts("BDD lint: 0 warnings")
  end

  defp print_warnings(warnings) when is_list(warnings) do
    IO.puts("BDD lint: #{length(warnings)} warnings\n")

    Enum.each(warnings, fn w ->
      loc =
        cond do
          is_binary(w.file) and is_integer(w.line) -> "#{w.file}:#{w.line}"
          is_binary(w.file) -> w.file
          true -> "(unknown)"
        end

      raw = String.trim(to_string(w.raw || ""))
      raw_part = if raw == "", do: "", else: "\n  raw: #{raw}"

      IO.puts("""
      - [#{w.rule}] scenario=#{w.scenario_id} loc=#{loc}
        #{w.message}#{raw_part}
      """)
    end)
  end

  defp print_help(:global) do
    IO.puts("""
    bdd_compiler (escript)

    这是一个“BDD 文档(DSL) -> Elixir 测试代码”的编译器，分两类命令：
    1) 编译器本体命令（纯转换/校验）：compile / lint / check
    2) 项目命令（保持与既有 mix bdd.* 行为一致）：
       annotations.check / registry.scaffold / registry.upsert / instructions.docs
       factories.scaffold / factories.upsert / factories.check
       runtime.caps.sync
       domain.autowire
       contract.check / fuzz / mutation.report / mutation.run

    配置文件:
      默认会读取项目根目录下的 .bddc.toml（命令行参数优先级更高）。

    用法:
      bdd_compiler <command> [args...]

    查看某个子命令帮助:
      bdd_compiler <command> --help
      bdd_compiler help <command>

    编译器本体命令:
      compile            编译 docs 下的 BDD 场景，生成 ExUnit 测试文件
      lint               只做静态检查（规范/最佳实践提示），不生成代码
      check              compile + lint（可选 fail-on-warn）

    项目命令:
      init              初始化脚手架（生成 .bddc.toml/spec/runtime/示例 DSL/门禁脚本）
      annotations.check  检查代码里的 bdd(...) 注解是否符合约定
      registry.scaffold  从模块函数签名反推指令 spec（草稿）
      registry.upsert    将 scaffold 写回 target 文件的 GENERATED 区域（只覆盖标记区块，不改手写部分）
      instructions.docs  生成/更新“指令集说明”文档
      factories.scaffold  数据工厂 scaffold（CLI 内置实现）
      factories.upsert    数据工厂 upsert（CLI 内置实现）
      factories.check     数据工厂门禁检查（CLI 内置实现）
      runtime.caps.sync   同步 runtime capabilities 离线文件（供 check 离线校验）
      domain.autowire     一键串联：注解检查→指令生成/写回→文档→(可选 runtime caps 同步)→check
      contract.check      Contract/CDC 门禁检查
      fuzz                Fuzz（deterministic）检查
      mutation.report     Mutation 报告（接入点）
      mutation.run        Mutation 最小可运行链路
    """)
  end

  defp print_help("init") do
    IO.puts("""
    bdd_compiler init

    作用:
      初始化一个项目接入 bddc 所需的最小脚手架（协议落地）：
      - .bddc.toml
      - priv/bdd/instructions_v1.exs
      - test/support/bdd/*.ex（runtime dispatcher + runtime use 宏 + common 指令包）
      - docs/bdd/hello.dsl
      - scripts/bdd_gate.sh

    用法:
      bdd_compiler init [--project-root /path/to/project]
                       [--namespace MyApp]
                       [--force]
                       [--dry-run]
    """)
  end

  defp print_help("compile") do
    IO.puts("""
    bdd_compiler compile

    作用:
      编译 BDD 场景文档（默认 docs/bdd）为 ExUnit 测试代码（默认 test/bdd_generated）。

    用法:
      bdd_compiler compile --instructions instructions_v1.exs [--instructions-v2 instructions_v2.exs]
                          [--in docs/bdd] [--out test/bdd_generated]
                          [--project-root /path/to/project] [--registry-module Shop.BDD.InstructionRegistry]
                          [--runtime-module BDD.Instructions.V1]
                          [--test-case ExUnit.Case]
                          [--module-prefix BDD.Generated]
                          [--docs-root docs]

    必填:
      --instructions PATH         v1 指令集 .exs（返回 map：%{instruction_atom => spec_map}）

    可选:
      --instructions-v2 PATH      v2 指令集 .exs（可只提供 v2 delta；未命中会回退 v1）
      --project-root DIR          项目根目录（自动装载 registry 时使用，默认当前目录）
      --registry-module MOD       指令注册表模块（未传 --instructions 时自动装载）
      --in DIR                    BDD 文档目录（默认 docs/bdd）
      --out DIR                   生成测试目录（默认 test/bdd_generated）
      --runtime-module MOD        运行期指令执行模块（默认 BDD.Instructions.V1）
      --test-case MOD             测试基类（默认 ExUnit.Case）
      --module-prefix MOD         生成模块前缀（默认 BDD.Generated）
      --docs-root DIR             docs 根目录（用于更友好的 path/模块名推导）

    指令源优先级:
      --instructions > --registry-module（自动装载）> 编译失败
    """)
  end

  defp print_help("lint") do
    IO.puts("""
    bdd_compiler lint

    作用:
      对 BDD 文档做静态检查（规范/最佳实践），不生成测试代码。

    用法:
      bdd_compiler lint --instructions instructions_v1.exs [--instructions-v2 instructions_v2.exs]
                       [--in docs/bdd] [--project-root /path/to/project]
                       [--registry-module Shop.BDD.InstructionRegistry]
                       [--fail-on-warn true|false]
                       [--fail-tags strict_evidence,strict_interaction]
                       [--fail-globs "docs/bdd/critical/**"]

    参数:
      --fail-on-warn              若存在 warning 则返回非 0 退出码（也支持 --no-fail-on-warn）
      --fail-tags                仅当 warning 命中场景 tags 时才阻断（逗号分隔）
      --fail-globs               仅当 warning 命中文件路径 glob 时才阻断（逗号分隔，支持 **）
    """)
  end

  defp print_help("check") do
    IO.puts("""
    bdd_compiler check

    作用:
      一键门禁：
      1) (可选) annotations.check（如项目提供 mix task）
      2) runtime 覆盖校验（capabilities/0 必须覆盖 DSL 用到的指令）
      3) compile（生成 ExUnit）
      4) lint（可选 fail-on-warn / tags / globs）
      5) 运行生成测试：MIX_ENV=test mix test <out_dir>（可选排除 flaky / rerun failed）

    说明:
      - 如果 --project-root 下不存在 mix.exs，则默认不会执行 mix test（仍会完成编译 + lint + runtime 覆盖校验）。

    用法:
      bdd_compiler check --instructions instructions_v1.exs [--instructions-v2 instructions_v2.exs]
                        [--in docs/bdd] [--out test/bdd_generated]
                        [--project-root /path/to/project]
                        [--registry-module Shop.BDD.InstructionRegistry]
                        [--runtime-module BDD.Instructions.V1]
                        [--runtime-caps-file docs/bdd/_generated/runtime_caps_v1.exs]
                        [--fail-on-warn true|false]
                        [--fail-tags strict_evidence,strict_interaction]
                        [--fail-globs "docs/bdd/critical/**"]
                        [--exclude-flaky]
                        [--rerun-failures N]
                        [--skip-annotations-check]
                        [--skip-bdd-test]
    """)
  end

  defp print_help("annotations.check") do
    IO.puts("""
    bdd_compiler annotations.check

    作用:
      项目模式 wrapper。等价执行：mix bdd.annotations.check

    用法:
      bdd_compiler annotations.check [--project-root /path/to/project] [--...其余参数原样透传...]
    """)
  end

  defp print_help("registry.scaffold") do
    IO.puts("""
    bdd_compiler registry.scaffold

    作用:
      默认：优先尝试执行 mix bdd.registry.scaffold（若项目未提供该 task，则自动 fallback 为 standalone）。
      standalone：扫描源码中的 @bdd_instruction 字面量注解并生成 scaffold 指令 spec。
      等价执行：mix bdd.registry.scaffold（当项目提供该 task 时）

    用法:
      bdd_compiler registry.scaffold [--project-root /path/to/project] [--...其余参数原样透传...]
      bdd_compiler registry.scaffold --standalone [--project-root /path/to/project] [--src lib] [--out priv/bdd/_generated/instructions_v1_scaffold.exs]
    """)
  end

  defp print_help("registry.upsert") do
    IO.puts("""
    bdd_compiler registry.upsert

    作用:
      默认：优先尝试执行 mix bdd.registry.upsert（若项目未提供该 task，则自动 fallback 为 standalone）。
      standalone：把 scaffold 写入 target 文件的 GENERATED 区域（只覆盖标记区块，不改手写部分）。
      等价执行：mix bdd.registry.upsert（当项目提供该 task 时）

    用法:
      bdd_compiler registry.upsert [--project-root /path/to/project] [--...其余参数原样透传...]
      bdd_compiler registry.upsert --standalone [--project-root /path/to/project] [--scaffold priv/bdd/_generated/instructions_v1_scaffold.exs] [--target priv/bdd/instructions_v1.exs]
    """)
  end

  defp print_help("instructions.docs") do
    IO.puts("""
    bdd_compiler instructions.docs

    作用:
      项目模式 wrapper。等价执行：mix bdd.instructions_docs
      生成/更新指令集说明文档（通常会改动 docs/bdd/指令集.md）。

    用法:
      bdd_compiler instructions.docs [--project-root /path/to/project] [--...其余参数原样透传...]
    """)
  end

  defp print_help("factories.scaffold") do
    IO.puts("""
    bdd_compiler factories.scaffold

    作用:
      CLI 内置实现（兼容 mix bdd.factories.scaffold 行为口径）。
      从 schema/changeset 生成低层数据工厂骨架（scaffold）。

    用法:
      bdd_compiler factories.scaffold [--project-root /path/to/project] [--scope priv/bdd/factories_scope.exs] [--out /tmp/out_dir]
    """)
  end

  defp print_help("factories.upsert") do
    IO.puts("""
    bdd_compiler factories.upsert

    作用:
      CLI 内置实现（兼容 mix bdd.factories.upsert 行为口径）。
      将数据工厂 scaffold 结果幂等写回默认 generated 目录。

    用法:
      bdd_compiler factories.upsert [--project-root /path/to/project] [--scope priv/bdd/factories_scope.exs]
    """)
  end

  defp print_help("factories.check") do
    IO.puts("""
    bdd_compiler factories.check

    作用:
      CLI 内置实现（兼容 mix bdd.factories.check 行为口径）。
      执行数据工厂门禁检查（例如禁止随机/隐式当前时间默认值）。

    用法:
      bdd_compiler factories.check [--project-root /path/to/project] [--paths path1 --paths path2]
    """)
  end

  defp print_help("runtime.caps.sync") do
    IO.puts("""
    bdd_compiler runtime.caps.sync

    作用:
      从 runtime 指令实现源码中提取 capabilities，生成离线 caps 文件（用于 check 的 --runtime-caps-file）。

    用法:
      bdd_compiler runtime.caps.sync [--project-root /path/to/project]
                                    [--runtime-module Shop.BDD.Instructions.V1]
                                    [--runtime-source test/support/bdd/instructions_v1.ex]...
                                    [--out docs/bdd/_generated/runtime_caps_v1.exs]
                                    [--out-meta docs/bdd/_generated/runtime_caps_v1_meta.exs]

    说明:
      - 未传 --runtime-source 时，会按 runtime_module 后缀自动查找：
        test/support/**/instructions*<suffix>.ex
      - 默认输出：
        docs/bdd/_generated/runtime_caps_<suffix>.exs
      - 可选输出 meta：
        指令来源映射（%{instruction => [%{kind,file,line}, ...]}）
    """)
  end

  defp print_help("domain.autowire") do
    IO.puts("""
    bdd_compiler domain.autowire

    作用:
      一键串联执行：
      1) annotations.check
      2) registry.scaffold + registry.upsert
      3) instructions.docs
      3.5) runtime.caps.sync（可选）
      4) bdd_compiler check
      5) bdd_compiler factories.check（默认只检查 factories_generated）

    用法:
      bdd_compiler domain.autowire --project-root /path/to/project --module My.Module --functions f/1,g/2
                                  [--prefix fs] [--kind when] [--version v1]
                                  [--registry-module Shop.BDD.InstructionRegistry]
                                  [--runtime-module Shop.BDD.Instructions.V1]
                                  [--runtime-caps-file docs/bdd/_generated/runtime_caps_v1.exs]
                                  [--docs-root docs] [--in docs/bdd] [--out test/bdd_generated]
                                  [--strict true|false]
                                  [--strict-factories true|false]
                                  [--fail-on-warn true|false]
                                  [--sync-runtime-caps]
                                  [--dry-run]
                                  [--skip-annotations-check] [--skip-registry] [--skip-docs]
                                  [--skip-factories-check]

    说明:
      - --strict 默认 true；未显式传 --fail-on-warn 时，会按 strict 自动决定是否阻断 warning。
      - --strict-factories 默认 false；为 true 时会把 test/support/bdd/semantic_givens 一并纳入门禁。
      - 若未传 --skip-registry，则 --module 与 --functions 必填。
      - 默认会执行 factories.check；可用 --skip-factories-check 跳过。
      - --dry-run 只打印将执行步骤与 check 参数，不执行任何子步骤。
      - check 阶段可通过 --registry-module（推荐）或 --instructions 指令源运行。
      - 若传 --sync-runtime-caps，会在 check 前自动生成/更新 runtime caps 离线文件（供 --runtime-caps-file 使用）。
    """)
  end

  defp print_help("contract.check") do
    IO.puts("""
    bdd_compiler contract.check

    作用:
      项目模式 wrapper。等价执行：mix bdd.contract.check

    用法:
      bdd_compiler contract.check [--project-root /path/to/project] [--...其余参数原样透传...]
    """)
  end

  defp print_help("fuzz") do
    IO.puts("""
    bdd_compiler fuzz

    作用:
      项目模式 wrapper。等价执行：mix bdd.fuzz

    用法:
      bdd_compiler fuzz [--project-root /path/to/project] [--...其余参数原样透传...]
    """)
  end

  defp print_help("mutation.report") do
    IO.puts("""
    bdd_compiler mutation.report

    作用:
      项目模式 wrapper。等价执行：mix bdd.mutation.report

    用法:
      bdd_compiler mutation.report [--project-root /path/to/project] [--...其余参数原样透传...]
    """)
  end

  defp print_help("mutation.run") do
    IO.puts("""
    bdd_compiler mutation.run

    作用:
      项目模式 wrapper。等价执行：mix bdd.mutation.run

    用法:
      bdd_compiler mutation.run [--project-root /path/to/project] [--...其余参数原样透传...]
    """)
  end

  defp print_help(_unknown) do
    print_help(:global)
  end
end
