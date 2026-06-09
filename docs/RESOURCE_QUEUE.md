# Resource Queue

This queue is the staging area for future read-only resources. It is not the
enabled catalog. Entries here do not expose new API surface, do not change
runtime coverage, and do not replace the catalog, SDK shape review, or runtime
validation gates.

Use this file to avoid branch sprawl while live tenant testing is unavailable:
record candidates, scaffold intent, and endpoint investigation notes here; apply
only one small batch to production files when a live smoke operator is
available. Queue entries are not validation evidence.

## Operating Rules

- Queue only read/list/get surfaces. Write, update, delete, activate, or import
  SDK functions are not resource-queue candidates.
- Keep at most one active resource PR open for live smoke.
- Prefer one resource per PR unless the resources are tightly coupled. If a PR
  contains more than one resource, record pass/fail outcomes per resource and
  trim failed resources instead of blocking proven ones.
- Do not merge resource PRs without a focused `make live-smoke` pass.
- Do not stack un-smoked resource branches behind an unmerged resource PR.
- While live-smoke access is unavailable, do not open merge-track resource PRs
  behind the current live-smoke gate. A single disposable smoke-lab branch may
  wire queued resources for future broad live smoke, but it must remain
  clearly non-release-track until live validation trims or promotes it.
- Do not commit generated scaffold bundles from `scratch/resource-drafts/`.
- Regenerate scaffolds from current SDK source and current generator code when a
  batch is ready to apply. Do not replay stale commands blindly.
- For non-ZIA or unclear surfaces, run `make sdk-surface-inventory` first and
  record the category from `docs/SDK_SURFACE_INVENTORY.md` before queueing.
  Use the full pinned SDK module cache to decide what is available to add;
  `vendor/` only shows packages already imported by the current catalog.
- Keep failed live-smoke endpoints in the deferred list until their endpoint or
  auth-mode behavior is understood.
- Do not use dev-tenant OneAPI availability to demote resources already proven
  under the current production legacy path. Production OneAPI becomes
  authoritative only after a controlled production smoke run.
- Treat live-smoke artifacts as confidential operational records. Store,
  retain, or dispose of them only according to the operator's approved records
  and evidence-handling policy.
- Keep runtime evidence, tenant-specific result notes, and authorization or
  change-record references outside this repo when required by the operator's
  environment.

## Validation States

Use these state names when recording resource status so nobody has to infer
what "validated" means:

| State | Meaning |
| --- | --- |
| `queued` | Candidate recorded here only; no catalog, reader, or smoke surface exists. |
| `scaffolded` | Generated locally or in a draft PR; ordinary gates may pass, but no live tenant proof exists. |
| `gates-passed` | Unit, shape, projection, redaction, and smoke-script self-tests pass without live tenant evidence. |
| `dev-oneapi-availability:<status>` | Dev OneAPI returned an availability signal such as `200-records`, `200-empty`, `400`, `401`, `403`, or `404`. This is not production shape validation. |
| `legacy-zia-runtime-validated` | Focused runtime validation passed with explicit ZIA legacy credentials in an approved operator environment. |
| `prod-oneapi-runtime-validated` | Focused runtime validation passed with a dedicated read-only production OneAPI client. This is the canonical future validation state. |
| `deferred:<reason>` | Resource was removed or paused because endpoint behavior, auth support, shape, or entitlement is not understood. |
| `unsupported:<auth-mode>/<reason>` | Resource is supported in another auth mode, but this auth mode failed or is not expected to work. |

## Auth And Environment Posture

OneAPI is the expansion target and has runtime validation for the current ZPA
and ZTW catalog resources. Legacy ZIA remains supported and proven for ZIA
resources. Do not remove, downgrade, or mark legacy-validated ZIA resources
unsupported based only on dev OneAPI results or on unrelated product-family
failures.

Dev `zscalertwo` OneAPI is useful for endpoint availability scouting:

