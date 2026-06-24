#!/usr/bin/env python3
"""Verify that normal CLI output is clean in an interactive PTY.

Runs the built zscalerctl binary in a real pseudo-terminal and checks that
`version --format json` emits no ESC bytes and produces valid JSON. This is a
regression guard against Bubble Tea v1.x package-init terminal probing, which can
emit OSC/DSR sequences before main() when linked into the normal binary.
"""

import json
import os
import pty
import subprocess
import sys


def main() -> int:
    repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    binary = os.path.join(repo_root, "zscalerctl-pty-check")

    # Build a fresh binary for the exact module under test.
    subprocess.run(
        ["go", "build", "-mod=vendor", "-o", binary, "./cmd/zscalerctl"],
        cwd=repo_root,
        check=True,
    )

    pid, master_fd = pty.fork()
    if pid == 0:
        # Child: run the command in the PTY.
        os.execv(binary, [binary, "version", "--format", "json"])

    # Parent: read until EOF.
    chunks = []
    try:
        while True:
            chunk = os.read(master_fd, 4096)
            if not chunk:
                break
            chunks.append(chunk)
    except OSError:
        pass
    os.close(master_fd)

    _, status = os.waitpid(pid, 0)
    exit_code = os.waitstatus_to_exitcode(status)

    output = b"".join(chunks)
    esc_count = output.count(b"\x1b")

    errors = []
    if esc_count != 0:
        errors.append(f"ESC bytes detected: {esc_count}")

    if exit_code != 0:
        errors.append(f"exit code {exit_code}")

    try:
        parsed = json.loads(output.decode("utf-8", errors="replace"))
    except json.JSONDecodeError as e:
        errors.append(f"invalid JSON: {e}")
        parsed = None

    if parsed is not None and not isinstance(parsed, dict):
        errors.append(f"JSON root is {type(parsed).__name__}, want object")

    if errors:
        print("PTY escape-clean check FAILED:", file=sys.stderr)
        for err in errors:
            print(f"  - {err}", file=sys.stderr)
        print(f"output ({len(output)} bytes): {output!r}", file=sys.stderr)
        return 1

    print("PTY escape-clean check OK: 0 ESC bytes, valid JSON object")
    return 0


if __name__ == "__main__":
    sys.exit(main())
