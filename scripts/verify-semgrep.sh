#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RULES="${ROOT}/semgrep/rules/zscalerctl.yml"
SEMGREP_VERSION="${SEMGREP_VERSION:-1.179.0}"

semgrep_cmd() {
  if [[ -n "${HOME:-}" && -x "$HOME/.local/bin/semgrep" ]]; then
    "$HOME/.local/bin/semgrep" "$@"
    return
  fi
  if command -v semgrep >/dev/null 2>&1; then
    semgrep "$@"
    return
  fi
  if command -v uvx >/dev/null 2>&1; then
    uvx --from "semgrep==${SEMGREP_VERSION}" semgrep "$@"
    return
  fi
  cat >&2 <<EOF
semgrep is required for this check.

Install it with one of:
  pipx install semgrep
  uv tool install semgrep

or run through uvx:
  uvx --from semgrep==${SEMGREP_VERSION} semgrep --version
EOF
  exit 127
}

semgrep_bad_out="$(mktemp)"
semgrep_bad_err="$(mktemp)"
trap 'rm -f "$semgrep_bad_out" "$semgrep_bad_err"' EXIT

semgrep_cmd scan --quiet --error --config "${RULES}" "${ROOT}/internal"
semgrep_cmd scan --quiet --error --config "${RULES}" "${ROOT}/semgrep/tests/reveal_ok.go"

# A bad fixture must fail on its own rule, not merely on some rule.
expect_rule() {
  local fixture="$1" rule="$2" rc=0
  semgrep_cmd scan --quiet --error --json --config "${RULES}" "${ROOT}/semgrep/tests/${fixture}" >"$semgrep_bad_out" 2>"$semgrep_bad_err" || rc=$?
  if (( rc == 1 )) && grep -Fq "${rule}\"" "$semgrep_bad_out"; then
    return
  fi
  cat "$semgrep_bad_out" "$semgrep_bad_err" >&2
  echo "expected ${fixture} to fail on ${rule}; semgrep exited ${rc}" >&2
  exit 1
}

expect_rule reveal_bad.go zscalerctl.secret-reveal-outside-sdk-boundary
semgrep_cmd scan --quiet --error --config "${RULES}" "${ROOT}/semgrep/tests/projection_ok.go"
expect_rule projection_bad.go zscalerctl.raw-projection-outside-verify
