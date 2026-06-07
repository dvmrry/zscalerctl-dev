#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

mkdir -p \
	"$tmp_dir/good/workflows" \
	"$tmp_dir/bad-env/workflows" \
	"$tmp_dir/bad-secret/workflows" \
	"$tmp_dir/bad-zia-env/workflows" \
	"$tmp_dir/bad-zpa-secret/workflows" \
	"$tmp_dir/bad-oneapi-env/workflows" \
	"$tmp_dir/bad-oneapi-secret/workflows" \
	"$tmp_dir/bad-zscalerctl-zia-env/workflows" \
	"$tmp_dir/bad-zscalerctl-zpa-env/workflows" \
	"$tmp_dir/bad-composite-action/actions/live-smoke"

cat >"$tmp_dir/good/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: make release-check
YAML

cat >"$tmp_dir/bad-env/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  smoke:
    runs-on: ubuntu-latest
    env:
      ZSCALERCTL_CLIENT_SECRET: ${{ secrets.CLIENT_SECRET }}
    steps:
      - run: zscalerctl zia locations list
YAML

cat >"$tmp_dir/bad-secret/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  smoke:
    runs-on: ubuntu-latest
    steps:
      - run: zscalerctl zia locations list
        env:
          CLIENT_SECRET: ${{ secrets.PROD_ZSCALER_CLIENT_SECRET }}
YAML

cat >"$tmp_dir/bad-zia-env/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  smoke:
    runs-on: ubuntu-latest
    env:
      ZIA_USERNAME: analyst@example.com
      ZIA_API_KEY: ${{ secrets.LEGACY_API_KEY }}
    steps:
      - run: zscalerctl zia locations list
YAML

cat >"$tmp_dir/bad-zpa-secret/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  smoke:
    runs-on: ubuntu-latest
    steps:
      - run: zscalerctl zpa app-segments list
        env:
          CLIENT_ID: ${{ secrets.PROD_ZPA_CLIENT_ID }}
YAML

cat >"$tmp_dir/bad-oneapi-env/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  smoke:
    runs-on: ubuntu-latest
    env:
      ZSCALER_CLIENT_ID: ${{ secrets.ONEAPI_CLIENT_ID }}
      ZSCALER_PRIVATE_KEY: ${{ secrets.ONEAPI_PRIVATE_KEY }}
      ZSCALER_CLOUD: PRODUCTION
    steps:
      - run: zscalerctl zia locations list
YAML

cat >"$tmp_dir/bad-oneapi-secret/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  smoke:
    runs-on: ubuntu-latest
    steps:
      - run: zscalerctl zia locations list
        env:
          CLIENT_SECRET: ${{ secrets.PROD_ONEAPI_CLIENT_SECRET }}
YAML

cat >"$tmp_dir/bad-zscalerctl-zia-env/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  smoke:
    runs-on: ubuntu-latest
    env:
      ZSCALERCTL_AUTH_MODE: zia-legacy
      ZSCALERCTL_ZIA_USERNAME: analyst@example.com
      ZSCALERCTL_ZIA_PASSWORD: ${{ secrets.LEGACY_ZIA_PASSWORD }}
      ZSCALERCTL_ZIA_API_KEY: ${{ secrets.LEGACY_ZIA_API_KEY }}
      ZSCALERCTL_ZIA_CLOUD: zscalerthree
    steps:
      - run: zscalerctl zia locations list
YAML

cat >"$tmp_dir/bad-zscalerctl-zpa-env/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  smoke:
    runs-on: ubuntu-latest
    env:
      ZSCALERCTL_ZPA_CUSTOMER_ID: "123456789"
    steps:
      - run: zscalerctl zpa server-groups list
YAML

cat >"$tmp_dir/bad-composite-action/actions/live-smoke/action.yml" <<'YAML'
name: live smoke
runs:
  using: composite
  steps:
    - shell: bash
      env:
        ZSCALERCTL_CLIENT_SECRET: ${{ inputs.client-secret }}
      run: zscalerctl zia locations list
YAML

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good" \
	"$repo_root/scripts/verify-ci-no-live-creds.sh"

for bad_dir in "$tmp_dir/bad-env" "$tmp_dir/bad-secret" "$tmp_dir/bad-zia-env" "$tmp_dir/bad-zpa-secret" "$tmp_dir/bad-oneapi-env" "$tmp_dir/bad-oneapi-secret" "$tmp_dir/bad-zscalerctl-zia-env" "$tmp_dir/bad-zscalerctl-zpa-env" "$tmp_dir/bad-composite-action"; do
	if ZSCALERCTL_GITHUB_DIR="$bad_dir" \
		"$repo_root/scripts/verify-ci-no-live-creds.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
		echo "verify-ci-no-live-creds accepted a workflow with live credential inputs: $bad_dir" >&2
		cat "$tmp_dir/out" >&2
		cat "$tmp_dir/err" >&2
		exit 1
	fi

	if ! grep -q "GitHub Actions config references live Zscaler credential inputs" "$tmp_dir/err"; then
		echo "verify-ci-no-live-creds failed without the expected credential-input message" >&2
		cat "$tmp_dir/err" >&2
		exit 1
	fi
done
