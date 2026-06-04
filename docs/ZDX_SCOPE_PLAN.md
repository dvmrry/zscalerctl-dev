# ZDX Scope Plan

This document records why ZDX is out of pre-`v1.0.0` implementation scope for
`zscalerctl`. It is not an enabled catalog, live-smoke result, entitlement
proof, or implementation plan.

## Verdict

Do not implement ZDX before `v1.0.0` unless Zscaler exposes actual ZDX
configuration-inventory APIs.

The SDK surfaces inspected so far are report, telemetry, user/device activity,
alert, and software inventory APIs. Those reads may be useful, but they are not
the configuration inventory surface this project is prioritizing. They should
not be forced into the current `list|get|dump` model, and they should not be
implemented as a pre-`v1.0.0` feature just because the SDK exposes them.

If Zscaler later surfaces ZDX configured applications, probes, collections,
thresholds, alert rules, or other deterministic configuration objects through
the SDK/API, revisit ZDX as ordinary configuration inventory. Until then, ZDX
is a post-`v1.0.0` report/export track.

## Why ZDX Is Different

The current resource model assumes deterministic read-only configuration:

- catalog specs describe relatively static tenant objects;
- `list` and `get` return projected records;
- `dump` can collect resources without extra query semantics;
- validation compares output shape to an allow-list.

ZDX report APIs are different:

- SDK comments and filters define a default "last 2 hours" report window.
- Many endpoints accept `from`/`to`, location, department, geolocation, offset,
  and limit filters.
- Returned values are telemetry aggregates, metrics, user/device activity, or
  time-series data.
- A record without its time window is incomplete evidence. The same query run
  later can legitimately return different values.

For that reason, forcing ZDX into the existing config dump model would make the
output look more deterministic than it is.

## SDK Findings

Inspected SDK: `github.com/zscaler/zscaler-sdk-go/v3` `v3.8.37`.

### Report Surface Reviewed

`zscaler/zdx/services/reports/applications`

- `GetAllApps(ctx, service, filters)` returns `[]Apps`.
- `GetApp(ctx, service, appID, filters)` returns one `Apps`.
- Both accept `common.GetFromToFilters`.
- SDK comments say the endpoint defaults to the last 2 hours when the time
  range is omitted.
- The functions accept the shared `*zscaler.Service`, so they could fit the
  OneAPI boundary used by ZIA/ZPA/ZTW/ZCC work. That does not make them config
  inventory.

`Apps` fields:

| JSON field | Proposed class | Proposed mode | Notes |
| --- | --- | --- | --- |
| `id` | Operational metadata | `standard`, `share`, `paranoid` | Stable app identifier. |
| `name` | Tenant configuration | `standard`, `share` | Application display/config name. |
| `score` | Operational telemetry | `standard` only | Time-windowed experience score. |
| `total_users` | Operational telemetry | `standard` only | Aggregate user count for the window. |
| `stats.active_users` | Operational telemetry | `standard` only | Aggregate count. |
| `stats.active_devices` | Operational telemetry | `standard` only | Aggregate count. |
| `stats.num_poor` | Operational telemetry | `standard` only | Aggregate health bucket. |
| `stats.num_okay` | Operational telemetry | `standard` only | Aggregate health bucket. |
| `stats.num_good` | Operational telemetry | `standard` only | Aggregate health bucket. |
| `most_impacted_region.*` | Sensitive identifier | `standard` only | Region/city/country/geotype can reveal operational geography. |

No free-text field is present in the application summary type.

### Deferred Report Surfaces

`GetAppScores` and `GetAppMetrics` return time-series data. They remain
post-`v1.0.0` report/export work unless Zscaler exposes deterministic
configuration inventory for the same objects.

`zdx/users` and `zdx/devices` should be deferred. They include user email,
device names, geolocation, hardware, network, software, usernames, hostnames,
serials, MAC/BSSID, and similar endpoint/person data.

`zdx/alerts` should be deferred. Alert details include departments, locations,
geolocations, and affected devices with user IDs, usernames, and user email.

`zdx/software-inventory` should be deferred. It is endpoint-sensitive and has
software-to-user/device drill-down semantics.

`zdx/administration` may be lower risk than users/devices, but it is still a
ZDX-specific helper surface. Hold it unless it proves to expose deterministic
configuration inventory rather than report helpers.

## Future Report Command Model

This section is not a pre-`v1.0.0` implementation plan. It records the minimum
shape required if the project deliberately chooses to add ZDX reports after
`v1.0.0`.

Do not add ZDX reports to default config `dump`.

Preferred future report shape:

