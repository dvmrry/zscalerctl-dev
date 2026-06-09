package dump

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const dumpCanary = "dump-canary-secret"

func TestEnsureDirRejectsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", target, err)
	}
	link := filepath.Join(dir, "out")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("os.Symlink(%q, %q) error = %v; symlinks unavailable", target, link, err)
	}

	err := ensureDir(link)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("ensureDir(symlink) error = %v, want ErrUnsafePath", err)
	}
}

func TestWriteFileExclusiveWritesAtomicallyAndLeavesNoTemp(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := writeFileExclusive(path, []byte("payload")); err != nil {
		t.Fatalf("writeFileExclusive(new) error = %v, want nil", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	if string(got) != "payload" {
		t.Errorf("final file content = %q, want %q", got, "payload")
	}
	assertMode(t, path, filePerm)

	// The temp+rename path must not leave the intermediate file behind.
	leftovers, err := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	if err != nil {
		t.Fatalf("glob temp files error = %v", err)
	}
	if len(leftovers) != 0 {
		t.Errorf("temp files left after write = %v, want none", leftovers)
	}
}

func TestWriteFileExclusiveRefusesExistingPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte("existing"), filePerm); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	err := writeFileExclusive(path, []byte("new"))
	if !errors.Is(err, ErrUnsafeOverwrite) {
		t.Fatalf("writeFileExclusive(existing) error = %v, want ErrUnsafeOverwrite", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	if string(got) != "existing" {
		t.Errorf("existing file content = %q, want unchanged", got)
	}
}

func TestWriteCompleteDumpShapePermissionsAndRedaction(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "dump")
	entry := projectedDumpEntry(t, resources.ProductZIA, "locations", []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{
			"id":          1,
			"name":        "HQ",
			"description": "operator note psk=" + dumpCanary,
			"secretValue": dumpCanary,
		}),
		resources.NewSourceRecord(map[string]any{
			"id":          2,
			"name":        "Branch",
			"description": "",
			"secretValue": dumpCanary,
		}),
	})

	if err := Write(dir, redact.ModeStandard, Result{Entries: []ResourceDump{entry}}); err != nil {
		t.Fatalf("Write(%q, complete result) error = %v, want nil", dir, err)
	}

	assertMode(t, dir, 0o700)
	assertMode(t, filepath.Join(dir, "resources"), 0o700)
	assertMode(t, filepath.Join(dir, "resources", "zia"), 0o700)

	resourcePath := filepath.Join(dir, "resources", "zia", "locations.json")
	manifestPath := filepath.Join(dir, "manifest.json")
	reportPath := filepath.Join(dir, "redaction_report.json")
	for _, path := range []string{resourcePath, manifestPath, reportPath} {
		assertMode(t, path, 0o600)
	}
	if _, err := os.Stat(filepath.Join(dir, "errors.ndjson")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(errors.ndjson) error = %v, want os.ErrNotExist", err)
	}

	var records []map[string]any
	readJSON(t, resourcePath, &records)
	if got, want := len(records), 2; got != want {
		t.Fatalf("resource records length = %d, want %d", got, want)
	}
	body := readFile(t, resourcePath)
	if strings.Contains(body, dumpCanary) {
		t.Errorf("resource file = %q, want no canary value %q", body, dumpCanary)
	}
	if !strings.Contains(body, "<REDACTED:SECRET>") {
		t.Errorf("resource file = %q, want redaction marker", body)
	}
	if _, ok := records[0]["secretValue"]; ok {
		t.Errorf("resource record keys = %#v, want secretValue dropped", records[0])
	}

	var manifest Manifest
	readJSON(t, manifestPath, &manifest)
	if manifest.Schema != "zscalerctl.dump.manifest.v1" {
		t.Errorf("manifest schema = %q, want zscalerctl.dump.manifest.v1", manifest.Schema)
	}
	if manifest.Status != "complete" {
		t.Errorf("manifest status = %q, want complete", manifest.Status)
	}
	if manifest.Warning != dumpWarning {
		t.Errorf("manifest warning = %q, want %q", manifest.Warning, dumpWarning)
	}
	if manifest.Errors != 0 || manifest.ErrorsPath != "" {
		t.Errorf("manifest error metadata = (%d, %q), want zero values", manifest.Errors, manifest.ErrorsPath)
	}
	if got, want := len(manifest.Resources), 1; got != want {
		t.Fatalf("manifest resources length = %d, want %d", got, want)
	}
	wantResource := ManifestResource{
		Product: "zia",
		Name:    "locations",
		Status:  "ok",
		Path:    "resources/zia/locations.json",
		Records: 2,
	}
	if manifest.Resources[0] != wantResource {
		t.Errorf("manifest resource = %#v, want %#v", manifest.Resources[0], wantResource)
	}

	var report RedactionReport
	readJSON(t, reportPath, &report)
	if got, want := len(report.Resources), 1; got != want {
		t.Fatalf("redaction report resources length = %d, want %d", got, want)
	}
	gotReport := report.Resources[0]
	if gotReport.Path != "resources/zia/locations.json" || gotReport.Records != 2 {
		t.Errorf("redaction report resource = %#v, want path resources/zia/locations.json and 2 records", gotReport)
	}
	assertStringSliceEqual(t, "included_fields", gotReport.IncludedFields, []string{"description", "id", "name"})
	assertStringSliceEqual(t, "dropped_fields", gotReport.DroppedFields, []string{"secretValue"})
	assertStringSliceEqual(t, "redacted_fields", gotReport.RedactedFields, []string{"description"})
}

