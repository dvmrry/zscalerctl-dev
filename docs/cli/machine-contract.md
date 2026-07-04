# Machine Contract And Presentation Layers

`zscalerctl` is machine-first. Agents, scripts, CI jobs, and operators should
be able to rely on the same projected and redacted resource model regardless of
whether the final renderer is JSON, NDJSON, table, pretty output, a future TUI,
or a desktop client.

This document defines the contract split. It is not a requirement to split the
current `zscalerctl` binary. The first boundary is internal: keep the machine
contract independent from human presentation choices.

The in-process candidate contract types live in `internal/machine`. Adapters may
translate Cobra argv, future stdio/JSON-RPC messages, or UI events into
`machine.Request` values and receive `machine.Response` or `machine.MachineError`
values. Those types are a typed internal boundary, not a 1.0 public API.
Stdio-style adapters that need a small JSON transport convention can use
`internal/machineio` to decode one bounded request, execute it, and encode the
response without importing CLI rendering. `machineio.ExecuteJSON` rejects
unknown request fields and trailing JSON values before executing. The
`machineio` transport convention is also candidate surface until explicitly
promoted.

For the agent-facing command workflow, see
[agent-machine-workflow.md](agent-machine-workflow.md).

## 1.0 Stability Boundary

The 1.0-supported machine-facing CLI surfaces are:

- `zscalerctl --format json machine manifest`
- JSON and NDJSON resource command output where those modes are supported
- the stderr machine-readable error envelope
- the documented process exit-code mapping
- committed schemas and fixtures for supported machine-readable artifacts

`machine manifest` output is a supported CLI JSON surface. Its `version` field,
currently `machine.v1`, is the manifest contract version. The published schema
is [machine-manifest.schema.json](../schema/machine-manifest.schema.json), and
`scripts/verify-machine-contract.sh` validates the committed manifest fixture
against it. Changing the supported manifest shape after 1.0 requires semver
treatment.

`machine.Request`, `machine.Response`, `MachineError`, the in-process executor
shape, `internal/runtime`, and `internal/machineio` remain candidate/internal
surfaces for 1.0. Request/response version fields are not required before 1.0
while those envelopes stay candidate. A later PR may deliberately promote a
request/response transport, but that promotion must add the appropriate schema,
fixture, compatibility, and semver gates.

The stderr CLI error envelope and exit-code mapping are supported. The machine
error-kind taxonomy has representative fixtures for the current stable kinds:
`usage`, `unsupported_capability`, `unsupported_operation`, `unknown_resource`,
`not_found`, `live_access_failed`, `canceled`, `deadline_exceeded`, and
`internal`. Further taxonomy changes need fixture coverage before any supported
machine request/response promotion.

## Error Vocabulary Map

Envelope `message` text is diagnostic prose, not a stable machine surface.
Consumers should branch on error kinds and process exit codes; those are the
stable compatibility surface.

