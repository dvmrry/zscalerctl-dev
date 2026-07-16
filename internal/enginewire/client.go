package enginewire

import (
	"fmt"
	"io"
)

// ClientFrame is the sealed post-ready client-to-server frame union.
type ClientFrame interface {
	frameValidator
	clientFrame()
}

// ManifestRequest requests config-free engine capability discovery.
type ManifestRequest struct {
	Type       string      `json:"type" wire:"required"`
	ID         SafeInteger `json:"id" wire:"required"`
	Capability Capability  `json:"capability" wire:"required"`
	Operation  Operation   `json:"operation" wire:"required"`
}

func (ManifestRequest) clientFrame()      {}
func (f ManifestRequest) Validate() error { return f.validate() }
func (f ManifestRequest) validate() error {
	return validateRequestHeader(f.Type, f.ID, f.Capability, f.Operation)
}

// CatalogRequest requests config-free projected catalog discovery.
type CatalogRequest struct {
	Type       string      `json:"type" wire:"required"`
	ID         SafeInteger `json:"id" wire:"required"`
	Capability Capability  `json:"capability" wire:"required"`
	Operation  Operation   `json:"operation" wire:"required"`
}

func (CatalogRequest) clientFrame()      {}
func (f CatalogRequest) Validate() error { return f.validate() }
func (f CatalogRequest) validate() error {
	return validateRequestHeader(f.Type, f.ID, f.Capability, f.Operation)
}

// DoctorRequest requests the sanitized doctor status view.
type DoctorRequest struct {
	Type       string      `json:"type" wire:"required"`
	ID         SafeInteger `json:"id" wire:"required"`
	Capability Capability  `json:"capability" wire:"required"`
	Operation  Operation   `json:"operation" wire:"required"`
}

func (DoctorRequest) clientFrame()      {}
func (f DoctorRequest) Validate() error { return f.validate() }
func (f DoctorRequest) validate() error {
	return validateRequestHeader(f.Type, f.ID, f.Capability, f.Operation)
}

// AuthStatusRequest requests the sanitized authentication status view.
type AuthStatusRequest struct {
	Type       string      `json:"type" wire:"required"`
	ID         SafeInteger `json:"id" wire:"required"`
	Capability Capability  `json:"capability" wire:"required"`
	Operation  Operation   `json:"operation" wire:"required"`
}

func (AuthStatusRequest) clientFrame()      {}
func (f AuthStatusRequest) Validate() error { return f.validate() }
func (f AuthStatusRequest) validate() error {
	return validateRequestHeader(f.Type, f.ID, f.Capability, f.Operation)
}

// ConfigStatusRequest requests the sanitized configuration-presence view.
type ConfigStatusRequest struct {
	Type       string      `json:"type" wire:"required"`
	ID         SafeInteger `json:"id" wire:"required"`
	Capability Capability  `json:"capability" wire:"required"`
	Operation  Operation   `json:"operation" wire:"required"`
}

func (ConfigStatusRequest) clientFrame()      {}
func (f ConfigStatusRequest) Validate() error { return f.validate() }
func (f ConfigStatusRequest) validate() error {
	return validateRequestHeader(f.Type, f.ID, f.Capability, f.Operation)
}

// URLLookupInput carries one bounded URL-classification batch.
type URLLookupInput struct {
	URLs []string `json:"urls" wire:"required"`
}

func (i URLLookupInput) validate() error {
	if len(i.URLs) < 1 || len(i.URLs) > MaxURLCount {
		return fmt.Errorf("%w: URLs must contain 1..%d entries", ErrInvalidFrame, MaxURLCount)
	}
	for index, value := range i.URLs {
		if err := validateStructuralString(fmt.Sprintf("urls[%d]", index), value, 1, MaxControlStringBytes, MaxControlStringBytes); err != nil {
			return err
		}
	}
	return nil
}

// URLLookupRequest requests sanitized ZIA URL classifications.
type URLLookupRequest struct {
	Type       string         `json:"type" wire:"required"`
	ID         SafeInteger    `json:"id" wire:"required"`
	Capability Capability     `json:"capability" wire:"required"`
	Operation  Operation      `json:"operation" wire:"required"`
	Input      URLLookupInput `json:"input" wire:"required"`
}

func (URLLookupRequest) clientFrame()      {}
func (f URLLookupRequest) Validate() error { return f.validate() }
func (f URLLookupRequest) validate() error {
	if err := validateRequestHeader(f.Type, f.ID, f.Capability, f.Operation); err != nil {
		return err
	}
	return f.Input.validate()
}

