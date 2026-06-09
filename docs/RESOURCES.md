# Resource Reference

**Note on Output Fields:** Output field names mirror the upstream Zscaler SDK/API JSON tags verbatim, including known upstream oddities (such as the doubled-scope `adminScopescopeGroupMemberEntities`). This ensures output cross-references cleanly against the API. Do not rename SDK fields in output.

This document describes the currently enabled resource catalog. The catalog is
the output allow-list: reader adapters map SDK response shapes into source
records, and projection decides which fields can render for each redaction mode.

## Redaction Modes

- `standard`: local operational use. Allows explicitly reviewed tenant
  configuration and free-text fields, with secret scanning and rendered-string
  high-entropy token scanning still applied. Structured display-name fields
  skip only the high-entropy heuristic in this local mode.
- `share`: lower-detail output for tickets, reviews, and chat. Drops free text
  and sensitive identifiers.
- `paranoid`: minimal identifiers and counts only.

All fields, including allowed strings, pass through the final redaction backstop
before stdout or dump files. Rendered string values usually receive a
conservative high-entropy token scan for bare unlabeled secret material.
Structured display-name fields such as `name`, `configuredName`, and
`displayName` skip only the high-entropy heuristic in `standard` mode so long
cloud-style identifiers remain readable during local operation. `share` and
`paranoid` redact high-entropy display-name values. Self-describing secrets such
as `psk=...`, credential URLs, JWTs, and private keys still redact in display
names in every mode. Canonical UUIDs and contextual git commit SHAs are
preserved. In `standard` mode, structured rendered strings also preserve
compact UUIDs and 40/64-character hex fingerprints; `share` and `paranoid`
redact those fingerprint-shaped values. Free-text prose may redact bare hashes
without context.

## Selective Dumps

By default, `zscalerctl dump` collects every catalog resource in the selected
products. Use `--resources` to limit a dump to specific resource names:

```sh
zscalerctl dump --products zia --resources locations,static-ips --out ./dump
zscalerctl dump --resources zia/locations --out ./dump-locations
```

Unqualified names are matched within the selected products. Product-qualified
names use `product/name` and are the safer form once multiple products expose
similarly named resources. Unknown resources fail before live API access.

Dump commands fail closed by default. If a selected resource fails, no files are
written. Use `--continue-on-error` only when a partial dump is acceptable; the
manifest is marked `partial`, successful resources remain in `resources/`, and
value-free per-resource failures are written to `errors.ndjson`.

## ZIA Locations

Commands:

```sh
zscalerctl zia locations list
zscalerctl zia locations get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | ZIA location identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `ipAddresses` | Sensitive identifier | `standard` | Dropped from `share` and `paranoid`. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `preSharedKey` | Secret | never | Explicitly modeled so it cannot render. |
| `vpnCredentials` | Secret | never | SDK nested credentials are mapped into source records and dropped by projection. |

## ZIA Location Groups

Commands:

```sh
zscalerctl zia location-groups list
zscalerctl zia location-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Location group identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `deleted` | Operational metadata | `standard`, `share`, `paranoid` | Whether the group is marked deleted. |
| `groupType` | Operational metadata | `standard`, `share`, `paranoid` | Static or dynamic location group type. |
| `comments` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `lastModTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |
| `predefined` | Operational metadata | `standard`, `share`, `paranoid` | Whether the group is predefined by Zscaler. |

The SDK also returns nested dynamic criteria, member locations, and admin
references such as `lastModUser`. The reader maps those objects into source
records, but the catalog does not allow them to render, so projection drops
them.

## ZIA Rule Labels

Commands:

```sh
zscalerctl zia rule-labels list
zscalerctl zia rule-labels get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | ZIA rule-label identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `lastModifiedTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |
| `referencedRuleCount` | Operational metadata | `standard`, `share`, `paranoid` | Number of referencing rules. |

The SDK also returns admin references such as `createdBy` and `lastModifiedBy`.
The reader maps those nested objects into source records, but the catalog does
not allow them to render, so projection drops them.

## ZIA Auth Settings

Commands:

```sh
zscalerctl zia auth-settings list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `orgAuthType`, `oneTimeAuth` | Operational metadata | `standard`, `share` | Organization authentication mode metadata. |
| `samlEnabled`, `kerberosEnabled`, `mobileAdminSamlIdpEnabled` | Operational metadata | `standard`, `share` | Enabled-state flags for authentication integrations. |
| `authFrequency`, `authCustomFrequency` | Operational metadata | `standard`, `share` | Authentication frequency policy metadata. |
| `lastSyncStartTime`, `lastSyncEndTime` | Operational metadata | `standard`, `share` | Directory sync timestamps from the SDK. |
| `autoProvision`, `directorySyncMigrateToScimEnabled` | Operational metadata | `standard`, `share` | Provisioning and migration flags. |
| `kerberosPwd`, `passwordStrength`, `passwordExpiry` | Secret | never | Password-bearing settings are mapped into source records and dropped by projection. |

This is a singleton settings object. The CLI exposes it through `list` so dumps
and live smoke can treat singleton resources as one-record arrays.

## ZIA Static IPs

Commands:

```sh
zscalerctl zia static-ips list
zscalerctl zia static-ips get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Static IP identifier. |
| `ipAddress` | Sensitive identifier | `standard` | Static IP address; dropped from `share` and `paranoid`. |
| `routableIP` | Operational metadata | `standard`, `share`, `paranoid` | Whether the address is publicly routable. |
| `geoOverride` | Operational metadata | `standard`, `share` | Whether geographic coordinates are explicitly configured. |
| `latitude` | Sensitive identifier | `standard` | Geographic coordinate; dropped from `share` and `paranoid`. |
| `longitude` | Sensitive identifier | `standard` | Geographic coordinate; dropped from `share` and `paranoid`. |
| `comment` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `lastModificationTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |

The SDK also returns nested `city`, `managedBy`, and `lastModifiedBy` objects.
The reader maps those objects into source records, but the catalog does not
allow them to render, so projection drops them.

## ZIA GRE Tunnels

Commands:

```sh
zscalerctl zia gre-tunnels list
zscalerctl zia gre-tunnels get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | GRE tunnel identifier. |
| `sourceIp` | Sensitive identifier | `standard` | Source IP address; dropped from `share` and `paranoid`. |
| `internalIpRange` | Sensitive identifier | `standard` | Internal tunnel range; dropped from `share` and `paranoid`. |
| `lastModificationTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |
| `withinCountry` | Operational metadata | `standard`, `share`, `paranoid` | Whether destination VIPs are restricted to the source-IP country. |
| `comment` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `ipUnnumbered` | Operational metadata | `standard`, `share` | Whether the tunnel uses IP unnumbered mode. |
| `subcloud` | Tenant configuration | `standard`, `share` | Configured subcloud restriction. |

The SDK also returns nested `managedBy`, `lastModifiedBy`, `primaryDestVip`,
and `secondaryDestVip` objects. The reader maps those objects into source
records, but the catalog does not allow them to render, so projection drops
them.

## ZIA Sublocations

Commands:

```sh
zscalerctl zia sublocations list
zscalerctl zia sublocations get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Sublocation identifier. |
| `parentId` | Operational metadata | `standard`, `share`, `paranoid` | Parent location identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `ipAddresses` | Sensitive identifier | `standard` | Sublocation egress/internal/GRE tunnel IP entries; dropped from `share` and `paranoid`. |
| `ports` | Sensitive identifier | `standard` | Associated ports; dropped from `share` and `paranoid`. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `profile` | Tenant configuration | `standard`, `share` | Location traffic profile tag. |
| `country`, `state` | Sensitive identifier | `standard` | Geographic placement; dropped from `share` and `paranoid`. |
| `tz` | Operational metadata | `standard`, `share` | Time zone. |
| `authRequired`, `sslScanEnabled`, `ofwEnabled`, `ipsControl` | Tenant configuration | `standard`, `share` | Selected inherited/enforced policy controls. |
| `vpnCredentials` | Secret | never | SDK nested credentials are mapped into source records and dropped by projection. |

The SDK also returns additional auth, scope, group, extranet, IPv6, and policy
control fields. The reader maps reviewed high-risk parents such as
`vpnCredentials`, but the catalog keeps the first sublocation surface narrow.

## ZIA Ssl Inspection Rules

Commands:

```sh
zscalerctl zia ssl-inspection-rules list
zscalerctl zia ssl-inspection-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | SSL inspection rule identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `action.type` | Operational metadata | `standard`, `share`, `paranoid` | Rule action such as decrypt or do-not-decrypt. |
| `state` | Operational metadata | `standard`, `share`, `paranoid` | Rule state. |
| `order`, `rank` | Operational metadata | `standard`, `share`, `paranoid` | Rule ordering metadata. |
| `urlCategories` | Tenant configuration | `standard`, `share` | URL category identifiers referenced by the rule. |
| `platforms`, `cloudApplications` | Tenant configuration | `standard`, `share` | Selected non-principal criteria lists. |
| `lastModifiedTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |
| `defaultRule`, `predefined` | Operational metadata | `standard`, `share`, `paranoid` | Whether the rule is a default or predefined rule. |

The SDK also returns users, groups, departments, locations, device references,
labels, time windows, certificates, sub-actions, and admin references. The
reader maps these structures, but the catalog does not allow them to render in
this first surface.

## ZIA Url Categories

Commands:

```sh
zscalerctl zia url-categories list
zscalerctl zia url-categories get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | URL category identifier. |
| `configuredName` | Tenant configuration | `standard`, `share` | Custom category display name when present. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `type` | Operational metadata | `standard`, `share`, `paranoid` | Category type. |
| `customCategory` | Operational metadata | `standard`, `share`, `paranoid` | Whether the category is custom. |
| `editable` | Operational metadata | `standard`, `share` | Whether the caller can edit the category. |
| `customUrlsCount`, `customIpRangesCount`, `urlsRetainingParentCategoryCount`, `ipRangesRetainingParentCategoryCount` | Operational metadata | `standard`, `share`, `paranoid` | Category member counts. |
| `categoryGroup`, `superCategory` | Tenant configuration | `standard`, `share` | Category grouping metadata. |
| `urlType` | Operational metadata | `standard`, `share` | Match type such as exact or regex. |
| `urlKeywordCounts.*` | Operational metadata | `standard`, `share`, `paranoid` | URL and keyword counts only. |
| `urls`, `dbCategorizedUrls`, `ipRanges`, `ipRangesRetainingParentCategory`, `keywords`, `keywordsRetainingParentCategory`, `regexPatterns`, `regexPatternsRetainingParentCategory` | Sensitive identifier | `standard` | Raw category members. Values are scanned before output. |

The SDK also returns scope details. The reader maps those fields, but the
catalog drops `scopes` until they are separately classified.

## ZIA Url Filtering Rules

Commands:

```sh
zscalerctl zia url-filtering-rules list
zscalerctl zia url-filtering-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | URL filtering rule identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `state`, `order`, `rank` | Operational metadata | `standard`, `share`, `paranoid` | Rule state and ordering metadata. |
| `action`, `protocols`, `requestMethods`, `urlCategories`, `urlCategories2`, `userRiskScoreLevels`, `userAgentTypes` | Tenant configuration | `standard`, `share` | Primary rule criteria and action fields. |
| `sourceCountries`, `lastModifiedTime`, `enforceTimeValidity`, `validityStartTime`, `validityEndTime`, `validityTimeZoneId`, `blockOverride`, `timeQuota`, `sizeQuota`, `ciparule` | Operational metadata | varies | Reviewed non-principal rule metadata. |
| `endUserNotificationUrl`, `cbiProfileId` | Sensitive identifier | `standard` | Local-only URL/browser isolation references. |
| `labels`, `timeWindows` | Tenant configuration | `standard`, `share` | Nested references render reviewed `id`/`name` fields only. |
| `locations`, `locationGroups`, `sourceIpGroups`, `workloadGroups` | Tenant configuration | `standard` | Local-only scope references. Nested unreviewed fields are dropped. |

The SDK also returns admin, user, device, department, override, and CBI profile
objects. The reader maps those structures, but the catalog keeps them out of
rendered output until they are separately modeled.

## ZIA Firewall Filtering Rules

Commands:

```sh
zscalerctl zia firewall-filtering-rules list
zscalerctl zia firewall-filtering-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Firewall filtering rule identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `state`, `order`, `rank`, `enableFullLogging`, `defaultRule`, `predefined`, `excludeSrcCountries` | Operational metadata | `standard`, `share`, `paranoid` | Rule state, logging, ordering, and default/predefined flags. |
| `action`, `accessControl`, `nwApplications`, `deviceTrustLevels` | Tenant configuration | `standard`, `share` | Reviewed rule action and criteria metadata. |
| `sourceCountries`, `destCountries`, `lastModifiedTime` | Operational metadata | `standard`, `share` | Non-principal rule metadata. |
| `srcIps`, `destAddresses`, `destIpCategories` | Sensitive identifier | `standard` | Local-only IP and destination category criteria. |
| `labels`, `timeWindows` | Tenant configuration | `standard`, `share` | Nested references render reviewed `id`/`name` fields only. |
| `locations`, `locationGroups`, `srcIpGroups`, `destIpGroups`, `nwServices`, `nwServiceGroups`, `nwApplicationGroups`, `appServices`, `appServiceGroups`, `workloadGroups` | Tenant configuration | `standard` | Local-only scope and service references. Nested unreviewed fields are dropped. |

The SDK also returns admin, user, group, department, device, and ZPA segment
references. The reader maps those structures, but the catalog keeps them out of
rendered output until they are separately modeled.

## ZIA Forwarding Rules

Commands:

```sh
zscalerctl zia forwarding-rules list
zscalerctl zia forwarding-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Forwarding rule identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `type`, `state`, `order`, `rank`, `lastModifiedTime`, `zpaBrokerRule`, `destCountries` | Operational metadata | varies | Rule type, state, order, and forwarding metadata. |
| `forwardMethod` | Tenant configuration | `standard`, `share` | Forwarding method selected for the rule. |
| `srcIps`, `destAddresses`, `destIpCategories`, `resCategories` | Sensitive identifier | `standard` | Local-only source and destination criteria. |
| `labels` | Tenant configuration | `standard`, `share` | Nested references render reviewed `id`/`name` fields only. |
| `locations`, `locationGroups`, `ecGroups`, `srcIpGroups`, `srcIpv6Groups`, `destIpGroups`, `destIpv6Groups`, `nwServices`, `nwServiceGroups`, `nwApplicationGroups`, `appServiceGroups`, `proxyGateway`, `dedicatedIPGateway`, `zpaGateway` | Tenant configuration | `standard` | Local-only scope, service, and gateway references. Nested unreviewed fields are dropped. |

The SDK also returns admin, user, group, department, device, and ZPA segment
objects. The reader maps those structures, but the catalog keeps them out of
rendered output until they are separately modeled.

## ZIA IPs Signature Rules

Commands:

```sh
zscalerctl zia ips-signature-rules list
zscalerctl zia ips-signature-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | IPS signature rule identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `enabled`, `deleted`, `dynamicValidationSubmitted`, `dynamicValidationRejected`, `dynamicValidationSucceeded`, `disabledFromZSCM`, `dynamicValRejectCode` | Operational metadata | `standard`, `share`, `paranoid` | Rule state and dynamic-validation status flags. |
| `promoteTime`, `ruleTextModTime` | Operational metadata | `standard`, `share` | Non-principal rule timestamps. |

