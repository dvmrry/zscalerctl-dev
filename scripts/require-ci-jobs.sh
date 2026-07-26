#!/usr/bin/env bash
set -euo pipefail

if (( $# != 1 )); then
	echo "usage: $0 'RESULT ...'" >&2
	exit 2
fi

read -r -a results <<<"$1"
if (( ${#results[@]} == 0 )); then
	echo "no required CI job results were provided" >&2
	exit 1
fi

echo "job results: $1"

for result in "${results[@]}"; do
	if [[ "$result" != "success" ]]; then
		echo "required CI job did not succeed: $result" >&2
		exit 1
	fi
done
