package runtime

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

type preparedDiffRequest struct {
	oldDir            string
	newDir            string
	specs             []resources.ResourceSpec
	products          map[resources.Product]bool
	resources         map[diff.ResourceKey]bool
	ignoreOperational bool
	allowPartial      bool
}

// Diff validates a complete local comparison request before filesystem access,
// then compares two admitted dump artifacts through one synchronous event
// stream. It never loads config, resolves providers, constructs an SDK reader,
// executes a process, or contacts Zscaler.
func (e *Engine) Diff(
	ctx context.Context,
	req machine.DiffRequest,
	sink machine.EventSink,
) (machine.DiffResult, error) {
	ctx = nonNilContext(ctx)
	if e == nil {
		stream, err := machine.StartEventStream(sink, machine.OperationDiff, "", "", 0)
		if err != nil {
			return machine.DiffResult{}, err
		}
		return machine.DiffResult{}, failTypedDiffStream(stream, diffInternalError())
	}

	prepared, prepareErr := prepareDiffRequest(ctx, req, e.opts.Catalog)
	stream, err := machine.StartEventStream(
		sink,
		machine.OperationDiff,
		"",
		"",
		len(prepared.specs),
	)
	if err != nil {
		return machine.DiffResult{}, err
	}
	if prepareErr != nil {
		return machine.DiffResult{}, failTypedDiffStream(stream, prepareErr)
	}
	if err := ctx.Err(); err != nil {
		return machine.DiffResult{}, failTypedDiffStream(stream, diffBoundaryError(err))
	}

	var deliveryErr error
	report, err := diff.CompareContext(ctx, prepared.oldDir, prepared.newDir, diff.Options{
		Catalog:           copyCatalog(e.opts.Catalog),
		Products:          copyProductSelection(prepared.products),
		Resources:         copyDiffResourceSelection(prepared.resources),
		IgnoreOperational: prepared.ignoreOperational,
		AllowPartial:      prepared.allowPartial,
	}, func(progress diff.Progress) error {
		emitErr := stream.Emit(machine.Event{
			Kind:     machine.EventProgress,
			Product:  string(progress.Product),
			Resource: progress.Resource,
			Done:     progress.Done,
			Total:    progress.Total,
		})
		if emitErr != nil {
			deliveryErr = emitErr
		}
		return emitErr
	})
	if deliveryErr != nil {
		return machine.DiffResult{}, deliveryErr
	}
	if err != nil {
		safeErr := diffBoundaryError(err)
		return machine.DiffResult{}, failTypedDiffStream(stream, safeErr)
	}

	result := machine.NewDiffResult(report)
	if err := stream.Complete(machine.Event{
		Resources: len(prepared.specs),
	}); err != nil {
		return result, err
	}
	return result, nil
}

func prepareDiffRequest(
	ctx context.Context,
	req machine.DiffRequest,
	catalog resources.ResourceCatalog,
) (preparedDiffRequest, error) {
	prepared := preparedDiffRequest{
		oldDir:            req.OldDir,
		newDir:            req.NewDir,
		ignoreOperational: req.IgnoreOperational,
		allowPartial:      req.AllowPartial,
	}
	if err := ctx.Err(); err != nil {
		return prepared, diffBoundaryError(err)
	}
	if strings.TrimSpace(req.OldDir) == "" || strings.TrimSpace(req.NewDir) == "" {
		return prepared, diffUsageError("two dump directories are required")
	}
	if err := resources.AssertReadOnly(catalog...); err != nil {
		return prepared, diffCatalogError(resources.ErrMutatingOperation)
	}

	knownProducts := make(map[resources.Product]bool)
	diffSpecs := make(map[diff.ResourceKey]resources.ResourceSpec)
	seenCatalog := make(map[diff.ResourceKey]bool)
	for _, spec := range catalog {
		if err := ctx.Err(); err != nil {
			return prepared, diffBoundaryError(err)
		}
		if err := spec.Validate(); err != nil {
			return prepared, diffCatalogError(resources.ErrInvalidResourceSpec)
		}
		key := diff.ResourceKey{Product: spec.Product, Name: spec.Name}
		if seenCatalog[key] {
			return prepared, diffCatalogError(nil)
		}
		seenCatalog[key] = true
		knownProducts[spec.Product] = true
		if spec.SupportsReadOperation("list") || spec.SupportsReadOperation("show") {
			diffSpecs[key] = copyResourceSpec(spec)
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
				return prepared, diffBoundaryError(err)
			}
			if rawProduct == "" || strings.TrimSpace(rawProduct) != rawProduct {
				return prepared, diffUsageError("diff selection is invalid")
			}
			product := resources.Product(rawProduct)
			if !knownProducts[product] || selectedProducts[product] {
				return prepared, diffUsageError("diff selection is invalid")
			}
			selectedProducts[product] = true
		}
	}

	selectedResources := make(map[diff.ResourceKey]bool)
	for _, selector := range req.Resources {
		if err := ctx.Err(); err != nil {
			return prepared, diffBoundaryError(err)
		}
		if selector.Product == "" || selector.Resource == "" ||
			strings.TrimSpace(selector.Product) != selector.Product ||
			strings.TrimSpace(selector.Resource) != selector.Resource {
			return prepared, diffUsageError("diff selection is invalid")
		}
		key := diff.ResourceKey{
			Product: resources.Product(selector.Product),
			Name:    selector.Resource,
		}
		if !selectedProducts[key.Product] || selectedResources[key] {
			return prepared, diffUsageError("diff selection is invalid")
		}
		if _, ok := diffSpecs[key]; !ok {
			return prepared, diffUsageError("diff selection is invalid")
		}
		selectedResources[key] = true
	}

	for key, spec := range diffSpecs {
		if err := ctx.Err(); err != nil {
			return prepared, diffBoundaryError(err)
		}
		if !selectedProducts[key.Product] {
			continue
		}
		if len(selectedResources) > 0 && !selectedResources[key] {
			continue
		}
		prepared.specs = append(prepared.specs, copyResourceSpec(spec))
	}
	sort.Slice(prepared.specs, func(i, j int) bool {
		if prepared.specs[i].Product != prepared.specs[j].Product {
			return prepared.specs[i].Product < prepared.specs[j].Product
		}
		return prepared.specs[i].Name < prepared.specs[j].Name
	})
	if len(prepared.specs) == 0 {
		return prepared, diffUsageError("diff selection is empty")
	}

	prepared.products = make(map[resources.Product]bool, len(selectedProducts))
	for product := range selectedProducts {
		prepared.products[product] = true
	}
	if len(selectedResources) > 0 {
		prepared.resources = make(map[diff.ResourceKey]bool, len(selectedResources))
		for key := range selectedResources {
			prepared.resources[key] = true
		}
	}
	prepared.specs = copyResourceSpecs(prepared.specs)
	if err := ctx.Err(); err != nil {
		return prepared, diffBoundaryError(err)
	}
	return prepared, nil
}

