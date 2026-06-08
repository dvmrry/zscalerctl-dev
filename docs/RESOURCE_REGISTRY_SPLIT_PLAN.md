# Resource Registry Split Plan

## Context
As highlighted in recent pull requests (PR #77 and earlier), the monolithic registry files in `zscalerctl` have grown significantly. The concentration of resource state in large files makes them expensive to review and increases the risk of merge conflicts and mistakes (e.g., resource-name strings, field-mode choices, SDK-to-source mappings).

The files currently affected:
- `internal/resources/resources.go`: owns the catalog, validation, helper constructors, and a huge `Catalog()` literal.
- `internal/zscaler/reader.go`: central handler registry with a very large SDK import block.
- `internal/zscaler/sdk_schema_test.go`: large central SDK-shape registry.

## Objective
Split the monolithic registry files by product or resource family to improve maintainability and ease of review before the next major resource batch. This is purely a maintainability refactor with no behavior changes.

## Proposed Structure

### 1. Catalog Split (`internal/resources/`)
Extract the `ResourceCatalog` elements by product.
- `internal/resources/catalog_zia.go`
- `internal/resources/catalog_ztw.go`
- `internal/resources/resources.go` (Keep common validation, structs, and the `Catalog()` function that aggregates the split catalogs).

**Example `Catalog()` Implementation:**
```go
func Catalog() ResourceCatalog {
    var catalog ResourceCatalog
    catalog = append(catalog, catalogZIA()...)
    catalog = append(catalog, catalogZTW()...)
    return catalog
}
```

### 2. Reader Split (`internal/zscaler/`)
Extract the SDK reader handlers by product.
- `internal/zscaler/reader_zia.go`
- `internal/zscaler/reader_ztw.go`
- `internal/zscaler/reader.go` (Keep the main struct, common helper functions, and the handler map aggregation).

### 3. Schema Test Split (`internal/zscaler/`)
Split the `reviewedSDKShapes()` list by product.
- `internal/zscaler/schema_zia_test.go`
- `internal/zscaler/schema_ztw_test.go`
- `internal/zscaler/sdk_schema_test.go` (Test logic without the massive literals).

## Execution Plan
1. **Refactor Catalog:** Move ZIA and ZTW resources from `Catalog()` into `catalogZIA()` and `catalogZTW()` within their respective files.
2. **Refactor Reader:** Move SDK handler bindings for ZIA and ZTW into separate files, maintaining the central `ResourceReader` interface.
3. **Refactor Schema Tests:** Move SDK shapes into product-specific files and aggregate them in the test run.
4. **Validation:** Run all existing tests, including `make check`, `go test ./...`, and `verify-sdk-boundary.sh` to ensure no behavioral changes.

This split should be executed in a dedicated refactor PR before adding any new resource batches.
