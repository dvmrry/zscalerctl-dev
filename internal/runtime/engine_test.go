package runtime

import (
	"context"
	"reflect"
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
	engine := NewEngine(Options{
		Env:     env,
		Catalog: catalog,
		loadConfig: func(got []string, _ config.LoadOptions) (config.Config, error) {
			loaderInputs = append(loaderInputs, append([]string(nil), got...))
			got[0] = "loader-mutated"
			return loadedConfig, nil
		},
	})
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