| Scenario | Machine kind | Envelope kind | Exit code | Internal sentinel | Notes |
| --- | --- | --- | --- | --- | --- |
| Usage error | `usage` | `usage` | `2` for `cli.ErrUsage`; `1` if an unadapted `MachineError{Kind: usage}` reaches `main.go` | `cli.ErrUsage`; `resources.ErrMissingID`; `resources.ErrUnknownField` | `machineErrorFromLoadError` maps missing IDs and unknown projected fields to machine usage. Command usage reaches `main.go` as `cli.ErrUsage`. |
| Unknown/unsupported resource | `unknown_resource` | `unknown_resource` for machine errors; `unsupported_resource` for the Zscaler sentinel; `not_found` for CLI catalog misses | `4` for command-boundary unsupported/not-found errors; `1` if an unadapted `MachineError{Kind: unknown_resource}` reaches `main.go` | `resources.ErrUnknownResource`; `zscaler.ErrUnsupportedResource`; `cli.ErrNotFound` | The executor vocabulary is `unknown_resource`. `main.go` maps `zscaler.ErrUnsupportedResource` to `unsupported_resource`, and CLI catalog misses unwrap to `cli.ErrNotFound`. |
| Record not found (get of nonexistent id) | `not_found` | `not_found` | `4` | `resources.ErrRecordNotFound`; `zscaler.ErrResourceNotFound`; `cli.ErrNotFound` | `main.go` special-cases `machine.ErrorKindNotFound` to the not-found exit code. |
| Missing credentials | — | `missing_credentials` | `3` | `zscaler.ErrMissingCredentials` | No executor machine kind; config/runtime construction reports the Zscaler sentinel. |
| Invalid resource id | — | `invalid_resource_id` | `2` | `zscaler.ErrInvalidResourceID` | Command-boundary usage-class error; `machineErrorFromLoadError` has no separate invalid-ID kind. |
| Live access failure | `live_access_failed` | `live_access_failed` | `5` for `zscaler.ErrLiveAccessFailed`; `1` if an unadapted `MachineError{Kind: live_access_failed}` reaches `main.go` | `zscaler.ErrLiveAccessFailed`; executor default branch | The executor default hides backend details as machine `live_access_failed`. `main.go` maps only the Zscaler sentinel to exit 5. |
| Deadline exceeded | `deadline_exceeded` | `deadline_exceeded` | `5` | `context.DeadlineExceeded` | `main.go` special-cases `machine.ErrorKindDeadlineExceeded` to the live-access failure exit code. |
| Canceled | `canceled` | `canceled` | `1` | `context.Canceled` | `main.go` special-cases `machine.ErrorKindCanceled` to the internal error exit code. |
| Partial dump | — | `partial_dump` | `6` | `cli.ErrPartialDump` | For non-JSON formats, `run` returns the code without writing the JSON envelope. |
| Drift detected | — | `drift_detected` | `7` | `cli.ErrDriftDetected` | Set by `diff --fail-on-drift`. |
| Invalid config | — | `invalid_config` | `2` | `config.ErrInvalidConfig` | Config parsing/loading is outside `internal/machine`. |
| Invalid proxy config | — | `invalid_proxy_config` | `2` | `zscaler.ErrInvalidProxyConfig` | Proxy validation is outside `internal/machine`. |
| Internal | `internal` | `internal` | `1` | `machine.ErrorKindInternal`; default branch | Executor wiring errors use machine `internal`; otherwise unmapped command errors fall through to internal. |

## Machine Contract

The machine contract is the product floor:

- deterministic JSON and NDJSON output
- published JSON Schemas for committed machine-readable artifacts
- machine-readable stderr error envelopes
- stable exit-code mapping
- config-free completion, generated CLI docs, and introspection
- allow-list projection before redaction
- redaction and final byte scanning before bytes leave the process
- stdout for data and stderr for diagnostics
- no ANSI escape sequences or terminal control bytes
- no terminal probing or width-sensitive field meaning
- no interactive prompts, key handling, spinners, or progress animations
- no dependency on human help, usage text, table layout, or styling behavior

Machine consumers should pass `--format json` or `--format ndjson` explicitly
when those formats are supported. `--format auto` remains convenient, but a
PTY-based harness may look interactive and receive human output.

Changes to supported JSON, NDJSON, stderr error envelopes, exit codes,
completion, introspection, or generated CLI docs are machine-contract changes.
They require the same surface review, schema/golden coverage, and semver
treatment as any other compatibility-affecting change.
`scripts/verify-machine-contract.sh` keeps the internal machine JSON fixtures,
machine manifest schema validation, strict `machineio` decode behavior, and
projected-record reconstruction guard together as the mechanical contract gate.

## Streaming And Progress Direction

The current core read model is intentionally single-shot. A
`machine.Executor.Execute` call returns one complete `machine.Response`, and the
supported CLI `list`, `get`, and `show` flows render that response as JSON,
NDJSON, table, or pretty output. Runtime dump collection reports progress
through a trusted callback used by CLI logging/progress paths, while dump data
is still written as dump files rather than as machine response envelopes.

This remains valid for bounded resource reads and small operator workflows:
the response is already projected, redacted, verified, and easy for scripts to
consume. The design pressure appears when operations become large or long
running. Large lists and dumps can create memory pressure if every record must
be buffered before any consumer sees progress or data. Human frontends also
need structured progress without importing the CLI, while machine consumers
need stable data streams that do not depend on spinners, table layout, or
stderr prose.

Future streaming or progress work should keep these boundaries:

- shared libraries may provide iterators, cursors, or event streams over
  already-owned operation semantics
