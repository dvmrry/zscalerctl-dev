//go:build windows

package main

import "os"

func hostSignals() []os.Signal { return []os.Signal{os.Interrupt} }

func signalExitCode(os.Signal) int { return 130 }

func configureBrokenPipe() {}
