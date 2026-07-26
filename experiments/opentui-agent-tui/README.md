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
- A zscalerctl-specific visual grammar: raised operator turns, flat assistant
  output, restrained spacing, and tenant-read-only status integrated into the
  composer.
- A responsive Poison FIGlet `zscalerctl` identity on roomy terminals, with a
  short color sweep that never changes the banner's text or cell geometry.
- A quiet responsive context rail where the independently scrollable JSON tree
  remains the dominant workspace rather than another transcript attachment.
- Typed result cards that turn adapter-owned command metadata into bounded
  metrics, facets, notes, evidence links, and contextual actions. Catalog,
  manifest, status, read, lookup, and diff results each summarize only semantics
  their adapter can substantiate; arbitrary record fields are not promoted into
  inferred insights.
- Immutable in-memory result snapshots behind every evidence link, so a click
  on an older transcript result restores the exact data and context it came
  from rather than applying a stale path to the newest result.
- A compact working set above the composer with up to eight manually pinned
  evidence references. Pins survive result changes without copying whole result
  trees and can be activated or removed with the mouse.
- Mouse-wheel transcript and tree scrolling, clickable expansion carets, and
  click-to-select JSON rows.
- A reusable, viewport-clamped picker window with categories, two-line detail
  rows, stable selection IDs, contextual actions, keyboard paging, and mouse
  selection that does not jump merely because content moved under a stationary
  pointer.
- A product-first resource map that exposes every discovered Zscaler product
  and readable-resource count before scrolling. Search stays inside the active
  product; product pills support mouse selection and Tab / Shift+Tab cycling.
- Structured search built on that picker: logical record groups, deterministic
  name/field/value ranking, fuzzy matching for names and fields only, and exact
  scalar matching inside collapsed branches. Every hit retains its exact JSON
  path.
- Transactional search navigation: moving through hits previews the tree,
  Enter commits the reveal, Ctrl+O commits into the inspector, and Escape
  restores the selection and expansion state from before search opened.
- OSC 52 copy actions for sanitized scalar values and exact JSON paths, with
  latest-action toasts whose lifetime reflects informational versus warning
  severity.
- Keyboard tree navigation without taking arrow keys away from the composer.
- A wide JSON inspector, slash-command palette, configurable activity frames
  with a Hangul default, and OpenCode's 33-theme catalog plus four local
  compatibility themes.
- JSON theme references, ANSI colors, transparent colors, dark/light variants,
  and automatic appearance selection from the terminal background.
- Exact JSON number lexemes through the existing TypeScript client's
  `WireNumber`, including values larger than JavaScript's safe integer range.
- Config-free engine bootstrap and catalog discovery, a searchable resource
  picker, typed `list`/`get`/`show` reads, safe status views, URL lookup, and
  local sanitized-dump comparison.
- One active operation at a time, truthful completed-work progress in the
  context rail, Ctrl+C or `/cancel` cancellation, and orderly child-process
  shutdown. Activity frames indicate liveness but never invent elapsed
  progress.
- A delayed, data-reactive operation scene in the transcript that uses only the
  real transport, authority, operation label, and completed-work counter. Full,
  reduced, and off motion policies share one bounded clock.

This is a UI vertical slice, not a second client implementation. The React
shell receives data through a project-neutral `WorkspaceAdapter`; fixture and
zscalerctl stdio adapters implement that boundary independently.

The shell borrows useful terminal interaction patterns from several agent
interfaces, but its information architecture follows zscalerctl: product
scope, read surface, projected fields, local effects, and sanitized tenant
data are the primary concepts.

## Requirements

- Bun 1.4 canary or newer. Until Bun 1.4 has a stable release, the experiment
  identifies its runtime line as `1.4.0-canary.1`; CI pins an official Bun
  container by immutable OCI digest and verifies the exact runtime revision.
- An interactive terminal with mouse reporting and Unicode support.
- For engine mode, a locally built `zscalerctl-engine` executable at an
  absolute path.

The initial local verification used the Rust-based Bun build
`1.4.0-canary.1+88403d981`. CI uses `1.4.0-canary.1+ae4b17de6`.

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

Start with a different theme or activity style:

```sh
bun start -- --theme tron
bun start -- --theme tokyonight --theme-mode light
bun start -- --motion reduced
bun start -- --spinner braille
```

The default is `tokyonight` with automatic dark/light detection. Use
`--theme-mode auto|dark|light` to control appearance explicitly. Run `/theme`
(or `/theme list`) inside the TUI to browse and filter all 37 themes in the
floating picker; `/theme mode toggle` switches the current theme between dark
and light without restarting. Activity uses the distinctive Hangul sequence by
default; `--spinner braille|hangul|pipe|dots` selects a different fixed-width
liveness animation at startup. `--motion full|reduced|off` selects the startup
motion policy; run `/motion` inside the TUI to choose the same modes from a
floating picker. Full mode enables the brief banner sweep and normal liveness,
reduced mode slows liveness and holds decorative artwork still, and off mode
uses static artwork and activity indicators with no repeating motion timer.

## Controls

