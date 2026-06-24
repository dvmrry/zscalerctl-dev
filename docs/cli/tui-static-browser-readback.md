# Static TUI Browser Readback

This readback belongs to the `feature/tui` integration line. It records the
visual behavior of the static/fake-data product browser demo
(`scripts/tui-browser-demo.go` and `cmd/zscalerctl-tui`) and confirms that normal
`zscalerctl` CLI paths remain free of TUI terminal sequences.

## Scope

- Browser demo: `go run -mod=vendor ./scripts/tui-browser-demo.go` (default), or with `--projected-fixture`, or with `--collector-fixture`.
- Standalone binary: `go run -mod=vendor ./cmd/zscalerctl-tui` (default), or with `--fixture`, or with `--collector-fixture`.
- Model: `internal/tui/tea.BrowserModel`.
- Default data: hard-coded fake products, resources, and records via `NewFakeBrowserData()`.
- Projected data: `internal/tui/browserdata.Build` adapts fake projected records into `BrowserData`.
- Collector data: `internal/tui/browserdata.Collector` coordinates fake reader, projection, and `Build` into `BrowserData`.
- Experimental command: `zscalerctl browse --tui` is a hidden, fixture-backed command that exercises the real Cobra → `internal/tui/launcher` → Bubble Tea path.
- No Cobra command is added to normal `zscalerctl` usage (the command is hidden and requires `--tui`).
- No config is loaded.
- No credentials are resolved.
- No Zscaler client or network path is used.
- No subprocess or filesystem-write path is used by project code.

## Method

All captures were produced in a real PTY with `TERM=xterm-256color`,
`COLORTERM=truecolor`, and `NO_COLOR` unset. The terminal dimensions were set
with `TIOCSWINSZ` before spawning the demo, and the final rendered frame was
captured after each key sequence. The transcripts below are the ANSI-stripped
final frames from the captured sessions.

## Viewport stabilization update

The browser model now uses explicit left and right pane viewports. Long resource
catalogs render only visible left-pane rows, long record lists render only
visible right-pane records, and `pgup`/`pgdown`/`home`/`end` clamp selection and
offsets after resize. The code-level readback for this update covers a
200-resource catalog, a 1000-record resource, 120x32 to 60x16 resize, long field
value truncation, and unloaded/loading/error states in small geometry.

The static fixture captures below are retained as baseline visual evidence for
the isolated browser shape. Current footer text includes page/home/end
navigation and may differ from the older captured footer line.

## Visual readback

### 80x24 — product selected

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ zia                                                │
│                        ││                                                    │
│ zia                    ││ Product: zia                                       │
│   locations            ││ Resources: 4                                       │
│   url-filtering-rules  ││                                                    │
│   forwarding-rules     ││                                                    │
│   settings             ││                                                    │
│ zpa                    ││                                                    │
│   app-segments         ││                                                    │
│   connectors           ││                                                    │
│ zcc                    ││                                                    │
│   devices              ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
zia · 1/10
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

### 80x24 — resource selected with records

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ locations                                          │
│                        ││                                                    │
│ zia                    ││   HQ (id=123, status=active)                       │
│   locations            ││     US East                                        │
│   url-filtering-rules  ││   Branch (id=124, status=active)                   │
│   forwarding-rules     ││     EU West                                        │
│   settings             ││   Remote (id=125, status=inactive)                 │
│ zpa                    ││     APAC                                           │
│   app-segments         ││                                                    │
│   connectors           ││                                                    │
│ zcc                    ││                                                    │
│   devices              ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
zia / locations · 2/10 · 3 records
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

### 80x24 — empty resource state

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ forwarding-rules                                   │
│                        ││                                                    │
│ zia                    ││                                                    │
│   locations            ││ No records for this resource                       │
│   url-filtering-rules  ││                                                    │
│   forwarding-rules     ││ Select a different resource to browse data.        │
│   settings             ││                                                    │
│ zpa                    ││                                                    │
│   app-segments         ││                                                    │
│   connectors           ││                                                    │
│ zcc                    ││                                                    │
│   devices              ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
zia / forwarding-rules · 4/10 · 0 records
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

### 80x24 — error resource state

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ connectors                                         │
│                        ││                                                    │
│ zia                    ││                                                    │
│   locations            ││ Error loading resource                             │
│   url-filtering-rules  ││ connector list unavailable                         │
│   forwarding-rules     ││                                                    │
│   settings             ││ Press enter to retry from the top of this list.    │
│ zpa                    ││                                                    │
│   app-segments         ││                                                    │
│   connectors           ││                                                    │
│ zcc                    ││                                                    │
│   devices              ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
zpa / connectors · 8/10 · 0 records
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

