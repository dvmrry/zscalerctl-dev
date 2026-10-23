# Builder Handoff

## Intent

feat(resources)!: secret means credential material only

`secret` had become a catch-all: 783 catalog fields never rendered, though only
a handful were credentials. Link ids, ports, settings, admin audit references,
risk-profile criteria, NSS filters, DLP content and ZCC profile targeting were
hidden, which blocked basic audit tasks (which connector groups are used,
which groups a ZCC profile targets, segment port exposure). The operator
decided: secrets must not block basic tasks; reference ids are fine to share;
ZPA/ZCC structural link ids should be readable; credentials must never render.

Model: `FieldSpec.SecretReason` is required on every secret field and forbidden
elsewhere, from a closed set (credential, token, key_material, opaque_blob,
unstructured), shown as `secret_reason` in `schema list`; `secretField` is
gone. Every former secret field was reclassified by content (ids, links and
settings in every mode; config names standard+share; people and tenant or
cloud identifiers standard-only; free text standard-only and scanned; nested
objects modeled from the SDK shape). SDK structs a reader hands over are
normalized through their JSON encoding at the source boundary instead of being
quarantined; the catalog still decides which keys render.

## Base / Head

- Base: `integration/2026-10-09` at `5c9a080`.
- Reviewed head: `5b1a525` (rounds 1 and 2; no code change between them).
- Final head: this artifact on top of `5b1a525`.

## Files Changed

- `internal/resources/resources.go` (model, helpers, JSON normalization)
- `internal/resources/catalog_{zia,zpa,zcc,ztw,zidentity}.go`
- `internal/resources/references_{zia,zia_rules,zpa,zcc,ztw}.go`
- tests in `internal/resources`, `internal/zscaler`, `internal/cli` and
  others whose fixtures declare secret fields
- `docs/FIELD_COVERAGE.md`, `docs/field-coverage.json` (regenerated)
- `AGENTS.md`, `skills/zscalerctl/SKILL.md` (+ `.agents` copy),
  `docs/cli/agent-machine-workflow.md`
- `cmd/zscalerctl/testdata/surface/zia-advanced-settings-show-pretty.stdout.golden`,
  `surface_changes.md`

## Tests Run

- `go test ./...` (Go 1.26.9, `/bin/chmod` first in `PATH`), `gofmt -l`,
  `go vet`, `verify-docs`, `verify-cli-docs`,
  `verify-surface-changes-manifest`, `verify-machine-contract`,
  `verify-typescript-client`: pass.
- Live, non-production tenant: `zpa server-groups --count-by
  appConnectorGroups.id` (39 linked connector groups), `zcc
  application-profiles --count-by groupAll,groups.id`, and
  `zpa application-segments --count-by segmentGroupId` in share mode now
  answer. A full standard-mode dump wrote all 165 resources with no
  projection or verification failure (3 list failures, all pre-existing).
  ZPA `modifiedBy` on all 29 application segments is digits only (an admin
  id), checked without printing values.

## Result

