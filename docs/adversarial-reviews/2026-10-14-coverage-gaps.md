# Builder Handoff

## Intent

feat(schema): declare coverage gaps; name them on unknown resources

Three fresh LLM agents drove the CLI against a live non-production tenant
(2026-10-14). The policy and identity agents each spent several calls and a
repo-doc search to learn that ZPA policy rules are not read, and could not
tell "this tenant has none" from "zscalerctl cannot see it". The only record
of what zscalerctl does not read was prose in `docs/RESOURCE_QUEUE.md`.

That record is now a typed table (`internal/resources/gaps_table.go`). Each
gap has a surface name, aliases a caller is likely to try, a closed reason
(`entitlement`, `response_shape`, `not_modeled`, `privacy_review`,
`credential_material`, `action`, `telemetry`, `alternate_view`) from which
the status (`deferred` or `out_of_scope`) derives, a value-free note, and
covered resources to read `instead`. `schema gaps [product]` lists them
(config-free). Asking to read a declared gap or alias still exits 4
(`not_found`) with the same envelope, but the message names the gap, its
status and reason, and what to read instead; other unknown resources also
point at `schema gaps <product>`.

## Base / Head

- Base: `integration/2026-10-09` at `5636835`.
- Reviewed heads: `3b356ed` (round 1), `9a81cc8` (round 2), `6f14669`
  (round 3).
- Final head: this artifact on top of `6f14669`; it changes no reviewed code.

## Files Changed

- `.agents/skills/zscalerctl/SKILL.md`
- `AGENTS.md`
- `README.md`
- `cmd/zscalerctl/testdata/surface/*` (schema-help, schema-bare, inventory,
  introspect, introspect-pretty goldens; `surface_changes.md`)
- `docs/RESOURCE_QUEUE.md`
- `docs/cli/agent-machine-workflow.md`
- `docs/cli/zscalerctl.md`
- `internal/cli/commands_config_schema_auth.go`
- `internal/cli/commands_schema_gaps.go`
- `internal/cli/commands_schema_gaps_test.go`
- `internal/cli/errors.go`
- `internal/cli/introspect_test.go`
- `internal/cli/ndjson_policy_test.go`
- `internal/resources/gaps.go`
- `internal/resources/gaps_table.go`
- `internal/resources/gaps_test.go`
- `man/zscalerctl.1`
- `skills/zscalerctl/SKILL.md`

## Tests Run

- `go test ./...` (Go 1.26.9, `/bin/chmod` first in `PATH` for the macOS ACL
  tests), `gofmt -l`, `go vet ./internal/...`, `verify-docs`,
  `verify-cli-docs`, `verify-surface-changes-manifest`,
  `verify-machine-contract`: pass.
- Built binary, offline: `zpa access-policies list` and `zia locations-lite
  list` exit 4 naming the gap; `zpa no-such-thing list` exits 4 pointing at
  `schema gaps zpa`; `schema gaps bogus` exits 2.

## Known Gaps

- Only the CLI read path names gaps. The machine executor's
  `unknown resource` error, and `schema describe`/`schema references` on an
  unknown resource, keep their existing messages.
- The table is hand-transcribed from `RESOURCE_QUEUE.md`. Only the deferred
  backlog is coupled by test; the non-coverage buckets are not parsed, since
  they name SDK packages rather than CLI resources.
- Telemetry (reports, logs, read-only exports) is `deferred` because
  `RESOURCE_QUEUE.md` names a future report track for it; `out_of_scope`
  means "not planned", not "never".
- A first draft declared `zia/admin-audit-logs` as a telemetry gap; it is a
  real command (`zia admin-audit-logs status`). The router-level test
  (`TestEveryGapRoutesToItsGapError`) was added to catch that class.

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, surrounding source, the `RESOURCE_QUEUE.md` records the table was
transcribed from, and this repository's reviewer prompt and template:
Strata serving Qwen3.8-Flash-Next (IQ3_S) on gm01. The reviewer could not run
commands.

- Round 1, request changes (the review ran out of output budget before its
  verdict line; its three blocking findings were taken as request changes).
  (1) Telemetry gaps were `out_of_scope`, documented as "never", though the
  source record names a future report/telemetry track: telemetry is now
  `deferred`, and `out_of_scope` is documented as "not planned". (2)
  `instead` named merely related resources (`zpa/cbi-banners` -> isolation
  profiles, `zia/scim` -> users, `zia/sandbox-submissions` -> sandbox rules,
  `zia/gre-tunnel-info` -> GRE tunnels): `instead` is now only the resource
  an alternate view shows or an action's readable status, enforced by test.
  (3) `FindGap`/`CoverageGaps` returned slices aliasing the package table:
  they now clone, pinned by test. Also: `RESOURCE_QUEUE.md` now says the
  table holds the operator-facing surfaces, not every SDK package.
- Round 2, request changes. `action` and `telemetry` both claimed exports, so
  `policy-export` could be read either way. The source record lists ZIA
  `policy_export` as export telemetry, so the definitions are now disjoint
  (action: mutating or executing; telemetry: reports, logs, read-only
  exports) and a test pins `policy-export` as deferred telemetry.
- Round 3, approve with nits (below).

