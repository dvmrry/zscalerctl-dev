# TUI Demo Readback

This readback belongs to the `feature/tui` integration line. It records the
visual behavior of the isolated Bubble Tea demo harness
(`scripts/tui-demo.go`) and confirms that normal `zscalerctl` CLI paths do
not emit TUI terminal sequences.

## Scope

- Demo harness: `go run -mod=vendor ./scripts/tui-demo.go`
- Model: `internal/tui/tea.DemoModel`
- No Cobra command is added.
- No config is loaded.
- No credentials are resolved.
- No Zscaler client or network path is used.
- No subprocess or filesystem-write path is used by project code.

## Method

All captures were produced in a real PTY with `TERM=xterm-256color`,
`COLORTERM=truecolor`, and `NO_COLOR` unset. The terminal dimensions were set
with `TIOCSWINSZ` before spawning the demo, and a single exit key was sent after
the initial render completed. The transcripts below are the ANSI-stripped
versions of the captured sessions; full raw recordings are attached to the PR
as evidence.

## Visual readback

### 80x24

```text
┌────────────────────────────────────────────────────────────────────────┐
│ zscalerctl TUI demo                                                    │
│                                                                        │
│ status: Bubble Tea running                                             │
│ terminal: 80x24                                                        │
│ style: 256-color render                                                │
│                                                                        │
│ keys: q quits | esc quits | ctrl+c quits                               │
└────────────────────────────────────────────────────────────────────────┘
```

The panel renders at the full 80-column width, uses 256-color styling, and the
demo reports `terminal: 80x24`.

### 60x16

```text
┌────────────────────────────────────────────────────────┐
│ zscalerctl TUI demo                                    │
│                                                        │
│ status: Bubble Tea running                             │
│ terminal: 60x16                                        │
│ style: 256-color render                                │
│                                                        │
│ keys: q quits | esc quits | ctrl+c quits               │
└────────────────────────────────────────────────────────┘
```

The panel narrows to fit the 60-column terminal. The content still fits cleanly
and the demo reports `terminal: 60x16`.

### 120x32

```text
┌────────────────────────────────────────────────────────────────────────┐
│ zscalerctl TUI demo                                                    │
│                                                                        │
│ status: Bubble Tea running                                             │
│ terminal: 120x32                                                       │
│ style: 256-color render                                                │
│                                                                        │
│ keys: q quits | esc quits | ctrl+c quits                               │
└────────────────────────────────────────────────────────────────────────┘
```

The terminal is 120 columns, but the demo clamps the content width to 72
characters. The panel remains readable and the demo reports
`terminal: 120x32`.

## Exit behavior

Each session was started in the same 80x24 PTY and exited with a single key
press. The demo exited with status 0 in every case:

| Key | Result |
| --- | --- |
| `q` | Exited cleanly. |
| `esc` | Exited cleanly. |
| `ctrl+c` | Exited cleanly. |

The ANSI-stripped final frame is identical for all three exit keys because the
exit key is consumed before the next render.

## Terminal startup behavior

The raw recordings show that `scripts/tui-demo.go` emits the standard Bubble
Tea TUI setup and teardown sequences:

- `ESC[?25l` / `ESC[?25h` — hide/show cursor
- `ESC[?2004h` / `ESC[?2004l` — enable/disable bracketed paste
- `ESC[?1002l` / `ESC[?1003l` / `ESC[?1006l` — disable mouse tracking modes on
  exit

No OSC (`ESC]`) or device-status-report/cursor-position-report (`ESC[6n`)
probes were captured in these sessions. This run did not trigger Lip Gloss
background detection because the demo renderer explicitly pins its color
profile and does not ask `termenv` to auto-detect the terminal.

These sequences are emitted only when the isolated demo harness is started.
Normal `zscalerctl` command paths do not import `internal/tui/tea` or Bubble
Tea, so they cannot produce this output.

## Normal CLI paths remain clean

The following commands were run from a non-TTY pipe and their stdout/stderr
were inspected for terminal escape sequences. None contained any `ESC` bytes,
OSC, DSR, bracketed-paste, mouse, or cursor hide/show sequences.

### `version --format json`

```json
{
  "version": "dev",
  "commit": "unknown",
  "date": "unknown",
  "go": "go1.26.4",
  "os": "darwin",
  "arch": "arm64"
}
```

### `version --format pretty --color never`

```text
┌──────────┬──────────────┐
│ field    │ value        │
├──────────┼──────────────┤
│ Version  │ dev          │
│ Commit   │ unknown      │
│ Date     │ unknown      │
│ Go       │ go1.26.4     │
│ Platform │ darwin/arm64 │
└──────────┴──────────────┘
```

### `introspect --format json`

Produces a JSON surface map (≈360 KB in this checkout). No terminal escape
sequences were found in stdout or stderr.

## Enforcement

The import boundary is still guarded by
`scripts/verify-tui-import-boundary.sh`. As of this readback, the forbidden
packages (`cmd/`, `internal/cli/`, gate-only `internal/tui`) do not import
Bubble Tea.

## Verdict

**Continue.** The isolated demo harness renders correctly at 80x24, 60x16, and
120x32, exits cleanly on `q`, `esc`, and `ctrl+c`, and does not leak TUI
terminal sequences into normal `zscalerctl` command paths. The import boundary
from `docs/cli/tui-import-boundary.md` holds.
