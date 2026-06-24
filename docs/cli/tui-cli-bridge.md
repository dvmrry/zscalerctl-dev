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
build ResourceReader (entrypoint)
  ↓
run Collector (internal/tui/browserdata)
  ↓
produce BrowserData (internal/tui/data)
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

Projected records are converted to `data.BrowserData` by the existing adapter. The
TUI model consumes `data.BrowserData` only; it never calls the reader or handles config.

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
| `internal/tui/data` | Neutral `BrowserData` view model types shared by collector and TUI. Bubble-free. |
| `internal/tui/browserdata` | Catalog filtering, reader coordination, projection, `BrowserData` conversion. Bubble-free. |
| `internal/tui/launcher` | Bubble-free gate evaluation and `BrowserData` collection. Does not import `internal/tui/tea`, Bubble Tea, or Bubbles. |
| `internal/tui/tea` | Bubble Tea model, Bubbles components, view, and update loop. Consumes `internal/tui/data` only. |
| `cmd/zscalerctl` or `internal/cli` | Normal Cobra command tree. Must not import Bubble Tea, Bubbles, or `internal/tui/tea` (verified by `go list -deps`). |
| Isolated TUI entrypoint | `cmd/zscalerctl-tui` (experimental), `scripts/tui-demo.go`, or `scripts/tui-browser-demo.go`. May import Bubble Tea, Bubbles, and `internal/tui/tea`. |

## Implementation order for the isolated TUI entrypoint

1. Add an isolated TUI entrypoint (`cmd/zscalerctl-tui` or `scripts/tui-browser-demo.go`).
   This entrypoint is the only place that may import `charm.land/bubbletea/v2`,
   `charm.land/bubbles/v2`, and `internal/tui/tea`.
2. In the entrypoint, call `tui.DecideLaunch(opts)` before any heavy work.
3. If disabled, print the reason and exit `2`.
4. If enabled, load config, resolve credentials, build the reader (or use a fake
   reader for the demo).
5. Run the collector with the requested filters and redaction mode.
6. Convert the result to `data.BrowserData`.
7. Launch `internal/tui/tea.NewBrowserModel` via `tea.NewProgram`.
8. Return the program exit code.

The normal `cmd/zscalerctl` and `internal/cli` packages must never import
Bubble Tea, Bubbles, or `internal/tui/tea`; this is enforced by `go list -deps` in
`scripts/verify-tui-import-boundary.sh`.

## Foundation promotion criteria (now satisfied)

- Isolated TUI entrypoint exists as `cmd/zscalerctl-tui`; no Bubble Tea in `cmd/zscalerctl` or `internal/cli`.
- `go list -deps ./cmd/zscalerctl` and `go list -deps ./internal/cli` must not
  include `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, or `internal/tui/tea`.
- Real TTY readbacks for `80x24`, `60x16`, and `120x32` from the isolated entrypoint.
- Evidence that normal CLI paths (non-TTY, `--format json`, `NO_COLOR`) still
  produce no Bubble Tea sequences; verified in a PTY by
  `scripts/verify-pty-escape-clean.sh`.
- Golden CLI surface tests pass.
- `scripts/verify-tui-import-boundary.sh` passes.
- No TUI command appears in generated `zscalerctl` CLI docs until intentionally promoted.

## Remaining work before a user-facing TUI feature

- At least one live-reader-backed readback on a scratch tenant with no secrets in output.
- A decision on whether the main `zscalerctl` binary should ever launch the separate TUI binary.
- Live-smoke validation of all `--products`, `--resources`, and `--continue-on-error` paths.

## Status

The experimental `zscalerctl browse --tui` command was implemented on `feature/tui`
and exercised the full real CLI path: gate → config → credentials → reader →
collector → BrowserData → launcher → Bubble Tea. However, that path transitively
linked Bubble Tea into the normal `zscalerctl` binary (`cmd/zscalerctl` and
`internal/cli`), which violates the TUI import boundary. Normal JSON/NDJSON,
completion, introspection, and machine error paths must remain independent of
TUI runtime behavior.

The blocker fix removed the hidden `browse` command from the normal command tree
and made `internal/tui/launcher` Bubble-free. An isolated experimental TUI entrypoint
now exists as `cmd/zscalerctl-tui`. It may import `internal/tui/tea`, Bubble Tea,
and Bubbles, while the normal `zscalerctl` binary and `internal/cli` remain Bubble-free. The
binary supports both fixture-only modes and a `--live` mode that loads config, resolves
credentials, builds a real Zscaler reader, and collects tenant data before launching
the TUI. Future work is to capture live-tenant readback evidence and decide whether
the main binary should ever `exec` the separate TUI binary.

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