| Input | Action |
|---|---|
| `/` in the composer | Open the command menu; keep typing to narrow it |
| `/` while the tree is focused | Open structured-data search |
| Up / Down | Choose a slash suggestion or move through the tree when it has focus |
| Tab | Accept the selected autocomplete suggestion; when none exists, move focus forward |
| Shift+Tab | Move focus backward between composer, transcript, and tree |
| Tab / Shift+Tab in the resource map | Select the next / previous product scope |
| Enter | Accept a partial suggestion or submit the composer |
| Shift+Enter | Insert a composer newline |
| Ctrl+B / Ctrl+F in a text input | Move the editing cursor backward / forward |
| Ctrl+B from the transcript or tree | Toggle the context rail |
| Ctrl+F from the transcript or tree | Open structured-data search |
| Ctrl+O | Toggle the wide JSON inspector |
| Up / Down while searching | Preview the previous / next exact match |
| Page Up / Page Down while searching | Move through search results five at a time |
| Home / End while searching | Preview the first / last search result |
| Enter while searching | Commit the previewed result and close search |
| Ctrl+O while searching | Commit the result and open it in the inspector |
| Shift+C / Shift+P while searching | Copy the sanitized scalar value / exact JSON path with OSC 52 |
| Escape | Close an overlay or return focus to the composer |
| Mouse wheel | Scroll the region under the pointer |
| Click a transcript evidence row | Restore its exact result snapshot and reveal it in the tree |
| Click `+ pin` | Add that evidence reference to the working set |
| Click row | Select a JSON value |
| Click product pill | Restrict the resource map and its search to that product |
| Click caret | Expand or collapse an object or array |
| P while tree is focused | Pin the selected JSON value to the working set |
| S while tree is focused | Toggle named-array ordering between source index and name |
| Page Up / Page Down | Move through the focused tree eight rows at a time |
| Ctrl+C | Cancel the active engine operation; exit and restore the terminal when idle |
| `/motion` | Browse full, reduced, and off motion; valid motion changes remain available while an operation is active |
| `/quit` | Close the engine, exit, and restore the terminal |

Local commands include `/demo`, `/help`, `/clear`, `/find`, `/pin`, `/unpin`,
`/theme`, `/motion`, `/sort`, `/sidebar`, `/inspect`, `/cancel`, and `/quit`; `/demo`
appears only in fixture mode. `/pin` saves the current tree selection and
`/unpin all` clears the working set without adding command noise to the
transcript. Engine mode adds `/manifest`, `/catalog`, `/doctor`, `/auth`,
`/config`, `/lookup`, `/list`, `/get`, `/show`, and `/diff`. Theme examples
include `/theme next`, `/theme github light`, and `/theme mode auto`.

## Verify it

```sh
bun run check
```

To include the real config-free process integration test:

```sh
go build -o /tmp/zscalerctl-engine ./cmd/zscalerctl-engine
ZSCALERCTL_ENGINE_TEST_BINARY=/tmp/zscalerctl-engine bun run check
```

The experiment has its own dependency lock and checks. A path-filtered GitHub
workflow runs them whenever this experiment, its shared TypeScript engine
client, or that workflow changes. It remains outside the repository's
supported release surface and root Go gate suite.

## Current boundaries

- The tree displays at most 800 visible rows so an accidentally large result
  cannot overwhelm one render pass.
- Workspace data is copied and deeply frozen when a result commits, preserving
  historical evidence even if a project adapter retains and mutates its source
  object. Cyclic values and structures deeper than 128 levels fail at this
  boundary instead of entering the snapshot registry.
- Search visits at most 5,000 nodes and retains at most 200 matches. It searches
  only the sanitized tree presented to the frontend; it cannot widen the Go
  engine's projection or recover dropped fields. Approximate matching is
  intentionally limited to record names and field names; scalar values require
  an exact substring so a fuzzy search cannot produce surprising data hits.
- Tenant-derived tree, transcript, picker, and toast presentation values cross
  `safeInlineText` into a nominal `SafeString` before their component models
  accept them. The type checker rejects an ordinary `string` at those leaves;
  the runtime conversion visibly replaces terminal controls and every Unicode
  format rune before display.
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
- The transcript and its referenced result snapshots are session-memory only
  and currently unbounded. `/clear` releases snapshots that are not active or
  pinned; the working set itself is capped at eight references.
- Transcript summaries are deterministic presentation metadata, not model
  context. Evidence enters the working set only through an explicit pin, and no
  local or remote model transport exists in this experiment yet.
- No generic relationship graph is inferred from matching names or `*id`
  fields. A future dependency explorer will need catalog-backed targets or
  resource-specific extractors so unresolved candidates are never presented as
  authoritative edges.
- The context rail becomes an overlay on narrow terminals and remains inline
  on wide terminals.
- Floating windows, drawers, and modal inspectors share explicit overlay
  layers; search remains non-modal so the tree behind it stays explorable.
- The operation scene waits 320 ms before appearing, so quick operations do not
  flash transient chrome. It disappears on success, cancellation, or failure;
  late progress cannot revive it. The shared motion clock ticks only while the
  complete Poison banner is inside the transcript viewport or a real operation
  is active. Scrolling any banner row out of view pauses the sweep without
  consuming its duration; mouse-scrolling back to the top starts a fresh full
  sweep unless ordinary keyboard interaction has already dismissed the startup
  motion.
- Motion uses React-rendered fixed-cell glyphs rather than OpenTUI timelines or
  post-processing, keeping layout and mouse hit targets synchronized. Global
  CRT, glitch, bloom, and other buffer effects remain future experiments.
- Selection is local UI state; no operation is dispatched from a tree click.
- Clipboard actions use the terminal's OSC 52 capability and fail visibly when
  the terminal declines it; they never invoke a platform clipboard process.
- The generic registry can accept additional validated theme definitions, but
  filesystem discovery for custom themes is intentionally not wired into this
  fixture-only experiment yet.

The Poison banner attribution and vendored OpenCode theme notices are recorded in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
