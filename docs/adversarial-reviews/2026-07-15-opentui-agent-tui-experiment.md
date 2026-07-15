# Builder Handoff

## Intent

Add an unsupported, source-only Bun/OpenTUI experiment for a full-screen,
conversation-first zscalerctl workspace. Keep the React shell reusable behind
a project-neutral `WorkspaceAdapter`, with a committed sanitized fixture as
the default and an optional adapter over the repository's existing typed
TypeScript stdio client.

The experiment has no model client, shell runner, arbitrary process command,
web server, socket listener, OpenAPI path, telemetry, browser integration, or
dynamic plugin loader. In engine mode its only child is an operator-selected
absolute `zscalerctl-engine` path launched by the existing process adapter with
`shell: false`.

## Base / Head

- Publication base: PR #111 head
  `498840b4418c83d22e0a49e91c29351802f3e526`
- Builder/reviewer checkout: `feature/ink-agent-tui` at
  `3b335dcc162ee54fa092278c3485b96a668a2d81`; its only committed delta from
  the publication base was the explicitly excluded Ink experiment
- Head: reviewed uncommitted source tree, copied byte-for-byte to the isolated
  publication branch `agent/opentui-agent-tui`
- Process-doc baseline: `origin/main` at
  `b0597dfb8e673a06d99995e6e1360cfcc709f0a8`
- Initial review scope: all 78 non-`node_modules` files under
  `experiments/opentui-agent-tui/`
- Pre-review experiment-tree aggregate SHA-256:
  `8f77bdca1cbc9d45346af8d6edb407cd84cfaee886acc1e5ab1f9ed2a5799085`
- Post-fix delta scope: all 79 non-`node_modules` files under the experiment;
  aggregate SHA-256
  `bd63550c48e82b5292a78f071237af93293ee1efd01cb27f090bd2ed9f89364b`
- Publication hygiene removed only one extra blank line at EOF from `.gitignore`
  and `src/fixture.ts`; final aggregate SHA-256
  `35f03c573a19c98935a97322ed5f4db44b36eed27e787c7ffb8efa8dd5653fa2`

The existing dirty `experiments/ink-agent-tui/` changes are user-owned and are
explicitly outside this review scope.

## Files Changed

- Isolated Bun manifest, exact lockfile, TypeScript configuration,
  third-party notices, and experiment documentation under
  `experiments/opentui-agent-tui/`
- Project-neutral full-screen OpenTUI shell, transcript, pinned composer,
  interaction command router, context rail, floating windows, inspector,
  toasts, mouse-aware picker, and terminal text safety under `src/`
- Structured JSON tree with exact wire-number handling, named-array ordering,
  bounded traversal, grouped search, and exact path/value copy support
- OpenCode-derived theme catalog assets plus local themes and dark/light/auto
  resolution
- Strict local startup options and TTY lifecycle in `src/options.ts` and
  `src/main.tsx`
- Fixture and generic workspace interfaces in `src/workspace.ts`
- Deterministic zscalerctl command grammar and typed engine adapter in
  `src/zscalerctl/`
- Unit, component, process-adapter, parser, picker, tree, theme, text-safety,
  cancellation, and real-engine integration tests under `test/`
- This adversarial-review handoff; the review report will be appended after a
  fresh-context read-only review

No existing Go, CLI, engine, protocol, TypeScript client, schema, resource
catalog, golden, generated CLI reference, release, or root dependency file was
changed.

## Source Inputs Consulted

- `clients/typescript/src/`, especially typed methods, validated response
  types, exact `WireNumber`, error taxonomy, process lifecycle, capability
  checks, and Unicode format ranges
- `cmd/zscalerctl-engine` and the real built host used by integration tests
- The independently reviewed Ink experiment's deterministic command grammar
  and zscalerctl adapter as a behavioral reference; no cross-experiment import
  was introduced
- `docs/ENGINE_STDIO_PROTOCOL_V1.md`, repository machine-surface guidance, and
  the root experiment-boundary policy
- OpenTUI React/core APIs and installed type declarations
- OpenCode theme definitions and license notice recorded in
  `THIRD_PARTY_NOTICES.md`
