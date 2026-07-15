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

Verdict: approve with nits