| Result | Interpretation |
| --- | --- |
| `200` with records | Endpoint and auth path work in dev; response shape may be useful but is not production proof. |
| `200` with an empty array | Endpoint and auth path work, but shape validation is weak. |
| `400` | Likely parameter/default mismatch; inspect before cataloging. |
| `401` | Auth/config failure. |
| `403` | Permission or role issue. |
| `404` | Not entitled, unavailable in that tenant/cloud, wrong path, or SDK mismatch; do not treat as a permanent resource failure without another signal. |

Production OneAPI runtime validation should be a controlled evidence run: dedicated
read-only client, approved workstation, no CI secrets, no committed artifacts,
and artifact handling governed by the operator's records policy. Treat any
required change ticket or authorization record as an external operational
control, not as repository content.

## Current Gates

The legacy-ZIA, ZPA, and ZTW runtime gates are closed for the current catalog.
Those product surfaces were promoted only after focused runtime validation and
after trimming or deferring observed endpoint failures.

Product-track status:

| Product | Resources | Status | Next action |
| --- | --- | --- | --- |
| ZIA | Current queued legacy-ZIA resources, singleton settings, focused ordinary-recheck batch, identity/device recheck for `zia/departments`, `zia/users`, and `zia/devices`, the focused DLP/capture sensitive batch, plus the list-only/static batch | Prior surfaces cataloged after focused runtime validation and review; the list-only/static batch remains runtime-gated until its focused smoke pass. | Continue only through the remaining shape-decision tracks below. |
| ZPA | 28 read-only config/reference resources across connectors, connector groups, app/server/service-edge references, app segments, browser/PRA/inspection app segments, portals, client/platform/version metadata, and microtenants. | Closed for the current validated OneAPI config catalog after focused runtime validation and trimming or deferring unavailable endpoints. | Keep ZPA closed unless scopes/features change, a future SDK exposes new read-only config surfaces, or one of the non-coverage buckets below gets an explicit design. |
| ZTW | Initial reference batch, admin-governance resources, and the pinned-SDK close-out configuration/policy batch | Cataloged after focused runtime validation and review. | Keep ZTW closed unless a future SDK bump exposes new read-only config packages. |
| ZCC | `trusted-networks`, `notification-templates`, `zia-postures` | Deferred after `404` endpoint responses across the first ZCC list batch. | Deferred; investigate endpoint/auth/entitlement behavior before retrying ZCC. |
| Zidentity | `groups`, `users`, `resource-servers` | Cataloged after focused runtime validation and review. | Keep membership expansion as a separate child-query design. |

Do not merge product stacks on green CI alone. Promote only runtime-validated
resources, and trim or defer any endpoints that fail with tenant/auth
availability errors.

## ZPA Non-Coverage Record

The ZPA catalog intentionally covers only runtime-validated, read-only
configuration and reference inventory. It does not claim that every pinned SDK
package belongs in `dump`. The current catalog covers these ZPA resources:

- `app-connector-groups`, `app-connectors`, `app-servers`,
  `application-segments`;
- `branch-connectors`, `browser-access`, `c2c-ip-ranges`,
  `cbi-zpa-profiles`, `client-types`;
- `cloud-connector-groups`, `cloud-connectors`, `config-overrides`;
- `inspection-app-segments`, `isolation-profiles`, `machine-groups`,
  `microtenants`;
- `platforms`, `posture-profiles`, `pra-app-segments`, `segment-groups`;
- `server-groups`, `service-edge-groups`, `service-edges`,
  `trusted-networks`;
- `user-portal-aups`, `user-portal-links`, `user-portals`,
  `version-profiles`.

The following ZPA SDK surfaces are intentionally not covered by the current
catalog:

