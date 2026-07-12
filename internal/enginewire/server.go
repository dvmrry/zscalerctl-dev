package enginewire

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// ServerFrame is the sealed post-ready server-to-client frame union.
type ServerFrame interface {
	frameValidator
	serverFrame()
}

// Ready completes negotiation and advertises the exact host contract.
type Ready struct {
	Type     string         `json:"type" wire:"required"`
	Protocol string         `json:"protocol" wire:"required"`
	Version  string         `json:"version" wire:"required"`
	Schema   SchemaIdentity `json:"schema" wire:"required"`
	Server   ServerBuild    `json:"server" wire:"required"`
	Limits   Limits         `json:"limits" wire:"required"`
	Engine   EngineManifest `json:"engine" wire:"required"`
}

func (Ready) serverFrame()      {}
func (f Ready) Validate() error { return f.validate() }
func (f Ready) validate() error {
	if f.Type != "ready" || f.Protocol != Protocol || f.Version != V1Version {
		return fmt.Errorf("%w: invalid ready discriminants", ErrInvalidFrame)
	}
	if err := f.Schema.validate(); err != nil {
		return err
	}
	if err := f.Server.validate(); err != nil {
		return err
	}
	if err := f.Limits.validate(); err != nil {
		return err
	}
	return f.Engine.validate()
}

// RequestRejected reports a valid request refused while another is active.
type RequestRejected struct {
	Type   string      `json:"type" wire:"required"`
	ID     SafeInteger `json:"id" wire:"required"`
	Reason string      `json:"reason" wire:"required"`
}

func (RequestRejected) serverFrame()      {}
func (f RequestRejected) Validate() error { return f.validate() }
func (f RequestRejected) validate() error {
	if f.Type != "request_rejected" || f.Reason != "busy" {
		return fmt.Errorf("%w: invalid request rejection", ErrInvalidFrame)
	}
	return validatePositive("id", f.ID)
}

// Started opens one accepted request lifecycle.
type Started struct {
	Type       string      `json:"type" wire:"required"`
	ID         SafeInteger `json:"id" wire:"required"`
	Sequence   SafeInteger `json:"seq" wire:"required"`
	Capability Capability  `json:"capability" wire:"required"`
	Operation  Operation   `json:"operation" wire:"required"`
}

func (Started) serverFrame()      {}
func (f Started) Validate() error { return f.validate() }
func (f Started) validate() error {
	if f.Type != "started" {
		return fmt.Errorf("%w: invalid started type", ErrInvalidFrame)
	}
	if err := validateRequestSequence(f.ID, f.Sequence); err != nil {
		return err
	}
	return validateCapabilityOperation(f.Capability, f.Operation)
}

// ItemValue is the sealed semantic item payload union.
type ItemValue interface {
	validateItem() error
}

func (r CatalogResource) validateItem() error   { return r.validate() }
func (c URLClassification) validateItem() error { return c.validate() }
func (r ProjectedRecord) validateItem() error   { return r.validate() }
func (r DiffResource) validateItem() error      { return r.validate() }
func (r DiffRecordRef) validateItem() error     { return r.validate() }
func (c DiffFieldChange) validateItem() error   { return c.validate() }

// Item carries one inline semantic payload.
type Item[T ItemValue] struct {
	Type     string      `json:"type" wire:"required"`
	ID       SafeInteger `json:"id" wire:"required"`
	Sequence SafeInteger `json:"seq" wire:"required"`
	Kind     ItemKind    `json:"kind" wire:"required"`
	Item     T           `json:"item" wire:"required"`
}

func (Item[T]) serverFrame()      {}
func (f Item[T]) Validate() error { return f.validate() }
func (f Item[T]) validate() error {
	if f.Type != "item" {
		return fmt.Errorf("%w: invalid item type", ErrInvalidFrame)
	}
	if err := validateRequestSequence(f.ID, f.Sequence); err != nil {
		return err
	}
	if err := validateItemValueKind(f.Kind, f.Item); err != nil {
		return err
	}
	return f.Item.validateItem()
}

