import hashlib
import pathlib
import tempfile
import threading
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
REUSABLE_PATH = REPO_ROOT / ".github/workflows/niuma-orchestrate-reusable.yml"


def resolve_event_id(
    input_event_id: str,
    payload_event_id: str,
    run_id: str,
    run_attempt: str,
) -> tuple[str, bool]:
    raw_event_id = input_event_id or payload_event_id
    if raw_event_id:
        return raw_event_id, True
    return f"run-{run_id}-attempt-{run_attempt}", False


def loop_guard(event_name: str, actor: str) -> tuple[bool, str]:
    if event_name == "issues" and actor == "github-actions[bot]":
        return True, "issues_actor_bot"
    return False, "none"


def resolve_concurrency_group(repo: str, concurrency_key: str) -> str:
    if concurrency_key == "niuma-orchestrate-${repo}":
        return f"niuma-orchestrate-{repo}"
    return concurrency_key


def run_idempotency_guard(
    *,
    dedup_root: pathlib.Path,
    repo: str,
    event_id: str,
    event_id_from_external: bool,
    dedup_window_hours: str,
    now_epoch: int,
) -> tuple[bool, str]:
    if not event_id_from_external:
        return False, "run_level_unique"

    if not dedup_window_hours.isdigit() or int(dedup_window_hours) <= 0:
        raise ValueError(f"dedup_window_hours 必须为正整数，当前值: {dedup_window_hours}")

    window_hours = int(dedup_window_hours)
    window_seconds = window_hours * 3600
    safe_repo = repo.replace("/", "__").replace(":", "__")
    dedup_dir = dedup_root / safe_repo
    dedup_dir.mkdir(parents=True, exist_ok=True)

    cutoff_epoch = now_epoch - window_seconds
    for marker in dedup_dir.glob("*.ts"):
        try:
            seen_epoch = int(marker.read_text(encoding="utf-8").strip() or "0")
        except ValueError:
            seen_epoch = 0
        if seen_epoch < cutoff_epoch:
            marker.unlink(missing_ok=True)

    event_hash = hashlib.sha256(event_id.encode("utf-8")).hexdigest()
    marker_file = dedup_dir / f"{event_hash}.ts"
    duplicate_event = False
    reason = "first_seen"

    if marker_file.exists():
        try:
            seen_epoch = int(marker_file.read_text(encoding="utf-8").strip() or "0")
        except ValueError:
            seen_epoch = 0
        if seen_epoch > 0 and (now_epoch - seen_epoch) <= window_seconds:
            duplicate_event = True
            reason = "duplicate_event_id"

    if not duplicate_event:
        marker_file.write_text(str(now_epoch), encoding="utf-8")

    return duplicate_event, reason


class SerialOrchestrator:
    def __init__(self, dedup_root: pathlib.Path) -> None:
        self._dedup_root = dedup_root
        self._locks: dict[str, threading.Lock] = {}
        self._locks_guard = threading.Lock()

    def process_event(self, repo: str, event_id: str, now_epoch: int) -> tuple[bool, str]:
        group = resolve_concurrency_group(repo=repo, concurrency_key="niuma-orchestrate-${repo}")
        with self._locks_guard:
            if group not in self._locks:
                self._locks[group] = threading.Lock()
            lock = self._locks[group]
        with lock:
            duplicate_event, reason = run_idempotency_guard(
                dedup_root=self._dedup_root,
                repo=repo,
                event_id=event_id,
                event_id_from_external=True,
                dedup_window_hours="24",
                now_epoch=now_epoch,
            )
            return not duplicate_event, reason


class TestOrchestrateIdempotency(unittest.TestCase):
    def test_idempotency_step_exists(self) -> None:
        content = REUSABLE_PATH.read_text(encoding="utf-8")
        self.assertIn("Idempotency Guard", content)
        self.assertIn("/tmp/niuma-orchestrate-dedup", content)
        self.assertIn("window_hours_raw", content)
        self.assertIn("duplicate_event_id", content)

    def test_missing_event_id_falls_back_to_run_unique_key(self) -> None:
        event_id, from_external = resolve_event_id(
            input_event_id="",
            payload_event_id="",
            run_id="12345",
            run_attempt="2",
        )
        self.assertEqual(event_id, "run-12345-attempt-2")
        self.assertFalse(from_external)

    def test_loop_guard_blocks_bot_self_trigger(self) -> None:
        blocked, reason = loop_guard(event_name="issues", actor="github-actions[bot]")
        self.assertTrue(blocked)
        self.assertEqual(reason, "issues_actor_bot")
        blocked, reason = loop_guard(event_name="repository_dispatch", actor="github-actions[bot]")
        self.assertFalse(blocked)
        self.assertEqual(reason, "none")

    def test_dedup_window_hours_invalid_values_raise(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            dedup_root = pathlib.Path(tmp)
            for invalid_value in ("0", "-1", "1.5"):
                with self.subTest(invalid_value=invalid_value):
                    with self.assertRaises(ValueError):
                        run_idempotency_guard(
                            dedup_root=dedup_root,
                            repo="biantaishabi2/Cli",
                            event_id="evt-invalid",
                            event_id_from_external=True,
                            dedup_window_hours=invalid_value,
                            now_epoch=1_700_000_000,
                        )

    def test_duplicate_event_id_is_noop_within_window_and_expires_after_boundary(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            dedup_root = pathlib.Path(tmp)
            first = run_idempotency_guard(
                dedup_root=dedup_root,
                repo="biantaishabi2/Cli",
                event_id="evt-1",
                event_id_from_external=True,
                dedup_window_hours="24",
                now_epoch=1_700_000_000,
            )
            second = run_idempotency_guard(
                dedup_root=dedup_root,
                repo="biantaishabi2/Cli",
                event_id="evt-1",
                event_id_from_external=True,
                dedup_window_hours="24",
                now_epoch=1_700_000_000 + 24 * 3600,
            )
            third = run_idempotency_guard(
                dedup_root=dedup_root,
                repo="biantaishabi2/Cli",
                event_id="evt-1",
                event_id_from_external=True,
                dedup_window_hours="24",
                now_epoch=1_700_000_000 + 24 * 3600 + 1,
            )
            self.assertEqual(first, (False, "first_seen"))
            self.assertEqual(second, (True, "duplicate_event_id"))
            self.assertEqual(third, (False, "first_seen"))

    def test_concurrency_group_serializes_same_repo_event_processing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            orchestrator = SerialOrchestrator(pathlib.Path(tmp))
            barrier = threading.Barrier(2)
            results: list[tuple[bool, str]] = []
            results_guard = threading.Lock()

            def worker() -> None:
                barrier.wait()
                result = orchestrator.process_event(
                    repo="biantaishabi2/Cli",
                    event_id="evt-concurrent",
                    now_epoch=1_700_100_000,
                )
                with results_guard:
                    results.append(result)

            t1 = threading.Thread(target=worker)
            t2 = threading.Thread(target=worker)
            t1.start()
            t2.start()
            t1.join()
            t2.join()

            self.assertEqual(len(results), 2)
            self.assertEqual(sum(1 for accepted, _ in results if accepted), 1)
            self.assertEqual(sum(1 for _, reason in results if reason == "duplicate_event_id"), 1)


if __name__ == "__main__":
    unittest.main()
