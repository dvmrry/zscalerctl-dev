# Zscaler Product Scope Plan

This is a scout plan for SDK-exposed products outside the currently active ZIA
and ZPA resource tracks. It is not an enabled catalog, entitlement check, safety
proof, or live response-shape validation.

The inventory was generated from the full `github.com/zscaler/zscaler-sdk-go/v3`
module cache for SDK `v3.8.38`, not only from the committed vendor tree:

```sh
SDK_DIR="$(go list -m -f '{{.Dir}}' -mod=mod github.com/zscaler/zscaler-sdk-go/v3)"
go run ./scripts/sdk-surface-inventory.go --sdk-dir "$SDK_DIR" --format json
```

Official docs checked during this scout:

- ZDX API docs describe application, device, user, alert, and troubleshooting
  endpoints, with report APIs requiring explicit time ranges:
  <https://help.zscaler.com/zdx/understanding-zdx-api>
- ZIdentity API docs describe users, groups, resource servers, and entitlements
  as identity/admin API resources:
  <https://help.zscaler.com/legacy-apis/understanding-zidentity-apis>
- Workload Group docs describe workload groups as cloud workload policy
  references:
  <https://help.zscaler.com/zia/about-workload-groups>
- Cloud Connector docs describe Cloud Connectors as traffic-forwarding
  infrastructure for ZIA/ZPA and cloud-native workloads:
  <https://help.zscaler.com/zpa/about-cloud-connectors>

The docs support the same split as the SDK: ZTW is closest to config/workload
inventory, ZDX is report/telemetry, ZIdentity is identity-plane data, and ZWA is
audit/incident data.

Preproduction OneAPI access can prove that an endpoint is reachable for that
deployment. It cannot prove production entitlement, production response shape,
or field-level safety for a financial-production tenant. Any product family that
is used only in production remains `scaffolded` or `gates-passed` until a
read-only production OneAPI runtime validation can run.

## Product Auth Posture

The current OneAPI configuration already wires direct, SDK-cache-free,
SDK-logger-free HTTP clients for ZIA, ZPA, ZTW, ZCC, and ZDX through
`NewOneAPIClient`; Zidentity uses admin/common routing through the same service
boundary. First implementations for new products should use SDK service
functions that accept `*zscaler.Service` and must not instantiate product-local
legacy clients directly.

The full SDK also exposes product-local client/config packages for ZCC, ZDX,
ZTW, and ZWA. Those clients have separate credential, logging, proxy, and cache
behavior. If a future resource requires one of those clients instead of the
OneAPI service, it needs a boundary review before implementation. ZWA especially
needs auth-path verification because the committed OneAPI client does not expose
a dedicated `ZWAHTTPClient` slot.

The CI no-live-credentials gate already blocks ZCC, ZDX, ZTW/ZTC, ZPA, ZIA, and
OneAPI credential-shaped environment names in `.github/**` YAML. That protects
the repository from accidental live-credential workflow additions, but it does
not validate product entitlement.

## Inventory Summary

| Product | SDK packages | Ordinary list/get | List/get with mutating neighbors | Read-only nonstandard | Mixed read/write | Notes |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| ZCC | 25 | 0 | 4 | 8 | 10 | Client Connector admin/config plus device and secret-adjacent surfaces. |
| ZDX | 11 | 3 | 0 | 3 | 2 | Monitoring/reporting data, not ordinary configuration inventory. |
| ZTW | 30 | 2 | 20 | 3 | 2 | Workload/Cloud Connector-style configuration and policy surfaces. |
| Zidentity | 6 | 1 | 2 | 2 | 1 | Identity-plane resources; treat as privacy and authorization sensitive. |
| ZWA | 5 | 0 | 0 | 2 | 1 | Audit and DLP incident/evidence surfaces; not ordinary config inventory. |

Counts are scout signals only. `list-get-with-mutating-neighbors` can still be
safe when zscalerctl wires only read functions, but it requires explicit review.

## Recommended Sequence

1. Keep product-family batches focused and promote only after approved runtime
   validation. ZPA, ZTW, and Zidentity have each established at least one
   focused OneAPI path.