467 of 783 former secret fields render. Still secret: 14 credential, 4
token, 9 opaque_blob, 273 unstructured (almost all `extensions` maps). The
reclassification was applied by four workers from a fixed decision table,
reviewed by the builder (a before/after table of every former secret field,
plus a scan of every share-visible object's children), and ~75 tests were
updated by a worker under the rule that no assertion about a still-secret
field may be weakened.

## Known Gaps

- Admin reference shapes differ in a few places (`zia/risk-profiles`
  `modifiedBy` is standard-only at the parent; ZIA `managedBy` shapes vary);
  none leaks.
- A few children allow wider modes than their parent (harmless: the parent
  bounds them).
- `firewall-dns-rules` `dnsGateway.parent` is modeled as an object but the
  SDK sends a string; the reader drops it.
- Rule criteria (users, groups, departments, locations) remain standard-only
  at the parent; rendering their ids in share mode is a separate change.

## Review History

Reviewed with no repository access by a fresh-context local model given the
core diff, the exemplar and a table of every former secret field with its new
classification: Strata serving Qwen3.8-Flash-Next (IQ3_S) on gm01. The
reviewer could not run commands.

- Round 1, request changes: three blockers, each citing a parent row (admin
  names, NSS `vpnCredentials`, ZCC `appDataBlob` / ZTW `expressionJson` /
  `vpcInfo`). Refuted with the child rows: only ids, operators, protocols and
  flags render beyond standard; names, IPs, FQDNs, ports and tag values are
  standard-only. NSS `vpnCredentials` is a log filter of credential objects
  (ids and names), not key material; the location `vpnCredentials` with the
  PSK stays `credential`. ZPA `modifiedBy` values are numeric admin ids.
- Round 2, approve with nits (below).

# Adversarial Review

Fresh-context reviewer: Strata (Qwen3.8-Flash-Next IQ3_S, gm01), read-only, no command execution, 2026-10-15 (round 2)

## Blocking Findings

None.

The three round-1 blockers were based on parent rows. Given the supplied child
rows, each cited parent row is narrower than the parent row alone appears to
allow.

1. **Admin/person audit names**

   The cited object-reference parents render in `standard,share,paranoid`, but
   the name subfields are `sensitive_identifier` with modes `standard` only:

   - `zia/url-filtering-rules.lastModifiedBy.name`: `standard`
   - `zia/location-groups.lastModUser.name`: `standard`
   - `ztw/public-cloud-info.lastModUser.name`: `standard`
   - `ztw/public-cloud-info.lastModUser.externalId`: `standard`

   The only leaf reference row that could still be sensitive is:

   - `zpa/application-segments.modifiedBy`: `operational_metadata`,
     `standard,share,paranoid`

   The builder’s live-tenant observation says the values are admin ids, not
   names. That is not enough to prove the field is always an id, but it does
   not establish an active leak. This is treated as a non-blocking risk below,
   not a blocking finding.

2. **`zia/nss-feeds vpnCredentials`**

   The parent row is `tenant_configuration` in all modes, but its child rows
   restrict the sensitive parts:

   - `vpnCredentials.id`, `pid`, `getlId`, `deleted`: operational metadata,
     all modes.
   - `vpnCredentials.name`: `sensitive_identifier`, `standard` only.
   - `vpnCredentials.description`: `free_text`, `standard` only.

   This matches the builder’s description of an NSS log filter object listing
   VPN credential references, not the credential itself. The actual PSK-bearing
   location `vpnCredentials` remains classified as `secret:credential` per the
   handoff. No blocker holds from the child rows.

3. **ZCC `appDataBlob`, ZTW `expressionJson`, and ZTW `vpcInfo`**

   The child rows narrow the sensitive leaves:

   - ZCC `appDataBlob.fqdn`, `ipaddr`, `port`: `sensitive_identifier`,
     `standard` only.
   - ZCC `appDataBlob.proto`: operational metadata, all modes.

   - ZTW `expressionJson.expressionContainers.tagContainer.tags`:
     `standard` only.
   - ZTW `expressionJson...tags.key`, `tags.value`:
     `sensitive_identifier`, `standard` only.
   - ZTW `expressionJson...operator`, `tagType`: operational metadata,
     all modes.

   - ZTW `vpcInfo.cloudMeta.name`: `sensitive_identifier`, `standard` only.
   - ZTW `vpcInfo.cloudMeta.id`: operational metadata, all modes.
   - ZTW `vpcInfo.cloudProvider`: `standard,share`.

   These child rows contradict the round-1 claim that customer IPs, FQDNs,
   ports, tag expressions, or cloud resource names render in `share` or
   `paranoid`.

## Non-Blocking Risks

- **`zpa/application-segments.modifiedBy` is a leaf classified as
  operational metadata.**

  This is the weakest cited row. It renders in `standard,share,paranoid`. If
  the value is an admin id, this is fine under the operator requirements. If a
  tenant or future API version returns a display name, email, or other person
  identifier, it would leak in `share`/`paranoid`.

  How to verify:

  - Add a fixture where `modifiedBy` is a human-readable value such as
    `"Alice Admin"` or `"alice@example.com"`.
  - Run share/paranoid projection/goldens.
  - The value must be suppressed, reclassified, or rejected unless it matches a
    known id pattern.

  Suggested test:

  - Assert `modifiedBy` either matches a numeric/id pattern or is omitted from
    `share`/`paranoid`.

- **Parent rows are broad; child rows are narrow.**

  The safety of this table depends on the renderer honoring leaf child rows.
  If projection stops at the parent row and emits the whole object, the
  round-1 blockers would return.

  How to verify:

  - Add goldens for each parent object in `share` and `paranoid`.
  - Assert only the child rows with `share`/`paranoid` modes appear.
  - Add an unknown-child test to confirm unknown descendants are not emitted
    merely because the parent is tenant configuration.

- **Missing baseline and test evidence.**

  The review inputs do not include the process-doc baseline, diff, source
  artifacts, or test commands. The child rows are strong evidence, but they do
  not prove runtime projection behavior.

  How to verify:

  - Re-run the projection/golden tests for the cited resources.
  - Confirm generated docs/schemas/manifests reflect child rows, not only
    parent rows.

- **`free_text` descriptions can still contain sensitive prose.**

  `zia/nss-feeds.vpnCredentials.description` is `standard` only, which is the
  correct containment for now. It should not be promoted to `share` without a
  content-aware review.

- **`cloudMeta.id` is all modes.**

  This is acceptable under “ids are fine to share,” but cloud account IDs can
  be sensitive in some environments. Keep this as an explicit product decision,
  not an accident.

## Machine Contract Review

- JSON/NDJSON shape may now include parent objects in `share`/`paranoid` while
  omitting sensitive children. That is expected if child-row projection is
  implemented correctly.
- Parent rows marked `tenant_configuration` in all modes are not, by themselves,
  evidence of leakage. The child rows define the actual emitted leaves.
- No generated artifact deltas were provided. If the classification table is
  generated into docs, schemas, manifests, or agent-skill copies, those artifacts
  must show child-row modes, not only parent-row modes.
- Diff/dump artifacts should still be treated as confidential even when
  redacted. The child rows reduce sensitive content, but do not make the
  artifacts public.

## Safety Review

- Sensitive identifiers are narrowed to `standard`:
  - admin/person names,
  - FQDN/IP/port,
  - workload tag keys/values,
  - cloud resource names.
- Operational metadata is allowed in `share`/`paranoid`:
  - ids,
  - protocol,
  - operators,
  - tag type,
  - cloud provider,
  - cloud metadata id.
- Credential material is not classified as shareable in the cited rows. The
  actual PSK-bearing `vpnCredentials` remains `secret:credential` per the
  handoff.
- The main safety risk is the leaf `zpa/application-segments.modifiedBy` row.
  It should be constrained by an id-pattern test or reclassified if it can
  contain human identity data.

## Generated Artifact Review

- No code changed since round 1.
- No generated artifact diff was supplied.
- If the classification table is generated from source or used to generate
  docs/schemas/fixtures, verify that child rows are present in those artifacts.
- Watch for suspicious parent-only generated entries that would overstate
  shareability of sensitive children.

## Verdict

Verdict: approve with nits
