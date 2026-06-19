package keyring

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	maxKeyringStdout = 64 * 1024
	maxKeyringStderr = 16 * 1024
)

// runKeyringCmd runs argv without a shell. Non-zero process exits are reported
// through exitCode with nil error; err is reserved for start failures, timeouts,
// cancellation, and bounded-output failures. It never includes stdout/stderr
// content in returned errors.
func runKeyringCmd(ctx context.Context, timeout time.Duration, argv []string) (string, string, int, error) {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return "", "", -1, errors.New("keyring: empty command")
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// #nosec G204 -- argv is a fixed OS keychain helper with service/key passed
	// as separate arguments and no shell.
	cmd := exec.CommandContext(cctx, argv[0], argv[1:]...)
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = filterEnv(os.Environ())

	stdout := &cappedWriter{limit: maxKeyringStdout}
	stderr := &cappedWriter{limit: maxKeyringStderr, truncate: true}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	runErr := cmd.Run()
	if stdout.err != nil {
		return "", "", -1, fmt.Errorf("keyring: %q stdout too large", argv[0])
	}

	out := strings.TrimRight(stdout.String(), "\r\n")
	errOut := stderr.String()
	if runErr != nil {
		if errors.Is(cctx.Err(), context.DeadlineExceeded) {
			return "", "", -1, fmt.Errorf("keyring: %q timed out after %s (keychain may be locked or require interaction); use env:/file:/cmd: %w (%w)", argv[0], timeout, context.DeadlineExceeded, ErrUnavailable)
		}
		if cctx.Err() != nil {
			return "", "", -1, cctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return out, errOut, exitErr.ExitCode(), nil
		}
		return "", "", -1, fmt.Errorf("keyring: %q could not run: %w (%w)", argv[0], runErr, ErrUnavailable)
	}
	return out, errOut, 0, nil
}

func filterEnv(environ []string) []string {
	var filtered []string
	for _, env := range environ {
		if !strings.HasPrefix(env, "ZSCALERCTL_") {
			filtered = append(filtered, env)
		}
	}
	return filtered
}

type cappedWriter struct {
	buf      bytes.Buffer
	limit    int
	truncate bool
	err      error
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.buf.Len()+len(p) > w.limit {
		if w.truncate {
			if room := w.limit - w.buf.Len(); room > 0 {
				_, _ = w.buf.Write(p[:room])
			}
			return len(p), nil
		}
		w.err = errors.New("output limit exceeded")
		return 0, w.err
	}
	return w.buf.Write(p)
}

func (w *cappedWriter) String() string {
	return w.buf.String()
}
