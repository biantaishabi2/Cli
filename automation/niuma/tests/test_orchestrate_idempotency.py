import pathlib
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
REUSABLE_PATH = REPO_ROOT / ".github/workflows/niuma-orchestrate-reusable.yml"


class TestOrchestrateIdempotency(unittest.TestCase):
    def test_repeated_event_id_is_guarded(self) -> None:
        content = REUSABLE_PATH.read_text(encoding="utf-8")
        self.assertIn("Idempotency Guard", content)
        self.assertIn("/tmp/niuma-orchestrate-dedup", content)
        self.assertIn("event_id_from_external", content)
        self.assertIn("duplicate_event", content)
        self.assertIn("dedup_window_hours", content)
        self.assertIn("find \"$dedup_dir\" -type f -mmin", content)

    def test_missing_event_id_falls_back_to_run_unique_key(self) -> None:
        content = REUSABLE_PATH.read_text(encoding="utf-8")
        self.assertIn("event_id=\"run-${GITHUB_RUN_ID}-attempt-${GITHUB_RUN_ATTEMPT}\"", content)
        self.assertIn("run_level_unique", content)

    def test_loop_guard_blocks_bot_self_trigger(self) -> None:
        content = REUSABLE_PATH.read_text(encoding="utf-8")
        self.assertIn("Loop Guard", content)
        self.assertIn("github-actions[bot]", content)
        self.assertIn("issues_actor_bot", content)


if __name__ == "__main__":
    unittest.main()
