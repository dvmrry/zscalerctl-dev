# Location Group Members Validation

Use this focused check when validating a build that changes
`zia/location-groups` collection. The original downstream symptom was that
`get <id>` could contain associated locations while `list` and dump omitted
them because the list request explicitly sent `fetchLocations=false`.

The reader now sends `fetchLocations=true` and walks the endpoint with the
existing fail-closed ZIA paginator. Zscaler documents a maximum page size of
1,000 for this endpoint; the reader stops at the first short page and errors
instead of returning a partial result if a later page fails or 1,000 full pages
are returned.

Only reviewed member `id` and `name` fields render, and only in `standard`
redaction mode. SDK extension maps and the `lastModUser` admin reference remain
dropped. The API returns group membership as a flat list, so zscalerctl does
not infer a parent-child relationship between a location and a sublocation.

## Automated Regression

Run the focused ordinary and race tests without credentials:

```sh
go test ./internal/zscaler ./internal/livesmoke \
  -run '^(TestGetZIALocationGroupsAllPagesFetchesMemberLocations|TestZIAPaginate.*|TestZIAHighRecordEndpointsAvoidUnboundedSDKPagination|TestReaderListLocationGroupsProjectsSDKShapeThroughAllowList|TestLocationGroup.*|TestFindDeniedKeys|TestGoodFixturePasses|TestMisplacedLocationGroupFieldsFailListAndDumpSmoke)$' \
  -count=1
go test -race ./internal/zscaler ./internal/livesmoke \
  -run '^(TestGetZIALocationGroupsAllPagesFetchesMemberLocations|TestZIAPaginate.*|TestZIAHighRecordEndpointsAvoidUnboundedSDKPagination|TestReaderListLocationGroupsProjectsSDKShapeThroughAllowList|TestLocationGroup.*|TestFindDeniedKeys|TestGoodFixturePasses|TestMisplacedLocationGroupFieldsFailListAndDumpSmoke)$' \
  -count=1
```

The transport regression verifies the exact product host, one-based page,
1,000-record page size, `fetchLocations=true`, and SDK decoding of both a
parent-location reference and a known sublocation reference. Projection tests
prove that only reviewed fields survive, while the live-smoke regression keeps
member fields allowed and still rejects nested SDK extensions and admin
identity data.

## Live Tenant Check

Build the branch and use the normal `ZSCALERCTL_*` environment configuration.
Do not add credentials or tenant payloads to this repository. Set the fragment
to a group that has at least one known sublocation in the ZIA admin console.

```bash
go build -mod=vendor -o ./zscalerctl ./cmd/zscalerctl
./zscalerctl doctor
GROUP_FRAGMENT='replace with a known group name fragment'
set -o pipefail
./zscalerctl --timeout 30s --format json \
  zia location-groups list --filter "name~$GROUP_FRAGMENT" |
  jq -e -f docs/testdata/location-group-members-summary.jq
```

Expected result:

- The selected group appears with a nonzero `member_count` matching the current
  admin-console membership.
- `members` includes the known sublocation by id and name, even if its parent
  location is not independently assigned to that group.
- The validation filter checks the un-narrowed records before producing the
  summary. It fails nonzero if any group contains `lastModUser` or any member
  object contains a field other than `id` and `name`.
- The command exits 0. A page error must produce the normal structured error
  envelope and a nonzero exit instead of a shorter successful array.

PowerShell equivalent for a Windows validation host:

```powershell
go build -mod=vendor -o zscalerctl.exe ./cmd/zscalerctl
$GroupFragment = 'replace with a known group name fragment'
$json = .\zscalerctl.exe --timeout 30s --format json `
  zia location-groups list --filter "name~$GroupFragment"
if ($LASTEXITCODE -ne 0) { throw "zscalerctl exited $LASTEXITCODE" }
$groups = ConvertFrom-Json -InputObject ($json -join "`n")
foreach ($group in $groups) {
  if ($group.PSObject.Properties.Name -contains 'lastModUser') {
    throw "forbidden lastModUser field present"
  }
  foreach ($member in @($group.locations)) {
    $unexpected = @($member.PSObject.Properties.Name | Where-Object {
      $_ -notin @('id', 'name')
    })
    if ($unexpected.Count -ne 0) {
      throw "unexpected location member field(s): $($unexpected -join ', ')"
    }
  }
}
$groups | ForEach-Object {
  [pscustomobject]@{
    Id = $_.id
    Name = $_.name
    GroupType = $_.groupType
    MemberCount = @($_.locations).Count
    MemberNames = (@($_.locations) | ForEach-Object { $_.name }) -join ', '
  }
}
```

## Redaction And Dump Parity

Member references are standard-only. This check should print `true`:

```sh
set -o pipefail
./zscalerctl --timeout 30s --format json --redaction share \
  zia location-groups list |
  jq -e 'all(.[]; has("locations") | not)'
```

List and dump use the same reader. A focused dump should carry the same group
members without writing unrelated product data:

```sh
mkdir -p -m 700 ./scratch
out="./scratch/location-group-members-$(date +%Y%m%d-%H%M%S)"
./zscalerctl --timeout 30s dump \
  --resources zia/location-groups \
  --out "$out"
jq '.resources[] |
  select(.product == "zia" and .name == "location-groups") |
  {status, records, path}' "$out/manifest.json"
jq --arg fragment "$GROUP_FRAGMENT" \
  '[.[] | select(.name | ascii_downcase | contains($fragment | ascii_downcase)) |
    {id, name, member_count: ((.locations // []) | length), locations}]' \
  "$out/resources/zia/location-groups.json"
```

Dump artifacts remain confidential tenant inventory even after projection and
redaction. Keep them under ignored scratch storage and dispose of them under
the operator's normal evidence-handling policy.
