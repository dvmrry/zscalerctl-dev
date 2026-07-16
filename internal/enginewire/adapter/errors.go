package adapter

import (
	"context"
	"errors"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/machine"
)

// ErrorConversion is either a canceled outcome or one static wire failure.
type ErrorConversion struct {
	Canceled bool
	Failure  enginewire.OperationFailure
}

// ToOperationError strips all source text and context from an engine error.
func ToOperationError(source error) ErrorConversion {
	if errors.Is(source, context.Canceled) {
		return ErrorConversion{Canceled: true}
	}
	if errors.Is(source, context.DeadlineExceeded) {
		return ErrorConversion{Failure: enginewire.NonCredentialFailure{Kind: enginewire.FailureDeadlineExceeded}}
	}
	machineError, ok := asMachineError(source)
	if !ok {
		return ErrorConversion{Failure: enginewire.NonCredentialFailure{Kind: enginewire.FailureInternal}}
	}
	if machineError.Kind == machine.ErrorKindCanceled {
		return ErrorConversion{Canceled: true}
	}
	if machineError.Kind == machine.ErrorKindMissingCredentials {
		return ErrorConversion{Failure: enginewire.MissingCredentialsFailure{
			Kind:    machine.ErrorKindMissingCredentials,
			Missing: safeMissingCredentials(machineError.Missing),
		}}
	}
	return ErrorConversion{Failure: enginewire.NonCredentialFailure{Kind: operationFailureKind(machineError.Kind)}}
}

func asMachineError(source error) (machine.MachineError, bool) {
	var pointer *machine.MachineError
	if errors.As(source, &pointer) && pointer != nil {
		copyValue := *pointer
		copyValue.Missing = append([]string(nil), pointer.Missing...)
		return copyValue, true
	}
	var value machine.MachineError
	if errors.As(source, &value) {
		value.Missing = append([]string(nil), value.Missing...)
		return value, true
	}
	return machine.MachineError{}, false
}

func operationFailureKind(kind string) enginewire.OperationFailureKind {
	switch kind {
	case machine.ErrorKindUsage:
		return enginewire.FailureUsage
	case machine.ErrorKindUnsupportedCapability:
		return enginewire.FailureUnsupportedCapability
	case machine.ErrorKindUnsupportedOperation:
		return enginewire.FailureUnsupportedOperation
	case machine.ErrorKindUnknownResource:
		return enginewire.FailureUnknownResource
	case machine.ErrorKindNotFound:
		return enginewire.FailureNotFound
	case machine.ErrorKindInvalidResourceID:
		return enginewire.FailureInvalidResourceID
	case machine.ErrorKindLiveAccessFailed:
		return enginewire.FailureLiveAccess
	case machine.ErrorKindDeadlineExceeded:
		return enginewire.FailureDeadlineExceeded
	case machine.ErrorKindInvalidConfig:
		return enginewire.FailureInvalidConfig
	case machine.ErrorKindInvalidProxyConfig:
		return enginewire.FailureInvalidProxyConfig
	case machine.ErrorKindUnsupportedResource:
		return enginewire.FailureUnsupportedResource
	case machine.ErrorKindInternal:
		return enginewire.FailureInternal
	default:
		return enginewire.FailureInternal
	}
}

func safeMissingCredentials(source []string) []string {
	allowed := map[string]bool{
		"ZSCALERCTL_CLIENT_ID": true, "ZSCALERCTL_CLIENT_SECRET": true,
		"ZSCALERCTL_VANITY_DOMAIN": true, "ZSCALERCTL_ZPA_CUSTOMER_ID": true,
		"ZSCALERCTL_ZIA_USERNAME": true, "ZSCALERCTL_ZIA_PASSWORD": true,
		"ZSCALERCTL_ZIA_API_KEY": true, "ZSCALERCTL_ZIA_CLOUD": true,
	}
	out := make([]string, 0, len(source))
	seen := map[string]bool{}
	for _, name := range source {
		if !allowed[name] || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
