#!/usr/bin/env python3
"""Verify that normal CLI output is clean in an interactive PTY.

Runs the built zscalerctl binary in a real pseudo-terminal and checks that
`version --format json` and `introspect --format json` emit no ESC bytes and
produce valid JSON. This is a regression guard against TUI runtime behavior
leaking into the normal binary.

The child environment is sanitized (CI removed, TERM set to a color-capable
value) so that termenv does not short-circuit TTY detection in CI environments.
"""

import atexit
import json
import os
import pty
import select
import subprocess
import sys

TIMEOUT_SECONDS = 30


def build_binary(repo_root: str) -> str:
    binary = os.path.join(repo_root, "zscalerctl-pty-check")
    subprocess.run(
        ["go", "build", "-mod=vendor", "-o", binary, "./cmd/zscalerctl"],
        cwd=repo_root,
        check=True,
    )
    return binary


def clean_env() -> dict[str, str]:
    """Return a sanitized environment for the PTY child.

    Remove CI-style variables that can cause termenv/lipgloss to skip TTY
    detection, and set a color-capable TERM so the terminal is treated as
    interactive.
    """
    env = dict(os.environ)
    for key in list(env.keys()):
        if key.upper() in {"CI", "GITHUB_ACTIONS", "BUILDKITE", "GITLAB_CI", "CIRCLECI", "TRAVIS"}:
            env.pop(key, None)
    env["TERM"] = "xterm-256color"
    return env


def run_in_pty(binary: str, args: list[str]) -> tuple[bytes, int]:
    """Run a command in a PTY and return its stdout bytes and exit code."""
    env = clean_env()

    pid, master_fd = pty.fork()
    if pid == 0:
        os.execvpe(binary, [binary] + args, env)

    chunks = []
    try:
        while True:
            ready, _, _ = select.select([master_fd], [], [], TIMEOUT_SECONDS)
            if not ready:
                os.kill(pid, 9)
                raise TimeoutError(f"PTY read timed out after {TIMEOUT_SECONDS}s")
            chunk = os.read(master_fd, 4096)
            if not chunk:
                break
            chunks.append(chunk)
    except OSError:
        pass
    finally:
        os.close(master_fd)

    _, status = os.waitpid(pid, 0)
    exit_code = os.waitstatus_to_exitcode(status)

    return b"".join(chunks), exit_code


def validate_command(name: str, binary: str, args: list[str]) -> list[str]:
    output, exit_code = run_in_pty(binary, args)
    esc_count = output.count(b"\x1b")
    errors = []
    if esc_count != 0:
        errors.append(f"{name}: ESC bytes detected: {esc_count}")
    if exit_code != 0:
        errors.append(f"{name}: exit code {exit_code}")
    try:
        parsed = json.loads(output.decode("utf-8", errors="replace"))
    except json.JSONDecodeError as e:
        errors.append(f"{name}: invalid JSON: {e}")
        parsed = None
    if parsed is not None and not isinstance(parsed, dict):
        errors.append(f"{name}: JSON root is {type(parsed).__name__}, want object")
    return errors, output


def main() -> int:
    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    binary = build_binary(repo_root)
    atexit.register(lambda: os.path.exists(binary) and os.remove(binary))

    all_errors = []
    outputs = {}

    for name, args in [
        ("version --format json", ["version", "--format", "json"]),
        ("introspect --format json", ["introspect", "--format", "json"]),
    ]:
        errors, output = validate_command(name, binary, args)
        all_errors.extend(errors)
        outputs[name] = output

    if all_errors:
        print("PTY escape-clean check FAILED:", file=sys.stderr)
        for err in all_errors:
            print(f"  - {err}", file=sys.stderr)
        for name, output in outputs.items():
            print(f"{name} output ({len(output)} bytes): {output!r}", file=sys.stderr)
        return 1

    print("PTY escape-clean check OK: 0 ESC bytes, valid JSON object(s)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