The SDK also returns the signature rule text and category reference. The reader
keeps the detection text and category out of rendered output until they are
separately modeled.

## ZIA IPs Policies

Commands:

```sh
zscalerctl zia ips-policies list
zscalerctl zia ips-policies get <id>
zscalerctl dump --products zia --resources zia/ips-policies --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `order`, `rank`, `lastModifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | IPS policy identifiers, ordering, and timestamp metadata. |
| `name`, `accessControl`, `enableFullLogging`, `action`, `state`, `defaultRule`, `capturePCAP`, `predefined`, `isEunEnabled`, `eunTemplateId` | Tenant configuration | `standard`, `share` | Policy controls and enabled-state metadata. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `sourceCountries`, `destCountries` | Operational metadata | `standard`, `share` | Country selector metadata. |
| `srcIps`, `destAddresses`, `destIpCategories`, `resCategories` | Sensitive identifier | `standard` | Local-only IP and category selectors. |
| `labels`, `timeWindows` | Tenant configuration | `standard`, `share` | Nested references render reviewed `id`/`name` fields only. |
| `locations`, `locationGroups`, `departments`, `groups`, `users`, `destIpGroups`, `destIpv6Groups`, `nwServices`, `nwServiceGroups`, `srcIpGroups`, `srcIpv6Groups`, `deviceGroups`, `devices`, `threatCategories`, `zpaAppSegments` | Tenant configuration | `standard` | Local-only scope and policy references. Nested unreviewed fields are dropped. |
| `lastModifiedBy` | Secret | never | Admin identity reference is mapped and dropped. |

## ZIA IP Source Groups

Commands:

```sh
zscalerctl zia ip-source-groups list
zscalerctl zia ip-source-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Source IP group identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `ipAddresses` | Sensitive identifier | `standard` | Local-only source IP addresses/ranges. |
| `isNonEditable` | Operational metadata | `standard`, `share`, `paranoid` | Whether the group is predefined/non-editable. |

## ZIA IP Destination Groups

Commands:

```sh
zscalerctl zia ip-destination-groups list
zscalerctl zia ip-destination-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Destination IP group identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `type` | Operational metadata | `standard`, `share`, `paranoid` | Destination group type. |
| `addresses`, `ipCategories` | Sensitive identifier | `standard` | Local-only destination addresses, FQDNs, wildcard FQDNs, and URL category references. |
| `countries` | Operational metadata | `standard`, `share` | Destination country criteria. |
| `isNonEditable` | Operational metadata | `standard`, `share`, `paranoid` | Whether the group is predefined/non-editable. |

## ZIA Network Services

Commands:

```sh
zscalerctl zia network-services list
zscalerctl zia network-services get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Network service identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `tag`, `protocol` | Tenant configuration | `standard`, `share` | Service tag and protocol metadata. |
| `type`, `isNameL10nTag` | Operational metadata | `standard`, `share`, `paranoid` | Service type and localization flag. |
| `srcTcpPorts`, `destTcpPorts`, `srcUdpPorts`, `destUdpPorts` | Tenant configuration | `standard`, `share` | Port ranges render reviewed `start`/`end` values only. |

## ZIA Network Service Groups

Commands:

```sh
zscalerctl zia network-service-groups list
zscalerctl zia network-service-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Network service group identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `services` | Tenant configuration | `standard` | Local-only service references render reviewed `id`/`name` fields only. Query `zia/network-services` for service details. |

## ZIA Network Applications

Commands:

```sh
zscalerctl zia network-applications list
zscalerctl zia network-applications get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `deprecated` | Operational metadata | `standard`, `share`, `paranoid` | Network application identifier and deprecated flag. |
| `parentCategory` | Tenant configuration | `standard`, `share` | ZIA network application parent category. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |

## ZIA Application Services

Commands:

```sh
zscalerctl zia application-services list
zscalerctl zia application-services get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Application service identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `nameL10nTag` | Operational metadata | `standard`, `share`, `paranoid` | Whether the name is a localization tag. |

## ZIA Application Service Groups

Commands:

```sh
zscalerctl zia application-service-groups list
zscalerctl zia application-service-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Application service group identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `nameL10nTag` | Operational metadata | `standard`, `share`, `paranoid` | Whether the name is a localization tag. |

The ZIA SDK does not expose a direct get-by-id helper for application services
or application service groups, so `get` uses the list endpoint and returns the
matching identifier.

## ZIA Network Application Groups

Commands:

```sh
zscalerctl zia network-application-groups list
zscalerctl zia network-application-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Network application group identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `networkApplications` | Tenant configuration | `standard`, `share` | Network application identifiers included in the group. |

## ZIA Time Windows

Commands:

```sh
zscalerctl zia time-windows list
zscalerctl zia time-windows get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Time-window identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `startTime`, `endTime`, `dayOfWeek` | Tenant configuration | `standard`, `share` | Scheduled matching criteria. |

The ZIA SDK does not expose a direct get-by-id helper for time windows, so
`get` uses the list endpoint and returns the matching identifier.

## ZIA Proxies

Commands:

```sh
zscalerctl zia proxies list
zscalerctl zia proxies get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Proxy identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `type` | Operational metadata | `standard`, `share`, `paranoid` | Proxy type. |
| `address` | Sensitive identifier | `standard` | Proxy address or FQDN. |
| `port` | Operational metadata | `standard`, `share` | Proxy listener port. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `insertXauHeader`, `base64EncodeXauHeader` | Tenant configuration | `standard`, `share` | X-Authenticated-User header controls. |
| `cert` | Tenant configuration | `standard`, `share` | Certificate reference; `externalId` and `extensions` are dropped. |
| `lastModifiedTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |

The SDK also returns `lastModifiedBy`. The reader maps it into source records,
but the catalog does not allow it to render, so projection drops it.

## ZIA Proxy Gateways

Commands:

```sh
zscalerctl zia proxy-gateways list
zscalerctl zia proxy-gateways get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Proxy gateway identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `failClosed`, `type` | Operational metadata | `standard`, `share`, `paranoid` | Gateway behavior and type. |
| `primaryProxy`, `secondaryProxy` | Tenant configuration | `standard`, `share` | Proxy references; `externalId` and `extensions` are dropped. |
| `lastModifiedTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |

The ZIA SDK does not expose a direct get-by-id helper for proxy gateways, so
`get` uses the list endpoint and returns the matching identifier. The SDK also
returns `lastModifiedBy`; projection drops it.

## ZIA Dedicated IP Gateways

Commands:

```sh
zscalerctl zia dedicated-ip-gateways list
zscalerctl zia dedicated-ip-gateways get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Dedicated IP gateway identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `primaryDataCenter`, `secondaryDataCenter` | Tenant configuration | `standard`, `share` | Data-center references; `extensions` is dropped. |
| `createTime`, `lastModifiedTime` | Operational metadata | `standard`, `share` | SDK timestamp values. |
| `default` | Operational metadata | `standard`, `share`, `paranoid` | Whether this is the default gateway. |

Dedicated IP gateways use the SDK lite endpoint and list-derived `get`. The
SDK also returns `lastModifiedBy`; projection drops it.

## ZIA Time Intervals

Commands:

```sh
zscalerctl zia time-intervals list
zscalerctl zia time-intervals get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Time interval identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `startTime`, `endTime`, `daysOfWeek` | Tenant configuration | `standard`, `share` | Scheduled matching criteria. |

## ZIA Bandwidth Classes

Commands:

```sh
zscalerctl zia bandwidth-classes list
zscalerctl zia bandwidth-classes get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Bandwidth class identifier. |
| `isNameL10nTag`, `type` | Operational metadata | `standard`, `share`, `paranoid` | Localization flag and class type. |
| `name`, `getfileSize`, `fileSize`, `webApplications`, `applicationServiceGroups`, `networkApplications`, `networkServices`, `urlCategories`, `applications` | Tenant configuration | `standard`, `share` | Reviewed class matching and sizing metadata. |
| `urls` | Sensitive identifier | `standard` | Local-only URL criteria; values are scanned before output. |

## ZIA Bandwidth Control Rules

Commands:

```sh
zscalerctl zia bandwidth-control-rules list
zscalerctl zia bandwidth-control-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Bandwidth rule identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `order`, `state`, `rank`, `defaultRule` | Operational metadata | `standard`, `share`, `paranoid` | Rule ordering, state, and default metadata. |
| `lastModifiedTime`, `accessControl` | Operational metadata | `standard`, `share` | SDK timestamp and access metadata. |
| `maxBandwidth`, `minBandwidth`, `protocols`, `deviceTrustLevels`, `bandwidthClasses`, `labels`, `timeWindows` | Tenant configuration | `standard`, `share` | Reviewed bandwidth and rule criteria. |
| `locations`, `locationGroups`, `devices`, `deviceGroups` | Tenant configuration | `standard` | Local-only scope references. Nested unreviewed fields are dropped. |

The SDK also returns `lastModifiedBy`. The reader maps it into source records,
but the catalog does not allow it to render, so projection drops it.

## ZIA Dns Gateways

Commands:

```sh
zscalerctl zia dns-gateways list
zscalerctl zia dns-gateways get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | DNS gateway identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `dnsGatewayType`, `autoCreated`, `natZtrGateway` | Operational metadata | `standard`, `share`, `paranoid` | Gateway type and flags. |
| `primaryIpOrFqdn`, `secondaryIpOrFqdn` | Sensitive identifier | `standard` | Local-only gateway endpoints. |
| `primaryPorts`, `secondaryPorts`, `protocols`, `failureBehavior`, `dnsGatewayProtocols` | Tenant configuration | `standard`, `share` | Reviewed DNS gateway connectivity settings. |
| `lastModifiedTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |

The SDK also returns `lastModifiedBy`. The reader maps it into source records,
but the catalog does not allow it to render, so projection drops it.

## ZIA Nat Control Rules

Commands:

```sh
zscalerctl zia nat-control-rules list
zscalerctl zia nat-control-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | NAT control rule identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `order`, `rank`, `state`, `trustedResolverRule`, `enableFullLogging`, `predefined`, `defaultRule` | Operational metadata | `standard`, `share`, `paranoid` | Rule state, order, logging, and default/predefined metadata. |
| `accessControl`, `redirectPort`, `destCountries`, `labels`, `timeWindows` | Tenant configuration or operational metadata | `standard`, `share` | Reviewed non-secret rule configuration. |
| `redirectFqdn`, `redirectIp`, `destAddresses`, `srcIps`, `destIpCategories`, `resCategories` | Sensitive identifier | `standard` | Local-only NAT endpoints and criteria. |
| `locations`, `locationGroups`, `srcIpGroups`, `srcIpv6Groups`, `destIpGroups`, `destIpv6Groups`, `nwServices`, `nwServiceGroups`, `groups`, `departments`, `users`, `devices`, `deviceGroups` | Tenant configuration | `standard` | Local-only scope, principal, service, and device references. Nested unreviewed fields are dropped. |

The SDK also returns `lastModifiedBy`. The reader maps it into source records,
but the catalog does not allow it to render, so projection drops it.

## ZIA Groups

Commands:

```sh
zscalerctl zia groups list
zscalerctl zia groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Group identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `idpId` | Tenant configuration | `standard`, `share` | Identity provider identifier. |
| `comments` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `isSystemDefined` | Operational metadata | `standard`, `share`, `paranoid` | Whether the group is system-defined. |

## ZIA Departments

Commands:

```sh
zscalerctl zia departments list
zscalerctl zia departments get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `deleted` | Operational metadata | `standard`, `share`, `paranoid` | Department identifier and deleted flag. |
| `name`, `idpId` | Tenant configuration | `standard`, `share` | Identity-provider department metadata. |
| `comments` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |

## ZIA Users

Commands:

```sh
zscalerctl zia users list
zscalerctl zia users get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `deleted` | Operational metadata | `standard`, `share`, `paranoid` | User identifier and deleted flag. |
| `adminUser` | Operational metadata | `standard`, `share` | Administrator flag. |
| `type` | Tenant configuration | `standard`, `share` | User type/classification returned by ZIA. |
| `name`, `email`, `tempAuthEmail` | Sensitive identifier | `standard` | Employee identity/contact fields; visible to the local administrator, dropped from `share` and `paranoid`. |
| `groups`, `department` | Tenant configuration | `standard` | Rendered as id/name references only. Query `zia/groups` or `zia/departments` for authoritative metadata. |
| `comments` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `authMethods` | Tenant configuration | `standard` | Authentication methods are local-only because they describe employee auth posture. |
| `password` | Secret | none | Dropped in all modes. |

## ZIA Device Groups

Commands:

```sh
zscalerctl zia device-groups list
zscalerctl zia device-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Device group identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `groupType`, `osType`, `predefined`, `deviceCount` | Operational metadata | `standard`, `share`, `paranoid` | Group type, OS, predefined flag, and member count. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `deviceNames` | Sensitive identifier | `standard` | Local-only device-name list when returned by the SDK. |

Device groups use a list-derived `get`.

## ZIA Devices

Commands:

```sh
zscalerctl zia devices list
zscalerctl zia devices get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Device identifier. |
| `deviceGroupType`, `osType`, `osVersion` | Tenant configuration | `standard`, `share` | Device classification and OS metadata. |
| `name`, `deviceModel`, `ownerUserId`, `ownerName`, `hostName` | Sensitive identifier | `standard` | Employee/device locating fields; visible to the local administrator, dropped from `share` and `paranoid`. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |

## ZIA Workload Groups

Commands:

```sh
zscalerctl zia workload-groups list
zscalerctl zia workload-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Workload group identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `expression` | Sensitive identifier | `standard` | Local-only workload tag expression. |
| `lastModifiedTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |

The SDK also returns structured expression JSON and `lastModifiedBy`. The
reader maps those structures, but the catalog does not allow them to render, so
projection drops them.

## ZIA Alert Subscriptions

Commands:

```sh
zscalerctl zia alert-subscriptions list
zscalerctl zia alert-subscriptions get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `deleted` | Operational metadata | `standard`, `share`, `paranoid` | Subscription identifier and deletion state. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `email` | Sensitive identifier | `standard` | Local-only notification recipient address. |
| `pt0Severities`, `secureSeverities`, `manageSeverities`, `complySeverities`, `systemSeverities` | Tenant configuration | `standard`, `share` | Reviewed alert severity selections. |

