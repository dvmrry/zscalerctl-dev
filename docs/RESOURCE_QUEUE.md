# Resource Queue

This queue is the staging area for future read-only resources. It is not the
enabled catalog. Entries here do not expose new API surface, do not change
live-smoke coverage, and do not replace the catalog, SDK shape review, or live
smoke gates.

Use this file to avoid branch sprawl while live tenant testing is unavailable:
record candidates, scaffold intent, and known live-smoke outcomes here; apply
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
- Keep failed live-smoke endpoints in the deferred list until their endpoint or
  auth-mode behavior is understood.
- Do not use dev-tenant OneAPI availability to demote resources already proven
  under the current production legacy path. Production OneAPI becomes
  authoritative only after a controlled production smoke run.
- Treat live-smoke artifacts as confidential operational records. Store,
  retain, or dispose of them only according to the operator's approved records
  and evidence-handling policy.
- When recording live-smoke results, include the resource, auth mode,
  tenant/cloud class, date, commit, and whether the run used source or a release
  binary. Keep authorization or change-record references outside this repo if
  required by the operator's environment.

## Validation States

Use these state names when recording resource status so nobody has to infer
what "validated" means:

| State | Meaning |
| --- | --- |
| `queued` | Candidate recorded here only; no catalog, reader, or smoke surface exists. |
| `scaffolded` | Generated locally or in a draft PR; ordinary gates may pass, but no live tenant proof exists. |
| `gates-passed` | Unit, shape, projection, redaction, and smoke-script self-tests pass without live tenant evidence. |
| `dev-oneapi-availability:<status>` | Dev OneAPI returned an availability signal such as `200-records`, `200-empty`, `400`, `401`, `403`, or `404`. This is not production shape validation. |
| `legacy-zia-smoke-pass:<date>` | Focused live smoke passed with explicit ZIA legacy credentials in the current production-like environment. |
| `prod-oneapi-smoke-pass:<date>` | Focused live smoke passed with a dedicated read-only production OneAPI client. This is the canonical future validation state. |
| `deferred:<reason>` | Resource was removed or paused because endpoint behavior, auth support, shape, or entitlement is not understood. |
| `unsupported:<auth-mode>/<reason>` | Resource is supported in another auth mode, but this auth mode failed or is not expected to work. |

## Auth And Environment Posture

OneAPI is the expansion target and has been production-smoked for the current
ZPA and ZTW catalog resources. Legacy ZIA remains supported and proven for ZIA
resources. Do not remove, downgrade, or mark legacy-proven ZIA resources
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

Production OneAPI smoke should be a controlled evidence run: dedicated
read-only client, approved workstation, no CI secrets, no committed artifacts,
and artifact handling governed by the operator's records policy. Treat any
required change ticket or authorization record as an external operational
control, not as repository content.

## Current Gates

The legacy-ZIA, ZPA, and ZTW smoke gates are closed for the current catalog.
Those product surfaces were promoted only after focused work-machine live smoke
and after trimming or deferring observed tenant-specific failures.

Product-track status:

| Product | Resources | Status | Next action |
| --- | --- | --- | --- |
| ZIA | Current queued legacy-ZIA resources and singleton settings | Live-smoked and merged. | Continue only through the remaining shape-decision tracks below. |
| ZPA | Tier-1 resources plus `zpa/application-segments` | Production OneAPI smoke passed after trimming unavailable private-cloud endpoints. | Continue later from the remaining ZPA SDK surface; keep focused smoke/trim discipline. |
| ZTW | Initial reference batch plus admin-governance resources (`admin-users`, `admin-roles`) | Production OneAPI smoke passed for both the initial reference batch and admin-governance batch. | Continue policy/control surfaces only after explicit review. |
| ZCC | `trusted-networks`, `notification-templates`, `zia-postures` | Production OneAPI smoke returned 404 for all three list endpoints. | Deferred; investigate endpoint/auth/entitlement behavior before retrying ZCC. |
| Zidentity | `groups`, `users`, `resource-servers` | Production OneAPI smoke passed for the focused workforce identity/reference batch; `zidentity/users` returned 31,205 records and completed dump/manifest validation after guarded pagination was added. | Keep membership expansion as a separate child-query design. |

Do not merge product stacks on green CI alone. Promote only the resources that
pass focused live smoke, and trim or defer any endpoints that fail with
tenant/auth availability errors.

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

The current enabled catalog contains 58 ZIA resources, 16 ZPA resources, and 10
ZTW resources. The rows below are package-level scouting notes, not a promise
that every surface should become a resource.

The pinned Go SDK (`github.com/zscaler/zscaler-sdk-go/v3` v3.8.37) remains the
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
| List-only or name-get candidates | `browser_isolation`, `dlp/dlp_exact_data_match_lite`, `location/locationlite`, `trafficforwarding/dc_exclusions`, `trafficforwarding/sub_clouds`, PAC files, cloud application policy lists | Add explicit list-only/dump-only or name-get semantics before queueing. `locationlite` should wait for a concrete performance or pagination reason because it overlaps `locations` and `sublocations`. |
| SaaS/CASB split candidates | `saas_security_api`, `saas_security_api/casb_dlp_rules`, `saas_security_api/casb_malware_rules` | Split `saas_security_api` into separate resources such as domain profiles, quarantine tombstone templates, CASB email labels, CASB tenants, and SaaS scan info. CASB DLP/malware rules can use list/dump via `/all`, but `get` needs a rule-type decision. |
| Deferred live/auth failures | See [Deferred Resource SDK Recheck](DEFERRED_RESOURCE_RECHECK.md). | Retry only as focused endpoint/auth probes that record exact status code, auth mode, product cloud, endpoint path, SDK version, and source commit. |
| Adjacent-to-failure scout | `ips_control_policies/ips_policies` | Ordinary list/get shape, but adjacent to the failed IPS signature-rule endpoint. Probe separately before queueing. |
| Privacy, identity, export, or material surfaces | `adminauditlogs`, ZIA `adminuserrolemgmt/admins`, ZIA `adminuserrolemgmt/roles`, `intermediatecacertificates`, `scim_api`, `trafficforwarding/vpncredentials` | Hold for explicit privacy/material policy. These are not ordinary inventory resources. ZTW admin governance is tracked separately in the product scope plan. |
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

