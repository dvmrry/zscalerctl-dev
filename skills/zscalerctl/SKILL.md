---
name: zscalerctl
description: Use when asked about Zscaler tenant configuration, inventory, or audit — ZIA/ZPA/ZTW/ZCC/Zidentity locations, rules, policies, app segments, connectors, groups — or to export Zscaler config, when `zscalerctl` is available or should be checked.
---

# zscalerctl

Use `zscalerctl` for authorized, tenant-read-only Zscaler inventory and
configuration reads. The allow-list projection and redaction layers are the
output boundary: a field absent from machine output is intentionally
unavailable, and a read names any field the active mode omits on stderr.

## Quick start

```sh
zscalerctl zia --help                                    # resources and operations for a product
zscalerctl --format json schema list | jq '.[] | select(.product=="zia" and .name=="users")'
zscalerctl --format json --redaction standard --count-by groups.id,department.id zia users list
zscalerctl --format json --redaction standard --filter groups.id=<id> zia url-filtering-rules list
zscalerctl --format json schema references zia groups   # every field that points at a group
zscalerctl --format json schema gaps zpa                 # what zscalerctl does not read
```

The default mode is `share`: a rule's user, group, department and location
references render there as ids only (`[{"id": ...}]`), so rule scope is
visible; names, emails and other identifying values need `--redaction
standard`, and a usage error names the mode when you ask for an omitted field. `--count-by` takes several comma-separated paths at once, so
"members per group, and how many have no department" is one call. Exit codes:
0 success, 1 internal or canceled, 2 usage or unusable setting, 3 missing
credentials (credentials the API rejects are 5), 4 not found or unsupported (a resource zscalerctl does not read names its
gap), 5 live API failure (with `reason`), 6 partial dump, 7 drift.

## Version and capability preflight

Pair the installed binary with the installed copy of this skill. Start with
`zscalerctl version`; then discover the resource and operation instead of
guessing a product or resource name. The development workflow uses
`machine manifest` and bounded JSON list pages. A public `v0.68.1` binary
may predate `machine manifest` and `--limit/--offset`, so do not assume
those commands or flags exist: if the manifest is unsupported, fall back to
`introspect` and `schema list`, and use only the flags those surfaces
advertise. Updating this checkout's skill does not update an already-installed
binary or publish a release.

If the CLI is missing, ask the operator to install it. Stop on failed
discovery or field validation before attempting a live read. Run the shell
recipes in order with `set -euo pipefail` so failure stops later commands.

The manifest and catalog are large; select one resource with `jq` rather than
printing them into a prompt:

```sh
set -euo pipefail
PRODUCT=zia
RESOURCE=locations
MODE=standard
FIELDS=id,name

MANIFEST=$(mktemp)
trap 'rm -f "$MANIFEST"' EXIT
if zscalerctl --format json machine manifest >"$MANIFEST"; then
  jq -e -c --arg product "$PRODUCT" --arg resource "$RESOURCE" '
    [ .capabilities[]
      | select(.name == "resources.read"
               and .input.product == $product
               and .input.resource == $resource) ] as $matches
    | if ($matches | length) != 1
      then error("resource is not advertised by this binary")
      else $matches[0]
        | {resource: .input.resource, operations,
           shape: .meta.shape, get_key: (.meta.get_key // null)}
      end
  ' "$MANIFEST"
else
  manifest_status=$?
  if [ "$manifest_status" -ne 2 ]; then exit "$manifest_status"; fi
  # Compatibility fallback for binaries without machine manifest.
  zscalerctl --format json introspect |
    jq -e -c --arg product "$PRODUCT" --arg resource "$RESOURCE" '
      [ .commands[]
        | select(.path == ($product + " " + $resource + " list")) ] as $matches
      | if ($matches | length) != 1
        then error("resource list command is not advertised by this binary")
        else $matches[0]
          | {path, inherited_flags, effects, output_fields}
        end
    '
fi
```

