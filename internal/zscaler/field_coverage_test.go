package zscaler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestFieldCoverageReportIsCurrent regenerates the committed field-coverage
// report (docs/FIELD_COVERAGE.md and docs/field-coverage.json) in memory from
// reviewedSDKShapes() and the resource catalog, then fails on any drift. This
// makes the coverage numbers un-driftable: a new SDK field, a re-classification,
// or a changed ignore reason that is not reflected in the committed report
// breaks the build. Set FIELD_COVERAGE_WRITE=1 (via `make field-coverage`) to
// rewrite the committed artifacts instead of asserting against them.
//
// It follows the committed-artifact drift-test precedent of
// internal/dump/published_schema_test.go.
func TestFieldCoverageReportIsCurrent(t *testing.T) {
	t.Parallel()

	report := buildFieldCoverageReport(t)
	wantJSON := renderFieldCoverageJSON(t, report)
	wantMarkdown := renderFieldCoverageMarkdown(report)

	jsonPath := filepath.Join("..", "..", "docs", "field-coverage.json")
	mdPath := filepath.Join("..", "..", "docs", "FIELD_COVERAGE.md")

	if os.Getenv("FIELD_COVERAGE_WRITE") == "1" {
		writeFieldCoverageFile(t, jsonPath, wantJSON)
		writeFieldCoverageFile(t, mdPath, wantMarkdown)
		t.Logf("wrote %s and %s", jsonPath, mdPath)
		return
	}

	assertFieldCoverageFile(t, jsonPath, wantJSON)
	assertFieldCoverageFile(t, mdPath, wantMarkdown)

	assertFieldCoverageSanity(t, report)
}

func assertFieldCoverageFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (run `make field-coverage`)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is out of date; run `make field-coverage` to regenerate it", path)
	}
}

func writeFieldCoverageFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// assertFieldCoverageSanity pins a few invariants directly so a generator bug
// that happened to round-trip its own output (write garbage, read garbage) still
// gets caught.
func assertFieldCoverageSanity(t *testing.T, report fieldCoverageReport) {
	t.Helper()

	// locations is the canonical wide SDK struct and the wave-1 expansion
	// flagship: 35 of its 60 fields are classified. These assertions are
	// FLOORS, not exact pins — coverage may only grow, so future promotions
	// pass while an accidental classification regression (a field silently
	// dropped from the catalog) fails loudly.
	var locations *fieldCoverageResource
	for i := range report.Resources {
		r := &report.Resources[i]
		if r.Product == string(resources.ProductZIA) && r.Resource == "locations" {
			locations = r
			break
		}
	}
	if locations == nil {
		t.Fatalf("field coverage report missing zia/locations")
	}
	if locations.ClassifiedFields < 35 {
		t.Errorf("zia/locations classified = %d, want >= 35 (coverage must not regress)", locations.ClassifiedFields)
	}
	if locations.TotalFields != 60 {
		t.Errorf("zia/locations total SDK fields = %d, want 60 (changes only on an SDK bump, which warrants re-review)", locations.TotalFields)
	}

	// Every resource row must be internally consistent: classified + ignored ==
	// total exported SDK fields, and every ignored field must land in exactly
	// one decided-state bucket.
	for _, r := range report.Resources {
		if r.ClassifiedFields+r.IgnoredFields != r.TotalFields {
			t.Errorf("%s/%s classified(%d)+ignored(%d) != total(%d)",
				r.Product, r.Resource, r.ClassifiedFields, r.IgnoredFields, r.TotalFields)
		}
		if r.DeliberateFields+r.DeferredFields != r.IgnoredFields {
			t.Errorf("%s/%s deliberate(%d)+deferred(%d) != ignored(%d)",
				r.Product, r.Resource, r.DeliberateFields, r.DeferredFields, r.IgnoredFields)
		}
	}

	// Totals must be the sum of the resource rows.
	var sumTotal, sumClassified, sumIgnored, sumDeliberate, sumDeferred int
	for _, r := range report.Resources {
		sumTotal += r.TotalFields
		sumClassified += r.ClassifiedFields
		sumIgnored += r.IgnoredFields
		sumDeliberate += r.DeliberateFields
		sumDeferred += r.DeferredFields
	}
	if report.Totals.TotalFields != sumTotal {
		t.Errorf("repo total fields = %d, want %d (sum of resources)", report.Totals.TotalFields, sumTotal)
	}
	if report.Totals.ClassifiedFields != sumClassified {
		t.Errorf("repo classified fields = %d, want %d (sum of resources)", report.Totals.ClassifiedFields, sumClassified)
	}
	if report.Totals.IgnoredFields != sumIgnored {
		t.Errorf("repo ignored fields = %d, want %d (sum of resources)", report.Totals.IgnoredFields, sumIgnored)
	}
	if report.Totals.DeliberateFields != sumDeliberate {
		t.Errorf("repo deliberate fields = %d, want %d (sum of resources)", report.Totals.DeliberateFields, sumDeliberate)
	}
	if report.Totals.DeferredFields != sumDeferred {
		t.Errorf("repo deferred fields = %d, want %d (sum of resources)", report.Totals.DeferredFields, sumDeferred)
	}

	// Ratchet: deferred reached zero in the field-coverage workstream and must
	// stay there. Every new SDK field must be classified in the catalog or
	// recorded as ignored with a "deliberate: " reason before merge — parking a
	// field as "deferred: " is no longer a quiet option. If a future wave
	// intentionally parks work, this assertion is the visible thing it edits.
	if report.Totals.DeferredFields != 0 {
		t.Errorf("repo deferred fields = %d, want 0: every SDK field must be classified or recorded as \"deliberate: \" before merge; if a wave intentionally parks work as \"deferred: \", it must edit this assertion to say so", report.Totals.DeferredFields)
	}

	// With zero deferred fields, decided coverage (classified + deliberate over
	// total) is exactly 100.0 repo-wide. Derived from the report struct, not the
	// rendered markdown, so a renderer bug cannot fake it.
	if report.Totals.DecidedCoveragePercent != 100.0 {
		t.Errorf("repo decided coverage = %.1f%%, want 100.0%%: some SDK field is neither classified nor deliberately excluded", report.Totals.DecidedCoveragePercent)
	}
}

