#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

go run ./scripts/catalog-draft.go \
  --package ./scripts/testdata/catalogdraft/fixture \
  --type Example \
  --product zia \
  --resource draft-example >"$tmp_dir/draft.txt"

require_contains() {
  local pattern="$1"
  if ! grep -Fq "$pattern" "$tmp_dir/draft.txt"; then
    echo "catalog draft missing expected pattern: $pattern" >&2
    cat "$tmp_dir/draft.txt" >&2
    exit 1
  fi
}

require_absent() {
  local pattern="$1"
  if grep -Fq "$pattern" "$tmp_dir/draft.txt"; then
    echo "catalog draft contained unexpected pattern: $pattern" >&2
    cat "$tmp_dir/draft.txt" >&2
    exit 1
  fi
}

require_field_contains() {
  local field="$1"
  local pattern="$2"
  if ! awk -v field="$field" -v pattern="$pattern" '
    $0 ~ "Name:[[:space:]]+\"" field "\"" { in_field = 1 }
    in_field && index($0, pattern) { found = 1 }
    in_field && $0 ~ /^[[:space:]]*},/ { in_field = 0 }
    END { exit found ? 0 : 1 }
  ' "$tmp_dir/draft.txt"; then
    echo "catalog draft field $field missing expected pattern: $pattern" >&2
    cat "$tmp_dir/draft.txt" >&2
    exit 1
  fi
}

require_contains 'Product:    ProductZIA'
require_contains 'Review posture: generated drafts classify every SDK field they can model.'
require_contains 'Only approved global names render by default; ambiguous, unknown, opaque, and secret-like names stay ClassSecret.'
require_contains 'Shape-review import hint: fixture "github.com/dvmrry/zscalerctl/scripts/testdata/catalogdraft/fixture"'
require_contains 'Name:       "draft-example"'
require_contains 'Name:           "clientSecret"'
require_contains 'Name:           "sessionToken"'
require_contains 'Classification: ClassSecret'
require_field_contains 'clientId' 'Classification: ClassSecret'
require_field_contains 'value' 'Classification: ClassSecret'
require_field_contains 'metadata' 'Classification: ClassSecret'
require_field_contains 'NoTag' 'Classification: ClassSecret'
require_contains 'catalogFields: []string{'
require_contains '"clientSecret",'
require_contains '"sessionToken",'
require_field_contains 'description' 'Classification: ClassFreeText'
require_contains 'StandardFreeTextReason: standardFreeTextReason("TODO description")'
require_contains 'Name:           "children"'
require_contains 'Fields: []FieldSpec{'
require_field_contains 'id' 'Classification: ClassOperational'
require_field_contains 'name' 'Classification: ClassTenantConfig'
require_contains 'ignoredFields: map[string]string{}'
require_absent 'ignoredBecause('
require_absent 'Hidden'
require_absent 'privateTagged'

if go run ./scripts/catalog-draft.go \
  --package example.com/elsewhere \
  --type Example \
  --product zia \
  --resource draft-example >"$tmp_dir/outside.txt" 2>"$tmp_dir/outside.err"; then
  echo "catalog draft accepted a package outside the Zscaler SDK module" >&2
  exit 1
fi

if ! grep -Fq 'outside the Zscaler SDK module' "$tmp_dir/outside.err"; then
  echo "catalog draft outside-package failure did not explain the boundary" >&2
  cat "$tmp_dir/outside.err" >&2
  exit 1
fi
