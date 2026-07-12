package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// Engine is the common local operation coordinator. It owns host-supplied
// runtime construction options but loads config and constructs live readers
// per operation, matching the CLI's current credential lifetime.
type Engine struct {
	opts Options
}

// NewEngine constructs a coordinator without loading config, resolving a
// provider, constructing an SDK reader, or contacting Zscaler.
func NewEngine(opts Options) (*Engine, error) {
	opts.Env = append([]string(nil), opts.Env...)
	opts.Catalog = catalogFromOptions(opts.Catalog)
	if err := resources.AssertReadOnly(opts.Catalog...); err != nil {
		return nil, fmt.Errorf("construct engine: catalog must be tenant read-only: %w", err)
	}
	return &Engine{opts: opts}, nil
}

// Manifest returns the supported machine.v1 compatibility manifest for the
// engine catalog without loading config.
func (e *Engine) Manifest() machine.Manifest {
	if e == nil {
		return machine.ManifestFromCatalog(nil)
	}
	return machine.ManifestFromCatalog(e.opts.Catalog)
}

// EngineManifest returns candidate typed capability discovery without loading
// config or constructing a live runtime.
func (e *Engine) EngineManifest() machine.EngineManifest {
	if e == nil {
		manifest := machine.EngineManifestFromCatalog(nil)
		manifest.Capabilities = manifest.Capabilities[:1]
		return manifest
	}
	return machine.EngineManifestFromCatalog(e.opts.Catalog)
}

// DiscoverCatalog returns a deep typed snapshot without loading config.
func (e *Engine) DiscoverCatalog(
	ctx context.Context,
	req machine.CatalogRequest,
) (machine.CatalogResult, error) {
	if e == nil {
		return machine.CatalogResult{}, errors.New("engine runtime is nil")
	}
	return (machine.Executor{Catalog: e.opts.Catalog}).DiscoverCatalog(ctx, req)
}

// InspectStatus loads config for one action and returns a sanitized SDK-free
// status result.
func (e *Engine) InspectStatus(
	ctx context.Context,
	req machine.StatusRequest,
) (machine.StatusResult, error) {
	if e == nil {
		return machine.StatusResult{}, errors.New("engine runtime is nil")
	}
	if !isSupportedStatusOperation(req.Operation) {
		return machine.StatusResult{}, unsupportedStatusOperationError()
	}
	inspector, err := newStatusInspector(ctx, e.options(), req.Operation)
	if err != nil {
		return machine.StatusResult{}, err
	}
	return inspector.Inspect(ctx, req)
}

// LookupURL validates one typed URL-classification batch before loading config
// or constructing a live reader, then returns only sanitized SDK-free values.
func (e *Engine) LookupURL(
	ctx context.Context,
	req machine.URLLookupRequest,
) (machine.URLLookupResult, error) {
	if e == nil {
		return machine.URLLookupResult{}, errors.New("engine runtime is nil")
	}
	ctx = nonNilContext(ctx)
	urls, err := prepareURLLookupRequest(ctx, req)
	if err != nil {
		return machine.URLLookupResult{}, err
	}
	lookup, err := NewURLLookup(ctx, e.options())
	if err != nil {
		return machine.URLLookupResult{}, err
	}
	return lookup.lookupPrepared(ctx, urls)
}

// Read constructs one live runtime and executes a typed resource read.
func (e *Engine) Read(
	ctx context.Context,
	req machine.ResourceReadRequest,
) (machine.ResourceReadResult, error) {
	if e == nil {
		return machine.ResourceReadResult{}, errors.New("engine runtime is nil")
	}
	if !machine.IsResourceReadOperation(req.Operation) {
		return (machine.Executor{}).Read(ctx, req)
	}
	machineRuntime, err := NewMachine(ctx, e.options())
	if err != nil {
		return machine.ResourceReadResult{}, err
	}
	return machineRuntime.Read(ctx, req)
}

// ReadStream constructs one live runtime and delivers a typed resource-read
// event stream synchronously.
func (e *Engine) ReadStream(
	ctx context.Context,
	req machine.ResourceReadRequest,
	sink machine.EventSink,
) error {
	if e == nil {
		return errors.New("engine runtime is nil")
	}
	if !machine.IsResourceReadOperation(req.Operation) {
		return (machine.Executor{}).ReadStream(ctx, req, sink)
	}
	machineRuntime, err := NewMachine(ctx, e.options())
	if err != nil {
		return err
	}
	return machineRuntime.ReadStream(ctx, req, sink)
}

// Execute constructs one live runtime and runs the candidate compatibility
// request envelope.
func (e *Engine) Execute(
	ctx context.Context,
	req machine.Request,
) (machine.Response, error) {
	if e == nil {
		return machine.Response{}, errors.New("engine runtime is nil")
	}
	machineRuntime, err := NewMachine(ctx, e.options())
	if err != nil {
		return machine.Response{}, err
	}
	return machineRuntime.Execute(ctx, req)
}

// ExecuteStream constructs one live runtime and delivers compatibility events
// synchronously.
func (e *Engine) ExecuteStream(
	ctx context.Context,
	req machine.Request,
	sink machine.EventSink,
) error {
	if e == nil {
		return errors.New("engine runtime is nil")
	}
	machineRuntime, err := NewMachine(ctx, e.options())
	if err != nil {
		return err
	}
	return machineRuntime.ExecuteStream(ctx, req, sink)
}

func (e *Engine) options() Options {
	opts := e.opts
	opts.Env = append([]string(nil), e.opts.Env...)
	opts.Catalog = copyCatalog(e.opts.Catalog)
	return opts
}
