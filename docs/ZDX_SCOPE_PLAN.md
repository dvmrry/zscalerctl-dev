# ZDX Scope Plan

This document records why ZDX is out of pre-`v1.0.0` implementation scope for
`zscalerctl`. It is not an enabled catalog, live-smoke result, entitlement
proof, or implementation plan.

## Verdict

Do not implement ZDX before `v1.0.0` unless Zscaler exposes deterministic ZDX
configuration-inventory APIs.

The SDK surfaces inspected so far are report, telemetry, user/device activity,
alert, troubleshooting, administration helper, and software inventory APIs.
Those reads may be useful for an operator, but they are not the deterministic
configuration inventory surface this project is prioritizing. They should not be
forced into the current `list|get|dump` model, and they should not be
implemented as a pre-`v1.0.0` feature just because the SDK exposes them.

If Zscaler later surfaces ZDX configured applications, probes, collections,
thresholds, alert rules, or other deterministic configuration objects through
the SDK/API, revisit ZDX under the same catalog, projection, smoke, and dump
rules as other products.

## Why ZDX Is Different

The current resource model assumes deterministic read-only configuration:

- catalog specs describe relatively static tenant objects;
- `list` and `get` return projected records;
- `dump` can collect resources without extra query semantics;
- validation compares output shape to an allow-list.

ZDX report APIs are different:

- SDK comments and filters define default report windows.
- Many endpoints accept time ranges, location, department, geolocation, offset,
  and limit filters.
- Returned values are telemetry aggregates, metrics, user/device activity, or
  time-series data.
- A record without its time window is incomplete evidence. The same query run
  later can legitimately return different values.

For that reason, forcing ZDX into the existing config dump model would make the
output look more deterministic than it is.

## SDK Findings

Inspected SDK: `github.com/zscaler/zscaler-sdk-go/v3` `v3.8.37`.

| SDK surface | Finding | Scope posture |
| --- | --- | --- |
| `zscaler/zdx/services/reports/applications` | `GetAllApps` and `GetApp` return application experience report records and accept time-window filters. | Report/telemetry, not config inventory. |
| `zscaler/zdx/services/reports/users` | User activity and experience report data. | Privacy-sensitive report surface. |
| `zscaler/zdx/services/reports/devices` | Device telemetry, hardware, network, and metric data. | Endpoint-sensitive report surface. |
| `zscaler/zdx/services/alerts` | Alert and affected-device/user details. | Time-windowed alert data, not config inventory. |
| `zscaler/zdx/services/inventory` | Software inventory and user/device drill-down semantics. | Endpoint-sensitive inventory/report surface. |
| `zscaler/zdx/services/administration` | Department and location dimension reads used by reports. | Confirmed report-filter dimensions, not configuration inventory. |

## Future Exception Criteria

No ZDX implementation branch should be opened before `v1.0.0` unless it scopes
actual configuration-inventory APIs rather than report or telemetry reads. A
qualifying ZDX config surface should meet the same baseline as other product
tracks:

- deterministic tenant configuration object;
- read-only SDK/API path;
- stable `list|get`-like semantics, or an explicitly designed singleton shape;
- allow-list projection and SDK shape review support;
- controlled live-smoke path before promotion.

Report-only work remains out of scope for pre-`v1.0.0`. If the project later
chooses to add report/export commands, design that as a separate operation and
artifact family. Do not mix report outputs into configuration `dump`.
