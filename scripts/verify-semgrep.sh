#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RULES="${ROOT}/semgrep/rules/zscalerctl.yml"
SEMGREP_VERSION="${SEMGREP_VERSION:-1.164.0}"

semgrep_cmd() {
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
trap 'rm -f "$semgrep_bad_out"' EXIT

semgrep_cmd scan --quiet --error --config "${RULES}" "${ROOT}/internal"
semgrep_cmd scan --quiet --error --config "${RULES}" "${ROOT}/semgrep/tests/reveal_ok.go"

semgrep_rc=0
semgrep_cmd scan --quiet --error --config "${RULES}" "${ROOT}/semgrep/tests/reveal_bad.go" >"$semgrep_bad_out" 2>&1 || semgrep_rc=$?
if (( semgrep_rc == 0 )); then
  cat "$semgrep_bad_out" >&2
  echo "expected Semgrep Reveal() fixture to fail, but it passed" >&2
  exit 1
elif (( semgrep_rc >= 2 )); then
  cat "$semgrep_bad_out" >&2
  echo "semgrep error (exit $semgrep_rc) running Reveal() fixture" >&2
  exit 1
fi

semgrep_cmd scan --quiet --error --config "${RULES}" "${ROOT}/semgrep/tests/projection_ok.go"

semgrep_rc=0
semgrep_cmd scan --quiet --error --config "${RULES}" "${ROOT}/semgrep/tests/projection_bad.go" >"$semgrep_bad_out" 2>&1 || semgrep_rc=$?
if (( semgrep_rc == 0 )); then
  cat "$semgrep_bad_out" >&2
  echo "expected Semgrep raw projection fixture to fail, but it passed" >&2
  exit 1
elif (( semgrep_rc >= 2 )); then
  cat "$semgrep_bad_out" >&2
  echo "semgrep error (exit $semgrep_rc) running raw projection fixture" >&2
  exit 1
fi
