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

For full-product scouting beyond the committed vendor tree, point the script at
the SDK module cache:

```sh
SDK_DIR="$(go list -m -f '{{.Dir}}' -mod=mod github.com/zscaler/zscaler-sdk-go/v3)"
go run ./scripts/sdk-surface-inventory.go --sdk-dir "$SDK_DIR" --format json
```

The script parses Go source under `vendor/github.com/zscaler/zscaler-sdk-go/v3`
with the Go AST. It records exported structs, exported read-like functions,
mutating-looking functions, exported functions with unknown verbs, name/method
ambiguities, static endpoint literals, and product/client packages. It is
intentionally conservative: SDK packages that contain both read and write
helpers are marked as mixed, and zscalerctl must wire only the read functions.
The JSON output carries the same scout-only notice and SDK provenance as the
Markdown output so generated inventory cannot be mistaken for validation data.

## Current Findings

As of the module-cache SDK `github.com/zscaler/zscaler-sdk-go/v3@v3.8.37`:

- ZIA and ZPA have broad high-level service package trees.
- ZTW has the next strongest config-like service tree and is the best candidate
  for a separate Cloud Connector / workload-oriented product track.
- ZCC has useful read-like service packages, but many sit beside mutating
  helpers or device/privacy-sensitive data.
- ZDX exposes report, alert, device, user, and application telemetry surfaces.
  Treat it as out of pre-`v1.0.0` scope unless Zscaler exposes deterministic
  configuration APIs.
- Zidentity is exposed under `zscaler/zid/services/...`; treat it as a partial
  product track. The current vendored service packages are resource servers,
  groups, and users. They are administrator-visible read inventory when bound
  through read-only handlers; group/user membership child lookups need a
  separate command shape and must not wire adjacent membership mutators.
- ZWA is exposed under `zscaler/zwa/services/...`; treat customer audit and DLP
  incident surfaces as out of pre-`v1.0.0` scope unless Zscaler exposes
  deterministic configuration APIs.

These findings are SDK-shape evidence only. They do not prove entitlement,
tenant availability, pagination behavior, or real response shape.

For the broader SDK-module-cache scout across ZCC, ZDX, ZTW, Zidentity, and
ZWA, see [Zscaler Product Scope Plan](ZSCALER_PRODUCT_SCOPE_PLAN.md).

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
4. Move packages with unknown exported functions, ambiguous function signals,
   `read-only-nonstandard`, singleton, parameterized, identity-plane, versioned,
   file-like, or client/config surfaces to shape-decision work.
5. Use dev OneAPI only as endpoint availability scouting unless production
   OneAPI smoke has been explicitly run.

This keeps "ZCC/ZDX/ZTW/Zidentity/ZWA exists in the SDK" separate from "this
resource is safe to expose in zscalerctl before `v1.0.0`."
