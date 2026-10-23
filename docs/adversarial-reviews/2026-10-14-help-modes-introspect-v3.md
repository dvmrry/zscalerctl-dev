# Builder Handoff

## Intent

fix(cli): list resources in product help; state allowed_modes on secrets
feat(introspect)!: drop the derived mutating flag (introspect v3)

Three items from the 2026-10-14 agent runs against a live non-production
tenant:

- The unknown-resource error tells callers to run `zscalerctl <product>
  --help` to enumerate resources, but resources are positional arguments,
  not Cobra subcommands, so product help listed only flags. Product help now
  lists every catalog resource with its read operations, ahead of the usage
  block, and points at `schema gaps <product>`. Subcommand help and resource
  help are unchanged.
- `schema list` omitted `allowed_modes` on secret fields (`omitempty` on a
  nil slice). Every field now states it; a secret field shows `[]`.
- introspect's derived `mutating` was true for every command (any
  `--output` write or credential-provider execution counted), so agents read
  every read as mutating. It is removed; `effects` is the per-command answer
  and `read_only` the tenant guarantee. zscalerctl is pre-1.0 with no
  external consumers, so there is no compatibility shim; per
  docs/VERSIONING.md the contract is `introspect_version` "3" with a new
  `introspect-v3.schema.json`, and v2 is unchanged and hash-pinned.

Considered and left alone: `version` printing "dev" is accurate for local
builds (release builds set the version by ldflags).

## Base / Head

- Base: `integration/2026-10-09` at `b29782e`.
- Reviewed head: `8b18a24` (rounds 1 and 2; no code change between them).
- Final head: this artifact on top of `8b18a24`.

## Files Changed

- `.agents/skills/zscalerctl/examples/offline-workflow.sh`
- `cmd/zscalerctl/testdata/surface/*` (zia-help, introspect,
  introspect-pretty goldens; `surface_changes.md`)
- `docs/cli/machine-contract.md`
- `docs/schema/introspect-v3.schema.json`
- `internal/cli/commands_product.go`
- `internal/cli/help.go`
- `internal/cli/introspect.go`
- `internal/cli/introspect_schema_test.go`
- `internal/cli/introspect_test.go`
- `internal/cli/product_help_test.go`
- `internal/resources/resources.go`
- `internal/resources/resources_test.go`
- `man/zscalerctl.1`
- `skills/zscalerctl/examples/offline-workflow.sh`

## Tests Run

- `go test ./...` (Go 1.26.9, `/bin/chmod` first in `PATH`), `gofmt -l`,
  `verify-docs`, `verify-cli-docs`, `verify-surface-changes-manifest`,
  `verify-machine-contract`: pass.
- Built binary: `zcc --help` and `zpa --help` list resources before the
  usage block; `zia locations --help` still prints the resource field help.

## Known Gaps

- No golden covers `schema list` output; the `allowed_modes` shape is pinned
  by `TestFieldSpecJSONAlwaysStatesAllowedModes`.

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, surrounding source, and this repository's reviewer prompt and template:
Strata serving Qwen3.8-Flash-Next (IQ3_S) on gm01. The reviewer could not run
commands.

- Attempt 0 (commit 1 only) exhausted its output budget without a review;
  its one concrete concern, that `surface_changes.md` did not record the
  `allowed_modes` change, was addressed in commit 2.
- Round 1, request changes. One blocker: the `c == cmd` guard might add the
  product resource list to `zia <resource> --help`. It cannot: a known
  resource returns resource help before the section is computed, and the
  unchanged `zia-locations-help` and `zia-locations-list-help` goldens pin
  that output. The prompt had omitted the full help func.
- Round 2, given that evidence, approve with nits (below).

Accepted and not changed: an extra test asserting resource help lacks the
product block (the unchanged goldens already pin it); a golden for
`zia --help locations` argument order.

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-14 (round 2)

## Blocking Findings

None.

The round-1 blocker is resolved by the provided source evidence:

