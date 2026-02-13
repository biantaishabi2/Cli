defmodule FixtureProject.MixProject do
  use Mix.Project

  def project do
    [
      app: :fixture_project,
      version: "0.1.0",
      elixir: "~> 1.14",
      start_permanent: Mix.env() == :prod,
      deps: []
    ]
  end

  def application do
    [extra_applications: [:logger]]
  end
end

