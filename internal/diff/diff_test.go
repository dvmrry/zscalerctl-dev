package diff

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestCompareKeyedResourceNormalizesIdentityAndReportsFieldChanges(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{
			{
				spec:    testKeyedSpec(),
				payload: `[{"id":123,"name":"old","lastModifiedTime":"2026-01-01T00:00:00Z"}]`,
			},
		},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{
			{
				spec:    testKeyedSpec(),
				payload: `[{"id":"123","name":"new","lastModifiedTime":"2026-01-01T00:00:00Z"}]`,
			},
		},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog, IgnoreOperational: true})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	resource := onlyResourceDiff(t, report)
	if resource.Identity.Mode != "get_key" || resource.Identity.Field != "id" {
		t.Fatalf("identity = %+v, want get_key/id", resource.Identity)
	}
	if len(resource.Added) != 0 || len(resource.Removed) != 0 || len(resource.Changed) != 1 {
		t.Fatalf("diff counts added=%d removed=%d changed=%d, want 0/0/1", len(resource.Added), len(resource.Removed), len(resource.Changed))
	}
	if resource.Changed[0].Key != "123" {
		t.Fatalf("changed key = %q, want 123", resource.Changed[0].Key)
	}
	assertChangedFields(t, resource.Changed[0], []string{"name"})
}

func TestCompareKeyedResourceReportsRecreatedRecordAsRemoveAndAdd(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{
			{
				spec:    testKeyedSpec(),
				payload: `[{"id":"1","name":"policy"}]`,
			},
		},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{
			{
				spec:    testKeyedSpec(),
				payload: `[{"id":"2","name":"policy"}]`,
			},
		},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	resource := onlyResourceDiff(t, report)
	if len(resource.Added) != 1 || len(resource.Removed) != 1 || len(resource.Changed) != 0 {
		t.Fatalf("recreated record diff counts added=%d removed=%d changed=%d, want 1/1/0", len(resource.Added), len(resource.Removed), len(resource.Changed))
	}
	if resource.Removed[0].Key != "1" || resource.Added[0].Key != "2" {
		t.Fatalf("recreated record keys removed=%+v added=%+v, want removed key 1 and added key 2", resource.Removed, resource.Added)
	}
}

func TestCompareSingletonResourceReportsChanges(t *testing.T) {
	catalog := resources.ResourceCatalog{testSingletonSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testSingletonSpec(), payload: `{"enabled":false}`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testSingletonSpec(), payload: `{"enabled":true}`}},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	resource := onlyResourceDiff(t, report)
	if resource.Identity.Mode != "singleton" {
		t.Fatalf("identity mode = %q, want singleton", resource.Identity.Mode)
	}
	if len(resource.Changed) != 1 || resource.Changed[0].Key != "singleton" {
		t.Fatalf("changed = %+v, want singleton change", resource.Changed)
	}
	assertChangedFields(t, resource.Changed[0], []string{"enabled"})
}

func TestCompareContentHashResourceIgnoresOperationalOnlyChanges(t *testing.T) {
	catalog := resources.ResourceCatalog{testIdentitylessSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{
			{
				spec:    testIdentitylessSpec(),
				payload: `[{"name":"policy","lastModifiedTime":"2026-01-01T00:00:00Z"}]`,
			},
		},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{
			{
				spec:    testIdentitylessSpec(),
				payload: `[{"name":"policy","lastModifiedTime":"2026-01-02T00:00:00Z"}]`,
			},
		},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	resource := onlyResourceDiff(t, report)
	if resource.Identity.Mode != "content_hash" {
		t.Fatalf("identity mode = %q, want content_hash", resource.Identity.Mode)
	}
	if resource.HasDrift() {
		t.Fatalf("content hash reported drift for operational-only change: %+v", resource)
	}
}

func TestCompareContentHashResourceReportsEditAsRemoveAndAdd(t *testing.T) {
	catalog := resources.ResourceCatalog{testIdentitylessSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testIdentitylessSpec(), payload: `[{"name":"old"}]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testIdentitylessSpec(), payload: `[{"name":"new"}]`}},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	resource := onlyResourceDiff(t, report)
	if len(resource.Added) != 1 || len(resource.Removed) != 1 || len(resource.Changed) != 0 {
		t.Fatalf("content-hash edit counts added=%d removed=%d changed=%d, want 1/1/0", len(resource.Added), len(resource.Removed), len(resource.Changed))
	}
	if resource.Added[0].Hash == "" || resource.Removed[0].Hash == "" {
		t.Fatalf("content-hash refs must include hashes: added=%+v removed=%+v", resource.Added, resource.Removed)
	}
}

func TestCompareContentHashResourceCanonicalizesObjectKeyOrder(t *testing.T) {
	catalog := resources.ResourceCatalog{testIdentitylessSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testIdentitylessSpec(), payload: `[{"name":"same","nested":{"a":1,"b":2}}]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testIdentitylessSpec(), payload: `[{"nested":{"b":2,"a":1},"name":"same"}]`}},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	resource := onlyResourceDiff(t, report)
	if resource.HasDrift() {
		t.Fatalf("content hash reported drift for key-order-only change: %+v", resource)
	}
}

func TestComparePreservesExactLargeNumberLexemes(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":"1","name":9007199254740993}]`}},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	resource := onlyResourceDiff(t, report)
	if len(resource.Added) != 1 {
		t.Fatalf("added = %#v, want one record", resource.Added)
	}
	number, ok := resource.Added[0].Record["name"].(json.Number)
	if !ok || number.String() != "9007199254740993" {
		t.Fatalf("added number = %#v, want exact json.Number", resource.Added[0].Record["name"])
	}
}

func TestCompareTreatsEquivalentNumberLexemesAsEqual(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":"1","name":1}]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":"1","name":1.0}]`}},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if resource := onlyResourceDiff(t, report); resource.HasDrift() {
		t.Fatalf("equivalent numeric lexemes reported drift: %#v", resource)
	}
}

func TestCompareTreatsEquivalentNumericIdentityLexemesAsSameRecord(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":1.0,"name":"same"}]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":1e0,"name":"same"}]`}},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	resource := onlyResourceDiff(t, report)
	if len(resource.Added) != 0 || len(resource.Removed) != 0 || len(resource.Changed) != 0 {
		t.Fatalf("equivalent numeric identities reported drift: %#v", resource)
	}
}

