package runtime

import (
	"context"
	"errors"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

type preparedDumpRequest struct {
	outputDir       string
	specs           []resources.ResourceSpec
	continueOnError bool
	force           bool
}

type dumpSelectionKey struct {
	product  resources.Product
	resource string
}

// Dump validates a complete artifact request before loading config, then
// collects and writes the dump while delivering one synchronous event stream.
// A completed event means every artifact file has been finalized.
func (e *Engine) Dump(
	ctx context.Context,
	req machine.DumpRequest,
	sink machine.EventSink,
) (machine.DumpResult, error) {
	ctx = nonNilContext(ctx)
	if e == nil {
		stream, err := machine.StartEventStream(sink, machine.OperationDump, "", "", 0)
		if err != nil {
			return machine.DumpResult{}, err
		}
		return machine.DumpResult{}, failTypedDumpStream(stream, dumpInternalError())
	}
	prepared, prepareErr := prepareDumpRequest(ctx, req, e.opts.Catalog)
	stream, err := machine.StartEventStream(
		sink,
		machine.OperationDump,
		"",
		"",
		len(prepared.specs),
	)
	if err != nil {
		return machine.DumpResult{}, err
	}
	if prepareErr != nil {
		return machine.DumpResult{}, failTypedDumpStream(stream, prepareErr)
	}
	if err := ctx.Err(); err != nil {
		return machine.DumpResult{}, failTypedDumpStream(stream, dumpRuntimeBoundaryError(err))
	}
	collector, err := NewDumpCollector(ctx, e.options())
	if err != nil {
		return machine.DumpResult{}, failTypedDumpStream(stream, dumpRuntimeBoundaryError(err))
	}
	return collector.dumpPrepared(ctx, prepared, stream)
}

// Dump validates, collects, and writes one typed request using an already
// trusted collector. It is the injected-reader seam used by adapters and tests.
func (c *DumpCollector) Dump(
	ctx context.Context,
	req machine.DumpRequest,
	sink machine.EventSink,
) (machine.DumpResult, error) {
	ctx = nonNilContext(ctx)
	if c == nil {
		stream, err := machine.StartEventStream(sink, machine.OperationDump, "", "", 0)
		if err != nil {
			return machine.DumpResult{}, err
		}
		return machine.DumpResult{}, failTypedDumpStream(stream, dumpInternalError())
	}
	prepared, prepareErr := prepareDumpRequest(ctx, req, c.catalog)
	stream, err := machine.StartEventStream(
		sink,
		machine.OperationDump,
		"",
		"",
		len(prepared.specs),
	)
	if err != nil {
		return machine.DumpResult{}, err
	}
	if prepareErr != nil {
		return machine.DumpResult{}, failTypedDumpStream(stream, prepareErr)
	}
	return c.dumpPrepared(ctx, prepared, stream)
}

func (c *DumpCollector) dumpPrepared(
	ctx context.Context,
	prepared preparedDumpRequest,
	stream *machine.EventStream,
) (machine.DumpResult, error) {
	collected, counts, err := c.collectIntoStream(ctx, prepared.specs, DumpCollectOptions{
		ContinueOnError: prepared.continueOnError,
	}, stream)
	if err != nil {
		return machine.DumpResult{}, dumpCollectionBoundaryError(err)
	}
	if err := dump.PublishContext(
		ctx,
		prepared.outputDir,
		c.redaction,
		collected,
		prepared.force,
	); err != nil {
		safeErr := dumpOutputBoundaryError(err)
		return machine.DumpResult{}, failTypedDumpStream(stream, safeErr)
	}

	resourceErrors := make([]machine.DumpResourceError, len(collected.Errors))
	for i, resourceErr := range collected.Errors {
		resourceErrors[i] = machine.DumpResourceError{
			Product:   resourceErr.Product,
			Resource:  resourceErr.Name,
			Operation: machine.Operation(resourceErr.Operation),
			Kind:      resourceErr.Kind,
		}
	}
	result := machine.NewDumpResult(
		counts.records,
		counts.resources,
		string(c.redaction),
		resourceErrors,
	)
	if err := stream.Complete(machine.Event{
		Records:   counts.records,
		Resources: counts.resources,
		Warnings:  counts.warnings,
	}); err != nil {
		return result, err
	}
	return result, nil
}

func prepareDumpRequest(
	ctx context.Context,
	req machine.DumpRequest,
	catalog resources.ResourceCatalog,
) (preparedDumpRequest, error) {
	prepared := preparedDumpRequest{
		outputDir:       req.OutputDir,
		continueOnError: req.ContinueOnError,
		force:           req.Force,
	}
	if err := ctx.Err(); err != nil {
		return prepared, dumpRuntimeBoundaryError(err)
	}
	if strings.TrimSpace(req.OutputDir) == "" {
		return prepared, dumpUsageError("dump output directory is required", "output_dir")
	}
	if err := resources.AssertReadOnly(catalog...); err != nil {
		return prepared, dumpCatalogError(resources.ErrMutatingOperation)
	}

	knownProducts := make(map[resources.Product]bool)
	dumpSpecs := make(map[dumpSelectionKey]resources.ResourceSpec)
	seenCatalog := make(map[dumpSelectionKey]bool)
	for _, spec := range catalog {
		if err := ctx.Err(); err != nil {
			return prepared, dumpRuntimeBoundaryError(err)
		}
		if err := spec.Validate(); err != nil {
			return prepared, dumpCatalogError(resources.ErrInvalidResourceSpec)
		}
		key := dumpSelectionKey{product: spec.Product, resource: spec.Name}
		if seenCatalog[key] {
			return prepared, dumpCatalogError(nil)
		}
		seenCatalog[key] = true
		knownProducts[spec.Product] = true
		if spec.SupportsReadOperation("list") || spec.SupportsReadOperation("show") {
			dumpSpecs[key] = copyResourceSpec(spec)
		}
	}

	selectedProducts := make(map[resources.Product]bool)
	if len(req.Products) == 0 {
		for product := range knownProducts {
			selectedProducts[product] = true
		}
	} else {
		for _, rawProduct := range req.Products {
			if err := ctx.Err(); err != nil {
				return prepared, dumpRuntimeBoundaryError(err)
			}
			if rawProduct == "" || strings.TrimSpace(rawProduct) != rawProduct {
				return prepared, dumpUsageError("dump selection is invalid")
			}
			product := resources.Product(rawProduct)
			if !knownProducts[product] || selectedProducts[product] {
				return prepared, dumpUsageError("dump selection is invalid")
			}
			selectedProducts[product] = true
		}
	}

	selectedResources := make(map[dumpSelectionKey]bool)
	if len(req.Resources) > 0 {
		for _, selector := range req.Resources {
			if err := ctx.Err(); err != nil {
				return prepared, dumpRuntimeBoundaryError(err)
			}
			if selector.Product == "" || selector.Resource == "" ||
				strings.TrimSpace(selector.Product) != selector.Product ||
				strings.TrimSpace(selector.Resource) != selector.Resource {
				return prepared, dumpUsageError("dump selection is invalid")
			}
			key := dumpSelectionKey{
				product:  resources.Product(selector.Product),
				resource: selector.Resource,
			}
			if !selectedProducts[key.product] || selectedResources[key] {
				return prepared, dumpUsageError("dump selection is invalid")
			}
			if _, ok := dumpSpecs[key]; !ok {
				return prepared, dumpUsageError("dump selection is invalid")
			}
			selectedResources[key] = true
		}
	}

	for _, spec := range catalog {
		if err := ctx.Err(); err != nil {
			return prepared, dumpRuntimeBoundaryError(err)
		}
		key := dumpSelectionKey{product: spec.Product, resource: spec.Name}
		if !selectedProducts[spec.Product] {
			continue
		}
		if _, ok := dumpSpecs[key]; !ok {
			continue
		}
		if len(selectedResources) > 0 && !selectedResources[key] {
			continue
		}
		prepared.specs = append(prepared.specs, copyResourceSpec(spec))
	}
	if len(prepared.specs) == 0 {
		return prepared, dumpUsageError("dump selection is empty")
	}
	if err := ctx.Err(); err != nil {
		return prepared, dumpRuntimeBoundaryError(err)
	}
	prepared.specs = copyResourceSpecs(prepared.specs)
	return prepared, nil
}

func dumpUsageError(message string, missing ...string) error {
	return &machine.MachineError{
		Kind:      machine.ErrorKindUsage,
		Message:   message,
		Missing:   append([]string(nil), missing...),
		Operation: machine.OperationDump,
	}
}

func dumpCatalogError(sentinel error) error {
	return newBoundaryError(&machine.MachineError{
		Kind:      machine.ErrorKindInternal,
		Message:   "dump catalog is invalid",
		Operation: machine.OperationDump,
	}, sentinel)
}

func dumpInternalError() error {
	return &machine.MachineError{
		Kind:      machine.ErrorKindInternal,
		Message:   "dump runtime is not configured",
		Operation: machine.OperationDump,
	}
}

func failTypedDumpStream(stream *machine.EventStream, safeErr error) error {
	machineErr := machine.MachineError{
		Kind:      machine.ErrorKindInternal,
		Message:   "dump operation failed",
		Operation: machine.OperationDump,
	}
	var typedErr *machine.MachineError
	if errors.As(safeErr, &typedErr) {
		machineErr = *typedErr
		machineErr.Missing = append([]string(nil), typedErr.Missing...)
	}
	if err := stream.Fail(machineErr); err != nil {
		return errors.Join(safeErr, err)
	}
	return safeErr
}

func dumpRuntimeBoundaryError(err error) error {
	if err == nil {
		return nil
	}
	machineErr := &machine.MachineError{}
	var sentinel error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		machineErr.Kind = machine.ErrorKindDeadlineExceeded
		machineErr.Message = "request deadline exceeded"
		machineErr.Operation = machine.OperationDump
		sentinel = context.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		machineErr.Kind = machine.ErrorKindCanceled
		machineErr.Message = "request canceled"
		machineErr.Operation = machine.OperationDump
		sentinel = context.Canceled
	case errors.Is(err, config.ErrInvalidConfig):
		machineErr.Kind = machine.ErrorKindInvalidConfig
		machineErr.Message = "invalid configuration"
		sentinel = config.ErrInvalidConfig
	case errors.Is(err, zscaler.ErrInvalidProxyConfig):
		machineErr.Kind = machine.ErrorKindInvalidProxyConfig
		machineErr.Message = "invalid proxy configuration"
		sentinel = zscaler.ErrInvalidProxyConfig
	case errors.Is(err, zscaler.ErrMissingCredentials):
		machineErr.Kind = machine.ErrorKindMissingCredentials
		machineErr.Message, sentinel = sanitizedRuntimeMissingCredentials(err)
		var missingErr *zscaler.MissingCredentialsError
		if errors.As(sentinel, &missingErr) {
			machineErr.Missing = append([]string(nil), missingErr.Missing...)
		}
	case errors.Is(err, zscaler.ErrUnsupportedResource):
		machineErr.Kind = machine.ErrorKindUnsupportedResource
		machineErr.Message = "unsupported dump resource"
		machineErr.Operation = machine.OperationDump
		sentinel = zscaler.ErrUnsupportedResource
	case errors.Is(err, zscaler.ErrLiveAccessFailed):
		machineErr.Kind = machine.ErrorKindLiveAccessFailed
		machineErr.Message = "dump resource read failed"
		machineErr.Operation = machine.OperationDump
		sentinel = zscaler.ErrLiveAccessFailed
	case errors.Is(err, browser.ErrMissingReader):
		machineErr.Kind = machine.ErrorKindInternal
		machineErr.Message = "dump reader is not configured"
		machineErr.Operation = machine.OperationDump
		sentinel = browser.ErrMissingReader
	default:
		machineErr.Kind = machine.ErrorKindInternal
		machineErr.Message = "dump runtime failed"
		machineErr.Operation = machine.OperationDump
	}
	return newBoundaryError(machineErr, sentinel)
}

