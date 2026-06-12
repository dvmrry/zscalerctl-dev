# Deferred Resource SDK Recheck

This note records a source-only recheck of deferred resource candidates against
the pinned Go SDK. It exists to prevent the deferred list from becoming a bag of
"it failed once" entries with no explanation. The deferred and queued candidates
themselves live in [RESOURCE_QUEUE.md](RESOURCE_QUEUE.md); this is the SDK
source review that backs them.

This is not live validation and does not promote any resource. It only answers:

- Does the pinned SDK expose a plausible read path?
- What endpoint literal does the SDK use?
- Are write, import, export, identity, or material operations adjacent to the
  read path?
- Does the earlier live failure look more like an SDK mapping mistake, an
  endpoint/auth/entitlement problem, or a deliberate safety hold?

## Method

Rechecked local source for `github.com/zscaler/zscaler-sdk-go/v3` v3.8.37, the
version pinned by `go.mod` at the time of the original review. The focused
ordinary-recheck batch updated the pinned SDK to v3.8.38 because that release
contains the ZIA packages used by the promoted resources. The review inspected SDK
package names, exported read functions, adjacent mutating functions, and
endpoint string literals. It did not call tenant APIs, inspect live artifacts, or
use secrets.

The source tree used for the check can be located with:

```sh
SDK_DIR="$(go list -m -f '{{.Dir}}' -mod=mod github.com/zscaler/zscaler-sdk-go/v3)"
```

Then inspect the package named in the table below under `$SDK_DIR/zscaler/...`
for exported read functions, adjacent mutators, and endpoint literals. The table
is intentionally compact; the queue remains the source of truth for resource
posture.

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
  `404` during endpoint investigation. Treat that as a ZCC route/auth/entitlement
  boundary investigation before treating the resources independently.
- The ZPA private-cloud rows returned different auth-class failures (`403` and
  `401`) and have mutating neighbors. Keep them out until the role/scope and
  product-feature behavior is understood.

## Retest Buckets

| Bucket | Resources | Next action |
| --- | --- | --- |
| Promoted ordinary endpoint rechecks | `zia/network-service-groups`, `zia/network-applications`, `zia/email-profiles`, `zia/dlp-incident-receiver-servers`, `zia/dlp-notification-templates` | Cataloged in a focused branch using SDK v3.8.38 after endpoint-specific review. |
| Promoted identity/device rechecks | `zia/departments`, `zia/users`, `zia/devices` | Cataloged with standard-mode visibility and share/paranoid identifier stripping. |
| DLP and capture policy batch | `zia/dlp-engines`, `zia/dlp-dictionaries`, `zia/dlp-edm-schemas`, `zia/dlp-idm-profile-lite`, `zia/dlp-idm-profiles`, `zia/dlp-web-rules`, `zia/c2c-incident-receivers` | Cataloged on main with conservative field projection. |
| Access-gated capture/forwarding holds | `zia/traffic-capture-rules`, `zia/extranets` | SDK-backed read paths exist, but focused runtime validation returned `403`. Keep deferred until role/scope or product-feature availability is understood. |
| Access-gated admin policy hold | `zia/password-expiry-settings` | SDK-backed singleton read path exists, but focused runtime validation returned `403`. Keep deferred until admin-policy read scope or product-feature availability is understood. |
| Policy/status metadata (cataloged) | `zia/ips-policies`, `zia/activation-status`, `zia/eusa-status`, `zia/auth-exempted-urls`, `zia/intermediate-ca-certificates` | Cataloged on main with conservative field projection. IPS signature import/export and intermediate CA material/download operations remain excluded. |
| IPS signature rules (cataloged) | `zia/ips-signature-rules` | Cataloged read-only on main; the package's import, export, and validation operations remain excluded. |
| ZPA product-feature/auth holds | `zpa/private-cloud-groups`, `zpa/private-cloud-controllers` | Do not batch with ordinary ZPA references. Retry only after role/scope or feature availability changes. |
| ZCC product-boundary hold | `zcc/notification-templates`, `zcc/zia-postures` | First prove whether OneAPI should route `/zcc/papi/public/v2/...` in this tenant/cloud. (`zcc/trusted-networks`, originally part of this hold, has since been cataloged.) |

