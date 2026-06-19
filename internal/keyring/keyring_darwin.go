//go:build darwin

package keyring

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	errSecItemNotFoundExit          = 44
	errSecInteractionNotAllowedExit = 36
)

type macOSGetter struct {
	run     runnerFunc
	timeout time.Duration
}

func newBackend() Getter {
	return &macOSGetter{run: runKeyringCmd, timeout: defaultKeyringTimeout}
}

func (g *macOSGetter) Get(ctx context.Context, service, key string) (string, error) {
	argv := []string{"/usr/bin/security", "find-generic-password", "-s", service, "-a", key, "-w"}
	stdout, _, code, err := g.run(ctx, g.timeout, argv)
	if err != nil {
		return "", err
	}
	switch code {
	case 0:
		if stdout == "" {
			return "", fmt.Errorf("keyring: item has no value")
		}
		value, err := g.normalizeSecurityPassword(ctx, service, key, stdout)
		if err != nil {
			return "", err
		}
		return value, nil
	case errSecItemNotFoundExit:
		return "", ErrNotFound
	case errSecInteractionNotAllowedExit:
		return "", fmt.Errorf("keyring: macOS Keychain is locked or requires interaction; unlock it or use env:/file:/cmd: (%w)", ErrUnavailable)
	default:
		return "", fmt.Errorf("keyring: /usr/bin/security failed (exit %d)", code)
	}
}

func (g *macOSGetter) normalizeSecurityPassword(ctx context.Context, service, key, stdout string) (string, error) {
	if !isHexString(stdout) {
		return stdout, nil
	}

	// security(1) prints some non-ASCII generic-password values as hex with -w.
	// The -g form disambiguates that from a literal hex-looking password:
	// quoted strings are literal, while 0x... is encoded password bytes.
	argv := []string{"/usr/bin/security", "find-generic-password", "-s", service, "-a", key, "-g"}
	_, stderr, code, err := g.run(ctx, g.timeout, argv)
	if err != nil {
		return "", err
	}
	switch code {
	case 0:
		if decoded, ok, err := decodeSecurityHexPassword(stdout, stderr); err != nil {
			return "", err
		} else if ok {
			return decoded, nil
		}
		return "", fmt.Errorf("keyring: /usr/bin/security returned an unrecognized password format")
	case errSecItemNotFoundExit:
		return "", ErrNotFound
	case errSecInteractionNotAllowedExit:
		return "", fmt.Errorf("keyring: macOS Keychain is locked or requires interaction; unlock it or use env:/file:/cmd: (%w)", ErrUnavailable)
	default:
		return "", fmt.Errorf("keyring: /usr/bin/security failed (exit %d)", code)
	}
}

func isHexString(s string) bool {
	if s == "" || len(s)%2 != 0 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func decodeSecurityHexPassword(wOutput, gStderr string) (string, bool, error) {
	for _, line := range strings.Split(gStderr, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "password: 0x"):
			fields := strings.Fields(strings.TrimPrefix(line, "password: 0x"))
			if len(fields) == 0 || !strings.EqualFold(fields[0], wOutput) {
				return "", false, fmt.Errorf("keyring: /usr/bin/security returned inconsistent password encoding")
			}
			b, err := hex.DecodeString(fields[0])
			if err != nil {
				return "", false, fmt.Errorf("keyring: /usr/bin/security returned invalid password encoding")
			}
			return string(b), true, nil
		case strings.HasPrefix(line, "password: \""):
			return wOutput, true, nil
		default:
			continue
		}
	}
	return "", false, nil
}
