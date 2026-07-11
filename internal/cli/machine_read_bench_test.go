package cli

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const benchmarkMachineReadRecords = 1_000

func BenchmarkMachineReadResultVerification(b *testing.B) {
	spec := benchmarkMachineReadSpec()
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
		b.Fatalf("NewVerifiedProjectedRecordsFromProjectedFields(%d records) error = %v, want nil", len(rows), err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	var retained resources.ProjectedRecords
	for b.Loop() {
		result := machine.NewResourceReadResult(projected)
		retained, err = verifiedProjectedRecordsFromMachineResult(
			spec,
			redact.ModeStandard,
			result,
		)
		if err != nil {
			b.Fatalf("verifiedProjectedRecordsFromMachineResult(%d records) error = %v, want nil", len(rows), err)
		}
	}
	b.ReportMetric(float64(len(rows)), "records/op")
	runtime.KeepAlive(retained)
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
