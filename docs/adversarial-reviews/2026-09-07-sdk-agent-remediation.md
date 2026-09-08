# Builder Handoff

## Intent

Remediate the offline SDK/agent-workflow review: collect Zidentity pages
without silently accepting known truncation, keep URL-rule detail enrichment
bounded, reject unknown catalog fields before reads, offer explicit bounded
JSON list output, and teach agents to discover and consume the installed
CLI's actual contract. Upgrade the SDK and address reachable dependency
vulnerabilities discovered during validation. Tenant mutation remains out of
scope.

## Base / Head

Base commit: `2e01faab8bf053c8e0fc72c80ea8d13a1a387218`.
Head branch: `feature/sdk-agent-remediation`, uncommitted working tree.
Initial implementation patch (before review fixes): `scratch/remediation/review-input.patch`,
SHA-256 `8397395a859845f92b6116e863e5ff106b6c50d46ae3caa7d2d95156318d6990`.
The patch includes tracked changes and added files, excluding this review
artifact so recording the verdict does not change the implementation digest.
Final implementation patch after review fixes: `scratch/remediation/review-final-input.patch`,
SHA-256 `a799803e9a11f41b41a28ae71101d0b6201e3b863828e2639a07e8c19d70ea2c`.
Process baseline: `origin/main:docs/adversarial-review.md`,
`origin/main:docs/review-handoff-template.md`, and
`origin/main:docs/adversarial-review-run-prompt.md`.
Review the entire diff against the base, including added files; the handoff
is an index of claims to verify, not evidence of correctness.

## Files Changed

- CLI global flags, paging validation/rendering, early field validation, tests,
  process goldens, surface-change manifest, generated reference, man page,
  and the new list-page JSON Schema.
- Shared resource narrowing validation and machine executor preflight/errors.
- SDK readers, response adapters, catalogs, shape/coverage fixtures, generated
  field coverage, and a scoped SDK threat-model delta note.
- Root/vendor SDK dependencies, Go module floors and CI/toolchain verifier
  pins, tools-module vulnerability remediation, dependency/install guidance.
- Canonical and generated agent skills, offline workflow examples, README,
  agent workflow documentation, and release pairing guidance.
- SDK inventory provenance code, tests, and documentation.

## Source Inputs Consulted

- Official Zscaler Go SDK v3.8.38 and v3.8.48 source, including Zidentity
  pagination, URL filtering, PAC files, changed response shapes, OAuth/cloud
  routing and product clients. The SDK v3.8.48 module is vendored in this tree.
- Existing catalog, source projection helpers, SDK ignored-field registry,
  schema fixtures, CLI/machine interfaces and generation scripts.
- Official Go vulnerability database entries GO-2026-6218, GO-2026-6090,
  GO-2026-5972, GO-2026-5026, and GO-2026-6214.
- Public CLI v0.68.1 skill and the repository's explicit 1.0 promotion hold.

## Generated Artifacts

- `make gen-cli-docs`: adds the two global page flags.
- `go test ./cmd/zscalerctl/... -run 'TestGoldenSurface|TestCommandTreeInventory' -update`:
  refreshes intentional flags/catalog drift and adds complete-array,
  first/final-page, unknown-field and unknown-filter process fixtures.
- `make field-coverage`: accounts for SDK response-shape changes.
- `scripts/sync-agents-skill.sh`: mirrors the canonical installable skill.
- `go mod tidy` and `go mod vendor`: update the SDK and its selected dependency
  graph; upstream source is not hand patched.

## Expected Delta

- Resource capabilities and supported product/resource operations do not gain
  new endpoints. Newly reviewed fields on existing resources are deliberate.
- Introspection keeps the same 292 command paths and exit-code definitions;
  global flags increase from 13 to 15.
- SDK coverage remains 165 reviewed resources: 2,979 -> 3,010 source fields,
  2,961 -> 2,982 classified fields, and 18 -> 28 deliberately excluded fields.
  Decided coverage remains 100%; rendered coverage changes 99.4% -> 99.1%
  because ten newly present fields are explicitly excluded. Resource ordering
  in the generated report changes with the field counts.
- `--limit` and `--offset` appear as global flags but are valid only for JSON
  resource `list`. Default JSON arrays and NDJSON record framing remain.
- Opt-in page output has `records` plus `pagination` metadata; it slices after
  complete collection, projection, redaction, filtering, and field narrowing.
- Unknown filter names change from a successful empty result plus warning to
  structured usage failure (exit 2). Unknown fields also fail before config or
  credential/provider access. Known but suppressed fields remain valid.
- SDK v3.8.38 becomes v3.8.48; Go minimum becomes 1.26.6. Build tooling uses
  patched go-git v5.19.2. No public version/release is created.

## Invariants Claimed

- Only projected records enter paging/rendering; field narrowing cannot widen
  the allow-list or recover suppressed fields.
- Pagination failures discard partial records. Zidentity continuation URLs
  are opaque markers; requests stay on the original endpoint.
- URL-rule fallback is limited to ISOLATE details with a missing profile and
  a nonzero profile ID; all fallback pages use the CLI's bounded reader.
