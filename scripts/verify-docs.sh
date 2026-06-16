#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

paths=(README.md AGENTS.md docs examples skills)
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

# Posture artifacts must stay well-formed: the OpenVEX document is consumed
# by tooling and cited by policy, so a syntax error is a doc failure.
if ! python3 - <<'PYEOF'
import json, sys
doc = json.load(open(".openvex.json"))
ok = doc.get("@context", "").startswith("https://openvex.dev/") and isinstance(doc.get("statements"), list)
allowed = {"not_affected", "affected", "fixed", "under_investigation"}
for st in doc["statements"]:
    ok = ok and st.get("status") in allowed and st.get("vulnerability", {}).get("name")
    if st.get("status") == "not_affected":
        ok = ok and bool(st.get("justification"))
sys.exit(0 if ok else 1)
PYEOF
then
  echo "verify-docs: .openvex.json is missing, malformed, or has invalid statements" >&2
  fail=1
fi

sdk_version="$(go list -m -mod=mod -f '{{.Version}}' github.com/zscaler/zscaler-sdk-go/v3)"
if ! grep -Fq "github.com/zscaler/zscaler-sdk-go/v3 ${sdk_version}" docs/THREAT_MODEL.md; then
  echo "verify-docs: docs/THREAT_MODEL.md review stamp must cite github.com/zscaler/zscaler-sdk-go/v3 ${sdk_version}" >&2
  fail=1
fi

if (( fail != 0 )); then
  exit 1
fi
