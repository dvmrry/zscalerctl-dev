# SDK v3.8.43 Validation

This note covers the `zscaler-sdk-go` refresh from `v3.8.38` to `v3.8.43`.
The change adds no new zscalerctl resource endpoint. It updates the dependency,
keeps local bounded pagination around URL filtering reads, exposes reviewed new
fields returned for existing ZIA rule resources, and adopts the SDK's corrected
API-error preservation and deterministic-5xx retry behavior.

The v3.8.43 client now preserves the final structured API response and declines
to retry a deterministic 5xx API verdict. zscalerctl still treats all SDK error
bodies as untrusted: its boundary retains only the safe HTTP status code and
product/resource/operation labels. API codes, messages, and response bodies do
not enter CLI or engine error envelopes.

## Automated validation

No credentials are required:

```sh
go test -mod=vendor ./internal/resources ./internal/zscaler -count=1
make verify-vendor
make field-coverage
make check
```

The focused tests pin:

- bounded CBI-profile enrichment for `url-filtering-rules get`;
- preservation of a successful direct rule when optional enrichment fails;
- compact endpoint application/group references for SSL inspection, firewall
  filtering, and firewall DNS rules;
- standard/share/paranoid mode boundaries;
- omission of endpoint filenames, bundles, descriptions, versions, signature
  metadata, nested inventories, and HTTP-header reference extensions;
- classification of every reviewed top-level SDK field.
- a deterministic 500 response is attempted once, retains its safe status at
  the zscalerctl boundary, and cannot expose the SDK response message;
- ZPA Browser Access retains an explicit false `bypassOnReauth` value instead
  of losing it during SDK JSON conversion;
- mocked OneAPI `GOV` and `GOVUS` OAuth/product host routing using the corrected
  v3.8.43 `zidentitygov.us` GOVUS contract, while preserving the existing
  ZPATWO identity-host compatibility behavior.

These routing tests cover OAuth token exchange and ZIA product-gateway
selection only. They do not claim GOV/GOVUS Zidentity `/admin` routing, whose
tagged SDK path lacks a matching authoritative host test and still requires a
live environment-specific smoke.

## Optional live smoke

Run only with operator-provided `ZSCALERCTL_*` environment credentials. These
commands are tenant-read-only and write no files:

```sh
zscalerctl --format json --fields id,endPointApplications,endPointApplicationGroups zia ssl-inspection-rules list
zscalerctl --format json --fields id,httpHeaderProfiles,httpHeaderActionProfiles,cbiProfileId,cbiProfile zia url-filtering-rules list
zscalerctl --format json --fields id,isEunEnabled,eunTemplateId,excludeContextShieldEndPoint,endPointApplications,endPointApplicationGroups zia firewall-filtering-rules list
zscalerctl --format json --fields id,isEunEnabled,eunTemplateId,excludeContextShieldEndPoint,endPointApplications,endPointApplicationGroups zia firewall-dns-rules list
```

An absent optional field is not by itself a failure; it may simply be unused in
that tenant. Validate a configured rule when possible. Check that endpoint
application entries contain only `resourceId`, `applicationName`, `osType`, and
`applicationType`, and groups contain only `groupId` and `name`. Do not paste
tenant records into issues or review comments.

For an ISOLATE URL rule whose direct API response omits `cbiProfile`, also run:

```sh
zscalerctl --format json --fields id,action,cbiProfileId,cbiProfile zia url-filtering-rules get <id>
```

The returned profile may include only its reviewed ID, name, and sequence; its
URL must remain absent. The fallback stops at the shared 1,000-page ceiling.
The list operation reports a completeness error at that boundary; `get`
preserves its already-successful direct response without optional profile
enrichment, while still propagating caller cancellation and deadlines.

## New package queue

The 23 SDK packages plus new reads in existing packages added by v3.8.39 through
v3.8.41 are triaged in
[Resource Queue](RESOURCE_QUEUE.md#sdk-v3841-addition-triage). Releases v3.8.42
and v3.8.43 add no service package. All queued packages remain disabled until
their own field review, pagination proof, and live endpoint smoke are complete.