These were generated and locally validated, then removed after live smoke
reported list/request failures. Do not retry them as an ordinary breadth batch.
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
| `zia/network-service-groups` | Early live-smoke request failure; exact status not recorded. | Retry under current legacy-ZIA config with improved smoke summary; record status and endpoint. |
| `zia/network-applications` | Early live-smoke request failure while `zia/network-application-groups` passed; exact status not recorded. | Retry as a paired probe with `zia/network-application-groups` so endpoint behavior can be compared. |
| `zia/departments` | Legacy-ZIA list request failed; exact status not recorded. | Retry as identity-like metadata only; record status before deciding whether this is endpoint/auth failure or privacy hold. |
| `zia/users` | Legacy-ZIA list request failed; exact status not recorded. | Retry only as a privacy-scoped probe; even if reachable, user output still needs PII review before cataloging. |
| `zia/devices` | Legacy-ZIA list request failed; exact status not recorded. | Retry only as a privacy-scoped probe; even if reachable, device output still needs PII/device review before cataloging. |
| `zia/email-profiles` | Legacy-ZIA list request failed; exact status not recorded. | Retry under current legacy-ZIA config; likely ordinary config if reachable, but record status and shape first. |
| `zia/dlp-engines` | Legacy-ZIA list request failed; exact status not recorded. | Retry DLP endpoints as a small family, not mixed with unrelated policy resources. |
| `zia/dlp-dictionaries` | Legacy-ZIA list request failed; exact status not recorded. | Retry with DLP family; inspect dictionary fields for sensitive sample/content values before cataloging. |
| `zia/dlp-incident-receiver-servers` | Legacy-ZIA list request failed; exact status not recorded. | Retry with DLP family; keep receiver destination/contact fields standard-only or dropped until reviewed. |
| `zia/dlp-notification-templates` | Legacy-ZIA list request failed; exact status not recorded. | Retry with DLP family; inspect for free-text notification body fields before cataloging. |
| `zia/ips-signature-rules` | Legacy-ZIA list request failed; exact status not recorded. | Retry before `ips_policies`; do not use IPS policy adjacency as proof either way. |
| `zia/c2c-incident-receivers` | Legacy-ZIA live smoke failed with `live_access_failed`; exact status not recorded. | Retry as a single endpoint probe; receiver details may be sensitive destination metadata if reachable. |
| `zia/dlp-edm-schemas` | Legacy-ZIA live smoke failed with `live_access_failed`; exact status not recorded. | Retry with DLP family; EDM schema names/columns may be sensitive and need conservative projection. |
| `zia/dlp-idm-profile-lite` | Legacy-ZIA live smoke failed with `live_access_failed`; exact status not recorded. | Retry with DLP family; compare with full IDM profile endpoint before cataloging both. |
| `zia/dlp-idm-profiles` | Legacy-ZIA live smoke failed with `live_access_failed`; exact status not recorded. | Retry with DLP family; inspect nested matching criteria and identifiers before cataloging. |
| `zia/dlp-web-rules` | Legacy-ZIA live smoke failed with `live_access_failed`; exact status not recorded. | Retry with DLP family; policy-rule surface likely needs the same conservative nested-reference pattern as other rules. |
| `zia/traffic-capture-rules` | Legacy-ZIA live smoke failed with `live_access_failed`; exact status not recorded. | Retry as a sensitive diagnostic/capture policy probe, not an ordinary policy batch. |
| `zia/extranets` | Legacy-ZIA live smoke failed with `live_access_failed`; exact status not recorded. | Retry as a network-identifier-heavy probe; endpoints and IP/range fields should remain local-only if reachable. |
| `zpa/private-cloud-groups` | Production OneAPI/ZPA list failed with status 403. | Treat as permission/role or product-feature availability; retry only if RO client scopes/roles change. |
| `zpa/private-cloud-controllers` | Production OneAPI/ZPA list failed with status 401. | Treat as auth/config or endpoint-specific authorization; retry only with captured endpoint path and confirmed ZPA customer ID. |
| `zcc/trusted-networks` | Production OneAPI/ZCC list failed with status 404. | Probe the ZCC endpoint boundary before catalog work: compare OneAPI `/zcc/papi/public/v2/trusted-networks` routing against documented/product-local ZCC PAPI behavior and confirm whether 404 is path, cloud, entitlement, or SDK mismatch. |
| `zcc/notification-templates` | Production OneAPI/ZCC list failed with status 404. | Include in the same ZCC endpoint-boundary probe; do not interpret three 404s as three independent resource failures until the product route is proven. |
| `zcc/zia-postures` | Production OneAPI/ZCC list failed with status 404. | Include in the same ZCC endpoint-boundary probe; if the product route is corrected, re-evaluate posture criteria fields before cataloging. |

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
