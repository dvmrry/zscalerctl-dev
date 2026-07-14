package enginehost

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/enginewire/adapter"
)

var (
	// ErrProtocol marks fatal client input or unsupported negotiation.
	ErrProtocol = errors.New("engine stdio protocol failure")
	// ErrTransport marks an indeterminate or broken stdout transport.
	ErrTransport = errors.New("engine stdio transport failure")
	// ErrInternal marks an impossible host, engine, or codec state.
	ErrInternal = errors.New("engine stdio internal failure")
	// ErrJoinTimeout marks a goroutine that did not stop within the shutdown ceiling.
	ErrJoinTimeout = errors.New("engine stdio shutdown join timed out")
)

const defaultJoinTimeout = 5 * time.Second

// Streams are the child-process byte streams and optional unblock hooks used
// by Host.Serve. CloseInput and CloseOutput must be safe to call at most once.
// CloseOutput is a failure-path unblock hook; a graceful in-process session
// does not take ownership of or close an arbitrary caller-provided writer.
type Streams struct {
	Input       io.Reader
	Output      io.Writer
	CloseInput  func() error
	CloseOutput func() error
}

// Host owns one long-lived local stdio session over a trusted synchronous
// Engine. A Host is single-use and must not serve concurrent sessions.
type Host struct {
	engine       Engine
	ready        enginewire.Ready
	availability capabilityAvailability
	joinTimeout  time.Duration
	served       atomic.Bool
}

// New constructs a config-free host contract. It does not load credentials,
// resolve providers, create an SDK client, access the filesystem, or contact a
// tenant.
func New(engine Engine, buildVersion string) (*Host, error) {
	if isNilEngine(engine) {
		return nil, fmt.Errorf("%w: nil engine", ErrInternal)
	}
	ready, err := readyFromEngine(engine, buildVersion)
	if err != nil {
		return nil, fmt.Errorf("%w: construct ready frame: %v", ErrInternal, err)
	}
	availability := availabilityFromManifest(ready.Engine)
	if failure := availability.failure(enginewire.CapabilityEngineManifest, enginewire.OperationManifest); failure != "" {
		return nil, fmt.Errorf("%w: engine manifest capability is unavailable", ErrInternal)
	}
	return &Host{
		engine: engine, ready: ready, availability: availability, joinTimeout: defaultJoinTimeout,
	}, nil
}

func readyFromEngine(engine Engine, buildVersion string) (ready enginewire.Ready, err error) {
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("engine manifest panicked")
		}
	}()
	return adapter.ToReady(engine.EngineManifest(), buildVersion)
}

type capabilityAvailability map[enginewire.Capability]map[enginewire.Operation]struct{}

func availabilityFromManifest(manifest enginewire.EngineManifest) capabilityAvailability {
	out := make(capabilityAvailability, len(manifest.Capabilities))
	for _, capability := range manifest.Capabilities {
		operations := make(map[enginewire.Operation]struct{}, len(capability.Operations))
		for _, operation := range capability.Operations {
			operations[operation] = struct{}{}
		}
		out[capability.Name] = operations
	}
	return out
}

func (a capabilityAvailability) failure(
	capability enginewire.Capability,
	operation enginewire.Operation,
) enginewire.OperationFailureKind {
	operations, ok := a[capability]
	if !ok {
		return enginewire.FailureUnsupportedCapability
	}
	if _, ok := operations[operation]; !ok {
		return enginewire.FailureUnsupportedOperation
	}
	return ""
}

type sessionPhase uint8

const (
	phaseHello sessionPhase = iota + 1
	phaseBootstrap
	phaseReady
	phaseRunning
)

type shutdownMode uint8

const (
	shutdownGraceful shutdownMode = iota + 1
	shutdownFatal
	shutdownTransport
)

type shutdownState struct {
	mode           shutdownMode
	result         error
	protocolKind   enginewire.ProtocolErrorKind
	bootstrapFatal bool
	fatalQueued    bool
	fatalWritten   bool
	timedOut       bool
	timer          *time.Timer
}

type activeRequest struct {
	request wireRequest
	seq     enginewire.SafeInteger

	opCtx         context.Context
	opCancel      context.CancelFunc
	messageCtx    context.Context
	messageCancel context.CancelFunc
	outcomeCtx    context.Context
	outcomeCancel context.CancelFunc
	messages      chan operationMessage
	done          chan struct{}

	workerStarted    bool
	workerDone       bool
	cancelAsked      bool
	cancelTimer      *time.Timer
	effectCommitting bool
	effectCommitted  bool
	committed        bool
	success          bool
	abandoned        bool
	cursor           *successCursor
	terminalWrote    bool
}