// fieldCoverageReport is the full machine-readable report. JSON field order is
// fixed by struct order and all slices are sorted deterministically, so the
// rendered artifact is byte-stable across runs.
type fieldCoverageReport struct {
	Schema    string                  `json:"schema"`
	Totals    fieldCoverageTotals     `json:"totals"`
	Products  []fieldCoverageProduct  `json:"products"`
	Resources []fieldCoverageResource `json:"resources"`
}

type fieldCoverageTotals struct {
	Resources              int     `json:"resources"`
	TotalFields            int     `json:"total_fields"`
	ClassifiedFields       int     `json:"classified_fields"`
	IgnoredFields          int     `json:"ignored_fields"`
	DeliberateFields       int     `json:"deliberate_fields"`
	DeferredFields         int     `json:"deferred_fields"`
	CoveragePercent        float64 `json:"coverage_percent"`
	DecidedCoveragePercent float64 `json:"decided_coverage_percent"`
}

type fieldCoverageProduct struct {
	Product                string  `json:"product"`
	Resources              int     `json:"resources"`
	TotalFields            int     `json:"total_fields"`
	ClassifiedFields       int     `json:"classified_fields"`
	IgnoredFields          int     `json:"ignored_fields"`
	DeliberateFields       int     `json:"deliberate_fields"`
	DeferredFields         int     `json:"deferred_fields"`
	CoveragePercent        float64 `json:"coverage_percent"`
	DecidedCoveragePercent float64 `json:"decided_coverage_percent"`
}

type fieldCoverageResource struct {
	Product                string                `json:"product"`
	Resource               string                `json:"resource"`
	SDKTypes               []string              `json:"sdk_types"`
	TotalFields            int                   `json:"total_fields"`
	ClassifiedFields       int                   `json:"classified_fields"`
	IgnoredFields          int                   `json:"ignored_fields"`
	DeliberateFields       int                   `json:"deliberate_fields"`
	DeferredFields         int                   `json:"deferred_fields"`
	CoveragePercent        float64               `json:"coverage_percent"`
	DecidedCoveragePercent float64               `json:"decided_coverage_percent"`
	ClassBreakdown         []fieldCoverageClass  `json:"class_breakdown"`
	IgnoredByReason        []fieldCoverageReason `json:"ignored_by_reason"`
}