- Adversarial-review process docs and templates from `origin/main`

## Generated Artifacts

`experiments/opentui-agent-tui/bun.lock` is the only generated experiment
artifact. `bun install --frozen-lockfile` reports no changes. The JSON theme
assets are vendored source assets with attribution, not generated supported
artifacts. No root or release artifact was regenerated.

## Expected Delta

- Add one isolated unsupported experiment containing 79 source, test,
  configuration, lock, documentation, and theme-asset files.
- Keep the root Go module, default `make check` dependency graph, release
  archives, CLI, machine manifest, schemas, catalog, and generated artifacts
  unchanged.
- Default to a credential-free sanitized fixture and Tokyo Night with terminal
  dark/light detection.
- When given an absolute engine path, negotiate stdio v1 and load the catalog
  without credentials, then expose deterministic `/manifest`, `/catalog`,
  `/doctor`, `/auth`, `/config`, `/lookup`, `/list`, `/get`, `/show`, and
  `/diff` commands.
- Deliberately omit `/dump` because it has local write/delete effects and stdio
  v1 has no wire-visible publication commit marker.
- Plain-language entries remain informational because no model is attached.

## Invariants Claimed

- The UI does not evaluate shell syntax. Command tokenization is local syntax
  splitting only, and typed operations are sent through the existing client.
- The executable must be an absolute path; credentials remain in inherited
  `ZSCALERCTL_*` environment variables and never cross a credential argument
  or stdio request API.
- Projection, redaction, SDK access, protocol validation, capability checks,
  exact JSON-number lexemes, and tenant-read-only enforcement remain owned by
  the existing engine/client.
- Config-free bootstrap performs only process execution plus manifest/catalog
  reads. Live commands state network and configuration-dependent local/process
  effects in the context rail.
- Catalog picker dispatch follows each resource's advertised operations rather
  than inferring an operation from resource shape.
- Raw backend and child-stderr prose is never rendered. Errors map to a fixed
  local taxonomy; only canonical missing `ZSCALERCTL_*` names may render.
- C0/C1 controls and every Unicode format range rejected by the typed client
  are rejected in command/options boundaries or replaced before terminal
  presentation.
- One workspace operation enters before React paints busy state. Ctrl+C and
  `/cancel` abort the active operation, while idle Ctrl+C and `/quit` close the
  engine before restoring the terminal.
- Tree flattening is capped at 800 visible rows; search visits at most 5,000
  nodes and retains at most 200 matches; the resource picker retains at most 80
  visible matches per query.
- JSON search and tree clicks operate only on already projected data and can
  neither widen fields nor initiate a tenant mutation.

## Tests Run

- `go build -mod=vendor -o /tmp/zscalerctl-opentui-engine ./cmd/zscalerctl-engine`:
  pass
- `ZSCALERCTL_ENGINE_TEST_BINARY=/tmp/zscalerctl-opentui-engine bun run check`:
  pass after review fixes; strict typecheck and 53/53 tests with 612
  assertions, including the
  real config-free Go process integration
- `make verify-typescript-client`: pass; 40/40 typed client and real-process
  tests
- `make verify-experiment-boundaries`: pass
- `make verify-release-artifacts`: pass
- `bun install --frozen-lockfile`: pass with no changes
- Clean-stack revalidation on the publication base: 53/53 experiment tests
  with 612 assertions against a freshly built real engine, followed by a full
  passing `make check`
- Trailing-whitespace scan over experiment docs/config/source/tests: pass
- Manual PTY: real engine connection, 165-resource config-free catalog,
  floating searchable picker, `/quit`, terminal restoration, process exit 0,
  and no remaining engine process

No credentialed live read was required or performed.

## Known Deferrals

- No model or agent loop is attached; this validates the deterministic shell
  and structured workspace only.
- No `/dump`, arbitrary shell execution, OpenAPI client, web server, or plugin
  loading is included.
- Completed transcript entries remain in memory for the session; long-session
  compaction is deferred until the interaction model proves useful.
- The absolute engine executable remains an operator trust boundary; binary
  signing or identity policy is outside this experiment.
