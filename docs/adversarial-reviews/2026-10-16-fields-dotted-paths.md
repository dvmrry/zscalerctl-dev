# Builder Handoff

## Intent

feat(fields): accept dotted paths in --fields

`--fields` took top-level names only, so an agent wanting each rule's group
ids had to pull whole `groups` objects (names included) and post-process. It
now takes the dotted paths `--filter` and `--count-by` already take. JSON keeps
the nested shape with only the selected keys (`"groups":[{"id":1},{"id":2}]`),
one entry per list element, elements lacking the key kept as `{}`; a
whole-field selection wins over paths beneath it. Table and pretty output get
one column per path. A path renders only where every field along it renders,
so selection cannot widen output, and the omitted-field notice names
requested paths the mode drops.

## Base / Head

- Base: `integration/2026-10-09` at `df37021`.
- Reviewed head: `a7f925a`.
- Final head: review-nit fixes and this artifact on top of `a7f925a`.

## Files Changed

- `internal/resources/resources.go` (`Select` selection tree,
  `ProjectedPathValue`, `ValidateNarrowOptions`, `EffectiveFields`,
  `OmittedFields`/`fieldChain`), `internal/cli/render_records.go`,
  `internal/cli/globalflags.go`
- tests: `resources_test.go`, `select_internal_test.go`,
  `filter_search_test.go`
- `AGENTS.md`, `skills/zscalerctl/SKILL.md` (+ `.agents` copy), `README.md`,
  `docs/cli/agent-machine-workflow.md`, `docs/cli/zscalerctl.md`, surface
  goldens, `surface_changes.md`

## Tests Run

- `go test ./...`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client`: pass.
- Live, non-production tenant, share mode: `--fields
  id,locations.id,groups.id,urlCategories zia url-filtering-rules list`
  returned nested `groups:[{id}]`; a table read with `groups.name` printed a
  `groups.id` column and named `groups.name` on stderr as omitted in share
  mode.

## Known Gaps

- A table cell for a path through a list does not keep element positions
  (elements lacking the key are skipped); JSON does, with `{}`. Documented on
  `ProjectedPathValue`.
- No machine-executor or engine-stdio test with a dotted field; both call the
  same `NarrowProjectedRecords`/`ValidateNarrowOptionsInMode` the CLI tests
  cover.

## Review History

Strata on gm01 remained stopped, so the review ran in a fresh-context Claude
subagent with no conversation history and read-only repository access.

- Round 1, approve with nits (below). Acted on after review: README and
  `agent-machine-workflow.md` no longer say `--fields` is top-level only; the
  aliasing test now inspects stored state in an internal test (mutation-checked:
  removing the copy in `Select` fails it); `OmittedFields` de-duplicates
  overlapping requests; `ProjectedPathValue` documents the null-leaf and
  position behavior; a `get` key/value test covers a dotted field.

# Adversarial Review

Fresh-context reviewer: Claude subagent (no conversation history, read-only repository access), 2026-10-16 (round 1)

## Blocking Findings

None. The reviewer probed share and paranoid mode with `groups.name`,
`groups.id` and overlapping selections: no omitted key appeared and
`VerifyProjectedRecords` passed in every case. `Select` only removes keys from
already-projected fields, `EffectiveFields` drops any path a mode does not
render along its length, and the CLI re-verifies executor output.

## Non-Blocking Risks

1. minor: README.md still said `--fields` takes top-level names only. Fixed.
2. minor: the public-API aliasing check could not detect aliasing, since
   `Fields()` copies. Replaced with an internal test on stored state.
3. nit: overlapping requests repeated names in the stderr notice. Fixed.
4. nit: table cells for list paths skip elements lacking the key and flatten
   nested lists; JSON keeps positions. Documented.
5. nit: `agent-machine-workflow.md` implied nested fields cannot be selected.
   Fixed.
6. minor: no test for `get`/`show` text with a dotted field, the machine and
   engine paths, or the end-to-end stderr notice for a path. `get` covered;
   the rest share tested functions.

## Verdict

Verdict: approve with nits
