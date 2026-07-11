package machine

import "testing"

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