- Credentialed tenant reads and Windows terminal behavior were not exercised.
  The real engine bootstrap/catalog path and inherited-environment boundary
  were exercised without credentials on macOS.
- Custom theme filesystem discovery is not wired; only validated built-ins are
  available.

## Review Focus

- Try to escape the deterministic grammar into shell/process execution or
  smuggle credentials, backend prose, child stderr, controls, or bidi/format
  characters into terminal output.
- Verify process startup/close, rapid-submit exclusion, cancellation races,
  idle Ctrl+C, renderer destruction, and absence of child leaks.
- Verify every typed request mapping, filter first-operator semantics, exact
  `WireNumber` preservation, partial-diff warning semantics, and catalog
  operation selection.
- Attack tree/search/picker traversal with large or deeply nested data,
  duplicate/stable IDs, query changes, mouse/keyboard races, and narrow
  terminals.
- Confirm fixture/engine mode help and completion match runtime behavior and
  that `/dump` remains unreachable.
- Confirm dependency, license, root-module, release-artifact, generated-file,
  and supported-surface isolation.

# Adversarial Review

## Reviewer

- Fresh read-only Codex CLI context using `gpt-5.5`
- Session: `019f655e-ad33-7dc3-880b-5fc0dde40626`
- The reviewer inspected the tree directly, excluded the user-owned Ink
  experiment changes, ran its own checks, and did not edit the worktree.

## Initial Verdict

`request changes`

### Blocking Finding: Picker Metadata Presentation Boundary

The reusable `WorkspaceAdapter` could return picker-owned `title`,
`placeholder`, `instruction`, `emptyMessage`, or badge text containing terminal
controls or unsafe Unicode format characters. Those fields reached OpenTUI
without the shell's terminal-text normalization, despite the experiment's
claim that adapter-controlled presentation text is sanitized.

Resolution:

- Added `normalizeWorkspacePicker` at the adapter-result boundary. It sanitizes
  and bounds every picker presentation and search field while retaining exact
  opaque item IDs and commands.
- Added render-boundary defense in depth for picker window metadata and badges.
- Added a direct normalization regression with C0 and bidi-format characters.
- Extended the injected-engine component test to verify hostile picker metadata
  reaches the captured terminal frame only as replacement text, with no raw
  control or format characters.

### Non-Blocking Documentation Risk

The controls table described Ctrl+C and `/quit` as equivalent even though
Ctrl+C cancels an active operation and exits only while idle.

Resolution:

- Split the controls-table descriptions to state the active/idle Ctrl+C
  behavior and the unconditional orderly `/quit` behavior precisely.
- Added a documentation regression test for that distinction.

## Post-Fix Delta

The review-driven delta is limited to:

- `README.md`
- `src/App.tsx`
- `src/components/PickerWindow.tsx`
- `src/workspace.ts`
- `test/app.test.ts`
- `test/workspace.test.ts`
- new `test/docs.test.ts`

Builder verification after the delta:

- strict TypeScript typecheck: pass
- full experiment suite against a freshly built real Go engine: 53/53 pass,
  612 assertions
- no credentialed tenant access performed

## Final Delta Review

Fresh-context reviewer: Codex CLI `gpt-5.5`, read-only ephemeral session
`019f6564-7733-7d73-9a9d-2792b6a23fff`.

The reviewer inspected the current source rather than relying on this handoff,
rechecked the original finding at both boundaries, verified exact opaque ID and
command preservation, inspected the README correction, and ran:

- `bun test test/workspace.test.ts test/app.test.ts test/docs.test.ts`: 11 pass
- `bun run check`: typecheck pass; 52 pass and the real-engine test skipped
  because the reviewer intentionally had no engine binary configured

No blocking findings survived. The reviewer retained one non-blocking test
hardening nit: the app-level hostile-picker test exercises a visible picker
path and the unit test covers every field, but separate component cases could
also force the `instruction` and `emptyMessage` render branches. Source review
confirmed that both branches use `safeInlineText`; this does not weaken the
approved safety disposition.

No core machine contract, projection, redaction, generated artifact, or
supported CLI surface changed.

## Final Repository Verification

