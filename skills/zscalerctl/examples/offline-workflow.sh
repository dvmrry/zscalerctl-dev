#!/usr/bin/env bash
#
# Exercise the agent workflow with a synthetic CLI. This deliberately never
# loads credentials, config, a tenant, or the network.
set -euo pipefail

command -v jq >/dev/null || {
  echo "offline workflow requires jq" >&2
  exit 1
}

fixture_dir=$(mktemp -d)
trap 'rm -rf "$fixture_dir"' EXIT

# A real binary can be supplied for config-free discovery checks. The list/get
# scenarios below still use the synthetic fixture, so this never needs tenant
# credentials or network access.
if [ -n "${ZSCALERCTL_BIN:-}" ]; then
  real_manifest="$fixture_dir/real-manifest.json"
  real_schema="$fixture_dir/real-schema.json"
  real_introspect="$fixture_dir/real-introspect.json"
  "$ZSCALERCTL_BIN" version >"$fixture_dir/real-version.txt"
  "$ZSCALERCTL_BIN" --format json machine manifest >"$real_manifest"
  jq -e '
    .version == "machine.v1"
    and ([.capabilities[]
          | select(.name == "resources.read"
                   and .input.product == "zia"
                   and .input.resource == "locations")] | length == 1)
  ' "$real_manifest" >/dev/null
  "$ZSCALERCTL_BIN" --format json schema list >"$real_schema"
  jq -e '[.[] | select(.product == "zia" and .name == "locations")] | length == 1' "$real_schema" >/dev/null
  "$ZSCALERCTL_BIN" --format json introspect >"$real_introspect"
  jq -e '
    any(.commands[];
        .path == "zia locations list"
        and ((.inherited_flags // []) | index("limit")) != null
        and ((.inherited_flags // []) | index("offset")) != null)
  ' "$real_introspect" >/dev/null
  echo "validated real binary discovery JSON from $ZSCALERCTL_BIN"
fi

stub="$fixture_dir/zscalerctl"

cat >"$stub" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail

format=json
fields=
redaction=standard
limit=
offset=0
filters=()
positionals=()

while [ "$#" -gt 0 ]; do
  case "$1" in
    --format)
      [ "$#" -ge 2 ] || exit 2
      format=$2
      shift 2
      ;;
    --format=*)
      format=${1#*=}
      shift
      ;;
    --fields)
      [ "$#" -ge 2 ] || exit 2
      fields=$2
      shift 2
      ;;
    --redaction)
      [ "$#" -ge 2 ] || exit 2
      redaction=$2
      shift 2
      ;;
    --filter)
      [ "$#" -ge 2 ] || exit 2
      filters+=("$2")
      shift 2
      ;;
    --timeout|--limit|--offset)
      [ "$#" -ge 2 ] || exit 2
      case "$1" in
        --limit) limit=$2 ;;
        --offset) offset=$2 ;;
      esac
      shift 2
      ;;
    --*)
      echo "unexpected fixture flag: $1" >&2
      exit 2
      ;;
    *)
      positionals+=("$1")
      shift
      ;;
  esac
done

usage_error() {
  jq -cn --arg message "$1" '{error:{kind:"usage",message:$message}}' >&2
  exit 2
}

