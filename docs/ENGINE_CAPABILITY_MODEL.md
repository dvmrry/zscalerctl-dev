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

## Initial typed capability set

| Capability | Operations | Input family | Result family | Possible effects |
| --- | --- | --- | --- | --- |
| `engine.manifest` | `manifest` | none | engine manifest | none |
| `resources.read` | catalog-derived union of `list`, `get`, `show` | resource read | projected records | configuration-dependent local read; network access; configuration-dependent process execution |

All capabilities are tenant-read-only. The initial set has no local write or
delete effect. Those effect kinds exist in the closed model so later operations
such as dump writing or config initialization cannot hide local mutation behind
a generic boolean.

`engine.manifest` is derived from the static resource catalog. Calling it does
not load config, resolve a provider, construct an SDK client, access the local
filesystem, execute a process, or contact Zscaler. The `resources.read` effects
describe construction and execution of the normal live runtime: config or
secret files and provider helpers may be used before the always-possible
network read.

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

The projected-value domain is intentionally narrower than arbitrary Go values:
JSON-safe primitive scalars and finite numbers, supported scalar sequences,
and the catalog-modeled `map[string]any`, `[]any`, and
`[]map[string]any` families. Structs, pointers, complex or non-finite numbers,
typed maps, nested typed containers, and process-like handles are rejected as
invalid projected values. New value families require an explicit projection,
redaction, copy, verification, and rendering design before admission.

## Compatibility boundary

This checkpoint deliberately leaves these supported surfaces unchanged:

- `zscalerctl --format json machine manifest` and its `machine.v1` schema
- resource list/get/show JSON, NDJSON, table, and pretty output
- CLI stderr error envelopes and exit-code mapping
- `introspect` v1/v2 schemas and goldens
- `machineio` request/response transport behavior, except that strict decoding
  now rejects the removed candidate `input.options` field as unknown

The legacy candidate `machine.Request`/`machine.Response` execution methods
remain as compatibility adapters over the event path. New in-process resource
consumers use the typed `Read` method. No `engine.v1` CLI command or JSON schema
is introduced by this checkpoint.

## Next extensions

Catalog/schema discovery, sanitized status, URL lookup, dump, and diff each get
their own typed request/result family and engine-manifest entry in separate
reviewable slices. Local-write and local-delete operations must expose those
effects before an adapter can invoke them. Wire DTO and stdio work remains
blocked on the dedicated protocol-design checkpoint.
