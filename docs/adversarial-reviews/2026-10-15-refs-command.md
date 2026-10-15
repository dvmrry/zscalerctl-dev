# Builder Handoff

## Intent

feat(refs): find which records reference an object

`schema references` says which fields can point at a resource; answering "is
this object used, and where" still took one filtered list per site. The
operator asked for exactly that and nothing more: "is it being used and where
is it being used or referenced? That's fine", while the tool must not explain
what a reference does. `refs to <product> <resource> <id>` reads every resource
with such a field (live or `--from-dump`) and lists the records whose field
holds the id, with each record's id and name. `refs check <product> <resource>`
reads a resource and the targets of its reference fields and lists each
referenced id that is not in its target lists as read. Every site the mode
omits, every unreadable resource, every failed read (failure reason only) and
every field whose target could not be read is listed under `not_checked`.

## Base / Head

- Base: `integration/2026-10-09` at `b00cf79`.
- Reviewed heads: `2c90eba` (round 1), `e51374b` (round 2).
- Final head: a summary-wording nit fix and this artifact on top of `e51374b`.

## Files Changed

- `internal/resources/refs.go` (new), `internal/resources/references.go`
  (shared `referenceMatch`)
- `internal/cli/commands_refs.go` (new), `internal/cli/snapshot.go`
  (`savedDumpRuntime` factored out), `internal/cli/app.go`,
  `internal/cli/commands_helpers.go`, `internal/cli/render_help.go`
- tests: `internal/resources/refs_test.go`, `internal/cli/commands_refs_test.go`,
  NDJSON policy and introspect counts
- `man/zscalerctl.1`, `docs/cli/zscalerctl.md`, surface goldens,
  `surface_changes.md`, `AGENTS.md`, `skills/zscalerctl/SKILL.md` (+ `.agents`
  copy), `docs/cli/agent-machine-workflow.md`

## Tests Run

- `go test ./...`, `gofmt -l`, `go vet`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client`: pass.
- Live, non-production tenant, share mode: `refs check zia
  url-filtering-rules` read 153 records and compared 1,119 references; not
  found in what was read: one source IP group (known from an earlier audit),
  8 references to one negative built-in location id, 135 `lastModifiedBy`
  references to 2 admin ids absent from `admin-users`. `refs to zia groups
  <most-referenced group>` found 37 records, equal to an independent count.

## Known Gaps

- `--format table` and `pretty` produce the same text.
- `refs check` on a resource with no list or show operation returns an
  untyped error.
- A hit on a field with several targets does not say which target list holds
  the id.

## Review History

Strata on gm01 was unavailable (the service was stopped on the host during
the first request), so the review ran in a fresh-context Claude subagent with
no conversation history and read-only repository access.

- Round 1, request changes: refs text output interpolated record names and
  ids without `output.TerminalCell`, so control characters could forge result
  lines (reproduced in a scratch copy). Fixed in `e51374b`; a control-character
  test covers both subcommands. `refs to` also gained `sites_checked`.
- Round 2, approve with nits (below). The main nit (the summary counted hits
  as records) is fixed after review by rewording the line to "references to
  the id found".

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-15 (round 2)

## Blocking Findings

None.

Round-1 B1 (unescaped record values in `refs` text output) is fixed in e51374b. I checked this in two ways:

- **Code:** `refsRecordText` and the miss id in `renderRefsCheck` now go through `output.TerminalCell`. The other values interpolated into the text output come from the catalog (product, resource, field, `target.String()`, `doc.Source`). The failure strings come from the fixed reason/kind vocabulary. None of these come from record data.
- **Repro:** I re-ran my round-1 repro (hostile name: newline, tab, ESC) against the new sources in a scratch copy, in both table and pretty. The output is one hit line with the name escaped (`name=Evil\nzia firewall-rules\tgroups\tid=9 name=Forged\x1b[2J`), with no raw control bytes and no forged line. The new `TestRefsTextEscapesRecordValues` test pins this for both `refs to` and `refs check`.

## Non-Blocking Risks

- **Summary line overcounts records.** The new `refs to` summary line says `%d records reference the id`, but the number is `len(ReferencedBy)`. That counts (site, record) hits, so a record that holds the id in two reference fields is counted twice. Say "references" or "hits", or count distinct records.
- **`sites_checked` is only a count.** It is the number of reference sites read, without a list of which ones. That is enough to tell "zero sites" from "all sites read, none hit". A list would let a consumer reconcile it against `schema references`.
- **Carried over from round 1, by decision:**
  - `--format table` and `--format pretty` produce identical output.
  - `refs check` on a resource with no list or show operation returns the untyped `ErrResourceNotReadable`.
  - The float64 path for ids at or above 2^53 is a fallback only, since records decode numbers as `json.Number`.
  - A hit on a field with more than one target (`locations` / `sublocations`) does not say which target list the id belongs to.

## Machine Contract Review

- **JSON change:** `RefsTo` gains `sites_checked` (an int, always present). This adds to a document introduced on this same branch, so no released contract changes. The docs do not spell out field names for the refs JSON (they name no JSON fields at all), so no doc drift.
- **Text change:** the `refs to` text now always starts with a summary line. It used to print only "No record read references this ... id." when nothing was found.
- **Unchanged since round 1:** exit codes, `--from-dump` admission, and introspection metadata.
- **Tests:** `go test ./internal/resources ./internal/cli ./cmd/...` passes on e51374b, including the surface goldens.

## Safety Review

- The text sink now escapes control characters in every record-derived value.
- The JSON path is unchanged: `Select` only narrows already-projected records, the queried id is still not echoed, and failure strings still come from the closed vocabulary.
- No change to projection, redaction or skip/miss logic. `sites_checked` is incremented only after a site has been read successfully.

## Generated Artifact Review

- e51374b touches only `internal/cli/commands_refs.go`, its test, and `internal/resources/refs.go`.
- No generated artifacts changed, and none needed to: the help, introspection and inventory surface is unchanged, and the goldens pass.

## Verdict

Verdict: approve with nits