func TestCompareTreatsHugeNegativeExponentIdentitiesAsSameRecord(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":1e-1000000000,"name":"same"}]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":10e-1000000001,"name":"same"}]`}},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if resource := onlyResourceDiff(t, report); resource.HasDrift() {
		t.Fatalf("equivalent huge-exponent identities reported drift: %#v", resource)
	}
}

func TestCanonicalIdentityNumberBoundsExponentExpansion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lexeme  string
		want    string
		maxSize int
	}{
		{name: "huge negative exponent", lexeme: "1e-1000000000", want: "1e-1000000000", maxSize: 32},
		{name: "equivalent huge exponent", lexeme: "10e-1000000001", want: "1e-1000000000", maxSize: 32},
		{name: "int64 boundary exponent", lexeme: "1e-09223372036854775808", want: "1e-9223372036854775808", maxSize: 32},
		{name: "zero never expands", lexeme: "0.0e+1000000000", want: "0", maxSize: 1},
		{name: "ordinary identity stays plain", lexeme: "1.2300e2", want: "123", maxSize: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := canonicalIdentityNumber(tt.lexeme)
			if err != nil {
				t.Fatalf("canonicalIdentityNumber(%q) error = %v", tt.lexeme, err)
			}
			if got != tt.want {
				t.Fatalf("canonicalIdentityNumber(%q) = %q, want %q", tt.lexeme, got, tt.want)
			}
			if len(got) > tt.maxSize {
				t.Fatalf("canonicalIdentityNumber(%q) length = %d, want <= %d", tt.lexeme, len(got), tt.maxSize)
			}
		})
	}
}

func TestCompareReportsDistinctLargeNumbersExactly(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":"1","name":9007199254740992}]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":"1","name":9007199254740993}]`}},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	resource := onlyResourceDiff(t, report)
	if len(resource.Changed) != 1 || len(resource.Changed[0].Changes) != 1 {
		t.Fatalf("changes = %#v, want one exact numeric change", resource.Changed)
	}
	change := resource.Changed[0].Changes[0]
	oldNumber, oldOK := change.Old.(json.Number)
	newNumber, newOK := change.New.(json.Number)
	if !oldOK || !newOK || oldNumber.String() != "9007199254740992" || newNumber.String() != "9007199254740993" {
		t.Fatalf("numeric change = old:%#v new:%#v, want exact json.Number values", change.Old, change.New)
	}
}

func TestCompareContentHashTreatsEquivalentNumberLexemesAsEqual(t *testing.T) {
	catalog := resources.ResourceCatalog{testIdentitylessSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testIdentitylessSpec(), payload: `[{"name":1}]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testIdentitylessSpec(), payload: `[{"name":1e0}]`}},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if resource := onlyResourceDiff(t, report); resource.HasDrift() {
		t.Fatalf("content hash reported drift for equivalent numeric lexemes: %#v", resource)
	}
}

func TestCompareRejectsRedactionMismatch(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{redaction: redact.ModeStandard})
	newDir := writeTestDump(t, catalog, dumpFixture{redaction: redact.ModeShare})

	_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if !errors.Is(err, ErrRedactionMismatch) {
		t.Fatalf("Compare() error = %v, want ErrRedactionMismatch", err)
	}
}

func TestCompareRejectsPartialDumpUnlessAllowed(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{status: "partial"})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[]`}},
	})

	_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if !errors.Is(err, ErrPartialDumpInput) {
		t.Fatalf("Compare() error = %v, want ErrPartialDumpInput", err)
	}
	if _, err := Compare(oldDir, newDir, Options{Catalog: catalog, AllowPartial: true}); err != nil {
		t.Fatalf("Compare(... AllowPartial) error = %v", err)
	}
}

func TestCompareRejectsMismatchedCollectionScope(t *testing.T) {
	keyed := testKeyedSpec()
	other := testProgressSpec("other-rules")
	catalog := resources.ResourceCatalog{keyed, other}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: other, payload: `[]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: keyed, payload: `[{"id":"1","name":"HQ"}]`}},
	})

	_, err := Compare(oldDir, newDir, Options{
		Catalog: catalog,
		Resources: map[ResourceKey]bool{
			{Product: keyed.Product, Name: keyed.Name}: true,
		},
	})
	if !errors.Is(err, ErrCollectionScopeMismatch) {
		t.Fatalf("Compare(mismatched collection scope) error = %v, want ErrCollectionScopeMismatch", err)
	}
}

func TestCompareRejectsManifestShapeCatalogMismatch(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[]`}},
	})
	rewriteManifest(t, oldDir, func(manifest *dump.Manifest) {
		manifest.Resources[0].Shape = "singleton"
	})

	_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("Compare(shape mismatch) error = %v, want ErrInvalidDump", err)
	}
	if !strings.Contains(err.Error(), "manifest shape does not match the catalog") {
		t.Errorf("Compare(shape mismatch) error = %q, want catalog-shape context", err)
	}
}

func TestCompareAllowPartialDoesNotTreatFailedCollectionAsEmpty(t *testing.T) {
	keyed := testKeyedSpec()
	catalog := resources.ResourceCatalog{keyed}
	oldDir := writeTestDump(t, catalog, dumpFixture{status: "partial"})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: keyed, payload: `[{"id":"1","name":"HQ"}]`}},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog, AllowPartial: true})
	if err != nil {
		t.Fatalf("Compare(failed versus successful collection) error = %v, want nil", err)
	}
	resource := onlyResourceDiff(t, report)
	if resource.HasDrift() || len(resource.Added) != 0 || len(resource.Removed) != 0 || len(resource.Changed) != 0 {
		t.Errorf("Compare(failed versus successful collection) resource = %#v, want no fabricated drift", resource)
	}
	if !strings.Contains(resource.Note, "collection failed in old dump") {
		t.Errorf("Compare(failed versus successful collection) note = %q, want old failure context", resource.Note)
	}
	if resource.WasCompared() {
		t.Error("Compare(failed versus successful collection) WasCompared() = true, want false")
	}
}

func TestCompareRejectsMissingRedactionReport(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{})
	newDir := writeTestDump(t, catalog, dumpFixture{})
	if err := os.Remove(filepath.Join(oldDir, "redaction_report.json")); err != nil {
		t.Fatalf("os.Remove(redaction_report.json) error = %v, want nil", err)
	}

	_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("Compare(missing redaction report) error = %v, want ErrInvalidDump", err)
	}
}