type fieldCoverageClass struct {
	Class string `json:"class"`
	Count int    `json:"count"`
}

type fieldCoverageReason struct {
	Reason string   `json:"reason"`
	Bucket string   `json:"bucket"`
	Fields []string `json:"fields"`
}

const fieldCoverageSchema = "zscalerctl/field-coverage/v2"

// fieldCoverageBucket maps an ignore reason to its decided-state bucket from
// the mandatory reason prefix that assertReviewed enforces. The empty-string
// fallback never reaches the rendered report: buildFieldCoverageReport fails
// the test on it so a convention violation cannot serialize silently.
func fieldCoverageBucket(reason string) string {
	switch {
	case strings.HasPrefix(reason, ignoreReasonDeliberatePrefix):
		return "deliberate"
	case strings.HasPrefix(reason, ignoreReasonDeferredPrefix):
		return "deferred"
	default:
		return ""
	}
}

// buildFieldCoverageReport walks every reviewed SDK shape that is attributed to
// a catalog read-resource and aggregates per (product, resource): the SDK types
// whose exported fields the readers traverse, the classified field count (with a
// per-class breakdown taken from the catalog spec), and the ignored field count
// grouped by the registry's reason strings. Resources with multiple reviewed
// shapes (a top-level struct plus nested helper structs attributed to the same
// resource) are merged into one row.
func buildFieldCoverageReport(t *testing.T) fieldCoverageReport {
	t.Helper()

	type accumulator struct {
		product         resources.Product
		resource        string
		sdkTypes        map[string]struct{}
		total           int
		classCounts     map[string]int
		classified      int
		ignored         int
		deliberate      int
		deferred        int
		ignoredByReason map[string]map[string]struct{}
	}

	byResource := map[string]*accumulator{}

	for _, shape := range reviewedSDKShapes() {
		if shape.resource == "" && shape.resourceName == "" {
			// Nested helper struct not attributed to a resource (its fields are
			// covered/dropped by a parent). Not a resource row of its own.
			continue
		}

		key := resourceReviewKey(shape.resource, shape.resourceName)
		acc := byResource[key]
		if acc == nil {
			acc = &accumulator{
				product:         shape.resource,
				resource:        shape.resourceName,
				sdkTypes:        map[string]struct{}{},
				classCounts:     map[string]int{},
				ignoredByReason: map[string]map[string]struct{}{},
			}
			byResource[key] = acc
		}

		acc.sdkTypes[shape.name] = struct{}{}

		classByName := catalogClassByName(t, shape.resource, shape.resourceName)
		exported := exportedJSONFields(shape.typ)
		classifiedSet := namesSet(shape.catalogFields)
		acc.total += len(exported)

		for _, field := range exported {
			if _, ok := classifiedSet[field]; ok {
				acc.classified++
				class := classByName[field]
				if class == "" {
					class = "unknown"
				}
				acc.classCounts[class]++
				continue
			}
			if reason, ok := shape.ignoredFields[field]; ok {
				acc.ignored++
				switch fieldCoverageBucket(reason) {
				case "deliberate":
					acc.deliberate++
				case "deferred":
					acc.deferred++
				default:
					t.Errorf("%s/%s SDK type %s ignored field %q reason %q has no deliberate:/deferred: prefix; the report cannot bucket it",
						shape.resource, shape.resourceName, shape.name, field, reason)
				}
				if acc.ignoredByReason[reason] == nil {
					acc.ignoredByReason[reason] = map[string]struct{}{}
				}
				acc.ignoredByReason[reason][field] = struct{}{}
				continue
			}
			// An exported field neither classified nor ignored is a registry gap
			// that the existing assertReviewed guard already fails on; surface it
			// here too so the report cannot silently undercount.
			t.Errorf("%s/%s SDK type %s field %q is neither classified nor ignored",
				shape.resource, shape.resourceName, shape.name, field)
		}
	}

	resourcesOut := make([]fieldCoverageResource, 0, len(byResource))
	for _, acc := range byResource {
		sdkTypes := mapKeysSorted(acc.sdkTypes)

		classBreakdown := make([]fieldCoverageClass, 0, len(acc.classCounts))
		for class, count := range acc.classCounts {
			classBreakdown = append(classBreakdown, fieldCoverageClass{Class: class, Count: count})
		}
		sort.Slice(classBreakdown, func(i, j int) bool {
			return classBreakdown[i].Class < classBreakdown[j].Class
		})

		ignoredByReason := make([]fieldCoverageReason, 0, len(acc.ignoredByReason))
		for reason, fields := range acc.ignoredByReason {
			ignoredByReason = append(ignoredByReason, fieldCoverageReason{
				Reason: reason,
				Bucket: fieldCoverageBucket(reason),
				Fields: mapKeysSorted(fields),
			})
		}
		sort.Slice(ignoredByReason, func(i, j int) bool {
			return ignoredByReason[i].Reason < ignoredByReason[j].Reason
		})

		resourcesOut = append(resourcesOut, fieldCoverageResource{
			Product:                string(acc.product),
			Resource:               acc.resource,
			SDKTypes:               sdkTypes,
			TotalFields:            acc.total,
			ClassifiedFields:       acc.classified,
			IgnoredFields:          acc.ignored,
			DeliberateFields:       acc.deliberate,
			DeferredFields:         acc.deferred,
			CoveragePercent:        coveragePercent(acc.classified, acc.total),
			DecidedCoveragePercent: coveragePercent(acc.classified+acc.deliberate, acc.total),
			ClassBreakdown:         classBreakdown,
			IgnoredByReason:        ignoredByReason,
		})
	}

	sortFieldCoverageResources(resourcesOut)

	products := aggregateProducts(resourcesOut)
	totals := aggregateTotals(resourcesOut)

	return fieldCoverageReport{
		Schema:    fieldCoverageSchema,
		Totals:    totals,
		Products:  products,
		Resources: resourcesOut,
	}
}

