#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

cat >"$tmp_dir/cmd-good.deps" <<'EOF'
github.com/dvmrry/zscalerctl/cmd/zscalerctl
github.com/dvmrry/zscalerctl/internal/browser
github.com/dvmrry/zscalerctl/internal/cli
github.com/charmbracelet/lipgloss
github.com/spf13/cobra
EOF

cat >"$tmp_dir/cli-good.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/cli
github.com/dvmrry/zscalerctl/internal/config
github.com/dvmrry/zscalerctl/internal/output
github.com/dvmrry/zscalerctl/internal/runtime
EOF

cat >"$tmp_dir/browser-good.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/browser
github.com/dvmrry/zscalerctl/internal/resources
github.com/dvmrry/zscalerctl/internal/redact
EOF

cat >"$tmp_dir/resources-good.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/resources
github.com/dvmrry/zscalerctl/internal/redact
EOF

cat >"$tmp_dir/machine-good.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/machine
EOF

cat >"$tmp_dir/machineio-good.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/machineio
github.com/dvmrry/zscalerctl/internal/machine
EOF

cat >"$tmp_dir/enginewire-good.imports" <<'EOF'
bytes
encoding/json
io
unicode/utf8
EOF

cat >"$tmp_dir/enginewire-package-bad.imports" <<'EOF'
bytes
github.com/dvmrry/zscalerctl/internal/machine
EOF

cat >"$tmp_dir/enginewire-cgo-bad.imports" <<'EOF'
C
bytes
EOF

cat >"$tmp_dir/enginewire-adapter-good.imports" <<'EOF'
context
fmt
github.com/dvmrry/zscalerctl/internal/enginewire
github.com/dvmrry/zscalerctl/internal/machine
github.com/dvmrry/zscalerctl/internal/resources
github.com/dvmrry/zscalerctl/internal/diff
github.com/dvmrry/zscalerctl/internal/redact
EOF

cat >"$tmp_dir/enginewire-adapter-sdk-bad.imports" <<'EOF'
github.com/dvmrry/zscalerctl/internal/enginewire
github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia
EOF

cat >"$tmp_dir/enginewire-adapter-bridge-bad.imports" <<'EOF'
github.com/dvmrry/zscalerctl/internal/enginewire
github.com/dvmrry/zscalerctl/internal/unreviewedbridge
EOF

cat >"$tmp_dir/enginewire-adapter-third-party-bad.imports" <<'EOF'
github.com/dvmrry/zscalerctl/internal/enginewire
example.com/unreviewed/codec
EOF

cat >"$tmp_dir/enginewire-adapter-cgo-bad.imports" <<'EOF'
C
github.com/dvmrry/zscalerctl/internal/enginewire
EOF

cat >"$tmp_dir/enginehost-good.imports" <<'EOF'
context
github.com/dvmrry/zscalerctl/internal/enginewire
github.com/dvmrry/zscalerctl/internal/enginewire/adapter
github.com/dvmrry/zscalerctl/internal/machine
io
EOF

cat >"$tmp_dir/enginehost-runtime-bad.imports" <<'EOF'
github.com/dvmrry/zscalerctl/internal/enginewire
github.com/dvmrry/zscalerctl/internal/runtime
EOF

cat >"$tmp_dir/enginehost-cgo-bad.imports" <<'EOF'
C
github.com/dvmrry/zscalerctl/internal/enginewire
EOF

cat >"$tmp_dir/enginecmd-good.imports" <<'EOF'
context
flag
github.com/dvmrry/zscalerctl/internal/enginehost
github.com/dvmrry/zscalerctl/internal/machine
github.com/dvmrry/zscalerctl/internal/redact
github.com/dvmrry/zscalerctl/internal/runtime
github.com/dvmrry/zscalerctl/internal/version
os
EOF

cat >"$tmp_dir/enginecmd-cli-bad.imports" <<'EOF'
github.com/dvmrry/zscalerctl/internal/cli
github.com/dvmrry/zscalerctl/internal/enginehost
EOF

cat >"$tmp_dir/enginecmd-sdk-bad.imports" <<'EOF'
github.com/dvmrry/zscalerctl/internal/enginehost
github.com/zscaler/zscaler-sdk-go/v3/zscaler
EOF

