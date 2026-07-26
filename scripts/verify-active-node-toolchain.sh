#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
node_version_file="$repo_root/.node-version"
required_version="24.15.0"

if [[ ! -f "$node_version_file" ]]; then
	echo "$node_version_file: shared Node version file not found" >&2
	exit 1
fi

file_version="$(cat "$node_version_file")"
if [[ "$file_version" != "$required_version" ]]; then
	echo "$node_version_file: Node version $file_version does not match the reviewed runtime pin $required_version" >&2
	exit 1
fi

if ! active_version="$(node --version 2>&1)"; then
	echo "node --version failed: $active_version" >&2
	exit 1
fi
active_version="${active_version#v}"
if [[ "$active_version" != "$required_version" ]]; then
	echo "active Node version $active_version does not match the reviewed runtime pin $required_version" >&2
	exit 1
fi