- `make check`: pass in both the builder checkout and the isolated publication
  branch, including race tests, vet, `govulncheck`, staticcheck,
  license and documentation checks, gitleaks, Go toolchain enforcement, typed
  client tests, boundary gates, machine-contract validation, PTY escape
  cleanliness, release-artifact checks, and generated skill synchronization
- Final experiment-tree SHA-256 after the whitespace-only publication cleanup:
  `35f03c573a19c98935a97322ed5f4db44b36eed27e787c7ffb8efa8dd5653fa2`
- `git diff --check` and the explicit untracked-file trailing-whitespace scan:
  pass
- No `zscalerctl-engine` or `zscalerctl-opentui-engine` process remained

## Publication Hygiene Delta Review

Fresh-context publication reviewer: Codex CLI `gpt-5.5`, read-only ephemeral
session `019f6570-6b26-79b1-9ebc-6147e121a8e1`.

The reviewer inspected only the final unstaged delta against the approved
staged tree, independently recomputed the 79-file aggregate digest, and
confirmed that the two source changes only removed extra blank EOF lines. It
reported no blockers or nits and approved the publication-hygiene delta.

## Composer Layout Follow-Up

User testing after publication reported that the pinned composer felt
squeezed, especially around the breakpoint where the 48-column context rail
becomes inline and abruptly narrows the conversation pane.

Delta from PR #112 commit
`89ee315b6228076c77d262179a54049e560a049d`:

- Give the editor three rows plus bottom breathing room on terminals at least
  20 rows high, retaining the smaller footprint on short terminals.
- Select full, compact, or minimal composer chrome from the conversation
  pane's actual width rather than total terminal width.
- Preserve full status/help text when it fits; collapse to
  `Explore · <workspace>` and `Enter send`, then `Explore` and `Enter`, before
  labels can collide.
- Add a 121x30 regression at the exact inline-rail breakpoint.

Changed source/test files:

- `src/components/Composer.tsx`
- `src/App.tsx`
- `test/app.test.ts`

Verification before delta review:

- strict TypeScript typecheck: pass
- real-engine experiment suite: 54/54 pass, 618 assertions
- experiment-boundary gate: pass
- post-delta 79-file aggregate SHA-256:
  `0c2481e9fdf8f5545fffa4eb290d05051ae7097d3019102c9f6cb3f7d22e8e7b`
- captured renders inspected at 121x30, 140x32, 64x20, and 50x12

Fresh-context composer delta reviewer: Codex CLI `gpt-5.5`, read-only
ephemeral session `019f657b-6e69-7bc3-ae0d-db17715838fc`.

The reviewer independently inspected the four-file delta, ran the focused and
full credential-free suites, and rendered boundary probes at 57, 58, 87, 88,
120, and 121 columns plus short heights. It found no overflow, collision,
machine-contract change, or safety-path change and reported no blocking
finding.

The reviewer retained one evidence nit: its sandbox had no
`ZSCALERCTL_ENGINE_TEST_BINARY`, so it reproduced 53 passes plus the skipped
real-engine integration rather than the builder's 54/54 real-engine run. It
independently confirmed the post-delta aggregate digest. The builder's
real-process command and result remain recorded above.

Composer delta disposition: approve with nits.

## Theme Picker Follow-Up

User testing requested that bare `/theme` behave like theme listing and use the
existing floating searchable picker rather than adding transcript prose.

Delta from PR #112 commit
`bce911e840e43037a5e7b2be713f2ff2d9ac8209`:

- Route bare `/theme` and `/theme list` to the reusable picker modal.
- Keep `/theme <name>`, `/theme next`, and `/theme mode ...` as direct local
  commands.
- Present the active theme first and selected, group OpenCode-catalog and local
  experiment themes, and filter all 37 built-ins.
- Keep theme selection local to the shell while preserving the existing
  generated-command dispatch for engine/catalog pickers.
- Correct the command usage text to include the supported `toggle` mode.

Changed files:

- `experiments/opentui-agent-tui/README.md`
- `experiments/opentui-agent-tui/src/App.tsx`
- `experiments/opentui-agent-tui/src/commands.ts`
- `experiments/opentui-agent-tui/test/app.test.ts`