func dumpCollectionBoundaryError(err error) error {
	var producerErr *dumpStreamError
	if !errors.As(err, &producerErr) {
		var machineErr *machine.MachineError
		if errors.As(err, &machineErr) {
			copyErr := *machineErr
			copyErr.Missing = append([]string(nil), machineErr.Missing...)
			return &copyErr
		}
		return newBoundaryError(&machine.MachineError{
			Kind:      machine.ErrorKindLiveAccessFailed,
			Message:   "dump resource read failed",
			Operation: machine.OperationDump,
		}, zscaler.ErrLiveAccessFailed)
	}
	machineErr := producerErr.machineErr
	machineErr.Missing = append([]string(nil), producerErr.machineErr.Missing...)
	return newBoundaryError(&machineErr, dumpSafeCollectionSentinel(producerErr.cause, machineErr.Kind))
}

func dumpSafeCollectionSentinel(err error, kind string) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, config.ErrInvalidConfig):
		return config.ErrInvalidConfig
	case errors.Is(err, zscaler.ErrInvalidProxyConfig):
		return zscaler.ErrInvalidProxyConfig
	case errors.Is(err, zscaler.ErrMissingCredentials):
		_, safe := sanitizedRuntimeMissingCredentials(err)
		return safe
	case errors.Is(err, zscaler.ErrUnsupportedResource):
		return zscaler.ErrUnsupportedResource
	case errors.Is(err, browser.ErrMissingReader):
		return browser.ErrMissingReader
	case kind == machine.ErrorKindLiveAccessFailed:
		return zscaler.ErrLiveAccessFailed
	default:
		return nil
	}
}

