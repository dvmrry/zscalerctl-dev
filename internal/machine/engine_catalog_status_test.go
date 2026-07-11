package machine_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestExecutorDiscoverCatalogIsConfigFreeAndDefensive(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{{
			Name:           "id",
			Classification: resources.ClassOperational,
			AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			Fields: []resources.FieldSpec{{
				Name:           "nested",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			}},
		}},
	}}
	result, err := (machine.Executor{Catalog: catalog}).DiscoverCatalog(
		context.Background(),
		machine.CatalogRequest{RequestID: "catalog-1"},
	)
	if err != nil {
		t.Fatalf("Executor.DiscoverCatalog() error = %v, want nil", err)
	}

	catalog[0].Name = "mutated-source"
	catalog[0].Operations[0].Name = "mutated-source"
	catalog[0].Fields[0].AllowedModes[0] = redact.ModeParanoid
	catalog[0].Fields[0].Fields[0].Name = "mutated-source"
	first := result.Catalog()
	if first[0].Name != "locations" ||
		first[0].Operations[0].Name != "list" ||
		first[0].Fields[0].AllowedModes[0] != redact.ModeStandard ||
		first[0].Fields[0].Fields[0].Name != "nested" {
		t.Fatalf("CatalogResult after source mutation = %#v, want original deep snapshot", first)
	}

	first[0].Name = "mutated-result"
	first[0].Operations[0].Name = "mutated-result"
	first[0].Fields[0].AllowedModes[0] = redact.ModeParanoid
	first[0].Fields[0].Fields[0].Name = "mutated-result"
	second := result.Catalog()
	if second[0].Name != "locations" ||
		second[0].Operations[0].Name != "list" ||
		second[0].Fields[0].AllowedModes[0] != redact.ModeStandard ||
		second[0].Fields[0].Fields[0].Name != "nested" {
		t.Fatalf("CatalogResult after returned-value mutation = %#v, want original deep snapshot", second)
	}
}

func TestExecutorDiscoverCatalogRejectsCanceledAndMutatingCatalog(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (machine.Executor{}).DiscoverCatalog(ctx, machine.CatalogRequest{})
	machineErr := assertMachineError(
		t,
		err,
		machine.ErrorKindCanceled,
		machine.OperationList,
		"",
		"",
	)
	if strings.Contains(machineErr.Message, context.Canceled.Error()) {
		t.Fatalf("DiscoverCatalog(canceled) message = %q, want static message", machineErr.Message)
	}

	mutating := resources.ResourceCatalog{{
		Product: resources.ProductZIA,
		Name:    "mutating",
		Operations: []resources.Operation{{
			Name: "update", Capability: resources.CapabilityWrite,
		}},
	}}
	_, err = (machine.Executor{Catalog: mutating}).DiscoverCatalog(
		context.Background(),
		machine.CatalogRequest{},
	)
	assertMachineError(
		t,
		err,
		machine.ErrorKindUnsupportedOperation,
		machine.OperationList,
		"",
		"",
	)
	if !errors.Is(err, resources.ErrMutatingOperation) {
		t.Fatalf("DiscoverCatalog(mutating) error = %v, want ErrMutatingOperation identity", err)
	}
}

func TestCatalogAndStatusEngineValuesRejectDirectJSON(t *testing.T) {
	t.Parallel()

	values := []struct {
		name  string
		value any
	}{
		{name: "catalog request", value: machine.CatalogRequest{RequestID: "must-not-render"}},
		{
			name: "catalog result",
			value: machine.NewCatalogResult(resources.ResourceCatalog{{
				Product: resources.ProductZIA, Name: "must-not-render",
			}}),
		},
		{
			name: "status request",
			value: machine.StatusRequest{
				RequestID: "must-not-render", Operation: machine.OperationDoctor,
			},
		},
		{
			name: "status result",
			value: machine.NewDoctorStatusResult(machine.DoctorStatus{
				Profile: "must-not-render",
			}),
		},
	}
	for _, tt := range values {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.value)
			if err == nil {
				t.Fatalf("json.Marshal(%s) error = nil; body = %s, want no wire format", tt.name, body)
			}
			if strings.Contains(string(body), "must-not-render") {
				t.Fatalf("json.Marshal(%s) body = %q, want no value bytes", tt.name, body)
			}
		})
	}

	var catalogRequest machine.CatalogRequest
	if err := json.Unmarshal([]byte(`{"RequestID":"must-not-render"}`), &catalogRequest); err == nil {
		t.Fatalf("json.Unmarshal(CatalogRequest) error = nil; request = %#v", catalogRequest)
	}
	var catalogResult machine.CatalogResult
	if err := json.Unmarshal([]byte(`{"catalog":[]}`), &catalogResult); err == nil {
		t.Fatalf("json.Unmarshal(CatalogResult) error = nil; result = %#v", catalogResult)
	}
	var statusRequest machine.StatusRequest
	if err := json.Unmarshal([]byte(`{"Operation":"doctor"}`), &statusRequest); err == nil {
		t.Fatalf("json.Unmarshal(StatusRequest) error = nil; request = %#v", statusRequest)
	}
	var statusResult machine.StatusResult
	if err := json.Unmarshal([]byte(`{"doctor":{}}`), &statusResult); err == nil {
		t.Fatalf("json.Unmarshal(StatusResult) error = nil; result = %#v", statusResult)
	}
}

func TestStatusResultAccessorsAreClosed(t *testing.T) {
	t.Parallel()

	doctor := machine.DoctorStatus{Status: "OK", Profile: "default"}
	auth := machine.AuthStatus{Credentials: "configured"}
	config := machine.ConfigStatus{Profile: "default"}
	tests := []struct {
		name      string
		result    machine.StatusResult
		operation machine.Operation
	}{
		{name: "doctor", result: machine.NewDoctorStatusResult(doctor), operation: machine.OperationDoctor},
		{name: "auth", result: machine.NewAuthStatusResult(auth), operation: machine.OperationAuthStatus},
		{name: "config", result: machine.NewConfigStatusResult(config), operation: machine.OperationConfigStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Operation() != tt.operation {
				t.Fatalf("StatusResult.Operation() = %q, want %q", tt.result.Operation(), tt.operation)
			}
			gotDoctor, doctorOK := tt.result.Doctor()
			gotAuth, authOK := tt.result.Auth()
			gotConfig, configOK := tt.result.Config()
			switch tt.operation {
			case machine.OperationDoctor:
				if !doctorOK || authOK || configOK || !reflect.DeepEqual(gotDoctor, doctor) {
					t.Fatalf("doctor StatusResult accessors = %#v/%t %#v/%t %#v/%t", gotDoctor, doctorOK, gotAuth, authOK, gotConfig, configOK)
				}
			case machine.OperationAuthStatus:
				if doctorOK || !authOK || configOK || !reflect.DeepEqual(gotAuth, auth) {
					t.Fatalf("auth StatusResult accessors = %#v/%t %#v/%t %#v/%t", gotDoctor, doctorOK, gotAuth, authOK, gotConfig, configOK)
				}
			case machine.OperationConfigStatus:
				if doctorOK || authOK || !configOK || !reflect.DeepEqual(gotConfig, config) {
					t.Fatalf("config StatusResult accessors = %#v/%t %#v/%t %#v/%t", gotDoctor, doctorOK, gotAuth, authOK, gotConfig, configOK)
				}
			}
		})
	}
}