2. Continue ZTW only through reviewed governance or configuration inventory
   slices. Initial reference resources, admin governance, and the pinned-SDK
   close-out configuration batch are cataloged after focused runtime
   validation; newly exposed SDK packages still need explicit source review
   before cataloging.
3. Keep Zidentity to top-level read-only inventory unless a child-query command
   shape is designed. Resource servers, groups, and users are cataloged; group
   membership remains a separate follow-up.
4. Defer ZCC until endpoint/auth/entitlement behavior is understood. The first
   ZCC PAPI v2 list endpoint probe returned 404 across the initial reference
   batch.
5. Do not implement ZDX before `v1.0.0` unless Zscaler exposes deterministic
   ZDX configuration APIs. The current SDK surfaces are report/telemetry.
6. Do not implement ZWA before `v1.0.0` unless Zscaler exposes deterministic
   ZWA configuration APIs. The current SDK surfaces are audit/incident data.
7. Any future Zidentity entitlement package must be reviewed from source before
   it is queued.

## ZCC

ZCC has configuration-shaped resources, but several SDK packages sit next to
mutating helpers, device data, or secret-like read functions. The first
conservative ZCC PAPI v2 endpoint probe (`trusted_network_v2`,
`notification_template`, and `zia_posture`) returned status 404 for every list
endpoint. Treat ZCC as deferred until a focused
endpoint/auth/entitlement scout proves which ZCC API path is reachable from the
shared OneAPI boundary.

| Candidate | SDK package | Scout category | Queue posture |
| --- | --- | --- | --- |
| `zcc/trusted-networks` | `zscaler/zcc/services/trusted_network_v2` | `list-get-with-mutating-neighbors` | Deferred: endpoint probe returned 404 for list. Network identifiers likely standard-only if retried later. |
| `zcc/notification-templates` | `zscaler/zcc/services/notification_template` | `list-get-with-mutating-neighbors` | Deferred: endpoint probe returned 404 for list. Revisit only after ZCC endpoint/auth behavior is proven. |
| `zcc/zia-postures` | `zscaler/zcc/services/zia_posture` | `list-get-with-mutating-neighbors` | Deferred: endpoint probe returned 404 for list. Revisit only after ZCC endpoint/auth behavior is proven. |
| `zcc/custom-ip-apps` | `zscaler/zcc/services/custom_ip_apps` | `read-only-nonstandard` | Useful, but list semantics are package-specific (`GetCustomIPApps`, `GetByAppID`, `GetByName`). Design as list-only/name-get if needed. |
| `zcc/predefined-ip-apps` | `zscaler/zcc/services/predefined_ip_apps` | `read-only-nonstandard` | Similar to custom IP apps; likely lower sensitivity because predefined. |
| `zcc/process-based-apps` | `zscaler/zcc/services/process_based_apps` | `read-only-nonstandard` | Useful but process/app identifiers may be endpoint-sensitive. |
| `zcc/devices` | `zscaler/zcc/services/devices` | `list-get-with-mutating-neighbors` | High value, but device/user/privacy heavy. Defer until base ZCC endpoint/auth and smoke behavior are proven. |

Do not queue as ordinary inventory:

| SDK package | Reason |
| --- | --- |
| `zscaler/zcc/services/secrets/getotp` | Secret/OTP surface by name and purpose. |
| `zscaler/zcc/services/secrets/getpasswords` | Password retrieval surface by name and purpose. |
| `zscaler/zcc/services/download_devices` | Export/download semantics, not ordinary JSON inventory. |
| `zscaler/zcc/services/manage_pass` and device-removal packages | Mutating or credential-adjacent admin operations. |

## ZDX

ZDX is primarily monitoring, reporting, user, device, and application experience
data. It is explicitly out of pre-`v1.0.0` implementation scope unless Zscaler
surfaces deterministic ZDX configuration APIs. Report and telemetry reads should
not be treated as plain configuration inventory.

See [ZDX Scope Plan](ZDX_SCOPE_PLAN.md) for the out-of-scope rationale and the
configuration-API exception.

