package enginehost

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/effectcommit"
	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestHostRejectsSecondServe(t *testing.T) {
	engine := &fakeEngine{manifest: machine.EngineManifestFromCatalog(nil)}
	host, err := New(engine, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	var firstOutput bytes.Buffer
	if err := host.Serve(context.Background(), Streams{
		Input: bytes.NewReader(nil), Output: &firstOutput,
	}); err != nil {
		t.Fatalf("Host.Serve(first) error = %v", err)
	}
	var secondOutput bytes.Buffer
	err = host.Serve(context.Background(), Streams{
		Input: bytes.NewReader(nil), Output: &secondOutput,
	})
	if !errors.Is(err, ErrInternal) {
		t.Errorf("Host.Serve(second) error = %v, want ErrInternal", err)
	}
	if secondOutput.Len() != 0 {
		t.Errorf("Host.Serve(second) output = %q, want empty", secondOutput.Bytes())
	}
}

type fakeEngine struct {
	manifest machine.EngineManifest

	discover func(context.Context, machine.CatalogRequest) (machine.CatalogResult, error)
	status   func(context.Context, machine.StatusRequest) (machine.StatusResult, error)
	lookup   func(context.Context, machine.URLLookupRequest) (machine.URLLookupResult, error)
	read     func(context.Context, machine.ResourceReadRequest) (machine.ResourceReadResult, error)
	dump     func(context.Context, machine.DumpRequest, machine.EventSink) (machine.DumpResult, error)
	diff     func(context.Context, machine.DiffRequest, machine.EventSink) (machine.DiffResult, error)
}

type panicManifestEngine struct {
	*fakeEngine
}

func (panicManifestEngine) EngineManifest() machine.EngineManifest {
	panic("manifest panic containing forbidden-canary")
}

func TestNewContainsEngineManifestPanic(t *testing.T) {
	engine := panicManifestEngine{fakeEngine: &fakeEngine{}}
	host, err := New(engine, "test")
	if host != nil {
		t.Errorf("New() host = %#v, want nil", host)
	}
	if !errors.Is(err, ErrInternal) {
		t.Errorf("New() error = %v, want ErrInternal", err)
	}
}

func (e *fakeEngine) EngineManifest() machine.EngineManifest { return e.manifest }

func (e *fakeEngine) DiscoverCatalog(ctx context.Context, req machine.CatalogRequest) (machine.CatalogResult, error) {
	if e.discover == nil {
		return machine.CatalogResult{}, errors.New("unexpected DiscoverCatalog")
	}
	return e.discover(ctx, req)
}

func (e *fakeEngine) InspectStatus(ctx context.Context, req machine.StatusRequest) (machine.StatusResult, error) {
	if e.status == nil {
		return machine.StatusResult{}, errors.New("unexpected InspectStatus")
	}
	return e.status(ctx, req)
}

func (e *fakeEngine) LookupURL(ctx context.Context, req machine.URLLookupRequest) (machine.URLLookupResult, error) {
	if e.lookup == nil {
		return machine.URLLookupResult{}, errors.New("unexpected LookupURL")
	}
	return e.lookup(ctx, req)
}

func (e *fakeEngine) Read(ctx context.Context, req machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
	if e.read == nil {
		return machine.ResourceReadResult{}, errors.New("unexpected Read")
	}
	return e.read(ctx, req)
}

func (e *fakeEngine) Dump(ctx context.Context, req machine.DumpRequest, sink machine.EventSink) (machine.DumpResult, error) {
	if e.dump == nil {
		return machine.DumpResult{}, errors.New("unexpected Dump")
	}
	return e.dump(ctx, req, sink)
}

func (e *fakeEngine) Diff(ctx context.Context, req machine.DiffRequest, sink machine.EventSink) (machine.DiffResult, error) {
	if e.diff == nil {
		return machine.DiffResult{}, errors.New("unexpected Diff")
	}
	return e.diff(ctx, req, sink)
}

type hostHarness struct {
	input       *io.PipeWriter
	output      *enginewire.FrameReader
	outputClose func() error
	result      <-chan error
	done        <-chan struct{}
	cancel      context.CancelFunc
}

func newHostHarness(t *testing.T, engine Engine) *hostHarness {
	return newHostHarnessWithTimeout(t, engine, time.Second)
}

func newHostHarnessWithTimeout(t *testing.T, engine Engine, joinTimeout time.Duration) *hostHarness {
	t.Helper()
	host, err := New(engine, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	host.joinTimeout = joinTimeout
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	result := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		result <- host.Serve(ctx, Streams{
			Input: inputReader, Output: outputWriter,
			CloseInput: inputReader.Close, CloseOutput: outputWriter.Close,
		})
	}()
	t.Cleanup(func() {
		cancel()
		_ = inputWriter.Close()
		_ = outputReader.Close()
		_ = outputWriter.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("host did not stop during cleanup")
		}
	})
	return &hostHarness{
		input: inputWriter, output: enginewire.NewFrameReader(outputReader, enginewire.V1FrameBytes),
		outputClose: outputReader.Close, result: result, done: done, cancel: cancel,
	}
}

