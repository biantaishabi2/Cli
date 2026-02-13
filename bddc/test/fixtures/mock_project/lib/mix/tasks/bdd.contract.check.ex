defmodule Mix.Tasks.Bdd.Contract.Check do
  use Mix.Task

  @shortdoc false

  @impl Mix.Task
  def run(args), do: Fixture.MixTaskProbe.emit("bdd.contract.check", args)
end
