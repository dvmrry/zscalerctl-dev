package enginehost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
)

func TestSafeWireCountRejectsOutOfRange(t *testing.T) {
	t.Parallel()

	valid := []int{0, 1}
	invalid := []int{-1}
	if strconv.IntSize >= 64 {
		maximum := uint64(enginewire.MaxSafeInteger)
		valid = append(valid, int(maximum))
		invalid = append(invalid, int(maximum+1))
	}
	for _, value := range valid {
		got, err := safeWireCount(value)
		if err != nil || got.Uint64() != uint64(value) {
			t.Errorf("safeWireCount(%d) = %d, %v; want %d, nil", value, got, err, value)
		}
	}
	for _, value := range invalid {
		if got, err := safeWireCount(value); !errors.Is(err, errResponseTooLarge) || got != 0 {
			t.Errorf("safeWireCount(%d) = %d, %v; want 0, errResponseTooLarge", value, got, err)
		}
	}
}

func TestPreflightFragmentsAndReconstructsLargeItem(t *testing.T) {
	t.Parallel()

	record, err := enginewire.NewWireRecord(map[string]any{
		"blob": strings.Repeat("x", enginewire.V1FrameBytes),
	})
	if err != nil {
		t.Fatalf("NewWireRecord() error = %v", err)
	}
	value := enginewire.ProjectedRecord{
		Product: enginewire.ProductZIA, Resource: "locations", Record: record,
	}
	request := wireRequest{
		id: 1, capability: enginewire.CapabilityResourcesRead, operation: enginewire.OperationList,
	}
	plan, err := preflightSuccess(context.Background(), request, operationData{
		items: []semanticItem{{kind: enginewire.ItemProjectedRecord, value: value}},
		result: enginewire.ResourceReadSummary{
			Kind: "resource_read_summary", Records: 1, StreamItemsEmitted: 1,
		},
	})
	if err != nil {
		t.Fatalf("preflightSuccess() error = %v", err)
	}
	if len(plan.items) != 1 || !plan.items[0].fragmented || plan.items[0].chunks < 2 {
		t.Fatalf("fragment plan = %#v", plan.items)
	}

	cursor := newSuccessCursor(plan)
	var payload bytes.Buffer
	var begin enginewire.ItemBegin
	var end enginewire.ItemEnd
	sequence := enginewire.SafeInteger(2)
	frames := uint64(0)
	for {
		frame, terminal, err := cursor.Next(1, sequence)
		if err != nil {
			t.Fatalf("successCursor.Next() error = %v", err)
		}
		if _, err := enginewire.MarshalServerFrame(frame); err != nil {
			t.Fatalf("MarshalServerFrame(%T) error = %v", frame, err)
		}
		frames++
		switch typed := frame.(type) {
		case enginewire.ItemBegin:
			begin = typed
		case enginewire.ItemChunk:
			decoded, err := base64.StdEncoding.DecodeString(typed.Data)
			if err != nil {
				t.Fatalf("decode chunk: %v", err)
			}
			if typed.Index != enginewire.SafeInteger(frames-2) {
				t.Fatalf("chunk index = %d, frame count = %d", typed.Index, frames)
			}
			payload.Write(decoded)
		case enginewire.ItemEnd:
			end = typed
		case enginewire.Completed[enginewire.CompletionResult]:
			if !terminal {
				t.Fatal("completed frame was not terminal")
			}
		default:
			t.Fatalf("unexpected success frame %T", frame)
		}
		if terminal {
			break
		}
		sequence++
	}
	if frames != plan.frameCount || begin.Bytes != enginewire.SafeInteger(payload.Len()) || end.Chunks != enginewire.SafeInteger(plan.items[0].chunks) {
		t.Fatalf("fragment reconciliation frames=%d/%d begin=%#v end=%#v payload=%d", frames, plan.frameCount, begin, end, payload.Len())
	}
	digest := sha256.Sum256(payload.Bytes())
	if got := hex.EncodeToString(digest[:]); got != end.SHA256 {
		t.Fatalf("fragment digest = %s, want %s", got, end.SHA256)
	}
	decoded, err := enginewire.DecodeItemPayload(enginewire.ItemProjectedRecord, payload.Bytes())
	if err != nil {
		t.Fatalf("DecodeItemPayload() error = %v", err)
	}
	if _, ok := decoded.(enginewire.ProjectedRecord); !ok {
		t.Fatalf("DecodeItemPayload() type = %T", decoded)
	}
}