func (h *hostHarness) initialize(t *testing.T) enginewire.Ready {
	t.Helper()
	data, err := h.output.ReadFrameLimit(enginewire.BootstrapFrameBytes)
	if err != nil {
		t.Fatalf("read hello: %v", err)
	}
	frame, err := enginewire.DecodeBootstrapServerFrame(data)
	if err != nil {
		t.Fatalf("decode hello: %v", err)
	}
	if _, ok := frame.(enginewire.Hello); !ok {
		t.Fatalf("bootstrap frame = %T, want Hello", frame)
	}
	if err := enginewire.WriteBootstrapClientFrame(h.input, enginewire.Initialize{
		Type: "initialize", Protocol: enginewire.Protocol, Version: enginewire.V1Version,
	}); err != nil {
		t.Fatalf("write initialize: %v", err)
	}
	return readServerFrame[enginewire.Ready](t, h)
}

func readServerFrame[T enginewire.ServerFrame](t *testing.T, h *hostHarness) T {
	t.Helper()
	data, err := h.output.ReadFrameLimit(enginewire.V1FrameBytes)
	if err != nil {
		var zero T
		t.Fatalf("read server frame: %v", err)
		return zero
	}
	frame, err := enginewire.DecodeServerFrame(data)
	if err != nil {
		var zero T
		t.Fatalf("decode server frame %s: %v", data, err)
		return zero
	}
	typed, ok := frame.(T)
	if !ok {
		var zero T
		t.Fatalf("server frame = %T (%s), want %T", frame, data, zero)
		return zero
	}
	return typed
}