// FilterOperator identifies an exact or substring projected-data filter.
type FilterOperator string

const (
	FilterExact    FilterOperator = "exact"
	FilterContains FilterOperator = "contains"
)

// Filter narrows one projected field without widening the result.
type Filter struct {
	Field    string         `json:"field" wire:"required"`
	Operator FilterOperator `json:"operator" wire:"required"`
	Value    string         `json:"value" wire:"required"`
}

func (f Filter) validate() error {
	if err := validateFieldName("filter.field", f.Field); err != nil {
		return err
	}
	if f.Operator != FilterExact && f.Operator != FilterContains {
		return fmt.Errorf("%w: invalid filter operator", ErrInvalidFrame)
	}
	return validateControlString("filter.value", f.Value)
}

// ResourceListInput carries the exact list operation input shape.
type ResourceListInput struct {
	Product  Product  `json:"product" wire:"required"`
	Resource string   `json:"resource" wire:"required"`
	Fields   []string `json:"fields" wire:"required"`
	Filters  []Filter `json:"filters" wire:"required"`
	Search   string   `json:"search" wire:"required"`
}

func (i ResourceListInput) validate() error {
	if err := validateResourceInput(i.Product, i.Resource, i.Fields); err != nil {
		return err
	}
	if i.Filters == nil || len(i.Filters) > MaxReadFilterCount {
		return fmt.Errorf("%w: filters must be an array of at most %d entries", ErrInvalidFrame, MaxReadFilterCount)
	}
	for _, filter := range i.Filters {
		if err := filter.validate(); err != nil {
			return err
		}
	}
	return validateControlString("search", i.Search)
}

// ResourceListRequest requests a projected resource collection.
type ResourceListRequest struct {
	Type       string            `json:"type" wire:"required"`
	ID         SafeInteger       `json:"id" wire:"required"`
	Capability Capability        `json:"capability" wire:"required"`
	Operation  Operation         `json:"operation" wire:"required"`
	Input      ResourceListInput `json:"input" wire:"required"`
}

func (ResourceListRequest) clientFrame()      {}
func (f ResourceListRequest) Validate() error { return f.validate() }
func (f ResourceListRequest) validate() error {
	if err := validateRequestHeader(f.Type, f.ID, f.Capability, f.Operation); err != nil {
		return err
	}
	return f.Input.validate()
}

// ResourceGetInput carries the exact ID-backed get input shape.
type ResourceGetInput struct {
	Product  Product  `json:"product" wire:"required"`
	Resource string   `json:"resource" wire:"required"`
	RecordID string   `json:"record_id" wire:"required"`
	Fields   []string `json:"fields" wire:"required"`
}

func (i ResourceGetInput) validate() error {
	if err := validateResourceInput(i.Product, i.Resource, i.Fields); err != nil {
		return err
	}
	return validateStructuralString("record_id", i.RecordID, 1, MaxControlStringBytes, MaxControlStringBytes)
}

// ResourceGetRequest requests one projected record by ID.
type ResourceGetRequest struct {
	Type       string           `json:"type" wire:"required"`
	ID         SafeInteger      `json:"id" wire:"required"`
	Capability Capability       `json:"capability" wire:"required"`
	Operation  Operation        `json:"operation" wire:"required"`
	Input      ResourceGetInput `json:"input" wire:"required"`
}

func (ResourceGetRequest) clientFrame()      {}
func (f ResourceGetRequest) Validate() error { return f.validate() }
func (f ResourceGetRequest) validate() error {
	if err := validateRequestHeader(f.Type, f.ID, f.Capability, f.Operation); err != nil {
		return err
	}
	return f.Input.validate()
}

// ResourceShowInput carries the exact singleton show input shape.
type ResourceShowInput struct {
	Product  Product  `json:"product" wire:"required"`
	Resource string   `json:"resource" wire:"required"`
	Fields   []string `json:"fields" wire:"required"`
}

func (i ResourceShowInput) validate() error {
	return validateResourceInput(i.Product, i.Resource, i.Fields)
}

// ResourceShowRequest requests one projected singleton resource.
type ResourceShowRequest struct {
	Type       string            `json:"type" wire:"required"`
	ID         SafeInteger       `json:"id" wire:"required"`
	Capability Capability        `json:"capability" wire:"required"`
	Operation  Operation         `json:"operation" wire:"required"`
	Input      ResourceShowInput `json:"input" wire:"required"`
}

