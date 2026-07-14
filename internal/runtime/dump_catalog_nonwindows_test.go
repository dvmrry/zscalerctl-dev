//go:build !windows

package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	dumpartifact "github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestDumpCollectorForceUsesActiveCatalog(t *testing.T) {
	t.Parallel()

	const resource = "custom-engine-resource"
	if _, ok := resources.Catalog().FindSpec(resources.ProductZIA, resource); ok {
		t.Fatalf("built-in catalog unexpectedly contains zia/%s", resource)
	}
	spec := runtimeDumpListSpec(resources.ProductZIA, resource)
	catalog := resources.ResourceCatalog{spec}
	reader := &runtimeDumpReader{list: map[runtimeResourceKey][]resources.SourceRecord{
		{product: resources.ProductZIA, resource: resource}: {
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "HQ"}),
		},
	}}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)
	outDir := filepath.Join(t.TempDir(), "dump")
	request := machine.DumpRequest{
		OutputDir: outDir,
		Products:  []string{"zia"},
		Resources: []machine.DumpResourceSelector{{Product: "zia", Resource: resource}},
	}
	if _, err := collector.Dump(context.Background(), request, func(machine.Event) error { return nil }); err != nil {
		t.Fatalf("DumpCollector.Dump(custom catalog initial) error = %v, want nil", err)
	}
	request.Force = true
	if _, err := collector.Dump(context.Background(), request, func(machine.Event) error { return nil }); err != nil {
		t.Fatalf("DumpCollector.Dump(custom catalog force) error = %v, want nil", err)
	}
}

func TestDumpCollectorForceRejectsArtifactOutsideActiveCatalog(t *testing.T) {
	t.Parallel()

	globalSpec, ok := resources.Catalog().FindSpec(resources.ProductZIA, "locations")
	if !ok {
		t.Fatal("built-in catalog has no zia/locations resource")
	}
	outDir := filepath.Join(t.TempDir(), "dump")
	if err := dumpartifact.Write(outDir, redact.ModeStandard, dumpartifact.Result{
		Entries: []dumpartifact.ResourceDump{{
			Spec: globalSpec,
			Records: resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{
				"id": "1", "name": "global artifact",
			}}),
		}},
	}); err != nil {
		t.Fatalf("dump.Write(global artifact) error = %v", err)
	}
	manifestPath := filepath.Join(outDir, "manifest.json")
	manifestBefore, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("os.ReadFile(initial manifest) error = %v", err)
	}

	const customResource = "restricted-engine-resource"
	customSpec := runtimeDumpListSpec(resources.ProductZIA, customResource)
	reader := &runtimeDumpReader{list: map[runtimeResourceKey][]resources.SourceRecord{
		{product: resources.ProductZIA, resource: customResource}: {
			resources.NewSourceRecord(map[string]any{"id": "2", "name": "custom"}),
		},
	}}
	collector := NewDumpCollectorFromReader(
		reader,
		resources.ResourceCatalog{customSpec},
		redact.ModeStandard,
	)
	_, err = collector.Dump(context.Background(), machine.DumpRequest{
		OutputDir: outDir,
		Products:  []string{"zia"},
		Resources: []machine.DumpResourceSelector{{Product: "zia", Resource: customResource}},
		Force:     true,
	}, func(machine.Event) error { return nil })
	if err == nil {
		t.Fatal("DumpCollector.Dump(restricted catalog over global artifact) error = nil, want rejection")
	}
	manifestAfter, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		t.Fatalf("os.ReadFile(manifest after rejection) error = %v", readErr)
	}
	if !bytes.Equal(manifestAfter, manifestBefore) {
		t.Error("restricted-catalog force rejection changed the existing artifact")
	}
}
