# Builder Handoff

## Intent

fix(redact): one rule per JSON value type under secret-named keys

The final scan replaces the value of a JSON key that ends in a secret word.
Two problems, one cause (the rule was patched per type: strings and numbers
replaced, containers scanned only by their strings, literals kept since
601c889):

- **Leak (external review of 12bf72b):** `{"password":{"v":"..."}}` and
  `{"password":["..."]}` passed through unchanged in every mode, because
  containers were never replaced and their inner strings sit under neutral
  keys. Defense in depth only (projection is allow-list), but it applies to
  error envelopes, sdklog and pasted JSON.
- **Over-redaction (found live):** `zcc application-profiles`
  `refreshKerberosToken` and `policyExtension.zpaAuthExpOnWinLogonSession`
  are integer settings the catalog renders in every mode, yet every record
  showed `<REDACTED:SECRET>`, because any number under a key ending in
  `token`/`session` was replaced.

`secretKeyValueReplaced(key, first byte)` now decides by value type:

- string: replaced (unchanged);
- object or array: replaced whole, with the key's classification marker;
- number: replaced (unchanged from base), except under the exact reviewed
  setting keys `refreshKerberosToken` and `zpaAuthExpOnWinLogonSession`
  (`numericSettingKeys`). Two rounds of pattern exemptions (a list of
  credential words, then compound keys ending in setting words) each kept
  real credential numbers; the catalog is the only thing that knows a field
  is a setting, so the exemption is the reviewed field names;
- `true`/`false`/`null`: kept (unchanged).

`TestFinalScanKeepsCatalogAllowedValues` (internal/resources) pins this for
JSON output: for every catalog field at any depth and every mode that renders
it, `{"<field>": <sample of its type>}` (all types for an untyped field) must
pass the final scan unchanged. Run against the old `redact.go` it fails on
exactly the two ZCC fields. `TestNumericSettingKeysAreCatalogFields` is the
converse: each exempt key is a rendered numeric catalog field.

Scope: JSON output only (`--format json`/`ndjson`, dumps, diff, the engine).
`table` and `pretty` render a nested object as `map[k:v]` text and the final
writer scans that buffer with the plain-text assignment rules, so the nested
`policyExtension.zpaAuthExpOnWinLogonSession` still renders as a marker there
(the top-level `refreshKerberosToken` does not, since a table row has no
`:`/`=` cue). Fixing that means the text writer scanning rendered structure as
structure, a separate change.

## Base / Head

- Base: `integration/2026-10-09` at `12bf72b`; merged onto `517350e`
  (the 12bf72b review fixes, which do not touch `internal/redact`).
- Reviewed head: `fix/redact-secret-key-values`.

## Files Changed

- `internal/redact/redact.go`, `internal/redact/redact_test.go`,
  `internal/redact/redact_prefilter_test.go` (fuzz oracle and seeds)
- `internal/resources/final_scan_test.go` (new)
- `docs/ARCHITECTURE.md`

## Tests Run

- `PATH=/bin:$PATH go test ./...`, `make fmt-check`: pass.
- `TestRedactorJSONReplacesSecretKeyValuesByType` (was
  `TestRedactorJSONKeepsLiteralsUnderSecretKeys`, which pinned the number
  replacement) is a table over every value type in all three modes.
- The fuzz oracle `scanStringWithoutPrefilters` now scans a string that is a
  JSON document as one, as production does; seeded with the round-1 cases.
  60 s of `FuzzScanStringPrefiltersMatchUnfilteredRules` afterwards: pass.
- Fuzz, 45 s each: `FuzzScanStringPrefiltersMatchUnfilteredRules`,
  `FuzzRedactorPreservesValidJSON`, `FuzzRedactorPreservesValidNDJSON`.
- Live (read-only), before the change: both ZCC fields rendered as markers.

## Known Gaps

- Inner strings of a replaced container are still scanned, so the report
  can count a redaction inside a value that was replaced whole.
