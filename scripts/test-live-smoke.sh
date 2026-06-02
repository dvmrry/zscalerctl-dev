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
  -u ZSCALERCTL_ZPA_CUSTOMER_ID
  -u ZSCALERCTL_ZPA_MICROTENANT_ID
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
resources=(zia/advanced-settings zia/atp-malware-policy zia/gre-tunnels zia/location-groups zia/locations zia/mobile-threat-settings zia/org-information zia/rule-labels zia/static-ips zia/url-filtering-rules zpa/server-groups zpa/app-connectors zpa/service-edge-groups zpa/service-edges zpa/cloud-connector-groups zpa/cloud-connectors zpa/posture-profiles zpa/cbi-zpa-profiles zpa/c2c-ip-ranges zpa/private-cloud-groups zpa/config-overrides zpa/private-cloud-controllers)

schema_fields() {
  case "$1" in
    advanced-settings)
      printf '[{"name":"apiSessionTimeout","allowed_modes":["standard"]},{"name":"authBypassUrls","allowed_modes":["standard"]}]'
      ;;
    atp-malware-policy)
      printf '[{"name":"blockPasswordProtectedArchiveFiles","allowed_modes":["standard"]},{"name":"blockUnscannableFiles","allowed_modes":["standard"]}]'
      ;;
    gre-tunnels)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"sourceIp","allowed_modes":["standard"]},{"name":"internalIpRange","allowed_modes":["standard"]},{"name":"comment","allowed_modes":["standard"]},{"name":"withinCountry","allowed_modes":["standard"]}]'
      ;;
    location-groups)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"comments","allowed_modes":["standard"]},{"name":"groupType","allowed_modes":["standard"]},{"name":"predefined","allowed_modes":["standard"]}]'
      ;;
    locations)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"ipAddresses","allowed_modes":["standard"]}]'
      ;;
    mobile-threat-settings)
      printf '[{"name":"blockAppsSendingUnencryptedUserCredentials","allowed_modes":["standard"]},{"name":"blockAppsSendingDeviceIdentifier","allowed_modes":["standard"]}]'
      ;;
    org-information)
      printf '[{"name":"name","allowed_modes":["standard"]},{"name":"city","allowed_modes":["standard"]}]'
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
    server-groups)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]}]'
      ;;
    app-connectors)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"location","allowed_modes":["standard"]},{"name":"assistantVersion","allowed_modes":[]}]'
      ;;
    service-edge-groups)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"serviceEdges","allowed_modes":[]}]'
      ;;
    service-edges)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"provisioningKeyName","allowed_modes":[]}]'
      ;;
    cloud-connector-groups)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"cloudConnectors","allowed_modes":[]},{"name":"geoLocationId","allowed_modes":[]}]'
      ;;
    cloud-connectors)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"edgeConnectorGroupName","allowed_modes":["standard"]},{"name":"enrollmentCert","allowed_modes":[]},{"name":"fingerprint","allowed_modes":[]}]'
      ;;
    posture-profiles)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"domain","allowed_modes":["standard"]},{"name":"postureType","allowed_modes":["standard"]},{"name":"rootCert","allowed_modes":[]}]'
      ;;
    cbi-zpa-profiles)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"cbiProfileId","allowed_modes":["standard"]},{"name":"cbiTenantId","allowed_modes":[]}]'
      ;;
    c2c-ip-ranges)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"subnetCidr","allowed_modes":["standard"]},{"name":"customerId","allowed_modes":[]}]'
      ;;
    private-cloud-groups)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"location","allowed_modes":["standard"]},{"name":"microtenantId","allowed_modes":[]}]'
      ;;
    config-overrides)
      printf '[{"name":"brokerName","allowed_modes":["standard"]},{"name":"customerName","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"targetName","allowed_modes":["standard"]},{"name":"targetType","allowed_modes":["standard"]},{"name":"configValue","allowed_modes":[]}]'
      ;;
    private-cloud-controllers)
      printf '[{"name":"id","allowed_modes":["standard"]},{"name":"name","allowed_modes":["standard"]},{"name":"description","allowed_modes":["standard"]},{"name":"enabled","allowed_modes":["standard"]},{"name":"location","allowed_modes":["standard"]},{"name":"enrollmentCert","allowed_modes":[]}]'
      ;;
    *)
      echo "unexpected resource: $1" >&2
      exit 2
      ;;
  esac
}

