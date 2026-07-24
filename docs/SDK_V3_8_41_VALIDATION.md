# SDK v3.8.41 Validation

This note covers the `zscaler-sdk-go` refresh from `v3.8.38` to `v3.8.41`.
The change adds no new zscalerctl resource endpoint. It updates the dependency,
keeps local bounded pagination around URL filtering reads, and exposes reviewed
new fields returned for existing ZIA rule resources.

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
- mocked OneAPI `GOV` and `GOVUS` OAuth/product host routing while preserving
  the existing ZPATWO identity-host compatibility behavior.

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

The 23 SDK packages plus new reads in existing packages added since `v3.8.38` are triaged in
[Resource Queue](RESOURCE_QUEUE.md#sdk-v3841-addition-triage). They remain
disabled until their own field review, pagination proof, and live endpoint
smoke are complete.
