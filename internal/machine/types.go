// Package machine defines transport-neutral request, response, and manifest
// types for the machine/core capability API.
//
// The package is intentionally data-only. Cobra, stdio, JSON-RPC, TUI, Wails,
// or other adapters can translate their own inputs into these structs, but
// execution, config loading, SDK access, rendering, and process behavior live
// elsewhere.
package machine

// Operation names a machine capability operation.
type Operation string

const (
	OperationList     Operation = "list"
	OperationGet      Operation = "get"
	OperationShow     Operation = "show"
	OperationManifest Operation = "manifest"
)

// Request is the typed in-process input envelope for a future machine
// capability executor.
type Request struct {
	RequestID  string    `json:"request_id,omitempty"`
	Capability string    `json:"capability,omitempty"`
	Operation  Operation `json:"operation"`
	Input      *Input    `json:"input,omitempty"`
	Meta       *Meta     `json:"meta,omitempty"`
}

// Response is the typed in-process output envelope for a future machine
// capability executor.
type Response struct {
	RequestID  string           `json:"request_id,omitempty"`
	Capability string           `json:"capability,omitempty"`
	Operation  Operation        `json:"operation,omitempty"`
	Records    []map[string]any `json:"records,omitempty"`
	Manifest   *Manifest        `json:"manifest,omitempty"`
	Error      *MachineError    `json:"error,omitempty"`
	Meta       *Meta            `json:"meta,omitempty"`
}

// OutputSafe marks Response as eligible for the existing safe JSON renderer.
// The contract assumes records have already passed projection and redaction
// before they enter this type.
func (Response) OutputSafe() {}

// MachineError is a sanitized machine-facing error payload. It mirrors the
// current stderr error-envelope fields without taking ownership of CLI
// rendering or exit-code mapping.
type MachineError struct {
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
	Missing   []string  `json:"missing,omitempty"`
	Operation Operation `json:"operation,omitempty"`
	Product   string    `json:"product,omitempty"`
	Resource  string    `json:"resource,omitempty"`
}

// Error returns a stable human fallback for callers that handle MachineError as
// an error. Machine clients should use the structured fields instead.
func (e MachineError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Kind != "" {
		return e.Kind
	}
	return "machine error"
}

// OutputSafe marks MachineError as eligible for the existing safe JSON
// renderer. The message must already be sanitized by the core error boundary.
func (MachineError) OutputSafe() {}

// Capability describes one callable machine capability.
type Capability struct {
	Name        string      `json:"name"`
	Title       string      `json:"title,omitempty"`
	Description string      `json:"description,omitempty"`
	Operations  []Operation `json:"operations,omitempty"`
	Input       *Input      `json:"input,omitempty"`
	Output      *SchemaRef  `json:"output,omitempty"`
	Examples    []Example   `json:"examples,omitempty"`
	Meta        *Meta       `json:"meta,omitempty"`
}

// Input describes capability inputs without tying them to Cobra flags or a
// specific transport encoding.
type Input struct {
	Product  string            `json:"product,omitempty"`
	Resource string            `json:"resource,omitempty"`
	RecordID string            `json:"record_id,omitempty"`
	Fields   []string          `json:"fields,omitempty"`
	Filters  []Filter          `json:"filters,omitempty"`
	Search   string            `json:"search,omitempty"`
	Options  map[string]string `json:"options,omitempty"`
}

// Filter describes one projected-data filter requested by a caller.
type Filter struct {
	Field    string `json:"field,omitempty"`
	Operator string `json:"operator,omitempty"`
	Value    string `json:"value,omitempty"`
}

// SchemaRef identifies a schema associated with a capability, request, or
// response.
type SchemaRef struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	URI     string `json:"uri,omitempty"`
	Ref     string `json:"ref,omitempty"`
}

// Example documents a representative request and optional response.
type Example struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Request     Request   `json:"request"`
	Response    *Response `json:"response,omitempty"`
}

// Manifest describes the machine capabilities exposed by a build or adapter.
type Manifest struct {
	Version      string       `json:"version"`
	Capabilities []Capability `json:"capabilities"`
	Schemas      []SchemaRef  `json:"schemas,omitempty"`
	Meta         *Meta        `json:"meta,omitempty"`
}

// OutputSafe marks Manifest as eligible for the existing safe JSON renderer.
func (Manifest) OutputSafe() {}

// Meta carries transport-neutral metadata. It is intentionally small so
// adapters do not smuggle config, credentials, or renderer state through the
// machine contract.
type Meta struct {
	Version     string `json:"version,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
	GeneratedAt string `json:"generated_at,omitempty"`
	Product     string `json:"product,omitempty"`
	Resource    string `json:"resource,omitempty"`
	Shape       string `json:"shape,omitempty"`
	GetKey      string `json:"get_key,omitempty"`
	ReadOnly    bool   `json:"read_only,omitempty"`
	Count       int    `json:"count,omitempty"`
}
