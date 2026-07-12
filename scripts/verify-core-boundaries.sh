#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
stdlib_imports_file="$tmp_dir/stdlib.imports"
go list std >"$stdlib_imports_file"

check_package() {
  local label="$1"
  local package="$2"
  local deps_file_env="$3"
  local forbidden_re="$4"
  local guidance="$5"
  local mode="${6:-deps}"
  local deps_file="$tmp_dir/${label//[^A-Za-z0-9]/_}.deps"
  local matches

  if [[ -n "${!deps_file_env:-}" ]]; then
    cat "${!deps_file_env}" >"$deps_file"
  else
    case "$mode" in
      deps)
        go list -deps -mod=vendor "$package" >"$deps_file"
        ;;
      imports)
        go list -f '{{range .Imports}}{{.}}{{"\n"}}{{end}}' -mod=vendor "$package" >"$deps_file"
        ;;
      *)
        echo "verify-core-boundaries: unknown check mode $mode" >&2
        exit 1
        ;;
    esac
  fi

  matches="$(grep -E "$forbidden_re" "$deps_file" || true)"
  if [[ -n "$matches" ]]; then
    echo "verify-core-boundaries: $label imports forbidden dependencies:" >&2
    sed 's/^/  /' <<<"$matches" >&2
    echo "$guidance" >&2
    exit 1
  fi
}

check_package_import_allowlist() {
  local label="$1"
  local package="$2"
  local imports_file_env="$3"
  local allowed_project_re="$4"
  local guidance="$5"
  local imports_file="$tmp_dir/${label//[^A-Za-z0-9]/_}.imports"
  local import_path
  local matches=""

  if [[ -n "${!imports_file_env:-}" ]]; then
    cat "${!imports_file_env}" >"$imports_file"
  else
    go list -f '{{range .Imports}}{{.}}{{"\n"}}{{end}}' -mod=vendor "$package" >"$imports_file"
  fi

  while IFS= read -r import_path; do
    if [[ -z "$import_path" ]] || grep -Fxq -- "$import_path" "$stdlib_imports_file"; then
      continue
    fi
    if [[ -n "$allowed_project_re" && "$import_path" =~ $allowed_project_re ]]; then
      continue
    fi
    matches+="${matches:+$'\n'}$import_path"
  done <"$imports_file"
  if [[ -n "$matches" ]]; then
    echo "verify-core-boundaries: $label imports dependencies outside its allowlist:" >&2
    sed 's/^/  /' <<<"$matches" >&2
    echo "$guidance" >&2
    exit 1
  fi
}

ui_runtime_re='github\.com/charmbracelet/(bubbletea|bubbles)|github\.com/wailsapp/wails|vite|react|internal/tui'
cli_rendering_re='github\.com/spf13/cobra|github\.com/charmbracelet/lipgloss|internal/(cli|output)'
raw_runtime_re='github\.com/dvmrry/zscalerctl/internal/(config|credentials|secret|secretref|zscaler|runtime)'
cli_zscaler_re='^github\.com/dvmrry/zscalerctl/internal/zscaler$'
enginewire_adapter_allowed_re='^github\.com/dvmrry/zscalerctl/internal/(diff|enginewire|machine|redact|resources)$'
enginehost_allowed_re='^github\.com/dvmrry/zscalerctl/internal/(enginewire(/adapter)?|machine)$'
enginecmd_allowed_re='^github\.com/dvmrry/zscalerctl/internal/(enginehost|machine|redact|runtime|version)$'

check_package \
  "cmd/zscalerctl" \
  "./cmd/zscalerctl" \
  "ZSCALERCTL_CMD_DEPS_FILE" \
  "(^|/)(${ui_runtime_re})(/|$)" \
  "cmd/zscalerctl must remain the normal CLI binary; UI runtimes belong outside this dependency graph."

check_package \
  "internal/resources" \
  "./internal/resources" \
  "ZSCALERCTL_RESOURCES_DEPS_FILE" \
  "(^|/)(${ui_runtime_re}|${cli_rendering_re}|${raw_runtime_re})(/|$)" \
  "internal/resources must remain a safe catalog/projection seam: no CLI/UI/rendering packages and no raw runtime, secret, credential, SDK adapter, or runtime facade packages."

check_package \
  "internal/browser" \
  "./internal/browser" \
  "ZSCALERCTL_BROWSER_DEPS_FILE" \
  "(^|/)(${ui_runtime_re}|${cli_rendering_re}|${raw_runtime_re})(/|$)" \
  "internal/browser must remain an overlay-facing projected-record seam: no CLI/UI/rendering packages and no raw config, secret, credential, or SDK adapter packages."

check_package \
  "internal/machine" \
  "./internal/machine" \
  "ZSCALERCTL_MACHINE_DEPS_FILE" \
  "(^|/)(${ui_runtime_re}|${cli_rendering_re}|${raw_runtime_re})(/|$)" \
  "internal/machine must remain transport-neutral and projected-record only: no CLI/UI/rendering packages and no raw config, secret, credential, or SDK adapter packages."

check_package \
  "internal/machineio" \
  "./internal/machineio" \
  "ZSCALERCTL_MACHINEIO_DEPS_FILE" \
  "(^|/)(${ui_runtime_re}|${cli_rendering_re}|${raw_runtime_re})(/|$)" \
  "internal/machineio must remain a machine JSON adapter helper: no CLI/UI/rendering packages and no raw config, secret, credential, or SDK adapter packages."

check_package_import_allowlist \
  "internal/enginewire" \
  "./internal/enginewire" \
  "ZSCALERCTL_ENGINEWIRE_IMPORTS_FILE" \
  "" \
  "internal/enginewire must remain a standard-library-only transport contract; cgo, in-process engine, and third-party dependencies belong outside it."

check_package_import_allowlist \
  "internal/enginewire/adapter" \
  "./internal/enginewire/adapter" \
  "ZSCALERCTL_ENGINEWIRE_ADAPTER_IMPORTS_FILE" \
  "$enginewire_adapter_allowed_re" \
  "internal/enginewire/adapter may directly import only the standard library and the exact enginewire, machine, resources, redact, and diff seams."

check_package_import_allowlist \
  "internal/enginehost" \
  "./internal/enginehost" \
  "ZSCALERCTL_ENGINEHOST_IMPORTS_FILE" \
  "$enginehost_allowed_re" \
  "internal/enginehost may orchestrate only the wire contract, its explicit adapter, and machine DTOs; config, runtime, SDK, CLI, UI, and cgo dependencies are forbidden."

check_package_import_allowlist \
  "cmd/zscalerctl-engine" \
  "./cmd/zscalerctl-engine" \
  "ZSCALERCTL_ENGINECMD_IMPORTS_FILE" \
  "$enginecmd_allowed_re" \
  "cmd/zscalerctl-engine is a narrow process adapter: it may assemble runtime policy and the host but must not import CLI/UI, config/secret packages, SDK adapters, third-party packages, or cgo."

check_package \
  "internal/cli" \
  "./internal/cli" \
  "ZSCALERCTL_CLI_DEPS_FILE" \
  "${cli_zscaler_re}" \
  "internal/cli must route live Zscaler access through internal/runtime and must not import internal/zscaler directly." \
  "imports"

echo "verify-core-boundaries: PASS"