func TestCompareRejectsInconsistentErrorMetadata(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{status: "partial"})
	newDir := writeTestDump(t, catalog, dumpFixture{})
	rewriteManifest(t, oldDir, func(manifest *dump.Manifest) {
		manifest.Errors = 2
	})

	_, err := Compare(oldDir, newDir, Options{Catalog: catalog, AllowPartial: true})
	if !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("Compare(inconsistent error metadata) error = %v, want ErrInvalidDump", err)
	}
}

func TestCompareRejectsInvalidManifestStatus(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{status: "degraded"})
	newDir := writeTestDump(t, catalog, dumpFixture{})

	_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("Compare() error = %v, want ErrInvalidDump", err)
	}
	if !strings.Contains(err.Error(), "invalid manifest status") {
		t.Fatalf("Compare() error = %v, want invalid manifest status context", err)
	}
	if !strings.Contains(err.Error(), "want complete or partial") {
		t.Fatalf("Compare() error = %v, want accepted-status hint", err)
	}
}

func TestCompareRejectsUnsupportedManifestSchemaWithExpectedSchema(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{})
	newDir := writeTestDump(t, catalog, dumpFixture{})
	rewriteManifest(t, oldDir, func(manifest *dump.Manifest) {
		manifest.Schema = "zscalerctl.dump.manifest.v0"
	})

	_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("Compare() error = %v, want ErrInvalidDump", err)
	}
	for _, want := range []string{
		"unsupported manifest schema",
		dump.ManifestSchemaID,
		"docs/schema/manifest.schema.json",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Compare() error = %v, want %q", err, want)
		}
	}
}

func TestCompareRejectsOversizedManifest(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{})
	newDir := writeTestDump(t, catalog, dumpFixture{})
	truncateTestFile(t, filepath.Join(oldDir, "manifest.json"), (1<<20)+1)

	_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("Compare() error = %v, want ErrInvalidDump", err)
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("Compare() error = %v, want too large context", err)
	}
}

func TestCompareRejectsOversizedResource(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":"1","name":"old"}]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":"1","name":"old"}]`}},
	})
	truncateTestFile(t, filepath.Join(oldDir, "resources", "zia", "rules.json"), maxResourceBytes+1)

	_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("Compare() error = %v, want ErrInvalidDump", err)
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("Compare() error = %v, want too large context", err)
	}
}

func TestCompareIgnoreOperationalSuppressesKeyedOperationalChanges(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":"1","name":"same","lastModifiedTime":"old"}]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":"1","name":"same","lastModifiedTime":"new"}]`}},
	})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if !onlyResourceDiff(t, report).HasDrift() {
		t.Fatalf("Compare() without IgnoreOperational did not report operational drift")
	}
	report, err = Compare(oldDir, newDir, Options{Catalog: catalog, IgnoreOperational: true})
	if err != nil {
		t.Fatalf("Compare(... IgnoreOperational) error = %v", err)
	}
	if onlyResourceDiff(t, report).HasDrift() {
		t.Fatalf("Compare(... IgnoreOperational) reported operational drift: %+v", report.Resources[0])
	}
}

func TestCompareWrapperMatchesContextReportBytes(t *testing.T) {
	catalog := resources.ResourceCatalog{testKeyedSpec()}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":"1","name":"same"}]`}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: testKeyedSpec(), payload: `[{"id":"1","name":"changed"}]`}},
	})

	compat, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	contextReport, err := CompareContext(context.Background(), oldDir, newDir, Options{Catalog: catalog}, nil)
	if err != nil {
		t.Fatalf("CompareContext() error = %v", err)
	}
	compatBody, err := json.Marshal(compat)
	if err != nil {
		t.Fatalf("json.Marshal(Compare()) error = %v", err)
	}
	contextBody, err := json.Marshal(contextReport)
	if err != nil {
		t.Fatalf("json.Marshal(CompareContext()) error = %v", err)
	}
	if string(compatBody) != string(contextBody) {
		t.Fatalf("Compare and CompareContext report bytes differ:\ncompat=%s\ncontext=%s", compatBody, contextBody)
	}
}

