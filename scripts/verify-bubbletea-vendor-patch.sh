#!/usr/bin/env bash
set -euo pipefail

# Guard the Bubble Tea vendor tree against startup terminal probes. Bubble Tea
# v2 no longer vendors the v1 tea_init.go background-detection probe, so there is
# no local patch to restore. This script keeps that assumption explicit after
# `go mod vendor` or dependency refreshes.

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

bubbletea_dir="vendor/charm.land/bubbletea/v2"

if [[ ! -d "$bubbletea_dir" ]]; then
  echo "missing Bubble Tea v2 vendor directory: $bubbletea_dir" >&2
  exit 1
fi

if find "$bubbletea_dir" -name '*.go' -print0 |
  xargs -0 grep -nE '^[[:space:]]*func[[:space:]]+init[[:space:]]*\(' |
  grep -q .; then
  echo "Bubble Tea v2 vendor tree contains package init functions; review for startup terminal probes" >&2
  find "$bubbletea_dir" -name '*.go' -print0 |
    xargs -0 grep -nE '^[[:space:]]*func[[:space:]]+init[[:space:]]*\(' >&2
  exit 1
fi

if find "$bubbletea_dir" -name '*.go' -print0 |
  xargs -0 grep -nF 'HasDarkBackground' |
  grep -q .; then
  echo "Bubble Tea v2 vendor tree references HasDarkBackground; review for startup terminal probes" >&2
  find "$bubbletea_dir" -name '*.go' -print0 |
    xargs -0 grep -nF 'HasDarkBackground' >&2
  exit 1
fi

echo "Bubble Tea v2 vendor OK: no startup terminal probe detected"
