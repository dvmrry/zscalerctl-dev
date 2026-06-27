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