Accepted and not changed: the action-gap `instead` rule is enforced by
reason rather than by "must be a status resource" (only `activation` uses
it); the slice-copy test mutates one gap (the clone is generic); a single
test pins the one classification the definitions left open; `schema gaps`
takes the product case-sensitively, like every other command; the machine
executor and `schema describe`/`schema references` keep their
unknown-resource messages (Known Gaps).

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-14 (round 3)

## Blocking Findings

None.

The round-2 blocker is resolved in the visible diff:

- `GapAction` no longer claims exports. It is now scoped to mutating or executing actions, with examples that match the source record’s “Submission or execution-adjacent” bucket: activate, submit, move, remove.
- `GapTelemetry` now claims reports, logs, and read-only exports, matching the provided `docs/RESOURCE_QUEUE.md` ZIA record, which places `policy_export` under “Reporting, discovery, log, or export telemetry” and says to treat it as a future report/telemetry command track.
- `GapTelemetry.Status()` derives `deferred`, which is consistent with “future report/telemetry command track.”
- The added `TestPolicyExportIsTelemetry` pins `FindGap(ProductZIA, "policy-export")` as `GapTelemetry` and `GapStatusDeferred`. If `policy-export` were still classified as `action/out_of_scope`, this test would fail.

The change does not appear to introduce a new contradiction between the definitions, the source record, or the status mapping.

## Non-Blocking Risks

1. **Evidence gap: the `coverageGaps` table itself is not included in the review inputs.**

   - File/path: `internal/resources/gaps.go` and the unshown gap table file.
   - Concrete issue: The diff shows the test and comments, but not the actual `coverageGaps` entry for `policy-export`. The added test is the intended proof, but the reviewer cannot independently inspect the table from the supplied artifacts.
   - Why it matters: The classification lives in the table, not in the comments. If the table were wrong, the test would catch it, but the review evidence is incomplete.
   - How to reproduce or verify:
     - `grep -n 'policy-export' internal/resources/*.go`
     - `go test ./internal/resources -run TestPolicyExportIsTelemetry`
   - Test that would catch it: The added `TestPolicyExportIsTelemetry` test.

2. **The new test pins only the contested `policy-export` case, not the general disjointness rule.**

   - File/path: `internal/resources/gaps_test.go`
   - Concrete issue: The test ensures `policy-export` is telemetry, but it does not assert that other export/report/log surfaces are telemetry, nor that action gaps do not claim export/report/log surfaces.
   - Why it matters: The round-2 problem was a taxonomy overlap. A single regression test fixes the known case but does not harden the whole classification boundary.
   - How to reproduce or verify: Inspect `coverageGaps` for other surfaces with export/report/log/discovery semantics and check their `Reason` values.
   - Test that would catch it: A table-driven invariant test that scans all gaps and asserts:
     - surfaces/notes matching export/report/log/discovery are telemetry unless explicitly action/mutating;
     - `GapAction` gaps do not claim export/report/log surfaces.

3. **The Go-doc wording could be slightly tightened.**

   - File/path: `internal/resources/gaps.go`
   - Concrete issue: `GapTelemetry` says telemetry is “rather than configuration inventory.” `policy_export` is an export of policy/configuration, so a literal reader might think it is configuration inventory. The source record explicitly resolves this by classifying export surfaces as telemetry, so this is not a contradiction, but the wording could be clearer.
   - Why it matters: Future maintainers may read the comment without the source record and reclassify export surfaces as action or not-modeled.
   - How to reproduce or verify: Compare the comment with `docs/RESOURCE_QUEUE.md`, which says “Report, discovery, audit-log, and export surfaces are telemetry, not configuration inventory.”
   - Test that would catch it: The existing `TestPolicyExportIsTelemetry` test plus a future invariant test would catch drift.

## Machine Contract Review

- Did JSON, NDJSON, error envelope, exit code, dump, diff, schema, manifest, or introspection behavior change?
  - No visible machine-contract change. The `GapReason` string values are unchanged: `"action"` and `"telemetry"` remain the same.
  - The `Status()` mapping is unchanged: `GapTelemetry` remains deferred, `GapAction` remains out-of-scope.
  - The `CoverageGap` JSON shape is unchanged.
  - The `schema gaps` output contract should be unchanged unless the unshown gap table changed; the diff does not show a table change.
- Are contract deltas intentional and documented?
  - No contract delta is present in the diff. The commit message and test explain the classification rationale.
- Are generated docs, schemas, and fixtures in sync with runtime behavior?
  - No generated artifacts are in the diff. If godoc, `.agents`, or docs are generated from these comments, `verify-docs` should cover them, but no generated-file delta is visible.

## Safety Review

- Did redaction, projection, field coverage, or sensitive-data classification change?
  - No. The diff changes only comments and adds a test.
- Can a narrowed field set widen output or reveal previously dropped data?
  - No output field changes are present.
- Are dump and diff artifacts still sanitized but treated as confidential?
  - Not affected by this diff.

## Generated Artifact Review

- Were generated files intentionally regenerated?
  - No generated files are present in the diff.
- Are deltas explained?
  - No generated artifact deltas exist to explain.
- Any suspicious drops, additions, ordering changes, or count changes?
  - None visible. The only executable change is a new test.

## Verdict

Verdict: approve with nits