func validateItemValueKind(kind ItemKind, value any) error {
	valid := false
	switch value.(type) {
	case CatalogResource:
		valid = kind == ItemCatalogResource
	case URLClassification:
		valid = kind == ItemURLClassification
	case ProjectedRecord:
		valid = kind == ItemProjectedRecord
	case DiffResource:
		valid = kind == ItemDiffResource
	case DiffRecordRef:
		valid = kind == ItemDiffAdded || kind == ItemDiffRemoved
	case DiffFieldChange:
		valid = kind == ItemDiffFieldChange
	}
	if !valid {
		return fmt.Errorf("%w: item kind does not match item DTO", ErrInvalidFrame)
	}
	return nil
}

// MarshalItemPayload validates and encodes the exact object carried by an
// inline item member or a fragmented item's reconstructed payload. The depth
// budget reserves one level for the enclosing server frame.
func MarshalItemPayload(kind ItemKind, value ItemValue) ([]byte, error) {
	if isNilInterface(value) {
		return nil, fmt.Errorf("%w: nil item payload", ErrInvalidFrame)
	}
	if err := validateItemValueKind(kind, value); err != nil {
		return nil, err
	}
	if err := value.validateItem(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: item payload encode: %v", ErrInvalidFrame, err)
	}
	if len(data) > AggregateItemBytes {
		return nil, ErrFrameTooLarge
	}
	if err := validateJSONObject(data, V1JSONDepth-1); err != nil {
		return nil, fmt.Errorf("%w: item payload failed strict validation: %v", ErrInvalidFrame, err)
	}
	return data, nil
}

// DecodeItemPayload strictly decodes one reconstructed inline or fragmented
// semantic item object according to its exact kind.
func DecodeItemPayload(kind ItemKind, data []byte) (ItemValue, error) {
	if err := validateInbound(data, AggregateItemBytes, V1JSONDepth-1); err != nil {
		return nil, err
	}
	switch kind {
	case ItemCatalogResource:
		return decodeItemPayloadAs[CatalogResource](data)
	case ItemURLClassification:
		return decodeItemPayloadAs[URLClassification](data)
	case ItemProjectedRecord:
		return decodeItemPayloadAs[ProjectedRecord](data)
	case ItemDiffResource:
		return decodeItemPayloadAs[DiffResource](data)
	case ItemDiffAdded, ItemDiffRemoved:
		return decodeItemPayloadAs[DiffRecordRef](data)
	case ItemDiffFieldChange:
		return decodeItemPayloadAs[DiffFieldChange](data)
	default:
		return nil, fmt.Errorf("%w: unknown item payload kind", ErrInvalidFrame)
	}
}

func decodeItemPayloadAs[T ItemValue](data []byte) (ItemValue, error) {
	var value T
	if err := decodeExact(data, &value); err != nil {
		return nil, err
	}
	if err := value.validateItem(); err != nil {
		return nil, err
	}
	return value, nil
}

// ItemBegin opens one fragmented semantic payload.
type ItemBegin struct {
	Type     string      `json:"type" wire:"required"`
	ID       SafeInteger `json:"id" wire:"required"`
	Sequence SafeInteger `json:"seq" wire:"required"`
	ItemID   SafeInteger `json:"item_id" wire:"required"`
	Kind     ItemKind    `json:"kind" wire:"required"`
	Encoding string      `json:"encoding" wire:"required"`
	Bytes    SafeInteger `json:"bytes" wire:"required"`
}

func (ItemBegin) serverFrame()      {}
func (f ItemBegin) Validate() error { return f.validate() }
func (f ItemBegin) validate() error {
	if f.Type != "item_begin" || f.Encoding != "json" {
		return fmt.Errorf("%w: invalid item-begin discriminants", ErrInvalidFrame)
	}
	if err := validateRequestSequence(f.ID, f.Sequence); err != nil {
		return err
	}
	if err := validatePositive("item_id", f.ItemID); err != nil {
		return err
	}
	if err := validateItemKind(f.Kind); err != nil {
		return err
	}
	if f.Bytes < 2 || f.Bytes > AggregateItemBytes {
		return fmt.Errorf("%w: item byte count outside aggregate bounds", ErrInvalidFrame)
	}
	return nil
}

