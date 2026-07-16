package machine

// DumpResourceSelector identifies one exact catalog resource. Adapters may
// accept shorthand, but must resolve it before constructing this request.
type DumpResourceSelector struct {
	Product  string
	Resource string
}

// DumpRequest selects one sanitized tenant collection and local artifact
// write. Empty Products means all catalog products; empty Resources means all
// dumpable resources in the selected products. Runtime settings and
// credentials never enter the request.
type DumpRequest struct {
	RequestID       string
	OutputDir       string
	Products        []string
	Resources       []DumpResourceSelector
	ContinueOnError bool
	Force           bool
}

// MarshalJSON rejects direct DumpRequest serialization.
func (DumpRequest) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct DumpRequest deserialization.
func (*DumpRequest) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}

// DumpResourceError is value-free metadata for one resource that failed in a
// successful continue-on-error dump.
type DumpResourceError struct {
	Product   string
	Resource  string
	Operation Operation
	Kind      string
}

// DumpResult owns the safe summary of a completed dump artifact. It carries no
// output path, records, config, credentials, SDK value, or backend error.
type DumpResult struct {
	records   int
	resources int
	redaction string
	errors    []DumpResourceError
}

// NewDumpResult constructs a summary from trusted, already-sanitized metadata.
func NewDumpResult(
	records int,
	resources int,
	redaction string,
	errors []DumpResourceError,
) DumpResult {
	return DumpResult{
		records:   records,
		resources: resources,
		redaction: redaction,
		errors:    append([]DumpResourceError{}, errors...),
	}
}

// Records returns the number of records written across successful resources.
func (r DumpResult) Records() int { return r.records }

// Resources returns the number of successfully written resources.
func (r DumpResult) Resources() int { return r.resources }

// Warnings returns the number of value-free resource failures.
func (r DumpResult) Warnings() int { return len(r.errors) }

// Redaction returns the effective artifact redaction mode.
func (r DumpResult) Redaction() string { return r.redaction }

// Partial reports whether the artifact contains resource failures.
func (r DumpResult) Partial() bool { return len(r.errors) > 0 }

// Errors returns a defensive copy of the value-free resource failures.
func (r DumpResult) Errors() []DumpResourceError {
	return append([]DumpResourceError{}, r.errors...)
}

// MarshalJSON rejects direct DumpResult serialization.
func (DumpResult) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct DumpResult deserialization.
func (*DumpResult) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}
