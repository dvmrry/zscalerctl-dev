# Builder Handoff

## Intent

Refine PR #112's unsupported Bun/OpenTUI experiment after user testing and an
external ergonomics review. Preserve native text editing while making focus
navigation symmetric, add truthful visual operation progress, and make
transient feedback readable without creating a stale toast queue.

## Base / Head

- Delta base: `5e1ac223df7a744887bebdf13d1f548dc90e7f73`
- Initial reviewed candidate:
  `430896b774f6b8185c92c03ce341106364fbdd79`
- Post-fix reviewed head:
  `561ef6aa7cfb0b12da6d73f443d6845d28d879ab`
- Branch: `agent/opentui-agent-tui` (PR #112)
- PR/process-doc baseline:
  `498840b4418c83d22e0a49e91c29351802f3e526`

The unrelated dirty Ink experiment remains in a separate worktree and was not
read from, modified, staged, or included in this delta.

## Files Changed

The source candidate changes only `experiments/opentui-agent-tui/**`:

- shell interaction routing, contextual key hints, responsive rail geometry,
  workspace operation lifecycle, and toast integration in `src/App.tsx`
- focus-aware commands and composer fallback navigation
- reusable `OperationIndicator` and toast policy/controller primitives
- context-rail, picker cell-fit extraction, toast, transcript, and welcome
  presentation
- explicit completed-work semantics in the project-neutral workspace model
  and zscalerctl adapter
- README controls and focused unit/renderer/real-engine regression coverage

The post-review fix changes only:

- `experiments/opentui-agent-tui/src/App.tsx`
- `experiments/opentui-agent-tui/src/toast.ts`
- `experiments/opentui-agent-tui/test/app.test.ts`
- `experiments/opentui-agent-tui/test/helpers.ts`
- `experiments/opentui-agent-tui/test/toast.test.ts`
- `experiments/opentui-agent-tui/test/zscalerctl-adapter.test.ts`

This review artifact is the only file outside the experiment added after the
approved source recheck.

## Source Inputs Consulted

- OpenTUI 0.4.3 textarea defaults and global/focused event dispatch
- `docs/ENGINE_STDIO_PROTOCOL_V1.md` progress semantics
- typed TypeScript client progress decoding and operation callbacks
- existing OpenTUI App, composer, picker, rail, toast, workspace, and adapter
  source/tests at the delta base
- adversarial-review process documents from the explicit PR baseline

## Generated Artifacts

None. `experiments/opentui-agent-tui/bun.lock` is unchanged. No supported CLI
docs, schemas, goldens, generated skills, release artifacts, or root
dependencies changed.

## Expected Delta

- Tab accepts an actionable completion first, otherwise advances focus;
  Shift+Tab moves backward. Visible product scopes retain Tab priority.
- Ctrl+F/Ctrl+B remain native cursor controls in composer/search/picker text
  inputs and remain search/context shortcuts from transcript/tree.
- Slash while tree-focused opens structured search.
- Stdio `resource_started(current,total)` is translated to fully completed
  work as `completed = current - 1`; in-flight work is never counted complete.
- Valid running progress receives an exact counter and bounded 4-10 cell bar.
  Invalid or absent progress remains indeterminate; Hangul frames indicate
  liveness without inventing elapsed work.
- Context-free success cannot leave a stale running state, and late progress
  callbacks are ignored.
- The context rail clamps to the viewport.
- Toasts remain latest-wins with tone-aware durations. Identity-scoped
  dismissal clears a cancellation-request toast at terminal completion while
  preserving any newer unrelated toast.

## Invariants Claimed

- No supported Go CLI, engine protocol, TypeScript client, machine contract,
  schema, redaction, projection, credential, or tenant-read-only behavior
  changed.
- No model, shell runner, web server, OpenAPI client, plugin loader, or `/dump`
  path was added.
- Product scoping, overlay priority, and Ctrl+C idle/active semantics remain
  unchanged.
- Progress counters are positive-safe-integer derived and conservative.
- Terminal controls and unsafe formatting remain sanitized; cell fitting is
  grapheme-aware and verified with Hangul/combining text.
- Toast replacement cannot be cleared by stale timeout or mismatched identity.
- No credentialed tenant access was required or performed.

## Tests Run

- `bun install --frozen-lockfile`: pass; no lockfile drift.
- `go build -mod=vendor -o /tmp/zscalerctl-engine-opentui-pr112
  ./cmd/zscalerctl-engine`: pass.
- Before review:
  `ZSCALERCTL_ENGINE_TEST_BINARY=/tmp/zscalerctl-engine-opentui-pr112 bun run
  check`: 75/75 tests, 853 assertions, including the real config-free engine.
- After fixes, the same command: 78/78 tests, 860 assertions, including the
  real config-free engine.
- Reviewer focused recheck: 26/26 tests, 127 assertions.
- `git diff --check`: pass before review and after fixes.
- `git diff --exit-code 5e1ac223df7a744887bebdf13d1f548dc90e7f73
  -- experiments/opentui-agent-tui/bun.lock`: pass.
- `ZSCALERCTL_BASE_REF=498840b4418c83d22e0a49e91c29351802f3e526
  make verify-experiment-boundaries verify-release-artifacts
  verify-surface-changes-manifest`: pass.
- Final 84-file non-`node_modules` experiment aggregate SHA-256:
  `3ddea16e9eaa7b631c0b36807f5708799fd427baf36be1deceebb4b0e0b2abaf`.

## Known Deferrals

- No toast queue; asynchronous operation outcomes remain durable transcript
  entries. A bounded queue is deferred until independent background events
  exist.
- No generalized motion/reduced-motion system. The bounded Hangul liveness
  sequence remains the only animation.
- The composer retains its compact static `삼 Working` label.
- No agent/model execution or additional backend transport is introduced.

## Review Focus

The reviewer was asked to attack global-versus-focused key dispatch, Tab
priority, stdio progress truthfulness, malformed counters, terminal operation
transitions, cell geometry, toast races/lifecycle, experiment isolation, and
README accuracy.

# Adversarial Review

Fresh-context reviewer: Codex subagent `Banach`, session
`019f65f1-98f5-7041-87ad-589de46b39c3`, read-only against the isolated PR
worktree.

## Initial Blocking Finding

The initial review reproduced one blocking contradiction: after Ctrl+C, the
informational toast said cancellation was still waiting for engine
acknowledgment even after the operation had reached and rendered a terminal
canceled state.

The reviewer required a renderer regression proving that the waiting message
disappears after cancellation settles and after a cancellation attempt ends in
a terminal failure.

## Initial Non-Blocking Risk

Progress translation was tested with an impossible mock: a list operation
emitted non-contiguous progress. The reviewer requested a progress-capable
`diff` operation with contiguous `current = 1,2,3` events instead.

## Resolution Mapping

Finding:
stale cancellation-request toast after terminal operation outcome.

Root cause:
the latest-wins toast controller had no identity-scoped dismissal, and the
operation terminal path did not reconcile its request toast.

Fix:
`LatestToastController.show` returns the active toast ID and `dismiss(id)`
clears only that exact active toast. App tracks the cancellation request ID and
dismisses it in the operation `finally` path. A newer unrelated toast is not
cleared by the stale cancellation ID.

Regression test:
renderer tests assert that neither acknowledged cancellation nor terminal
cancellation failure retains “waiting for engine acknowledgment.” Controller
tests prove a mismatched ID cannot dismiss a newer toast.

Verification:
the full config-free suite passes 78 tests with 860 assertions; the focused
recheck passes 26 tests with 127 assertions.

Risk resolution:
the impossible list-progress mock was removed. A diff mock now emits
contiguous starts `1,2,3` and verifies completed-work values `0,1,2`.

## Recheck

The same fresh-context reviewer inspected only
`430896b774f6b8185c92c03ce341106364fbdd79..561ef6aa7cfb0b12da6d73f443d6845d28d879ab`,
reproduced the focused and full checks, closed the blocking finding and the
progress-test nit, and reported no new findings introduced by the fix.

## Machine Contract Review

The reviewer confirmed that supported JSON/NDJSON, errors, schemas, manifest,
introspection, protocol, and TypeScript-client sources are unchanged. It
independently verified the protocol's pre-resource progress meaning and the
conservative `current - 1` translation.

## Safety Review

No redaction, projection, field coverage, credential, or tenant-read-only
behavior changed. Real-engine checks were config-free and used local sanitized
fixtures only.

## Generated Artifact Review

No generated or release artifact changed, `bun.lock` is absent from the source
delta, and diff hygiene passed.

Verdict: approve
