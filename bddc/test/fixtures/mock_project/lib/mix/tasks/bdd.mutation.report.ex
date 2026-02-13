defmodule Mix.Tasks.Bdd.Mutation.Report do
  use Mix.Task

  @shortdoc false

  @impl Mix.Task
  def run(args), do: Fixture.MixTaskProbe.emit("bdd.mutation.report", args)
end
