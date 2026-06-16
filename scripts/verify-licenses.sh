#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

allowed="${ZSCALERCTL_ALLOWED_LICENSES:-Apache-2.0,BSD-2-Clause,BSD-3-Clause,MIT,ISC,MPL-2.0}"
target="${ZSCALERCTL_LICENSE_TARGET:-./cmd/zscalerctl}"

# go-licenses is pinned in tools/go.mod for Renovate visibility. Resolve the
# version from that module so the verifier and Renovate pin cannot drift.
tool_version="$(cd tools && go list -m -f '{{.Version}}' github.com/google/go-licenses)"
if [[ -z "$tool_version" || "$tool_version" == "<nil>" ]]; then
  echo "go-licenses version not found in tools/go.mod" >&2
  exit 1
fi

# GOFLAGS=-mod=mod lets this verifier execute the pinned tool without
# interacting with vendor/.
GOFLAGS=-mod=mod go run "github.com/google/go-licenses@${tool_version}" check "$target" --allowed_licenses="$allowed"