// ItemChunk carries one canonical-base64 payload fragment.
type ItemChunk struct {
	Type     string      `json:"type" wire:"required"`
	ID       SafeInteger `json:"id" wire:"required"`
	Sequence SafeInteger `json:"seq" wire:"required"`
	ItemID   SafeInteger `json:"item_id" wire:"required"`
	Index    SafeInteger `json:"index" wire:"required"`
	Data     string      `json:"data" wire:"required"`
}

func (ItemChunk) serverFrame()      {}
func (f ItemChunk) Validate() error { return f.validate() }
func (f ItemChunk) validate() error {
	if f.Type != "item_chunk" {
		return fmt.Errorf("%w: invalid item-chunk type", ErrInvalidFrame)
	}
	if err := validateRequestSequence(f.ID, f.Sequence); err != nil {
		return err
	}
	if err := validatePositive("item_id", f.ItemID); err != nil {
		return err
	}
	if f.Index > 127 {
		return fmt.Errorf("%w: chunk index outside 0..127", ErrInvalidFrame)
	}
	decoded, err := base64.StdEncoding.DecodeString(f.Data)
	if err != nil || len(decoded) < 1 || len(decoded) > FragmentChunkBytes || base64.StdEncoding.EncodeToString(decoded) != f.Data {
		return fmt.Errorf("%w: chunk data is not canonical bounded base64", ErrInvalidFrame)
	}
	return nil
}

// ItemEnd closes one fragmented payload with count and digest metadata.
type ItemEnd struct {
	Type     string      `json:"type" wire:"required"`
	ID       SafeInteger `json:"id" wire:"required"`
	Sequence SafeInteger `json:"seq" wire:"required"`
	ItemID   SafeInteger `json:"item_id" wire:"required"`
	Chunks   SafeInteger `json:"chunks" wire:"required"`
	SHA256   string      `json:"sha256" wire:"required"`
}

func (ItemEnd) serverFrame()      {}
func (f ItemEnd) Validate() error { return f.validate() }
func (f ItemEnd) validate() error {
	if f.Type != "item_end" {
		return fmt.Errorf("%w: invalid item-end type", ErrInvalidFrame)
	}
	if err := validateRequestSequence(f.ID, f.Sequence); err != nil {
		return err
	}
	if err := validatePositive("item_id", f.ItemID); err != nil {
		return err
	}
	if f.Chunks < 1 || f.Chunks > 128 || !isLowerHex(f.SHA256, 64) {
		return fmt.Errorf("%w: invalid item-end chunks or digest", ErrInvalidFrame)
	}
	return nil
}

// Progress announces that one catalog resource is about to begin.
type Progress struct {
	Type     string      `json:"type" wire:"required"`
	ID       SafeInteger `json:"id" wire:"required"`
	Sequence SafeInteger `json:"seq" wire:"required"`
	Phase    string      `json:"phase" wire:"required"`
	Current  SafeInteger `json:"current" wire:"required"`
	Total    SafeInteger `json:"total" wire:"required"`
	Product  Product     `json:"product" wire:"required"`
	Resource string      `json:"resource" wire:"required"`
}

func (Progress) serverFrame()      {}
func (f Progress) Validate() error { return f.validate() }
func (f Progress) validate() error {
	if f.Type != "progress" || f.Phase != "resource_started" {
		return fmt.Errorf("%w: invalid progress discriminants", ErrInvalidFrame)
	}
	if err := validateRequestSequence(f.ID, f.Sequence); err != nil {
		return err
	}
	if err := validatePositive("progress current", f.Current); err != nil {
		return err
	}
	if err := validatePositive("progress total", f.Total); err != nil {
		return err
	}
	if f.Current > f.Total {
		return fmt.Errorf("%w: progress current exceeds total", ErrInvalidFrame)
	}
	if err := validateProduct("progress product", f.Product); err != nil {
		return err
	}
	return validateResourceName("progress resource", f.Resource)
}

// Warning carries one provisional value-free dump failure.
type Warning struct {
	Type     string      `json:"type" wire:"required"`
	ID       SafeInteger `json:"id" wire:"required"`
	Sequence SafeInteger `json:"seq" wire:"required"`
	Warning  DumpFailure `json:"warning" wire:"required"`
}