cat >"$tmp_dir/enginecmd-cgo-bad.imports" <<'EOF'
C
github.com/dvmrry/zscalerctl/internal/enginehost
EOF

cat >"$tmp_dir/cmd-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/cmd/zscalerctl
github.com/charmbracelet/bubbletea
github.com/dvmrry/zscalerctl/internal/browser
EOF

cat >"$tmp_dir/cli-zscaler-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/cli
github.com/dvmrry/zscalerctl/internal/config
github.com/dvmrry/zscalerctl/internal/zscaler
EOF

cat >"$tmp_dir/browser-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/browser
github.com/dvmrry/zscalerctl/internal/cli
github.com/charmbracelet/lipgloss
github.com/spf13/cobra
EOF

cat >"$tmp_dir/browser-raw-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/browser
github.com/dvmrry/zscalerctl/internal/config
github.com/dvmrry/zscalerctl/internal/zscaler
EOF

cat >"$tmp_dir/resources-runtime-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/resources
github.com/dvmrry/zscalerctl/internal/runtime
EOF

cat >"$tmp_dir/machine-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/machine
github.com/dvmrry/zscalerctl/internal/output
github.com/charmbracelet/lipgloss
github.com/spf13/cobra
EOF

cat >"$tmp_dir/machine-raw-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/machine
github.com/dvmrry/zscalerctl/internal/credentials
github.com/dvmrry/zscalerctl/internal/secretref
github.com/dvmrry/zscalerctl/internal/zscaler
EOF

cat >"$tmp_dir/machineio-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/machineio
github.com/dvmrry/zscalerctl/internal/output
github.com/charmbracelet/lipgloss
github.com/spf13/cobra
EOF

cat >"$tmp_dir/machineio-raw-bad.deps" <<'EOF'
github.com/dvmrry/zscalerctl/internal/machineio
github.com/dvmrry/zscalerctl/internal/config
github.com/dvmrry/zscalerctl/internal/zscaler
EOF

ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
ZSCALERCTL_CLI_DEPS_FILE="$tmp_dir/cli-good.deps" \
ZSCALERCTL_RESOURCES_DEPS_FILE="$tmp_dir/resources-good.deps" \
ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-good.deps" \
ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-good.deps" \
ZSCALERCTL_MACHINEIO_DEPS_FILE="$tmp_dir/machineio-good.deps" \
ZSCALERCTL_ENGINEWIRE_IMPORTS_FILE="$tmp_dir/enginewire-good.imports" \
ZSCALERCTL_ENGINEWIRE_ADAPTER_IMPORTS_FILE="$tmp_dir/enginewire-adapter-good.imports" \
ZSCALERCTL_ENGINEHOST_IMPORTS_FILE="$tmp_dir/enginehost-good.imports" \
ZSCALERCTL_ENGINECMD_IMPORTS_FILE="$tmp_dir/enginecmd-good.imports" \
  "$repo_root/scripts/verify-core-boundaries.sh" >/dev/null

for fixture in package cgo; do
  if ZSCALERCTL_ENGINEWIRE_IMPORTS_FILE="$tmp_dir/enginewire-${fixture}-bad.imports" \
    ZSCALERCTL_ENGINEWIRE_ADAPTER_IMPORTS_FILE="$tmp_dir/enginewire-adapter-good.imports" \
    "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/enginewire-${fixture}.out" 2>"$tmp_dir/enginewire-${fixture}.err"; then
    echo "verify-core-boundaries accepted enginewire $fixture dependency outside its allowlist" >&2
    cat "$tmp_dir/enginewire-${fixture}.out" >&2
    cat "$tmp_dir/enginewire-${fixture}.err" >&2
    exit 1
  fi

  if ! grep -q "internal/enginewire imports dependencies outside its allowlist" "$tmp_dir/enginewire-${fixture}.err"; then
    echo "verify-core-boundaries failed without the expected enginewire allowlist message" >&2
    cat "$tmp_dir/enginewire-${fixture}.err" >&2
    exit 1
  fi
done

