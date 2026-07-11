package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

const dumpEventOperation machine.Operation = "dump"

// DumpCollectOptions configures one dump collection run.
type DumpCollectOptions struct {
	ContinueOnError bool
}

// DumpCollector is the trusted live dump collection facade.
type DumpCollector struct {
	reader    browser.RecordReader
	catalog   resources.ResourceCatalog
	redaction redact.Mode
}

type resourceSessionProvider interface {
	Session(context.Context, resources.Product) (zscaler.ResourceSession, error)
}

// NewDumpCollector loads runtime config, resolves credentials, constructs the
// SDK-backed read-only reader, and returns a dump collection facade.
func NewDumpCollector(ctx context.Context, opts Options) (*DumpCollector, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	env := append([]string(nil), opts.Env...)
	loadConfig := opts.loadConfig
	if loadConfig == nil {
		loadConfig = config.LoadConfig
	}
	cfg, err := loadConfig(env, config.LoadOptions{
		Profile:    opts.Profile,
		ConfigPath: opts.ConfigPath,
	})
	if err != nil {
		return nil, err
	}
	return NewDumpCollectorFromConfig(ctx, cfg, opts)
}

// NewDumpCollectorFromConfig resolves credentials from an already-loaded
// effective config, constructs the SDK-backed read-only reader, and returns a
// dump collection facade.
func NewDumpCollectorFromConfig(
	ctx context.Context,
	cfg config.Config,
	opts Options,
) (*DumpCollector, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	reader, err := newReaderFromConfig(ctx, &cfg, opts)
	if err != nil {
		return nil, err
	}
	return newDumpCollectorFromReader(reader, catalogFromOptions(opts.Catalog), cfg.Defaults.Redaction), nil
}

// NewDumpCollectorFromReader constructs a dump collector around an
// already-trusted read-only record reader.
func NewDumpCollectorFromReader(
	reader browser.RecordReader,
	catalog resources.ResourceCatalog,
	mode redact.Mode,
) *DumpCollector {
	return newDumpCollectorFromReader(reader, catalog, mode)
}

// Collect reads, projects, and verifies the selected resource specs into a dump
// result through the same event-producing path used by streaming consumers.
// Fatal runtime failures return errors; per-resource failures are recorded when
// ContinueOnError is set.
func (c *DumpCollector) Collect(
	ctx context.Context,
	specs []resources.ResourceSpec,
	opts DumpCollectOptions,
) (dump.Result, error) {
	return c.CollectStream(ctx, specs, opts, func(machine.Event) error { return nil })
}

// CollectStream reads, projects, and verifies selected resource specs while
// delivering ordered candidate events synchronously on the caller's goroutine.
// It starts no goroutines. Record events cross the boundary only after
// projection and verification; fatal events are sanitized while the returned
// Go error preserves the original in-process error identity. If terminal-event
// delivery also fails, the return joins both errors without exposing the sink's
// raw error or panic value.
func (c *DumpCollector) CollectStream(
	ctx context.Context,
	specs []resources.ResourceSpec,
	opts DumpCollectOptions,
	sink machine.EventSink,
) (dump.Result, error) {
	result := dump.Result{}
	selectedSpecs := copyResourceSpecs(specs)
	if ctx == nil {
		ctx = context.Background()
	}
	stream, err := machine.StartEventStream(sink, dumpEventOperation, "", "", len(selectedSpecs))
	if err != nil {
		return result, err
	}
	result, counts, err := c.collectIntoStream(ctx, selectedSpecs, opts, stream)
	if err != nil {
		return result, err
	}
	if err := stream.Complete(machine.Event{
		Records:   counts.records,
		Resources: counts.resources,
		Warnings:  counts.warnings,
	}); err != nil {
		return result, err
	}
	return result, nil
}

type dumpCollectionCounts struct {
	records   int
	resources int
	warnings  int
}

