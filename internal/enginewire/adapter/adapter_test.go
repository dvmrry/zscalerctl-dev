package adapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	dumpdiff "github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/enginewire/adapter"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestRequestConvertersProduceTypedDefensiveInputs(t *testing.T) {
	t.Parallel()

	listFrame := enginewire.ResourceListRequest{
		Type: "request", ID: 7, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
		Input: enginewire.ResourceListInput{
			Product: enginewire.ProductZIA, Resource: "locations", Fields: []string{"id"},
			Filters: []enginewire.Filter{{Field: "name", Operator: enginewire.FilterContains, Value: "hq"}}, Search: "branch",
		},
	}
	request := adapter.ToResourceListRequest(listFrame)
	listFrame.Input.Fields[0] = "mutated"
	listFrame.Input.Filters[0].Field = "mutated"
	if request.RequestID != "7" || request.Operation != machine.OperationList || request.Input.Product != "zia" || request.Input.Resource != "locations" {
		t.Fatalf("ToResourceListRequest() = %#v", request)
	}
	if got, want := request.Input.Fields, []string{"id"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("resource fields = %#v, want %#v", got, want)
	}
	if got, want := request.Input.Filters, []machine.Filter{{Field: "name", Operator: "contains", Value: "hq"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("resource filters = %#v, want %#v", got, want)
	}

	get := adapter.ToResourceGetRequest(enginewire.ResourceGetRequest{
		Type: "request", ID: 8, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationGet,
		Input: enginewire.ResourceGetInput{Product: enginewire.ProductZPA, Resource: "app-segments", RecordID: "42", Fields: []string{}},
	})
	if get.Operation != machine.OperationGet || get.Input.RecordID != "42" || get.Input.Filters != nil || get.Input.Search != "" {
		t.Fatalf("ToResourceGetRequest() = %#v", get)
	}
	show := adapter.ToResourceShowRequest(enginewire.ResourceShowRequest{
		Type: "request", ID: 9, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationShow,
		Input: enginewire.ResourceShowInput{Product: enginewire.ProductZIA, Resource: "advanced-settings", Fields: []string{}},
	})
	if show.Operation != machine.OperationShow || show.Input.RecordID != "" || show.Input.Filters != nil {
		t.Fatalf("ToResourceShowRequest() = %#v", show)
	}
}

func TestRequestConvertersCopySelectorsAndProcessOnlyWireOperators(t *testing.T) {
	t.Parallel()

	dumpFrame := enginewire.DumpRequest{
		Type: "request", ID: 10, Capability: enginewire.CapabilityDumpWrite, Operation: enginewire.OperationDump,
		Input: enginewire.DumpInput{
			OutputDir: "out", Products: []enginewire.Product{enginewire.ProductZIA},
			Resources:       []enginewire.ResourceSelector{{Product: enginewire.ProductZIA, Resource: "locations"}},
			ContinueOnError: true, Force: true,
		},
	}
	dumpRequest := adapter.ToDumpRequest(dumpFrame)
	dumpFrame.Input.Products[0] = enginewire.ProductZPA
	dumpFrame.Input.Resources[0].Resource = "mutated"
	if got, want := dumpRequest.Products, []string{"zia"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("dump products = %#v, want %#v", got, want)
	}
	if got := dumpRequest.Resources[0].Resource; got != "locations" {
		t.Fatalf("dump resource = %q, want locations", got)
	}

	diffRequest := adapter.ToDiffRequest(enginewire.DiffRequest{
		Type: "request", ID: 11, Capability: enginewire.CapabilityDiffCompare, Operation: enginewire.OperationDiff,
		Input: enginewire.DiffInput{OldDir: "old", NewDir: "new", Products: []enginewire.Product{}, Resources: []enginewire.ResourceSelector{}, AllowPartial: true},
	})
	if diffRequest.RequestID != "11" || diffRequest.OldDir != "old" || diffRequest.NewDir != "new" || !diffRequest.AllowPartial {
		t.Fatalf("ToDiffRequest() = %#v", diffRequest)
	}
}

func TestManifestAndCatalogConvertersProduceWireValidatedDTOs(t *testing.T) {
	t.Parallel()

	spec := adapterTestSpec()
	manifest := machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec})
	ready, err := adapter.ToReady(manifest, "test-build")
	if err != nil {
		t.Fatalf("ToReady() error = %v", err)
	}
	if _, err := enginewire.MarshalServerFrame(ready); err != nil {
		t.Fatalf("MarshalServerFrame(ready) error = %v", err)
	}
	if len(ready.Engine.Capabilities) != 7 {
		t.Fatalf("ready capabilities = %d, want 7", len(ready.Engine.Capabilities))
	}

	catalog, err := adapter.ToCatalogResources(machine.NewCatalogResult(resources.ResourceCatalog{spec}))
	if err != nil {
		t.Fatalf("ToCatalogResources() error = %v", err)
	}
	if len(catalog) != 1 || catalog[0].GetKey == nil || *catalog[0].GetKey != "id" || len(catalog[0].Fields) != 1 {
		t.Fatalf("catalog conversion = %#v", catalog)
	}
	frame := enginewire.Item[enginewire.CatalogResource]{Type: "item", ID: 1, Sequence: 2, Kind: enginewire.ItemCatalogResource, Item: catalog[0]}
	encoded, err := enginewire.MarshalServerFrame(frame)
	if err != nil {
		t.Fatalf("MarshalServerFrame(catalog) error = %v", err)
	}
	for _, forbidden := range []string{"SensitiveNameReason", "sensitive_name_reason", "StandardFreeTextReason"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("catalog wire output leaked source-only field %q: %s", forbidden, encoded)
		}
	}
}

