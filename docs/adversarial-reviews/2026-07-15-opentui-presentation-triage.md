# Builder Handoff

## Intent

Triage externally supplied presentation changes for PR #112's unsupported
Bun/OpenTUI experiment. Retain the useful JSON key/value hierarchy and
operator-selectable activity animation, repair unstable spinner geometry and
render isolation, and keep every semantic JSON value readable across the full
theme catalog.

## Base / Head

- Delta base: `8ac59834150474e3c254801a8cb3f91e08f259c2`
- Initial reviewed candidate:
  `4b3237c583db49dd2ab5aab1a4cd8cfc3c9e0eaa`
- Post-fix reviewed head:
  `faee126fbef81fbc1d59579d3719681ce9fddebf`
- Branch: `agent/opentui-agent-tui` (PR #112)
- PR/process-doc baseline:
  `498840b4418c83d22e0a49e91c29351802f3e526`

The unrelated dirty Ink experiment remains in a separate worktree and was not
modified, staged, or included in this delta.

## Triage Disposition

Retained and refined:

- JSON tree labels use the theme's primary foreground while value previews use
  type-aware semantic roles where those roles are readable.
- `--spinner braille|hangul|pipe|dots` selects a fixed-cell-width liveness
  sequence; Hangul remains the experiment default.
- Composer and operation indicator consume one provider-owned animation clock,
  so passive application components do not rerender on every frame.

Corrected before the initial review:

- the supplied dots sequence included a blank frame and variable cell widths;
  every sequence is now nonblank and fixed-width;
- animation ownership was centralized instead of giving each component an
  independent timer;
- option parsing, help, README text, and focused renderer tests were reconciled.

Not accepted:

- the externally described `ui_ux_improvements_review.md` did not exist in the
  worktree or supplied patch, so no report was invented or committed;
- the claimed Braille default was not adopted because this experiment already
  intentionally uses Hangul liveness glyphs.

## Files Changed

The initial candidate changes only `experiments/opentui-agent-tui/**`:

- spinner definitions, the shared frame provider, option parsing, and wiring;
- Composer and OperationIndicator liveness presentation;
- JSON tree key/value presentation;
- README/help documentation and focused test coverage.

The post-review fix adds the contrast helper and changes only JSON-tree and
spinner tests/presentation under the same experiment. This review artifact is
the only file outside the experiment added after the approved source recheck.

## Generated Artifacts

None. `experiments/opentui-agent-tui/bun.lock` is unchanged. No supported CLI
docs, schemas, goldens, generated skills, release artifacts, or root
dependencies changed.

## Invariants Claimed

- No supported Go CLI, stdio engine protocol, TypeScript client, machine
  contract, schema, credential, redaction, projection, or tenant-read-only
  behavior changed.
- No model execution, shell command surface, web server, OpenAPI client, dump
  path, or credentialed tenant access was introduced.
- Spinner frames are bounded, nonblank, and fixed-width in terminal cells.
- One active provider owns the animation timer and cleans it up on inactivity
  or unmount; components outside its context value do not rerender per tick.
- Semantic JSON roles must reach a 3:1 contrast floor over resolvable layered
  backgrounds. Otherwise they fall back to the theme's primary foreground.
  Transparent terminal underlays also fail safely to that foreground.
- Selected, active-match, matched, hovered, and normal presentation retain
  explicit precedence.

## Tests Run

- Initial full real-engine check: 84 tests, 1,038 assertions, pass.
- Final
  `ZSCALERCTL_ENGINE_TEST_BINARY=/tmp/zscalerctl-engine-opentui-pr112 bun run
  check`: 89 tests, 2,239 assertions, pass, including config-free discovery
  through the real Go engine.
- Final focused typecheck and color/tree/spinner tests: 10 tests, 1,371
  assertions, pass.
- Spinner timing/isolation test file repeated ten times: 10/10 pass.
- `make verify-experiment-boundaries verify-release-artifacts
  verify-surface-changes-manifest`: pass.
- `git diff --check`: pass.
- Lockfile delta check from the source base: pass with no changes.
- Final reviewed source-delta SHA-256:
  `9e2a8f6d89e6b397d543c4e0e5735cebeef30cb28b36832c7ca2d8ba1930f701`.

## Review Focus

The reviewer was asked to attack frame geometry, timer ownership and cleanup,
render isolation, option/help consistency, JSON label/value precedence,
semantic-role contrast over normal and selected surfaces, theme-mode coverage,
experiment boundaries, and supported machine/safety contracts.

# Adversarial Review

Fresh-context reviewer: Codex subagent `Volta`, session
`019f661f-5c3c-7a63-89ed-9a21f541ac9a`, read-only against the isolated PR
worktree.

## Initial Blocking Finding

The initial review found that direct semantic-role coloring made JSON values
nearly invisible in several light themes. Matrix light numbers rendered at
approximately 1.13:1 over the panel and approximately 1.03:1 over the hover
surface; Dracula and Vesper light also contained sub-1.5:1 combinations. The
Tokyo Night-only renderer test could not catch the catalog-wide failure.

## Initial Non-Blocking Risk

The spinner render-isolation test could pass without proving that the animation
clock had emitted a tick. The reviewer requested an active consumer assertion
before relying on the passive sibling's render count.

## Resolution Mapping

Finding:
semantic JSON value roles were unreadable on multiple light-theme surfaces.

Root cause:
the renderer applied semantic palette colors without resolving their contrast
against the panel, selection overlay, and terminal background layers.

Fix:
`src/color.ts` parses normalized theme colors, composites alpha layers, computes
contrast, and falls back at a 3:1 terminal-text floor. `JsonTree` resolves the
normal and active role sets once per palette and applies explicit selected,
active-match, matched, hovered, and normal precedence.

Regression test:
all 37 themes, both appearances, and all six tree kinds are checked over every
resolvable normal and active surface. Renderer tests verify the Matrix-light
fallback and normal, active-match, and selected precedence.

Risk resolution:
the spinner isolation test now observes more than one frame from an active
consumer before asserting that a passive sibling rendered exactly once. The
test passed ten consecutive runs.

## Recheck

The same fresh-context reviewer inspected only
`4b3237c583db49dd2ab5aab1a4cd8cfc3c9e0eaa..faee126fbef81fbc1d59579d3719681ce9fddebf`,
independently reran the full real-engine suite, repeated the spinner test ten
times, and ran the boundary and diff-hygiene gates. The reviewer closed both
prior concerns and found no regression in the corrective delta.

## Machine Contract Review

Supported JSON/NDJSON, error envelopes, exit codes, dump/diff, schema,
manifest, introspection, stdio protocol, and TypeScript-client sources are
unchanged. The new flag belongs only to the unsupported experiment's launcher.

## Safety Review

No redaction, projection, field coverage, credential, secret-provider, or
tenant-read-only behavior changed. Real-engine verification used config-free
catalog discovery and did not contact a tenant.

## Generated Artifact Review

No generated or release artifact changed, `bun.lock` is absent from the source
delta, and diff hygiene passed.

Verdict: approve
