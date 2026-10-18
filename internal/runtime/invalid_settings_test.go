package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

// TestBoundariesClassifyInvalidSettings pins that every machine boundary
// reports an unusable setting as invalid_config with the reader's fixed
// reason, keeps the settings sentinel, and drops wrapper text.
func TestBoundariesClassifyInvalidSettings(t *testing.T) {
	t.Parallel()

	reason := zscaler.ValidateSettings(zscaler.Settings{Cloud: "zscalertwo"})
	if !errors.Is(reason, zscaler.ErrInvalidSettings) {
		t.Fatalf("ValidateSettings(product cloud) = %v, want ErrInvalidSettings", reason)
	}
	err := fmt.Errorf("wrapper-canary: %w", reason)
	for name, got := range map[string]error{
		"resource read": resourceReadRuntimeBoundaryError(err, machine.ResourceReadRequest{Operation: machine.OperationList}),
		"dump":          dumpRuntimeBoundaryError(err),
		"url lookup":    urlLookupBoundaryError(err),
	} {
		var machineErr *machine.MachineError
		if !errors.As(got, &machineErr) || machineErr.Kind != machine.ErrorKindInvalidConfig {
			t.Errorf("%s boundary error = %T %v, want invalid_config MachineError", name, got, got)
			continue
		}
		if machineErr.Message != reason.Error() {
			t.Errorf("%s boundary message = %q, want %q", name, machineErr.Message, reason.Error())
		}
		if !errors.Is(got, zscaler.ErrInvalidSettings) || errors.Is(got, zscaler.ErrMissingCredentials) {
			t.Errorf("%s boundary error = %v, want only the settings sentinel", name, got)
		}
		if strings.Contains(got.Error(), "wrapper-canary") {
			t.Errorf("%s boundary error kept wrapper text: %v", name, got)
		}
	}
	dumpErr := dumpMachineError(err, machine.OperationList, "zia", "locations", machine.ErrorKindLiveAccessFailed)
	if dumpErr.Kind != machine.ErrorKindInvalidConfig || dumpErr.Message != reason.Error() {
		t.Errorf("dumpMachineError = %#v, want invalid_config with the fixed reason", dumpErr)
	}
	if safe := dumpSafeCollectionSentinel(err, ""); !errors.Is(safe, zscaler.ErrInvalidSettings) {
		t.Errorf("dumpSafeCollectionSentinel = %v, want the settings sentinel", safe)
	}
}
