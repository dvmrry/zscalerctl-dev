# Agent Machine Workflow

This guide is for agents, scripts, and CI jobs that drive `zscalerctl`. The
machine path is the contract to use for discovery and reads; human help, pretty
output, and tables are fallback ergonomics for operators.

## Match the binary and skill

Run `zscalerctl version` before relying on a workflow, and keep that binary
paired with the installed copy of `skills/zscalerctl/SKILL.md`. The current
development workflow includes the config-free `machine manifest` and bounded
JSON list pages. A public `v0.68.1` binary may predate `machine manifest`
and `--limit/--offset`. If the manifest is unsupported, use the
`introspect` and `schema list` fallback below and do not pass flags those
surfaces do not advertise. These checkout docs describe capabilities; they do
not mean that a distribution release has been published.

## Discover a resource without guessing

Begin with the config-free machine manifest. Stop on failed discovery or field
validation before a live read. Run the shell recipes in order:

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

`machine manifest` is JSON-only, carries `version: "machine.v1"`, and
advertises `resources.read` capabilities with product/resource names,
operations, projected-record output metadata, shape, get key, and
`read_only`. It does not contain the full field catalog. The manifest can be
large; the targeted query above emits one resource instead of dumping all
capabilities into the agent context.

Use `schema list` for field names, classifications, and redaction modes. It
is also a large document, so redirect it and select the exact catalog entry.
The following validates a requested top-level field set for the active mode:

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
      | ($requested
         | split(",")
         | map(gsub("^\\s+|\\s+$"; "") | select(length > 0))) as $wanted
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

This preflight is intentionally stricter than the output narrowing operation:
the CLI accepts a known field that the active mode suppresses, then omits it.
Choose only fields in the active mode's renderable set. Secrets are never
recoverable through field selection, `jq`, config, credentials, raw SDK
responses, or another redaction mode. A nested catalog field does not imply a
stable top-level `--fields` name; use the field names returned for the target
resource.

To confirm whether bounded output is available on this exact binary, inspect
the selected command's inherited global flags:

```sh
zscalerctl --format json introspect |
  jq -e --arg product "$PRODUCT" --arg resource "$RESOURCE" '
    any(.commands[];
        .path == ($product + " " + $resource + " list")
        and ((.inherited_flags // []) | index("limit")) != null
        and ((.inherited_flags // []) | index("offset")) != null)
  '
```

The manifest describes resources and operations; `introspect` is the source
for command flags and structured effects. Use `--help` only after discovery
when syntax details still need confirmation.

## Inspect effects and credentials before live reads

`zscalerctl --format json introspect` reports top-level `read_only: true`
for tenant scope and per-command `effects`. Resource reads can read local
configuration or secret files, execute an operator-configured `cmd:` or
keyring helper, contact Zscaler, and write a local file when `--output` is
set. Inspect `effects` before delegating a command and ensure any local writes
are part of the intended task.

Agents inject `ZSCALERCTL_*` environment variables rather than selecting
profiles. Run `zscalerctl doctor` to see missing variables or profile-backed
secret references without contacting Zscaler. If credentials are missing, ask
the operator to provide them; do not invent values or provider commands.

## Read JSON and keep the result bounded

Use explicit JSON and separate commands for `list`, `get`, and `show`.
When the selected command advertises `--limit`, request a bounded JSON page:

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

`--limit N` requires positive N, and `--offset K` requires `--limit` and
nonnegative K. They apply to JSON resource `list` only. A bounded response
has `records` and `pagination`: `matched_count` is the total after
projection, redaction, and local filters, and `returned_count` is the count
in this page. Follow `next_offset` while `has_more` is true; it is JSON
`null` on the final page. `collection_complete: true` means the complete
projected and filtered collection was obtained before the output cut.

The page is an output view over one complete collection. It does not reduce
API calls or collection memory, and every invocation recollects the resource.
Offsets therefore provide no cross-call snapshot or consistency guarantee.
Keep the command's process or orchestration deadline separate from
`--timeout 30s`: that flag caps each HTTP request, not the whole list or dump
run. If a later API page fails, the command fails and does not present a
successful page as a complete answer.