- Invalid page values/scope/format and invalid catalog names fail before
  configuration-dependent effects. Explicit help retains precedence.
- Machine request/event schemas remain unchanged; the page envelope is CLI
  presentation only. Existing error-envelope and exit-code definitions remain.
- No live credentials or Zscaler calls were used for implementation checks.

## Tests Run

- `go test ./...`: passed on the integrated SDK/CLI tree.
- `go test -race -mod=vendor ./...`: passed on the final integrated source
  after all reviewer-requested implementation fixes.
- `make staticcheck verify-licenses`: passed.
- `make vuln`: root and tools scans passed after the documented updates.
- `make vet docs-cli-check verify-sdk-boundary verify-core-boundaries verify-machine-contract verify-experiment-boundaries verify-typescript-client verify-pty-escape-clean`:
  passed, including 41 TypeScript client tests.
- `make docs-check`: passed after the SDK review stamp and coverage update.
- `make semgrep-check secret-scan verify-gitleaks-allowlist verify-actions-pinned verify-ci-no-live-creds verify-release-automation verify-release-artifacts verify-catalog-draft verify-resource-scaffold verify-sdk-surface-inventory verify-script-registry`:
  passed.
- `make verify-go-toolchain verify-node-toolchain`: passed under explicit
  Go 1.26.6 and automatic toolchain selection.
- Draft 2020-12 JSON Schema validation against the actual first/final page
  process goldens passed; false completeness and inconsistent continuation
  mutations were rejected.
- `make vendor` was repeated with before/after SHA-256 inventories of
  `go.mod`, `go.sum`, and every vendor file: no byte changes.
- The skill offline harness passed with synthetic reads and with
  `ZSCALERCTL_BIN` pointing at the candidate binary for config-free discovery.
  The exact first two shell blocks from the canonical skill also passed
  against candidate `machine manifest` and `schema list` JSON.
- Skill `quick_validate.py`, `make verify-agents-skill`, and final
  `make docs-check` passed after canonical/generated sync.

These checks establish offline behavior only. Fresh review and its resolution
are recorded separately below.

## Known Deferrals

- No tenant key is available. Live entitlements, actual tenant pagination
  metadata, and government tenant behavior require a later operator smoke.
- List paging bounds rendered output, not API requests or memory. Each call
  recollects the resource; offset calls are not a consistent snapshot.
- Missing upstream completeness metadata cannot prove a tenant snapshot.
- Newly introduced SDK endpoints are not automatically added to the CLI.
- The scoped SDK delta review is not a new full security audit.
- Public release promotion remains held under the existing release checklist;
  the candidate binary and skill must be promoted together after release checks.

## Review Focus

Attack page completeness/progress, short-page offset arithmetic, detail
fallback failure paths, early-validation effects, unknown-name error redaction,
JSON/NDJSON separation, real output versus schema/examples, SDK field aliases,
nil PAC attribution, nested allow-lists, generated coverage accounting, and
compatibility with older installed binaries. Independently execute the skill
recipes against real discovery JSON and synthetic reads; do not accept a
fixture that merely restates an incorrect contract.

Independently vary pagination metadata and page contents: an advancing offset
does not establish content progress. Exercise repeated pages that would reach
the advertised total, and cycles that revisit an earlier page.

# Adversarial Review

Fresh-context reviewer: `/root/adversarial_sdk_runtime` and
`/root/adversarial_cli_workflow`, each created with no inherited conversation
and using `gpt-5.6-luna` at `max` reasoning. Neither reviewer implemented fixes.
The first reviews SDK/runtime/projection/dependencies; the second reviews the
CLI/machine contract, docs, examples, schemas, goldens, and skill sync.

## Blocking Findings and Resolution

### Repeated Zidentity content could masquerade as a complete collection

Finding: `reader_zidentity_pagination.go` accepted two identical 1,000-record
pages when the second response advanced `pageOffset` to 1,000, declared
`results_total=2000`, and omitted `next_link`. The collected length reached
the declared total even though half the inventory was duplicated.

Root cause: progress checks trusted offset and count metadata without checking
page content. The previous repeated-page test only exercised a stale offset.

Resolution: Zidentity and ZPA now fingerprint every complete page with SHA-256
and reject a previously seen payload before accepting a declared terminal
page. The shared helper assumes no per-record identity field and returns
serialization errors. All failures discard partial records. Regression tests
cover advancing offsets, unknown totals, declared-terminal A/B/A cycles, and
marshal failures. The SDK reviewer accepted the Zidentity fix; the final
shared-helper/ZPA recheck is recorded below when complete.

### Government Zidentity admin routing was not established by the SDK upgrade

Finding: SDK v3.8.48 updates government OAuth and product gateways but still
routes `/admin/api/v1` through the old `zslogin` government suffixes. Testing
OAuth and ZIA routing alone did not establish the Zidentity admin origin.

Resolution: explicitly reject OneAPI Zidentity on `gov`/`govus` as unsupported
before SDK construction or authentication. Carry the restriction into reused
sessions so authenticating for another product cannot bypass it. Tests cover
List/Get/Show/Session and cross-product fixed sessions with zero outbound
requests, plus preserved commercial/ZPATWO/other-government-product behavior.
README and INSTALL document the unsupported combination. No alternative admin
host was guessed and no vendor source was patched.

