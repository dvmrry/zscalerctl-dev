# Builder Handoff

## Intent

Preserve valid JSON and NDJSON when rendered tenant text contains an
Authorization header, without weakening the existing redaction backstop or
changing plain-text header behavior.

## Base / Head

- Base commit: `55270e07f184fde84db526899be5292eca9de56f`
- Initial reviewed candidate: `067ab6f4d052ce6816d09ae3643eeca74621c3d9`
- Reviewed fixed head: `f913b5559c4c9404943230dc39778916fc907b18`
- Process-document baseline: `feature/security-effects-remediation`

## Files Changed

- `internal/redact/redact.go`
- `internal/redact/redact_test.go`
- `internal/redact/redact_fuzz_test.go`
- `internal/redact/redact_prefilter_test.go`
- `internal/redact/bench_test.go`
- `internal/output/output_test.go`
- `internal/dump/dump_test.go`

## Source Inputs Consulted

- The base redaction rules, reports, writer, unit tests, fuzz targets, and
  benchmarks
- `internal/output` JSON and NDJSON render paths
- `internal/dump` JSON and NDJSON file writers
- The CLI's full-buffering outer redaction writer
- The versioning and adversarial-review policy from the stacked base
- Fable's source-to-sink reproduction of the original JSON corruption

## Generated Artifacts

None. No schema, CLI golden, machine manifest, generated documentation, or
agent-skill copy changed.

## Expected Delta

- A decoded Authorization header is redacted inside its JSON string token and
  cannot consume quotes, sibling fields, or NDJSON record boundaries.
- JSON and NDJSON remain parseable through both the renderer pass and the
  CLI's outer rendered-string pass.
- Unicode-escaped Authorization text and escaped high-entropy strings receive
  the same redaction as their literal encodings.
- Long numeric JSON tokens remain numeric and byte-identical.
- Sensitive suffix keys retain the base assignment scanner's marker order and
  report counts.
- Plain-text Authorization behavior, including Digest parameters and
  line-bounded redaction, remains unchanged.
- CLI commands, flags, exit codes, schemas, goldens, and generated artifacts
  remain unchanged.

## Invariants Claimed

- Valid JSON input remains valid JSON after `String`, `Bytes`, and
  `ScanRenderedString`.
- Valid NDJSON preserves record count and LF or CRLF boundaries.
- JSON/NDJSON redaction is idempotent.
- A parser or rewrite invariant failure emits a valid fail-closed marker
  document rather than raw input.
- Plaintext regexes run only inside decoded JSON strings; sensitive scalar
  object values are classified structurally.
- Existing literal redaction markers retain their canonical unescaped form.
- High-entropy JSON string values redact, while numeric JSON values are not
  replaced with syntactically invalid unquoted markers.
- Clean inputs that cannot satisfy a rule prefilter avoid the structured parser.

## Tests Run

- `env -u GOFLAGS make fmt-check test race vet staticcheck` — pass at the
  reviewed fixed head.
- `go test ./internal/redact ./internal/output ./internal/dump` — pass.
- `go test -race ./internal/redact ./internal/output ./internal/dump` — pass.
- `go test ./...` and `go vet ./...` — pass during each fix cycle.
- `FuzzRedactorPreservesValidJSON` — repeated 30-second runs passed; the oracle
  checks JSON validity and idempotence for `String`, `Bytes`, and
  `ScanRenderedString`.
- `FuzzRedactorPreservesValidNDJSON` — repeated 20-second runs passed.
- `FuzzScanStringPrefiltersMatchUnfilteredRules` — repeated 20- and 30-second
  runs passed.
- `FuzzJSONSensitiveKeyClassificationMatchesLegacyAssignments` — exact output
  and report parity passed for more than 200,000 generated field names.
- `FuzzScanRenderedStringRedactsEscapedJSONHighEntropyCanary` — 15-second run
  passed across JSON/NDJSON and all modes.
- `make semgrep-check`, `make secret-scan`, and `make vuln` — pass during the
  candidate/fix verification sequence.
- `git diff --check` — clean.

## Known Deferrals

- This patch does not change projection allow-lists, resource field
  classification, schemas, or dump confidentiality policy.
- No attempt is made to promote redaction reports or internal Go APIs beyond
  their existing supported-surface status.

## Review Focus

- JSON parser spans, escaping, nesting, replacement overlap, and fail-closed
  behavior
- Base-to-head assignment marker and report compatibility
- Literal and escaped Authorization and entropy candidates
- Multiline safe-text preservation
- JSON, NDJSON, dump, and renderer-to-outer-writer composition
- Tests that assert only syntax while missing leaks or data loss
- Clean-path allocation and throughput impact

# Adversarial Review

Fresh-context reviewer: Sol ultra (`Epicurus`, agent `019f4c3d-f585-7a91-91f8-eff09df71ff2`) and Terra ultra (`Bohr`, agent `019f4c3d-f60b-7db2-a4f8-b3f33566fc23`), reviewing `55270e0..f913b55` read-only under the stacked-base process rules.

## Review History

Both initial reviewers requested changes. They independently reproduced a
base-to-head leak for sensitive suffix keys. The Sol reviewer additionally
found that the outer entropy pass could corrupt long numeric JSON, raw-byte
activation missed Unicode-escaped Authorization text, and the JSON-only
dot-all rule dropped safe lines after a decoded newline.

The builder reproduced each issue before fixing it, added direct and composed
regressions, restored exact legacy assignment-marker/report behavior, and
reran focused, full, race, and fuzz verification. On the first recheck both
reviewers found the same remaining bypass: Unicode escapes could split a
decoded high-entropy token before the raw activation check. One reviewer also
identified overlapping-key marker/report drift. Those were reproduced, fixed,
and covered by escaped-token JSON/NDJSON fuzzing plus an exact legacy
classifier oracle.

Both reviewers then rechecked fixed head `f913b55`, the addressed findings,
and the newly changed surface. Each returned an unqualified approval.

## Blocking Findings

None remain.

## Non-Blocking Risks

None identified in the reviewed delta.

## Machine Contract Review

The reviewers verified valid final JSON and NDJSON through the real
renderer-to-outer-writer composition, preserved exact long numeric tokens,
preserved safe text after decoded newlines, and confirmed idempotence. No CLI,
error-envelope, exit-code, schema, manifest, introspection, or generated-output
contract changed beyond repairing previously invalid output.

## Safety Review

Literal and escaped Authorization headers and high-entropy credentials redact
without a fail-open path. Sensitive suffix keys reproduce legacy provisioning,
private-key, and secret marker precedence and every corresponding report count.
Projection, narrowing, field coverage, and dump confidentiality boundaries are
unchanged.

## Generated Artifact Review

No generated artifacts changed. The reviewers found no schema, golden, CLI
documentation, manifest, or skill-copy drift.

## Coverage Ledger

| Area | Disposition |
| --- | --- |
| `internal/redact/redact.go` | Full changed-code review; parser, activation, classification, entropy, and fallback paths attacked |
| Redaction unit/fuzz/benchmark tests | Reviewed; leak, validity, idempotence, parity, and performance oracles checked |
| `internal/output/output_test.go` | Reviewed; final JSON and NDJSON composition decoded and verified |
| `internal/dump/dump_test.go` | Reviewed; valid JSON and canonical marker behavior preserved |
| Generated/supported surface | No files changed; no unexplained drift |

Verdict: approve
