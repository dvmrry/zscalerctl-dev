//go:build linux

package keyring

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func newLinuxWithRunner(r runnerFunc) *linuxGetter {
	return &linuxGetter{
		run:      r,
		timeout:  time.Second,
		lookPath: func(string) (string, error) { return "/usr/bin/secret-tool", nil },
	}
}

func TestLinuxGetFound(t *testing.T) {
	g := newLinuxWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "s3cr3t", "", 0, nil
	})
	got, err := g.Get(context.Background(), "svc", "k")
	if err != nil || got != "s3cr3t" {
		t.Fatalf("Linux Get(success) = %q, %v; want s3cr3t, nil", got, err)
	}
}

func TestLinuxGetNotFound(t *testing.T) {
	g := newLinuxWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "", "No matching secrets", 1, nil
	})
	if _, err := g.Get(context.Background(), "svc", "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Linux Get(not found) error = %v, want ErrNotFound", err)
	}
}

func TestLinuxGetServiceUnavailable(t *testing.T) {
	g := newLinuxWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "", "Failed to connect to the D-Bus session bus", 1, nil
	})
	if _, err := g.Get(context.Background(), "svc", "k"); err == nil || errors.Is(err, ErrNotFound) || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Linux Get(D-Bus failure) error = %v, want ErrUnavailable not ErrNotFound", err)
	}
}

func TestLinuxGetToolMissing(t *testing.T) {
	g := &linuxGetter{
		timeout:  time.Second,
		lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
	}
	_, err := g.Get(context.Background(), "svc", "k")
	if err == nil || errors.Is(err, ErrNotFound) || !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "libsecret-tools") {
		t.Fatalf("Linux Get(missing tool) error = %v, want ErrUnavailable install hint", err)
	}
}
