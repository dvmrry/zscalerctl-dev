package cli

import (
	"errors"
	"fmt"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

var ErrUsage = errors.New("usage error")

var ErrPartialDump = errors.New("partial dump")

var ErrNotFound = errors.New("not found")

var ErrDriftDetected = errors.New("drift detected")

type UsageError struct {
	Message string
}

func (e UsageError) Error() string {
	return e.Message
}

func (e UsageError) Unwrap() error {
	return ErrUsage
}

type PartialDumpError struct {
	Dir    string
	Errors int
}

func (e PartialDumpError) Error() string {
	return fmt.Sprintf("partial dump written: %s (%d errors; see errors.ndjson)", e.Dir, e.Errors)
}

func (e PartialDumpError) Unwrap() error {
	return ErrPartialDump
}

type DriftDetectedError struct{}

func (e DriftDetectedError) Error() string {
	return "drift detected"
}

func (e DriftDetectedError) Unwrap() error {
	return ErrDriftDetected
}

type ResourceNotFoundError struct {
	Product  resources.Product
	Resource string
}

func (e ResourceNotFoundError) Error() string {
	// Point the caller (human or agent) at the two enumeration paths instead
	// of leaving them to guess names.
	return fmt.Sprintf("unsupported resource %s/%s; run \"zscalerctl %s --help\" or \"zscalerctl --format json schema list\" to enumerate resources", e.Product, e.Resource, e.Product)
}

func (e ResourceNotFoundError) Unwrap() error {
	return ErrNotFound
}

func cliErrorFromMachineRead(err error) error {
	if err == nil {
		return nil
	}
	machineErr, ok := asMachineError(err)
	if !ok {
		return err
	}
	switch machineErr.Kind {
	case machine.ErrorKindUsage:
		return UsageError{Message: machineErr.Error()}
	default:
		return err
	}
}

// ExitCodeForMachineErrorKind is the single supported mapping from stable
// machine error kinds to process exit codes. The values mirror the public CLI
// contract documented by introspection and docs/VERSIONING.md.
func ExitCodeForMachineErrorKind(kind string) (int, bool) {
	switch kind {
	case machine.ErrorKindInternal, machine.ErrorKindCanceled:
		return 1, true
	case machine.ErrorKindUsage,
		machine.ErrorKindInvalidResourceID,
		machine.ErrorKindInvalidConfig,
		machine.ErrorKindInvalidProxyConfig:
		return 2, true
	case machine.ErrorKindMissingCredentials:
		return 3, true
	case machine.ErrorKindUnsupportedCapability,
		machine.ErrorKindUnsupportedOperation,
		machine.ErrorKindUnknownResource,
		machine.ErrorKindUnsupportedResource,
		machine.ErrorKindNotFound:
		return 4, true
	case machine.ErrorKindLiveAccessFailed, machine.ErrorKindDeadlineExceeded:
		return 5, true
	default:
		return 0, false
	}
}

func asMachineError(err error) (*machine.MachineError, bool) {
	var machineErr *machine.MachineError
	if errors.As(err, &machineErr) {
		return machineErr, true
	}
	var machineErrValue machine.MachineError
	if errors.As(err, &machineErrValue) {
		return &machineErrValue, true
	}
	return nil, false
}
