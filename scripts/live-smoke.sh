#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

denied_exact_keys_json='["preSharedKey","vpnCredentials","createdBy","lastModifiedBy","managedBy","city","primaryDestVip","secondaryDestVip"]'
denied_resource_exact_keys_json='{"location-groups":["lastModUser","dynamicLocationGroupCriteria","locations"]}'
allowed_resource_denied_keys_json='{"atp-malware-policy":["blockPasswordProtectedArchiveFiles"],"mobile-threat-settings":["blockAppsSendingUnencryptedUserCredentials"],"org-information":["city"]}'
denied_key_pattern='(?i)(password|secret|token|api[_-]?key|preSharedKey|credential|cert|fingerprint)'
manifest_warning='sanitized dumps remain confidential operational data'

out_dir=""
default_manifest="live-smoke.manifest"
manifest_path=""
disable_manifest=0
require_nonempty=0
require_credentials=0
skip_credential_check=0
strict_counts=0
failures=0
resources=()
requested_resources=()
failure_messages=()
result_rows=()
summary_stderr_lines=4
summary_stderr_chars=220
summary_note_chars=72

usage() {
  cat <<'EOF'
usage: scripts/live-smoke.sh [--out DIR] [--bin PATH] [--resources LIST] [--manifest FILE] [--no-manifest] [--require-credentials] [--require-nonempty] [--strict-counts]

Runs a read-only live smoke against the currently configured zscalerctl
credentials and prints PASS/FAIL markers for pre-PR validation.
By default, all current ZIA read resources are validated. A manifest or
--resources value may select qualified ZPA resources such as zpa/server-groups.

Options:
  --out DIR            Write validation artifacts under DIR. Defaults to a
                       secure temporary directory that is kept for inspection.
  --bin PATH           zscalerctl binary to run. Defaults to
                       "go run -mod=vendor ./cmd/zscalerctl".
  --resources LIST     Optional comma-separated resource filter, using bare ZIA
                       names or product/name. Defaults to all ZIA resources.
  --manifest FILE      Read the resource filter from a line-oriented manifest.
                       Comments, Markdown bullets, and comma-separated entries
                       are accepted.
  --no-manifest        Disable automatic live-smoke.manifest discovery.
  --require-credentials
                       Fail instead of SKIP when no supported live credential
                       family is configured. Use this for release gating.
  --require-nonempty   Treat a zero-record resource list as a failure.
  --strict-counts      Fail if a list count differs from the dump count.
                       By default this is INFO because live data can change.
  --skip-credential-check
                       Internal test hook for fake CLIs.
  -h, --help           Show this help.

This script does not print credential values or live resource payloads. It
recognizes explicit zscalerctl OneAPI credentials and explicit ZIA legacy
credentials; selected ZPA resources require OneAPI credentials plus
ZSCALERCTL_ZPA_CUSTOMER_ID. Raw SDK env vars such as ZIA_USERNAME are
intentionally ignored.
EOF
}

trim_space() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

normalize_requested_resource() {
  local resource

  resource="$(trim_space "$1")"
  if [[ -z "$resource" ]]; then
    echo "--resources contains an empty entry" >&2
    exit 2
  fi

  case "$resource" in
    zia/*|zpa/*)
      ;;
    */*)
      echo "--resources supports only zia/ or zpa/ qualified resources; got: $resource" >&2
      exit 2
      ;;
    *)
      resource="zia/$resource"
      ;;
  esac

  if [[ -z "${resource#*/}" ]]; then
    echo "--resources contains an empty resource name" >&2
    exit 2
  fi

  printf '%s' "$resource"
}

add_requested_resources() {
  local list="$1"
  local entry
  local entries

  IFS=',' read -r -a entries <<<"$list"
  for entry in "${entries[@]}"; do
    requested_resources+=("$(normalize_requested_resource "$entry")")
  done
}

