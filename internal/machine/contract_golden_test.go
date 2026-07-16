package machine_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestMachineContractGoldenRequestFixturesExecute(t *testing.T) {
	tests := []struct {
		name            string
		fixture         string
		executor        machine.Executor
		wantRecords     []map[string]any
		responseFixture string
	}{
		{
			name:    "list",
			fixture: "request-list.json",
			executor: contractExecutor(
				projectedRecordsFromFields(t,
					map[string]any{"id": "loc-1", "name": "HQ", "status": "active"},
					map[string]any{"id": "loc-2", "name": "Branch", "status": "inactive"},
				),
				resources.ProjectedRecords{},
			),
			wantRecords: []map[string]any{
				{"id": "loc-1", "name": "HQ"},
			},
			responseFixture: "response-records.json",
		},
		{
			name:    "get",
			fixture: "request-get.json",
			executor: contractExecutor(
				resources.ProjectedRecords{},
				projectedRecordsFromFields(t,
					map[string]any{"id": "loc-1", "name": "HQ", "status": "active"},
				),
			),
			wantRecords: []map[string]any{
				{"id": "loc-1", "name": "HQ"},
			},
		},
		{
			name:    "show",
			fixture: "request-show.json",
			executor: contractExecutor(
				projectedRecordsFromFields(t,
					map[string]any{"id": "settings", "name": "Advanced Settings"},
				),
				resources.ProjectedRecords{},
			),
			wantRecords: []map[string]any{
				{"id": "settings", "name": "Advanced Settings"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := decodeGoldenRequest(t, tt.fixture)
			got, err := tt.executor.Execute(context.Background(), req)
			if err != nil {
				t.Fatalf("Executor.Execute(%s request fixture) error = %v, want nil", tt.name, err)
			}
			if !reflect.DeepEqual(got.Records, tt.wantRecords) {
				t.Fatalf("Executor.Execute(%s request fixture).Records = %#v, want %#v",
					tt.name, got.Records, tt.wantRecords)
			}
			if tt.responseFixture != "" {
				assertGoldenJSON(t, tt.responseFixture, got)
			}
		})
	}
}

func TestMachineContractGoldenErrorKindFixtures(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		executor machine.Executor
		request  machine.Request
	}{
		{
			name:    "usage",
			fixture: "machine-error.json",
			executor: contractExecutor(
				resources.ProjectedRecords{},
				resources.ProjectedRecords{},
			),
			request: machine.Request{
				RequestID:  "contract-missing-id",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationGet,
				Input:      &machine.Input{Product: "zia", Resource: "locations"},
			},
		},
		{
			name:     "unsupported_capability",
			fixture:  "machine-error-unsupported-capability.json",
			executor: machine.Executor{Catalog: contractCatalog()},
			request: machine.Request{
				RequestID:  "contract-unsupported-capability",
				Capability: "inventory.read",
				Operation:  machine.OperationList,
				Input:      &machine.Input{Product: "zia", Resource: "locations"},
			},
		},
		{
			name:     "unsupported_operation",
			fixture:  "machine-error-unsupported-operation.json",
			executor: machine.Executor{Catalog: contractCatalog()},
			request: machine.Request{
				RequestID:  "contract-unsupported-operation",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.Operation("delete"),
				Input:      &machine.Input{Product: "zia", Resource: "locations"},
			},
		},
		{
			name:    "unknown_resource",
			fixture: "machine-error-unknown-resource.json",
			executor: machine.Executor{
				Browser: &fakeBrowserLoader{err: resources.ErrUnknownResource},
				Catalog: contractCatalog(),
			},
			request: machine.Request{
				RequestID:  "contract-unknown-resource",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationList,
				Input:      &machine.Input{Product: "zia", Resource: "missing-resource"},
			},
		},
		{
			name:    "not_found",
			fixture: "machine-error-not-found.json",
			executor: machine.Executor{
				Browser:   &fakeBrowserLoader{getErr: resources.ErrRecordNotFound},
				Catalog:   contractCatalog(),
				Redaction: redact.ModeStandard,
			},
			request: machine.Request{
				RequestID:  "contract-record-not-found",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationGet,
				Input:      &machine.Input{Product: "zia", Resource: "locations", RecordID: "loc-missing"},
			},
		},
		{
			name:    "invalid_resource_id",
			fixture: "machine-error-invalid-resource-id.json",
			executor: machine.Executor{
				Browser:   &fakeBrowserLoader{getErr: resources.ErrInvalidResourceID},
				Catalog:   contractCatalog(),
				Redaction: redact.ModeStandard,
			},
			request: machine.Request{
				RequestID:  "contract-invalid-resource-id",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationGet,
				Input:      &machine.Input{Product: "zia", Resource: "locations", RecordID: "invalid-id"},
			},
		},
		{
			name:    "live_access_failed",
			fixture: "machine-error-live-access-failed.json",
			executor: machine.Executor{
				Browser: &fakeBrowserLoader{err: errors.New("raw backend failure")},
				Catalog: contractCatalog(),
			},
			request: machine.Request{
				RequestID:  "contract-live-access-failed",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationList,
				Input:      &machine.Input{Product: "zia", Resource: "locations"},
			},
		},
		{
			name:    "canceled",
			fixture: "machine-error-canceled.json",
			executor: machine.Executor{
				Browser: &fakeBrowserLoader{err: context.Canceled},
				Catalog: contractCatalog(),
			},
			request: machine.Request{
				RequestID:  "contract-canceled",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationList,
				Input:      &machine.Input{Product: "zia", Resource: "locations"},
			},
		},
		{
			name:    "deadline_exceeded",
			fixture: "machine-error-deadline-exceeded.json",
			executor: machine.Executor{
				Browser: &fakeBrowserLoader{err: context.DeadlineExceeded},
				Catalog: contractCatalog(),
			},
			request: machine.Request{
				RequestID:  "contract-deadline-exceeded",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationList,
				Input:      &machine.Input{Product: "zia", Resource: "locations"},
			},
		},
		{
			name:     "internal",
			fixture:  "machine-error-internal.json",
			executor: machine.Executor{Catalog: contractCatalog()},
			request: machine.Request{
				RequestID:  "contract-internal",
				Capability: machine.CapabilityResourcesRead,
				Operation:  machine.OperationList,
				Input:      &machine.Input{Product: "zia", Resource: "locations"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.executor.Execute(context.Background(), tt.request)
			var machineErr *machine.MachineError
			if !errors.As(err, &machineErr) {
				t.Fatalf("Executor.Execute(%s fixture request) error = %T %v, want *MachineError", tt.name, err, err)
			}
			if got.Error == nil {
				t.Fatalf("Executor.Execute(%s fixture request).Error = nil, want MachineError response", tt.name)
			}
			assertGoldenJSON(t, tt.fixture, *machineErr)
		})
	}
}

func TestMachineContractDocsListStableErrorKinds(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "cli", "machine-contract.md"))
	if err != nil {
		t.Fatalf("read machine-contract.md: %v", err)
	}
	doc := string(body)
	start := strings.Index(doc, "representative fixtures for the current stable kinds:")
	end := strings.Index(doc, "Further taxonomy changes")
	if start < 0 || end <= start {
		t.Fatalf("machine-contract.md stable-kind paragraph not found")
	}
	taxonomy := doc[start:end]
	for _, kind := range []string{
		machine.ErrorKindUsage,
		machine.ErrorKindUnsupportedCapability,
		machine.ErrorKindUnsupportedOperation,
		machine.ErrorKindUnknownResource,
		machine.ErrorKindNotFound,
		machine.ErrorKindInvalidResourceID,
		machine.ErrorKindLiveAccessFailed,
		machine.ErrorKindCanceled,
		machine.ErrorKindDeadlineExceeded,
		machine.ErrorKindInternal,
	} {
		if !strings.Contains(taxonomy, "`"+kind+"`") {
			t.Errorf("machine-contract.md stable-kind paragraph omits %q", kind)
		}
	}
	const invalidIDRow = "| Invalid resource id | `invalid_resource_id` | `invalid_resource_id` | `2` | `resources.ErrInvalidResourceID`; `zscaler.ErrInvalidResourceID` |"
	if !strings.Contains(doc, invalidIDRow) {
		t.Errorf("machine-contract.md invalid-resource-ID row does not match the implemented machine/envelope/sentinel vocabulary")
	}
}