for fixture in sdk bridge third-party cgo; do
  if ZSCALERCTL_ENGINEWIRE_IMPORTS_FILE="$tmp_dir/enginewire-good.imports" \
    ZSCALERCTL_ENGINEWIRE_ADAPTER_IMPORTS_FILE="$tmp_dir/enginewire-adapter-${fixture}-bad.imports" \
    "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/enginewire-adapter-${fixture}.out" 2>"$tmp_dir/enginewire-adapter-${fixture}.err"; then
    echo "verify-core-boundaries accepted enginewire adapter $fixture dependency outside its allowlist" >&2
    cat "$tmp_dir/enginewire-adapter-${fixture}.out" >&2
    cat "$tmp_dir/enginewire-adapter-${fixture}.err" >&2
    exit 1
  fi

  if ! grep -q "internal/enginewire/adapter imports dependencies outside its allowlist" "$tmp_dir/enginewire-adapter-${fixture}.err"; then
    echo "verify-core-boundaries failed without the expected enginewire adapter allowlist message" >&2
    cat "$tmp_dir/enginewire-adapter-${fixture}.err" >&2
    exit 1
  fi
done

for fixture in runtime cgo; do
  if ZSCALERCTL_ENGINEWIRE_IMPORTS_FILE="$tmp_dir/enginewire-good.imports" \
    ZSCALERCTL_ENGINEWIRE_ADAPTER_IMPORTS_FILE="$tmp_dir/enginewire-adapter-good.imports" \
    ZSCALERCTL_ENGINEHOST_IMPORTS_FILE="$tmp_dir/enginehost-${fixture}-bad.imports" \
    ZSCALERCTL_ENGINECMD_IMPORTS_FILE="$tmp_dir/enginecmd-good.imports" \
    "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/enginehost-${fixture}.out" 2>"$tmp_dir/enginehost-${fixture}.err"; then
    echo "verify-core-boundaries accepted enginehost $fixture dependency outside its allowlist" >&2
    cat "$tmp_dir/enginehost-${fixture}.out" >&2
    cat "$tmp_dir/enginehost-${fixture}.err" >&2
    exit 1
  fi

  if ! grep -q "internal/enginehost imports dependencies outside its allowlist" "$tmp_dir/enginehost-${fixture}.err"; then
    echo "verify-core-boundaries failed without the expected enginehost allowlist message" >&2
    cat "$tmp_dir/enginehost-${fixture}.err" >&2
    exit 1
  fi
done

for fixture in cli sdk cgo; do
  if ZSCALERCTL_ENGINEWIRE_IMPORTS_FILE="$tmp_dir/enginewire-good.imports" \
    ZSCALERCTL_ENGINEWIRE_ADAPTER_IMPORTS_FILE="$tmp_dir/enginewire-adapter-good.imports" \
    ZSCALERCTL_ENGINEHOST_IMPORTS_FILE="$tmp_dir/enginehost-good.imports" \
    ZSCALERCTL_ENGINECMD_IMPORTS_FILE="$tmp_dir/enginecmd-${fixture}-bad.imports" \
    "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/enginecmd-${fixture}.out" 2>"$tmp_dir/enginecmd-${fixture}.err"; then
    echo "verify-core-boundaries accepted engine command $fixture dependency outside its allowlist" >&2
    cat "$tmp_dir/enginecmd-${fixture}.out" >&2
    cat "$tmp_dir/enginecmd-${fixture}.err" >&2
    exit 1
  fi

  if ! grep -q "cmd/zscalerctl-engine imports dependencies outside its allowlist" "$tmp_dir/enginecmd-${fixture}.err"; then
    echo "verify-core-boundaries failed without the expected engine command allowlist message" >&2
    cat "$tmp_dir/enginecmd-${fixture}.err" >&2
    exit 1
  fi
done

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-bad.deps" \
  ZSCALERCTL_CLI_DEPS_FILE="$tmp_dir/cli-good.deps" \
  ZSCALERCTL_RESOURCES_DEPS_FILE="$tmp_dir/resources-good.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-good.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-good.deps" \
  ZSCALERCTL_MACHINEIO_DEPS_FILE="$tmp_dir/machineio-good.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/cmd.out" 2>"$tmp_dir/cmd.err"; then
  echo "verify-core-boundaries accepted a CLI dependency on Bubble Tea" >&2
  cat "$tmp_dir/cmd.out" >&2
  cat "$tmp_dir/cmd.err" >&2
  exit 1
