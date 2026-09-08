---
# GENERATED from skills/zscalerctl/ - do not edit directly. Run scripts/sync-agents-skill.sh.
name: zscalerctl
description: Use when asked about Zscaler tenant configuration, inventory, or audit — ZIA/ZPA/ZTW/ZCC/Zidentity locations, rules, policies, app segments, connectors, groups — or to export Zscaler config, when `zscalerctl` is available or should be checked.
---

# zscalerctl

Use `zscalerctl` for authorized, tenant-read-only Zscaler inventory and
configuration reads. The allow-list projection and redaction layers are the
output boundary: a field absent from machine output is intentionally
unavailable.

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
mode and requested fields before a live read. This targeted query selects
top-level renderable fields; it rejects a typo or a field that is not rendered
in the chosen mode:

```sh
SCHEMA=$(mktemp)
trap 'rm -f "$SCHEMA" "$MANIFEST"' EXIT
zscalerctl --format json schema list >"$SCHEMA"
jq -e -c --arg product "$PRODUCT" --arg resource "$RESOURCE" \
  --arg mode "$MODE" --arg requested "$FIELDS" '
  [ .[] | select(.product == $product and .name == $resource) ] as $matches
  | if ($matches | length) != 1
    then error("expected exactly one resource catalog entry")
    else $matches[0] as $spec
      | [ $spec.fields[]
          | select(.classification != "secret")
          | select(((.allowed_modes // []) | index($mode)) != null)
          | (.json_name // .name) ] as $renderable
      | ($requested | split(",") | map(gsub("^\\s+|\\s+$"; "")) | map(select(length > 0))) as $wanted
      | ($wanted - $renderable) as $unavailable
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

The CLI itself accepts a known field that the active mode suppresses, then
omits it; the preflight above deliberately asks the caller to choose only
renderable fields. Secret fields are never recoverable through `--fields`,
`jq`, raw SDK values, config, or credentials.

## Read a small, explicit result

Inspect `introspect` effects before delegating a command. Resource reads can
read configuration or execute an operator-configured provider, contact the
Zscaler API, and write a local file when `--output` is set. Agents use
`ZSCALERCTL_*` environment variables; `zscalerctl doctor` reports missing
values without contacting Zscaler.
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
`list`, `get`, and `show`. `--filter key=value` does exact matching,
`--filter key~value` does case-insensitive substring matching, repeated
filters are ANDed, and `--search term` scans rendered field values; filters
and search apply to resource `list` only. Unknown `--fields` or
`--filter` names are usage errors (exit 2) before config and live reads. A
known field omitted by redaction is silently unavailable for output or
matching. An absent field therefore means omitted or unknown; it does not mean
false, empty, or unconfigured. No matches is a successful empty result.

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

Failures use a JSON envelope on stderr and stable process exits: 0 success, 1
internal/canceled, 2 usage, 3 missing or invalid credentials, 4 not found or
unsupported, 5 live API failure, 6 partial dump, and 7 drift detected from
`diff --fail-on-drift`. `--timeout 30s` caps each HTTP request, not the
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
