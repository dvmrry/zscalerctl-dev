# CLI/Core/UI Boundary

`zscalerctl` keeps `main` as a CLI product surface with a machine-first product
floor. The contract split is documented in
[machine-contract.md](machine-contract.md): machine JSON/NDJSON/error behavior
is independent from human CLI and UI presentation layers.

Future interactive or desktop UI work should consume a small backend seam
instead of importing CLI command plumbing or adding UI frameworks to the normal
`zscalerctl` binary.

## CLI Responsibilities

`cmd/zscalerctl` and `internal/cli` own:

- command routing and Cobra integration
- global and local flag parsing
- stdout/stderr behavior
- output formats, including JSON, NDJSON, table, and pretty rendering
- machine-readable error envelopes and exit-code mapping
- help, completion, generated CLI docs, and introspection
- command-specific usage errors and user-facing CLI behavior

The CLI may consume backend/core packages, but backend/core packages must not
depend on the CLI package.

## Core Responsibilities

Core packages own UI-agnostic backend behavior:

- transport-neutral request/response contract types
- use of an already-configured resource reader
- catalog and resource metadata
- product/resource filtering
- list/show/get resource loading abstractions
- allow-list projection and redaction
- secret/config/SDK/client boundaries when those capabilities are in scope
- structured, already-safe records that callers can render

Core packages return data and errors. They do not decide terminal layout,
process exits, or command wording.

## Package Ownership

The shared runtime boundary is split by package ownership:

| Package | Owner Role | Overlay Posture |
| --- | --- | --- |
| `internal/machine` | Transport-neutral request, response, manifest, and executor contracts over already-projected records. | Safe for CLI, agents, and future overlays to consume as the machine API. |
| `internal/browser` | UI-agnostic catalog browsing and projected resource loading over an injected reader. | Safe for overlays to consume as a narrow backend seam. |
| `internal/resources` | Catalog metadata, source-record projection, projected-record containers, and redaction allow-list enforcement. | Safe for overlays to consume projected records and metadata; raw `SourceRecord` production remains adapter-owned. |
| `internal/redact` | Redaction modes and value scrubbing primitives used by the projection boundary. | Overlays may pass a mode through a trusted service, but must not widen allow-lists or redact raw SDK records themselves. |
| `internal/config`, `internal/credentials`, `internal/secretref`, `internal/secret` | Operator configuration, credential files, secret refs, and secret-value handling. | Not overlay-facing. Runtime assembly code owns these capabilities. |
| `internal/zscaler` | SDK adapter, SDK sessions, auth-mode wiring, and raw source-record mapping. | Not overlay-facing. It may feed projected seams but must not be imported by UI/TUI/Wails/desktop overlays. |
| `internal/cli`, `internal/output`, `cmd/zscalerctl` | CLI command routing, rendering, process behavior, and human/machine stdout/stderr behavior. | Not a core backend for overlays. |

The intended dependency direction for overlays is therefore:

```text
overlay -> internal/machine
overlay -> internal/browser
overlay -> internal/resources projected records/catalog metadata
overlay -/-> internal/config, internal/credentials, internal/secretref, internal/secret
overlay -/-> internal/zscaler raw SDK adapter
overlay -/-> internal/cli or internal/output rendering internals
```

In other words: overlays consume capabilities and projected records, not raw
SDK clients, credential-bearing config, secret refs, or raw source records.

## Forbidden In Core

Overlay-facing core packages such as `internal/machine` and `internal/browser`
must not import or use:

- `internal/cli`
- `internal/output`
- Cobra command packages
- Bubble Tea, Bubbles, Fang, Wails, React, Vite, or frontend assets
- Lip Gloss rendering or other terminal layout code
- `internal/config`, `internal/credentials`, `internal/secret`,
  `internal/secretref`, or `internal/zscaler`
- stdout/stderr writes
- `os.Exit`
- command-specific usage errors or machine-error envelope rendering

## Future UI Boundary

A future TUI, Wails, or React desktop application may consume the core seam, but
it must own its UI/product/support burden separately from the CLI. It must not
import `internal/cli` or rely on CLI rendering internals.

UI layers should receive narrow capabilities such as projected resource loading,
not raw config, credentials, SDK clients, source records, or redaction authority.
The core security boundary is described in
[machine-contract.md](machine-contract.md#core-security-boundary).

The dependency direction is:

```text
CLI -> core
UI  -> core
core -> no CLI, no UI, no Cobra, no terminal UI/runtime dependencies
```

`make verify-core-boundaries` enforces this dependency direction for the
normal CLI binary, the `internal/browser` seam, and the `internal/machine`
contract types.

Experimental UI work should stay on a separate branch or in a separate
application repository until it is deliberately promoted. The surface classes
and default build/check exclusion rules for experiments are defined in
[../DEV_PUBLIC_SURFACE_MODEL.md](../DEV_PUBLIC_SURFACE_MODEL.md).