fi

if ! grep -q "cmd/zscalerctl imports forbidden dependencies" "$tmp_dir/cmd.err"; then
  echo "verify-core-boundaries failed without the expected CLI boundary message" >&2
  cat "$tmp_dir/cmd.err" >&2
  exit 1
fi

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
  ZSCALERCTL_CLI_DEPS_FILE="$tmp_dir/cli-zscaler-bad.deps" \
  ZSCALERCTL_RESOURCES_DEPS_FILE="$tmp_dir/resources-good.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-good.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-good.deps" \
  ZSCALERCTL_MACHINEIO_DEPS_FILE="$tmp_dir/machineio-good.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/cli-zscaler.out" 2>"$tmp_dir/cli-zscaler.err"; then
  echo "verify-core-boundaries accepted internal/cli dependencies on internal/zscaler" >&2
  cat "$tmp_dir/cli-zscaler.out" >&2
  cat "$tmp_dir/cli-zscaler.err" >&2
  exit 1
fi

if ! grep -q "internal/cli imports forbidden dependencies" "$tmp_dir/cli-zscaler.err"; then
  echo "verify-core-boundaries failed without the expected internal/cli zscaler boundary message" >&2
  cat "$tmp_dir/cli-zscaler.err" >&2
  exit 1
fi

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
  ZSCALERCTL_CLI_DEPS_FILE="$tmp_dir/cli-good.deps" \
  ZSCALERCTL_RESOURCES_DEPS_FILE="$tmp_dir/resources-runtime-bad.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-good.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-good.deps" \
  ZSCALERCTL_MACHINEIO_DEPS_FILE="$tmp_dir/machineio-good.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/resources-runtime.out" 2>"$tmp_dir/resources-runtime.err"; then
  echo "verify-core-boundaries accepted resources dependencies on the runtime facade" >&2
  cat "$tmp_dir/resources-runtime.out" >&2
  cat "$tmp_dir/resources-runtime.err" >&2
  exit 1
fi

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
  ZSCALERCTL_CLI_DEPS_FILE="$tmp_dir/cli-good.deps" \
  ZSCALERCTL_RESOURCES_DEPS_FILE="$tmp_dir/resources-good.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-bad.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-good.deps" \
  ZSCALERCTL_MACHINEIO_DEPS_FILE="$tmp_dir/machineio-good.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/browser.out" 2>"$tmp_dir/browser.err"; then
  echo "verify-core-boundaries accepted browser dependencies on CLI/UI packages" >&2
  cat "$tmp_dir/browser.out" >&2
  cat "$tmp_dir/browser.err" >&2
  exit 1
fi

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
  ZSCALERCTL_CLI_DEPS_FILE="$tmp_dir/cli-good.deps" \
  ZSCALERCTL_RESOURCES_DEPS_FILE="$tmp_dir/resources-good.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-raw-bad.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-good.deps" \
  ZSCALERCTL_MACHINEIO_DEPS_FILE="$tmp_dir/machineio-good.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/browser-raw.out" 2>"$tmp_dir/browser-raw.err"; then
  echo "verify-core-boundaries accepted browser dependencies on raw runtime packages" >&2
  cat "$tmp_dir/browser-raw.out" >&2
  cat "$tmp_dir/browser-raw.err" >&2
  exit 1
fi

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
  ZSCALERCTL_CLI_DEPS_FILE="$tmp_dir/cli-good.deps" \
  ZSCALERCTL_RESOURCES_DEPS_FILE="$tmp_dir/resources-good.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-good.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-bad.deps" \
  ZSCALERCTL_MACHINEIO_DEPS_FILE="$tmp_dir/machineio-good.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/machine.out" 2>"$tmp_dir/machine.err"; then
  echo "verify-core-boundaries accepted machine dependencies on CLI/UI/rendering packages" >&2
  cat "$tmp_dir/machine.out" >&2
  cat "$tmp_dir/machine.err" >&2
  exit 1
