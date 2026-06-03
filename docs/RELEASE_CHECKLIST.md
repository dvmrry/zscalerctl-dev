# Release Checklist

This checklist is for the first public release and any release that changes the
credentialed SDK boundary. A gate is complete only when its enforcement column is
satisfied for the exact release commit.

## Gate Map

| Gate | Enforcement |
| --- | --- |
| Project license chosen and committed. | Manual project decision. Must have a committed `LICENSE` file before public release. |
| Git author identity and public attribution are intentional before first public push. | Manual project decision. `LICENSE` uses legal attribution `David Murray <github@mrry.io>`; git/signing identity may use common-name attribution `Dave Murray <github@mrry.io>` intentionally. |
| Security reporting policy is committed. | Must have a committed `SECURITY.md` before public release. |
| Versioning policy is committed and semver labels are enforced. | `docs/VERSIONING.md`, `.github/workflows/semver-label.yml`, `.github/workflows/release.yml`, and local `bash scripts/test-verify-semver-label.sh` plus `bash scripts/test-next-version.sh`. |
| Release artifacts are verifiable. | `.github/workflows/release.yml` publishes release archives, per-target CycloneDX SBOMs, `SHA256SUMS`, and GitHub build provenance attestations over the checksum subject list; CI and local `make check` run `bash scripts/verify-release-artifacts.sh` plus `bash scripts/test-verify-release-artifacts.sh`. |
| Vendored dependency tree is current. | CI and local `make release-check`: `go mod tidy`, `go mod vendor`, then `git diff --exit-code -- go.mod go.sum vendor`. |
| Dependency policy checks pass. | CI and local `make check`, including tests, race tests, vet, staticcheck, govulncheck, docs scan, Semgrep invariant checks, SDK-boundary scripts, workflow credential scan, GitHub Actions pinning scan, and live-smoke script self-test. |
| GitHub Actions remain SHA-pinned and Renovate-managed. | CI and local `bash scripts/verify-actions-pinned.sh` plus `bash scripts/test-verify-actions-pinned.sh`; `renovate.json` extends `helpers:pinGitHubActionDigests`. |
| No real, plausible, or generated secrets in examples/docs. | CI SHA-pinned `gitleaks/gitleaks-action`, plus `bash scripts/verify-docs.sh` for docs/examples patterns. Manual review still required for new examples. |
| OpenSSF Scorecard runs against the repository posture. | `.github/workflows/scorecard.yml` runs on `main` pushes and weekly schedule, with all external actions SHA-pinned. |
| Advisory static and dependency scanners are enabled without becoming PR gates. | `.github/workflows/codeql.yml`, `.github/workflows/osv.yml`, and `.github/workflows/gosec.yml` run on `main`, schedule, or manual dispatch as applicable. Findings require triage before promotion to blocking gates. |
| Rendered and dumped output is limited to allow-listed fields, with secret-shaped values, bare high-entropy rendered-string tokens outside standard-mode structured display-name fields, and unmodeled nested values dropped or redacted. | `go test -mod=vendor ./internal/cli ./internal/zscaler ./internal/resources` tests `TestResourceListProjectsAndRedactsFixture`, `TestResourceListRuleLabelsUsesCatalogProjection`, `TestDumpWritesRestrictedFilesAndReportsWithoutCanaries`, `TestReaderListRuleLabelsProjectsSDKShapeThroughAllowList`, and `TestCatalogRenderedFieldsRedactSecretShapes`. |
| Free-text fields are standard-only catalog exceptions with explicit justification and scanner-backed canary coverage. | `go test -mod=vendor ./internal/resources` tests `TestCatalogIsValidAndReadOnly`, `TestCatalogFreeTextFieldsRedactBareHighEntropyCanary`, `TestResourceSpecValidationRequiresStandardFreeTextReason`, and `TestResourceSpecValidationRejectsFreeTextOutsideStandard`. |
| Mapped SDK response fields have been reviewed against the catalog or explicitly ignored with a reason. | `go test -mod=vendor ./internal/zscaler` test `TestReviewedSDKShapesMatchCatalogOrIgnoredRegistry`. |
| Redaction and projection invariants survive fuzz/property seed inputs. | `go test -mod=vendor ./...` runs `FuzzRedactorPreservesValidJSON`, `FuzzScanRenderedStringRedactsBareHighEntropyCanary`, and `FuzzProjectRecordSubsetAndCanaryRedaction` seed corpora. `.github/workflows/fuzz.yml` restores/saves each target's Go fuzz corpus with `actions/cache`, and local `make fuzz-smoke` runs a short advisory fuzz check. |
| `config show`, `doctor`, and `auth status` do not reveal credential values. | `go test -mod=vendor ./internal/cli` tests `TestConfigShowDoesNotExposeEnvironmentSecrets`, `TestDoctorDoesNotExposeEnvironmentSecrets`, and `TestAuthStatusDoesNotExposeEnvironmentSecrets`. |
| `completion bash|zsh|fish` and `help` do not read credential files, construct readers, or contact Zscaler. | `go test -mod=vendor ./internal/cli` tests `TestCompletionScriptsDoNotReadCredentialFilesOrUseReader` and `TestHelpDoesNotReadCredentialFile`. |
| GitHub Actions workflows and local composite actions do not reference live Zscaler credential variables, OneAPI/SDK env families (`ZSCALER_*`, `ONEAPI_*`), legacy product env families (`ZIA_*`, `ZPA_*`, etc.), explicit legacy `ZSCALERCTL_ZIA_*` variables, or Zscaler-named repository secrets. | CI and local `bash scripts/verify-ci-no-live-creds.sh` plus `bash scripts/test-verify-ci-no-live-creds.sh`. |
| Dump writer refuses unsafe overwrites before writing files. | `go test -mod=vendor ./internal/cli` test `TestDumpRefusesOverwriteBeforeWritingNewFiles`. |
| Partial dumps cannot look complete and contain only value-free error metadata; credential/session failures stay fatal. | `go test -mod=vendor ./internal/cli` tests `TestDumpAbortsWithoutWritingOnResourceErrorByDefault`, `TestDumpContinueOnErrorWritesPartialManifestAndValueFreeErrors`, and `TestDumpContinueOnErrorTreatsSessionFailureAsFatal`. |
| Live reads fail closed when required `ZSCALERCTL_*` credentials are missing. | `go test -mod=vendor ./internal/cli` test `TestResourceListDefaultReaderRequiresExplicitCredentials` enumerates current ZIA list resources from the catalog. |
| SDK boundary does not drift into SDK env/file/log/cache discovery, and proxy use remains explicit opt-in. | CI and local `bash scripts/verify-sdk-boundary.sh` plus `bash scripts/test-verify-sdk-boundary.sh`; `go test -mod=vendor ./internal/zscaler` tests default direct transport and explicit proxy opt-in. |
| `secret.Secret.Reveal()` remains confined to the SDK configuration boundary. | CI and local `bash scripts/verify-semgrep.sh` with passing and failing fixtures under `semgrep/tests`. |
| Resource docs match the catalog. | `go test -mod=vendor ./internal/resources` test `TestResourceReferenceListsCatalogResources`. |
| Catalog names remain safe for shell completion and dump path generation. | `go test -mod=vendor ./internal/resources` test `TestCatalogIsValidAndReadOnly`; `ResourceSpec.Validate` rejects product, resource, and operation names outside `[a-z0-9-]`. |
| Live tenant behavior matches fixture assumptions. | Manual release blocker with read-only credentials, described below. |

