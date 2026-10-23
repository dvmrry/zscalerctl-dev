# Builder Handoff

## Intent

refactor: share dump/diff selection, credential sanitizing and record rows

From a code-level pass over integration (`deadcode`, `dupl`, a render
benchmark). Behaviour-preserving; messages, kinds and check order unchanged.

- `selectReadSpecs` (new `internal/runtime/selection.go`): `prepareDumpRequest`
  and `prepareDiffRequest` each repeated about 90 lines of catalog validation,
  product selection, resource selection and spec filtering, differing only in
  key type and error constructors. Each now passes its own errors
  (`selectionErrors`); diff still sorts its specs, dump keeps catalog order
  and its final cancellation check. `dumpSelectionKey` is gone (both use
  `diff.ResourceKey`). `selection_test.go` pins the selected specs, products
  and resources and each failure path, which no test asserted before. The
  blank and padded checks are defensive: catalog names allow only
  `[a-z0-9-]`, so such input is also rejected as unknown, and the tests
  cannot tell the two apart.
- One missing-credentials sanitizer: `url_lookup.go` and `dump_engine.go`
  had copies with different known-name lists (URL lookup lacked the ZPA
  customer and microtenant ids). One `isKnownCredentialName` now holds the
  full list. URL lookup is ZIA-only, so its output does not change.
- `render_records.go`: the four record renderers built cells and rows
  separately; `scannedCells` and `scannedKeyValues` now do it once, so the
  table/pretty/JSON redaction parity rests on one row builder.
- Deleted with no callers: `runtime.NewStatusInspector` (superseded by
  `NewStatusInspectorFromConfig`), `zscaler.ValidateReaderConfig` (its
  agent-eval fixture no longer exists).

Left alone: the per-product SDK field mappings (the reader's explicit
field lists, kept on purpose) and the handler registrations, which already
share `newListGetHandler`; the enginewire marker methods; and the
table/pretty render cost (about 11 us/record vs 5 us for JSON, negligible
against live reads).

## Base / Head

- Base: `integration/2026-10-09` at `2cb1657`.
- Reviewed head: `chore/dedup-pass` (staged).

## Tests Run

- `go test ./...`, `make fmt-check vet staticcheck`: pass.
- `deadcode -test ./...`: only the enginewire marker methods and the
  `livesmoke` runner (used by the `//go:build ignore` live-smoke script).

## Review History

- Round 1 (fresh-context Claude subagent), approve with nits. Taken: the
  selection test now asserts the products set (a mutant dropping it passed
  every test, and diff would then compare every product); the handoff no
  longer claims the blank/padded checks are pinned. Accepted: under a
  cancellation landing inside one call with nothing readable, diff now
  returns the boundary error where it returned "selection is empty".

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-23 (round 1)

## Blocking Findings

None.

## Non-Blocking Risks

- The products set was not asserted; a mutant dropping it survived. Fixed
  after review (killed).
- Blank/padded checks are equivalent mutants: the key is also unknown.
  Accepted as defensive; handoff corrected.
- Diff only: a cancellation landing after the catalog loop with nothing
  readable now reports the boundary error, not "selection is empty".
  Accepted (needs a race within one call; arguably more correct).

## Checked, Sound

- Error precedence, kinds, messages, sentinels and Operation match base for
  dump and diff; fresh error values and maps per call.
- Same specs and order (dump catalog order, diff sorted); products and
  resources sets match base (resources nil without selectors).
- URL lookup may now name the two ZPA id env vars: names on the allow-list,
  not values, and the list dump/reads already used.
- `render_records.go` output byte-identical; every value from
  `scan(record.Fields())`.
- Removed symbols have no remaining references.

## Tests Run

- Overlay mutants of `selection.go`: 6 killed (readable check, duplicate
  product, duplicate resource, product filter, non-nil resources, empty
  success); padded-resource survived (equivalent); products killed after the
  fix.
- `go vet` and `go test` on runtime, cli, zscaler, diff: pass.

Verdict: approve with nits
