# SDK Pagination Validation

Use these checks to validate the bounded ZIA, ZTW, and ZCC readers without
recording tenant payloads. ZPA has additional response-metadata checks in
[ZPA Pagination Validation](ZPA_PAGINATION_VALIDATION.md).

## Automated Regression

No credentials are required:

```sh
go test -mod=vendor ./internal/zscaler \
  -run 'Test(ZIAPaginate|ZTWPaginate|ZCCPaginate|GetZIAAllPages|GetZIAUsersAllPages|GetZIAURLCategoriesAllRequestsAllCategoryTypes|GetZIASublocationByID|NormalizeZIABrowserIsolationListError|GetZTWAllPages|GetZCCAllPages|ReadersAvoidVendoredUnboundedPagination)' \
  -count=1
go test -race -mod=vendor ./internal/zscaler \
  -run 'Test(ZIAPaginate|ZTWPaginate|ZCCPaginate|GetZIAAllPages|GetZIAUsersAllPages|GetZIAURLCategoriesAllRequestsAllCategoryTypes|GetZIASublocationByID|NormalizeZIABrowserIsolationListError|GetZTWAllPages|GetZCCAllPages|ReadersAvoidVendoredUnboundedPagination)' \
  -count=1
```

The source-aware regression test resolves production SDK calls into the
vendored implementation. It rejects both obvious `GetAll` calls and
non-obvious helpers that ultimately use an unbounded SDK paginator.

## Live Tenant Check

Build the branch, use normal `ZSCALERCTL_*` environment configuration, and
confirm `doctor` before making live reads:

```sh
go build -mod=vendor -o ./zscalerctl ./cmd/zscalerctl
./zscalerctl doctor
```

The following shell loop emits only resource names, counts, and ID
cardinalities. It does not write or print complete records:

```sh
set -o pipefail

for spec in \
  zia/users \
  zia/locations \
  zia/sublocations \
  zia/location-groups \
  zia/devices \
  zia/url-categories \
  zia/firewall-filtering-rules \
  zia/forwarding-rules \
  zia/dlp-web-rules \
  zia/casb-dlp-rules \
  zia/admin-users \
  ztw/account-groups \
  ztw/admin-users \
  ztw/dns-gateways \
  ztw/ec-groups \
  ztw/forwarding-gateways \
  ztw/forwarding-rules \
  ztw/ip-destination-groups \
  ztw/ip-groups \
  ztw/ip-source-groups \
  ztw/location-templates \
  ztw/locations \
  ztw/network-service-groups \
  ztw/network-services \
  ztw/public-cloud-accounts \
  ztw/public-cloud-info \
  ztw/traffic-dns-rules \
  ztw/traffic-log-rules \
  ztw/workload-groups \
  ztw/zpa-application-segments \
  zcc/admin-roles \
  zcc/application-profiles \
  zcc/custom-ip-apps \
  zcc/devices \
  zcc/fail-open-policy \
  zcc/forwarding-profiles \
  zcc/predefined-ip-apps \
  zcc/process-based-apps \
  zcc/trusted-networks \
  zcc/web-app-services
do
  product=${spec%%/*}
  resource=${spec#*/}
  ./zscalerctl --timeout 30s --format json "$product" "$resource" list |
    jq -c --arg product "$product" --arg resource "$resource" '
      ([.[] | (.id? // .profileId? // empty)]) as $ids |
      {product: $product, resource: $resource, count: length,
       ids_present: ($ids | length), unique_ids: ($ids | unique | length)}'
done
```

Entitlement-gated resources may return the normal structured error envelope;
record those separately rather than treating them as empty. For successful
resources:

- compare `count` with the matching admin-console view;
- when `ids_present` equals `count`, require `unique_ids` to equal `count`;
- verify known high-cardinality resources cross their prior first-page
  boundary; and
- require any later-page or ceiling failure to exit nonzero instead of
  returning a shorter successful array.

### URL-category 20-record clamp regression

On a tenant whose console contains more than 20 URL and TLD categories, run:

```sh
./zscalerctl --timeout 30s --format json zia url-categories list |
  jq -c '
    ([.[] | .id? // empty]) as $ids |
    {count: length, ids_present: ($ids | length),
     unique_ids: ($ids | unique | length),
     types: ([.[].type] | unique | sort)}'
```

Pass criteria:

- `count` is greater than 20 and matches the corresponding console inventory;
- `ids_present`, `unique_ids`, and `count` are equal; and
- `types` includes `URL_CATEGORY` and, where configured or returned by the
  tenant, `TLD_CATEGORY`.

Record only the aggregate object above. Do not save or paste category records.

## Explicit Single-Read Targets

The following ZIA list endpoints still issue one GET because source review did
not establish that they paginate:

```text
admin-roles
dc-exclusions
ip-destination-groups
ip-source-groups
nss-servers
sandbox-rules
ssl-inspection-rules
time-windows
```

Compare their counts with the corresponding console views. If any stops at a
stable boundary, capture only the resource name, observed CLI count, expected
console count, HTTP status, and relevant response pagination headers or
metadata. Do not commit tenant records. That evidence is the prerequisite for
an endpoint-specific pagination adapter.
