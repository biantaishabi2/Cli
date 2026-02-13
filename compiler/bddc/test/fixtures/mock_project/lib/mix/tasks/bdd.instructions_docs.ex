defmodule Mix.Tasks.Bdd.InstructionsDocs do
  use Mix.Task

  @shortdoc false

  @impl Mix.Task
  def run(args), do: Fixture.MixTaskProbe.emit("bdd.instructions_docs", args)
end