## ZIA Activation Status

Commands:

```sh
zscalerctl zia activation-status show
zscalerctl dump --products zia --resources zia/activation-status --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `status` | Operational metadata | `standard`, `share` | Activation status metadata. |

## ZIA Eusa Status

Commands:

```sh
zscalerctl zia eusa-status show
zscalerctl dump --products zia --resources zia/eusa-status --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | EUSA status identifier. |
| `version` | Tenant configuration | `standard`, `share` | Rendered as an id/name reference; SDK extension data is dropped. |
| `acceptedStatus` | Tenant configuration | `standard`, `share` | Current acceptance status. |

## ZIA Auth Exempted Urls

Commands:

```sh
zscalerctl zia auth-exempted-urls show
zscalerctl dump --products zia --resources zia/auth-exempted-urls --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `urls` | Sensitive identifier | `standard` | Authentication-exempt URL list; dropped from `share` and `paranoid`. |

## ZIA Intermediate Ca Certificates

Commands:

```sh
zscalerctl zia intermediate-ca-certificates list
zscalerctl zia intermediate-ca-certificates get <id>
zscalerctl dump --products zia --resources zia/intermediate-ca-certificates --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Certificate metadata identifier. |
| `name`, `type`, `status`, `defaultCertificate`, `currentState` | Tenant configuration | `standard`, `share` | Certificate metadata and workflow state. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `region`, `csrFileName` | Sensitive identifier | `standard` | Location and file identifiers are visible locally and dropped from `share` and `paranoid`. |
| `certStartDate`, `certExpDate`, `keyGenerationTime`, `hsmAttestationVerifiedTime`, `csrGenerationTime` | Operational metadata | `standard`, `share` | Certificate lifecycle timestamps. |
| `publicKey` | Secret | never | Key material is mapped and dropped. |

## ZIA Cloud App Instances

Commands:

```sh
zscalerctl zia cloud-app-instances list
zscalerctl zia cloud-app-instances get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `instanceId`, `instanceType`, `modifiedAt` | Operational metadata | `standard`, `share`, `paranoid` for ID/type; `standard`, `share` for modified time | Instance identity, type, and timestamp metadata. |
| `instanceName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `modifiedBy`, `instanceIdentifiers` | Secret | never | Admin references and tenant identifiers are mapped into source records and dropped by projection. |

## ZIA Tenancy Restriction Profiles

Commands:

```sh
zscalerctl zia tenancy-restriction-profiles list
zscalerctl zia tenancy-restriction-profiles get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `appType`, `itemTypePrimary`, `itemTypeSecondary`, `lastModifiedTime` | Operational metadata | `standard`, `share`, `paranoid` for identifiers/types; `standard`, `share` for modified time | Profile identity, app type, item type, and timestamp metadata. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `restrictPersonalO365Domains`, `allowGoogleConsumers`, `msLoginServicesTrV2`, `allowGoogleVisitors`, `allowGcpCloudStorageRead` | Tenant configuration | `standard`, `share` | Reviewed tenancy restriction flags. |
| `itemDataPrimary`, `itemDataSecondary`, `itemValue`, `lastModifiedUserId` | Sensitive identifier | `standard` | Local-only tenant data values and admin identifier. |

## ZIA Vzen Clusters

Commands:

```sh
zscalerctl zia vzen-clusters list
zscalerctl zia vzen-clusters get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `status`, `type`, `ipSecEnabled` | Operational metadata | `standard`, `share`, `paranoid` | Cluster identifier, state, type, and IPSec flag. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `ipAddress`, `subnetMask`, `defaultGateway` | Sensitive identifier | `standard` | Local-only network addressing. |
| `virtualZenNodes` | Tenant configuration | `standard` | Local-only node references render reviewed `id`/`name` fields only; external IDs and extensions drop. |

## ZIA Vzen Nodes

Commands:

```sh
zscalerctl zia vzen-nodes list
zscalerctl zia vzen-nodes get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `zgatewayId`, `status`, `inProduction`, `type`, `ipSecEnabled`, `onDemandSupportTunnelEnabled`, `establishSupportTunnelEnabled`, `deploymentMode`, `vzenSkuType` | Operational metadata | `standard`, `share`, `paranoid` | Node identity, status, deployment, support-tunnel, and SKU metadata. |
| `name`, `clusterName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `ipAddress`, `subnetMask`, `defaultGateway`, `loadBalancerIpAddress` | Sensitive identifier | `standard` | Local-only network addressing. |

## ZIA Dlp Engines

Commands:

```sh
zscalerctl zia dlp-engines list
zscalerctl zia dlp-engines get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `customDlpEngine` | Operational metadata | `standard`, `share`, `paranoid` | Engine identity and custom/predefined flag. |
| `name`, `predefinedEngineName` | Tenant configuration | `standard`, `share` | Engine names; scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `engineExpression` | Sensitive identifier | `standard` | Local-only dictionary expression; dropped from `share` and `paranoid`. |

## ZIA Dlp Dictionaries

Commands:

```sh
zscalerctl zia dlp-dictionaries list
zscalerctl zia dlp-dictionaries get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `nameL10nTag`, `custom`, `proximity`, `ignoreExactMatchIdmDict`, `includeBinNumbers`, `dictTemplateId`, `predefinedClone`, `proximityLengthEnabled`, `proximityEnabledForCustomDictionary`, `dictionaryCloningEnabled`, `customPhraseSupported`, `hierarchicalDictionary`, `thresholdAllowed` | Operational metadata | `standard`, `share`, `paranoid` | Dictionary identity, broad type/status flags, and numeric thresholds. |
| `name`, `confidenceThreshold`, `customPhraseMatchType`, `thresholdType`, `dictionaryType`, `predefinedCountActionType`, `confidenceLevelForPredefinedDict` | Tenant configuration | `standard`, `share` | Detection-mode metadata; scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `phrases`, `patterns`, `exactDataMatchDetails`, `idmProfileMatchAccuracyDetails`, `binNumbers`, `hierarchicalIdentifiers`, `predefinedPhrases` | Secret | never | Detector content, data-model mappings, BIN lists, and predefined/custom phrase material are dropped. |

## ZIA Dlp Edm Schemas

Commands:

```sh
zscalerctl zia dlp-edm-schemas list
zscalerctl zia dlp-edm-schemas get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `schemaId`, `revision`, `fileUploadStatus`, `schemaStatus`, `lastModifiedTime`, `schemaActive`, `schedulePresent` | Operational metadata | `standard`, `share`, `paranoid` | Schema identity, revision, status, and broad scheduling/activity flags. |
| `origColCount`, `cellsUsed` | Operational metadata | `standard` | Local-only EDM scale metadata. |
| `edmClient` | Tenant configuration | `standard` | Rendered as an id/name reference only. |
| `projectName`, `filename`, `originalFileName` | Sensitive identifier | `standard` | Local-only schema and upload identifiers. |
| `modifiedBy`, `createdBy`, `tokenList`, `schedule` | Secret | never | Admin identities, token/column metadata, and schedule detail are dropped. |

## ZIA Dlp Idm Profile Lite

Commands:

```sh
zscalerctl zia dlp-idm-profile-lite list
zscalerctl zia dlp-idm-profile-lite get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `profileId`, `lastModifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | IDM profile identifier and timestamp. |
| `numDocuments` | Operational metadata | `standard` | Local-only indexed-document volume. |
| `templateName` | Sensitive identifier | `standard` | Local-only IDM template name. |
| `clientVm` | Tenant configuration | `standard` | Rendered as an id/name reference only. |
| `modifiedBy` | Secret | never | Admin identity is dropped. |

## ZIA Dlp Idm Profiles

Commands:

```sh
zscalerctl zia dlp-idm-profiles list
zscalerctl zia dlp-idm-profiles get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `profileId`, `version`, `lastModifiedTime`, `uploadStatus` | Operational metadata | `standard`, `share`, `paranoid` | IDM profile identity, version, upload status, and timestamp. |
| `scheduleDay`, `scheduleTime`, `scheduleDisabled`, `volumeOfDocuments`, `numDocuments` | Operational metadata | `standard` | Local-only schedule and indexed-document volume. |
| `profileType` | Tenant configuration | `standard`, `share` | IDM profile type. |
| `scheduleType`, `scheduleDayOfMonth`, `scheduleDayOfWeek`, `idmClient` | Tenant configuration | `standard` | Local-only schedule and Index Tool reference. |
| `profileName`, `host`, `port`, `profileDirPath`, `userName` | Sensitive identifier | `standard` | Local-only host, path, and account identifiers. |
| `profileDesc` | Free text | `standard` | Admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `modifiedBy` | Secret | never | Admin identity is dropped. |

## ZIA Dlp Web Rules

Commands:

```sh
zscalerctl zia dlp-web-rules list
zscalerctl zia dlp-web-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `order`, `rank`, `minSize`, `state`, `matchOnly`, `lastModifiedTime`, `withoutContentInspection`, `ocrEnabled`, `dlpDownloadScanEnabled`, `zccNotificationsEnabled`, `zscalerIncidentReceiver`, `parentRule`, `inspectHttpGetEnabled` | Operational metadata | `standard`, `share`, `paranoid` | Rule identity, ordering, state, and broad scan/notification flags. |
| `accessControl`, `protocols`, `name`, `fileTypes`, `cloudApplications`, `action`, `severity`, `dlpContentLocationsScopes` | Tenant configuration | `standard`, `share` | Rule metadata and action criteria. |
| `description` | Free text | `standard` | Admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `eunTemplateId`, `externalAuditorEmail` | Sensitive identifier | `standard` | Local-only template and auditor identifiers. |
| `auditor`, `notificationTemplate`, `icapServer`, `receiver`, `locations`, `locationGroups`, `groups`, `departments`, `users`, `urlCategories`, `dlpEngines`, `timeWindows`, `labels`, `excludedGroups`, `excludedDepartments`, `excludedUsers`, `includedDomainProfiles`, `excludedDomainProfiles`, `sourceIpGroups`, `workloadGroups`, `fileTypeCategories`, `userRiskScoreLevels` | Tenant configuration | `standard` | Local-only constrained references; nested objects render reviewed id/name fields only. |
| `lastModifiedBy`, `subRules` | Secret | never | Admin identity and recursive nested rules are dropped. |

## ZIA Dlp Icap Servers

Commands:

```sh
zscalerctl zia dlp-icap-servers list
zscalerctl zia dlp-icap-servers get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `status` | Operational metadata | `standard`, `share`, `paranoid` | ICAP server identity and status. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `url` | Sensitive identifier | `standard` | Local-only ICAP server endpoint. |

## ZIA Dlp Incident Receiver Servers

Commands:

```sh
zscalerctl zia dlp-incident-receiver-servers list
zscalerctl zia dlp-incident-receiver-servers get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `flags` | Operational metadata | `standard`, `share`, `paranoid` for `id`; `standard`, `share` for flags | Incident receiver identifier and SDK flag value. |
| `name`, `status` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `url` | Sensitive identifier | `standard` | Local-only incident receiver endpoint. |

## ZIA Dlp Notification Templates

Commands:

```sh
zscalerctl zia dlp-notification-templates list
zscalerctl zia dlp-notification-templates get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | DLP notification template identifier. |
| `name`, `tlsEnabled` | Tenant configuration | `standard`, `share` | Template name and TLS flag. |
| `subject` | Free text | `standard` | Notification subject; scanned before output, including bare high-entropy tokens. |
| `attachContent` | Tenant configuration | `standard` | Local-only attachment behavior because it can indicate whether violating content is emailed. |
| `plainTextMessage`, `htmlMessage` | Secret | never | Notification bodies may contain incident context, placeholders, or internal response instructions and are dropped. |

## ZIA C2c Incident Receivers

Commands:

```sh
zscalerctl zia c2c-incident-receivers list
zscalerctl zia c2c-incident-receivers get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `status`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | Receiver identity, status, and modification timestamp. |
| `lastTenantValidationTime` | Operational metadata | `standard` | Local-only validation timestamp. |
| `name` | Tenant configuration | `standard`, `share` | Receiver name; scanned for pasted secret-shaped values. |
| `onboardableEntity` | Tenant configuration | `standard` | Local-only cloud/onboarded entity reference; authorization details inside the nested SDK shape are dropped. |
| `lastValidationMsg`, `lastModifiedBy` | Secret | never | Validation messages and admin identity are dropped. |

## ZIA Risk Profiles

Commands:

```sh
zscalerctl zia risk-profiles list
zscalerctl zia risk-profiles get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `profileType`, `status`, `createTime`, `lastModTime` | Operational metadata | `standard`, `share`, `paranoid` | Risk-profile identity, type, state, and timestamp metadata. |
| `profileName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `customTags`, `modifiedBy`, `sourceIpRestrictions` | Sensitive identifier / tenant configuration | `standard` | Local-only tag, admin, and source-restriction references. |
| `adminAuditLogs`, `certifications`, `dataBreach`, `dataEncryptionInTransit`, `dnsCaaPolicy`, `domainBasedMessageAuth`, `domainKeysIdentifiedMail`, `evasive`, `excludeCertificates`, `fileSharing`, `httpSecurityHeaders`, `malwareScanningForContent`, `mfaSupport`, `passwordStrength`, `poorItemsOfService`, `remoteScreenSharing`, `riskIndex`, `senderPolicyFramework`, `sslCertKeySize`, `sslCertValidity`, `sslPinned`, `supportForWaf`, `vulnerability`, `vulnerabilityDisclosure`, `vulnerableToHeartBleed`, `vulnerableToLogJam`, `vulnerableToPoodle`, `weakCipherSupport` | Secret | never | Security posture detail is mapped into source records and dropped by projection until separately modeled. |

## ZIA Nss Servers

Commands:

```sh
zscalerctl zia nss-servers list
zscalerctl zia nss-servers get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `status`, `state`, `type` | Operational metadata | `standard`, `share`, `paranoid` | NSS server identity, state, and type metadata. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `icapSvrId` | Sensitive identifier | `standard` | Local-only ICAP server reference. |

## ZIA Nss Feeds

Commands:

```sh
zscalerctl zia nss-feeds list
zscalerctl zia nss-feeds get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `feedStatus`, `nssLogType`, `nssFeedType`, `epsRateLimit`, `jsonArrayToggle`, `maxBatchSize`, `lastSuccessFullTest`, `testConnectivityCode`, `nssType`, `cloudNss`, `oauthAuthentication` | Operational metadata | varies | Feed identity, status, type, limits, test, and OAuth-state metadata. |
| `name`, `feedOutputFormat`, `userObfuscation`, `timeZone`, `siemType`, `grantType`, `firewallLoggingMode`, `actionFilter`, `emailDlpPolicyAction`, `direction`, `event`, and reviewed enum/filter lists | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| Network, host, URL, file, project, repository, channel, user-agent, and tunnel fields | Sensitive identifier | `standard` | Local-only feed targeting and logging metadata. |
| Reference fields such as `urlCategories`, `dlpEngines`, `dlpDictionaries`, `rules`, and `nwServices` | Tenant configuration | `standard` | Local-only references render reviewed `id`/`name` fields only. |
| Authentication fields, connection headers, certificates, VPN credentials, collaborator/user/location references, and high-risk object/filter fields | Secret | never | Mapped into source records and dropped by projection until separately modeled. |

## ZIA File Type Rules

Commands:

```sh
zscalerctl zia file-type-rules list
zscalerctl zia file-type-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `state`, `order`, `rank`, `timeQuota`, `sizeQuota`, `accessControl`, `capturePCAP`, `activeContent`, `unscannable`, `minSize`, `maxSize`, `lastModifiedTime` | Operational metadata | varies | Rule identity, ordering, quota, and status metadata. |
| `name`, `filteringAction`, `passwordProtected`, `operation`, `cloudApplications`, `fileTypes`, `protocols`, `urlCategories`, `deviceTrustLevels` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `browserEunTemplateId`, scope references such as `locations`, `groups`, and `devices` | Sensitive identifier / tenant configuration | `standard` | Local-only rule target and notification references. |
| `lastModifiedBy` | Secret | never | Admin reference is mapped into source records but dropped by projection. |

## ZIA Sandbox Rules

Commands:

```sh
zscalerctl zia sandbox-rules list
zscalerctl zia sandbox-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `state`, `order`, `rank`, `firstTimeEnable`, `mlActionEnabled`, `byThreatScore`, `accessControl`, `lastModifiedTime`, `defaultRule` | Operational metadata | varies | Rule identity, ordering, status, and behavior metadata. |
| `name`, `baRuleAction`, `firstTimeOperation`, `protocols`, `baPolicyCategories`, `fileTypes`, `urlCategories` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| Scope references such as `locations`, `groups`, `devices`, and `zpaAppSegments` | Tenant configuration | `standard` | Local-only rule target references render reviewed `id`/`name` fields only. |
| `lastModifiedBy` | Secret | never | Admin reference is mapped into source records but dropped by projection. |

## ZIA Firewall Dns Rules

Commands:

```sh
zscalerctl zia firewall-dns-rules list
zscalerctl zia firewall-dns-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `order`, `rank`, `accessControl`, `state`, `lastModifiedTime`, `defaultRule`, `capturePCAP`, `predefined`, `isWebEunEnabled`, `defaultDnsRuleNameUsed` | Operational metadata | varies | Rule identity, ordering, status, and predefined/default metadata. |
| `name`, `action`, `blockResponseCode`, `applications`, `dnsRuleRequestTypes`, `protocols` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `redirectIp`, `srcIps`, `destAddresses`, `destIpCategories`, `resCategories` | Sensitive identifier | `standard` | Local-only DNS and network targeting data. |
| Scope references such as `locations`, `groups`, `dnsGateway`, and IP groups | Tenant configuration | `standard` | Local-only rule target references render reviewed `id`/`name` fields only. |
| `lastModifiedBy` | Secret | never | Admin reference is mapped into source records but dropped by projection. |

## ZIA Custom File Types

Commands:

```sh
zscalerctl zia custom-file-types list
zscalerctl zia custom-file-types get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `fileTypeId` | Operational metadata | `standard`, `share`, `paranoid` | Custom file type identifiers. |
| `name`, `extension` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |

## ZIA Zpa Gateways

Commands:

```sh
zscalerctl zia zpa-gateways list
zscalerctl zia zpa-gateways get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `lastModifiedTime`, `type` | Operational metadata | varies | Gateway identity, modified timestamp, and gateway type. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `zpaTenantId`, `zpaServerGroup`, `zpaAppSegments` | Sensitive identifier / tenant configuration | `standard` | Local-only ZPA tenant and reference metadata render reviewed `id`/`name` fields only. |
| `lastModifiedBy` | Secret | never | Admin reference is mapped into source records but dropped by projection. |

## ZIA Browser Isolation Profiles

Commands:

```sh
zscalerctl zia browser-isolation-profiles list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `url` | Sensitive identifier | `standard` | Local-only browser-isolation profile identifier and URL. |
| `name` | Tenant configuration | `standard`, `share` | Profile name; scanned for pasted secret-shaped values. |
| `defaultProfile` | Operational metadata | `standard`, `share`, `paranoid` | Whether Zscaler marks the profile as default. |

This is a list-only resource because the SDK exposes list and name lookup, but
no integer ID `get` path.

## ZIA Dlp Edm Schemas Lite

Commands:

```sh
zscalerctl zia dlp-edm-schemas-lite list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `schema` | Tenant configuration | `standard` | EDM schema reference; `id`, `name`, and `externalId` render only in standard mode. |
| `tokenList` | Secret | never | Token/column criteria are mapped into source records and dropped by projection. |

This is a list-only lite view of EDM schemas. Full EDM schema metadata remains
available through `zia/dlp-edm-schemas`.

## ZIA Dc Exclusions

Commands:

```sh
zscalerctl zia dc-exclusions list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `dcid`, `expired` | Operational metadata | `standard`, `share`, `paranoid` | Data-center exclusion identity and expiry state. |
| `startTime`, `endTime` | Operational metadata | `standard`, `share` | Exclusion window timestamps. |
| `description` | Free text | `standard` | Admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `dcName` | Tenant configuration | `standard` | Data-center reference rendered as reviewed `id`/`name` fields only. |

This is a list-only resource because the SDK exposes list and name lookup, but
no integer ID `get` path.

## ZIA Sub Clouds

Commands:

```sh
zscalerctl zia sub-clouds list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Sub-cloud identifier. |
| `name` | Tenant configuration | `standard`, `share` | Sub-cloud name; scanned for pasted secret-shaped values. |
| `dcs` | Tenant configuration | `standard` | Data-center topology references render only in standard mode. |
| `exclusions` | Tenant configuration | `standard` | Exclusion metadata renders only in standard mode; nested admin identity is dropped. |

This is list-only because the SDK's integer `Get` path answers a different
"last DC in country" question rather than returning the listed sub-cloud record.

## ZIA Ipv6 Config

Commands:

```sh
zscalerctl zia ipv6-config show
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `ipV6Enabled` | Operational metadata | `standard`, `share`, `paranoid` | IPv6 enabled-state flag. |
| `natPrefixes` | Tenant configuration | `standard` | NAT64 prefix references render only in standard mode. |
| `dnsPrefix` | Sensitive identifier | `standard` | DNS64 prefix value; dropped from `share` and `paranoid`. |

This is a singleton settings object exposed through `show`.

## ZIA Ipv6 Dns64 Prefixes

Commands:

```sh
zscalerctl zia ipv6-dns64-prefixes list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Prefix identifier. |
| `name` | Tenant configuration | `standard`, `share` | Prefix name; scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `prefixMask` | Sensitive identifier | `standard` | Prefix value; dropped from `share` and `paranoid`. |
| `dnsPrefix`, `nonEditable` | Operational metadata | `standard`, `share`, `paranoid` | Prefix flags returned by the SDK. |

This is a list-only prefix catalog.

## ZIA Ipv6 Nat64 Prefixes

Commands:

```sh
zscalerctl zia ipv6-nat64-prefixes list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Prefix identifier. |
| `name` | Tenant configuration | `standard`, `share` | Prefix name; scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `prefixMask` | Sensitive identifier | `standard` | Prefix value; dropped from `share` and `paranoid`. |
| `dnsPrefix`, `nonEditable` | Operational metadata | `standard`, `share`, `paranoid` | Prefix flags returned by the SDK. |

This is a list-only prefix catalog.

## ZIA Pac Files

Commands:

```sh
zscalerctl zia pac-files list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `editable`, `pacUrlObfuscated`, `pacVersion` | Operational metadata | `standard`, `share`, `paranoid` | PAC identity, editability, and version metadata. |
| `totalHits`, `lastModificationTime`, `createTime` | Operational metadata | `standard`, `share` | PAC usage and lifecycle metadata. |
| `name`, `pacVersionStatus`, `pacVerificationStatus` | Tenant configuration / operational metadata | `standard`, `share` | PAC name and state. |
| `description`, `pacContent`, `pacCommitMessage` | Free text | `standard` | Admin-controlled text and PAC script content; scanned before output and dropped from shared modes. |
| `domain`, `pacUrl`, `pacSubURL` | Sensitive identifier | `standard` | Tenant domain and PAC URL values. |
| `lastModifiedBy` | Secret | never | Admin identity is mapped into source records but dropped by projection. |

This is list-only because the SDK exposes list and name lookup for PAC files,
but no base integer ID `get` path.

## ZIA Cloud App Control

Commands:

```sh
zscalerctl zia cloud-app-control list
zscalerctl dump --products zia --resources zia/cloud-app-control --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Cloud App Control rule identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `state`, `rank`, `type`, `order`, `cascadingEnabled`, `numberOfApplications`, `eunEnabled`, `eunTemplateId`, `browserEunTemplateId`, `predefined`, `enforceTimeValidity` | Operational metadata | `standard`, `share`, `paranoid` | Rule state, type, ordering, and feature flags. |
| `accessControl`, `validityStartTime`, `validityEndTime`, `validityTimeZoneId`, `lastModifiedTime` | Operational metadata | `standard`, `share` | Non-principal rule metadata. |
| `actions`, `applications`, `userAgentTypes`, `deviceTrustLevels`, `userRiskScoreLevels`, `timeQuota`, `sizeQuota` | Tenant configuration | `standard`, `share` | Reviewed rule action, application, and quota criteria. |
| `labels`, `timeWindows` | Tenant configuration | `standard`, `share` | Nested references render reviewed `id`/`name` fields only. |
| `locations`, `locationGroups`, `tenancyProfileIds` | Tenant configuration | `standard` | Local-only scope and tenancy references. Nested unreviewed fields are dropped. |

Cloud App Control has no flat list endpoint; the reader enumerates the rule
types and concatenates each type's rules. Only `list` is supported (per-rule
`get` requires a rule-type and id pair).

The SDK also returns user, group, department, and device references plus nested
cloud-app-instance, risk-profile, and isolation-profile structures. The reader
keeps those out of rendered output until they are separately modeled.

## ZIA Cloud Application Policy

Commands:

```sh
zscalerctl zia cloud-application-policy list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `app`, `appName`, `parent`, `parentName` | Tenant configuration | `standard`, `share` | Cloud application and category identifiers returned by the policy catalog. |

This is a list-only catalog resource.

## ZIA Cloud Application Ssl Policy

Commands:

```sh
zscalerctl zia cloud-application-ssl-policy list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `app`, `appName`, `parent`, `parentName` | Tenant configuration | `standard`, `share` | Cloud application and category identifiers returned by the SSL policy catalog. |

This is a list-only catalog resource.

## ZIA Domain Profiles

Commands:

```sh
zscalerctl zia domain-profiles list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `profileId`, `includeCompanyDomains`, `includeSubdomains` | Operational metadata | `standard`, `share`, `paranoid` | Domain-profile identity and flags. |
| `profileName` | Tenant configuration | `standard`, `share` | Domain-profile name; scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Admin-controlled text; scanned before output. |
| `customDomains` | Sensitive identifier | `standard` | Tenant-defined domains. |
| `predefinedEmailDomains` | Tenant configuration | `standard` | Provider domain catalog references. |

This is a list-only SaaS Security API resource.

## ZIA Casb Tombstone Templates

Commands:

```sh
zscalerctl zia casb-tombstone-templates list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Template identifier. |
| `name` | Tenant configuration | `standard`, `share` | Template name; scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Tombstone text; scanned before output and dropped from shared modes. |

This is a list-only SaaS Security API resource.

## ZIA Casb Email Labels

Commands:

```sh
zscalerctl zia casb-email-labels list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `labelDeleted` | Operational metadata | `standard`, `share`, `paranoid` | Label identity and deletion flag. |
| `name`, `labelColor` | Tenant configuration | `standard`, `share` | Label name and color. |
| `labelDesc` | Free text | `standard` | Label description; scanned before output. |

This is a list-only SaaS Security API resource.

## ZIA Casb Tenants

Commands:

```sh
zscalerctl zia casb-tenants list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `tenantId`, `tenantDeleted`, `tenantWebhookEnabled`, `reAuth` | Operational metadata | `standard`, `share`, `paranoid` | Tenant identity and state flags. |
| `modifiedTime` | Operational metadata | `standard`, `share` | Tenant lifecycle metadata. |
| `lastTenantValidationTime`, `zscalerAppTenantId` | Operational metadata / tenant configuration | `standard` | Tenant validation and app-tenant reference details. |
| `featuresSupported`, `status`, `saasApplication` | Tenant configuration | `standard`, `share` | SaaS tenant feature and status metadata. |
| `enterpriseTenantId`, `tenantName` | Sensitive identifier | `standard` | External tenant identifiers and names. |

This is a list-only SaaS Security API resource.

## ZIA Casb Dlp Rules

Commands:

```sh
zscalerctl zia casb-dlp-rules list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `order`, `rank`, `lastModifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | Rule identity, order, rank, and lifecycle metadata. |
| `type`, `name`, `state`, `action`, `severity`, `accessControl`, `fileTypes`, `collaborationScope`, `components`, `deviceTrustLevels`, `watermarkDeleteOldVersion`, `includeCriteriaDomainProfile`, `includeEmailRecipientProfile`, `withoutContentInspection`, `includeEntityGroups`, `numberOfInternalCollaborators`, `numberOfExternalCollaborators` | Tenant configuration | `standard`, `share` | Rule configuration and matching flags. |
| `description` | Free text | `standard` | Admin-controlled rule text; scanned before output. |
| `bucketOwner`, `externalAuditorEmail`, `contentLocation`, `recipient`, `quarantineLocation`, `domains` | Sensitive identifier | `standard` | Tenant, recipient, domain, and storage-location identifiers. |
| `objectTypes`, `buckets`, `includedDomainProfiles`, `excludedDomainProfiles`, `criteriaDomainProfiles`, `emailRecipientProfiles`, `devices`, `deviceGroups`, `entityGroups`, `cloudAppTenants`, `users`, `groups`, `departments`, `dlpEngines`, `auditor`, `zscalerIncidentReceiver`, `auditorNotification`, `tag`, `watermarkProfile`, `redactionProfile`, `casbEmailLabel`, `casbTombstoneTemplate`, `receiver` | Tenant configuration | `standard` | Nested references render constrained `id`/`name` fields only. |
| `labels` | Tenant configuration | `standard`, `share` | Rule labels. |
| `lastModifiedBy` | Secret | never | Admin identity is mapped into source records but dropped by projection. |

This is list-only because the SDK's ID `get` path requires a rule type in
addition to the rule ID.

## ZIA Casb Malware Rules

Commands:

```sh
zscalerctl zia casb-malware-rules list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `order` | Operational metadata | `standard`, `share`, `paranoid` | Rule identity and order. |
| `lastModifiedTime` | Operational metadata | `standard`, `share` | Rule lifecycle metadata. |
| `type`, `name`, `state`, `action`, `accessControl` | Tenant configuration | `standard`, `share` | Rule configuration and state. |
| `quarantineLocation`, `scanInboundEmailLink` | Sensitive identifier | `standard` | Tenant storage and email-link identifiers. |
| `casbEmailLabel`, `casbTombstoneTemplate`, `buckets`, `cloudAppTenantIds`, `cloudAppTenants`, `cloudApplicationTenant` | Tenant configuration | `standard` | Nested CASB references render constrained fields only. |
| `labels` | Tenant configuration | `standard`, `share` | Rule labels. |
| `lastModifiedBy` | Secret | never | Admin identity is mapped into source records but dropped by projection. |

