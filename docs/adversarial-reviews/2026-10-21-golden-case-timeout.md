# Builder Handoff

## Intent

test(golden): report golden case timeouts, with a wider per-case limit

An external run saw `TestGoldenSurface/introspect` exceed its 15 s per-case
limit under parallel load; it passes alone.

- Not reproduced here: with the rest of the suite running alongside, the
  slowest cases took 0.43 s (`introspect`) and 0.88 s (`introspect-pretty`);
  with 36 busy loops on 12 cores, 0.61 s and 1.30 s. The cause on the
  external host is unknown.
- Bug found while checking: a context kill makes `cmd.Run` return an
  `*exec.ExitError` (signal: killed), which both runners checked before the
  deadline, so a timeout reported `exit code = -1` and a golden mismatch,
  never "timed out". The deadline is now checked first.
- `goldenCaseTimeout` (60 s) replaces the two 15 s literals, and the timeout
  message reports the elapsed time and the limit.

Test-only; no production code changes.

## Base / Head

- Base: `integration/2026-10-09` at `63de9e0`.
- Reviewed head: `test/golden-case-timeout` (staged).

## Files Changed

- `cmd/zscalerctl/golden_surface_test.go`

## Tests Run

- `go test -count=1 ./cmd/zscalerctl`, `make fmt-check`: pass.
- With the limit temporarily set to 1 ms, `introspect` fails with
  `case timed out after 1ms (limit 1ms): args=[introspect]`; before the
  reordering it failed with `exit code = -1, want 0`.

## Review History

- Round 1 (fresh-context Claude subagent), approve with nits. Taken: the
  timeout message includes the case's stderr so far (truncated to 2000
  bytes). Left: a non-zero exit landing within microseconds of the 60 s
  deadline is labelled a timeout; no `WaitDelay` (pre-existing).

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-21 (round 1)

## Blocking Findings

None.

## Non-Blocking Risks

- A case exiting non-zero on its own just before the deadline is labelled
  "timed out"; the window is microseconds. Accepted.
- The timeout message omitted output. Fixed (stderr so far).
- No `cmd.WaitDelay`; a grandchild holding stdout could block past the kill.
  Pre-existing; no evidence it happens.

## Checked, Sound

- With the default Kill cancel, `Wait` returns an `*exec.ExitError`, so the
  old timeout branch was unreachable; checking `ctx.Err()` first is correct
  in both runners. Normal non-zero exits still report their code.
- 60 s per case under `t.Parallel()` stays far inside the 10 min
  `go test -timeout`.
- No caller depends on the old message or on a -1 code.

## Tests Run

- `go test -count=1 ./cmd/zscalerctl`: ok.

Verdict: approve with nits
