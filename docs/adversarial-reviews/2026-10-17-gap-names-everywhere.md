# Builder Handoff

## Intent

fix(gaps): name declared coverage gaps on every unknown-resource path

Only the CLI read path said why zscalerctl cannot read a declared coverage
gap (for example `zpa policy-rules`). The machine executor returned a bare
"unknown resource", `schema describe` and `schema references` said "resource
is not in the catalog", and `dump --resources` said "unsupported dump
resource". An agent on those paths could not tell "zscalerctl cannot see
this" from a mistyped name, although the skill says exit 4 names the gap.

`resources.UnreadMessage(gap)` builds the sentence from the static gap entry
only. The CLI error, both machine executor sites, both schema commands and
`dump --resources` use it when `resources.FindGap` matches the name (surface
or alias, any case). The requested name or alias is never echoed: paths that
avoided reflecting input still do.

## Base / Head

- Base: `integration/2026-10-09` at `8e09f6d`.
- Reviewed head: staged tree on `fix/gap-names-everywhere` (round 1).
- Final head: round-1 nit fixes (`dump --resources`, alias tests) and this
  artifact.

## Files Changed

- `internal/resources/gaps.go` (`UnreadMessage`), `internal/cli/errors.go`,
  `internal/machine/executor.go`, `internal/cli/commands_schema_describe.go`,
  `internal/cli/commands_schema_references.go`, `internal/cli/dump_diff.go`
- tests: `internal/machine/executor_test.go`,
  `internal/cli/commands_schema_describe_test.go`, `internal/cli/app_test.go`

## Tests Run

- `PATH=/bin:$PATH go test ./...`, `go vet`, `verify-docs`,
  `verify-cli-docs`, `verify-machine-contract`: pass.
- Mutations: reverting the preflight site, echoing the name in the fallback,
  disabling the `schema references` branch, or disabling the bare-name dump
  branch each fails a test.

## Known Gaps

- `unknownResourceMessage` reads the global gap table while the executor's
  catalog can be injected; "Covered instead" could name a resource a custom
  catalog lacks. No production path injects one.
- The engine stdio wire carries failure kinds only, so it gains nothing here.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, approve with nits (below). Acted on: `dump --resources` names the
  gap for `product/name` and bare names (one match in the selected products);
  tests cover an alias in odd case on the machine and dump paths and check it
  is not echoed.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-17 (round 1)

## Blocking Findings

None. `UnreadMessage` reads only gap fields; aliases never render; every gap
note is static text, and passing each message through the machine
sanitizer changed none. The CLI gap output is byte-identical (same format
string and arguments, moved). The machine fixture uses a non-gap name and
message text is documented as diagnostic prose, not a stable surface.

## Non-Blocking Risks

1. minor: `dump --resources` still missed the gap. Fixed.
2. nit: no `surface_changes.md` row. Not added: no golden changes, and the
   manifest records golden deltas.
3. nit: no alias test. Added.
4. nit: the gap table is global while the executor catalog is injectable.
   Known Gaps.

## Verdict

Verdict: approve with nits
