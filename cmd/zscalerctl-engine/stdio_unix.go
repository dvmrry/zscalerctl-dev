//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

func processStreams(
	stdin *os.File,
	stdout *os.File,
) (interruptibleInput, interruptibleOutput, error) {
	input, err := duplicateNonblockingFile(stdin)
	if err != nil {
		return interruptibleInput{}, interruptibleOutput{}, err
	}
	output, err := duplicateNonblockingFile(stdout)
	if err != nil {
		_ = input.Close()
		return interruptibleInput{}, interruptibleOutput{}, err
	}
	return interruptibleInput{File: input}, interruptibleOutput{File: output}, nil
}

func duplicateNonblockingFile(file *os.File) (*os.File, error) {
	if file == nil {
		return nil, fmt.Errorf("nil stdio file")
	}
	fd, err := syscall.Dup(int(file.Fd()))
	if err != nil {
		return nil, fmt.Errorf("duplicate stdio file: %w", err)
	}
	syscall.CloseOnExec(fd)
	if err := syscall.SetNonblock(fd, true); err != nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("make stdio file nonblocking: %w", err)
	}
	duplicate := os.NewFile(uintptr(fd), file.Name())
	if duplicate == nil {
		_ = syscall.Close(fd)
		return nil, fmt.Errorf("construct duplicated stdio file")
	}
	return duplicate, nil
}
