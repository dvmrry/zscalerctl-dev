# Resource Queue

This queue is the staging area for future read-only resources. It is not the
enabled catalog. Entries here do not expose new API surface, do not change
live-smoke coverage, and do not replace the catalog, SDK shape review, or live
smoke gates.

Use this file to avoid branch sprawl while live tenant testing is unavailable:
record candidates, scaffold intent, and known live-smoke outcomes here; apply
only one small batch to production files when a live smoke operator is
available. Queue entries are not validation evidence.

## Operating Rules

- Queue only read/list/get surfaces. Write, update, delete, activate, or import
  SDK functions are not resource-queue candidates.
- Keep at most one active resource PR open for live smoke.
- Prefer one resource per PR unless the resources are tightly coupled. If a PR
  contains more than one resource, record pass/fail outcomes per resource and
  trim failed resources instead of blocking proven ones.
- Do not merge resource PRs without a focused `make live-smoke` pass.
- Do not stack un-smoked resource branches behind an unmerged resource PR.
- While live-smoke access is unavailable, do not apply queued resource
  scaffolds to production files. Use the time for SDK scouting, queue
  refinement, and shape-decision notes only.
- Do not commit generated scaffold bundles from `scratch/resource-drafts/`.
- Regenerate scaffolds from current SDK source and current generator code when a
  batch is ready to apply. Do not replay stale commands blindly.
- For non-ZIA or unclear surfaces, run `make sdk-surface-inventory` first and
  record the category from `docs/SDK_SURFACE_INVENTORY.md` before queueing.
- Keep failed live-smoke endpoints in the deferred list until their endpoint or
  auth-mode behavior is understood.
- Do not use dev-tenant OneAPI availability to demote resources already proven
  under the current production legacy path. Production OneAPI becomes
  authoritative only after a controlled production smoke run.
- Treat live-smoke artifacts as confidential operational records. Store,
  retain, or dispose of them only according to the operator's approved records
  and evidence-handling policy.
- When recording live-smoke results, include the resource, auth mode,
  tenant/cloud class, date, commit, and whether the run used source or a release
  binary. Keep authorization or change-record references outside this repo if
  required by the operator's environment.

## Validation States

Use these state names when recording resource status so nobody has to infer
what "validated" means:

| State | Meaning |
| --- | --- |
| `queued` | Candidate recorded here only; no catalog, reader, or smoke surface exists. |
| `scaffolded` | Generated locally or in a draft PR; ordinary gates may pass, but no live tenant proof exists. |
| `gates-passed` | Unit, shape, projection, redaction, and smoke-script self-tests pass without live tenant evidence. |
| `dev-oneapi-availability:<status>` | Dev OneAPI returned an availability signal such as `200-records`, `200-empty`, `400`, `401`, `403`, or `404`. This is not production shape validation. |
| `legacy-zia-smoke-pass:<date>` | Focused live smoke passed with explicit ZIA legacy credentials in the current production-like environment. |
| `prod-oneapi-smoke-pass:<date>` | Focused live smoke passed with a dedicated read-only production OneAPI client. This is the canonical future validation state. |
| `deferred:<reason>` | Resource was removed or paused because endpoint behavior, auth support, shape, or entitlement is not understood. |
| `unsupported:<auth-mode>/<reason>` | Resource is supported in another auth mode, but this auth mode failed or is not expected to work. |

## Auth And Environment Posture

OneAPI is the expansion target, but legacy ZIA remains the current
production-proven path until production OneAPI is available and smoked. Do not
remove, downgrade, or mark legacy-proven resources unsupported based only on
dev OneAPI results.

Dev `zscalertwo` OneAPI is useful for endpoint availability scouting:

| Result | Interpretation |
| --- | --- |
| `200` with records | Endpoint and auth path work in dev; response shape may be useful but is not production proof. |
| `200` with an empty array | Endpoint and auth path work, but shape validation is weak. |
| `400` | Likely parameter/default mismatch; inspect before cataloging. |
| `401` | Auth/config failure. |
| `403` | Permission or role issue. |
| `404` | Not entitled, unavailable in that tenant/cloud, wrong path, or SDK mismatch; do not treat as a permanent resource failure without another signal. |

