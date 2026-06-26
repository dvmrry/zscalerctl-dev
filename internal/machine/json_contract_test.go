package machine_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
)

func TestMachineContractRequestJSONRoundTrip(t *testing.T) {
	want := machine.Request{
		RequestID:  "req-get-location",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Input: &machine.Input{
			Product:  "zia",
			Resource: "locations",
			RecordID: "loc-1",
			Fields:   []string{"id", "name", "status"},
			Filters: []machine.Filter{
				{Field: "status", Operator: "=", Value: "active"},
			},
			Search:  "hq",
			Options: map[string]string{"encoding": "json"},
		},
		Meta: &machine.Meta{Version: "1", RequestID: "req-get-location"},
	}

	assertMachineJSONRoundTrip(t, "machine.Request", want)

	body := decodeJSONMap(t, want)
	input, ok := body["input"].(map[string]any)
	if !ok {
		t.Fatalf("json.Marshal(machine.Request).input = %T, want object; body %#v", body["input"], body)
	}
	if got, want := input["record_id"], "loc-1"; got != want {
		t.Fatalf("json.Marshal(machine.Request).input.record_id = %#v, want %#v", got, want)
	}
	if _, ok := body["record_id"]; ok {
		t.Fatalf("json.Marshal(machine.Request) = %#v, want record_id nested under input only", body)
	}
}

func TestMachineContractResponseJSONRoundTrip(t *testing.T) {
	want := machine.Response{
		RequestID:  "req-get-location",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Records: []map[string]any{
			{
				"id":     "loc-1",
				"name":   "HQ",
				"status": "active",
				"groups": []any{"admins", "network"},
			},
		},
		Meta: &machine.Meta{
			Version:   "1",
			RequestID: "req-get-location",
			Product:   "zia",
			Resource:  "locations",
			ReadOnly:  true,
			Count:     1,
		},
	}

	assertMachineJSONRoundTrip(t, "machine.Response", want)

	body := decodeJSONMap(t, want)
	if _, ok := body["manifest"]; ok {
		t.Fatalf("json.Marshal(machine.Response) = %#v, want empty manifest omitted", body)
	}
	if _, ok := body["error"]; ok {
		t.Fatalf("json.Marshal(machine.Response) = %#v, want empty error omitted", body)
	}
}

func TestMachineContractErrorJSONRoundTrip(t *testing.T) {
	want := machine.MachineError{
		Kind:      machine.ErrorKindUsage,
		Message:   "missing required input: input.record_id",
		Missing:   []string{"input.record_id"},
		Operation: machine.OperationGet,
		Product:   "zia",
		Resource:  "locations",
	}

	assertMachineJSONRoundTrip(t, "machine.MachineError", want)

	resp := machine.Response{
		RequestID:  "req-error",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Error:      &want,
		Meta: &machine.Meta{
			RequestID: "req-error",
			Product:   "zia",
			Resource:  "locations",
			ReadOnly:  true,
		},
	}
	got := assertMachineJSONRoundTrip(t, "machine.Response with MachineError", resp)
	if got.Error == nil || !reflect.DeepEqual(*got.Error, want) {
		t.Fatalf("json round trip(machine.Response with MachineError).Error = %#v, want %#v", got.Error, want)
	}
}

func TestMachineContractManifestJSONRoundTripWithResourceReadExample(t *testing.T) {
	want := machine.Manifest{
		Version: machine.ManifestVersion,
		Capabilities: []machine.Capability{
			{
				Name:        machine.CapabilityResourcesRead,
				Title:       "Read zia/locations",
				Description: "Read projected and redacted zia/locations resource records.",
				Operations: []machine.Operation{
					machine.OperationList,
					machine.OperationGet,
					machine.OperationShow,
				},
				Input: &machine.Input{Product: "zia", Resource: "locations"},
				Output: &machine.SchemaRef{
					Name:    machine.ProjectedRecordsSchemaName,
					Version: machine.ProjectedRecordsSchemaVersion,
				},
				Examples: []machine.Example{
					{
						Name:        "get location",
						Description: "Read one projected location record by ID.",
						Request: machine.Request{
							RequestID:  "example-get-location",
							Capability: machine.CapabilityResourcesRead,
							Operation:  machine.OperationGet,
							Input: &machine.Input{
								Product:  "zia",
								Resource: "locations",
								RecordID: "loc-1",
							},
						},
						Response: &machine.Response{
							RequestID:  "example-get-location",
							Capability: machine.CapabilityResourcesRead,
							Operation:  machine.OperationGet,
							Records: []map[string]any{
								{"id": "loc-1", "name": "HQ", "status": "active"},
							},
							Meta: &machine.Meta{
								RequestID: "example-get-location",
								Product:   "zia",
								Resource:  "locations",
								ReadOnly:  true,
								Count:     1,
							},
						},
					},
				},
				Meta: &machine.Meta{
					Product:  "zia",
					Resource: "locations",
					Shape:    "collection",
					GetKey:   "id",
					ReadOnly: true,
				},
			},
		},
		Schemas: []machine.SchemaRef{machine.ProjectedRecordsSchemaRef()},
		Meta: &machine.Meta{
			Version:  "1",
			ReadOnly: true,
			Count:    1,
		},
	}

	got := assertMachineJSONRoundTrip(t, "machine.Manifest", want)
	if len(got.Capabilities) != 1 || len(got.Capabilities[0].Examples) != 1 {
		t.Fatalf("json round trip(machine.Manifest).Capabilities = %#v, want one capability with one example",
			got.Capabilities)
	}
	example := got.Capabilities[0].Examples[0]
	if example.Request.Input == nil || example.Request.Input.RecordID != "loc-1" {
		t.Fatalf("json round trip(machine.Manifest) example request input = %#v, want record_id loc-1",
			example.Request.Input)
	}
	if example.Response == nil || len(example.Response.Records) != 1 {
		t.Fatalf("json round trip(machine.Manifest) example response = %#v, want one projected record",
			example.Response)
	}
}

func assertMachineJSONRoundTrip[T any](t *testing.T, label string, want T) T {
	t.Helper()
	body, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("json.Marshal(%s) error = %v, want nil", label, err)
	}
	var got T
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal(json.Marshal(%s)) error = %v; body %s", label, err, body)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("json round trip(%s) = %#v, want %#v; body %s", label, got, want, body)
	}
	return got
}
