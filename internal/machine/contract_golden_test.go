package machine_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

func TestMachineContractGoldenErrorFixture(t *testing.T) {
	req := machine.Request{
		RequestID:  "contract-missing-id",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}
	executor := contractExecutor(resources.ProjectedRecords{}, resources.ProjectedRecords{})

	got, err := executor.Execute(context.Background(), req)
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) {
		t.Fatalf("Executor.Execute(missing id) error = %T %v, want *MachineError", err, err)
	}
	if got.Error == nil {
		t.Fatalf("Executor.Execute(missing id).Error = nil, want MachineError response")
	}
	assertGoldenJSON(t, "machine-error.json", *machineErr)
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
