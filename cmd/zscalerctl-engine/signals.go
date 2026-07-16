package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
)

type signalController struct {
	ctx     context.Context
	cancel  context.CancelFunc
	signals chan os.Signal
	done    chan struct{}
	stopped chan struct{}
	force   func(int)
	code    atomic.Int32
	stop    sync.Once
}

func newSignalController(parent context.Context, forceExit func(int)) *signalController {
	ctx, cancel := context.WithCancel(parent)
	controller := &signalController{
		ctx: ctx, cancel: cancel,
		// Capacity two preserves the first graceful signal and the one allowed
		// force signal without an arbitrary event queue.
		signals: make(chan os.Signal, 2), done: make(chan struct{}), stopped: make(chan struct{}),
		force: forceExit,
	}
	signal.Notify(controller.signals, hostSignals()...)
	go controller.run()
	return controller
}

func (c *signalController) run() {
	defer close(c.stopped)
	select {
	case <-c.done:
		return
	case received := <-c.signals:
		code := signalExitCode(received)
		c.code.Store(code)
		c.cancel()
	}
	select {
	case <-c.done:
		return
	case <-c.signals:
		c.force(c.ExitCode())
	}
}

func (c *signalController) Context() context.Context { return c.ctx }

func (c *signalController) ExitCode() int { return int(c.code.Load()) }

func (c *signalController) Stop() {
	c.stop.Do(func() {
		signal.Stop(c.signals)
		close(c.done)
		c.cancel()
		<-c.stopped
	})
}
