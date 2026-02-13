defmodule Mix.Tasks.Bdd.Factories.Upsert do
  use Mix.Task

  @shortdoc false

  @impl Mix.Task
  def run(args) do
    Mix.Task.run("app.start")

    {opts, _rest, _invalid} =
      OptionParser.parse(args,
        strict: [scope: :string]
      )

    scope = Shop.BDD.Factories.Scope.load!(opts[:scope])
    out_dir = scope.default_out_dir
    files = Shop.BDD.Factories.Generator.generate_files!(scope.schemas)

    Enum.each(files, fn %{path: rel, content: content} ->
      path = Path.join(out_dir, Path.basename(rel))
      File.mkdir_p!(Path.dirname(path))
      File.write!(path, content)
    end)

    IO.puts("upserted=#{length(files)} out_dir=#{out_dir}")
  end
end
