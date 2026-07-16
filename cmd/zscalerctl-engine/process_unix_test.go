//go:build !windows

package main

import (
	"os"
	"syscall"
	"testing"
)

func TestDuplicateNonblockingFileIsCloseOnExec(t *testing.T) {
	duplicate, err := duplicateNonblockingFile(os.Stdin)
	if err != nil {
		t.Fatalf("duplicateNonblockingFile() error = %v", err)
	}
	t.Cleanup(func() { _ = duplicate.Close() })
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, duplicate.Fd(), syscall.F_GETFD, 0)
	if errno != 0 {
		t.Fatalf("fcntl(F_GETFD) error = %v", errno)
	}
	if flags&syscall.FD_CLOEXEC == 0 {
		t.Errorf("duplicated descriptor flags = %#x, want FD_CLOEXEC", flags)
	}
}

func TestProcessSIGTERMExits143WithoutProtocolNoise(t *testing.T) {
	process := startEngineProcess(t)
	_ = process.initialize(t)
	if err := process.command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("Process.Signal(SIGTERM) error = %v", err)
	}
	if code := process.wait(t); code != 143 {
		t.Errorf("process exit code = %d, want 143", code)
	}
	if process.stderr.Len() != 0 {
		t.Errorf("process stderr = %q, want empty", process.stderr.Bytes())
	}
}