func dumpOutputBoundaryError(err error) error {
	if err == nil {
		return nil
	}
	machineErr := &machine.MachineError{
		Kind:      machine.ErrorKindInternal,
		Message:   "dump output failed",
		Operation: machine.OperationDump,
	}
	var sentinel error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		machineErr.Kind = machine.ErrorKindDeadlineExceeded
		machineErr.Message = "request deadline exceeded"
		sentinel = context.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		machineErr.Kind = machine.ErrorKindCanceled
		machineErr.Message = "request canceled"
		sentinel = context.Canceled
	case errors.Is(err, dump.ErrAtomicReplaceUnsupported):
		machineErr.Kind = machine.ErrorKindUnsupportedOperation
		machineErr.Message = "atomic dump directory replacement is unsupported"
		sentinel = dump.ErrAtomicReplaceUnsupported
	case errors.Is(err, dump.ErrUnsafePath):
		sentinel = dump.ErrUnsafePath
	case errors.Is(err, dump.ErrUnsafeOverwrite):
		sentinel = dump.ErrUnsafeOverwrite
	}
	return &dumpOutputError{
		safe:            newBoundaryError(machineErr, sentinel),
		adapterMessage:  sanitizeEngineString(redact.New(redact.ModeStandard), err.Error()),
		adapterSentinel: sentinel,
	}
}

