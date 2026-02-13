ExUnit.start()

# Make runtime module available for generated tests.
Code.require_file(Path.expand("support/bdd/instructions_v1.ex", __DIR__))
