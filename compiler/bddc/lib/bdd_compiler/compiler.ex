defmodule BDDCompiler.Compiler do
  @moduledoc """
  DSL 编译入口：Parse -> Validate -> Emit -> 写入输出目录。

  额外支持：
  - 从 docs/*.md 的 fenced code block（```bdd/```dsl/```bdd-dsl）抽取 DSL 并参与编译。
  """

  alias BDDCompiler.{DslParser, Emitter, MarkdownExtractor, Validator}

  @spec compile_file!(String.t(), String.t(), BDDCompiler.InstructionSet.t(), keyword()) :: String.t()
  def compile_file!(dsl_path, out_dir, instruction_set, opts \\ [])
      when is_binary(dsl_path) and is_binary(out_dir) and is_list(opts) do
    scenarios = DslParser.parse_file!(dsl_path)
    :ok = Validator.validate!(dsl_path, scenarios, instruction_set)

    source = Emitter.emit_to_string(dsl_path, scenarios, opts)
    File.mkdir_p!(out_dir)

    out_path =
      out_dir
      |> Path.join(Path.basename(dsl_path, ".dsl") <> "_generated_test.exs")

    File.write!(out_path, source)
    out_path
  end

  @spec compile_dir!(String.t(), String.t(), BDDCompiler.InstructionSet.t(), keyword()) :: [String.t()]
  def compile_dir!(in_dir, out_dir, instruction_set, opts \\ [])
      when is_binary(in_dir) and is_binary(out_dir) and is_list(opts) do
    docs_root =
      case Keyword.fetch(opts, :docs_root) do
        {:ok, nil} -> nil
        {:ok, v} when is_binary(v) -> v
        :error -> default_docs_root(in_dir)
      end

    dsl_paths = discover_dsl_paths(in_dir)

    md_paths =
      if is_binary(docs_root) do
        docs_root
        |> Path.join("**/*.md")
        |> Path.wildcard()
        |> Enum.sort()
      else
        []
      end

    parsed =
      Enum.flat_map(dsl_paths, fn path ->
        [{path, path, Path.basename(path, ".dsl"), DslParser.parse_file!(path)}]
      end) ++
        Enum.flat_map(md_paths, fn md_path ->
          blocks = MarkdownExtractor.extract_blocks!(md_path)
          base_hash = :erlang.phash2(md_path)

          Enum.map(blocks, fn b ->
            module_base = "md_#{base_hash}_block_#{b.index}"
            virtual_source = md_path <> "#block:" <> Integer.to_string(b.index)
            scenarios = DslParser.parse_string!(b.content, file: md_path, line_offset: b.start_line - 1)
            {virtual_source, md_path, module_base, scenarios}
          end)
        end)

    assert_unique_scenario_ids!(parsed)

    File.mkdir_p!(out_dir)

    Enum.map(parsed, fn {source_id, validate_path, module_base, scenarios} ->
      :ok = Validator.validate!(validate_path, scenarios, instruction_set)
      source = Emitter.emit_to_string(source_id, scenarios, Keyword.put(opts, :module_base, module_base))

      out_path =
        out_dir
        |> Path.join(module_base <> "_generated_test.exs")

      File.write!(out_path, source)
      out_path
    end)
  end

  # 仅在默认目录结构（docs/bdd）下启用 Markdown 内嵌 DSL。
  defp default_docs_root(in_dir) do
    if Path.basename(in_dir) == "bdd" and Path.basename(Path.dirname(in_dir)) == "docs" do
      Path.dirname(in_dir)
    else
      nil
    end
  end

  # 优先消费 bcc organize 产出的 features/*.dsl，避免与 scenarios/*.dsl 重复读入同一 scenario。
  # 若不存在 features 目录，则兼容历史结构：根目录 *.dsl；再回退到递归扫描。
  defp discover_dsl_paths(in_dir) when is_binary(in_dir) do
    feature_paths =
      in_dir
      |> Path.join("features/**/*.dsl")
      |> Path.wildcard()
      |> Enum.sort()

    cond do
      feature_paths != [] ->
        feature_paths

      true ->
        root_paths =
          in_dir
          |> Path.join("*.dsl")
          |> Path.wildcard()
          |> Enum.sort()

        if root_paths != [] do
          root_paths
        else
          in_dir
          |> Path.join("**/*.dsl")
          |> Path.wildcard()
          |> Enum.sort()
        end
    end
  end

  defp assert_unique_scenario_ids!(parsed) when is_list(parsed) do
    parsed
    |> Enum.flat_map(fn {_source_id, _validate_path, _module_base, scenarios} -> scenarios end)
    |> Enum.reduce(%{}, fn s, acc ->
      id = Map.fetch!(s, :id)
      meta = Map.get(s, :meta, %{})

      case Map.fetch(acc, id) do
        :error ->
          Map.put(acc, id, meta)

        {:ok, first_meta} ->
          raise BDDCompiler.CompileError,
            message:
              "重复的 SCENARIO ID：#{id}（#{first_meta[:file]}:#{first_meta[:line]} 与 #{meta[:file]}:#{meta[:line]}）",
            file: meta[:file],
            line: meta[:line],
            raw: meta[:raw]
      end
    end)

    :ok
  end
end