func (Warning) serverFrame()      {}
func (f Warning) Validate() error { return f.validate() }
func (f Warning) validate() error {
	if f.Type != "warning" {
		return fmt.Errorf("%w: invalid warning type", ErrInvalidFrame)
	}
	if err := validateRequestSequence(f.ID, f.Sequence); err != nil {
		return err
	}
	return f.Warning.validate()
}

// CompletionResult is the sealed operation-specific success-result union.
type CompletionResult interface {
	validateResult() error
}

// EngineManifestResult returns candidate engine capability discovery.
type EngineManifestResult struct {
	Kind     string         `json:"kind" wire:"required"`
	Manifest EngineManifest `json:"manifest" wire:"required"`
}

func (r EngineManifestResult) validateResult() error {
	if r.Kind != "engine_manifest" {
		return fmt.Errorf("%w: invalid engine manifest result kind", ErrInvalidFrame)
	}
	return r.Manifest.validate()
}

// CatalogSummary reconciles emitted catalog resources.
type CatalogSummary struct {
	Kind               string      `json:"kind" wire:"required"`
	Resources          SafeInteger `json:"resources" wire:"required"`
	StreamItemsEmitted SafeInteger `json:"stream_items_emitted" wire:"required"`
}

func (r CatalogSummary) validateResult() error {
	if r.Kind != "catalog_summary" || r.Resources != r.StreamItemsEmitted {
		return fmt.Errorf("%w: invalid catalog summary", ErrInvalidFrame)
	}
	return validateSafe("catalog resources", r.Resources)
}

// DoctorStatusResult returns one sanitized doctor view.
type DoctorStatusResult struct {
	Kind   string       `json:"kind" wire:"required"`
	Status DoctorStatus `json:"status" wire:"required"`
}

func (r DoctorStatusResult) validateResult() error {
	if r.Kind != "doctor_status" {
		return fmt.Errorf("%w: invalid doctor result kind", ErrInvalidFrame)
	}
	return r.Status.validate()
}

// AuthStatusResult returns one sanitized authentication view.
type AuthStatusResult struct {
	Kind   string     `json:"kind" wire:"required"`
	Status AuthStatus `json:"status" wire:"required"`
}

func (r AuthStatusResult) validateResult() error {
	if r.Kind != "auth_status" {
		return fmt.Errorf("%w: invalid auth result kind", ErrInvalidFrame)
	}
	return r.Status.validate()
}

// ConfigStatusResult returns one sanitized configuration view.
type ConfigStatusResult struct {
	Kind   string       `json:"kind" wire:"required"`
	Status ConfigStatus `json:"status" wire:"required"`
}

func (r ConfigStatusResult) validateResult() error {
	if r.Kind != "config_status" {
		return fmt.Errorf("%w: invalid config result kind", ErrInvalidFrame)
	}
	return r.Status.validate()
}

// URLLookupSummary reconciles emitted URL classifications.
type URLLookupSummary struct {
	Kind               string      `json:"kind" wire:"required"`
	Classifications    SafeInteger `json:"classifications" wire:"required"`
	StreamItemsEmitted SafeInteger `json:"stream_items_emitted" wire:"required"`
}

func (r URLLookupSummary) validateResult() error {
	if r.Kind != "url_lookup_summary" || r.Classifications != r.StreamItemsEmitted {
		return fmt.Errorf("%w: invalid URL lookup summary", ErrInvalidFrame)
	}
	return validateSafe("URL classifications", r.Classifications)
}

// ResourceReadSummary reconciles emitted projected records.
type ResourceReadSummary struct {
	Kind               string      `json:"kind" wire:"required"`
	Records            SafeInteger `json:"records" wire:"required"`
	StreamItemsEmitted SafeInteger `json:"stream_items_emitted" wire:"required"`
}

func (r ResourceReadSummary) validateResult() error {
	if r.Kind != "resource_read_summary" || r.Records != r.StreamItemsEmitted {
		return fmt.Errorf("%w: invalid resource read summary", ErrInvalidFrame)
	}
	return validateSafe("resource records", r.Records)
}