func TestValueResultConvertersCopyAndValidateProjectedData(t *testing.T) {
	t.Parallel()

	urlSource := []machine.URLClassification{{
		URL: "https://example.test/", Classifications: []string{"BUSINESS"},
		SecurityAlertClassifications: []string{}, Application: "browser",
	}}
	urls, err := adapter.ToURLClassifications(machine.NewURLLookupResult(urlSource))
	if err != nil {
		t.Fatalf("ToURLClassifications() error = %v", err)
	}
	urlSource[0].Classifications[0] = "mutated"
	if urls[0].Classifications[0] != "BUSINESS" {
		t.Fatalf("URL conversion retained source alias: %#v", urls)
	}

	records := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{
		"id": json.Number("9007199254740993"), "ports": []int{80, 443}, "bytes": []byte{1, 2, 3},
	}})
	converted, err := adapter.ToProjectedRecords("zia", "locations", machine.NewResourceReadResult(records))
	if err != nil {
		t.Fatalf("ToProjectedRecords() error = %v", err)
	}
	frame := enginewire.Item[enginewire.ProjectedRecord]{Type: "item", ID: 1, Sequence: 2, Kind: enginewire.ItemProjectedRecord, Item: converted[0]}
	encoded, err := enginewire.MarshalServerFrame(frame)
	if err != nil {
		t.Fatalf("MarshalServerFrame(projected) error = %v", err)
	}
	for _, want := range []string{"9007199254740993", `"ports":[80,443]`, `"bytes":"AQID"`} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("projected wire output = %s, want %s", encoded, want)
		}
	}
}

func TestStatusAndDumpConvertersUseClosedValueOnlyShapes(t *testing.T) {
	t.Parallel()

	doctor := machine.NewDoctorStatusResult(machine.DoctorStatus{
		Status: "OK", Mode: "read-only", Profile: "", Config: "environment", AuthMode: "oneapi",
		Redaction: "standard", Timeout: "30s", Cache: "enabled", Proxy: "not configured",
		Credentials: "configured", LiveAPI: "available",
	})
	result, err := adapter.ToStatusResult(doctor)
	if err != nil {
		t.Fatalf("ToStatusResult(doctor) error = %v", err)
	}
	if _, ok := result.(enginewire.DoctorStatusResult); !ok {
		t.Fatalf("ToStatusResult(doctor) type = %T", result)
	}

	dumpResult := machine.NewDumpResult(2, 1, "share", []machine.DumpResourceError{{
		Product: "zia", Resource: "locations", Operation: machine.OperationList, Kind: "list_failed",
	}})
	summary, err := adapter.ToDumpSummary(dumpResult)
	if err != nil {
		t.Fatalf("ToDumpSummary() error = %v", err)
	}
	completed := enginewire.Completed[enginewire.DumpSummary]{Type: "completed", ID: 1, Sequence: 3, Result: summary}
	if _, err := enginewire.MarshalServerFrame(completed); err != nil {
		t.Fatalf("MarshalServerFrame(dump summary) error = %v", err)
	}
	if summary.WarningCount != 1 || !summary.Partial || len(summary.Failures) != 1 || summary.StreamItemsEmitted != 0 {
		t.Fatalf("dump summary = %#v", summary)
	}
}

