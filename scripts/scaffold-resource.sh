#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
usage: scripts/scaffold-resource.sh --product zia|zpa|ztw --resource NAME --package PKG --type TYPE [--out DIR] [--force]

Creates a fail-closed resource review bundle under scratch/resource-drafts by
default. The bundle is intentionally not applied to production files; review the
generated snippets, then apply the catalog, reader, docs, and tests deliberately.
USAGE
}

fail() {
  echo "scaffold-resource: $*" >&2
  exit 2
}

product=""
resource=""
package_path=""
type_name=""
out=""
force=false

while (($#)); do
  case "$1" in
    --product)
      (($# >= 2)) || fail "--product needs a value"
      product="$2"
      shift 2
      ;;
    --resource)
      (($# >= 2)) || fail "--resource needs a value"
      resource="$2"
      shift 2
      ;;
    --package)
      (($# >= 2)) || fail "--package needs a value"
      package_path="$2"
      shift 2
      ;;
    --type)
      (($# >= 2)) || fail "--type needs a value"
      type_name="$2"
      shift 2
      ;;
    --out)
      (($# >= 2)) || fail "--out needs a value"
      out="$2"
      shift 2
      ;;
    --force)
      force=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument $1"
      ;;
  esac
done

[[ -n "$product" ]] || fail "--product is required"
[[ -n "$resource" ]] || fail "--resource is required"
[[ -n "$package_path" ]] || fail "--package is required"
[[ -n "$type_name" ]] || fail "--type is required"

case "$product" in
  zia|zpa|ztw) ;;
  *) fail "--product must be zia, zpa, or ztw" ;;
esac

if [[ ! "$resource" =~ ^[a-z0-9-]+$ ]]; then
  fail "--resource must match [a-z0-9-]"
fi
if [[ ! "$type_name" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
  fail "--type must be a Go identifier"
fi
case "$package_path" in
  github.com/zscaler/zscaler-sdk-go/v3/*|./*) ;;
  *) fail "--package must be a Zscaler SDK import path or a relative test fixture path" ;;
esac

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root"

if [[ -z "$out" ]]; then
  out="scratch/resource-drafts/$product/$resource"
fi

marker="$out/.zscalerctl-resource-scaffold"
if [[ -e "$out" ]]; then
  if [[ "$force" != true ]]; then
    fail "$out already exists; pass --force to replace a prior scaffold"
  fi
  if [[ ! -f "$marker" ]]; then
    fail "$out exists but is not a scaffold directory; refusing --force"
  fi
  rm -rf -- "$out"
fi
mkdir -p "$out"
touch "$marker"

package_name="$(go list -f '{{.Name}}' -mod=mod "$package_path")"

title_resource() {
  local input="$1"
  local part
  local out_parts=()
  IFS='-' read -ra parts <<< "$input"
  for part in "${parts[@]}"; do
    case "$part" in
      ip|ips) out_parts+=("IP${part#ip}") ;;
      gre) out_parts+=("GRE") ;;
      zia) out_parts+=("ZIA") ;;
      zpa) out_parts+=("ZPA") ;;
      ztw) out_parts+=("ZTW") ;;
      "") ;;
      *) out_parts+=("$(tr '[:lower:]' '[:upper:]' <<< "${part:0:1}")${part:1}") ;;
    esac
  done
  printf '%s ' "${out_parts[@]}" | sed 's/[[:space:]]$//'
}

lower_camel_resource() {
  local input="$1"
  local part
  local first=true
  local out=""
  IFS='-' read -ra parts <<< "$input"
  for part in "${parts[@]}"; do
    [[ -n "$part" ]] || continue
    if [[ "$first" == true ]]; then
      out="$part"
      first=false
      continue
    fi
    out+="$(tr '[:lower:]' '[:upper:]' <<< "${part:0:1}")${part:1}"
  done
  printf '%s' "$out"
}

resource_title="$(title_resource "$resource")"
qualified_type="$package_name.$type_name"
product_upper="$(tr '[:lower:]' '[:upper:]' <<< "$product")"
mapper_name="$(lower_camel_resource "$resource")SourceRecord"

go run ./scripts/catalog-draft.go \
  --package "$package_path" \
  --type "$type_name" \
  --product "$product" \
  --resource "$resource" > "$out/catalog-and-shape-review.txt"

cat > "$out/README.md" <<EOF
# $product/$resource Resource Scaffold

Generated from:

- package: \`$package_path\`
- type: \`$type_name\`
- SDK reference: \`$qualified_type\`

This directory is a review bundle. It is not a patch. Apply each snippet only
after reviewing the generated classifications and ignored fields.

## Files

- \`catalog-and-shape-review.txt\`: generated catalog and SDK shape-review seeds.
- \`reader-wiring.md\`: reader adapter checklist for the generic list/get path.
- \`docs.md\`: starting resource-reference section.
- \`validation.md\`: local and live validation commands.

## Review Order

1. Review generated classifications and promote only fields that should render.
2. Keep ambiguous names dropped unless this resource gives them safe context.
3. Drop or explicitly model nested structures; do not render nested blobs whole.
4. Map the SDK shape faithfully in the reader; projection remains the curator.
5. Add nested-drop and canary tests for admin/user/secret/free-text subtrees.
6. Run the validation commands before opening the PR.
EOF

cat > "$out/reader-wiring.md" <<EOF
# Reader Wiring Notes

Target resource: \`$product/$resource\`

1. Locate the SDK list/get calls for \`$qualified_type\`.
2. Add the SDK import where the existing \`$product\` handlers are registered:

\`\`\`go
$package_name "$package_path"
\`\`\`

3. Register the resource with \`newListGetHandler[$qualified_type]\`.
4. Keep list/get closures thin: acquire through the existing service boundary,
   call the SDK read method, and return SDK records.
5. Add a mapper named after the resource, for example:

\`\`\`go
func $mapper_name(item $qualified_type) resources.SourceRecord {
    // Map the SDK response shape faithfully. Do not pre-filter for safety here.
}
\`\`\`

6. If the SDK get path uses integer identifiers, use the shared integer-ID
   helper already used by existing resources.
7. Add or update SDK shape-review entries for every mapped top-level and nested
   SDK type.
EOF

cat > "$out/docs.md" <<EOF
## $product_upper $resource_title

Commands:

\`\`\`sh
zscalerctl $product $resource list
zscalerctl $product $resource get <id>
zscalerctl dump --products $product --resources $product/$resource --out ./scratch-live-smoke
\`\`\`

Fields:

Use \`catalog-and-shape-review.txt\` as the starting point, then document only
the fields that remain renderable after review.

Notes:

- State any nested SDK structures that are mapped into source records but
  dropped by projection.
- State any free-text fields and their standard-only reason.
EOF

if [[ "$product" == "zia" ]]; then
  live_smoke_focus="To limit a focused retry to this resource only, run:

\`\`\`sh
make live-smoke LIVE_SMOKE_RESOURCES=$product/$resource
\`\`\`"
else
  live_smoke_focus="The default live smoke currently validates ZIA resources. For this non-ZIA
resource, run direct read commands and a focused dump with read-only credentials:

\`\`\`sh
zscalerctl $product $resource list
zscalerctl dump --products $product --resources $product/$resource --out ./scratch-live-smoke
\`\`\`"
fi

cat > "$out/validation.md" <<EOF
# Validation

Run after applying the reviewed scaffold:

\`\`\`sh
go test -mod=vendor ./internal/resources ./internal/zscaler -count=1
make check
make fuzz-smoke FUZZTIME=1s
\`\`\`

Run with read-only tenant credentials before merge when available:

\`\`\`sh
make live-smoke
\`\`\`

The default live smoke validates every current ZIA dump-supported resource.

$live_smoke_focus

Inspect the live smoke output for:

- unexpected empty projections
- denied or dropped sensitive keys
- over-redaction in identifiers/names
- pagination/count mismatches
- unexpected fields compared with the SDK shape review
EOF

cat > "$out/command.sh" <<EOF
#!/usr/bin/env bash
set -euo pipefail

scripts/scaffold-resource.sh \\
  --product "$product" \\
  --resource "$resource" \\
  --package "$package_path" \\
  --type "$type_name" \\
  --out "$out" \\
  --force
EOF
chmod +x "$out/command.sh"

cat <<EOF
resource scaffold written: $out

Next:
  1. review $out/catalog-and-shape-review.txt
  2. apply the reviewed catalog, shape review, mapper, docs, and tests
  3. run the commands in $out/validation.md
EOF
