#!/usr/bin/env bash
set -euo pipefail

# Regression check: zscalerctl-tui --live --profile <invalid> must fail
# promptly without emitting terminal escape sequences or opening the full-screen
# TUI. This catches the Bubble Tea v1.x package-init hang where
# lipgloss.HasDarkBackground() emitted OSC/DSR probes before main() and blocked
# on failure paths.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

python3 scripts/verify-zscalerctl-tui-live-failure.py