Use `schema list` for the field catalog, and validate the intended redaction
mode and requested fields before a live read. This targeted query lists the
top-level renderable fields and checks each requested name or dotted path; it
rejects a typo or a field that is not rendered in the chosen mode:

```sh
SCHEMA=$(mktemp)
trap 'rm -f "$SCHEMA" "$MANIFEST"' EXIT
zscalerctl --format json schema list >"$SCHEMA"
jq -e -c --arg product "$PRODUCT" --arg resource "$RESOURCE" \
  --arg mode "$MODE" --arg requested "$FIELDS" '
  def renderable($prefix):
    .[] | select(.classification != "secret")
        | select(((.allowed_modes // []) | index($mode)) != null)
        | ($prefix + (.json_name // .name)) as $path
        | $path, ((.fields // []) | renderable($path + "."));
  [ .[] | select(.product == $product and .name == $resource) ] as $matches
  | if ($matches | length) != 1
    then error("expected exactly one resource catalog entry")
    else $matches[0] as $spec
      | [ $spec.fields[]
          | select(.classification != "secret")
          | select(((.allowed_modes // []) | index($mode)) != null)
          | (.json_name // .name) ] as $renderable
      | [ $spec.fields | renderable("") ] as $paths
      | ($requested | split(",") | map(gsub("^\\s+|\\s+$"; "")) | map(select(length > 0))) as $wanted
      | ($wanted - $paths) as $unavailable
      | if ($unavailable | length) > 0
        then error("requested field is unknown or not renderable in this mode")
        else {product: $spec.product, resource: $spec.name,
               shape: ($spec.shape // "list"),
               operations: $spec.operations,
               get_key: ($spec.get_key // null),
               redaction_mode: $mode, fields: $renderable,
               requested_fields: $wanted}
        end
    end
' "$SCHEMA"
```

The CLI drops a requested field that the active mode suppresses and names it
on stderr (`zscalerctl: requested ... fields omitted in share mode: ...`); the
preflight above still lets the caller choose renderable fields up front. Secret fields are never recoverable through `--fields`,
`jq`, raw SDK values, config, or credentials.

Each `schema list` field, nested ones included, may have a `type`:
`string`, `integer`, `number`, `boolean`, `object`, or `array<T>` of those,
the JSON type zscalerctl emits for it (observed by running each reader on
sample SDK values). It is absent where that is not fixed: secret fields,
values the reader derives from content, and open SDK shapes. Any typed field
may also be `null` (an unset value), so guard list iteration (`.x // []`). A
redacted value renders as a marker string whatever its type.

## Read a small, explicit result

Inspect `introspect` effects before delegating a command. Resource reads can
read configuration or execute an operator-configured provider, contact the
Zscaler API, and write a local file when `--output` is set. Agents use
`ZSCALERCTL_*` environment variables; `zscalerctl doctor` reports missing
or invalid values without contacting Zscaler (`status` is `OK` only when live
reads can run).
If credentials are missing, ask the operator to set them; do not invent values,
search shell configuration, or change provider commands. Prefer stdout;
use `--output` only when the local file write is intended. Sanitized tenant
inventory is still confidential.

Use explicit JSON and separate commands for list, get, and show. If the
selected list command advertises `--limit`, use a bounded JSON page:

```sh
set -euo pipefail
zscalerctl --format json --timeout 30s --redaction "$MODE" \
  --fields "$FIELDS" --filter 'name~hq' --limit 20 --offset 0 \
  "$PRODUCT" "$RESOURCE" list |
  jq -e '
    if (.records | type) != "array" or (.pagination | type) != "object"
    then error("expected a bounded JSON list page")
    else {offset: .pagination.offset, limit: .pagination.limit,
          returned_count: .pagination.returned_count,
          matched_count: .pagination.matched_count,
          has_more: .pagination.has_more,
          next_offset: .pagination.next_offset,
          collection_complete: .pagination.collection_complete,
          records: .records}
    end
  '
```

