package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

func TestNewMachineAssemblesReaderConfigAndExecutes(t *testing.T) {
	t.Parallel()

	catalog := runtimeTestCatalog(t, resources.ProductZIA, "locations")
	reader := &runtimeFakeReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, resource: "locations"}: {
				resources.NewSourceRecord(map[string]any{
					"id":     "loc-1",
					"name":   "HQ",
					"status": "ACTIVE",
					"raw":    "not-rendered",
				}),
			},
		},
	}
	var gotReaderConfig zscaler.ReaderConfig
	rt, err := NewMachine(context.Background(), Options{
		Env: []string{
			config.EnvClientID + "=client-id",
			config.EnvClientSecret + "=client-secret",
			config.EnvVanityDomain + "=example",
			config.EnvCloud + "=PRODUCTION",
			config.EnvZPACustomerID + "=customer-id",
			config.EnvZPAMicrotenantID + "=microtenant-id",
			config.EnvRedaction + "=share",
			config.EnvNoCache + "=true",
			config.EnvProxyURL + "=https://proxy.example.invalid:8443",
		},
		Timeout: 7 * time.Second,
		Catalog: catalog,
		newReader: func(cfg zscaler.ReaderConfig) (browser.RecordReader, error) {
			gotReaderConfig = cfg
			return reader, nil
		},
	})
	if err != nil {
		t.Fatalf("NewMachine(env runtime) error = %v, want nil", err)
	}

	if got := gotReaderConfig.ClientID.Reveal(); got != "client-id" {
		t.Errorf("NewMachine(env runtime) ClientID = %q, want client-id", got)
	}
	if got := gotReaderConfig.ClientSecret.Reveal(); got != "client-secret" {
		t.Errorf("NewMachine(env runtime) ClientSecret = %q, want client-secret", got)
	}
	if gotReaderConfig.VanityDomain != "example" ||
		gotReaderConfig.Cloud != "PRODUCTION" ||
		gotReaderConfig.ZPACustomerID != "customer-id" ||
		gotReaderConfig.ZPAMicrotenantID != "microtenant-id" {
		t.Errorf("NewMachine(env runtime) reader config = %+v, want env-derived tenant fields", gotReaderConfig)
	}
	if gotReaderConfig.AuthMode != zscaler.AuthMode(config.AuthModeOneAPI) {
		t.Errorf("NewMachine(env runtime) AuthMode = %q, want %q", gotReaderConfig.AuthMode, config.AuthModeOneAPI)
	}
	if gotReaderConfig.Timeout != 7*time.Second {
		t.Errorf("NewMachine(env runtime) Timeout = %s, want 7s", gotReaderConfig.Timeout)
	}
	if !gotReaderConfig.NoCache {
		t.Errorf("NewMachine(env runtime) NoCache = false, want true")
	}
	if gotReaderConfig.Proxy.URL != "https://proxy.example.invalid:8443" {
		t.Errorf("NewMachine(env runtime) Proxy.URL = %q, want configured proxy", gotReaderConfig.Proxy.URL)
	}
	if got := rt.Redaction(); got != redact.ModeShare {
		t.Fatalf("Machine.Redaction() = %q, want %q", got, redact.ModeShare)
	}

	resp, err := rt.Execute(context.Background(), machine.Request{
		RequestID:  "req-1",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	})
	if err != nil {
		t.Fatalf("Machine.Execute(list locations) error = %v, want nil", err)
	}
	wantRecords := []map[string]any{{"id": "loc-1", "name": "HQ"}}
	if !reflect.DeepEqual(resp.Records, wantRecords) {
		t.Fatalf("Machine.Execute(list locations).Records = %#v, want %#v", resp.Records, wantRecords)
	}
	wantCalls := []string{"list:zia/locations"}
	if !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("Machine.Execute(list locations) reader calls = %#v, want %#v", reader.calls, wantCalls)
	}
}

