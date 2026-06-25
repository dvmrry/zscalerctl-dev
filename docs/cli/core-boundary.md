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

- use of an already-configured resource reader
- catalog and resource metadata
- product/resource filtering
- list/show resource loading abstractions
- allow-list projection and redaction
- secret/config/SDK/client boundaries when those capabilities are in scope
- structured, already-safe records that callers can render

Core packages return data and errors. They do not decide terminal layout,
process exits, or command wording.

## Forbidden In Core

Core packages must not import or use:

- `internal/cli`
- Cobra command packages
- Bubble Tea, Bubbles, Fang, Wails, React, Vite, or frontend assets
- Lip Gloss rendering or other terminal layout code
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
normal CLI binary and the `internal/browser` seam.

Experimental UI work should stay on a separate branch or in a separate
application repository until it is deliberately promoted.