func TestWritePartialDumpShapeAndErrorNDJSON(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "dump")
	entry := projectedDumpEntry(t, resources.ProductZIA, "locations", []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "HQ", "description": ""}),
	})
	result := Result{
		Entries: []ResourceDump{entry},
		Errors: []ResourceError{
			NewResourceError(resources.ProductZIA, "rule-labels", "list", "list_failed"),
		},
	}

	if err := Write(dir, redact.ModeShare, result); err != nil {
		t.Fatalf("Write(%q, partial result) error = %v, want nil", dir, err)
	}

	errorsPath := filepath.Join(dir, "errors.ndjson")
	assertMode(t, errorsPath, 0o600)

	var manifest Manifest
	readJSON(t, filepath.Join(dir, "manifest.json"), &manifest)
	if manifest.Status != "partial" {
		t.Errorf("manifest status = %q, want partial", manifest.Status)
	}
	if manifest.Errors != 1 || manifest.ErrorsPath != "errors.ndjson" {
		t.Errorf("manifest error metadata = (%d, %q), want (1, errors.ndjson)", manifest.Errors, manifest.ErrorsPath)
	}
	if got, want := len(manifest.Resources), 2; got != want {
		t.Fatalf("manifest resources length = %d, want %d", got, want)
	}
	if manifest.Resources[1] != (ManifestResource{
		Product:   "zia",
		Name:      "rule-labels",
		Status:    "error",
		Records:   0,
		Operation: "list",
		ErrorKind: "list_failed",
	}) {
		t.Errorf("manifest error resource = %#v, want value-free error entry", manifest.Resources[1])
	}

	lines := strings.Split(strings.TrimSpace(readFile(t, errorsPath)), "\n")
	if got, want := len(lines), 1; got != want {
		t.Fatalf("errors.ndjson lines = %d, want %d", got, want)
	}
	var gotErr ResourceError
	if err := json.Unmarshal([]byte(lines[0]), &gotErr); err != nil {
		t.Fatalf("json.Unmarshal(errors.ndjson line) error = %v, want nil", err)
	}
	wantErr := NewResourceError(resources.ProductZIA, "rule-labels", "list", "list_failed")
	if gotErr != wantErr {
		t.Errorf("errors.ndjson record = %#v, want %#v", gotErr, wantErr)
	}
}

