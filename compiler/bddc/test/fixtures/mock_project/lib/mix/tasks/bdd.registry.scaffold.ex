defmodule Mix.Tasks.Bdd.Registry.Scaffold do
  use Mix.Task

  @shortdoc false

  @impl Mix.Task
  def run(args), do: Fixture.MixTaskProbe.emit("bdd.registry.scaffold", args)
end