This is list-only because the SDK's ID `get` path requires a rule type in
addition to the rule ID.

## ZIA Browser Control Settings

Commands:

```sh
zscalerctl zia browser-control-settings show
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `pluginCheckFrequency`, `bypassAllBrowsers`, `allowAllBrowsers`, `enableWarnings`, `enableSmartBrowserIsolation` | Tenant configuration | `standard`, `share` | Browser warning and Smart Isolation controls. |
| `bypassPlugins`, `bypassApplications`, `blockedInternetExplorerVersions`, `blockedChromeVersions`, `blockedFirefoxVersions`, `blockedSafariVersions`, `blockedOperaVersions` | Sensitive identifier | `standard` | Application/plugin and browser-version targeting. |
| `smartIsolationProfileId`, `smartIsolationUsers`, `smartIsolationGroups`, `smartIsolationProfile` | Tenant configuration | `standard` | Smart Isolation references render constrained fields only. |

This is a singleton settings resource.

## ZIA Supported Browser Versions

Commands:

```sh
zscalerctl zia supported-browser-versions list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `browserType`, `versions`, `olderVersions` | Tenant configuration | `standard`, `share` | Browser support catalog values returned by the Browser Control API. |

This is a list-only browser-support catalog.

## ZIA Ftp Control Policy

Commands:

```sh
zscalerctl zia ftp-control-policy show
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `ftpOverHttpEnabled`, `ftpEnabled`, `urlCategories` | Tenant configuration | `standard`, `share` | FTP enablement and category controls. |
| `urls` | Sensitive identifier | `standard` | Tenant-defined FTP URL entries. |

This is a singleton settings resource.

## ZIA Remote Assistance

Commands:

```sh
zscalerctl zia remote-assistance show
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `viewOnlyUntil`, `fullAccessUntil` | Operational metadata | `standard`, `share` | Remote-assistance access timestamps. |
| `usernameObfuscated`, `deviceInfoObfuscate` | Tenant configuration | `standard`, `share` | Dashboard obfuscation controls. |

This is a singleton settings resource.

## ZIA Admin Users

Commands:

```sh
zscalerctl zia admin-users list
zscalerctl zia admin-users get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `disabled`, `isNonEditable`, `isAuditor` | Operational metadata | `standard`, `share`, `paranoid` | Admin identity and state flags. |
| `pwdLastModifiedTime` | Operational metadata | `standard` | Password lifecycle timestamp. |
| `loginName`, `userName`, `email` | Sensitive identifier | `standard` | Admin person identifiers. |
| `comments` | Free text | `standard` | Admin-controlled notes; scanned before output. |
| `isSecurityReportCommEnabled`, `isServiceUpdateCommEnabled`, `isProductUpdateCommEnabled`, `isExecMobileAppEnabled`, `adminScopeType` | Tenant configuration | `standard`, `share` | Admin communication, app-access, and scope-type controls. |
| `adminScopescopeGroupMemberEntities`, `adminScopeScopeEntities`, `role` | Tenant configuration | `standard` | Admin scope and role references render constrained fields only. |
| `password`, `isPasswordLoginAllowed`, `isPasswordExpired`, `execMobileAppTokens` | Secret | never | Password, password-state, and token material are mapped into source records but dropped by projection. |

## ZIA Admin Roles

Commands:

```sh
zscalerctl zia admin-roles list
zscalerctl zia admin-roles get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `isAuditor`, `isNonEditable` | Operational metadata | `standard`, `share`, `paranoid` | Role identity and state flags. |
| `rank` | Operational metadata | `standard` | Admin-rank metadata. |
| `name`, `roleType` | Tenant configuration | `standard`, `share` | Role name and type. |
| `policyAccess`, `alertingAccess`, `dashboardAccess`, `reportAccess`, `analysisAccess`, `usernameAccess`, `adminAcctAccess`, `deviceInfoAccess`, `permissions`, `logsLimit`, `reportTimeDuration` | Tenant configuration | `standard` | Detailed authorization and log-access inventory. |
| `featurePermissions`, `extFeaturePermissions` | Secret | never | Arbitrary permission maps are dropped until modeled with a stable shape. |

## ZIA Email Profiles

Commands:

```sh
zscalerctl zia email-profiles list
zscalerctl zia email-profiles get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Email recipient profile identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `emails` | Sensitive identifier | `standard` | Local-only recipient addresses. |

## ZIA Advanced Settings

Commands:

```sh
zscalerctl zia advanced-settings show
```

This singleton settings page renders reviewed global ZIA advanced-setting
toggles and keeps bypass app, URL, category, and ECS object details local-only
or dropped.

## ZIA Advanced Threat Settings

Commands:

```sh
zscalerctl zia advanced-threat-settings show
```

This singleton settings page renders reviewed advanced-threat policy toggles.
Country blocks render only in `standard`; secret-looking boolean control names
carry explicit catalog reasons.

## ZIA Mobile Threat Settings

Commands:

```sh
zscalerctl zia mobile-threat-settings show
```

This singleton settings page renders reviewed mobile-threat protection toggles,
including a credential-exfiltration control modeled as boolean metadata rather
than credential material.

## ZIA Sandbox Settings

Commands:

```sh
zscalerctl zia sandbox-settings show
```

This singleton settings page renders reviewed sandbox settings. File hashes to
block are sensitive identifiers and render only in `standard`.

## ZIA End User Notification Settings

Commands:

```sh
zscalerctl zia end-user-notification-settings show
```

This singleton settings page renders reviewed notification toggles and public
display metadata. Custom message bodies and policy-review text are dropped as
free-form tenant content.

## ZIA Org Information

Commands:

```sh
zscalerctl zia org-information show
```

This singleton settings page renders narrow organization metadata. Addresses,
domains, and tenant identifiers render only in `standard`; contact details and
logo payloads are dropped.

## ZIA Atp Malware Policy

Commands:

```sh
zscalerctl zia atp-malware-policy show
```

This singleton settings page renders reviewed ATP malware policy toggles. The
password-protected archive control is modeled as boolean metadata, not password
material.

## ZIA Atp Malware Settings

Commands:

```sh
zscalerctl zia atp-malware-settings show
```

This singleton settings page renders reviewed ATP malware family block/capture
toggles.

## ZIA Atp Malware Inspection

Commands:

```sh
zscalerctl zia atp-malware-inspection show
```

This singleton settings page renders reviewed inbound and outbound malware
inspection toggles.

## ZIA Atp Malware Protocols

Commands:

```sh
zscalerctl zia atp-malware-protocols show
```

This singleton settings page renders reviewed protocol inspection toggles.

## ZIA Malicious Urls

Commands:

```sh
zscalerctl zia malicious-urls show
```

This singleton settings page renders the tenant malicious-URL list only in
`standard`.

## ZIA Security Exceptions

Commands:

```sh
zscalerctl zia security-exceptions show
```

This singleton settings page renders the tenant security-exception URL list
only in `standard`.

## ZIA Url Allow List

Commands:

```sh
zscalerctl zia url-allow-list show
```

This singleton settings page renders the global URL allow list only in
`standard`. The deny-list field is explicitly modeled and dropped.

## ZIA Url Deny List

Commands:

```sh
zscalerctl zia url-deny-list show
```

This singleton settings page renders the global URL deny list only in
`standard`. The allow-list field is explicitly modeled and dropped.

## ZPA Inspection Profiles

Commands:

```sh
zscalerctl zpa inspection-profiles list
zscalerctl zpa inspection-profiles get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Inspection profile identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `apiProfile`, `creationTime`, `zsDefinedControlChoice`, `globalControlActions`, `incarnationNumber`, `modifiedBy`, `modifiedTime`, `paranoiaLevel`, `predefinedControlsVersion`, `checkControlDeploymentStatus` | Operational metadata | `standard`, `share`, `paranoid` | Profile metadata and global control settings. |
| `overrideAction` | Tenant configuration | `standard`, `share` | Global override action. |

The SDK also returns nested control collections (`commonGlobalOverrideActionsConfig`, `controlsInfo`, `customControls`, `predefinedApiControls`, `predefinedControls`, `websocketControls`, `threatlabzControls`); the catalog keeps those out of rendered output until separately modeled.

## ZPA Inspection Custom Controls

Commands:

```sh
zscalerctl zpa inspection-custom-controls list
zscalerctl zpa inspection-custom-controls get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Custom control identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `controlNumber`, `controlType`, `creationTime`, `modifiedBy`, `modifiedTime`, `paranoiaLevel`, `protocolType`, `severity`, `type`, `version` | Operational metadata | `standard`, `share`, `paranoid` | Control metadata. |
| `action`, `actionValue`, `defaultAction`, `defaultActionValue`, `associatedInspectionProfileNames` | Tenant configuration | `standard`, `share` | Control action and profile-association configuration. |

The SDK also returns the control rule definition (`controlRuleJson`, `rules`); the catalog keeps that detection logic out of rendered output until separately modeled.

## ZPA Inspection Predefined Controls

Commands:

```sh
zscalerctl zpa inspection-predefined-controls list
zscalerctl zpa inspection-predefined-controls get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Predefined control identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `attachment`, `controlGroup`, `controlType`, `controlNumber`, `creationTime`, `modifiedBy`, `modifiedTime`, `paranoiaLevel`, `protocolType`, `severity`, `version` | Operational metadata | `standard`, `share`, `paranoid` | Predefined control metadata. |
| `action`, `actionValue`, `defaultAction`, `defaultActionValue`, `associatedInspectionProfileNames` | Tenant configuration | `standard`, `share` | Control action and profile-association configuration. |

## ZPA Tag Groups

Commands:

```sh
zscalerctl zpa tag-groups list
zscalerctl zpa tag-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Tag group identifier. |
| `name`, `microtenantName` | Tenant configuration | `standard`, `share` | Tag group name and microtenant name. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |

The SDK also returns the microtenant ID and nested tags (`microtenantId`, `tags`); the catalog keeps those out of rendered output as tenant-identifying or unmodeled.

## ZPA Tag Keys

Commands:

```sh
zscalerctl zpa tag-keys list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `origin`, `type`, `skipAudit` | Operational metadata | `standard`, `share`, `paranoid` | Tag key state and metadata. |
| `name`, `namespaceId`, `microtenantName` | Tenant configuration | `standard`, `share` | Tag key configuration references. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |

The SDK also returns the customer ID, microtenant ID, and nested tag values (`customerId`, `microtenantId`, `tagValues`); the catalog keeps those out of rendered output as tenant-identifying or unmodeled.

## ZPA Tag Namespaces

Commands:

```sh
zscalerctl zpa tag-namespaces list
zscalerctl zpa tag-namespaces get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `origin`, `type` | Operational metadata | `standard`, `share`, `paranoid` | Namespace state and metadata. |
| `name`, `microtenantName` | Tenant configuration | `standard`, `share` | Namespace name and microtenant name. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |

The SDK also returns the microtenant ID (`microtenantId`), kept out of rendered output as tenant-identifying.

## ZPA Version Profiles

Commands:

```sh
zscalerctl zpa version-profiles list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Version profile identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `creationTime`, `modifiedBy`, `modifiedTime`, `visibilityScope`, `numberOfAssistants`, `numberOfCustomers`, `numberOfPrivateBrokers`, `numberOfSiteControllers`, `numberOfUpdatedAssistants`, `numberOfUpdatedPrivateBrokers`, `numberOfUpdatedSiteControllers` | Operational metadata | `standard`, `share`, `paranoid` | Profile metadata and broker/assistant counts. |
| `upgradePriority` | Tenant configuration | `standard`, `share` | Upgrade priority setting. |

The SDK also returns the ZPA customer ID, custom-scope customer ID lists, and nested version details (`customerId`, `customScopeCustomerIds`, `customScopeRequestCustomerIds`, `versions`); the catalog keeps those out of rendered output as tenant-identifying or unmodeled.

## ZPA Client Types

Commands:

```sh
zscalerctl zpa client-types list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `zpn_client_type_exporter`, `zpn_client_type_exporter_noauth`, `zpn_client_type_browser_isolation`, `zpn_client_type_machine_tunnel`, `zpn_client_type_ip_anchoring`, `zpn_client_type_edge_connector`, `zpn_client_type_zapp`, `zpn_client_type_slogger`, `zpn_client_type_branch_connector`, `zpn_client_type_zapp_partner`, `zpn_client_type_vdi`, `zpn_client_type_zia_inspection` | Operational metadata | `standard`, `share`, `paranoid` | Client-type reference identifiers. |

## ZPA Platforms

Commands:

```sh
zscalerctl zpa platforms list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `linux`, `android`, `windows`, `ios`, `mac` | Operational metadata | `standard`, `share`, `paranoid` | Supported client platform reference values. |

## ZPA Microtenants

Commands:

