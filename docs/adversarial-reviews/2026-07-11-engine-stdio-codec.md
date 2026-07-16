# Builder Handoff

## Intent

Implement the strict Go DTO/codec foundation for candidate engine stdio
protocol v1 without exposing a process or changing the supported CLI command
tree. Add explicit trusted-engine adapters, shared language-neutral fixtures,
and exact JSON-number handling required by diff results.

## Base / Head

- Base: `648c47f71a06de69a0fca827eac88de06811f4da`
- Head: working tree on `feature/stdio-engine-api`
- Process baseline: `origin/main` at
  `b0597dfb8e673a06d99995e6e1360cfcc709f0a8`

## Files Changed

- `internal/enginewire/*.go`
- `internal/enginewire/adapter/*.go`
- `internal/enginewire/testdata/conformance/*.json`
- `internal/diff/diff.go`
- `internal/diff/diff_test.go`
- `scripts/verify-core-boundaries.sh`
- `scripts/test-verify-core-boundaries.sh`
- `docs/ENGINE_API_DESIGN.md`
- `docs/ENGINE_STDIO_PROTOCOL_V1.md`
- `docs/ROADMAP.md`
- this review artifact

## Source Inputs Consulted

- immutable bootstrap and v1 protocol schemas
- candidate stdio protocol design
- machine request, result, event, manifest, and error source types
- resource catalog, projected-record, filtering, redaction, dump, and diff
  source types
- Go JSON, buffered-I/O, number, and standard-package metadata contracts

## Generated Artifacts

None. The new conformance files are source fixtures. The immutable protocol
schemas are unchanged.

## Expected Delta

- Supported CLI, help, completion, goldens, machine manifest, introspection,
  schema-list output, error envelopes, and exit codes remain unchanged.
- All four bootstrap and 31 v1 root frame branches gain concrete closed Go
  DTO/codec paths.
- Diff report shape remains unchanged. Dynamic JSON numbers retain exact
  lexemes, while mathematically equivalent spellings remain equal and acquire
  consistent identities and content hashes.
- No executable stdio host is added in this checkpoint.

## Invariants Claimed

- The root transport package imports only packages in Go's authoritative
  standard-library set and cannot use cgo.
- Its adapter imports only standard packages plus the exact `diff`,
  `enginewire`, `machine`, `redact`, and `resources` seams.
- Unknown or case-variant fields, decoded duplicate keys, invalid UTF-8 or
  surrogates, malformed scalars, excessive depth, oversized or unterminated
  frames, trailing values, and bare carriage returns fail closed.
- Structural numeric work is bounded and JavaScript-safe integers are exact.
- Dynamic-number canonicalization never allocates in proportion to an
  exponent's numeric magnitude.
- Fragment chunks use canonical base64 and enforce decoded bounds.
- Adapters strip local paths, raw error text, source-only metadata, and
  unsupported credential names.
- Dynamic values are defensively copied and unsupported values, cycles, and
  non-finite floats fail closed.

## Tests Run

- `go test ./internal/diff ./internal/enginewire/... -count=1`: pass
- `go test -race ./internal/diff ./internal/enginewire/... -count=1`: pass
- `go vet ./internal/diff ./internal/enginewire/...`: pass
- `env -u GOFLAGS go test ./... -count=1`: pass
- `bash scripts/verify-core-boundaries.sh`: pass
- `bash scripts/test-verify-core-boundaries.sh`: pass
- `git diff --check`: pass

## Known Deferrals

- Coordinator, operation worker, success barrier, fragmentation sequencing,
  signals, EOF/broken-pipe handling, and an executable host belong to the next
  checkpoint.
- TypeScript and Rust clients remain unimplemented.
- Cross-platform process conformance and protocol promotion remain blocked.

## Finding Resolution

The initial fresh-context review found two blockers:

1. `canonicalIdentityNumber` expanded a source-controlled exponent with
   `strings.Repeat`. Canonical exponent arithmetic now uses `big.Int` only in
   proportion to source digits, and plain identity rendering is capped at
   8 KiB. Regressions cover `1e-1000000000`, equivalent spellings, the signed
   64-bit boundary, zero, and ordinary plain IDs.
2. The adapter dependency check was a denylist that admitted direct SDK and
   arbitrary bridge imports. It became a closed direct-import allowlist with
   independent SDK, bridge, and third-party rejection fixtures.

The focused re-review then found that the dotless standard-package heuristic
also admitted cgo's special `C` import. Both transport checks now determine
standard-library membership from exact `go list std` output, and independent
`C` fixtures are rejected for both packages.

The builder also addressed the original non-blocking conformance risks:

- both immutable schema IDs and exact byte hashes are pinned and tested;
- successful shared codec cases assert their complete canonical output; and
- all four bootstrap root branches are represented in the shared corpus.

# Adversarial Review

Fresh-context reviewer: Bacon (`gpt-5.6-sol`, high,
`019f54cc-c1fd-7473-9ab7-36e04ee2478c`)

The reviewer was read-only, confirmed the process baseline from `origin/main`,
inspected the actual working tree, ran focused tests, and independently supplied
adversarial exponent, SDK, bridge, third-party, and cgo fixtures.

## Blocking Findings

The reviewer requested changes for the three issues recorded above. The final
narrow re-review independently confirmed the fixes. No blocking findings
remain.

## Non-Blocking Risks

None within this checkpoint after the schema-hash and canonical-output fixture
hardening. Host lifecycle and cross-language promotion work remain explicit
deferrals rather than codec claims.

## Machine Contract Review

- No supported command or machine-contract shape changes.
- Diff numbers preserve exact lexemes without exponent-sized expansion.
- Equivalent huge-exponent identities compare consistently.
- Both immutable schema byte hashes match their checked-in files.
- Complete expected fixture output is now compared, not only frame type.

## Safety Review

- The original exponent-driven allocation sink is no longer reachable.
- Direct SDK, arbitrary internal bridge, arbitrary third-party, and cgo imports
  are mechanically rejected at the transport boundary.
- Current adapters expose no raw paths, backend messages, source metadata, or
  unsupported credential names.
- Canonical base64, dynamic-value copying, cycle rejection, and non-finite
  value rejection were independently checked.

## Generated Artifact Review

No generated artifact changed. Protocol schemas remain byte-for-byte
unchanged. New source fixtures and documentation deltas are intentional.

## Verdict

Verdict: approve
