# Experimental standalone TUI binary: `cmd/zscalerctl-tui`

`zscalerctl-tui` is an experimental, separate binary for the interactive TUI
browser. It lives in `cmd/zscalerctl-tui/` and is allowed to import
`github.com/charmbracelet/bubbletea` and `internal/tui/tea`. The normal
`cmd/zscalerctl` binary and `internal/cli` package remain Bubble Tea-free.

## Why a separate binary?

Bubble Tea v1.x runs package-initialization terminal probing (via Lip Gloss
background detection) that can emit OSC/DSR sequences before `main()`. If Bubble
Tea were linked into the normal `zscalerctl` binary, every invocation — even
`zscalerctl version --format json` on a non-TTY — could be affected. Keeping
the TUI runtime in a separate binary preserves the safety of the main CLI.

## Boundary

| Package / binary | May import Bubble Tea |
| --- | --- |
| `cmd/zscalerctl` | No |
| `internal/cli` | No |
| `internal/tui` | No |
| `internal/tui/data` | No |
| `internal/tui/browserdata` | No |
| `internal/tui/launcher` | No |
| `internal/tui/tea` | Yes |
| `cmd/zscalerctl-tui` | Yes |
| `scripts/tui-demo.go` | Yes |
| `scripts/tui-browser-demo.go` | Yes |

The boundary is enforced by `scripts/verify-tui-import-boundary.sh`, which runs
`go list -deps` on `./cmd/zscalerctl`, `./internal/cli`, and `./internal/tui`.

## Current modes

Only fixture-backed modes are implemented. No config, credentials, live
reader, or network are used.

```sh
# Run the fake-reader-backed collector fixture (default)
go run ./cmd/zscalerctl-tui --collector-fixture

# Use the hard-coded static fixture
go run ./cmd/zscalerctl-tui --fixture

# Build the binary
go build -o zscalerctl-tui ./cmd/zscalerctl-tui
./zscalerctl-tui --collector-fixture
```

## Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--collector-fixture` | `true` (default) | Use the fake-reader-backed collector fixture. |
| `--fixture` | `false` | Use the hard-coded static fixture. |
| `--color` | `auto` | Color mode: `auto`, `always`, `never`. |
| `--format` | `auto` | Output format gate (used by the TUI eligibility check). |

`--fixture` and `--collector-fixture` are mutually exclusive. If neither is
supplied, the binary defaults to `--collector-fixture`.

## Gate behavior

The binary uses the same `internal/tui` gate as the removed `browse --tui`
path. It refuses to start unless stdin, stdout, and stderr are all TTYs, the
format is not `json`/`ndjson`, no `--output` path is set, and styling is not
disabled by `--color never`, `NO_COLOR`, or `TERM=dumb`.

## Future work

- Load config and credentials inside `cmd/zscalerctl-tui` and wire the real
  `ResourceReader` into the collector.
- Decide whether the main `zscalerctl` binary should ever `exec` this separate
  binary, or whether it remains a standalone command.
- Capture live-reader-backed readbacks on a scratch tenant with no secrets.
