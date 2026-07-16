#!/usr/bin/env bash
set -euo pipefail

repo_root="${ZSCALERCTL_REPO_ROOT:-}"
if [[ -z "$repo_root" ]]; then
	repo_root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
fi

if [[ -z "$repo_root" ]]; then
	exit 0
fi

cd "$repo_root"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	exit 0
fi

if bash scripts/verify-adversarial-review.sh; then
	exit 0
fi

cat >&2 <<'EOF'
Adversarial review is required before this implementation can be completed.

This is a Codex Stop hook, not a user prompt. Do not present the work as done
until the workflow is complete:

1. Generate a builder handoff from docs/review-handoff-template.md.
2. Spawn a fresh-context reviewer using docs/adversarial-review-run-prompt.md.
3. Do not implement fixes in the reviewer context.
4. If the reviewer requests changes, fix them and request a narrow recheck.
5. Add an approved artifact under docs/adversarial-reviews/.
6. Rerun make verify-adversarial-review.

If no fresh-context reviewer is available, stop and report that the local hook
blocked completion because adversarial review could not be performed.
EOF

exit 1
