package main

import (
	"context"
	"errors"
	"os"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/enginehost"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	machineruntime "github.com/dvmrry/zscalerctl/internal/runtime"
)

const testResource = "engine-test-locations"

var errUnexpectedOperation = errors.New("unexpected dump test-engine operation")

type reader struct{}

func (reader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	return []resources.SourceRecord{
		resources.NewSourceRecord(map[string]any{"id": "1", "name": "new"}),
	}, nil
}

func (reader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errUnexpectedOperation
}

func (reader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errUnexpectedOperation
}

var _ browser.RecordReader = reader{}

type engine struct {
	manifest  machine.EngineManifest
	collector *machineruntime.DumpCollector
}

func (e *engine) EngineManifest() machine.EngineManifest { return e.manifest }

func (*engine) DiscoverCatalog(context.Context, machine.CatalogRequest) (machine.CatalogResult, error) {
	return machine.CatalogResult{}, errUnexpectedOperation
}

func (*engine) InspectStatus(context.Context, machine.StatusRequest) (machine.StatusResult, error) {
	return machine.StatusResult{}, errUnexpectedOperation
}

func (*engine) LookupURL(context.Context, machine.URLLookupRequest) (machine.URLLookupResult, error) {
	return machine.URLLookupResult{}, errUnexpectedOperation
}

func (*engine) Read(context.Context, machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
	return machine.ResourceReadResult{}, errUnexpectedOperation
}

func (e *engine) Dump(
	ctx context.Context,
	request machine.DumpRequest,
	sink machine.EventSink,
) (machine.DumpResult, error) {
	return e.collector.Dump(ctx, request, sink)
}

func (*engine) Diff(context.Context, machine.DiffRequest, machine.EventSink) (machine.DiffResult, error) {
	return machine.DiffResult{}, errUnexpectedOperation
}

func main() {
	catalog := resources.ResourceCatalog{{
		Product:    resources.ProductZIA,
		Name:       testResource,
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
		},
	}}
	testEngine := &engine{
		manifest:  machine.EngineManifestFromCatalog(catalog),
		collector: machineruntime.NewDumpCollectorFromReader(reader{}, catalog, redact.ModeStandard),
	}
	host, err := enginehost.New(testEngine, "test")
	if err != nil {
		os.Exit(1)
	}
	err = host.Serve(context.Background(), enginehost.Streams{
		Input: os.Stdin, Output: os.Stdout,
		CloseInput: os.Stdin.Close, CloseOutput: os.Stdout.Close,
	})
	os.Exit(enginehost.ExitCode(err))
}