schema_operations() {
  case "$1" in
    advanced-settings|atp-malware-policy|mobile-threat-settings|org-information)
      printf '[{"name":"show","capability":"read"}]'
      ;;
    cloud-connectors|config-overrides)
      printf '[{"name":"list","capability":"read"}]'
      ;;
    *)
      printf '[{"name":"list","capability":"read"},{"name":"get","capability":"read"}]'
      ;;
  esac
}

write_schema() {
  local product
  local resource

  printf '[\n'
  for resource in "${resources[@]}"; do
    if [[ "$resource" != "${resources[0]}" ]]; then
      printf ',\n'
    fi
    product="${resource%%/*}"
    resource="${resource#*/}"
    printf '  {"product":"%s","name":"%s","operations":%s,"fields":%s}' "$product" "$resource" "$(schema_operations "$resource")" "$(schema_fields "$resource")"
  done
  printf '\n]\n'
}

write_resource() {
  local product="$1"
  local resource="$2"
  case "$mode:$product:$resource" in
    empty-object:zia:advanced-settings)
      printf '{}\n'
      ;;
    *:zia:advanced-settings)
      printf '{"apiSessionTimeout":30,"authBypassUrls":["admin.internal.example"]}\n'
      ;;
    *:zia:atp-malware-policy)
      printf '{"blockPasswordProtectedArchiveFiles":true,"blockUnscannableFiles":false}\n'
      ;;
    leaky-settings:zia:mobile-threat-settings)
      printf '{"blockAppsSendingUnencryptedUserCredentials":true,"clientCredential":"should-fail"}\n'
      ;;
    *:zia:mobile-threat-settings)
      printf '{"blockAppsSendingUnencryptedUserCredentials":true,"blockAppsSendingDeviceIdentifier":false}\n'
      ;;
    *:zia:org-information)
      printf '{"name":"Example tenant","city":"New York"}\n'
      ;;
    leaky:zia:locations)
      printf '[{"id":1,"name":"HQ","preSharedKey":"plain-secret"}]\n'
      ;;
    invalid-json:zia:gre-tunnels)
      printf '{"broken":'
      ;;
    list-fails:zia:gre-tunnels)
      echo "mock API 404 not entitled" >&2
      exit 7
      ;;
    json-list-fails:zia:gre-tunnels)
      cat >&2 <<'JSON'
{
  "error": {
    "kind": "live_access_failed",
    "message": "zscaler API request failed: list zia/gre-tunnels"
  }
}
JSON
      exit 7
      ;;
    *:zia:locations)
      printf '[{"id":1,"name":"HQ","description":"<REDACTED:SECRET>","ipAddresses":["192.0.2.10"]}]\n'
      ;;
    leaky-location-groups:zia:location-groups)
      printf '[{"id":5,"name":"Branch groups","lastModUser":{"id":1,"name":"Admin"},"dynamicLocationGroupCriteria":{"name":{"matchString":"secret branch"}},"locations":[{"id":1,"name":"HQ"}]}]\n'
      ;;
    *:zia:location-groups)
      printf '[{"id":5,"name":"Branch groups","comments":"","groupType":"STATIC_GROUP","predefined":false}]\n'
      ;;
    unexpected-field:zia:rule-labels)
      printf '[{"id":2,"name":"Production","description":"","lastModifiedTime":1632411150,"referencedRuleCount":4,"unexpectedField":"not a value to print"}]\n'
      ;;
    *:zia:rule-labels)
      printf '[{"id":2,"name":"Production","description":"","lastModifiedTime":1632411150,"referencedRuleCount":4}]\n'
      ;;
    *:zia:static-ips)
      printf '[{"id":3,"ipAddress":"198.51.100.10","routableIP":true,"comment":""}]\n'
      ;;
    *:zia:gre-tunnels)
      printf '[{"id":4,"sourceIp":"203.0.113.10","internalIpRange":"10.0.0.0/24","comment":"","withinCountry":true}]\n'
      ;;
    *:zia:url-filtering-rules)
      printf '[{"id":6,"name":"URL rule","locations":[{"id":1,"name":"HQ"}]}]\n'
      ;;
    *:zpa:server-groups)
      printf '[{"id":"sg-1","name":"Server group","description":"","enabled":true}]\n'
      ;;
    *:zpa:app-connectors)
      printf '[{"id":"app-connector-1","name":"App connector","description":"","enabled":true,"location":"San Jose, CA"}]\n'
      ;;
    *:zpa:service-edge-groups)
      printf '[{"id":"seg-1","name":"Service edge group","description":"","enabled":true}]\n'
      ;;
    *:zpa:service-edges)
      printf '[{"id":"se-1","name":"Service edge","description":"","enabled":true}]\n'
      ;;
    *:zpa:cloud-connector-groups)
      printf '[{"id":"ccg-1","name":"Cloud connector group","description":"","enabled":true}]\n'
      ;;
    *:zpa:cloud-connectors)
      printf '[{"id":"cloud-connector-1","name":"Cloud connector","description":"","enabled":true,"edgeConnectorGroupName":"Cloud connector group"}]\n'
      ;;
    *:zpa:posture-profiles)
      printf '[{"id":"posture-1","name":"Posture profile","domain":"example.internal","postureType":"cert"}]\n'
      ;;
    *:zpa:cbi-zpa-profiles)
      printf '[{"id":"cbi-zpa-profile-1","name":"CBI ZPA profile","description":"","enabled":true,"cbiProfileId":"cbi-profile-1"}]\n'
      ;;
    *:zpa:c2c-ip-ranges)
      printf '[{"id":"c2c-ip-range-1","name":"C2C IP range","description":"","enabled":true,"subnetCidr":"198.51.100.0/24"}]\n'
      ;;
    *:zpa:private-cloud-groups)
      printf '[{"id":"private-cloud-group-1","name":"Private cloud group","description":"","enabled":true,"location":"San Jose, CA"}]\n'
      ;;
    *:zpa:config-overrides)
      printf '[{"brokerName":"Broker","customerName":"Customer","description":"","targetName":"Target","targetType":"BROKER"}]\n'
      ;;
    *:zpa:private-cloud-controllers)
      printf '[{"id":"private-cloud-controller-1","name":"Private cloud controller","description":"","enabled":true,"location":"San Jose, CA"}]\n'
      ;;
    *)
      echo "unexpected resource: $resource" >&2
      exit 2
      ;;
  esac
}

