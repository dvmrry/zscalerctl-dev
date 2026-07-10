#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
verifier="$repo_root/scripts/verify-go-toolchain.sh"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

make_fixture() {
	local dir="$1"
	mkdir -p "$dir/tools" "$dir/experiments/adapter" "$dir/.github/workflows"
	for module in "$dir/go.mod" "$dir/tools/go.mod" "$dir/experiments/adapter/go.mod"; do
		module_name="example.invalid/$(basename "$(dirname "$module")")"
		cat >"$module" <<EOF
module $module_name

go 1.26.5
EOF
	done
	cat >"$dir/.github/workflows/ci.yml" <<'YAML'
jobs:
  test:
    steps:
      - uses: actions/setup-go@example
        with:
          go-version: '1.26.5'
YAML
}

run_verify() {
	local fixture="$1"
	ZSCALERCTL_REPO_ROOT="$fixture" "$verifier"
}

good="$tmpdir/good"
make_fixture "$good"
run_verify "$good"

named_step="$tmpdir/named-step"
make_fixture "$named_step"
perl -0pi -e 's/- uses: actions\/setup-go@example/- name: Setup Go\n        uses: actions\/setup-go@example/' "$named_step/.github/workflows/ci.yml"
run_verify "$named_step"

flow_style_good="$tmpdir/flow-style-good"
make_fixture "$flow_style_good"
cat >>"$flow_style_good/.github/workflows/ci.yml" <<'YAML'

  flow-style-good:
    steps: [{uses: actions/setup-go@example, with: {go-version: '1.26.5'}}]
YAML
run_verify "$flow_style_good"

flow_style_stale="$tmpdir/flow-style-stale"
make_fixture "$flow_style_stale"
cat >>"$flow_style_stale/.github/workflows/ci.yml" <<'YAML'

  flow-style-stale:
    steps: [{uses: actions/setup-go@example, with: {go-version: '1.26.4'}}]
YAML
if run_verify "$flow_style_stale" >"$tmpdir/flow-style-stale.out" 2>"$tmpdir/flow-style-stale.err"; then
	echo "verify-go-toolchain accepted a stale flow-style setup-go step" >&2
	exit 1
fi
if ! grep -q 'does not match root security minimum' "$tmpdir/flow-style-stale.err"; then
	echo "verify-go-toolchain reported the wrong flow-style stale setup-go error" >&2
	cat "$tmpdir/flow-style-stale.err" >&2
	exit 1
fi

case_varied_stale="$tmpdir/case-varied-stale"
make_fixture "$case_varied_stale"
perl -0pi -e 's#actions/setup-go@example#Actions/Setup-Go@example#; s/1\.26\.5/1.26.4/' "$case_varied_stale/.github/workflows/ci.yml"
if run_verify "$case_varied_stale" >"$tmpdir/case-varied-stale.out" 2>"$tmpdir/case-varied-stale.err"; then
	echo "verify-go-toolchain accepted a stale case-varied setup-go step" >&2
	exit 1
fi
grep -q 'does not match root security minimum' "$tmpdir/case-varied-stale.err"

named_unpinned="$tmpdir/named-unpinned"
make_fixture "$named_unpinned"
perl -0pi -e 's/- uses: actions\/setup-go@example/- name: Setup Go\n        uses: actions\/setup-go@example/; s/\n[[:space:]]+go-version: '\''1\.26\.5'\''//' "$named_unpinned/.github/workflows/ci.yml"
if run_verify "$named_unpinned" >"$tmpdir/named-unpinned.out" 2>"$tmpdir/named-unpinned.err"; then
	echo "verify-go-toolchain accepted an unpinned named setup-go step" >&2
	exit 1
fi
grep -q 'setup-go step is missing with.go-version' "$tmpdir/named-unpinned.err"

nested_local="$tmpdir/nested-local"
make_fixture "$nested_local"
mkdir -p "$nested_local/local-action" "$nested_local/nested-action"
cat >"$nested_local/.github/workflows/ci.yml" <<'YAML'
jobs:
  test:
    steps:
      - uses: ./local-action
