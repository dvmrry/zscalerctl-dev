package enginehost

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
)

var errResponseTooLarge = errors.New("engine stdio response item is too large")

type plannedItem struct {
	kind       enginewire.ItemKind
	value      enginewire.ItemValue
	fragmented bool
	payload    []byte
	chunks     int
	digest     string
}

type successPlan struct {
	items      []plannedItem
	result     enginewire.CompletionResult
	frameCount uint64
}

func preflightSuccess(ctx context.Context, request wireRequest, data operationData) (*successPlan, error) {
	return preflightSuccessWithAggregateLimit(ctx, request, data, enginewire.AggregateItemBytes)
}

func preflightSuccessWithAggregateLimit(
	ctx context.Context,
	request wireRequest,
	data operationData,
	aggregateItemBytes int,
) (*successPlan, error) {
	if aggregateItemBytes <= 0 || aggregateItemBytes > enginewire.AggregateItemBytes {
		return nil, fmt.Errorf("invalid aggregate item limit")
	}
	if err := validateOperationData(request, data); err != nil {
		return nil, err
	}
	maximum := enginewire.SafeInteger(enginewire.MaxSafeInteger)
	completed := enginewire.Completed[enginewire.CompletionResult]{
		Type: "completed", ID: maximum, Sequence: maximum, Result: data.result,
	}
	if _, err := enginewire.MarshalServerFrame(completed); err != nil {
		if errors.Is(err, enginewire.ErrFrameTooLarge) {
			return nil, errResponseTooLarge
		}
		return nil, fmt.Errorf("preflight completion: %w", err)
	}

	plan := &successPlan{
		items:      make([]plannedItem, len(data.items)),
		result:     data.result,
		frameCount: 1, // completed
	}
	for i, item := range data.items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		payload, err := enginewire.MarshalItemPayload(item.kind, item.value)
		if err != nil {
			if errors.Is(err, enginewire.ErrFrameTooLarge) {
				return nil, errResponseTooLarge
			}
			return nil, fmt.Errorf("preflight item payload: %w", err)
		}
		if len(payload) > aggregateItemBytes {
			return nil, errResponseTooLarge
		}
		inline := enginewire.Item[enginewire.ItemValue]{
			Type: "item", ID: maximum, Sequence: maximum, Kind: item.kind, Item: item.value,
		}
		if _, err := enginewire.MarshalServerFrame(inline); err == nil {
			plan.items[i] = plannedItem{kind: item.kind, value: item.value}
			if err := addPlanFrames(plan, 1); err != nil {
				return nil, err
			}
			continue
		} else if !errors.Is(err, enginewire.ErrFrameTooLarge) {
			return nil, fmt.Errorf("preflight inline item: %w", err)
		}

		chunks := (len(payload) + enginewire.FragmentChunkBytes - 1) / enginewire.FragmentChunkBytes
		if chunks < 1 || chunks > 128 {
			return nil, errResponseTooLarge
		}
		digestBytes := sha256.Sum256(payload)
		planned := plannedItem{
			kind:       item.kind,
			value:      item.value,
			fragmented: true,
			payload:    append([]byte(nil), payload...),
			chunks:     chunks,
			digest:     hex.EncodeToString(digestBytes[:]),
		}
		if err := preflightFragments(ctx, maximum, planned); err != nil {
			return nil, err
		}
		plan.items[i] = planned
		if err := addPlanFrames(plan, uint64(chunks+2)); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

func preflightFragments(
	ctx context.Context,
	maximum enginewire.SafeInteger,
	item plannedItem,
) error {
	begin := enginewire.ItemBegin{
		Type: "item_begin", ID: maximum, Sequence: maximum, ItemID: maximum,
		Kind: item.kind, Encoding: "json", Bytes: enginewire.SafeInteger(len(item.payload)),
	}
	if _, err := enginewire.MarshalServerFrame(begin); err != nil {
		return fmt.Errorf("preflight item begin: %w", err)
	}
	for index := 0; index < item.chunks; index++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		start := index * enginewire.FragmentChunkBytes
		end := min(start+enginewire.FragmentChunkBytes, len(item.payload))
		chunk := enginewire.ItemChunk{
			Type: "item_chunk", ID: maximum, Sequence: maximum, ItemID: maximum,
			Index: enginewire.SafeInteger(index),
			Data:  base64.StdEncoding.EncodeToString(item.payload[start:end]),
		}
		if _, err := enginewire.MarshalServerFrame(chunk); err != nil {
			return fmt.Errorf("preflight item chunk: %w", err)
		}
	}
	end := enginewire.ItemEnd{
		Type: "item_end", ID: maximum, Sequence: maximum, ItemID: maximum,
		Chunks: enginewire.SafeInteger(item.chunks), SHA256: item.digest,
	}
	if _, err := enginewire.MarshalServerFrame(end); err != nil {
		return fmt.Errorf("preflight item end: %w", err)
	}
	return nil
}