write_dump() {
  local out=""
  local selected_products=(zia)
  local selected_resources=("${resources[@]}")
  local explicit_resources=0
  shift
  while (($#)); do
    case "$1" in
      --products)
        IFS=',' read -r -a selected_products <<<"$2"
        shift 2
        ;;
      --resources)
        IFS=',' read -r -a selected_resources <<<"$2"
        explicit_resources=1
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
  if ((explicit_resources == 0)); then
    selected_resources=()
    local candidate
    for candidate in "${resources[@]}"; do
      if product_in_list "${candidate%%/*}" "${selected_products[@]}"; then
        selected_resources+=("$candidate")
      fi
    done
  fi
  if [[ -z "$out" ]]; then
    echo "missing --out" >&2
    exit 2
  fi

  mkdir -p "$out/resources"
  chmod 700 "$out" "$out/resources"

  local product
  local resource
  for resource in "${selected_resources[@]}"; do
    product="${resource%%/*}"
    resource="${resource#*/}"
    mkdir -p "$out/resources/$product"
    chmod 700 "$out/resources/$product"
    write_resource "$product" "$resource" >"$out/resources/$product/$resource.json"
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
        product="${resource%%/*}"
        resource="${resource#*/}"
        if ((first)); then
          first=0
        else
          printf ',\n'
        fi
        printf '    {"product": "%s", "name": "%s", "status": "complete", "path": "resources/%s/%s.json", "records": 1}' "$product" "$resource" "$product" "$resource"
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

  find "$out/resources" -type f -name '*.json' -exec chmod 600 {} +
  chmod 600 "$out"/manifest.json "$out"/redaction_report.json
  echo "dump written: $out"
}

