#!/usr/bin/env bash
set -euo pipefail

# Regression check: zscalerctl-tui --live --profile <invalid> must fail
# promptly without emitting terminal escape sequences or opening the full-screen
# TUI. This is a regression guard for startup terminal probes before main()
# reaches zscalerctl's own live-mode validation path.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

python3 scripts/verify-zscalerctl-tui-live-failure.py
