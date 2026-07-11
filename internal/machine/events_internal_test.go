package machine

import (
	"runtime"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestEventEmitterRejectsEventsAfterTerminal(t *testing.T) {
	var got []EventKind
	emitter := eventEmitter{sink: func(event Event) error {
		got = append(got, event.Kind)
		return nil
	}}
	if failure := emitter.emit(Event{Kind: EventStarted}); failure != nil {
		t.Fatalf("eventEmitter.emit(started) failure = %#v, want nil", failure)
	}
	if failure := emitter.emit(Event{Kind: EventCompleted}); failure != nil {
		t.Fatalf("eventEmitter.emit(completed) failure = %#v, want nil", failure)
	}
	failure := emitter.emit(Event{Kind: EventProgress})
	if failure == nil {
		t.Fatal("eventEmitter.emit(progress after completed) failure = nil, want internal failure")
	}
	if failure.Kind != ErrorKindInternal {
		t.Errorf("eventEmitter.emit(progress after completed) kind = %q, want %q", failure.Kind, ErrorKindInternal)
	}
	if len(got) != 2 || got[0] != EventStarted || got[1] != EventCompleted {
		t.Errorf("eventEmitter sink calls = %#v, want [started completed]", got)
	}
}

func TestCopyEventForSinkDoesNotCloneRecordPayload(t *testing.T) {
	ports := make([]int, 4_096)
	for i := range ports {
		ports[i] = i
	}
	records := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{
		"id":    "record-1",
		"ports": ports,
	}})
	record := records.Records()[0]
	event := Event{Kind: EventRecord, Record: &record}
	var retained *resources.ProjectedRecord

	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			copied := copyEventForSink(event)
			retained = copied.Record
		}
	})
	runtime.KeepAlive(retained)
	t.Logf("copyEventForSink(record): %d allocs/op, %d bytes/op", result.AllocsPerOp(), result.AllocedBytesPerOp())
	if got := result.AllocsPerOp(); got > 1 {
		t.Errorf("copyEventForSink(record) allocations = %d/op, want <= 1", got)
	}
	if got := result.AllocedBytesPerOp(); got > 64 {
		t.Errorf("copyEventForSink(record) allocated bytes = %d/op, want <= 64 (record wrapper only)", got)
	}
}

func TestCopyEventForSinkCopiesTypedResultWrapper(t *testing.T) {
	t.Parallel()

	projected := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{"id": "record-1"}})
	result := NewResourceReadResult(projected)
	copied := copyEventForSink(Event{Kind: EventCompleted, resourceResult: &result})
	if copied.resourceResult == nil {
		t.Fatal("copyEventForSink(typed result) resource result = nil, want copy")
	}
	if copied.resourceResult == &result {
		t.Fatal("copyEventForSink(typed result) retained wrapper pointer, want distinct copy")
	}
	copied.resourceResult.records = resources.ProjectedRecords{}
	if got := result.Records().Len(); got != 1 {
		t.Errorf("copyEventForSink(typed result) source records after copy mutation = %d, want 1", got)
	}
}
