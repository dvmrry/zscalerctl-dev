#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

fake_bin="$tmp_dir/zscalerctl"
without_live_creds=(
  env
  -u ZSCALERCTL_AUTH_MODE
  -u ZSCALERCTL_CLIENT_ID
  -u ZSCALERCTL_CLIENT_SECRET
  -u ZSCALERCTL_CLIENT_SECRET_FILE
  -u ZSCALERCTL_VANITY_DOMAIN
  -u ZSCALERCTL_ZIA_USERNAME
  -u ZSCALERCTL_ZIA_PASSWORD
  -u ZSCALERCTL_ZIA_PASSWORD_FILE
  -u ZSCALERCTL_ZIA_API_KEY
  -u ZSCALERCTL_ZIA_API_KEY_FILE
  -u ZSCALERCTL_ZIA_CLOUD
)

cat >"$fake_bin" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

mode="${ZSCALERCTL_FAKE_MODE:-good}"
resources=(gre-tunnels location-groups locations rule-labels static-ips url-filtering-rules)

schema_fields() {
  case "$1" in
    gre-tunnels)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"sourceIp","allowed_modes":["standard"]},{"name":"internalIpRange","allowed_modes":["standard"]},{"name":"comment","allowed_modes":["standard"]},{"name":"withinCountry","allowed_modes":["standard"]}]'
      ;;
    location-groups)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"comments","allowed_modes":["standard"]},{"name":"groupType","allowed_modes":["standard"]},{"name":"predefined","allowed_modes":["standard"]}]'
      ;;
    locations)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"ipAddresses","allowed_modes":["standard"]}]'
      ;;
    rule-labels)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"lastModifiedTime","allowed_modes":["standard"]},{"name":"referencedRuleCount","allowed_modes":["standard"]}]'
      ;;
    static-ips)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"ipAddress","allowed_modes":["standard"]},{"name":"routableIP","allowed_modes":["standard"]},{"name":"comment","allowed_modes":["standard"]}]'
      ;;
    url-filtering-rules)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"locations","allowed_modes":["standard"]}]'
      ;;
    *)
      echo "unexpected resource: $1" >&2
      exit 2
      ;;
  esac
}

write_schema() {
  local resource

  printf '[\n'
  for resource in "${resources[@]}"; do
    if [[ "$resource" != "${resources[0]}" ]]; then
      printf ',\n'
    fi
    printf '  {"product":"zia","name":"%s","operations":[{"name":"list","capability":"read"},{"name":"get","capability":"read"}],"fields":%s}' "$resource" "$(schema_fields "$resource")"
  done
  printf '\n]\n'
}

write_resource() {
  local resource="$1"
  case "$mode:$resource" in
    leaky:locations)
      printf '[{"id":1,"name":"HQ","preSharedKey":"plain-secret"}]\n'
      ;;
    invalid-json:gre-tunnels)
      printf '{"broken":'
      ;;
    *:locations)
      printf '[{"id":1,"name":"HQ","description":"<REDACTED:SECRET>","ipAddresses":["192.0.2.10"]}]\n'
      ;;
    leaky-location-groups:location-groups)
      printf '[{"id":5,"name":"Branch groups","lastModUser":{"id":1,"name":"Admin"},"dynamicLocationGroupCriteria":{"name":{"matchString":"secret branch"}},"locations":[{"id":1,"name":"HQ"}]}]\n'
      ;;
    *:location-groups)
      printf '[{"id":5,"name":"Branch groups","comments":"","groupType":"STATIC_GROUP","predefined":false}]\n'
      ;;
    unexpected-field:rule-labels)
      printf '[{"id":2,"name":"Production","description":"","lastModifiedTime":1632411150,"referencedRuleCount":4,"unexpectedField":"not a value to print"}]\n'
      ;;
    *:rule-labels)
      printf '[{"id":2,"name":"Production","description":"","lastModifiedTime":1632411150,"referencedRuleCount":4}]\n'
      ;;
    *:static-ips)
      printf '[{"id":3,"ipAddress":"198.51.100.10","routableIP":true,"comment":""}]\n'
      ;;
    *:gre-tunnels)
      printf '[{"id":4,"sourceIp":"203.0.113.10","internalIpRange":"10.0.0.0/24","comment":"","withinCountry":true}]\n'
      ;;
    *:url-filtering-rules)
      printf '[{"id":6,"name":"URL rule","locations":[{"id":1,"name":"HQ"}]}]\n'
      ;;
    *)
      echo "unexpected resource: $resource" >&2
      exit 2
      ;;
  esac
}