| Bucket | SDK packages or resources | Why not covered |
| --- | --- | --- |
| Runtime, entitlement, or feature holds | `zpa/private-cloud-groups`, `zpa/private-cloud-controllers`, `zpa/app-connector-schedule`, `zpa/service-edge-schedule`, `zpa/cbi-banners`, `zpa/cbi-profiles`, `zpa/cbi-regions`, `zpa/inspection-profiles`, `zpa/inspection-custom-controls`, `zpa/inspection-predefined-controls`, `zpa/pra-approvals`, `zpa/pra-consoles`, `zpa/pra-portals`, `zpa/tag-groups`, `zpa/tag-keys`, `zpa/tag-namespaces` | Generated and reviewed, then deferred after focused endpoint probes. Keep them in the deferred table below and retry only when entitlement, feature enablement, or scope changes. |
| Credential, certificate, or provisioning material | `api_keys`, `provisioningkey`, `enrollmentcert`, `bacertificate` | These are credential, enrollment, certificate, or provisioning surfaces rather than ordinary inventory. They need a separate material-handling policy before any read path is considered. |
| Mutating or operational control helpers | `applicationsegment_move`, `applicationsegment_share`, `custom_config_controller`, `customer_controller`, `customer_dr_tool`, `emergencyaccess`, `extranet_resource`, `np_client`, `oauth2_user` | These packages are control, move/share, customer operation, emergency-access, or execution-adjacent APIs. The read-only inventory CLI does not execute them as dump resources. |
| Identity, admin, SCIM, SAML, or auth administration | `admin_sso_controller`, `administrator_controller`, `idpcontroller`, `role_controller`, `scim_api`, `scimattributeheader`, `scimgroup`, `samlattribute`, `step_up_auth` | These belong to an identity/admin boundary, not the current ZPA config-reference catalog. If enabled later, they need privacy/admin classification and a focused command shape. |
| Policy-set controllers | `policysetcontroller`, `policysetcontrollerv2` | These are policy-rule control APIs with mutating neighbors. Treat as a future policy-rule track, not part of the current reference/config close-out. |
| Lookup helpers, alternate views, or nonstandard command shapes | `applicationsegmentbytype`, `branch_connector_group`, `browser_protection`, `client_settings`, `location_controller`, `lssconfigcontroller`, `managed_browser`, `workload_tag_group` | These look like helper, alternate-view, singleton/settings, or nonstandard child-query surfaces. Do not catalog them until the command semantics and field classifications are explicit. |

## ZIA Non-Coverage Record

The ZIA catalog covers 102 runtime-validated, read-only configuration and
reference resources. As with ZPA, this does not claim that every pinned SDK
package belongs in `dump`. Several ZIA SDK packages back resources that are
already cataloged (for example `secure_browsing` backs `browser-control-settings`
and `supported-browser-versions`; `user_authentication_settings` backs
`auth-exempted-urls`; `cloudapplications/cloudapplications` backs
`cloud-application-policy` and `cloud-application-ssl-policy`), so package count
is not resource count.

The following ZIA SDK surfaces are intentionally not covered by the current
catalog:

| Bucket | SDK packages or resources | Why not covered |
| --- | --- | --- |
| Reporting, discovery, log, or export telemetry | `adminauditlogs`, `eventlogentryreport`, `iotreport`, `shadowitreport`, `policy_export`, `sandbox/sandbox_report` | Report, discovery, audit-log, and export surfaces are telemetry, not configuration inventory. Treat as a future report/telemetry command track rather than dump resources. |
| Submission or execution-adjacent | `sandbox/sandbox_submission`, and the activate action behind `activation` (only `activation-status` is read) | File-submission and activation actions mutate or execute. The read-only dumper reads status but does not invoke them. |
| Credential, certificate, or material | `trafficforwarding/vpncredentials`, plus the key-material, CSR, attestation, and download operations on `intermediatecacertificates` (its ordinary metadata is cataloged as `intermediate-ca-certificates`) | Secret, credential, and certificate material need a separate material-handling policy before any read path is considered. |
| Identity or SCIM administration | `scim_api` | A SCIM provisioning/identity-admin boundary, not ZIA configuration inventory. If enabled later it needs privacy/admin classification and a focused command shape. |
| Lookup, diagnostic, or alternate-view helpers | `apptotal`, `trafficforwarding/region`, `trafficforwarding/greinternalipranges`, `trafficforwarding/gretunnelinfo`, `trafficforwarding/virtualipaddress`, `location/locationlite` | Lookup, diagnostic, or lite/overlapping views with nonstandard command shapes. Do not force into config-dump semantics; `locationlite` overlaps `locations`/`sublocations`. |
| Feature or entitlement holds | `traffic_capture`, `trafficforwarding/extranet` | Generated and reviewed, then deferred after 403 under both OneAPI and legacy ZIA auth. Keep them in the deferred table below and retry only when entitlement or scope changes. |

