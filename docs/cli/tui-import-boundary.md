# TUI Import Boundary

This document explains the package-level boundary that separates the
TUI eligibility gate and collection layer from the Bubble Tea runtime on the
`feature/tui` integration branch.

## Finding

Bubble Tea v1.x performs package-initialization work in
`github.com/charmbracelet/bubbletea` that calls Lip Gloss background detection.
That detection can emit terminal OSC/cursor-position queries before
`main` runs and before `zscalerctl` can evaluate its own TUI eligibility gate.

A concrete symptom is that transitively importing Bubble Tea from a normal CLI
execution path can probe the terminal even when the user never requested a TUI,
is not on a TTY, or asked for machine-readable output. A textual import scan is
therefore not enough: the boundary must be checked at the dependency level.

## Rule

Normal CLI startup packages may import the TUI eligibility gate
(`github.com/dvmrry/zscalerctl/internal/tui`) and the Bubble-free collection
layer (`internal/tui/browserdata`, `internal/tui/launcher`) but must not
*transitively* import Bubble Tea or the Bubble Tea runtime package
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
depend on `github.com/charmbracelet/bubbletea` or
`github.com/dvmrry/zscalerctl/internal/tui/tea`. The script is exercised by
`scripts/test-verify-tui-import-boundary.sh` and is included in `make check`
via `make verify-tui-import-boundary`.

A PTY regression verifier, `scripts/verify-pty-escape-clean.sh`, runs the built
`zscalerctl version --format json` inside a real pseudo-terminal and confirms
that the output contains zero `ESC` bytes and parses as valid JSON. This guards
against Bubble Tea package-init probing leaking into normal interactive CLI paths.

## Non-Goals

This boundary does **not** wire the TUI into normal `zscalerctl` execution,
does not add a Cobra command, and does not change command parsing, dispatch,
completion, introspection, JSON/NDJSON, machine error envelopes, or the golden
CLI surface.
