#!/usr/bin/env bash
set -euo pipefail

repo_root="${ZSCALERCTL_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
cd "$repo_root"

surface_dir="cmd/zscalerctl/testdata/surface"
manifest="$surface_dir/surface_changes.md"
base_ref="${ZSCALERCTL_BASE_REF:-origin/main}"

if [[ ! -d "$surface_dir" ]]; then
	echo "surface directory not found: $surface_dir" >&2
	exit 1
fi

if ! git rev-parse --quiet --verify "$base_ref" >/dev/null; then
	if [[ "$base_ref" == "origin/main" ]]; then
		echo "base ref $base_ref not found; fetching origin/main" >&2
		git fetch origin main
	else
		echo "base ref $base_ref not found" >&2
		exit 1
	fi
fi

# Two-dot diff so the gate also catches uncommitted changes locally and in
# pre-commit hooks, while still working on a clean CI checkout.
changed_files="$(git diff --name-only "$base_ref" --)"

if ! grep -qE "^$surface_dir/.*\.golden$" <<<"$changed_files"; then
	exit 0
fi

if grep -qE "^$manifest$" <<<"$changed_files"; then
	exit 0
fi

cat >&2 <<EOF
error: surface golden files changed but $manifest was not updated.

A human-readable explanation is required whenever the frozen CLI surface
changes. Update $manifest to document the intentional change, then commit
both the golden files and the manifest together.
EOF

exit 1
