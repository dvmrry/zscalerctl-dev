#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_verifier="$repo_root/scripts/verify-active-node-toolchain.sh"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

fixture="$tmpdir/repo"
mkdir -p "$fixture/scripts" "$tmpdir/bin"
cp "$source_verifier" "$fixture/scripts/verify-active-node-toolchain.sh"
chmod +x "$fixture/scripts/verify-active-node-toolchain.sh"
printf '24.15.0\n' >"$fixture/.node-version"

write_fake_node() {
	local version="$1"
	cat >"$tmpdir/bin/node" <<EOF
#!/bin/sh
printf '%s\\n' '$version'
EOF
	chmod +x "$tmpdir/bin/node"
}

write_fake_node 'v24.15.0'
PATH="$tmpdir/bin:$PATH" "$fixture/scripts/verify-active-node-toolchain.sh"

write_fake_node 'v24.12.0'
if PATH="$tmpdir/bin:$PATH" "$fixture/scripts/verify-active-node-toolchain.sh" >"$tmpdir/runtime.out" 2>"$tmpdir/runtime.err"; then
	echo "verify-active-node-toolchain accepted a downgraded active runtime" >&2
	exit 1
fi
grep -q 'active Node version 24.12.0 does not match the reviewed runtime pin 24.15.0' "$tmpdir/runtime.err"

write_fake_node 'v24.15.0'
printf '24.12.0\n' >"$fixture/.node-version"
if PATH="$tmpdir/bin:$PATH" "$fixture/scripts/verify-active-node-toolchain.sh" >"$tmpdir/file.out" 2>"$tmpdir/file.err"; then
	echo "verify-active-node-toolchain accepted a downgraded shared version file" >&2
	exit 1
fi
grep -q 'does not match the reviewed runtime pin 24.15.0' "$tmpdir/file.err"

printf '24.15.0\n' >"$fixture/.node-version"
write_fake_node 'v24.15.0-extra'
if PATH="$tmpdir/bin:$PATH" "$fixture/scripts/verify-active-node-toolchain.sh" >"$tmpdir/malformed.out" 2>"$tmpdir/malformed.err"; then
	echo "verify-active-node-toolchain accepted a non-exact active runtime version" >&2
	exit 1
fi
grep -q 'does not match the reviewed runtime pin 24.15.0' "$tmpdir/malformed.err"