- `commands_product.go` help func now checks first positional args before computing any product resources section:
  ```go
  positionals := c.Flags().Args()
  if len(positionals) >= 1 {
      if spec, ok := catalog.FindSpec(product, positionals[0]); ok {
          fmt.Fprintln(c.OutOrStdout(), helpRenderer.renderResourceUsage(product, spec, 0))
          return
      }
  }
  ```
- Because this returns before `section := ""` and before `if c == cmd { section = productResourcesHelp(...) }`, a known resource help invocation cannot receive the product `Resources` block from this path.
- The provided golden evidence supports the same behavior:
  - `zia-locations-help` and `zia-locations-list-help` exist under `cmd/zscalerctl/testdata/surface`.
  - `git diff --stat b29782e..8b18a24` for those goldens is empty.
  - `TestGoldenSurface` passes.
- The built binary output for `zia locations --help` starts with resource usage rather than the product help/resources section.

## Non-Blocking Risks

- `cmd/zscalerctl/testdata/surface/zia-locations-help`
  - The provided golden excerpt shows the beginning of resource help, but it does not by itself prove that the full golden lacks any product `Resources` section.
  - The source early-return closes the specific concern, but a stronger regression test would assert that resource help output does not contain the product resources section.
  - Suggested test:
    ```bash
    zscalerctl zia locations --help | grep -n '^Resources:'
    ```
    Expected: no match.

- `cmd/zscalerctl/testdata/surface/zia-locations-list-help`
  - The named golden exists and is unchanged, but the review input does not show the exact command invocation used to generate it.
  - If mode-specific help is intended for `zia locations list --help`, a golden or unit test should explicitly cover that invocation and assert it does not fall back to product help.
  - Suggested test:
    ```bash
    zscalerctl zia locations list --help | grep -n '^Resources:'
    ```
    Expected: no match.

- `commands_product.go`
  - The help func uses `c.Flags().Args()` rather than the `args` parameter passed to Cobra’s help func. This appears harmless in the shown path, but it is worth keeping a golden test for argument ordering and help-flag placement if CLI help behavior is considered a contract.
  - Suggested test:
    ```bash
    zscalerctl zia --help locations
    zscalerctl zia locations --help
    ```
    Expected: only the second prints resource usage; the first prints product help.

## Machine Contract Review

- Did JSON, NDJSON, error envelope, exit code, dump, diff, schema, manifest, or introspection behavior change?
  - The provided round-2 artifacts only show CLI help behavior and help goldens.
  - The broader `allowed_modes always present` and `introspect v3 without mutating` changes were not re-verified in this round because no code changed since round 1 and the round-1 non-blocking notes were accepted as-is.
  - No new blocking machine-contract regression is visible from the supplied evidence.

- Are contract deltas intentional and documented?
  - The help behavior delta is intentional: known-resource help returns before product resources help.
  - The introspect/`allowed_modes` deltas are outside the newly provided evidence, but they are not reopened here because there is no new code change and round 1 already accepted them as non-blocking.

- Are generated docs, schemas, and fixtures in sync with runtime behavior?
  - For the help surface, goldens are unchanged and `TestGoldenSurface` passes.
  - No suspicious generated-artifact drift is visible from the provided evidence.

## Safety Review

- Did redaction, projection, field coverage, or sensitive-data classification change?
  - No redaction or projection behavior is visible in the provided help func.
  - The shown diff surface is CLI help rendering, not data output.

- Can a narrowed field set widen output or reveal previously dropped data?
  - The shown change does not narrow or widen data fields.
  - Help output may expose resource usage and field names, but that is expected CLI help behavior and was already covered by existing goldens.

- Are dump and diff artifacts still sanitized but treated as confidential?
  - No dump/diff artifact behavior is shown in the supplied evidence.
  - No evidence of a new safety regression.

## Generated Artifact Review

- Were generated files intentionally regenerated?
  - The relevant help goldens are not regenerated by this branch.

- Are deltas explained?
  - The empty `git diff --stat` for the relevant goldens is a useful signal that the help output did not drift.

- Any suspicious drops, additions, ordering changes, or count changes?
  - None visible in the provided golden evidence.

## Verdict

Verdict: approve with nits
