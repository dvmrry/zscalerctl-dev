# OpenTUI agent-shell experiment

This is an unsupported interface experiment. It tests whether a
conversation-first shell can combine an agent-style transcript with a
clickable structured-data workspace before we choose a frontend direction.

It starts in a credential-free fixture mode by default. With an explicit
absolute `zscalerctl-engine` path, it instead uses the repository's existing
typed TypeScript client and immutable stdio-v1 protocol. The Go engine remains
authoritative for capabilities, resource discovery, projection, redaction,
effects, and error semantics. No model or shell execution is attached.

## What it explores

- A full-screen alternate-screen layout with a composer pinned to the bottom.
- An OpenCode-leaning visual grammar: raised user turns, flat assistant output,
  restrained spacing, and status integrated into the composer.
- A quiet responsive context rail where the independently scrollable JSON tree
  remains the dominant workspace rather than another transcript attachment.
- Mouse-wheel transcript and tree scrolling, clickable expansion carets, and
  click-to-select JSON rows.
- A reusable, viewport-clamped picker window with categories, two-line detail
  rows, stable selection IDs, contextual actions, keyboard paging, and mouse
  selection that does not jump merely because content moved under a stationary
  pointer.
- Structured search built on that picker: logical record groups, deterministic
  name/field/value ranking, fuzzy matching for names and fields only, and exact
  scalar matching inside collapsed branches. Every hit retains its exact JSON
  path.
- Transactional search navigation: moving through hits previews the tree,
  Enter commits the reveal, Ctrl+O commits into the inspector, and Escape
  restores the selection and expansion state from before search opened.
- OSC 52 copy actions for sanitized scalar values and exact JSON paths, with
  transient success or capability-failure toasts.
- Keyboard tree navigation without taking arrow keys away from the composer.
- A wide JSON inspector, slash-command palette, Hangul activity frames, and
  OpenCode's 33-theme catalog plus four local compatibility themes.
- JSON theme references, ANSI colors, transparent colors, dark/light variants,
  and automatic appearance selection from the terminal background.
- Exact JSON number lexemes through the existing TypeScript client's
  `WireNumber`, including values larger than JavaScript's safe integer range.
- Config-free engine bootstrap and catalog discovery, a searchable resource
  picker, typed `list`/`get`/`show` reads, safe status views, URL lookup, and
  local sanitized-dump comparison.
- One active operation at a time, progress in the context rail, Ctrl+C or
  `/cancel` cancellation, and orderly child-process shutdown.

This is a UI vertical slice, not a second client implementation. The React
shell receives data through a project-neutral `WorkspaceAdapter`; fixture and
zscalerctl stdio adapters implement that boundary independently.

## Requirements

- Bun 1.4 or newer.
- An interactive terminal with mouse reporting and Unicode support.
- For engine mode, a locally built `zscalerctl-engine` executable at an
  absolute path.

The initial verification used the Rust-based Bun build
`1.4.0-canary.1+88403d981`.

## Run it

```sh
cd experiments/opentui-agent-tui
bun install --frozen-lockfile
bun start
```

Try the real config-free engine discovery surface without credentials:

```sh
go build -o /tmp/zscalerctl-engine ./cmd/zscalerctl-engine
cd experiments/opentui-agent-tui
bun start -- --engine /tmp/zscalerctl-engine
```

`ZSCALERCTL_ENGINE_PATH=/absolute/path/to/zscalerctl-engine` is equivalent.
Use `--fixture` to ignore that environment variable temporarily. Engine mode
inherits `ZSCALERCTL_*` variables from its process environment; credentials are
never accepted as arguments or sent through a separate credential API.

Optional engine policy flags are `--profile`, `--config`, `--timeout`,
`--redaction standard|share|paranoid`, and `--no-cache`. As elsewhere in this
repository, agents should prefer injected `ZSCALERCTL_*` environment variables
over profiles.

Start with a different theme:

```sh
bun start -- --theme tron
bun start -- --theme tokyonight --theme-mode light
```

The default is `tokyonight` with automatic dark/light detection. Use
`--theme-mode auto|dark|light` to control appearance explicitly. Run `/theme`
(or `/theme list`) inside the TUI to browse and filter all 37 themes in the
floating picker; `/theme mode toggle` switches the current theme between dark
and light without restarting.

