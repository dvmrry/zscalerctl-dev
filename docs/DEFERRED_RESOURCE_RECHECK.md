# Deferred Resource SDK Recheck

This note records a source-only recheck of deferred resource candidates against
the pinned Go SDK. It exists to prevent the deferred list from becoming a bag of
"it failed once" entries with no explanation.

This is not live validation and does not promote any resource. It only answers:

- Does the pinned SDK expose a plausible read path?
- What endpoint literal does the SDK use?
- Are write, import, export, identity, or material operations adjacent to the
  read path?
- Does the earlier live failure look more like an SDK mapping mistake, an
  endpoint/auth/entitlement problem, or a deliberate safety hold?

## Method

Rechecked local source for `github.com/zscaler/zscaler-sdk-go/v3` v3.8.37, the
version pinned by `go.mod` at the time of this review. The review inspected SDK
package names, exported read functions, adjacent mutating functions, and
endpoint string literals. It did not call tenant APIs, inspect live artifacts, or
use secrets.

The source tree used for the check can be located with:

```sh
SDK_DIR="$(go list -m -f '{{.Dir}}' -mod=mod github.com/zscaler/zscaler-sdk-go/v3)"
```

Then inspect the package named in the table below under `$SDK_DIR/zscaler/...`
for exported read functions, adjacent mutators, and endpoint literals. The table
is intentionally compact; the queue remains the source of truth for live-smoke
state.

## Summary

- Most deferred rows do have plausible SDK read functions and endpoint literals.
  They should not be treated as "bad SDK package" by default.
- Some rows remain inappropriate for ordinary breadth work even if their
  endpoint becomes reachable, because they are identity, DLP, capture, audit, or
  credential-adjacent surfaces.
- `zia/devices` is the clearest naming/mapping caveat: the SDK package is
  `devicegroups`, and the endpoint is `/zia/api/v1/deviceGroups/devices`, not a
  standalone `/devices` surface.
- The three ZCC rows share the same PAPI v2 product route shape and all returned
  `404` in production OneAPI smoke. Treat that as a ZCC route/auth/entitlement
  boundary investigation before treating the resources independently.
- The ZPA private-cloud rows returned different auth-class failures (`403` and
  `401`) and have mutating neighbors. Keep them out until the role/scope and
  product-feature behavior is understood.

## Retest Buckets

| Bucket | Resources | Next action |
| --- | --- | --- |
| Ordinary endpoint rechecks | `zia/network-service-groups`, `zia/network-applications`, `zia/email-profiles`, `zia/dlp-incident-receiver-servers`, `zia/dlp-notification-templates` | Retry as focused probes with exact status, endpoint, auth mode, cloud, SDK version, and commit. |
| Privacy holds | `zia/departments`, `zia/users`, `zia/devices` | Even if reachable, require privacy review before cataloging. `zia/devices` also needs the device-group endpoint naming decision below. |
| DLP and capture policy holds | `zia/dlp-engines`, `zia/dlp-dictionaries`, `zia/dlp-edm-schemas`, `zia/dlp-idm-profile-lite`, `zia/dlp-idm-profiles`, `zia/dlp-web-rules`, `zia/traffic-capture-rules`, `zia/c2c-incident-receivers`, `zia/extranets` | Retry only in small, family-specific probes. Cataloging still needs conservative field review because these carry policy logic, destinations, identifiers, or matching criteria. |
| IPS policy-adjacent hold | `zia/ips-signature-rules` | SDK read path exists, but the package includes import, export, and validation operations. Probe endpoint availability before considering IPS policy surfaces. |
| ZPA product-feature/auth holds | `zpa/private-cloud-groups`, `zpa/private-cloud-controllers` | Do not batch with ordinary ZPA references. Retry only after role/scope or feature availability changes. |
| ZCC product-boundary hold | `zcc/trusted-networks`, `zcc/notification-templates`, `zcc/zia-postures` | First prove whether OneAPI should route `/zcc/papi/public/v2/...` in this tenant/cloud. Treat three 404s as one boundary question. |

## SDK Evidence

