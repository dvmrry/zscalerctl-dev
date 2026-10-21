# Builder Handoff

## Intent

fix(output): scan table/pretty record values like JSON, not the rendered text

Record output in `table` and `pretty` (list, get, show) ran the final scan
over the whole rendered text, which reads layout as prose. A nested object in
a cell renders as `map[zpaAuthExpOnWinLogonSession:480]`, which the plain-text
assignment rules took for a pasted assignment, so table/pretty showed markers
where JSON showed values: ZCC `zpaAuthExpOnWinLogonSession` and
`refreshKerberosToken` numbers, and the nested `generateCliPasswordContract`
booleans. Separately, the stderr omitted-field notice went through the prose
scan, which read long catalog field names with digits as tokens: live,
`requested zia/advanced-settings fields omitted in share mode:
<REDACTED:SECRET>` for `dnsResolutionOnTransparentProxyIPv6Apps`.

Model: every projected record gets JSON output's final scan, in every
format.

- `output.Renderer.ScanRecord(record)`: encodes the record (the object JSON
  output emits, already narrowed by `--fields` in the read), applies the
  renderer's `Bytes` scan as one document, and decodes with `UseNumber`. A
  record that does not encode, or whose scan does not decode, has every
  member replaced by the secret marker.
- `output.Renderer.WriteScannedText(w, build)`: `build` gets `ScanRecord` and
  must render every value from scanned records; the text is written without
  `WriteText`'s whole-text scan.
- The four record renderers in `internal/cli/render_records.go` take `scan`
  and render cells from `scan(record.Fields())`. Their other text is field
  names (catalog paths, `--fields` validated against the catalog) and layout.
  Terminal escaping (`TerminalCell`) is unchanged and runs after the scan.
- Numbers in these cells now render as JSON writes them (`1000000`, not
  `1e+06` from a float64), since every record is decoded with `UseNumber`.
- Notices built only from catalog names (`noteOmittedFields`: omitted and
  secret field notices) go to `App.catalogNotices`, written after the other
  notices, on success only, unscanned. Other notices (implicit profile) stay
  in `App.notices`, scanned as before.

Unchanged: JSON/NDJSON; all other text output (count tables, diff, refs,
url-lookup, status, introspect, help, error lines) keeps the whole-text scan.

## Base / Head

- Base: `integration/2026-10-09` at `dc2461c`.
- Reviewed head: `fix/record-text-value-scan` (staged).

## Files Changed

- `internal/output/output.go`, `internal/output/scan_value_test.go` (new)
- `internal/cli/render_records.go`, `internal/cli/app.go`,
  `internal/cli/omitted_notice.go`
- `internal/cli/render_parity_internal_test.go` (new),
  `internal/cli/render_parity_test.go` (new),
  `internal/cli/omitted_notice_test.go`, `internal/cli/app_internal_test.go`
- `docs/ARCHITECTURE.md`

## Tests Run

- `PATH=/bin:$PATH go test ./...`, `make fmt-check`: pass; no golden changed.
- `TestRecordTextRedactsLikeJSON`: every catalog field at every depth, every
  mode that renders it, six sample types, under the top-level field and the
  dotted path, through all four record renderers (table/pretty, list/show),
  with the record `Select`ed as a read narrows it: text shows a marker
  exactly when JSON does. The old path (identity scan plus the whole-text
  scan) fails it on the ZCC numbers and booleans.
- `TestRecordTextRedactsSiblingContextLikeJSON`: the round-1 ZTW
  `workload-groups` tag pair (`{"key":"password","value":...}` and a
  credential-shaped key), with `--fields` selecting both, the parent, or one
  side, through a list.
- Mutations: rendering `show` from an unscanned record fails 135 cases;
  every renderer unscanned fails 540.
- `TestOmittedFieldNoticeKeepsLongCatalogNames` fails without the notice
  change with exactly the live output above.
