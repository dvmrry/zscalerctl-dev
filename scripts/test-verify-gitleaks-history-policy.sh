#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
verifier="$repo_root/scripts/verify-gitleaks-history-policy.sh"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

fixture="$tmpdir/fixture"
git init -q "$fixture"
git -C "$fixture" config user.name "zscalerctl test"
git -C "$fixture" config user.email "test@example.invalid"
printf 'one\n' >"$fixture/fixture.txt"
git -C "$fixture" add fixture.txt
git -C "$fixture" commit -qm "fixture one"
first_commit="$(git -C "$fixture" rev-parse HEAD)"
printf 'two\n' >>"$fixture/fixture.txt"
git -C "$fixture" commit -qam "fixture two"

printf '%s\n' "$first_commit:fixture.txt:private-key:1" >"$fixture/.gitleaksignore"
ZSCALERCTL_REPO_ROOT="$fixture" "$verifier"

printf '%s\n' 'fixture.txt:private-key:1' >"$fixture/.gitleaksignore"
if ZSCALERCTL_REPO_ROOT="$fixture" "$verifier" >"$tmpdir/global.out" 2>"$tmpdir/global.err"; then
	echo "verify-gitleaks-history-policy accepted a non-commit-bound fingerprint" >&2
	exit 1
fi
grep -q 'exact commit:path:rule:line fingerprint' "$tmpdir/global.err"

printf '%s\n' '0000000000000000000000000000000000000000:fixture.txt:private-key:1' >"$fixture/.gitleaksignore"
if ZSCALERCTL_REPO_ROOT="$fixture" "$verifier" >"$tmpdir/missing.out" 2>"$tmpdir/missing.err"; then
	echo "verify-gitleaks-history-policy accepted a fingerprint for a missing commit" >&2
	exit 1
fi
grep -q 'referenced commit is missing' "$tmpdir/missing.err"

shallow="$tmpdir/shallow"
git clone -q --depth 1 "file://$fixture" "$shallow"
printf '%s\n' "$(git -C "$shallow" rev-parse HEAD):fixture.txt:private-key:2" >"$shallow/.gitleaksignore"
if ZSCALERCTL_REPO_ROOT="$shallow" "$verifier" >"$tmpdir/shallow.out" 2>"$tmpdir/shallow.err"; then
	echo "verify-gitleaks-history-policy accepted a shallow checkout" >&2
	exit 1
fi
grep -q 'requires a full Git checkout' "$tmpdir/shallow.err"