// catalogClassByName builds a flat JSON-field-name to classification map for a
// resource spec, walking nested fields. Nested-helper reviews attributed to a
// resource classify nested field names (e.g. displayName under an id/name/
// displayName subobject), so the recursive walk lets those resolve to a class.
func catalogClassByName(t *testing.T, product resources.Product, name string) map[string]string {
	t.Helper()
	spec, ok := resources.FindSpec(product, name)
	if !ok {
		t.Fatalf("resources.FindSpec(%s, %s) ok = false, want true", product, name)
	}
	out := map[string]string{}
	var walk func(fields []resources.FieldSpec)
	walk = func(fields []resources.FieldSpec) {
		for _, field := range fields {
			out[field.JSONField()] = string(field.Classification)
			walk(field.Fields)
		}
	}
	walk(spec.Fields)
	return out
}

// sortFieldCoverageResources orders rows worst-coverage-first (ascending
// coverage percent), then most-fields-first, then product, then resource, so
// the table puts the biggest classification gaps at the top and the order is
// fully deterministic.
func sortFieldCoverageResources(rs []fieldCoverageResource) {
	sort.Slice(rs, func(i, j int) bool {
		a, b := rs[i], rs[j]
		if a.CoveragePercent != b.CoveragePercent {
			return a.CoveragePercent < b.CoveragePercent
		}
		if a.TotalFields != b.TotalFields {
			return a.TotalFields > b.TotalFields
		}
		if a.Product != b.Product {
			return a.Product < b.Product
		}
		return a.Resource < b.Resource
	})
}

