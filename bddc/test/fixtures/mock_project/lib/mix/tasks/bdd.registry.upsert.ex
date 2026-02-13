defmodule Mix.Tasks.Bdd.Registry.Upsert do
  use Mix.Task

  @shortdoc false

  @impl Mix.Task
  def run(args), do: Fixture.MixTaskProbe.emit("bdd.registry.upsert", args)
end