func TestDiffConverterFlattensItemsAndOmitsPaths(t *testing.T) {
	t.Parallel()

	report := dumpdiff.Report{
		Schema:  dumpdiff.SchemaID,
		Old:     dumpdiff.DumpRef{Path: "/private/old-path-canary", ManifestSchema: "zscalerctl.dump.manifest.v2", Redaction: "standard", Status: "complete"},
		New:     dumpdiff.DumpRef{Path: "/private/new-path-canary", ManifestSchema: "zscalerctl.dump.manifest.v2", Redaction: "standard", Status: "complete"},
		Summary: dumpdiff.Summary{ResourcesCompared: 1, ResourcesWithDrift: 1, RecordsAdded: 1, RecordsChanged: 1},
		Resources: []dumpdiff.ResourceDiff{{
			Product: "zia", Resource: "locations", Identity: dumpdiff.Identity{Mode: "get_key", Field: "id"},
			Added: []dumpdiff.RecordRef{{Key: "2", Record: map[string]any{"id": json.Number("9007199254740993")}}},
			Changed: []dumpdiff.RecordChange{{Key: "1", Changes: []dumpdiff.FieldChange{
				{Field: "name", Old: "old", New: "new"},
				{Field: "enabled", Old: false, New: true},
			}}},
		}},
	}
	converted, err := adapter.ToDiffResult(machine.NewDiffResult(report))
	if err != nil {
		t.Fatalf("ToDiffResult() error = %v", err)
	}
	if got, want := len(converted.Items), 4; got != want {
		t.Fatalf("diff items = %d, want %d", got, want)
	}
	if converted.Summary.StreamItemsEmitted != 4 || !converted.Summary.HasDrift {
		t.Fatalf("diff summary = %#v", converted.Summary)
	}
	body, err := json.Marshal(converted)
	if err != nil {
		t.Fatalf("json.Marshal(diff conversion) error = %v", err)
	}
	for _, forbidden := range []string{"/private/old-path-canary", "/private/new-path-canary", `"path"`} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("diff conversion leaked %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(string(body), "9007199254740993") {
		t.Fatalf("diff conversion lost exact number: %s", body)
	}
}

func TestErrorConverterNeverCarriesMessagesPathsOrUnknownCredentialNames(t *testing.T) {
	t.Parallel()

	canary := "/private/backend-error-canary"
	converted := adapter.ToOperationError(&machine.MachineError{
		Kind: machine.ErrorKindMissingCredentials, Message: canary,
		Missing: []string{"ZSCALERCTL_CLIENT_ID", "ZSCALERCTL_PROXY_URL", "ZSCALERCTL_CLIENT_ID"},
		Product: canary, Resource: canary,
	})
	missing, ok := converted.Failure.(enginewire.MissingCredentialsFailure)
	if !ok || converted.Canceled {
		t.Fatalf("ToOperationError(missing) = %#v", converted)
	}
	if got, want := missing.Missing, []string{"ZSCALERCTL_CLIENT_ID"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("missing names = %#v, want %#v", got, want)
	}
	failed := enginewire.Failed[enginewire.MissingCredentialsFailure]{Type: "failed", ID: 1, Sequence: 2, Error: missing}
	body, err := enginewire.MarshalServerFrame(failed)
	if err != nil {
		t.Fatalf("MarshalServerFrame(failed) error = %v", err)
	}
	if strings.Contains(string(body), canary) || strings.Contains(string(body), "PROXY") {
		t.Fatalf("wire failure leaked source context: %s", body)
	}

	if got := adapter.ToOperationError(context.Canceled); !got.Canceled || got.Failure != nil {
		t.Fatalf("ToOperationError(canceled) = %#v", got)
	}
	deadline := adapter.ToOperationError(context.DeadlineExceeded)
	if failure, ok := deadline.Failure.(enginewire.NonCredentialFailure); !ok || failure.Kind != enginewire.FailureDeadlineExceeded {
		t.Fatalf("ToOperationError(deadline) = %#v", deadline)
	}
	unknown := adapter.ToOperationError(errors.New(canary))
	if failure, ok := unknown.Failure.(enginewire.NonCredentialFailure); !ok || failure.Kind != enginewire.FailureInternal {
		t.Fatalf("ToOperationError(unknown) = %#v", unknown)
	}
}

func adapterTestSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product: resources.ProductZIA, Name: "locations", Shape: resources.ShapeList,
		Operations: resources.ReadOperations(), GetKey: "id",
		Fields: []resources.FieldSpec{{
			Name: "id", Classification: resources.ClassOperational, AllowedModes: []redact.Mode{redact.ModeStandard},
		}},
	}
}