func (c *DumpCollector) collectIntoStream(
	ctx context.Context,
	selectedSpecs []resources.ResourceSpec,
	opts DumpCollectOptions,
	stream *machine.EventStream,
) (dump.Result, dumpCollectionCounts, error) {
	result := dump.Result{}
	if c == nil {
		err := errors.New("dump collector is nil")
		return result, dumpCollectionCounts{}, finishDumpStreamFailure(
			stream, err, dumpEventOperation, "", "", machine.ErrorKindInternal,
		)
	}
	if err := resources.AssertReadOnly(c.catalog...); err != nil {
		return result, dumpCollectionCounts{}, finishDumpStreamFailure(
			stream, err, dumpEventOperation, "", "", machine.ErrorKindUnsupportedOperation,
		)
	}
	if err := resources.AssertReadOnly(selectedSpecs...); err != nil {
		return result, dumpCollectionCounts{}, finishDumpStreamFailure(
			stream, err, dumpEventOperation, "", "", machine.ErrorKindUnsupportedOperation,
		)
	}

	readers := make(map[resources.Product]browser.RecordReader)
	recordCount := 0
	for i, spec := range selectedSpecs {
		if err := ctx.Err(); err != nil {
			return result, dumpCollectionCounts{}, finishDumpStreamFailure(
				stream, err, dumpReadOperation(spec), string(spec.Product), spec.Name, machine.ErrorKindLiveAccessFailed,
			)
		}
		reader, ok := readers[spec.Product]
		if !ok {
			var cleanup func()
			var err error
			reader, cleanup, err = c.readerForProduct(ctx, spec.Product)
			if err != nil {
				return result, dumpCollectionCounts{}, finishDumpStreamFailure(
					stream, err, dumpReadOperation(spec), string(spec.Product), spec.Name, machine.ErrorKindLiveAccessFailed,
				)
			}
			readers[spec.Product] = reader
			defer cleanup()
		}
		if err := stream.Emit(machine.Event{
			Kind:     machine.EventProgress,
			Product:  string(spec.Product),
			Resource: spec.Name,
			Done:     i + 1,
			Total:    len(selectedSpecs),
		}); err != nil {
			return result, dumpCollectionCounts{}, err
		}
		if spec.SupportsReadOperation("show") {
			count, err := c.collectShow(ctx, reader, spec, opts.ContinueOnError, stream, &result)
			if err != nil {
				return result, dumpCollectionCounts{}, err
			}
			recordCount += count
			continue
		}
		count, err := c.collectList(ctx, reader, spec, opts.ContinueOnError, stream, &result)
		if err != nil {
			return result, dumpCollectionCounts{}, err
		}
		recordCount += count
	}
	return result, dumpCollectionCounts{
		records:   recordCount,
		resources: len(result.Entries),
		warnings:  len(result.Errors),
	}, nil
}

func (c *DumpCollector) readerForProduct(
	ctx context.Context,
	product resources.Product,
) (browser.RecordReader, func(), error) {
	if c.reader == nil {
		return nil, nil, browser.ErrMissingReader
	}
	provider, ok := c.reader.(resourceSessionProvider)
	if !ok {
		return c.reader, func() {}, nil
	}
	session, err := provider.Session(ctx, product)
	if err != nil {
		if errors.Is(err, zscaler.ErrUnsupportedResource) {
			return c.reader, func() {}, nil
		}
		return nil, nil, err
	}
	if session == nil {
		return nil, nil, errors.New("reader session provider returned nil session")
	}
	return session, session.Close, nil
}

