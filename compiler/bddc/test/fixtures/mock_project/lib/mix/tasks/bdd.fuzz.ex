defmodule Mix.Tasks.Bdd.Fuzz do
  use Mix.Task

  @shortdoc false

  @impl Mix.Task
  def run(args), do: Fixture.MixTaskProbe.emit("bdd.fuzz", args)
end
