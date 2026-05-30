# SDK Surface Inventory

`scripts/sdk-surface-inventory.go` is a scouting helper for the vendored
Zscaler SDK. It does not enable resources and does not replace catalog review,
SDK shape review, or live smoke. Its job is to answer "what kind of SDK surface
is this?" before a resource is queued.

Run:

```sh
make sdk-surface-inventory
make sdk-surface-inventory FORMAT=json
```

The script parses Go source under `vendor/github.com/zscaler/zscaler-sdk-go/v3`
with the Go AST. It records exported structs, exported read-like functions,
mutating-looking functions, static endpoint literals, and product/client
packages. It is intentionally conservative: SDK packages that contain both read
and write helpers are marked as mixed, and zscalerctl must wire only the read
functions.

## Current Findings

As of the vendored SDK currently in this repository:

- ZIA has the only broad high-level service package tree:
  `zscaler/zia/services/...`.
- ZPA, ZCC, ZDX, and ZTW have product client/config packages in this SDK
  snapshot, but not comparable high-level resource service package trees.
- Zidentity/admin routing appears in the core OneAPI client through
  `/admin/api/v1` URL handling. Treat it as identity-plane work, not as ordinary
  resource expansion.
- Cloud Connector concepts appear in existing ZIA policy, location, and
  workload fields. A distinct Cloud Connector API surface should be mapped from
  SDK evidence before being queued.

These findings are SDK-shape evidence only. They do not prove entitlement,
tenant availability, pagination behavior, or real response shape.

## Categories

| Category | Meaning |
| --- | --- |
| `ordinary-list-get` | Package has read-like list and get functions and no mutating-looking exported helpers. |
| `list-get-with-mutating-neighbors` | Package has list/get functions but also mutating-looking SDK helpers. zscalerctl may still use the read functions, but the mapper must stay read-only by construction. |
| `read-only-nonstandard` | Package has read functions, but not the ordinary list/get shape. It likely needs custom resource semantics. |
| `mixed-read-write-sdk-package` | Package contains both read and mutating helpers without a clean resource shape. Treat as manual design work. |
| `product-client-config` | Product client or configuration package, not a high-level resource wrapper. |
| `types-or-client-config` | Exported types exist, but no read resource functions were detected. |
| `mutating-only` | Mutating-looking functions were detected without read functions. Not a resource-queue candidate. |
| `other` | Package did not match the resource heuristics. |

## How To Use The Inventory

Use the inventory before adding non-ZIA surfaces:

1. Run `make sdk-surface-inventory`.
2. Look for the product/package you want to explore.
3. Queue only `ordinary-list-get` or carefully reviewed
   `list-get-with-mutating-neighbors` read surfaces.
4. Move `read-only-nonstandard`, singleton, parameterized, identity-plane,
   versioned, file-like, or client/config surfaces to shape-decision work.
5. Use dev OneAPI only as endpoint availability scouting unless production
   OneAPI smoke has been explicitly run.

This keeps "ZCC/ZDX/ZTW/Zidentity exists in the SDK" separate from "this
resource is safe to expose in zscalerctl."
