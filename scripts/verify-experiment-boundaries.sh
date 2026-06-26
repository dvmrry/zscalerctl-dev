#!/usr/bin/env bash
set -euo pipefail

repo_root="${ZSCALERCTL_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
cd "$repo_root"

fail=0

report_matches() {
  local message="$1"
  local matches="$2"

  if [[ -z "$matches" ]]; then
    return
  fi

  echo "$message" >&2
  sed 's/^/  /' <<<"$matches" >&2
  fail=1
}

check_file_forbidden() {
  local file="$1"
  local pattern="$2"
  local message="$3"

  if [[ ! -f "$file" ]]; then
    return
  fi

  local matches
  matches="$(grep -nE "$pattern" "$file" || true)"
  report_matches "$message" "$matches"
}

if [[ -f go.work ]]; then
  echo "verify-experiment-boundaries: root go.work is not allowed; experiments must stay outside the default root workspace." >&2
  fail=1
fi

experiment_dependency_re='github\.com/charmbracelet/(bubbletea|bubbles|fang)(/|$)|github\.com/charmbracelet/lipgloss/v2(/|$)|github\.com/wailsapp/wails(/|$)|(^|/)internal/tui(/|$)|(^|/)experiments(/|$)'
root_manifest_re='github\.com/charmbracelet/(bubbletea|bubbles|fang)|github\.com/charmbracelet/lipgloss/v2|github\.com/wailsapp/wails|(^|[[:space:]])replace[[:space:]].*experiments/|experiments/'
web_manifest_re='(@vitejs/|(^|[^[:alnum:]_-])vite([^[:alnum:]_-]|$)|(^|[^[:alnum:]_-])react([^[:alnum:]_-]|$)|wails)'
default_gate_re='experiments/|check-experiment-'

check_file_forbidden \
  "go.mod" \
  "$root_manifest_re" \
  "verify-experiment-boundaries: root go.mod references experiment-only dependencies or paths:"

check_file_forbidden \
  "go.sum" \
  "$root_manifest_re" \
  "verify-experiment-boundaries: root go.sum references experiment-only dependencies or paths:"

check_file_forbidden \
  "vendor/modules.txt" \
  "$root_manifest_re" \
  "verify-experiment-boundaries: root vendor/modules.txt references experiment-only dependencies or paths:"

for web_manifest in package.json package-lock.json pnpm-lock.yaml yarn.lock; do
  check_file_forbidden \
    "$web_manifest" \
    "$web_manifest_re" \
    "verify-experiment-boundaries: root $web_manifest references experiment-only frontend dependencies:"
done

check_file_forbidden \
  "Makefile" \
  "$default_gate_re" \
  "verify-experiment-boundaries: default Makefile surface references experiment paths or experiment checks:"

check_file_forbidden \
  ".github/workflows/ci.yml" \
  "$default_gate_re" \
  "verify-experiment-boundaries: default CI workflow references experiment paths or experiment checks:"

deps_file="$(mktemp)"
trap 'rm -f "$deps_file"' EXIT

if [[ -n "${ZSCALERCTL_ROOT_DEPS_FILE:-}" ]]; then
  cat "$ZSCALERCTL_ROOT_DEPS_FILE" >"$deps_file"
else
  go list -deps -mod=vendor ./... >"$deps_file"
fi

deps_matches="$(grep -nE "$experiment_dependency_re" "$deps_file" || true)"
report_matches \
  "verify-experiment-boundaries: root module imports experiment-only dependencies or packages:" \
  "$deps_matches"

experiments_dir="${ZSCALERCTL_EXPERIMENTS_DIR:-experiments}"
if [[ -d "$experiments_dir" ]]; then
  experiments_abs="$(cd "$experiments_dir" && pwd)"
  while IFS= read -r go_file; do
    dir="$(cd "$(dirname "$go_file")" && pwd)"
    found_module=0
    while [[ "$dir" == "$experiments_abs"* ]]; do
      if [[ -f "$dir/go.mod" ]]; then
        found_module=1
        break
      fi
      if [[ "$dir" == "$experiments_abs" ]]; then
        break
      fi
      dir="$(dirname "$dir")"
    done
    if (( found_module == 0 )); then
      echo "verify-experiment-boundaries: experiment Go file is not inside a nested module: $go_file" >&2
      fail=1
    fi
  done < <(find "$experiments_dir" -type f -name '*.go' ! -path '*/vendor/*' | LC_ALL=C sort)
fi

if (( fail != 0 )); then
  exit 1
fi

echo "verify-experiment-boundaries: PASS"
