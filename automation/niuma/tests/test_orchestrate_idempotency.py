import os
import pathlib
import shutil
import subprocess
import tempfile
import threading
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
REUSABLE_PATH = REPO_ROOT / ".github/workflows/niuma-orchestrate-reusable.yml"


def extract_step_run_script(workflow_path: pathlib.Path, step_name: str) -> str:
    lines = workflow_path.read_text(encoding="utf-8").splitlines()
    step_index = None
    step_indent = 0
    for index, line in enumerate(lines):
        if line.strip() == f"- name: {step_name}":
            step_index = index
            step_indent = len(line) - len(line.lstrip(" "))
            break
    if step_index is None:
        raise AssertionError(f"未找到 step: {step_name}")

    run_index = None
    run_indent = 0
    for index in range(step_index + 1, len(lines)):
        line = lines[index]
        indent = len(line) - len(line.lstrip(" "))
        if line.lstrip().startswith("- name:") and indent <= step_indent:
            break
        if line.strip() == "run: |":
            run_index = index
            run_indent = indent
            break
    if run_index is None:
        raise AssertionError(f"step {step_name} 缺少 run: |")

    block_lines: list[str] = []
    for index in range(run_index + 1, len(lines)):
        line = lines[index]
        if line.strip() == "":
            block_lines.append("")
            continue
        indent = len(line) - len(line.lstrip(" "))
        if indent <= run_indent:
            break
        block_lines.append(line)

    min_indent = min(
        (len(line) - len(line.lstrip(" ")) for line in block_lines if line.strip()),
        default=0,
    )
    return "\n".join(line[min_indent:] if line else "" for line in block_lines)


def extract_concurrency_group_expression(workflow_path: pathlib.Path) -> str:
    for line in workflow_path.read_text(encoding="utf-8").splitlines():
        stripped = line.strip()
        if stripped.startswith("group:"):
            return stripped.split("group:", 1)[1].strip()
    raise AssertionError("未找到 concurrency.group")


def parse_output_file(path: pathlib.Path) -> dict[str, str]:
    outputs: dict[str, str] = {}
    if not path.exists():
        return outputs
    for line in path.read_text(encoding="utf-8").splitlines():
        if "=" in line:
            key, value = line.split("=", 1)
            outputs[key] = value
    return outputs


def run_shell_script(
    script: str,
    env: dict[str, str],
    *,
    path_prefix: str | None = None,
) -> tuple[subprocess.CompletedProcess[str], dict[str, str]]:
    with tempfile.TemporaryDirectory() as tmp_dir:
        tmp_path = pathlib.Path(tmp_dir)
        github_output = tmp_path / "github_output"
        github_env = tmp_path / "github_env"
        github_output.write_text("", encoding="utf-8")
        github_env.write_text("", encoding="utf-8")

        run_env = os.environ.copy()
        run_env.update(env)
        run_env["GITHUB_OUTPUT"] = str(github_output)
        run_env["GITHUB_ENV"] = str(github_env)
        if path_prefix:
            run_env["PATH"] = f"{path_prefix}:{run_env['PATH']}"

        completed = subprocess.run(
            ["bash", "-c", f"set -euo pipefail\n{script}"],
            check=False,
            capture_output=True,
            text=True,
            env=run_env,
        )

        return completed, parse_output_file(github_output)


def create_date_stub(stub_dir: pathlib.Path, epoch: int) -> str:
    real_date = shutil.which("date")
    if real_date is None:
        raise AssertionError("未找到 date 命令")
    date_stub = stub_dir / "date"
    date_stub.write_text(
        "\n".join(
            [
                "#!/usr/bin/env bash",
                f"if [ \"${{1:-}}\" = \"+%s\" ]; then echo \"{epoch}\"; exit 0; fi",
                f"exec \"{real_date}\" \"$@\"",
            ]
        )
        + "\n",
        encoding="utf-8",
    )
    date_stub.chmod(0o755)
    return str(stub_dir)


def safe_repo(repo: str) -> str:
    return repo.replace("/", "__").replace(":", "__")