// DumpSummary describes a completed local dump without its path or records.
type DumpSummary struct {
	Kind               string        `json:"kind" wire:"required"`
	RecordsWritten     SafeInteger   `json:"records_written" wire:"required"`
	ResourcesWritten   SafeInteger   `json:"resources_written" wire:"required"`
	WarningCount       SafeInteger   `json:"warning_count" wire:"required"`
	Partial            bool          `json:"partial" wire:"required"`
	Redaction          Redaction     `json:"redaction" wire:"required"`
	Failures           []DumpFailure `json:"failures" wire:"required"`
	StreamItemsEmitted SafeInteger   `json:"stream_items_emitted" wire:"required"`
}

func (r DumpSummary) validateResult() error {
	if r.Kind != "dump_summary" || r.Failures == nil || len(r.Failures) > MaxResourceSelectorCount || r.StreamItemsEmitted != 0 {
		return fmt.Errorf("%w: invalid dump summary shape", ErrInvalidFrame)
	}
	if r.WarningCount != SafeInteger(len(r.Failures)) || r.Partial != (len(r.Failures) > 0) {
		return fmt.Errorf("%w: dump warning counters do not match failures", ErrInvalidFrame)
	}
	for _, failure := range r.Failures {
		if err := failure.validate(); err != nil {
			return err
		}
	}
	if err := validateSafe("dump records", r.RecordsWritten); err != nil {
		return err
	}
	if err := validateSafe("dump resources", r.ResourcesWritten); err != nil {
		return err
	}
	return validateRedaction("dump redaction", r.Redaction)
}

// DumpSideRef describes one path-free admitted diff input.
type DumpSideRef struct {
	Side           string    `json:"side" wire:"required"`
	ManifestSchema string    `json:"manifest_schema" wire:"required"`
	Redaction      Redaction `json:"redaction" wire:"required"`
	Status         string    `json:"status" wire:"required"`
	Partial        bool      `json:"partial" wire:"required"`
}

func (r DumpSideRef) validate(expectedSide string) error {
	if r.Side != expectedSide || (r.Status != "complete" && r.Status != "partial") || r.Partial != (r.Status == "partial") {
		return fmt.Errorf("%w: invalid dump-side reference", ErrInvalidFrame)
	}
	if err := validateControlString("manifest schema", r.ManifestSchema); err != nil {
		return err
	}
	return validateRedaction("dump-side redaction", r.Redaction)
}

// DiffCounts carries the admitted report's aggregate counters.
type DiffCounts struct {
	ResourcesCompared  SafeInteger `json:"resources_compared" wire:"required"`
	ResourcesWithDrift SafeInteger `json:"resources_with_drift" wire:"required"`
	RecordsAdded       SafeInteger `json:"records_added" wire:"required"`
	RecordsRemoved     SafeInteger `json:"records_removed" wire:"required"`
	RecordsChanged     SafeInteger `json:"records_changed" wire:"required"`
}

func (c DiffCounts) validate() error {
	if c.ResourcesWithDrift > c.ResourcesCompared {
		return fmt.Errorf("%w: resources with drift exceeds compared", ErrInvalidFrame)
	}
	values := []struct {
		name  string
		value SafeInteger
	}{
		{"resources compared", c.ResourcesCompared},
		{"resources with drift", c.ResourcesWithDrift},
		{"records added", c.RecordsAdded},
		{"records removed", c.RecordsRemoved},
		{"records changed", c.RecordsChanged},
	}
	for _, value := range values {
		if err := validateSafe(value.name, value.value); err != nil {
			return err
		}
	}
	return nil
}

// DiffSummary describes one completed comparison without caller paths.
type DiffSummary struct {
	Kind               string      `json:"kind" wire:"required"`
	Schema             string      `json:"schema" wire:"required"`
	Old                DumpSideRef `json:"old" wire:"required"`
	New                DumpSideRef `json:"new" wire:"required"`
	Summary            DiffCounts  `json:"summary" wire:"required"`
	HasDrift           bool        `json:"has_drift" wire:"required"`
	StreamItemsEmitted SafeInteger `json:"stream_items_emitted" wire:"required"`
}

