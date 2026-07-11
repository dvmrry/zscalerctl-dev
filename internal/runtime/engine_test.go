package runtime

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestNewEngineDefensivelyCopiesHostOptions(t *testing.T) {
	t.Parallel()

	env := []string{"ENGINE_HOST_OPTION=original"}
	catalog := resources.ResourceCatalog{{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{{
			Name:           "id",
			Classification: resources.ClassOperational,
			AllowedModes:   []redact.Mode{redact.ModeStandard},
		}},
	}}
	loadedConfig, err := config.LoadConfig(nil, config.LoadOptions{})
	if err != nil {
		t.Fatalf("config.LoadConfig(engine fixture) error = %v, want nil", err)
	}
	var loaderInputs [][]string
	engine, err := NewEngine(Options{
		Env:     env,
		Catalog: catalog,
		loadConfig: func(got []string, _ config.LoadOptions) (config.Config, error) {
			loaderInputs = append(loaderInputs, append([]string(nil), got...))
			got[0] = "loader-mutated"
			return loadedConfig, nil
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	env[0] = "caller-mutated"
	catalog[0].Name = "caller-mutated"
	catalog[0].Operations[0].Name = "caller-mutated"
	catalog[0].Fields[0].AllowedModes[0] = redact.ModeParanoid

	discovered, err := engine.DiscoverCatalog(context.Background(), machine.CatalogRequest{})
	if err != nil {
		t.Fatalf("Engine.DiscoverCatalog() error = %v, want nil", err)
	}
	gotCatalog := discovered.Catalog()
	if len(gotCatalog) != 1 || gotCatalog[0].Name != "locations" ||
		gotCatalog[0].Operations[0].Name != "list" ||
		gotCatalog[0].Fields[0].AllowedModes[0] != redact.ModeStandard {
		t.Fatalf("Engine.DiscoverCatalog() = %#v, want original options snapshot", gotCatalog)
	}

	for range 2 {
		if _, err := engine.InspectStatus(context.Background(), machine.StatusRequest{
			Operation: machine.OperationAuthStatus,
		}); err != nil {
			t.Fatalf("Engine.InspectStatus() error = %v, want nil", err)
		}
	}
	wantInputs := [][]string{
		{"ENGINE_HOST_OPTION=original"},
		{"ENGINE_HOST_OPTION=original"},
	}
	if !reflect.DeepEqual(loaderInputs, wantInputs) {
		t.Fatalf("Engine status loader inputs = %#v, want fresh copies %#v", loaderInputs, wantInputs)
	}
}

func TestNewEngineRejectsMutatingCatalog(t *testing.T) {
	t.Parallel()

	engine, err := NewEngine(Options{Catalog: resources.ResourceCatalog{{
		Product: resources.ProductZIA,
		Name:    "mutating",
		Operations: []resources.Operation{{
			Name: "update", Capability: resources.CapabilityWrite,
		}},
	}}})
	if engine != nil {
		t.Fatalf("NewEngine(mutating catalog) = %#v, want nil", engine)
	}
	if !errors.Is(err, resources.ErrMutatingOperation) {
		t.Fatalf("NewEngine(mutating catalog) error = %v, want ErrMutatingOperation", err)
	}
}

func TestEngineInspectStatusRejectsUnknownBeforeConfigLoad(t *testing.T) {
	t.Parallel()

	configLoads := 0
	engine, err := NewEngine(Options{
		Catalog: resources.ResourceCatalog{},
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			configLoads++
			return config.Config{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	const canary = "unknown-status-operation-canary"
	_, err = engine.InspectStatus(context.Background(), machine.StatusRequest{
		Operation: machine.Operation(canary),
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindUnsupportedOperation {
		t.Fatalf("Engine.InspectStatus(unknown) error = %v, want unsupported-operation MachineError", err)
	}
	if machineErr.Operation != "" || strings.Contains(err.Error(), canary) {
		t.Fatalf("Engine.InspectStatus(unknown) error = %#v, want no echoed operation", machineErr)
	}
	if configLoads != 0 {
		t.Fatalf("Engine.InspectStatus(unknown) config loads = %d, want 0", configLoads)
	}
}

func TestEngineInspectStatusSanitizesConfigLoaderErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		loadErr      error
		wantKind     string
		wantSentinel error
	}{
		{
			name:         "invalid config",
			loadErr:      fmt.Errorf("%w: /private/status-config-canary.yaml", config.ErrInvalidConfig),
			wantKind:     machine.ErrorKindInvalidConfig,
			wantSentinel: config.ErrInvalidConfig,
		},
		{
			name:     "unexpected loader failure",
			loadErr:  errors.New("run cmd:/private/status-loader-canary"),
			wantKind: machine.ErrorKindInternal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := NewEngine(Options{
				Catalog: resources.ResourceCatalog{},
				loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
					return config.Config{}, tt.loadErr
				},
			})
			if err != nil {
				t.Fatalf("NewEngine() error = %v, want nil", err)
			}
			_, err = engine.InspectStatus(context.Background(), machine.StatusRequest{
				Operation: machine.OperationAuthStatus,
			})
			var machineErr *machine.MachineError
			if !errors.As(err, &machineErr) || machineErr.Kind != tt.wantKind {
				t.Fatalf("Engine.InspectStatus() error = %v, want MachineError kind %q", err, tt.wantKind)
			}
			if machineErr.Operation != machine.OperationAuthStatus {
				t.Fatalf("Engine.InspectStatus() operation = %q, want auth_status", machineErr.Operation)
			}
			if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "canary") {
				t.Fatalf("Engine.InspectStatus() error = %q, want no loader details", err)
			}
			if errors.Is(err, tt.loadErr) {
				t.Fatalf("Engine.InspectStatus() retained raw loader error %v", tt.loadErr)
			}
			if tt.wantSentinel != nil && !errors.Is(err, tt.wantSentinel) {
				t.Fatalf("Engine.InspectStatus() error = %v, want sentinel %v", err, tt.wantSentinel)
			}
		})
	}
}
