package resources_test

import (
	"encoding/json"
	"fmt"
	"runtime"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

const benchmarkProjectedRecordCount = 1_000

func BenchmarkProjectedRecordsMarshalJSON(b *testing.B) {
	projected := benchmarkProjectedRecords(b)
	body, err := json.MarshalIndent(projected, "", "  ")
	if err != nil {
		b.Fatalf("json.MarshalIndent(%d projected records) error = %v, want nil", benchmarkProjectedRecordCount, err)
	}

	b.ReportAllocs()
	b.ReportMetric(benchmarkProjectedRecordCount, "records/op")
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for b.Loop() {
		body, err = json.MarshalIndent(projected, "", "  ")
		if err != nil {
			b.Fatalf("json.MarshalIndent(%d projected records) error = %v, want nil", benchmarkProjectedRecordCount, err)
		}
	}
	runtime.KeepAlive(body)
}

func BenchmarkFilterProjectedRecords(b *testing.B) {
	projected := benchmarkProjectedRecords(b)
	filters := []resources.ProjectedFilter{{
		Field:     "name",
		Value:     "branch-09",
		Substring: true,
	}}

	b.ReportAllocs()
	b.ReportMetric(benchmarkProjectedRecordCount, "records/op")
	b.ResetTimer()
	var filtered resources.ProjectedRecords
	for b.Loop() {
		filtered = resources.FilterProjectedRecords(projected, filters, "us-east")
	}
	runtime.KeepAlive(filtered)
}

func BenchmarkProjectRecordAndVerify(b *testing.B) {
	spec := benchmarkProjectedSpec()
	source := resources.NewSourceRecord(benchmarkProjectedFields(42))

	b.ReportAllocs()
	b.ResetTimer()
	var projected resources.ProjectedRecord
	var report resources.ProjectionReport
	var err error
	for b.Loop() {
		projected, report, err = resources.ProjectRecordAndVerify(spec, redact.ModeStandard, source)
		if err != nil {
			b.Fatalf("ProjectRecordAndVerify(benchmark record) error = %v, want nil", err)
		}
	}
	runtime.KeepAlive(projected)
	runtime.KeepAlive(report)
}

func benchmarkProjectedRecords(tb testing.TB) resources.ProjectedRecords {
	tb.Helper()
	rows := make([]map[string]any, benchmarkProjectedRecordCount)
	for i := range rows {
		rows[i] = benchmarkProjectedFields(i)
	}
	projected, err := resources.NewVerifiedProjectedRecordsFromProjectedFields(
		benchmarkProjectedSpec(),
		redact.ModeStandard,
		rows,
	)
	if err != nil {
		tb.Fatalf("NewVerifiedProjectedRecordsFromProjectedFields(%d records) error = %v, want nil", len(rows), err)
	}
	return projected
}

func benchmarkProjectedFields(i int) map[string]any {
	return map[string]any{
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

func benchmarkProjectedSpec() resources.ResourceSpec {
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
