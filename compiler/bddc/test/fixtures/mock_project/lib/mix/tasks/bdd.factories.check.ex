defmodule Mix.Tasks.Bdd.Factories.Check do
  use Mix.Task

  @shortdoc false

  @impl Mix.Task
  def run(args) do
    Mix.Task.run("app.start")

    {opts, _rest, _invalid} =
      OptionParser.parse(args,
        strict: [paths: :keep]
      )

    paths =
      case opts[:paths] do
        nil -> ["test/support/bdd/factories_generated", "test/support/bdd/semantic_givens"]
        one when is_binary(one) -> [one]
        list when is_list(list) -> list
      end

    Shop.BDD.Factories.Checker.check_paths!(paths)
    IO.puts("ok paths=#{inspect(paths)}")
  end
end
