package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const benchmarkMachineReadRecords = 1_000

type benchmarkMachineReadLoader struct {
	records resources.ProjectedRecords
}

func (l benchmarkMachineReadLoader) ListProjected(
	context.Context,
	string,
	string,
) (resources.ProjectedRecords, error) {
	return l.records, nil
}

func (l benchmarkMachineReadLoader) ShowProjected(
	context.Context,
	string,
	string,
) (resources.ProjectedRecords, error) {
	return l.records, nil
}

func BenchmarkMachineReadJSONPath(b *testing.B) {
	spec := benchmarkMachineReadSpec()
	projected := benchmarkMachineReadProjectedRecords(b, spec)
	executor := machine.Executor{
		Browser:   benchmarkMachineReadLoader{records: projected},
		Catalog:   resources.ResourceCatalog{spec},
		Redaction: redact.ModeStandard,
	}
	request := machine.ResourceReadRequest{
		Operation: machine.OperationList,
		Input: machine.ResourceReadInput{
			Product:  string(spec.Product),
			Resource: spec.Name,
		},
	}

	b.ReportAllocs()
	b.ReportMetric(benchmarkMachineReadRecords, "records/op")
	b.ResetTimer()
	var body []byte
	for b.Loop() {
		result, err := executor.Read(context.Background(), request)
		if err != nil {
			b.Fatalf("Executor.Read(%d records) error = %v, want nil", benchmarkMachineReadRecords, err)
		}
		verified, err := verifiedProjectedRecordsFromMachineResult(spec, redact.ModeStandard, result)
		if err != nil {
			b.Fatalf("verifiedProjectedRecordsFromMachineResult(%d records) error = %v, want nil", benchmarkMachineReadRecords, err)
		}
		body, err = json.MarshalIndent(verified, "", "  ")
		if err != nil {
			b.Fatalf("json.MarshalIndent(machine result with %d records) error = %v, want nil", benchmarkMachineReadRecords, err)
		}
	}
	runtime.KeepAlive(body)
}

func BenchmarkMachineReadResultVerification(b *testing.B) {
	spec := benchmarkMachineReadSpec()
	projected := benchmarkMachineReadProjectedRecords(b, spec)

	b.ReportAllocs()
	b.ResetTimer()
	var retained resources.ProjectedRecords
	var err error
	for b.Loop() {
		result := machine.NewResourceReadResult(projected)
		retained, err = verifiedProjectedRecordsFromMachineResult(
			spec,
			redact.ModeStandard,
			result,
		)
		if err != nil {
			b.Fatalf("verifiedProjectedRecordsFromMachineResult(%d records) error = %v, want nil", benchmarkMachineReadRecords, err)
		}
	}
	b.ReportMetric(benchmarkMachineReadRecords, "records/op")
	runtime.KeepAlive(retained)
}

func benchmarkMachineReadProjectedRecords(tb testing.TB, spec resources.ResourceSpec) resources.ProjectedRecords {
	tb.Helper()
	rows := make([]map[string]any, benchmarkMachineReadRecords)
	for i := range rows {
		rows[i] = map[string]any{
			"id":      i,
			"name":    fmt.Sprintf("branch-%04d", i),
			"enabled": i%2 == 0,
			"ports":   []int{80, 443, 8443},
			"metadata": map[string]any{
				"region": "us-east",
				"labels": []string{"branch", "managed", "benchmark"},
			},
		}
	}
	projected, err := resources.NewVerifiedProjectedRecordsFromProjectedFields(
		spec,
		redact.ModeStandard,
		rows,
	)
	if err != nil {
		tb.Fatalf("NewVerifiedProjectedRecordsFromProjectedFields(%d records) error = %v, want nil", len(rows), err)
	}
	return projected
}

func benchmarkMachineReadSpec() resources.ResourceSpec {
	standard := []redact.Mode{redact.ModeStandard}
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "benchmark-locations",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational, AllowedModes: standard},
			{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: standard},
			{Name: "enabled", Classification: resources.ClassTenantConfig, AllowedModes: standard},
			{Name: "ports", Classification: resources.ClassOperational, AllowedModes: standard},
			{
				Name:           "metadata",
				Classification: resources.ClassOperational,
				AllowedModes:   standard,
				Fields: []resources.FieldSpec{
					{Name: "region", Classification: resources.ClassOperational, AllowedModes: standard},
					{Name: "labels", Classification: resources.ClassOperational, AllowedModes: standard},
				},
			},
		},
	}
}