func addPlanFrames(plan *successPlan, count uint64) error {
	if count > enginewire.MaxSafeInteger || plan.frameCount > enginewire.MaxSafeInteger-count {
		return errResponseTooLarge
	}
	plan.frameCount += count
	return nil
}

func validateOperationData(request wireRequest, data operationData) error {
	if isNilCompletionResult(data.result) {
		return fmt.Errorf("operation returned a nil completion result")
	}
	validItem := func(kind enginewire.ItemKind) bool { return false }
	validResult := false
	switch {
	case request.capability == enginewire.CapabilityEngineManifest && request.operation == enginewire.OperationManifest:
		_, validResult = data.result.(enginewire.EngineManifestResult)
	case request.capability == enginewire.CapabilityCatalogSchema && request.operation == enginewire.OperationList:
		validItem = func(kind enginewire.ItemKind) bool { return kind == enginewire.ItemCatalogResource }
		_, validResult = data.result.(enginewire.CatalogSummary)
	case request.capability == enginewire.CapabilityStatusInspect && request.operation == enginewire.OperationDoctor:
		_, validResult = data.result.(enginewire.DoctorStatusResult)
	case request.capability == enginewire.CapabilityStatusInspect && request.operation == enginewire.OperationAuthStatus:
		_, validResult = data.result.(enginewire.AuthStatusResult)
	case request.capability == enginewire.CapabilityStatusInspect && request.operation == enginewire.OperationConfigStatus:
		_, validResult = data.result.(enginewire.ConfigStatusResult)
	case request.capability == enginewire.CapabilityZIAURLLookup && request.operation == enginewire.OperationLookup:
		validItem = func(kind enginewire.ItemKind) bool { return kind == enginewire.ItemURLClassification }
		_, validResult = data.result.(enginewire.URLLookupSummary)
	case request.capability == enginewire.CapabilityResourcesRead &&
		(request.operation == enginewire.OperationList || request.operation == enginewire.OperationGet || request.operation == enginewire.OperationShow):
		validItem = func(kind enginewire.ItemKind) bool { return kind == enginewire.ItemProjectedRecord }
		_, validResult = data.result.(enginewire.ResourceReadSummary)
	case request.capability == enginewire.CapabilityDumpWrite && request.operation == enginewire.OperationDump:
		_, validResult = data.result.(enginewire.DumpSummary)
	case request.capability == enginewire.CapabilityDiffCompare && request.operation == enginewire.OperationDiff:
		validItem = func(kind enginewire.ItemKind) bool {
			switch kind {
			case enginewire.ItemDiffResource, enginewire.ItemDiffAdded, enginewire.ItemDiffRemoved, enginewire.ItemDiffFieldChange:
				return true
			default:
				return false
			}
		}
		summary, ok := data.result.(enginewire.DiffSummary)
		validResult = ok
		if ok {
			if err := validateDiffItems(data.items, summary); err != nil {
				return err
			}
		}
	}
	if !validResult {
		return fmt.Errorf("operation result does not match %s/%s", request.capability, request.operation)
	}
	for _, item := range data.items {
		if !validItem(item.kind) {
			return fmt.Errorf("item kind %q does not match %s/%s", item.kind, request.capability, request.operation)
		}
	}
	return nil
}

