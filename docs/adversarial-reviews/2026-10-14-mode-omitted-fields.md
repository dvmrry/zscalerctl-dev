# Builder Handoff

## Intent

fix: make redaction-mode omissions explicit instead of silent

Three fresh LLM agents drove the CLI against a live non-production tenant
(2026-10-14). In the default share mode, `--filter groups.id=X zia
url-filtering-rules list` returned `[]` with exit 0, a false "no rule
references this group", and `--fields id,name,groups` silently returned only
`id`. The docs asked callers to preflight fields against the mode; every agent
still fell into the trap.

The catalog now answers whether a field or dotted path renders in a mode and
where it does (`FieldPathAvailability`; a nested path renders only where every
parent renders). The executor's pre-read validation uses the active mode: a
`--filter` (or machine `filters[].field`) on a field the mode omits, or on a
secret field, is a usage error (exit 2) before any read, naming the mode that
renders it and never echoing the filter value. A successful CLI read in a mode
that omits fields writes one stderr notice naming them, including nested
paths, through the buffered notice channel, so a failure still produces a
single error envelope. Standard mode omits only secrets in the shipped
catalog (pinned by a test), so it prints no mode-omission notice. Empty lists
are still omitted rather than emitted as `[]`, which would have turned every
existing dump diff into drift.

## Base / Head