add_manifest_resources() {
  local path="$1"
  local before="${#requested_resources[@]}"
  local line

  if [[ ! -f "$path" ]]; then
    echo "live smoke manifest not found: $path" >&2
    exit 2
  fi

  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%%#*}"
    line="$(trim_space "$line")"
    case "$line" in
      "- "*|"* "*)
        line="$(trim_space "${line#?}")"
        ;;
    esac
    if [[ -z "$line" ]]; then
      continue
    fi
    add_requested_resources "$line"
  done <"$path"

  if ((${#requested_resources[@]} == before)); then
    echo "live smoke manifest contains no resources: $path" >&2
    exit 2
  fi
}

git_branch_name() {
  git branch --show-current 2>/dev/null || true
}

manifest_changed_from_base() {
  local base="${LIVE_SMOKE_MANIFEST_BASE:-origin/main}"

  if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    return 0
  fi
  if ! git rev-parse --verify -q "$base" >/dev/null 2>&1; then
    return 0
  fi

  if ! git diff --quiet "$base"...HEAD -- "$default_manifest"; then
    return 0
  fi
  if ! git diff --quiet -- "$default_manifest"; then
    return 0
  fi
  [[ -n "$(git ls-files --others --exclude-standard -- "$default_manifest")" ]]
}

should_use_default_manifest() {
  local branch

  if ((disable_manifest)); then
    return 1
  fi
  if ((${#requested_resources[@]} != 0)); then
    return 1
  fi
  if [[ ! -f "$default_manifest" ]]; then
    return 1
  fi

  branch="$(git_branch_name)"
  case "$branch" in
    ""|main|master)
      return 1
      ;;
  esac

  manifest_changed_from_base
}

while (($#)); do
  case "$1" in
    --out)
      if (($# < 2)); then
        echo "--out requires a directory" >&2
        exit 2
      fi
      out_dir="$2"
      shift 2
      ;;
    --bin)
      if (($# < 2)); then
        echo "--bin requires a path" >&2
        exit 2
      fi
      ZSCALERCTL_BIN="$2"
      shift 2
      ;;
    --resources)
      if (($# < 2)); then
        echo "--resources requires a comma-separated list" >&2
        exit 2
      fi
      add_requested_resources "$2"
      shift 2
      ;;
    --manifest)
      if (($# < 2)); then
        echo "--manifest requires a file" >&2
        exit 2
      fi
      manifest_path="$2"
      shift 2
      ;;
    --no-manifest)
      disable_manifest=1
      shift
      ;;
    --require-credentials)
      require_credentials=1
      shift
      ;;
    --require-nonempty)
      require_nonempty=1
      shift
      ;;
    --strict-counts)
      strict_counts=1
      shift
      ;;
    --skip-credential-check)
      skip_credential_check=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

pass() {
  printf '[PASS] %s\n' "$*"
}

info() {
  printf '[INFO] %s\n' "$*"
}

skip() {
  printf '[SKIP] %s\n' "$*"
}

fail() {
  local message="$*"
  printf '[FAIL] %s\n' "$message" >&2
  failure_messages+=("$message")
  failures=$((failures + 1))
}

artifact_note() {
  local path="$1"
  printf '%s' "${path#"$out_dir/"}"
}

record_result() {
  local resource="$1"
  local phase="$2"
  local status="$3"
  local records="$4"
  local note="$5"

  note="${note//$'\n'/ }"
  note="${note//|/ }"
  if [[ -z "$records" ]]; then
    records="-"
  fi
  result_rows+=("$resource|$phase|$status|$records|$note")
}

record_result_from_failures() {
  local resource="$1"
  local phase="$2"
  local start_failures="$3"
  local records="$4"
  local pass_note="$5"
  local fail_note="$6"

  if ((failures == start_failures)); then
    record_result "$resource" "$phase" "PASS" "$records" "$pass_note"
  else
    record_result "$resource" "$phase" "FAIL" "$records" "$fail_note"
  fi
}

fit_cell() {
  local value="$1"
  local width="$2"

  value="${value//$'\n'/ }"
  if ((${#value} > width)); then
    printf '%s...' "${value:0:$((width - 3))}"
    return
  fi
  printf '%s' "$value"
}

print_result_table() {
  local resource_width=36
  local phase_width=10
  local status_width=6
  local records_width=7
  local resource
  local phase
  local status
  local records
  local note
  local row

  if ((${#result_rows[@]} == 0)); then
    return
  fi

  printf '\n'
  printf 'live smoke results\n'
  printf '%-*s  %-*s  %-*s  %-*s  %s\n' \
    "$resource_width" "RESOURCE" \
    "$phase_width" "PHASE" \
    "$status_width" "STATUS" \
    "$records_width" "RECORDS" \
    "NOTE"
  printf '%-*s  %-*s  %-*s  %-*s  %s\n' \
    "$resource_width" "------------------------------------" \
    "$phase_width" "----------" \
    "$status_width" "------" \
    "$records_width" "-------" \
    "----"

  for row in "${result_rows[@]}"; do
    IFS='|' read -r resource phase status records note <<<"$row"
    note="$(fit_cell "$note" "$summary_note_chars")"
    printf '%-*s  %-*s  %-*s  %-*s  %s\n' \
      "$resource_width" "$(fit_cell "$resource" "$resource_width")" \
      "$phase_width" "$(fit_cell "$phase" "$phase_width")" \
      "$status_width" "$status" \
      "$records_width" "$(fit_cell "$records" "$records_width")" \
      "$note"
  done
  printf '\n'
}

write_failure_summary() {
  local failure_count="$1"
  local summary_file="$out_dir/failure-summary.txt"
  local file
  local message
  local found_stderr=0

  {
    printf 'zscalerctl live-smoke failure summary\n'
    printf 'failures: %s\n' "$failure_count"
    printf 'artifacts: %s\n' "$out_dir"
    printf '\n'
    printf 'failure markers:\n'
    for message in "${failure_messages[@]}"; do
      printf -- '- %s\n' "$message"
    done
    printf '\n'
    printf 'non-empty stderr snippets (first %s compacted lines each):\n' "$summary_stderr_lines"
    while IFS= read -r file; do
      if [[ ! -s "$file" ]]; then
        continue
      fi
      found_stderr=1
      printf '\n'
      printf '===== %s =====\n' "$file"
      write_stderr_summary "$file"
    done < <(find "$out_dir" -type f -name '*.stderr' -print | sort)
    if ((found_stderr == 0)); then
      printf '<none>\n'
    fi
  } >"$summary_file"
  chmod 600 "$summary_file"
  printf '%s' "$summary_file"
}

write_stderr_summary() {
  local file="$1"
  local kind
  local message

  if kind="$(jq -r '.error.kind // empty' "$file" 2>/dev/null)" && [[ -n "$kind" ]]; then
    message="$(jq -r '.error.message // empty' "$file" 2>/dev/null)"
    printf 'error: %s' "$kind"
    if [[ -n "$message" ]]; then
      printf ' - %s' "$message"
    fi
    printf '\n'
    return
  fi

  awk -v max_lines="$summary_stderr_lines" -v max_chars="$summary_stderr_chars" '
    NR > max_lines {
      omitted = 1
      next
    }
    {
      line = $0
      gsub(/[[:space:]]+/, " ", line)
      if (length(line) > max_chars) {
        line = substr(line, 1, max_chars) "... <truncated>"
      }
      print line
    }
    END {
      if (omitted) {
        print "... <additional stderr omitted; see full artifact file>"
      }
    }
  ' "$file"
}

print_failure_summary() {
  local summary_file="$1"

  printf '[INFO] failure summary: %s\n' "$summary_file" >&2
  printf '\n' >&2
  sed -n '1,220p' "$summary_file" >&2
}

if [[ -n "$manifest_path" ]]; then
  add_manifest_resources "$manifest_path"
  info "using live smoke manifest: $manifest_path"
elif should_use_default_manifest; then
  add_manifest_resources "$default_manifest"
  info "using live smoke manifest: $default_manifest"
fi

is_set() {
  [[ -n "${!1:-}" ]]
}

credential_family() {
  local mode="${ZSCALERCTL_AUTH_MODE:-}"
  local oneapi=0
  local legacy=0

  if is_set ZSCALERCTL_CLIENT_ID &&
    (is_set ZSCALERCTL_CLIENT_SECRET || is_set ZSCALERCTL_CLIENT_SECRET_FILE) &&
    is_set ZSCALERCTL_VANITY_DOMAIN; then
    oneapi=1
  fi

  if is_set ZSCALERCTL_ZIA_USERNAME &&
    (is_set ZSCALERCTL_ZIA_PASSWORD || is_set ZSCALERCTL_ZIA_PASSWORD_FILE) &&
    (is_set ZSCALERCTL_ZIA_API_KEY || is_set ZSCALERCTL_ZIA_API_KEY_FILE) &&
    is_set ZSCALERCTL_ZIA_CLOUD; then
    legacy=1
  fi

  case "$mode" in
    zia-legacy)
      if ((legacy)); then
        printf 'ZIA legacy'
        return 0
      fi
      return 1
      ;;
    oneapi|"")
      if ((oneapi)); then
        printf 'OneAPI'
        return 0
      fi
      if [[ -z "$mode" ]] && ((legacy)); then
        printf 'ZIA legacy'
        return 0
      fi
      return 1
      ;;
    *)
      return 1
      ;;
  esac
}

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 2
  fi
}

mode_of() {
  case "$(uname -s)" in
    Darwin|FreeBSD)
      stat -f '%Lp' "$1"
      ;;
    *)
      stat -c '%a' "$1"
      ;;
  esac
}

find_denied_keys() {
  local resource="$1"
  local file="$2"

  jq -r --argjson global_exact "$denied_exact_keys_json" --argjson resource_exact "$denied_resource_exact_keys_json" --argjson resource_allowed "$allowed_resource_denied_keys_json" --arg resource "$resource" --arg pattern "$denied_key_pattern" '
    ($global_exact + ($resource_exact[$resource] // [])) as $exact
    | ($resource_allowed[$resource] // []) as $allowed
    |
    [.. | objects | keys[] | select(. as $k | (($allowed | index($k)) | not) and (($exact | index($k)) or ($k | test($pattern))))] | unique | .[]
  ' "$file"
}

validate_json_array() {
  local label="$1"
  local file="$2"
  local count

  if ! jq -e 'type == "array"' "$file" >/dev/null 2>&1; then
    fail "$label is not a JSON array: $file"
    return 1
  fi
  pass "$label returned a JSON array"

  count="$(jq 'length' "$file")"
  if [[ "$count" == "0" ]]; then
    if ((require_nonempty)); then
      fail "$label returned 0 records"
      return 1
    else
      info "$label returned 0 records"
    fi
  else
    pass "$label returned $count records"
  fi
  return 0
}

validate_json_object() {
  local label="$1"
  local file="$2"

  if ! jq -e 'type == "object"' "$file" >/dev/null 2>&1; then
    fail "$label is not a JSON object: $file"
    return 1
  fi
  pass "$label returned a JSON object"
  if ! jq -e 'keys | length > 0' "$file" >/dev/null 2>&1; then
    fail "$label returned an empty JSON object"
    return 1
  fi
  pass "$label returned 1 record"
  return 0
}

validate_no_denied_keys() {
  local label="$1"
  local resource="$2"
  local file="$3"
  local denied

  denied="$(find_denied_keys "$resource" "$file")"
  if [[ -n "$denied" ]]; then
    fail "$label contains denied field key(s): $(tr '\n' ' ' <<<"$denied")"
    return
  fi
  pass "$label contains no denied field keys"
}

find_non_catalog_keys() {
  local product="$1"
  local resource="$2"
  local file="$3"
  local schema="$4"

  jq -r --slurpfile schema "$schema" --arg product "$product" --arg resource "$resource" '
    ($schema[0] | map(select(.product == $product and .name == $resource)) | .[0]) as $spec
    | if $spec == null then
        ["<missing schema resource>"]
      else
        ([
          $spec.fields[]?
          | select(any(.allowed_modes[]?; . == "standard"))
          | (.json_name // .name)
        ]) as $allowed
        | [
          (if type == "array" then .[]? elif type == "object" then . else empty end)
          | objects
          | keys[] as $key
          | select(($allowed | index($key)) | not)
          | $key
        ]
        | unique
      end
    | .[]
  ' "$file"
}

validate_catalog_subset() {
  local label="$1"
  local product="$2"
  local resource="$3"
  local file="$4"
  local schema="$5"
  local unexpected

  unexpected="$(find_non_catalog_keys "$product" "$resource" "$file" "$schema")"
  if [[ -n "$unexpected" ]]; then
    fail "$label contains non-catalog field key(s): $(tr '\n' ' ' <<<"$unexpected")"
    return
  fi
  pass "$label contains only catalog-allowed top-level fields"
}

redaction_marker_paths() {
  jq -r '
    def fieldpath:
      reduce .[] as $part ("";
        if ($part | type) == "number" then
          . + "[]"
        elif . == "" then
          . + ($part | tostring)
        else
          . + "." + ($part | tostring)
        end
      );

    [
      paths(strings) as $path
      | select(getpath($path) | contains("<REDACTED:"))
      | $path | fieldpath
    ]
    | unique
    | .[]
  ' "$1"
}

summarize_redaction_markers() {
  local label="$1"
  local file="$2"
  local paths
  local summary

  paths="$(redaction_marker_paths "$file")"
  if [[ -n "$paths" ]]; then
    summary="$(printf '%s\n' "$paths" | paste -sd ' ' -)"
    info "$label redaction markers at: $summary"
    return
  fi
  pass "$label has no redaction markers"
}

load_smoke_resources() {
  local schema_file="$1"
  local stderr_file="$2"
  local product
  local resource
  local requested
  local all_resources=()
  local default_resources=()

  if "${cli[@]}" --format json schema list >"$schema_file" 2>"$stderr_file"; then
    pass "schema list command completed"
  else
    fail "schema list command failed; stderr captured at $stderr_file"
    record_result "schema" "list" "FAIL" "-" "see $(artifact_note "$stderr_file")"
    return 1
  fi

  if ! jq -e 'type == "array"' "$schema_file" >/dev/null 2>&1; then
    fail "schema list is not a JSON array: $schema_file"
    record_result "schema" "list" "FAIL" "-" "invalid JSON; see $(artifact_note "$schema_file")"
    return 1
  fi
  pass "schema list returned a JSON array"

  while IFS=$'\t' read -r product resource; do
    all_resources+=("$product/$resource")
    if [[ "$product" == "zia" ]]; then
      default_resources+=("$product/$resource")
    fi
  done < <(jq -r '
    [
      .[]
      | select(.product == "zia" or .product == "zpa")
      | select(any(.operations[]?; (.name == "list" or .name == "show") and .capability == "read"))
      | [.product, .name]
    ]
    | sort_by(.[0], .[1])
    | .[]
    | @tsv
  ' "$schema_file")

  if ((${#default_resources[@]} == 0)); then
    fail "schema list contains no default ZIA read resources"
    return 1
  fi

  if ((${#requested_resources[@]} == 0)); then
    resources=("${default_resources[@]}")
  else
    for requested in "${requested_resources[@]}"; do
      if ! resource_in_list "$requested" "${all_resources[@]}"; then
        fail "requested resource is not a read resource: $requested"
        return 1
      fi
      if ((${#resources[@]} == 0)) || ! resource_in_list "$requested" "${resources[@]}"; then
        resources+=("$requested")
      fi
    done
  fi

  pass "schema list found ${#all_resources[@]} ZIA/ZPA read resource(s)"
  pass "live smoke selected ${#resources[@]} resource(s): ${resources[*]}"
  record_result "schema" "list" "PASS" "${#all_resources[@]}" "selected ${#resources[@]} resources"
  return 0
}

resource_product() {
  printf '%s' "${1%%/*}"
}

resource_name() {
  printf '%s' "${1#*/}"
}

resource_artifact_name() {
  printf '%s' "${1//\//-}"
}

selected_products() {
  local seen=""
  local qualified
  local product

  for qualified in "${resources[@]}"; do
    product="$(resource_product "$qualified")"
    if [[ " $seen " != *" $product "* ]]; then
      printf '%s\n' "$product"
      seen+=" $product"
    fi
  done
}

selected_has_product() {
  local want="$1"
  local qualified

  for qualified in "${resources[@]}"; do
    if [[ "$(resource_product "$qualified")" == "$want" ]]; then
      return 0
    fi
  done
  return 1
}

resource_operation() {
  local qualified="$1"
  local schema="$2"
  local product
  local resource

  product="$(resource_product "$qualified")"
  resource="$(resource_name "$qualified")"
  jq -r --arg product "$product" --arg resource "$resource" '
    .[]
    | select(.product == $product and .name == $resource)
    | if any(.operations[]?; .name == "list" and .capability == "read") then
        "list"
      elif any(.operations[]?; .name == "show" and .capability == "read") then
        "show"
      else
        empty
      end
  ' "$schema" | head -n 1
}

resource_in_list() {
  local needle="$1"
  shift

  local item
  for item in "$@"; do
    if [[ "$item" == "$needle" ]]; then
      return 0
    fi
  done
  return 1
}

join_csv() {
  local first=1
  local item

  for item in "$@"; do
    if ((first)); then
      first=0
    else
      printf ','
    fi
    printf '%s' "$item"
  done
}

compare_counts() {
  local qualified="$1"
  local operation="$2"
  local list_count="$3"
  local dump_count="$4"

  if [[ "$list_count" == "$dump_count" ]]; then
    pass "$qualified $operation and dump counts match ($list_count records)"
    return
  fi
  if ((strict_counts)); then
    fail "$qualified $operation count = $list_count, dump count = $dump_count"
  else
    info "$qualified $operation count = $list_count, dump count = $dump_count; live data may have changed between reads"
  fi
}

write_expected_dump_paths() {
  local output="$1"
  local qualified

  : >"$output"
  for qualified in "${resources[@]}"; do
    printf 'resources/%s/%s.json\n' "$(resource_product "$qualified")" "$(resource_name "$qualified")" >>"$output"
  done
  sort -o "$output" "$output"
}

validate_manifest_resource_set() {
  local manifest="$1"
  local expected="$2"
  local actual="$3"
  local diff_file="$4"

  jq -r '.resources[]? | .path' "$manifest" | sort >"$actual"
  if diff -u "$expected" "$actual" >"$diff_file"; then
    pass "dump manifest resource set matches selected resources"
  else
    fail "dump manifest resource set differs from selected resources; diff captured at $diff_file"
  fi
}

validate_dump_file_set() {
  local dump_dir="$1"
  local expected="$2"
  local actual="$3"
  local diff_file="$4"

  if [[ ! -d "$dump_dir/resources" ]]; then
    fail "dump resources directory missing: $dump_dir/resources"
    return
  fi

  find "$dump_dir/resources" -mindepth 2 -maxdepth 2 -type f -name '*.json' -print |
    while IFS= read -r path; do
      printf '%s\n' "${path#"$dump_dir/"}"
    done | sort >"$actual"

  if diff -u "$expected" "$actual" >"$diff_file"; then
    pass "dump resource files match selected resources"
  else
    fail "dump resource files differ from selected resources; diff captured at $diff_file"
  fi
}

summarize_redaction_report() {
  local report="$1"
  local found=0
  local product
  local name
  local dropped
  local redacted

  while IFS=$'\t' read -r product name dropped redacted; do
    found=1
    info "redaction report $product $name: dropped fields [$dropped], redacted fields [$redacted]"
  done < <(jq -r '
    .resources[]?
    | select(((.dropped_fields // []) | length) > 0 or ((.redacted_fields // []) | length) > 0)
    | [.product, .name, ((.dropped_fields // []) | join(",")), ((.redacted_fields // []) | join(","))]
    | @tsv
  ' "$report")

  if ((found == 0)); then
    pass "redaction report has no dropped or redacted field entries"
  fi
}

validate_file_mode() {
  local label="$1"
  local path="$2"
  local want="$3"
  local got

  if [[ ! -e "$path" ]]; then
    fail "$label missing: $path"
    return
  fi

  got="$(mode_of "$path")"
  if [[ "$got" != "$want" ]]; then
    fail "$label mode = $got, want $want: $path"
    return
  fi
  pass "$label mode is $want"
}

if [[ -n "${ZSCALERCTL_BIN:-}" ]]; then
  if ! command -v "$ZSCALERCTL_BIN" >/dev/null 2>&1; then
    echo "zscalerctl binary not found or not executable: $ZSCALERCTL_BIN" >&2
    exit 2
  fi
  cli=( "$ZSCALERCTL_BIN" )
else
  cli=( go run -mod=vendor ./cmd/zscalerctl )
fi

need jq

if ((skip_credential_check)); then
  info "credential preflight skipped for fake CLI validation"
else
  if family="$(credential_family)"; then
    pass "live credential preflight found $family credentials"
  else
    message="no supported live credentials configured; set explicit zscalerctl OneAPI or ZIA legacy env vars"
    if ((require_credentials)); then
      fail "$message"
      exit 1
    fi
    skip "$message"
    exit 0
  fi
fi

if [[ -z "$out_dir" ]]; then
  out_dir="$(mktemp -d "${TMPDIR:-/tmp}/zscalerctl-live-smoke.XXXXXX")"
else
  if [[ -e "$out_dir" ]] && [[ -n "$(find "$out_dir" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
    echo "output directory already exists and is not empty: $out_dir" >&2
    exit 2
  fi
  mkdir -p "$out_dir"
fi
chmod 700 "$out_dir"

lists_dir="$out_dir/lists"
mkdir -p "$lists_dir"
chmod 700 "$lists_dir"

info "artifacts: $out_dir"
info "using CLI: ${cli[*]}"

schema_file="$lists_dir/schema.json"
schema_stderr="$lists_dir/schema.stderr"
if ! load_smoke_resources "$schema_file" "$schema_stderr"; then
  fail "live smoke cannot continue without a valid schema resource set"
  print_result_table >&2
  summary_file="$(write_failure_summary "$failures")"
  print_failure_summary "$summary_file"
  exit 1
fi

if ((skip_credential_check == 0)) && selected_has_product zpa; then
  if [[ "${ZSCALERCTL_AUTH_MODE:-}" == "zia-legacy" ]]; then
    fail "selected ZPA resources require OneAPI credentials"
    print_result_table >&2
    summary_file="$(write_failure_summary "$failures")"
    print_failure_summary "$summary_file"
    exit 1
  fi
  if ! is_set ZSCALERCTL_CLIENT_ID ||
    (! is_set ZSCALERCTL_CLIENT_SECRET && ! is_set ZSCALERCTL_CLIENT_SECRET_FILE) ||
    ! is_set ZSCALERCTL_VANITY_DOMAIN; then
    fail "selected ZPA resources require OneAPI credentials"
    print_result_table >&2
    summary_file="$(write_failure_summary "$failures")"
    print_failure_summary "$summary_file"
    exit 1
  fi
  if ! is_set ZSCALERCTL_ZPA_CUSTOMER_ID; then
    fail "selected ZPA resources require ZSCALERCTL_ZPA_CUSTOMER_ID"
    print_result_table >&2
    summary_file="$(write_failure_summary "$failures")"
    print_failure_summary "$summary_file"
    exit 1
  fi
fi

expected_paths_file="$lists_dir/expected-dump-paths.txt"
write_expected_dump_paths "$expected_paths_file"

products=()
while IFS= read -r product; do
  products+=("$product")
done < <(selected_products)
products_csv="$(join_csv "${products[@]}")"

for qualified in "${resources[@]}"; do
  product="$(resource_product "$qualified")"
  resource="$(resource_name "$qualified")"
  artifact="$(resource_artifact_name "$qualified")"
  stdout_file="$lists_dir/${artifact}.json"
  stderr_file="$lists_dir/${artifact}.stderr"
  list_start_failures="$failures"
  list_records="-"
  operation="$(resource_operation "$qualified" "$schema_file")"

  if [[ -z "$operation" ]]; then
    fail "schema list does not expose a supported read operation for $qualified"
    record_result "$qualified" "schema" "FAIL" "-" "no supported read operation"
    continue
  fi

  if "${cli[@]}" --format json "$product" "$resource" "$operation" >"$stdout_file" 2>"$stderr_file"; then
    pass "$product $resource $operation command completed"
  else
    fail "$product $resource $operation command failed; stderr captured at $stderr_file"
    record_result "$qualified" "$operation" "FAIL" "-" "command failed; see $(artifact_note "$stderr_file")"
    continue
  fi

  if [[ "$operation" == "show" ]]; then
    if validate_json_object "$product $resource show" "$stdout_file"; then
      list_records="1"
      printf '1\n' >"$lists_dir/${artifact}.count"
      validate_no_denied_keys "$product $resource show" "$resource" "$stdout_file"
      validate_catalog_subset "$product $resource show" "$product" "$resource" "$stdout_file" "$schema_file"
      summarize_redaction_markers "$product $resource show" "$stdout_file"
    fi
  elif validate_json_array "$product $resource list" "$stdout_file"; then
    list_records="$(jq 'length' "$stdout_file")"
    printf '%s\n' "$list_records" >"$lists_dir/${artifact}.count"
    validate_no_denied_keys "$product $resource list" "$resource" "$stdout_file"
    validate_catalog_subset "$product $resource list" "$product" "$resource" "$stdout_file" "$schema_file"
    summarize_redaction_markers "$product $resource list" "$stdout_file"
  fi
  record_result_from_failures \
    "$qualified" \
    "$operation" \
    "$list_start_failures" \
    "$list_records" \
    "" \
    "see $(artifact_note "$stdout_file") / $(artifact_note "$stderr_file")"
done

dump_dir="$out_dir/dump"
dump_stdout="$out_dir/dump.stdout"
dump_stderr="$out_dir/dump.stderr"
dump_command_start_failures="$failures"
dump_args=(dump --products "$products_csv")
if ((${#requested_resources[@]} != 0)); then
  dump_args+=(--resources "$(join_csv "${resources[@]}")")
fi
dump_args+=(--out "$dump_dir")

if "${cli[@]}" "${dump_args[@]}" >"$dump_stdout" 2>"$dump_stderr"; then
  pass "$products_csv dump command completed"
else
  fail "$products_csv dump command failed; stderr captured at $dump_stderr"
fi
record_result_from_failures \
  "dump" \
  "command" \
  "$dump_command_start_failures" \
  "-" \
  "$(artifact_note "$dump_dir")" \
  "see $(artifact_note "$dump_stderr")"

if [[ -d "$dump_dir" ]]; then
  manifest_start_failures="$failures"
  validate_file_mode "dump root directory" "$dump_dir" "700"
  validate_file_mode "dump resources directory" "$dump_dir/resources" "700"
  for product in "${products[@]}"; do
    validate_file_mode "dump $product directory" "$dump_dir/resources/$product" "700"
  done

  manifest="$dump_dir/manifest.json"
  report="$dump_dir/redaction_report.json"
  if [[ -f "$manifest" ]]; then
    validate_file_mode "dump manifest" "$manifest" "600"
    if jq empty "$manifest" >/dev/null 2>&1; then
      pass "dump manifest is valid JSON"
      if jq -e --arg warning "$manifest_warning" '.warning == $warning' "$manifest" >/dev/null; then
        pass "dump manifest includes confidentiality warning"
      else
        fail "dump manifest missing confidentiality warning"
      fi
      if jq -e '.status == "complete"' "$manifest" >/dev/null; then
        pass "dump manifest status is complete"
      else
        fail "dump manifest status is not complete"
      fi
      if jq -e '(.errors // 0) == 0 and (.errors_path // "") == ""' "$manifest" >/dev/null; then
        pass "dump manifest has no partial-error metadata"
      else
        fail "dump manifest includes unexpected partial-error metadata"
      fi
      validate_manifest_resource_set "$manifest" "$expected_paths_file" "$lists_dir/manifest-dump-paths.txt" "$lists_dir/manifest-dump-paths.diff"
    else
      fail "dump manifest is not valid JSON: $manifest"
    fi
  else
    fail "dump manifest missing: $manifest"
  fi

  if [[ -f "$report" ]]; then
    validate_file_mode "redaction report" "$report" "600"
    if jq empty "$report" >/dev/null 2>&1; then
      pass "redaction report is valid JSON"
      if grep -q '<REDACTED:' "$report"; then
        fail "redaction report contains redaction marker values"
      else
        pass "redaction report is value-free"
      fi
      summarize_redaction_report "$report"
    else
      fail "redaction report is not valid JSON: $report"
    fi
  else
    fail "redaction report missing: $report"
  fi

  if [[ -e "$dump_dir/errors.ndjson" ]]; then
    fail "complete dump unexpectedly wrote errors.ndjson"
  else
    pass "complete dump did not write errors.ndjson"
  fi
  record_result_from_failures \
    "dump" \
    "manifest" \
    "$manifest_start_failures" \
    "${#resources[@]}" \
    "complete" \
    "see $(artifact_note "$dump_dir")"

  dump_files_start_failures="$failures"
  validate_dump_file_set "$dump_dir" "$expected_paths_file" "$lists_dir/actual-dump-paths.txt" "$lists_dir/actual-dump-paths.diff"
  record_result_from_failures \
    "dump" \
    "files" \
    "$dump_files_start_failures" \
    "${#resources[@]}" \
    "resource set matches" \
    "see $(artifact_note "$lists_dir/actual-dump-paths.diff")"

  for qualified in "${resources[@]}"; do
    product="$(resource_product "$qualified")"
    resource="$(resource_name "$qualified")"
    artifact="$(resource_artifact_name "$qualified")"
    file="$dump_dir/resources/$product/${resource}.json"
    dump_resource_start_failures="$failures"
    dump_records="-"
    operation="$(resource_operation "$qualified" "$schema_file")"
    if [[ ! -f "$file" ]]; then
      fail "dump resource file missing: $file"
      record_result "$qualified" "dump" "FAIL" "-" "missing $(artifact_note "$file")"
      continue
    fi
    validate_file_mode "dump $product $resource file" "$file" "600"
    if [[ "$operation" == "show" ]]; then
      if validate_json_object "dump $product $resource" "$file"; then
        dump_records="1"
        printf '1\n' >"$lists_dir/dump-${artifact}.count"
        validate_no_denied_keys "dump $product $resource" "$resource" "$file"
        validate_catalog_subset "dump $product $resource" "$product" "$resource" "$file" "$schema_file"
        summarize_redaction_markers "dump $product $resource" "$file"
        if [[ -f "$lists_dir/${artifact}.count" ]]; then
          compare_counts "$qualified" "$operation" "$(cat "$lists_dir/${artifact}.count")" "$(cat "$lists_dir/dump-${artifact}.count")"
        fi
      fi
    elif validate_json_array "dump $product $resource" "$file"; then
      dump_records="$(jq 'length' "$file")"
      printf '%s\n' "$dump_records" >"$lists_dir/dump-${artifact}.count"
      validate_no_denied_keys "dump $product $resource" "$resource" "$file"
      validate_catalog_subset "dump $product $resource" "$product" "$resource" "$file" "$schema_file"
      summarize_redaction_markers "dump $product $resource" "$file"
      if [[ -f "$lists_dir/${artifact}.count" ]]; then
        compare_counts "$qualified" "$operation" "$(cat "$lists_dir/${artifact}.count")" "$(cat "$lists_dir/dump-${artifact}.count")"
      fi
    fi
    record_result_from_failures \
      "$qualified" \
      "dump" \
      "$dump_resource_start_failures" \
      "$dump_records" \
      "" \
      "see $(artifact_note "$file")"
  done

  if [[ -f "$manifest" ]]; then
    while IFS=$'\t' read -r rel_path want_records; do
      file="$dump_dir/$rel_path"
      if [[ ! -f "$file" ]]; then
        fail "manifest references missing resource file: $rel_path"
        continue
      fi
      got_records="$(jq 'if type == "array" then length elif type == "object" then 1 else -1 end' "$file")"
      if [[ "$got_records" == "$want_records" ]]; then
        pass "manifest count matches $rel_path ($got_records records)"
      else
        fail "manifest count for $rel_path = $want_records, file has $got_records"
      fi
    done < <(jq -r '.resources[]? | [.path, (.records|tostring)] | @tsv' "$manifest" 2>/dev/null || true)
  fi
fi

if ((failures != 0)); then
  failure_count="$failures"
  print_result_table >&2
  summary_file="$(write_failure_summary "$failure_count")"
  print_failure_summary "$summary_file"
  printf '[FAIL] live smoke completed with %s failure(s); artifacts kept at %s\n' "$failure_count" "$out_dir" >&2
  exit 1
fi

print_result_table
pass "live smoke completed; artifacts kept at $out_dir"
