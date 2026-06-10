#!/usr/bin/env bash
set -euo pipefail

sdk_dir="${ZSCALERCTL_SDK_DIR:-vendor/github.com/zscaler/zscaler-sdk-go/v3}"
if [[ ! -d "$sdk_dir" ]]; then
  sdk_dir="$(go list -m -f '{{.Dir}}' github.com/zscaler/zscaler-sdk-go/v3)"
fi

oneapi="$sdk_dir/zscaler/oneapiconfig.go"
if [[ ! -f "$oneapi" ]]; then
  echo "missing SDK oneapiconfig.go at $oneapi" >&2
  exit 1
fi

adapter_dir="${ZSCALERCTL_ADAPTER_DIR:-internal/zscaler}"
adapter_files=()
while IFS= read -r -d '' file; do
  adapter_files+=("$file")
done < <(find "$adapter_dir" -name '*.go' ! -name '*_test.go' -print0)

if [[ ${#adapter_files[@]} -eq 0 ]]; then
  echo "missing zscaler adapter files in $adapter_dir" >&2
  exit 1
fi

grep_rc=0
matches="$(grep -nE '\b(NewConfiguration|zsdk\.NewConfiguration|zscaler\.NewConfiguration)\s*\(' "${adapter_files[@]}")" || grep_rc=$?
if (( grep_rc >= 2 )); then
  echo "grep error (exit $grep_rc) scanning adapter files for NewConfiguration" >&2
  exit 1
fi
if (( grep_rc == 0 )) && [[ -n "$matches" ]]; then
  echo "$matches" >&2
  echo "internal/zscaler must not call SDK NewConfiguration; it reads SDK env/config before setters" >&2
  exit 1
fi

if ! grep -nE '\b(NewOneAPIClient|zsdk\.NewOneAPIClient|zscaler\.NewOneAPIClient)\s*\(' "${adapter_files[@]}" >/dev/null; then
  echo "internal/zscaler no longer calls SDK NewOneAPIClient; re-review the SDK boundary" >&2
  exit 1
fi

body="$(
  awk '
    /^func NewOneAPIClient\(/ { in_fn = 1 }
    /^func \(c \*Client\) startTokenRenewalTicker\(/ { exit }
    in_fn { print }
  ' "$oneapi"
)"

if [[ -z "$body" ]]; then
  echo "could not locate SDK NewOneAPIClient body" >&2
  exit 1
fi

if grep -E 'readConfigFrom(Environment|System)|envconfig\.Process|os\.Getenv|GetDefaultLogger|ZSCALER_SDK_|ZSCALER_CLIENT_' <<<"$body" >/dev/null; then
  echo "SDK NewOneAPIClient appears to perform env/file/log discovery; re-review the adapter boundary" >&2
  exit 1
fi

auth_body="$(
  awk '
    /^func \(c \*Client\) authenticate\(/ { in_fn = 1 }
    /^func containsInt\(/ { exit }
    in_fn { print }
  ' "$oneapi"
)"

if [[ -z "$auth_body" ]]; then
  echo "could not locate SDK authenticate body" >&2
  exit 1
fi

if grep -E 'readConfigFrom(Environment|System)|envconfig\.Process|os\.Getenv|GetDefaultLogger|ZSCALER_SDK_|ZSCALER_CLIENT_' <<<"$auth_body" >/dev/null; then
  echo "SDK authenticate appears to perform env/file/log discovery; re-review the adapter boundary" >&2
  exit 1
fi

if [[ "${ZSCALERCTL_SKIP_GO_TEST:-}" != "1" ]]; then
  go test -mod=vendor ./internal/zscaler -run 'TestNewSDKConfigurationDoesNotUseSDKDiscoveryOrLogging|TestNewReaderIgnoresSDKEnvironmentNames'
fi
