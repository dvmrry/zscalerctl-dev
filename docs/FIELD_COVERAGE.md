# zscalerctl Field Coverage

This report is generated. Do not edit by hand; run `make field-coverage`
to regenerate it. `TestFieldCoverageReportIsCurrent` fails the build on any
drift, so the numbers below cannot silently go stale.

## What This Measures

For every read-only resource, the SDK response struct exposes some set of
exported fields. Each field is in exactly one of two states:

- **Classified** — the field is in the resource catalog with an explicit
  classification (operational, tenant configuration, sensitive identifier,
  free text, or secret) that drives allow-list projection and per-mode
  redaction. Only classified fields are eligible to be emitted.
- **Ignored, with a reason** — the field is not in the catalog and carries a
  non-empty reason string. Ignored fields are **fail-closed dropped**: they
  are never emitted in any mode.

Coverage percent is classified / total exported SDK fields. A low number is
not a leak — every unclassified field is dropped — it is a measure of how
much of the available SDK surface has been deliberately reviewed and exposed.
This report exists so that "field coverage" is a verifiable number, not an
assurance. See [DATA_CLASSIFICATION.md](DATA_CLASSIFICATION.md) for the class
definitions and the fail-closed output rules. The machine-readable companion
[field-coverage.json](field-coverage.json) lists every ignored field name and
its reason, which feeds field-expansion planning.

## Ignored Fields Are Decided, Not Vague

Every ignore reason must begin with one of two prefixes, so each ignored
field is in a decided state:

- **Deliberate** (`deliberate: `) — permanently excluded, with a stated why:
  bookkeeping, UI display hints, computed counters, opaque SDK helpers, or
  cross-references whose details are documented on another resource.
- **Deferred** (`deferred: `) — genuinely not yet classified; the field still
  needs future modeling before it can be exposed.

**Decided coverage** is (classified + deliberate) / total: the share of the
SDK surface with a final answer. The end-state goal is **zero deferred
fields**, at which point decided coverage reaches 100% and every SDK field
is either rendered by classification or permanently excluded on the record.

## Repo-Wide Totals

- Resources: 165
- Total exported SDK fields: 2979
- Classified: 2961
- Ignored (fail-closed dropped): 18
  - Deliberate (permanently excluded): 18
  - Deferred (awaiting modeling): 0
- Coverage: 99.4%
- Decided coverage (classified + deliberate): 100.0%

## Per-Product Totals

Ranked worst coverage first.

| Product | Resources | Total | Classified | Deliberate | Deferred | Coverage | Decided |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| zia | 102 | 1681 | 1663 | 18 | 0 | 98.9% | 100.0% |
| zcc | 11 | 325 | 325 | 0 | 0 | 100.0% | 100.0% |
| zidentity | 3 | 43 | 43 | 0 | 0 | 100.0% | 100.0% |
| zpa | 28 | 638 | 638 | 0 | 0 | 100.0% | 100.0% |
| ztw | 21 | 292 | 292 | 0 | 0 | 100.0% | 100.0% |

## Per-Resource Coverage

Ranked worst coverage first within the whole catalog. `Deliberate` and
`Deferred` split the ignored fields by decided state; both are dropped before
any output. See [field-coverage.json](field-coverage.json) for the field
names, buckets, and reasons behind each row.

