//go:build darwin

package keyring

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func newDarwinWithRunner(r runnerFunc) *macOSGetter {
	return &macOSGetter{run: r, timeout: time.Second}
}

func TestDarwinGetNotFoundExit44(t *testing.T) {
	g := newDarwinWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "", "security: item could not be found", errSecItemNotFoundExit, nil
	})
	if _, err := g.Get(context.Background(), "svc", "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("macOS Get(exit 44) error = %v, want ErrNotFound", err)
	}
}

func TestDarwinGetInteractionNotAllowedUnavailable(t *testing.T) {
	g := newDarwinWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "", "User interaction is not allowed.", errSecInteractionNotAllowedExit, nil
	})
	if _, err := g.Get(context.Background(), "svc", "k"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("macOS Get(exit 36) error = %v, want ErrUnavailable", err)
	}
}

func TestDarwinGetSuccess(t *testing.T) {
	g := newDarwinWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "s3cr3t", "", 0, nil
	})
	got, err := g.Get(context.Background(), "svc", "k")
	if err != nil || got != "s3cr3t" {
		t.Fatalf("macOS Get(success) = %q, %v; want s3cr3t, nil", got, err)
	}
}

func TestDarwinGetDecodesSecurityHexPassword(t *testing.T) {
	var calls [][]string
	g := newDarwinWithRunner(func(_ context.Context, _ time.Duration, argv []string) (string, string, int, error) {
		calls = append(calls, append([]string(nil), argv...))
		if len(calls) == 1 {
			return "c3a9c3a9", "", 0, nil
		}
		return "", `password: 0xC3A9C3A9  "\303\251\303\251"`, 0, nil
	})
	got, err := g.Get(context.Background(), "svc", "k")
	if err != nil || got != "éé" {
		t.Fatalf("macOS Get(hex password) = %q, %v; want éé, nil", got, err)
	}
	if len(calls) != 2 || calls[1][len(calls[1])-1] != "-g" {
		t.Fatalf("macOS Get(hex password) calls = %v, want second security call with -g", calls)
	}
}

func TestDarwinGetPreservesLiteralHexPassword(t *testing.T) {
	var calls int
	g := newDarwinWithRunner(func(_ context.Context, _ time.Duration, _ []string) (string, string, int, error) {
		calls++
		if calls == 1 {
			return "68656c6c6f", "", 0, nil
		}
		return "", `password: "68656c6c6f"`, 0, nil
	})
	got, err := g.Get(context.Background(), "svc", "k")
	if err != nil || got != "68656c6c6f" {
		t.Fatalf("macOS Get(literal hex password) = %q, %v; want 68656c6c6f, nil", got, err)
	}
	if calls != 2 {
		t.Fatalf("macOS Get(literal hex password) calls = %d, want 2", calls)
	}
}

func TestDarwinGetHexPasswordErrorsOnUnrecognizedSecurityFormat(t *testing.T) {
	var calls int
	g := newDarwinWithRunner(func(_ context.Context, _ time.Duration, _ []string) (string, string, int, error) {
		calls++
		if calls == 1 {
			return "c3a9c3a9", "", 0, nil
		}
		return "", "password: <redacted by future security output>", 0, nil
	})
	got, err := g.Get(context.Background(), "svc", "k")
	if err == nil || got != "" {
		t.Fatalf("macOS Get(unrecognized hex format) = %q, %v; want empty value and error", got, err)
	}
	if strings.Contains(err.Error(), "c3a9") || strings.Contains(err.Error(), "redacted") {
		t.Fatalf("macOS Get(unrecognized hex format) error = %v, want value-free error", err)
	}
}

// twoCallRunner returns a hex -w output on the first call (so Get makes the -g
// disambiguation call), then the given result on the second (-g) call.
func twoCallRunner(wHex, gStderr string, gCode int, gErr error) runnerFunc {
	var calls int
	return func(context.Context, time.Duration, []string) (string, string, int, error) {
		calls++
		if calls == 1 {
			return wHex, "", 0, nil
		}
		return "", gStderr, gCode, gErr
	}
}

func TestDarwinGetHexPasswordSecondCallNotFound(t *testing.T) {
	// Item vanished between -w and -g (TOCTOU): surface ErrNotFound, not a value.
	g := newDarwinWithRunner(twoCallRunner("c3a9c3a9", "", errSecItemNotFoundExit, nil))
	if _, err := g.Get(context.Background(), "svc", "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("-g exit 44 must map to ErrNotFound, got %v", err)
	}
}

func TestDarwinGetHexPasswordSecondCallLocked(t *testing.T) {
	g := newDarwinWithRunner(twoCallRunner("c3a9c3a9", "", errSecInteractionNotAllowedExit, nil))
	if _, err := g.Get(context.Background(), "svc", "k"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("-g exit 36 must map to ErrUnavailable, got %v", err)
	}
}

func TestDarwinGetHexPasswordSecondCallRunError(t *testing.T) {
	g := newDarwinWithRunner(twoCallRunner("c3a9c3a9", "", -1, ErrUnavailable))
	if _, err := g.Get(context.Background(), "svc", "k"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("-g run error must propagate, got %v", err)
	}
}

func TestDarwinGetEmptyValueErrors(t *testing.T) {
	g := newDarwinWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "", "", 0, nil
	})
	if _, err := g.Get(context.Background(), "svc", "k"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("macOS Get(empty) error = %v, want non-not-found error", err)
	}
}

func TestDarwinGetLive(t *testing.T) {
	if os.Getenv("ZSCALERCTL_KEYRING_LIVE") == "" {
		t.Skip("set ZSCALERCTL_KEYRING_LIVE=1 to run against the real Keychain")
	}
	const svc, acct, want = "zscalerctl-livetest", "k", "naïve-pä$$wörd"
	if out, err := exec.Command("/usr/bin/security", "add-generic-password", "-s", svc, "-a", acct, "-w", want, "-U").CombinedOutput(); err != nil {
		t.Fatalf("seed keychain item failed: %v: %s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("/usr/bin/security", "delete-generic-password", "-s", svc, "-a", acct).Run()
	})
	got, err := New().Get(context.Background(), svc, acct)
	if err != nil || got != want {
		t.Fatalf("Keychain Get(live) = %q, %v; want %q, nil", got, err, want)
	}
}
