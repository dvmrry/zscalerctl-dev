#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

make_fixture() {
  local dir="$1"
  mkdir -p "$dir/.github/workflows" "$dir/vendor"
  cat >"$dir/go.mod" <<'EOF'
module example.com/fixture

go 1.26
EOF
  : >"$dir/go.sum"
  : >"$dir/vendor/modules.txt"
  cat >"$dir/Makefile" <<'EOF'
check: verify-experiment-boundaries
verify-experiment-boundaries:
	bash scripts/verify-experiment-boundaries.sh
EOF
  cat >"$dir/.github/workflows/ci.yml" <<'EOF'
name: ci
jobs:
  verify-gates:
    steps:
      - run: make verify-experiment-boundaries
EOF
}

write_deps() {
  local path="$1"
  cat >"$path" <<'EOF'
example.com/fixture/cmd/tool
example.com/fixture/internal/core
github.com/charmbracelet/lipgloss
EOF
}

run_verify() {
  local fixture="$1"
  local deps="$2"
  ZSCALERCTL_REPO_ROOT="$fixture" \
  ZSCALERCTL_ROOT_DEPS_FILE="$deps" \
    "$repo_root/scripts/verify-experiment-boundaries.sh"
}

good="$tmp_dir/good"
make_fixture "$good"
write_deps "$tmp_dir/good.deps"
run_verify "$good" "$tmp_dir/good.deps" >/dev/null

nested="$tmp_dir/nested"
make_fixture "$nested"
mkdir -p "$nested/experiments/tui"
cat >"$nested/experiments/tui/go.mod" <<'EOF'
module example.com/fixture/experiments/tui

go 1.26
EOF
cat >"$nested/experiments/tui/main.go" <<'EOF'
package main

func main() {}
EOF
run_verify "$nested" "$tmp_dir/good.deps" >/dev/null

go_work="$tmp_dir/go-work"
make_fixture "$go_work"
: >"$go_work/go.work"
if run_verify "$go_work" "$tmp_dir/good.deps" >"$tmp_dir/go-work.out" 2>"$tmp_dir/go-work.err"; then
  echo "verify-experiment-boundaries accepted a root go.work" >&2
  exit 1
fi
grep -q "root go.work is not allowed" "$tmp_dir/go-work.err"

go_mod_bad="$tmp_dir/go-mod-bad"
make_fixture "$go_mod_bad"
cat >>"$go_mod_bad/go.mod" <<'EOF'

require github.com/charmbracelet/fang v0.1.0
EOF
if run_verify "$go_mod_bad" "$tmp_dir/good.deps" >"$tmp_dir/go-mod.out" 2>"$tmp_dir/go-mod.err"; then
  echo "verify-experiment-boundaries accepted Fang in root go.mod" >&2
  exit 1
fi
grep -q "root go.mod references experiment-only dependencies" "$tmp_dir/go-mod.err"

vendor_bad="$tmp_dir/vendor-bad"
make_fixture "$vendor_bad"
cat >"$vendor_bad/vendor/modules.txt" <<'EOF'
# github.com/charmbracelet/bubbletea v1.0.0
github.com/charmbracelet/bubbletea
EOF
if run_verify "$vendor_bad" "$tmp_dir/good.deps" >"$tmp_dir/vendor.out" 2>"$tmp_dir/vendor.err"; then
  echo "verify-experiment-boundaries accepted Bubble Tea in vendor/modules.txt" >&2
  exit 1
fi
grep -q "root vendor/modules.txt references experiment-only dependencies" "$tmp_dir/vendor.err"

make_bad="$tmp_dir/make-bad"
make_fixture "$make_bad"
cat >>"$make_bad/Makefile" <<'EOF'
check: check-experiment-tui
check-experiment-tui:
	go test ./experiments/tui/...
EOF
if run_verify "$make_bad" "$tmp_dir/good.deps" >"$tmp_dir/make.out" 2>"$tmp_dir/make.err"; then
  echo "verify-experiment-boundaries accepted default Makefile experiment checks" >&2
  exit 1
fi
grep -q "default Makefile surface references experiment paths" "$tmp_dir/make.err"

deps_bad="$tmp_dir/deps-bad"
make_fixture "$deps_bad"
cat >"$tmp_dir/bad.deps" <<'EOF'
example.com/fixture/cmd/tool
github.com/wailsapp/wails/v2/pkg/runtime
EOF
if run_verify "$deps_bad" "$tmp_dir/bad.deps" >"$tmp_dir/deps.out" 2>"$tmp_dir/deps.err"; then
  echo "verify-experiment-boundaries accepted a root dependency on Wails" >&2
  exit 1
fi
grep -q "root module imports experiment-only dependencies" "$tmp_dir/deps.err"

experiment_bad="$tmp_dir/experiment-bad"
make_fixture "$experiment_bad"
mkdir -p "$experiment_bad/experiments/tui"
cat >"$experiment_bad/experiments/tui/main.go" <<'EOF'
package main

func main() {}
EOF
if run_verify "$experiment_bad" "$tmp_dir/good.deps" >"$tmp_dir/experiment.out" 2>"$tmp_dir/experiment.err"; then
  echo "verify-experiment-boundaries accepted experiment Go files outside nested modules" >&2
  exit 1
fi
grep -q "experiment Go file is not inside a nested module" "$tmp_dir/experiment.err"
