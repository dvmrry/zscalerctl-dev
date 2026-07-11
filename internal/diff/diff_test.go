package diff

import (
	"context"
	"encoding/json"
	"errors"
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
	newDir := writeTestDump(t, catalog, dumpFixture{})

	_, err := Compare(oldDir, newDir, Options{Catalog: catalog})
	if !errors.Is(err, ErrPartialDumpInput) {
		t.Fatalf("Compare() error = %v, want ErrPartialDumpInput", err)
	}
	if _, err := Compare(oldDir, newDir, Options{Catalog: catalog, AllowPartial: true}); err != nil {
		t.Fatalf("Compare(... AllowPartial) error = %v", err)
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
	truncateTestFile(t, filepath.Join(oldDir, "manifest.json"), maxManifestBytes+1)

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
			Shape:   string(entry.spec.EffectiveShape()),
			Status:  "ok",
			Path:    relPath,
			Records: countRecords(t, entry.payload),
		})
	}
	if status == "partial" {
		manifest.Errors = 1
		manifest.Resources = append(manifest.Resources, dump.ManifestResource{
			Product:   string(catalog[0].Product),
			Name:      catalog[0].Name,
			Status:    "error",
			Operation: "list",
			ErrorKind: "live_access_failed",
		})
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent(manifest): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0o600); err != nil {
		t.Fatalf("WriteFile(manifest): %v", err)
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