## Zidentity Non-Coverage Record

The Zidentity catalog covers three top-level read-only identity-inventory
resources — `groups`, `users`, and `resource-servers`. Zidentity is
identity-plane data and is treated as privacy- and authorization-sensitive, so
coverage is deliberately limited to administrator-visible inventory objects with
identifier stripping (for example, arbitrary `customAttrsInfo` on users is
dropped).

The following Zidentity surfaces are intentionally not covered by the current
catalog:

| Bucket | Surface | Why not covered |
| --- | --- | --- |
| Membership child queries | Group-to-user and user-to-group membership reads (`groups.GetUsers`, `users.GetGroupsByUser`) | Membership belongs to an explicit child-query command design, not top-level group/user inventory. Model later if a concrete need appears. |
| Membership or account mutation | Add/remove/update/delete group membership, reset-password, and other mutating helpers that sit next to the read methods | The read-only CLI never wires mutating helpers, and handlers must bind only the specific read functions a resource uses. |
| Entitlements | User/role entitlement surfaces described in the Zidentity API docs | Absent from the pinned SDK revision (no `user_entitlement` package). Scout from source before queueing if a future SDK release exposes them. |
| Credential or administration surfaces | Zidentity admin-API surfaces beyond the three cataloged inventory objects | Out of scope for the current read-only inventory; they need identity/privacy review and an explicit command shape before any read path. |

## ZTW Non-Coverage Record

The ZTW catalog covers 20 runtime-validated, read-only configuration and
reference resources (admin management, DNS/forwarding gateways, EC groups,
locations and location templates, forwarding/traffic policy rules, IP and
network policy resources, partner-integration cloud accounts and info, and
workload groups). The ZTW SDK surface is nearly fully cataloged; the remaining
packages are intentionally not covered:

| Bucket | SDK packages or resources | Why not covered |
| --- | --- | --- |
| Activation or execution actions | `activation_cli`, and the activate action itself | Execution/activation actions mutate state and are not inventory. The read-only dumper does not invoke them. |
| Credential or provisioning material | `provisioning/api_keys`, `provisioning/provisioning_url` | API keys and provisioning/enrollment URLs are credential and enrollment material; they need a separate material-handling policy before any read path is considered. |
| Lookup or alternate-view helpers | `locationmanagement/locationlite` | A lite/overlapping view of `locations` with a nonstandard lookup shape. Do not force into config-dump semantics. |

One read-only candidate is **not** excluded: `activation.GetActivationStatus`
(ZTW activation status) is a clean status read mirroring `zia/activation-status`
that simply has not been wired yet. It is tracked as a focused resource-branch
candidate, not part of this non-coverage record.

## No-Live Work Mode

When read-only tenant credentials are unavailable, do not create more
merge-track production resource PRs behind the open draft PR. Safe work during
this period:

- refresh this queue from `make sdk-surface-inventory`;
- scout the full SDK module cache with:

  ```sh
  SDK_DIR="$(go list -m -f '{{.Dir}}' -mod=mod github.com/zscaler/zscaler-sdk-go/v3)"
  go run ./scripts/sdk-surface-inventory.go --sdk-dir "$SDK_DIR"
  ```

  The full module cache is the denominator for "what can be added." The
  committed `vendor/` tree is intentionally pruned by `go mod vendor` and only
  proves which SDK packages are already imported.

