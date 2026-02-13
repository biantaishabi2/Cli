defmodule Mix.Tasks.Bdd.Mutation.Run do
  use Mix.Task

  @shortdoc false

  @impl Mix.Task
  def run(args), do: Fixture.MixTaskProbe.emit("bdd.mutation.run", args)
end
