# Local Engine API — Candidate Architecture

Status: ACCEPTED for candidate implementation on 2026-07-10. This document
defines the common Go operation engine that the existing CLI and future local
frontends will share. It does not promote a public Go package or wire protocol.

## Purpose

`zscalerctl` should have one implementation of Zscaler-facing behavior. The
official Go SDK remains the only vendor API implementation. The engine above it
owns zscalerctl-specific policy and semantics:

- config precedence and secret-provider resolution
- SDK reader construction and tenant-read-only enforcement
- catalog discovery and resource routing
- projection, redaction, and final output-safety verification
- post-projection fields, filters, and search
- stable machine errors and operation effects
- dump and diff behavior
- structured progress, cancellation, and completion

Frontends consume those operations; they do not receive SDK clients, raw API
models, credentials, tokens, source records, or unsanitized errors.

```text
Cobra CLI -----------+
Wails backend -------+--> Go engine --> trusted runtime --> official Go SDK
MCP adapter ---------+
stdio protocol host -+--> TypeScript / Rust / other local clients
```

There is no HTTP server in this design. A future transport is a local stdio
adapter over the same engine.

## Contract layers

The project has distinct contracts that must not be conflated:

1. The supported CLI machine surfaces: JSON/NDJSON output, stderr envelopes,
   exit codes, schemas, `introspect`, and `machine manifest`.
2. The candidate in-process Go engine under `internal/machine` and
   `internal/runtime`.
3. A future, explicitly versioned stdio protocol with transport DTOs and JSON
   schemas.
4. Adapter-specific contracts such as MCP tools or a Wails binding.

The supported `machine.v1` manifest remains immutable. Expanding the engine
does not mutate it. A future engine capability manifest and wire protocol get
their own versions and immutable schema URLs before promotion.

## Engine scope

The engine eventually owns these domain operations:

- build and engine capability discovery
- catalog and projected-field schema discovery
- sanitized doctor, auth, and config status
- catalog-driven resource `list`, `get`, and `show`
- specialized reads such as ZIA URL lookup
- dump collection and the existing safe dump writer
- diff of existing dump directories
- config initialization only as a later, explicitly local-effectful operation

Cobra help, shell completion, human rendering, global `--output`, terminal
styling, and exit-process control remain adapter responsibilities. They may use
engine results but are not engine operations.

The catalog-driven resource capability prevents a second handwritten endpoint
registry. Resource records remain dynamic by catalog, but cross the engine seam
only as `resources.ProjectedRecord` values whose fields are already allow-list
projected, redacted, and verified.

## Candidate Go shape

The candidate execution surface is deliberately small. The initial typed
resource path is defined by
[ENGINE_CAPABILITY_MODEL.md](ENGINE_CAPABILITY_MODEL.md):

`runtime.Engine` is the capability-manifest owner and common coordinator. It
keeps host construction options, but loads config and constructs any live
reader per operation. The narrower `runtime.Machine` remains the already-built
live resource executor used inside a read action.

```go
NewEngine(Options) (*Engine, error)
Manifest() machine.Manifest // supported machine.v1 compatibility surface
EngineManifest() machine.EngineManifest
DiscoverCatalog(ctx, machine.CatalogRequest) (machine.CatalogResult, error)
InspectStatus(ctx, machine.StatusRequest) (machine.StatusResult, error)
LookupURL(ctx, machine.URLLookupRequest) (machine.URLLookupResult, error)
Read(ctx, machine.ResourceReadRequest) (machine.ResourceReadResult, error)
Execute(ctx, request) (machine.Response, error)
ExecuteStream(ctx, request, sink) error
```

Construction is config-free but validates the injected static catalog and
rejects any tenant-mutating operation before a capability can be advertised.

Config-backed status is assembled through `runtime.StatusInspector` and its
typed `Inspect(ctx, machine.StatusRequest)` method. Construction computes and
retains only sanitized doctor/auth/config views; it does not retain raw config,
resolve deferred providers, construct an SDK reader, or contact Zscaler. Its
error boundary emits static machine-safe classifications and preserves only
safe sentinels; status values cannot carry terminal-control or Unicode-format
runes into a frontend.

