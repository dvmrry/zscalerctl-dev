//go:build linux

package keyring

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type linuxGetter struct {
	run      runnerFunc
	timeout  time.Duration
	lookPath func(string) (string, error)
}

func newBackend() Getter {
	return &linuxGetter{run: runKeyringCmd, timeout: defaultKeyringTimeout, lookPath: exec.LookPath}
}

func (g *linuxGetter) Get(ctx context.Context, service, key string) (string, error) {
	if _, err := g.lookPath("secret-tool"); err != nil {
		return "", fmt.Errorf("keyring: secret-tool not found; install libsecret-tools or use env:/file:/cmd: (%w)", ErrUnavailable)
	}
	argv := []string{"secret-tool", "lookup", "service", service, "account", key}
	stdout, stderr, code, err := g.run(ctx, g.timeout, argv)
	if err != nil {
		return "", err
	}
	if code == 0 {
		if stdout == "" {
			return "", ErrNotFound
		}
		return stdout, nil
	}
	if code == 1 && stdout == "" {
		if looksLikeServiceUnavailable(stderr) {
			return "", fmt.Errorf("keyring: Secret Service unavailable; ensure gnome-keyring or kwallet is running, or use env:/file:/cmd: for headless use (%w)", ErrUnavailable)
		}
		return "", ErrNotFound
	}
	return "", fmt.Errorf("keyring: secret-tool failed (exit %d)", code)
}

func looksLikeServiceUnavailable(stderr string) bool {
	s := strings.ToLower(stderr)
	for _, marker := range []string{"failed to connect", "could not connect", "no such interface", "org.freedesktop.dbus", "gdbus"} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}
