package enginewire

import (
	"fmt"
	"reflect"
)

// SchemaIdentity pins the negotiated schema ID and exact checked-in bytes.
type SchemaIdentity struct {
	ID     string `json:"id" wire:"required"`
	SHA256 string `json:"sha256" wire:"required"`
}

func (s SchemaIdentity) validate() error {
	if s.ID != V1SchemaID || s.SHA256 != V1SchemaSHA256 {
		return fmt.Errorf("%w: invalid schema identity", ErrInvalidFrame)
	}
	return nil
}

// ServerBuild identifies the local engine host build.
type ServerBuild struct {
	Name    string `json:"name" wire:"required"`
	Version string `json:"version" wire:"required"`
}

func (s ServerBuild) validate() error {
	if s.Name != "zscalerctl-engine" {
		return fmt.Errorf("%w: invalid server build name", ErrInvalidFrame)
	}
	return validateStructuralString("server.version", s.Version, 1, 128, MaxControlStringBytes)
}

// Limits carries every fixed v1 transport and collection bound.
type Limits struct {
	ClientFrameBytes      int `json:"client_frame_bytes" wire:"required"`
	ServerFrameBytes      int `json:"server_frame_bytes" wire:"required"`
	JSONDepth             int `json:"json_depth" wire:"required"`
	AggregateItemBytes    int `json:"aggregate_item_bytes" wire:"required"`
	FragmentChunkBytes    int `json:"fragment_chunk_bytes" wire:"required"`
	URLCount              int `json:"url_count" wire:"required"`
	ReadFieldCount        int `json:"read_field_count" wire:"required"`
	ReadFilterCount       int `json:"read_filter_count" wire:"required"`
	ProductSelectorCount  int `json:"product_selector_count" wire:"required"`
	ResourceSelectorCount int `json:"resource_selector_count" wire:"required"`
	PathBytes             int `json:"path_bytes" wire:"required"`
	ControlStringBytes    int `json:"control_string_bytes" wire:"required"`
}

// V1Limits returns the immutable negotiated v1 limits.
func V1Limits() Limits {
	return Limits{
		ClientFrameBytes:      V1FrameBytes,
		ServerFrameBytes:      V1FrameBytes,
		JSONDepth:             V1JSONDepth,
		AggregateItemBytes:    AggregateItemBytes,
		FragmentChunkBytes:    FragmentChunkBytes,
		URLCount:              MaxURLCount,
		ReadFieldCount:        MaxReadFieldCount,
		ReadFilterCount:       MaxReadFilterCount,
		ProductSelectorCount:  MaxProductSelectorCount,
		ResourceSelectorCount: MaxResourceSelectorCount,
		PathBytes:             MaxPathBytes,
		ControlStringBytes:    MaxControlStringBytes,
	}
}

func (l Limits) validate() error {
	if !reflect.DeepEqual(l, V1Limits()) {
		return fmt.Errorf("%w: invalid v1 limits", ErrInvalidFrame)
	}
	return nil
}

// EffectKind identifies one observable local, process, or network effect.
type EffectKind string

const (
	EffectLocalFilesystemRead   EffectKind = "local_filesystem_read"
	EffectLocalFilesystemWrite  EffectKind = "local_filesystem_write"
	EffectLocalFilesystemDelete EffectKind = "local_filesystem_delete"
	EffectNetworkAccess         EffectKind = "network_access"
	EffectProcessExecution      EffectKind = "process_execution"
)

// EffectCondition identifies when a capability effect may occur.
type EffectCondition string

const (
	EffectAlways                 EffectCondition = "always"
	EffectRequestDependent       EffectCondition = "request_dependent"
	EffectConfigurationDependent EffectCondition = "configuration_dependent"
)

// Effect describes one conservative capability effect.
type Effect struct {
	Kind EffectKind      `json:"kind" wire:"required"`
	When EffectCondition `json:"when" wire:"required"`
}

func (e Effect) validate() error {
	switch e.Kind {
	case EffectLocalFilesystemRead, EffectLocalFilesystemWrite, EffectLocalFilesystemDelete, EffectNetworkAccess, EffectProcessExecution:
	default:
		return fmt.Errorf("%w: invalid effect kind", ErrInvalidFrame)
	}
	switch e.When {
	case EffectAlways, EffectRequestDependent, EffectConfigurationDependent:
		return nil
	default:
		return fmt.Errorf("%w: invalid effect condition", ErrInvalidFrame)
	}
}

// EngineCapability is the wire-owned capability metadata DTO.
type EngineCapability struct {
	Name           Capability  `json:"name" wire:"required"`
	Operations     []Operation `json:"operations" wire:"required"`
	Input          string      `json:"input" wire:"required"`
	Result         string      `json:"result" wire:"required"`
	TenantReadOnly bool        `json:"tenant_read_only" wire:"required"`
	Effects        []Effect    `json:"effects" wire:"required"`
}

