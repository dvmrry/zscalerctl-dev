# Engine Capability Model — Candidate Design Checkpoint

Status: ACCEPTED for candidate implementation on 2026-07-11. This checkpoint
refines [ENGINE_API_DESIGN.md](ENGINE_API_DESIGN.md) for typed in-process
capabilities. It does not define a public Go API, CLI surface, or wire protocol.

## Decisions

1. The supported `machine.v1` manifest and existing CLI machine output remain
   separate and immutable. The candidate engine manifest is `engine.v1` and is
   available only as an in-process Go value.
2. Each engine operation family gets a capability-specific request and result.
   There is no generic `map[string]string` option escape hatch.
3. Resource reads use `ResourceReadRequest`, `ResourceReadInput`, and
   `ResourceReadResult`. The result can contain only
   `resources.ProjectedRecords`; it cannot carry source records, SDK models,
   arbitrary maps, config, credentials, or renderer state.
4. `Executor.Read` runs through the existing synchronous operation-event path.
   It does not duplicate list/get/show loading, narrowing, error, cancellation,
   or redaction behavior.
5. Typed engine requests, results, settings, events, and manifests reject direct
   JSON. A future stdio adapter must define, version, bound, and validate
   separate transport DTOs.
6. Execution settings belong to runtime construction rather than individual
   operation requests. They may select profile, config path, timeout, redaction,
   and cache behavior, but never contain environment entries, credentials,
   secret references, tokens, or resolved secret values.
7. Capability effects are conservative end-to-end possibilities. Consumers
   must treat `configuration_dependent` effects as possible unless the host
   environment and effective config are reviewed and pinned.

## Current typed capability set

| Capability | Operations | Input family | Result family | Possible effects |
| --- | --- | --- | --- | --- |
| `engine.manifest` | `manifest` | none | engine manifest | none |
| `catalog.schema` | `list` | none | resource catalog | none |
| `status.inspect` | `doctor`, `auth_status`, `config_status` | status | sanitized status | configuration-dependent local read |
| `zia.url_lookup` | `lookup` | URL lookup | sanitized URL classifications | configuration-dependent local read; network access; configuration-dependent process execution |
| `resources.read` | catalog-derived union of `list`, `get`, `show` | resource read | projected records | configuration-dependent local read; network access; configuration-dependent process execution |
| `dump.write` | `dump` | dump | value-free dump summary | local filesystem read and write; request-dependent local delete; network access; configuration-dependent process execution |

All capabilities are tenant-read-only. `dump.write` has explicit local read,
write, and request-dependent delete effects; tenant read-only does not hide
local mutation. Config initialization remains a later capability and must
declare its own effects before admission.

`engine.manifest` and `catalog.schema` are derived from the static resource
catalog. Calling either does not load config, resolve a provider, construct an
SDK client, access the local filesystem, execute a process, or contact Zscaler.
`runtime.NewEngine` rejects any injected catalog containing a mutating
operation. The lower-level manifest derivation also fails closed by omitting
catalog and resource-read capabilities for such a catalog, so discovery never
advertises an operation that the typed catalog executor would reject.
`status.inspect` may load config and explicitly selected secret files, but it
does not resolve deferred `env:`, `file:`, `keyring:`, or `cmd:` providers,
construct a reader, execute a process, or contact Zscaler. It retains only
precomputed sanitized status values, not raw config or secret sources. Status
strings are redacted and normalize Unicode control and format runes before
crossing the engine boundary. Configuration failures become static machine
errors that preserve only safe sentinel classification, never paths, provider
details, or backend text. Supported status operations honor canceled and
expired contexts; unsupported operations are rejected before config loading
without echoing caller-controlled operation text.

`zia.url_lookup` validates and normalizes the complete request before config
loading. It accepts hierarchical absolute URLs and ZIA's bare `host[/path]`
form; root-relative, scheme-relative, opaque, and hostless scheme-only
references are rejected. Bare hosts reject userinfo-like or escaped delimiter
forms rather than guessing how a downstream parser will interpret them. The
original boundary string is checked for valid UTF-8, C0, C1, and Unicode format
controls before surrounding ASCII spaces are trimmed. Other Unicode whitespace,
embedded raw spaces, and unsafe or invalid UTF-8 decoded host/path components
are rejected.
Userinfo, query, and fragment data are removed before the URL reaches Zscaler.
SDK-returned URLs cross the same normalization boundary again, so an echoed or
independently supplied response cannot reintroduce those fields. Malformed
response URLs fail closed. Every returned string is redacted and
control-normalized before entering the closed result. One call handles the
whole batch synchronously and preserves SDK order and duplicates.

