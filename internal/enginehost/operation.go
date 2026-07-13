// Package enginehost runs the candidate local stdio protocol over the trusted
// in-process operation engine. It owns process/session lifecycle, but never
// owns credentials, SDK values, rendering, or tenant mutation.
package enginehost

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/enginewire/adapter"
	"github.com/dvmrry/zscalerctl/internal/machine"
)

var (
	errInvalidEngineEvents = errors.New("invalid engine event lifecycle")
	errSessionClosing      = errors.New("engine stdio session is closing")
)

// Engine is the complete trusted operation surface consumed by the stdio
// host. Implementations execute synchronously; the host owns the one worker
// goroutine that calls them.
type Engine interface {
	EngineManifest() machine.EngineManifest
	DiscoverCatalog(context.Context, machine.CatalogRequest) (machine.CatalogResult, error)
	InspectStatus(context.Context, machine.StatusRequest) (machine.StatusResult, error)
	LookupURL(context.Context, machine.URLLookupRequest) (machine.URLLookupResult, error)
	Read(context.Context, machine.ResourceReadRequest) (machine.ResourceReadResult, error)
	Dump(context.Context, machine.DumpRequest, machine.EventSink) (machine.DumpResult, error)
	Diff(context.Context, machine.DiffRequest, machine.EventSink) (machine.DiffResult, error)
}

type wireRequest struct {
	frame       enginewire.ClientFrame
	id          enginewire.SafeInteger
	capability  enginewire.Capability
	operation   enginewire.Operation
	unavailable enginewire.OperationFailureKind
}

func requestFromFrame(frame enginewire.ClientFrame) (wireRequest, bool) {
	switch typed := frame.(type) {
	case enginewire.ManifestRequest:
		return wireRequest{frame: typed, id: typed.ID, capability: typed.Capability, operation: typed.Operation}, true
	case enginewire.CatalogRequest:
		return wireRequest{frame: typed, id: typed.ID, capability: typed.Capability, operation: typed.Operation}, true
	case enginewire.DoctorRequest:
		return wireRequest{frame: typed, id: typed.ID, capability: typed.Capability, operation: typed.Operation}, true
	case enginewire.AuthStatusRequest:
		return wireRequest{frame: typed, id: typed.ID, capability: typed.Capability, operation: typed.Operation}, true
	case enginewire.ConfigStatusRequest:
		return wireRequest{frame: typed, id: typed.ID, capability: typed.Capability, operation: typed.Operation}, true
	case enginewire.URLLookupRequest:
		return wireRequest{frame: typed, id: typed.ID, capability: typed.Capability, operation: typed.Operation}, true
	case enginewire.ResourceListRequest:
		return wireRequest{frame: typed, id: typed.ID, capability: typed.Capability, operation: typed.Operation}, true
	case enginewire.ResourceGetRequest:
		return wireRequest{frame: typed, id: typed.ID, capability: typed.Capability, operation: typed.Operation}, true
	case enginewire.ResourceShowRequest:
		return wireRequest{frame: typed, id: typed.ID, capability: typed.Capability, operation: typed.Operation}, true
	case enginewire.DumpRequest:
		return wireRequest{frame: typed, id: typed.ID, capability: typed.Capability, operation: typed.Operation}, true
	case enginewire.DiffRequest:
		return wireRequest{frame: typed, id: typed.ID, capability: typed.Capability, operation: typed.Operation}, true
	default:
		return wireRequest{}, false
	}
}

type semanticItem struct {
	kind  enginewire.ItemKind
	value enginewire.ItemValue
}

type operationData struct {
	items  []semanticItem
	result enginewire.CompletionResult
}

type operationMessage struct {
	provisional provisionalFrame
	outcome     *operationOutcome
}

type operationOutcome struct {
	plan       *successPlan
	conversion adapter.ErrorConversion
}

