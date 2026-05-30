# Resource Queue

This queue is the staging area for future read-only resources. It is not the
enabled catalog. Entries here do not expose new API surface, do not change
live-smoke coverage, and do not replace the catalog, SDK shape review, or live
smoke gates.

Use this file to avoid branch sprawl while live tenant testing is unavailable:
record candidates, exact scaffold commands, and known live-smoke outcomes here;
apply only one small batch to production files when a live smoke operator is
available.

## Operating Rules

- Keep at most one active resource PR open for live smoke.
- Do not merge resource PRs without a focused `make live-smoke` pass.
- Do not stack un-smoked resource branches behind an unmerged resource PR.
- Do not commit generated scaffold bundles from `scratch/resource-drafts/`.
- Regenerate scaffolds from SDK source when a batch is ready to apply.
- Keep failed live-smoke endpoints in the deferred list until their endpoint or
  auth-mode behavior is understood.

## Current Gate

Open draft PR:

| PR | Resources | Status | Smoke command |
| --- | --- | --- | --- |
| `#33` | `zia/risk-profiles`, `zia/nss-servers` | Awaiting work-machine live smoke | `make live-smoke` |

Do not start applying the next batch until this PR is either merged or trimmed
and merged.

## Next Apply Batches

These batches are ordered for small, focused PRs. Each batch should become a
single production change only after the preceding draft PR is resolved.

### Batch A: File And Sandbox Policy Rules

These are high-value policy surfaces with ordinary list/get SDK functions, but
they include many nested references. Keep the first pass conservative: map the
full SDK shape, allow only the generated safe names and any deliberately
reviewed policy metadata, and drop admin/user/device/ZPA nested details unless
they are explicitly modeled.

| Resource | SDK package | SDK type | List | Get | Notes |
| --- | --- | --- | --- | --- | --- |
| `zia/file-type-rules` | `github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/filetypecontrol` | `FileTypeRules` | `GetAll` | `Get` | Policy rule surface; expect nested locations, groups, users, devices, labels, and ZPA segment references. |
| `zia/sandbox-rules` | `github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sandbox/sandbox_rules` | `SandboxRules` | `GetAll` | `Get` | Policy rule surface; expect nested locations, groups, users, devices, labels, URL categories, and ZPA segment references. |

Scaffold commands:

```sh
make scaffold-resource PRODUCT=zia RESOURCE=file-type-rules PACKAGE=github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/filetypecontrol TYPE=FileTypeRules FORCE=1
make scaffold-resource PRODUCT=zia RESOURCE=sandbox-rules PACKAGE=github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/sandbox/sandbox_rules TYPE=SandboxRules FORCE=1
```

### Batch B: DNS And IPS Policy Rules

These are useful policy surfaces but likely to be noisier. Keep them separate
from Batch A so a single endpoint failure can be trimmed without losing the
whole policy-rule wave.

| Resource | SDK package | SDK type | List | Get | Notes |
| --- | --- | --- | --- | --- | --- |
| `zia/firewall-dns-rules` | `github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewalldnscontrolpolicies` | `FirewallDNSRules` | `GetAll` | `Get` | DNS policy rules; expect location, group, source/destination, gateway, and ZPA IP group references. |
| `zia/ips-policies` | `github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/ips_control_policies/ips_policies` | `FirewallIPSRules` | `GetAll` | `Get` | IPS policy rules; expect location, group, service, threat category, and ZPA segment references. |

Scaffold commands:

```sh
make scaffold-resource PRODUCT=zia RESOURCE=firewall-dns-rules PACKAGE=github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewalldnscontrolpolicies TYPE=FirewallDNSRules FORCE=1
make scaffold-resource PRODUCT=zia RESOURCE=ips-policies PACKAGE=github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/ips_control_policies/ips_policies TYPE=FirewallIPSRules FORCE=1
```

### Batch C: Certificate And Custom File References

These may need small custom list/get closures because the SDK function names are
not exactly `GetAll` and `Get`, but they are still read-only resource-shaped
surfaces.

| Resource | SDK package | SDK type | List | Get | Notes |
| --- | --- | --- | --- | --- | --- |
| `zia/intermediate-ca-certificates` | `github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/intermediatecacertificates` | `IntermediateCACertificate` | `GetAll` | `GetCertificate` | Treat certificate material and CSR/download fields as secret unless explicitly modeled as public metadata. |
| `zia/custom-file-types` | `github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/filetypecontrol/custom_file_types` | `CustomFileTypes` | `GetCustomFileTypes` | `Get` | Useful companion to file-type rules; inspect file-extension and pattern fields conservatively. |

Scaffold commands:

```sh
make scaffold-resource PRODUCT=zia RESOURCE=intermediate-ca-certificates PACKAGE=github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/intermediatecacertificates TYPE=IntermediateCACertificate FORCE=1
make scaffold-resource PRODUCT=zia RESOURCE=custom-file-types PACKAGE=github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/filetypecontrol/custom_file_types TYPE=CustomFileTypes FORCE=1
```

## Needs A Shape Decision Before Applying

These SDK surfaces are potentially valuable, but they do not fit the current
list/get resource model cleanly. Do not rush them into the catalog until the
reader shape is explicit.

| Candidate | Reason to pause |
| --- | --- |
| Singleton settings resources such as advanced settings, auth settings, malware protection, mobile threat settings, secure browsing, and security policy settings | They are read-only but not list resources. They likely need a singleton reader pattern and manifest semantics before cataloging. |
| Browser isolation profiles | SDK exposes `GetAll` and name lookup, but no integer `Get`; decide whether list-only resources are allowed before enabling. |
| PAC files | SDK exposes versioned/list functions that do not match the current `list`/`get <id>` model directly. |
| Cloud application policy lists | SDK functions take parameter maps; decide stable defaults before exposing. |

## Deferred After Live Smoke Failures

These were generated and locally validated, then removed after live smoke
reported list/request failures. Do not retry as ordinary batch work; investigate
endpoint behavior and auth-mode support first.

| Resource | Observed status |
| --- | --- |
| `zia/network-service-groups` | Request failure in the first policy-reference batch. |
| `zia/network-applications` | Request failure while `zia/network-application-groups` succeeded. |
| `zia/departments` | List request failure under ZIA legacy credentials. |
| `zia/users` | List request failure under ZIA legacy credentials. |
| `zia/devices` | List request failure under ZIA legacy credentials. |
| `zia/email-profiles` | List request failure under ZIA legacy credentials. |
| `zia/dlp-engines` | List request failure under ZIA legacy credentials. |
| `zia/dlp-dictionaries` | List request failure under ZIA legacy credentials. |
| `zia/dlp-incident-receiver-servers` | List request failure under ZIA legacy credentials. |
| `zia/dlp-notification-templates` | List request failure under ZIA legacy credentials. |
| `zia/ips-signature-rules` | List request failure under ZIA legacy credentials. |

## Return-To-Work Checklist

When live smoke is available again:

1. Resolve PR `#33` first with `make live-smoke`.
2. Merge or trim PR `#33`.
3. Regenerate Batch A scaffolds from this queue.
4. Apply only Batch A to production files.
5. Open one draft PR with a focused `live-smoke.manifest`.
6. Repeat only after that PR is merged or trimmed.
