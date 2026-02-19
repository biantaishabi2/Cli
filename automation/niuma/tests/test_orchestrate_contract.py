import json
import pathlib
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
SCHEMA_PATH = REPO_ROOT / "automation/niuma/contracts/orchestrate_inputs.schema.json"
REUSABLE_PATH = REPO_ROOT / ".github/workflows/niuma-orchestrate-reusable.yml"
ENTRYPOINT_PATH = REPO_ROOT / ".github/workflows/niuma-orchestrate.yml"


class TestOrchestrateContract(unittest.TestCase):
    def test_schema_defaults_and_required(self) -> None:
        schema = json.loads(SCHEMA_PATH.read_text(encoding="utf-8"))
        self.assertEqual(schema["required"], ["repo"])
        props = schema["properties"]
        self.assertEqual(props["repo_dir"]["default"], ".")
        self.assertTrue(props["build_niuma"]["default"])
        self.assertEqual(props["label_whitelist"]["default"], "bot:queued,bot:pr-reviewable")
        self.assertTrue(props["enable_dispatch_wakeup"]["default"])
        self.assertEqual(props["event_id"]["default"], "")
        self.assertEqual(props["dedup_window_hours"]["default"], 24)
        self.assertEqual(props["concurrency_key"]["default"], "niuma-orchestrate-${repo}")

    def test_reusable_declares_minimal_permissions(self) -> None:
        content = REUSABLE_PATH.read_text(encoding="utf-8")
        self.assertIn("permissions:", content)
        self.assertIn("issues: write", content)
        self.assertIn("contents: read", content)
        self.assertNotIn("actions: write", content)

    def test_entrypoint_is_thin_wrapper(self) -> None:
        content = ENTRYPOINT_PATH.read_text(encoding="utf-8")
        self.assertIn("types: [labeled]", content)
        self.assertIn("types: [niuma.task.completed]", content)
        self.assertIn("uses: ./.github/workflows/niuma-orchestrate-reusable.yml", content)
        self.assertIn("label_whitelist: \"bot:queued,bot:pr-reviewable\"", content)
        self.assertNotIn("control close-merged", content)


if __name__ == "__main__":
    unittest.main()
