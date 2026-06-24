#!/usr/bin/env python3
"""Verify that zscalerctl-tui --live --profile <invalid> fails promptly.

Runs the built zscalerctl-tui binary in a real pseudo-terminal and checks that
an invalid-profile live invocation:

- returns promptly (does not hang on Bubble Tea/Lip Gloss terminal probes),
- emits no ESC bytes before exit,
- does not open the full-screen TUI,
- exits non-zero with a profile/config error.

The child environment is sanitized (CI variables removed, TERM set to a
color-capable value) so that the TUI gate does not short-circuit in CI
environments. This is a regression guard against Bubble Tea v1.x package-init
terminal probing that previously caused failure paths to hang before main().
"""

import atexit
import os
import pty
import re
import select
import subprocess
import sys

TIMEOUT_SECONDS = 10


def build_binary(repo_root: str) -> str:
    binary = os.path.join(repo_root, "zscalerctl-tui-live-failure-check")
    subprocess.run(
        ["go", "build", "-mod=vendor", "-o", binary, "./cmd/zscalerctl-tui"],
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
        if key.upper() in {"CI", "GITHUB_ACTIONS", "BUILDKITE", "GITLAB_CI", "CIRCLECI", "TRAVIS", "NO_COLOR"}:
            env.pop(key, None)
    env["TERM"] = "xterm-256color"
    return env


def run_in_pty(binary: str, args: list[str]) -> tuple[bytes, int]:
    """Run a command in a PTY and return its combined stdout+stderr bytes and exit code."""
    env = clean_env()

    pid, master_fd = pty.fork()
    if pid == 0:
        os.execvpe(binary, [binary] + args, env)

    chunks = []
    timed_out = False
    try:
        while True:
            ready, _, _ = select.select([master_fd], [], [], TIMEOUT_SECONDS)
            if not ready:
                timed_out = True
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

    return b"".join(chunks), exit_code if not timed_out else -1


def main() -> int:
    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    binary = build_binary(repo_root)
    atexit.register(lambda: os.path.exists(binary) and os.remove(binary))

    output, exit_code = run_in_pty(binary, ["--live", "--profile", "definitely-not-real"])
    text = output.decode("utf-8", errors="replace")
    esc_count = output.count(b"\x1b")

    errors = []
    if esc_count != 0:
        errors.append(f"ESC bytes detected: {esc_count}")
    if exit_code == 0:
        errors.append(f"expected non-zero exit code for invalid profile, got {exit_code}")
    if not re.search(r"profile|not found|invalid|config|credential", text, re.IGNORECASE):
        errors.append("output does not mention profile/config/credential error")
    if "Products / Resources" in text or "↑/↓" in text or "tab switch" in text:
        errors.append("full-screen TUI appears to have launched")
    if exit_code == -1:
        errors.append("process timed out (possible Bubble Tea init probe hang)")

    if errors:
        print("zscalerctl-tui live failure-path check FAILED:", file=sys.stderr)
        for err in errors:
            print(f"  - {err}", file=sys.stderr)
        print(f"exit code: {exit_code}", file=sys.stderr)
        print(f"output ({len(output)} bytes): {output!r}", file=sys.stderr)
        return 1

    print(f"zscalerctl-tui live failure-path check OK: exit={exit_code}, 0 ESC bytes, profile error returned promptly")
    return 0


if __name__ == "__main__":
    sys.exit(main())