### Invalid machine request values were reflected into error messages

Finding: unsupported filter operators could place arbitrary client text into
machine and terminal-event diagnostics before the loader ran. Unsupported
capability/operation diagnostics used the same reflection pattern.

Resolution: static diagnostics for these three invalid-value cases, with
sanitization at the shared machine error boundary. Canary and ANSI regression
tests cover Execute, ExecuteStream and typed Read; they verify usage/error
kinds, zero loader calls, and one terminal failure event. Two contract golden
messages changed intentionally; structured context and schemas remain stable.

### Output-file documentation overpromised Windows atomic replacement

Finding: `os.Rename` provides no universal atomic replacement guarantee on
Windows, while the documentation claimed atomic replacement without a
platform qualification.

Resolution: README, AGENTS, agent workflow, man page and implementation
comments now limit the atomicity guarantee to Unix and explicitly state the
Windows limitation. The implementation continues to use a same-directory
temporary file, fsync and rename. This is a corrected documented guarantee,
not a claim of newly verified Windows atomic behavior.

## Additional Findings Addressed

- Output destinations: non-regular files (including directories, symlinks and
  FIFOs) and inspection failures now produce usage errors before creating a
  temporary file. Unit and process tests verify exit 2, destination preservation,
  regular overwrite/new-file success and temporary-file cleanup. Documentation
  also explicitly states that Windows mode bits do not restrict inherited
  ACLs; an already restricted output directory is required.
- Skill preflight: whitespace in `FIELDS="id, name"` is trimmed consistently
  with CLI/workflow parsing. The synthetic harness checks both spellings, and
  the exact discovery/preflight recipe passed against the candidate binary.
- ZPA A/B/A page cycles were a baseline limitation discovered during review;
  the shared whole-page fingerprint fix above also addresses them.

## Scope and Rejected Generalization

The SDK reviewer withdrew a proposed blanket partial-overlap/record-deduplication
blocker after rechecking the contract. Generic records have no guaranteed
identity field and independently collected pages are not a stable tenant
snapshot. Exact repeated whole pages are rejected; partial overlap between
different pages is not claimed detectable. Remaining finite page ceilings are
preserved. This limitation is recorded rather than inventing a uniqueness
contract.

## Final Independent Recheck

CLI/workflow reviewer: approved after independently rechecking both initial
blockers and the final Windows ACL documentation correction. Verification
included a separately built candidate, config-free CLI behavior, machine/CLI
and full Go tests, vet, machine/PTY contracts, skill harness and sync, docs and
diff checks. No remaining CLI/workflow finding was reported.

SDK/runtime reviewer: approved with nits. The final bounded recheck reported
nothing remaining to verify beyond completed evidence and no confirmed
blocker. It independently confirmed advancing-offset repeats, repeats without
a declared total, A/B/A cycles and marshal failures fail before returning
partial data; ZPA uses the shared all-seen helper. It confirmed government
Zidentity List/Get/Show/Session rejection before authentication and the
fixed-service cross-product restriction. Focused tests and the scoped
zscaler/resources/machine/CLI race suite passed; root's final full Go and race
suite results provide integrated coverage.

The SDK reviewer's remaining nits are scope limitations, not pending fixes:
ZIA/ZCC/ZTW retain their existing short-page logic and finite page ceilings;
they do not use the new shared fingerprint set. The generic adapters do not
establish per-record identity or a tenant snapshot contract, so stronger
partial-overlap checks would require endpoint-specific evidence. Exact
repeated full pages in those existing readers remain bounded by their ceiling.

SDK reviewer commands:

```sh
go test -mod=vendor ./internal/zscaler -run 'Test(UnsupportedOneAPIZidentityCloud|ReaderGovZidentity|FixedServiceGovZidentity|ReadAllZidentityPages|ZPAPaginateRejectsRepeatedNonAdjacentPage|SDKCloudRoutes)' -count=1
go test -race -mod=vendor ./internal/zscaler ./internal/resources ./internal/machine ./internal/cli
```

Verdict: approve with nits

No live-tenant compatibility or public release approval is implied. The known
limitations above remain explicit: no stable cross-call snapshot, no generic
partial-overlap detector, unsupported government Zidentity admin routing, and
platform-specific local output guarantees.


## Final Integration Result

After both independent approvals, `make verify-adversarial-review` and the full
`make check` passed (exit 0). The aggregate check includes Go tests, race, vet,
root/tools vulnerability scans, staticcheck, license and secret checks,
generated documentation and contract checks, TypeScript client tests, boundary
and release-policy checks, script tests, and canonical/generated skill sync.
The complete log is retained locally at
`scratch/remediation/make-check-approved.log`.

The final implementation patch was recomputed after verification and matched
SHA-256 `a799803e9a11f41b41a28ae71101d0b6201e3b863828e2639a07e8c19d70ea2c`
byte for byte. This artifact is excluded from that implementation digest.
No commit, push, PR publication, release, or tenant call was performed.