```sh
zscalerctl zdx applications report --from <unix-seconds> --to <unix-seconds>
zscalerctl zdx applications report --from <unix-seconds> --to <unix-seconds> --id <app-id>
```

Output should be an envelope, not a bare array:

```json
{
  "schema": "zscalerctl.report.v1",
  "product": "zdx",
  "resource": "applications",
  "window": {
    "requested": {
      "from": 1719864000,
      "to": 1719871200
    },
    "effective": {
      "from": 1719864000,
      "to": 1719871200
    }
  },
  "records": []
}
```

Reasons:

- The report window becomes part of the machine-readable contract.
- Future time-series endpoints can use the same envelope.
- Config `dump` remains deterministic inventory.
- The existing projection/redaction machinery can still protect individual
  records before they enter the report envelope.

`window.requested` records the operator's input. `window.effective` records the
window actually queried after any ZDX/API clamping, quantization, or validation.
If the API does not report a normalized range, a future report implementation
should set `effective` equal to `requested` and document that limitation in the
command help. Do not silently report only the requested range if the
implementation can determine that the server used a different one.

Empty reports are successful report results:

```json
{
  "schema": "zscalerctl.report.v1",
  "product": "zdx",
  "resource": "applications",
  "window": {
    "requested": {"from": 1719864000, "to": 1719871200},
    "effective": {"from": 1719864000, "to": 1719871200}
  },
  "records": []
}
```

An empty `records` array means no records matched the requested/effective
window. It must not be treated as a query failure unless the API request itself
failed.

Rejected shape:

```sh
zscalerctl zdx applications list --from <unix-seconds> --to <unix-seconds>
zscalerctl zdx applications get <app-id> --from <unix-seconds> --to <unix-seconds>
```

Do not use this shape in v1. `list|get` carry config-resource expectations:
deterministic objects, dump eligibility, and future diff compatibility. ZDX
reports are time-windowed telemetry, so the operation name should make that
boundary structural.

Reports are not diffable like configuration resources. Two report outputs are
comparable only when the report type and effective window are intentionally the
same. Different windows are different evidence, not configuration drift.

Closed historical windows may be cacheable in the future, but report caching is
out of scope for v1. The current global no-cache stance remains unchanged.

## Required Implementation Decisions

Future report implementation decisions:

1. ZDX uses a dedicated `report` operation. Do not overload config `list|get`
   for report-shaped output.
2. `from` and `to` are required in v1. Avoid SDK-default "last 2 hours" output
   because it is implicit and non-reproducible.
3. ZDX report resources may appear in `schema list`, but their operation must be
   shown as `report`, not config `list|get`.
4. Report envelopes are not written by config `dump` in v1. Revisit only after
   report artifact retention and evidence semantics are defined.
5. The report envelope uses schema `zscalerctl.report.v1`. Changes to this
   envelope follow the versioning policy for machine-readable output schemas.

## Safety Requirements For Any Future Report Work

Any future ZDX report implementation should keep the existing safety floor:

- Use the shared OneAPI `*zscaler.Service` path only.
- Do not instantiate `zdx.Client` or product-local ZDX config.
- Keep SDK env/file/cache/logger/proxy suppression unchanged.
- Add `ProductZDX` only where explicit ZDX commands need it.
- Keep ZIA legacy credentials fail-closed for ZDX before service construction.
- Add SDK-shape review entries for every mapped ZDX type.
- Add projection canaries for any deferred nested geography or telemetry fields.
- Add live-smoke manifest support before promotion.
- Treat `200-empty` as availability only, not response-shape proof.

## Pre-1.0 Project Posture

No ZDX implementation branch should be opened before `v1.0.0` unless it is
scoping actual configuration-inventory APIs rather than reports. A qualifying
ZDX config surface should meet the same baseline as the other product tracks:

- deterministic tenant configuration object;
- read-only SDK/API path;
- stable `list|get`-like semantics, or an explicitly designed singleton shape;
- allow-list projection and SDK shape review support;
- controlled live-smoke path before promotion.

Report-only work remains post-`v1.0.0` and should use the report envelope
contract above if it is ever prioritized.

## Review Questions

Resolved by review and current project posture:

1. No ZDX report implementation before `v1.0.0`.
2. `id/name` are acceptable in share mode for application summaries; scores,
   counts, and geography would remain standard-only if reports are implemented
   after `v1.0.0`.
3. `zdx/administration` does not follow applications pre-`v1.0.0`; it waits for
   either real config inventory semantics or a post-`v1.0.0` report/export
   decision.
4. Report outputs remain separate artifacts with explicit window metadata. They
   do not join config `dump`.