| Resource | SDK source evidence | Recheck conclusion |
| --- | --- | --- |
| `zia/network-service-groups` | Package `firewallpolicies/networkservicegroups`; read functions `GetNetworkServiceGroups`, `GetNetworkServiceGroupsByName`, `GetAllNetworkServiceGroups`; endpoint `/zia/api/v1/networkServiceGroups`; create/update/delete neighbors. | Plausible read path. Retry as endpoint/auth probe; ensure adapter uses the package-specific read names rather than assuming generic `GetAll`. |
| `zia/network-applications` | Package `firewallpolicies/networkapplications`; read functions `GetNetworkApplication`, `GetByName`, `GetAll`; endpoint `/zia/api/v1/networkApplications`; no mutating functions found in the package. | Strong ordinary retest candidate. Earlier failure should be rechecked before leaving it deferred long term. |
| `zia/departments` | Package `usermanagement/departments`; read functions `GetDepartments`, `GetDepartmentLite`, `GetDepartmentsByName`, `GetAll`, `GetAllLite`; endpoints `/zia/api/v1/departments` and `/lite`; create/update/delete neighbors. | SDK-backed but identity-like. Keep privacy hold even if endpoint works. |
| `zia/users` | Package `usermanagement/users`; read functions `Get`, `GetUserByName`, `GetAllUsers`, `GetAllAuditors`, `GetUserReferences`; endpoints include `/zia/api/v1/users`, `/auditors`, `/references`, `/bulkDelete`; create/update/delete/bulk-delete neighbors. | SDK-backed but PII and mutation-adjacent. Not ordinary inventory without explicit privacy policy. |
| `zia/devices` | Package `devicegroups`; read functions include `GetAllDevicesGroups`, `GetDevicesByID`, `GetDevicesByName`, `GetDevicesByModel`, `GetDevicesByOwner`, `GetDevicesByOSType`, `GetDevicesByOSVersion`, `GetAllDevices`; endpoints `/zia/api/v1/deviceGroups` and `/zia/api/v1/deviceGroups/devices`. | SDK-backed but the resource name hides that this is nested under device groups. Revisit naming and privacy before retrying. |
| `zia/email-profiles` | Package `email_profiles`; read functions `Get`, `GetEmailProfileByName`, `GetAllLite`, `GetAll`, `GetCount`; endpoints `/zia/api/v1/emailRecipientProfile`, `/lite`, `/count`; create/update/delete neighbors. | Plausible ordinary config if reachable. Retry as focused endpoint probe. |
| `zia/dlp-engines` | Package `dlp/dlp_engines`; read functions `Get`, `GetByName`, `GetAll`, `GetEngineLiteID`, `GetByPredefinedEngine`, `GetAllEngineLite`; endpoints `/zia/api/v1/dlpEngines` and `/zia/api/v1/dlpEngines/lite`; create/update/delete neighbors. | SDK-backed, but DLP matching logic is sensitive. Retry only with DLP-family review. |
| `zia/dlp-dictionaries` | Package `dlp/dlpdictionaries`; read functions `Get`, `GetByName`, `GetPredefinedIdentifiers`, `GetAll`; endpoint `/zia/api/v1/dlpDictionaries`; create/update/delete neighbor. | SDK-backed, but dictionary content and match criteria can be sensitive. Hold for DLP-family review. |
| `zia/dlp-incident-receiver-servers` | Package `dlp/dlp_incident_receiver_servers`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zia/api/v1/incidentReceiverServers`; no mutating functions found in the package. | Strong endpoint retest candidate, with receiver destination fields reviewed conservatively if reachable. |
| `zia/dlp-notification-templates` | Package `dlp/dlp_notification_templates`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zia/api/v1/dlpNotificationTemplates`; create/update/delete neighbors. | Plausible read path. Inspect for notification body/free-text fields before cataloging. |
| `zia/ips-signature-rules` | Package `ips_control_policies/ips_signature_rules`; read functions `Get`, `GetByName`, `GetAll`, `GetImportIPSSignatureRulesStatus`; endpoints include `/zia/api/v1/ipsSignatureRules`, `/export`, `/import`, `/validateRuleText`; create/update/delete/import/export/validate neighbors. | SDK-backed but not a simple config surface. Probe availability separately before using it to infer anything about IPS policies. |
| `zia/c2c-incident-receivers` | Package `c2c_incident_receiver`; read functions `Get`, `GetC2CIRName`, `GetAllLite`, `GetAll`; endpoints `/zia/api/v1/cloudToCloudIR`, `/lite`, `/count`; `ValidateDelete` neighbor. | SDK-backed but includes tenant authorization and receiver destination concepts. Hold for focused review. |
| `zia/dlp-edm-schemas` | Package `dlp/dlp_exact_data_match`; read functions `GetDLPEDMSchemaID`, `GetDLPEDMByName`, `GetAll`; endpoint `/zia/api/v1/dlpExactDataMatchSchemas`; no mutating functions found in the package. | SDK-backed but EDM schema names and columns can disclose sensitive data models. Hold for DLP-family review. |
| `zia/dlp-idm-profile-lite` | Package `dlp/dlp_idm_profile_lite`; read functions `GetDLPProfileLiteID`, `GetDLPProfileLiteByName`, `GetAll`; endpoint `/zia/api/v1/idmprofile/lite`; no mutating functions found in the package. | SDK-backed lite surface. Compare with full IDM profile before cataloging either one. |
| `zia/dlp-idm-profiles` | Package `dlp/dlp_idm_profiles`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zia/api/v1/idmprofile`; no mutating functions found in the package. | SDK-backed but likely sensitive nested matching criteria. Hold for DLP-family review. |
| `zia/dlp-web-rules` | Package `dlp/dlp_web_rules`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zia/api/v1/webDlpRules`; create/update/delete neighbors. | SDK-backed policy surface. Requires nested-reference and condition review before cataloging. |
| `zia/traffic-capture-rules` | Package `traffic_capture`; read functions `Get`, `GetByName`, `GetAll`, `GetTrafficCaptureRuleCount`, `GetTrafficCaptureRuleOrder`, `GetTrafficCaptureRuleLabels`; endpoints include `/zia/api/v1/trafficCaptureRules`, `/count`, `/order`, `/ruleLabels`; create/update/delete neighbors. | SDK-backed but capture policy is sensitive diagnostic/control-plane data. Hold for a focused probe. |
| `zia/extranets` | Package `trafficforwarding/extranet`; read functions `Get`, `GetExtranetByName`, `GetAll`, `GetLite`; endpoint `/zia/api/v1/extranet`; create/update/delete neighbors. | SDK-backed network-identifier-heavy surface. Requires conservative projection if endpoint works. |
| `zpa/private-cloud-groups` | Package `zpa/services/private_cloud_group`; read functions `Get`, `GetByName`, `GetAll`, `GetGroupSummary`; endpoints include `/zpa/mgmtconfig/v1/admin/customers/`, `/privateCloudControllerGroup`, `/summary`; create/update/delete neighbors. | Live `403` is consistent with role/scope or feature availability. Keep product-feature/auth hold. |
| `zpa/private-cloud-controllers` | Package `zpa/services/private_cloud_controller`; read functions `Get`, `GetByName`, `GetAll`; endpoints include `/zpa/mgmtconfig/v1/admin/customers/`, `/privateCloudController`, `/restart`; update/delete neighbors. | Live `401` is endpoint-specific auth/config-sensitive. Keep out until ZPA private-cloud behavior is understood. |
| `zcc/trusted-networks` | Package `zcc/services/trusted_network_v2`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zcc/papi/public/v2/trusted-networks`; create/update/delete neighbors. | Three ZCC 404s point to product route/auth/entitlement first, not this resource alone. |
| `zcc/notification-templates` | Package `zcc/services/notification_template`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zcc/papi/public/v2/notification-templates`; create/update/delete neighbors. | Same ZCC PAPI v2 boundary question as trusted networks. |
| `zcc/zia-postures` | Package `zcc/services/zia_posture`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zcc/papi/public/v2/zia-posture-profiles`; create/update/delete neighbors. | Same ZCC PAPI v2 boundary question; posture criteria still need conservative projection if reachable. |

## Focused Probe Requirements

Do not retry these as one large catalog batch. Each probe should capture:

- resource name;
- auth mode and product cloud;
- SDK version and source commit or release version;
- endpoint path, when visible from CLI stderr or SDK source;
- exact status code;
- whether the command used source (`go run`) or a release binary;
- whether the response shape was non-empty, empty, or access-failed.

If a probe passes, promote only that resource or tightly coupled family. If it
fails, update the deferred row with the exact status and keep the resource out
of the catalog.
