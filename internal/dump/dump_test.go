package dump

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const dumpCanary = "dump-canary-secret"

type cancelWhenTempContext struct {
	dir      string
	done     chan struct{}
	canceled bool
}

func (c *cancelWhenTempContext) Deadline() (time.Time, bool) { return time.Time{}, false }

func (c *cancelWhenTempContext) Done() <-chan struct{} { return c.done }

func (c *cancelWhenTempContext) Err() error {
	if c.canceled {
		return context.Canceled
	}
	entries, err := os.ReadDir(c.dir)
	if err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".tmp-") {
				c.canceled = true
				close(c.done)
				return context.Canceled
			}
		}
	}
	return nil
}

func (c *cancelWhenTempContext) Value(any) any { return nil }

type cancelWhenStagedPathExistsContext struct {
	parent   string
	name     string
	done     chan struct{}
	canceled bool
}

func (c *cancelWhenStagedPathExistsContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (c *cancelWhenStagedPathExistsContext) Done() <-chan struct{} { return c.done }
func (c *cancelWhenStagedPathExistsContext) Value(any) any         { return nil }

func (c *cancelWhenStagedPathExistsContext) Err() error {
	if c.canceled {
		return context.Canceled
	}
	matches, err := filepath.Glob(filepath.Join(c.parent, ".zscalerctl-staging-*", c.name))
	if err == nil && len(matches) > 0 {
		c.canceled = true
		close(c.done)
		return context.Canceled
	}
	return nil
}

type authorizationJSONFixture struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

func (authorizationJSONFixture) OutputSafe() {}

func TestWriteContextPreCanceledCreatesNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		make func() (context.Context, context.CancelFunc)
		want error
	}{
		{
			name: "canceled",
			make: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			want: context.Canceled,
		},
		{
			name: "deadline exceeded",
			make: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			},
			want: context.DeadlineExceeded,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.make()
			defer cancel()
			dir := filepath.Join(t.TempDir(), "dump")
			err := WriteContext(ctx, dir, redact.ModeStandard, Result{})
			if err != tt.want {
				t.Fatalf("WriteContext(%q) error = %v, want identity %v", dir, err, tt.want)
			}
			if _, statErr := os.Stat(dir); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", dir, statErr)
			}
		})
	}
}

func TestWriteContextCancellationBeforeFinalizationCleansTempAndFinal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	ctx := &cancelWhenTempContext{dir: dir, done: make(chan struct{})}
	err := writeFileExclusiveContext(ctx, path, []byte("payload"))
	if err != context.Canceled {
		t.Fatalf("writeFileExclusiveContext(%q) error = %v, want identity %v", path, err, context.Canceled)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", path, statErr)
	}
	leftovers, globErr := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	if globErr != nil {
		t.Fatalf("glob temp files error = %v", globErr)
	}
	if len(leftovers) != 0 {
		t.Errorf("temp files left after canceled finalization = %v, want none", leftovers)
	}
}

