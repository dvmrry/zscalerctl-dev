package machine

import "github.com/dvmrry/zscalerctl/internal/resources"

const (
	// EngineManifestVersion is the version of the candidate in-process engine
	// capability model. It is independent from the supported machine.v1
	// manifest and from any future wire-protocol version.
	EngineManifestVersion = "engine.v1"

	// CapabilityEngineManifest identifies config-free engine discovery.
	CapabilityEngineManifest = "engine.manifest"
)

// EngineInputKind identifies one closed family of typed engine inputs.
type EngineInputKind string

const (
	EngineInputNone         EngineInputKind = "none"
	EngineInputResourceRead EngineInputKind = "resource_read"
)

// EngineResultKind identifies one closed family of safe engine results.
type EngineResultKind string

const (
	EngineResultManifest         EngineResultKind = "engine_manifest"
	EngineResultProjectedRecords EngineResultKind = "projected_records"
)

// EngineEffectKind identifies a possible observable engine effect.
type EngineEffectKind string

const (
	EngineEffectLocalFilesystemRead   EngineEffectKind = "local_filesystem_read"
	EngineEffectLocalFilesystemWrite  EngineEffectKind = "local_filesystem_write"
	EngineEffectLocalFilesystemDelete EngineEffectKind = "local_filesystem_delete"
	EngineEffectNetworkAccess         EngineEffectKind = "network_access"
	EngineEffectProcessExecution      EngineEffectKind = "process_execution"
)

// EngineEffectCondition identifies when a possible effect may occur.
type EngineEffectCondition string

const (
	EngineEffectAlways                 EngineEffectCondition = "always"
	EngineEffectRequestDependent       EngineEffectCondition = "request_dependent"
	EngineEffectConfigurationDependent EngineEffectCondition = "configuration_dependent"
)

// EngineEffect describes one conservative end-to-end capability effect.
type EngineEffect struct {
	Kind EngineEffectKind
	When EngineEffectCondition
}

// EngineCapability describes one candidate in-process operation family.
type EngineCapability struct {
	Name           string
	Operations     []Operation
	Input          EngineInputKind
	Result         EngineResultKind
	TenantReadOnly bool
	Effects        []EngineEffect
}

// EngineManifest describes typed capabilities implemented by the common local
// engine. It is candidate Go API metadata, not the supported machine.v1
// manifest and not a wire representation.
type EngineManifest struct {
	Version        string
	TenantReadOnly bool
	Capabilities   []EngineCapability
}

// MarshalJSON rejects direct EngineManifest serialization. Future transports
// must define a separately versioned capability-manifest DTO.
func (EngineManifest) MarshalJSON() ([]byte, error) {
	return nil, errEngineTypeHasNoWireFormat
}

// UnmarshalJSON rejects direct EngineManifest deserialization.
func (*EngineManifest) UnmarshalJSON([]byte) error {
	return errEngineTypeHasNoWireFormat
}

// EngineManifestFromCatalog derives candidate engine discovery from the same
// catalog that drives resource execution. It loads no config, resolves no
// credentials, constructs no SDK client, and contacts no tenant.
func EngineManifestFromCatalog(catalog resources.ResourceCatalog) EngineManifest {
	capabilities := []EngineCapability{{
		Name:           CapabilityEngineManifest,
		Operations:     []Operation{OperationManifest},
		Input:          EngineInputNone,
		Result:         EngineResultManifest,
		TenantReadOnly: true,
		Effects:        []EngineEffect{},
	}}

	readOps := map[Operation]bool{}
	for _, spec := range catalog {
		for _, op := range readOperationsFromSpec(spec) {
			if isSupportedReadOperation(op) {
				readOps[op] = true
			}
		}
	}
	if len(readOps) > 0 {
		capabilities = append(capabilities, EngineCapability{
			Name:           CapabilityResourcesRead,
			Operations:     sortedOperations(readOps),
			Input:          EngineInputResourceRead,
			Result:         EngineResultProjectedRecords,
			TenantReadOnly: true,
			Effects: []EngineEffect{
				{
					Kind: EngineEffectLocalFilesystemRead,
					When: EngineEffectConfigurationDependent,
				},
				{
					Kind: EngineEffectNetworkAccess,
					When: EngineEffectAlways,
				},
				{
					Kind: EngineEffectProcessExecution,
					When: EngineEffectConfigurationDependent,
				},
			},
		})
	}

	return EngineManifest{
		Version:        EngineManifestVersion,
		TenantReadOnly: true,
		Capabilities:   capabilities,
	}
}