- `TestRecordTextScansEveryRecordWrite`: through `App.Run`, table and pretty
  x list, get and show (synthetic catalog with a list/get and a singleton
  resource), with and without `--fields`: a `{"key":"password","value":...}`
  tag value, which only the final scan's sibling rule catches, stays hidden.
  `TestRecordTextShowsWhatJSONShows`: the nested ZCC
  `zpaAuthExpOnWinLogonSession` 480 shows in table and pretty. Mutations that
  pass an identity scan from the list-table or show-table builder, or re-add
  the whole-text scan inside `WriteScannedText`, each fail them.
- `TestPastedCredentialRedactedInEveryFormat`: a pasted `token=ghp_...` in a
  rendered `description` is redacted in json, table and pretty, all modes
  (projection already catches it; this pins the formats agree).
- `TestScanRecordAppliesTheJSONScanToTheWholeRecord`: secret-named keys,
  containers, reviewed settings, literals, a key/value pair, float
  formatting, an unencodable member.

## Known Gaps

- `--fields` narrows a record in the read, before the final scan, so a
  selection that drops a sibling also drops the context its rule needs:
  `--fields tags.value` shows a `{"key":"password"}` pair's value in JSON
  output as in text. Pre-existing on the JSON path; text now matches it.

- Count tables, diff tables and `refs` text still scan the rendered text, so
  a nested value there can still show a marker that JSON would not.
- The whole-text scan also redacted by adjacency across cells (a `name` of
  `password:` beside a `description`); JSON output never did, and neither do
  table and pretty now.

## Review History

- Round 1 (fresh-context Claude subagent), request changes:
  1. Scanning each cell alone (`ScanValue(key, value)`) lost sibling
     context JSON's scan uses: on the shipped ZTW `workload-groups` tags, a
     `{"key":"password","value":...}` pair showed its value in table output
     where JSON and the old table redacted it. Replaced by `ScanRecord`
     (whole record), which the reviewer's harness showed leaves no case where
     text shows what JSON hides.
  2. Nothing tested that the `show` renderers scan. The parity test now runs
     all four renderers.
  Also taken: stable number formatting, the stale `pretty.go` comment and the
  ARCHITECTURE.md wording.
- Round 2 (fresh-context Claude subagent), request changes: nothing tested
  the `App.Run` wiring, so a builder passing an identity scan passed the
  suite and leaked (reproduced). Added the two `App.Run` tests above and a
  top-level name/value pair case for `ScanRecord`. Recorded: the `--fields`
  narrowing gap; per-record failure granularity; number and invalid-UTF-8
  display now match JSON.
- Round 3 (fresh-context Claude subagent), approve with nits. Production
  code unchanged since round 2. Noted: routing catalog notices through
  `Redactor.Bytes` instead of the stream writer would not fail the notice
  test (fail-safe either way).

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-21 (round 3)

## Blocking Findings

None.

## Non-Blocking Risks

- The notice test catches routing back through `redact.NewWriter`, not a
  switch to `Redactor.Bytes`; scanning more is fail-safe.
- The synthetic `pairCatalog` is not run through a catalog validator; the
  identity-scan mutants leak through it, so it renders the pair in every mode.

## Checked, Sound

- Production code byte-identical to the round-2 head; only tests and this
  document changed.
- Mutants via `-overlay`, each killed: identity scan in every builder
  (list/get/show x table/pretty), call sites back on `WriteText`, the
  whole-text scan re-added in `WriteScannedText`, a per-member top-level
  scan, and the round-1/2 set.
- Differential vs base `dc2461c` (45 records, 11 `--fields` sets, 3 modes,
  list/get/show, single and multi-record): text shows what JSON hides 0, text
  hides what JSON shows 0, JSON identical to base. Values head text now shows
  that base text hid: the nested ZCC `480` and cross-cell adjacency, all of
  which JSON shows.
- The `--fields` narrowing gap is pre-existing on the JSON path at base.
- Catalog notices carry only catalog-resolved names and mode names, on
  success only.

## Tests Run

- `go test ./...`; about 25 overlay mutants on `internal/cli` and
  `internal/output`; the base/head differential harness. `make fmt-check`
  was run by the builder after the reviewer's disk filled.

Verdict: approve with nits
