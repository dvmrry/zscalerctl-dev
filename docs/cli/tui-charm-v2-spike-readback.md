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
[ZIA]  ZPA  ZCC
┌────────────────────────────┐┌──────────────────────────────────────┐┌────────────────────────────────────────────────┐
│ Resources                  ││ Records          ID        Status    ││ HQ                                             │
│ locations                  ││ HQ               123       active    ││                                                │
│ url-filtering-rules        ││ Branch           124       active    ││ id: 123                                        │
│ forwarding-rules           ││ Remote           125       inactive  ││ status: active                                 │
│ settings                   ││                                      ││ description: US East                           │
└────────────────────────────┘└──────────────────────────────────────┘└────────────────────────────────────────────────┘
zia / locations · 1/4 · 3 records
↑ move • ↓ move • ← pane • → pane • [/] product • q/esc quit • enter open • ? help
```

## 80x24

```text
[ZIA]  ZPA  ZCC
┌────────────────────────┐┌────────────────────────────────────────────────────┐
│ Resources              ││ Records                      ID          Status    │
│ locations              ││ HQ                           123         active    │
│ url-filtering-rules    ││ Branch                       124         active    │
│ forwarding-rules       ││ Remote                       125         inactive  │
│ settings               ││                                                    │
│                        ││ HQ                                                 │
│                        ││                                                    │
│                        ││ id: 123                                            │
│                        ││ status: active                                     │
│                        ││ description: US East                               │
└────────────────────────┘└────────────────────────────────────────────────────┘
zia / locations · 1/4 · 3 records
↑ move • ↓ move • ← pane • → pane • [/] product • q/esc quit • enter open …
```

## 60x16

```text
[ZIA]  ZPA  ZCC
┌──────────────────────────────────────────────────────┐
│ Resources                                            │
│ locations                                            │
│ url-filtering-rules                                  │
│ forwarding-rules                                     │
└──────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────┐
│ Records                            ID          Status    │
│ HQ                                 123         active    │
│ Branch                             124         active    │
│ Remote                             125         inactive  │
└──────────────────────────────────────────────────────┘
zia / locations · 1/4 · 3 records
↑ move • ↓ move • ← pane • → pane • [/] product …
```

## UX Notes

- Wide terminals use three panes: resources, records, detail.
- Medium terminals keep resources left and combine records/detail on the right.
- Narrow terminals stack resources over records.
- Multiple products render as a tab strip. The active tab scopes the resource
  list, so large ZIA catalogs no longer bury ZPA/ZCC resources in the same pane.
- `[` and `]` switch product tabs while left/right remains pane navigation.
- Records show name, ID, and compact Status columns; missing status renders as
  `-`.
- Left resource rows omit verbose state tags such as `[unloaded]`; the selected
  resource state remains in the status line and detail pane.
- Details own record body fields and use a Bubbles viewport for vertical scroll.
- Structured projected/redacted fields render as readable lists where possible
  rather than raw JSON or Go `map[...]` dumps.
- Left/right switch focused pane; up/down, page up/down, home/end act inside the
  focused pane.