[ ${#positionals[@]} -gt 0 ] || usage_error "missing command"
command=${positionals[0]}

if [ "$command" = "version" ]; then
  printf '%s\n' 'v0.68.1'
  exit 0
fi

if [ "$command" = "machine" ] && [ ${#positionals[@]} -ge 2 ] &&
   [ "${positionals[1]}" = "manifest" ]; then
  if [ "${OFFLINE_LEGACY:-0}" = "1" ]; then
    jq -cn '{error:{kind:"not_found",message:"unknown command: machine manifest"}}' >&2
    exit 4
  fi
  jq -cn '{
    version:"machine.v1",
    capabilities:[{
      name:"resources.read",
      title:"Read zia/locations",
      description:"Synthetic projected resource read",
      operations:["list","get"],
      input:{product:"zia",resource:"locations"},
      output:{name:"projected-records",version:"1"},
      meta:{product:"zia",resource:"locations",shape:"list",get_key:"id",read_only:true}
    }],
    schemas:[],
    meta:{version:"machine.v1",read_only:true,count:1}
  }'
  exit 0
fi

if [ "$command" = "introspect" ]; then
  jq -cn '{
    introspect_version:2,
    read_only:true,
    commands:[{
      path:"zia locations list",
      inherited_flags:["fields","filter","format","redaction","search","timeout"],
      effects:[{kind:"network_access",when:"always"}],
      output_fields:["id","name","enabled"]
    }]
  }'
  exit 0
fi

if [ "$command" = "schema" ] && [ ${#positionals[@]} -ge 2 ] &&
   [ "${positionals[1]}" = "list" ]; then
  jq -cn '[{
    product:"zia",
    name:"locations",
    operations:[{name:"list",capability:"read"},{name:"get",capability:"read"}],
    get_key:"id",
    fields:[
      {name:"id",classification:"operational_metadata",allowed_modes:["standard","share","paranoid"]},
      {name:"name",classification:"tenant_configuration",allowed_modes:["standard","share"]},
      {name:"enabled",classification:"tenant_configuration",allowed_modes:["standard"]},
      {name:"token",classification:"secret"}
    ]
  }]'
  exit 0
fi

[ "$command" = "zia" ] &&
  [ ${#positionals[@]} -ge 3 ] &&
  [ "${positionals[1]}" = "locations" ] ||
  usage_error "unsupported fixture command"
operation=${positionals[2]}

for filter in "${filters[@]}"; do
  case "$filter" in
    *=*|*~*) key=${filter%%[=~]*} ;;
    *) usage_error "invalid filter syntax" ;;
  esac
  case "$key" in
    id|name|enabled) ;;
    *) usage_error "unknown filter field" ;;
  esac
done

if [ -n "$fields" ]; then
  IFS=',' read -r -a requested_fields <<< "$fields"
  for field in "${requested_fields[@]}"; do
    [ -z "$field" ] && continue
    case "$field" in
      id|name|enabled|token) ;;
      *) usage_error "unknown projected field" ;;
    esac
  done
fi

if [ -n "${OFFLINE_READ_MARKER:-}" ]; then
  printf '%s\n' "$operation" >>"$OFFLINE_READ_MARKER"
fi

