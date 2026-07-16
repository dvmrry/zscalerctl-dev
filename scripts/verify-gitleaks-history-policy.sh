#!/usr/bin/env bash
set -euo pipefail

repo_root="${ZSCALERCTL_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
cd "$repo_root"

ignore_file="${GITLEAKS_IGNORE_FILE:-.gitleaksignore}"

if [[ ! -f "$ignore_file" ]]; then
	echo "$ignore_file: Gitleaks history ignore file not found" >&2
	exit 1
fi

if [[ "$(git rev-parse --is-shallow-repository)" == "true" ]]; then
	echo "Gitleaks history policy requires a full Git checkout; fetch with --unshallow before scanning" >&2
	exit 1
fi

line_no=0
while IFS= read -r line || [[ -n "$line" ]]; do
	line_no=$((line_no + 1))
	if [[ "$line" =~ ^[[:space:]]*$ || "$line" =~ ^[[:space:]]*# ]]; then
		continue
	fi

	if [[ ! "$line" =~ ^([0-9a-f]{40}):([^:]+):([^:]+):([1-9][0-9]*)$ ]]; then
		echo "$ignore_file:$line_no: ignore must be an exact commit:path:rule:line fingerprint" >&2
		exit 1
	fi

	commit="${BASH_REMATCH[1]}"
	if ! git cat-file -e "${commit}^{commit}" 2>/dev/null; then
		echo "$ignore_file:$line_no: referenced commit is missing from local history: $commit" >&2
		exit 1
	fi
done <"$ignore_file"

