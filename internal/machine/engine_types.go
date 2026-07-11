package machine

import (
	"errors"
	"time"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

var errEngineTypeHasNoWireFormat = errors.New("machine engine type has no wire format")

// ResourceReadInput is the complete typed input for catalog-driven list, get,
// and show operations. It contains no generic option escape hatch.
//
// The JSON tags preserve the existing candidate machine.Request input shape;
// ResourceReadRequest itself is deliberately not a wire type.
type ResourceReadInput struct {
	Product  string   `json:"product,omitempty"`
	Resource string   `json:"resource,omitempty"`
	RecordID string   `json:"record_id,omitempty"`
	Fields   []string `json:"fields,omitempty"`
	Filters  []Filter `json:"filters,omitempty"`
	Search   string   `json:"search,omitempty"`
}

// ResourceReadRequest is the candidate in-process request for one projected
// resource read. The capability is encoded by the Read method, so callers
// cannot select a different capability through a string field.
type ResourceReadRequest struct {
	RequestID string
	Operation Operation
	Input     ResourceReadInput
}

// MarshalJSON rejects direct ResourceReadRequest serialization. Future
// transports must define and version their own DTOs.
func (ResourceReadRequest) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct ResourceReadRequest deserialization. Future
// transports must validate a versioned DTO before constructing this type.
func (*ResourceReadRequest) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}

// ResourceReadResult contains only projected, redacted resource records. Its
// state is private so callers cannot replace the record family with raw maps.
type ResourceReadResult struct {
	records resources.ProjectedRecords
}

// NewResourceReadResult constructs a result from trusted projected records.
// It does not verify records against a catalog spec. Adapters crossing a trust
// boundary must call resources.VerifyProjectedRecords before rendering them.
func NewResourceReadResult(records resources.ProjectedRecords) ResourceReadResult {
	return ResourceReadResult{
		records: resources.NewProjectedRecords(records.Records()),
	}
}

// Records returns a defensive copy of the projected-record collection.
func (r ResourceReadResult) Records() resources.ProjectedRecords {
	return resources.NewProjectedRecords(r.records.Records())
}

// MarshalJSON rejects direct ResourceReadResult serialization. Future
// transports must define and version their own safe result DTOs.
func (ResourceReadResult) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct ResourceReadResult deserialization.
func (*ResourceReadResult) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}

// ExecutionSettings selects trusted runtime construction behavior without
// carrying environment entries, credentials, secret references, or resolved
// secret values. It belongs at runtime construction, not on operation
// requests.
type ExecutionSettings struct {
	Profile    string
	ConfigPath string
	Timeout    time.Duration

	Redaction    redact.Mode
	RedactionSet bool
	NoCache      bool
}

// MarshalJSON rejects direct ExecutionSettings serialization. Config paths and
// runtime policy do not cross a transport without an explicit versioned DTO.
func (ExecutionSettings) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct ExecutionSettings deserialization.
func (*ExecutionSettings) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}

func cloneResourceReadInput(input ResourceReadInput) ResourceReadInput {
	input.Fields = append([]string(nil), input.Fields...)
	input.Filters = append([]Filter(nil), input.Filters...)
	return input
}
