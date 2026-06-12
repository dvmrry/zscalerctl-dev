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

	// locations is the canonical narrow-identity-surface resource: it classifies
	// a small set and intentionally ignores the rest of a wide SDK struct. This
	// must track the registry in schema_zia_test.go.
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
	if locations.ClassifiedFields != 5 {
		t.Errorf("zia/locations classified = %d, want 5", locations.ClassifiedFields)
	}
	if locations.IgnoredFields < 35 {
		t.Errorf("zia/locations ignored = %d, want >= 35", locations.IgnoredFields)
	}

	// Every resource row must be internally consistent: classified + ignored ==
	// total exported SDK fields.
	for _, r := range report.Resources {
		if r.ClassifiedFields+r.IgnoredFields != r.TotalFields {
			t.Errorf("%s/%s classified(%d)+ignored(%d) != total(%d)",
				r.Product, r.Resource, r.ClassifiedFields, r.IgnoredFields, r.TotalFields)
		}
	}

	// Totals must be the sum of the resource rows.
	var sumTotal, sumClassified, sumIgnored int
	for _, r := range report.Resources {
		sumTotal += r.TotalFields
		sumClassified += r.ClassifiedFields
		sumIgnored += r.IgnoredFields
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
	Resources        int     `json:"resources"`
	TotalFields      int     `json:"total_fields"`
	ClassifiedFields int     `json:"classified_fields"`
	IgnoredFields    int     `json:"ignored_fields"`
	CoveragePercent  float64 `json:"coverage_percent"`
}

type fieldCoverageProduct struct {
	Product          string  `json:"product"`
	Resources        int     `json:"resources"`
	TotalFields      int     `json:"total_fields"`
	ClassifiedFields int     `json:"classified_fields"`
	IgnoredFields    int     `json:"ignored_fields"`
	CoveragePercent  float64 `json:"coverage_percent"`
}

type fieldCoverageResource struct {
	Product          string                `json:"product"`
	Resource         string                `json:"resource"`
	SDKTypes         []string              `json:"sdk_types"`
	TotalFields      int                   `json:"total_fields"`
	ClassifiedFields int                   `json:"classified_fields"`
	IgnoredFields    int                   `json:"ignored_fields"`
	CoveragePercent  float64               `json:"coverage_percent"`
	ClassBreakdown   []fieldCoverageClass  `json:"class_breakdown"`
	IgnoredByReason  []fieldCoverageReason `json:"ignored_by_reason"`
}

type fieldCoverageClass struct {
	Class string `json:"class"`
	Count int    `json:"count"`
}

type fieldCoverageReason struct {
	Reason string   `json:"reason"`
	Fields []string `json:"fields"`
}

const fieldCoverageSchema = "zscalerctl/field-coverage/v1"

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
				Fields: mapKeysSorted(fields),
			})
		}
		sort.Slice(ignoredByReason, func(i, j int) bool {
			return ignoredByReason[i].Reason < ignoredByReason[j].Reason
		})

		resourcesOut = append(resourcesOut, fieldCoverageResource{
			Product:          string(acc.product),
			Resource:         acc.resource,
			SDKTypes:         sdkTypes,
			TotalFields:      acc.total,
			ClassifiedFields: acc.classified,
			IgnoredFields:    acc.ignored,
			CoveragePercent:  coveragePercent(acc.classified, acc.total),
			ClassBreakdown:   classBreakdown,
			IgnoredByReason:  ignoredByReason,
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
	}
	out := make([]fieldCoverageProduct, 0, len(byProduct))
	for product, a := range byProduct {
		out = append(out, fieldCoverageProduct{
			Product:          product,
			Resources:        a.resources,
			TotalFields:      a.total,
			ClassifiedFields: a.classified,
			IgnoredFields:    a.ignored,
			CoveragePercent:  coveragePercent(a.classified, a.total),
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
	var total, classified, ignored int
	for _, r := range rs {
		total += r.TotalFields
		classified += r.ClassifiedFields
		ignored += r.IgnoredFields
	}
	return fieldCoverageTotals{
		Resources:        len(rs),
		TotalFields:      total,
		ClassifiedFields: classified,
		IgnoredFields:    ignored,
		CoveragePercent:  coveragePercent(classified, total),
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
	b.WriteString("- **Ignored, with a reason** — the field is deliberately not classified yet\n")
	b.WriteString("  and carries a non-empty reason string. Ignored fields are **fail-closed\n")
	b.WriteString("  dropped**: they are never emitted in any mode.\n\n")
	b.WriteString("Coverage percent is classified / total exported SDK fields. A low number is\n")
	b.WriteString("not a leak — every unclassified field is dropped — it is a measure of how\n")
	b.WriteString("much of the available SDK surface has been deliberately reviewed and exposed.\n")
	b.WriteString("This report exists so that \"field coverage\" is a verifiable number, not an\n")
	b.WriteString("assurance. See [DATA_CLASSIFICATION.md](DATA_CLASSIFICATION.md) for the class\n")
	b.WriteString("definitions and the fail-closed output rules. The machine-readable companion\n")
	b.WriteString("[field-coverage.json](field-coverage.json) lists every ignored field name and\n")
	b.WriteString("its reason, which feeds field-expansion planning.\n\n")

	b.WriteString("## Repo-Wide Totals\n\n")
	fmt.Fprintf(&b, "- Resources: %d\n", report.Totals.Resources)
	fmt.Fprintf(&b, "- Total exported SDK fields: %d\n", report.Totals.TotalFields)
	fmt.Fprintf(&b, "- Classified: %d\n", report.Totals.ClassifiedFields)
	fmt.Fprintf(&b, "- Ignored (fail-closed dropped): %d\n", report.Totals.IgnoredFields)
	fmt.Fprintf(&b, "- Coverage: %s%%\n\n", formatPercent(report.Totals.CoveragePercent))

	b.WriteString("## Per-Product Totals\n\n")
	b.WriteString("Ranked worst coverage first.\n\n")
	b.WriteString("| Product | Resources | Total | Classified | Ignored | Coverage |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, p := range report.Products {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %s%% |\n",
			p.Product, p.Resources, p.TotalFields, p.ClassifiedFields, p.IgnoredFields, formatPercent(p.CoveragePercent))
	}
	b.WriteString("\n")

	b.WriteString("## Per-Resource Coverage\n\n")
	b.WriteString("Ranked worst coverage first within the whole catalog. `Ignored` fields are\n")
	b.WriteString("dropped before any output; see [field-coverage.json](field-coverage.json) for\n")
	b.WriteString("the field names and reasons behind each row.\n\n")
	b.WriteString("| Product | Resource | Total | Classified | Ignored | Coverage |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | ---: |\n")
	for _, r := range report.Resources {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %s%% |\n",
			r.Product, r.Resource, r.TotalFields, r.ClassifiedFields, r.IgnoredFields, formatPercent(r.CoveragePercent))
	}
	b.WriteString("\n")

	return []byte(b.String())
}

// formatPercent renders a one-decimal percentage so 100 prints as "100.0" and
// ordering of identical values stays stable.
func formatPercent(pct float64) string {
	return fmt.Sprintf("%.1f", pct)
}
