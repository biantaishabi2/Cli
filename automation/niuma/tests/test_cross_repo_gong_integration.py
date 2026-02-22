import json
import os
import pathlib
import shutil
import subprocess
import tempfile
import unittest


REPO_ROOT = pathlib.Path(__file__).resolve().parents[3]
CLI_ENTRYPOINT_PATH = REPO_ROOT / ".github/workflows/niuma-orchestrate.yml"
REUSABLE_PATH = REPO_ROOT / ".github/workflows/niuma-orchestrate-reusable.yml"
GONG_ORCHESTRATE_PATH = REPO_ROOT / "gong/.github/workflows/niuma-orchestrate.yml"
GONG_DISPATCH_PATH = REPO_ROOT / "gong/.github/workflows/niuma-dispatch-completed.yml"


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


def extract_job_if_expression(workflow_path: pathlib.Path, job_name: str) -> str:
    lines = workflow_path.read_text(encoding="utf-8").splitlines()
    job_index = None
    job_indent = 0
    for index, line in enumerate(lines):
        if line.strip() == f"{job_name}:":
            job_index = index
            job_indent = len(line) - len(line.lstrip(" "))
            break
    if job_index is None:
        raise AssertionError(f"未找到 job: {job_name}")

    for index in range(job_index + 1, len(lines)):
        line = lines[index]
        indent = len(line) - len(line.lstrip(" "))
        if indent <= job_indent and line.strip():
            break
        if line.strip().startswith("if: >"):
            parts: list[str] = []
            for sub_index in range(index + 1, len(lines)):
                sub_line = lines[sub_index]
                sub_indent = len(sub_line) - len(sub_line.lstrip(" "))
                if sub_indent <= indent and sub_line.strip():
                    break
                text = sub_line.strip()
                if text:
                    parts.append(text)
            return " ".join(parts)
        if line.strip().startswith("if:"):
            return line.split("if:", 1)[1].strip()
    raise AssertionError(f"job {job_name} 缺少 if 条件")


def evaluate_entrypoint_if(
    expression: str,
    *,
    event_name: str,
    label_name: str = "",
    action: str = "",
    event_source: str = "",
) -> bool:
    compiled = " ".join(expression.split())
    compiled = compiled.replace("&&", " and ").replace("||", " or ")
    replacements = {
        "github.event.client_payload.event_source": repr(event_source),
        "github.event.label.name": repr(label_name),
        "github.event.action": repr(action),
        "github.event_name": repr(event_name),
    }
    for key, value in replacements.items():
        compiled = compiled.replace(key, value)
    if "github." in compiled:
        raise AssertionError(f"存在未替换上下文: {compiled}")
    return bool(eval(compiled, {"__builtins__": {}}, {}))


def parse_key_value_file(path: pathlib.Path) -> dict[str, str]:
    values: dict[str, str] = {}
    if not path.exists():
        return values
    for line in path.read_text(encoding="utf-8").splitlines():
        if "=" in line:
            key, value = line.split("=", 1)
            values[key] = value
    return values


def run_shell_script(
    script: str,
    env: dict[str, str],
    *,
    path_prefix: str | None = None,
    isolate_path: bool = False,
) -> tuple[subprocess.CompletedProcess[str], dict[str, str], dict[str, str]]:
    bash_bin = shutil.which("bash")
    if bash_bin is None:
        raise AssertionError("未找到 bash 命令")
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
            if isolate_path:
                run_env["PATH"] = path_prefix
            else:
                run_env["PATH"] = f"{path_prefix}:{run_env['PATH']}"

        completed = subprocess.run(
            [bash_bin, "-c", f"set -euo pipefail\n{script}"],
            check=False,
            capture_output=True,
            text=True,
            env=run_env,
        )

        return completed, parse_key_value_file(github_output), parse_key_value_file(github_env)


