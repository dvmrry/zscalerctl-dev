#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

sdk="$tmp/sdk"
mkdir -p "$sdk/zscaler/zia/services/example"
mkdir -p "$sdk/zscaler/zcc"
mkdir -p "$sdk/zscaler/core"

cat > "$sdk/zscaler/zia/services/example/example.go" <<'GO'
package example

const endpoint = "/zia/api/v1/examples"

type Example struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func Get(_ any, _ any, _ int) (*Example, error) {
	_ = "GET"
	return nil, nil
}

func GetAll(_ any, _ any) ([]Example, error) {
	_ = "GET"
	return nil, nil
}

func Update(_ any, _ any, _ int, _ Example) (*Example, error) {
	_ = "PUT"
	return nil, nil
}
GO

cat > "$sdk/zscaler/zcc/v2_client.go" <<'GO'
package zcc

type Client struct {
	BaseURL string
}

func NewClient() *Client {
	return &Client{}
}
GO

cat > "$sdk/zscaler/core/oneapi.go" <<'GO'
package core

const adminEndpoint = "/admin/api/v1/users"
GO

go run ./scripts/sdk-surface-inventory.go \
  --sdk-dir "$sdk" \
  --module-path github.com/zscaler/zscaler-sdk-go/v3 \
  > "$tmp/inventory.md"

grep -q '| `zia` | `zscaler/zia/services/example` | `list-get-with-mutating-neighbors` ' "$tmp/inventory.md"
grep -q '`Get`<br>`GetAll`' "$tmp/inventory.md"
grep -q '`Update`' "$tmp/inventory.md"
grep -q '`/zia/api/v1/examples`' "$tmp/inventory.md"
grep -q '| `zcc` | `zscaler/zcc` | `product-client-config` ' "$tmp/inventory.md"
grep -q 'no high-level resource service package detected in this SDK snapshot' "$tmp/inventory.md"
grep -q '| `zidentity` | `zscaler/core` | `other` ' "$tmp/inventory.md"
grep -q 'identity-plane work' "$tmp/inventory.md"

go run ./scripts/sdk-surface-inventory.go \
  --sdk-dir "$sdk" \
  --module-path github.com/zscaler/zscaler-sdk-go/v3 \
  --format json > "$tmp/inventory.json"

grep -q '"product": "zia"' "$tmp/inventory.json"
grep -q '"category": "list-get-with-mutating-neighbors"' "$tmp/inventory.json"

if go run ./scripts/sdk-surface-inventory.go --sdk-dir "$sdk" --format xml >"$tmp/xml.out" 2>"$tmp/xml.err"; then
  echo "sdk-surface-inventory accepted unsupported format" >&2
  exit 1
fi
grep -q 'unsupported format "xml"' "$tmp/xml.err"