The `resources.read` and `zia.url_lookup` effects describe construction and
execution of normal live runtimes: config or secret files and provider helpers
may be used before the always-possible network read.

`dump.write` is advertised only for a valid, duplicate-free, tenant-read-only
catalog with at least one executable list/show resource. Requests carry an
output directory plus canonical product and exact product/resource selectors;
adapters may resolve shorthand, but the engine validates the resulting
selection again before config access. Collection remains in catalog order and
emits projected records and value-free warnings. Force safety rejects empty,
cwd, root, home, symlink, non-directory, and non-dump targets and performs no
delete until collection succeeds. The context-aware writer preserves
owner-only permissions, exclusive file creation, same-directory temporary-file
fsync and rename finalization, and writes the ownership manifest last. `completed` means
the full artifact exists, while failed/canceled writes never emit completion.

## Trust and copying boundaries

The engine produces projected and redacted records through the trusted browser
service. `ResourceReadResult` keeps its collection private and returns
defensive collection copies. `ProjectedRecord` values expose only copied field
maps and values.

`NewResourceReadResult` accepts already-trusted projected records and does not
reclassify arbitrary data. An adapter receiving a result across an injected or
otherwise distinct trust boundary must call
`resources.VerifyProjectedRecords` against the selected catalog spec and active
redaction mode before rendering. The Cobra resource adapter retains this final
verification barrier while consuming `Machine.Read` directly.

Input slices are copied before execution. Returned manifests and result
collections are fresh, so caller mutation cannot change runtime catalog state
or later discovery results. Projected values with cycles, non-data handles, or
mutable state hidden in unexported struct fields fail closed as invalid
projected values instead of retaining aliases or reaching a renderer.

`CatalogResult` owns a recursively copied snapshot, including operation,
allowed-mode, and nested-field slices. `StatusResult` is a closed union whose
doctor/auth/config values are sanitized before return. Status construction
precomputes those values and discards raw config, credentials, secret sources,
provider commands, and proxy values before the inspector can be retained.
`URLLookupResult` owns recursively copied classification slices and returns a
fresh result collection. Both request and result reject direct JSON. Raw URL
input, SDK response types, config, credentials, and backend errors cannot enter
the result.

`DumpResult` owns a copied, value-free failure list and exposes only record,
resource, and warning counts plus the effective redaction mode. `DumpRequest`
and `DumpResult` reject direct JSON. The result never retains the output path,
projected records, dump writer state, config, credentials, SDK values, or raw
errors. Record events remain projected/redacted values governed by the shared
event boundary; dump artifacts remain confidential tenant inventory on disk.

The projected-value domain is intentionally narrower than arbitrary Go values:
method-free built-in primitive scalars, valid `json.Number` values, finite
numbers, supported method-free scalar sequences, and the catalog-modeled
`map[string]any`, `[]any`, and `[]map[string]any` families. Named source
scalars and scalar sequences are normalized to built-in values before they can
enter a projected record; the already-projected constructors reject them so
custom JSON, text, or string methods cannot run during rendering. Structs,
pointers, complex or non-finite numbers, typed maps, nested typed containers,
and process-like handles are rejected as invalid projected values. New value
families require an explicit projection, redaction, copy, verification, and
rendering design before admission.

## Compatibility boundary

This checkpoint deliberately leaves these supported surfaces unchanged:

- `zscalerctl --format json machine manifest` and its `machine.v1` schema
- resource list/get/show JSON, NDJSON, table, and pretty output
- `zia url-lookup` JSON, table, pretty, usage, and unsupported-format behavior
- dump directory schemas/bytes, status prose, partial-dump exit 6, and force
  safety behavior
- CLI stderr error envelopes and exit-code mapping
- `introspect` v1/v2 schemas and goldens
- `machineio` request/response transport behavior, except that strict decoding
  now rejects the removed candidate `input.options` field as unknown

The legacy candidate `machine.Request`/`machine.Response` execution methods
remain as compatibility adapters over the event path. New in-process resource
consumers use the typed `Read` method. The existing `schema list`, `doctor`,
`auth status`, and `config show` commands adapt typed catalog/status results
back to their unchanged supported render shapes. `zia url-lookup` adapts the
typed URL result into its existing output DTO. `dump` adapts the typed summary
and events into its existing spinner, diagnostics, local artifact, and partial
exit behavior. No `engine.v1` CLI command or JSON schema is introduced by this
checkpoint.

## Next extensions

Diff gets its own typed request/result family and engine-manifest entry in the
next reviewable slice. Wire DTO and stdio work remains blocked on the dedicated
protocol-design checkpoint.