func TestNewMachineFromConfigAssemblesReaderConfig(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadConfig([]string{
		config.EnvClientID + "=client-id",
		config.EnvClientSecret + "=client-secret",
		config.EnvVanityDomain + "=example",
		config.EnvRedaction + "=paranoid",
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadConfig(runtime fixture) error = %v, want nil", err)
	}

	var gotReaderConfig zscaler.ReaderConfig
	rt, err := NewMachineFromConfig(context.Background(), cfg, Options{
		Timeout: 3 * time.Second,
		Catalog: runtimeTestCatalog(t, resources.ProductZIA, "locations"),
		newReader: func(cfg zscaler.ReaderConfig) (browser.RecordReader, error) {
			gotReaderConfig = cfg
			return &runtimeFakeReader{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewMachineFromConfig(effective config) error = %v, want nil", err)
	}
	if gotReaderConfig.Timeout != 3*time.Second {
		t.Fatalf("NewMachineFromConfig(effective config) Timeout = %s, want 3s", gotReaderConfig.Timeout)
	}
	if got := rt.Redaction(); got != redact.ModeParanoid {
		t.Fatalf("Machine.Redaction() = %q, want %q", got, redact.ModeParanoid)
	}
}

func TestNewMachineWrapsDeferredSecretResolutionErrors(t *testing.T) {
	t.Parallel()

	configPath := runtimeWriteConfig(t, `
profiles:
  default:
    vanity_domain: example
    client_id: client-id
    client_secret_ref: env:ZSCALERCTL_TEST_MISSING_SECRET
`)
	_, err := NewMachine(context.Background(), Options{
		ConfigPath: configPath,
		newReader: func(zscaler.ReaderConfig) (browser.RecordReader, error) {
			t.Fatal("NewMachine(deferred secret error) called reader factory, want failure before reader construction")
			return nil, nil
		},
	})
	if !errors.Is(err, zscaler.ErrMissingCredentials) {
		t.Fatalf("NewMachine(deferred secret error) error = %v, want ErrMissingCredentials", err)
	}
}

func TestNewMachineRejectsInvalidRuntimeOptionsBeforeReader(t *testing.T) {
	t.Parallel()

	_, err := NewMachine(context.Background(), Options{
		Env: []string{
			config.EnvClientID + "=client-id",
			config.EnvClientSecret + "=client-secret",
			config.EnvVanityDomain + "=example",
		},
		Redaction:    redact.Mode("verbose"),
		RedactionSet: true,
		newReader: func(zscaler.ReaderConfig) (browser.RecordReader, error) {
			t.Fatal("NewMachine(invalid redaction) called reader factory, want validation first")
			return nil, nil
		},
	})
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("NewMachine(invalid redaction) error = %v, want ErrInvalidConfig", err)
	}
}

func TestMachineExecutePreservesOriginalLiveLoadError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("backend sentinel")
	rt := newMachineFromReader(&runtimeFakeReader{listErr: sentinel},
		runtimeTestCatalog(t, resources.ProductZIA, "locations"), redact.ModeStandard)

	resp, err := rt.Execute(context.Background(), machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("Machine.Execute(live load error) error = %v, want original sentinel", err)
	}
	var machineErr *machine.MachineError
	if errors.As(err, &machineErr) {
		t.Fatalf("Machine.Execute(live load error) returned %#v, want original loader error", machineErr)
	}
	if resp.Error == nil || resp.Error.Kind != machine.ErrorKindLiveAccessFailed {
		t.Fatalf("Machine.Execute(live load error) response error = %#v, want live_access_failed", resp.Error)
	}
}

func TestMachineManifestAndCatalogAreDefensiveCopies(t *testing.T) {
	t.Parallel()

	catalog := runtimeDeepCopyCatalog()
	reader := &runtimeFakeReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, resource: "locations"}: {
				resources.NewSourceRecord(map[string]any{
					"outer": map[string]any{
						"inner": "value",
					},
				}),
			},
		},
	}
	rt := NewMachineFromReader(reader, catalog, redact.ModeStandard)

	catalog[0].Name = "mutated"
	catalog[0].Operations[0].Capability = resources.CapabilityWrite
	catalog[0].Fields[0].Name = "mutated"
	catalog[0].Fields[0].AllowedModes[0] = redact.ModeShare
	catalog[0].Fields[0].Fields[0].Name = "mutated-inner"
	catalog[0].Fields[0].Fields[0].AllowedModes[0] = redact.ModeParanoid
	assertRuntimeCatalogUnchanged(t, rt, "after input catalog mutation")

	gotCatalog := rt.Catalog()
	gotCatalog[0].Name = "changed"
	gotCatalog[0].Operations[0].Capability = resources.CapabilityWrite
	gotCatalog[0].Fields[0].Name = "changed"
	gotCatalog[0].Fields[0].AllowedModes[0] = redact.ModeShare
	gotCatalog[0].Fields[0].Fields[0].Name = "changed-inner"
	gotCatalog[0].Fields[0].Fields[0].AllowedModes[0] = redact.ModeParanoid
	assertRuntimeCatalogUnchanged(t, rt, "after returned catalog mutation")
}

