package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
)

// The hint covers a failure to connect as well as a timeout, and is added once.
func TestWithProxyHintCoversTransportFailuresOnce(t *testing.T) {
	t.Parallel()
	environ := []string{"HTTPS_PROXY=http://proxy.example.invalid:8080"}
	transport := &machine.MachineError{Kind: machine.ErrorKindLiveAccessFailed, Message: "zscaler list zia/url-filtering-rules failed",
		Detail: machine.FailureDetail{Reason: machine.FailureReasonTransport, Retryable: true}}
	forbidden := &machine.MachineError{Kind: machine.ErrorKindLiveAccessFailed, Message: "zscaler list zia/url-filtering-rules failed",
		Detail: machine.FailureDetail{Reason: machine.FailureReasonForbidden, Status: 403}}

	err := withProxyHint(withProxyHint(transport, environ), environ)
	if got := strings.Count(err.Error(), config.IgnoredProxyHint); got != 1 {
		t.Errorf("transport error %q carries the hint %d times, want once", err, got)
	}
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr != transport {
		t.Errorf("hinted error does not unwrap to the original MachineError")
	}
	if err := withProxyHint(forbidden, environ); strings.Contains(err.Error(), config.IgnoredProxyHint) {
		t.Errorf("forbidden error %q carries the proxy hint", err)
	}
	// An unparseable ZSCALERCTL_PROXY_FROM_ENV is not "ignored": a real run
	// rejects it as invalid_config before any read, so no hint is claimed.
	bogus := append(environ, config.EnvProxyFromEnv+"=sometimes")
	if err := withProxyHint(transport, bogus); strings.Contains(err.Error(), config.IgnoredProxyHint) {
		t.Errorf("error %q carries the proxy hint with an unparseable %s", err, config.EnvProxyFromEnv)
	}
}