Production OneAPI smoke should be a controlled evidence run: dedicated
read-only client, approved workstation, no CI secrets, no committed artifacts,
and artifact handling governed by the operator's records policy. Treat any
required change ticket or authorization record as an external operational
control, not as repository content.

## Current Gate

Open draft PR:

| PR | Resources | Status | Smoke command |
| --- | --- | --- | --- |
| `#33` | `zia/risk-profiles`, `zia/nss-servers` | Draft rebased onto current `main`; parked until work-machine live smoke is available | `make live-smoke` |

Do not start applying the next batch until this PR is either merged or trimmed
and merged. PR `#33` predates the preferred one-resource PR rule; record smoke
outcomes for each resource independently and trim only the failing resource if
they diverge.

## No-Live Work Mode

When read-only tenant credentials are unavailable, do not create more
production resource branches behind the open draft PR. Safe work during this
period:

- refresh this queue from `make sdk-surface-inventory`;
- scout the full SDK module cache with:

  ```sh
  SDK_DIR="$(go list -m -f '{{.Dir}}' -mod=mod github.com/zscaler/zscaler-sdk-go/v3)"
  go run ./scripts/sdk-surface-inventory.go --sdk-dir "$SDK_DIR"
  ```

- add or refine batch notes, shape-decision notes, and deferred-resource notes;
- regenerate scratch scaffolds locally for review, but do not commit
  `scratch/resource-drafts/` or apply them to production files until live smoke
  is available again.

## Next Apply Batches

These batches are ordered for small, focused PRs. Each batch should become a
series of one-resource production changes unless the resources are deliberately
paired. Do not start the next batch until the preceding draft PR is resolved.

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

### Batch B: DNS Policy Rules

This is a useful policy surface but likely to be noisier. Keep it separate from
Batch A so a single endpoint failure can be trimmed without losing the whole
policy-rule wave.

| Resource | SDK package | SDK type | List | Get | Notes |
| --- | --- | --- | --- | --- | --- |
| `zia/firewall-dns-rules` | `github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewalldnscontrolpolicies` | `FirewallDNSRules` | `GetAll` | `Get` | DNS policy rules; expect location, group, source/destination, gateway, and ZPA IP group references. |

Scaffold commands:

```sh
make scaffold-resource PRODUCT=zia RESOURCE=firewall-dns-rules PACKAGE=github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/firewalldnscontrolpolicies TYPE=FirewallDNSRules FORCE=1
```

### Batch C: Custom File References

This may need a small custom list closure because the SDK function name is not
exactly `GetAll`, but it still looks like a read-only resource-shaped surface.

| Resource | SDK package | SDK type | List | Get | Notes |
| --- | --- | --- | --- | --- | --- |
| `zia/custom-file-types` | `github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/filetypecontrol/custom_file_types` | `CustomFileTypes` | `GetCustomFileTypes` | `Get` | Useful companion to file-type rules; inspect file-extension and pattern fields conservatively. |

Scaffold commands:

```sh
make scaffold-resource PRODUCT=zia RESOURCE=custom-file-types PACKAGE=github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/filetypecontrol/custom_file_types TYPE=CustomFileTypes FORCE=1
```

## Scouted Backlog

The following candidates came from a full SDK module-cache scout, not from the
current vendored import set. They are queue evidence only. Re-run the scaffold
commands from current SDK source before applying any of them.

### Batch D: Traffic Capture Rules

This looks resource-shaped in the SDK, but it is a policy/control surface with
nested references and filter options. Keep the first pass conservative and use a
small custom list closure if the SDK requires explicit default options.

| Resource | SDK package | SDK type | List | Get | Notes |
| --- | --- | --- | --- | --- | --- |
| `zia/traffic-capture-rules` | `github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/traffic_capture` | `TrafficCaptureRules` | `GetAll` | `Get` | Policy/control surface; expect labels and capture-specific nested references. |

### Batch E: Remaining ZIA Traffic Forwarding References