func TestWriteRefusesOverwriteBeforeCreatingNewFiles(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "dump")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("existing"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(manifest.json) error = %v, want nil", err)
	}

	err := Write(dir, redact.ModeStandard, Result{
		Entries: []ResourceDump{
			projectedDumpEntry(t, resources.ProductZIA, "locations", []resources.SourceRecord{
				resources.NewSourceRecord(map[string]any{"id": 1, "name": "HQ", "description": ""}),
			}),
		},
	})
	if !errors.Is(err, ErrUnsafeOverwrite) {
		t.Fatalf("Write(%q, existing manifest) error = %v, want ErrUnsafeOverwrite", dir, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "redaction_report.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(redaction_report.json) error = %v, want os.ErrNotExist", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "resources")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(resources) error = %v, want os.ErrNotExist", err)
	}
}

func TestWriteRejectsUnsafeProductAndResourceSegments(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		product  resources.Product
		resource string
	}{
		{name: "empty product", product: "", resource: "locations"},
		{name: "product slash", product: "zia/private", resource: "locations"},
		{name: "product dot", product: "zia.private", resource: "locations"},
		{name: "empty resource", product: resources.ProductZIA, resource: ""},
		{name: "resource traversal", product: resources.ProductZIA, resource: "../locations"},
		{name: "resource slash", product: resources.ProductZIA, resource: "location/groups"},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(t.TempDir(), "dump")
			err := Write(dir, redact.ModeStandard, Result{
				Entries: []ResourceDump{{
					Spec: resources.ResourceSpec{
						Product: tt.product,
						Name:    tt.resource,
					},
					Records: resources.NewProjectedRecords(nil),
				}},
			})
			if !errors.Is(err, ErrUnsafePath) {
				t.Fatalf("Write(%q, %s/%s) error = %v, want ErrUnsafePath", dir, tt.product, tt.resource, err)
			}
			if _, statErr := os.Stat(dir); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", dir, statErr)
			}
		})
	}
}

func TestWriteAggregatesReportFields(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "dump")
	spec := testSpec(resources.ProductZIA, "locations")
	result := Result{
		Entries: []ResourceDump{{
			Spec:    spec,
			Records: resources.NewProjectedRecords(nil),
			Reports: []resources.ProjectionReport{
				{
					IncludedFields: []string{"name", "id", "name"},
					DroppedFields:  []string{"vpnCredentials", "secretValue"},
					RedactedFields: []string{"description"},
				},
				{
					IncludedFields: []string{"description", "id"},
					DroppedFields:  []string{"secretValue"},
					RedactedFields: []string{"name", "description"},
				},
			},
		}},
	}

	if err := Write(dir, redact.ModeStandard, result); err != nil {
		t.Fatalf("Write(%q, report aggregation result) error = %v, want nil", dir, err)
	}

	var report RedactionReport
	readJSON(t, filepath.Join(dir, "redaction_report.json"), &report)
	if got, want := len(report.Resources), 1; got != want {
		t.Fatalf("redaction report resources length = %d, want %d", got, want)
	}
	assertStringSliceEqual(t, "included_fields", report.Resources[0].IncludedFields, []string{"description", "id", "name"})
	assertStringSliceEqual(t, "dropped_fields", report.Resources[0].DroppedFields, []string{"secretValue", "vpnCredentials"})
	assertStringSliceEqual(t, "redacted_fields", report.Resources[0].RedactedFields, []string{"description", "name"})
}

func projectedDumpEntry(t *testing.T, product resources.Product, name string, records []resources.SourceRecord) ResourceDump {
	t.Helper()

	spec := testSpec(product, name)
	projected, reports, err := resources.ProjectRecordsAndVerify(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(%s/%s) error = %v, want nil", product, name, err)
	}
	return ResourceDump{
		Spec:    spec,
		Records: projected,
		Reports: reports,
	}
}

func testSpec(product resources.Product, name string) resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    product,
		Name:       name,
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
			},
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
			{
				Name:                   "description",
				Classification:         resources.ClassFreeText,
				AllowedModes:           []redact.Mode{redact.ModeStandard},
				StandardFreeTextReason: "dump package test free text",
			},
		},
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	return string(body)
}

func readJSON(t *testing.T, path string, out any) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v, want nil; body=%s", path, err, string(body))
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("os.Stat(%q).Mode().Perm() = %#o, want %#o", path, got, want)
	}
}

func assertStringSliceEqual(t *testing.T, label string, got []string, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s = %#v, want %#v", label, got, want)
			return
		}
	}
}
