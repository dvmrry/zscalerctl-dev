# Charm v2 TUI Spike Readback

This readback captures the `feature/tui-charm-v2-spike` behavior for reviewer
orientation. The spike uses fixture data only for these renders: no config,
credentials, network, subprocess, or filesystem-write paths are involved.

## Gate Readback

`zscalerctl-tui --fixture --color never` exits before Bubble Tea launch:

```text
zscalerctl-tui: disabled: terminal styling disabled
```

This preserves the current styled-only TUI policy.

## PTY Startup

`zscalerctl-tui --fixture` was run in a real pseudo-terminal at `120x32`,
`80x24`, and `60x16`, then quit with `q`.

```text
120x32: exit=0, TUI painted, q quit
80x24:  exit=0, TUI painted, q quit
60x16:  exit=0, TUI painted, q quit
```

Raw PTY output contains normal terminal control sequences while the TUI is
active. The normal CLI PTY guard remains separate and verifies `zscalerctl`
JSON output has zero ESC bytes.

## 120x32

```text
┌────────────────────────────┐┌──────────────────────────────────────┐┌────────────────────────────────────────────────┐
│ Products / Resources       ││ Records          ID        Status    ││ zia                                            │
│ zia                        ││ Select a resource                    ││                                                │
│   locations                ││                                      ││ Product: zia                                   │
│   url-filtering-rules      ││                                      ││ Resources: 4                                   │
│   forwarding-rules         ││                                      ││                                                │
│   settings                 ││                                      ││                                                │
│ zpa                        ││                                      ││                                                │
│   app-segments             ││                                      ││                                                │
│   connectors               ││                                      ││                                                │
│ zcc                        ││                                      ││                                                │
│   devices                  ││                                      ││                                                │
└────────────────────────────┘└──────────────────────────────────────┘└────────────────────────────────────────────────┘
zia · 1/10
↑ move • ↓ move • ← pane • → pane • q/esc quit • enter open • ? help
```

## 80x24

```text
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Products / Resources   ││ Records                      ID          Status    │
│ zia                    ││ Select a resource                                  │
│   locations            ││                                                    │
│   url-filtering-rules  ││                                                    │
│   forwarding-rules     ││                                                    │
│   settings             ││                                                    │
│ zpa                    ││                                                    │
│   app-segments         ││                                                    │
│   connectors           ││ zia                                                │
│ zcc                    ││                                                    │
│   devices              ││ Product: zia                                       │
│                        ││ Resources: 4                                       │
└────────────────────────┘└────────────────────────────────────────────────────┘
zia · 1/10
↑ move • ↓ move • ← pane • → pane • q/esc quit • enter open • ? help
```

## 60x16

```text
┌──────────────────────────────────────────────────────┐
│ Products / Resources                                 │
│ zia                                                  │
│   locations                                          │
│   url-filtering-rules                                │
└──────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────┐
│ Records                            ID          Status    │
│ Select a resource                                    │
│                                                      │
│                                                      │
│                                                      │
└──────────────────────────────────────────────────────┘
zia · 1/10
↑ move • ↓ move • ← pane • → pane • q/esc quit …
```

## UX Notes

- Wide terminals use three panes: resources, records, detail.
- Medium terminals keep resources left and combine records/detail on the right.
- Narrow terminals stack resources over records.
- Records show name, ID, and compact Status columns; missing status renders as
  `-`.
- Left resource rows omit verbose state tags such as `[unloaded]`; the selected
  resource state remains in the status line and detail pane.
- Details own record body fields and use a Bubbles viewport for vertical scroll.
- Structured projected/redacted fields render as readable lists where possible
  rather than raw JSON or Go `map[...]` dumps.
- Left/right switch focused pane; up/down, page up/down, home/end act inside the
  focused pane.
