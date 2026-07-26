# Builder Handoff

## Intent

Restore the automated release gate after the candidate TypeScript client made
Node 24.12 or newer mandatory, prevent CI and release Node setup from drifting
again, and prepare accurate cumulative `v0.16.0` notes for everything
unreleased since `v0.15.1`.

## Base / Head

Base commit: `4fe01d98f4e96cad2865b671ca1f61552e7a90c6`

Reviewed implementation head:
`274b07a9e6c3b166dbd26e2de634a9781bb89747`

## Files Changed

- `.node-version`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
- `Makefile`
- `scripts/verify-workflow-policies.go`
- `scripts/verify-node-toolchain.sh`
- `scripts/verify-active-node-toolchain.sh`
- `scripts/test-verify-node-toolchain.sh`
- `scripts/test-verify-active-node-toolchain.sh`
- `scripts/verify-semgrep.sh`
- `scripts/require-ci-jobs.sh`
- `scripts/test-require-ci-jobs.sh`
- `docs/DEPENDENCY_POLICY.md`
- `docs/SCRIPTS.md`
- `docs/releases/v0.16.0.md`

## Source Inputs Consulted

- Failed release runs `30209754199`, `30209980091`, and `30210137609`.
  Each reached the TypeScript gate and failed because release supplied Node
  `22.23.1`, while the client requires Node `>=24.12`.
- The existing CI Node `24.15.0` setup and full-SHA action pins.
- `clients/typescript/package.json` and
  `scripts/verify-typescript-client.sh` for the runtime floor.
- `docs/VERSIONING.md`, merged labels, merged PR bodies, and the complete
  `v0.15.1..main` first-parent/surface delta. Merged PRs #111 and #113 carry
  `semver:minor`, so the cumulative release is `v0.16.0`.
- `internal/zscaler/reader.go` for the exact legacy ZIA cloud allow-list and
  redirect behavior described in the release notes.
- `.openvex.json`; no product-specific entry was added because the redirect
  fix has no standardized vulnerability identifier.

## Generated Artifacts

None. `docs/releases/v0.16.0.md` is hand-curated input to the existing release
workflow.

## Expected Delta

- Supported CLI, machine schemas, resource catalog, field coverage, exit-code
  numbers, and release artifact shape: unchanged.
- CI retains Node `24.15.0`; CI and release now read it from one committed
  `.node-version` file.
- Release provisions and proves Node `24.15.0` before `make release-check`
  instead of inheriting the runner's Node 22 release.
- CI and release policy jobs use reviewed checkout/setup-go/setup-node action
  SHAs, canonical inputs and ordering, exact protected commands, and a direct
  active-runtime proof adjacent to each protected Node consumer.
- The stable CI sink exactly needs every other root CI job. The real
  TypeScript validation command must appear exactly once in the root
  `typescript-client` job; a same-named nested job cannot satisfy the rule.
- Job- and step-level `continue-on-error` is rejected throughout root and
  locally traversed CI workflows and composite actions.
- Release is constrained to the exact read-only `release-gate` plus its
  dependent publisher; no extra publisher can be added.
- Release classification is `semver:minor`, preserving the highest unreleased
  merged classification after earlier publication attempts failed.

## Invariants Claimed

- External actions remain full-SHA-pinned, package installation remains absent
  for the dependency-free TypeScript client, and package-manager caching stays
  disabled.
- A synchronized workflow edit cannot silently lower the Node runtime: the
  policy independently pins `24.15.0` and verifies both `.node-version` and the
  active `node --version` result.
- Conditional, reordered, cross-job, failure-suppressed, environment/path
  redirected, duplicate-proof, foreign-checkout, alternate-action, and
  staged-pin variants covered by the mutation suite fail closed.
- The stable CI sink cannot omit any current prerequisite or detach the real
  TypeScript validation behind a no-op job-name placeholder.
- Release runs its complete gate before artifact construction, signing,
  attestation, tagging, or publication.
- This branch creates no tag or release before merge.

## Tests Run

- `bash scripts/test-verify-node-toolchain.sh`
- `bash scripts/test-verify-active-node-toolchain.sh`
- `bash scripts/test-require-ci-jobs.sh`
- `make verify-node-toolchain verify-go-toolchain verify-actions-pinned`
- `make verify-release-automation verify-release-artifacts docs-check
  verify-script-registry`
