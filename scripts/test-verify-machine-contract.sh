#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

mkdir -p "$tmpdir/internal/resources" "$tmpdir/internal/cli"

cat >"$tmpdir/internal/resources/resources.go" <<'EOF'
package resources

func NewProjectedRecordsFromProjectedFields() {}
EOF

cat >"$tmpdir/internal/resources/resources_test.go" <<'EOF'
package resources

func TestTrustedHelperUse() {
	NewProjectedRecordsFromProjectedFields()
}
EOF

ZSCALERCTL_MACHINE_CONTRACT_SKIP_GO_TESTS=1 \
  ZSCALERCTL_MACHINE_CONTRACT_SCAN_ROOT="$tmpdir" \
  bash scripts/verify-machine-contract.sh >/dev/null

cp docs/schema/machine-manifest.schema.json "$tmpdir/machine-manifest.schema.json"
cp internal/machine/testdata/contract/manifest.json "$tmpdir/manifest.json"
ZSCALERCTL_MACHINE_CONTRACT_SKIP_GO_TESTS=1 \
  ZSCALERCTL_MACHINE_CONTRACT_SCAN_ROOT="$tmpdir" \
  ZSCALERCTL_MACHINE_MANIFEST_SCHEMA="$tmpdir/machine-manifest.schema.json" \
  ZSCALERCTL_MACHINE_MANIFEST_FIXTURE="$tmpdir/manifest.json" \
  bash scripts/verify-machine-contract.sh >/dev/null

cat >"$tmpdir/bad-manifest.json" <<'EOF'
{
  "version": "machine.v1",
  "capabilities": [],
  "schemas": []
}
EOF

out="$tmpdir/schema-out.txt"
if ZSCALERCTL_MACHINE_CONTRACT_SKIP_GO_TESTS=1 \
  ZSCALERCTL_MACHINE_CONTRACT_SCAN_ROOT="$tmpdir" \
  ZSCALERCTL_MACHINE_MANIFEST_SCHEMA="$tmpdir/machine-manifest.schema.json" \
  ZSCALERCTL_MACHINE_MANIFEST_FIXTURE="$tmpdir/bad-manifest.json" \
  bash scripts/verify-machine-contract.sh >"$out" 2>&1; then
  echo "verify-machine-contract accepted a manifest fixture that does not match schema" >&2
  exit 1
fi

if ! grep -q "machine manifest schema validation failed" "$out"; then
  echo "verify-machine-contract schema failure did not explain manifest schema validation" >&2
  cat "$out" >&2
  exit 1
fi

cat >"$tmpdir/internal/cli/app.go" <<'EOF'
package cli

import "github.com/dvmrry/zscalerctl/internal/resources"

func bad() {
	resources.NewProjectedRecordsFromProjectedFields(nil)
}
EOF

out="$tmpdir/out.txt"
if ZSCALERCTL_MACHINE_CONTRACT_SKIP_GO_TESTS=1 \
  ZSCALERCTL_MACHINE_CONTRACT_SCAN_ROOT="$tmpdir" \
  bash scripts/verify-machine-contract.sh >"$out" 2>&1; then
  echo "verify-machine-contract accepted trusted-only constructor use in internal/cli" >&2
  exit 1
fi

if ! grep -q "internal/cli/app.go" "$out"; then
  echo "verify-machine-contract failure did not name internal/cli/app.go" >&2
  cat "$out" >&2
  exit 1
fi