func runOperation(
	opCtx context.Context,
	eventCtx context.Context,
	sessionCtx context.Context,
	engine Engine,
	request wireRequest,
	manifest enginewire.EngineManifest,
	messages chan<- operationMessage,
) {
	defer func() {
		if recover() != nil {
			sendOperationOutcome(sessionCtx, messages, operationOutcome{conversion: adapter.ErrorConversion{
				Failure: enginewire.NonCredentialFailure{Kind: enginewire.FailureInternal},
			}})
		}
	}()
	if request.unavailable != "" {
		outcome := operationOutcome{conversion: adapter.ErrorConversion{
			Failure: enginewire.NonCredentialFailure{Kind: request.unavailable},
		}}
		sendOperationOutcome(sessionCtx, messages, outcome)
		return
	}
	emit := func(frame provisionalFrame) error {
		select {
		case messages <- operationMessage{provisional: frame}:
			return nil
		case <-eventCtx.Done():
			return errSessionClosing
		}
	}

	data, err := executeOperation(opCtx, engine, request, manifest, emit)
	var outcome operationOutcome
	if err == nil {
		outcome.plan, err = preflightSuccess(opCtx, request, data)
	}
	if err != nil {
		if errors.Is(err, errResponseTooLarge) {
			outcome.conversion = adapter.ErrorConversion{Failure: enginewire.NonCredentialFailure{
				Kind: enginewire.FailureResponseTooLarge,
			}}
		} else {
			outcome.conversion = adapter.ToOperationError(err)
		}
	}

	sendOperationOutcome(sessionCtx, messages, outcome)
}

func sendOperationOutcome(
	sessionCtx context.Context,
	messages chan<- operationMessage,
	outcome operationOutcome,
) {
	select {
	case messages <- operationMessage{outcome: &outcome}:
	case <-sessionCtx.Done():
	}
}

func executeOperation(
	ctx context.Context,
	engine Engine,
	request wireRequest,
	manifest enginewire.EngineManifest,
	emit func(provisionalFrame) error,
) (operationData, error) {
	if err := ctx.Err(); err != nil {
		return operationData{}, err
	}
	switch frame := request.frame.(type) {
	case enginewire.ManifestRequest:
		if err := ctx.Err(); err != nil {
			return operationData{}, err
		}
		return operationData{result: enginewire.EngineManifestResult{Kind: "engine_manifest", Manifest: manifest}}, nil

	case enginewire.CatalogRequest:
		result, err := engine.DiscoverCatalog(ctx, adapter.ToCatalogRequest(frame))
		if err != nil {
			return operationData{}, err
		}
		resources, err := adapter.ToCatalogResources(result)
		if err != nil {
			return operationData{}, err
		}
		items := make([]semanticItem, len(resources))
		for i, resource := range resources {
			items[i] = semanticItem{kind: enginewire.ItemCatalogResource, value: resource}
		}
		count, err := safeWireCount(len(items))
		if err != nil {
			return operationData{}, err
		}
		return operationData{items: items, result: enginewire.CatalogSummary{
			Kind: "catalog_summary", Resources: count, StreamItemsEmitted: count,
		}}, nil

	case enginewire.DoctorRequest:
		return executeStatus(ctx, engine, frame.ID, frame.Operation)
	case enginewire.AuthStatusRequest:
		return executeStatus(ctx, engine, frame.ID, frame.Operation)
	case enginewire.ConfigStatusRequest:
		return executeStatus(ctx, engine, frame.ID, frame.Operation)

	case enginewire.URLLookupRequest:
		result, err := engine.LookupURL(ctx, adapter.ToURLLookupRequest(frame))
		if err != nil {
			return operationData{}, err
		}
		classifications, err := adapter.ToURLClassifications(result)
		if err != nil {
			return operationData{}, err
		}
		items := make([]semanticItem, len(classifications))
		for i, classification := range classifications {
			items[i] = semanticItem{kind: enginewire.ItemURLClassification, value: classification}
		}
		count, err := safeWireCount(len(items))
		if err != nil {
			return operationData{}, err
		}
		return operationData{items: items, result: enginewire.URLLookupSummary{
			Kind: "url_lookup_summary", Classifications: count, StreamItemsEmitted: count,
		}}, nil

	case enginewire.ResourceListRequest:
		converted := adapter.ToResourceListRequest(frame)
		converted.Input.Resource = strings.TrimSpace(converted.Input.Resource)
		return executeRead(ctx, engine, converted, string(frame.Input.Product), converted.Input.Resource)
	case enginewire.ResourceGetRequest:
		converted := adapter.ToResourceGetRequest(frame)
		converted.Input.Resource = strings.TrimSpace(converted.Input.Resource)
		converted.Input.RecordID = strings.TrimSpace(converted.Input.RecordID)
		return executeRead(ctx, engine, converted, string(frame.Input.Product), converted.Input.Resource)
	case enginewire.ResourceShowRequest:
		converted := adapter.ToResourceShowRequest(frame)
		converted.Input.Resource = strings.TrimSpace(converted.Input.Resource)
		return executeRead(ctx, engine, converted, string(frame.Input.Product), converted.Input.Resource)

	case enginewire.DumpRequest:
		bridge := newEventBridge(machine.OperationDump, emit)
		result, err := engine.Dump(ctx, adapter.ToDumpRequest(frame), bridge.Accept)
		if finishErr := bridge.Finish(err); finishErr != nil {
			return operationData{}, finishErr
		}
		if err != nil {
			return operationData{}, err
		}
		summary, err := adapter.ToDumpSummary(result)
		if err != nil {
			return operationData{}, err
		}
		if !equalDumpFailures(summary.Failures, bridge.Warnings()) {
			return operationData{}, fmt.Errorf("%w: dump warnings do not match completed summary", errInvalidEngineEvents)
		}
		completion := bridge.Completion()
		records, err := safeWireCount(completion.Records)
		if err != nil {
			return operationData{}, err
		}
		resourcesWritten, err := safeWireCount(completion.Resources)
		if err != nil {
			return operationData{}, err
		}
		warnings, err := safeWireCount(completion.Warnings)
		if err != nil {
			return operationData{}, err
		}
		if summary.RecordsWritten != records ||
			summary.ResourcesWritten != resourcesWritten ||
			summary.WarningCount != warnings {
			return operationData{}, fmt.Errorf("%w: dump result does not match completed counters", errInvalidEngineEvents)
		}
		return operationData{result: summary}, nil

	case enginewire.DiffRequest:
		bridge := newEventBridge(machine.OperationDiff, emit)
		result, err := engine.Diff(ctx, adapter.ToDiffRequest(frame), bridge.Accept)
		if finishErr := bridge.Finish(err); finishErr != nil {
			return operationData{}, finishErr
		}
		if err != nil {
			return operationData{}, err
		}
		converted, err := adapter.ToDiffResult(result)
		if err != nil {
			return operationData{}, err
		}
		completion := bridge.Completion()
		resourcesCompared, err := safeWireCount(completion.Resources)
		if err != nil {
			return operationData{}, err
		}
		if converted.Summary.Summary.ResourcesCompared > resourcesCompared {
			return operationData{}, fmt.Errorf("%w: diff report exceeds selected resource count", errInvalidEngineEvents)
		}
		items := make([]semanticItem, len(converted.Items))
		for i, item := range converted.Items {
			items[i] = semanticItem{kind: item.Kind, value: item.Value}
		}
		return operationData{items: items, result: converted.Summary}, nil
	default:
		return operationData{}, fmt.Errorf("unsupported decoded request frame %T", request.frame)
	}
}