class TestCrossRepoGongIntegration(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.cli_entrypoint_if = extract_job_if_expression(CLI_ENTRYPOINT_PATH, "orchestrate")
        cls.gong_entrypoint_if = extract_job_if_expression(GONG_ORCHESTRATE_PATH, "orchestrate")
        cls.dispatch_script = extract_step_run_script(GONG_DISPATCH_PATH, "Dispatch orchestrate wakeup")
        cls.warn_dispatch_script = extract_step_run_script(GONG_DISPATCH_PATH, "Warn Dispatch Failure").replace(
            "${{ github.event.pull_request.number }}",
            "${PR_NUMBER}",
        )
        cls.setup_niuma_script = extract_step_run_script(REUSABLE_PATH, "Setup niuma binary")

    def test_gong_consumes_cli_reusable_with_sha_pin(self) -> None:
        content = GONG_ORCHESTRATE_PATH.read_text(encoding="utf-8")
        self.assertRegex(
            content,
            r"uses:\s*biantaishabi2/Cli/\.github/workflows/niuma-orchestrate-reusable\.yml@[0-9a-f]{40}",
        )
        # build_niuma 已从 reusable workflow 移除，调用方传递会被忽略，不强制检查
        self.assertIn("label_whitelist: \"bot:queued,bot:pr-reviewable,bot:premerged\"", content)
        self.assertIn("types: [niuma.task.completed]", content)

    def test_gong_and_cli_entrypoint_trigger_matrix_consistent(self) -> None:
        cases = [
            {
                "name": "issues_queued",
                "event_name": "issues",
                "label_name": "bot:queued",
                "expected": True,
            },
            {
                "name": "issues_pr_reviewable",
                "event_name": "issues",
                "label_name": "bot:pr-reviewable",
                "expected": True,
            },
            {
                "name": "issues_premerged",
                "event_name": "issues",
                "label_name": "bot:premerged",
                "expected": True,
            },
            {
                "name": "issues_unknown",
                "event_name": "issues",
                "label_name": "bot:unknown",
                "expected": False,
            },
            {
                "name": "dispatch_completed",
                "event_name": "repository_dispatch",
                "action": "niuma.task.completed",
                "event_source": "close-after-integration-merge",
                "expected": True,
            },
            {
                "name": "dispatch_other_source",
                "event_name": "repository_dispatch",
                "action": "niuma.task.completed",
                "event_source": "other-source",
                "expected": False,
            },
        ]

        for case in cases:
            with self.subTest(case=case["name"]):
                cli_result = evaluate_entrypoint_if(
                    self.cli_entrypoint_if,
                    event_name=case["event_name"],
                    label_name=case.get("label_name", ""),
                    action=case.get("action", ""),
                    event_source=case.get("event_source", ""),
                )
                gong_result = evaluate_entrypoint_if(
                    self.gong_entrypoint_if,
                    event_name=case["event_name"],
                    label_name=case.get("label_name", ""),
                    action=case.get("action", ""),
                    event_source=case.get("event_source", ""),
                )
                self.assertEqual(cli_result, case["expected"])
                self.assertEqual(gong_result, case["expected"])

    def test_dispatch_step_builds_completed_payload_from_real_script(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = pathlib.Path(tmp_dir)
            gh_capture = tmp_path / "dispatch_payload.json"
            gh_stub = tmp_path / "gh"
            gh_stub.write_text(
                "\n".join(
                    [
                        "#!/usr/bin/env bash",
                        "cat > \"${GH_CAPTURE_FILE}\"",
                        "if [ \"${GH_FORCE_FAIL:-0}\" = \"1\" ]; then exit 1; fi",
                        "exit 0",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
            gh_stub.chmod(0o755)

            completed, _, _ = run_shell_script(
                self.dispatch_script,
                {
                    "GH_TOKEN": "fake-token",
                    "REPO": "biantaishabi2/gong",
                    "PR_NUMBER": "42",
                    "SOURCE_ISSUE": "340",
                    "SOURCE_ISSUES": "[340,341]",
                    "GITHUB_RUN_ID": "1001",
                    "GITHUB_RUN_ATTEMPT": "2",
                    "GH_CAPTURE_FILE": str(gh_capture),
                },
                path_prefix=tmp_dir,
            )

            self.assertEqual(completed.returncode, 0, msg=completed.stderr + completed.stdout)
            payload = json.loads(gh_capture.read_text(encoding="utf-8"))
            self.assertEqual(payload["event_type"], "niuma.task.completed")
            client_payload = payload["client_payload"]
            self.assertEqual(client_payload["source_issue"], 340)
            self.assertEqual(client_payload["source_issues"], [340, 341])
            self.assertEqual(client_payload["trigger_pr"], 42)
            self.assertEqual(client_payload["event_source"], "close-after-integration-merge")
            self.assertEqual(client_payload["event_id"], "pr-42-run-1001-2")

    def test_dispatch_failure_degrades_with_warning_and_continue_on_error_contract(self) -> None:
        content = GONG_DISPATCH_PATH.read_text(encoding="utf-8")
        self.assertIn("continue-on-error: true", content)

        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = pathlib.Path(tmp_dir)
            gh_capture = tmp_path / "dispatch_payload_fail.json"
            gh_stub = tmp_path / "gh"
            gh_stub.write_text(
                "\n".join(
                    [
                        "#!/usr/bin/env bash",
                        "cat > \"${GH_CAPTURE_FILE}\"",
                        "if [ \"${GH_FORCE_FAIL:-0}\" = \"1\" ]; then exit 1; fi",
                        "exit 0",
                    ]
                )
                + "\n",
                encoding="utf-8",
            )
            gh_stub.chmod(0o755)

            dispatch_run, _, _ = run_shell_script(
                self.dispatch_script,
                {
                    "GH_TOKEN": "fake-token",
                    "REPO": "biantaishabi2/gong",
                    "PR_NUMBER": "77",
                    "SOURCE_ISSUE": "",
                    "SOURCE_ISSUES": "[]",
                    "GITHUB_RUN_ID": "1002",
                    "GITHUB_RUN_ATTEMPT": "1",
                    "GH_CAPTURE_FILE": str(gh_capture),
                    "GH_FORCE_FAIL": "1",
                },
                path_prefix=tmp_dir,
            )
            self.assertNotEqual(dispatch_run.returncode, 0)

            warn_run, _, _ = run_shell_script(
                self.warn_dispatch_script,
                {
                    "PR_NUMBER": "77",
                },
            )
            self.assertEqual(warn_run.returncode, 0, msg=warn_run.stderr + warn_run.stdout)
            self.assertIn("::warning title=orchestrate dispatch failed::", warn_run.stdout)

    def test_setup_niuma_auto_detect_uses_preinstalled_binary(self) -> None:
        """无 automation/niuma 目录时，自动 fallback 到系统 niuma"""
        with tempfile.TemporaryDirectory() as niuma_dir:
            niuma_bin = pathlib.Path(niuma_dir) / "niuma"
            niuma_bin.write_text("#!/usr/bin/env bash\nexit 0\n", encoding="utf-8")
            niuma_bin.chmod(0o755)

            success_run, _, success_env = run_shell_script(
                self.setup_niuma_script,
                {
                    "NIUMA_BIN": "",
                },
                path_prefix=niuma_dir,
            )
            self.assertEqual(success_run.returncode, 0, msg=success_run.stderr + success_run.stdout)
            self.assertEqual(success_env.get("NIUMA_BIN"), str(niuma_bin))

        with tempfile.TemporaryDirectory() as empty_path_dir:
            failure_run, _, _ = run_shell_script(
                self.setup_niuma_script,
                {
                    "NIUMA_BIN": "",
                },
                path_prefix=empty_path_dir,
                isolate_path=True,
            )
            self.assertNotEqual(failure_run.returncode, 0)
            self.assertIn("找不到 niuma 二进制", failure_run.stderr + failure_run.stdout)


if __name__ == "__main__":
    unittest.main()