func TestCompareRejectsUnknownTopLevelAndNestedFieldsWithoutLeakage(t *testing.T) {
	const canary = "unknown-field-canary"
	tests := []struct {
		name    string
		spec    resources.ResourceSpec
		payload string
	}{
		{
			name:    "top-level",
			spec:    testKeyedSpec(),
			payload: `[{"id":"1","name":"safe","` + canary + `":"must-not-render"}]`,
		},
		{
			name:    "nested",
			spec:    testNestedSpec(),
			payload: `[{"id":"1","nested":{"allowed":"safe","` + canary + `":"must-not-render"}}]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := resources.ResourceCatalog{tt.spec}
			oldDir := writeTestDump(t, catalog, dumpFixture{
				entries: []dumpEntryFixture{{spec: tt.spec, payload: tt.payload}},
			})
			newDir := writeTestDump(t, catalog, dumpFixture{})

			report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
			if !errors.Is(err, ErrInvalidDump) {
				t.Fatalf("Compare() error = %v, want ErrInvalidDump", err)
			}
			if strings.Contains(err.Error(), canary) {
				t.Fatalf("Compare() error = %q contains input canary", err)
			}
			body, marshalErr := json.Marshal(report)
			if marshalErr != nil {
				t.Fatalf("json.Marshal(error report) error = %v", marshalErr)
			}
			if strings.Contains(string(body), canary) {
				t.Fatalf("error report = %s contains input canary", body)
			}
		})
	}
}

func TestCompareScopesRecordAdmissionToSelectedResources(t *testing.T) {
	t.Parallel()

	const canary = "unselected-client-secret-canary"
	selectedSpec := testKeyedSpec()
	unselectedSpec := testProgressSpec("unselected-resource")
	catalog := resources.ResourceCatalog{selectedSpec, unselectedSpec}
	oldDir := writeTestDump(t, catalog, dumpFixture{entries: []dumpEntryFixture{
		{spec: selectedSpec, payload: `[{"id":"1","name":"old"}]`},
		{spec: unselectedSpec, payload: `[{"label":"safe","clientSecret":"` + canary + `"}]`},
	}})
	newDir := writeTestDump(t, catalog, dumpFixture{entries: []dumpEntryFixture{
		{spec: selectedSpec, payload: `[{"id":"1","name":"new"}]`},
		{spec: unselectedSpec, payload: `[{"label":"safe","clientSecret":"` + canary + `"}]`},
	}})

	report, err := Compare(oldDir, newDir, Options{
		Catalog: catalog,
		Resources: map[ResourceKey]bool{
			{Product: selectedSpec.Product, Name: selectedSpec.Name}: true,
		},
	})
	if err != nil {
		t.Fatalf("Compare(selected resource) error = %v, want nil", err)
	}
	resource := onlyResourceDiff(t, report)
	if resource.Resource != selectedSpec.Name || len(resource.Changed) != 1 {
		t.Fatalf("Compare(selected resource) = %#v, want one selected-resource change", resource)
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal(selected report) error = %v", err)
	}
	if strings.Contains(string(body), canary) || strings.Contains(string(body), unselectedSpec.Name) {
		t.Fatalf("selected report = %s, want no unselected resource data", body)
	}
}

func TestCompareStillParsesUnselectedResourceBodies(t *testing.T) {
	t.Parallel()

	selectedSpec := testKeyedSpec()
	unselectedSpec := testProgressSpec("unselected-resource")
	catalog := resources.ResourceCatalog{selectedSpec, unselectedSpec}
	oldDir := writeTestDump(t, catalog, dumpFixture{entries: []dumpEntryFixture{
		{spec: selectedSpec, payload: `[{"id":"1","name":"same"}]`},
		{spec: unselectedSpec, payload: `[42]`},
	}})
	newDir := writeTestDump(t, catalog, dumpFixture{entries: []dumpEntryFixture{
		{spec: selectedSpec, payload: `[{"id":"1","name":"same"}]`},
	}})

	_, err := Compare(oldDir, newDir, Options{
		Catalog: catalog,
		Resources: map[ResourceKey]bool{
			{Product: selectedSpec.Product, Name: selectedSpec.Name}: true,
		},
	})
	if !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("Compare(structurally invalid unselected resource) error = %v, want ErrInvalidDump", err)
	}
}

func TestScanResourceJSONMatchesFullDecodeStructure(t *testing.T) {
	invalidUTF8 := append([]byte(`[{"label":"`), 0xff)
	invalidUTF8 = append(invalidUTF8, []byte(`"}]`)...)
	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "object", payload: []byte(`{"label":"one","nested":{"items":[1,true,null]}}`)},
		{name: "empty array", payload: []byte(`[]`)},
		{name: "object array", payload: []byte(`[{"label":"one"},{"label":"two"}]`)},
		{name: "top-level null", payload: []byte(`null`)},
		{name: "top-level string", payload: []byte(`"value"`)},
		{name: "top-level number", payload: []byte(`42`)},
		{name: "top-level boolean", payload: []byte(`true`)},
		{name: "array null", payload: []byte(`[null]`)},
		{name: "array scalar", payload: []byte(`[42]`)},
		{name: "array nested array", payload: []byte(`[[{"label":"nested"}]]`)},
		{name: "malformed", payload: []byte(`[{"label":]`)},
		{name: "malformed after non-object", payload: []byte(`[42,{"label":]`)},
		{name: "trailing value", payload: []byte(`[{"label":"one"}] {}`)},
		{name: "duplicate keys", payload: []byte(`[{"label":"one","label":"two"}]`)},
		{name: "invalid utf8", payload: invalidUTF8},
		{name: "invalid surrogate", payload: []byte(`[{"label":"\ud800"}]`)},
		{name: "float overflow", payload: []byte(`[{"value":1e400}]`)},
		{name: "empty input", payload: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertResourceJSONMatchesFullDecode(t, tt.payload)
		})
	}
}

func TestScanResourceJSONCancellationWins(t *testing.T) {
	t.Run("during streamed array", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		reader := &cancelingChunkReader{
			reader:      bytes.NewReader(benchmarkDiffPayload(100)),
			cancel:      cancel,
			cancelAfter: 4,
		}
		_, err := scanResourceJSON(ctx, reader)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("scanResourceJSON(canceled streamed array) error = %v, want context.Canceled", err)
		}
	})

	t.Run("after decoder error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		reader := &cancelAtEOFReader{
			reader: bytes.NewReader([]byte(`[{"label":`)),
			cancel: cancel,
		}
		_, err := scanResourceJSON(ctx, reader)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("scanResourceJSON(canceled decoder error) error = %v, want context.Canceled", err)
		}
	})
}

func TestScanResourceJSONDoesNotClassifyReaderErrorAsContent(t *testing.T) {
	payload := []byte(`[{"label":"safe"}]`)
	for _, tt := range []struct {
		name      string
		remaining int
	}{
		{name: "during record decode", remaining: 8},
		{name: "before closing bracket", remaining: len(payload) - 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want := errors.New("one-shot read failure")
			reader := &oneShotErrorReader{
				reader:    bytes.NewReader(payload),
				err:       want,
				remaining: tt.remaining,
			}
			_, err := scanResourceJSON(context.Background(), reader)
			if !errors.Is(err, want) {
				t.Fatalf("scanResourceJSON(one-shot reader error) error = %v, want %v", err, want)
			}
			if isJSONContentError(err) {
				t.Fatalf("isJSONContentError(one-shot reader error) = true, want false")
			}
		})
	}
}

func TestScanResourceJSONReadErrorWinsOverSyntax(t *testing.T) {
	want := errors.New("read failure alongside malformed data")
	reader := &dataAndErrorReader{
		data: []byte(`[{"label":]`),
		err:  want,
	}
	_, err := scanResourceJSON(context.Background(), reader)
	if !errors.Is(err, want) {
		t.Fatalf("scanResourceJSON(data and reader error) error = %v, want %v", err, want)
	}
	if isJSONContentError(err) {
		t.Fatalf("isJSONContentError(data and reader error) = true, want false")
	}
}

func TestCompareRejectsSelectedAndUnselectedRecordCountMismatch(t *testing.T) {
	selectedSpec := testKeyedSpec()
	unselectedSpec := testProgressSpec("unselected-resource")
	catalog := resources.ResourceCatalog{selectedSpec, unselectedSpec}
	fixture := dumpFixture{entries: []dumpEntryFixture{
		{spec: selectedSpec, payload: `[{"id":"1","name":"same"}]`},
		{spec: unselectedSpec, payload: `[{"label":"same"}]`},
	}}
	tests := []struct {
		name     string
		resource string
	}{
		{name: "selected", resource: selectedSpec.Name},
		{name: "unselected", resource: unselectedSpec.Name},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldDir := writeTestDump(t, catalog, fixture)
			newDir := writeTestDump(t, catalog, fixture)
			rewriteManifest(t, oldDir, func(manifest *dump.Manifest) {
				for i := range manifest.Resources {
					if manifest.Resources[i].Name == tt.resource {
						manifest.Resources[i].Records++
					}
				}
			})

			_, err := Compare(oldDir, newDir, Options{
				Catalog: catalog,
				Resources: map[ResourceKey]bool{
					{Product: selectedSpec.Product, Name: selectedSpec.Name}: true,
				},
			})
			if !errors.Is(err, ErrInvalidDump) || !strings.Contains(err.Error(), "record count") {
				t.Fatalf("Compare(%s record-count mismatch) error = %v, want ErrInvalidDump with record-count context", tt.name, err)
			}
		})
	}
}

