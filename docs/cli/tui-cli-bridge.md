# TUI CLI Bridge Design

This document defines the command-line boundary for the TUI feature on the
`feature/tui` integration branch. It exists so that the next wiring PR does not
have to simultaneously invent:

- the user-facing request shape,
- eligibility gating,
- stream ownership,
- config/credential/reader construction,
- collection and error handling,
- Bubble Tea launch semantics.

The design is intentionally conservative. The goal is to merge a hidden or
narrow experimental command into `feature/tui`, prove it on a real TTY without
disturbing the existing CLI surface, and only then promote it.

## Scope

This bridge applies to the first user-facing TUI command. It is not a generic
wrapper around every `zscalerctl` list/get/show command. It is a dedicated
entry point that exists solely to launch the interactive browser.

## Request shape

Options considered:

| Shape | Pros | Cons |
| --- | --- | --- |
| `zscalerctl --tui` | Global flag, easy to discover | Pollutes every command; hard to gate per-command |
| `zscalerctl zia locations list --tui` | Contextual | Too broad; every resource would need TUI semantics |
| `zscalerctl tui browse` | Explicit namespace | New top-level command, still experimental |
| `zscalerctl browse --tui` | Clear purpose, narrow surface | Still a new top-level command |
| `zscalerctl experimental tui` | Safest for early iteration | Hidden, may be moved later |

**Recommended first shape:** `zscalerctl browse --tui`.

Rationale: the command name describes exactly what it does (browse tenant
inventory), the `--tui` flag is explicit, and the surface is narrow enough to
gate without affecting normal `list`/`get`/`show` execution. If the feature
stays experimental for a long time, it can be renamed or hidden behind
`zscalerctl experimental tui` before promotion.

## TUI eligibility timing

TUI eligibility is evaluated **before** any config is loaded, any credential is
resolved, or any network call is made. The evaluation is pure: it only inspects
terminal state, format flags, output flags, and environment variables.

The decision function lives in `internal/tui` and is Bubble-free. It returns a
structured `LaunchDecision`:

```go
type LaunchDecision struct {
    Enabled bool
    Reason  string
}
```

A disabled decision becomes a user-friendly error message on stderr and a
non-zero exit code. The program never imports or initializes Bubble Tea on a
disabled path.

### Launch gates

The TUI is enabled only when **all** of the following are true:

- The user explicitly requested it (`--tui` on the browse command).
- `stdin` is an interactive TTY.
- `stdout` is an interactive TTY.
- `stderr` is an interactive TTY.
- The active format is not `json` or `ndjson`.
- No `--output` path was supplied.
- Color is not disabled via `--color never`, `NO_COLOR`, or `TERM=dumb`.

If any gate fails, the program exits with a clear reason before touching config,
credentials, or the network.

### Format and output behavior

- The TUI is **not** a format. `--format pretty` is allowed but irrelevant; the
  TUI renders its own view model.
- `--format json` and `--format ndjson` are rejected.
- `--output <path>` is rejected because the TUI owns the terminal streams.
- Piped output always disables the TUI because `stdout` is no longer a TTY.

### Color behavior

- `--color auto` is the default and works when `stdout` is a TTY.
- `--color always` is allowed only if the TTY gates also pass; it does not
  override the non-TTY gate.
- `--color never`, `NO_COLOR`, and `TERM=dumb` disable the TUI because the
  browser relies on styled terminal rendering.

## Stream ownership

Bubble Tea must own the terminal streams only after **all** of the following have
succeeded:

1. Eligibility check passes.
2. Global flags are parsed and validated.
3. Config is loaded (if needed) and is valid.
4. Credentials are resolved (if needed) and are valid.
5. A `ResourceReader` is constructed.
6. Collection produces a `tea.BrowserData` (possibly with resource-level errors
   if `ContinueOnError` is true).

No Bubble Tea package is imported or initialized on any path where the TUI is not
requested. The launch layer must live in a package that is only reached when the
TUI command is selected, to avoid the Lip Gloss background-detection probe on
every `zscalerctl` invocation.

## Config, credentials, and reader construction

The bridge uses the same construction path as normal read commands:

```
parse global flags
  ↓
load config (internal/config)
  ↓
resolve credentials (internal/credentials)
  ↓
build ResourceReader (internal/cli App or equivalent)
  ↓
run Collector (internal/tui/browserdata)
  ↓
launch Bubble Tea (internal/tui/tea)
```

The construction layer is responsible for the same error contracts as normal
CLI commands: missing credentials exit `3`, unsupported resources exit `4`, live
API failures exit `5`, etc. The TUI is only launched when construction and
initial collection succeed.

For the first wiring PR, the command should support `--products` and
`--resources` filters that mirror the collector's `CollectOptions`, so the user
can narrow the browser scope before it starts.

