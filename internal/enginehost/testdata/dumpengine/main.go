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

const (
	testResource       = "engine-test-locations"
	testSecondResource = "engine-test-rules"
)

var errUnexpectedOperation = errors.New("unexpected test-engine operation")

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
	runtime   *machineruntime.Engine
	collector *machineruntime.DumpCollector
}

func (e *engine) EngineManifest() machine.EngineManifest { return e.runtime.EngineManifest() }

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

func (e *engine) Diff(
	ctx context.Context,
	request machine.DiffRequest,
	sink machine.EventSink,
) (machine.DiffResult, error) {
	return e.runtime.Diff(ctx, request, sink)
}

func resourceSpec(name string) resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       name,
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
	}
}

func main() {
	catalog := resources.ResourceCatalog{
		resourceSpec(testResource),
		resourceSpec(testSecondResource),
	}
	runtimeEngine, err := machineruntime.NewEngine(machineruntime.Options{Catalog: catalog})
	if err != nil {
		os.Exit(1)
	}
	testEngine := &engine{
		runtime:   runtimeEngine,
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