func TestPreflightLaterOversizedItemReturnsNoSuccessPlan(t *testing.T) {
	t.Parallel()

	makeRecord := func(value string) enginewire.ProjectedRecord {
		record, err := enginewire.NewWireRecord(map[string]any{"value": value})
		if err != nil {
			t.Fatalf("NewWireRecord() error = %v", err)
		}
		return enginewire.ProjectedRecord{Product: enginewire.ProductZIA, Resource: "locations", Record: record}
	}
	request := wireRequest{id: 1, capability: enginewire.CapabilityResourcesRead, operation: enginewire.OperationList}
	plan, err := preflightSuccessWithAggregateLimit(context.Background(), request, operationData{
		items: []semanticItem{
			{kind: enginewire.ItemProjectedRecord, value: makeRecord("small")},
			{kind: enginewire.ItemProjectedRecord, value: makeRecord(strings.Repeat("x", 256))},
		},
		result: enginewire.ResourceReadSummary{Kind: "resource_read_summary", Records: 2, StreamItemsEmitted: 2},
	}, 128)
	if !errors.Is(err, errResponseTooLarge) || plan != nil {
		t.Fatalf("preflightSuccessWithAggregateLimit() = %#v, %v; want nil, response-too-large", plan, err)
	}
}

func TestDiffItemStateValidationRejectsCrossResourceAndOrderMismatches(t *testing.T) {
	t.Parallel()

	key := "1"
	header := enginewire.DiffResource{
		Product: enginewire.ProductZIA, Resource: "locations",
		Identity: enginewire.DiffIdentity{Mode: enginewire.DiffIdentityGetKey, Field: &key},
		Added:    1,
	}
	record, err := enginewire.NewWireRecord(map[string]any{"id": "1"})
	if err != nil {
		t.Fatalf("NewWireRecord() error = %v", err)
	}
	ref := enginewire.DiffRecordRef{
		Product: enginewire.ProductZIA, Resource: "other", Key: &key, Record: record,
	}
	summary := enginewire.DiffSummary{
		StreamItemsEmitted: 2,
		Summary: enginewire.DiffCounts{
			ResourcesCompared: 1, ResourcesWithDrift: 1, RecordsAdded: 1,
		},
	}
	items := []semanticItem{
		{kind: enginewire.ItemDiffResource, value: header},
		{kind: enginewire.ItemDiffAdded, value: ref},
	}
	if err := validateDiffItems(items, summary); err == nil {
		t.Fatal("validateDiffItems(cross-resource) error = nil")
	}
	items[1] = semanticItem{kind: enginewire.ItemDiffRemoved, value: ref}
	if err := validateDiffItems(items, summary); err == nil {
		t.Fatal("validateDiffItems(wrong-order) error = nil")
	}
}

func TestDiffItemStateValidationRejectsDuplicateFieldChange(t *testing.T) {
	t.Parallel()

	field := "id"
	header := enginewire.DiffResource{
		Product: enginewire.ProductZIA, Resource: "locations",
		Identity:      enginewire.DiffIdentity{Mode: enginewire.DiffIdentityGetKey, Field: &field},
		ChangedFields: 2,
	}
	oldValue, err := enginewire.NewWireValue("old")
	if err != nil {
		t.Fatalf("NewWireValue(old) error = %v", err)
	}
	newValue, err := enginewire.NewWireValue("new")
	if err != nil {
		t.Fatalf("NewWireValue(new) error = %v", err)
	}
	change := enginewire.DiffFieldChange{
		Product: enginewire.ProductZIA, Resource: "locations", Key: "1", Field: "name",
		Old: oldValue, New: newValue,
	}
	items := []semanticItem{
		{kind: enginewire.ItemDiffResource, value: header},
		{kind: enginewire.ItemDiffFieldChange, value: change},
		{kind: enginewire.ItemDiffFieldChange, value: change},
	}
	summary := enginewire.DiffSummary{
		StreamItemsEmitted: 3,
		Summary: enginewire.DiffCounts{
			ResourcesCompared: 1, ResourcesWithDrift: 1, RecordsChanged: 1,
		},
	}
	if err := validateDiffItems(items, summary); err == nil {
		t.Fatal("validateDiffItems(duplicate field change) error = nil")
	}
}