func TestWriteContextCancellationLeavesDestinationUntouched(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	dir := filepath.Join(parent, "dump")
	ctx := &cancelWhenStagedPathExistsContext{
		parent: parent,
		name:   "redaction_report.json",
		done:   make(chan struct{}),
	}
	err := WriteContext(ctx, dir, redact.ModeStandard, Result{})
	if err != context.Canceled {
		t.Fatalf("WriteContext(cancel after report) error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Stat(dir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("os.Stat(destination) error = %v, want os.ErrNotExist", statErr)
	}
	leftovers, globErr := filepath.Glob(filepath.Join(parent, ".zscalerctl-staging-*"))
	if globErr != nil {
		t.Fatalf("filepath.Glob(staging) error = %v, want nil", globErr)
	}
	if len(leftovers) != 0 {
		t.Errorf("staging directories after canceled WriteContext = %v, want none", leftovers)
	}
}

func TestPublishContextFailureDoesNotDeleteSubstitutedStagingPath(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	destination := filepath.Join(parent, "dump")
	if err := Write(destination, redact.ModeStandard, Result{}); err != nil {
		t.Fatalf("Write(%q, existing destination) error = %v, want nil", destination, err)
	}
	const sentinel = "foreign staging replacement must survive\n"
	var hookErr error
	var originalStaging string
	err := publishContextWithHooks(
		context.Background(),
		destination,
		redact.ModeShare,
		Result{},
		false,
		publishContextHooks{
			beforeStagingCleanupRelocate: func(stagingPath string) {
				originalStaging = stagingPath + "-original"
				if renameErr := os.Rename(stagingPath, originalStaging); renameErr != nil {
					hookErr = renameErr
					return
				}
				hookErr = os.WriteFile(stagingPath, []byte(sentinel), filePerm)
			},
		},
	)
	if hookErr != nil {
		t.Fatalf("staging substitution hook error = %v, want nil", hookErr)
	}
	if !errors.Is(err, ErrUnsafeOverwrite) {
		t.Fatalf("publishContextWithHooks(existing destination) error = %v, want ErrUnsafeOverwrite", err)
	}
	matches, globErr := filepath.Glob(filepath.Join(parent, ".zscalerctl-staging-*"))
	if globErr != nil {
		t.Fatalf("filepath.Glob(staging) error = %v, want nil", globErr)
	}
	if len(matches) == 0 {
		t.Fatal("staging substitution paths = none, want preserved sentinel and original staging")
	}
	foundSentinel := false
	for _, path := range matches {
		body, readErr := os.ReadFile(path)
		if readErr == nil && string(body) == sentinel {
			foundSentinel = true
		}
	}
	if !foundSentinel {
		t.Errorf("staging paths %v contain no preserved sentinel", matches)
	}
	if info, statErr := os.Stat(originalStaging); statErr != nil || !info.IsDir() {
		t.Errorf("original process staging after substitution = (%v, %v), want preserved directory", info, statErr)
	}
}

func TestPublishContextFailureDoesNotDeleteSubstitutedDiscardPath(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	destination := filepath.Join(parent, "dump")
	if err := Write(destination, redact.ModeStandard, Result{}); err != nil {
		t.Fatalf("Write(%q, existing destination) error = %v, want nil", destination, err)
	}
	const sentinel = "foreign discard replacement must survive\n"
	var hookErr error
	var originalDiscard string
	var stagingPath string
	err := publishContextWithHooks(
		context.Background(),
		destination,
		redact.ModeShare,
		Result{},
		false,
		publishContextHooks{
			beforeStagingCleanupRelocate: func(path string) {
				stagingPath = path
				matches, globErr := filepath.Glob(filepath.Join(parent, ".zscalerctl-discard-*"))
				if globErr != nil || len(matches) != 1 {
					hookErr = fmt.Errorf("discard glob = %v, %v; want one path", matches, globErr)
					return
				}
				discardPath := matches[0]
				originalDiscard = discardPath + "-original"
				if renameErr := os.Rename(discardPath, originalDiscard); renameErr != nil {
					hookErr = renameErr
					return
				}
				hookErr = os.WriteFile(discardPath, []byte(sentinel), filePerm)
			},
		},
	)
	if hookErr != nil {
		t.Fatalf("discard substitution hook error = %v, want nil", hookErr)
	}
	if !errors.Is(err, ErrUnsafeOverwrite) {
		t.Fatalf("publishContextWithHooks(existing destination) error = %v, want ErrUnsafeOverwrite", err)
	}
	matches, globErr := filepath.Glob(filepath.Join(parent, ".zscalerctl-discard-*"))
	if globErr != nil {
		t.Fatalf("filepath.Glob(discard) error = %v, want nil", globErr)
	}
	foundSentinel := false
	for _, path := range matches {
		body, readErr := os.ReadFile(path)
		if readErr == nil && string(body) == sentinel {
			foundSentinel = true
		}
	}
	if !foundSentinel {
		t.Errorf("discard paths %v contain no preserved sentinel", matches)
	}
	if info, statErr := os.Stat(originalDiscard); statErr != nil || !info.IsDir() {
		t.Errorf("original discard after substitution = (%v, %v), want preserved directory", info, statErr)
	}
	if info, statErr := os.Stat(stagingPath); statErr != nil || !info.IsDir() {
		t.Errorf("process staging after discard substitution = (%v, %v), want preserved directory", info, statErr)
	}
}

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

func TestRenameNoReplaceRefusesExistingDirectory(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	source := filepath.Join(parent, "source")
	destination := filepath.Join(parent, "destination")
	if err := os.Mkdir(source, dirPerm); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", source, err)
	}
	if err := os.Mkdir(destination, dirPerm); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", destination, err)
	}

	err := renameNoReplace(source, destination)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("renameNoReplace(existing destination) error = %v, want os.ErrExist", err)
	}
	for _, path := range []string{source, destination} {
		if info, statErr := os.Stat(path); statErr != nil || !info.IsDir() {
			t.Errorf("os.Stat(%q) = (%v, %v), want existing directory", path, info, statErr)
		}
	}
}