func (r *activeRequest) effectProtected() bool {
	return r != nil && (r.effectCommitting || r.effectCommitted)
}

func (r *activeRequest) effectInFlight() bool {
	return r != nil && !r.workerDone && r.effectProtected()
}

type coordinator struct {
	host    *Host
	streams Streams

	ctx    context.Context
	cancel context.CancelFunc

	decodeLimits  chan int
	decodeResults chan decodeResult
	decoderDone   chan struct{}
	writes        chan outboundFrame
	acks          chan writeAck
	writerDone    chan struct{}

	decoderStopped  bool
	writerStopped   bool
	readOutstanding bool
	inputClosed     bool

	phase    sessionPhase
	pending  *outboundFrame
	inflight *outboundFrame
	held     *wireRequest
	heldBusy bool
	active   *activeRequest
	lastID   enginewire.SafeInteger

	shutdown *shutdownState

	closeInput  func()
	closeOutput func()
}

// Serve performs one complete stdio session. The caller supplies process
// signal cancellation through ctx; Serve attempts a graceful operation
// terminal before returning that context error.
func (h *Host) Serve(ctx context.Context, streams Streams) error {
	if h == nil || isNilEngine(h.engine) {
		return fmt.Errorf("%w: nil host", ErrInternal)
	}
	if isNilIO(streams.Input) || isNilIO(streams.Output) {
		return fmt.Errorf("%w: nil session stream", ErrInternal)
	}
	if !h.served.CompareAndSwap(false, true) {
		return fmt.Errorf("%w: host already served a session", ErrInternal)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	joinTimeout := h.joinTimeout
	if joinTimeout <= 0 {
		joinTimeout = defaultJoinTimeout
	}
	sessionCtx, sessionCancel := context.WithCancel(context.Background())
	c := &coordinator{
		host: h, streams: streams,
		ctx: sessionCtx, cancel: sessionCancel,
		decodeLimits: make(chan int), decodeResults: make(chan decodeResult, 1), decoderDone: make(chan struct{}),
		writes: make(chan outboundFrame), acks: make(chan writeAck, 1), writerDone: make(chan struct{}),
		phase: phaseHello,
	}
	c.closeInput = onceClose(streams.CloseInput, streams.Input)
	c.closeOutput = onceClose(streams.CloseOutput, streams.Output)
	hello := enginewire.Hello{
		Type: "hello", Protocol: enginewire.Protocol, Versions: []string{enginewire.V1Version},
		Bootstrap: enginewire.BootstrapLimits{FrameBytes: enginewire.BootstrapFrameBytes, JSONDepth: enginewire.BootstrapJSONDepth},
	}
	c.pending = &outboundFrame{kind: outboundHello, bootstrap: hello}

	go runDecoder(sessionCtx, streams.Input, c.decodeLimits, c.decodeResults, c.decoderDone)
	go runWriter(sessionCtx, streams.Output, c.writes, c.acks, c.writerDone)

	result := c.run(ctx, joinTimeout)
	return result
}

func (c *coordinator) run(externalCtx context.Context, joinTimeout time.Duration) error {
	for {
		c.fillPending()
		if c.shouldFinish() {
			break
		}

		var writeCh chan<- outboundFrame
		var writeValue outboundFrame
		if c.pending != nil && c.inflight == nil && !c.writerStopped && c.pendingWriteReady() {
			writeCh = c.writes
			writeValue = *c.pending
		}

		var limitCh chan<- int
		limit := 0
		if c.canRead() {
			limitCh = c.decodeLimits
			if c.phase == phaseBootstrap {
				limit = enginewire.BootstrapFrameBytes
			} else {
				limit = enginewire.V1FrameBytes
			}
		}

		var workerMessages <-chan operationMessage
		var workerDone <-chan struct{}
		if c.active != nil && c.active.workerStarted {
			if !c.active.workerDone {
				workerDone = c.active.done
			}
			canReceive := c.pending == nil || c.active.effectCommitting
			if c.shutdown != nil &&
				(c.shutdown.mode == shutdownFatal || c.shutdown.mode == shutdownTransport) &&
				c.active.effectCommitted {
				canReceive = true
			}
			if !c.active.abandoned && canReceive && !c.active.committed {
				workerMessages = c.active.messages
			}
		}

		var shutdownTimer <-chan time.Time
		if c.shutdown != nil && c.shutdown.timer != nil {
			shutdownTimer = c.shutdown.timer.C
		}
		var externalDone <-chan struct{}
		if c.shutdown == nil {
			externalDone = externalCtx.Done()
		}
		var operationCancelTimer <-chan time.Time
		if c.shutdown == nil && c.active != nil && c.active.cancelTimer != nil && !c.active.workerDone {
			operationCancelTimer = c.active.cancelTimer.C
		}
		var decoderDone <-chan struct{}
		if !c.decoderStopped {
			decoderDone = c.decoderDone
		}
		var writerDone <-chan struct{}
		if !c.writerStopped {
			writerDone = c.writerDone
		}

		select {
		case writeCh <- writeValue:
			copyFrame := writeValue
			c.inflight = &copyFrame
			c.pending = nil

		case limitCh <- limit:
			c.readOutstanding = true

		case decoded := <-c.decodeResults:
			c.readOutstanding = false
			c.handleDecoded(decoded, joinTimeout)

		case ack := <-c.acks:
			c.handleWriteAck(ack, joinTimeout)

		case message := <-workerMessages:
			c.handleWorkerMessage(message, joinTimeout)

		case <-workerDone:
			c.markWorkerDone(joinTimeout)
			c.maybeReleaseActive()

		case <-decoderDone:
			c.decoderStopped = true

		case <-writerDone:
			c.writerStopped = true
			if c.shutdown == nil && c.inflight == nil {
				c.beginTransport(fmt.Errorf("%w: writer stopped", ErrTransport), joinTimeout)
			}

		case <-externalDone:
			c.beginGraceful(externalCtx.Err(), joinTimeout)

		case <-operationCancelTimer:
			c.handleOperationCancelTimeout(joinTimeout)

		case <-shutdownTimer:
			if c.active != nil && c.active.effectInFlight() {
				c.shutdown.timer = nil
				continue
			}
			c.shutdown.timedOut = true
			c.closeInput()
			c.closeOutput()
			if c.active != nil {
				c.active.opCancel()
				c.active.messageCancel()
				c.active.outcomeCancel()
			}
		}
	}

	c.cancel()
	c.closeInput()
	if c.shutdown != nil && (c.shutdown.mode == shutdownTransport || c.shutdown.timedOut) {
		c.closeOutput()
	}

	result := error(nil)
	if c.shutdown != nil {
		if c.shutdown.timer != nil {
			c.shutdown.timer.Stop()
		}
		result = c.shutdown.result
		if c.shutdown.timedOut {
			if result == nil {
				result = ErrJoinTimeout
			} else {
				result = errors.Join(result, ErrJoinTimeout)
			}
			return result
		}
	}
	if err := c.waitForGoroutines(joinTimeout); err != nil {
		c.closeOutput()
		if result == nil {
			return err
		}
		return errors.Join(result, err)
	}
	return result
}

func (c *coordinator) pendingWriteReady() bool {
	if c.pending == nil || c.pending.kind != outboundTerminal || c.active == nil {
		return true
	}
	return !c.active.workerStarted || c.active.workerDone
}

func (c *coordinator) canRead() bool {
	return c.shutdown == nil && !c.inputClosed && !c.readOutstanding && c.held == nil &&
		!c.terminalQueued() && (c.phase == phaseBootstrap || c.phase == phaseRunning)
}

func (c *coordinator) fillPending() {
	if c.pending != nil {
		return
	}
	if c.shutdown != nil && c.shutdown.mode == shutdownFatal && !c.shutdown.fatalQueued {
		c.shutdown.fatalQueued = true
		failure := enginewire.BootstrapError{Kind: c.shutdown.protocolKind}
		if c.shutdown.bootstrapFatal {
			frame := enginewire.BootstrapProtocolError{Type: "protocol_error", Fatal: true, Error: failure}
			c.pending = &outboundFrame{kind: outboundFatal, bootstrap: frame}
		} else {
			frame := enginewire.ProtocolError{Type: "protocol_error", Fatal: true, Error: failure}
			c.pending = &outboundFrame{kind: outboundFatal, v1: frame}
		}
		return
	}
	if c.shutdown == nil && c.held != nil && !c.terminalWriteInFlight() {
		request := *c.held
		c.held = nil
		if c.heldBusy {
			c.heldBusy = false
			frame := enginewire.RequestRejected{Type: "request_rejected", ID: request.id, Reason: "busy"}
			c.pending = &outboundFrame{kind: outboundRejected, v1: frame}
			return
		}
		c.processRequest(request)
		return
	}
	if c.active != nil && c.active.success && c.active.cursor != nil && !c.active.abandoned {
		c.queueNextSuccess()
	}
}

func (c *coordinator) handleDecoded(decoded decodeResult, joinTimeout time.Duration) {
	if c.shutdown != nil {
		return
	}
	if decoded.err != nil {
		if errors.Is(decoded.err, io.EOF) {
			c.inputClosed = true
			c.beginGraceful(nil, joinTimeout)
			return
		}
		kind := enginewire.ProtocolErrorViolation
		if errors.Is(decoded.err, enginewire.ErrFrameTooLarge) {
			kind = enginewire.ProtocolErrorFrameTooLarge
		}
		c.beginFatal(fmt.Errorf("%w: invalid frame", ErrProtocol), kind, joinTimeout)
		return
	}

	switch c.phase {
	case phaseBootstrap:
		frame, err := enginewire.DecodeBootstrapClientFrame(decoded.data)
		if err != nil {
			kind := enginewire.ProtocolErrorViolation
			if errors.Is(err, enginewire.ErrFrameTooLarge) {
				kind = enginewire.ProtocolErrorFrameTooLarge
			}
			c.beginFatal(fmt.Errorf("%w: invalid bootstrap", ErrProtocol), kind, joinTimeout)
			return
		}
		switch typed := frame.(type) {
		case enginewire.Initialize:
			if typed.Version != enginewire.V1Version {
				c.beginFatal(fmt.Errorf("%w: unsupported protocol", ErrProtocol), enginewire.ProtocolErrorUnsupportedProtocol, joinTimeout)
				return
			}
			ready := c.host.ready
			c.pending = &outboundFrame{kind: outboundReady, v1: ready}
			c.phase = phaseReady
		case enginewire.Reject:
			c.beginFatal(fmt.Errorf("%w: unsupported protocol", ErrProtocol), enginewire.ProtocolErrorUnsupportedProtocol, joinTimeout)
		default:
			c.beginFatal(fmt.Errorf("%w: invalid bootstrap direction", ErrProtocol), enginewire.ProtocolErrorViolation, joinTimeout)
		}

	case phaseRunning:
		frame, err := enginewire.DecodeClientFrame(decoded.data)
		if err != nil {
			kind := enginewire.ProtocolErrorViolation
			if errors.Is(err, enginewire.ErrFrameTooLarge) {
				kind = enginewire.ProtocolErrorFrameTooLarge
			}
			c.beginFatal(fmt.Errorf("%w: invalid client frame", ErrProtocol), kind, joinTimeout)
			return
		}
		if cancel, ok := frame.(enginewire.Cancel); ok {
			c.handleCancel(cancel.ID, joinTimeout)
			return
		}
		request, ok := requestFromFrame(frame)
		if !ok {
			c.beginFatal(fmt.Errorf("%w: invalid client frame type", ErrProtocol), enginewire.ProtocolErrorViolation, joinTimeout)
			return
		}
		if request.id <= c.lastID {
			c.beginFatal(fmt.Errorf("%w: request ID is not increasing", ErrProtocol), enginewire.ProtocolErrorViolation, joinTimeout)
			return
		}
		c.lastID = request.id
		request.unavailable = c.host.availability.failure(request.capability, request.operation)
		if c.pending != nil {
			c.held = &request
			c.heldBusy = c.pending.kind == outboundTerminal
			return
		}
		if c.terminalWriteInFlight() {
			c.held = &request
			// A terminal can be visible to the client before the writer's
			// acknowledgement is selected. Hold the one already-authorized
			// read so a request sent after observing the terminal is not
			// nondeterministically rejected as busy.
			c.heldBusy = false
			return
		}
		c.processRequest(request)
	default:
		c.beginFatal(fmt.Errorf("%w: input before ready", ErrProtocol), enginewire.ProtocolErrorViolation, joinTimeout)
	}
}

func (c *coordinator) processRequest(request wireRequest) {
	if c.active != nil {
		frame := enginewire.RequestRejected{Type: "request_rejected", ID: request.id, Reason: "busy"}
		c.pending = &outboundFrame{kind: outboundRejected, v1: frame}
		return
	}
	opCtx, opCancel := context.WithCancel(c.ctx)
	messageCtx, messageCancel := context.WithCancel(c.ctx)
	outcomeCtx, outcomeCancel := context.WithCancel(c.ctx)
	active := &activeRequest{
		request: request, seq: 1,
		opCtx: opCtx, opCancel: opCancel, messageCtx: messageCtx, messageCancel: messageCancel,
		outcomeCtx: outcomeCtx, outcomeCancel: outcomeCancel,
		messages: make(chan operationMessage), done: make(chan struct{}),
	}
	c.active = active
	started := enginewire.Started{
		Type: "started", ID: request.id, Sequence: 1,
		Capability: request.capability, Operation: request.operation,
	}
	c.pending = &outboundFrame{kind: outboundStarted, v1: started}
}

func (c *coordinator) terminalWriteInFlight() bool {
	return c.inflight != nil && c.inflight.kind == outboundTerminal
}

func (c *coordinator) terminalQueued() bool {
	return (c.pending != nil && c.pending.kind == outboundTerminal) || c.terminalWriteInFlight()
}

func (c *coordinator) handleCancel(id enginewire.SafeInteger, joinTimeout time.Duration) {
	if c.active == nil || c.active.request.id != id || c.active.committed ||
		c.active.effectCommitted || c.active.abandoned {
		return
	}
	if c.active.cancelAsked {
		return
	}
	c.active.cancelAsked = true
	if c.active.effectCommitting {
		return
	}
	c.cancelActiveOperation(joinTimeout)
}

func (c *coordinator) cancelActiveOperation(joinTimeout time.Duration) {
	if c.active == nil || c.active.workerDone || c.active.cancelTimer != nil {
		return
	}
	c.active.opCancel()
	c.active.messageCancel()
	c.active.cancelTimer = time.NewTimer(joinTimeout)
}

func (c *coordinator) launchWorker() {
	active := c.active
	if active == nil || active.workerStarted || active.abandoned {
		return
	}
	active.workerStarted = true
	go func() {
		defer close(active.done)
		runOperation(
			active.opCtx, active.messageCtx, active.outcomeCtx, c.host.engine, active.request,
			c.host.ready.Engine, active.messages,
		)
	}()
}

func (c *coordinator) markWorkerDone(joinTimeout time.Duration) {
	if c.active == nil {
		return
	}
	c.active.workerDone = true
	if c.active.cancelTimer != nil {
		c.active.cancelTimer.Stop()
		c.active.cancelTimer = nil
	}
	if c.shutdown != nil && c.shutdown.timer == nil && !c.shutdown.timedOut {
		c.shutdown.timer = time.NewTimer(joinTimeout)
	}
}

func (c *coordinator) handleOperationCancelTimeout(joinTimeout time.Duration) {
	if c.active == nil || c.active.workerDone || !c.active.cancelAsked || c.active.effectProtected() {
		return
	}
	select {
	case <-c.active.done:
		c.markWorkerDone(joinTimeout)
		c.maybeReleaseActive()
		return
	default:
	}
	c.shutdown = &shutdownState{mode: shutdownTransport, timedOut: true}
	c.inputClosed = true
	c.held = nil
	c.heldBusy = false
	c.pending = nil
	c.closeInput()
	c.closeOutput()
	c.active.abandoned = true
	c.active.opCancel()
	c.active.messageCancel()
	c.active.outcomeCancel()
	c.active.cursor = nil
}

func (c *coordinator) handleWorkerMessage(message operationMessage, joinTimeout time.Duration) {
	if message.effect != nil {
		c.handleEffectBoundary(*message.effect, joinTimeout)
		return
	}
	if c.active == nil || c.active.abandoned || c.active.committed {
		c.beginFatal(fmt.Errorf("%w: unexpected worker message", ErrInternal), enginewire.ProtocolErrorInternal, joinTimeout)
		return
	}
	if c.shutdown != nil && c.active.effectProtected() &&
		(c.shutdown.mode == shutdownFatal || c.shutdown.mode == shutdownTransport) {
		if message.outcome != nil {
			c.active.committed = true
		}
		return
	}
	if message.provisional != nil {
		sequence, ok := c.nextSequence()
		if !ok {
			c.beginFatal(fmt.Errorf("%w: sequence exhausted", ErrInternal), enginewire.ProtocolErrorInternal, joinTimeout)
			return
		}
		frame := message.provisional.serverFrame(c.active.request.id, sequence)
		c.pending = &outboundFrame{kind: outboundProvisional, v1: frame}
		return
	}
	if message.outcome == nil {
		c.beginFatal(fmt.Errorf("%w: empty worker outcome", ErrInternal), enginewire.ProtocolErrorInternal, joinTimeout)
		return
	}
	outcome := *message.outcome
	if c.active.cancelAsked {
		c.active.committed = true
		c.queueCanceled(joinTimeout)
		return
	}
	if outcome.plan != nil {
		if outcome.conversion.Canceled || outcome.conversion.Failure != nil {
			c.beginFatal(fmt.Errorf("%w: mixed worker outcome", ErrInternal), enginewire.ProtocolErrorInternal, joinTimeout)
			return
		}
		if uint64(c.active.seq) > enginewire.MaxSafeInteger-outcome.plan.frameCount {
			c.active.committed = true
			c.queueFailure(enginewire.NonCredentialFailure{Kind: enginewire.FailureResponseTooLarge}, joinTimeout)
			return
		}
		c.active.committed = true
		c.active.success = true
		c.active.cursor = newSuccessCursor(outcome.plan)
		return
	}
	c.active.committed = true
	if outcome.conversion.Canceled {
		c.queueCanceled(joinTimeout)
		return
	}
	if outcome.conversion.Failure == nil {
		c.beginFatal(fmt.Errorf("%w: failure outcome has no failure", ErrInternal), enginewire.ProtocolErrorInternal, joinTimeout)
		return
	}
	c.queueFailure(outcome.conversion.Failure, joinTimeout)
}

func (c *coordinator) handleEffectBoundary(boundary effectBoundary, joinTimeout time.Duration) {
	ack := func(err error) {
		select {
		case boundary.ack <- err:
		default:
		}
	}
	if c.active == nil || c.active.abandoned || c.active.committed || boundary.ack == nil {
		ack(errSessionClosing)
		return
	}
	switch boundary.stage {
	case effectBoundaryBegin:
		c.handleReadyInputBeforeEffect(joinTimeout)
		if c.active == nil || c.active.abandoned || c.active.committed {
			ack(errSessionClosing)
			return
		}
		if c.active.effectCommitting || c.active.effectCommitted {
			ack(fmt.Errorf("%w: invalid effect commit transition", ErrInternal))
			return
		}
		if c.active.cancelAsked || c.shutdown != nil {
			ack(context.Canceled)
			return
		}
		c.active.effectCommitting = true
		ack(nil)

	case effectBoundaryFinish:
		if !c.active.effectCommitting || c.active.effectCommitted {
			ack(fmt.Errorf("%w: invalid effect commit transition", ErrInternal))
			return
		}
		c.active.effectCommitting = false
		if boundary.applied {
			c.active.effectCommitted = true
			c.active.cancelAsked = false
			if c.active.cancelTimer != nil {
				c.active.cancelTimer.Stop()
				c.active.cancelTimer = nil
			}
			if c.shutdown != nil && c.shutdown.timer != nil {
				c.shutdown.timer.Stop()
				c.shutdown.timer = nil
			}
			ack(nil)
			return
		}
		if c.shutdown != nil {
			// The shutdown timer was deliberately suppressed while the atomic
			// effect result was indeterminate. A failed effect releases that
			// protection, so restore the ordinary bounded shutdown behavior.
			if c.shutdown.timer == nil {
				c.shutdown.timer = time.NewTimer(joinTimeout)
			}
			if c.shutdown.mode == shutdownGraceful {
				c.active.cancelAsked = true
				c.active.opCancel()
				c.active.messageCancel()
				ack(context.Canceled)
				return
			}
			c.active.abandoned = true
			c.active.opCancel()
			c.active.messageCancel()
			c.active.outcomeCancel()
			c.active.cursor = nil
			ack(errSessionClosing)
			return
		}
		if c.active.cancelAsked {
			c.cancelActiveOperation(joinTimeout)
			ack(context.Canceled)
			return
		}
		ack(nil)

	default:
		ack(fmt.Errorf("%w: invalid effect commit stage", ErrInternal))
	}
}

func (c *coordinator) handleReadyInputBeforeEffect(joinTimeout time.Duration) {
	// The decoder receives at most one frame permit at a time. If that frame is
	// already queued when an effect-begin rendezvous is selected, preserve input
	// arrival order before acknowledging the irreversible attempt. Input that is
	// not yet decoded remains concurrent and linearizes on a later select.
	select {
	case decoded := <-c.decodeResults:
		c.readOutstanding = false
		c.handleDecoded(decoded, joinTimeout)
	default:
	}
}

func (c *coordinator) queueFailure(failure enginewire.OperationFailure, joinTimeout time.Duration) {
	sequence, ok := c.nextSequence()
	if !ok {
		c.beginFatal(fmt.Errorf("%w: sequence exhausted", ErrInternal), enginewire.ProtocolErrorInternal, joinTimeout)
		return
	}
	frame := enginewire.Failed[enginewire.OperationFailure]{
		Type: "failed", ID: c.active.request.id, Sequence: sequence, Error: failure,
	}
	c.pending = &outboundFrame{kind: outboundTerminal, v1: frame}
}

func (c *coordinator) queueCanceled(joinTimeout time.Duration) {
	sequence, ok := c.nextSequence()
	if !ok {
		c.beginFatal(fmt.Errorf("%w: sequence exhausted", ErrInternal), enginewire.ProtocolErrorInternal, joinTimeout)
		return
	}
	frame := enginewire.Canceled{
		Type: "canceled", ID: c.active.request.id, Sequence: sequence,
		Error: enginewire.CanceledError{Kind: "canceled"},
	}
	c.pending = &outboundFrame{kind: outboundTerminal, v1: frame}
}

func (c *coordinator) queueNextSuccess() {
	sequence, ok := c.nextSequence()
	if !ok {
		c.beginFatal(fmt.Errorf("%w: committed sequence exhausted", ErrInternal), enginewire.ProtocolErrorInternal, c.host.joinTimeout)
		return
	}
	frame, terminal, err := c.active.cursor.Next(c.active.request.id, sequence)
	if err != nil {
		c.beginFatal(fmt.Errorf("%w: committed success stream: %v", ErrInternal, err), enginewire.ProtocolErrorInternal, c.host.joinTimeout)
		return
	}
	kind := outboundSuccess
	if terminal {
		kind = outboundTerminal
	}
	c.pending = &outboundFrame{kind: kind, v1: frame}
	if terminal {
		c.active.cursor = nil
	}
}

func (c *coordinator) nextSequence() (enginewire.SafeInteger, bool) {
	if c.active == nil || uint64(c.active.seq) >= enginewire.MaxSafeInteger {
		return 0, false
	}
	c.active.seq++
	return c.active.seq, true
}

func (c *coordinator) handleWriteAck(ack writeAck, joinTimeout time.Duration) {
	if c.inflight == nil || ack.frame.kind != c.inflight.kind {
		c.beginFatal(fmt.Errorf("%w: writer acknowledgement mismatch", ErrInternal), enginewire.ProtocolErrorInternal, joinTimeout)
		return
	}
	c.inflight = nil
	if ack.err != nil {
		if outboundCodecError(ack.err) {
			c.beginFatal(fmt.Errorf("%w: outbound codec rejected preflighted frame", ErrInternal), enginewire.ProtocolErrorInternal, joinTimeout)
		} else {
			c.beginTransport(fmt.Errorf("%w: stdout write failed", ErrTransport), joinTimeout)
		}
		return
	}
	switch ack.frame.kind {
	case outboundHello:
		c.phase = phaseBootstrap
	case outboundReady:
		c.phase = phaseRunning
	case outboundStarted:
		if c.shutdown == nil || c.shutdown.mode == shutdownGraceful {
			c.launchWorker()
		}
	case outboundTerminal:
		if c.active == nil {
			c.beginFatal(fmt.Errorf("%w: terminal write without active request", ErrInternal), enginewire.ProtocolErrorInternal, joinTimeout)
			return
		}
		c.active.terminalWrote = true
		c.maybeReleaseActive()
	case outboundFatal:
		if c.shutdown == nil || c.shutdown.mode != shutdownFatal {
			c.beginTransport(fmt.Errorf("%w: unexpected fatal write", ErrTransport), joinTimeout)
			return
		}
		c.shutdown.fatalWritten = true
	}
}

func (c *coordinator) maybeReleaseActive() {
	if c.active == nil || !c.active.terminalWrote || !c.active.workerDone {
		return
	}
	c.active.opCancel()
	c.active.messageCancel()
	c.active.outcomeCancel()
	if c.active.cancelTimer != nil {
		c.active.cancelTimer.Stop()
	}
	c.active = nil
}

func (c *coordinator) beginGraceful(result error, joinTimeout time.Duration) {
	if c.shutdown != nil {
		return
	}
	var timer *time.Timer
	if c.active == nil || !c.active.effectInFlight() {
		timer = time.NewTimer(joinTimeout)
	}
	c.shutdown = &shutdownState{mode: shutdownGraceful, result: result, timer: timer}
	c.inputClosed = true
	c.held = nil
	c.heldBusy = false
	c.closeInput()
	if c.active != nil && !c.active.committed && !c.active.effectInFlight() && !c.active.abandoned {
		if c.active.cancelTimer != nil {
			c.active.cancelTimer.Stop()
			c.active.cancelTimer = nil
		}
		c.active.cancelAsked = true
		c.active.opCancel()
		c.active.messageCancel()
	}
}

func (c *coordinator) beginFatal(
	result error,
	kind enginewire.ProtocolErrorKind,
	joinTimeout time.Duration,
) {
	if c.shutdown != nil {
		if c.shutdown.mode != shutdownGraceful {
			return
		}
		if c.shutdown.timer != nil {
			c.shutdown.timer.Stop()
		}
	}
	protected := c.active != nil && c.active.effectInFlight()
	var timer *time.Timer
	if !protected {
		timer = time.NewTimer(joinTimeout)
	}
	c.shutdown = &shutdownState{
		mode: shutdownFatal, result: result, protocolKind: kind,
		bootstrapFatal: c.phase != phaseRunning, timer: timer,
	}
	c.inputClosed = true
	c.held = nil
	c.heldBusy = false
	c.pending = nil
	c.closeInput()
	if c.active != nil {
		if c.active.cancelTimer != nil {
			c.active.cancelTimer.Stop()
			c.active.cancelTimer = nil
		}
		c.active.messageCancel()
		c.active.cursor = nil
		if !protected {
			c.active.abandoned = true
			c.active.opCancel()
			c.active.outcomeCancel()
			if !c.active.workerStarted {
				c.active.workerDone = true
			}
		}
	}
}

func (c *coordinator) beginTransport(result error, joinTimeout time.Duration) {
	if c.shutdown != nil && c.shutdown.mode == shutdownTransport {
		return
	}
	protected := c.active != nil && c.active.effectInFlight()
	var timer *time.Timer
	if !protected {
		timer = time.NewTimer(joinTimeout)
	}
	if c.shutdown == nil {
		c.shutdown = &shutdownState{mode: shutdownTransport, result: result, timer: timer}
	} else {
		if c.shutdown.timer != nil {
			c.shutdown.timer.Stop()
		}
		c.shutdown.mode = shutdownTransport
		c.shutdown.result = result
		c.shutdown.timer = timer
	}
	c.inputClosed = true
	c.held = nil
	c.heldBusy = false
	c.pending = nil
	c.closeInput()
	c.closeOutput()
	if c.active != nil {
		if c.active.cancelTimer != nil {
			c.active.cancelTimer.Stop()
			c.active.cancelTimer = nil
		}
		c.active.messageCancel()
		if !protected {
			c.active.abandoned = true
			c.active.opCancel()
			c.active.outcomeCancel()
			if !c.active.workerStarted {
				c.active.workerDone = true
			}
		}
	}
}

func (c *coordinator) shouldFinish() bool {
	if c.shutdown == nil {
		return false
	}
	if c.shutdown.timedOut {
		return true
	}
	workerDone := c.active == nil || c.active.workerDone
	switch c.shutdown.mode {
	case shutdownGraceful:
		return c.active == nil && c.pending == nil && c.inflight == nil
	case shutdownFatal:
		return workerDone && c.shutdown.fatalWritten && c.pending == nil && c.inflight == nil
	case shutdownTransport:
		return workerDone
	default:
		return false
	}
}

func (c *coordinator) waitForGoroutines(timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	decoderDone := c.decoderStopped
	writerDone := c.writerStopped
	workerDone := c.active == nil || c.active.workerDone
	for !decoderDone || !writerDone || !workerDone {
		select {
		case <-c.decoderDone:
			decoderDone = true
		case <-c.writerDone:
			writerDone = true
		case <-activeDone(c.active, workerDone):
			workerDone = true
		case <-deadline.C:
			return ErrJoinTimeout
		}
	}
	return nil
}

func activeDone(active *activeRequest, alreadyDone bool) <-chan struct{} {
	if active == nil || alreadyDone || !active.workerStarted {
		return nil
	}
	return active.done
}

func onceClose(explicit func() error, stream any) func() {
	closeFn := explicit
	if closeFn == nil {
		if closer, ok := stream.(io.Closer); ok && !isNilIO(closer) {
			closeFn = closer.Close
		}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			if closeFn != nil {
				// Close is a best-effort unblock operation. The coordinator's
				// transport/join result remains authoritative.
				_ = closeFn()
			}
		})
	}
}

func isNilIO(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

// ExitCode maps host failures to the candidate process exit contract.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrProtocol) {
		return 2
	}
	return 1
}
