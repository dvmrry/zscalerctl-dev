#!/usr/bin/env bash
set -euo pipefail
umask 077

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

bash scripts/verify-gitleaks-history-policy.sh

version="${GITLEAKS_VERSION:-v8.30.1}"
if [[ "$version" != v* ]]; then
	version="v$version"
fi

if ! command -v openssl >/dev/null 2>&1; then
	echo "test-gitleaks-allowlist requires openssl" >&2
	exit 1
fi

tmpdir="$(mktemp -d)"
cleanup() {
	rm -rf "$tmpdir"
}
trap cleanup EXIT

bin_dir="$tmpdir/bin"
mkdir -p "$bin_dir"
GOBIN="$bin_dir" GOFLAGS=-mod=mod go install "github.com/zricethezav/gitleaks/v8@$version"

run_gitleaks() {
	"$bin_dir/gitleaks" "$@"
}

expect_private_key_finding() {
	local label="$1"
	local scan_dir="$2"
	local report="$tmpdir/$label.json"
	local log="$tmpdir/$label.log"

	set +e
	run_gitleaks dir \
		--no-banner \
		--no-color \
		--enable-rule=private-key \
		--redact=100 \
		--exit-code=23 \
		--report-format=json \
		--report-path="$report" \
		--config="$repo_root/.gitleaks.toml" \
		"$scan_dir" >"$log" 2>&1
	local status=$?
	set -e

	if [[ $status -ne 23 ]]; then
		echo "$label: gitleaks exit = $status, want 23 for a detected private key" >&2
		cat "$log" >&2
		exit 1
	fi
	if ! grep -Eq '"RuleID"[[:space:]]*:[[:space:]]*"private-key"' "$report"; then
		echo "$label: gitleaks report did not contain a private-key finding" >&2
		cat "$log" >&2
		exit 1
	fi
}

key="$tmpdir/generated.pem"
openssl genpkey \
	-algorithm RSA \
	-pkeyopt rsa_keygen_bits:2048 \
	-out "$key" >/dev/null 2>&1

real_dir="$tmpdir/real"
mkdir -p "$real_dir"
cp "$key" "$real_dir/real_test.go"
expect_private_key_finding "real-key-control" "$real_dir"

# Construct the markers from fragments so this verifier does not itself embed
# a private-key detector fixture in the repository. The unclosed fake prefix
# reproduces the historical bypass: a broad allowlist regex could match the
# fake marker inside the one detector match that also contained the real key.
begin_marker='-----BEGIN PRIVATE'
begin_marker+=' KEY-----'
fake_marker='fake-key-'
fake_marker+='material-canary-not-a-real-key'

composite_dir="$tmpdir/composite"
mkdir -p "$composite_dir"
{
	printf '%s\n' "$begin_marker" "$fake_marker"
	cat "$key"
} >"$composite_dir/composite_test.go"
expect_private_key_finding "composite-bypass" "$composite_dir"

history_report="$tmpdir/history.json"
history_log="$tmpdir/history.log"
set +e
run_gitleaks git \
	--no-banner \
	--no-color \
	--enable-rule=private-key \
	--redact=100 \
	--exit-code=23 \
	--report-format=json \
	--report-path="$history_report" \
	--config="$repo_root/.gitleaks.toml" \
	"$repo_root" >"$history_log" 2>&1
history_status=$?
set -e

if [[ $history_status -ne 0 ]]; then
	echo "history-control: gitleaks exit = $history_status, want 0" >&2
	cat "$history_log" >&2
	exit 1
fi
