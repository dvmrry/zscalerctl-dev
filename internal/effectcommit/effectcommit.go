// Package effectcommit carries an internal operation-scoped linearization
// hook from effectful runtimes to transports that arbitrate cancellation.
package effectcommit

import (
	"context"
	"errors"
)

var errNilEffect = errors.New("effect commit callback is nil")

type runnerKey struct{}

// Runner brackets one atomic effect attempt. The transport may reject the
// attempt before it starts, defer concurrent cancellation while it runs, and
// commit or release that cancellation after the attempt returns.
type Runner func(func() error) error

// WithRunner attaches an operation-scoped effect runner to ctx.
func WithRunner(ctx context.Context, runner Runner) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if runner == nil {
		return ctx
	}
	return context.WithValue(ctx, runnerKey{}, runner)
}

// Run executes effect through the operation-scoped runner when one is present.
// Without a transport runner, it preserves the ordinary synchronous behavior.
func Run(ctx context.Context, effect func() error) error {
	if effect == nil {
		return errNilEffect
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	runner, _ := ctx.Value(runnerKey{}).(Runner)
	if runner == nil {
		return effect()
	}
	return runner(effect)
}