case "$operation" in
  list)
    if [ -n "$limit" ]; then
      jq -cn --argjson offset "$offset" --argjson limit "$limit" '{
        records:[{id:42,name:"HQ"}],
        pagination:{
          offset:$offset,limit:$limit,returned_count:1,matched_count:1,
          has_more:false,next_offset:null,collection_complete:true
        }
      }'
    elif [ ${#filters[@]} -gt 0 ]; then
      jq -cn '[{id:42,name:"HQ"}]'
    else
      jq -cn '[range(0;2500) | {id:(. + 1),name:("record-" + ((. + 1)|tostring))}]'
    fi
    ;;
  get)
    [ ${#positionals[@]} -ge 4 ] || usage_error "missing id"
    [ "${positionals[3]}" = "42" ] || {
      jq -cn '{error:{kind:"not_found",message:"synthetic id not found"}}' >&2
      exit 4
    }
    jq -cn '{id:42,name:"HQ"}'
    ;;
  *)
    usage_error "unsupported synthetic operation"
    ;;
esac
STUB
chmod +x "$stub"
export PATH="$fixture_dir:$PATH"
export OFFLINE_READ_MARKER="$fixture_dir/reads"

PRODUCT=zia
RESOURCE=locations
MODE=share
FIELDS=id,name
manifest="$fixture_dir/manifest.json"
schema="$fixture_dir/schema.json"

zscalerctl --format json machine manifest >"$manifest"
jq -e --arg product "$PRODUCT" --arg resource "$RESOURCE" '
  [ .capabilities[]
    | select(.name == "resources.read"
             and .input.product == $product
             and .input.resource == $resource) ] | length == 1
' "$manifest" >/dev/null

zscalerctl --format json schema list >"$schema"
jq -e --arg product "$PRODUCT" --arg resource "$RESOURCE" --arg mode "$MODE" --arg requested "$FIELDS" '
  [ .[] | select(.product == $product and .name == $resource) ] as $matches
  | $matches | length == 1
' "$schema" >/dev/null
jq -e --arg product "$PRODUCT" --arg resource "$RESOURCE" --arg mode "$MODE" '
  [ .[] | select(.product == $product and .name == $resource) ][0].fields
  | map(select(((.allowed_modes // []) | index($mode)) != null)
        | (.json_name // .name))
  | (index("id") != null and index("name") != null and index("enabled") == null)
' "$schema" >/dev/null

for requested in 'id,name' 'id, name'; do
  jq -e --arg product "$PRODUCT" --arg resource "$RESOURCE" --arg mode "$MODE" --arg requested "$requested" '
    [ .[] | select(.product == $product and .name == $resource) ][0] as $spec
    | [ $spec.fields[]
        | select(.classification != "secret")
        | select(((.allowed_modes // []) | index($mode)) != null)
        | (.json_name // .name) ] as $renderable
    | ($requested | split(",") | map(gsub("^\\s+|\\s+$"; "")) | map(select(length > 0))) as $wanted
    | $wanted == ["id", "name"] and (($wanted - $renderable) | length == 0)
  ' "$schema" >/dev/null
done

page="$fixture_dir/page.json"
zscalerctl --format json --timeout 30s --redaction "$MODE" --fields "$FIELDS" --filter 'name~hq' --limit 20 --offset 0 "$PRODUCT" "$RESOURCE" list >"$page"
jq -e '
  (.records | length) == 1
  and .records[0].id == 42
  and (.records[0] | has("enabled") | not)
  and .pagination.offset == 0
  and .pagination.limit == 20
  and .pagination.returned_count == 1
  and .pagination.matched_count == 1
  and .pagination.has_more == false
  and .pagination.next_offset == null
  and .pagination.collection_complete == true
' "$page" >/dev/null

record_id=$(jq -er '.records[0].id' "$page")
get_output="$fixture_dir/get.json"
zscalerctl --format json --timeout 30s "$PRODUCT" "$RESOURCE" get "$record_id" >"$get_output"
jq -e '.id == 42 and .name == "HQ"' "$get_output" >/dev/null

all="$fixture_dir/all.json"
zscalerctl --format json --timeout 30s --fields id,name "$PRODUCT" "$RESOURCE" list >"$all"
jq -e 'length == 2500 and .[0].id == 1 and .[2499].id == 2500' "$all" >/dev/null
jq -e '{matched_count:length,sample:.[:20],sample_complete:(length <= 20)}
  | .matched_count == 2500 and (.sample | length) == 20
    and .sample_complete == false' "$all" >/dev/null

read_count() {
  if [ -f "$OFFLINE_READ_MARKER" ]; then
    wc -l <"$OFFLINE_READ_MARKER"
  else
    printf '%s\n' 0
  fi
}

before=$(read_count)
status=0
zscalerctl --format json --fields typo "$PRODUCT" "$RESOURCE" list >/dev/null 2>"$fixture_dir/unknown-field.err" || status=$?
[ "$status" -eq 2 ] && [ "$(read_count)" = "$before" ]

status=0
zscalerctl --format json --filter typo=HQ "$PRODUCT" "$RESOURCE" list >/dev/null 2>"$fixture_dir/unknown-filter.err" || status=$?
[ "$status" -eq 2 ] && [ "$(read_count)" = "$before" ]

set +e
zscalerctl --format json --filter typo=HQ "$PRODUCT" "$RESOURCE" list 2>"$fixture_dir/pipeline.err" |
  jq -e '.' >"$fixture_dir/pipeline.out"
pipeline_status=$?
set -e
[ "$pipeline_status" -ne 0 ]

export OFFLINE_LEGACY=1
if zscalerctl --format json machine manifest >"$fixture_dir/legacy.out" 2>"$fixture_dir/legacy.err"; then
  echo "legacy fixture unexpectedly advertised machine manifest" >&2
  exit 1
fi
zscalerctl --format json introspect |
  jq -e '.commands[] | select(.path == "zia locations list")
    | ((.inherited_flags | index("limit")) == null
       and (.inherited_flags | index("offset")) == null)' >/dev/null
unset OFFLINE_LEGACY

echo "offline zscalerctl workflow checks passed"