| Candidate | SDK package | Scout category | Queue posture |
| --- | --- | --- | --- |
| `zdx/applications` | `zscaler/zdx/services/reports/applications` | `ordinary-list-get` | Post-`v1.0.0` report/export only. Not configuration inventory. |
| `zdx/users` | `zscaler/zdx/services/reports/users` | `ordinary-list-get` | Post-`v1.0.0` report/export only. User telemetry is privacy-sensitive. |
| `zdx/devices` | `zscaler/zdx/services/reports/devices` | `ordinary-list-get` | Post-`v1.0.0` report/export only. Device telemetry and metric fields need separate posture. |
| `zdx/alerts` | `zscaler/zdx/services/alerts` | `read-only-nonstandard` | Post-`v1.0.0` report/export only. Alert time-window and affected-device semantics are not config inventory. |
| `zdx/software-inventory` | `zscaler/zdx/services/inventory` | `read-only-nonstandard` | Post-`v1.0.0` report/export only. Software inventory is endpoint-sensitive. |
| `zdx/administration` | `zscaler/zdx/services/administration` | `read-only-nonstandard` | Out of pre-`v1.0.0` scope. Confirmed report-filter dimension reads, not configuration inventory. |

Do not enable ZDX report resources before `v1.0.0`. If Zscaler exposes actual
ZDX configuration objects, evaluate those separately under the ordinary
configuration-inventory model. Defaulting dynamic telemetry into configuration
dumps would weaken the deterministic-config story.

## ZTW

ZTW has workload, cloud account, gateway, DNS, EC group, and policy resources.
It is likely closer to Cloud Connector/Workload than to ZIA inventory.

| Candidate | SDK package | Scout category | Queue posture |
| --- | --- | --- | --- |
| `ztw/workload-groups` | `zscaler/ztw/services/workload_groups` | `ordinary-list-get` | Included in the first ZTW reference batch. Tag-expression graph is mapped but dropped in v1. |
| `ztw/public-cloud-accounts` | `zscaler/ztw/services/provisioning/public_cloud_account` | `ordinary-list-get` | Included in the first ZTW reference batch. Cloud account identifiers render standard-only. |
| `ztw/forwarding-gateways` | `zscaler/ztw/services/forwarding_gateways` | `list-get-with-mutating-neighbors` | Included in the first ZTW reference batch. Network endpoints render standard-only; admin/options internals are dropped. |
| `ztw/dns-gateways` | `zscaler/ztw/services/dns_gateway` | `list-get-with-mutating-neighbors` | Included in the first ZTW reference batch. Network endpoints render standard-only; admin/options internals are dropped. |
| `ztw/ec-groups` | `zscaler/ztw/services/ecgroup` | `list-get-with-mutating-neighbors` | Included in the first ZTW reference batch. EC VM/network internals are dropped. |
| `ztw/ip-source-groups` | `zscaler/ztw/services/policyresources/ipsourcegroups` | `list-get-with-mutating-neighbors` | Included in the first ZTW reference batch. Addresses render standard-only. |
| `ztw/ip-destination-groups` | `zscaler/ztw/services/policyresources/ipdestinationgroups` | `list-get-with-mutating-neighbors` | Included in the first ZTW reference batch. Addresses and category references render standard-only. |
| `ztw/ip-groups` | `zscaler/ztw/services/policyresources/ipgroups` | `list-get-with-mutating-neighbors` | Included in the first ZTW reference batch. Addresses render standard-only. |
| `ztw/network-services` | `zscaler/ztw/services/policyresources/networkservices` | `list-get-with-mutating-neighbors` | Included in the first ZTW reference batch. Port ranges render; free text is standard-only. |
| `ztw/network-service-groups` | `zscaler/ztw/services/policyresources/networkservicegroups` | `list-get-with-mutating-neighbors` | Included in the first ZTW reference batch. Child services render as id/name references only. |
| `ztw/admin-users` | `zscaler/ztw/services/adminuserrolemgmt/adminusers` | `list-get-with-mutating-neighbors` | Included in the admin-governance batch. Person-identifying fields render standard-only; password and token fields are dropped. |
| `ztw/admin-roles` | `zscaler/ztw/services/adminuserrolemgmt/adminroles` | `list-get-with-mutating-neighbors` | Included in the admin-governance batch. Role identity renders share-safe; permission details render standard-only. |
| `ztw/locations` | `zscaler/ztw/services/locationmanagement/location` | `list-get-with-mutating-neighbors` | Included in the pinned-SDK close-out batch. Location addresses and detailed controls render standard-only; VPN/VPC internals are dropped. |
| `ztw/location-templates` | `zscaler/ztw/services/locationmanagement/locationtemplate` | `list-get-with-mutating-neighbors` | Included in the pinned-SDK close-out batch. Template controls render standard-only; admin identity is dropped. |
| `ztw/account-groups` | `zscaler/ztw/services/partner_integrations/account_groups` | `list-get-with-mutating-neighbors` | Included in the pinned-SDK close-out batch. Child account and connector group references render standard-only. |
| `ztw/public-cloud-info` | `zscaler/ztw/services/partner_integrations/public_cloud_info` | `list-get-with-mutating-neighbors` | Included in the pinned-SDK close-out batch. Cloud account display identifiers render standard-only; external/account detail internals are dropped. |
| `ztw/zpa-application-segments` | `zscaler/ztw/services/policyresources/zparesources` | `read-only-nonstandard` | Included in the pinned-SDK close-out batch as list-only ZPA segment references. |
| `ztw/forwarding-rules` | `zscaler/ztw/services/policy_management/forwarding_rules` | `list-get-with-mutating-neighbors` | Included in the pinned-SDK close-out batch. Network criteria and nested policy references render standard-only. |
| `ztw/traffic-dns-rules` | `zscaler/ztw/services/policy_management/traffic_dns_rules` | `list-get-with-mutating-neighbors` | Included in the pinned-SDK close-out batch. Network criteria and gateway references render standard-only. |
| `ztw/traffic-log-rules` | `zscaler/ztw/services/policy_management/traffic_log_rules` | `list-get-with-mutating-neighbors` | Included in the pinned-SDK close-out batch. Location/proxy/EC references render standard-only. |
| Covered by `ztw/dns-gateways` | `zscaler/ztw/services/forwarding_gateways/dns_forwarding_gateway` | `list-get-with-mutating-neighbors` | Same `/ztw/api/v1/dnsGateways` endpoint family as the existing DNS gateways resource; do not catalog a duplicate unless the SDK diverges. |

