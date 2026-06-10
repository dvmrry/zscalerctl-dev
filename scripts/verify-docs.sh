#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

paths=(README.md docs examples)
fail=0

check_pattern() {
  local label="$1"
  local pattern="$2"
  local out grep_rc=0
  out="$(mktemp)"

  git grep -n -E -i -e "$pattern" -- "${paths[@]}" >"$out" || grep_rc=$?
  if (( grep_rc == 0 )); then
    echo "docs/examples contain forbidden $label pattern:" >&2
    cat "$out" >&2
    fail=1
  elif (( grep_rc != 1 )); then
    rm -f "$out"
    echo "git grep error (exit $grep_rc) checking $label pattern" >&2
    exit 1
  fi
  rm -f "$out"
}

check_pattern "inline client secret env" 'ZSCALERCTL_CLIENT_SECRET='
check_pattern "private key block" '-----BEGIN [A-Z ]*PRIVATE KEY-----'
check_pattern "aws access key" 'AKIA[0-9A-Z]{16}'
check_pattern "bearer token" 'bearer[[:space:]]+[A-Za-z0-9._~+/=-]{12,}'
check_pattern "assigned api key or client secret" '(client_secret|api[_-]?key)[[:space:]]*[:=][[:space:]]*[^[:space:]<]'

if (( fail != 0 )); then
  exit 1
fi
