package machine

import dumpdiff "github.com/dvmrry/zscalerctl/internal/diff"

// DiffResourceSelector identifies one exact catalog resource. Adapters may
// accept shorthand, but must resolve it before constructing this request.
type DiffResourceSelector struct {
	Product  string
	Resource string
}

// DiffRequest selects two existing local dump artifacts and the catalog-backed
// comparison policy. It carries no config, credentials, renderer state, or
// process-exit policy.
type DiffRequest struct {
	RequestID         string
	OldDir            string
	NewDir            string
	Products          []string
	Resources         []DiffResourceSelector
	IgnoreOperational bool
	AllowPartial      bool
}

// MarshalJSON rejects direct DiffRequest serialization.
func (DiffRequest) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct DiffRequest deserialization.
func (*DiffRequest) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}

// DiffResult owns a recursively copied, admitted diff report. Report returns a
// fresh copy so an adapter cannot mutate engine-owned result state.
type DiffResult struct {
	report dumpdiff.Report
}

// NewDiffResult constructs a closed result from an already-admitted report.
func NewDiffResult(report dumpdiff.Report) DiffResult {
	return DiffResult{report: dumpdiff.CloneReport(report)}
}

// Report returns a recursively defensive copy of the admitted report.
func (r DiffResult) Report() dumpdiff.Report {
	return dumpdiff.CloneReport(r.report)
}

// HasDrift reports whether the admitted report contains configuration drift.
func (r DiffResult) HasDrift() bool { return r.report.HasDrift() }

// MarshalJSON rejects direct DiffResult serialization.
func (DiffResult) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct DiffResult deserialization.
func (*DiffResult) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}
