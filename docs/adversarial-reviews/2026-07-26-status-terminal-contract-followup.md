# Builder Handoff

## Intent

Close the two deferred terminal-contract follow-ups from PR #122 without
changing production behavior: document the existing distinction between
lossless projected resource/diff presentation values and normalized typed
status metadata, assert the exact status-normalization disposition, and
exhaustively pin `TerminalCell` coverage of every Unicode `Cf` rune in the
repository's Go toolchain.

## Base / Head

Base commit: `eea0d30e2fbe29bbfa18737489da0b2ed859a9d1`

Reviewed head: `62455a80dc29774f7c0e0b289e5eeda05b866929`

## Files Changed

- `docs/THREAT_MODEL.md`
- `internal/output/terminal_test.go`
- `internal/runtime/status_test.go`

## Source Inputs Consulted

- `internal/output/terminal.go`
- `internal/runtime/errors.go`
- `internal/runtime/status.go`
- `internal/machine/engine_status.go`
- `internal/cli/render_records.go`
- PR #122's approved artifact at
  `docs/adversarial-reviews/2026-07-24-output-auth-wire-hardening.md`

## Generated Artifacts

None.

## Expected Delta

- Production code, CLI surface, output formats, and machine schemas: unchanged.
- Existing status values: unchanged; their pre-render space-normalization
  contract is now stated and asserted exactly, including provider-derived
  scheme metadata.
- `TerminalCell`: unchanged; its tests now enumerate every `unicode.Cf` rune
  and pin the Go 1.26.5 table count at 170.

## Invariants Claimed

- Retained post-projection/redaction resource and diff values are not changed
  by terminal presentation escaping in machine JSON or stdio output.
- Search and filtering use a separate normalized comparison representation.
- Terminal resource/diff presentation continues visibly escaping unsafe runes
  at the sink.
- Doctor/auth/config typed status views continue replacing C0/DEL/C1 and `Cf`
  runes with spaces before either machine or human rendering.
- No secret provider resolves during status inspection.
- No output format, wire shape, exit code, or CLI surface changes.

## Tests Run

- `go test ./internal/output -run 'TestTerminalCell' -count=1 -v`
- `go test ./internal/runtime -run 'TestStatusInspectorNormalizesTerminalControlAndFormatRunes' -count=1 -v`
- `go test ./internal/output ./internal/runtime ./internal/cli -count=1`
- `make fmt-check`
- `make docs-check`
- `make check` at implementation head `5807d22` before the docs/test-only
  review-resolution commit
- Focused status test and docs check at final reviewed head `62455a8`

All commands passed.

## Known Deferrals

The branded OpenTUI `SafeString` presentation boundary and path-filtered
experiment CI are handled in a separate follow-up branch and review.

## Review Focus

- Re-derive the resource/diff and status data flows rather than trusting the
  documentation wording.
- Verify exact status normalization and provider non-resolution.
- Independently enumerate Go's `unicode.Cf` table and verify escape spelling.
- Reject claims of losslessness or visual safety broader than production
  behavior.

## Review Resolution

The initial review approved with three nits at `5807d22`. Commit `62455a8`
scoped the value-preservation and spoofing claims, distinguished typed status
fields from closed static labels, added DEL to the exact normalization input,
and asserted the provider-derived scheme exactly. The reviewer rechecked only
that delta and its affected surface.

# Adversarial Review

Fresh-context reviewer: Goodall (`019f9ee5-4b47-7f00-9121-dc333516cd67`)

Reviewed base: `eea0d30e2fbe29bbfa18737489da0b2ed859a9d1`

Reviewed head: `62455a80dc29774f7c0e0b289e5eeda05b866929`

## Blocking Findings

None.

## Non-Blocking Risks

None.

## Resolution Verification

- The threat model now scopes value preservation to retained
  post-projection/redaction dynamic values, separates filter/search
  normalization from stored values, and limits visual-safety claims to the
  escaped C0/DEL/C1 and Unicode `Cf` classes.
- The status regression includes DEL, checks exact space normalization, checks
  `ClientSecretScheme` as `cmd:` plus the normalized value, and retains the
  zero-provider-resolution assertion.
- Human status rendering is accurately described as typed normalized fields
  plus closed static labels; no raw configuration string is rendered.

## Machine Contract Review

No production, JSON, NDJSON, stdio-wire, error-envelope, exit-code, schema,
manifest, or CLI-surface change was found.

## Safety Review

No redaction, projection, field-classification, provider, or rendering
implementation changed. Source inspection and focused tests confirmed terminal
escaping remains presentation-only, status values normalize before either
machine or human rendering, and status inspection does not resolve command
providers.

## Generated Artifact Review

No generated artifact changed.

## Independent Verification

At exact head `62455a80dc29774f7c0e0b289e5eeda05b866929`, the reviewer independently
passed focused terminal/status/provider/CLI/diff/matching tests, focused race
tests, `go vet` on the affected packages, `make fmt-check`, `make docs-check`,
and `git diff --check`.

Verdict: approve
