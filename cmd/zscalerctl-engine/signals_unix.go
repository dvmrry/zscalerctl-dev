//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func hostSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}

func signalExitCode(value os.Signal) int32 {
	if value == syscall.SIGTERM {
		return 143
	}
	return 130
}

func configureBrokenPipe() {
	signal.Ignore(syscall.SIGPIPE)
}
