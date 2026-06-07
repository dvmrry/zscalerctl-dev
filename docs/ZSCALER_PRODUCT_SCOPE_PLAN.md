# Zscaler Product Scope Plan

This is a scout plan for SDK-exposed products outside the currently active ZIA
and ZPA resource tracks. It is not an enabled catalog, entitlement check, safety
proof, or live response-shape validation.

The inventory was generated from the full `github.com/zscaler/zscaler-sdk-go/v3`
module cache for SDK `v3.8.37`, not only from the committed vendor tree:

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
read-only production OneAPI smoke can run.

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

1. Keep ZPA resource work separate from this plan until the parked ZPA stack has
   one focused production OneAPI smoke pass.
2. Start ZTW as the first separate non-ZIA/ZPA product family. It has the
   strongest config-like SDK surface and maps most directly to Cloud Connector /
   workload inventory.
3. Add a focused ZTW reference batch first, prove product auth and live smoke,
   then decide whether to continue into policy/control surfaces.
4. Defer ZCC until endpoint/auth/entitlement behavior is understood. The first
   production OneAPI smoke for ZCC PAPI v2 list endpoints returned 404 across
   the initial reference batch.
5. Do not implement ZDX before `v1.0.0` unless Zscaler exposes deterministic
   ZDX configuration APIs. The current SDK surfaces are report/telemetry.
6. Do not implement ZWA before `v1.0.0` unless Zscaler exposes deterministic
   ZWA configuration APIs. The current SDK surfaces are audit/incident data.
7. Treat Zidentity as a partial config track only: `resource_servers` is the
   cleanest read-only config candidate, `user_entitlement` is sensitive
   read-only authorization data, and users/groups are PII identity management
   surfaces with adjacent mutators.

## ZCC

ZCC has configuration-shaped resources, but several SDK packages sit next to
mutating helpers, device data, or secret-like read functions. The first
production OneAPI smoke pass against the conservative ZCC PAPI v2 batch
(`trusted_network_v2`, `notification_template`, and `zia_posture`) returned
status 404 for every list endpoint. Treat ZCC as deferred until a focused
endpoint/auth/entitlement scout proves which ZCC API path is reachable from the
shared OneAPI boundary.

| Candidate | SDK package | Scout category | Queue posture |
| --- | --- | --- | --- |
| `zcc/trusted-networks` | `zscaler/zcc/services/trusted_network_v2` | `list-get-with-mutating-neighbors` | Deferred: production OneAPI smoke returned 404 for list. Network identifiers likely standard-only if retried later. |
| `zcc/notification-templates` | `zscaler/zcc/services/notification_template` | `list-get-with-mutating-neighbors` | Deferred: production OneAPI smoke returned 404 for list. Revisit only after ZCC endpoint/auth behavior is proven. |
| `zcc/zia-postures` | `zscaler/zcc/services/zia_posture` | `list-get-with-mutating-neighbors` | Deferred: production OneAPI smoke returned 404 for list. Revisit only after ZCC endpoint/auth behavior is proven. |
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

See [ZDX Scope Plan](ZDX_SCOPE_PLAN.md) for the post-`v1.0.0` report/export
contract and the configuration-API exception.

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
| `ztw/locations` | `zscaler/ztw/services/location` | `list-get-with-mutating-neighbors` | Useful but may overlap with ZIA location semantics; review separately. |

Do not queue as ordinary inventory:

| SDK package | Reason |
| --- | --- |
| `zscaler/ztw/services/provisioning/api_keys` | API key surface by name. |
| `zscaler/ztw/services/provisioning/provisioning_url` | Provisioning URL surface by name. |
| `zscaler/ztw/services/administration/admin_users` | Admin identity surface. |
| `zscaler/ztw/services/administration/admin_roles` | Admin authorization surface. |
| `zscaler/ztw/services/policy/...` | Policy/control surfaces; valuable, but should follow the simpler ZTW references. |

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

Zidentity is the mixed case. The SDK exposes a thin read-only configuration
slice next to PII identity-management APIs and authorization data.

| Candidate | SDK package | Scout category | Queue posture |
| --- | --- | --- | --- |
| `zidentity/resource-servers` | `zscaler/zid/services/resource_servers` | `ordinary-list-get` | In scope as the only clean pre-`v1.0.0` Zidentity config candidate. Review identifiers and OAuth-style fields conservatively. |
| `zidentity/user-entitlements` | `zscaler/zid/services/user_entitlement` | `read-only-nonstandard` | Sensitive read-only authorization data. Possible later, but not the first Zidentity resource. |
| `zidentity/groups` | `zscaler/zid/services/groups` | `list-get-with-mutating-neighbors` | Hard-defer. PII/membership identity management surface with adjacent create/update/delete and membership mutators. |
| `zidentity/users` | `zscaler/zid/services/users` | `list-get-with-mutating-neighbors` | Hard-defer. PII identity management surface with adjacent mutators, including reset-password semantics. |

If Zidentity is wired, bind only the specific read functions used by the
resource. Do not pass broad groups/users clients into handlers where mutating SDK
methods sit next to read methods.

## Cross-Product Implementation Rules

Apply these before any resource PR from this plan:

1. Use the OneAPI `*zscaler.Service` path when the SDK package supports it.
2. Do not add product-local legacy clients without a boundary review for
   credential discovery, logging, proxy use, cache behavior, and stdout/stderr
   writes.
3. Add product-specific live-smoke manifest support before the first resource in
   a new product family is promoted.
4. Keep resource PRs small: one product family and one coherent API section.
5. Keep reference-only expansion: when a nested object has its own resource or
   could reasonably have one, render only id/name-style references in the parent.
6. For production-only entitlements, accept `gates-passed` without promotion and
   record the missing smoke explicitly.
7. Do not use dev/preprod 404s as proof that a resource is invalid in
   production; record them as entitlement or deployment-shape unknowns.

## First Branch Recommendations

Suggested independent branches, in order:

| Branch | Scope | Expected outcome |
| --- | --- | --- |
| `feature/ztw-workload-groups` | Verify OneAPI SDK call path for ZTW and scaffold the first ZTW reference batch. | Establish Cloud/Workload product semantics without touching provisioning credentials. |
| `feature/zcc-scope-plan` | Verify OneAPI SDK call path for ZCC and scaffold `trusted_network_v2` or `notification_template`. | Establish whether ZCC can use the current service boundary cleanly. |
| `feature/zidentity-scope-plan` | Scope `resource_servers` only. | Keep identity work to the thin read-only config slice; users/groups remain hard-deferred. |

ZWA is deliberately not in the first-branch queue. Do not open a ZWA branch
before `v1.0.0` unless Zscaler exposes deterministic configuration APIs.