- core owns structured operation meaning: start, record delivery, warning,
  partial error, completion, cancellation, and failure
- trusted runtime owns live collection, product sessions, config loading,
  credential resolution, SDK reader construction, and retry/session mechanics
- CLI owns human rendering of progress, status text, colors, and spinners
- presentation layers must reuse runtime/core capabilities instead of
  duplicating config, credential, secret, SDK, or raw reader setup
- safe seams must still avoid importing trusted runtime assembly

One candidate model is an internal operation event stream. This is not a
committed schema, but it gives future changes a vocabulary:

- `started`: value-free operation metadata such as operation kind, product,
  resource, selected count when known, and redaction mode
- `progress`: value-free progress counters and catalog names, never record
  values, credentials, raw IDs from unprojected payloads, headers, or SDK errors
- `record`: one projected, redacted, verified record, or a batch of those
  records, and only after the same projection rules used by machine responses
- `warning` or `partial_error`: stable kind plus product/resource/operation
  context; messages must stay value-free and must not contain source payloads
  or raw transport details
- `completed`: value-free totals such as record count, resource count, and
  warning/error count
- `failed` or `canceled`: stable error kind and sanitized message, with context
  cancellation and deadline behavior mapped deliberately

The compatibility strategy is additive. Existing `Execute` remains the stable
one-shot adapter for current resource reads. A future event API can start as an
internal runtime/core helper, and a one-shot `machine.Response` can be built
from that stream later if the event model proves correct. No current CLI JSON,
NDJSON, table, pretty, stderr error envelope, exit code, or dump behavior
changes until a separate promotion explicitly changes the supported surface.

Dump should remain a separate artifact model unless a later design deliberately
promotes a dump event schema into the machine contract. Runtime dump collection
can eventually consume structured progress or operation events, but dump file
schemas, manifest files, and partial dump error records should not be folded
into `machine.Response` accidentally. Partial dump errors must remain
value-free, preserving the current safety property that failure metadata can be
reported without leaking tenant record values.

Semver follows the surface being changed:

- docs-only design updates are `semver:none`
- internal unexported streaming helpers are usually `semver:patch`
- a supported machine event schema, supported CLI streaming command, or new
  supported output mode is usually `semver:minor`
- after 1.0, breaking supported JSON/NDJSON behavior, stderr error envelope,
  exit-code mapping, dump schema, supported command behavior, or a future
  promoted machine response schema is `semver:major`

Non-decisions for this design:

- no transport choice
- no frontend implementation
- no supported event schema
- no command or output-mode promotion
- no process lifecycle model
- no change to current one-shot reads, dump collection, or CLI rendering

## Core Security Boundary

The security win from layering comes from capability boundaries, not package
names alone. Presentation layers are safer only when they cannot bypass the
core decisions that make output safe.

The trusted runtime owns:

- config loading and precedence
- credential and secret-reference resolution
- SDK/client construction and auth-mode decisions
- catalog and resource authorization boundaries
- live resource access
- projection from raw source records to allow-listed records
- redaction mode handling and final byte scanning
- filtering and field narrowing over projected data
- machine-safe serialization
- error sanitization

For the current CLI binary, `internal/cli` consumes the trusted runtime facade
in `internal/runtime` for catalog resource reads after Cobra parsing and config
option handling. `internal/runtime` owns config-backed reader construction and
wires `internal/browser`, `internal/machine`, and the `internal/zscaler` SDK
adapter. Overlay-facing packages such as `internal/machine`,
`internal/machineio`, `internal/browser`, and `internal/resources` expose the
safe side of that assembly: catalog metadata, typed machine envelopes, JSON
request/response helpers, projected records, and narrow loading capabilities.

Presentation layers must not own or receive:

- raw secret values
- tokens, headers, or credential-bearing config
- raw SDK clients or arbitrary SDK method access
- raw API payloads or unprojected source records
- redaction decisions or field allow-list expansion
- direct secret-reference resolution
- direct provider-specific resource authorization logic
- unsanitized SDK, HTTP, or transport errors

Overlays should receive capabilities, not internals. Prefer interfaces shaped
like this:

```go
type BrowserService interface {
    Resources(ctx context.Context, filter Filter) ([]ResourceInfo, error)
    ListProjected(ctx context.Context, product, resource string) ([]ProjectedRecord, error)
    ShowProjected(ctx context.Context, product, resource string) ([]ProjectedRecord, error)
    GetProjectedByID(ctx context.Context, product, resource, id string) ([]ProjectedRecord, error)
}
```

Avoid overlay shapes that expose config, credentials, raw readers, SDK clients,
or tokens:

```go
type UI struct {
    Config config.Config
    Reader *zscaler.Reader
    Token  string
}
```

Future overlays must consume `internal/machine`, `internal/machineio`,
`internal/browser`, or already-projected `internal/resources` values. They must
not import
`internal/config`, `internal/credentials`, `internal/secretref`,
`internal/secret`, or `internal/zscaler` to construct their own raw runtime.
If an overlay needs a new capability, add a narrow projected seam instead of
passing raw SDK/config/secret objects through the UI boundary.

Machine request narrowing is owned by the machine/core boundary. `fields`,
`filters`, and `search` are applied only after projection and redaction.
Filters and search apply to list operations; fields can narrow list/get/show
records. Non-empty machine `options` are rejected as usage errors until a
specific option is deliberately added to the contract. Response metadata is
server-generated; clients must not rely on request metadata being echoed.

If a future Wails or React desktop app exists, the React frontend must never
receive credentials, secret refs, tokens, SDK clients, or raw source records.
The Wails backend may call the core service and return already-projected,
already-redacted records to the frontend. Anything shipped to a frontend bundle
or browser-like runtime is observable and must be treated as public.

Overlays can still create security problems by logging projected data,
insecurely caching exports, making excessive API calls, exposing records through
clipboard/screenshots, or introducing risky dependencies. Those risks require
their own review, but overlays must not be able to bypass credentials,
projection, redaction, or machine-output safety.

## Human CLI Layer

The human CLI layer is an overlay on the machine contract. It may improve local
operator ergonomics, but it must consume the same projected and redacted data
that machine renderers consume.

Human CLI responsibilities include:

- pretty and table output
- terminal-aware color policy
- help and usage readability
- human-oriented error wording around the existing machine error categories
- progress or spinner text, only when explicitly gated away from machine output
- possible renderer or help polish through Lip Gloss, Fang, or similar tools

Human CLI work must not change:

- JSON or NDJSON output
- machine-readable stderr error envelopes
- exit-code mapping
- completion protocol
- introspection schema or output
- resource routing
- global parsing contracts such as `parseGlobal`
- projection, redaction, or field allow-list behavior

Lip Gloss, Fang, or any other presentation dependency is acceptable only as a
human-output implementation detail. If a tool needs to own command dispatch,
machine error rendering, completion, introspection, or resource routing, it is
crossing the boundary and must be rejected or explicitly scoped as a machine
contract change.

## UI Clients

Future TUI, desktop, Wails, or other visual clients are presentation layers.
They may consume `internal/browser` or another UI-agnostic core seam, or they
may shell out to the JSON contract, but they must not define the machine
contract.

UI clients must not import `internal/cli` or rely on CLI rendering internals.
They must not cause UI runtime dependencies to enter the normal
`cmd/zscalerctl` dependency graph. They also must not import low-level secret,
credential, SDK adapter, or raw source-record packages directly unless a future
security review deliberately promotes that access.

The dependency direction remains:

```text
core -> no CLI, no UI, no terminal styling/runtime dependencies
machine CLI -> core
human CLI -> core and machine-safe output models
TUI/Wails/desktop -> core or JSON contract
```

Never:

```text
core -> CLI
core -> TUI/Wails/desktop
CLI -> Wails
machine contract -> human renderer
```

## Current Binary Shape

The current `zscalerctl` binary intentionally serves both machine and human CLI
use:

```text
config and credentials
  -> reader
  -> core/browser/resource projection
  -> machine request/response contract types where needed
  -> projected and redacted records
  -> JSON/NDJSON machine renderers
  -> table/pretty human renderers
```

That shape is acceptable because the split is by contract and package
boundary, not necessarily by release artifact. A future `zscalerctl-core`,
`zscalerctl-tui`, or desktop application may be useful later, but it should be
a release decision over this boundary, not a prerequisite for keeping the
machine contract clean.