- Base: `integration/2026-10-09` at `badd8b4`.
- Reviewed heads: `9015b28` (round 1), `eedd578` (round 2).
- Final head: the wording commit after `eedd578` ("prints no mode-omission
  notice") plus this artifact; neither changes reviewed code.

## Files Changed

- `.agents/skills/zscalerctl/SKILL.md`
- `AGENTS.md`
- `README.md`
- `docs/cli/agent-machine-workflow.md`
- `internal/cli/app.go`
- `internal/cli/app_test.go`
- `internal/cli/filter_search_test.go`
- `internal/cli/omitted_notice.go`
- `internal/cli/omitted_notice_test.go`
- `internal/cli/snapshot_test.go`
- `internal/machine/executor.go`
- `internal/machine/executor_test.go`
- `internal/resources/resources.go`
- `internal/resources/resources_test.go`
- `skills/zscalerctl/SKILL.md`

## Tests Run

- `go test ./...` (Go 1.26.9; the macOS ACL tests need `/bin/chmod` ahead of
  the Nix coreutils one in `PATH`), `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`: pass.
- Live binary from this branch against the non-production tenant: share-mode
  `--filter groups.id=1 zia url-filtering-rules list` exits 2 with a usage
  envelope naming standard; share `--fields id,name,groups zia users list`
  prints `requested zia/users fields omitted in share mode: name, groups`;
  a share-mode default list prints one notice naming seven omitted fields; a
  standard-mode list has empty stderr.

## Review History

Reviewed with no repository access by a fresh-context local model given the
diff, surrounding source and call sites, and this repository's reviewer
prompt and template: Strata serving Qwen3.8-Flash-Next (IQ3_S) on gm01. The
reviewer could not run commands.

- Round 1, request changes. Blocking: the docs claimed a key missing from a
  record is empty or null unless the notice names it, which overclaims
  (secret fields are absent and unnamed unless requested; projection also
  drops values outside the catalog's projection). Fixed by stating the three
  cases precisely. Also fixed: "standard mode is quiet" is now an invariant
  pinned against the shipped catalog; a non-secret field with no render modes
  is no longer called secret; the executor test checks the response message.
  Refuted: other callers of `NarrowProjectedRecords` (the executor is the only
  non-test caller; the CLI's catalog-only check runs before config). Kept: the
  `--filter:` prefix in machine messages matches the existing unknown-filter
  message.
- Round 2, approve with nits (below). Applied: "prints no notice" now says "no
  mode-omission notice", since a requested secret field is still named.

Accepted and not changed: the stdio engine emits no notice (a protocol
change; machine clients read per-field `allowed_modes` from `schema list`);
secret fields never rendering rests on `AllowedIn` returning false for
`ClassSecret`, covered by existing redaction tests.

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-14 (round 2)

## Blocking Findings

None.

The round-1 blocking docs overclaim appears resolved in the delta:

- `AGENTS.md`, `skills/zscalerctl/SKILL.md`, and `docs/cli/agent-machine-workflow.md` no longer say that a key missing from a record is empty/null unless the omitted-field notice names it.
- The docs now separate three cases:
  - mode-omitted non-secret fields are named on stderr for CLI reads;
  - secret fields never render and are named only when requested;
  - other missing keys mean no renderable value for that read, including values outside projection, and must not be interpreted as `false` or unconfigured.
- The “standard mode quiet” claim is now backed by `TestShippedCatalogRendersEveryNonSecretFieldInStandardMode` in `internal/resources/resources_test.go`, which asserts that no shipped-catalog resource has non-secret fields omitted in standard mode.
- The stdio/machine gap is documented: the stdio engine does not emit the CLI stderr notice, and machine clients are told to use catalog metadata instead.
- The `SuppressedFieldError` message no longer labels a non-secret never-rendered field as secret.
- The executor test now asserts the machine response message does not echo the filter value.

I did not find a concrete, source-verifiable blocking defect in the delta.

## Non-Blocking Risks

1. **`standard mode ... prints no notice` can be read as “no stderr notice at all.”**

   - **file/path:** `AGENTS.md`, `skills/zscalerctl/SKILL.md`, `docs/cli/agent-machine-workflow.md`
   - **concrete issue:** The docs say standard mode omits only secrets and prints no notice. In `internal/cli/omitted_notice.go`, `noteOmittedFields` prints a secret notice whenever requested secret fields are present:

     ```go
     if len(secret) > 0 {
         fmt.Fprintf(&a.notices, "zscalerctl: requested %s/%s fields are secret and never render: %s\n",
             spec.Product, spec.Name, strings.Join(secret, ", "))
     }
     ```

     So a standard-mode CLI read can print stderr if the caller requested a secret field. The docs are probably trying to say “standard mode prints no non-secret mode-omission notice,” but the wording can be misread.
   - **why it matters:** Agent/machine docs are operational. If an agent assumes standard mode never writes stderr, it may ignore a useful notice about requested secret fields.
   - **how to reproduce or verify:** Run a standard-mode CLI read with `--fields` naming a catalog secret field, or inspect `noteOmittedFields`. The stderr line should contain `requested ... fields are secret and never render`.
   - **test that would catch it:** A CLI/App test or golden for standard mode with a requested secret field, asserting stderr contains the secret notice.

2. **The standard-mode invariant test pins the non-secret half, not the full “secrets never render” classification.**

   - **file/path:** `internal/resources/resources_test.go`
   - **concrete issue:** `TestShippedCatalogRendersEveryNonSecretFieldInStandardMode` checks that `OmittedFields(spec, redact.ModeStandard, nil)` returns no omitted non-secret fields. That pins the “standard mode prints no non-secret omitted-field notice” invariant. It does not directly prove that fields classified as secret are never rendered in standard mode.
   - **why it matters:** The docs say “Secret fields never render in any mode.” That is a stronger safety claim than the new test alone proves.
   - **how to reproduce or verify:** Inspect existing redaction/resources tests for secret-field classification. If no test asserts that secret catalog fields are unavailable in every mode, the docs depend on another invariant.
   - **test that would catch it:** A resources test over `resources.Catalog()` asserting that every field marked secret has no rendered availability in standard, share, or paranoid modes, or that `OmittedFields`/projection behavior classifies it as secret and unavailable.

3. **Generated `.agents` copy is excluded from the displayed diff.**

   - **file/path:** generated agent-skill copy under `.agents`
   - **concrete issue:** The supplied diff excludes the generated `.agents` copy. The verify output says `verify-docs: ok` and `verify-cli-docs: ok`, which is good evidence, but the reviewer cannot directly inspect the generated-copy delta.
   - **why it matters:** Generated agent-skill copies can drift from `skills/zscalerctl/SKILL.md` and become the actual runtime docs for agents.
   - **how to reproduce or verify:** Run the repo’s generated-artifact verifier with the generated copy included, or inspect the generated `.agents` skill file after regeneration.
   - **test that would catch it:** The existing `verify-docs` / generated-copy checker should fail if `.agents` is not synchronized with `skills/zscalerctl/SKILL.md`.

4. **The filter behavior change is a real machine-contract change and should remain visibly intentional.**

   - **file/path:** docs and `internal/resources/resources.go`
   - **concrete issue:** A filter on a known field omitted by the active mode, or on a secret field, now becomes a usage error rather than silently producing no matches. This is documented and `verify-machine-contract` passed, but it can break consumers that previously relied on empty results.
   - **why it matters:** Machine consumers may treat empty results as success. This change intentionally converts a false-negative read into a usage error.
   - **how to reproduce or verify:** Use a CLI or machine request with `--filter <known-omitted-field>=value` in a mode that omits that field and confirm exit `2` / usage error envelope.
   - **test that would catch it:** Machine-contract golden for omitted-field filters, asserting usage error kind/message and no loader call.

5. **The stdio replacement guidance points to `allowed_modes`, but field-level availability may be what machine clients actually need.**

   - **file/path:** `docs/cli/agent-machine-workflow.md`
   - **concrete issue:** The docs say stdio machine clients should read `allowed_modes` from the catalog because the stdio engine does not emit the CLI omitted-field notice. If `allowed_modes` is resource-level only, it may not identify which fields are renderable in a given mode.
   - **why it matters:** A machine client could know that a resource supports `standard` and `share`, then still omit fields in `share` without a notice.
   - **how to reproduce or verify:** Inspect introspection/schema output for field-level renderability or secret marking. If only resource-level `allowed_modes` is present, the docs understate the preflight requirement.
   - **test that would catch it:** A machine-contract test asserting that introspection/schema output contains enough field-level availability to preflight omitted fields, or docs wording that points to field-level catalog metadata rather than only `allowed_modes`.

6. **The executor test checks absence of the filter value, but not the full machine error shape.**

   - **file/path:** `internal/machine/executor_test.go`
   - **concrete issue:** The new assertion checks that `got.Error.Message` does not contain `host-canary-01`. It does not assert the expected error kind or that the message still names the omitted field and the mode that renders it.
   - **why it matters:** The response could omit the filter value but regress the useful diagnostic or use the wrong error kind.
   - **how to reproduce or verify:** Run the executor test and inspect the response envelope. It should have a usage/filter error kind and a message naming the field/mode without echoing the value.
   - **test that would catch it:** Extend the executor test to assert error kind and expected message substring, while still asserting the filter value is absent.

## Machine Contract Review

- **JSON/NDJSON stdout:** No evidence in the delta that JSON or NDJSON stdout shape changed. The CLI stderr notice is a new observable stderr behavior, not stdout.
- **Exit codes:** The documented behavior now says filters on fields omitted by the active mode, and filters on secret fields, are usage errors with exit `2`. This is a meaningful CLI/machine behavior change and appears intentional.
- **Error envelopes:** The machine executor test now asserts the response message does not echo the filter value. The response error shape is not fully asserted, but the no-echo requirement is covered.
- **Error message text:** `SuppressedFieldError.Error()` changed for non-secret never-rendered fields. It no longer says “secret”; it says “renders in no redaction mode.” This is a user-visible machine message change. `verify-machine-contract: ok` is evidence that existing contract tests tolerate it.
- **Stdio behavior:** The docs now explicitly state that the stdio engine does not emit the CLI stderr omitted-field notice. This is a documented machine-path distinction.
- **Manifest/surface verification:** The supplied verify output includes `verify-surface-changes-manifest: ok` and `verify-machine-contract: ok`.

## Safety Review

- **Redaction/projection:** No evidence that the delta widens output or reveals previously omitted data. It primarily changes docs, error wording, and tests.
- **Sensitive-data classification:** The `SuppressedFieldError` fix reduces overclassification: a non-secret field that renders in no mode is no longer called secret.
- **Filter safety:** Filtering on omitted or secret fields now fails before matching instead of silently producing no matches. This reduces false-negative inference and avoids treating unavailable fields as absent data.
- **Value leakage:** The executor test checks that the response message does not echo the filter value. The CLI stderr notice uses catalog field names, not tenant data, according to `internal/cli/omitted_notice.go`.
- **Dump/diff artifacts:** No dump/diff artifact behavior is changed in the supplied delta. The omitted-field notice has a dump-specific hint in existing code, but the delta does not alter that behavior.

## Generated Artifact Review

- The displayed diff intentionally excludes the generated `.agents` copy.
- The supplied verify output says `verify-docs: ok`, `verify-cli-docs: ok`, and `verify-surface-changes-manifest: ok`.
- I cannot directly inspect the generated `.agents` delta from the supplied evidence, so I treat that as a non-blocking evidence gap rather than a blocking finding.
- No suspicious generated-artifact drops, additions, ordering changes, or count changes are visible in the supplied docs diff.

## Verdict

Verdict: approve with nits
