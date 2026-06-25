package machine

import (
	"fmt"
	"sort"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

const (
	// ManifestVersion is the version string for catalog-derived machine
	// manifests.
	ManifestVersion = "machine.v1"

	// CapabilityResourcesRead is the generic resource-read capability name.
	CapabilityResourcesRead = "resources.read"

	// ProjectedRecordsSchemaName identifies projected/redacted resource record
	// responses.
	ProjectedRecordsSchemaName = "projected-records"

	// ProjectedRecordsSchemaVersion is the first projected-record response
	// schema version.
	ProjectedRecordsSchemaVersion = "1"
)

// ManifestFromCatalog derives a machine capability manifest from the same
// resource catalog used by CLI/resource execution. It does not execute
// resources, load config, construct clients, or maintain a second registry.
func ManifestFromCatalog(catalog resources.ResourceCatalog) Manifest {
	entries := map[string]manifestEntry{}
	for _, spec := range catalog {
		ops := readOperationsFromSpec(spec)
		if len(ops) == 0 {
			continue
		}
		key := manifestEntryKey(spec)
		entry, ok := entries[key]
		if !ok {
			entry = manifestEntry{
				product:  string(spec.Product),
				resource: spec.Name,
				shape:    string(spec.EffectiveShape()),
				getKey:   spec.EffectiveGetKey(),
				ops:      map[Operation]bool{},
			}
		}
		if entry.getKey == "" {
			entry.getKey = spec.EffectiveGetKey()
		}
		for _, op := range ops {
			entry.ops[op] = true
		}
		entries[key] = entry
	}

	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := entries[keys[i]]
		right := entries[keys[j]]
		if left.product != right.product {
			return left.product < right.product
		}
		return left.resource < right.resource
	})

	capabilities := make([]Capability, 0, len(keys))
	for _, key := range keys {
		capabilities = append(capabilities, capabilityFromEntry(entries[key]))
	}

	return Manifest{
		Version:      ManifestVersion,
		Capabilities: capabilities,
		Schemas:      []SchemaRef{ProjectedRecordsSchemaRef()},
		Meta: &Meta{
			Version:  "1",
			ReadOnly: true,
			Count:    len(capabilities),
		},
	}
}

// ProjectedRecordsSchemaRef returns the schema reference used by resource-read
// capabilities.
func ProjectedRecordsSchemaRef() SchemaRef {
	return SchemaRef{
		Name:    ProjectedRecordsSchemaName,
		Version: ProjectedRecordsSchemaVersion,
	}
}

type manifestEntry struct {
	product  string
	resource string
	shape    string
	getKey   string
	ops      map[Operation]bool
}

func capabilityFromEntry(entry manifestEntry) Capability {
	meta := &Meta{
		Product:  entry.product,
		Resource: entry.resource,
		Shape:    entry.shape,
		ReadOnly: true,
	}
	if entry.getKey != "" {
		meta.GetKey = entry.getKey
	}
	return Capability{
		Name:  CapabilityResourcesRead,
		Title: fmt.Sprintf("Read %s/%s", entry.product, entry.resource),
		Description: fmt.Sprintf(
			"Read projected and redacted %s/%s resource records.",
			entry.product,
			entry.resource,
		),
		Operations: sortedOperations(entry.ops),
		Input: &Input{
			Product:  entry.product,
			Resource: entry.resource,
		},
		Output: schemaRefPtr(ProjectedRecordsSchemaRef()),
		Meta:   meta,
	}
}

func readOperationsFromSpec(spec resources.ResourceSpec) []Operation {
	ops := make([]Operation, 0, len(spec.Operations))
	for _, op := range spec.Operations {
		if op.Capability != resources.CapabilityRead {
			continue
		}
		ops = append(ops, Operation(op.Name))
	}
	return ops
}

func sortedOperations(values map[Operation]bool) []Operation {
	ops := make([]Operation, 0, len(values))
	for op := range values {
		ops = append(ops, op)
	}
	sort.Slice(ops, func(i, j int) bool {
		leftRank, leftKnown := operationRank(ops[i])
		rightRank, rightKnown := operationRank(ops[j])
		if leftKnown && rightKnown && leftRank != rightRank {
			return leftRank < rightRank
		}
		if leftKnown != rightKnown {
			return leftKnown
		}
		return ops[i] < ops[j]
	})
	return ops
}

func operationRank(op Operation) (int, bool) {
	switch op {
	case OperationList:
		return 0, true
	case OperationGet:
		return 1, true
	case OperationShow:
		return 2, true
	case OperationManifest:
		return 3, true
	default:
		return 0, false
	}
}

func schemaRefPtr(value SchemaRef) *SchemaRef {
	return &value
}

func manifestEntryKey(spec resources.ResourceSpec) string {
	return string(spec.Product) + "\x00" + spec.Name
}
