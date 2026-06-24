# Static TUI Browser Readback

This readback belongs to the `feature/tui` integration line. It records the
visual behavior of the static/fake-data product browser demo
(`scripts/tui-browser-demo.go`) and confirms that normal `zscalerctl` CLI
paths remain free of TUI terminal sequences.

## Scope

- Browser demo: `go run -mod=vendor ./scripts/tui-browser-demo.go`
- Model: `internal/tui/tea.BrowserModel`
- Data: hard-coded fake products, resources, and records via `NewFakeBrowserData()`.
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
OSC, DSR, bracketed-paste, mouse, or cursor hide/show sequences.

- `zscalerctl version --format json`
- `zscalerctl version --format pretty --color never`
- `zscalerctl introspect --format json`

## Import boundary

`scripts/verify-tui-import-boundary.sh` still passes. `cmd/`, `internal/cli/`,
and the gate-only `internal/tui` package do not import Bubble Tea.

## Verdict

**Continue.** The polished fake-data browser is usable at 80x24, 60x16, and
120x32, shows explicit empty/error states, supports a help overlay, and keeps
the TUI isolated behind the import boundary. The next step can connect this
shape to real projected CLI data.