func (r DiffSummary) validateResult() error {
	if r.Kind != "diff_summary" || r.Schema != "zscalerctl.diff.v1" {
		return fmt.Errorf("%w: invalid diff summary discriminants", ErrInvalidFrame)
	}
	if err := r.Old.validate("old"); err != nil {
		return err
	}
	if err := r.New.validate("new"); err != nil {
		return err
	}
	if err := r.Summary.validate(); err != nil {
		return err
	}
	hasDrift := r.Summary.ResourcesWithDrift > 0 || r.Summary.RecordsAdded > 0 || r.Summary.RecordsRemoved > 0 || r.Summary.RecordsChanged > 0
	if r.HasDrift != hasDrift {
		return fmt.Errorf("%w: diff has_drift does not match summary", ErrInvalidFrame)
	}
	return validateSafe("diff stream items", r.StreamItemsEmitted)
}

// Completed closes one request with an operation-specific success result.
type Completed[T CompletionResult] struct {
	Type     string      `json:"type" wire:"required"`
	ID       SafeInteger `json:"id" wire:"required"`
	Sequence SafeInteger `json:"seq" wire:"required"`
	Result   T           `json:"result" wire:"required"`
}

func (Completed[T]) serverFrame()      {}
func (f Completed[T]) Validate() error { return f.validate() }
func (f Completed[T]) validate() error {
	if f.Type != "completed" {
		return fmt.Errorf("%w: invalid completed type", ErrInvalidFrame)
	}
	if err := validateRequestSequence(f.ID, f.Sequence); err != nil {
		return err
	}
	return f.Result.validateResult()
}

// OperationFailureKind identifies one closed value-free operation failure.
type OperationFailureKind string

const (
	FailureUsage                 OperationFailureKind = "usage"
	FailureUnsupportedCapability OperationFailureKind = "unsupported_capability"
	FailureUnsupportedOperation  OperationFailureKind = "unsupported_operation"
	FailureUnknownResource       OperationFailureKind = "unknown_resource"
	FailureNotFound              OperationFailureKind = "not_found"
	FailureInvalidResourceID     OperationFailureKind = "invalid_resource_id"
	FailureLiveAccess            OperationFailureKind = "live_access_failed"
	FailureDeadlineExceeded      OperationFailureKind = "deadline_exceeded"
	FailureInvalidConfig         OperationFailureKind = "invalid_config"
	FailureInvalidProxyConfig    OperationFailureKind = "invalid_proxy_config"
	FailureUnsupportedResource   OperationFailureKind = "unsupported_resource"
	FailureResponseTooLarge      OperationFailureKind = "response_too_large"
	FailureInternal              OperationFailureKind = "internal"
)

// OperationFailure is the sealed missing-credential or static failure union.
type OperationFailure interface {
	validateFailure() error
}

// MissingCredentialsFailure optionally lists allow-listed missing variables.
type MissingCredentialsFailure struct {
	Kind    string   `json:"kind" wire:"required"`
	Missing []string `json:"missing,omitempty"`
}

func (f MissingCredentialsFailure) validateFailure() error {
	if f.Kind != "missing_credentials" || len(f.Missing) > 32 {
		return fmt.Errorf("%w: invalid missing-credentials failure", ErrInvalidFrame)
	}
	allowed := map[string]struct{}{
		"ZSCALERCTL_CLIENT_ID": {}, "ZSCALERCTL_CLIENT_SECRET": {}, "ZSCALERCTL_VANITY_DOMAIN": {}, "ZSCALERCTL_ZPA_CUSTOMER_ID": {},
		"ZSCALERCTL_ZIA_USERNAME": {}, "ZSCALERCTL_ZIA_PASSWORD": {}, "ZSCALERCTL_ZIA_API_KEY": {}, "ZSCALERCTL_ZIA_CLOUD": {},
	}
	seen := map[string]struct{}{}
	for _, name := range f.Missing {
		if _, ok := allowed[name]; !ok {
			return fmt.Errorf("%w: invalid missing credential name", ErrInvalidFrame)
		}
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("%w: duplicate missing credential name", ErrInvalidFrame)
		}
		seen[name] = struct{}{}
	}
	return nil
}

