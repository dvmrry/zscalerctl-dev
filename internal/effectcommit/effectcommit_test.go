package effectcommit

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestRunUsesOperationScopedRunner(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("effect failed")
	var events []string
	ctx := WithRunner(context.Background(), func(effect func() error) error {
		events = append(events, "begin")
		err := effect()
		events = append(events, "finish")
		return err
	})
	err := Run(ctx, func() error {
		events = append(events, "effect")
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
	if want := []string{"begin", "effect", "finish"}; !reflect.DeepEqual(events, want) {
		t.Errorf("events = %#v, want %#v", events, want)
	}
}

func TestRunWithoutRunnerPreservesSynchronousEffect(t *testing.T) {
	t.Parallel()

	called := false
	if err := Run(context.Background(), func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !called {
		t.Fatal("Run() did not call effect")
	}
}

func TestRunCanceledContextRejectsBeforeRunner(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	ctx = WithRunner(ctx, func(effect func() error) error {
		called = true
		return effect()
	})
	if err := Run(ctx, func() error {
		called = true
		return nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context.Canceled", err)
	}
	if called {
		t.Fatal("Run() called runner or effect after cancellation")
	}
}

func TestRunRejectsNilEffect(t *testing.T) {
	t.Parallel()

	if err := Run(context.Background(), nil); !errors.Is(err, errNilEffect) {
		t.Fatalf("Run() error = %v, want errNilEffect", err)
	}
}
