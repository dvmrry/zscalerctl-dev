#!/usr/bin/env bash
# verify-cli-docs.sh — drift gate for docs/cli/zscalerctl.md.
#
# Regenerates the CLI reference to a temp file and compares it against the
# committed docs/cli/zscalerctl.md. Fails with a diff if the committed file
# is stale (i.e. a command or flag changed but the docs were not regenerated).
#
# Fix: run `go run ./scripts/gen-cli-docs.go` and commit the updated file.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

committed="docs/cli/zscalerctl.md"
tmpfile="$(mktemp)"
cleanup() { rm -f "$tmpfile"; }
trap cleanup EXIT

if ! go run -mod=vendor ./scripts/gen-cli-docs.go --out "$tmpfile"; then
  echo "verify-cli-docs: generator failed" >&2
  exit 1
fi

if ! diff -u "$committed" "$tmpfile"; then
  echo "" >&2
  echo "verify-cli-docs: $committed is stale — run 'go run ./scripts/gen-cli-docs.go' and commit the result" >&2
  exit 1
fi