- add or refine batch notes, shape-decision notes, and deferred-resource notes;
- regenerate scratch scaffolds locally for review;
- optionally maintain one disposable smoke-lab branch that applies reviewed
  queued scaffolds to production files and carries a `live-smoke.manifest` for
  broad later testing. Do not mark that branch ready, merge it, or release from
  it until live smoke has identified which resources survive.

## Future Platform Improvements

These are reusable CLI/resource-model improvements. Do not mix them into a
resource smoke-lab PR unless a resource explicitly depends on the platform
change and the change can be tested without live credentials.

### Selector-Based `get`

Current `get` semantics are ID-centered. A future improvement should let
`get <selector>` accept either an ID or a catalog-declared natural key such as
`name`, `configuredName`, `commonName`, or `fqdn`.

Required contract:

- Numeric selectors use the SDK's ID get path when one exists.
- Non-numeric selectors match only catalog-declared lookup fields.
- Matching is exact only; fuzzy search belongs in a separate future `search`
  command, not `get`.
- Zero matches return not found.
- One match returns that projected record.
- Multiple matches fail closed with an ambiguous-selector error and tell the
  operator to use ID. Do not pick an arbitrary duplicate.
- List-only resources may support `get` by listing and exact-matching the
  declared lookup fields.
- Errors must remain value-free: show safe field names, IDs, and counts rather
  than dumping raw records.

This should be implemented through shared resolver logic plus small
per-resource catalog metadata, not by hardwiring lookup behavior into every
resource handler.

## Remaining SDK Package Review

The current enabled catalog contains 102 ZIA resources, 28 ZPA resources, 20
ZTW resources, and 3 Zidentity resources. The rows below are package-level
scouting notes, not a promise that every surface should become a resource.

The pinned Go SDK (`github.com/zscaler/zscaler-sdk-go/v3` v3.8.38) remains the
implementation authority. The Python SDK is useful only as scout evidence for
resource names, endpoint intent, and default query semantics; do not use it to
override the pinned Go SDK shape.

Python SDK spot-checks confirmed four important shape notes:

- Browser isolation profiles are list/search oriented and have no integer get
  equivalent.
- SaaS Security API is several separate lite/list surfaces, not one ordinary
  resource.
- Sub-clouds mixes a normal sub-cloud list with a different "last DC in
  country" lookup.
- Intermediate CA certificates mix ordinary certificate metadata with
  certificate, CSR, attestation, and public-key material/download endpoints.

Remaining work is grouped by the decision that blocks catalog work:

| Track | Surfaces | Decision before catalog work |
| --- | --- | --- |
| List-only or name-get candidates | `location/locationlite` | Add explicit list-only/dump-only or name-get semantics before queueing. `locationlite` should wait for a concrete performance or pagination reason because it overlaps `locations` and `sublocations`. |
| SaaS/CASB follow-ups | CASB tenant tag policy and SaaS scan info | The core SaaS/CASB config surfaces are staged as list-only resources. Tenant tag policy needs a tenant-ID child-query design, and scan info is telemetry rather than config. |
| Deferred live/auth failures | See [Deferred Resource SDK Recheck](DEFERRED_RESOURCE_RECHECK.md). | Retry only as focused endpoint/auth probes that record exact status code, auth mode, product cloud, endpoint path, SDK version, and source commit. |
| Staged policy/status metadata | `ips_control_policies/ips_policies`, `activation`, `user_authentication_settings`, `intermediatecacertificates` | Policy/status metadata is staged with conservative field projection. Merge requires focused runtime validation. Intermediate CA key material and download-style operations remain excluded. |
| Privacy, identity, export, or material surfaces | `adminauditlogs`, `scim_api`, `trafficforwarding/vpncredentials` | Hold for explicit privacy/material policy. These are not ordinary inventory resources. ZIA and ZTW admin governance are cataloged as read-only admin inventory with identifier stripping. |
| Helper/catalog/diagnostic surfaces | `apptotal`, `trafficforwarding/virtualipaddress` | Do not force into config dump semantics. Treat as future lookup/report/diagnostic commands if needed. |
| Product-family tracks | ZPA, ZTW, ZCC, Zidentity, ZDX, ZWA | Keep product-specific posture in [Zscaler Product Scope Plan](ZSCALER_PRODUCT_SCOPE_PLAN.md). The queue should not duplicate that product map. |