```sh
zscalerctl zpa microtenants list
zscalerctl zpa microtenants get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Microtenant identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `enabled`, `privilegedApprovalsEnabled`, `operator`, `priority`, `creationTime`, `modifiedBy`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | Microtenant state and metadata. |
| `criteriaAttribute` | Tenant configuration | `standard`, `share` | Scoping criteria attribute. |
| `criteriaAttributeValues` | Sensitive identifier | `standard` | Local-only criteria values. |

The SDK also returns nested role and user structures (`roles`, `user`); the
catalog keeps those out of rendered output until they are separately modeled.

## ZPA Cbi Banners

Commands:

```sh
zscalerctl zpa cbi-banners list
zscalerctl zpa cbi-banners get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `banner`, `isDefault`, `persist` | Operational metadata | `standard`, `share`, `paranoid` | Banner identifier and toggles. |
| `name`, `primaryColor`, `textColor`, `notificationTitle`, `notificationText` | Tenant configuration | `standard`, `share` | Banner branding and notification text. |

The SDK also returns the banner logo blob (`logo`); the catalog keeps it out of rendered output as unmodeled embedded data.

## ZPA Cbi Profiles

Commands:

```sh
zscalerctl zpa cbi-profiles list
zscalerctl zpa cbi-profiles get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `isDefault`, `href`, `creationTime`, `modifiedBy`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | Profile state and metadata. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `cbiProfileId`, `cbiUrl` | Sensitive identifier | `standard` | Tenant-scoped isolation profile reference and URL. |

The SDK also returns the CBI tenant ID, banner reference, region/certificate ID lists, and nested control structures (`cbiTenantId`, `bannerId`, `securityControls`, `regions`, `regionIds`, `userExperience`, `certificates`, `certificateIds`, `banner`, `debugMode`); the catalog keeps those out of rendered output as tenant-identifying or unmodeled.

## ZPA Cbi Regions

Commands:

```sh
zscalerctl zpa cbi-regions list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Region identifier. |
| `name` | Tenant configuration | `standard`, `share` | Region name. |

## ZPA Isolation Profiles

Commands:

```sh
zscalerctl zpa isolation-profiles list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | Profile state and metadata. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `isolationProfileId`, `isolationUrl` | Sensitive identifier | `standard` | Tenant-scoped isolation profile reference and URL. |

The SDK also returns the isolation tenant ID (`isolationTenantId`), kept out of rendered output as tenant-identifying.

## ZPA Branch Connectors

Commands:

```sh
zscalerctl zpa branch-connectors list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | Connector state and metadata. |
| `name`, `branchConnectorGroupName`, `edgeConnectorGroupName` | Tenant configuration | `standard`, `share` | Connector and group names. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `ipAcl` | Sensitive identifier | `standard` | Connector IP ACL entries. |

The SDK also returns group IDs and enrollment material (`branchConnectorGroupId`, `edgeConnectorGroupId`, `enrollmentCert`, `fingerprint`, `issuedCertId`); the catalog keeps those out of rendered output as tenant-identifying or secret.

## ZPA Pra Approvals

Commands:

```sh
zscalerctl zpa pra-approvals list
zscalerctl zpa pra-approvals get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `creationTime`, `endTime`, `modifiedBy`, `modifiedTime`, `startTime`, `status` | Operational metadata | `standard`, `share`, `paranoid` | Approval window and state. |
| `microtenantName` | Tenant configuration | `standard`, `share` | Microtenant name. |
| `emailIds` | Sensitive identifier | `standard` | Approval recipient email addresses. |

The SDK also returns the microtenant ID, nested applications, and working-hours structure (`microtenantId`, `applications`, `workingHours`); the catalog keeps those out of rendered output as tenant-identifying or unmodeled.

## ZPA Pra Consoles

Commands:

```sh
zscalerctl zpa pra-consoles list
zscalerctl zpa pra-consoles get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | Console state and metadata. |
| `name`, `iconText`, `microtenantName` | Tenant configuration | `standard`, `share` | Console name, icon text, and microtenant name. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |

The SDK also returns the microtenant ID and nested application/portal references (`microtenantId`, `praApplication`, `praPortals`); the catalog keeps those out of rendered output as tenant-identifying or unmodeled.

## ZPA Pra Portals

Commands:

```sh
zscalerctl zpa pra-portals list
zscalerctl zpa pra-portals get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime`, `action`, `certManagedByZsRadio`, `getcName`, `hideInfoTooltip`, `isSRAPortal`, `managedByZs`, `objectType`, `restrictedEntity`, `userNotificationEnabled` | Operational metadata | `standard`, `share`, `paranoid` | Portal state and metadata. |
| `name`, `certificateName`, `extLabel`, `microtenantName`, `scopeName`, `userNotification`, `userPortalName` | Tenant configuration | `standard`, `share` | Portal naming and notification text. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `cName`, `domain`, `extDomain`, `extDomainName`, `extDomainTranslation` | Sensitive identifier | `standard` | Portal hostnames and domains. |

The SDK also returns the certificate ID, microtenant ID, user-portal GID, and nested reviewers (`certificateId`, `microtenantId`, `userPortalGid`, `approvalReviewers`); the catalog keeps those out of rendered output as tenant-identifying or unmodeled.

## ZPA User Portals

Commands:

```sh
zscalerctl zpa user-portals list
zscalerctl zpa user-portals get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime`, `getcName`, `managedByZs`, `userNotificationEnabled` | Operational metadata | `standard`, `share`, `paranoid` | Portal state and metadata. |
| `name`, `certificateName`, `extLabel`, `microtenantName`, `userNotification` | Tenant configuration | `standard`, `share` | Portal naming and notification text. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `domain`, `extDomain`, `extDomainName`, `extDomainTranslation` | Sensitive identifier | `standard` | Portal hostnames and domains. |

The SDK also returns the certificate ID, image blob, and microtenant ID (`certificateId`, `imageData`, `microtenantId`); the catalog keeps those out of rendered output as tenant-identifying, secret, or unmodeled embedded data.

## ZPA User Portal Aups

Commands:

```sh
zscalerctl zpa user-portal-aups list
zscalerctl zpa user-portal-aups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | AUP state and metadata. |
| `name`, `microtenantName` | Tenant configuration | `standard`, `share` | AUP name and microtenant name. |
| `description`, `aup` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `email`, `phoneNum` | Sensitive identifier | `standard` | AUP contact email and phone number. |

The SDK also returns the microtenant ID (`microtenantId`), kept out of rendered output as tenant-identifying.

## ZPA User Portal Links

Commands:

```sh
zscalerctl zpa user-portal-links list
zscalerctl zpa user-portal-links get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime`, `protocol` | Operational metadata | `standard`, `share`, `paranoid` | Link state and metadata. |
| `name`, `iconText`, `microtenantName`, `nameWithoutTrim` | Tenant configuration | `standard`, `share` | Link naming and microtenant name. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `link`, `linkPath` | Sensitive identifier | `standard` | Link target URL and path. |

The SDK also returns the application ID, microtenant ID, and nested portal references (`applicationId`, `microtenantId`, `userPortalId`, `userPortals`); the catalog keeps those out of rendered output as tenant-identifying or unmodeled.

## ZPA Browser Access

Commands:

```sh
zscalerctl zpa browser-access list
zscalerctl zpa browser-access get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `bypassType`, `bypassOnReauth`, `enabled`, `passiveHealthEnabled`, `fqdnDnsCheck`, `apiProtectionEnabled`, `selectConnectorCloseToApp`, `doubleEncrypt`, `healthCheckType`, `isCnameEnabled`, `ipAnchored`, `tcpKeepAlive`, `useInDrMode`, `inspectTrafficWithZia`, `healthReporting`, `icmpAccessType`, `creationTime`, `modifiedBy`, `modifiedTime`, `readOnly`, `restrictionType` | Operational metadata | `standard`, `share`, `paranoid` | Segment state and metadata. |
| `name`, `segmentGroupName`, `microtenantName` | Tenant configuration | `standard`, `share` | Segment, group, and microtenant names. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `domainNames`, `tcpPortRanges`, `udpPortRanges` | Sensitive identifier | `standard` | Application domains and port ranges. |

The SDK also returns group/microtenant IDs, DR config, port-range objects, and nested clientless/server-group structures (`segmentGroupId`, `microtenantId`, `appRecommendationId`, `matchStyle`, `configSpace`, `extranetEnabled`, `isIncompleteDRConfig`, `zscalerManaged`, `weightedLoadBalancing`, `tcpPortRange`, `udpPortRange`, `clientlessApps`, `serverGroups`, `sharedMicrotenantDetails`, `policyStyle`, `zpnErId`); the catalog keeps those out of rendered output as tenant-identifying or unmodeled.

## ZPA Inspection App Segments

Commands:

```sh
zscalerctl zpa inspection-app-segments list
zscalerctl zpa inspection-app-segments get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `bypassType`, `bypassOnReauth`, `enabled`, `adpEnabled`, `autoAppProtectEnabled`, `passiveHealthEnabled`, `fqdnDnsCheck`, `apiProtectionEnabled`, `selectConnectorCloseToApp`, `doubleEncrypt`, `healthCheckType`, `isCnameEnabled`, `ipAnchored`, `tcpKeepAlive`, `useInDrMode`, `healthReporting`, `icmpAccessType`, `creationTime`, `modifiedBy`, `modifiedTime`, `readOnly`, `restrictionType`, `tcpProtocols`, `udpProtocols` | Operational metadata | `standard`, `share`, `paranoid` | Segment state and metadata. |
| `name`, `segmentGroupName`, `microtenantName` | Tenant configuration | `standard`, `share` | Segment, group, and microtenant names. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `domainNames`, `tcpPortRanges`, `udpPortRanges` | Sensitive identifier | `standard` | Application domains and port ranges. |

The SDK also returns group/microtenant IDs, DR config, port-range objects, and nested inspection/app/server-group structures (`segmentGroupId`, `microtenantId`, `appRecommendationId`, `matchStyle`, `configSpace`, `extranetEnabled`, `isIncompleteDRConfig`, `zscalerManaged`, `weightedLoadBalancing`, `tcpPortRange`, `udpPortRange`, `inspectionApps`, `commonAppsDto`, `serverGroups`, `sharedMicrotenantDetails`, `policyStyle`); the catalog keeps those out of rendered output as tenant-identifying or unmodeled.

## ZPA Pra App Segments

Commands:

```sh
zscalerctl zpa pra-app-segments list
zscalerctl zpa pra-app-segments get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `bypassType`, `bypassOnReauth`, `enabled`, `passiveHealthEnabled`, `fqdnDnsCheck`, `apiProtectionEnabled`, `selectConnectorCloseToApp`, `doubleEncrypt`, `healthCheckType`, `isCnameEnabled`, `ipAnchored`, `tcpKeepAlive`, `useInDrMode`, `healthReporting`, `icmpAccessType`, `creationTime`, `modifiedBy`, `modifiedTime`, `readOnly`, `restrictionType` | Operational metadata | `standard`, `share`, `paranoid` | Segment state and metadata. |
| `name`, `segmentGroupName`, `microtenantName` | Tenant configuration | `standard`, `share` | Segment, group, and microtenant names. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `domainNames`, `tcpPortRanges`, `udpPortRanges` | Sensitive identifier | `standard` | Application domains and port ranges. |

The SDK also returns group/microtenant IDs, DR config, timeouts, port-range objects, and nested PRA/app/server-group structures (`segmentGroupId`, `microtenantId`, `appRecommendationId`, `matchStyle`, `configSpace`, `applications`, `extranetEnabled`, `isIncompleteDRConfig`, `zscalerManaged`, `weightedLoadBalancing`, `tcpPortRange`, `udpPortRange`, `defaultIdleTimeout`, `defaultMaxAge`, `praApps`, `commonAppsDto`, `serverGroups`, `sharedMicrotenantDetails`, `policyStyle`, `zpnErId`); the catalog keeps those out of rendered output as tenant-identifying or unmodeled.

## ZPA Server Groups

Commands:

```sh
zscalerctl zpa server-groups list
zscalerctl zpa server-groups get <id>
zscalerctl dump --products zpa --resources zpa/server-groups --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `ipAnchored`, `creationTime`, `modifiedTime`, `readOnly`, `restrictionType`, `zscalerManaged` | Operational metadata | `standard`, `share`, `paranoid` | Server group identity, state, lifecycle, and operational flags. |
| `name`, `microtenantName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `dynamicDiscovery` | Sensitive identifier | `standard` | Local-only dynamic discovery setting. |
| `configSpace`, `microtenantId`, `appConnectorGroups`, `servers`, `applications`, `extranetDTO` | Secret or unmodeled nested structure | none | Dropped until the nested ZPA reference shapes are separately reviewed. |

## ZPA Segment Groups

Commands:

```sh
zscalerctl zpa segment-groups list
zscalerctl zpa segment-groups get <id>
zscalerctl dump --products zpa --resources zpa/segment-groups --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedTime`, `policyMigrated`, `tcpKeepAliveEnabled` | Operational metadata | `standard`, `share`, `paranoid` | Segment group identity, state, lifecycle, and operational flags. |
| `name`, `microtenantName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `configSpace`, `microtenantId`, `addedApps`, `deletedApps`, `applications`, `applicationNames` | Secret or unmodeled nested structure | none | Dropped until application references are separately reviewed. |

## ZPA Application Segments

Commands:

```sh
zscalerctl zpa application-segments list
zscalerctl zpa application-segments get <id>
zscalerctl dump --products zpa --resources zpa/application-segments --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `bypassType`, `healthCheckType`, `icmpAccessType`, `healthReporting`, `passiveHealthEnabled`, `ipAnchored`, `fqdnDnsCheck`, `tcpKeepAlive`, `isCnameEnabled`, `selectConnectorCloseToApp`, `restrictionType`, `useInDrMode` | Operational metadata | `standard`, `share`, `paranoid` | Application segment identity and reviewed behavior flags. |
| `creationTime`, `modifiedTime` | Operational metadata | `standard`, `share` | Lifecycle timestamps. |
| `apiProtectionEnabled`, `inspectTrafficWithZia`, `doubleEncrypt`, `adpEnabled`, `autoAppProtectEnabled`, `bypassOnReauth` | Operational metadata | `standard` | Local-only posture flags. |
| `name`, `segmentGroupName`, `microtenantName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `domainNames`, `tcpPortRanges`, `udpPortRanges` | Sensitive identifier | `standard` | Local-only application matching surface and traffic ports. |
| `serverGroups` | Tenant configuration reference | `standard`, `share`, `paranoid` for `id`; `standard`, `share` for `name` | Renders reviewed `id`/`name` references only. Query `zpa/server-groups` and `zpa/app-servers` for group and server details. |
| `modifiedBy`, `segmentGroupId`, `microtenantId`, `appRecommendationId`, `applications`, `configSpace`, `matchStyle`, `policyStyle`, `isIncompleteDRConfig`, `defaultMaxAge`, `defaultIdleTimeout`, `readOnly`, `zscalerManaged`, `clientlessApps`, `sharedMicrotenantDetails`, `shareToMicrotenants`, `tags`, `zpnErId`, `tcpPortRange`, `udpPortRange`, `weightedLoadBalancing`, `extranetEnabled` | Secret or unmodeled nested structure | none | Dropped until admin identity, browser-access, tenant-sharing, tag, ER, structured-port, and weighted-load-balancing sub-shapes are separately reviewed. |

## ZPA App Connector Groups

Commands:

```sh
zscalerctl zpa app-connector-groups list
zscalerctl zpa app-connector-groups get <id>
zscalerctl dump --products zpa --resources zpa/app-connector-groups --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedTime`, `overrideVersionProfile`, `upgradeDay`, `upgradeTimeInSecs`, `versionProfileId`, `versionProfileName`, `versionProfileVisibilityScope`, `readOnly`, `praEnabled`, `wafDisabled`, `lssAppConnectorGroup`, `tcpConnectTimeout`, `useInDrMode`, `zscalerManaged` | Operational metadata | `standard`, `share`, `paranoid` | Connector group identity, lifecycle, version, and operational flags. |
| `name`, `location`, `dnsQueryType`, `microtenantName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `latitude`, `longitude`, `cityCountry`, `countryCode`, `geolocationId`, `siteId`, `microtenantId` | Sensitive identifier | `standard` | Local-only placement and tenant identifiers. |
| `appConnectorGroupCommonDTO`, `connectors`, `platform`, `serverTypes`, `assistantGroups`, `versionProfile` | Secret or unmodeled nested structure | none | Dropped until connector and platform sub-shapes are separately reviewed. |

