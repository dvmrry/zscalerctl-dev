# TUI Import Boundary

This document explains the package-level boundary that separates the
TUI eligibility gate from the Bubble Tea runtime on the `feature/tui`
integration branch.

## Finding

Bubble Tea v1.x performs package-initialization work in
`github.com/charmbracelet/bubbletea` that calls Lip Gloss background detection.
That detection can emit terminal OSC/cursor-position queries before
`main` runs and before `zscalerctl` can evaluate its own TUI eligibility gate.

A concrete symptom is that importing any package that imports Bubble Tea from
a normal CLI execution path can probe the terminal even when the user never
requested a TUI, is not on a TTY, or asked for machine-readable output.

## Rule

Normal CLI startup packages may import the TUI eligibility gate
(`github.com/dvmrry/zscalerctl/internal/tui`) but must not import Bubble Tea.

Bubble Tea imports are allowed only in the demo/runtime package
`internal/tui/tea` and in the demo entry point `scripts/tui-demo.go`.

## Package Shape

| Package | May import Bubble Tea | Purpose |
| --- | --- | --- |
| `internal/tui` | No | Eligibility gate and shared types; safe to load at startup. |
| `internal/tui/tea` | Yes | Bubble Tea `tea.Model` implementation used by the demo. |
| `scripts/tui-demo.go` | Yes (via `internal/tui/tea`) | Explicit, development-only demo harness. |
| `cmd/zscalerctl` | No | Normal CLI entry point. |
| `internal/cli` | No | Command dispatch and global flag parsing. |

## Enforcement

`scripts/verify-tui-import-boundary.sh` scans the forbidden directories and
fails if any `.go` file imports `github.com/charmbracelet/bubbletea`. The
script is exercised by `scripts/test-verify-tui-import-boundary.sh` and can be
added to routine checks with `make verify-tui-import-boundary`.

The gate package also contains a unit test that runs `go list` to assert that
`internal/tui` does not directly depend on Bubble Tea.

## Non-Goals

This boundary does **not** wire the TUI into normal `zscalerctl` execution,
does not add a Cobra command, and does not change command parsing, dispatch,
completion, introspection, JSON/NDJSON, machine error envelopes, or the golden
CLI surface.
