# Builder Handoff

## Intent

docs, test: bring backlog docs current and close small test gaps

Backlog sweep after the settings-validation change:

- Exit-code wording: exit 2 now covers an unusable setting (`invalid_config`)
  and exit 3 means credentials unset, incomplete, or a secret that could not be
  resolved; credentials the API rejects are exit 5. AGENTS.md, SKILL.md (and
  its generated copy), README.md and agent-machine-workflow.md said "missing or
  invalid credentials". VERSIONING.md is unchanged by owner decision until 1.0.
- RESOURCE_QUEUE.md still listed `location/locationlite` as uncovered and
  carried stale hand-written resource counts (ZIA 102, ZTW 20) and SDK pin
  (v3.8.38). Counts are gone in favor of `zscalerctl schema list`; the pin
  points at go.mod. Dated historical SDK versions are kept.
- RESOURCES.md, ZIA Locations Lite: which records the unfiltered view leaves
  out, and that a `refs` run that cannot read it skips every rule `locations`
  field.
- Tests: the locations-lite handler end to end (endpoint, unfiltered query,
  decode), dotted `input.fields` through the machine executor,
  `sdkStatusCode`'s parsed-status fallback, and the OneAPI retry budget.
- `newLegacyZIAConfiguration` dropped three RateLimit retry settings the SDK
  never reads (the explicit HTTPClient replaces the client they configure).
- `verify-semgrep.sh`: a bad fixture must fail on its own rule id, not on any
  rule.

## Base / Head

- Base: `integration/2026-10-09` at `4fe1151`.
- Reviewed head: working tree on `docs/resource-queue-current`.
- Final head: the reviewed change, round-3 comment nits, and this artifact.

## Files Changed

- `AGENTS.md`, `README.md`, `skills/zscalerctl/SKILL.md`,
  `.agents/skills/zscalerctl/SKILL.md` (synced)
- `docs/RESOURCE_QUEUE.md`, `docs/RESOURCES.md`,
  `docs/ZSCALER_PRODUCT_SCOPE_PLAN.md`, `docs/cli/agent-machine-workflow.md`
- `internal/zscaler/reader.go` (legacy RateLimit lines, comments)
- `internal/zscaler/reader_locations_lite_test.go`,
  `internal/zscaler/failure_sdk_test.go`, `internal/zscaler/reader_test.go`,
  `internal/zscaler/reader_pagination_test.go`,
  `internal/zscaler/admin_audit_logs_test.go` (helper rename to
  `newZIATestService`)
- `internal/machine/executor_test.go`,
  `internal/cli/commands_schema_references_test.go`
- `scripts/verify-semgrep.sh`

## Tests Run

- `PATH=/bin:$PATH go test ./...`: pass.
- `make fmt-check`, `make verify-agents-skill`,
  `ZSCALERCTL_BASE_REF=integration/2026-10-09 bash scripts/verify-surface-changes-manifest.sh`,
  `bash scripts/verify-semgrep.sh`: pass.
- Mutations: the lite handler test fails with an added `includeSubLocations`
  query and with the `/locations` endpoint; `verify-semgrep.sh` fails with the
  two bad fixtures swapped.
- Live (share redaction, counts only): `zia locations-lite` returns
  `zappSslScanEnabled` on all 3 records, so the lite/full spelling difference
  is the API's; `refs check zcc forwarding-profiles` checked 14 integer
  `trustedNetworkIds` against string ids with none missing.

## Known Gaps

- The OneAPI sign-in failure reason is `unknown` (the SDK flattens the token
  status and transport error to text); fixed separately on
  `fix/sign-in-failure-reason`, with the introspect exit 3/5 wording.
- No strict `tsc` step for the TypeScript client (needs a pinned npm
  toolchain); no `schema list` JSON golden (1.2 MB, churns with the catalog;
  shape is pinned by `TestSchemaListJSONIncludesGetKeyForGetResources`).

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, request changes: SKILL.md named reason `unauthorized` for rejected
  credentials (a OneAPI sign-in rejection is `unknown` today), and AGENTS.md
  kept the old exit wording. Both fixed; also attributed the lite exclusion to
  the API, dropped the remaining counts, renamed the shared ZIA test helper.
- Round 2, request changes: the retry test claimed an SDK default of 4 (only
  `NewConfiguration` sets it; zero is unbounded on this path) and pinned legacy
  settings the SDK never reads. Test now pins OneAPI only; the inert legacy
  lines are removed. Semgrep stderr is kept out of the rule-id grep.
- Round 3, approve with nits: comment precision (legacy loop retries only a
  401; a 429's Retry-After can exceed the 3s cap; stale OneAPI comment), taken.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-19 (round 3)

## Blocking Findings

None.

## Non-Blocking Risks

1. The legacy comment in `newLegacyZIAConfiguration` matches the vendored SDK:
   `sdkzia.NewClient` builds its retrying client only when `HTTPClient` is nil,
   `getHTTPClient` is the only reader of the legacy RateLimit fields, and the
   request loop is `retry <= 5`, continuing only on 401. It could add the
   inline session refresh and that 429/5xx are not retried. (Taken.)
2. `TestOneAPIConfigurationBoundsRetries` is accurate; "waiting 1s to 3s" is
   loose for a 429, whose wait is floored at Retry-After. (Taken.)
3. The OneAPI comment about the legacy struct lacking a field is stale. (Taken.)
4. `verify-semgrep.sh` now greps only JSON stdout with `rc == 1`, closing the
   round-2 edge; `time` stays in use in reader.go.
5. RESOURCES.md rewrap is within 80 columns; round-1 and round-2 fixes hold.
6. `PATH=/bin:$PATH go test ./internal/zscaler ./internal/machine ./internal/cli -count=1`
   passes; `go vet ./internal/zscaler` is clean.

## Verdict

Verdict: approve with nits