func aggregateProducts(rs []fieldCoverageResource) []fieldCoverageProduct {
	type agg struct {
		resources  int
		total      int
		classified int
		ignored    int
		deliberate int
		deferred   int
	}
	byProduct := map[string]*agg{}
	for _, r := range rs {
		a := byProduct[r.Product]
		if a == nil {
			a = &agg{}
			byProduct[r.Product] = a
		}
		a.resources++
		a.total += r.TotalFields
		a.classified += r.ClassifiedFields
		a.ignored += r.IgnoredFields
		a.deliberate += r.DeliberateFields
		a.deferred += r.DeferredFields
	}
	out := make([]fieldCoverageProduct, 0, len(byProduct))
	for product, a := range byProduct {
		out = append(out, fieldCoverageProduct{
			Product:                product,
			Resources:              a.resources,
			TotalFields:            a.total,
			ClassifiedFields:       a.classified,
			IgnoredFields:          a.ignored,
			DeliberateFields:       a.deliberate,
			DeferredFields:         a.deferred,
			CoveragePercent:        coveragePercent(a.classified, a.total),
			DecidedCoveragePercent: coveragePercent(a.classified+a.deliberate, a.total),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CoveragePercent != out[j].CoveragePercent {
			return out[i].CoveragePercent < out[j].CoveragePercent
		}
		return out[i].Product < out[j].Product
	})
	return out
}

func aggregateTotals(rs []fieldCoverageResource) fieldCoverageTotals {
	var total, classified, ignored, deliberate, deferred int
	for _, r := range rs {
		total += r.TotalFields
		classified += r.ClassifiedFields
		ignored += r.IgnoredFields
		deliberate += r.DeliberateFields
		deferred += r.DeferredFields
	}
	return fieldCoverageTotals{
		Resources:              len(rs),
		TotalFields:            total,
		ClassifiedFields:       classified,
		IgnoredFields:          ignored,
		DeliberateFields:       deliberate,
		DeferredFields:         deferred,
		CoveragePercent:        coveragePercent(classified, total),
		DecidedCoveragePercent: coveragePercent(classified+deliberate, total),
	}
}

// coveragePercent returns classified/total as a percentage rounded to one
// decimal place. Rounding to a fixed precision keeps the rendered artifact
// byte-stable (no long trailing float expansions).
func coveragePercent(classified, total int) float64 {
	if total == 0 {
		return 0
	}
	pct := float64(classified) / float64(total) * 100
	return float64(int64(pct*10+0.5)) / 10
}

func mapKeysSorted(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func renderFieldCoverageJSON(t *testing.T, report fieldCoverageReport) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(report); err != nil {
		t.Fatalf("encode field coverage json: %v", err)
	}
	return buf.Bytes()
}

func renderFieldCoverageMarkdown(report fieldCoverageReport) []byte {
	var b strings.Builder

	b.WriteString("# zscalerctl Field Coverage\n\n")
	b.WriteString("This report is generated. Do not edit by hand; run `make field-coverage`\n")
	b.WriteString("to regenerate it. `TestFieldCoverageReportIsCurrent` fails the build on any\n")
	b.WriteString("drift, so the numbers below cannot silently go stale.\n\n")

	b.WriteString("## What This Measures\n\n")
	b.WriteString("For every read-only resource, the SDK response struct exposes some set of\n")
	b.WriteString("exported fields. Each field is in exactly one of two states:\n\n")
	b.WriteString("- **Classified** — the field is in the resource catalog with an explicit\n")
	b.WriteString("  classification (operational, tenant configuration, sensitive identifier,\n")
	b.WriteString("  free text, or secret) that drives allow-list projection and per-mode\n")
	b.WriteString("  redaction. Only classified fields are eligible to be emitted.\n")
	b.WriteString("- **Ignored, with a reason** — the field is not in the catalog and carries a\n")
	b.WriteString("  non-empty reason string. Ignored fields are **fail-closed dropped**: they\n")
	b.WriteString("  are never emitted in any mode.\n\n")
	b.WriteString("Coverage percent is classified / total exported SDK fields. A low number is\n")
	b.WriteString("not a leak — every unclassified field is dropped — it is a measure of how\n")
	b.WriteString("much of the available SDK surface has been deliberately reviewed and exposed.\n")
	b.WriteString("This report exists so that \"field coverage\" is a verifiable number, not an\n")
	b.WriteString("assurance. See [DATA_CLASSIFICATION.md](DATA_CLASSIFICATION.md) for the class\n")
	b.WriteString("definitions and the fail-closed output rules. The machine-readable companion\n")
	b.WriteString("[field-coverage.json](field-coverage.json) lists every ignored field name and\n")
	b.WriteString("its reason, which feeds field-expansion planning.\n\n")

	b.WriteString("## Ignored Fields Are Decided, Not Vague\n\n")
	b.WriteString("Every ignore reason must begin with one of two prefixes, so each ignored\n")
	b.WriteString("field is in a decided state:\n\n")
	b.WriteString("- **Deliberate** (`deliberate: `) — permanently excluded, with a stated why:\n")
	b.WriteString("  bookkeeping, UI display hints, computed counters, opaque SDK helpers, or\n")
	b.WriteString("  cross-references whose details are documented on another resource.\n")
	b.WriteString("- **Deferred** (`deferred: `) — genuinely not yet classified; the field still\n")
	b.WriteString("  needs future modeling before it can be exposed.\n\n")
	b.WriteString("**Decided coverage** is (classified + deliberate) / total: the share of the\n")
	b.WriteString("SDK surface with a final answer. The end-state goal is **zero deferred\n")
	b.WriteString("fields**, at which point decided coverage reaches 100% and every SDK field\n")
	b.WriteString("is either rendered by classification or permanently excluded on the record.\n\n")

	b.WriteString("## Repo-Wide Totals\n\n")
	fmt.Fprintf(&b, "- Resources: %d\n", report.Totals.Resources)
	fmt.Fprintf(&b, "- Total exported SDK fields: %d\n", report.Totals.TotalFields)
	fmt.Fprintf(&b, "- Classified: %d\n", report.Totals.ClassifiedFields)
	fmt.Fprintf(&b, "- Ignored (fail-closed dropped): %d\n", report.Totals.IgnoredFields)
	fmt.Fprintf(&b, "  - Deliberate (permanently excluded): %d\n", report.Totals.DeliberateFields)
	fmt.Fprintf(&b, "  - Deferred (awaiting modeling): %d\n", report.Totals.DeferredFields)
	fmt.Fprintf(&b, "- Coverage: %s%%\n", formatPercent(report.Totals.CoveragePercent))
	fmt.Fprintf(&b, "- Decided coverage (classified + deliberate): %s%%\n\n", formatPercent(report.Totals.DecidedCoveragePercent))

	b.WriteString("## Per-Product Totals\n\n")
	b.WriteString("Ranked worst coverage first.\n\n")
	b.WriteString("| Product | Resources | Total | Classified | Deliberate | Deferred | Coverage | Decided |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, p := range report.Products {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %s%% | %s%% |\n",
			p.Product, p.Resources, p.TotalFields, p.ClassifiedFields, p.DeliberateFields, p.DeferredFields,
			formatPercent(p.CoveragePercent), formatPercent(p.DecidedCoveragePercent))
	}
	b.WriteString("\n")

	b.WriteString("## Per-Resource Coverage\n\n")
	b.WriteString("Ranked worst coverage first within the whole catalog. `Deliberate` and\n")
	b.WriteString("`Deferred` split the ignored fields by decided state; both are dropped before\n")
	b.WriteString("any output. See [field-coverage.json](field-coverage.json) for the field\n")
	b.WriteString("names, buckets, and reasons behind each row.\n\n")
	b.WriteString("| Product | Resource | Total | Classified | Deliberate | Deferred | Coverage | Decided |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, r := range report.Resources {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %s%% | %s%% |\n",
			r.Product, r.Resource, r.TotalFields, r.ClassifiedFields, r.DeliberateFields, r.DeferredFields,
			formatPercent(r.CoveragePercent), formatPercent(r.DecidedCoveragePercent))
	}
	b.WriteString("\n")

	return []byte(b.String())
}

// formatPercent renders a one-decimal percentage so 100 prints as "100.0" and
// ordering of identical values stays stable.
func formatPercent(pct float64) string {
	return fmt.Sprintf("%.1f", pct)
}
