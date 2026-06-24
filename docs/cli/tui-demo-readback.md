# TUI Demo Readback

This readback belongs to the `feature/tui` integration line. It is not a
mainline product contract.

## Scope

- Demo harness: `go run ./scripts/tui-demo.go`
- Model: `internal/tui.DemoModel`
- No Cobra command is added.
- No config is loaded.
- No credentials are resolved.
- No Zscaler client or network path is used.
- No subprocess or filesystem-write path is used by project code.

## Live Terminal Readback

The demo was run in a PTY with `NO_COLOR` unset and
`TERM=xterm-256color COLORTERM=truecolor`.

### 80x24

Command:

```sh
stty cols 80 rows 24
env -u NO_COLOR TERM=xterm-256color COLORTERM=truecolor go run ./scripts/tui-demo.go
```

Result:

- Bubble Tea started.
- The panel rendered at `terminal: 80x24`.
- The view showed `style: 256-color render`.
- Pressing `q` exited cleanly with status 0.

### 60x16

Command:

```sh
stty cols 60 rows 16
env -u NO_COLOR TERM=xterm-256color COLORTERM=truecolor go run ./scripts/tui-demo.go
```

Result:

- Bubble Tea started.
- The panel rendered at `terminal: 60x16`.
- The content fit within the narrow terminal.
- Pressing `ctrl+c` exited cleanly with status 0.
- `esc` is covered by the model-level test. The raw PTY escape byte used in
  this readback did not map to `tea.KeyEsc`, so it is not claimed as a live PTY
  exit proof.

## Disabled Paths

The demo refuses to start before creating a Bubble Tea program when the
eligibility gate is not met:

- `--color never` -> `disabled: terminal styling disabled`
- `NO_COLOR=1 --color always` -> `disabled: terminal styling disabled`
- `TERM=dumb --color always` -> `disabled: TERM=dumb`
- `--format json` -> `disabled: machine output format requested`

Non-TTY runs also refuse before launch with `disabled: stdin is not
interactive`.

## Important Finding

Bubble Tea v1.x has package initialization that calls Lip Gloss background
detection. In a real TTY, that can emit OSC/cursor-position terminal queries
before `main` runs and before `zscalerctl` can evaluate its own TUI eligibility
gate.

This is acceptable for the isolated demo harness because invoking
`scripts/tui-demo.go` is itself an explicit TUI request. It is not acceptable
for ordinary `zscalerctl` command paths. Until this is deliberately resolved,
the main CLI should not statically import Bubble Tea packages.