Fresh-context read-only reviewer session
`019f6587-6619-7633-9314-7e0edbc411b7` initially requested changes for two
source-verifiable issues:

1. The first implementation selected `opencode` rather than the active theme,
   so immediate Enter unexpectedly changed the default Tokyo Night theme.
2. Reusing generated-command dispatch logged both the picker launcher and its
   generated `/theme <name>` command, while Escape left an unanswered launcher
   entry.

Resolution mapping:

- Active selection: order the active theme first within its category and pass
  it as the preferred picker ID; regress immediate Enter for both catalog and
  local themes.
- Transcript semantics: intercept modal launch before transcript append, tag
  picker purpose locally, validate selected built-in IDs, and apply theme
  selections without `submitRef` or `workspace.execute`; regress apply,
  cancellation, direct-command logging, and injected-workspace isolation.
- Help nit: advertise `toggle` only in the `/theme mode` branch.

Post-fix builder verification:

- `ZSCALERCTL_ENGINE_TEST_BINARY=/tmp/zscalerctl-engine-theme-picker bun run
  check`: 55/55 pass, 639 assertions, including the real config-free Go engine
  process integration.
- `make verify-experiment-boundaries`: pass.
- `make verify-release-artifacts`: pass.
- `git diff --check`: pass.
- Runnable and isolated publication copies of all four changed files:
  byte-identical.
- Post-fix 79-file aggregate SHA-256:
  `f587b18ae829bc403715b637c18fdee286676399b62870445006250064352236`.
- No credentialed tenant access was performed and no engine process remained.

The same reviewer rechecked only the findings, their fix surface, and the help
nit. It independently reproduced 55 passing tests with 639 assertions plus
focused keyboard, mouse, transcript, direct-command, local-theme, and
workspace-picker probes. Both findings were closed, no new or non-blocking
findings survived, engine/catalog picker dispatch remained intact, and no
supported CLI, engine, machine, schema, redaction, projection, secret, or
tenant contract changed.

Prior disposition: approve

## Product-Scoped Resource Map Follow-Up

User testing found that the 165-resource catalog was difficult to navigate
because available Zscaler products were discoverable only by scrolling through
group headers or typing a product name.

Delta from PR #112 commit
`54fbbae85f4e9b89fb393d49819487f054fa4437`:

- Add optional, adapter-owned scopes to the reusable workspace picker model.
  Scope labels and counts cross the same normalized presentation boundary as
  the existing picker metadata; scope IDs remain opaque matching keys.
- Turn the engine catalog into a Zscaler resource map with persistent `ALL`,
  `ZIA`, `ZPA`, `ZCC`, `ZTW`, and `ZIDENTITY` pills and exact readable-resource
  counts derived from the catalog response.
- Group resources in product-navigation order while preserving source order
  within each product. Rows identify the product family and report recursive
  projected-field counts.
- Keep search inside the selected product. Tab and Shift+Tab cycle scopes;
  mouse clicks select a scope; Left and Right remain available to the focused
  search input.
- Wrap scope pills by terminal cell width, including Hangul-width accounting,
  and include scope chrome in the floating window's height calculation.
- Remove OpenCode branding from shell-owned theme picker categories and clarify
  in the README that the information architecture is zscalerctl-specific.

Changed files:

- `experiments/opentui-agent-tui/README.md`
- `experiments/opentui-agent-tui/src/App.tsx`
- `experiments/opentui-agent-tui/src/commands.ts`
- `experiments/opentui-agent-tui/src/components/PickerWindow.tsx`
- `experiments/opentui-agent-tui/src/components/WorkspacePicker.tsx`
- `experiments/opentui-agent-tui/src/workspace.ts`
- `experiments/opentui-agent-tui/src/zscalerctl/adapter.ts`
- `experiments/opentui-agent-tui/test/app.test.ts`
- `experiments/opentui-agent-tui/test/commands.test.ts`
- `experiments/opentui-agent-tui/test/integration/engine.test.ts`
- `experiments/opentui-agent-tui/test/picker-window.test.ts`
- `experiments/opentui-agent-tui/test/workspace.test.ts`
- `experiments/opentui-agent-tui/test/zscalerctl-adapter.test.ts`

