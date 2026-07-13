package adapter

import (
	"fmt"

	dumpdiff "github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// ToReady builds and validates the exact negotiated v1 contract frame.
func ToReady(source machine.EngineManifest, buildVersion string) (enginewire.Ready, error) {
	manifest, err := ToEngineManifest(source)
	if err != nil {
		return enginewire.Ready{}, err
	}
	ready := enginewire.Ready{
		Type:     "ready",
		Protocol: enginewire.Protocol,
		Version:  enginewire.V1Version,
		Schema: enginewire.SchemaIdentity{
			ID:     enginewire.V1SchemaID,
			SHA256: enginewire.V1SchemaSHA256,
		},
		Server: enginewire.ServerBuild{
			Name:    "zscalerctl-engine",
			Version: buildVersion,
		},
		Limits: enginewire.V1Limits(),
		Engine: manifest,
	}
	if err := ready.Validate(); err != nil {
		return enginewire.Ready{}, err
	}
	return ready, nil
}

// ToEngineManifest explicitly copies candidate in-process capability metadata.
func ToEngineManifest(source machine.EngineManifest) (enginewire.EngineManifest, error) {
	capabilities := make([]enginewire.EngineCapability, len(source.Capabilities))
	for i, capability := range source.Capabilities {
		operations := make([]enginewire.Operation, len(capability.Operations))
		for j, operation := range capability.Operations {
			operations[j] = enginewire.Operation(operation)
		}
		effects := make([]enginewire.Effect, len(capability.Effects))
		for j, effect := range capability.Effects {
			effects[j] = enginewire.Effect{
				Kind: enginewire.EffectKind(effect.Kind),
				When: enginewire.EffectCondition(effect.When),
			}
		}
		capabilities[i] = enginewire.EngineCapability{
			Name:           enginewire.Capability(capability.Name),
			Operations:     operations,
			Input:          string(capability.Input),
			Result:         string(capability.Result),
			TenantReadOnly: capability.TenantReadOnly,
			Effects:        effects,
		}
	}
	manifest := enginewire.EngineManifest{
		Version:        source.Version,
		TenantReadOnly: source.TenantReadOnly,
		Capabilities:   capabilities,
	}
	probe := enginewire.Ready{
		Type:     "ready",
		Protocol: enginewire.Protocol,
		Version:  enginewire.V1Version,
		Schema:   enginewire.SchemaIdentity{ID: enginewire.V1SchemaID, SHA256: enginewire.V1SchemaSHA256},
		Server:   enginewire.ServerBuild{Name: "zscalerctl-engine", Version: "conversion-check"},
		Limits:   enginewire.V1Limits(),
		Engine:   manifest,
	}
	if err := probe.Validate(); err != nil {
		return enginewire.EngineManifest{}, fmt.Errorf("convert engine manifest: %w", err)
	}
	return manifest, nil
}

// ToCatalogResources drops source-only metadata and returns wire-owned DTOs.
func ToCatalogResources(source machine.CatalogResult) ([]enginewire.CatalogResource, error) {
	catalog := source.Catalog()
	out := make([]enginewire.CatalogResource, len(catalog))
	for i, spec := range catalog {
		converted, err := toCatalogResource(spec)
		if err != nil {
			return nil, err
		}
		out[i] = converted
	}
	return out, nil
}

func toCatalogResource(spec resources.ResourceSpec) (enginewire.CatalogResource, error) {
	operations := make([]enginewire.Operation, 0, len(spec.Operations))
	for _, operation := range spec.Operations {
		if operation.Capability != resources.CapabilityRead {
			return enginewire.CatalogResource{}, fmt.Errorf("convert catalog: mutating operation on %s/%s", spec.Product, spec.Name)
		}
		operations = append(operations, enginewire.Operation(operation.Name))
	}
	fields := make([]enginewire.CatalogField, len(spec.Fields))
	for i, field := range spec.Fields {
		converted, err := toCatalogField(field)
		if err != nil {
			return enginewire.CatalogResource{}, err
		}
		fields[i] = converted
	}
	var getKey *string
	if value := spec.EffectiveGetKey(); value != "" {
		valueCopy := value
		getKey = &valueCopy
	}
	converted := enginewire.CatalogResource{
		Product:    enginewire.Product(spec.Product),
		Name:       spec.Name,
		Shape:      string(spec.EffectiveShape()),
		Operations: operations,
		GetKey:     getKey,
		Fields:     fields,
	}
	probe := enginewire.Item[enginewire.CatalogResource]{Type: "item", ID: 1, Sequence: 1, Kind: enginewire.ItemCatalogResource, Item: converted}
	if err := probe.Validate(); err != nil {
		return enginewire.CatalogResource{}, fmt.Errorf("convert catalog resource: %w", err)
	}
	return converted, nil
}

func toCatalogField(field resources.FieldSpec) (enginewire.CatalogField, error) {
	fields := make([]enginewire.CatalogField, len(field.Fields))
	for i, nested := range field.Fields {
		converted, err := toCatalogField(nested)
		if err != nil {
			return enginewire.CatalogField{}, err
		}
		fields[i] = converted
	}
	modes := make([]enginewire.Redaction, len(field.AllowedModes))
	for i, mode := range field.AllowedModes {
		modes[i] = enginewire.Redaction(redact.EffectiveMode(mode))
	}
	var jsonName *string
	if field.JSONName != "" {
		value := field.JSONName
		jsonName = &value
	}
	return enginewire.CatalogField{
		Name:           field.Name,
		JSONName:       jsonName,
		Classification: enginewire.FieldClassification(field.Classification),
		AllowedModes:   modes,
		Fields:         fields,
	}, nil
}

// ToURLClassifications recursively copies sanitized lookup answers.
func ToURLClassifications(source machine.URLLookupResult) ([]enginewire.URLClassification, error) {
	classifications := source.Classifications()
	out := make([]enginewire.URLClassification, len(classifications))
	for i, classification := range classifications {
		converted := enginewire.URLClassification{
			URL:                          classification.URL,
			Classifications:              append([]string{}, classification.Classifications...),
			SecurityAlertClassifications: append([]string{}, classification.SecurityAlertClassifications...),
			Application:                  classification.Application,
		}
		probe := enginewire.Item[enginewire.URLClassification]{Type: "item", ID: 1, Sequence: 1, Kind: enginewire.ItemURLClassification, Item: converted}
		if err := probe.Validate(); err != nil {
			return nil, fmt.Errorf("convert URL classification: %w", err)
		}
		out[i] = converted
	}
	return out, nil
}

// ToProjectedRecords recursively copies verified projected records.
func ToProjectedRecords(product, resource string, source machine.ResourceReadResult) ([]enginewire.ProjectedRecord, error) {
	records := source.Records().Records()
	out := make([]enginewire.ProjectedRecord, len(records))
	for i, record := range records {
		wireRecord, err := enginewire.NewWireRecord(record.Fields())
		if err != nil {
			return nil, fmt.Errorf("convert projected record: %w", err)
		}
		converted := enginewire.ProjectedRecord{
			Product:  enginewire.Product(product),
			Resource: resource,
			Record:   wireRecord,
		}
		probe := enginewire.Item[enginewire.ProjectedRecord]{Type: "item", ID: 1, Sequence: 1, Kind: enginewire.ItemProjectedRecord, Item: converted}
		if err := probe.Validate(); err != nil {
			return nil, fmt.Errorf("convert projected record: %w", err)
		}
		out[i] = converted
	}
	return out, nil
}

// ToStatusResult converts the closed typed status union without raw config.
func ToStatusResult(source machine.StatusResult) (enginewire.CompletionResult, error) {
	switch source.Operation() {
	case machine.OperationDoctor:
		status, ok := source.Doctor()
		if !ok {
			return nil, fmt.Errorf("convert doctor status: typed result is empty")
		}
		return enginewire.DoctorStatusResult{Kind: "doctor_status", Status: toDoctorStatus(status)}, nil
	case machine.OperationAuthStatus:
		status, ok := source.Auth()
		if !ok {
			return nil, fmt.Errorf("convert auth status: typed result is empty")
		}
		return enginewire.AuthStatusResult{Kind: "auth_status", Status: toAuthStatus(status)}, nil
	case machine.OperationConfigStatus:
		status, ok := source.Config()
		if !ok {
			return nil, fmt.Errorf("convert config status: typed result is empty")
		}
		return enginewire.ConfigStatusResult{Kind: "config_status", Status: toConfigStatus(status)}, nil
	default:
		return nil, fmt.Errorf("convert status: unsupported operation %q", source.Operation())
	}
}

func toDoctorStatus(source machine.DoctorStatus) enginewire.DoctorStatus {
	return enginewire.DoctorStatus{
		Status: source.Status, Mode: source.Mode, Profile: source.Profile, Config: source.Config,
		AuthMode: enginewire.AuthMode(source.AuthMode), Redaction: enginewire.Redaction(source.Redaction),
		Timeout: source.Timeout, Cache: source.Cache, Proxy: source.Proxy, Credentials: source.Credentials, LiveAPI: source.LiveAPI,
	}
}

func toAuthStatus(source machine.AuthStatus) enginewire.AuthStatus {
	return enginewire.AuthStatus{
		Credentials: source.Credentials, CredentialExchange: source.CredentialExchange, LiveAPI: source.LiveAPI,
	}
}

func toConfigStatus(source machine.ConfigStatus) enginewire.ConfigStatus {
	return enginewire.ConfigStatus{
		Source: source.Source, ConfigFileSet: source.ConfigFileSet, Profile: source.Profile,
		AuthMode: enginewire.AuthMode(source.AuthMode), VanityDomainSet: source.VanityDomainSet, Cloud: source.Cloud,
		Credentials: enginewire.ConfigCredentials{
			ClientIDSet: source.Credentials.ClientIDSet, ClientSecretSet: source.Credentials.ClientSecretSet,
			ClientSecretFileSet: source.Credentials.ClientSecretFileSet, ClientSecretScheme: source.Credentials.ClientSecretScheme,
		},
		ZPA: enginewire.ConfigZPA{CustomerIDSet: source.ZPA.CustomerIDSet, MicrotenantIDSet: source.ZPA.MicrotenantIDSet},
		ZIALegacy: enginewire.ConfigZIALegacy{
			UsernameSet: source.ZIALegacy.UsernameSet, PasswordSet: source.ZIALegacy.PasswordSet,
			PasswordFileSet: source.ZIALegacy.PasswordFileSet, PasswordScheme: source.ZIALegacy.PasswordScheme,
			APIKeySet: source.ZIALegacy.APIKeySet, APIKeyFileSet: source.ZIALegacy.APIKeyFileSet,
			APIKeyScheme: source.ZIALegacy.APIKeyScheme, CloudSet: source.ZIALegacy.CloudSet,
		},
		Proxy:    enginewire.ConfigProxy{URLSet: source.Proxy.URLSet, FromEnvironment: source.Proxy.FromEnvironment},
		Defaults: enginewire.ConfigDefaults{Redaction: enginewire.Redaction(source.Defaults.Redaction), NoCache: source.Defaults.NoCache},
	}
}

// ToDumpSummary converts a path-free value-only dump result.
func ToDumpSummary(source machine.DumpResult) (enginewire.DumpSummary, error) {
	records, err := safeCount("dump records", source.Records())
	if err != nil {
		return enginewire.DumpSummary{}, err
	}
	resourcesWritten, err := safeCount("dump resources", source.Resources())
	if err != nil {
		return enginewire.DumpSummary{}, err
	}
	errorsSource := source.Errors()
	failures := make([]enginewire.DumpFailure, len(errorsSource))
	for i, failure := range errorsSource {
		failures[i] = enginewire.DumpFailure{
			Product: enginewire.Product(failure.Product), Resource: failure.Resource,
			Phase: string(failure.Operation), Kind: failure.Kind,
		}
		probe := enginewire.Warning{Type: "warning", ID: 1, Sequence: 1, Warning: failures[i]}
		if err := probe.Validate(); err != nil {
			return enginewire.DumpSummary{}, fmt.Errorf("convert dump failure: %w", err)
		}
	}
	warningCount, err := safeCount("dump warnings", len(failures))
	if err != nil {
		return enginewire.DumpSummary{}, err
	}
	summary := enginewire.DumpSummary{
		Kind: "dump_summary", RecordsWritten: records, ResourcesWritten: resourcesWritten,
		WarningCount: warningCount, Partial: source.Partial(),
		Redaction: enginewire.Redaction(source.Redaction()), Failures: failures, StreamItemsEmitted: 0,
	}
	probe := enginewire.Completed[enginewire.DumpSummary]{Type: "completed", ID: 1, Sequence: 1, Result: summary}
	if err := probe.Validate(); err != nil {
		return enginewire.DumpSummary{}, fmt.Errorf("convert dump summary: %w", err)
	}
	return summary, nil
}

// SemanticItem pairs a diff payload with its exact wire discriminant.
type SemanticItem struct {
	Kind  enginewire.ItemKind
	Value enginewire.ItemValue
}

// DiffConversion contains flattened path-free diff items and summary.
type DiffConversion struct {
	Items   []SemanticItem
	Summary enginewire.DiffSummary
}

// ToDiffResult flattens one admitted report without copying local paths.
func ToDiffResult(source machine.DiffResult) (DiffConversion, error) {
	report := source.Report()
	items := make([]SemanticItem, 0)
	for _, resource := range report.Resources {
		changedFields := 0
		for _, change := range resource.Changed {
			changedFields += len(change.Changes)
		}
		added, err := safeCount("diff added", len(resource.Added))
		if err != nil {
			return DiffConversion{}, err
		}
		removed, err := safeCount("diff removed", len(resource.Removed))
		if err != nil {
			return DiffConversion{}, err
		}
		changed, err := safeCount("diff changed fields", changedFields)
		if err != nil {
			return DiffConversion{}, err
		}
		identity := enginewire.DiffIdentity{Mode: enginewire.DiffIdentityMode(resource.Identity.Mode)}
		if resource.Identity.Field != "" {
			field := resource.Identity.Field
			identity.Field = &field
		}
		header := enginewire.DiffResource{
			Product: enginewire.Product(resource.Product), Resource: resource.Resource,
			Identity: identity, Added: added, Removed: removed, ChangedFields: changed, Note: resource.Note,
		}
		items = append(items, SemanticItem{Kind: enginewire.ItemDiffResource, Value: header})
		for _, record := range resource.Added {
			converted, err := toDiffRecordRef(resource.Product, resource.Resource, record)
			if err != nil {
				return DiffConversion{}, err
			}
			items = append(items, SemanticItem{Kind: enginewire.ItemDiffAdded, Value: converted})
		}
		for _, record := range resource.Removed {
			converted, err := toDiffRecordRef(resource.Product, resource.Resource, record)
			if err != nil {
				return DiffConversion{}, err
			}
			items = append(items, SemanticItem{Kind: enginewire.ItemDiffRemoved, Value: converted})
		}
		for _, change := range resource.Changed {
			for _, field := range change.Changes {
				oldValue, err := enginewire.NewWireValue(field.Old)
				if err != nil {
					return DiffConversion{}, fmt.Errorf("convert old diff value: %w", err)
				}
				newValue, err := enginewire.NewWireValue(field.New)
				if err != nil {
					return DiffConversion{}, fmt.Errorf("convert new diff value: %w", err)
				}
				items = append(items, SemanticItem{
					Kind: enginewire.ItemDiffFieldChange,
					Value: enginewire.DiffFieldChange{
						Product: enginewire.Product(resource.Product), Resource: resource.Resource,
						Key: change.Key, Field: field.Field, Old: oldValue, New: newValue,
					},
				})
			}
		}
	}
	for _, item := range items {
		if err := validateSemanticItem(item); err != nil {
			return DiffConversion{}, err
		}
	}
	summary, err := toDiffSummary(report, len(items))
	if err != nil {
		return DiffConversion{}, err
	}
	return DiffConversion{Items: items, Summary: summary}, nil
}

func validateSemanticItem(item SemanticItem) error {
	var err error
	switch value := item.Value.(type) {
	case enginewire.DiffResource:
		err = (enginewire.Item[enginewire.DiffResource]{Type: "item", ID: 1, Sequence: 1, Kind: item.Kind, Item: value}).Validate()
	case enginewire.DiffRecordRef:
		err = (enginewire.Item[enginewire.DiffRecordRef]{Type: "item", ID: 1, Sequence: 1, Kind: item.Kind, Item: value}).Validate()
	case enginewire.DiffFieldChange:
		err = (enginewire.Item[enginewire.DiffFieldChange]{Type: "item", ID: 1, Sequence: 1, Kind: item.Kind, Item: value}).Validate()
	default:
		return fmt.Errorf("convert diff item: unsupported payload %T", item.Value)
	}
	if err != nil {
		return fmt.Errorf("convert diff item: %w", err)
	}
	return nil
}

func toDiffRecordRef(product, resource string, source dumpdiff.RecordRef) (enginewire.DiffRecordRef, error) {
	record, err := enginewire.NewWireRecord(source.Record)
	if err != nil {
		return enginewire.DiffRecordRef{}, fmt.Errorf("convert diff record: %w", err)
	}
	converted := enginewire.DiffRecordRef{
		Product: enginewire.Product(product), Resource: resource, Record: record,
	}
	if source.Key != "" {
		key := source.Key
		converted.Key = &key
	}
	if source.Hash != "" {
		hash := source.Hash
		converted.Hash = &hash
	}
	return converted, nil
}

func toDiffSummary(report dumpdiff.Report, streamItems int) (enginewire.DiffSummary, error) {
	counts := []int{
		report.Summary.ResourcesCompared, report.Summary.ResourcesWithDrift,
		report.Summary.RecordsAdded, report.Summary.RecordsRemoved, report.Summary.RecordsChanged,
	}
	convertedCounts := make([]enginewire.SafeInteger, len(counts))
	for i, count := range counts {
		converted, err := safeCount("diff summary", count)
		if err != nil {
			return enginewire.DiffSummary{}, err
		}
		convertedCounts[i] = converted
	}
	itemCount, err := safeCount("diff stream items", streamItems)
	if err != nil {
		return enginewire.DiffSummary{}, err
	}
	summary := enginewire.DiffSummary{
		Kind: "diff_summary", Schema: report.Schema,
		Old: enginewire.DumpSideRef{
			Side: "old", ManifestSchema: report.Old.ManifestSchema,
			Redaction: enginewire.Redaction(report.Old.Redaction), Status: report.Old.Status, Partial: report.Old.Partial,
		},
		New: enginewire.DumpSideRef{
			Side: "new", ManifestSchema: report.New.ManifestSchema,
			Redaction: enginewire.Redaction(report.New.Redaction), Status: report.New.Status, Partial: report.New.Partial,
		},
		Summary: enginewire.DiffCounts{
			ResourcesCompared: convertedCounts[0], ResourcesWithDrift: convertedCounts[1],
			RecordsAdded: convertedCounts[2], RecordsRemoved: convertedCounts[3], RecordsChanged: convertedCounts[4],
		},
		HasDrift: sourceReportHasDrift(report), StreamItemsEmitted: itemCount,
	}
	probe := enginewire.Completed[enginewire.DiffSummary]{Type: "completed", ID: 1, Sequence: 1, Result: summary}
	if err := probe.Validate(); err != nil {
		return enginewire.DiffSummary{}, fmt.Errorf("convert diff summary: %w", err)
	}
	return summary, nil
}

func sourceReportHasDrift(report dumpdiff.Report) bool {
	return report.Summary.RecordsAdded > 0 || report.Summary.RecordsRemoved > 0 || report.Summary.RecordsChanged > 0
}

func safeCount(name string, value int) (enginewire.SafeInteger, error) {
	if value < 0 {
		return 0, fmt.Errorf("convert %s: count outside safe integer range", name)
	}
	converted := uint64(value) // #nosec G115 -- nonnegative int is losslessly representable by uint64.
	if converted > enginewire.MaxSafeInteger {
		return 0, fmt.Errorf("convert %s: count outside safe integer range", name)
	}
	return enginewire.SafeInteger(converted), nil
}
