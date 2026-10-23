# Builder Handoff

## Intent

chore(security): clear the gosec and CodeQL alerts on PR #135

PR #135's code-scanning checks reported 16 new alerts (11 gosec, 5 CodeQL),
all false positives or idiom fixes. No behaviour change.

- gosec G101 (8): four constant lines named after credential concepts
  (`credential_material`, `opaque_blob`, two punctuation sets) and four
  `*UnmappedReferences` blocks whose API field names mention tokens and
  certificates. gosec reports a map entry's whole inner literal at its key
  line, so those four `#nosec` comments cover the block; the
  `*FieldReferences` copies of the same keys raised nothing and are not
  annotated.
- gosec G304 (2): `dump` opens operator-supplied paths by design; the
  justification names the check done on the opened handle.
- gosec G104 (1): `sign_in.go` now discards `Body.Close()` explicitly.
- CodeQL go/allocation-size-overflow (5 alerts, 3 sites): capacity hints
  `len(a)+len(b)` and `len(m)+1` dropped to `len(a)` / `len(m)`. Only the
  initial allocation changes; append and map insert grow as needed.

## Base / Head

- Base: `integration/2026-10-09` at `9496024`.
- Reviewed head: `fix/scanner-alerts` (staged).

## Tests Run

- `go test ./...`, `make fmt-check vet`: pass.
- `gosec@v2.26.1 ./...`: 0 issues (45 nosec). CodeQL runs only in CI.

## Review History

- Round 1 (fresh-context Claude subagent), approve with nits. Taken: the
  key-line G101 suppressions are cut to the four alerted blocks (the
  `*FieldReferences` copies were unneeded), with a comment that the block is
  covered; the counts above are reconciled; the root-entry justification
  names parent-ancestry paths too.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-23 (round 1)

## Blocking Findings

None.

## Non-Blocking Risks

- A key-line `#nosec` suppresses G101 for the whole inner map literal, so a
  future entry there goes unscanned. Accepted for catalog metadata; narrowed
  after review to the four alerted blocks.
- Handoff counts did not match the diff. Fixed after review.

## Checked, Sound

- G304 justifications are true: the dump root is opened `O_DIRECTORY`
  (`O_NOFOLLOW` unless following symlinks) and `SameFile`-checked; entries
  are fstat'd and compared with an earlier `Lstat` by all three callers.
- G101 constants are names or punctuation, not secrets.
- Capacity changes and `_ = response.Body.Close()` are behaviour-neutral.
- Nothing else changed; `redact.go` strings are byte-identical.

## Tests Run

- `go test -count=1 ./internal/diff ./internal/resources ./internal/zscaler ./internal/dump`: ok.

Verdict: approve with nits
