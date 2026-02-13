defmodule Mix.Tasks.Bdd.Annotations.Check do
  use Mix.Task

  @shortdoc false

  @impl Mix.Task
  def run(args), do: Fixture.MixTaskProbe.emit("bdd.annotations.check", args)
end