class TestOrchestrateIdempotency(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.trigger_context_script = (
            extract_step_run_script(REUSABLE_PATH, "Resolve Trigger Context")
            .replace("${{ github.event.label.name }}", "${EVENT_LABEL_NAME}")
            .replace("${{ github.event.client_payload.event_source }}", "${EVENT_SOURCE}")
            .replace("${{ inputs.event_id }}", "${INPUT_EVENT_ID}")
            .replace("${{ github.event.issue.number }}", "${EVENT_ISSUE_NUMBER}")
            .replace("${{ github.event.client_payload.source_issue }}", "${PAYLOAD_SOURCE_ISSUE}")
            .replace("${{ github.event.client_payload.trigger_pr }}", "${PAYLOAD_TRIGGER_PR}")
            .replace("${{ github.event.client_payload.event_id }}", "${PAYLOAD_EVENT_ID}")
            .replace("${{ github.event.action }}", "${EVENT_ACTION}")
        )
        cls.loop_guard_script = extract_step_run_script(REUSABLE_PATH, "Loop Guard")
        cls.idempotency_script = (
            extract_step_run_script(REUSABLE_PATH, "Idempotency Guard")
            .replace("${{ steps.trigger_context.outputs.event_id_from_external }}", "${EVENT_ID_FROM_EXTERNAL}")
            .replace("${{ inputs.repo }}", "${INPUT_REPO}")
            .replace("${{ steps.trigger_context.outputs.event_id }}", "${TRIGGER_EVENT_ID}")
            .replace("${{ inputs.dedup_window_hours }}", "${INPUT_DEDUP_WINDOW_HOURS}")
        )
        cls.concurrency_group_expr = extract_concurrency_group_expression(REUSABLE_PATH)

    def test_trigger_context_falls_back_to_run_level_unique_key(self) -> None:
        completed, outputs = run_shell_script(
            self.trigger_context_script,
            {
                "GITHUB_EVENT_NAME": "schedule",
                "EVENT_LABEL_NAME": "",
                "EVENT_SOURCE": "",
                "INPUT_EVENT_ID": "",
                "EVENT_ISSUE_NUMBER": "",
                "PAYLOAD_SOURCE_ISSUE": "",
                "PAYLOAD_TRIGGER_PR": "",
                "PAYLOAD_EVENT_ID": "",
                "EVENT_ACTION": "",
                "GITHUB_RUN_ID": "98765",
                "GITHUB_RUN_ATTEMPT": "3",
            },
        )
        self.assertEqual(completed.returncode, 0, msg=completed.stderr + completed.stdout)
        self.assertEqual(outputs.get("event_id"), "run-98765-attempt-3")
        self.assertEqual(outputs.get("event_id_from_external"), "false")

    def test_loop_guard_blocks_bot_self_trigger(self) -> None:
        blocked_case, blocked_outputs = run_shell_script(
            self.loop_guard_script,
            {
                "GITHUB_EVENT_NAME": "issues",
                "GITHUB_ACTOR": "github-actions[bot]",
            },
        )
        self.assertEqual(blocked_case.returncode, 0, msg=blocked_case.stderr + blocked_case.stdout)
        self.assertEqual(blocked_outputs.get("blocked"), "true")
        self.assertEqual(blocked_outputs.get("reason"), "issues_actor_bot")

        pass_case, pass_outputs = run_shell_script(
            self.loop_guard_script,
            {
                "GITHUB_EVENT_NAME": "repository_dispatch",
                "GITHUB_ACTOR": "github-actions[bot]",
            },
        )
        self.assertEqual(pass_case.returncode, 0, msg=pass_case.stderr + pass_case.stdout)
        self.assertEqual(pass_outputs.get("blocked"), "false")
        self.assertEqual(pass_outputs.get("reason"), "none")

    def test_dedup_window_hours_invalid_values_fail_in_real_script(self) -> None:
        repo = f"biantaishabi2/Cli-dedup-invalid-{os.getpid()}"
        shutil.rmtree(pathlib.Path("/tmp/niuma-orchestrate-dedup") / safe_repo(repo), ignore_errors=True)

        for invalid in ("0", "-1", "1.5"):
            with self.subTest(invalid=invalid), tempfile.TemporaryDirectory() as stub_tmp:
                completed, _ = run_shell_script(
                    self.idempotency_script,
                    {
                        "EVENT_ID_FROM_EXTERNAL": "true",
                        "INPUT_REPO": repo,
                        "TRIGGER_EVENT_ID": f"evt-{invalid}",
                        "INPUT_DEDUP_WINDOW_HOURS": invalid,
                    },
                    path_prefix=create_date_stub(pathlib.Path(stub_tmp), 1_700_000_000),
                )
                self.assertNotEqual(completed.returncode, 0)
                self.assertIn(
                    "dedup_window_hours 必须为正整数",
                    completed.stderr + completed.stdout,
                )

    def test_duplicate_event_boundary_24h_with_real_idempotency_step(self) -> None:
        repo = f"biantaishabi2/Cli-dedup-boundary-{os.getpid()}"
        dedup_dir = pathlib.Path("/tmp/niuma-orchestrate-dedup") / safe_repo(repo)
        shutil.rmtree(dedup_dir, ignore_errors=True)
        event_id = "evt-boundary"
        base = 1_700_100_000

        with tempfile.TemporaryDirectory() as first_stub, tempfile.TemporaryDirectory() as second_stub, tempfile.TemporaryDirectory() as third_stub:
            first_run, first_outputs = run_shell_script(
                self.idempotency_script,
                {
                    "EVENT_ID_FROM_EXTERNAL": "true",
                    "INPUT_REPO": repo,
                    "TRIGGER_EVENT_ID": event_id,
                    "INPUT_DEDUP_WINDOW_HOURS": "24",
                },
                path_prefix=create_date_stub(pathlib.Path(first_stub), base),
            )
            second_run, second_outputs = run_shell_script(
                self.idempotency_script,
                {
                    "EVENT_ID_FROM_EXTERNAL": "true",
                    "INPUT_REPO": repo,
                    "TRIGGER_EVENT_ID": event_id,
                    "INPUT_DEDUP_WINDOW_HOURS": "24",
                },
                path_prefix=create_date_stub(pathlib.Path(second_stub), base + 24 * 3600),
            )
            third_run, third_outputs = run_shell_script(
                self.idempotency_script,
                {
                    "EVENT_ID_FROM_EXTERNAL": "true",
                    "INPUT_REPO": repo,
                    "TRIGGER_EVENT_ID": event_id,
                    "INPUT_DEDUP_WINDOW_HOURS": "24",
                },
                path_prefix=create_date_stub(pathlib.Path(third_stub), base + 24 * 3600 + 1),
            )

        self.assertEqual(first_run.returncode, 0, msg=first_run.stderr + first_run.stdout)
        self.assertEqual(second_run.returncode, 0, msg=second_run.stderr + second_run.stdout)
        self.assertEqual(third_run.returncode, 0, msg=third_run.stderr + third_run.stdout)
        self.assertEqual(first_outputs.get("duplicate_event"), "false")
        self.assertEqual(first_outputs.get("reason"), "first_seen")
        self.assertEqual(second_outputs.get("duplicate_event"), "true")
        self.assertEqual(second_outputs.get("reason"), "duplicate_event_id")
        self.assertEqual(third_outputs.get("duplicate_event"), "false")
        self.assertEqual(third_outputs.get("reason"), "first_seen")

    def test_parallel_same_event_id_is_serialized_by_concurrency_group(self) -> None:
        self.assertIn("niuma-orchestrate-${repo}", self.concurrency_group_expr)
        repo = f"biantaishabi2/Cli-dedup-concurrent-{os.getpid()}"
        dedup_dir = pathlib.Path("/tmp/niuma-orchestrate-dedup") / safe_repo(repo)
        shutil.rmtree(dedup_dir, ignore_errors=True)

        group = f"niuma-orchestrate-{repo}"
        lock_by_group: dict[str, threading.Lock] = {group: threading.Lock()}
        barrier = threading.Barrier(2)
        results: list[tuple[subprocess.CompletedProcess[str], dict[str, str]]] = []
        results_guard = threading.Lock()

        def worker() -> None:
            with tempfile.TemporaryDirectory() as stub_tmp:
                barrier.wait()
                with lock_by_group[group]:
                    result = run_shell_script(
                        self.idempotency_script,
                        {
                            "EVENT_ID_FROM_EXTERNAL": "true",
                            "INPUT_REPO": repo,
                            "TRIGGER_EVENT_ID": "evt-concurrency",
                            "INPUT_DEDUP_WINDOW_HOURS": "24",
                        },
                        path_prefix=create_date_stub(pathlib.Path(stub_tmp), 1_701_000_000),
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
        for completed, _ in results:
            self.assertEqual(completed.returncode, 0, msg=completed.stderr + completed.stdout)

        duplicate_count = sum(1 for _, outputs in results if outputs.get("duplicate_event") == "true")
        first_seen_count = sum(1 for _, outputs in results if outputs.get("reason") == "first_seen")
        self.assertEqual(duplicate_count, 1)
        self.assertEqual(first_seen_count, 1)


if __name__ == "__main__":
    unittest.main()