## Controls

| Input | Action |
|---|---|
| `/` | Open the command menu; keep typing to narrow it |
| Up / Down | Choose a slash suggestion or move through the tree when it has focus |
| Tab | Accept the selected autocomplete suggestion |
| Shift+Tab | Cycle focus between composer, transcript, and tree |
| Enter | Accept a partial suggestion or submit the composer |
| Shift+Enter | Insert a composer newline |
| Ctrl+B | Toggle the context rail |
| Ctrl+F | Open structured-data search |
| Ctrl+O | Toggle the wide JSON inspector |
| Up / Down while searching | Preview the previous / next exact match |
| Page Up / Page Down while searching | Move through search results five at a time |
| Home / End while searching | Preview the first / last search result |
| Enter while searching | Commit the previewed result and close search |
| Ctrl+O while searching | Commit the result and open it in the inspector |
| Shift+C / Shift+P while searching | Copy the sanitized scalar value / exact JSON path with OSC 52 |
| Escape | Close an overlay or return focus to the composer |
| Mouse wheel | Scroll the region under the pointer |
| Click row | Select a JSON value |
| Click caret | Expand or collapse an object or array |
| S while tree is focused | Toggle named-array ordering between source index and name |
| Page Up / Page Down | Move through the focused tree eight rows at a time |
| Ctrl+C | Cancel the active engine operation; exit and restore the terminal when idle |
| `/quit` | Close the engine, exit, and restore the terminal |

Local commands include `/demo`, `/help`, `/clear`, `/find`, `/theme`, `/sort`,
`/sidebar`, `/inspect`, `/cancel`, and `/quit`; `/demo` appears only in fixture
mode. Engine mode adds `/manifest`, `/catalog`, `/doctor`, `/auth`, `/config`,
`/lookup`, `/list`, `/get`, `/show`, and `/diff`. Theme examples include
`/theme next`, `/theme github light`, and `/theme mode auto`.

## Verify it

```sh
bun run check
```

To include the real config-free process integration test:

```sh
go build -o /tmp/zscalerctl-engine ./cmd/zscalerctl-engine
ZSCALERCTL_ENGINE_TEST_BINARY=/tmp/zscalerctl-engine bun run check
```

The experiment has its own dependency lock and checks. It remains outside the
repository's supported release surface and root Go gate suite.

## Current boundaries

- The tree displays at most 800 visible rows so an accidentally large result
  cannot overwhelm one render pass.
- Search visits at most 5,000 nodes and retains at most 200 matches. It searches
  only the sanitized tree presented to the frontend; it cannot widen the Go
  engine's projection or recover dropped fields. Approximate matching is
  intentionally limited to record names and field names; scalar values require
  an exact substring so a fuzzy search cannot produce surprising data hits.
- Display strings strip terminal controls and bidirectional formatting marks.
- Engine mode requires an absolute executable path, launches without a shell,
  drains but never presents child stderr, and renders normalized failure
  categories rather than backend error prose. Only validated missing
  `ZSCALERCTL_*` variable names may be surfaced.
- `/dump` is deliberately absent: it has local write/delete effects and stdio
  v1 has no wire-visible publication commit marker. `/diff` reads two existing
  sanitized dump directories but does not contact Zscaler.
- Live reads can perform network access and configuration-dependent local file,
  keyring, or operator-configured `cmd:` provider access. The context rail
  states those effects; set `ZSCALERCTL_DISALLOW_CMD=true` when process-backed
  secret providers should be forbidden.
- The transcript is session-memory only and currently unbounded.
- The context rail becomes an overlay on narrow terminals and remains inline
  on wide terminals.
- Floating windows, drawers, and modal inspectors share explicit overlay
  layers; search remains non-modal so the tree behind it stays explorable.
- Selection is local UI state; no operation is dispatched from a tree click.
- Clipboard actions use the terminal's OSC 52 capability and fail visibly when
  the terminal declines it; they never invoke a platform clipboard process.
- The generic registry can accept additional validated theme definitions, but
  filesystem discovery for custom themes is intentionally not wired into this
  fixture-only experiment yet.

The vendored OpenCode theme assets and their MIT attribution are recorded in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