product_in_list() {
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

if [[ "${1:-}" == "--format" && "${2:-}" == "json" && "${3:-}" == "schema" && "${4:-}" == "list" ]]; then
  write_schema
  exit 0
fi

if [[ "${1:-}" == "--format" ]]; then
  if [[ "${2:-}" != "json" || ("${5:-}" != "list" && "${5:-}" != "show") ]]; then
    echo "unexpected resource args: $*" >&2
    exit 2
  fi
  write_resource "$3" "$4"
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
    "$repo_root/scripts/live-smoke.sh" --skip-credential-check --no-manifest --out "$out" >"$stdout" 2>"$stderr"; then
    return 0
  fi
  return 1
}

if ! "${without_live_creds[@]}" "$repo_root/scripts/live-smoke.sh" --no-manifest --out "$tmp_dir/out-skip" >"$tmp_dir/stdout-skip" 2>"$tmp_dir/stderr-skip"; then
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

if "${without_live_creds[@]}" "$repo_root/scripts/live-smoke.sh" --no-manifest --require-credentials --out "$tmp_dir/out-require-creds" >"$tmp_dir/stdout-require-creds" 2>"$tmp_dir/stderr-require-creds"; then
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

if env \
  -u ZSCALERCTL_AUTH_MODE \
  -u ZSCALERCTL_ZPA_CUSTOMER_ID \
  -u ZSCALERCTL_ZPA_MICROTENANT_ID \
  ZSCALERCTL_CLIENT_ID=client-id \
  ZSCALERCTL_CLIENT_SECRET=client-secret \
  ZSCALERCTL_VANITY_DOMAIN=vanity \
  ZSCALERCTL_BIN="$fake_bin" \
  "$repo_root/scripts/live-smoke.sh" --no-manifest --require-credentials --resources zpa/server-groups --out "$tmp_dir/out-zpa-missing-customer" >"$tmp_dir/stdout-zpa-missing-customer" 2>"$tmp_dir/stderr-zpa-missing-customer"; then
  echo "live-smoke accepted selected ZPA resources without ZPA customer ID" >&2
  cat "$tmp_dir/stdout-zpa-missing-customer" >&2
  cat "$tmp_dir/stderr-zpa-missing-customer" >&2
  exit 1
fi

if ! grep -q '\[FAIL\] selected ZPA resources require ZSCALERCTL_ZPA_CUSTOMER_ID' "$tmp_dir/stderr-zpa-missing-customer"; then
  echo "live-smoke missing-customer failure did not mention ZSCALERCTL_ZPA_CUSTOMER_ID" >&2
  cat "$tmp_dir/stderr-zpa-missing-customer" >&2
  exit 1
fi

if "$repo_root/scripts/live-smoke.sh" --skip-credential-check --no-manifest --bin "$tmp_dir/missing-zscalerctl" --out "$tmp_dir/out-missing-bin" >"$tmp_dir/stdout-missing-bin" 2>"$tmp_dir/stderr-missing-bin"; then
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