func (c EngineCapability) validate() error {
	if !c.TenantReadOnly || c.Operations == nil || c.Effects == nil {
		return fmt.Errorf("%w: invalid engine capability basics", ErrInvalidFrame)
	}
	for _, effect := range c.Effects {
		if err := effect.validate(); err != nil {
			return err
		}
	}
	var expectedOperations []Operation
	var expectedInput, expectedResult string
	var expectedEffects []Effect
	switch c.Name {
	case CapabilityEngineManifest:
		expectedOperations = []Operation{OperationManifest}
		expectedInput, expectedResult = "none", "engine_manifest"
		expectedEffects = []Effect{}
	case CapabilityCatalogSchema:
		expectedOperations = []Operation{OperationList}
		expectedInput, expectedResult = "none", "resource_catalog"
		expectedEffects = []Effect{}
	case CapabilityStatusInspect:
		expectedOperations = []Operation{OperationDoctor, OperationAuthStatus, OperationConfigStatus}
		expectedInput, expectedResult = "status", "status"
		expectedEffects = []Effect{{Kind: EffectLocalFilesystemRead, When: EffectConfigurationDependent}}
	case CapabilityZIAURLLookup:
		expectedOperations = []Operation{OperationLookup}
		expectedInput, expectedResult = "url_lookup", "url_classifications"
		expectedEffects = liveReadEffects()
	case CapabilityResourcesRead:
		if err := validateReadOperations(c.Operations); err != nil {
			return err
		}
		expectedOperations = c.Operations
		expectedInput, expectedResult = "resource_read", "projected_records"
		expectedEffects = liveReadEffects()
	case CapabilityDumpWrite:
		expectedOperations = []Operation{OperationDump}
		expectedInput, expectedResult = "dump", "dump_summary"
		expectedEffects = []Effect{
			{Kind: EffectLocalFilesystemRead, When: EffectAlways},
			{Kind: EffectLocalFilesystemWrite, When: EffectAlways},
			{Kind: EffectLocalFilesystemDelete, When: EffectRequestDependent},
			{Kind: EffectNetworkAccess, When: EffectAlways},
			{Kind: EffectProcessExecution, When: EffectConfigurationDependent},
		}
	case CapabilityDiffCompare:
		expectedOperations = []Operation{OperationDiff}
		expectedInput, expectedResult = "diff", "diff_report"
		expectedEffects = []Effect{{Kind: EffectLocalFilesystemRead, When: EffectAlways}}
	default:
		return fmt.Errorf("%w: invalid engine capability name", ErrInvalidFrame)
	}
	if !reflect.DeepEqual(c.Operations, expectedOperations) || c.Input != expectedInput || c.Result != expectedResult || !reflect.DeepEqual(c.Effects, expectedEffects) {
		return fmt.Errorf("%w: engine capability metadata does not match %q", ErrInvalidFrame, c.Name)
	}
	return nil
}

func liveReadEffects() []Effect {
	return []Effect{
		{Kind: EffectLocalFilesystemRead, When: EffectConfigurationDependent},
		{Kind: EffectNetworkAccess, When: EffectAlways},
		{Kind: EffectProcessExecution, When: EffectConfigurationDependent},
	}
}

func validateReadOperations(operations []Operation) error {
	if len(operations) < 1 || len(operations) > 3 {
		return fmt.Errorf("%w: resource read operations must contain 1..3 entries", ErrInvalidFrame)
	}
	seen := map[Operation]struct{}{}
	for _, operation := range operations {
		if operation != OperationList && operation != OperationGet && operation != OperationShow {
			return fmt.Errorf("%w: invalid resource read operation", ErrInvalidFrame)
		}
		if _, duplicate := seen[operation]; duplicate {
			return fmt.Errorf("%w: duplicate resource read operation", ErrInvalidFrame)
		}
		seen[operation] = struct{}{}
	}
	return nil
}

// EngineManifest is the wire-owned candidate engine discovery DTO.
type EngineManifest struct {
	Version        string             `json:"version" wire:"required"`
	TenantReadOnly bool               `json:"tenant_read_only" wire:"required"`
	Capabilities   []EngineCapability `json:"capabilities" wire:"required"`
}

func (m EngineManifest) validate() error {
	if m.Version != "engine.v1" || !m.TenantReadOnly || len(m.Capabilities) < 1 || len(m.Capabilities) > 64 {
		return fmt.Errorf("%w: invalid engine manifest", ErrInvalidFrame)
	}
	seen := make(map[Capability]struct{}, len(m.Capabilities))
	for _, capability := range m.Capabilities {
		if err := capability.validate(); err != nil {
			return err
		}
		if _, duplicate := seen[capability.Name]; duplicate {
			return fmt.Errorf("%w: duplicate engine capability", ErrInvalidFrame)
		}
		seen[capability.Name] = struct{}{}
	}
	return nil
}

func isLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, b := range []byte(value) {
		if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'f') {
			return false
		}
	}
	return true
}
