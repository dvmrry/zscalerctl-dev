package machine

import (
	"context"
	"errors"
	"fmt"
)

// FailureReasonTimeout is the failure reason for a read that hit its deadline.
// The live reader supplies every other reason (see zscaler.Failure*).
const FailureReasonTimeout = "timeout"

// FailureDetail says why a live read failed in value-free terms: a reason from
// a closed vocabulary, the HTTP status when there was a response, and whether
// a retry could plausibly succeed. The zero value means "not known".
type FailureDetail struct {
	Reason    string
	Status    int
	Retryable bool
}

// failureDetailer is implemented by live-read errors that classified their own
// failure. It is an interface so this package does not depend on the reader.
type failureDetailer interface {
	FailureDetail() (reason string, status int, retryable bool)
}

// FailureDetailOf extracts the failure detail from an error chain. A deadline
// is a timeout even when the reader wrapped it, because a longer --timeout or
// a retry can succeed.
func FailureDetailOf(err error) FailureDetail {
	if err == nil {
		return FailureDetail{}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return FailureDetail{Reason: FailureReasonTimeout, Retryable: true}
	}
	var detailer failureDetailer
	if errors.As(err, &detailer) {
		reason, status, retryable := detailer.FailureDetail()
		return FailureDetail{Reason: reason, Status: status, Retryable: retryable}
	}
	return FailureDetail{}
}

// AttachFailure records why a live read failed on a machine error built from
// it. Every site that turns a reader error into a live_access_failed or
// deadline_exceeded MachineError calls this while it still holds the original
// error, because the error it builds keeps only a sanitized sentinel cause.
// Other kinds are left unchanged.
func (e *MachineError) AttachFailure(err error) {
	if e.Kind != ErrorKindLiveAccessFailed && e.Kind != ErrorKindDeadlineExceeded {
		return
	}
	if e.Detail.Reason != "" {
		return
	}
	e.Detail = FailureDetailOf(err)
	if e.Kind == ErrorKindLiveAccessFailed {
		e.Message = e.Detail.Describe(e.Message)
	}
}

// Describe appends the reason and status to a fixed message, for example
// "resource read failed: forbidden (HTTP 403)".
func (d FailureDetail) Describe(message string) string {
	switch {
	case d.Reason == "":
		return message
	case d.Status > 0:
		return fmt.Sprintf("%s: %s (HTTP %d)", message, d.Reason, d.Status)
	default:
		return fmt.Sprintf("%s: %s", message, d.Reason)
	}
}