## ZPA App Servers

Commands:

```sh
zscalerctl zpa app-servers list
zscalerctl zpa app-servers get <id>
zscalerctl dump --products zpa --resources zpa/app-servers --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | Application server identity, state, and lifecycle metadata. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `address` | Sensitive identifier | `standard` | Local-only server address. |
| `appServerGroupIds`, `configSpace`, `microtenantId` | Secret or unmodeled nested structure | none | Dropped until server group references and tenant scope are separately reviewed. |

## ZPA Machine Groups

Commands:

```sh
zscalerctl zpa machine-groups list
zscalerctl zpa machine-groups get <id>
zscalerctl dump --products zpa --resources zpa/machine-groups --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `creationTime`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | Machine group identity and lifecycle metadata. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `microtenantId`, `machines` | Secret or unmodeled nested structure | none | Dropped until machine membership and tenant scope are separately reviewed. |

## ZPA Trusted Networks

Commands:

```sh
zscalerctl zpa trusted-networks list
zscalerctl zpa trusted-networks get <id>
zscalerctl dump --products zpa --resources zpa/trusted-networks --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `creationTime`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | Trusted network identity and lifecycle metadata. |
| `name`, `domain` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `masterCustomerId`, `networkId`, `zscalerCloud` | Secret or unmodeled nested structure | none | Dropped until tenant and cloud identifiers are separately reviewed. |

## ZPA Service Edge Groups

Commands:

```sh
zscalerctl zpa service-edge-groups list
zscalerctl zpa service-edge-groups get <id>
zscalerctl dump --products zpa --resources zpa/service-edge-groups --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime`, `isPublic`, `objectType`, `overrideVersionProfile`, `readOnly`, `restrictedEntity`, `restrictionType`, `graceDistanceEnabled`, `exclusiveForBusinessContinuity`, `upgradeDay`, `upgradeTimeInSecs`, `useInDrMode` | Operational metadata | `standard`, `share`, `paranoid` | Service edge group identity, state, lifecycle, and operational flags. |
| `name`, `microtenantName`, `scopeName`, `siteName`, `versionProfileName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `cityCountry`, `countryCode`, `latitude`, `location`, `longitude` | Sensitive identifier | `standard` | Local-only placement metadata. |
| `altCloud`, `city`, `enrollmentCertId`, `geoLocationId`, `graceDistanceValue`, `graceDistanceValueUnit`, `microtenantId`, `nameWithoutTrim`, `serviceEdges`, `siteId`, `trustedNetworks`, `versionProfileId`, `versionProfileVisibilityScope`, `zscalerManaged` | Secret or unmodeled nested structure | none | Dropped until service edge, trusted network, certificate, and tenant sub-shapes are separately reviewed. |

## ZPA Service Edges

Commands:

```sh
zscalerctl zpa service-edges list
zscalerctl zpa service-edges get <id>
zscalerctl dump --products zpa --resources zpa/service-edges --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `applicationStartTime`, `controlChannelStatus`, `creationTime`, `expectedUpgradeTime`, `lastBrokerConnectTime`, `lastBrokerConnectTimeDuration`, `lastBrokerDisconnectTime`, `lastBrokerDisconnectTimeDuration`, `lastUpgradeTime`, `modifiedBy`, `modifiedTime`, `publishIpv6`, `runtimeOS`, `upgradeStatus` | Operational metadata | `standard`, `share`, `paranoid` | Service edge identity, state, lifecycle, connection, runtime, and upgrade metadata. |
| `name`, `ctrlBrokerName`, `currentVersion`, `expectedVersion`, `microtenantName`, `platform`, `platformDetail`, `previousVersion`, `sargeVersion`, `serviceEdgeGroupName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `ipAcl`, `latitude`, `listenIps`, `location`, `longitude`, `privateIp`, `publicIp`, `publishIps` | Sensitive identifier | `standard` | Local-only placement and network metadata. |
| `enrollmentCert`, `fingerprint`, `issuedCertId`, `microtenantId`, `privateBrokerVersion`, `provisioningKeyId`, `provisioningKeyName`, `serviceEdgeGroupId`, `upgradeAttempt` | Secret or unmodeled nested structure | none | Dropped until broker, certificate, provisioning, and tenant sub-shapes are separately reviewed. |

## ZPA Cloud Connector Groups

Commands:

```sh
zscalerctl zpa cloud-connector-groups list
zscalerctl zpa cloud-connector-groups get <id>
zscalerctl dump --products zpa --resources zpa/cloud-connector-groups --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime`, `znfGroupType` | Operational metadata | `standard`, `share`, `paranoid` | Cloud connector group identity, state, lifecycle, and operational type metadata. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `cloudConnectors`, `geoLocationId`, `ziaCloud`, `ziaOrgId` | Secret or unmodeled nested structure | none | Dropped until cloud connector, geolocation, cloud, and tenant sub-shapes are separately reviewed. |

## ZPA Cloud Connectors

Commands:

```sh
zscalerctl zpa cloud-connectors list
zscalerctl dump --products zpa --resources zpa/cloud-connectors --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | Cloud connector identity, state, and lifecycle metadata. |
| `name`, `edgeConnectorGroupName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `ipAcl` | Sensitive identifier | `standard` | Local-only cloud connector ACL metadata. |
| `edgeConnectorGroupId`, `enrollmentCert`, `fingerprint`, `issuedCertId` | Secret or unmodeled nested structure | none | Dropped until group, certificate, and fingerprint sub-shapes are separately reviewed. |

## ZPA Posture Profiles

Commands:

```sh
zscalerctl zpa posture-profiles list
zscalerctl zpa posture-profiles get <id>
zscalerctl dump --products zpa --resources zpa/posture-profiles --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `applyToMachineTunnelEnabled`, `creationTime`, `crlCheckEnabled`, `modifiedBy`, `modifiedTime`, `postureType` | Operational metadata | `standard`, `share`, `paranoid` | Posture profile identity, lifecycle, posture type, and certificate-check flags. |
| `name`, `platform` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `domain` | Sensitive identifier | `standard` | Local-only posture domain metadata. |
| `masterCustomerId`, `nonExportablePrivateKeyEnabled`, `postureUdid`, `rootCert`, `zscalerCloud`, `zscalerCustomerId` | Secret or unmodeled nested structure | none | Dropped until certificate, tenant, cloud, and device-posture identifiers are separately reviewed. |

## ZPA Cbi Zpa Profiles

Commands:

```sh
zscalerctl zpa cbi-zpa-profiles list
zscalerctl zpa cbi-zpa-profiles get <id>
zscalerctl dump --products zpa --resources zpa/cbi-zpa-profiles --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime` | Operational metadata | `standard`, `share`, `paranoid` | CBI ZPA profile identity, state, and lifecycle metadata. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `cbiProfileId`, `cbiUrl` | Sensitive identifier | `standard` | Local-only CBI profile and tenant URL metadata. |
| `cbiTenantId` | Secret or unmodeled nested structure | none | Dropped until tenant identifiers are separately reviewed. |

## ZPA App Connectors

Commands:

```sh
zscalerctl zpa app-connectors list
zscalerctl zpa app-connectors get <id>
zscalerctl dump --products zpa --resources zpa/app-connectors --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `applicationStartTime`, `controlChannelStatus`, `creationTime`, `expectedUpgradeTime`, `lastBrokerConnectTime`, `lastBrokerConnectTimeDuration`, `lastBrokerDisconnectTime`, `lastBrokerDisconnectTimeDuration`, `lastUpgradeTime`, `modifiedBy`, `modifiedTime`, `readOnly`, `restrictionType`, `runtimeOS`, `upgradeStatus` | Operational metadata | `standard`, `share`, `paranoid` | Connector identity, lifecycle, control-channel, runtime, and upgrade metadata. |
| `name`, `appConnectorGroupName`, `ctrlBrokerName`, `currentVersion`, `expectedVersion`, `microtenantName`, `platform`, `platformDetail`, `previousVersion`, `sargeVersion` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `ipAcl`, `latitude`, `location`, `longitude`, `privateIp`, `publicIp` | Sensitive identifier | `standard` | Local-only connector placement and network metadata. |
| `appConnectorGroupId`, `assistantVersion`, `enrollmentCert`, `fingerprint`, `issuedCertId`, `microtenantId`, `provisioningKeyId`, `provisioningKeyName`, `upgradeAttempt`, `zpnSubModuleUpgradeList`, `zscalerManaged` | Secret or unmodeled nested structure | none | Dropped until connector-group IDs, assistant-version details, certificate, provisioning, tenant, upgrade-attempt, module, and management-scope fields are separately reviewed. |

## ZPA C2c IP Ranges

Commands:

```sh
zscalerctl zpa c2c-ip-ranges list
zscalerctl zpa c2c-ip-ranges get <id>
zscalerctl dump --products zpa --resources zpa/c2c-ip-ranges --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `enabled`, `creationTime`, `modifiedBy`, `modifiedTime`, `availableIps`, `totalIps`, `usedIps`, `isDeleted`, `sccmFlag` | Operational metadata | `standard`, `share`, `paranoid` | C2C IP range identity, lifecycle, state, and utilization metadata. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `countryCode`, `ipRangeBegin`, `ipRangeEnd`, `latitudeInDb`, `location`, `locationHint`, `longitudeInDb`, `subnetCidr` | Sensitive identifier | `standard` | Local-only network range and placement metadata. |
| `customerId` | Secret or unmodeled nested structure | none | Dropped until tenant identifiers are separately reviewed. |

## ZPA Config Overrides

Commands:

```sh
zscalerctl zpa config-overrides list
zscalerctl dump --products zpa --resources zpa/config-overrides --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `targetType` | Operational metadata | `standard`, `share`, `paranoid` | Override target type. |
| `brokerName`, `customerName`, `targetName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | Standard-only operator context; scanned with free-text and rendered-string backstops. |
| `configKey`, `configValue`, `configValueInt`, `customerId`, `targetGid` | Secret or unmodeled nested structure | none | Dropped because override keys, values, and tenant/target identifiers require resource-specific review before exposure. |

## ZTW Workload Groups

Commands:

```sh
zscalerctl ztw workload-groups list
zscalerctl ztw workload-groups get <id>
zscalerctl dump --products ztw --resources ztw/workload-groups --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | ZTW workload group identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `expression` | Secret | never | Tag-expression text is dropped in the first ZTW pass because it can encode tenant tag keys and values. |
| `lastModifiedTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |
| `lastModifiedBy` | Secret | never | Admin identity reference is mapped into source records and dropped by projection. |
| `expressionJson` | Secret | never | Structured tag-expression graph is mapped into source records and dropped by projection. |

The SDK also returns nested expression containers and tag key/value pairs under
`expressionJson`. The reader maps that graph so SDK shape drift is visible to
tests, but the catalog does not allow it to render.

## ZTW Public Cloud Accounts

Commands:

```sh
zscalerctl ztw public-cloud-accounts list
zscalerctl ztw public-cloud-accounts get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `platformId` | Operational metadata | `standard`, `share`, `paranoid` | Internal record ID and cloud platform. |
| `accountId` | Sensitive identifier | `standard` | Cloud account/subscription identifier; local-only. |

## ZTW Dns Gateways

Commands:

```sh
zscalerctl ztw dns-gateways list
zscalerctl ztw dns-gateways get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `dnsGatewayType` | Operational metadata | `standard`, `share`, `paranoid` | Gateway identifier and type. |
| `name`, `failureBehavior` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `primaryIp`, `secondaryIp` | Sensitive identifier | `standard` | Local-only gateway addresses. |
| `lastModifiedTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |
| `ecDnsGatewayOptionsPrimary`, `ecDnsGatewayOptionsSecondary`, `lastModifiedBy` | Secret | never | Gateway option internals and admin identity are dropped in the first ZTW pass. |

## ZTW Forwarding Gateways

Commands:

```sh
zscalerctl ztw forwarding-gateways list
zscalerctl ztw forwarding-gateways get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `failClosed`, `primaryType`, `secondaryType`, `type`, `dnsGatewayType` | Operational metadata | `standard`, `share`, `paranoid` | Gateway identity and behavior flags. |
| `name`, `failureBehavior` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `manualPrimary`, `manualSecondary`, `primaryIp`, `secondaryIp` | Sensitive identifier | `standard` | Local-only gateway endpoints. |
| `subcloudPrimary`, `subcloudSecondary` | Tenant configuration | `standard` | Rendered as id/name references only; `externalId` and extensions are dropped. |
| `lastModifiedTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |
| `ecDnsGatewayOptionsPrimary`, `ecDnsGatewayOptionsSecondary`, `lastModifiedBy` | Secret | never | Gateway option internals and admin identity are dropped in the first ZTW pass. |

## ZTW Ec Groups

Commands:

```sh
zscalerctl ztw ec-groups list
zscalerctl ztw ec-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `deployType`, `status`, `platform`, `maxEcCount` | Operational metadata | `standard`, `share`, `paranoid` | EC group identity and state. |
| `name`, `tunnelMode` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `desc` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `awsAvailabilityZone`, `azureAvailabilityZone` | Sensitive identifier | `standard` | Local-only cloud placement metadata. |
| `location`, `provTemplate` | Tenant configuration | `standard` | Rendered as id/name references only; `externalId` and extensions are dropped. |
| `ecVMs` | Secret | never | EC VM and network internals are dropped; use a dedicated resource later if needed. |

## ZTW IP Source Groups

Commands:

```sh
zscalerctl ztw ip-source-groups list
zscalerctl ztw ip-source-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `creatorContext`, `isNonEditable` | Operational metadata | `standard`, `share`, `paranoid` | Group identity and immutable/predefined state. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `ipAddresses` | Sensitive identifier | `standard` | Local-only source addresses. |

## ZTW IP Destination Groups

Commands:

```sh
zscalerctl ztw ip-destination-groups list
zscalerctl ztw ip-destination-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `type`, `countries`, `isNonEditable` | Operational metadata | `standard`, `share`, `paranoid` for `id`, `type`, `isNonEditable`; `standard`, `share` for `countries` | Group identity and broad metadata. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `addresses`, `ipCategories` | Sensitive identifier | `standard` | Local-only destination addresses and category references. |

## ZTW IP Groups

Commands:

```sh
zscalerctl ztw ip-groups list
zscalerctl ztw ip-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `creatorContext`, `isNonEditable`, `extranetIpPool`, `isPredefined` | Operational metadata | `standard`, `share`, `paranoid` | Group identity and immutable/predefined state. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `ipAddresses` | Sensitive identifier | `standard` | Local-only addresses. |

## ZTW Network Services

Commands:

```sh
zscalerctl ztw network-services list
zscalerctl ztw network-services get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `type`, `isNameL10nTag`, `creatorContext` | Operational metadata | `standard`, `share`, `paranoid` | Service identity and metadata. |
| `name`, `tag` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `srcTcpPorts`, `destTcpPorts`, `srcUdpPorts`, `destUdpPorts` | Tenant configuration | `standard`, `share` | Port ranges are rendered as `start`/`end` pairs. |

## ZTW Network Service Groups

Commands:

```sh
zscalerctl ztw network-service-groups list
zscalerctl ztw network-service-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `creatorContext` | Operational metadata | `standard`, `share`, `paranoid` | Group identity and source context. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `services` | Tenant configuration | `standard` | Rendered as id/name references only; service ports/details are owned by `ztw/network-services`. |

## ZTW Admin Users

Commands:

```sh
zscalerctl ztw admin-users list
zscalerctl ztw admin-users get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `disabled`, `isNonEditable`, `isAuditor` | Operational metadata | `standard`, `share`, `paranoid` | Admin account identity and broad state. |
| `loginName`, `userName`, `email` | Sensitive identifier | `standard` | Person-identifying admin account fields. Visible to the local administrator, dropped from `share` and `paranoid` output. |
| `comments` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `pwdLastModifiedTime` | Operational metadata | `standard` | Password-age metadata is visible only in the local admin view. |
| `isPasswordLoginAllowed`, `isPasswordExpired`, `password`, `execMobileAppTokens` | Secret or auth-sensitive field | never | Password flags and mobile app token data are dropped even if a GET response includes them. |
| `isSecurityReportCommEnabled`, `isServiceUpdateCommEnabled`, `isProductUpdateCommEnabled`, `isExecMobileAppEnabled`, `adminScopeType` | Tenant configuration | `standard`, `share` | Admin communication, app access, and scope-type settings. |
| `adminScopescopeGroupMemberEntities`, `adminScopeScopeEntities` | Tenant configuration | `standard` | Scope references render as id/name only; dropped from shared output because they can identify locations, departments, or groups. |
| `role` | Tenant configuration | `standard` | Role reference renders as id/name plus localization flag; extensions are dropped. |

## ZTW Admin Roles

Commands:

```sh
zscalerctl ztw admin-roles list
zscalerctl ztw admin-roles get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `isAuditor`, `isNonEditable` | Operational metadata | `standard`, `share`, `paranoid` | Role identity and broad state. |
| `name`, `roleType` | Tenant configuration | `standard`, `share` | Share-safe role identity metadata. |
| `rank`, `policyAccess`, `alertingAccess`, `dashboardAccess`, `reportAccess`, `analysisAccess`, `usernameAccess`, `adminAcctAccess`, `deviceInfoAccess`, `permissions`, `logsLimit` | Tenant configuration | `standard` | Authorization details are visible to the local administrator and dropped from `share` and `paranoid`. |
| `featurePermissions` | Secret or unmodeled nested structure | never | Arbitrary feature-permission map is dropped until its shape is reviewed. |

## ZTW Locations

Commands:

```sh
zscalerctl ztw locations list
zscalerctl ztw locations get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | ZTW location identifier. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `parentId`, `upBandwidth`, `dnBandwidth`, `overrideUpBandwidth`, `overrideDnBandwidth`, `sharedUpBandwidth`, `sharedDownBandwidth`, `unusedUpBandwidth`, `otherSubLocation`, `other6SubLocation`, `idleTimeInMinutes`, `displayTimeUnit`, `surrogateRefreshTimeInMinutes`, `surrogateRefreshTimeUnit`, `aupTimeoutInDays`, `childCount` | Operational metadata | `standard` | Location hierarchy, counters, and timing/bandwidth values are local-admin only. |
| `country`, `state`, `language`, `tz`, `ports`, `authRequired`, `sslScanEnabled`, `zappSSLScanEnabled`, `xffForwardEnabled`, `ecLocation`, `surrogateIP`, `surrogateIPEnforcedForKnownBrowsers`, `ofwEnabled`, `ipsControl`, `aupEnabled`, `cautionEnabled`, `aupBlockInternetUntilAccepted`, `aupForceSslInspection`, `profile`, `ipv6Enabled`, `ipv6Dns64Prefix`, `kerberosAuth`, `digestAuthEnabled`, `matchInChild`, `excludeFromDynamicGroups`, `excludeFromManualGroups` | Tenant configuration | `standard` | Detailed location controls are visible to the local administrator and dropped from shared output. |
| `ipAddresses` | Sensitive identifier | `standard` | Local-only location addresses. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `vpnCredentials`, `vpcInfo` | Secret or unmodeled nested structure | never | VPN credentials and VPC/cloud internals are dropped. |
| `virtualZens`, `virtualZenClusters`, `staticLocationGroups`, `dynamiclocationGroups` | Tenant configuration | `standard` | Rendered as id/name/external-ID references only. |
| `publicCloudAccountId` | Tenant configuration | `standard` | Rendered as an id/name reference only. |

## ZTW Location Templates

Commands:

```sh
zscalerctl ztw location-templates list
zscalerctl ztw location-templates get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `editable` | Operational metadata | `standard`, `share`, `paranoid` | Template identity and editability flag. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `desc` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `template` | Tenant configuration | `standard` | Nested template controls render only in local-admin output. |
| `lastModTime` | Operational metadata | `standard`, `share` | SDK timestamp value. |
| `lastModUid` | Secret | never | Admin identity is dropped. |

## ZTW Account Groups

Commands:

```sh
zscalerctl ztw account-groups list
zscalerctl ztw account-groups get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Account group identifier. |
| `name`, `cloudType` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `publicCloudAccounts`, `cloudConnectorGroups` | Tenant configuration | `standard` | Rendered as id/name references only; child resource details are owned by their dedicated resources. |

## ZTW Public Cloud Info

Commands:

```sh
zscalerctl ztw public-cloud-info list
zscalerctl ztw public-cloud-info get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Public cloud info identifier. |
| `name` | Sensitive identifier | `standard` | Cloud account/display identifier; local-only. |
| `cloudType` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `externalId`, `lastModUser`, `accountDetails` | Secret or unmodeled nested structure | never | External account IDs, admin identity, and account details are dropped. |
| `lastModTime`, `lastSyncTime` | Operational metadata | `standard`, `share` | SDK timestamp values. |
| `accountGroups`, `regionStatus`, `supportedRegions` | Tenant configuration | `standard` | Rendered as reviewed references/status summaries only. |

## ZTW Zpa Application Segments

Commands:

```sh
zscalerctl ztw zpa-application-segments list
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `deleted` | Operational metadata | `standard`, `share`, `paranoid` | Segment identity and deletion state. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `zpaId` | Sensitive identifier | `standard` | Local-only ZPA tenant/application reference. |

## ZTW Forwarding Rules

Commands:

```sh
zscalerctl ztw forwarding-rules list
zscalerctl ztw forwarding-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `type`, `order`, `rank`, `defaultRule`, `state`, `lastModifiedTime`, `sourceIpGroupExclusion`, `zpaBrokerRule` | Operational metadata | `standard`, `share`, `paranoid` for broad flags; timestamp is `standard`, `share` | Rule identity, ordering, state, and broad flags. |
| `name`, `accessControl`, `forwardMethod`, `wanSelection`, `blockResponseCode`, `nwApplications`, `destCountries`, `sourceCountries`, `labels` | Tenant configuration | `standard`, `share` | Share-safe rule metadata and broad criteria. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `srcIps`, `destAddresses`, `destIpCategories`, `resCategories` | Sensitive identifier | `standard` | Local-only network and resource criteria. |
| `locations`, `locationGroups`, `ecGroups`, `departments`, `groups`, `users`, `srcIpGroups`, `srcIpv6Groups`, `destIpGroups`, `destIpv6Groups`, `nwServices`, `nwServiceGroups`, `nwApplicationGroups`, `appServiceGroups`, `srcWorkloadGroups`, `proxyGateway`, `zpaApplicationSegments`, `zpaApplicationSegmentGroups` | Tenant configuration | `standard` | Rendered as constrained references; child graph details are owned by dedicated resources. |
| `lastModifiedBy` | Secret | never | Admin identity is dropped. |

## ZTW Traffic Dns Rules

Commands:

```sh
zscalerctl ztw traffic-dns-rules list
zscalerctl ztw traffic-dns-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `type`, `order`, `rank`, `state`, `predefined`, `defaultRule`, `lastModifiedTime` | Operational metadata | `standard`, `share`, `paranoid` for broad flags; timestamp is `standard`, `share` | Rule identity, ordering, state, and broad flags. |
| `name`, `action` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `srcIps`, `destAddresses` | Sensitive identifier | `standard` | Local-only network criteria. |
| `locations`, `locationGroups`, `ecGroups`, `srcIpGroups`, `destIpGroups`, `dnsGateway`, `zpaIpGroup` | Tenant configuration | `standard` | Rendered as constrained references only. |
| `lastModifiedBy` | Secret | never | Admin identity is dropped. |

## ZTW Traffic Log Rules

Commands:

```sh
zscalerctl ztw traffic-log-rules list
zscalerctl ztw traffic-log-rules get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `order`, `rank`, `state`, `type`, `defaultRule`, `lastModifiedTime` | Operational metadata | `standard`, `share`, `paranoid` for broad flags; timestamp is `standard`, `share` | Rule identity, ordering, state, and broad flags. |
| `name`, `forwardMethod` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `locations`, `proxyGateway`, `ecGroups` | Tenant configuration | `standard` | Rendered as constrained references only. |
| `lastModifiedBy` | Secret | never | Admin identity is dropped. |

## ZIDENTITY Groups

Commands:

```sh
zscalerctl zidentity groups list
zscalerctl zidentity groups get <id>
zscalerctl dump --products zidentity --resources zidentity/groups --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `isDynamicGroup`, `dynamicGroup` | Operational metadata | `standard`, `share`, `paranoid` | Group identity and dynamic-group flags. |
| `name`, `source`, `adminEntitlementEnabled`, `serviceEntitlementEnabled`, `idp` | Tenant configuration | `standard`, `share` | Workforce directory and entitlement-toggle metadata. `idp` renders id/name/displayName only. |
| `description` | Free text | `standard` | Admin-controlled text; scanned before output, including bare high-entropy tokens. |

## ZIDENTITY Users

Commands:

```sh
zscalerctl zidentity users list
zscalerctl zidentity users get <id>
zscalerctl dump --products zidentity --resources zidentity/users --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `status` | Operational metadata | `standard`, `share`, `paranoid` | User identifier and enabled/status flag. |
| `source`, `department`, `idp` | Tenant configuration | `standard`, `share` | Workforce directory context visible to Zidentity administrators. `department` and `idp` render id/name/displayName only. |
| `loginName`, `displayName`, `firstName`, `lastName`, `primaryEmail`, `secondaryEmail` | Sensitive identifier | `standard` | Person-identifying workforce directory fields. Visible to the local administrator, dropped from `share` and `paranoid` output. |
| `customAttrsInfo` | Secret | none | Tenant-defined arbitrary attributes; dropped until specific keys are reviewed. |

## ZIDENTITY Resource Servers

Commands:

```sh
zscalerctl zidentity resource-servers list
zscalerctl zidentity resource-servers get <id>
zscalerctl dump --products zidentity --resources zidentity/resource-servers --out ./scratch-live-smoke
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `defaultApi` | Operational metadata | `standard`, `share`, `paranoid` | Resource server identifier and default API flag. |
| `name`, `displayName` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `description` | Free text | `standard` | High-risk admin-controlled text; scanned before output, including bare high-entropy tokens. |
| `primaryAud` | Sensitive identifier | `standard` | OAuth audience value; dropped from `share` and `paranoid`. |
| `serviceScopes` | Tenant configuration | `standard` | Rendered as service and scope id/name references only. Service `cloudName` and `orgName` are dropped pending product-specific review. |

## Adding A Resource

Before enabling another resource:

- Start with `scripts/scaffold-resource.sh --product <zia|zpa|ztw|zidentity> --resource
  <name> --package <sdk-package> --type <sdk-type>` to create a review bundle
  under `scratch/resource-drafts/`. The bundle wraps `catalog-draft.go`, adds
  reader/docs/validation notes, and does not mutate production files.
- Treat the generated scaffold as a fail-closed starting point: only approved
  global field names render by default, while ambiguous names such as `value`,
  `key`, `data`, `content`, and `metadata` stay `secret` unless modeled with
  resource-specific context.
- Map the SDK response shape into source records without using the reader as a
  second safety allow-list.
- Classify every candidate output field in the catalog.
- Mark known secret-bearing fields as `secret`, even when they are expected to
  be dropped.
- Treat free-text fields as standard-only exceptions. Each emitted free-text
  field must carry a catalog `standard_free_text_reason`, must not be allowed in
  `share` or `paranoid`, and must retain scanner backstop coverage.
- Add canary tests proving secret-looking names or descriptions are redacted,
  including bare high-entropy tokens for emitted string fields.
- Add nested drop tests for any SDK object that contains user, admin, key,
  token, credential, or free-text data.
- Confirm `AssertRenderedSubset` runs before rendering and dump writing.
- Update this reference and the shell completion tests.
