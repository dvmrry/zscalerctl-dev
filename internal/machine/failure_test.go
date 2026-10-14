package machine_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

type classifiedFailure struct {
	reason    string
	status    int
	retryable bool
}

func (f classifiedFailure) Error() string { return "classified failure" }
func (f classifiedFailure) FailureDetail() (string, int, bool) {
	return f.reason, f.status, f.retryable
}

func TestFailureDetailOf(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("read: %w", classifiedFailure{"forbidden", 403, false})
	if got := machine.FailureDetailOf(wrapped); got != (machine.FailureDetail{Reason: "forbidden", Status: 403}) {
		t.Errorf("FailureDetailOf(wrapped classified) = %+v", got)
	}
	deadline := fmt.Errorf("zscaler list zia/users timed out: %w", context.DeadlineExceeded)
	if got := machine.FailureDetailOf(deadline); got != (machine.FailureDetail{Reason: machine.FailureReasonTimeout, Retryable: true}) {
		t.Errorf("FailureDetailOf(deadline) = %+v, want retryable timeout", got)
	}
	if got := machine.FailureDetailOf(errors.New("plain")); got != (machine.FailureDetail{}) {
		t.Errorf("FailureDetailOf(plain) = %+v, want zero", got)
	}
	for _, tc := range []struct {
		detail machine.FailureDetail
		want   string
	}{
		{machine.FailureDetail{Reason: "forbidden", Status: 403}, "resource read failed: forbidden (HTTP 403)"},
		{machine.FailureDetail{Reason: "pagination_incomplete"}, "resource read failed: pagination_incomplete"},
		{machine.FailureDetail{}, "resource read failed"},
	} {
		if got := tc.detail.Describe("resource read failed"); got != tc.want {
			t.Errorf("Describe(%+v) = %q, want %q", tc.detail, got, tc.want)
		}
	}
}

// TestExecutorLiveFailureCarriesDetail keeps the cause of a failed read on the
// machine error: the message names the reason and status, and Detail carries
// them for the CLI envelope, instead of a bare "resource read failed".
func TestExecutorLiveFailureCarriesDetail(t *testing.T) {
	t.Parallel()

	loader := &fakeBrowserLoader{err: fmt.Errorf("load: %w", classifiedFailure{"rate_limited", 429, true})}
	executor := machine.Executor{
		Browser: loader,
		Catalog: resources.ResourceCatalog{
			testExecutorSpec(resources.ProductZIA, "locations", resources.ReadOperations(), "id"),
		},
	}
	_, err := executor.Execute(context.Background(), machine.Request{
		RequestID:  "req-live-failure",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) {
		t.Fatalf("Execute error = %v, want MachineError", err)
	}
	if machineErr.Kind != machine.ErrorKindLiveAccessFailed {
		t.Errorf("Kind = %q, want %q", machineErr.Kind, machine.ErrorKindLiveAccessFailed)
	}
	if want := (machine.FailureDetail{Reason: "rate_limited", Status: 429, Retryable: true}); machineErr.Detail != want {
		t.Errorf("Detail = %+v, want %+v", machineErr.Detail, want)
	}
	if want := "resource read failed: rate_limited (HTTP 429)"; machineErr.Message != want {
		t.Errorf("Message = %q, want %q", machineErr.Message, want)
	}
}
