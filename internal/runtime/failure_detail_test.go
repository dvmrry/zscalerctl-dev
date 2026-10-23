package runtime

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

type classifiedLiveError struct{}

func (classifiedLiveError) Error() string {
	return "zscaler API request failed: list zia/locations (status 403)"
}
func (classifiedLiveError) Unwrap() error { return zscaler.ErrLiveAccessFailed }
func (classifiedLiveError) FailureDetail() (string, int, bool) {
	return zscaler.FailureForbidden, 403, false
}

// TestRuntimeBoundariesKeepFailureDetail covers the construction sites that
// rebuild a reader error into a MachineError from a sanitized sentinel: each
// must keep why the read failed.
func TestRuntimeBoundariesKeepFailureDetail(t *testing.T) {
	t.Parallel()

	source := fmt.Errorf("read: %w", classifiedLiveError{})
	want := machine.FailureDetail{Reason: zscaler.FailureForbidden, Status: 403}
	for name, err := range map[string]error{
		"resource read": resourceReadRuntimeBoundaryError(source, machine.ResourceReadRequest{
			Operation: machine.OperationList,
			Input:     machine.ResourceReadInput{Product: "zia", Resource: "locations"},
		}),
		"dump runtime": dumpRuntimeBoundaryError(source),
		"url lookup":   urlLookupBoundaryError(source),
	} {
		var machineErr *machine.MachineError
		if !errors.As(err, &machineErr) {
			t.Fatalf("%s: error = %v, want MachineError", name, err)
		}
		if machineErr.Detail != want {
			t.Errorf("%s: Detail = %+v, want %+v", name, machineErr.Detail, want)
		}
		if got := machine.FailureDetailOf(err); got != want {
			t.Errorf("%s: FailureDetailOf = %+v, want %+v", name, got, want)
		}
	}
	dumpErr := dumpMachineError(source, machine.OperationList, "zia", "locations", machine.ErrorKindLiveAccessFailed)
	if dumpErr.Detail != want || dumpErr.Message != "resource read failed: forbidden (HTTP 403)" {
		t.Errorf("dumpMachineError = %+v, want detail %+v and a described message", dumpErr, want)
	}
	record := withFailure(dumpResourceErrorForTest(), source)
	if record.Reason != zscaler.FailureForbidden || record.Status != 403 || record.Retryable {
		t.Errorf("withFailure = %+v, want forbidden/403", record)
	}
}

func dumpResourceErrorForTest() dump.ResourceError {
	return dump.NewResourceError(resources.ProductZIA, "locations", "list", "list_failed")
}
