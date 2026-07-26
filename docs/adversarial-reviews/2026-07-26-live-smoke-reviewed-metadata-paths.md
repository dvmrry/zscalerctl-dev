# Builder Handoff

## Intent

Correct false-positive release live-smoke failures for metadata already
deliberately projected by the ZIA readers and catalog: static-IP city
references, GRE destination VIP references, and SSL inspection rule
certificate-selection/reference metadata. Keep every exception scoped to an
exact resource and object path so certificate material and similarly named
fields elsewhere remain denied.

## Base / Head

Base commit: `1f686d6f2973065f72a86be68a7e83ce21fde2a2`

Reviewed implementation head:
`84d728249763f9e4aa034aefd1f7ce88ed998633`

Branch: `feature/release-node-toolchain`

PR: `dvmrry/zscalerctl-dev#125`

## Files Changed

- `internal/livesmoke/checks.go`
- `internal/livesmoke/checks_test.go`

## Source Inputs Consulted

- `internal/resources/catalog_zia.go`
- `internal/zscaler/reader.go`
- `docs/adversarial-reviews/2026-07-20-ssl-inspection-action-details.md`
- A credentialed downstream smoke report naming the rejected keys.
- The adversarial-review process and templates from `origin/main`.

## Generated Artifacts

None changed. No regeneration command applies.

## Expected Delta

- Runtime resource output, readers, catalog fields, machine contracts, and
  generated surfaces: unchanged.
- Live-smoke denied-key exceptions: exact reviewed paths added for
  `zia/static-ips`, `zia/gre-tunnels`, and `zia/ssl-inspection-rules`.
- Regression coverage: positive live-shape fixtures plus wrong-path and
  certificate-material rejection fixtures for every added path family.
- Entitlement handling and dump behavior: unchanged.

## Invariants Claimed

- Certificate bodies, private keys, fingerprints, and unreviewed certificate
  fields remain outside the projection surface and are not exempted by the
  smoke validator.
- SSL certificate metadata remains limited to the already-reviewed rule action
  paths and existing projected fields.
- An allowed key at one resource/path remains denied at every other resource or
  path.
- Array traversal omits indices but preserves the complete object path, so an
  allowance applies consistently to array elements without matching siblings
  or differently nested fields.
- JSON/NDJSON shapes, schemas, manifests, redaction modes, exit codes, and dump
  behavior are unchanged.
- `cloud-app-instances` entitlement failures remain failures.

## Tests Run

- `gofmt -d internal/livesmoke/checks.go internal/livesmoke/checks_test.go`
- `go vet ./internal/livesmoke`
- `go test ./internal/livesmoke ./internal/resources ./internal/zscaler -count=1`
- `go test -race ./internal/livesmoke -count=1`
- `go test ./internal/livesmoke -shuffle=on -count=20`
- `go test ./internal/livesmoke -run TestFindDeniedKeys -count=20 -shuffle=on`
- `git diff --check`
- Every `make check` and `make release-check` prerequisite except
  `verify-adversarial-review`; that gate intentionally awaited this artifact.

All listed commands passed at the reviewed implementation head or its
code-identical predecessor before the three test-only review fixes.

## Known Deferrals

- An operator must rerun credentialed smoke against this updated branch.
- `zia/cloud-app-instances` is unavailable in the downstream test tenant due
  to entitlement and is not changed or suppressed here.
- The reported full-dump failure cascaded from resource failures. This change
  resolves the three validator false positives but does not recast entitlement
  failures as success.

## Review Focus

- Independently derive each JSON path from reader and catalog source.
- Attack cross-resource, sibling, nested, and array traversal behavior.
- Try certificate body, key, fingerprint, and wrong-path fields against the SSL
  allowances.
- Confirm the delta changes no runtime output or entitlement behavior.

# Adversarial Review

Fresh-context reviewer: Chandrasekhar
(`019fa05f-7b62-7d50-ab15-85430d6dfde5`)

Reviewed base: `1f686d6f2973065f72a86be68a7e83ce21fde2a2`

Initially reviewed head:
`a46bc528723a93286b76cee5dfcd61d98465b2a4`

Final reviewed implementation head:
`84d728249763f9e4aa034aefd1f7ce88ed998633`

Two earlier reviewer agents were stopped without a verdict; their incomplete
work was not used as approval evidence.

## Evidence Matrix

| Allowance | Reader path | Catalog path | Positive fixture | Negative fixture |
|---|---|---|---|---|
| Static-IP city | `city` | `city` | Present | `metadata.city` denied |
| GRE primary destination | `primaryDestVip` and `primaryDestVip.city` | Same | Present | Wrong parent denied |
| GRE secondary destination | `secondaryDestVip` and `secondaryDestVip.city` | Same | Present | Wrong parent denied |
| SSL selected/default certificate reference | `action.overrideDefaultCertificate`, `action.sslInterceptionCert`, and its `defaultCertificate` member | Same | Present | Wrong parent and injected certificate material denied |
| SSL decrypt/do-not-decrypt certificate policy | Both `action.*SubActions.serverCertificates` paths | Same | Present | Each wrong nesting denied independently |

The reviewer confirmed that allowances are selected per resource and compared
to the complete dot-joined object path. Array indices are intentionally omitted
without dropping object parents, so array traversal does not create a sibling,
nested, or cross-resource escape.

The focused `TestFindDeniedKeys` run passed.

## Initial Finding Resolution

The initial review found no code defect and returned approval with two test
nits: direct wrong-path coverage was absent for `secondaryDestVip` and for the
two `serverCertificates` paths.

Finding: symmetric allowed paths lacked independent negative fixtures.

Root cause: the first regression set exercised the primary GRE sibling and SSL
certificate-reference/material boundaries, but not every symmetric path.

Fix: added one wrong-path fixture for `secondaryDestVip` and one for each
decrypt/do-not-decrypt `serverCertificates` path.

Regression test: `TestFindDeniedKeys` now requires each misplaced key to remain
denied.

Verification: the focused test passed 20 shuffled runs.

The same reviewer rechecked only this test-only delta, confirmed all three
missing cases closed, found no regression, and approved exact head
`84d728249763f9e4aa034aefd1f7ce88ed998633`.

## Blocking Findings

None.

## Non-Blocking Risks

None remaining from the scoped review.

## Machine Contract Review

No runtime projection, JSON/NDJSON contract, error envelope, exit code, dump,
diff, schema, manifest, or introspection code changed.

## Safety Review

The live-smoke validator now recognizes only already-reviewed metadata at exact
resource paths. Broad secret-shaped matching remains active elsewhere, and an
injected `certificate` field remains denied. No redaction or field projection
was widened.

## Generated Artifact Review

No generated artifacts changed.

## Verdict

Verdict: approve