Invariants claimed:

- No supported CLI, Go engine, stdio protocol, TypeScript client, schema,
  projection, redaction, credential, or tenant-read-only contract changes.
- Product counts are derived only from read-capable catalog resources and sum
  to the picker item count; they are not hardcoded.
- Product scoping narrows the already projected catalog and cannot widen fields
  or initiate a live read until the operator selects a resource row.
- The existing 80-row render bound remains in force. Scope counts describe the
  full readable catalog while the bottom status retains the `+` truncation
  marker for a bounded result list.
- Picker selection is reset to a valid first row whenever scope changes, and
  the synchronous picker ref is updated before a subsequent commit event.
- Scope navigation is intercepted only when a picker advertises at least two
  scopes, so the theme picker does not lose its prior Tab behavior.

Builder verification:

- `go build -mod=vendor -o /tmp/zscalerctl-engine-opentui-pr112
  ./cmd/zscalerctl-engine`: pass.
- `ZSCALERCTL_ENGINE_TEST_BINARY=/tmp/zscalerctl-engine-opentui-pr112 bun run
  check`: strict typecheck and 58/58 tests pass with 728 assertions, including
  the real config-free Go engine process integration.
- Real integration confirms scope order `zia`, `zpa`, `zcc`, `ztw`,
  `zidentity`, exact scope-count summation, and product-narrowed queries.
- `make verify-experiment-boundaries verify-release-artifacts`: pass.
- `bun install --frozen-lockfile`: pass without lockfile changes.
- `git diff --check`: pass.
- Current 80-file non-`node_modules` experiment aggregate SHA-256:
  `370f8b0ac27a92b50fd372fe6d022cdc8d0d3cc397adf658a18fe1cc7d97c394`.
- No credentialed tenant access was required or performed.

Known deferrals:

- Dynamic FUI-style loading animation remains a later motion-system slice. It
  should include delayed display, reduced-motion fallback, and a bounded frame
  rate rather than being coupled to this navigation change.
- The experiment remains unsupported and source-only; no model loop, shell
  runner, web server, OpenAPI path, `/dump`, or plugin loader is introduced.

Fresh review should attack scope-count truthfulness, product ordering,
selection/query races, opaque scope-ID handling, Tab interception, mouse
dispatch, narrow-terminal wrapping/height calculations, Hangul cell width,
terminal-text normalization, and any path by which scoping could alter or
bypass typed resource command generation.

### Initial Product-Map Delta Review

Fresh-context reviewer: Codex subagent `Socrates`, session
`019f65a7-77b0-7e93-928d-bc2786b2e4d9`, read-only against the isolated PR
worktree.

Initial disposition: request changes. The reviewer reproduced three blocking
picker-primitive defects and one related generic-contract risk:

1. A scope pill wider than the available inner width wrapped internally even
   though row allocation counted it as one row. At 16 columns, `ZIDENTITY 3`
   collided with the first ZIA category.
2. The scope bar remained enabled when `FloatingWindow` clamped the requested
   height below its fixed chrome. At short heights, pills or rows crossed the
   bottom border; when hidden by the old height threshold, Tab could still
   change scope invisibly.
3. The synthetic ALL React key `__all__` collided with that valid opaque scope
   ID, and duplicate adapter scope IDs could mark multiple pills active and
   trap Tab cycling.
4. A stale advertised scope count could claim items that did not exist in the
   picker.

Resolution mapping:

- Finding 1 root cause: row calculation measured the unconstrained text, but
  the renderable retained wrapping behavior. Fix: fit headings and pills to a
  bounded terminal-cell width, preserve counts while ellipsizing labels, mark
  scope text `wrapMode="none"`, and calculate rows from the exact rendered
  strings. Regression: captured 16x30 render plus direct overwide-pill and
  Hangul-width assertions; no `3ZIA` collision survives.