func (ResourceShowRequest) clientFrame()      {}
func (f ResourceShowRequest) Validate() error { return f.validate() }
func (f ResourceShowRequest) validate() error {
	if err := validateRequestHeader(f.Type, f.ID, f.Capability, f.Operation); err != nil {
		return err
	}
	return f.Input.validate()
}

// ResourceSelector identifies one exact catalog resource.
type ResourceSelector struct {
	Product  Product `json:"product" wire:"required"`
	Resource string  `json:"resource" wire:"required"`
}

func (s ResourceSelector) validate() error {
	if err := validateProduct("selector.product", s.Product); err != nil {
		return err
	}
	return validateResourceName("selector.resource", s.Resource)
}

// DumpInput carries one explicit local dump-artifact request.
type DumpInput struct {
	OutputDir       string             `json:"output_dir" wire:"required"`
	Products        []Product          `json:"products" wire:"required"`
	Resources       []ResourceSelector `json:"resources" wire:"required"`
	ContinueOnError bool               `json:"continue_on_error" wire:"required"`
	Force           bool               `json:"force" wire:"required"`
}

func (i DumpInput) validate() error {
	if err := validatePath("output_dir", i.OutputDir); err != nil {
		return err
	}
	return validateSelections(i.Products, i.Resources)
}

// DumpRequest requests a sanitized local dump artifact.
type DumpRequest struct {
	Type       string      `json:"type" wire:"required"`
	ID         SafeInteger `json:"id" wire:"required"`
	Capability Capability  `json:"capability" wire:"required"`
	Operation  Operation   `json:"operation" wire:"required"`
	Input      DumpInput   `json:"input" wire:"required"`
}

func (DumpRequest) clientFrame()      {}
func (f DumpRequest) Validate() error { return f.validate() }
func (f DumpRequest) validate() error {
	if err := validateRequestHeader(f.Type, f.ID, f.Capability, f.Operation); err != nil {
		return err
	}
	return f.Input.validate()
}

// DiffInput carries one comparison of two admitted local dump artifacts.
type DiffInput struct {
	OldDir            string             `json:"old_dir" wire:"required"`
	NewDir            string             `json:"new_dir" wire:"required"`
	Products          []Product          `json:"products" wire:"required"`
	Resources         []ResourceSelector `json:"resources" wire:"required"`
	IgnoreOperational bool               `json:"ignore_operational" wire:"required"`
	AllowPartial      bool               `json:"allow_partial" wire:"required"`
}

func (i DiffInput) validate() error {
	if err := validatePath("old_dir", i.OldDir); err != nil {
		return err
	}
	if err := validatePath("new_dir", i.NewDir); err != nil {
		return err
	}
	return validateSelections(i.Products, i.Resources)
}

// DiffRequest requests a local dump comparison.
type DiffRequest struct {
	Type       string      `json:"type" wire:"required"`
	ID         SafeInteger `json:"id" wire:"required"`
	Capability Capability  `json:"capability" wire:"required"`
	Operation  Operation   `json:"operation" wire:"required"`
	Input      DiffInput   `json:"input" wire:"required"`
}

func (DiffRequest) clientFrame()      {}
func (f DiffRequest) Validate() error { return f.validate() }
func (f DiffRequest) validate() error {
	if err := validateRequestHeader(f.Type, f.ID, f.Capability, f.Operation); err != nil {
		return err
	}
	return f.Input.validate()
}

// Cancel names the active request to cancel.
type Cancel struct {
	Type string      `json:"type" wire:"required"`
	ID   SafeInteger `json:"id" wire:"required"`
}

func (Cancel) clientFrame()      {}
func (f Cancel) Validate() error { return f.validate() }
func (f Cancel) validate() error {
	if f.Type != "cancel" {
		return fmt.Errorf("%w: invalid cancel type", ErrInvalidFrame)
	}
	return validatePositive("id", f.ID)
}

func validateRequestHeader(frameType string, id SafeInteger, capability Capability, operation Operation) error {
	if frameType != "request" {
		return fmt.Errorf("%w: request type must be request", ErrInvalidFrame)
	}
	if err := validatePositive("id", id); err != nil {
		return err
	}
	return validateCapabilityOperation(capability, operation)
}

func validateResourceInput(product Product, resource string, fields []string) error {
	if err := validateProduct("product", product); err != nil {
		return err
	}
	if err := validateResourceName("resource", resource); err != nil {
		return err
	}
	return validateUniqueStrings("fields", fields, MaxReadFieldCount, validateFieldName)
}

