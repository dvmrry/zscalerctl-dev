package enginehost

import (
	"context"
	"reflect"
	"testing"

	dumpdiff "github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/enginewire/adapter"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestExecuteOperationCoversConfigFreeAndStatusFamilies(t *testing.T) {
	t.Parallel()

	spec := hostListSpec()
	machineManifest := machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec})
	wireManifest, err := adapter.ToEngineManifest(machineManifest)
	if err != nil {
		t.Fatalf("ToEngineManifest() error = %v", err)
	}
	engine := &fakeEngine{
		manifest: machineManifest,
		discover: func(context.Context, machine.CatalogRequest) (machine.CatalogResult, error) {
			return machine.NewCatalogResult(resources.ResourceCatalog{spec}), nil
		},
		status: func(_ context.Context, request machine.StatusRequest) (machine.StatusResult, error) {
			switch request.Operation {
			case machine.OperationDoctor:
				return machine.NewDoctorStatusResult(machine.DoctorStatus{
					Status: "OK", Mode: "read-only", Config: "environment", AuthMode: "oneapi",
					Redaction: "standard", Timeout: "30s", Cache: "enabled", Proxy: "not configured",
					Credentials: "configured", LiveAPI: "available",
				}), nil
			case machine.OperationAuthStatus:
				return machine.NewAuthStatusResult(machine.AuthStatus{
					Credentials: "configured", CredentialExchange: "not requested", LiveAPI: "available",
				}), nil
			case machine.OperationConfigStatus:
				return machine.NewConfigStatusResult(machine.ConfigStatus{
					Source: "environment", AuthMode: "oneapi", VanityDomainSet: true,
					Credentials: machine.ConfigCredentialStatus{ClientIDSet: true, ClientSecretSet: true},
					Defaults:    machine.ConfigDefaultsStatus{Redaction: "standard"},
				}), nil
			default:
				t.Fatalf("unexpected status operation %q", request.Operation)
				return machine.StatusResult{}, nil
			}
		},
	}
	tests := []struct {
		name       string
		frame      enginewire.ClientFrame
		resultType any
		items      int
	}{
		{
			name:       "manifest",
			frame:      enginewire.ManifestRequest{Type: "request", ID: 1, Capability: enginewire.CapabilityEngineManifest, Operation: enginewire.OperationManifest},
			resultType: enginewire.EngineManifestResult{},
		},
		{
			name:       "catalog",
			frame:      enginewire.CatalogRequest{Type: "request", ID: 2, Capability: enginewire.CapabilityCatalogSchema, Operation: enginewire.OperationList},
			resultType: enginewire.CatalogSummary{}, items: 1,
		},
		{
			name:       "doctor",
			frame:      enginewire.DoctorRequest{Type: "request", ID: 3, Capability: enginewire.CapabilityStatusInspect, Operation: enginewire.OperationDoctor},
			resultType: enginewire.DoctorStatusResult{},
		},
		{
			name:       "auth",
			frame:      enginewire.AuthStatusRequest{Type: "request", ID: 4, Capability: enginewire.CapabilityStatusInspect, Operation: enginewire.OperationAuthStatus},
			resultType: enginewire.AuthStatusResult{},
		},
		{
			name:       "config",
			frame:      enginewire.ConfigStatusRequest{Type: "request", ID: 5, Capability: enginewire.CapabilityStatusInspect, Operation: enginewire.OperationConfigStatus},
			resultType: enginewire.ConfigStatusResult{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, ok := requestFromFrame(tt.frame)
			if !ok {
				t.Fatalf("requestFromFrame(%T) ok = false", tt.frame)
			}
			data, err := executeOperation(context.Background(), engine, request, wireManifest, func(provisionalFrame) error {
				t.Fatal("unexpected provisional event")
				return nil
			})
			if err != nil {
				t.Fatalf("executeOperation() error = %v", err)
			}
			if got := reflect.TypeOf(data.result); got != reflect.TypeOf(tt.resultType) || len(data.items) != tt.items {
				t.Fatalf("operation data result=%T items=%d, want %T/%d", data.result, len(data.items), tt.resultType, tt.items)
			}
			if _, err := preflightSuccess(context.Background(), request, data); err != nil {
				t.Fatalf("preflightSuccess() error = %v", err)
			}
		})
	}
}

