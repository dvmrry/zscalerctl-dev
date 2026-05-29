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

## Deferred Resource Follow-Ups

- `zia/network-service-groups`: generated and locally validated, but removed
  from the first policy-reference batch after live smoke reported a request
  failure. Investigate the live endpoint behavior separately before enabling it
  in the catalog.
- `zia/network-applications`: generated and locally validated, but removed
  from the application-reference batch after live smoke reported a request
  failure while `zia/network-application-groups` succeeded. Investigate the live
  endpoint behavior separately before enabling it in the catalog.

## Adding A Resource

Before enabling another resource:

- Start with `scripts/scaffold-resource.sh --product <zia|zpa> --resource
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