func assertRuntimeCatalogUnchanged(t *testing.T, rt *Machine, phase string) {
	t.Helper()
	manifest := rt.Manifest()
	if len(manifest.Capabilities) != 1 {
		t.Fatalf("Machine.Manifest(%s) capabilities = %d, want 1", phase, len(manifest.Capabilities))
	}
	if got := manifest.Capabilities[0].Input.Resource; got != "locations" {
		t.Fatalf("Machine.Manifest(%s) resource = %q, want locations", phase, got)
	}
	if got := rt.Redaction(); got != redact.ModeStandard {
		t.Fatalf("Machine.Redaction(%s) = %q, want %q", phase, got, redact.ModeStandard)
	}
	gotCatalog := rt.Catalog()
	wantCatalog := runtimeDeepCopyCatalog()
	if !reflect.DeepEqual(gotCatalog, wantCatalog) {
		t.Fatalf("Machine.Catalog(%s) = %#v, want %#v", phase, gotCatalog, wantCatalog)
	}

	resp, err := rt.Execute(context.Background(), machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	})
	if err != nil {
		t.Fatalf("Machine.Execute(%s) error = %v, want nil", phase, err)
	}
	wantRecords := []map[string]any{{
		"outer": map[string]any{
			"inner": "value",
		},
	}}
	if !reflect.DeepEqual(resp.Records, wantRecords) {
		t.Fatalf("Machine.Execute(%s).Records = %#v, want %#v", phase, resp.Records, wantRecords)
	}
}

func runtimeDeepCopyCatalog() resources.ResourceCatalog {
	return resources.ResourceCatalog{{
		Product: resources.ProductZIA,
		Name:    "locations",
		Operations: []resources.Operation{{
			Name:       "list",
			Capability: resources.CapabilityRead,
		}},
		Fields: []resources.FieldSpec{{
			Name:           "outer",
			Classification: resources.ClassTenantConfig,
			AllowedModes:   []redact.Mode{redact.ModeStandard},
			Fields: []resources.FieldSpec{{
				Name:           "inner",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			}},
		}},
	}}
}

type runtimeFakeReader struct {
	list    map[runtimeResourceKey][]resources.SourceRecord
	get     map[runtimeResourceIDKey]resources.SourceRecord
	show    map[runtimeResourceKey]resources.SourceRecord
	listErr error
	getErr  error
	showErr error
	calls   []string
}

func (r *runtimeFakeReader) List(_ context.Context, product resources.Product, resource string) ([]resources.SourceRecord, error) {
	r.calls = append(r.calls, "list:"+string(product)+"/"+resource)
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.list[runtimeResourceKey{product: product, resource: resource}], nil
}

func (r *runtimeFakeReader) Get(_ context.Context, product resources.Product, resource string, id string) (resources.SourceRecord, error) {
	r.calls = append(r.calls, "get:"+string(product)+"/"+resource+"/"+id)
	if r.getErr != nil {
		return resources.SourceRecord{}, r.getErr
	}
	return r.get[runtimeResourceIDKey{product: product, resource: resource, id: id}], nil
}

func (r *runtimeFakeReader) Show(_ context.Context, product resources.Product, resource string) (resources.SourceRecord, error) {
	r.calls = append(r.calls, "show:"+string(product)+"/"+resource)
	if r.showErr != nil {
		return resources.SourceRecord{}, r.showErr
	}
	return r.show[runtimeResourceKey{product: product, resource: resource}], nil
}

type runtimeResourceKey struct {
	product  resources.Product
	resource string
}

type runtimeResourceIDKey struct {
	product  resources.Product
	resource string
	id       string
}

func runtimeTestCatalog(t *testing.T, product resources.Product, resource string) resources.ResourceCatalog {
	t.Helper()
	spec, ok := resources.Catalog().FindSpec(product, resource)
	if !ok {
		t.Fatalf("resources.Catalog().FindSpec(%s, %q) ok = false, want true", product, resource)
	}
	return resources.ResourceCatalog{spec}
}

func runtimeWriteConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
	return path
}
