package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// savedDumpRuntime adapts an admitted immutable dump collection to the same
// typed machine read path used by live product reads. The collection is already
// projected and redacted; machine.Executor only applies further narrowing.
type savedDumpRuntime struct {
	executor  machine.Executor
	redaction redact.Mode
}

func (r *savedDumpRuntime) Read(
	ctx context.Context,
	req machine.ResourceReadRequest,
) (machine.ResourceReadResult, error) {
	return r.executor.Read(ctx, req)
}

func (r *savedDumpRuntime) Redaction() redact.Mode {
	return r.redaction
}

// savedCollectionBrowser preserves the saved-source classification at the
// machine boundary. A complete dump may intentionally omit resources outside
// its selected scope; querying one of those resources is a not-found result,
// rather than a live-access failure or an empty collection.
type savedCollectionBrowser struct {
	collection *diff.Collection
}

func (b savedCollectionBrowser) ListProjected(
	ctx context.Context,
	product string,
	resource string,
) (resources.ProjectedRecords, error) {
	records, err := b.collection.ListProjected(ctx, product, resource)
	return records, savedCollectionQueryError(err, machine.OperationList, product, resource)
}

func (b savedCollectionBrowser) ShowProjected(
	ctx context.Context,
	product string,
	resource string,
) (resources.ProjectedRecords, error) {
	records, err := b.collection.ShowProjected(ctx, product, resource)
	return records, savedCollectionQueryError(err, machine.OperationShow, product, resource)
}

func (b savedCollectionBrowser) GetProjectedByID(
	ctx context.Context,
	product string,
	resource string,
	id string,
) (resources.ProjectedRecords, error) {
	records, err := b.collection.GetProjectedByID(ctx, product, resource, id)
	return records, savedCollectionQueryError(err, machine.OperationGet, product, resource)
}

func savedCollectionQueryError(
	err error,
	operation machine.Operation,
	product string,
	resource string,
) error {
	if !errors.Is(err, diff.ErrCollectionScopeMismatch) {
		return err
	}
	return &machine.MachineError{
		Kind:      machine.ErrorKindNotFound,
		Message:   "resource is not present in saved collection",
		Operation: operation,
		Product:   product,
		Resource:  resource,
	}
}

// fromDumpUsageError preserves a safe usage classification and a stable input
// sentinel without exposing raw artifact contents through Error().
type fromDumpUsageError struct {
	message string
	cause   error
}

func (e fromDumpUsageError) Error() string { return e.message }

func (e fromDumpUsageError) Unwrap() error {
	if e.cause == nil {
		return ErrUsage
	}
	return errors.Join(ErrUsage, e.cause)
}

func newFromDumpUsageError(err error) error {
	if err == nil {
		return nil
	}
	message, _ := redact.New(redact.ModeStandard).ScanRenderedString(err.Error())
	return fromDumpUsageError{message: message, cause: err}
}

// savedDumpContextError converts collection admission cancellation into the
// same safe machine boundary used by live resource reads. The original context
// sentinel is retained through MachineError.Unwrap so callers can still use
// errors.Is without exposing artifact paths or loader details.
func savedDumpContextError(err error, productName string, args []string) error {
	if err == nil {
		return nil
	}
	var kind string
	var message string
	var sentinel error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		kind = machine.ErrorKindDeadlineExceeded
		message = "request deadline exceeded"
		sentinel = context.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		kind = machine.ErrorKindCanceled
		message = "request canceled"
		sentinel = context.Canceled
	default:
		return nil
	}
	resource := ""
	operation := machine.Operation("")
	if len(args) > 0 {
		resource = strings.TrimSpace(args[0])
	}
	if len(args) > 1 {
		operation = machine.Operation(strings.TrimSpace(args[1]))
	}
	return machine.ErrorWithCause(&machine.MachineError{
		Kind:      kind,
		Message:   message,
		Operation: operation,
		Product:   strings.TrimSpace(productName),
		Resource:  resource,
	}, sentinel)
}

func (a *App) runProductFromDump(
	ctx context.Context,
	opts globalOptions,
	productName string,
	args []string,
) error {
	collection, err := diff.LoadCollection(ctx, opts.fromDump, a.resourceCatalog())
	if err != nil {
		if contextErr := savedDumpContextError(err, productName, args); contextErr != nil {
			return contextErr
		}
		return newFromDumpUsageError(err)
	}
	mode := redact.EffectiveMode(collection.Redaction())
	if opts.redactionSet && redact.EffectiveMode(opts.redaction) != mode {
		return fromDumpUsageError{
			message: fmt.Sprintf(
				"--redaction %s does not match dump redaction mode %s",
				opts.redaction,
				mode,
			),
			cause: diff.ErrRedactionMismatch,
		}
	}
	rt := &savedDumpRuntime{
		executor: machine.Executor{
			Browser:   savedCollectionBrowser{collection: collection},
			Catalog:   a.resourceCatalog(),
			Redaction: mode,
		},
		redaction: mode,
	}
	cfg := config.Config{Defaults: config.Defaults{Redaction: mode}}
	applyOptions(&cfg, opts)
	return a.runProductWithRuntime(ctx, cfg, opts, productName, args, rt)
}
