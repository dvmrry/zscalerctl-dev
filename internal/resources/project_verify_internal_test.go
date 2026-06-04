package resources

import (
	"errors"
	"reflect"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

func TestProjectRecordsAndVerifyProjectsAndChecksSubset(t *testing.T) {
	t.Parallel()

	spec := projectVerifyTestSpec()
	records := []SourceRecord{NewSourceRecord(map[string]any{
		"id":            "123",
		"name":          "HQ",
		"new_sdk_field": "drop-me",
	})}

	projected, reports, err := ProjectRecordsAndVerify(spec, redact.ModeStandard, records)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(%s/%s) error = %v, want nil", spec.Product, spec.Name, err)
	}
	gotRecords := projected.Records()
	if len(gotRecords) != 1 {
		t.Fatalf("ProjectRecordsAndVerify(%s/%s) records = %d, want 1", spec.Product, spec.Name, len(gotRecords))
	}
	wantFields := map[string]any{
		"id":   "123",
		"name": "HQ",
	}
	if !reflect.DeepEqual(gotRecords[0].Fields(), wantFields) {
		t.Errorf("ProjectRecordsAndVerify(%s/%s) fields = %#v, want %#v", spec.Product, spec.Name, gotRecords[0].Fields(), wantFields)
	}
	if len(reports) != 1 {
		t.Fatalf("ProjectRecordsAndVerify(%s/%s) reports = %d, want 1", spec.Product, spec.Name, len(reports))
	}
	if !reflect.DeepEqual(reports[0].DroppedFields, []string{"new_sdk_field"}) {
		t.Errorf("ProjectRecordsAndVerify(%s/%s) DroppedFields = %#v, want [new_sdk_field]", spec.Product, spec.Name, reports[0].DroppedFields)
	}
}

func TestAssertProjectedRecordsSubsetRejectsBypass(t *testing.T) {
	t.Parallel()

	spec := projectVerifyTestSpec()
	projected := NewProjectedRecords([]ProjectedRecord{{
		fields: map[string]any{
			"id":            "123",
			"client_secret": "must-not-render",
		},
	}})

	err := assertProjectedRecordsSubset(spec, redact.ModeStandard, projected)
	if !errors.Is(err, ErrUnexpectedField) {
		t.Errorf("assertProjectedRecordsSubset(%s/%s) error = %v, want ErrUnexpectedField", spec.Product, spec.Name, err)
	}
}

func TestProjectRecordAndVerifyProjectsAndChecksSubset(t *testing.T) {
	t.Parallel()

	spec := projectVerifyTestSpec()
	projected, report, err := ProjectRecordAndVerify(spec, redact.ModeStandard, NewSourceRecord(map[string]any{
		"id":            "123",
		"new_sdk_field": "drop-me",
	}))
	if err != nil {
		t.Fatalf("ProjectRecordAndVerify(%s/%s) error = %v, want nil", spec.Product, spec.Name, err)
	}
	if got, ok := projected.Value("id"); !ok || got != "123" {
		t.Errorf("ProjectRecordAndVerify(%s/%s).Value(id) = %v, %t, want 123, true", spec.Product, spec.Name, got, ok)
	}
	if !reflect.DeepEqual(report.DroppedFields, []string{"new_sdk_field"}) {
		t.Errorf("ProjectRecordAndVerify(%s/%s) DroppedFields = %#v, want [new_sdk_field]", spec.Product, spec.Name, report.DroppedFields)
	}
}

func projectVerifyTestSpec() ResourceSpec {
	return ResourceSpec{
		Product:    ProductZIA,
		Name:       "project-verify",
		Operations: ReadOperations(),
		Fields: []FieldSpec{
			operationalField("id", allModes()),
			tenantConfigField("name", standardShareModes()),
		},
	}
}