func TestWriteJSONFilePreservesValidJSONForAuthorizationText(t *testing.T) {
	t.Parallel()

	const authorizationCanary = "dump-authorization-canary"
	path := filepath.Join(t.TempDir(), "resource.json")
	err := writeJSONFile(path, redact.ModeStandard, authorizationJSONFixture{
		Message: "set the Authorization: Bearer " + authorizationCanary + " header",
		Count:   42,
	})
	if err != nil {
		t.Fatalf("writeJSONFile(%q) error = %v, want nil", path, err)
	}

	body := readFile(t, path)
	if !json.Valid([]byte(body)) {
		t.Errorf("writeJSONFile(%q) = invalid JSON %q, want valid JSON", path, body)
	}
	if strings.Contains(body, authorizationCanary) {
		t.Errorf("writeJSONFile(%q) = %q, want no %q", path, body, authorizationCanary)
	}
	if !strings.Contains(body, "<REDACTED:SECRET>") {
		t.Errorf("writeJSONFile(%q) = %q, want secret marker", path, body)
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
	if manifest.Schema != "zscalerctl.dump.manifest.v2" {
		t.Errorf("manifest schema = %q, want zscalerctl.dump.manifest.v2", manifest.Schema)
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

func TestWriteManifestCollectionMetadata(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "dump")
	entry := projectedDumpEntry(t, resources.ProductZIA, "locations", []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": 1, "name": "HQ", "description": ""}),
	})
	before := time.Now().UTC().Add(-time.Minute)
	if err := Write(dir, redact.ModeStandard, Result{Entries: []ResourceDump{entry}}); err != nil {
		t.Fatalf("Write(%q, complete result) error = %v, want nil", dir, err)
	}
	after := time.Now().UTC().Add(time.Minute)

	var manifest Manifest
	readJSON(t, filepath.Join(dir, "manifest.json"), &manifest)

	collected, err := time.Parse(time.RFC3339, manifest.CollectedAt)
	if err != nil {
		t.Fatalf("time.Parse(RFC3339, %q) error = %v, want nil", manifest.CollectedAt, err)
	}
	if !strings.HasSuffix(manifest.CollectedAt, "Z") {
		t.Errorf("manifest collected_at = %q, want UTC timestamp ending in Z", manifest.CollectedAt)
	}
	if collected.Before(before) || collected.After(after) {
		t.Errorf("manifest collected_at = %v, want between %v and %v", collected, before, after)
	}
	if manifest.ToolVersion == "" {
		t.Errorf("manifest tool_version = %q, want non-empty version string", manifest.ToolVersion)
	}
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