func TestMachineContractGoldenManifestFixture(t *testing.T) {
	catalog := contractCatalog()
	manifest := machine.ManifestFromCatalog(catalog)
	assertGoldenJSON(t, "manifest.json", manifest)

	resp, err := (machine.Executor{Catalog: catalog}).Execute(context.Background(), machine.Request{
		RequestID: "contract-manifest",
		Operation: machine.OperationManifest,
	})
	if err != nil {
		t.Fatalf("Executor.Execute(manifest request) error = %v, want nil", err)
	}
	if !reflect.DeepEqual(resp.Manifest, &manifest) {
		t.Fatalf("Executor.Execute(manifest request).Manifest = %#v, want %#v", resp.Manifest, &manifest)
	}
}

func contractExecutor(records resources.ProjectedRecords, getRecords resources.ProjectedRecords) machine.Executor {
	return machine.Executor{
		Browser: &fakeBrowserLoader{
			records:    records,
			getRecords: getRecords,
		},
		Catalog:   contractCatalog(),
		Redaction: redact.ModeStandard,
	}
}

func contractCatalog() resources.ResourceCatalog {
	return resources.ResourceCatalog{
		testExecutorSpec(resources.ProductZIA, "advanced-settings", resources.ShowOperation(), "id", "name"),
		testExecutorSpec(resources.ProductZIA, "locations", resources.ReadOperations(), "id", "name", "status"),
	}
}

func decodeGoldenRequest(t *testing.T, name string) machine.Request {
	t.Helper()
	body := readGolden(t, name)
	var req machine.Request
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v; body %s", name, err, body)
	}
	assertGoldenJSON(t, name, req)
	return req
}

func assertGoldenJSON(t *testing.T, name string, value any) {
	t.Helper()
	got, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(%s, %T) error = %v, want nil", name, value, err)
	}
	got = append(got, '\n')
	want := readGolden(t, name)
	if !bytes.Equal(got, want) {
		t.Fatalf("golden fixture %s mismatch\nwant:\n%s\ngot:\n%s", name, want, got)
	}
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "contract", name))
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v, want nil", name, err)
	}
	return body
}