func TestExecuteOperationCoversURLAndEveryResourceRead(t *testing.T) {
	t.Parallel()

	spec := hostListSpec()
	machineManifest := machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec})
	wireManifest, err := adapter.ToEngineManifest(machineManifest)
	if err != nil {
		t.Fatalf("ToEngineManifest() error = %v", err)
	}
	engine := &fakeEngine{
		manifest: machineManifest,
		lookup: func(_ context.Context, request machine.URLLookupRequest) (machine.URLLookupResult, error) {
			if got, want := request.URLs, []string{"https://example.test/"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("lookup URLs = %#v, want %#v", got, want)
			}
			return machine.NewURLLookupResult([]machine.URLClassification{{
				URL: "https://example.test/", Classifications: []string{}, SecurityAlertClassifications: []string{}, Application: "",
			}}), nil
		},
		read: func(_ context.Context, request machine.ResourceReadRequest) (machine.ResourceReadResult, error) {
			if !machine.IsResourceReadOperation(request.Operation) {
				t.Fatalf("read operation = %q", request.Operation)
			}
			records := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{"id": "1"}})
			return machine.NewResourceReadResult(records), nil
		},
	}
	frames := []enginewire.ClientFrame{
		enginewire.URLLookupRequest{
			Type: "request", ID: 1, Capability: enginewire.CapabilityZIAURLLookup, Operation: enginewire.OperationLookup,
			Input: enginewire.URLLookupInput{URLs: []string{"https://example.test/"}},
		},
		enginewire.ResourceListRequest{
			Type: "request", ID: 2, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationList,
			Input: enginewire.ResourceListInput{Product: enginewire.ProductZIA, Resource: "locations", Fields: []string{}, Filters: []enginewire.Filter{}, Search: ""},
		},
		enginewire.ResourceGetRequest{
			Type: "request", ID: 3, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationGet,
			Input: enginewire.ResourceGetInput{Product: enginewire.ProductZIA, Resource: "locations", RecordID: "1", Fields: []string{}},
		},
		enginewire.ResourceShowRequest{
			Type: "request", ID: 4, Capability: enginewire.CapabilityResourcesRead, Operation: enginewire.OperationShow,
			Input: enginewire.ResourceShowInput{Product: enginewire.ProductZIA, Resource: "locations", Fields: []string{}},
		},
	}
	for _, frame := range frames {
		request, ok := requestFromFrame(frame)
		if !ok {
			t.Fatalf("requestFromFrame(%T) ok = false", frame)
		}
		data, err := executeOperation(context.Background(), engine, request, wireManifest, func(provisionalFrame) error {
			t.Fatal("unexpected provisional event")
			return nil
		})
		if err != nil {
			t.Fatalf("executeOperation(%T) error = %v", frame, err)
		}
		if len(data.items) != 1 {
			t.Fatalf("executeOperation(%T) items = %d, want 1", frame, len(data.items))
		}
		if _, err := preflightSuccess(context.Background(), request, data); err != nil {
			t.Fatalf("preflightSuccess(%T) error = %v", frame, err)
		}
	}
}