### 80x24 — long record (detail pane scroll)

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ settings                                           │
│                        ││                                                    │
│ zia                    ││   Global Policy (id=7001, status=active)           │
│   locations            ││     default tenant policy                          │
│   url-filtering-rules  ││     mode: strict                                   │
│   forwarding-rules     ││     log_level: info                                │
│   settings             ││     timeout: 30s                                   │
│ zpa                    ││     retries: 3                                     │
│   app-segments         ││     region: us-east                                │
│   connectors           ││     fail_open: false                               │
│ zcc                    ││     notify: true                                   │
│   devices              ││     audit: enabled                                 │
│                        ││     last_updated: 2024-06-24                       │
│                        ││     updated_by: admin                              │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
zia / settings · 5/10 · 1 records
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

After moving focus to the right pane and pressing `down`, the detail pane
scrolls so the selected record remains at the top of the visible area:

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││   Global Policy (id=7001, status=active)           │
│                        ││     default tenant policy                          │
│ zia                    ││     mode: strict                                   │
│   locations            ││     log_level: info                                │
│   url-filtering-rules  ││     timeout: 30s                                   │
│   forwarding-rules     ││     retries: 3                                     │
│   settings             ││     region: us-east                                │
│ zpa                    ││     fail_open: false                               │
│   app-segments         ││     notify: true                                   │
│   connectors           ││     audit: enabled                                 │
│ zcc                    ││     last_updated: 2024-06-24                       │
│   devices              ││     updated_by: admin                              │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
zia / settings · 5/10 · 1 records
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

### 80x24 — help overlay

```text

                    ┌──────────────────────────────────────┐
                    │                                      │
                    │  Keyboard help                       │
                    │                                      │
                    │  ↑ / down    move selection          │
                    │  tab         switch active pane      │
                    │  enter       reset record selection  │
                    │  ?           toggle this help        │
                    │  q / esc     quit                    │
                    │  ctrl+c      quit                    │
                    │                                      │
                    │  Press any key to close.             │
                    │                                      │
                    └──────────────────────────────────────┘


zia · 1/10
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

### 60x16 — stacked narrow layout

At 60 columns the browser stacks the panes vertically. The status bar and footer
fit without wrapping, and the right pane still shows the selected resource
details.

```text
│   app-segments                                           │
│   connectors                                             │
│ zcc                                                      │
│   devices                                                │
└──────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────┐
│ zia                                                      │
│                                                          │
│ Product: zia                                             │
│ Resources: 4                                             │
│                                                          │
└──────────────────────────────────────────────────────────┘
zia · 1/10
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

```text
│   app-segments                                           │
│   connectors                                             │
│ zcc                                                      │
│   devices                                                │
└──────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────┐
│ locations                                                │
│                                                          │
│   HQ (id=123, status=active)                             │
│     US East                                              │
│   Branch (id=124, status=active)                         │
└──────────────────────────────────────────────────────────┘
zia / locations · 2/10 · 3 records
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

### 120x32 — wide layout

```text
┌──────────────────────────────────────┐┌──────────────────────────────────────────────────────────────────────────────┐
│ Products / Resources                 ││ zia                                                                          │
│                                      ││                                                                              │
│ zia                                  ││ Product: zia                                                                 │
│   locations                          ││ Resources: 4                                                                 │
│   url-filtering-rules                ││                                                                              │
│   forwarding-rules                   ││                                                                              │
│   settings                           ││                                                                              │
│ zpa                                  ││                                                                              │
│   app-segments                       ││                                                                              │
│   connectors                         ││                                                                              │
│ zcc                                  ││                                                                              │
│   devices                            ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
└──────────────────────────────────────┘└──────────────────────────────────────────────────────────────────────────────┘
zia · 1/10
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