- Finding 2 root cause: scope visibility used a viewport-height threshold
  independent of the final floating-window rectangle. Fix: derive maximum
  clamped geometry through `placeFloatingWindow`, require room for all fixed
  scope chrome, and share that exact predicate with App-level Tab routing.
  Regression: captured renders at heights 8, 10, 12, 14, and the first visible
  16-row boundary; an App test confirms hidden scopes do not consume Tab.
- Finding 3 root cause: a data-derived ID doubled as a reserved React key and
  normalization admitted duplicate IDs. Fix: use position-owned React/render
  keys, preserve opaque IDs only as callback/filter values, and normalize
  scopes with stable first-ID deduplication. Regression: render and mouse-select
  a legitimate `__all__` scope and verify exact callback identity; normalize
  duplicate IDs deterministically.
- Risk 4 root cause: adapter-advertised counts were trusted separately from
  item membership. Fix: derive normalized counts from actual item `scopeId`
  membership and omit empty/stale scopes. Regression: hostile counts,
  duplicate IDs, and a stale scope normalize to two unique scopes with exact
  counts 1 and 2.

Post-fix builder verification:

- Focused picker/workspace/App suite: 20/20 pass, 128 assertions.
- `ZSCALERCTL_ENGINE_TEST_BINARY=/tmp/zscalerctl-engine-opentui-pr112 bun run
  check`: strict typecheck and 63/63 tests pass with 777 assertions, including
  the real config-free Go process integration.
- Manual real-catalog captures at 16x30, 30x8, 30x12, 30x14, 30x16, 42x26,
  60x26, and 100x30 confirm bounded scope chrome. At 16 columns the long pill
  renders as `ZIDENTI… 3`; at 30x16 all six pills fit inside the border.
- `git diff --check`: pass.
- Post-fix 80-file non-`node_modules` experiment aggregate SHA-256:
  `1c54801a954811408c5edcfb5679bfa0599812440d98d0cf8522e26c5de04756`.
- No credentialed tenant access was required or performed.

### First Product-Map Recheck

The same fresh-context reviewer closed the overwide-pill finding, the opaque
and duplicate-ID finding, and the stale-count risk. It retained `request
changes` for one transition missing from the first height fix: after selecting
a product in a visible 30x16 picker, shrinking to 30x14 hid the product strip
and active-scope status but left the prior product filter applied. Tab was then
correctly not intercepted, so the hidden filter could not be changed without
resizing or reopening the picker.

Second resolution mapping:

- Root cause: scope-bar visibility controlled rendering and keyboard routing,
  but persisted scope state independently controlled filtering.
- Fix: derive an effective scope synchronously from the shared visibility
  predicate. Hidden scope chrome now means ALL for filtering, selected-scope
  refs, and rendered status in the same render. A resize effect then clears the
  persisted scope and resets selection through the existing scope-transition
  function.
- Regression: open a realistic five-product picker at 30x16, cycle to ZPA,
  verify the ZPA-only row, resize to 30x14, verify the strip and ZPA status are
  absent while the first ALL resource is present, press Tab, and commit the ZIA
  row. This covers visible-selected-to-hidden rather than merely opening while
  already hidden.
- Verification: focused transition test passes; the complete real-engine suite
  remains 63/63 with 782 assertions; `git diff --check` passes.
- Current 80-file non-`node_modules` experiment aggregate SHA-256:
  `7de4f78707cc550e78a14d5fafcef3d3537c7a787867462a0f6669505f97ee08`.

### Final Product-Map Recheck

Fresh-context reviewer: Codex subagent `Socrates`, session
`019f65a7-77b0-7e93-928d-bc2786b2e4d9`, read-only against the unchanged
isolated PR worktree.

The reviewer independently reproduced the 30x16 ZPA selection followed by a
30x14 resize and confirmed that ALL rows return immediately, hidden Tab does
not change scope, Enter dispatches `/list zia locations`, and visible scope
cycling remains intact. It found no stale ref, invalid selection, hidden
filter/status drift, resize loop, or regression introduced by the final fix.

Reviewer verification reproduced 63/63 tests with 782 assertions against the
real engine, passed `git diff --check`, and matched the 80-file digest above.
No supported CLI, engine, protocol, schema, projection, redaction, credential,
or tenant-read-only contract changed.

Verdict: approve