if ! grep -q 'live smoke results' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not print the result table heading" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -Eq 'RESOURCE[[:space:]]+PHASE[[:space:]]+STATUS[[:space:]]+RECORDS[[:space:]]+NOTE' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not print the result table header" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -Eq 'zia/locations[[:space:]]+list[[:space:]]+PASS[[:space:]]+1' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not summarize the locations list row" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -Eq 'zia/locations[[:space:]]+dump[[:space:]]+PASS[[:space:]]+1' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not summarize the locations dump row" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -Eq 'dump[[:space:]]+manifest[[:space:]]+PASS' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not summarize the dump manifest row" >&2
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

if ! grep -q '\[PASS\] zia/locations list and dump counts match (1 records)' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not compare list and dump counts" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] zia/advanced-settings show and dump counts match (1 records)' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not compare show and dump counts" >&2
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

if ! grep -q '\[PASS\] zia mobile-threat-settings show contains no denied field keys' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not allow reviewed mobile threat credential-control field" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] zia org-information show contains no denied field keys' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not allow reviewed org-information city field" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] zia atp-malware-policy show contains no denied field keys' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not allow reviewed ATP password-control field" >&2
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

if ! grep -q '\[PASS\] dump manifest resource set matches selected resources' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not validate manifest resource set" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! grep -q '\[PASS\] dump resource files match selected resources' "$tmp_dir/stdout-good"; then
  echo "live-smoke good fixture did not validate dump file set" >&2
  cat "$tmp_dir/stdout-good" >&2
  exit 1
fi

if ! ZSCALERCTL_BIN="$fake_bin" "$repo_root/scripts/live-smoke.sh" --skip-credential-check --no-manifest --resources zia/locations,rule-labels --out "$tmp_dir/out-subset" >"$tmp_dir/stdout-subset" 2>"$tmp_dir/stderr-subset"; then
  echo "live-smoke rejected a valid resource subset" >&2
  cat "$tmp_dir/stdout-subset" >&2
  cat "$tmp_dir/stderr-subset" >&2
  exit 1
fi

if ! grep -q '\[PASS\] live smoke selected 2 resource(s): zia/locations zia/rule-labels' "$tmp_dir/stdout-subset"; then
  echo "live-smoke subset fixture did not report selected resources" >&2
  cat "$tmp_dir/stdout-subset" >&2
  exit 1
fi

if grep -q 'zia static-ips list command completed' "$tmp_dir/stdout-subset"; then
  echo "live-smoke subset fixture listed an unselected resource" >&2
  cat "$tmp_dir/stdout-subset" >&2
  exit 1
fi

manifest="$tmp_dir/live-smoke.manifest"
cat >"$manifest" <<'EOF'
# focused branch smoke
- zia/locations
* rule-labels
EOF

if ! ZSCALERCTL_BIN="$fake_bin" "$repo_root/scripts/live-smoke.sh" --skip-credential-check --manifest "$manifest" --out "$tmp_dir/out-manifest" >"$tmp_dir/stdout-manifest" 2>"$tmp_dir/stderr-manifest"; then
  echo "live-smoke rejected a valid manifest resource subset" >&2
  cat "$tmp_dir/stdout-manifest" >&2
  cat "$tmp_dir/stderr-manifest" >&2
  exit 1
fi

if ! grep -q '\[INFO\] using live smoke manifest:' "$tmp_dir/stdout-manifest"; then
  echo "live-smoke manifest fixture did not report manifest usage" >&2
  cat "$tmp_dir/stdout-manifest" >&2
  exit 1
fi

if ! grep -q '\[PASS\] live smoke selected 2 resource(s): zia/locations zia/rule-labels' "$tmp_dir/stdout-manifest"; then
  echo "live-smoke manifest fixture did not report selected resources" >&2
  cat "$tmp_dir/stdout-manifest" >&2
  exit 1
fi