// NonCredentialFailure carries one static non-credential failure kind.
type NonCredentialFailure struct {
	Kind OperationFailureKind `json:"kind" wire:"required"`
}

func (f NonCredentialFailure) validateFailure() error {
	switch f.Kind {
	case FailureUsage, FailureUnsupportedCapability, FailureUnsupportedOperation, FailureUnknownResource, FailureNotFound,
		FailureInvalidResourceID, FailureLiveAccess, FailureDeadlineExceeded, FailureInvalidConfig, FailureInvalidProxyConfig,
		FailureUnsupportedResource, FailureResponseTooLarge, FailureInternal:
		return nil
	default:
		return fmt.Errorf("%w: invalid operation failure kind", ErrInvalidFrame)
	}
}

// Failed closes one request with a value-free operation failure.
type Failed[T OperationFailure] struct {
	Type     string      `json:"type" wire:"required"`
	ID       SafeInteger `json:"id" wire:"required"`
	Sequence SafeInteger `json:"seq" wire:"required"`
	Error    T           `json:"error" wire:"required"`
}

func (Failed[T]) serverFrame()      {}
func (f Failed[T]) Validate() error { return f.validate() }
func (f Failed[T]) validate() error {
	if f.Type != "failed" {
		return fmt.Errorf("%w: invalid failed type", ErrInvalidFrame)
	}
	if err := validateRequestSequence(f.ID, f.Sequence); err != nil {
		return err
	}
	return f.Error.validateFailure()
}

// CanceledError is the fixed canceled terminal payload.
type CanceledError struct {
	Kind string `json:"kind" wire:"required"`
}

func (e CanceledError) validate() error {
	if e.Kind != "canceled" {
		return fmt.Errorf("%w: invalid canceled error", ErrInvalidFrame)
	}
	return nil
}

// Canceled closes one request canceled before outcome commit.
type Canceled struct {
	Type     string        `json:"type" wire:"required"`
	ID       SafeInteger   `json:"id" wire:"required"`
	Sequence SafeInteger   `json:"seq" wire:"required"`
	Error    CanceledError `json:"error" wire:"required"`
}

func (Canceled) serverFrame()      {}
func (f Canceled) Validate() error { return f.validate() }
func (f Canceled) validate() error {
	if f.Type != "canceled" {
		return fmt.Errorf("%w: invalid canceled type", ErrInvalidFrame)
	}
	if err := validateRequestSequence(f.ID, f.Sequence); err != nil {
		return err
	}
	return f.Error.validate()
}

// ProtocolError fatally closes a post-ready session.
type ProtocolError struct {
	Type  string         `json:"type" wire:"required"`
	Fatal bool           `json:"fatal" wire:"required"`
	Error BootstrapError `json:"error" wire:"required"`
}

func (ProtocolError) serverFrame()      {}
func (f ProtocolError) Validate() error { return f.validate() }
func (f ProtocolError) validate() error {
	if f.Type != "protocol_error" || !f.Fatal {
		return fmt.Errorf("%w: invalid v1 protocol-error envelope", ErrInvalidFrame)
	}
	return f.Error.validate()
}

func validateRequestSequence(id, sequence SafeInteger) error {
	if err := validatePositive("id", id); err != nil {
		return err
	}
	return validatePositive("seq", sequence)
}

func validateItemKind(kind ItemKind) error {
	switch kind {
	case ItemCatalogResource, ItemURLClassification, ItemProjectedRecord, ItemDiffResource, ItemDiffAdded, ItemDiffRemoved, ItemDiffFieldChange:
		return nil
	default:
		return fmt.Errorf("%w: invalid item kind", ErrInvalidFrame)
	}
}

