# URL Filtering Pagination Validation

Use this focused check when validating a build that changes
`zia/url-filtering-rules` collection. The original downstream symptom was a
100-rule CLI result while the admin console showed 133 rules. Re-check the
console immediately before testing because concurrent policy edits can change
the expected count.

The reader requests `page=1&pageSize=100` and advances until a page contains
fewer than 100 records. It fails instead of returning a partial result when a
later page fails or when 1,000 full pages are returned.

## Automated Regression

Run the focused ordinary and race tests without credentials:

```sh
go test ./internal/zscaler \
  -run 'Test(GetZIAURLFilteringRulesAllPages|ZIAHighRecordEndpointsAvoidUnboundedSDKPagination|ZIAPaginate)' \
  -count=1
go test -race ./internal/zscaler \
  -run 'Test(GetZIAURLFilteringRulesAllPages|ZIAHighRecordEndpointsAvoidUnboundedSDKPagination|ZIAPaginate)' \
  -count=1
```

The transport regressions cover both a 100+33 split and an exact 100-record
total. They verify that all records are returned, that the short page stops the
walk, and that an exact multiple causes the required empty terminal-page
request. The source guard prevents this resource from being rewired to the
vendored SDK's single-page `GetAll` implementation.

## Live Tenant Check

Build the branch and use the normal `ZSCALERCTL_*` environment configuration.
Do not add credentials or tenant payloads to this repository.

```bash
go build -mod=vendor -o ./zscalerctl ./cmd/zscalerctl
./zscalerctl doctor
set -o pipefail
./zscalerctl --timeout 30s --format json zia url-filtering-rules list |
  jq '{count: length, unique_ids: ([.[].id] | unique | length)}'
```

Expected result:

- `count` matches the current admin-console total (133 in the original report).
- `unique_ids` equals `count`.
- The command exits 0. A page error must produce the normal structured error
  envelope and a nonzero exit instead of a shorter successful array.

PowerShell equivalent for a Windows validation host:

```powershell
go build -mod=vendor -o zscalerctl.exe ./cmd/zscalerctl
$json = .\zscalerctl.exe --timeout 30s --format json zia url-filtering-rules list
if ($LASTEXITCODE -ne 0) { throw "zscalerctl exited $LASTEXITCODE" }
$rules = ConvertFrom-Json -InputObject ($json -join "`n")
[pscustomobject]@{
  Count = $rules.Count
  UniqueIds = ($rules.id | Sort-Object -Unique).Count
}
```

## Dump Parity

The dump path uses the same reader. A focused dump should report the same
record count without writing unrelated product data:

```sh
mkdir -p -m 700 ./scratch
out="./scratch/url-filtering-pagination-$(date +%Y%m%d-%H%M%S)"
./zscalerctl --timeout 30s dump \
  --resources zia/url-filtering-rules \
  --out "$out"
jq '.resources[] |
  select(.product == "zia" and .name == "url-filtering-rules") |
  {status, records, path}' "$out/manifest.json"
```

Expect `status: "ok"` and `records` equal to the list/console count. Dump
artifacts remain confidential tenant inventory even after projection and
redaction; keep them under ignored scratch storage and dispose of them under
the operator's normal evidence-handling policy.