```text
┌──────────────────────────────────────┐┌──────────────────────────────────────────────────────────────────────────────┐
│ Products / Resources                 ││ locations                                                                    │
│                                      ││                                                                              │
│ zia                                  ││   HQ (id=123, status=active)                                                 │
│   locations                          ││     US East                                                                  │
│   url-filtering-rules                ││   Branch (id=124, status=active)                                             │
│   forwarding-rules                   ││     EU West                                                                  │
│   settings                           ││   Remote (id=125, status=inactive)                                           │
│ zpa                                  ││     APAC                                                                     │
│   app-segments                       ││                                                                              │
│   connectors                         ││                                                                              │
│ zcc                                  ││                                                                              │
│   devices                            ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
│                                      ││                                                                              │
└──────────────────────────────────────┘└──────────────────────────────────────────────────────────────────────────────┘
zia / locations · 2/10 · 3 records
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

## Keyboard flow

| Sequence | Result |
| --- | --- |
| initial | `zia` product selected, status bar shows `zia · 1/10`. |
| `down` | `locations` selected, right pane shows records, status bar shows `zia / locations · 2/10 · 3 records`. |
| `down` | `url-filtering-rules` selected. |
| `down` | `forwarding-rules` selected, right pane shows empty-state message. |
| `down` | `settings` selected, right pane shows long record. |
| `tab` | Focus moves to right pane; border color changes. |
| `down` | Right pane scrolls to keep the selected record visible. |
| `?` | Help overlay appears; `q` or `esc` dismisses it and quits. |
| `q` / `esc` / `ctrl+c` | Demo exits cleanly with status 0. |

## Fixture demo readback (`scripts/tui-browser-demo.go --collector-fixture`)

The standalone binary (`cmd/zscalerctl-tui`) and the standalone demo script are
the fastest ways to exercise the browser shape without config or credentials.
With `--collector-fixture` they run the same `internal/tui/browserdata.Collector`
path used by the removed `browse --tui` command, but backed
by a fake reader. The readbacks below were produced by spawning the built demo
binary in a real PTY and pressing `q`.

### 80x24 — initial frame

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ zia                                                │
│                        ││                                                    │
│ zia                    ││ Product: zia                                       │
│   locations            ││ Resources: 3                                       │
│   url-filtering-rules  ││                                                    │
│   forwarding-rules     ││                                                    │
│ zpa                    ││                                                    │
│   application-segments ││                                                    │
│   app-connectors       ││                                                    │
│ zcc                    ││                                                    │
│   devices              ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
zia · 1/9
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

### 60x16 — initial frame

```text
│ zpa                                                      │
│   application-segments                                   │
│   app-connectors                                         │
│ zcc                                                      │
│   devices                                                │
└──────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────┐
│ zia                                                      │
│                                                          │
│ Product: zia                                             │
│ Resources: 3                                             │
│                                                          │
└──────────────────────────────────────────────────────────┘
zia · 1/9
↑/↓ move · tab switch · enter select · ? help · esc/q quit
```

## Normal `zscalerctl` CLI boundary

The hidden `zscalerctl browse --tui` command has been removed from the normal
command tree. Bubble Tea v1.x runs package-init terminal probing that can emit
OSC/DSR sequences before `main()`, so the TUI runtime must not be linked into the
normal `zscalerctl` binary. The gate/collector path (`internal/tui/launcher`)
remains available, but the actual Bubble Tea launch is restricted to isolated TUI
entrypoints such as `cmd/zscalerctl-tui` or `scripts/tui-browser-demo.go`.

Rejection readbacks were captured before the command was removed; they now live
only in the integration history. The important invariant is that no normal
`zscalerctl` invocation can reach Bubble Tea.

## Terminal startup behavior

The raw recordings show the same standard Bubble Tea TUI setup and teardown
sequences: cursor hide/show, bracketed paste on/off, and mouse tracking off on
exit. No OSC (`ESC]`) or
device-status-report/cursor-position-report (`ESC[6n`) probes were captured.

These sequences are emitted only by `scripts/tui-browser-demo.go`. Normal
`zscalerctl` command paths do not import `internal/tui/tea` or Bubble Tea.

## Normal CLI paths remain clean

The following commands were run from a non-TTY pipe and their stdout/stderr
were inspected for terminal escape sequences. None contained any `ESC` bytes,
OSC, DSR, bracketed-paste, mouse, or cursor hide/show sequences. In addition,
`zscalerctl version --format json` was run inside a real PTY and confirmed to
emit zero `ESC` bytes and valid JSON, proving that the normal binary is not
linked with Bubble Tea.

- `zscalerctl version --format json` (non-TTY pipe and PTY)
- `zscalerctl version --format pretty --color never`
- `zscalerctl introspect --format json`

## Import boundary

`scripts/verify-tui-import-boundary.sh` still passes. `cmd/`, `internal/cli/`,
and the gate-only `internal/tui` package do not import Bubble Tea.

## Verdict

**Continue with an isolated TUI entrypoint.** The fake-data browser is usable at
80x24, 60x16, and 120x32, shows explicit empty/error states, supports a help
overlay, and keeps the TUI isolated behind the import boundary. The Bubble Tea
runtime must remain outside the normal `zscalerctl` binary because Bubble Tea
v1.x package-init probing can corrupt interactive JSON output. The hidden
`browse --tui` command has been removed; the TUI is now exposed only through an
isolated entrypoint (`cmd/zscalerctl-tui` or `scripts/tui-browser-demo.go`) that
imports `internal/tui/tea` and Bubble Tea. The remaining work is to add live
config, credential, and reader support to `cmd/zscalerctl-tui` and capture a
live-tenant readback.