`--limit N` requires a positive N, and `--offset K` requires `--limit`
and a nonnegative K. They apply to JSON resource `list` only. A bounded page
has `records` and `pagination`; `matched_count` is the count after
projection, redaction, and local filters, while `returned_count` is the
number in this page. `next_offset` is the value for the next page when
`has_more` is true and is JSON `null` otherwise. Collection completes
before this output cut, so a successful page has `collection_complete: true`.
Each invocation recollects the resource; offsets do not provide a cross-call
snapshot or reduce the API collection work and memory.

If the binary has no page flags, keep the complete-list semantics and summarize
the array so a large raw list is never the agent's final answer:

```sh
set -euo pipefail
zscalerctl --format json --timeout 30s --redaction "$MODE" \
  --fields "$FIELDS" --filter 'name~hq' \
  "$PRODUCT" "$RESOURCE" list |
  jq -e '{matched_count: length, sample: .[:20],
          sample_complete: (length <= 20)}'
```

When a get operation is advertised and the task already supplies a valid ID,
use it directly; do not recollect the list:

```sh
: "${KNOWN_ID:?Set KNOWN_ID to the actual ID supplied by the task}"
zscalerctl --format json --timeout 30s --redaction "$MODE" --fields "$FIELDS" \
  "$PRODUCT" "$RESOURCE" get "$KNOWN_ID"
```

If you saved the page as `page.json`, extract its ID once with
`jq -er '.records[0].id' page.json`; for an older binary without page flags,
extract `.[0].id` from its complete array. Never use the shorthand
`list | get | show` as a shell pipeline; these are separate CLI operations.

## Narrowing, missing fields, and streams