func copyProductSelection(source map[resources.Product]bool) map[resources.Product]bool {
	if source == nil {
		return nil
	}
	out := make(map[resources.Product]bool, len(source))
	for product, selected := range source {
		out[product] = selected
	}
	return out
}

func copyDiffResourceSelection(source map[diff.ResourceKey]bool) map[diff.ResourceKey]bool {
	if source == nil {
		return nil
	}
	out := make(map[diff.ResourceKey]bool, len(source))
	for key, selected := range source {
		out[key] = selected
	}
	return out
}

func diffUsageError(message string) error {
	return &machine.MachineError{
		Kind:      machine.ErrorKindUsage,
		Message:   message,
		Operation: machine.OperationDiff,
	}
}

func diffCatalogError(sentinel error) error {
	return newBoundaryError(&machine.MachineError{
		Kind:      machine.ErrorKindInternal,
		Message:   "diff catalog is invalid",
		Operation: machine.OperationDiff,
	}, sentinel)
}

func diffInternalError() error {
	return &machine.MachineError{
		Kind:      machine.ErrorKindInternal,
		Message:   "diff runtime is not configured",
		Operation: machine.OperationDiff,
	}
}

func failTypedDiffStream(stream *machine.EventStream, safeErr error) error {
	machineErr := machine.MachineError{
		Kind:      machine.ErrorKindInternal,
		Message:   "diff operation failed",
		Operation: machine.OperationDiff,
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

func diffBoundaryError(err error) error {
	if err == nil {
		return nil
	}
	machineErr := &machine.MachineError{
		Kind:      machine.ErrorKindInternal,
		Message:   "diff operation failed",
		Operation: machine.OperationDiff,
	}
	var sentinel error
	var adapterMessage string
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		machineErr.Kind = machine.ErrorKindDeadlineExceeded
		machineErr.Message = "request deadline exceeded"
		sentinel = context.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		machineErr.Kind = machine.ErrorKindCanceled
		machineErr.Message = "request canceled"
		sentinel = context.Canceled
	case errors.Is(err, diff.ErrInvalidDump):
		machineErr.Kind = machine.ErrorKindUsage
		machineErr.Message = "invalid dump"
		sentinel = diff.ErrInvalidDump
		adapterMessage = sanitizeEngineString(redact.New(redact.ModeStandard), err.Error())
	case errors.Is(err, diff.ErrPartialDumpInput):
		machineErr.Kind = machine.ErrorKindUsage
		machineErr.Message = "partial dump input"
		sentinel = diff.ErrPartialDumpInput
		adapterMessage = sanitizeEngineString(redact.New(redact.ModeStandard), err.Error())
	case errors.Is(err, diff.ErrRedactionMismatch):
		machineErr.Kind = machine.ErrorKindUsage
		machineErr.Message = "redaction mode mismatch"
		sentinel = diff.ErrRedactionMismatch
		adapterMessage = sanitizeEngineString(redact.New(redact.ModeStandard), err.Error())
	}
	safe := newBoundaryError(machineErr, sentinel)
	if adapterMessage == "" {
		return safe
	}
	return &diffAdapterBoundaryError{
		safe:            safe,
		adapterMessage:  adapterMessage,
		adapterSentinel: sentinel,
	}
}

type diffAdapterBoundaryError struct {
	safe            error
	adapterMessage  string
	adapterSentinel error
}

func (e *diffAdapterBoundaryError) Error() string { return e.safe.Error() }
func (e *diffAdapterBoundaryError) Unwrap() error { return e.safe }

type legacyDiffAdapterError struct {
	message  string
	sentinel error
}

func (e *legacyDiffAdapterError) Error() string { return e.message }
func (e *legacyDiffAdapterError) Unwrap() error { return e.sentinel }

// LegacyDiffAdapterError returns the redacted pre-engine local-input message
// and safe sentinel retained solely for Cobra compatibility.
func LegacyDiffAdapterError(err error) (error, bool) {
	var adapterErr *diffAdapterBoundaryError
	if !errors.As(err, &adapterErr) {
		return nil, false
	}
	return &legacyDiffAdapterError{
		message:  adapterErr.adapterMessage,
		sentinel: adapterErr.adapterSentinel,
	}, true
}
