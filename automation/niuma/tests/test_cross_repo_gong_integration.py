import pathlib
import re
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
GONG_ORCHESTRATE_PATH = REPO_ROOT / "gong/.github/workflows/niuma-orchestrate.yml"
GONG_DISPATCH_PATH = REPO_ROOT / "gong/.github/workflows/niuma-dispatch-completed.yml"


class TestCrossRepoGongIntegration(unittest.TestCase):
    def test_gong_consumes_cli_reusable_with_sha_pin(self) -> None:
        content = GONG_ORCHESTRATE_PATH.read_text(encoding="utf-8")
        self.assertRegex(
            content,
            r"uses:\s*biantaishabi2/Cli/\.github/workflows/niuma-orchestrate-reusable\.yml@[0-9a-f]{40}",
        )
        self.assertIn("build_niuma: false", content)
        self.assertIn("label_whitelist: \"bot:queued,bot:pr-reviewable\"", content)
        self.assertIn("types: [niuma.task.completed]", content)

    def test_gong_dispatch_payload_matches_contract(self) -> None:
        content = GONG_DISPATCH_PATH.read_text(encoding="utf-8")
        self.assertIn("event_type: \"niuma.task.completed\"", content)
        self.assertIn("source_issue", content)
        self.assertIn("source_issues", content)
        self.assertIn("trigger_pr", content)
        self.assertIn("event_source: \"close-after-integration-merge\"", content)
        self.assertIn("event_id", content)
        self.assertIn("continue-on-error: true", content)
        self.assertIn("Warn Dispatch Failure", content)


if __name__ == "__main__":
    unittest.main()
