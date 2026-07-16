package machine

import (
	"context"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

type typedResultTestLoader struct {
	records resources.ProjectedRecords
}

func (l typedResultTestLoader) ListProjected(
	context.Context,
	string,
	string,
) (resources.ProjectedRecords, error) {
	return l.records, nil
}

func (l typedResultTestLoader) ShowProjected(
	context.Context,
	string,
	string,
) (resources.ProjectedRecords, error) {
	return l.records, nil
}

func TestExecuteStreamCarriesTypedResourceResultOnCompletion(t *testing.T) {
	t.Parallel()

	standard := []redact.Mode{redact.ModeStandard}
	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "typed-result-test",
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational, AllowedModes: standard},
			{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: standard},
		},
	}
	projected, err := resources.NewVerifiedProjectedRecordsFromProjectedFields(
		spec,
		redact.ModeStandard,
		[]map[string]any{{"id": 1, "name": "HQ"}},
	)
	if err != nil {
		t.Fatalf("NewVerifiedProjectedRecordsFromProjectedFields(typed result) error = %v, want nil", err)
	}

	var completed Event
	err = (Executor{
		Browser:   typedResultTestLoader{records: projected},
		Catalog:   resources.ResourceCatalog{spec},
		Redaction: redact.ModeStandard,
	}).ExecuteStream(context.Background(), Request{
		Capability: CapabilityResourcesRead,
		Operation:  OperationList,
		Input: &ResourceReadInput{
			Product:  string(spec.Product),
			Resource: spec.Name,
		},
	}, func(event Event) error {
		if event.Kind == EventCompleted {
			completed = event
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Executor.ExecuteStream(typed result) error = %v, want nil", err)
	}
	if completed.resourceResult == nil {
		t.Fatal("Executor.ExecuteStream(typed result) completion has nil typed result")
	}
	if got := completed.resourceResult.Records().Len(); got != 1 {
		t.Errorf("Executor.ExecuteStream(typed result) records = %d, want 1", got)
	}
}