Do not queue as ordinary inventory:

| SDK package | Reason |
| --- | --- |
| `zscaler/ztw/services/activation` and `zscaler/ztw/services/activation_cli` | Activation/control surfaces, not inventory. |
| `zscaler/ztw/services/locationmanagement/locationlite` | Lookup/helper surface rather than a full list/get inventory resource. |
| `zscaler/ztw/services/partner_integrations` | Mixed discovery/template/settings helpers; queue concrete child resources instead. |
| `zscaler/ztw/services/provisioning/api_keys` | API key surface by name. |
| `zscaler/ztw/services/provisioning/provisioning_url` | Provisioning URL surface by name. |

## ZWA

ZWA appears in the full SDK module cache as audit and DLP incident/evidence
surfaces. It is not present as a comparable high-level product client in the
committed vendor tree, and the committed OneAPI client does not expose a
dedicated `ZWAHTTPClient` field. It is explicitly out of pre-`v1.0.0`
implementation scope unless Zscaler exposes deterministic ZWA configuration
APIs.

| Candidate | SDK package | Scout category | Queue posture |
| --- | --- | --- | --- |
| `zwa/customer-audit` | `zscaler/zwa/services/customeraudit` | `read-only-nonstandard` | Out of pre-`v1.0.0` scope. Audit logs are not configuration inventory. |
| `zwa/dlp-incidents` | `zscaler/zwa/services/dlp_incidents` | `mixed-read-write-sdk-package` | Out of pre-`v1.0.0` scope. Incident, evidence, ticket, and history data are sensitive and sit next to mutating helpers. |
| `zwa/common` | `zscaler/zwa/services/common` | `read-only-nonstandard` | Helper/pagination/types package, not a catalog resource. |