func TestCompareCountsUnselectedEmptyAndSingletonBodies(t *testing.T) {
	selectedSpec := testKeyedSpec()
	unselectedSpec := testProgressSpec("unselected-resource")
	catalog := resources.ResourceCatalog{selectedSpec, unselectedSpec}
	for _, tt := range []struct {
		name    string
		payload string
	}{
		{name: "empty array", payload: `[]`},
		{name: "singleton object", payload: `{"unknown":"not-admitted"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := dumpFixture{entries: []dumpEntryFixture{
				{spec: selectedSpec, payload: `[{"id":"1","name":"same"}]`},
				{spec: unselectedSpec, payload: tt.payload},
			}}
			oldDir := writeTestDump(t, catalog, fixture)
			newDir := writeTestDump(t, catalog, fixture)
			_, err := Compare(oldDir, newDir, Options{
				Catalog: catalog,
				Resources: map[ResourceKey]bool{
					{Product: selectedSpec.Product, Name: selectedSpec.Name}: true,
				},
			})
			if err != nil {
				t.Fatalf("Compare(unselected %s) error = %v, want nil", tt.name, err)
			}
		})
	}
}

func TestCompareRejectsUnsafeUnselectedResourceFiles(t *testing.T) {
	selectedSpec := testKeyedSpec()
	unselectedSpec := testProgressSpec("unselected-resource")
	catalog := resources.ResourceCatalog{selectedSpec, unselectedSpec}
	fixture := dumpFixture{entries: []dumpEntryFixture{
		{spec: selectedSpec, payload: `[{"id":"1","name":"same"}]`},
		{spec: unselectedSpec, payload: `[{"label":"same"}]`},
	}}
	selectedOnly := Options{
		Catalog: catalog,
		Resources: map[ResourceKey]bool{
			{Product: selectedSpec.Product, Name: selectedSpec.Name}: true,
		},
	}

	t.Run("traversal path", func(t *testing.T) {
		oldDir := writeTestDump(t, catalog, fixture)
		newDir := writeTestDump(t, catalog, fixture)
		rewriteManifest(t, oldDir, func(manifest *dump.Manifest) {
			for i := range manifest.Resources {
				if manifest.Resources[i].Name == unselectedSpec.Name {
					manifest.Resources[i].Path = filepath.ToSlash(filepath.Join("..", "outside.json"))
				}
			}
		})
		_, err := Compare(oldDir, newDir, selectedOnly)
		if !errors.Is(err, ErrInvalidDump) || !strings.Contains(err.Error(), "unsafe path") {
			t.Fatalf("Compare(unselected traversal path) error = %v, want ErrInvalidDump with unsafe-path context", err)
		}
	})

	t.Run("absolute path", func(t *testing.T) {
		oldDir := writeTestDump(t, catalog, fixture)
		newDir := writeTestDump(t, catalog, fixture)
		rewriteManifest(t, oldDir, func(manifest *dump.Manifest) {
			for i := range manifest.Resources {
				if manifest.Resources[i].Name == unselectedSpec.Name {
					manifest.Resources[i].Path = filepath.ToSlash(filepath.Join(string(filepath.Separator), "outside.json"))
				}
			}
		})
		_, err := Compare(oldDir, newDir, selectedOnly)
		if !errors.Is(err, ErrInvalidDump) || !strings.Contains(err.Error(), "unsafe path") {
			t.Fatalf("Compare(unselected absolute path) error = %v, want ErrInvalidDump with unsafe-path context", err)
		}
	})

	t.Run("symlink escape", func(t *testing.T) {
		oldDir := writeTestDump(t, catalog, fixture)
		newDir := writeTestDump(t, catalog, fixture)
		outside := filepath.Join(t.TempDir(), "outside.json")
		if err := os.WriteFile(outside, []byte(`[{"label":"outside"}]`), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v, want nil", outside, err)
		}
		path := filepath.Join(oldDir, "resources", "zia", unselectedSpec.Name+".json")
		if err := os.Remove(path); err != nil {
			t.Fatalf("os.Remove(%q) error = %v, want nil", path, err)
		}
		if err := os.Symlink(outside, path); err != nil {
			t.Skipf("os.Symlink(%q, %q) unavailable: %v", outside, path, err)
		}
		_, err := Compare(oldDir, newDir, selectedOnly)
		if !errors.Is(err, ErrInvalidDump) {
			t.Fatalf("Compare(unselected symlink escape) error = %v, want ErrInvalidDump", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		oldDir := writeTestDump(t, catalog, fixture)
		newDir := writeTestDump(t, catalog, fixture)
		path := filepath.Join(oldDir, "resources", "zia", unselectedSpec.Name+".json")
		if err := os.Remove(path); err != nil {
			t.Fatalf("os.Remove(%q) error = %v, want nil", path, err)
		}
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatalf("os.Mkdir(%q) error = %v, want nil", path, err)
		}
		_, err := Compare(oldDir, newDir, selectedOnly)
		if !errors.Is(err, ErrInvalidDump) || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("Compare(unselected directory) error = %v, want ErrInvalidDump with regular-file context", err)
		}
	})

	t.Run("oversized", func(t *testing.T) {
		oldDir := writeTestDump(t, catalog, fixture)
		newDir := writeTestDump(t, catalog, fixture)
		path := filepath.Join(oldDir, "resources", "zia", unselectedSpec.Name+".json")
		truncateTestFile(t, path, maxResourceBytes+1)
		_, err := Compare(oldDir, newDir, selectedOnly)
		if !errors.Is(err, ErrInvalidDump) || !strings.Contains(err.Error(), "too large") {
			t.Fatalf("Compare(oversized unselected resource) error = %v, want ErrInvalidDump with size context", err)
		}
	})
}

func TestCompareRejectsSecretAndUnrenderableFields(t *testing.T) {
	tests := []struct {
		name string
		spec resources.ResourceSpec
		mode redact.Mode
	}{
		{name: "secret", spec: testSecretSpec(), mode: redact.ModeStandard},
		{name: "unrenderable", spec: testUnrenderableSpec(), mode: redact.ModeShare},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			catalog := resources.ResourceCatalog{tt.spec}
			payload := `[{"id":"1","token":"secret-field-canary"}]`
			if tt.name == "unrenderable" {
				payload = `[{"standardOnly":"mode-canary"}]`
			}
			oldDir := writeTestDump(t, catalog, dumpFixture{
				redaction: tt.mode,
				entries:   []dumpEntryFixture{{spec: tt.spec, payload: payload}},
			})
			newDir := writeTestDump(t, catalog, dumpFixture{redaction: tt.mode})

			report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
			if !errors.Is(err, ErrInvalidDump) {
				t.Fatalf("Compare() error = %v, want ErrInvalidDump", err)
			}
			for _, canary := range []string{"secret-field-canary", "mode-canary"} {
				if strings.Contains(err.Error(), canary) {
					t.Fatalf("Compare() error = %q contains input canary %q", err, canary)
				}
			}
			body, marshalErr := json.Marshal(report)
			if marshalErr != nil {
				t.Fatalf("json.Marshal(error report) error = %v", marshalErr)
			}
			if strings.Contains(string(body), "secret-field-canary") || strings.Contains(string(body), "mode-canary") {
				t.Fatalf("error report = %s contains input canary", body)
			}
		})
	}
}

func TestCompareRejectsNonIdempotentRedactionWithoutLeakage(t *testing.T) {
	const canary = "self-describing-secret-canary"
	spec := testSelfDescribingSecretSpec()
	catalog := resources.ResourceCatalog{spec}
	payload := `[{"name":"Authorization: Bearer ` + canary + `"}]`
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{{spec: spec, payload: payload}},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{})

	report, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("Compare() error = %v, want ErrInvalidDump", err)
	}
	if !strings.Contains(err.Error(), "non-idempotent") {
		t.Fatalf("Compare() error = %v, want non-idempotent admission context", err)
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatalf("Compare() error = %q contains input canary", err)
	}
	body, marshalErr := json.Marshal(report)
	if marshalErr != nil {
		t.Fatalf("json.Marshal(error report) error = %v", marshalErr)
	}
	if strings.Contains(string(body), canary) {
		t.Fatalf("error report = %s contains input canary", body)
	}
}

func TestCompareRejectsDuplicateManifestResource(t *testing.T) {
	spec := testKeyedSpec()
	catalog := resources.ResourceCatalog{spec}
	oldDir := writeTestDump(t, catalog, dumpFixture{
		entries: []dumpEntryFixture{
			{spec: spec, payload: `[{"id":"1","name":"same"}]`},
			{spec: spec, payload: `[{"id":"1","name":"same"}]`},
		},
	})
	newDir := writeTestDump(t, catalog, dumpFixture{})

	_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if !errors.Is(err, ErrInvalidDump) {
		t.Fatalf("Compare() error = %v, want ErrInvalidDump", err)
	}
	if !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("Compare() error = %v, want duplicate-resource context", err)
	}
}

func TestCompareContextPreCanceledContextWinsBeforeFilesystem(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	_, err := CompareContext(ctx, filepath.Join(t.TempDir(), "missing-old"), filepath.Join(t.TempDir(), "missing-new"), Options{
		Catalog: resources.ResourceCatalog{testKeyedSpec()},
	}, func(Progress) error {
		called = true
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CompareContext() error = %v, want context.Canceled", err)
	}
	if called {
		t.Fatal("CompareContext() invoked progress callback for pre-canceled context")
	}
}

func TestCompareContextProgressIsOrderedAndNilSafe(t *testing.T) {
	aSpec := testProgressSpec("a-resource")
	zSpec := testProgressSpec("z-resource")
	catalog := resources.ResourceCatalog{zSpec, aSpec}
	fixture := dumpFixture{entries: []dumpEntryFixture{
		{spec: zSpec, payload: `[{"label":"z"}]`},
		{spec: aSpec, payload: `[{"label":"a"}]`},
	}}
	oldDir := writeTestDump(t, catalog, fixture)
	newDir := writeTestDump(t, catalog, fixture)

	var got []Progress
	if _, err := CompareContext(context.Background(), oldDir, newDir, Options{Catalog: catalog}, func(progress Progress) error {
		got = append(got, progress)
		return nil
	}); err != nil {
		t.Fatalf("CompareContext(progress) error = %v", err)
	}
	want := []Progress{
		{Product: resources.ProductZIA, Resource: "a-resource", Done: 1, Total: 2},
		{Product: resources.ProductZIA, Resource: "z-resource", Done: 2, Total: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("progress = %#v, want %#v", got, want)
	}
	//lint:ignore SA1012 CompareContext deliberately accepts nil as a compatibility boundary.
	if _, err := CompareContext(nil, oldDir, newDir, Options{Catalog: catalog}, nil); err != nil {
		t.Fatalf("CompareContext(nil context, nil progress) error = %v", err)
	}
}

func TestCompareContextProgressCallbackErrorIsReturnedUnchanged(t *testing.T) {
	spec := testProgressSpec("only-resource")
	catalog := resources.ResourceCatalog{spec}
	fixture := dumpFixture{entries: []dumpEntryFixture{{spec: spec, payload: `[{"label":"same"}]`}}}
	oldDir := writeTestDump(t, catalog, fixture)
	newDir := writeTestDump(t, catalog, fixture)
	want := errors.New("stop progress")

	_, err := CompareContext(context.Background(), oldDir, newDir, Options{Catalog: catalog}, func(Progress) error {
		return want
	})
	if err != want {
		t.Fatalf("CompareContext() error = %v, want exact callback error %v", err, want)
	}
}

func TestCloneReportRecursivelyCopiesReportValues(t *testing.T) {
	source := Report{Resources: []ResourceDiff{{
		Added: []RecordRef{{Record: map[string]any{
			"nested": map[string]any{"items": []any{map[string]any{"value": "source"}}},
			"labels": []string{"one"},
		}}},
		Changed: []RecordChange{{Changes: []FieldChange{{
			Field: "nested",
			Old:   map[string]any{"items": []any{map[string]any{"value": "old"}}},
			New:   []any{map[string]any{"value": "new"}},
		}}}},
	}}}
	clone := CloneReport(source)

	source.Resources[0].Added[0].Record["nested"].(map[string]any)["items"].([]any)[0].(map[string]any)["value"] = "mutated-source"
	source.Resources[0].Changed[0].Changes[0].Old.(map[string]any)["items"].([]any)[0].(map[string]any)["value"] = "mutated-old"
	if got := clone.Resources[0].Added[0].Record["nested"].(map[string]any)["items"].([]any)[0].(map[string]any)["value"]; got != "source" {
		t.Fatalf("clone changed after source mutation = %#v, want source", got)
	}
	if got := clone.Resources[0].Changed[0].Changes[0].Old.(map[string]any)["items"].([]any)[0].(map[string]any)["value"]; got != "old" {
		t.Fatalf("clone field change changed after source mutation = %#v, want old", got)
	}

	clone.Resources[0].Added[0].Record["nested"].(map[string]any)["items"].([]any)[0].(map[string]any)["value"] = "mutated-clone"
	clone.Resources[0].Changed[0].Changes[0].New.([]any)[0].(map[string]any)["value"] = "mutated-new"
	if got := source.Resources[0].Added[0].Record["nested"].(map[string]any)["items"].([]any)[0].(map[string]any)["value"]; got != "mutated-source" {
		t.Fatalf("source changed after clone mutation = %#v, want mutated-source", got)
	}
	if got := source.Resources[0].Changed[0].Changes[0].New.([]any)[0].(map[string]any)["value"]; got != "new" {
		t.Fatalf("source field change changed after clone mutation = %#v, want new", got)
	}
}

func TestCloneReportPreservesNilSlicesForJSONCompatibility(t *testing.T) {
	t.Parallel()

	source := Report{Schema: SchemaID}
	clone := CloneReport(source)
	if clone.Resources != nil {
		t.Fatalf("CloneReport().Resources = %#v, want nil", clone.Resources)
	}
	body, err := json.Marshal(clone)
	if err != nil {
		t.Fatalf("json.Marshal(CloneReport()) error = %v", err)
	}
	if !strings.Contains(string(body), `"resources":null`) {
		t.Fatalf("json.Marshal(CloneReport()) = %s, want resources:null", body)
	}
}

func TestContextReaderStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader := contextReader{ctx: ctx, reader: strings.NewReader("abcdef")}
	buf := make([]byte, 3)
	if n, err := reader.Read(buf); n != 3 || err != nil {
		t.Fatalf("contextReader.Read() = %d, %v, want 3, nil", n, err)
	}
	cancel()
	if _, err := reader.Read(buf); !errors.Is(err, context.Canceled) {
		t.Fatalf("contextReader.Read() after cancellation error = %v, want context.Canceled", err)
	}
}

func TestCompareContextRejectsUnsafeCatalogWithoutLeakingValues(t *testing.T) {
	const canary = "catalog-canary"
	invalid := testKeyedSpec()
	invalid.Name = "rules/" + canary
	_, err := CompareContext(context.Background(), filepath.Join(t.TempDir(), "old"), filepath.Join(t.TempDir(), "new"), Options{
		Catalog: resources.ResourceCatalog{invalid},
	}, nil)
	if !errors.Is(err, ErrInvalidCatalog) {
		t.Fatalf("CompareContext() error = %v, want ErrInvalidCatalog", err)
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatalf("CompareContext() error = %q contains catalog canary", err)
	}
}

func FuzzScanResourceJSONMatchesFullDecode(f *testing.F) {
	for _, payload := range [][]byte{
		[]byte(`{}`),
		[]byte(`[]`),
		[]byte(`[{"label":"one"}]`),
		[]byte(`[null]`),
		[]byte(`[42,{"label":]`),
		[]byte(`[{"value":1e400}]`),
		[]byte(`[{"label":"one"}] {}`),
	} {
		f.Add(payload)
	}
	f.Fuzz(func(t *testing.T, payload []byte) {
		assertResourceJSONMatchesFullDecode(t, payload)
	})
}

func assertResourceJSONMatchesFullDecode(t *testing.T, payload []byte) {
	t.Helper()
	want, wantErr := fullDecodeResourceJSONShape(payload)
	got, gotErr := scanResourceJSON(context.Background(), bytes.NewReader(payload))
	if (gotErr != nil) != (wantErr != nil) {
		t.Fatalf(
			"scanResourceJSON(%q) error = %v, full decode error = %v",
			payload,
			gotErr,
			wantErr,
		)
	}
	if wantErr == nil && !reflect.DeepEqual(got, want) {
		t.Fatalf("scanResourceJSON(%q) = %#v, want full-decode shape %#v", payload, got, want)
	}
}

func fullDecodeResourceJSONShape(payload []byte) (resourceJSONShape, error) {
	var raw any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return resourceJSONShape{}, err
	}
	var shape resourceJSONShape
	switch value := raw.(type) {
	case []any:
		shape.objectOrArray = true
		for _, item := range value {
			if _, ok := item.(map[string]any); ok {
				shape.records++
			} else {
				shape.nonObjectRecord = true
			}
		}
	case map[string]any:
		shape.objectOrArray = true
		shape.records = 1
	}
	return shape, nil
}

type cancelingChunkReader struct {
	reader      io.Reader
	cancel      context.CancelFunc
	cancelAfter int
	reads       int
}

func (r *cancelingChunkReader) Read(p []byte) (int, error) {
	if len(p) > 32 {
		p = p[:32]
	}
	n, err := r.reader.Read(p)
	r.reads++
	if r.reads == r.cancelAfter {
		r.cancel()
	}
	return n, err
}

type cancelAtEOFReader struct {
	reader io.Reader
	cancel context.CancelFunc
}

type oneShotErrorReader struct {
	reader    io.Reader
	err       error
	remaining int
	failed    bool
}

type dataAndErrorReader struct {
	data []byte
	err  error
	done bool
}

func (r *dataAndErrorReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(p, r.data), r.err
}

func (r *oneShotErrorReader) Read(p []byte) (int, error) {
	if !r.failed && r.remaining == 0 {
		r.failed = true
		return 0, r.err
	}
	if !r.failed && len(p) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	if !r.failed {
		r.remaining -= n
	}
	return n, err
}

func (r *cancelAtEOFReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if errors.Is(err, io.EOF) {
		r.cancel()
	}
	return n, err
}

type dumpFixture struct {
	redaction redact.Mode
	status    string
	entries   []dumpEntryFixture
}

type dumpEntryFixture struct {
	spec    resources.ResourceSpec
	payload string
}

func writeTestDump(t *testing.T, catalog resources.ResourceCatalog, fixture dumpFixture) string {
	t.Helper()
	dir := t.TempDir()
	mode := fixture.redaction
	if mode == "" {
		mode = redact.ModeStandard
	}
	status := fixture.status
	if status == "" {
		status = "complete"
	}
	manifest := dump.Manifest{
		Schema:      dump.ManifestSchemaID,
		CollectedAt: "2026-01-01T00:00:00Z",
		ToolVersion: "test",
		Redaction:   string(mode),
		Warning:     "test fixture",
		Status:      status,
		Resources:   []dump.ManifestResource{},
	}
	report := dump.RedactionReport{
		Schema:    dump.RedactionReportSchemaID,
		Redaction: string(mode),
		Resources: []dump.ResourceReport{},
	}
	for _, entry := range fixture.entries {
		relPath := filepath.ToSlash(filepath.Join("resources", string(entry.spec.Product), entry.spec.Name+".json"))
		path := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(entry.payload), 0o600); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
		manifest.Resources = append(manifest.Resources, dump.ManifestResource{
			Product: string(entry.spec.Product),
			Name:    entry.spec.Name,
			Shape:   dump.ManifestResourceShape(entry.spec),
			Status:  "ok",
			Path:    relPath,
			Records: countRecords(t, entry.payload),
		})
		report.Resources = append(report.Resources, dump.ResourceReport{
			Product: string(entry.spec.Product),
			Name:    entry.spec.Name,
			Path:    relPath,
			Records: countRecords(t, entry.payload),
		})
	}
	if status == "partial" {
		manifest.Errors = 1
		manifest.ErrorsPath = "errors.ndjson"
		manifest.Resources = append(manifest.Resources, dump.ManifestResource{
			Product:   string(catalog[0].Product),
			Name:      catalog[0].Name,
			Status:    "error",
			Operation: "list",
			ErrorKind: "live_access_failed",
		})
		resourceError := dump.NewResourceError(
			catalog[0].Product,
			catalog[0].Name,
			"list",
			"live_access_failed",
		)
		errorBody, err := json.Marshal(resourceError)
		if err != nil {
			t.Fatalf("Marshal(errors.ndjson): %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(dir, "errors.ndjson"),
			append(errorBody, '\n'),
			0o600,
		); err != nil {
			t.Fatalf("WriteFile(errors.ndjson): %v", err)
		}
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent(manifest): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0o600); err != nil {
		t.Fatalf("WriteFile(manifest): %v", err)
	}
	reportBody, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent(redaction report): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "redaction_report.json"), reportBody, 0o600); err != nil {
		t.Fatalf("WriteFile(redaction report): %v", err)
	}
	return dir
}

func rewriteManifest(t *testing.T, dir string, mutate func(*dump.Manifest)) {
	t.Helper()
	path := filepath.Join(dir, "manifest.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	var manifest dump.Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatalf("Unmarshal(%s): %v", path, err)
	}
	mutate(&manifest)
	body, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent(manifest): %v", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func truncateTestFile(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.Truncate(path, size); err != nil {
		t.Fatalf("Truncate(%s, %d): %v", path, size, err)
	}
}

func countRecords(t *testing.T, payload string) int {
	t.Helper()
	var raw any
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		t.Fatalf("Unmarshal(%s): %v", payload, err)
	}
	switch value := raw.(type) {
	case []any:
		return len(value)
	case map[string]any:
		return 1
	default:
		t.Fatalf("payload %s is not an object or array", payload)
		return 0
	}
}

func onlyResourceDiff(t *testing.T, report Report) ResourceDiff {
	t.Helper()
	if len(report.Resources) != 1 {
		t.Fatalf("len(report.Resources) = %d, want 1; report=%+v", len(report.Resources), report)
	}
	return report.Resources[0]
}

func assertChangedFields(t *testing.T, change RecordChange, want []string) {
	t.Helper()
	if len(change.Changes) != len(want) {
		t.Fatalf("changed fields = %+v, want %v", change.Changes, want)
	}
	for i, field := range want {
		if change.Changes[i].Field != field {
			t.Fatalf("changed field %d = %q, want %q; changes=%+v", i, change.Changes[i].Field, field, change.Changes)
		}
	}
}

func testKeyedSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "rules",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational, AllowedModes: testAllModes()},
			{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: testStandardShareModes()},
			{Name: "lastModifiedTime", Classification: resources.ClassOperational, AllowedModes: testAllModes()},
		},
	}
}

func testSingletonSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "advanced-settings",
		Shape:      resources.ShapeSingleton,
		Operations: resources.SingletonOperations(),
		Fields: []resources.FieldSpec{
			{Name: "enabled", Classification: resources.ClassTenantConfig, AllowedModes: testStandardShareModes()},
		},
	}
}

func testIdentitylessSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "cloud-app-control",
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{
			{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: testStandardShareModes()},
			{
				Name:           "nested",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   testStandardShareModes(),
				Fields: []resources.FieldSpec{
					{Name: "a", Classification: resources.ClassOperational, AllowedModes: testAllModes()},
					{Name: "b", Classification: resources.ClassOperational, AllowedModes: testAllModes()},
				},
			},
			{Name: "lastModifiedTime", Classification: resources.ClassOperational, AllowedModes: testAllModes()},
		},
	}
}

func testNestedSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "nested-rules",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational, AllowedModes: testAllModes()},
			{
				Name:           "nested",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   testStandardShareModes(),
				Fields: []resources.FieldSpec{
					{Name: "allowed", Classification: resources.ClassTenantConfig, AllowedModes: testStandardShareModes()},
				},
			},
		},
	}
}

func testSecretSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "secret-rules",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational, AllowedModes: testAllModes()},
			{Name: "token", Classification: resources.ClassSecret},
		},
	}
}

func testUnrenderableSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "unrenderable-rules",
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{
			{Name: "standardOnly", Classification: resources.ClassTenantConfig, AllowedModes: []redact.Mode{redact.ModeStandard}},
		},
	}
}

func testSelfDescribingSecretSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "self-describing-rules",
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{
			{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: testStandardShareModes()},
		},
	}
}

func testProgressSpec(name string) resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       name,
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{
			{Name: "label", Classification: resources.ClassTenantConfig, AllowedModes: testStandardShareModes()},
		},
	}
}

func testAllModes() []redact.Mode {
	return []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}
}

func testStandardShareModes() []redact.Mode {
	return []redact.Mode{redact.ModeStandard, redact.ModeShare}
}