- `make verify-typescript-client` — 41/41 pass
- `go vet -mod=vendor ./...`
- `git diff --check`
- `env -u GOFLAGS make check` at reviewed head — every substantive gate
  passed, then the run stopped only because this approval artifact had not yet
  been committed.

The Node mutation suite removes each of the 12 current CI prerequisites in
turn; substitutes detached and no-op TypeScript jobs; exercises nested reusable
workflow identity; and applies job-, step-, and local-composite-action failure
suppression.

## Known Deferrals

Credentialed `make live-smoke` has not run. Do not merge this PR, which would
trigger publication, until an operator reports its final `[PASS]` from a
read-only tenant environment.

The security and legacy-cloud compatibility changes are called out prominently
in the release notes rather than represented by a product-specific OpenVEX
entry.

## Review Focus

- Prove release cannot inherit the runner Node, weaken or redirect the setup,
  or publish without the complete gate.
- Attack the active-runtime proof, bootstrap action source/order, stable CI
  sink, root TypeScript job binding, nested local references, and failure
  suppression.
- Verify cumulative release classification and security/compatibility claims
  against merged history and source.

# Adversarial Review

Fresh-context reviewer: Kepler
(`019f9f5c-f7b3-7613-a66a-5f6a2e857405`)

Reviewed base: `4fe01d98f4e96cad2865b671ca1f61552e7a90c6`

Reviewed implementation head:
`274b07a9e6c3b166dbd26e2de634a9781bb89747`

## Review History

Earlier exact heads were rejected for concrete policy bypasses, including
workflow-local Node-version overrides, conditional consumers, custom-shell and
environment/path redirection, setup/proof ordering, duplicate proofs,
foreign/duplicate bootstrap sources, a no-op stable sink, an extra publisher,
and incomplete release job/permission constraints. Each was fixed and mapped
to a negative regression fixture before review continued.

At `8485e51671d8a2f68f8e81b54ad303011d19dd96`, the reviewer found two
remaining blockers:

1. The real TypeScript consumer could move to an unrequired job while a no-op
   job retained the expected name.
2. The stable sink required only two named dependencies rather than every
   reviewed CI prerequisite.

The fix bound the exact consumer to the literal job and derived the sink's
exact `needs` set from every other root job. Regression tests remove all 12
current prerequisites individually and exercise both detached-consumer
variants.

At `8a8c7dc7ec8126e92215950083248dde252d3758`, the reviewer found two adjacent
blockers:

1. A same-named job in a local reusable workflow could satisfy the global
   consumer count.
2. Ordinary prerequisites could hide failures with job- or step-level
   `continue-on-error` while the sink observed success.

The final fix binds the consumer to the root workflow file and rejects failure
suppression at root workflow-job, workflow-step, nested reusable-workflow, and
local composite-action boundaries.

## Blocking Findings

None at the reviewed implementation head.

## Non-Blocking Risks

None.

## Independent Closure Verification

At exact clean head `274b07a9e6c3b166dbd26e2de634a9781bb89747`, the reviewer confirmed the
named base and merge-base, inspected the exact parent delta, passed
`git diff --check`, and passed `bash scripts/test-verify-node-toolchain.sh`.

The reviewer independently replayed these isolated probes:

| Probe | Result |
|---|---|
| Valid control fixture | Accepted |
| Same-named consumer in a conforming local reusable workflow, with a no-op root `typescript-client` | Rejected |
| Job-level `continue-on-error` on `verify-gates` | Rejected |
| Step-level `continue-on-error` in `verify-gates` | Rejected |
| `continue-on-error` inside a composite action invoked by `verify-gates` | Rejected |

The worktree remained clean and the reviewer edited no files.

## Machine Contract Review

No CLI, JSON/NDJSON, stdio wire, error-envelope, exit-code, schema, manifest,
redaction, projection, or tenant-data behavior changed.

## Safety Review

The release gate remains credential-free. Reviewed action sources, read-only
gate permissions, exact dependency topology, active runtime, failure
propagation, and pre-publication ordering are enforced by source-aware gates
and negative fixtures.

## Generated Artifact Review

No generated artifact changed. The curated release note is a hand-maintained
release input and its compatibility/security claims were checked against
source and merged history.

Verdict: approve