func equalDumpFailures(left, right []enginewire.DumpFailure) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func executeStatus(
	ctx context.Context,
	engine Engine,
	id enginewire.SafeInteger,
	operation enginewire.Operation,
) (operationData, error) {
	result, err := engine.InspectStatus(ctx, adapter.ToStatusRequest(id, operation))
	if err != nil {
		return operationData{}, err
	}
	converted, err := adapter.ToStatusResult(result)
	if err != nil {
		return operationData{}, err
	}
	return operationData{result: converted}, nil
}

func executeRead(
	ctx context.Context,
	engine Engine,
	request machine.ResourceReadRequest,
	product string,
	resource string,
) (operationData, error) {
	result, err := engine.Read(ctx, request)
	if err != nil {
		return operationData{}, err
	}
	records, err := adapter.ToProjectedRecords(product, resource, result)
	if err != nil {
		return operationData{}, err
	}
	items := make([]semanticItem, len(records))
	for i, record := range records {
		items[i] = semanticItem{kind: enginewire.ItemProjectedRecord, value: record}
	}
	count, err := safeWireCount(len(items))
	if err != nil {
		return operationData{}, err
	}
	return operationData{items: items, result: enginewire.ResourceReadSummary{
		Kind: "resource_read_summary", Records: count, StreamItemsEmitted: count,
	}}, nil
}

func safeWireCount(value int) (enginewire.SafeInteger, error) {
	if value < 0 {
		return 0, errResponseTooLarge
	}
	converted := uint64(value) // #nosec G115 -- nonnegative int is losslessly representable by uint64.
	if converted > enginewire.MaxSafeInteger {
		return 0, errResponseTooLarge
	}
	return enginewire.SafeInteger(converted), nil
}

func isNilEngine(engine Engine) bool {
	if engine == nil {
		return true
	}
	value := reflect.ValueOf(engine)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