if grep -q 'zia static-ips list command completed' "$tmp_dir/stdout-manifest"; then
  echo "live-smoke manifest fixture listed an unselected resource" >&2
  cat "$tmp_dir/stdout-manifest" >&2
  exit 1
fi

zpa_manifest="$tmp_dir/zpa-live-smoke.manifest"
cat >"$zpa_manifest" <<'EOF'
zpa/server-groups
EOF

if ! ZSCALERCTL_BIN="$fake_bin" "$repo_root/scripts/live-smoke.sh" --skip-credential-check --manifest "$zpa_manifest" --out "$tmp_dir/out-zpa-manifest" >"$tmp_dir/stdout-zpa-manifest" 2>"$tmp_dir/stderr-zpa-manifest"; then
  echo "live-smoke rejected a valid ZPA manifest resource subset" >&2
  cat "$tmp_dir/stdout-zpa-manifest" >&2
  cat "$tmp_dir/stderr-zpa-manifest" >&2
  exit 1
fi

if ! grep -q '\[PASS\] live smoke selected 1 resource(s): zpa/server-groups' "$tmp_dir/stdout-zpa-manifest"; then
  echo "live-smoke ZPA manifest fixture did not report selected resources" >&2
  cat "$tmp_dir/stdout-zpa-manifest" >&2
  exit 1
fi

if ! grep -q '\[PASS\] zpa server-groups list command completed' "$tmp_dir/stdout-zpa-manifest"; then
  echo "live-smoke ZPA manifest fixture did not run the ZPA list command" >&2
  cat "$tmp_dir/stdout-zpa-manifest" >&2
  exit 1
fi

if ! grep -q '\[PASS\] manifest count matches resources/zpa/server-groups.json (1 records)' "$tmp_dir/stdout-zpa-manifest"; then
  echo "live-smoke ZPA manifest fixture did not validate ZPA manifest counts" >&2
  cat "$tmp_dir/stdout-zpa-manifest" >&2
  exit 1
fi

if ZSCALERCTL_BIN="$fake_bin" "$repo_root/scripts/live-smoke.sh" --skip-credential-check --manifest "$tmp_dir/missing.manifest" --out "$tmp_dir/out-missing-manifest" >"$tmp_dir/stdout-missing-manifest" 2>"$tmp_dir/stderr-missing-manifest"; then
  echo "live-smoke accepted a missing manifest path" >&2
  cat "$tmp_dir/stdout-missing-manifest" >&2
  cat "$tmp_dir/stderr-missing-manifest" >&2
  exit 1
fi

if ! grep -q 'live smoke manifest not found' "$tmp_dir/stderr-missing-manifest"; then
  echo "live-smoke missing-manifest failure did not mention the manifest path problem" >&2
  cat "$tmp_dir/stderr-missing-manifest" >&2
  exit 1
fi

if ZSCALERCTL_BIN="$fake_bin" "$repo_root/scripts/live-smoke.sh" --skip-credential-check --no-manifest --resources zia/not-real --out "$tmp_dir/out-unknown-resource" >"$tmp_dir/stdout-unknown-resource" 2>"$tmp_dir/stderr-unknown-resource"; then
  echo "live-smoke accepted an unknown requested resource" >&2
  cat "$tmp_dir/stdout-unknown-resource" >&2
  cat "$tmp_dir/stderr-unknown-resource" >&2
  exit 1
fi

if ! grep -q 'requested resource is not a read resource: zia/not-real' "$tmp_dir/stderr-unknown-resource"; then
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

if ! grep -q 'failure summary:' "$tmp_dir/stderr-leaky"; then
  echo "live-smoke denied-key failure did not print a failure summary path" >&2
  cat "$tmp_dir/stderr-leaky" >&2
  exit 1
fi

if ! grep -q 'failure markers:' "$tmp_dir/stderr-leaky"; then
  echo "live-smoke denied-key failure summary did not include failure markers" >&2
  cat "$tmp_dir/stderr-leaky" >&2
  exit 1