Do not add ZWA to ordinary config dumps. If ZWA becomes necessary after
`v1.0.0`, treat it as an audit/incident export model with explicit retention and
privacy rules, not as static configuration inventory.

## Zidentity

Zidentity is the mixed case. The current vendored SDK exposes ordinary
workforce identity inventory next to membership mutation helpers. The CLI can
safely bind read functions for users and groups, but it must not expand group
membership or wire mutating helpers in broad list output.

| Candidate | SDK package | Scout category | Queue posture |
| --- | --- | --- | --- |
| `zidentity/resource-servers` | `zscaler/zid/services/resource_servers` | `ordinary-list-get` | In scope. OAuth-style identifiers render conservatively, and service/org internals are dropped. |
| `zidentity/groups` | `zscaler/zid/services/groups` | `list-get-with-mutating-neighbors` | In scope for group object inventory. Do not expand membership or wire add/remove/update/delete helpers. |
| `zidentity/users` | `zscaler/zid/services/users` | `list-get-with-mutating-neighbors` | In scope for administrator-visible workforce directory inventory. Drop arbitrary `customAttrsInfo`; do not wire reset-password or group-membership helpers. |

The same SDK packages also expose child membership reads
(`groups.GetUsers`, `users.GetGroupsByUser`) beside membership mutation helpers.
Model those later as explicit child queries if needed; do not fold them into the
top-level group/user inventory. This vendored SDK revision does not contain a
separate `zscaler/zid/services/user_entitlement` package; if one appears in a
future SDK release, scout it from source before queueing it.

If Zidentity is wired, bind only the specific read functions used by the
resource. Do not pass broad groups/users clients into handlers where mutating SDK
methods sit next to read methods.

## Cross-Product Implementation Rules

Apply these before any resource PR from this plan:

1. Use the OneAPI `*zscaler.Service` path when the SDK package supports it.
2. Do not add product-local legacy clients without a boundary review for
   credential discovery, logging, proxy use, cache behavior, and stdout/stderr
   writes.
3. Add product-specific runtime-validation manifest support before the first resource in
   a new product family is promoted.
4. Keep resource PRs small: one product family and one coherent API section.
5. Keep reference-only expansion: when a nested object has its own resource or
   could reasonably have one, render only id/name-style references in the parent.
6. For production-only entitlements, accept `gates-passed` without promotion and
   record the missing runtime validation explicitly.
7. Do not use dev/preprod 404s as proof that a resource is invalid in
   production; record them as entitlement or deployment-shape unknowns.

## Completed First Branches

These first-branch probes established product posture and should not be treated
as open recommendations:

| Branch | Scope | Expected outcome |
| --- | --- | --- |
| `feature/zpa-safe-surface-batch` | Verified the shared OneAPI service path for ZPA and promoted the Tier-1 ZPA reference surface. | Established the `ZSCALERCTL_ZPA_CUSTOMER_ID` requirement and trimmed unavailable private-cloud endpoints. |
| `feature/zpa-config-batch` | Promoted the remaining validated ZPA config/reference batch and recorded the explicit non-coverage buckets for feature-gated, credential/material, identity/admin, policy-control, and helper surfaces. | Treat ZPA as closed for the current validated config catalog; future work must start from the non-coverage record or a newly exposed read-only SDK surface. |
| `feature/ztw-workload-groups` | Verified the OneAPI SDK call path for ZTW and promoted the first ZTW reference batch. | Cloud/Workload product semantics established without touching provisioning credentials. |
| `feature/zcc-scope-plan` | Probed conservative ZCC PAPI v2 references. | Initial endpoint probe returned 404; ZCC remains an endpoint/auth/entitlement investigation. |
| `feature/zidentity-reference-batch` | Scoped resource servers, groups, and users. | Top-level Zidentity read-only inventory is cataloged; membership expansion remains a follow-up child-query design. |

ZWA is deliberately not in the first-branch queue. Do not open a ZWA branch
before `v1.0.0` unless Zscaler exposes deterministic configuration APIs.
