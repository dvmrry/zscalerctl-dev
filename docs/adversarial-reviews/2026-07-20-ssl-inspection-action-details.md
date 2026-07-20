# Builder Handoff

Change: promote ZIA SSL inspection rule action details through the projection
catalog.

Files changed:

- `internal/resources/catalog_zia.go`
- `internal/zscaler/reader_test.go`
- `internal/zscaler/reader_fields_ssl_inspection_rules_test.go`
- `internal/zscaler/schema_zia_test.go`

Intent:

- Expose `zia/ssl-inspection-rules.action` details that are needed by machine
  consumers and TUI exploration:
  - `showEUN`
  - `showEUNATP`
  - `overrideDefaultCertificate`
  - `sslInterceptionCert.id`
  - `sslInterceptionCert.name`
  - `sslInterceptionCert.defaultCertificate`
  - `decryptSubActions.*`
  - `doNotDecryptSubActions.*`
- Keep the newly promoted details in `standard` and `share` redaction modes.
- Keep `paranoid` limited to the pre-existing `action.type` field.
- Keep `lastModifiedBy` and other unlisted fields dropped. Certificate
  contents/key material remain outside the exposed SDK shape and are not
  rendered.

Generated artifacts:

- `make field-coverage` was run. It produced no committed field-coverage diff.
- `go test ./cmd/zscalerctl -run TestGoldenSurface -count=1` passed with no
  golden updates.

Builder verification:

- `gofmt -w internal/resources/catalog_zia.go internal/zscaler/reader_test.go internal/zscaler/reader_fields_ssl_inspection_rules_test.go internal/zscaler/schema_zia_test.go`
- `go test ./internal/resources ./internal/zscaler -run 'TestReaderListSSLInspectionRulesProjectsSDKShapeThroughAllowList|TestSSLInspectionRule(ActionDetailsProjectAcrossModes|PromotedFields|AccessControlModes)|TestReviewedSDKShapes' -count=1 -v`
- `make field-coverage`
- `go test ./internal/zscaler -run TestFieldCoverageReportIsCurrent -count=1`
- `go test ./cmd/zscalerctl -run TestGoldenSurface -count=1`
- `go test ./internal/resources ./internal/zscaler ./internal/browser ./internal/enginewire/... -count=1`
- `make verify-machine-contract docs-cli-check verify-surface-changes-manifest`
- `env -u GOFLAGS make check` progressed through all gates before failing at
  `verify-adversarial-review`, as expected before this artifact existed.

# Adversarial Review

Fresh-context reviewer: Galileo (`019f8086-7d1f-7611-b0de-8c89bc983c7d`)

Verdict: approve

Blocking findings: none.

Non-blocking risks: none.

Reviewer verification:

- Catalog promotion matches the requested redaction behavior:
  `action.type` remains available in all modes, while the new details are
  standard/share only.
- `sslInterceptionCert` exposes nested `id`, `name`, and
  `defaultCertificate` only; certificate contents/key material are not part of
  the exposed SDK shape.
- Recursive nested projection still applies the allow-list, so adding nested
  fields does not widen sibling or unknown fields.
- Tests assert standard/share exposure and paranoid drops for certificate
  reference metadata.
- Schema shape review still guards the SSL action nested SDK structs.
- `schema list` exposes the new nested fields/modes as expected; `introspect`
  remains top-level-only for catalog fields, matching existing behavior.

Reviewer commands:

- `gofmt -d ...`
- `git diff --check`
- `go test ./internal/zscaler ./internal/resources -run 'TestSSLInspectionRule|TestReaderListSSLInspectionRulesProjectsSDKShapeThroughAllowList|TestReviewedSDKShapesZIA|TestCatalog|TestProject'`
- `go test ./internal/machine -run 'Test.*Golden|TestManifest|TestEngineManifest|TestContract'`
- `go test ./internal/zscaler -run 'TestFieldCoverageReportIsCurrent|TestReviewedSDKShapesZIA'`
- `go test ./internal/cli -run 'TestCobraSchemaList|TestIntrospect|TestMachineManifest'`
- `go test ./internal/zscaler -run 'TestReviewedSDKShapes|TestResourceCatalog' -count=1 -v`

Notes:

- One attempted reviewer path, `./internal/cmd`, does not exist in this repo;
  the reviewer corrected it to `./internal/cli`.

## Delta Recheck

Fresh-context reviewer: Popper (`019f8096-9fbf-7623-b0bd-027013f1b9bb`)

Scope: after the initial approval, the certificate reference promotion was
widened from `sslInterceptionCert.name` to `sslInterceptionCert.id`,
`sslInterceptionCert.name`, and `sslInterceptionCert.defaultCertificate`.
The intent is to expose certificate reference metadata needed to evaluate rule
configuration while still excluding certificate contents and key material.

Delta verdict: approve with nits

Blocking findings: none.

Reviewer nit:

- Record the delta review verdict and commands in this artifact before treating
  it as complete.

Reviewer verification:

- `sslInterceptionCert` is tenant config, standard/share only, with `id`,
  `name`, and `defaultCertificate` as the only modeled children.
- Paranoid does not leak `sslInterceptionCert.id` because nested projection
  requires the parent field to be allowed first.
- Source mapping emits only certificate reference metadata.
- The SDK `sslinspection.SSLInterceptionCert` shape contains only `ID`, `Name`,
  and `DefaultCertificate`; no certificate contents or key material are present
  in this shape.
- Tests cover standard/share exposure and paranoid dropping.
- Schema-review notes cover the nested SDK fields.

Reviewer commands:

- `gofmt -d ...`
- `git diff --check`
- Focused `go test` for SSL inspection, catalog/schema review, CLI
  schema/introspect/goldens
- `go test ./internal/zscaler -run 'TestFieldCoverageReportIsCurrent|TestReviewedSDKShapes' -count=1`
- `make verify-adversarial-review`
