#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

mkdir -p "$tmp_dir/good/workflows" "$tmp_dir/good-action-only/workflows" "$tmp_dir/good-input-keys/workflows" "$tmp_dir/good-flow-step/workflows" "$tmp_dir/good-flow-inline/workflows" "$tmp_dir/good-flow-reusable/workflows" "$tmp_dir/good-reusable/workflows" "$tmp_dir/good-reusable/.github/workflows" "$tmp_dir/bad-reusable/workflows" "$tmp_dir/bad-reusable/.github/workflows" "$tmp_dir/bad-duplicate/workflows" "$tmp_dir/bad-tag/workflows" "$tmp_dir/bad-missing-comment/workflows" "$tmp_dir/bad-retired-runtime/workflows" "$tmp_dir/good-local/workflows" "$tmp_dir/good-local/local-action" "$tmp_dir/good-nested-local/workflows" "$tmp_dir/good-nested-local/local-action" "$tmp_dir/good-nested-local/nested-action" "$tmp_dir/flow-style-unpinned/workflows" "$tmp_dir/bad-local-external/workflows" "$tmp_dir/bad-local-external/local-action" "$tmp_dir/bad-escape/workflows" "$tmp_dir/bad-cycle/workflows" "$tmp_dir/bad-cycle/local-a" "$tmp_dir/bad-cycle/local-b" "$tmp_dir/bad-missing-metadata/workflows" "$tmp_dir/bad-missing-metadata/missing-action" "$tmp_dir/bad-malformed/workflows" "$tmp_dir/bad-dynamic/workflows"
mkdir -p "$tmp_dir/bad-retired-runtime-case/workflows" "$tmp_dir/bad-unreferenced-local/workflows" "$tmp_dir/bad-unreferenced-local/tools/proxy-action"
mkdir -p "$tmp_dir/bad-steps-alias/workflows" "$tmp_dir/bad-step-merge/workflows"
mkdir -p "$tmp_dir/good-workflows-override/.github/workflows" "$tmp_dir/good-workflows-override/.github/actions/local"

cat >"$tmp_dir/good/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
YAML

cat >"$tmp_dir/good-action-only/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c # v6.4.0
YAML

cat >"$tmp_dir/good-input-keys/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
        with:
          steps: build-and-test
          uses: documentation-only-input
YAML

cat >"$tmp_dir/good-reusable/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  call:
    uses: ./.github/workflows/reusable.yml
YAML

cat >"$tmp_dir/good-reusable/.github/workflows/reusable.yml" <<'YAML'
name: reusable
on:
  workflow_call:
jobs:
  nested:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
YAML

cat >"$tmp_dir/good-flow-reusable/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  call: {uses: example/reusable/.github/workflows/reusable.yml@0123456789012345678901234567890123456789} # v1.2.3
YAML

cat >"$tmp_dir/bad-duplicate/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
        uses: example/action@main
YAML

cat >"$tmp_dir/bad-reusable/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  call:
    uses: ./.github/workflows/reusable.yml
YAML

cat >"$tmp_dir/bad-reusable/.github/workflows/reusable.yml" <<'YAML'
name: reusable
on:
  workflow_call:
jobs:
  nested:
    runs-on: ubuntu-latest
    steps:
      - uses: example/action@main
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

cat >"$tmp_dir/bad-retired-runtime-case/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: Gitleaks/Gitleaks-Action@FF98106E4C7B2BC287B24EAF42907196329070C7 # v2.3.9
YAML

cat >"$tmp_dir/good-local/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: ./local-action
YAML

cat >"$tmp_dir/good-local/local-action/action.yml" <<'YAML'
name: local-action
description: valid local composite action fixture
runs:
  using: composite
  steps:
    - run: echo valid
      shell: bash
YAML

cat >"$tmp_dir/good-workflows-override/.github/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: ./.github/actions/local
YAML

cat >"$tmp_dir/good-workflows-override/.github/actions/local/action.yml" <<'YAML'
name: local-action
description: valid repository-root-relative local action fixture
runs:
  using: composite
  steps:
    - run: echo valid
      shell: bash
YAML

cat >"$tmp_dir/flow-style-unpinned/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps: [{uses: example/action@main}]
YAML

cat >"$tmp_dir/good-flow-step/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - {uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd} # v6.0.2
YAML

cat >"$tmp_dir/good-flow-inline/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps: [{uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd}] # v6.0.2
YAML

cat >"$tmp_dir/bad-local-external/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: ./local-action
YAML

cat >"$tmp_dir/bad-local-external/local-action/action.yml" <<'YAML'
name: bad-local-action
description: local composite with an unpinned external action
runs:
  using: composite
  steps:
    - uses: example/action@main
YAML

cat >"$tmp_dir/bad-unreferenced-local/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo no-local-reference
YAML

cat >"$tmp_dir/bad-unreferenced-local/tools/proxy-action/action.yml" <<'YAML'
name: unreferenced-proxy
description: unreferenced local action must still satisfy repository policy
runs:
  using: composite
  steps:
    - uses: example/action@main
YAML

cat >"$tmp_dir/good-nested-local/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: ./local-action
YAML

cat >"$tmp_dir/good-nested-local/local-action/action.yml" <<'YAML'
name: outer-local-action
description: nested local composite action
runs:
  using: composite
  steps:
    - uses: ./nested-action
YAML