## Collection and projection

Collection is performed before the TUI starts. The collector (`internal/tui/browserdata.Collector`) is configured with:

- the `ResourceCatalog` (full or filtered),
- the `ResourceReader`,
- the active redaction mode (`--redaction`, default `standard`),
- `ContinueOnError` behavior (default `false` for the first command).

Projected records are converted to `tea.BrowserData` by the existing adapter. The
TUI model consumes `BrowserData` only; it never calls the reader or handles config.

## Error handling

Three error classes:

1. **Launch errors** (eligibility, flag misuse, construction failure): reported
   on stderr as a JSON error envelope and a non-zero exit code. The TUI does not
   start.
2. **Collection errors with `ContinueOnError=false`**: the first resource error
   becomes a launch error.
3. **Collection errors with `ContinueOnError=true`**: each failing resource
   becomes an error-state card inside the browser. The user can still browse the
   resources that succeeded.

For the first command, the default should be `ContinueOnError=false` so that
configuration and entitlement problems surface immediately. A future flag can
enable in-browser error cards.

## Cancellation

The launch layer passes a `context.Context` to the collector. Cancellation is
honored at resource boundaries. If the user presses `ctrl+c` after the TUI has
started, Bubble Tea's own quit path takes over. The TUI must not perform
blocking network calls inside its `Update` or `View` methods; all collection
happens before `tea.NewProgram(...).Run()`.

## Exit behavior

- `0` — TUI ran and the user exited normally (`q`, `esc`, `ctrl+c`).
- `2` — usage error or launch ineligibility.
- `3` — credentials missing/invalid.
- `4` — unsupported resource or product.
- `5` — live API failure during collection.

The TUI itself does not introduce new exit codes beyond the normal CLI set.

## Non-goals

- **No Fang integration.** The TUI does not get a special configuration section.
- **No Cobra execution wrapper.** The TUI is a dedicated command, not a mode
  that wraps arbitrary commands.
- **No machine output.** `json` and `ndjson` are rejected for the TUI command.
- **No background/network inside Bubble Tea.** The first version collects data
  before launching the TUI.
- **No auto-launch.** The TUI starts only when the user explicitly requests it.

## Internal package responsibilities

| Package | Responsibility |
| --- | --- |
| `internal/tui` | Pure, Bubble-free launch eligibility and decision helpers. |
| `internal/tui/browserdata` | Catalog filtering, reader coordination, projection, `BrowserData` conversion. |
| `internal/tui/tea` | Bubble Tea model, view, and update loop. Consumes `BrowserData` only. |
| `cmd/zscalerctl` or `internal/cli` | Cobra command registration, global flag parsing, config/credential/reader construction, TUI launch orchestration. |

## Implementation order for the wiring PR

1. Add the `browse` command with a `--tui` flag (or hidden `experimental tui`).
2. In the command runner, call `tui.DecideLaunch(opts)` before any heavy work.
3. If disabled, print the reason and exit `2`.
4. If enabled, load config, resolve credentials, build the reader.
5. Run the collector with the requested filters and redaction mode.
6. Convert the result to `tea.BrowserData`.
7. Launch `internal/tui/tea.NewBrowserModel` via `tea.NewProgram`.
8. Return the program exit code.

## Promotion criteria before `feature/tui` merges to `main`

- Hidden or clearly experimental command shape.
- Real TTY readbacks for `80x24`, `60x16`, and `120x32`.
- Evidence that normal CLI paths (non-TTY, `--format json`, `NO_COLOR`) still
  produce no Bubble Tea sequences.
- At least one live-reader-backed readback on a scratch tenant with no secrets in
  output.
- Golden CLI surface tests pass.
- `scripts/verify-tui-import-boundary.sh` passes.
- No TUI command appears in generated CLI docs until intentionally promoted.

## Status

The first experimental wiring (`zscalerctl browse --tui`) is implemented on
`feature/tui` as a fixture-backed command. It validates the launch gates, stream
ownership, and exit behavior without config, credentials, or live network.

## Decision table

| Condition | TUI enabled | Reason |
| --- | --- | --- |
| `--tui` not set | no | `tui not requested` |
| stdin not TTY | no | `stdin is not interactive` |
| stdout not TTY | no | `stdout is not interactive` |
| stderr not TTY | no | `stderr is not interactive` |
| `--output <path>` | no | `output path is not supported for TUI` |
| `--format json` | no | `machine output format requested` |
| `--format ndjson` | no | `machine output format requested` |
| `--color never` | no | `terminal styling disabled` |
| `NO_COLOR` set | no | `terminal styling disabled` |
| `TERM=dumb` | no | `TERM=dumb` |
| all above false | yes | — |
