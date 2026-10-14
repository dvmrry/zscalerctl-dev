package resources

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestCoverageGapsStayDistinctFromCatalog keeps a gap from shadowing or
// contradicting a catalog resource: no surface or alias is a catalog name,
// names are unique per product and already normalized, every Instead target
// exists, and every entry has a note and a known reason.
func TestCoverageGapsStayDistinctFromCatalog(t *testing.T) {
	t.Parallel()

	catalog := Catalog()
	known := []GapReason{GapEntitlement, GapResponseShape, GapNotModeled, GapPrivacyReview,
		GapCredentialMaterial, GapAction, GapTelemetry, GapAlternateView}
	seen := map[string]string{}
	for _, gap := range coverageGaps {
		key := string(gap.Product) + "/" + gap.Surface
		if !slices.Contains(known, gap.Reason) {
			t.Errorf("%s: unknown reason %q", key, gap.Reason)
		}
		if strings.TrimSpace(gap.Note) == "" {
			t.Errorf("%s: empty note", key)
		}
		for _, name := range append([]string{gap.Surface}, gap.Aliases...) {
			if name != normalizeGapName(name) {
				t.Errorf("%s: name %q is not normalized", key, name)
			}
			if _, ok := catalog.FindSpec(gap.Product, name); ok {
				t.Errorf("%s: name %q is a catalog resource", key, name)
			}
			nameKey := string(gap.Product) + "/" + name
			if prior, dup := seen[nameKey]; dup {
				t.Errorf("%s: name %q already belongs to %s", key, name, prior)
			}
			seen[nameKey] = key
		}
		for _, ref := range gap.Instead {
			if _, ok := catalog.FindSpec(ref.Product, ref.Resource); !ok {
				t.Errorf("%s: instead target %s is not in the catalog", key, ref)
			}
		}
	}
}

// TestDeferredBacklogIsDeclared keeps docs/RESOURCE_QUEUE.md's deferred
// backlog and the gap table in step: every backlog row is a gap with a
// deferred status, so a caller asking for it learns why it is missing.
func TestDeferredBacklogIsDeclared(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("../../docs/RESOURCE_QUEUE.md")
	if err != nil {
		t.Fatal(err)
	}
	_, backlog, ok := strings.Cut(string(body), "## Deferred Retest / Investigation Backlog")
	if !ok {
		t.Fatal("RESOURCE_QUEUE.md has no deferred backlog section")
	}
	backlog, _, _ = strings.Cut(backlog, "\n## ")
	rows := regexp.MustCompile("(?m)^\\| `([a-z]+)/([a-z0-9-]+)` \\|").FindAllStringSubmatch(backlog, -1)
	if len(rows) == 0 {
		t.Fatal("no backlog rows found")
	}
	for _, row := range rows {
		gap, ok := FindGap(Product(row[1]), row[2])
		if !ok {
			t.Errorf("backlog row %s/%s is not a declared coverage gap", row[1], row[2])
			continue
		}
		if gap.Status != GapStatusDeferred {
			t.Errorf("backlog row %s/%s has status %q, want %q", row[1], row[2], gap.Status, GapStatusDeferred)
		}
	}
}

func TestFindGapMatchesAliasesLoosely(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"access-policies", "Access_Policies", " policy-rules "} {
		gap, ok := FindGap(ProductZPA, name)
		if !ok || gap.Surface != "policy-rules" || gap.Status != GapStatusDeferred || gap.Reason != GapNotModeled {
			t.Errorf("FindGap(zpa, %q) = %+v, %v; want deferred zpa/policy-rules", name, gap, ok)
		}
	}
	if gap, ok := FindGap(ProductZIA, "access-policies"); ok {
		t.Errorf("FindGap(zia, access-policies) = %+v; gaps are per product", gap)
	}
	if gap, ok := FindGap(ProductZIA, "vpn-credentials"); !ok || gap.Status != GapStatusOutOfScope {
		t.Errorf("FindGap(zia, vpn-credentials) = %+v, %v; want out_of_scope", gap, ok)
	}
}

func TestCoverageGapsFilterAndOrder(t *testing.T) {
	t.Parallel()

	all := CoverageGaps("")
	if len(all) != len(coverageGaps) {
		t.Fatalf("CoverageGaps(all) has %d entries, want %d", len(all), len(coverageGaps))
	}
	if !slices.IsSortedFunc(all, func(a, b CoverageGap) int {
		return strings.Compare(string(a.Product)+"/"+a.Surface, string(b.Product)+"/"+b.Surface)
	}) {
		t.Error("CoverageGaps(all) is not ordered by product then surface")
	}
	for _, gap := range CoverageGaps(ProductZCC) {
		if gap.Product != ProductZCC || gap.Status == "" {
			t.Errorf("CoverageGaps(zcc) returned %+v", gap)
		}
	}
}