// dumpOutputError keeps the typed engine error surface static while retaining
// one already-redacted compatibility message for the legacy Cobra adapter.
// Error, Unwrap, errors.Is, and errors.As expose only the safe typed boundary.
type dumpOutputError struct {
	safe            error
	adapterMessage  string
	adapterSentinel error
}

func (e *dumpOutputError) Error() string { return e.safe.Error() }
func (e *dumpOutputError) Unwrap() error { return e.safe }

type legacyDumpAdapterError struct {
	message  string
	sentinel error
}

func (e *legacyDumpAdapterError) Error() string { return e.message }
func (e *legacyDumpAdapterError) Unwrap() error { return e.sentinel }

// LegacyDumpAdapterError returns the redacted pre-engine error text and safe
// sentinel retained solely for Cobra compatibility. New engine consumers must
// use the static MachineError exposed by the original error instead.
func LegacyDumpAdapterError(err error) (error, bool) {
	var outputErr *dumpOutputError
	if !errors.As(err, &outputErr) {
		return nil, false
	}
	if errors.Is(outputErr.safe, context.Canceled) || errors.Is(outputErr.safe, context.DeadlineExceeded) {
		return nil, false
	}
	var machineErr *machine.MachineError
	if errors.As(outputErr.safe, &machineErr) && machineErr.Kind == machine.ErrorKindUnsupportedOperation {
		return nil, false
	}
	return &legacyDumpAdapterError{
		message:  outputErr.adapterMessage,
		sentinel: outputErr.adapterSentinel,
	}, true
}

func sanitizedRuntimeMissingCredentials(err error) (string, error) {
	var missingErr *zscaler.MissingCredentialsError
	if !errors.As(err, &missingErr) {
		return zscaler.ErrMissingCredentials.Error(), zscaler.ErrMissingCredentials
	}
	missing := make([]string, 0, len(missingErr.Missing))
	seen := map[string]bool{}
	for _, name := range missingErr.Missing {
		if !isKnownRuntimeCredentialName(name) || seen[name] {
			continue
		}
		seen[name] = true
		missing = append(missing, name)
	}
	if len(missing) == 0 {
		return zscaler.ErrMissingCredentials.Error(), zscaler.ErrMissingCredentials
	}
	safeErr := &zscaler.MissingCredentialsError{Missing: missing}
	return safeErr.Error(), safeErr
}

func isKnownRuntimeCredentialName(name string) bool {
	if isKnownCredentialName(name) {
		return true
	}
	return name == config.EnvZPACustomerID || name == config.EnvZPAMicrotenantID
}
