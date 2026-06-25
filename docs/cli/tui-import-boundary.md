# TUI Import Boundary

This document explains the package-level boundary that separates the
TUI eligibility gate and collection layer from the Bubble Tea runtime on the
`feature/tui` integration branch.

## Finding

The standalone TUI uses `charm.land/bubbletea/v2` and `charm.land/bubbles/v2`.
Those runtime/widget packages must stay out of the normal `zscalerctl` startup
path. A textual import scan is not enough: the boundary must be checked at the
dependency level so transitive imports are caught.

The historical blocker was Bubble Tea v1 package-initialization terminal
probing. The v2 spike removes the local v1 vendor patch, but preserves the
boundary because normal JSON/NDJSON, completion, introspection, and machine
error paths must remain independent of any TUI runtime behavior.

## Rule

Normal CLI startup packages may import the TUI eligibility gate
(`github.com/dvmrry/zscalerctl/internal/tui`) and the Bubble-free collection
layer (`internal/tui/browserdata`, `internal/tui/launcher`) but must not
*transitively* import Bubble Tea, Bubbles, or the Bubble Tea runtime package
(`internal/tui/tea`).

Bubble Tea imports are allowed only in isolated TUI entrypoints such as
`internal/tui/tea`, `cmd/zscalerctl-tui`, `scripts/tui-demo.go`, and
`scripts/tui-browser-demo.go`.

## Package Shape

| Package | May import Bubble Tea | Purpose |
| --- | --- | --- |
| `internal/tui` | No | Eligibility gate and decision helpers; safe to load at startup. |
| `internal/tui/data` | No | Neutral `BrowserData` view model shared by collector and TUI. |
| `internal/tui/browserdata` | No | Catalog filtering, reader coordination, projection, `BrowserData` conversion. |
| `internal/tui/launcher` | No | Gate evaluation and `BrowserData` collection; Bubble-free bridge. |
| `internal/tui/tea` | Yes | Bubble Tea `tea.Model` implementation used by isolated TUI entrypoints. |
| `cmd/zscalerctl-tui` | Yes | Experimental standalone TUI binary. |
| `scripts/tui-demo.go` | Yes (via `internal/tui/tea`) | Explicit, development-only demo harness. |
| `scripts/tui-browser-demo.go` | Yes (via `internal/tui/tea`) | Explicit, development-only static browser demo. |
| `cmd/zscalerctl` | No | Normal CLI entry point. |
| `internal/cli` | No | Command dispatch and global flag parsing. |

## Enforcement

`scripts/verify-tui-import-boundary.sh` uses `go list -deps` to verify that
`./cmd/zscalerctl`, `./internal/cli`, and `./internal/tui` do not transitively
depend on `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, or
`github.com/dvmrry/zscalerctl/internal/tui/tea`. The script is exercised by
`scripts/test-verify-tui-import-boundary.sh` and is included in `make check`
via `make verify-tui-import-boundary`.

A PTY regression verifier, `scripts/verify-pty-escape-clean.sh`, runs the built
`zscalerctl version --format json` inside a real pseudo-terminal and confirms
that the output contains zero `ESC` bytes and parses as valid JSON. This guards
against TUI runtime behavior leaking into normal interactive CLI paths.

`scripts/verify-bubbletea-vendor-patch.sh` now guards the vendored Bubble Tea
v2 tree by failing if package `init()` functions or `HasDarkBackground`
references appear. The failure-path PTY verifier,
`scripts/verify-zscalerctl-tui-live-failure.sh`, proves invalid live startup
returns promptly, emits zero `ESC` bytes, and exits with a config/profile error
without opening the full-screen TUI. Both verifiers are part of `make check`.

## Non-Goals

This boundary does **not** wire the TUI into normal `zscalerctl` execution,
does not add a Cobra command, and does not change command parsing, dispatch,
completion, introspection, JSON/NDJSON, machine error envelopes, or the golden
CLI surface.