func validateDiffItems(items []semanticItem, summary enginewire.DiffSummary) error {
	var resourcesCompared, resourcesWithDrift, recordsAdded, recordsRemoved uint64
	changedRecords := make(map[string]struct{})
	changedFields := make(map[string]struct{})
	for index := 0; index < len(items); {
		if items[index].kind != enginewire.ItemDiffResource {
			return fmt.Errorf("diff item %d is not a resource header", index)
		}
		header, ok := items[index].value.(enginewire.DiffResource)
		if !ok {
			return fmt.Errorf("diff resource item %d has payload %T", index, items[index].value)
		}
		index++
		resourcesCompared++
		if header.Added > 0 || header.Removed > 0 || header.ChangedFields > 0 {
			resourcesWithDrift++
		}
		for added := uint64(0); added < uint64(header.Added); added++ {
			if index >= len(items) || items[index].kind != enginewire.ItemDiffAdded {
				return fmt.Errorf("diff resource %s/%s added count does not match item order", header.Product, header.Resource)
			}
			value, ok := items[index].value.(enginewire.DiffRecordRef)
			if !ok || !diffRecordMatchesHeader(value, header) {
				return fmt.Errorf("diff added item does not match resource identity")
			}
			index++
			recordsAdded++
		}
		for removed := uint64(0); removed < uint64(header.Removed); removed++ {
			if index >= len(items) || items[index].kind != enginewire.ItemDiffRemoved {
				return fmt.Errorf("diff resource %s/%s removed count does not match item order", header.Product, header.Resource)
			}
			value, ok := items[index].value.(enginewire.DiffRecordRef)
			if !ok || !diffRecordMatchesHeader(value, header) {
				return fmt.Errorf("diff removed item does not match resource identity")
			}
			index++
			recordsRemoved++
		}
		for changed := uint64(0); changed < uint64(header.ChangedFields); changed++ {
			if index >= len(items) || items[index].kind != enginewire.ItemDiffFieldChange {
				return fmt.Errorf("diff resource %s/%s changed-field count does not match item order", header.Product, header.Resource)
			}
			value, ok := items[index].value.(enginewire.DiffFieldChange)
			if !ok || !diffFieldMatchesHeader(value, header) {
				return fmt.Errorf("diff field item does not match resource identity")
			}
			recordIdentity := string(header.Product) + "\x00" + header.Resource + "\x00" + value.Key
			fieldIdentity := recordIdentity + "\x00" + value.Field
			if _, duplicate := changedFields[fieldIdentity]; duplicate {
				return fmt.Errorf("diff field item is duplicated")
			}
			changedFields[fieldIdentity] = struct{}{}
			changedRecords[recordIdentity] = struct{}{}
			index++
		}
	}
	if resourcesCompared != uint64(summary.Summary.ResourcesCompared) ||
		resourcesWithDrift != uint64(summary.Summary.ResourcesWithDrift) ||
		recordsAdded != uint64(summary.Summary.RecordsAdded) ||
		recordsRemoved != uint64(summary.Summary.RecordsRemoved) ||
		uint64(len(changedRecords)) != uint64(summary.Summary.RecordsChanged) ||
		uint64(len(items)) != uint64(summary.StreamItemsEmitted) {
		return fmt.Errorf("diff items do not reconcile with completion summary")
	}
	return nil
}

func diffRecordMatchesHeader(value enginewire.DiffRecordRef, header enginewire.DiffResource) bool {
	if value.Product != header.Product || value.Resource != header.Resource {
		return false
	}
	switch header.Identity.Mode {
	case enginewire.DiffIdentityGetKey:
		return value.Key != nil && value.Hash == nil
	case enginewire.DiffIdentitySingleton:
		return value.Key != nil && *value.Key == "singleton" && value.Hash == nil
	case enginewire.DiffIdentityContentHash:
		return value.Key == nil && value.Hash != nil
	default:
		return false
	}
}

func diffFieldMatchesHeader(value enginewire.DiffFieldChange, header enginewire.DiffResource) bool {
	if value.Product != header.Product || value.Resource != header.Resource {
		return false
	}
	switch header.Identity.Mode {
	case enginewire.DiffIdentityGetKey:
		return value.Key != ""
	case enginewire.DiffIdentitySingleton:
		return value.Key == "singleton"
	default:
		return false
	}
}

func isNilCompletionResult(result enginewire.CompletionResult) bool {
	if result == nil {
		return true
	}
	value := reflect.ValueOf(result)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
