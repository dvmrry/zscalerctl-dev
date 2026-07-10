#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

mkdir -p "$tmp_dir/good/workflows" "$tmp_dir/bad-tag/workflows" "$tmp_dir/bad-missing-comment/workflows" "$tmp_dir/bad-retired-runtime/workflows" "$tmp_dir/good-local/workflows"

cat >"$tmp_dir/good/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
YAML

cat >"$tmp_dir/bad-tag/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
YAML

cat >"$tmp_dir/bad-missing-comment/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd
YAML

cat >"$tmp_dir/bad-retired-runtime/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: gitleaks/gitleaks-action@ff98106e4c7b2bc287b24eaf42907196329070c7 # v2.3.9
YAML

cat >"$tmp_dir/good-local/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: ./.github/actions/local-action
YAML

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good" \
	"$repo_root/scripts/verify-actions-pinned.sh"

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good-local" \
	"$repo_root/scripts/verify-actions-pinned.sh"

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-tag" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted a tag-pinned external action" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "not pinned to a full commit SHA" "$tmp_dir/err"; then
	echo "verify-actions-pinned failed without the expected tag-pinning message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-missing-comment" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted a SHA-pinned action without a Renovate version comment" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "missing a Renovate version comment" "$tmp_dir/err"; then
	echo "verify-actions-pinned failed without the expected missing-comment message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-retired-runtime" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted an action with a retired Node runtime" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ! grep -q "action uses a retired Node runtime" "$tmp_dir/err"; then
	echo "verify-actions-pinned failed without the expected retired-runtime message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi
