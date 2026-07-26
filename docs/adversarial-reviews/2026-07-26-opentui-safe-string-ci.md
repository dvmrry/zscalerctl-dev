# Builder Handoff

## Intent

Complete the deferred OpenTUI frontend-hardening work from PR #122 by making
terminal-safe presentation text a nominal TypeScript type at the main dynamic
rendering leaves and by adding path-filtered CI for the unsupported OpenTUI
experiment and its shared TypeScript engine client.

## Base / Head

Base commit: `eea0d30e2fbe29bbfa18737489da0b2ed859a9d1`

Reviewed head: `ede7904d01086d12c941c8685bcd16ae0cf9d6b4`

## Files Changed

- `.github/workflows/opentui-experiment.yml`
- `experiments/opentui-agent-tui/README.md`
- OpenTUI text, presentation-model, workspace-normalization, tree/transcript,
  picker, toast, and component files under
  `experiments/opentui-agent-tui/src/`
- Focused tests under `experiments/opentui-agent-tui/test/`

## Source Inputs Consulted

- `clients/typescript/src/unicode.ts` and its exported
  `isUnicodeFormatCodePoint`
- Existing OpenTUI runtime sanitizer and all tree, transcript, picker, and toast
  construction paths
- Existing workflow action pins and policy scripts
- The upstream `oven-sh/setup-bun` GitHub release/tag API

The upstream `v2.2.0` tag resolves to
`0c5077e51419868618aeaa5fe8019c62421857d6`.

## Generated Artifacts

None.

## Expected Delta

- Runtime wire values and engine contracts: unchanged.
- `safeInlineText` and `fitCellText` return nominal `SafeString` values;
  `fitCellText` sanitizes raw input before branding and truncating it.
- Transcript, tree/search, normalized picker, picker component, and toast
  presentation fields reject ordinary `string` values at compile time.
- Raw workspace adapter summaries and picker sources remain ordinary strings
  until their existing normalization or presentation boundary.
- CI runs only when the workflow, shared TypeScript client, or OpenTUI
  experiment changes, installs Bun 1.4.0 from a frozen lockfile, and runs the
  experiment check.

## Invariants Claimed

- Only the private assertion inside `src/text.ts` creates `SafeString`; no
  other source file casts to the brand.
- Every public brand-producing helper performs runtime C0/DEL/C1 and Unicode
  `Cf` replacement first.
- Exact `WireValue` data, search/copy paths, opaque IDs, commands, and adapter
  execution semantics remain unbranded and behaviorally unchanged.
- Context/operation metadata retains its existing runtime sanitization and
  injection tests rather than being nominally branded.
- The experiment remains outside the supported release surface and default Go
  gate suite.
- Workflow actions are full-SHA pinned, checkout credentials are not
  persisted, live credentials are not referenced, and permissions are
  `contents: read` only.

## Tests Run

- `bun install --frozen-lockfile`
- `bun run check` (123 pass, 1 optional integration skip, 0 fail)
- `bash scripts/verify-actions-pinned.sh`
- `bash scripts/test-verify-actions-pinned.sh`
- `bash scripts/verify-ci-no-live-creds.sh`
- `bash scripts/test-verify-ci-no-live-creds.sh`
- `make verify-experiment-boundaries`
- `make check`
- `git diff --check`

All commands passed.

## Known Deferrals

- Context and operation metadata remains runtime-sanitized at
  `safeContextState` and component defense-in-depth boundaries. Existing tests
  deliberately inject unsafe adapter-owned values directly into those
  components.
- The real-engine integration test remains opt-in through
  `ZSCALERCTL_ENGINE_TEST_BINARY`.

## Review Focus

- Find brand forgery, unsafe public brand producers, and composition that loses
  the brand without re-sanitizing.
- Find tenant-derived tree, transcript, picker, or toast leaves still typed as
  ordinary strings.
- Check preservation of wire values, search semantics, opaque identifiers,
  commands, and clipboard values.
- Audit workflow path coverage, action pins, permissions, credentials, Bun
  version, and frozen-lock behavior.
- Reject README claims broader than the actual branded surfaces and explicit
  context/operation deferral.

# Adversarial Review

Fresh-context reviewer: Harvey (`019f9f04-393a-7363-90ea-a12b81a5696e`)

Reviewed base: `eea0d30e2fbe29bbfa18737489da0b2ed859a9d1`

Reviewed head: `ede7904d01086d12c941c8685bcd16ae0cf9d6b4`

## Blocking Findings

None.

## Non-Blocking Risks

None.

## Machine Contract Review

The reviewer confirmed that all `SafeString` producers sanitize before
branding and that tree, transcript, picker, and toast presentation fields are
both nominally typed and runtime-defended. `WireNumber` lexemes, paths, opaque
IDs, filtering, commands, and clipboard values remain lossless where required.
Context and operation metadata retains the documented runtime-sanitized
boundary.

## Safety Review

The new workflow uses complete validated action SHAs, grants only
`contents: read`, disables checkout credential persistence, references no live
credentials, covers the experiment, shared TypeScript client, lockfile, tests,
and workflow path, and uses Bun 1.4.0 with a frozen-lockfile install.

## Generated Artifact Review

No generated artifact changed.

## Independent Verification

At exact clean head `ede7904d01086d12c941c8685bcd16ae0cf9d6b4`, the reviewer passed
the Bun typecheck and test suite (123 pass, 1 optional integration skip), action
pin, credential, and experiment-boundary policy tests, frozen-lockfile install,
and diff/head cleanliness checks.

Verdict: approve