fi

if run_smoke leaky-settings; then
  echo "live-smoke accepted an unreviewed credential-shaped settings key" >&2
  cat "$tmp_dir/stdout-leaky-settings" >&2
  cat "$tmp_dir/stderr-leaky-settings" >&2
  exit 1
fi

if ! grep -q 'clientCredential' "$tmp_dir/stderr-leaky-settings"; then
  echo "live-smoke settings denied-key failure did not mention clientCredential" >&2
  cat "$tmp_dir/stderr-leaky-settings" >&2
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

if run_smoke list-fails; then
  echo "live-smoke accepted a resource command failure" >&2
  cat "$tmp_dir/stdout-list-fails" >&2
  cat "$tmp_dir/stderr-list-fails" >&2
  exit 1
fi

if ! grep -q 'zia gre-tunnels list command failed' "$tmp_dir/stderr-list-fails"; then
  echo "live-smoke command failure summary did not include the failed resource" >&2
  cat "$tmp_dir/stderr-list-fails" >&2
  exit 1
fi

if ! grep -q 'mock API 404 not entitled' "$tmp_dir/stderr-list-fails"; then
  echo "live-smoke command failure summary did not include the stderr snippet" >&2
  cat "$tmp_dir/stderr-list-fails" >&2
  exit 1
fi

if ! grep -Eq 'zia/gre-tunnels[[:space:]]+list[[:space:]]+FAIL' "$tmp_dir/stderr-list-fails"; then
  echo "live-smoke command failure did not print a failed table row" >&2
  cat "$tmp_dir/stderr-list-fails" >&2
  exit 1
fi

if ! grep -q 'failure-summary.txt' "$tmp_dir/stderr-list-fails"; then
  echo "live-smoke command failure did not print the failure-summary.txt path" >&2
  cat "$tmp_dir/stderr-list-fails" >&2
  exit 1
fi

if run_smoke json-list-fails; then
  echo "live-smoke accepted a structured JSON resource command failure" >&2
  cat "$tmp_dir/stdout-json-list-fails" >&2
  cat "$tmp_dir/stderr-json-list-fails" >&2
  exit 1
fi

if ! grep -q 'error: live_access_failed - zscaler API request failed: list zia/gre-tunnels' "$tmp_dir/stderr-json-list-fails"; then
  echo "live-smoke structured command failure did not compact the JSON stderr snippet" >&2
  cat "$tmp_dir/stderr-json-list-fails" >&2
  exit 1
fi

if grep -q '"error"' "$tmp_dir/stderr-json-list-fails"; then
  echo "live-smoke structured command failure printed raw JSON in the stderr snippet" >&2
  cat "$tmp_dir/stderr-json-list-fails" >&2
  exit 1
fi

if run_smoke empty-object; then
  echo "live-smoke accepted an empty singleton object" >&2
  cat "$tmp_dir/stdout-empty-object" >&2
  cat "$tmp_dir/stderr-empty-object" >&2
  exit 1
fi

if ! grep -q 'returned an empty JSON object' "$tmp_dir/stderr-empty-object"; then
  echo "live-smoke empty-object failure did not mention empty JSON object validation" >&2
  cat "$tmp_dir/stderr-empty-object" >&2
  exit 1
fi

if run_smoke missing-manifest-resource; then
  echo "live-smoke accepted a manifest missing a catalog resource" >&2
  cat "$tmp_dir/stdout-missing-manifest-resource" >&2
  cat "$tmp_dir/stderr-missing-manifest-resource" >&2
  exit 1
fi

if ! grep -q 'dump manifest resource set differs from selected resources' "$tmp_dir/stderr-missing-manifest-resource"; then
  echo "live-smoke missing-manifest-resource failure did not mention resource-set drift" >&2
  cat "$tmp_dir/stderr-missing-manifest-resource" >&2
  exit 1
fi
