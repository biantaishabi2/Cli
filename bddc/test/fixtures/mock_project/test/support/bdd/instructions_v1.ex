defmodule Fixture.BDD.Instructions.V1 do
  import ExUnit.Assertions

  @spec capabilities() :: MapSet.t(atom())
  def capabilities do
    MapSet.new([:given_seed, :when_do, :assert_done])
  end

  @spec new_run_id() :: String.t()
  def new_run_id do
    "fixture_run_" <> Integer.to_string(System.unique_integer([:positive]))
  end

  @spec get!(map(), atom(), map()) :: term()
  def get!(ctx, key, _meta \\ %{}) when is_map(ctx) and is_atom(key) do
    case Map.fetch(ctx, key) do
      {:ok, v} -> v
      :error -> raise ArgumentError, "fixture ctx missing #{inspect(key)}"
    end
  end

  @spec run_step!(map(), :given | :when | :then, atom(), map(), map(), term()) :: map()
  def run_step!(ctx, kind, name, args, meta, _step_id \\ nil) when is_map(ctx) and is_map(args) do
    run!(ctx, kind, name, args, meta)
  end

  @spec run!(map(), :given | :when | :then, atom(), map(), map()) :: map()
  def run!(ctx, kind, name, args, _meta) when is_map(ctx) and is_atom(name) and is_map(args) do
    case {kind, name} do
      {:given, :given_seed} ->
        Map.put(ctx, :id, Map.fetch!(args, :id))

      {:when, :when_do} ->
        # no-op for fixture
        ctx

      {:then, :assert_done} ->
        assert Map.get(ctx, :id) == Map.fetch!(args, :id)
        ctx
    end
  end

  # 仅用于 runtime.caps.sync 的 AST 扫描基线（不参与执行）。
  def __caps_sync_fixture__ do
    case :caps_sync do
      {:given, :given_seed} -> :ok
      {:when, :when_do} -> :ok
      {:then, :assert_done} -> :ok
    end
  end
end
