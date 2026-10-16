# Builder Handoff

## Intent

fix(redact): keep true, false and null under secret-named keys

The JSON output scanner replaced any scalar whose key ends in a secret word
(`password`, `token`, `secret`, `session`, `psk`, ...) with
`<REDACTED:SECRET>`, whatever its type. Reviewed settings such as ZCC's
`policyExtension.generateCliPasswordContract.allowZdxDisableWithoutPassword`
(catalog: "boolean setting, not a password") therefore rendered as a secret
marker in JSON, telling an agent a setting is a secret. A literal cannot carry
a credential, so `true`, `false` and `null` now pass; strings and numbers
under such keys are still replaced.

## Base / Head

- Base: `integration/2026-10-09` at `716142e`.
- Reviewed head: `601c889`.
- Final head: review-nit fixes and this artifact on top of `601c889`.

## Files Changed

- `internal/redact/redact.go`, `internal/redact/redact_test.go`

## Tests Run

- `go test ./...`: pass.
- Live, non-production tenant: `--count-by` on the ZCC setting now returns
  21 `true` / 5 `false` instead of one `<REDACTED:SECRET>` bucket; table
  output shows `false`.
- Catalog-wide: of the non-secret catalog fields whose names match the
  secret-key pattern, 3 are booleans (now shown) and 2 are integers
  (`zcc/application-profiles` `refreshKerberosToken`,
  `policyExtension.zpaAuthExpOnWinLogonSession`), still replaced in JSON.

## Known Gaps

- The two integer settings above stay masked in JSON while table and pretty
  output show them (pre-existing: those formats have no `key:` shape for the
  text rules to match). Numbers stay covered deliberately, since a numeric PIN
  or OTP is possible.
- The plain-text assignment rules still replace `password: false` in free
  text; a pasted fragment has no types. Documented in the code comment.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, approve with nits (below). Acted on after review: the comment now
  says the text path stays stricter; the test also covers
  `ScanRenderedString`, the provisioning-key and private-key classifiers,
  whitespace before the value, and an NDJSON stream.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-16 (round 1)

## Blocking Findings

None. `scanJSONDocument` is reached only after `json.Valid` (whole document
and per NDJSON line), so a value starting with `t`, `f` or `n` is exactly a
literal; invalid near-JSON (`{"password":nul}`, a truncated document) falls to
the text path and is still masked. Strings under secret keys, including
`"false"`, are still replaced; the generic-label and paste-credential paths act
only on strings and are unchanged; idempotency holds. No test, golden or doc
depended on masked booleans.

## Non-Blocking Risks

1. nit: the plain-text literal rule still masks `key: false`, so the comment
   overstated parity. Comment updated.
2. nit: the test did not pin `ScanRenderedString`, NDJSON, nested JSON, other
   classifiers or whitespace. Added.
3. nit: `"password": null` versus a masked string reveals whether a credential
   is set; that is metadata, not credential material. Accepted.

## Verdict

Verdict: approve with nits