cat >"$tmp_dir/good-nested-local/nested-action/action.yaml" <<'YAML'
name: nested-local-action
description: leaf local composite action
runs:
  using: composite
  steps:
    - run: echo nested
      shell: bash
YAML

cat >"$tmp_dir/bad-escape/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    steps:
      - uses: ./../outside-action
YAML

cat >"$tmp_dir/bad-cycle/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    steps:
      - uses: ./local-a
YAML

cat >"$tmp_dir/bad-cycle/local-a/action.yml" <<'YAML'
name: local-a
description: cycle fixture A
runs:
  using: composite
  steps:
    - uses: ./local-b
YAML

cat >"$tmp_dir/bad-cycle/local-b/action.yml" <<'YAML'
name: local-b
description: cycle fixture B
runs:
  using: composite
  steps:
    - uses: ./local-a
YAML

cat >"$tmp_dir/bad-missing-metadata/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    steps:
      - uses: ./missing-action
YAML

cat >"$tmp_dir/bad-malformed/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    steps: [
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
YAML

cat >"$tmp_dir/bad-dynamic/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
jobs:
  test:
    steps: ${{ matrix.steps }}
YAML

cat >"$tmp_dir/bad-steps-alias/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
shared: &shared
  - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
jobs:
  test:
    steps: *shared
YAML

cat >"$tmp_dir/bad-step-merge/workflows/ci.yml" <<'YAML'
name: ci
on: [push]
shared: &shared
  uses: example/action@main
jobs:
  test:
    steps:
      - <<: *shared
YAML

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good" \
	"$repo_root/scripts/verify-actions-pinned.sh"

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good-action-only" \
	"$repo_root/scripts/verify-actions-pinned.sh"

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good-input-keys" \
	"$repo_root/scripts/verify-actions-pinned.sh"

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good-reusable" \
	"$repo_root/scripts/verify-actions-pinned.sh"

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good-flow-reusable" \
	"$repo_root/scripts/verify-actions-pinned.sh"

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good-local" \
	"$repo_root/scripts/verify-actions-pinned.sh"

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good-workflows-override/.github/workflows" \
	"$repo_root/scripts/verify-actions-pinned.sh"

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good-flow-step" \
	"$repo_root/scripts/verify-actions-pinned.sh"

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good-flow-inline" \
	"$repo_root/scripts/verify-actions-pinned.sh"

ZSCALERCTL_GITHUB_DIR="$tmp_dir/good-nested-local" \
	"$repo_root/scripts/verify-actions-pinned.sh"

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/flow-style-unpinned" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted an unpinned flow-style external action" >&2
	cat "$tmp_dir/out" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi
if ! grep -q "not pinned to a full commit SHA" "$tmp_dir/err"; then
	echo "verify-actions-pinned failed without the expected flow-style pinning message" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-duplicate" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted duplicate uses keys" >&2
	exit 1
fi
grep -q 'duplicate key "uses"' "$tmp_dir/err"

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-reusable" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted an unpinned action in a local reusable workflow" >&2
	exit 1
fi
grep -q "reusable.yml" "$tmp_dir/err"
grep -q "not pinned to a full commit SHA" "$tmp_dir/err"

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-local-external" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted an unpinned action in a reachable local composite action" >&2
	exit 1
fi
grep -q "action.yml" "$tmp_dir/err"
grep -q "not pinned to a full commit SHA" "$tmp_dir/err"

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-unreferenced-local" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned ignored an unreferenced repository-local action" >&2
	exit 1
fi
grep -q "proxy-action/action.yml" "$tmp_dir/err"
grep -q "not pinned to a full commit SHA" "$tmp_dir/err"

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-escape" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted a local action traversal escape" >&2
	exit 1
fi
grep -q "escapes repository" "$tmp_dir/err"

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-cycle" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted a local action dependency cycle" >&2
	exit 1
fi
grep -q "dependency cycle" "$tmp_dir/err"

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-missing-metadata" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted a local action without metadata" >&2
	exit 1
fi
grep -q "action.yml/action.yaml metadata not found" "$tmp_dir/err"

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-malformed" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted malformed YAML" >&2
	exit 1
fi
grep -q "malformed YAML" "$tmp_dir/err"
if grep -Fq '%!(EXTRA' "$tmp_dir/err"; then
	echo "verify-actions-pinned emitted a malformed Go formatting diagnostic" >&2
	cat "$tmp_dir/err" >&2
	exit 1
fi

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-dynamic" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted a dynamic steps structure" >&2
	exit 1
fi
grep -q "unsupported or dynamic steps structure" "$tmp_dir/err"

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-steps-alias" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted an aliased steps structure" >&2
	exit 1
fi
grep -q "unsupported or dynamic steps structure" "$tmp_dir/err"

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-step-merge" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted a merge-key action indirection" >&2
	exit 1
fi
grep -q "unsupported non-string key in workflow step" "$tmp_dir/err"

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

if ZSCALERCTL_GITHUB_DIR="$tmp_dir/bad-retired-runtime-case" \
	"$repo_root/scripts/verify-actions-pinned.sh" >"$tmp_dir/out" 2>"$tmp_dir/err"; then
	echo "verify-actions-pinned accepted a case-varied action with a retired Node runtime" >&2
	exit 1
fi
grep -q "action uses a retired Node runtime" "$tmp_dir/err"
