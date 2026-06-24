# Static TUI Browser Readback

This readback belongs to the `feature/tui` integration line. It records the
visual behavior of the static/fake-data product browser demo
(`scripts/tui-browser-demo.go`) and confirms that normal `zscalerctl` CLI paths
remain free of TUI terminal sequences.

## Scope

- Browser demo: `go run -mod=vendor ./scripts/tui-browser-demo.go`
- Model: `internal/tui/tea.BrowserModel`
- Data: hard-coded fake products, resources, and records.
- No Cobra command is added.
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

## Visual readback

### 80x24 — product selected

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ zia                                                │
│                        ││                                                    │
│ zia                    ││ Product: zia                                       │
│   locations            ││ Resources: 3                                       │
│   url-filtering-rules  ││                                                    │
│   forwarding-rules     ││                                                    │
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
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
↑/↓ move · tab switch pane · enter select · esc/q quit
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
│ zpa                    ││   Remote (id=125, status=inactive)               │
│   app-segments         ││     APAC                                           │
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
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
↑/↓ move · tab switch pane · enter select · esc/q quit
```

### 80x24 — empty resource state

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ forwarding-rules                                   │
│                        ││                                                    │
│ zia                    ││ No records                                         │
│   locations            ││                                                    │
│   url-filtering-rules  ││                                                    │
│   forwarding-rules     ││                                                    │
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
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
↑/↓ move · tab switch pane · enter select · esc/q quit
```

### 80x24 — error resource state

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ connectors                                         │
│                        ││                                                    │
│ zia                    ││ Error: connector list unavailable                  │
│   locations            ││                                                    │
│   url-filtering-rules  ││                                                    │
│   forwarding-rules     ││                                                    │
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
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
↑/↓ move · tab switch pane · enter select · esc/q quit
```

### 80x24 — right pane active (tab) with record selection

Pressing `tab` moves focus to the right pane; `down` then selects the second
record. The active pane is shown by the border color in the live session; the
frame below is the plain-text transcript.

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ url-filtering-rules                                │
│                        ││                                                    │
│ zia                    ││   Social (id=501, status=active)                   │
│   locations            ││     block social                                   │
│   url-filtering-rules  ││   Streaming (id=502, status=active)                │
│   forwarding-rules     ││     allow streaming                                │
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
│                        ││                                                    │
└────────────────────────┘└────────────────────────────────────────────────────┘
↑/↓ move · tab switch pane · enter select · esc/q quit
```

### 60x16 — stacked narrow layout

At 60 columns the browser stacks the panes vertically. The left pane is clipped
to its lower items in this capture; the right pane still shows the selected
resource details.

```text
│   forwarding-rules                                       │
│ zpa                                                      │
│   app-segments                                           │
│   connectors                                             │
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
↑/↓ move · tab switch pane · enter select · esc/q quit
```

```text
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
│     EU West                                              │
│   Remote (id=125, status=inactive)                       │
│     APAC                                                 │
└──────────────────────────────────────────────────────────┘
↑/↓ move · tab switch pane · enter select · esc/q quit
```

### 120x32 — wide layout

```text
┌──────────────────────────────────────┐┌──────────────────────────────────────────────────────────────────────────────┐
│ Products / Resources                 ││ zia                                                                          │
│                                      ││                                                                              │
│ zia                                  ││ Product: zia                                                                 │
│   locations                          ││ Resources: 3                                                                 │
│   url-filtering-rules                ││                                                                              │
│   forwarding-rules                   ││                                                                              │
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
│                                      ││                                                                              │
└──────────────────────────────────────┘└──────────────────────────────────────────────────────────────────────────────┘
↑/↓ move · tab switch pane · enter select · esc/q quit
```

```text
┌──────────────────────────────────────┐┌──────────────────────────────────────────────────────────────────────────────┐
│ Products / Resources                 ││ locations                                                                    │
│                                      ││                                                                              │
│ zia                                  ││   HQ (id=123, status=active)                                                 │
│   locations                          ││     US East                                                                  │
│   url-filtering-rules                ││   Branch (id=124, status=active)                                             │
│   forwarding-rules                   ││     EU West                                                                  │
│ zpa                                  ││   Remote (id=125, status=inactive)                                           │
│   app-segments                       ││     APAC                                                                     │
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
│                                      ││                                                                              │
└──────────────────────────────────────┘└──────────────────────────────────────────────────────────────────────────────┘
↑/↓ move · tab switch pane · enter select · esc/q quit
```

## Keyboard flow

| Sequence | Result |
| --- | --- |
| initial | `zia` product selected, right pane shows product summary. |
| `down` | `locations` selected, right pane shows records. |
| `down` | `url-filtering-rules` selected, right pane shows records. |
| `down` | `forwarding-rules` selected, right pane shows "No records". |
| `down` × 2 | `app-segments` selected, right pane shows records. |
| `down` | `connectors` selected, right pane shows error. |
| `tab` | Focus moves to right pane; border color changes. |
| `down` | Next record in the right pane is selected. |
| `q` / `esc` / `ctrl+c` | Demo exits cleanly with status 0. |

## Terminal startup behavior

The raw recordings show the same standard Bubble Tea TUI setup and teardown
sequences seen in the simple demo: cursor hide/show, bracketed paste on/off,
and mouse tracking off on exit. No OSC (`ESC]`) or
device-status-report/cursor-position-report (`ESC[6n`) probes were captured.

These sequences are emitted only by `scripts/tui-browser-demo.go`. Normal
`zscalerctl` command paths do not import `internal/tui/tea` or Bubble Tea.

## Normal CLI paths remain clean

The following commands were run from a non-TTY pipe and their stdout/stderr
were inspected for terminal escape sequences. None contained any `ESC` bytes,
OSC, DSR, bracketed-paste, mouse, or cursor hide/show sequences.

- `zscalerctl version --format json`
- `zscalerctl version --format pretty --color never`
- `zscalerctl introspect --format json`

## Import boundary

`scripts/verify-tui-import-boundary.sh` still passes. `cmd/`, `internal/cli/`,
and the gate-only `internal/tui` package do not import Bubble Tea.

## Verdict

**Continue.** The static browser shape renders correctly at 80x24, 60x16, and
120x32, supports the requested keyboard flow, and keeps the TUI isolated behind
the import boundary. The next step can connect this shape to real CLI data.