## SDK Evidence

| Resource | SDK source evidence | Recheck conclusion |
| --- | --- | --- |
| `zia/network-service-groups` | Package `firewallpolicies/networkservicegroups`; read functions `GetNetworkServiceGroups`, `GetNetworkServiceGroupsByName`, `GetAllNetworkServiceGroups`; endpoint `/zia/api/v1/networkServiceGroups`; create/update/delete neighbors. | Cataloged using the package-specific read names and reference-only child services. |
| `zia/network-applications` | Package `firewallpolicies/networkapplications`; read functions `GetNetworkApplication`, `GetByName`, `GetAll`; endpoint `/zia/api/v1/networkApplications`; no mutating functions found in the package. | Cataloged with a bounded single-page read because SDK `GetAll` can loop if this static catalog endpoint ignores pagination. |
| `zia/departments` | Package `usermanagement/departments`; read functions `GetDepartments`, `GetDepartmentLite`, `GetDepartmentsByName`, `GetAll`, `GetAllLite`; endpoints `/zia/api/v1/departments` and `/lite`; create/update/delete neighbors. | Cataloged as identity-like metadata. Comments remain standard-only free text. |
| `zia/users` | Package `usermanagement/users`; read functions `Get`, `GetUserByName`, `GetAllUsers`, `GetAllAuditors`, `GetUserReferences`; endpoints include `/zia/api/v1/users`, `/auditors`, `/references`, `/bulkDelete`; create/update/delete/bulk-delete neighbors. | Cataloged with employee identity/contact fields in standard mode only; password and custom/unreviewed child details are dropped. |
| `zia/devices` | Package `devicegroups`; read functions include `GetAllDevicesGroups`, `GetDevicesByID`, `GetDevicesByName`, `GetDevicesByModel`, `GetDevicesByOwner`, `GetDevicesByOSType`, `GetDevicesByOSVersion`, `GetAllDevices`; endpoints `/zia/api/v1/deviceGroups` and `/zia/api/v1/deviceGroups/devices`. | Cataloged under `zia/devices`; device and owner identifiers render in standard mode only. |
| `zia/email-profiles` | Package `email_profiles`; read functions `Get`, `GetEmailProfileByName`, `GetAllLite`, `GetAll`, `GetCount`; endpoints `/zia/api/v1/emailRecipientProfile`, `/lite`, `/count`; create/update/delete neighbors. | Cataloged with recipient emails visible only in standard mode. |
| `zia/dlp-engines` | Package `dlp/dlp_engines`; read functions `Get`, `GetByName`, `GetAll`, `GetEngineLiteID`, `GetByPredefinedEngine`, `GetAllEngineLite`; endpoints `/zia/api/v1/dlpEngines` and `/zia/api/v1/dlpEngines/lite`; create/update/delete neighbors. | Cataloged with expression data visible only in standard mode. |
| `zia/dlp-dictionaries` | Package `dlp/dlpdictionaries`; read functions `Get`, `GetByName`, `GetPredefinedIdentifiers`, `GetAll`; endpoint `/zia/api/v1/dlpDictionaries`; create/update/delete neighbor. | Cataloged with detector contents, patterns, match details, BINs, and phrase material dropped. |
| `zia/dlp-incident-receiver-servers` | Package `dlp/dlp_incident_receiver_servers`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zia/api/v1/incidentReceiverServers`; no mutating functions found in the package. | Cataloged with receiver URL visible only in standard mode. |
| `zia/dlp-notification-templates` | Package `dlp/dlp_notification_templates`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zia/api/v1/dlpNotificationTemplates`; create/update/delete neighbors. | Cataloged with notification message bodies dropped. |
| `zia/ips-signature-rules` | Package `ips_control_policies/ips_signature_rules`; read functions `Get`, `GetByName`, `GetAll`, `GetImportIPSSignatureRulesStatus`; endpoints include `/zia/api/v1/ipsSignatureRules`, `/export`, `/import`, `/validateRuleText`; create/update/delete/import/export/validate neighbors. | SDK-backed but not a simple config surface. Probe availability separately before using it to infer anything about IPS policies. |
| `zia/c2c-incident-receivers` | Package `c2c_incident_receiver`; read functions `Get`, `GetC2CIRName`, `GetAllLite`, `GetAll`; endpoints `/zia/api/v1/cloudToCloudIR`, `/lite`, `/count`; `ValidateDelete` neighbor. | Cataloged with tenant authorization details and validation messages dropped. |
| `zia/dlp-edm-schemas` | Package `dlp/dlp_exact_data_match`; read functions `GetDLPEDMSchemaID`, `GetDLPEDMByName`, `GetAll`; endpoint `/zia/api/v1/dlpExactDataMatchSchemas`; no mutating functions found in the package. | Cataloged with schema/file identifiers standard-only and token, column, admin, and schedule details dropped. |
| `zia/dlp-idm-profile-lite` | Package `dlp/dlp_idm_profile_lite`; read functions `GetDLPProfileLiteID`, `GetDLPProfileLiteByName`, `GetAll`; endpoint `/zia/api/v1/idmprofile/lite`; no mutating functions found in the package. | Cataloged with profile and client identifiers standard-only. |
| `zia/dlp-idm-profiles` | Package `dlp/dlp_idm_profiles`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zia/api/v1/idmprofile`; no mutating functions found in the package. | Cataloged with host, path, account, schedule, and volume details standard-only and admin identity dropped. |
| `zia/dlp-web-rules` | Package `dlp/dlp_web_rules`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zia/api/v1/webDlpRules`; create/update/delete neighbors. | Cataloged with criteria references standard-only and recursive sub-rules/admin identity dropped. |
| `zia/ips-policies` | Package `ips_control_policies/ips_policies`; read functions `Get`, `GetByName`, `GetAll`; endpoint `/zia/api/v1/firewallIpsRules`; create/update/delete neighbors. | Cataloged as ordinary read-only policy metadata with IP/category selectors and nested associations limited to standard mode. |
| `zia/traffic-capture-rules` | Package `traffic_capture`; read functions `Get`, `GetByName`, `GetAll`, `GetTrafficCaptureRuleCount`, `GetTrafficCaptureRuleOrder`, `GetTrafficCaptureRuleLabels`; endpoints include `/zia/api/v1/trafficCaptureRules`, `/count`, `/order`, `/ruleLabels`; create/update/delete neighbors. | SDK-backed, but runtime validation returned `403`. Keep deferred until role/scope or product-feature availability is understood. |
| `zia/extranets` | Package `trafficforwarding/extranet`; read functions `Get`, `GetExtranetByName`, `GetAll`, `GetLite`; endpoint `/zia/api/v1/extranet`; create/update/delete neighbors. | SDK-backed, but runtime validation returned `403`. Keep deferred until role/scope or product-feature availability is understood. |
| `zia/password-expiry-settings` | Package `adminuserrolemgmt/admins`; read function `GetPasswordExpirySettings`; endpoint `/zia/api/v1/passwordExpiry/settings`; update neighbor. | SDK-backed singleton settings read, but runtime validation returned `403`. Keep deferred until admin-policy read permissions or feature availability are confirmed. |
| `zia/intermediate-ca-certificates` | Package `intermediatecacertificates`; read functions `GetAll`, `GetCertificate`, `GetByName`; endpoint `/zia/api/v1/intermediateCaCertificate`; material/download and workflow neighbors include public-key, CSR, attestation, upload, and key-pair operations. | Cataloged for certificate metadata only. Public key material is dropped, CSR file names are standard-only, and material/download operations remain out of scope. |
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
