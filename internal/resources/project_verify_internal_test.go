package resources

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

const unisolatedProjectedCanary = "UNISOLATED_PROJECTED_CANARY_MUST_NOT_RENDER"

type unisolatedProjectedString string

func (unisolatedProjectedString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + unisolatedProjectedCanary + `"`), nil
}

func (unisolatedProjectedString) String() string { return unisolatedProjectedCanary }

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

func TestProjectedRecordsMarshalJSONQuarantinesUnisolatedPrivateState(t *testing.T) {
	t.Parallel()

	projected := NewProjectedRecords([]ProjectedRecord{{
		fields: map[string]any{"id": unisolatedProjectedString("private")},
	}})
	body, err := json.Marshal(projected)
	if !errors.Is(err, ErrInvalidProjectedValue) {
		t.Fatalf("json.Marshal(ProjectedRecords with unisolated private state) error = %v, want ErrInvalidProjectedValue", err)
	}
	if strings.Contains(string(body), unisolatedProjectedCanary) {
		t.Errorf("json.Marshal(ProjectedRecords with unisolated private state) = %q, want no canary", body)
	}
}

func TestProjectedRecordsMarshalJSONPreservesLegacyErrorChain(t *testing.T) {
	t.Parallel()

	projected := NewProjectedRecordsFromProjectedFields([]map[string]any{{
		"id": unisolatedProjectedString("private"),
	}})
	_, gotErr := projected.MarshalJSON()
	_, wantErr := legacyProjectedRecordsMarshalJSON(projected)
	if gotErr == nil || wantErr == nil {
		t.Fatalf("ProjectedRecords.MarshalJSON() errors = (%v, legacy %v), want both non-nil", gotErr, wantErr)
	}
	if gotErr.Error() != wantErr.Error() {
		t.Errorf("ProjectedRecords.MarshalJSON() error = %q, legacy error = %q", gotErr, wantErr)
	}
}

func TestFilterProjectedRecordsQuarantinesUnisolatedPrivateState(t *testing.T) {
	t.Parallel()

	projected := NewProjectedRecords([]ProjectedRecord{{
		fields: map[string]any{"id": unisolatedProjectedString("private")},
	}})
	filtered := FilterProjectedRecords(projected, nil, unisolatedProjectedCanary)
	if got := filtered.Len(); got != 0 {
		t.Errorf("FilterProjectedRecords(unisolated private canary) records = %d, want 0", got)
	}
}

func legacyProjectedRecordsMarshalJSON(projected ProjectedRecords) ([]byte, error) {
	out := make([]map[string]any, len(projected.records))
	for i, record := range projected.records {
		out[i] = record.Fields()
	}
	return json.Marshal(out)
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