- `refreshKerberosToken`/`zpaAuthExpOnWinLogonSession` keep a number in any
  JSON, not only catalog records (an error envelope too); they are settings
  by name.
- table/pretty: a container under a secret key is scanned as text
  (`map[password:map[v:...]]`), so an inner scalar can survive. Unchanged
  from base; no rendered catalog field is such a container.
- Plural keys (`secrets`, `passwords`, `apiKeys`) do not match the key rule,
  as before.
- Table/pretty nested values: see Scope.
- JSON embedded in prose (`HTTP 400 body: {"password":{...}}`) is scanned as
  text, as before.

## Review History

- Round 1 (fresh-context Claude subagent), request changes: the
  over-redaction still showed in table/pretty for nested values, and the
  handoff claimed the test stopped it recurring. Scoped to JSON output and
  documented (Scope, ARCHITECTURE.md). Also taken: the tighter number rule
  (credential-value words keep replacing numbers under compound keys), the
  fuzz oracle's nested-document recursion, the test rename and comment fixes.
- Round 2 (fresh-context Claude subagent), request changes: the number rule
  listed the credential words that keep replacing, so secret words missing
  from that list (`userSessionId`, `authJwt`, `deviceCertBlob`) were newly
  kept. Inverted: the exemption (compound key ending in token, session,
  cookie, authorization or key) is the closed list, so it fails closed. Also
  taken: Known Gaps for table/pretty containers and plural keys; a comment on
  the catalog test's samples.
- Round 3 (fresh-context Claude subagent), request changes: the compound
  exemption still kept credential numbers (`tlsPrivateKey`, `ikePreSharedKey`,
  `X-Auth-Token`, `_access_token`, `zpaProvisioningKey`). Third pattern leak,
  so the model changed: numbers are replaced as on base except under the two
  reviewed catalog keys, with a converse test tying the list to the catalog.
  `secretKeyValueReplaced` now checks the key's classification first, so its
  contract holds on its own. Every round-3 example is a test row.
- Round 4 (fresh-context Claude subagent), approve with nits. Taken: the
  handoff's base line and file list; the catalog test comment names
  `numericSettingKeys`. Noted, pre-existing on base: `{"key":"password",
  "value":N}` pair objects keep their value.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-21 (round 4)

## Blocking Findings

None.

## Non-Blocking Risks

- Base drift: integration moved to `517350e`; none of it touches
  `internal/redact`, and redact/resources/cli tests pass on the merged tree.
- Handoff file list omitted `redact_prefilter_test.go`. Fixed.
- `TestFinalScanKeepsCatalogAllowedValues` comment predated
  `numericSettingKeys`. Fixed.
- Pre-existing on base: pair objects (`{"key":"password","value":N}`) and
  plural keys keep their values.

## Checked, Sound

- Differential vs base (all classified keys x 12 wrappers incl. escaped keys,
  JSON-in-string, NDJSON, prose x 4 scanners x 3 modes, plus 45 s of
  differential fuzzing): the only survivors are numbers under exactly
  `refreshKerberosToken` and `zpaAuthExpOnWinLogonSession`; case and fold
  variants are replaced; 0 invalid JSON.
- Absolute check: no string, container or number sentinel survives under any
  classified key except those two numbers.
- Containers replaced whole with the classification marker, valid JSON,
  whitespace preserved; `pairKeyEntry` stays valid.
- Catalog tests: the final-scan test fails on base for exactly the two ZCC
  fields; dropping either map entry fails it; a bogus entry fails
  `TestNumericSettingKeysAreCatalogFields`.
- Docs match the code, including the table/pretty scope.

## Tests Run

- `go test ./...`, `make fmt-check`, `go vet` on redact and resources.
- Overlay mutations (base, bogus key, each key dropped).
- `FuzzScanStringPrefiltersMatchUnfilteredRules` and `FuzzDiffKeys`, 45 s each.

Verdict: approve with nits