func (c *DumpCollector) collectShow(
	ctx context.Context,
	reader browser.RecordReader,
	spec resources.ResourceSpec,
	continueOnError bool,
	stream *machine.EventStream,
	result *dump.Result,
) (int, error) {
	record, err := reader.Show(ctx, spec.Product, spec.Name)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, finishDumpStreamFailure(
				stream, ctxErr, machine.OperationShow, string(spec.Product), spec.Name, machine.ErrorKindLiveAccessFailed,
			)
		}
		if continueOnError {
			resourceErr := dump.NewResourceError(spec.Product, spec.Name, "show", "show_failed")
			result.Errors = append(result.Errors, resourceErr)
			return 0, emitDumpWarning(stream, resourceErr)
		}
		resultErr := fmt.Errorf("dump %s/%s show failed: %w", spec.Product, spec.Name, err)
		return 0, finishDumpStreamFailure(
			stream, resultErr, machine.OperationShow, string(spec.Product), spec.Name, machine.ErrorKindLiveAccessFailed,
		)
	}
	projected, report, err := resources.ProjectRecordAndVerify(spec, c.redaction, record)
	if err != nil {
		operation, kind := dumpProjectionErrorKind(err)
		if continueOnError {
			resourceErr := dump.NewResourceError(spec.Product, spec.Name, operation, kind)
			result.Errors = append(result.Errors, resourceErr)
			return 0, emitDumpWarning(stream, resourceErr)
		}
		resultErr := fmt.Errorf("dump %s/%s %s failed: %w", spec.Product, spec.Name, operation, err)
		return 0, finishDumpStreamFailure(
			stream, resultErr, machine.Operation(operation), string(spec.Product), spec.Name, machine.ErrorKindInternal,
		)
	}
	if err := stream.Emit(machine.Event{
		Kind:     machine.EventRecord,
		Product:  string(spec.Product),
		Resource: spec.Name,
		Record:   &projected,
	}); err != nil {
		return 0, err
	}
	result.Entries = append(result.Entries, dump.ResourceDump{
		Spec:    spec,
		Record:  &projected,
		Reports: []resources.ProjectionReport{report},
	})
	return 1, nil
}

func (c *DumpCollector) collectList(
	ctx context.Context,
	reader browser.RecordReader,
	spec resources.ResourceSpec,
	continueOnError bool,
	stream *machine.EventStream,
	result *dump.Result,
) (int, error) {
	records, err := reader.List(ctx, spec.Product, spec.Name)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, finishDumpStreamFailure(
				stream, ctxErr, machine.OperationList, string(spec.Product), spec.Name, machine.ErrorKindLiveAccessFailed,
			)
		}
		if continueOnError {
			resourceErr := dump.NewResourceError(spec.Product, spec.Name, "list", "list_failed")
			result.Errors = append(result.Errors, resourceErr)
			return 0, emitDumpWarning(stream, resourceErr)
		}
		resultErr := fmt.Errorf("dump %s/%s list failed: %w", spec.Product, spec.Name, err)
		return 0, finishDumpStreamFailure(
			stream, resultErr, machine.OperationList, string(spec.Product), spec.Name, machine.ErrorKindLiveAccessFailed,
		)
	}
	projected, reports, err := resources.ProjectRecordsAndVerify(spec, c.redaction, records)
	if err != nil {
		operation, kind := dumpProjectionErrorKind(err)
		if continueOnError {
			resourceErr := dump.NewResourceError(spec.Product, spec.Name, operation, kind)
			result.Errors = append(result.Errors, resourceErr)
			return 0, emitDumpWarning(stream, resourceErr)
		}
		resultErr := fmt.Errorf("dump %s/%s %s failed: %w", spec.Product, spec.Name, operation, err)
		return 0, finishDumpStreamFailure(
			stream, resultErr, machine.Operation(operation), string(spec.Product), spec.Name, machine.ErrorKindInternal,
		)
	}
	projectedRecords := projected.Records()
	for i := range projectedRecords {
		record := projectedRecords[i]
		if err := stream.Emit(machine.Event{
			Kind:     machine.EventRecord,
			Product:  string(spec.Product),
			Resource: spec.Name,
			Record:   &record,
		}); err != nil {
			return 0, err
		}
	}
	result.Entries = append(result.Entries, dump.ResourceDump{
		Spec:    spec,
		Records: projected,
		Reports: reports,
	})
	return len(projectedRecords), nil
}

func emitDumpWarning(stream *machine.EventStream, resourceErr dump.ResourceError) error {
	machineErr := machine.MachineError{
		Kind:      resourceErr.Kind,
		Message:   "dump resource failed",
		Operation: machine.Operation(resourceErr.Operation),
		Product:   resourceErr.Product,
		Resource:  resourceErr.Name,
	}
	return stream.Emit(machine.Event{
		Kind:     machine.EventWarning,
		Product:  resourceErr.Product,
		Resource: resourceErr.Name,
		Err:      &machineErr,
	})
}