func validateSelections(products []Product, resources []ResourceSelector) error {
	if products == nil || len(products) > MaxProductSelectorCount {
		return fmt.Errorf("%w: products must be an array of at most %d entries", ErrInvalidFrame, MaxProductSelectorCount)
	}
	seenProducts := make(map[Product]struct{}, len(products))
	for _, product := range products {
		if err := validateProduct("products", product); err != nil {
			return err
		}
		if _, duplicate := seenProducts[product]; duplicate {
			return fmt.Errorf("%w: products contains a duplicate", ErrInvalidFrame)
		}
		seenProducts[product] = struct{}{}
	}
	if resources == nil || len(resources) > MaxResourceSelectorCount {
		return fmt.Errorf("%w: resources must be an array of at most %d entries", ErrInvalidFrame, MaxResourceSelectorCount)
	}
	seenResources := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		if err := resource.validate(); err != nil {
			return err
		}
		key := string(resource.Product) + "\x00" + resource.Resource
		if _, duplicate := seenResources[key]; duplicate {
			return fmt.Errorf("%w: resources contains a duplicate", ErrInvalidFrame)
		}
		seenResources[key] = struct{}{}
	}
	return nil
}

// DecodeClientFrame strictly decodes one bounded post-ready client frame.
func DecodeClientFrame(data []byte) (ClientFrame, error) {
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
	if frameType == "cancel" {
		return decodeClientAs[Cancel](data)
	}
	if frameType != "request" {
		if isServerFrameType(frameType) {
			return nil, ErrWrongDirection
		}
		return nil, fmt.Errorf("%w: unknown client frame type", ErrInvalidFrame)
	}
	capabilityValue, present, err := discriminator(data, "capability")
	if err != nil || !present {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: missing capability", ErrInvalidFrame)
	}
	operationValue, present, err := discriminator(data, "operation")
	if err != nil || !present {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: missing operation", ErrInvalidFrame)
	}
	capability, operation := Capability(capabilityValue), Operation(operationValue)
	switch {
	case capability == CapabilityEngineManifest && operation == OperationManifest:
		return decodeClientAs[ManifestRequest](data)
	case capability == CapabilityCatalogSchema && operation == OperationList:
		return decodeClientAs[CatalogRequest](data)
	case capability == CapabilityStatusInspect && operation == OperationDoctor:
		return decodeClientAs[DoctorRequest](data)
	case capability == CapabilityStatusInspect && operation == OperationAuthStatus:
		return decodeClientAs[AuthStatusRequest](data)
	case capability == CapabilityStatusInspect && operation == OperationConfigStatus:
		return decodeClientAs[ConfigStatusRequest](data)
	case capability == CapabilityZIAURLLookup && operation == OperationLookup:
		return decodeClientAs[URLLookupRequest](data)
	case capability == CapabilityResourcesRead && operation == OperationList:
		return decodeClientAs[ResourceListRequest](data)
	case capability == CapabilityResourcesRead && operation == OperationGet:
		return decodeClientAs[ResourceGetRequest](data)
	case capability == CapabilityResourcesRead && operation == OperationShow:
		return decodeClientAs[ResourceShowRequest](data)
	case capability == CapabilityDumpWrite && operation == OperationDump:
		return decodeClientAs[DumpRequest](data)
	case capability == CapabilityDiffCompare && operation == OperationDiff:
		return decodeClientAs[DiffRequest](data)
	default:
		return nil, fmt.Errorf("%w: invalid capability/operation pair", ErrInvalidFrame)
	}
}

func isServerFrameType(frameType string) bool {
	switch frameType {
	case "ready", "request_rejected", "started", "item", "item_begin", "item_chunk", "item_end", "progress", "warning", "completed", "failed", "canceled", "protocol_error":
		return true
	default:
		return false
	}
}

func decodeClientAs[T ClientFrame](data []byte) (ClientFrame, error) {
	frame, err := decodeFrame[T](data)
	if err != nil {
		return nil, err
	}
	return frame, nil
}

// MarshalClientFrame validates and encodes one post-ready client frame.
func MarshalClientFrame(frame ClientFrame) ([]byte, error) {
	return marshalBounded(frame, V1FrameBytes, V1JSONDepth)
}

// WriteClientFrame writes one complete LF-terminated client frame.
func WriteClientFrame(writer io.Writer, frame ClientFrame) error {
	return writeBounded(writer, frame, V1FrameBytes, V1JSONDepth)
}
