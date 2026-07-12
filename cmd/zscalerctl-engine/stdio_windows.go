//go:build windows

package main

import "os"

func processStreams(
	stdin *os.File,
	stdout *os.File,
) (interruptibleInput, interruptibleOutput, error) {
	return interruptibleInput{File: stdin}, interruptibleOutput{File: stdout}, nil
}
