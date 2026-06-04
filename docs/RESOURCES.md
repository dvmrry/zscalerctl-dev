# Resource Reference

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

## ZCC Trusted Networks

Commands:

```sh
zscalerctl zcc trusted-networks list
zscalerctl zcc trusted-networks get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `active`, `conditionType` | Operational metadata | `standard`, `share`, `paranoid` | Trusted network identity and state. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `companyId`, `zpaId`, `dnsSearchDomains`, `dnsServerIps`, `guid`, `hostname`, `networkName`, `resolvedIpsForHostname`, `ssid`, `trustedDhcpServersIps`, `trustedEgressIps`, `trustedGatewayIps`, `trustedSubnetIps` | Sensitive identifier | `standard` | Local-only network, device, tenant, and endpoint identifiers. |
| `createdBy`, `editedBy` | Secret | never | Admin identity values are mapped into source records and dropped by projection. |

## ZCC Notification Templates

Commands:

```sh
zscalerctl zcc notification-templates list
zscalerctl zcc notification-templates get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `isDefaultTemplate`, `enableClient`, `enableZia`, `enableAppUpdates`, `enableServiceStatus`, `durationInSeconds`, `enablePersistent`, `enableDoNotDisturb` | Operational metadata | `standard`, `share`, `paranoid` | Template identity and behavior flags. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `ziaNotificationTemplate`, `zpaNotificationTemplate` | Tenant configuration | `standard`, `share` | Nested notification flags and timing values only. |
| `createdBy`, `editedBy` | Secret | never | Admin identity values are mapped into source records and dropped by projection. |

## ZCC Zia Postures

Commands:

```sh
zscalerctl zcc zia-postures list
zscalerctl zcc zia-postures get <id>
```

Fields:

| Field | Classification | Modes | Notes |
| --- | --- | --- | --- |
| `id`, `platform` | Operational metadata | `standard`, `share`, `paranoid` | Posture profile identity and platform. |
| `name` | Tenant configuration | `standard`, `share` | Scanned for pasted secret-shaped values. |
| `highTrustCriteria`, `mediumTrustCriteria`, `lowTrustCriteria` | Secret | never | Criteria graphs can contain endpoint/posture identifiers and are dropped in the first ZCC pass. |

## Deferred Resource Follow-Ups

- `zia/network-service-groups`: generated and locally validated, but removed
  from the first policy-reference batch after live smoke reported a request
  failure. Investigate the live endpoint behavior separately before enabling it
  in the catalog.
- `zia/network-applications`: generated and locally validated, but removed
  from the application-reference batch after live smoke reported a request
  failure while `zia/network-application-groups` succeeded. Investigate the live
  endpoint behavior separately before enabling it in the catalog.
- `zia/departments`: generated and locally validated, but removed from the
  identity-reference batch after live smoke reported a list request failure under
  ZIA legacy credentials. Investigate the live endpoint behavior separately
  before enabling it in the catalog.
- `zia/users`: generated and locally validated, but removed from the
  identity-reference batch after live smoke reported a list request failure under
  ZIA legacy credentials. Investigate the live endpoint behavior separately
  before enabling it in the catalog.
- `zia/devices`: generated and locally validated, but removed from the
  identity-reference batch after live smoke reported a list request failure under
  ZIA legacy credentials. Investigate the live endpoint behavior separately
  before enabling it in the catalog.
- `zia/email-profiles`: generated and locally validated, but removed from the
  security-profile batch after live smoke reported a list request failure under
  ZIA legacy credentials. Investigate the live endpoint behavior separately
  before enabling it in the catalog.
- `zia/dlp-engines`: generated and locally validated, but removed from the
  DLP-reference batch after live smoke reported a list request failure under
  ZIA legacy credentials. Investigate the live endpoint behavior separately
  before enabling it in the catalog.
- `zia/dlp-dictionaries`: generated and locally validated, but removed from the
  DLP-reference batch after live smoke reported a list request failure under
  ZIA legacy credentials. Investigate the live endpoint behavior separately
  before enabling it in the catalog.
- `zia/dlp-incident-receiver-servers`: generated and locally validated, but
  removed from the DLP-reference batch after live smoke reported a list request
  failure under ZIA legacy credentials. Investigate the live endpoint behavior
  separately before enabling it in the catalog.
- `zia/dlp-notification-templates`: generated and locally validated, but
  removed from the DLP-reference batch after live smoke reported a list request
  failure under ZIA legacy credentials. Investigate the live endpoint behavior
  separately before enabling it in the catalog.
- `zia/c2c-incident-receivers`, `zia/dlp-edm-schemas`,
  `zia/dlp-idm-profile-lite`, `zia/dlp-idm-profiles`, `zia/dlp-web-rules`,
  `zia/traffic-capture-rules`, and `zia/extranets`: generated and locally
  validated in the smoke-lab branch, but removed after work-machine live smoke
  reported `live_access_failed` list request failures under ZIA legacy
  credentials. Investigate endpoint behavior and auth-mode support separately
  before enabling them in the catalog.

## Adding A Resource

Before enabling another resource:

- Start with `scripts/scaffold-resource.sh --product <zia|zpa|ztw> --resource
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
