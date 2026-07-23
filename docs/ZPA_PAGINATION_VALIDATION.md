# ZPA Pagination Validation

Use this focused check when validating the bounded ZPA list adapter. Twenty-four
catalog resources share the adapter; it preserves the existing endpoint and
microtenant behavior while rejecting incomplete or contradictory pagination
metadata.

## Automated Regression

Run the focused ordinary and race tests without credentials:

```sh
go test -mod=vendor ./internal/zscaler \
  -run 'Test(ParseZPATotalPages|ZPAPaginate|GetZPAAllPages|ZPAListHandlers)' \
  -count=1
go test -race -mod=vendor ./internal/zscaler \
  -run 'Test(ParseZPATotalPages|ZPAPaginate|GetZPAAllPages|ZPAListHandlers)' \
  -count=1
```

The tests cover numeric and quoted page counts, malformed metadata, multiple
pages, metadata drift, empty and repeated pages, page errors, the 1,000-page
ceiling, exact query parameters, microtenant propagation, and all 24 handler
wirings.

## Live Tenant Check

Build the branch and use the normal `ZSCALERCTL_*` environment configuration.
Do not add credentials or tenant payloads to this repository.

```bash
go build -mod=vendor -o ./zscalerctl ./cmd/zscalerctl
./zscalerctl doctor

for resource in \
  app-connector-groups app-connectors app-servers application-segments \
  branch-connectors browser-access cloud-connector-groups cloud-connectors \
  config-overrides inspection-app-segments isolation-profiles machine-groups \
  microtenants posture-profiles pra-app-segments segment-groups server-groups \
  service-edge-groups service-edges trusted-networks user-portal-aups \
  user-portal-links user-portals version-profiles
do
  ./zscalerctl --timeout 30s --format json zpa "$resource" list |
    jq -c --arg resource "$resource" \
      '{resource: $resource, count: length,
        unique_ids: ([.[].id | select(. != null)] | unique | length)}'
done
```

Entitlement-gated resources may return the normal structured error envelope;
record those separately rather than treating them as empty. For every
successful resource:

- `count` matches the current ZPA admin-console count for that resource view;
- `unique_ids` matches `count` when the resource exposes IDs;
- the command exits 0; and
- a pagination metadata or later-page failure exits nonzero instead of
  returning a shorter successful array.

The three application views use the same `/application` collection and then
retain only records carrying the corresponding browser-access, inspection, or
PRA subtype data. Compare each view to the matching console category rather
than to the total application-segment count.

## Dump Parity

List and dump use the same reader. A focused dump can verify a high-cardinality
resource without writing unrelated product data:

```sh
mkdir -p -m 700 ./scratch
out="./scratch/zpa-pagination-$(date +%Y%m%d-%H%M%S)"
./zscalerctl --timeout 30s dump \
  --resources zpa/application-segments \
  --out "$out"
jq '.resources[] |
  select(.product == "zpa" and .name == "application-segments") |
  {status, records, path}' "$out/manifest.json"
```

Expect `status: "ok"` and `records` equal to the list/admin-console count.
Dump artifacts remain confidential tenant inventory even after projection and
redaction; keep them under ignored scratch storage and dispose of them under
the operator's normal evidence-handling policy.