func TestExecuteOperationCoversDumpAndDiffEventFamilies(t *testing.T) {
	t.Parallel()

	spec := hostListSpec()
	machineManifest := machine.EngineManifestFromCatalog(resources.ResourceCatalog{spec})
	wireManifest, err := adapter.ToEngineManifest(machineManifest)
	if err != nil {
		t.Fatalf("ToEngineManifest() error = %v", err)
	}
	record := resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{"id": "1"}}).Records()[0]
	report := dumpdiff.Report{
		Schema:  dumpdiff.SchemaID,
		Old:     dumpdiff.DumpRef{Path: "/private/old", ManifestSchema: "zscalerctl.dump.manifest.v2", Redaction: "standard", Status: "complete"},
		New:     dumpdiff.DumpRef{Path: "/private/new", ManifestSchema: "zscalerctl.dump.manifest.v2", Redaction: "standard", Status: "complete"},
		Summary: dumpdiff.Summary{ResourcesCompared: 1, ResourcesWithDrift: 1, RecordsAdded: 1},
		Resources: []dumpdiff.ResourceDiff{{
			Product: "zia", Resource: "locations", Identity: dumpdiff.Identity{Mode: "get_key", Field: "id"},
			Added: []dumpdiff.RecordRef{{Key: "1", Record: map[string]any{"id": "1"}}},
		}},
	}
	engine := &fakeEngine{
		manifest: machineManifest,
		dump: func(_ context.Context, _ machine.DumpRequest, sink machine.EventSink) (machine.DumpResult, error) {
			for _, event := range []machine.Event{
				{Kind: machine.EventStarted, Total: 1},
				{Kind: machine.EventProgress, Done: 1, Total: 1, Product: "zia", Resource: "locations"},
				{Kind: machine.EventRecord, Product: "zia", Resource: "locations", Record: &record},
				{Kind: machine.EventCompleted, Records: 1, Resources: 1},
			} {
				if err := sink(event); err != nil {
					return machine.DumpResult{}, err
				}
			}
			return machine.NewDumpResult(1, 1, "standard", []machine.DumpResourceError{}), nil
		},
		diff: func(_ context.Context, _ machine.DiffRequest, sink machine.EventSink) (machine.DiffResult, error) {
			for _, event := range []machine.Event{
				{Kind: machine.EventStarted, Total: 1},
				{Kind: machine.EventProgress, Done: 1, Total: 1, Product: "zia", Resource: "locations"},
				{Kind: machine.EventCompleted, Resources: 1},
			} {
				if err := sink(event); err != nil {
					return machine.DiffResult{}, err
				}
			}
			return machine.NewDiffResult(report), nil
		},
	}
	tests := []struct {
		name        string
		frame       enginewire.ClientFrame
		items       int
		provisional int
	}{
		{
			name: "dump",
			frame: enginewire.DumpRequest{
				Type: "request", ID: 1, Capability: enginewire.CapabilityDumpWrite, Operation: enginewire.OperationDump,
				Input: enginewire.DumpInput{OutputDir: "out", Products: []enginewire.Product{}, Resources: []enginewire.ResourceSelector{}},
			},
			provisional: 1,
		},
		{
			name: "diff",
			frame: enginewire.DiffRequest{
				Type: "request", ID: 2, Capability: enginewire.CapabilityDiffCompare, Operation: enginewire.OperationDiff,
				Input: enginewire.DiffInput{OldDir: "old", NewDir: "new", Products: []enginewire.Product{}, Resources: []enginewire.ResourceSelector{}},
			},
			items: 2, provisional: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request, ok := requestFromFrame(tt.frame)
			if !ok {
				t.Fatalf("requestFromFrame(%T) ok = false", tt.frame)
			}
			var provisional []provisionalFrame
			data, err := executeOperation(context.Background(), engine, request, wireManifest, func(frame provisionalFrame) error {
				provisional = append(provisional, frame)
				return nil
			})
			if err != nil {
				t.Fatalf("executeOperation() error = %v", err)
			}
			if len(data.items) != tt.items || len(provisional) != tt.provisional {
				t.Fatalf("operation data items=%d provisional=%d, want %d/%d", len(data.items), len(provisional), tt.items, tt.provisional)
			}
			if _, err := preflightSuccess(context.Background(), request, data); err != nil {
				t.Fatalf("preflightSuccess() error = %v", err)
			}
		})
	}
}