YAML
cat >"$nested_local/local-action/action.yml" <<'YAML'
name: outer-local-action
description: nested local action setup-go fixture
runs:
  using: composite
  steps:
    - uses: ./nested-action
YAML
cat >"$nested_local/nested-action/action.yaml" <<'YAML'
name: nested-local-action
description: leaf local action setup-go fixture
runs:
  using: composite
  steps:
    - uses: actions/setup-go@example
      with:
        go-version: '1.26.5'
YAML
run_verify "$nested_local"

reusable_local="$tmpdir/reusable-local"
make_fixture "$reusable_local"
cat >"$reusable_local/.github/workflows/ci.yml" <<'YAML'
jobs:
  call:
    uses: ./.github/workflows/reusable.yml
YAML
cat >"$reusable_local/.github/workflows/reusable.yml" <<'YAML'
name: reusable
on:
  workflow_call:
jobs:
  nested:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@example
        with:
          go-version: '1.26.5'
YAML
run_verify "$reusable_local"

bad_malformed_workflow="$tmpdir/bad-malformed-workflow"
make_fixture "$bad_malformed_workflow"
cat >"$bad_malformed_workflow/.github/workflows/ci.yml" <<'YAML'
jobs:
  test:
    steps: [
      - uses: actions/setup-go@example
        with:
          go-version: '1.26.5'
YAML
if run_verify "$bad_malformed_workflow" >"$tmpdir/bad-malformed-workflow.out" 2>"$tmpdir/bad-malformed-workflow.err"; then
	echo "verify-go-toolchain accepted malformed workflow YAML" >&2
	exit 1
fi
grep -q 'malformed YAML' "$tmpdir/bad-malformed-workflow.err"

bad_dynamic_workflow="$tmpdir/bad-dynamic-workflow"
make_fixture "$bad_dynamic_workflow"
cat >"$bad_dynamic_workflow/.github/workflows/ci.yml" <<'YAML'
jobs:
  test:
    steps: ${{ matrix.steps }}
YAML
if run_verify "$bad_dynamic_workflow" >"$tmpdir/bad-dynamic-workflow.out" 2>"$tmpdir/bad-dynamic-workflow.err"; then
	echo "verify-go-toolchain accepted a dynamic workflow steps structure" >&2
	exit 1
fi
grep -q 'unsupported or dynamic steps structure' "$tmpdir/bad-dynamic-workflow.err"

overrides="$tmpdir/overrides"
make_fixture "$overrides"
ZSCALERCTL_REPO_ROOT="$overrides" \
ZSCALERCTL_ROOT_GO_MOD="$overrides/go.mod" \
ZSCALERCTL_WORKFLOWS_DIR="$overrides/.github/workflows" \
	"$verifier"

bad_minimum="$tmpdir/bad-minimum"
make_fixture "$bad_minimum"
perl -0pi -e 's/go 1\.26\.5/go 1.26/' "$bad_minimum/go.mod"
if run_verify "$bad_minimum" >"$tmpdir/minimum.out" 2>"$tmpdir/minimum.err"; then
	echo "verify-go-toolchain accepted a root go directive without a patch minimum" >&2
	exit 1
fi
grep -q 'must pin a security patch minimum' "$tmpdir/minimum.err"

bad_synchronized="$tmpdir/bad-synchronized"
make_fixture "$bad_synchronized"
find "$bad_synchronized" -name go.mod -exec perl -0pi -e 's/go 1\.26\.5/go 1.26.4/' {} +
perl -0pi -e 's/1\.26\.5/1.26.4/' "$bad_synchronized/.github/workflows/ci.yml"
if run_verify "$bad_synchronized" >"$tmpdir/synchronized.out" 2>"$tmpdir/synchronized.err"; then
	echo "verify-go-toolchain accepted a synchronized downgrade below policy" >&2
	exit 1
fi
grep -q 'does not match the policy security minimum' "$tmpdir/synchronized.err"

bad_nested="$tmpdir/bad-nested"
make_fixture "$bad_nested"
perl -0pi -e 's/go 1\.26\.5/go 1.26.4/' "$bad_nested/tools/go.mod"
if run_verify "$bad_nested" >"$tmpdir/nested.out" 2>"$tmpdir/nested.err"; then
	echo "verify-go-toolchain accepted a stale nested module" >&2
	exit 1
fi
grep -q 'does not match root security minimum' "$tmpdir/nested.err"

bad_other_module="$tmpdir/bad-other-module"
make_fixture "$bad_other_module"
mkdir -p "$bad_other_module/other/module"
cat >"$bad_other_module/other/module/go.mod" <<'GOMOD'
module example.invalid/other

go 1.26.4
GOMOD
if run_verify "$bad_other_module" >"$tmpdir/other.out" 2>"$tmpdir/other.err"; then
	echo "verify-go-toolchain ignored a nested module outside known directories" >&2
	exit 1
fi
grep -q 'does not match root security minimum' "$tmpdir/other.err"

bad_workflow="$tmpdir/bad-workflow"
make_fixture "$bad_workflow"
perl -0pi -e "s/1\.26\.5/1.26.4/" "$bad_workflow/.github/workflows/ci.yml"
if run_verify "$bad_workflow" >"$tmpdir/workflow.out" 2>"$tmpdir/workflow.err"; then
	echo "verify-go-toolchain accepted a stale workflow Go version" >&2
	exit 1
fi
grep -q 'does not match root security minimum' "$tmpdir/workflow.err"

missing_workflow_pin="$tmpdir/missing-workflow-pin"
make_fixture "$missing_workflow_pin"
perl -0pi -e "s/\n[[:space:]]+go-version: '1\.26\.5'//" "$missing_workflow_pin/.github/workflows/ci.yml"
if run_verify "$missing_workflow_pin" >"$tmpdir/missing-pin.out" 2>"$tmpdir/missing-pin.err"; then
	echo "verify-go-toolchain accepted setup-go without go-version" >&2
	exit 1
fi
grep -q 'setup-go step is missing with.go-version' "$tmpdir/missing-pin.err"

comment_spoof="$tmpdir/comment-spoof"
make_fixture "$comment_spoof"
perl -0pi -e "s/[[:space:]]+go-version: '1\.26\.5'/          # go-version: '1.26.5'/" "$comment_spoof/.github/workflows/ci.yml"
if run_verify "$comment_spoof" >"$tmpdir/comment.out" 2>"$tmpdir/comment.err"; then
	echo "verify-go-toolchain accepted a commented-out go-version pin" >&2
	exit 1
fi
grep -q 'setup-go step is missing with.go-version' "$tmpdir/comment.err"

decoy_pin="$tmpdir/decoy-pin"
make_fixture "$decoy_pin"
cat >>"$decoy_pin/.github/workflows/ci.yml" <<'YAML'
      - uses: actions/setup-go@example
        env:
          go-version: '1.26.5'
YAML
if run_verify "$decoy_pin" >"$tmpdir/decoy.out" 2>"$tmpdir/decoy.err"; then
	echo "verify-go-toolchain allowed an unrelated go-version key to mask an unpinned setup-go step" >&2
	exit 1
fi
grep -q 'setup-go step is missing with.go-version' "$tmpdir/decoy.err"

block_scalar="$tmpdir/block-scalar"
make_fixture "$block_scalar"
perl -0pi -e "s/go-version: '1\.26\.5'/script: |\n            go-version: '1.26.5'/" "$block_scalar/.github/workflows/ci.yml"
if run_verify "$block_scalar" >"$tmpdir/block.out" 2>"$tmpdir/block.err"; then
	echo "verify-go-toolchain accepted a block-scalar go-version decoy" >&2
	exit 1
fi
grep -q 'setup-go step is missing with.go-version' "$tmpdir/block.err"