Typed ZIA URL lookup validates and copies a complete batch before config
loading. Its accepted syntax is a hierarchical absolute URL or ZIA's bare
`host[/path]` form; it rejects unsafe controls on the original boundary value
before trimming ordinary surrounding spaces, rejects invalid UTF-8, and does
not admit userinfo-like or escaped delimiter forms as a bare host. It removes
userinfo/query/fragment data from requests and SDK-returned URLs and pre-redacts
every returned string. It performs one synchronous SDK call, preserves response
order and duplicates, and returns a defensively copied closed result. Cobra
preserves its existing JSON/table/pretty DTO and usage surface as an adapter
over this method; unsupported format policy is resolved before engine input
validation.

`ExecuteStream` is the semantic path. Both typed `Read` and compatibility
`Execute` consume it, so the CLI and interactive adapters cannot acquire
separate resource-read behavior. The implementation is synchronous and starts
no goroutines; callers that need concurrency own the goroutine and its lifetime.

The current request/response envelopes remain candidate compatibility types.
`Input.Options` has been removed; new operation families must add
capability-specific typed inputs and safe result/item types. Generic resource
data remains catalog-described, but control fields, execution settings,
lifecycle events, errors, and non-resource results are typed.

## Event invariants

The event design is defined in [EVENT_STREAM_DESIGN.md](EVENT_STREAM_DESIGN.md).
The foundation enforces:

- exactly one `started` event first
- ordered, synchronous delivery with natural backpressure
- zero or more record/progress/warning events
- exactly one attempted terminal event: `completed`, `failed`, or `canceled`
- no event after terminal
- sink errors and panics converted to value-free machine failures
- projected records only; no source-record path
- no direct JSON representation for the in-process `machine.Event` type

The config-free manifest operation completes with an optional manifest payload
so its existing `machine.Response` can also be reconstructed from events. That
payload is catalog-derived public project data, not tenant data.

## Execution settings and credentials

Typed execution settings may select profile, config path, timeout, redaction
mode, and cache behavior. They never contain environment entries, credentials,
secret references, tokens, or resolved secret values. The spawning host
supplies the environment separately, and the trusted Go runtime performs config
loading and provider resolution.

The initial long-lived adapters create a runtime per action, matching the CLI's
current token lifetime. Any in-memory SDK session cache requires a separate
threat-model and invalidation design.

Every capability manifest must describe possible local-read, local-write,
local-delete, network, and process effects, including request-dependent and
configuration-dependent conditions. Adapter policy remains adapter-specific:
for example, MCP may impose `share` redaction and a call budget without changing
the generic engine contract.

## Future stdio protocol

The stdio transport is a separate adapter and never serializes internal event
structs directly. Its candidate v1 design must specify:

- a mandatory version/capability handshake
- bounded NDJSON frames with strict unknown-field, duplicate-key, trailing-data,
  UTF-8, and size validation
- request IDs and monotonic event sequence numbers
- request and cancellation input frames
- started, item, progress, warning, completed, failed, and canceled output
  frames
- one active operation initially, with caller-side queuing
- stdout for protocol frames and value-free stderr diagnostics
- deterministic EOF, broken-pipe, signal, and shutdown behavior
- no network listener and no credentials crossing the protocol

TypeScript and Rust reference clients must run the same transcript conformance
suite as the Go codec before the protocol is promoted.

## Delivery sequence

1. Event types, lifecycle enforcement, and one-shot reconstruction.
2. Runtime forwarding and dump migration; remove `DumpProgressFunc` when its
   only caller migrates.
3. Typed capability inputs/results and an engine capability manifest. The
   resource-read foundation is implemented; each remaining capability migrates
   in its own slice.
4. Migrate catalog/status/URL-lookup/dump/diff behavior behind the engine while
   keeping Cobra as an in-process adapter.
5. Specify and implement versioned wire DTOs and strict codecs.
6. Add the long-lived stdio experiment and shared Go/TypeScript/Rust transcript
   tests.
7. Build MCP, Wails, Ink/OpenTUI, Ratatui, or GUI experiments against those
   seams.
8. Promote only after the CLI and at least two independent consumers pass
   conformance and the compatibility/security reviews.

Every high-risk slice follows the repository's fresh-context adversarial-review
workflow. Candidate event and transport work must leave supported CLI goldens,
machine fixtures, schemas, error kinds, and exit codes unchanged unless a
separate semver-labeled promotion deliberately changes them.