## Required Local Release Commands

```sh
make release-check
gitleaks dir .
gitleaks git .
```

`make release-check` runs the vendoring drift check and every local check that
does not require a separately installed secret scanner. `gitleaks dir .` scans
the working tree, and `gitleaks git .` scans committed history. Both are
mandatory before release, and gitleaks is also enforced in CI.

`govulncheck` must report no reachable vulnerabilities. Non-reachable findings
in required modules require a written review note before release.

Release tags are created by `.github/workflows/release.yml` after merge to
`main` when the merged pull request has `semver:patch`, `semver:minor`, or
`semver:major`. `semver:none` intentionally skips release creation. Release
assets include the platform archives, per-target CycloneDX SBOMs,
`SHA256SUMS`, and GitHub provenance attestations for the subjects listed in
`SHA256SUMS`.

## Required Live Smoke

Before public release, run a live smoke against a non-sensitive tenant or
profile using read-only OneAPI credentials or explicit ZIA legacy credentials:

```sh
make live-smoke
```

By default this validates every current ZIA read/list resource in the current
source checkout with `go run -mod=vendor ./cmd/zscalerctl` and writes artifacts
to a secure temporary directory printed in the final `[PASS]` or `[FAIL]`
marker. For release artifact validation, pass the unpacked candidate binary so
the smoke runs against what will ship:

```sh
make live-smoke LIVE_SMOKE_BIN=./bin/zscalerctl
```

For focused retry after a single-resource failure, limit the selected resources:

```sh
make live-smoke LIVE_SMOKE_RESOURCES=zia/locations,zia/rule-labels
```

`./scratch-live-smoke` is gitignored because live smoke artifacts remain
confidential operational data even after projection and redaction. Use
`LIVE_SMOKE_OUT=./scratch-live-smoke` only when you want a predictable artifact
path; remove the directory or choose a new one before reusing it.

The script prints explicit `[PASS]`, `[FAIL]`, and `[INFO]` markers, captures
list and dump artifacts under the output directory, validates JSON shape,
checks dump file permissions and manifest counts, compares list and dump record
counts, summarizes dropped field names and redaction marker paths without
printing record values, and fails if resource JSON contains known secret or
unmodeled sensitive field keys. Without configured credentials it prints
`[SKIP]`; release gating uses `--require-credentials` so missing credentials
cannot be mistaken for a completed live smoke.

After the script passes, inspect the captured output for unexpected empty
projections, over-redaction, unknown fields, and any secret-shaped or
high-entropy rendered-string value that should have been dropped or redacted. Where
the API exposes total counts, compare returned records against the total so a
successful but incomplete page cannot look like a complete dump. Handle live
smoke artifacts according to the operator's approved records and evidence
retention policy; do not commit or share the artifact directory.

This smoke is intentionally manual and blocking because fixtures cannot prove
that real SDK response keys and pagination behavior match the catalog for a
specific tenant.
