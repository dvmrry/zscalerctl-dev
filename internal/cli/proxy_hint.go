package cli

import (
	"errors"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
)

// proxyHintError adds config.IgnoredProxyHint to a live read that timed out or could not
// connect. Proxy use is opt-in, so on a network that reaches the API only
// through the proxy every read fails this way, and without the hint only
// doctor says why. It unwraps to the original error, so the kind, reason and
// exit code are unchanged.
type proxyHintError struct{ err error }

func (e proxyHintError) Error() string { return e.err.Error() + " (" + config.IgnoredProxyHint + ")" }
func (e proxyHintError) Unwrap() error { return e.err }

// withProxyHint wraps err with the hint when it is a timeout or transport
// failure and environ sets HTTPS_PROXY that zscalerctl does not use.
func withProxyHint(err error, environ []string) error {
	if err == nil || errors.As(err, new(proxyHintError)) {
		return err
	}
	switch machine.FailureDetailOf(err).Reason {
	case machine.FailureReasonTimeout, machine.FailureReasonTransport:
	default:
		return err
	}
	if !config.EnvironmentProxyIgnored(environ) {
		return err
	}
	return proxyHintError{err: err}
}