fi

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
  ZSCALERCTL_CLI_DEPS_FILE="$tmp_dir/cli-good.deps" \
  ZSCALERCTL_RESOURCES_DEPS_FILE="$tmp_dir/resources-good.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-good.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-raw-bad.deps" \
  ZSCALERCTL_MACHINEIO_DEPS_FILE="$tmp_dir/machineio-good.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/machine-raw.out" 2>"$tmp_dir/machine-raw.err"; then
  echo "verify-core-boundaries accepted machine dependencies on raw runtime packages" >&2
  cat "$tmp_dir/machine-raw.out" >&2
  cat "$tmp_dir/machine-raw.err" >&2
  exit 1
fi

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
  ZSCALERCTL_CLI_DEPS_FILE="$tmp_dir/cli-good.deps" \
  ZSCALERCTL_RESOURCES_DEPS_FILE="$tmp_dir/resources-good.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-good.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-good.deps" \
  ZSCALERCTL_MACHINEIO_DEPS_FILE="$tmp_dir/machineio-bad.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/machineio.out" 2>"$tmp_dir/machineio.err"; then
  echo "verify-core-boundaries accepted machineio dependencies on CLI/UI/rendering packages" >&2
  cat "$tmp_dir/machineio.out" >&2
  cat "$tmp_dir/machineio.err" >&2
  exit 1
fi

if ZSCALERCTL_CMD_DEPS_FILE="$tmp_dir/cmd-good.deps" \
  ZSCALERCTL_CLI_DEPS_FILE="$tmp_dir/cli-good.deps" \
  ZSCALERCTL_RESOURCES_DEPS_FILE="$tmp_dir/resources-good.deps" \
  ZSCALERCTL_BROWSER_DEPS_FILE="$tmp_dir/browser-good.deps" \
  ZSCALERCTL_MACHINE_DEPS_FILE="$tmp_dir/machine-good.deps" \
  ZSCALERCTL_MACHINEIO_DEPS_FILE="$tmp_dir/machineio-raw-bad.deps" \
  "$repo_root/scripts/verify-core-boundaries.sh" >"$tmp_dir/machineio-raw.out" 2>"$tmp_dir/machineio-raw.err"; then
  echo "verify-core-boundaries accepted machineio dependencies on raw runtime packages" >&2
  cat "$tmp_dir/machineio-raw.out" >&2
  cat "$tmp_dir/machineio-raw.err" >&2
  exit 1
fi

if ! grep -q "internal/resources imports forbidden dependencies" "$tmp_dir/resources-runtime.err"; then
  echo "verify-core-boundaries failed without the expected resources runtime-facade boundary message" >&2
  cat "$tmp_dir/resources-runtime.err" >&2
  exit 1
fi

if ! grep -q "internal/machine imports forbidden dependencies" "$tmp_dir/machine.err"; then
  echo "verify-core-boundaries failed without the expected machine boundary message" >&2
  cat "$tmp_dir/machine.err" >&2
  exit 1
fi

if ! grep -q "internal/browser imports forbidden dependencies" "$tmp_dir/browser.err"; then
  echo "verify-core-boundaries failed without the expected browser boundary message" >&2
  cat "$tmp_dir/browser.err" >&2
  exit 1
fi

if ! grep -q "internal/browser imports forbidden dependencies" "$tmp_dir/browser-raw.err"; then
  echo "verify-core-boundaries failed without the expected browser raw-runtime boundary message" >&2
  cat "$tmp_dir/browser-raw.err" >&2
  exit 1
fi

if ! grep -q "internal/machine imports forbidden dependencies" "$tmp_dir/machine-raw.err"; then
  echo "verify-core-boundaries failed without the expected machine raw-runtime boundary message" >&2
  cat "$tmp_dir/machine-raw.err" >&2
  exit 1
fi

if ! grep -q "internal/machineio imports forbidden dependencies" "$tmp_dir/machineio.err"; then
  echo "verify-core-boundaries failed without the expected machineio boundary message" >&2
  cat "$tmp_dir/machineio.err" >&2
  exit 1
fi

if ! grep -q "internal/machineio imports forbidden dependencies" "$tmp_dir/machineio-raw.err"; then
  echo "verify-core-boundaries failed without the expected machineio raw-runtime boundary message" >&2
  cat "$tmp_dir/machineio-raw.err" >&2
  exit 1
fi