`--fields` can only narrow the sanitized projection and applies to resource
`list`, `get`, and `show`. A `--fields` name may be a dotted path through
nested fields listed by `schema list`: `--fields name,groups.id` keeps each
record's `groups` list with only `id` in each element
(`"groups":[{"id":1},{"id":2}]`), and a table gets a `groups.id` column. `--filter key=value` does exact matching,
`--filter key~value` does case-insensitive substring matching, a key may
be a dotted path ending at a nested leaf field listed by `schema list`
(`groups.name=APP_DEV` matches one group exactly, element by element), repeated filters are ANDed, and
`--search term` scans rendered field values; filters
and search apply to resource `list` only. Unknown `--fields` or
`--filter` names are usage errors (exit 2) before config and live reads. A
`--filter` on a known field the active mode omits, or on a secret field, is a
usage error (exit 2) before any read, naming the mode that renders it. When
the mode omits fields, a successful read names them on stderr (`zscalerctl:
share mode omits zia/users fields: ...`), including nested paths such as
`groups.name`; standard mode omits only secrets and prints no mode-omission notice. Secret
fields are credential material only (passwords, keys, tokens, opaque blobs,
unmodeled maps such as `extensions`); they never render and are named only
when requested (`schema list` marks each with its `secret_reason`). Any other missing key means the source had no renderable value (empty,
null, or outside the catalog's projection). A list field renders `[]` only when the API itself returned an empty list,
so `[]` means none. A missing list key means the API did not send it, the
mode omits it, or it is not modeled: it does not prove none are set, and it
never means false. No matches is a successful empty result.

Count instead of listing with `--count-by` (list only): it groups the narrowed
records by the values at each comma-separated path and returns only those
values and counts, so an audit can count without pulling records into
context. A path into a list counts per element, once per record
(`--count-by groups.id` on `zia users` gives members per group), and records
with no value count under `null`. Paths through the same list pair values
from the same element (`groups.id,groups.name` gives each group's own name);
paths on different branches combine. JSON output is
`{"count_by":[...],"records":N,"distinct":D,"truncated":false,"counts":[{"values":{"groups.id":10},"count":2},...]}`
with the largest counts first; `--limit N` keeps the top N and sets
`truncated` when it dropped any. `--fields`,
`--offset`, an empty or repeated `--count-by`, a repeated path, an object
path, or a field the mode omits is a usage error.

Use NDJSON only when the consumer intentionally handles a complete record
stream:

```sh
set -euo pipefail
zscalerctl --format ndjson --timeout 30s "$PRODUCT" "$RESOURCE" list |
  jq -c 'select((.name // "") | test("hq"; "i"))'
```

NDJSON is one compact record per line after the full collection has been
fetched, projected, redacted, and filtered. It is buffered collection framing,
not early API streaming; `head` does not prevent collection, and
`--limit/--offset` are not combined with NDJSON. `set -euo pipefail`
preserves CLI failure and stops subsequent commands. Use `jq -e` when a
missing result should itself be an error; a valid NDJSON filter may emit no
records.

When the config file defines more than one profile and neither `--profile`
nor `ZSCALERCTL_PROFILE` is set, a successful live read prints
`zscalerctl: using profile "<name>" (...)` to stderr; a failure's stderr is
only the error envelope. Pass `--profile` explicitly when more than one
tenant is configured.

Failures use a JSON envelope on stderr and stable process exits: 0 success, 1
internal/canceled, 2 usage or unusable setting (`invalid_config`), 3 missing
credentials (unset, incomplete, or a secret that could not be resolved), 4 not
found or unsupported, 5 live API failure, 6 partial dump, and 7 drift detected from
`diff --fail-on-drift`. A live read failure (exit `5`: `live_access_failed` or `deadline_exceeded`)
also carries `reason`, `status` (the HTTP status, when there was a response),
and `retryable: true` when a retry could succeed. `forbidden`, `unauthorized`
and `not_subscribed` mean the API role or licence does not allow the resource:
do not retry, report it. `rate_limited`, `server_error`, `transport` and
`timeout` are retryable: back off, and raise `--timeout` for `timeout`.
`pagination_incomplete` means the CLI refused a list it could not prove
complete; `decode_failed` means the response did not match the expected shape.
A dump's `errors.ndjson` records carry the same three fields, and so do
stdio engine `live_access_failed` errors and `list`/`show` dump warnings. `--timeout 30s` caps each HTTP request, not the
whole list or dump run; apply a separate process or orchestration deadline
when the total operation must be bounded.

## Offline acceptance

Run `bash skills/zscalerctl/examples/offline-workflow.sh` when changing this
workflow. It uses a synthetic `zscalerctl` fixture and `jq` only: no
credentials, config, tenant, or network. The harness checks targeted
manifest/schema extraction, active-mode field selection, unknown-field
preflight before reads, a 2,500-record summary, bounded page metadata,
known-ID get, absent-field handling, pipefail, and the old-binary manifest
fallback. Set `ZSCALERCTL_BIN=/path/to/candidate` to add config-free
validation of a real candidate binary's manifest, schema, and introspection
JSON; its list/get scenarios remain synthetic.

For the full effects, credential, dump, and local-diff boundaries, read
[`AGENTS.md`](../../AGENTS.md) and
[`docs/cli/agent-machine-workflow.md`](../../docs/cli/agent-machine-workflow.md)
from a repository checkout. An installed skill can use the corresponding
[AGENTS.md](https://github.com/dvmrry/zscalerctl/blob/main/AGENTS.md) and
[agent machine workflow](https://github.com/dvmrry/zscalerctl/blob/main/docs/cli/agent-machine-workflow.md)
links instead.

## Read an existing collection

When `introspect` advertises `--from-dump`, use an operator-supplied complete
dump for repeated investigation. The usual resource commands retain their JSON
shapes, field selection, filters, search and bounded list pages:

```sh
zscalerctl --format json --from-dump ./scratch-live-dump \
  --fields id,name --limit 20 zia locations list
zscalerctl --format json --from-dump ./scratch-live-dump \
  --fields id,name zia locations get 12345
zscalerctl --format json --from-dump ./scratch-live-dump \
  zia advanced-settings show
```

These commands read local files and bypass configuration, credentials, secret
providers, SDK construction and network access. `--profile` and `--config`
cannot be combined with `--from-dump`. The saved redaction mode is the default;
an explicitly different `--redaction` is rejected. This cannot recover a field
that was omitted during collection. `--output` still has its advertised local
write effect.

Only complete, internally consistent artifacts that pass current catalog and
redaction admission are accepted. A missing resource, failed collection, or
missing ID is an error; it is not an empty inventory. A present list with zero
records is a successful empty inventory. Loading is capped at 256 MiB of
aggregate serialized resource data; decoded memory can be larger. Narrowing
flags do not reduce this admission work. Each CLI invocation reloads the
directory, so externally modifying it between calls can change the answers.
The core collection object stays immutable after loading. Keep the directory
unchanged for a repeatable investigation and record which collection was used.

## Inspect reviewed field meaning

When `introspect` advertises `schema describe`, request one resource directly:

```sh
zscalerctl --format json --redaction standard schema describe zia url-filtering-rules
```

This config-free candidate contract describes selected existing fields for
`zia/locations`, `zia/url-filtering-rules`, and `zia/rule-labels`. It includes
SDK-derived types and descriptions, confirmed reference targets, and explicit
`unknown` collection ordering. It is not a complete field catalog or a policy
evaluation engine. Check `review_status`: known resources outside the pilot
return `not_reviewed` with empty `fields`, while unknown resources fail with
exit 4. Only fields renderable in the requested mode are described; the default
is `standard`, independent of configuration. Continue using `schema list` to
validate all available field names. An omitted value never establishes that a
setting is false, empty, or unconfigured.

## Find what points at a resource

Reference fields name their target in the catalog: `schema list` shows
`"references": [{"product": ..., "resource": ...}]` on each mapped field,
usually one target; a field whose ids may belong to several resources lists
them all (a ZIA rule's `locations` may name a location, a sublocation, or a
predefined location such as Road Warrior that only `zia locations-lite`
returns).
To find every field that points at one resource, ask once instead of reading
every resource:

```sh
zscalerctl --format json schema references zia locations
```

It is config-free. Each `referred_by` entry names the resource and field, the
`match` path to filter on, the modes that render it (`allowed_modes`), and the field's
`targets`, so the next call is direct:

```sh
zscalerctl --format json --redaction standard --filter locations.id=12345 zia url-filtering-rules list
```

A flat id list such as `urlCategories` matches when any one element equals
the id. A reference no mode renders (a secret one) is listed with no `match`
and no modes: it shows the relationship exists but cannot be read. A field whose target has not been
verified is not listed, so an empty `referred_by` does not prove nothing
points at the resource.

`refs` does those reads for you, live or with `--from-dump`:

```sh
zscalerctl --format json refs to zia groups 12345        # records whose fields hold that id
zscalerctl --format json refs check zia url-filtering-rules  # referenced ids not in the target lists
```

`refs to` lists each referencing record's id and name with the field that
holds the id. `refs check` lists each referenced id that is not in its target
lists as read; that is all it claims, not that the object is missing from the
tenant. Both list under `not_checked` every field the redaction mode omits,
every resource that could not be read (with the failure reason), and every
field whose target could not be read. They report what references what, never
what a reference does.

Some Zscaler surfaces are not read at all: ZPA policy rule sets, entitlement-
gated modules, credential material, reports. Before concluding a tenant has
none of something, check that zscalerctl reads it:

```sh
zscalerctl --format json schema gaps zpa
```

Each entry names the `surface`, a `status` (`deferred` may be added once its
blocker clears; `out_of_scope` is not planned), a `reason`, a value-free
`note`, and, for a lite or alternate view or an action whose status is read,
the covered resources to read `instead`. Asking for a declared
gap (or a common alias, such as `zpa access-policies`) exits 4 with a message
naming it, so an absent resource is not mistaken for an empty one.