func finishHarness(t *testing.T, h *hostHarness, want error) {
	t.Helper()
	if err := h.input.Close(); err != nil {
		t.Fatalf("close client input: %v", err)
	}
	select {
	case err := <-h.result:
		if !errors.Is(err, want) {
			t.Fatalf("Host.Serve() error = %v, want %v", err, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Host.Serve() did not return")
	}
}

func TestHostNegotiatesAndServesCachedManifest(t *testing.T) {
	engine := &fakeEngine{manifest: machine.EngineManifestFromCatalog(nil)}
	h := newHostHarness(t, engine)
	ready := h.initialize(t)
	if ready.Protocol != enginewire.Protocol || ready.Version != enginewire.V1Version {
		t.Fatalf("ready = %#v", ready)
	}

	request := enginewire.ManifestRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityEngineManifest, Operation: enginewire.OperationManifest,
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("write manifest request: %v", err)
	}
	started := readServerFrame[enginewire.Started](t, h)
	if started.ID != 1 || started.Sequence != 1 {
		t.Fatalf("started = %#v", started)
	}
	completed := readServerFrame[enginewire.Completed[enginewire.EngineManifestResult]](t, h)
	if completed.ID != 1 || completed.Sequence != 2 || !reflect.DeepEqual(completed.Result.Manifest, ready.Engine) {
		t.Fatalf("completed manifest = %#v, ready = %#v", completed, ready.Engine)
	}
	finishHarness(t, h, nil)
}

func TestHostRejectsNonIncreasingRequestIDFatally(t *testing.T) {
	engine := &fakeEngine{manifest: machine.EngineManifestFromCatalog(nil)}
	h := newHostHarness(t, engine)
	h.initialize(t)
	request := enginewire.ManifestRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityEngineManifest, Operation: enginewire.OperationManifest,
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("write request: %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	_ = readServerFrame[enginewire.Completed[enginewire.EngineManifestResult]](t, h)
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("write repeated request: %v", err)
	}
	protocolError := readServerFrame[enginewire.ProtocolError](t, h)
	if protocolError.Error.Kind != enginewire.ProtocolErrorViolation {
		t.Fatalf("protocol error = %#v", protocolError)
	}
	select {
	case err := <-h.result:
		if !errors.Is(err, ErrProtocol) || ExitCode(err) != 2 {
			t.Fatalf("Host.Serve() error = %v, exit = %d", err, ExitCode(err))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Host.Serve() did not return")
	}
}

func TestHostBusyRejectionAndCancellation(t *testing.T) {
	spec := hostListSpec()
	readEntered := make(chan struct{})
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
		read: func(ctx context.Context, req machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
			close(readEntered)
			<-ctx.Done()
			return machine.ResourceReadResult{}, ctx.Err()
		},
	}
	h := newHostHarness(t, engine)
	h.initialize(t)
	first := enginewire.ResourceListRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
		Input: enginewire.ResourceListInput{Product: enginewire.ProductZIA, Resource: "locations", Fields: []string{}, Filters: []enginewire.Filter{}, Search: ""},
	}
	if err := enginewire.WriteClientFrame(h.input, first); err != nil {
		t.Fatalf("write first request: %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	select {
	case <-readEntered:
	case <-time.After(time.Second):
		t.Fatal("fake read did not start")
	}
	second := enginewire.ManifestRequest{
		Type: "request", ID: 2, Capability: enginewire.CapabilityEngineManifest, Operation: enginewire.OperationManifest,
	}
	if err := enginewire.WriteClientFrame(h.input, second); err != nil {
		t.Fatalf("write busy request: %v", err)
	}
	rejected := readServerFrame[enginewire.RequestRejected](t, h)
	if rejected.ID != 2 || rejected.Reason != "busy" {
		t.Fatalf("request rejected = %#v", rejected)
	}
	if err := enginewire.WriteClientFrame(h.input, enginewire.Cancel{Type: "cancel", ID: 1}); err != nil {
		t.Fatalf("write cancel: %v", err)
	}
	canceled := readServerFrame[enginewire.Canceled](t, h)
	if canceled.ID != 1 || canceled.Sequence != 2 {
		t.Fatalf("canceled = %#v", canceled)
	}
	finishHarness(t, h, nil)
}

func TestCancelUnblocksBackpressuredDumpEvent(t *testing.T) {
	spec := hostListSpec()
	thirdAttempt := make(chan struct{})
	thirdResult := make(chan error, 1)
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
		dump: func(_ context.Context, _ machine.DumpRequest, sink machine.EventSink) (machine.DumpResult, error) {
			if err := sink(machine.Event{Kind: machine.EventStarted, Total: 3}); err != nil {
				return machine.DumpResult{}, err
			}
			for current := 1; current <= 3; current++ {
				if current == 3 {
					close(thirdAttempt)
				}
				err := sink(machine.Event{
					Kind: machine.EventProgress, Done: current, Total: 3,
					Product: "zia", Resource: "locations",
				})
				if current == 3 {
					thirdResult <- err
				}
				if err != nil {
					return machine.DumpResult{}, err
				}
			}
			return machine.DumpResult{}, errors.New("unexpected completed dump")
		},
	}
	h := newHostHarness(t, engine)
	h.initialize(t)
	request := enginewire.DumpRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityDumpWrite, Operation: enginewire.OperationDump,
		Input: enginewire.DumpInput{OutputDir: "out", Products: []enginewire.Product{}, Resources: []enginewire.ResourceSelector{}},
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("WriteClientFrame(dump) error = %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	select {
	case <-thirdAttempt:
	case <-time.After(time.Second):
		t.Fatal("third dump progress event was not attempted")
	}
	if err := enginewire.WriteClientFrame(h.input, enginewire.Cancel{Type: "cancel", ID: 1}); err != nil {
		t.Fatalf("WriteClientFrame(cancel) error = %v", err)
	}
	select {
	case err := <-thirdResult:
		if !isSessionClosingError(err) {
			t.Errorf("third progress sink error = %v, want session-closing sentinel", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not unblock the backpressured event sink")
	}
	first := readServerFrame[enginewire.Progress](t, h)
	second := readServerFrame[enginewire.Progress](t, h)
	if first.Current != 1 || second.Current != 2 {
		t.Errorf("progress currents = %d, %d, want 1, 2", first.Current, second.Current)
	}
	canceled := readServerFrame[enginewire.Canceled](t, h)
	if canceled.ID != 1 || canceled.Sequence != 4 {
		t.Errorf("canceled = %#v", canceled)
	}
	finishHarness(t, h, nil)
}

func TestCancelWithUncooperativeEngineHasJoinCeiling(t *testing.T) {
	spec := hostListSpec()
	entered := make(chan struct{})
	release := make(chan struct{})
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
		read: func(context.Context, machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
			close(entered)
			<-release
			return machine.ResourceReadResult{}, context.Canceled
		},
	}
	h := newHostHarnessWithTimeout(t, engine, 30*time.Millisecond)
	h.initialize(t)
	request := enginewire.ResourceListRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
		Input: enginewire.ResourceListInput{Product: enginewire.ProductZIA, Resource: "locations", Fields: []string{}, Filters: []enginewire.Filter{}, Search: ""},
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("WriteClientFrame(list) error = %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("uncooperative engine did not start")
	}
	if err := enginewire.WriteClientFrame(h.input, enginewire.Cancel{Type: "cancel", ID: 1}); err != nil {
		t.Fatalf("WriteClientFrame(cancel) error = %v", err)
	}
	select {
	case err := <-h.result:
		if !errors.Is(err, ErrJoinTimeout) {
			t.Errorf("Host.Serve() error = %v, want ErrJoinTimeout", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("wire cancellation exceeded the join ceiling")
	}
	close(release)
}

func TestGracefulContextCancelWritesCanceledNotProtocolError(t *testing.T) {
	spec := hostListSpec()
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
		read: func(ctx context.Context, _ machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
			<-ctx.Done()
			return machine.ResourceReadResult{}, ctx.Err()
		},
	}
	h := newHostHarness(t, engine)
	h.initialize(t)
	request := enginewire.ResourceListRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
		Input: enginewire.ResourceListInput{Product: enginewire.ProductZIA, Resource: "locations", Fields: []string{}, Filters: []enginewire.Filter{}, Search: ""},
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("WriteClientFrame(list) error = %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	h.cancel()
	canceled := readServerFrame[enginewire.Canceled](t, h)
	if canceled.ID != 1 || canceled.Sequence != 2 {
		t.Errorf("canceled = %#v", canceled)
	}
	select {
	case err := <-h.result:
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrProtocol) {
			t.Errorf("Host.Serve() error = %v, want context.Canceled only", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Host.Serve() did not return after context cancellation")
	}
}

func TestHostFailsUnavailableAdvertisedCapabilityWithoutCallingEngine(t *testing.T) {
	engine := &fakeEngine{manifest: machine.EngineManifestFromCatalog(nil)}
	h := newHostHarness(t, engine)
	h.initialize(t)
	request := enginewire.ResourceListRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
		Input: enginewire.ResourceListInput{Product: enginewire.ProductZIA, Resource: "locations", Fields: []string{}, Filters: []enginewire.Filter{}, Search: ""},
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("write unavailable request: %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	failed := readServerFrame[enginewire.Failed[enginewire.NonCredentialFailure]](t, h)
	if failed.Error.Kind != enginewire.FailureUnsupportedCapability {
		t.Fatalf("failed = %#v", failed)
	}
	finishHarness(t, h, nil)
}

func TestHostContainsEnginePanicAndContinuesSession(t *testing.T) {
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(nil),
		discover: func(context.Context, machine.CatalogRequest) (machine.CatalogResult, error) {
			panic("engine panic containing forbidden-canary")
		},
	}
	h := newHostHarness(t, engine)
	h.initialize(t)
	request := enginewire.CatalogRequest{
		Type: "request", ID: 1,
		Capability: enginewire.CapabilityCatalogSchema, Operation: enginewire.OperationList,
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("WriteClientFrame(catalog) error = %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	failed := readServerFrame[enginewire.Failed[enginewire.NonCredentialFailure]](t, h)
	if failed.Error.Kind != enginewire.FailureInternal {
		t.Errorf("failed error = %#v, want internal", failed.Error)
	}
	manifest := enginewire.ManifestRequest{
		Type: "request", ID: 2,
		Capability: enginewire.CapabilityEngineManifest, Operation: enginewire.OperationManifest,
	}
	if err := enginewire.WriteClientFrame(h.input, manifest); err != nil {
		t.Fatalf("WriteClientFrame(manifest) error = %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	completed := readServerFrame[enginewire.Completed[enginewire.EngineManifestResult]](t, h)
	if completed.ID != 2 || completed.Sequence != 2 {
		t.Errorf("completed manifest = %#v", completed)
	}
	finishHarness(t, h, nil)
}

func TestHostCanonicalizesSuccessfulResourceScope(t *testing.T) {
	spec := hostListSpec()
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
		read: func(_ context.Context, req machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
			if req.Input.Resource != "locations" {
				return machine.ResourceReadResult{}, errors.New("resource was not canonicalized")
			}
			records := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{"id": "1"}})
			return machine.NewResourceReadResult(records), nil
		},
	}
	h := newHostHarness(t, engine)
	h.initialize(t)
	request := enginewire.ResourceListRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
		Input: enginewire.ResourceListInput{Product: enginewire.ProductZIA, Resource: " locations ", Fields: []string{}, Filters: []enginewire.Filter{}, Search: ""},
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("write request: %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	item := readServerFrame[enginewire.Item[enginewire.ProjectedRecord]](t, h)
	if item.Item.Resource != "locations" {
		t.Fatalf("projected item resource = %q, want locations", item.Item.Resource)
	}
	completed := readServerFrame[enginewire.Completed[enginewire.ResourceReadSummary]](t, h)
	if completed.Result.Records != 1 || completed.Result.StreamItemsEmitted != 1 {
		t.Fatalf("resource completion = %#v", completed)
	}
	finishHarness(t, h, nil)
}

func TestHostEOFWhileActiveWritesCanceledAndExitsCleanly(t *testing.T) {
	spec := hostListSpec()
	readEntered := make(chan struct{})
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
		read: func(ctx context.Context, _ machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
			close(readEntered)
			<-ctx.Done()
			return machine.ResourceReadResult{}, ctx.Err()
		},
	}
	h := newHostHarness(t, engine)
	h.initialize(t)
	request := enginewire.ResourceListRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
		Input: enginewire.ResourceListInput{Product: enginewire.ProductZIA, Resource: "locations", Fields: []string{}, Filters: []enginewire.Filter{}, Search: ""},
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("write request: %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	select {
	case <-readEntered:
	case <-time.After(time.Second):
		t.Fatal("fake read did not start")
	}
	if err := h.input.Close(); err != nil {
		t.Fatalf("close input: %v", err)
	}
	canceled := readServerFrame[enginewire.Canceled](t, h)
	if canceled.ID != 1 || canceled.Sequence != 2 {
		t.Fatalf("canceled = %#v", canceled)
	}
	select {
	case err := <-h.result:
		if err != nil {
			t.Fatalf("Host.Serve() error = %v, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Host.Serve() did not return")
	}
}

func TestHostCancellationAfterVisibleItemDoesNotOverrideSuccess(t *testing.T) {
	spec := hostListSpec()
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
		read: func(context.Context, machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
			records := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{"id": "1"}})
			return machine.NewResourceReadResult(records), nil
		},
	}
	h := newHostHarness(t, engine)
	h.initialize(t)
	request := enginewire.ResourceListRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
		Input: enginewire.ResourceListInput{Product: enginewire.ProductZIA, Resource: "locations", Fields: []string{}, Filters: []enginewire.Filter{}, Search: ""},
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("write request: %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	_ = readServerFrame[enginewire.Item[enginewire.ProjectedRecord]](t, h)
	if err := enginewire.WriteClientFrame(h.input, enginewire.Cancel{Type: "cancel", ID: 1}); err != nil {
		t.Fatalf("write late cancel: %v", err)
	}
	completed := readServerFrame[enginewire.Completed[enginewire.ResourceReadSummary]](t, h)
	if completed.ID != 1 || completed.Sequence != 3 {
		t.Fatalf("completed = %#v", completed)
	}
	finishHarness(t, h, nil)
}

func TestCommittedSuccessSurvivesGracefulContextCancel(t *testing.T) {
	spec := hostListSpec()
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
		read: func(context.Context, machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
			records := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{"id": "1"}, {"id": "2"}})
			return machine.NewResourceReadResult(records), nil
		},
	}
	h := newHostHarness(t, engine)
	h.initialize(t)
	request := enginewire.ResourceListRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
		Input: enginewire.ResourceListInput{Product: enginewire.ProductZIA, Resource: "locations", Fields: []string{}, Filters: []enginewire.Filter{}, Search: ""},
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("WriteClientFrame(list) error = %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	first := readServerFrame[enginewire.Item[enginewire.ProjectedRecord]](t, h)
	firstID, err := first.Item.Record["id"].Value()
	if err != nil || firstID != "1" {
		t.Errorf("first item = %#v", first.Item)
	}
	h.cancel()
	second := readServerFrame[enginewire.Item[enginewire.ProjectedRecord]](t, h)
	secondID, err := second.Item.Record["id"].Value()
	if err != nil || secondID != "2" {
		t.Errorf("second item = %#v", second.Item)
	}
	completed := readServerFrame[enginewire.Completed[enginewire.ResourceReadSummary]](t, h)
	if completed.Result.Records != 2 || completed.Sequence != 4 {
		t.Errorf("completed = %#v", completed)
	}
	select {
	case err := <-h.result:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Host.Serve() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Host.Serve() did not finish committed success")
	}
}

func TestHostMalformedBootstrapWritesFatalProtocolError(t *testing.T) {
	engine := &fakeEngine{manifest: machine.EngineManifestFromCatalog(nil)}
	h := newHostHarness(t, engine)
	data, err := h.output.ReadFrameLimit(enginewire.BootstrapFrameBytes)
	if err != nil {
		t.Fatalf("read hello: %v", err)
	}
	if _, err := enginewire.DecodeBootstrapServerFrame(data); err != nil {
		t.Fatalf("decode hello: %v", err)
	}
	if _, err := io.WriteString(h.input, "{}\n"); err != nil {
		t.Fatalf("write malformed bootstrap: %v", err)
	}
	data, err = h.output.ReadFrameLimit(enginewire.BootstrapFrameBytes)
	if err != nil {
		t.Fatalf("read bootstrap protocol error: %v", err)
	}
	frame, err := enginewire.DecodeBootstrapServerFrame(data)
	if err != nil {
		t.Fatalf("decode bootstrap protocol error: %v", err)
	}
	protocolError, ok := frame.(enginewire.BootstrapProtocolError)
	if !ok || protocolError.Error.Kind != enginewire.ProtocolErrorViolation {
		t.Fatalf("bootstrap protocol error = %#v", frame)
	}
	select {
	case err := <-h.result:
		if !errors.Is(err, ErrProtocol) || ExitCode(err) != 2 {
			t.Fatalf("Host.Serve() error = %v, exit = %d", err, ExitCode(err))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Host.Serve() did not return")
	}
}

func TestHostMalformedV1WhileActiveWritesFatalProtocolError(t *testing.T) {
	spec := hostListSpec()
	engine := &fakeEngine{
		manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
		read: func(ctx context.Context, _ machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
			<-ctx.Done()
			return machine.ResourceReadResult{}, ctx.Err()
		},
	}
	h := newHostHarness(t, engine)
	h.initialize(t)
	request := enginewire.ResourceListRequest{
		Type: "request", ID: 1, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
		Input: enginewire.ResourceListInput{Product: enginewire.ProductZIA, Resource: "locations", Fields: []string{}, Filters: []enginewire.Filter{}, Search: ""},
	}
	if err := enginewire.WriteClientFrame(h.input, request); err != nil {
		t.Fatalf("WriteClientFrame(list) error = %v", err)
	}
	_ = readServerFrame[enginewire.Started](t, h)
	if _, err := io.WriteString(h.input, "{}\n"); err != nil {
		t.Fatalf("WriteString(malformed v1) error = %v", err)
	}
	protocolError := readServerFrame[enginewire.ProtocolError](t, h)
	if protocolError.Error.Kind != enginewire.ProtocolErrorViolation {
		t.Errorf("protocol error = %#v", protocolError)
	}
	select {
	case err := <-h.result:
		if !errors.Is(err, ErrProtocol) || errors.Is(err, ErrJoinTimeout) {
			t.Errorf("Host.Serve() error = %v, want ErrProtocol without ErrJoinTimeout", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Host.Serve() did not return after malformed v1 frame")
	}
}

func TestCoordinatorClassifiesRequestsAroundTerminalWrite(t *testing.T) {
	engine := &fakeEngine{manifest: machine.EngineManifestFromCatalog(nil)}
	host, err := New(engine, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	requestFrame := enginewire.ManifestRequest{
		Type: "request", ID: 2, Capability: enginewire.CapabilityEngineManifest, Operation: enginewire.OperationManifest,
	}
	data, err := enginewire.MarshalClientFrame(requestFrame)
	if err != nil {
		t.Fatalf("MarshalClientFrame() error = %v", err)
	}
	active := &activeRequest{request: wireRequest{id: 1}}
	before := &coordinator{
		host: host, phase: phaseRunning, lastID: 1, active: active,
		pending: &outboundFrame{kind: outboundTerminal},
	}
	before.handleDecoded(decodeResult{data: data}, time.Second)
	if before.held == nil || before.held.id != 2 || !before.heldBusy {
		t.Errorf("request before terminal write: held=%#v busy=%t", before.held, before.heldBusy)
	}

	after := &coordinator{
		host: host, phase: phaseRunning, lastID: 1, active: active,
		inflight: &outboundFrame{kind: outboundTerminal},
	}
	after.handleDecoded(decodeResult{data: data}, time.Second)
	if after.held == nil || after.held.id != 2 || after.heldBusy {
		t.Errorf("request during terminal write: held=%#v busy=%t", after.held, after.heldBusy)
	}
}

func TestCoordinatorFailedEffectReleasesProtectedShutdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		mode             shutdownMode
		cancelAsked      bool
		wantAck          error
		wantCancelAsked  bool
		wantAbandoned    bool
		wantOutcomeAlive bool
	}{
		{
			name: "graceful", mode: shutdownGraceful, wantAck: context.Canceled,
			wantCancelAsked: true, wantOutcomeAlive: true,
		},
		{
			name: "fatal", mode: shutdownFatal, wantAck: errSessionClosing,
			wantAbandoned: true,
		},
		{
			name: "transport", mode: shutdownTransport, wantAck: errSessionClosing,
			wantAbandoned: true,
		},
		{
			name: "graceful after cancel", mode: shutdownGraceful, cancelAsked: true,
			wantAck: context.Canceled, wantCancelAsked: true, wantOutcomeAlive: true,
		},
		{
			name: "fatal after cancel", mode: shutdownFatal, cancelAsked: true,
			wantAck: errSessionClosing, wantCancelAsked: true, wantAbandoned: true,
		},
		{
			name: "transport after cancel", mode: shutdownTransport, cancelAsked: true,
			wantAck: errSessionClosing, wantCancelAsked: true, wantAbandoned: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opCtx, opCancel := context.WithCancel(context.Background())
			messageCtx, messageCancel := context.WithCancel(context.Background())
			outcomeCtx, outcomeCancel := context.WithCancel(context.Background())
			active := &activeRequest{
				opCtx: opCtx, opCancel: opCancel,
				messageCtx: messageCtx, messageCancel: messageCancel,
				outcomeCtx: outcomeCtx, outcomeCancel: outcomeCancel,
				effectCommitting: true,
				cancelAsked:      tt.cancelAsked,
			}
			coordinator := &coordinator{
				active:   active,
				shutdown: &shutdownState{mode: tt.mode},
			}
			ack := make(chan error, 1)
			coordinator.handleEffectBoundary(effectBoundary{
				stage: effectBoundaryFinish, applied: false, ack: ack,
			}, time.Second)
			t.Cleanup(func() {
				if coordinator.shutdown.timer != nil {
					coordinator.shutdown.timer.Stop()
				}
				opCancel()
				messageCancel()
				outcomeCancel()
			})

			if err := <-ack; !errors.Is(err, tt.wantAck) {
				t.Fatalf("effect finish acknowledgement = %v, want %v", err, tt.wantAck)
			}
			if active.effectCommitting || active.effectCommitted {
				t.Errorf("effect state committing=%t committed=%t, want both false", active.effectCommitting, active.effectCommitted)
			}
			if coordinator.shutdown.timer == nil {
				t.Fatal("shutdown timer = nil, want bounded shutdown restored")
			}
			if active.cancelTimer != nil {
				active.cancelTimer.Stop()
				t.Fatal("operation cancel timer is armed after shutdown took ownership of the deadline")
			}
			if active.cancelAsked != tt.wantCancelAsked {
				t.Errorf("cancelAsked = %t, want %t", active.cancelAsked, tt.wantCancelAsked)
			}
			if active.abandoned != tt.wantAbandoned {
				t.Errorf("abandoned = %t, want %t", active.abandoned, tt.wantAbandoned)
			}
			if opCtx.Err() == nil || messageCtx.Err() == nil {
				t.Error("operation and message contexts remain live after failed protected effect")
			}
			if got := outcomeCtx.Err() == nil; got != tt.wantOutcomeAlive {
				t.Errorf("outcome context alive = %t, want %t", got, tt.wantOutcomeAlive)
			}
		})
	}
}

func TestCoordinatorReadyCancelWinsBeforeEffectBegin(t *testing.T) {
	t.Parallel()

	data, err := enginewire.MarshalClientFrame(enginewire.Cancel{Type: "cancel", ID: 1})
	if err != nil {
		t.Fatalf("MarshalClientFrame(cancel) error = %v", err)
	}
	opCtx, opCancel := context.WithCancel(context.Background())
	messageCtx, messageCancel := context.WithCancel(context.Background())
	outcomeCtx, outcomeCancel := context.WithCancel(context.Background())
	active := &activeRequest{
		request: wireRequest{id: 1},
		opCtx:   opCtx, opCancel: opCancel,
		messageCtx: messageCtx, messageCancel: messageCancel,
		outcomeCtx: outcomeCtx, outcomeCancel: outcomeCancel,
	}
	decoded := make(chan decodeResult, 1)
	decoded <- decodeResult{data: data}
	coordinator := &coordinator{
		phase:           phaseRunning,
		active:          active,
		decodeResults:   decoded,
		readOutstanding: true,
	}
	ack := make(chan error, 1)
	coordinator.handleEffectBoundary(effectBoundary{
		stage: effectBoundaryBegin, ack: ack,
	}, time.Second)
	t.Cleanup(func() {
		if active.cancelTimer != nil {
			active.cancelTimer.Stop()
		}
		opCancel()
		messageCancel()
		outcomeCancel()
	})

	if err := <-ack; !errors.Is(err, context.Canceled) {
		t.Fatalf("effect begin acknowledgement = %v, want context.Canceled", err)
	}
	if active.effectCommitting || active.effectCommitted {
		t.Errorf("effect state committing=%t committed=%t, want both false", active.effectCommitting, active.effectCommitted)
	}
	if !active.cancelAsked || active.cancelTimer == nil {
		t.Errorf("cancellation state asked=%t timer=%v, want queued cancel active", active.cancelAsked, active.cancelTimer)
	}
	if opCtx.Err() == nil || messageCtx.Err() == nil {
		t.Error("ready cancellation did not cancel operation and event contexts")
	}
	if outcomeCtx.Err() != nil {
		t.Error("ready cancellation canceled the outcome channel before canceled terminal")
	}
	if coordinator.readOutstanding {
		t.Error("ready cancellation frame remains outstanding after effect admission")
	}
}

func TestCoordinatorTransportFailureCannotAbandonCommittedEffect(t *testing.T) {
	t.Parallel()

	opCtx, opCancel := context.WithCancel(context.Background())
	messageCtx, messageCancel := context.WithCancel(context.Background())
	outcomeCtx, outcomeCancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	active := &activeRequest{
		opCtx: opCtx, opCancel: opCancel,
		messageCtx: messageCtx, messageCancel: messageCancel,
		outcomeCtx: outcomeCtx, outcomeCancel: outcomeCancel,
		workerStarted:   true,
		effectCommitted: true,
		done:            done,
	}
	coordinator := &coordinator{
		active:      active,
		closeInput:  func() {},
		closeOutput: func() {},
	}
	coordinator.beginTransport(ErrTransport, time.Second)
	t.Cleanup(func() {
		opCancel()
		messageCancel()
		outcomeCancel()
		close(done)
	})

	if coordinator.shutdown == nil || coordinator.shutdown.mode != shutdownTransport {
		t.Fatalf("shutdown = %#v, want transport shutdown", coordinator.shutdown)
	}
	if coordinator.shutdown.timer != nil {
		coordinator.shutdown.timer.Stop()
		t.Fatal("transport shutdown timer is armed during committed effect cleanup")
	}
	if active.abandoned {
		t.Fatal("committed effect worker was abandoned after transport failure")
	}
	if opCtx.Err() != nil || outcomeCtx.Err() != nil {
		t.Fatal("committed effect operation or outcome context was canceled")
	}
	if messageCtx.Err() == nil {
		t.Fatal("provisional event context remains live after output transport failure")
	}

	coordinator.handleWorkerMessage(operationMessage{outcome: &operationOutcome{}}, time.Second)
	if !active.committed {
		t.Fatal("post-commit outcome was not drained after output transport failure")
	}
	active.workerDone = true
	if !coordinator.shouldFinish() {
		t.Fatal("transport shutdown does not finish after committed worker exits")
	}
}

type blockAtWriter struct {
	buffer  bytes.Buffer
	blockAt int32
	writes  atomic.Int32
	closed  atomic.Int32
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockAtWriter(blockAt int32) *blockAtWriter {
	return &blockAtWriter{
		blockAt: blockAt,
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (w *blockAtWriter) Write(data []byte) (int, error) {
	if w.writes.Add(1) == w.blockAt {
		close(w.entered)
		<-w.release
		return 0, io.ErrClosedPipe
	}
	return w.buffer.Write(data)
}

func (w *blockAtWriter) Close() error {
	if w.closed.Add(1) == 1 {
		w.once.Do(func() { close(w.release) })
	}
	return nil
}

func TestCommittedEffectCleanupRestoresShutdownDeadline(t *testing.T) {
	tests := []struct {
		name  string
		fatal bool
	}{
		{name: "graceful"},
		{name: "fatal", fatal: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effectEntered := make(chan struct{})
			effectRelease := make(chan struct{})
			cleanupDone := make(chan struct{})
			spec := hostListSpec()
			engine := &fakeEngine{
				manifest: machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec}),
				dump: func(ctx context.Context, _ machine.DumpRequest, sink machine.EventSink) (machine.DumpResult, error) {
					if err := sink(machine.Event{Kind: machine.EventStarted, Total: 0}); err != nil {
						return machine.DumpResult{}, err
					}
					if err := effectcommit.Run(ctx, func() error {
						close(effectEntered)
						<-effectRelease
						return nil
					}); err != nil {
						return machine.DumpResult{}, err
					}
					close(cleanupDone)
					result := machine.NewDumpResult(0, 0, "standard", []machine.DumpResourceError{})
					if err := sink(machine.Event{Kind: machine.EventCompleted}); err != nil {
						return result, err
					}
					return result, nil
				},
			}
			host, err := New(engine, "test")
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			host.joinTimeout = 30 * time.Millisecond
			inputReader, inputWriter := io.Pipe()
			writer := newBlockAtWriter(4)
			result := make(chan error, 1)
			go func() {
				result <- host.Serve(context.Background(), Streams{
					Input: inputReader, Output: writer,
					CloseInput: inputReader.Close, CloseOutput: writer.Close,
				})
			}()
			t.Cleanup(func() {
				_ = inputWriter.Close()
				_ = writer.Close()
			})

			if err := enginewire.WriteBootstrapClientFrame(inputWriter, enginewire.Initialize{
				Type: "initialize", Protocol: enginewire.Protocol, Version: enginewire.V1Version,
			}); err != nil {
				t.Fatalf("WriteBootstrapClientFrame(initialize) error = %v", err)
			}
			if err := enginewire.WriteClientFrame(inputWriter, enginewire.DumpRequest{
				Type: "request", ID: 1,
				Capability: enginewire.CapabilityDumpWrite, Operation: enginewire.OperationDump,
				Input: enginewire.DumpInput{
					OutputDir: "out", Products: []enginewire.Product{}, Resources: []enginewire.ResourceSelector{},
				},
			}); err != nil {
				t.Fatalf("WriteClientFrame(dump) error = %v", err)
			}
			select {
			case <-effectEntered:
			case <-time.After(time.Second):
				t.Fatal("effect did not enter")
			}
			if tt.fatal {
				if _, err := io.WriteString(inputWriter, "{}\n"); err != nil {
					t.Fatalf("WriteString(malformed frame) error = %v", err)
				}
				select {
				case <-writer.entered:
				case <-time.After(time.Second):
					t.Fatal("fatal writer did not block")
				}
			} else if err := inputWriter.Close(); err != nil {
				t.Fatalf("inputWriter.Close() error = %v", err)
			}
			close(effectRelease)
			select {
			case <-cleanupDone:
			case <-time.After(time.Second):
				t.Fatal("committed effect cleanup did not finish")
			}
			if !tt.fatal {
				select {
				case <-writer.entered:
				case <-time.After(time.Second):
					t.Fatal("terminal writer did not block")
				}
			}
			select {
			case err := <-result:
				if !errors.Is(err, ErrJoinTimeout) {
					t.Fatalf("Host.Serve() error = %v, want ErrJoinTimeout", err)
				}
				if tt.fatal && !errors.Is(err, ErrProtocol) {
					t.Fatalf("Host.Serve() error = %v, want ErrProtocol", err)
				}
			case <-time.After(time.Second):
				t.Fatal("Host.Serve() did not restore the post-cleanup shutdown deadline")
			}
			if writer.closed.Load() != 1 {
				t.Errorf("blocked writer close calls = %d, want 1", writer.closed.Load())
			}
		})
	}
}

func hostListSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product: resources.ProductZIA, Name: "locations", Shape: resources.ShapeList,
		Operations: resources.ReadOperations(), GetKey: "id",
		Fields: []resources.FieldSpec{{
			Name: "id", Classification: resources.ClassOperational,
			AllowedModes: []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid},
		}},
	}
}