These are read-like traffic-forwarding references. Avoid `vpncredentials` in
ordinary batch work; it is credential-bearing by name and should be treated as a
separate secret-material decision.

| Resource | SDK package | SDK type | List | Get | Notes |
| --- | --- | --- | --- | --- | --- |
| `zia/zpa-gateways` | `github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/forwarding_control_policy/zpa_gateways` | `ZPAGateways` | `GetAll` | `Get` | ZPA gateway references used by forwarding policy. |
| `zia/extranets` | `github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/trafficforwarding/extranet` | `Extranet` | `GetAll` | `Get` | External connectivity metadata; inspect nested refs conservatively. |

### Future Non-ZIA Tracks

These tracks should not be mixed into ZIA or ZPA batch work. Each needs
product/auth design and a controlled OneAPI smoke path before any production
resource PR.

See [Zscaler Product Scope Plan](ZSCALER_PRODUCT_SCOPE_PLAN.md) for the full
SDK scout across ZCC, ZDX, ZTW, Zidentity, and ZWA. The short version:

| Product | Candidate surface | Scout result | Queue posture |
| --- | --- | --- | --- |
| ZPA | `servergroup`, `segmentgroup`, `appservercontroller`, `appconnectorgroup`, `cloud_connector`, `cloud_connector_group`, `branch_connector`, `machinegroup`, `postureprofile`, `trustednetwork`, `idpcontroller` | Many ordinary or list/get-with-mutating-neighbor SDK packages exist in the full SDK. | OneAPI-only future track; start with one low-risk reference after production OneAPI smoke is available. |
| ZTW | Workload groups, public cloud accounts, gateways, DNS gateways, EC groups, policy resources | Full SDK exposes many list/get-like config surfaces. | Best first separate product track. Start with workload groups; defer provisioning API keys/URLs and policy rules. |
| ZCC | Trusted networks, notification templates, ZIA posture, IP/process app references | Closest Client Connector fit for configuration inventory; several packages sit next to mutating helpers. | Second separate product track after ZTW. Defer devices and secret packages. |
| ZDX | Application, user, device, alert, and software reports | SDK exposes report/read surfaces rather than config inventory resources. | Explicitly out of pre-`v1.0.0` scope unless Zscaler exposes deterministic configuration APIs; see [ZDX Scope Plan](ZDX_SCOPE_PLAN.md). |
| Zidentity | Resource servers, entitlements, users, groups | SDK exposes a thin read-only config slice next to sensitive identity-management APIs. | Partial track only. `resource_servers` is the clean pre-`v1.0.0` candidate; `user_entitlement` is sensitive later work; users/groups are hard-deferred. |
| ZWA | Customer audit and DLP incidents | SDK exposes audit/incident surfaces rather than config inventory resources. | Explicitly out of pre-`v1.0.0` scope unless Zscaler exposes deterministic configuration APIs. |

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
| CASB SaaS Security API rules | `GetAll` exists, but `GetByRuleID` requires a rule-type parameter in addition to ID. Decide stable get semantics before exposing. |
| DC exclusions | SDK exposes `GetAll` and name lookup only. Decide whether list-only/name-get resources are allowed before enabling. |
| Intermediate CA certificates | Certificate and CSR/download fields need a public-metadata versus material/export decision before cataloging. |
| IPS policies | Adjacent to the deferred `zia/ips-signature-rules` endpoint; confirm the endpoint and entitlement behavior is genuinely distinct before applying. |
| Sub-clouds | `GetAll` returns `SubClouds`, but integer `Get` returns `SubCloudCountryDCExclusionInfo`; decide whether this is one resource, two resources, or list-only metadata. |
| ZIA VPN credentials | SDK exposes read-like functions, but the package and fields are credential-bearing by name. Decide whether any public metadata can render before cataloging. |
| ZCC/ZDX/ZTW/Zidentity/ZWA products | Full SDK evidence exists, but product auth, live-smoke commands, and output semantics are not established in this tool. Keep product tracks separate from ZIA and ZPA breadth work; see `docs/ZSCALER_PRODUCT_SCOPE_PLAN.md`. |

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