| Product | Resource | Total | Classified | Deliberate | Deferred | Coverage | Decided |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| zia | users | 22 | 18 | 4 | 0 | 81.8% | 100.0% |
| zia | workload-groups | 7 | 6 | 1 | 0 | 85.7% | 100.0% |
| zia | locations | 60 | 54 | 6 | 0 | 90.0% | 100.0% |
| zia | sublocations | 60 | 54 | 6 | 0 | 90.0% | 100.0% |
| zia | url-categories | 27 | 26 | 1 | 0 | 96.3% | 100.0% |
| zia | nss-feeds | 126 | 126 | 0 | 0 | 100.0% | 100.0% |
| zcc | company-info | 111 | 111 | 0 | 0 | 100.0% | 100.0% |
| zia | org-information | 69 | 69 | 0 | 0 | 100.0% | 100.0% |
| ztw | locations | 54 | 54 | 0 | 0 | 100.0% | 100.0% |
| zia | casb-dlp-rules | 53 | 53 | 0 | 0 | 100.0% | 100.0% |
| zpa | application-segments | 52 | 52 | 0 | 0 | 100.0% | 100.0% |
| zia | advanced-settings | 51 | 51 | 0 | 0 | 100.0% | 100.0% |
| zia | dlp-web-rules | 49 | 49 | 0 | 0 | 100.0% | 100.0% |
| zia | advanced-threat-settings | 48 | 48 | 0 | 0 | 100.0% | 100.0% |
| zpa | inspection-app-segments | 48 | 48 | 0 | 0 | 100.0% | 100.0% |
| zpa | pra-app-segments | 48 | 48 | 0 | 0 | 100.0% | 100.0% |
| zcc | application-profiles | 46 | 46 | 0 | 0 | 100.0% | 100.0% |
| zpa | app-connectors | 45 | 45 | 0 | 0 | 100.0% | 100.0% |
| zpa | browser-access | 45 | 45 | 0 | 0 | 100.0% | 100.0% |
| zpa | service-edges | 44 | 44 | 0 | 0 | 100.0% | 100.0% |
| zia | firewall-dns-rules | 43 | 43 | 0 | 0 | 100.0% | 100.0% |
| zia | url-filtering-rules | 42 | 42 | 0 | 0 | 100.0% | 100.0% |
| ztw | forwarding-rules | 42 | 42 | 0 | 0 | 100.0% | 100.0% |
| zpa | app-connector-groups | 41 | 41 | 0 | 0 | 100.0% | 100.0% |
| zpa | service-edge-groups | 41 | 41 | 0 | 0 | 100.0% | 100.0% |
| zia | cloud-app-control | 39 | 39 | 0 | 0 | 100.0% | 100.0% |
| zia | firewall-filtering-rules | 39 | 39 | 0 | 0 | 100.0% | 100.0% |
| zia | ips-policies | 39 | 39 | 0 | 0 | 100.0% | 100.0% |
| zcc | admin-roles | 38 | 38 | 0 | 0 | 100.0% | 100.0% |
| zia | forwarding-rules | 38 | 38 | 0 | 0 | 100.0% | 100.0% |
| zia | risk-profiles | 37 | 37 | 0 | 0 | 100.0% | 100.0% |
| zia | nat-control-rules | 36 | 36 | 0 | 0 | 100.0% | 100.0% |
| zia | file-type-rules | 35 | 35 | 0 | 0 | 100.0% | 100.0% |
| zia | ssl-inspection-rules | 32 | 32 | 0 | 0 | 100.0% | 100.0% |
| zia | end-user-notification-settings | 30 | 30 | 0 | 0 | 100.0% | 100.0% |
| zia | gre-tunnels | 30 | 30 | 0 | 0 | 100.0% | 100.0% |
| zia | dlp-dictionaries | 29 | 29 | 0 | 0 | 100.0% | 100.0% |
| zia | sandbox-rules | 29 | 29 | 0 | 0 | 100.0% | 100.0% |
| zia | location-groups | 28 | 28 | 0 | 0 | 100.0% | 100.0% |
| zcc | forwarding-profiles | 26 | 26 | 0 | 0 | 100.0% | 100.0% |
| zcc | devices | 25 | 25 | 0 | 0 | 100.0% | 100.0% |
| zia | admin-users | 21 | 21 | 0 | 0 | 100.0% | 100.0% |
| zia | bandwidth-control-rules | 21 | 21 | 0 | 0 | 100.0% | 100.0% |
| zia | dlp-idm-profiles | 21 | 21 | 0 | 0 | 100.0% | 100.0% |
| zpa | c2c-ip-ranges | 21 | 21 | 0 | 0 | 100.0% | 100.0% |
| zpa | user-portals | 21 | 21 | 0 | 0 | 100.0% | 100.0% |
| ztw | admin-users | 21 | 21 | 0 | 0 | 100.0% | 100.0% |
| ztw | traffic-dns-rules | 21 | 21 | 0 | 0 | 100.0% | 100.0% |
| zpa | server-groups | 20 | 20 | 0 | 0 | 100.0% | 100.0% |
| zia | admin-roles | 19 | 19 | 0 | 0 | 100.0% | 100.0% |
| zpa | version-profiles | 19 | 19 | 0 | 0 | 100.0% | 100.0% |
| ztw | forwarding-gateways | 19 | 19 | 0 | 0 | 100.0% | 100.0% |
| zia | casb-malware-rules | 18 | 18 | 0 | 0 | 100.0% | 100.0% |
| zcc | trusted-networks | 17 | 17 | 0 | 0 | 100.0% | 100.0% |
| zia | dlp-edm-schemas | 17 | 17 | 0 | 0 | 100.0% | 100.0% |
| zia | pac-files | 17 | 17 | 0 | 0 | 100.0% | 100.0% |
| zpa | user-portal-links | 17 | 17 | 0 | 0 | 100.0% | 100.0% |
| ztw | admin-roles | 17 | 17 | 0 | 0 | 100.0% | 100.0% |
| zia | atp-malware-settings | 16 | 16 | 0 | 0 | 100.0% | 100.0% |
| zia | browser-control-settings | 16 | 16 | 0 | 0 | 100.0% | 100.0% |
| zia | tenancy-restriction-profiles | 16 | 16 | 0 | 0 | 100.0% | 100.0% |
| zia | vzen-nodes | 16 | 16 | 0 | 0 | 100.0% | 100.0% |
| zidentity | resource-servers | 16 | 16 | 0 | 0 | 100.0% | 100.0% |
| zpa | posture-profiles | 16 | 16 | 0 | 0 | 100.0% | 100.0% |
| zpa | segment-groups | 16 | 16 | 0 | 0 | 100.0% | 100.0% |
| zia | intermediate-ca-certificates | 15 | 15 | 0 | 0 | 100.0% | 100.0% |
| zidentity | users | 15 | 15 | 0 | 0 | 100.0% | 100.0% |
| zpa | branch-connectors | 15 | 15 | 0 | 0 | 100.0% | 100.0% |
| zcc | fail-open-policy | 14 | 14 | 0 | 0 | 100.0% | 100.0% |
| zcc | web-app-services | 14 | 14 | 0 | 0 | 100.0% | 100.0% |
| zia | auth-settings | 14 | 14 | 0 | 0 | 100.0% | 100.0% |
| zia | dns-gateways | 14 | 14 | 0 | 0 | 100.0% | 100.0% |
| zia | ips-signature-rules | 14 | 14 | 0 | 0 | 100.0% | 100.0% |
| zpa | microtenants | 14 | 14 | 0 | 0 | 100.0% | 100.0% |
| ztw | traffic-log-rules | 14 | 14 | 0 | 0 | 100.0% | 100.0% |
| zcc | predefined-ip-apps | 13 | 13 | 0 | 0 | 100.0% | 100.0% |
| zia | bandwidth-classes | 13 | 13 | 0 | 0 | 100.0% | 100.0% |
| zpa | cloud-connectors | 13 | 13 | 0 | 0 | 100.0% | 100.0% |
| ztw | ec-groups | 13 | 13 | 0 | 0 | 100.0% | 100.0% |
| zia | casb-tenants | 12 | 12 | 0 | 0 | 100.0% | 100.0% |
| zidentity | groups | 12 | 12 | 0 | 0 | 100.0% | 100.0% |
| zpa | app-servers | 12 | 12 | 0 | 0 | 100.0% | 100.0% |
| zpa | client-types | 12 | 12 | 0 | 0 | 100.0% | 100.0% |
| zpa | cloud-connector-groups | 12 | 12 | 0 | 0 | 100.0% | 100.0% |
| zpa | user-portal-aups | 12 | 12 | 0 | 0 | 100.0% | 100.0% |
| zcc | custom-ip-apps | 11 | 11 | 0 | 0 | 100.0% | 100.0% |
| zia | network-services | 11 | 11 | 0 | 0 | 100.0% | 100.0% |
| zia | proxies | 11 | 11 | 0 | 0 | 100.0% | 100.0% |
| zia | static-ips | 11 | 11 | 0 | 0 | 100.0% | 100.0% |
| ztw | network-services | 11 | 11 | 0 | 0 | 100.0% | 100.0% |
| ztw | public-cloud-info | 11 | 11 | 0 | 0 | 100.0% | 100.0% |
| zcc | process-based-apps | 10 | 10 | 0 | 0 | 100.0% | 100.0% |
| zia | devices | 10 | 10 | 0 | 0 | 100.0% | 100.0% |
| zpa | cbi-zpa-profiles | 10 | 10 | 0 | 0 | 100.0% | 100.0% |
| zpa | config-overrides | 10 | 10 | 0 | 0 | 100.0% | 100.0% |
| zpa | isolation-profiles | 10 | 10 | 0 | 0 | 100.0% | 100.0% |
| zpa | machine-groups | 10 | 10 | 0 | 0 | 100.0% | 100.0% |
| ztw | dns-gateways | 10 | 10 | 0 | 0 | 100.0% | 100.0% |
| zia | alert-subscriptions | 9 | 9 | 0 | 0 | 100.0% | 100.0% |
| zia | dedicated-ip-gateways | 9 | 9 | 0 | 0 | 100.0% | 100.0% |
| zia | proxy-gateways | 9 | 9 | 0 | 0 | 100.0% | 100.0% |
| zia | vzen-clusters | 9 | 9 | 0 | 0 | 100.0% | 100.0% |
| zia | zpa-gateways | 9 | 9 | 0 | 0 | 100.0% | 100.0% |
| zpa | trusted-networks | 9 | 9 | 0 | 0 | 100.0% | 100.0% |
| zia | c2c-incident-receivers | 8 | 8 | 0 | 0 | 100.0% | 100.0% |
| zia | device-groups | 8 | 8 | 0 | 0 | 100.0% | 100.0% |
| zia | ip-destination-groups | 8 | 8 | 0 | 0 | 100.0% | 100.0% |
| zia | mobile-threat-settings | 8 | 8 | 0 | 0 | 100.0% | 100.0% |
| ztw | ip-destination-groups | 8 | 8 | 0 | 0 | 100.0% | 100.0% |
| ztw | ip-groups | 8 | 8 | 0 | 0 | 100.0% | 100.0% |
| zia | dlp-notification-templates | 7 | 7 | 0 | 0 | 100.0% | 100.0% |
| zia | domain-profiles | 7 | 7 | 0 | 0 | 100.0% | 100.0% |
| zia | rule-labels | 7 | 7 | 0 | 0 | 100.0% | 100.0% |
| ztw | location-templates | 7 | 7 | 0 | 0 | 100.0% | 100.0% |
| ztw | workload-groups | 7 | 7 | 0 | 0 | 100.0% | 100.0% |
| zia | cloud-app-instances | 6 | 6 | 0 | 0 | 100.0% | 100.0% |
| zia | dc-exclusions | 6 | 6 | 0 | 0 | 100.0% | 100.0% |
| zia | dlp-engines | 6 | 6 | 0 | 0 | 100.0% | 100.0% |
| zia | dlp-idm-profile-lite | 6 | 6 | 0 | 0 | 100.0% | 100.0% |
| zia | ipv6-dns64-prefixes | 6 | 6 | 0 | 0 | 100.0% | 100.0% |
| zia | ipv6-nat64-prefixes | 6 | 6 | 0 | 0 | 100.0% | 100.0% |
| zia | nss-servers | 6 | 6 | 0 | 0 | 100.0% | 100.0% |
| ztw | account-groups | 6 | 6 | 0 | 0 | 100.0% | 100.0% |
| ztw | ip-source-groups | 6 | 6 | 0 | 0 | 100.0% | 100.0% |
| zia | casb-email-labels | 5 | 5 | 0 | 0 | 100.0% | 100.0% |
| zia | custom-file-types | 5 | 5 | 0 | 0 | 100.0% | 100.0% |
| zia | departments | 5 | 5 | 0 | 0 | 100.0% | 100.0% |
| zia | dlp-incident-receiver-servers | 5 | 5 | 0 | 0 | 100.0% | 100.0% |
| zia | groups | 5 | 5 | 0 | 0 | 100.0% | 100.0% |
| zia | ip-source-groups | 5 | 5 | 0 | 0 | 100.0% | 100.0% |
| zia | time-intervals | 5 | 5 | 0 | 0 | 100.0% | 100.0% |
| zia | time-windows | 5 | 5 | 0 | 0 | 100.0% | 100.0% |
| zpa | platforms | 5 | 5 | 0 | 0 | 100.0% | 100.0% |
| ztw | network-service-groups | 5 | 5 | 0 | 0 | 100.0% | 100.0% |
| ztw | zpa-application-segments | 5 | 5 | 0 | 0 | 100.0% | 100.0% |
| zia | browser-isolation-profiles | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| zia | cloud-application-policy | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| zia | cloud-application-ssl-policy | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| zia | dlp-icap-servers | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| zia | email-profiles | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| zia | ftp-control-policy | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| zia | network-application-groups | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| zia | network-applications | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| zia | network-service-groups | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| zia | remote-assistance | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| zia | sub-clouds | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| ztw | activation-status | 4 | 4 | 0 | 0 | 100.0% | 100.0% |
| zia | application-service-groups | 3 | 3 | 0 | 0 | 100.0% | 100.0% |
| zia | application-services | 3 | 3 | 0 | 0 | 100.0% | 100.0% |
| zia | atp-malware-protocols | 3 | 3 | 0 | 0 | 100.0% | 100.0% |
| zia | casb-tombstone-templates | 3 | 3 | 0 | 0 | 100.0% | 100.0% |
| zia | eusa-status | 3 | 3 | 0 | 0 | 100.0% | 100.0% |
| zia | ipv6-config | 3 | 3 | 0 | 0 | 100.0% | 100.0% |
| zia | supported-browser-versions | 3 | 3 | 0 | 0 | 100.0% | 100.0% |
| ztw | public-cloud-accounts | 3 | 3 | 0 | 0 | 100.0% | 100.0% |
| zia | atp-malware-inspection | 2 | 2 | 0 | 0 | 100.0% | 100.0% |
| zia | atp-malware-policy | 2 | 2 | 0 | 0 | 100.0% | 100.0% |
| zia | dlp-edm-schemas-lite | 2 | 2 | 0 | 0 | 100.0% | 100.0% |
| zia | url-allow-list | 2 | 2 | 0 | 0 | 100.0% | 100.0% |
| zia | url-deny-list | 2 | 2 | 0 | 0 | 100.0% | 100.0% |
| zia | activation-status | 1 | 1 | 0 | 0 | 100.0% | 100.0% |
| zia | auth-exempted-urls | 1 | 1 | 0 | 0 | 100.0% | 100.0% |
| zia | malicious-urls | 1 | 1 | 0 | 0 | 100.0% | 100.0% |
| zia | sandbox-settings | 1 | 1 | 0 | 0 | 100.0% | 100.0% |
| zia | security-exceptions | 1 | 1 | 0 | 0 | 100.0% | 100.0% |