write_dump() {
  local out=""
  local selected_resources=("${resources[@]}")
  shift
  while (($#)); do
    case "$1" in
      --products)
        shift 2
        ;;
      --resources)
        IFS=',' read -r -a selected_resources <<<"$2"
        shift 2
        ;;
      --out)
        out="$2"
        shift 2
        ;;
      *)
        echo "unexpected dump arg: $1" >&2
        exit 2
        ;;
    esac
  done
  if [[ -z "$out" ]]; then
    echo "missing --out" >&2
    exit 2
  fi

  mkdir -p "$out/resources/zia"
  chmod 700 "$out" "$out/resources" "$out/resources/zia"

  local resource
  for resource in "${selected_resources[@]}"; do
    resource="${resource#zia/}"
    write_resource "$resource" >"$out/resources/zia/$resource.json"
  done

  if [[ "$mode" == "missing-manifest-resource" ]]; then
    cat >"$out/manifest.json" <<'JSON'
{
  "schema": "zscalerctl.dump.manifest.v1",
  "redaction": "standard",
  "warning": "sanitized dumps remain confidential operational data",
  "status": "complete",
  "resources": [
    {"product": "zia", "name": "locations", "status": "complete", "path": "resources/zia/locations.json", "records": 1},
    {"product": "zia", "name": "rule-labels", "status": "complete", "path": "resources/zia/rule-labels.json", "records": 1},
    {"product": "zia", "name": "static-ips", "status": "complete", "path": "resources/zia/static-ips.json", "records": 1}
  ]
}
JSON
  else
    {
      cat <<'JSON'
{
  "schema": "zscalerctl.dump.manifest.v1",
  "redaction": "standard",
  "warning": "sanitized dumps remain confidential operational data",
  "status": "complete",
  "resources": [
JSON
      local first=1
      for resource in "${selected_resources[@]}"; do
        resource="${resource#zia/}"
        if ((first)); then
          first=0
        else
          printf ',\n'
        fi
        printf '    {"product": "zia", "name": "%s", "status": "complete", "path": "resources/zia/%s.json", "records": 1}' "$resource" "$resource"
      done
      cat <<'JSON'

  ]
}
JSON
    } >"$out/manifest.json"
  fi

  cat >"$out/redaction_report.json" <<'JSON'
{
  "schema": "zscalerctl.redaction.report.v1",
  "redaction": "standard",
  "resources": [
    {
      "product": "zia",
      "name": "locations",
      "path": "resources/zia/locations.json",
      "records": 1,
      "included_fields": ["description", "id", "ipAddresses", "name"],
      "dropped_fields": ["vpnCredentials"],
      "redacted_fields": ["description"]
    }
  ]
}
JSON

  chmod 600 "$out"/manifest.json "$out"/redaction_report.json "$out"/resources/zia/*.json
  echo "dump written: $out"
}

if [[ "${1:-}" == "--format" && "${2:-}" == "json" && "${3:-}" == "schema" && "${4:-}" == "list" ]]; then
  write_schema
  exit 0
fi

if [[ "${1:-}" == "--format" ]]; then
  if [[ "${2:-}" != "json" || "${3:-}" != "zia" || "${5:-}" != "list" ]]; then
    echo "unexpected list args: $*" >&2
    exit 2
  fi
  write_resource "$4"
  exit 0
fi

if [[ "${1:-}" == "dump" ]]; then
  write_dump "$@"
  exit 0
fi

echo "unexpected args: $*" >&2
exit 2
SH
chmod +x "$fake_bin"

run_smoke() {
  local mode="$1"
  local out="$tmp_dir/out-$mode"
  local stdout="$tmp_dir/stdout-$mode"
  local stderr="$tmp_dir/stderr-$mode"

  if ZSCALERCTL_BIN="$fake_bin" ZSCALERCTL_FAKE_MODE="$mode" \
    "$repo_root/scripts/live-smoke.sh" --skip-credential-check --out "$out" >"$stdout" 2>"$stderr"; then
    return 0
  fi
  return 1
}

if ! "${without_live_creds[@]}" "$repo_root/scripts/live-smoke.sh" --out "$tmp_dir/out-skip" >"$tmp_dir/stdout-skip" 2>"$tmp_dir/stderr-skip"; then
  echo "live-smoke without credentials did not skip cleanly" >&2
  cat "$tmp_dir/stdout-skip" >&2
  cat "$tmp_dir/stderr-skip" >&2
  exit 1
fi

if ! grep -q '\[SKIP\] no supported live credentials configured' "$tmp_dir/stdout-skip"; then
  echo "live-smoke without credentials did not print SKIP marker" >&2
  cat "$tmp_dir/stdout-skip" >&2
  exit 1
fi

if "${without_live_creds[@]}" "$repo_root/scripts/live-smoke.sh" --require-credentials --out "$tmp_dir/out-require-creds" >"$tmp_dir/stdout-require-creds" 2>"$tmp_dir/stderr-require-creds"; then
  echo "live-smoke --require-credentials accepted missing credentials" >&2
  cat "$tmp_dir/stdout-require-creds" >&2
  cat "$tmp_dir/stderr-require-creds" >&2
  exit 1
fi

if ! grep -q '\[FAIL\] no supported live credentials configured' "$tmp_dir/stderr-require-creds"; then
  echo "live-smoke --require-credentials did not print missing-credentials failure" >&2
  cat "$tmp_dir/stderr-require-creds" >&2
  exit 1
fi

if "$repo_root/scripts/live-smoke.sh" --skip-credential-check --bin "$tmp_dir/missing-zscalerctl" --out "$tmp_dir/out-missing-bin" >"$tmp_dir/stdout-missing-bin" 2>"$tmp_dir/stderr-missing-bin"; then
  echo "live-smoke accepted a missing --bin path" >&2
  cat "$tmp_dir/stdout-missing-bin" >&2
  cat "$tmp_dir/stderr-missing-bin" >&2
  exit 1
fi

if ! grep -q 'zscalerctl binary not found or not executable' "$tmp_dir/stderr-missing-bin"; then
  echo "live-smoke missing-bin failure did not mention the binary path problem" >&2
  cat "$tmp_dir/stderr-missing-bin" >&2
  exit 1
fi

if ! run_smoke good; then
  echo "live-smoke rejected the good fixture" >&2
  cat "$tmp_dir/stdout-good" >&2
  cat "$tmp_dir/stderr-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] live smoke completed' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not print final PASS marker" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] manifest count matches resources/zia/locations.json (1 records)' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not validate manifest counts" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] dump manifest status is complete' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not validate manifest complete status" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] complete dump did not write errors.ndjson' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not validate absence of errors.ndjson" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[INFO\] redaction report zia locations: dropped fields \[vpnCredentials\]' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not summarize dropped fields without record values" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] zia locations list and dump counts match (1 records)' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not compare list and dump counts" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] zia location-groups list contains only catalog-allowed top-level fields' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not validate list catalog subset" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] dump zia location-groups contains only catalog-allowed top-level fields' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not validate dump catalog subset" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] zia url-filtering-rules list contains no denied field keys' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not allow policy-rule locations field" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -F -q '[INFO] zia locations list redaction markers at: [].description' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not summarize list redaction marker paths" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -F -q '[INFO] dump zia locations redaction markers at: [].description' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not summarize dump redaction marker paths" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] dump manifest resource set matches ZIA catalog' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not validate manifest resource set" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] dump resource files match ZIA catalog' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not validate dump file set" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! ZSCALERCTL_BIN="$fake_bin" "$repo_root/scripts/live-smoke.sh" --skip-credential-check --resources zia/locations,rule-labels --out "$tmp_dir/out-subset" >"$tmp_dir/stdout-subset" 2>"$tmp_dir/stderr-subset"; then
  echo "live-smoke rejected a valid resource subset" >&2
  cat "$tmp_dir/stdout-subset" >&2
  cat "$tmp_dir/stderr-subset" >&2
  exit 1
fi

if ! grep -q '\[PASS\] live smoke selected 2 ZIA resource(s): locations rule-labels' "$tmp_dir/stdout-subset"; then
  echo "live-smoke subset fixture did not report selected resources" >&2
  cat "$tmp_dir/stdout-subset" >&2
  exit 1
fi

if grep -q 'zia static-ips list command completed' "$tmp_dir/stdout-subset"; then
  echo "live-smoke subset fixture listed an unselected resource" >&2
  cat "$tmp_dir/stdout-subset" >&2
  exit 1
fi

if ZSCALERCTL_BIN="$fake_bin" "$repo_root/scripts/live-smoke.sh" --skip-credential-check --resources zia/not-real --out "$tmp_dir/out-unknown-resource" >"$tmp_dir/stdout-unknown-resource" 2>"$tmp_dir/stderr-unknown-resource"; then
  echo "live-smoke accepted an unknown requested resource" >&2
  cat "$tmp_dir/stdout-unknown-resource" >&2
  cat "$tmp_dir/stderr-unknown-resource" >&2
  exit 1
fi

if ! grep -q 'requested resource is not a ZIA read/list resource: zia/not-real' "$tmp_dir/stderr-unknown-resource"; then
  echo "live-smoke unknown-resource failure did not mention the requested resource" >&2
  cat "$tmp_dir/stderr-unknown-resource" >&2
  exit 1
fi

if run_smoke leaky; then
  echo "live-smoke accepted a fixture with a denied secret key" >&2
  cat "$tmp_dir/stdout-leaky" >&2
  cat "$tmp_dir/stderr-leaky" >&2
  exit 1
fi

if ! grep -q 'preSharedKey' "$tmp_dir/stderr-leaky"; then
  echo "live-smoke denied-key failure did not mention preSharedKey" >&2
  cat "$tmp_dir/stderr-leaky" >&2
  exit 1
fi

if run_smoke leaky-location-groups; then
  echo "live-smoke accepted a fixture with location-group denied keys" >&2
  cat "$tmp_dir/stdout-leaky-location-groups" >&2
  cat "$tmp_dir/stderr-leaky-location-groups" >&2
  exit 1
fi

if ! grep -Eq 'lastModUser|dynamicLocationGroupCriteria|locations' "$tmp_dir/stderr-leaky-location-groups"; then
  echo "live-smoke location-group denied-key failure did not mention the denied key" >&2
  cat "$tmp_dir/stderr-leaky-location-groups" >&2
  exit 1
fi

if run_smoke unexpected-field; then
  echo "live-smoke accepted a fixture with a non-catalog top-level field" >&2
  cat "$tmp_dir/stdout-unexpected-field" >&2
  cat "$tmp_dir/stderr-unexpected-field" >&2
  exit 1
fi

if ! grep -q 'non-catalog field key(s): unexpectedField' "$tmp_dir/stderr-unexpected-field"; then
  echo "live-smoke non-catalog field failure did not mention unexpectedField" >&2
  cat "$tmp_dir/stderr-unexpected-field" >&2
  exit 1
fi

if run_smoke invalid-json; then
  echo "live-smoke accepted invalid JSON" >&2
  cat "$tmp_dir/stdout-invalid-json" >&2
  cat "$tmp_dir/stderr-invalid-json" >&2
  exit 1
fi

if ! grep -q 'is not a JSON array' "$tmp_dir/stderr-invalid-json"; then
  echo "live-smoke invalid-JSON failure did not mention JSON array validation" >&2
  cat "$tmp_dir/stderr-invalid-json" >&2
  exit 1
fi

if run_smoke missing-manifest-resource; then
  echo "live-smoke accepted a manifest missing a catalog resource" >&2
  cat "$tmp_dir/stdout-missing-manifest-resource" >&2
  cat "$tmp_dir/stderr-missing-manifest-resource" >&2
  exit 1
fi

if ! grep -q 'dump manifest resource set differs from ZIA catalog' "$tmp_dir/stderr-missing-manifest-resource"; then
  echo "live-smoke missing-manifest-resource failure did not mention resource-set drift" >&2
  cat "$tmp_dir/stderr-missing-manifest-resource" >&2
  exit 1
fi
