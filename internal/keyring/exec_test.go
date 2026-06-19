package keyring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunKeyringCmdScrubsZscalerctlEnv(t *testing.T) {
	t.Setenv("GO_KEYRING_HELPER", "1")
	t.Setenv("ZSCALERCTL_CLIENT_SECRET", "leak-me")
	out, _, code, err := runKeyringCmd(context.Background(), 5*time.Second, helperCmd("printenv"))
	if err != nil || code != 0 {
		t.Fatalf("runKeyringCmd(printenv) code=%d err=%v, want 0 nil", code, err)
	}
	if strings.Contains(out, "leak-me") || strings.Contains(out, "ZSCALERCTL_") {
		t.Fatalf("runKeyringCmd(printenv) output leaked ZSCALERCTL env: %q", out)
	}
	if !strings.Contains(out, "GO_KEYRING_HELPER=1") {
		t.Fatalf("runKeyringCmd(printenv) output = %q, want GO_KEYRING_HELPER preserved", out)
	}
}

func TestRunKeyringCmdNonZeroExit(t *testing.T) {
	t.Setenv("GO_KEYRING_HELPER", "1")
	_, _, code, err := runKeyringCmd(context.Background(), 5*time.Second, helperCmd("exit", "7"))
	if err != nil || code != 7 {
		t.Fatalf("runKeyringCmd(exit 7) code=%d err=%v, want 7 nil", code, err)
	}
}

func TestRunKeyringCmdErrorsNeverCarryStderr(t *testing.T) {
	t.Setenv("GO_KEYRING_HELPER", "1")
	// A non-zero exit with a secret on stderr is reported via exitCode with a NIL
	// error; runKeyringCmd hands the raw stderr back to the caller (which must keep
	// it out of its own errors), so no runKeyringCmd error can carry the secret.
	_, stderr, code, err := runKeyringCmd(context.Background(), 5*time.Second, helperCmd("stderr", "TOKEN"))
	if err != nil || code != 1 {
		t.Fatalf("runKeyringCmd(stderr) code=%d err=%v, want 1 nil", code, err)
	}
	if stderr != "TOKEN" {
		t.Fatalf("runKeyringCmd must return raw stderr to the caller, got %q", stderr)
	}
	// runKeyringCmd's own error paths name only the command, never captured output.
	_, _, _, startErr := runKeyringCmd(context.Background(), time.Second, []string{"/nonexistent-zscalerctl-bin"})
	if startErr == nil || strings.Contains(startErr.Error(), "TOKEN") {
		t.Fatalf("start-failure error must not carry captured output: %v", startErr)
	}
}

func TestRunKeyringCmdTimeoutKillsProcess(t *testing.T) {
	t.Setenv("GO_KEYRING_HELPER", "1")
	start := time.Now()
	_, _, _, err := runKeyringCmd(context.Background(), 50*time.Millisecond, helperCmd("sleep"))
	if err == nil {
		t.Fatal("runKeyringCmd(sleep) error = nil, want timeout error")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("runKeyringCmd(sleep) took %s, want bounded timeout", time.Since(start))
	}
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("runKeyringCmd(sleep) error = %v, want DeadlineExceeded and ErrUnavailable", err)
	}
}

func TestRunKeyringCmdStdoutOverflow(t *testing.T) {
	t.Setenv("GO_KEYRING_HELPER", "1")
	_, _, _, err := runKeyringCmd(context.Background(), 5*time.Second, helperCmd("bigout"))
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("runKeyringCmd(bigout) error = %v, want too large", err)
	}
}

func TestRunKeyringCmdStderrOverflowDoesNotFailSuccessfulLookup(t *testing.T) {
	t.Setenv("GO_KEYRING_HELPER", "1")
	out, stderr, code, err := runKeyringCmd(context.Background(), 5*time.Second, helperCmd("bigstderr-ok"))
	if err != nil || code != 0 || out != "ok" {
		t.Fatalf("runKeyringCmd(bigstderr-ok) out=%q code=%d err=%v, want ok 0 nil", out, code, err)
	}
	if len(stderr) != maxKeyringStderr {
		t.Fatalf("runKeyringCmd(bigstderr-ok) stderr length=%d, want capped %d", len(stderr), maxKeyringStderr)
	}
}

func TestRunKeyringCmdEmptyArgv(t *testing.T) {
	if _, _, _, err := runKeyringCmd(context.Background(), time.Second, nil); err == nil {
		t.Fatal("runKeyringCmd(nil argv) error = nil, want error")
	}
}

func TestRunKeyringCmdStartFailureIsUnavailable(t *testing.T) {
	_, _, _, err := runKeyringCmd(context.Background(), time.Second, []string{"/nonexistent-zscalerctl-bin"})
	if err == nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("runKeyringCmd(nonexistent command) error = %v, want ErrUnavailable", err)
	}
}

func helperCmd(args ...string) []string {
	return append([]string{os.Args[0], "-test.run=^TestKeyringHelperProcess$", "--"}, args...)
}

func TestKeyringHelperProcess(t *testing.T) {
	if os.Getenv("GO_KEYRING_HELPER") != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		os.Exit(2)
	}
	switch args[0] {
	case "echo":
		fmt.Fprint(os.Stdout, args[1])
	case "stderr":
		fmt.Fprint(os.Stderr, args[1])
		os.Exit(1)
	case "exit":
		n, _ := strconv.Atoi(args[1])
		os.Exit(n)
	case "sleep":
		time.Sleep(5 * time.Second)
	case "bigout":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 70*1024))
	case "bigstderr-ok":
		fmt.Fprint(os.Stderr, strings.Repeat("x", 70*1024))
		fmt.Fprint(os.Stdout, "ok")
	case "printenv":
		for _, e := range os.Environ() {
			fmt.Fprintln(os.Stdout, e)
		}
	default:
		os.Exit(2)
	}
	os.Exit(0)
}
