# Builder Handoff

## Intent

perf(redact): make the structured-scan fast path fire

`Redactor.structuredScanMayMatch` returns the input unscanned when no rule
prefilter admits it. It always returned true: the last base rule,
`paste_credential`, had no prefilter, and a missing prefilter matches
everything. Every clean string paid for the full prefilter sweep, a JSON
decode and every rule.

- New prefilter kind `prefilterPasteCredential` (calls
  `pasteCredentialMayMatch`, which `scanPasteCredentials` already self-gated
  on) set on the `paste_credential` rule; the special-case call in the fast
  path is gone.
- The fast path also admits any input holding a redaction marker: the full
  scan re-encodes such a JSON string (escaping U+2028/U+2029, replacing
  invalid UTF-8), so skipping it changed bytes.
- `prefilterText.containsAnyFold` memoizes per needle slice; seven assignment
  rules share one list. The view is rebuilt whenever a rule rewrites the text.
- `TestFastPathAdmitsEveryJSONKeyClassifier` pins the invariant the fast path
  now relies on: every key the structured scanner redacts by JSON context
  (assignment-key, generic-credential, credential-context, primary-key,
  credential-array and paste-label regexes) contains a needle some prefilter
  admits. It expands each regex's alternatives with `regexp/syntax`.

Measured (darwin/arm64): ScanStringClean 9.6µs → 2.1µs; BytesCleanJSONBody
6 MB/s → 54 MB/s, 34,856 → 7 allocations.

## Base / Head

- Base: `integration/2026-10-09` at `1b567c8` (rebased from `14b1ae1`).
- Reviewed head: working tree on `perf/redact-prefilter`.
- Final head: the reviewed change, the round-1 fixes, and this artifact.

## Files Changed

- `internal/redact/redact.go`
- `internal/redact/redact_prefilter_test.go` (seeds)
- `internal/redact/redact_fastpath_test.go` (new)

## Tests Run

- `PATH=/bin:$PATH go test ./...`, `make fmt-check`: pass.
- `FuzzScanStringPrefiltersMatchUnfilteredRules` (ScanString must equal a scan
  with no prefilters and no fast path, all modes): 3 minutes before review,
  2 minutes after the fixes; no failure. The reviewer ran a differential fuzz
  against the base path (576k execs) and `-race`.
- Mutations: dropping the marker check fails the new U+2028 seed; adding `pin`
  to `genericCredentialJSONKeyRE` fails the classifier test.

## Known Gaps

- `pasteCredentialMayMatch` can run twice per string that reaches the rules
  (prefilter, then the scanner's own gate); left for clarity.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, approve with nits: no leak found, but output was not byte-identical
  for a marker-bearing string with U+2028 or invalid UTF-8 (fixed: the fast
  path admits markers; seeds added), and the key-classifier invariant was
  unpinned (fixed: new test). The double paste gate is a known gap.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-19 (round 1)

## Blocking Findings

None. Every non-rule rewrite on the structured path is gated by a literal that
is also a rule needle; any backslash forces the full scan; without one every
decoded token is a raw substring and all prefilter kinds are monotone under
substring; Unicode folding matches Go's `(?i)`; share-mode email and IP need
`@` and `.`; entropy and UUID handling run after ScanString. The memo is keyed
by distinct backing arrays and cleared with every view; views are per-call
locals. The fuzz oracle bypasses the fast path, so it is not vacuous. A
differential fuzz against the base (90 s, 576k execs, all modes) and `-race`
pass.

## Non-Blocking Risks

1. Not byte-identical: a JSON string holding `<REDACTED:EMAIL>` or
   `<REDACTED:IP>` with U+2028 or invalid UTF-8 was re-encoded by the base and
   is now returned raw; the PR's fuzz invariant has this counterexample.
   (Fixed.)
2. A new implicit invariant: every key-classifier literal must contain some
   prefilter needle; a future key without one would leak. (Pinned by a test.)
3. `pasteCredentialMayMatch` builds a fresh view and runs twice for strings
   that reach the rules. (Known gap.)

## Verdict

Verdict: approve with nits
