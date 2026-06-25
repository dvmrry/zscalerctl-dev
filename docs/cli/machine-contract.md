# Machine Contract And Presentation Layers

`zscalerctl` is machine-first. Agents, scripts, CI jobs, and operators should
be able to rely on the same projected and redacted resource model regardless of
whether the final renderer is JSON, NDJSON, table, pretty output, a future TUI,
or a desktop client.

This document defines the contract split. It is not a requirement to split the
current `zscalerctl` binary. The first boundary is internal: keep the machine
contract independent from human presentation choices.

The in-process contract types live in `internal/machine`. Adapters may translate
Cobra argv, future stdio/JSON-RPC messages, or UI events into
`machine.Request` values and receive `machine.Response` or `machine.MachineError`
values. Those types are a typed boundary, not an internal JSON transport.

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

Changes to JSON, NDJSON, machine error envelopes, exit codes, completion,
introspection, or generated CLI docs are machine-contract changes. They require
the same surface review, schema/golden coverage, and semver treatment as any
other compatibility-affecting change.

## Core Security Boundary

The security win from layering comes from capability boundaries, not package
names alone. Presentation layers are safer only when they cannot bypass the
core decisions that make output safe.

Core packages own:

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
    LoadProjected(ctx context.Context, product, resource string) ([]ProjectedRecord, error)
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
- stderr machine error envelopes
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
