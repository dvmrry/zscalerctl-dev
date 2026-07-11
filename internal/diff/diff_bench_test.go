package diff

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	benchmarkSelectedRecords   = 64
	benchmarkUnselectedRecords = 10_000
)

func BenchmarkCompareContextNarrowSelection(b *testing.B) {
	selected := benchmarkDiffSpec("selected-locations")
	unselected := benchmarkDiffSpec("unselected-locations")
	catalog := resources.ResourceCatalog{selected, unselected}
	selectedPayload := benchmarkDiffPayload(benchmarkSelectedRecords)
	unselectedPayload := benchmarkDiffPayload(benchmarkUnselectedRecords)
	dir := writeBenchmarkDiffDump(b, []benchmarkDiffEntry{
		{spec: selected, payload: selectedPayload, records: benchmarkSelectedRecords},
		{spec: unselected, payload: unselectedPayload, records: benchmarkUnselectedRecords},
	})
	opts := Options{
		Catalog: catalog,
		Resources: map[ResourceKey]bool{
			{Product: selected.Product, Name: selected.Name}: true,
		},
	}

	b.ReportAllocs()
	b.SetBytes(2 * int64(len(selectedPayload)+len(unselectedPayload)))
	b.ResetTimer()
	var retained Report
	for b.Loop() {
		var err error
		retained, err = CompareContext(context.Background(), dir, dir, opts, nil)
		if err != nil {
			b.Fatalf("CompareContext(narrow selection) error = %v, want nil", err)
		}
	}
	b.ReportMetric(float64(benchmarkUnselectedRecords), "unselected-records/op")
	runtime.KeepAlive(retained)
}

func BenchmarkUnselectedResourceParsing(b *testing.B) {
	for _, records := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("records-%d", records), func(b *testing.B) {
			benchmarkUnselectedResourceParsing(b, records)
		})
	}
}

func benchmarkUnselectedResourceParsing(b *testing.B, records int) {
	b.Helper()
	spec := benchmarkDiffSpec("unselected-locations")
	payload := benchmarkDiffPayload(records)
	dir := writeBenchmarkDiffDump(b, []benchmarkDiffEntry{{
		spec: spec, payload: payload, records: records,
	}})
	root, err := os.OpenRoot(dir)
	if err != nil {
		b.Fatalf("os.OpenRoot(%q) error = %v, want nil", dir, err)
	}
	b.Cleanup(func() {
		if err := root.Close(); err != nil {
			b.Errorf("os.Root.Close() error = %v, want nil", err)
		}
	})
	mr := dump.ManifestResource{
		Product: string(spec.Product),
		Name:    spec.Name,
		Status:  "ok",
		Path:    filepath.ToSlash(filepath.Join("resources", string(spec.Product), spec.Name+".json")),
		Records: records,
	}

	b.Run("full-decode", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(payload)))
		var retained []map[string]any
		for b.Loop() {
			var readErr error
			retained, readErr = readResource(context.Background(), root, mr, spec)
			if readErr != nil {
				b.Fatalf("readResource(%d records) error = %v, want nil", records, readErr)
			}
		}
		runtime.KeepAlive(retained)
	})

	b.Run("stream-count", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(payload)))
		var retained int
		for b.Loop() {
			var readErr error
			retained, readErr = readUnselectedResourceCount(context.Background(), root, mr, spec)
			if readErr != nil {
				b.Fatalf("readUnselectedResourceCount(%d records) error = %v, want nil", records, readErr)
			}
		}
		runtime.KeepAlive(retained)
	})
}

type benchmarkDiffEntry struct {
	spec    resources.ResourceSpec
	payload []byte
	records int
}

func writeBenchmarkDiffDump(b *testing.B, entries []benchmarkDiffEntry) string {
	b.Helper()
	dir := b.TempDir()
	manifest := dump.Manifest{
		Schema:      dump.ManifestSchemaID,
		CollectedAt: "2026-01-01T00:00:00Z",
		ToolVersion: "benchmark",
		Redaction:   string(redact.ModeStandard),
		Status:      "complete",
	}
	for _, entry := range entries {
		relPath := filepath.ToSlash(filepath.Join("resources", string(entry.spec.Product), entry.spec.Name+".json"))
		path := filepath.Join(dir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			b.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, entry.payload, 0o600); err != nil {
			b.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
		}
		manifest.Resources = append(manifest.Resources, dump.ManifestResource{
			Product: string(entry.spec.Product),
			Name:    entry.spec.Name,
			Shape:   string(entry.spec.EffectiveShape()),
			Status:  "ok",
			Path:    relPath,
			Records: entry.records,
		})
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		b.Fatalf("json.Marshal(benchmark manifest) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0o600); err != nil {
		b.Fatalf("os.WriteFile(benchmark manifest) error = %v, want nil", err)
	}
	return dir
}

func benchmarkDiffPayload(records int) []byte {
	var body bytes.Buffer
	body.Grow(records * 160)
	body.WriteByte('[')
	for i := 0; i < records; i++ {
		if i > 0 {
			body.WriteByte(',')
		}
		fmt.Fprintf(
			&body,
			`{"id":%d,"name":"branch-%06d","enabled":true,"ports":[80,443,8443],"metadata":{"region":"us-east","labels":["branch","managed","benchmark"]}}`,
			i,
			i,
		)
	}
	body.WriteByte(']')
	return body.Bytes()
}

func benchmarkDiffSpec(name string) resources.ResourceSpec {
	standard := []redact.Mode{redact.ModeStandard}
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       name,
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational, AllowedModes: standard},
			{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: standard},
			{Name: "enabled", Classification: resources.ClassTenantConfig, AllowedModes: standard},
			{Name: "ports", Classification: resources.ClassOperational, AllowedModes: standard},
			{
				Name:           "metadata",
				Classification: resources.ClassOperational,
				AllowedModes:   standard,
				Fields: []resources.FieldSpec{
					{Name: "region", Classification: resources.ClassOperational, AllowedModes: standard},
					{Name: "labels", Classification: resources.ClassOperational, AllowedModes: standard},
				},
			},
		},
	}
}