No remaining row should be wired as a normal list/get resource before one of
those track-level decisions is made. The core list-only and singleton
seams now exist:

- Catalog specs can declare only `list`, and the CLI rejects unsupported `get`
  before reader construction.
- The SDK adapter has a list-only handler for future resources with no ID
  lookup.
- Singleton specs are explicitly marked as `shape: singleton`, use the `list`
  operation contract, dump as one projected record, and record that shape in the
  dump manifest.

## Deferred Retest / Investigation Backlog

These were generated and locally validated, then removed after endpoint
failures. Do not retry them as an ordinary breadth batch.
Each retry must be a focused probe that records the exact status code, auth
mode, product cloud, endpoint path, SDK version, source commit, and whether the
run used source or a release binary.

The pinned-SDK source recheck for this backlog lives in
[Deferred Resource SDK Recheck](DEFERRED_RESOURCE_RECHECK.md). That document
records SDK package names, read functions, endpoint literals, mutating
neighbors, and the current recheck conclusion for each deferred resource. Use it
before deciding whether a failure was likely a mapping mistake, a product/auth
boundary issue, or a deliberate privacy/material hold.

| Resource | Last evidence | Required next probe |
| --- | --- | --- |
| `zia/ips-signature-rules` | Legacy-ZIA list request failed; exact status not recorded. | Retry before `ips_policies`; do not use IPS policy adjacency as proof either way. |
| `zpa/private-cloud-groups` | ZPA endpoint probe failed with status 403. | Treat as permission/role or product-feature availability; retry only if RO client scopes/roles change. |
| `zpa/private-cloud-controllers` | ZPA endpoint probe failed with status 401. | Treat as auth/config or endpoint-specific authorization; retry only with captured endpoint path and confirmed ZPA customer ID. |
| `zcc/notification-templates` | Live-smoke returned 404 (OneAPI) in the full ZCC batch. This is a v2 ZCC route (`/zcc/papi/public/v2/notification-templates`, SDK v3.8.38); the v1 ZCC surfaces on the same client succeeded. | ZCC v2 routes are not available on this tenant/cloud. Retry only if the v2 ZCC route is enabled; do not re-probe under the resolved v1 ZCC batch. |
| `zcc/zia-posture-profiles` | Live-smoke returned 404 (OneAPI) in the full ZCC batch. v2 ZCC route (`/zcc/papi/public/v2/zia-posture-profiles`, SDK v3.8.38). | Same v2-route unavailability as `zcc/notification-templates`; re-evaluate posture criteria fields when the v2 route is enabled. |
| `zcc/admin-users` | Live-smoke returned 400 (OneAPI). `admin_users.GetAdminUsers` (SDK v3.8.38) sends an empty `userType` query param. | ZCC likely requires a specific `userType` value; the empty default is rejected. Retry with a valid `userType` once the accepted values are confirmed. |
| `zcc/zpa-group-entitlements` | Live-smoke failed with a live-access error (CLI exit 5, OneAPI). v1 route (`/zcc/papi/public/v1/getZpaGroupEntitlements`, SDK v3.8.38). | The endpoint returns a single object with an embedded `totalCount`, which the SDK's paginated `ReadAllPages` may not parse, or the entitlement feature/scope is not enabled. Capture the raw response shape before re-cataloging. |
| `zcc/zdx-group-entitlements` | Live-smoke failed with a live-access error (CLI exit 5, OneAPI). v1 route (`/zcc/papi/public/v1/getZdxGroupEntitlements`, SDK v3.8.38). | Same single-object-vs-paginated shape question as `zcc/zpa-group-entitlements`. Capture the raw response shape before re-cataloging. |
| `zia/password-expiry-settings` | ZIA endpoint probe failed with status 403. | Treat as endpoint-specific permission, role, or product-feature availability. Retry only if admin-policy read scope changes or an alternate documented read path is identified. |
| `zia/traffic-capture-rules` | Live-smoke returned 403 under both OneAPI and legacy ZIA auth. Surface confirmed in pinned SDK v3.8.38 (`traffic_capture.TrafficCaptureRules`, list/get). | Treat as Traffic Capture feature entitlement or admin-role read permission, not a mapping issue. Retry only once the feature is licensed and the API role gains read access. |
| `zia/extranet` | Live-smoke returned 403 under both OneAPI and legacy ZIA auth. Surface confirmed in pinned SDK v3.8.38 (`trafficforwarding/extranet.Extranet`, list/get; nested DNS/IP-pool lists deferred). | Treat as Extranet feature entitlement or admin-role read permission. Retry only once the feature is licensed and the API role gains read access; model the nested DNS server and IP pool lists when promoting. |
| `zpa/app-connector-schedule` | Live-smoke returned 400 (OneAPI). Singleton on the standard authorized `mgmtconfig` path (`appconnectorschedule.GetSchedule`, `/zpa/mgmtconfig/v1/admin/customers/{customerId}/connectorSchedule`, SDK v3.8.38) — same path/auth as resources that passed, so not a scope or wiring issue. | ZPA returns 400 (`resource.not.found`) when no assistant/connector-disable schedule is configured on the tenant. Retry only on a tenant that has an assistant schedule configured; do not special-case the 400 as empty. |
| `zpa/service-edge-schedule` | Live-smoke returned 400 (OneAPI). Singleton on the standard authorized `mgmtconfig` path (`serviceedgeschedule.GetSchedule`, `/zpa/mgmtconfig/v1/admin/customers/{customerId}/serviceEdgeSchedule`, SDK v3.8.38) — same path/auth as resources that passed. | Same ZPA "400 until a schedule exists" behavior as `zpa/app-connector-schedule`. Retry only on a tenant with a service-edge schedule configured. |
| `zpa/cbi-banners` | Live-smoke returned 400 (OneAPI), confirmed on a second run after OneAPI scope review. Rides the separate CBI-config microservice `/zpa/cbiconfig/cbi/api/customers/{customerId}/banners` (`cbibannercontroller.GetAll`, SDK v3.8.38), not the standard mgmtconfig path. | Treat as Cloud Browser Isolation provisioning/entitlement on the `cbiconfig` service, not a wiring issue (`zpa/isolation-profiles` on the mgmtconfig path passed). Retry only once CBI is provisioned and the client can reach `cbiconfig`. |
| `zpa/cbi-profiles` | Live-smoke returned 400 (OneAPI), confirmed on re-smoke. CBI-config microservice path `/zpa/cbiconfig/cbi/api/customers/{customerId}/profiles` (`cbiprofilecontroller.GetAll`, SDK v3.8.38). | Same CBI-config provisioning/entitlement boundary as `zpa/cbi-banners`. Retry only once CBI is provisioned. |
| `zpa/cbi-regions` | Live-smoke returned 400 (OneAPI), confirmed on re-smoke. CBI-config microservice path `/zpa/cbiconfig/cbi/api/customers/{customerId}/regions` (`cbiregions.GetAll`, SDK v3.8.38). | Same CBI-config provisioning/entitlement boundary as `zpa/cbi-banners`. Retry only once CBI is provisioned. |
| `zpa/inspection-profiles` | Live-smoke returned 401 (OneAPI), confirmed on re-smoke after scope review. Standard mgmtconfig path (`inspectioncontrol/inspection_profile.GetAll`, SDK v3.8.38). | Treat as AppProtection/Inspection module licensing or OneAPI read-scope gap (sibling inspection surfaces returned 403). Retry only once Inspection is licensed and the client gains read scope. |
| `zpa/inspection-custom-controls` | Live-smoke returned 403 (OneAPI), confirmed on re-smoke. Standard mgmtconfig path (`inspectioncontrol/inspection_custom_controls.GetAll`, SDK v3.8.38). | Same Inspection module licensing / read-scope boundary as `zpa/inspection-profiles`. Retry only once Inspection is licensed and scoped. |
| `zpa/inspection-predefined-controls` | Live-smoke returned 403 (OneAPI), confirmed on re-smoke. Standard mgmtconfig path (`inspectioncontrol/inspection_predefined_controls.GetAll`, SDK v3.8.38). | Same Inspection module boundary as `zpa/inspection-profiles`. Retry only once Inspection is licensed and scoped. |
| `zpa/pra-approvals` | Live-smoke returned 400 (OneAPI), confirmed on re-smoke. Standard mgmtconfig path (`privilegedremoteaccess/praapproval.GetAll`, SDK v3.8.38). | Treat as Privileged Remote Access module licensing / read-scope gap (sibling PRA surfaces returned 403). Retry only once PRA is licensed and scoped. |
| `zpa/pra-consoles` | Live-smoke returned 403 (OneAPI), confirmed on re-smoke. Standard mgmtconfig path (`privilegedremoteaccess/praconsole.GetAll`, SDK v3.8.38). | Same PRA module boundary as `zpa/pra-approvals`. Retry only once PRA is licensed and scoped. |
| `zpa/pra-portals` | Live-smoke returned 403 (OneAPI), confirmed on re-smoke. Standard mgmtconfig path (`privilegedremoteaccess/praportal.GetAll`, SDK v3.8.38). | Same PRA module boundary as `zpa/pra-approvals`. Retry only once PRA is licensed and scoped. |
| `zpa/tag-groups` | Live-smoke returned 403 (OneAPI), confirmed on re-smoke. Standard mgmtconfig path (`tag_controller/tag_group.GetAll`, SDK v3.8.38). | Treat as resource-tagging feature entitlement / OneAPI read-scope gap. Retry only once tagging is enabled and the client gains read scope. |
| `zpa/tag-keys` | Live-smoke returned 403 (OneAPI), confirmed on re-smoke. Namespace-scoped enumeration over `tag_controller/tag_namespace` + `tag_controller/tag_key.GetAll` (SDK v3.8.38). | Same tagging entitlement / read-scope boundary as `zpa/tag-groups`. Retry only once tagging is enabled and scoped. |
| `zpa/tag-namespaces` | Live-smoke returned 403 (OneAPI), confirmed on re-smoke. Standard mgmtconfig path (`tag_controller/tag_namespace.GetAll`, SDK v3.8.38). | Same tagging entitlement / read-scope boundary as `zpa/tag-groups`. Retry only once tagging is enabled and scoped. |

## Return-To-Work Checklist

When revisiting deferred resources or applying a new batch:

1. Start from a current `main` branch and one focused product/resource family.
2. Run `make live-smoke` against the branch manifest or an explicit
   `LIVE_SMOKE_RESOURCES` list. For ZPA, include
   `ZSCALERCTL_ZPA_CUSTOMER_ID`.
3. Record pass/fail outcomes for every manifest resource with exact status
   code, auth mode, product cloud, endpoint path, source commit, and
   source-vs-release-binary context.
4. Trim failed resources from the branch or move them to the deferred table
   with the observed failure mode.
5. Merge only passing resources. If a stacked child exists, re-fetch and reset
   the child branch after the base merge, then repeat focused smoke/trim/merge.
