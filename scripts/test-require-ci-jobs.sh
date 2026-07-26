#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
checker="$repo_root/scripts/require-ci-jobs.sh"

"$checker" "success success success"

for results in "" "success failure" "cancelled" "success skipped"; do
	if "$checker" "$results" >/dev/null 2>&1; then
		echo "require-ci-jobs accepted non-success results: ${results:-<empty>}" >&2
		exit 1
	fi
done

if "$checker" success success >/dev/null 2>&1; then
	echo "require-ci-jobs accepted multiple result arguments" >&2
	exit 1
fi
