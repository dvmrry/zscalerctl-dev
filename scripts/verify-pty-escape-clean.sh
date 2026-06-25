#!/usr/bin/env bash
set -euo pipefail

# Regression check: normal CLI output must be clean even when run in a real PTY.
# This catches TUI runtime behavior that would emit OSC/DSR sequences before
# main() if Bubble Tea were linked into the normal zscalerctl binary.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

python3 scripts/verify-pty-escape-clean.py
