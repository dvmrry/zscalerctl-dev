# Builder Handoff

## Intent

feat(zia): add `zia admin-audit-logs status`

Reports the state of the tenant's shared administrator audit-log report
through one read-only status operation (HTTP retries follow the shared SDK
policy). The JSON document always includes status; progress_items_complete
appears only when the upstream count is present and non-negative, and an
explicit zero stays 0. A null or empty body or a missing status is an
upstream failure (exit 5); a context that ends during the read is reported
as canceled or deadline_exceeded, never as success or not_found. NDJSON is
rejected. Goldens, machine contract, man page and surface-change manifest are
updated.


## Base / Head

- Base: `main` at `50135e4`.
- Head reviewed: `193531c` on `pr/admin-audit-logs-status` (one squashed commit). This artifact
  is added to that commit afterwards and changes no reviewed code.

## Files Changed

- `cmd/zscalerctl/golden_surface_test.go`
- `cmd/zscalerctl/testdata/surface/introspect-pretty.stdout.golden`
- `cmd/zscalerctl/testdata/surface/introspect.stdout.golden`
- `cmd/zscalerctl/testdata/surface/inventory.golden`
- `cmd/zscalerctl/testdata/surface/surface_changes.md`
- `cmd/zscalerctl/testdata/surface/zia-admin-audit-logs-status-explicit-zero-json.stderr.golden`
- `cmd/zscalerctl/testdata/surface/zia-admin-audit-logs-status-explicit-zero-json.stdout.golden`
- `cmd/zscalerctl/testdata/surface/zia-admin-audit-logs-status-progress-unavailable-json.stderr.golden`
- `cmd/zscalerctl/testdata/surface/zia-admin-audit-logs-status-progress-unavailable-json.stdout.golden`
- `cmd/zscalerctl/testdata/surface/zia-help.stdout.golden`
- `docs/cli/machine-contract.md`
- `docs/cli/zscalerctl.md`
- `internal/cli/admin_audit_logs.go`
- `internal/cli/admin_audit_logs_test.go`
- `internal/cli/commands_product.go`
- `internal/cli/completion_internal_test.go`
- `internal/cli/introspect_test.go`
- `internal/cli/man_test.go`
- `internal/cli/ndjson_policy_test.go`
- `internal/cli/render_help.go`
- `internal/runtime/admin_audit_logs.go`
- `internal/zscaler/admin_audit_logs.go`
- `internal/zscaler/admin_audit_logs_test.go`
- `man/zscalerctl.1`

## Tests Run

- `make check` (gofmt, vet, `make test` and `make race` including the
  zscalerctl_engine_testhooks runs, docs, surface manifest, machine contract and
  the other repository gates); `make vuln`: no reachable vulnerabilities.
- gitleaks with `.gitleaks.toml` over `main..HEAD`: no leaks.

## Review History

The change was built on per-area branches, each reviewed by fresh-context
reviewers (GPT-6.1 Sol, then GPT-6 Astra) until approved, and then reviewed
again as this single squashed commit against main. The review below is that
final review.

# Adversarial Review

Fresh-context reviewer: gpt-6-astra (Codex CLI), read-only, 2026-10-08

## Blocking Findings

None identified under the supplied severity bar.

Reviewed `main` (`50135e4`) through `193531c`, using the three process documents from `main`. All change evidence came from Git objects, not the current checkout.

## Non-Blocking Risks

- **Upstream contract evidence:** `internal/zscaler/admin_audit_logs.go:33` defines a custom wire response; lines 107–124 assume particular status strings and millisecond timestamps. The synthetic tests confirm implementation consistency, but no independent endpoint specification or provenance-backed response fixture was found. Add that evidence to the handoff and tests. This is a verification gap, not a demonstrated defect.
- **Incomplete contract documentation:** `docs/cli/machine-contract.md:124` documents progress presence and invalid responses, but omits the normalized status vocabulary, completion-time representation, and the fact that every nonblank upstream error code becomes `"unknown"`. Document these rules so consumers need not infer them from implementation.

Go builds, tests, race checks, and generator checks could not be executed. The builder’s reported results remain unverified.

## Machine Contract Review

- The new diagnostic intentionally returns one JSON document and rejects NDJSON before reading configuration or contacting the API.
- Source preserves explicit zero progress and omits missing, null, or negative progress. Missing/null/blank status and empty responses become live-access failures.
- Runtime checks give cancellation precedence over returned status or errors. Deadline errors map to exit 5; cancellation maps to exit 1; HTTP 404 maps to `not_found`/4.
- Global gates reject fields, filtering, pagination, and `--from-dump` for this command before its read. Cobra argument errors are converted to usage errors by `execCobra`.
- Catalog, resource schemas, machine manifest, dump, and diff behavior remain unchanged. The CLI error schema permits the new `"status"` operation label.

## Safety Review

- The adapter issues a GET through the existing SDK read path. No report creation, download, or cancellation path is introduced.
- The response excludes free-form `errorMessage`. Status and error code use closed values; counts and timestamps receive validation, including a second validation at rendering.
- Existing projection, redaction, field coverage, and sensitive-data classifications are unchanged. Narrowing cannot widen this output.
- Existing dump/diff sanitization and confidentiality expectations remain unchanged.

## Generated Artifact Review

- Static JSON comparison confirmed exactly two added introspection entries: command count **293 → 295**, with no removed or modified existing entries and no changes to other top-level introspection data.
- Effect counts match the updated assertions. The two new JSON goldens distinguish omitted progress from explicit zero.
- CLI reference, help, inventory, man page, and surface-change notes correspond to the command addition. No unexplained catalog or skill-copy changes appeared.
- `git diff --check main...193531c` passed. Regeneration against the executable was not independently verified.

## Verdict

Verdict: approve with nits