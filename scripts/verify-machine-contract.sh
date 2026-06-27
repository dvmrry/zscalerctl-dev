#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [[ "${ZSCALERCTL_MACHINE_CONTRACT_SKIP_GO_TESTS:-}" != "1" ]]; then
  go test -mod=vendor ./internal/machine/... ./internal/machineio/...
fi

scan_root="${ZSCALERCTL_MACHINE_CONTRACT_SCAN_ROOT:-$repo_root}"
tmp_matches="$(mktemp)"
cleanup() {
  rm -f "$tmp_matches"
}
trap cleanup EXIT

(
  cd "$scan_root"
  while IFS= read -r -d '' file; do
    grep -nF 'NewProjectedRecordsFromProjectedFields(' "$file" |
      sed "s#^#${file}:#" || true
  done < <(
    find . -type f \
      -name '*.go' \
      ! -name '*_test.go' \
      ! -path './vendor/*' \
      ! -path './internal/resources/resources.go' \
      -print0
  )
) >"$tmp_matches"

if [[ -s "$tmp_matches" ]]; then
  echo "verify-machine-contract: trusted-only projected-record constructor used outside approved sites:" >&2
  sed 's/^/  /' "$tmp_matches" >&2
  echo "use resources.NewVerifiedProjectedRecordsFromProjectedFields at trust boundaries" >&2
  exit 1
fi

echo "verify-machine-contract: PASS"