Without bounded page flags on an older binary, preserve the complete-list
behavior and summarize the array:

```sh
set -euo pipefail
zscalerctl --format json --timeout 30s --redaction "$MODE" \
  --fields "$FIELDS" --filter 'name~hq' \
  "$PRODUCT" "$RESOURCE" list |
  jq -e '{matched_count: length, sample: .[:20],
          sample_complete: (length <= 20)}'
```

The CLI still fetches and projects the full collection before this local
summary. Do not print a bare large list into an agent context.

For an already supplied valid ID, use a direct get and avoid a fresh list:

```sh
: "${KNOWN_ID:?Set KNOWN_ID to the actual ID supplied by the task}"
zscalerctl --format json --timeout 30s --redaction "$MODE" --fields "$FIELDS" \
  "$PRODUCT" "$RESOURCE" get "$KNOWN_ID"
```

If an ID was obtained from a saved page, extract it once and reuse it:
`jq -er '.records[0].id' page.json`. For an older complete array, use
`jq -er '.[0].id' list.json`. The get operation is a separate command; do
not write `list | get | show` as a shell pipeline.

## Narrowing, omitted fields, and NDJSON

`--fields` applies to resource `list`, `get`, and `show`, and only
narrows the sanitized projection. `--filter key=value` does exact matching,
`--filter key~value` does case-insensitive substring matching, repeated
filters are ANDed, and `--search term` searches rendered field values.
Filters and search apply to resource `list` only. Unknown `--fields` or
`--filter` names are usage errors (exit 2) before configuration and live
reads. A known field that projection or redaction omits is unavailable for
output or matching. An absent field means omitted or unknown; do not interpret
it as false, empty, or unconfigured. No matches is a successful empty result.

Use NDJSON only when a consumer intentionally handles the complete record
stream:

```sh
set -euo pipefail
zscalerctl --format ndjson --timeout 30s "$PRODUCT" "$RESOURCE" list |
  jq -c 'select((.name // "") | test("hq"; "i"))'
```

NDJSON is one compact record per line after the full collection has been
fetched, projected, redacted, and filtered. It is buffered collection framing,
not early API streaming; `head` does not prevent collection, and
`--limit/--offset` cannot be combined with NDJSON. `set -euo pipefail`
preserves CLI failure and stops later commands. Use `jq -e` when a missing
result should itself be an error; a valid NDJSON filter may emit no records.

## Errors and local artifacts

With explicit JSON, failures carry a machine-readable error envelope on
stderr. Branch on its stable `kind` and the process exit code rather than
scraping `message` prose:

| Code | Meaning |
| --- | --- |
| `0` | success |
| `1` | internal or canceled |
| `2` | usage error |
| `3` | missing or invalid credentials |
| `4` | not found or unsupported |
| `5` | live API failure |
| `6` | partial dump |
| `7` | drift detected by `diff --fail-on-drift` |

Prefer stdout for data and stderr for diagnostics. `--output PATH` creates
or replaces a restricted regular file and is a local write. Publication uses
a same-directory rename and is atomic on Unix; Windows does not have an atomic
replacement guarantee (see [Go's rename contract](https://pkg.go.dev/os#Rename)).
On Windows, mode `0600` does not restrict ACLs; use a destination directory
whose ACL already limits access to the intended account.

`dump --out DIR` writes a sanitized but still confidential directory and is
not combined with `--output`. `diff` compares dump directories already on
disk and does not contact Zscaler or collect a new snapshot.

## Offline acceptance

Run `bash skills/zscalerctl/examples/offline-workflow.sh` after changing
this workflow. It uses a synthetic CLI fixture and `jq` to exercise
targeted manifest/schema extraction, active-mode field selection, unknown
field validation before reads, a 2,500-record summary, bounded page metadata,
known-ID get, absent-field handling, pipefail, and the legacy manifest
fallback. Set `ZSCALERCTL_BIN=/path/to/candidate` to additionally validate
the candidate's config-free manifest, schema, and introspection JSON; list/get
scenarios stay synthetic.

For the package boundary model, read
[`machine-contract.md`](machine-contract.md) and
[`core-boundary.md`](core-boundary.md).

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
