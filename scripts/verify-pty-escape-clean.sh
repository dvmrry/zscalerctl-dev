#!/usr/bin/env bash
set -euo pipefail

# Regression check: normal CLI machine output must be clean even when run in a
# real PTY.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

python3 scripts/verify-pty-escape-clean.py