// DecodeServerFrame strictly decodes one bounded post-ready server frame.
func DecodeServerFrame(data []byte) (ServerFrame, error) {
	if err := validateInbound(data, V1FrameBytes, V1JSONDepth); err != nil {
		return nil, err
	}
	frameType, present, err := discriminator(data, "type")
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, fmt.Errorf("%w: missing frame type", ErrInvalidFrame)
	}
	switch frameType {
	case "ready":
		return decodeServerAs[Ready](data)
	case "request_rejected":
		return decodeServerAs[RequestRejected](data)
	case "started":
		return decodeServerAs[Started](data)
	case "item":
		return decodeServerItem(data)
	case "item_begin":
		return decodeServerAs[ItemBegin](data)
	case "item_chunk":
		return decodeServerAs[ItemChunk](data)
	case "item_end":
		return decodeServerAs[ItemEnd](data)
	case "progress":
		return decodeServerAs[Progress](data)
	case "warning":
		return decodeServerAs[Warning](data)
	case "completed":
		return decodeServerCompleted(data)
	case "failed":
		return decodeServerFailed(data)
	case "canceled":
		return decodeServerAs[Canceled](data)
	case "protocol_error":
		return decodeServerAs[ProtocolError](data)
	case "request", "cancel":
		return nil, ErrWrongDirection
	default:
		return nil, fmt.Errorf("%w: unknown server frame type", ErrInvalidFrame)
	}
}

func decodeServerItem(data []byte) (ServerFrame, error) {
	kind, present, err := discriminator(data, "kind")
	if err != nil || !present {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: missing item kind", ErrInvalidFrame)
	}
	switch ItemKind(kind) {
	case ItemCatalogResource:
		return decodeServerAs[Item[CatalogResource]](data)
	case ItemURLClassification:
		return decodeServerAs[Item[URLClassification]](data)
	case ItemProjectedRecord:
		return decodeServerAs[Item[ProjectedRecord]](data)
	case ItemDiffResource:
		return decodeServerAs[Item[DiffResource]](data)
	case ItemDiffAdded, ItemDiffRemoved:
		return decodeServerAs[Item[DiffRecordRef]](data)
	case ItemDiffFieldChange:
		return decodeServerAs[Item[DiffFieldChange]](data)
	default:
		return nil, fmt.Errorf("%w: unknown item kind", ErrInvalidFrame)
	}
}

func decodeServerCompleted(data []byte) (ServerFrame, error) {
	kind, present, err := nestedDiscriminator(data, "result", "kind")
	if err != nil || !present {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: missing completion result kind", ErrInvalidFrame)
	}
	switch kind {
	case "engine_manifest":
		return decodeServerAs[Completed[EngineManifestResult]](data)
	case "catalog_summary":
		return decodeServerAs[Completed[CatalogSummary]](data)
	case "doctor_status":
		return decodeServerAs[Completed[DoctorStatusResult]](data)
	case "auth_status":
		return decodeServerAs[Completed[AuthStatusResult]](data)
	case "config_status":
		return decodeServerAs[Completed[ConfigStatusResult]](data)
	case "url_lookup_summary":
		return decodeServerAs[Completed[URLLookupSummary]](data)
	case "resource_read_summary":
		return decodeServerAs[Completed[ResourceReadSummary]](data)
	case "dump_summary":
		return decodeServerAs[Completed[DumpSummary]](data)
	case "diff_summary":
		return decodeServerAs[Completed[DiffSummary]](data)
	default:
		return nil, fmt.Errorf("%w: unknown completion result kind", ErrInvalidFrame)
	}
}

func decodeServerFailed(data []byte) (ServerFrame, error) {
	kind, present, err := nestedDiscriminator(data, "error", "kind")
	if err != nil || !present {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: missing failure kind", ErrInvalidFrame)
	}
	if kind == "missing_credentials" {
		return decodeServerAs[Failed[MissingCredentialsFailure]](data)
	}
	return decodeServerAs[Failed[NonCredentialFailure]](data)
}

func decodeServerAs[T ServerFrame](data []byte) (ServerFrame, error) {
	frame, err := decodeFrame[T](data)
	if err != nil {
		return nil, err
	}
	return frame, nil
}

// MarshalServerFrame validates and encodes one post-ready server frame.
func MarshalServerFrame(frame ServerFrame) ([]byte, error) {
	return marshalBounded(frame, V1FrameBytes, V1JSONDepth)
}

// WriteServerFrame writes one complete LF-terminated server frame.
func WriteServerFrame(writer io.Writer, frame ServerFrame) error {
	return writeBounded(writer, frame, V1FrameBytes, V1JSONDepth)
}
