package enginehost

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/machine"
)

var errTestWriter = errors.New("test writer failed")

type prefixErrorWriter struct {
	written int
}

func (w *prefixErrorWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, errTestWriter
	}
	w.written++
	return 1, errTestWriter
}

func TestHostTreatsPartialAndZeroWritesAsBrokenTransport(t *testing.T) {
	t.Parallel()

	engine := &fakeEngine{manifest: machine.EngineManifestFromCatalog(nil)}
	host, err := New(engine, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	host.joinTimeout = time.Second
	writer := &prefixErrorWriter{}
	err = host.Serve(context.Background(), Streams{Input: bytes.NewReader(nil), Output: writer})
	if !errors.Is(err, ErrTransport) || writer.written != 1 {
		t.Fatalf("Host.Serve(partial writer) = %v, writes=%d", err, writer.written)
	}

	host, err = New(engine, "test")
	if err != nil {
		t.Fatalf("New(second) error = %v", err)
	}
	err = host.Serve(context.Background(), Streams{Input: bytes.NewReader(nil), Output: zeroWriter{}})
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("Host.Serve(zero writer) = %v, want transport failure", err)
	}
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

type blockingWriter struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	closed  atomic.Int32
}

func newBlockingWriter() *blockingWriter {
	return &blockingWriter{entered: make(chan struct{}), release: make(chan struct{})}
}

func (w *blockingWriter) Write([]byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return 0, io.ErrClosedPipe
}

func (w *blockingWriter) Close() error {
	if w.closed.Add(1) == 1 {
		close(w.release)
	}
	return nil
}

func TestHostJoinTimeoutClosesBlockedOutput(t *testing.T) {
	t.Parallel()

	engine := &fakeEngine{manifest: machine.EngineManifestFromCatalog(nil)}
	host, err := New(engine, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	host.joinTimeout = 20 * time.Millisecond
	writer := newBlockingWriter()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- host.Serve(ctx, Streams{
			Input: bytes.NewReader(nil), Output: writer, CloseOutput: writer.Close,
		})
	}()
	select {
	case <-writer.entered:
	case <-time.After(time.Second):
		t.Fatal("writer did not block")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, ErrJoinTimeout) {
			t.Fatalf("Host.Serve(blocked writer) error = %v, want join timeout", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Host.Serve(blocked writer) did not return")
	}
	if writer.closed.Load() != 1 {
		t.Fatalf("blocked writer close calls = %d, want 1", writer.closed.Load())
	}
}