func finishDumpStreamFailure(
	stream *machine.EventStream,
	resultErr error,
	operation machine.Operation,
	product string,
	resource string,
	fallbackKind string,
) error {
	machineErr := dumpMachineError(resultErr, operation, product, resource, fallbackKind)
	producerErr := &dumpStreamError{cause: resultErr, machineErr: machineErr}
	if err := stream.Fail(machineErr); err != nil {
		return errors.Join(producerErr, err)
	}
	return producerErr
}

// dumpStreamError retains the lower collector's trusted in-process cause while
// making the exact sanitized terminal classification available to the typed
// engine boundary. Its Error and Unwrap behavior preserve CollectStream's
// existing error identity and text for compatibility callers.
type dumpStreamError struct {
	cause      error
	machineErr machine.MachineError
}

func (e *dumpStreamError) Error() string { return e.cause.Error() }
func (e *dumpStreamError) Unwrap() error { return e.cause }

func dumpMachineError(
	err error,
	operation machine.Operation,
	product string,
	resource string,
	fallbackKind string,
) machine.MachineError {
	machineErr := machine.MachineError{
		Kind:      fallbackKind,
		Message:   "dump collection failed",
		Operation: operation,
		Product:   product,
		Resource:  resource,
	}
	switch {
	case errors.Is(err, context.Canceled):
		machineErr.Kind = machine.ErrorKindCanceled
		machineErr.Message = "request canceled"
	case errors.Is(err, context.DeadlineExceeded):
		machineErr.Kind = machine.ErrorKindDeadlineExceeded
		machineErr.Message = "request deadline exceeded"
	case errors.Is(err, config.ErrInvalidConfig):
		machineErr.Kind = machine.ErrorKindInvalidConfig
		machineErr.Message = "invalid configuration"
	case errors.Is(err, zscaler.ErrInvalidProxyConfig):
		machineErr.Kind = machine.ErrorKindInvalidProxyConfig
		machineErr.Message = "invalid proxy configuration"
	case errors.Is(err, zscaler.ErrMissingCredentials):
		machineErr.Kind = machine.ErrorKindMissingCredentials
		message, safe := sanitizedDumpMissingCredentials(err)
		machineErr.Message = message
		var missingErr *zscaler.MissingCredentialsError
		if errors.As(safe, &missingErr) {
			machineErr.Missing = append([]string(nil), missingErr.Missing...)
		}
	case errors.Is(err, zscaler.ErrUnsupportedResource):
		machineErr.Kind = machine.ErrorKindUnsupportedResource
		machineErr.Message = "unsupported dump resource"
	case fallbackKind == machine.ErrorKindLiveAccessFailed:
		machineErr.Message = "resource read failed"
	case fallbackKind == machine.ErrorKindUnsupportedOperation:
		machineErr.Message = "unsupported dump resource operation"
	}
	return machineErr
}

func dumpReadOperation(spec resources.ResourceSpec) machine.Operation {
	if spec.SupportsReadOperation("show") {
		return machine.OperationShow
	}
	return machine.OperationList
}

func dumpProjectionErrorKind(err error) (operation string, kind string) {
	if errors.Is(err, resources.ErrUnexpectedField) {
		return "validate", "subset_failed"
	}
	return "project", "projection_failed"
}

func newDumpCollectorFromReader(
	reader browser.RecordReader,
	catalog resources.ResourceCatalog,
	mode redact.Mode,
) *DumpCollector {
	return &DumpCollector{
		reader:    reader,
		catalog:   copyCatalog(catalog),
		redaction: redact.EffectiveMode(mode),
	}
}

func copyResourceSpecs(specs []resources.ResourceSpec) []resources.ResourceSpec {
	out := make([]resources.ResourceSpec, len(specs))
	for i, spec := range specs {
		out[i] = copyResourceSpec(spec)
	}
	return out
}
