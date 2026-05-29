# Resource Reference

This document describes the currently enabled resource catalog. The catalog is
the output allow-list: reader adapters map SDK response shapes into source
records, and projection decides which fields can render for each redaction mode.

## Redaction Modes

- `standard`: local operational use. Allows explicitly reviewed tenant
  configuration and free-text fields, with secret scanning and rendered-string
  high-entropy token scanning still applied.
- `share`: lower-detail output for tickets, reviews, and chat. Drops free text
  and sensitive identifiers.
- `paranoid`: minimal identifiers and counts only.

All fields, including allowed strings, pass through the final redaction backstop
before stdout or dump files. Rendered string values also receive a conservative
high-entropy token scan for bare unlabeled secret material. Canonical UUIDs and
contextual git commit SHAs are preserved. In `standard` mode, structured
rendered strings also preserve compact UUIDs and 40/64-character hex
fingerprints; `share` and `paranoid` redact those fingerprint-shaped values.
Free-text prose may redact bare hashes without context.

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

## Adding A Resource

Before enabling another resource:

- Start with `go run ./scripts/catalog-draft.go --package <sdk-package> --type
  <sdk-type> --product <zia|zpa> --resource <name>` to generate a classified
  catalog and SDK shape-review scaffold from the SDK struct.
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
