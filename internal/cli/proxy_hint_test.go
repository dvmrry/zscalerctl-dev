package cli_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// failingListReader fails every list with err.
type failingListReader struct{ err error }

func (r failingListReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	return nil, r.err
}

func (r failingListReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, r.err
}

func (r failingListReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, r.err
}

// A read that times out while HTTPS_PROXY is set but not used says so in its
// message; the kind, reason and exit code stay those of the timeout, and the
// proxy URL is never echoed.
func TestLiveTimeoutNamesIgnoredEnvironmentProxy(t *testing.T) {
	t.Parallel()
	const proxyURL = "http://proxy-canary.example.invalid:8080"
	for _, test := range []struct {
		name     string
		environ  []string
		err      error
		wantHint bool
	}{
		{"timeout, proxy ignored", []string{"HTTPS_PROXY=" + proxyURL}, context.DeadlineExceeded, true},
		{"timeout, lowercase proxy ignored", []string{"https_proxy=" + proxyURL}, context.DeadlineExceeded, true},
		{"timeout, proxy used", []string{"HTTPS_PROXY=" + proxyURL, config.EnvProxyFromEnv + "=true"}, context.DeadlineExceeded, false},
		{"timeout, explicit proxy", []string{"HTTPS_PROXY=" + proxyURL, config.EnvProxyURL + "=" + proxyURL}, context.DeadlineExceeded, false},
		{"timeout, no proxy", nil, context.DeadlineExceeded, false},
		{"other failure, proxy ignored", []string{"HTTPS_PROXY=" + proxyURL}, errors.New("boom"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			app := cli.NewWithOptions(io.Discard, io.Discard, test.environ, cli.Options{Reader: failingListReader{err: test.err}})
			err := app.Run(context.Background(), []string{"--format", "json", "zia", "url-filtering-rules", "list"})
			if err == nil {
				t.Fatal("App.Run() error = nil, want the read failure")
			}
			if got := strings.Contains(err.Error(), config.IgnoredProxyHint); got != test.wantHint {
				t.Errorf("error %q carries the proxy hint = %v, want %v", err, got, test.wantHint)
			}
			if strings.Contains(err.Error(), "proxy-canary") {
				t.Errorf("error %q echoes the proxy URL", err)
			}
			if !errors.Is(test.err, context.DeadlineExceeded) {
				return
			}
			var machineErr *machine.MachineError
			if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindDeadlineExceeded {
				t.Fatalf("error = %#v, want a deadline_exceeded MachineError", err)
			}
			if got := machine.FailureDetailOf(err); got.Reason != machine.FailureReasonTimeout {
				t.Errorf("FailureDetailOf() = %+v, want reason timeout", got)
			}
			if code, ok := cli.ExitCodeForMachineErrorKind(machineErr.Kind); !ok || code != 5 {
				t.Errorf("exit code = %d, want 5", code)
			}
		})
	}
}
