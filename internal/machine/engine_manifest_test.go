package machine_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestEngineManifestFromCatalogAdvertisesTypedCapabilities(t *testing.T) {
	catalog := resources.ResourceCatalog{
		engineManifestSpec(resources.ProductZIA, "singleton", resources.ShowOperation()),
		engineManifestSpec(resources.ProductZPA, "groups", resources.ReadOperations()),
	}

	got := machine.EngineManifestFromCatalog(catalog)
	if got.Version != machine.EngineManifestVersion || !got.TenantReadOnly {
		t.Fatalf("EngineManifestFromCatalog version/read-only = %q/%t, want %q/true",
			got.Version, got.TenantReadOnly, machine.EngineManifestVersion)
	}
	if len(got.Capabilities) != 7 {
		t.Fatalf("EngineManifestFromCatalog capabilities = %#v, want manifest, catalog, status, URL lookup, resource read, dump, and diff", got.Capabilities)
	}
	manifestCapability := got.Capabilities[0]
	if manifestCapability.Name != machine.CapabilityEngineManifest ||
		!reflect.DeepEqual(manifestCapability.Operations, []machine.Operation{machine.OperationManifest}) ||
		manifestCapability.Input != machine.EngineInputNone ||
		manifestCapability.Result != machine.EngineResultManifest ||
		!manifestCapability.TenantReadOnly ||
		manifestCapability.Effects == nil || len(manifestCapability.Effects) != 0 {
		t.Fatalf("engine manifest capability = %#v, want config-free typed discovery", manifestCapability)
	}

	catalogCapability := got.Capabilities[1]
	if catalogCapability.Name != machine.CapabilityCatalogSchema ||
		!reflect.DeepEqual(catalogCapability.Operations, []machine.Operation{machine.OperationList}) ||
		catalogCapability.Input != machine.EngineInputNone ||
		catalogCapability.Result != machine.EngineResultCatalog ||
		!catalogCapability.TenantReadOnly ||
		catalogCapability.Effects == nil || len(catalogCapability.Effects) != 0 {
		t.Fatalf("catalog engine capability = %#v, want config-free catalog discovery", catalogCapability)
	}

	statusCapability := got.Capabilities[2]
	wantStatusOperations := []machine.Operation{
		machine.OperationDoctor,
		machine.OperationAuthStatus,
		machine.OperationConfigStatus,
	}
	wantStatusEffects := []machine.EngineEffect{{
		Kind: machine.EngineEffectLocalFilesystemRead,
		When: machine.EngineEffectConfigurationDependent,
	}}
	if statusCapability.Name != machine.CapabilityStatusInspect ||
		!reflect.DeepEqual(statusCapability.Operations, wantStatusOperations) ||
		statusCapability.Input != machine.EngineInputStatus ||
		statusCapability.Result != machine.EngineResultStatus ||
		!statusCapability.TenantReadOnly ||
		!reflect.DeepEqual(statusCapability.Effects, wantStatusEffects) {
		t.Fatalf("status engine capability = %#v, want ops %#v effects %#v",
			statusCapability, wantStatusOperations, wantStatusEffects)
	}

	urlLookupCapability := got.Capabilities[3]
	wantLiveEffects := []machine.EngineEffect{
		{
			Kind: machine.EngineEffectLocalFilesystemRead,
			When: machine.EngineEffectConfigurationDependent,
		},
		{
			Kind: machine.EngineEffectNetworkAccess,
			When: machine.EngineEffectAlways,
		},
		{
			Kind: machine.EngineEffectProcessExecution,
			When: machine.EngineEffectConfigurationDependent,
		},
	}
	if urlLookupCapability.Name != machine.CapabilityZIAURLLookup ||
		!reflect.DeepEqual(urlLookupCapability.Operations, []machine.Operation{machine.OperationLookup}) ||
		urlLookupCapability.Input != machine.EngineInputURLLookup ||
		urlLookupCapability.Result != machine.EngineResultURLClassifications ||
		!urlLookupCapability.TenantReadOnly ||
		!reflect.DeepEqual(urlLookupCapability.Effects, wantLiveEffects) {
		t.Fatalf("URL-lookup engine capability = %#v, want lookup effects %#v", urlLookupCapability, wantLiveEffects)
	}

	readCapability := got.Capabilities[4]
	wantOperations := []machine.Operation{
		machine.OperationList,
		machine.OperationGet,
		machine.OperationShow,
	}
	if readCapability.Name != machine.CapabilityResourcesRead ||
		!reflect.DeepEqual(readCapability.Operations, wantOperations) ||
		readCapability.Input != machine.EngineInputResourceRead ||
		readCapability.Result != machine.EngineResultProjectedRecords ||
		!readCapability.TenantReadOnly ||
		!reflect.DeepEqual(readCapability.Effects, wantLiveEffects) {
		t.Fatalf("resource-read engine capability = %#v, want ops %#v effects %#v",
			readCapability, wantOperations, wantLiveEffects)
	}

	dumpCapability := got.Capabilities[5]
	wantDumpEffects := []machine.EngineEffect{
		{Kind: machine.EngineEffectLocalFilesystemRead, When: machine.EngineEffectAlways},
		{Kind: machine.EngineEffectLocalFilesystemWrite, When: machine.EngineEffectAlways},
		{Kind: machine.EngineEffectLocalFilesystemDelete, When: machine.EngineEffectRequestDependent},
		{Kind: machine.EngineEffectNetworkAccess, When: machine.EngineEffectAlways},
		{Kind: machine.EngineEffectProcessExecution, When: machine.EngineEffectConfigurationDependent},
	}
	if dumpCapability.Name != machine.CapabilityDumpWrite ||
		!reflect.DeepEqual(dumpCapability.Operations, []machine.Operation{machine.OperationDump}) ||
		dumpCapability.Input != machine.EngineInputDump ||
		dumpCapability.Result != machine.EngineResultDumpSummary ||
		!dumpCapability.TenantReadOnly ||
		!reflect.DeepEqual(dumpCapability.Effects, wantDumpEffects) {
		t.Fatalf("dump engine capability = %#v, want dump effects %#v", dumpCapability, wantDumpEffects)
	}

	diffCapability := got.Capabilities[6]
	wantDiffEffects := []machine.EngineEffect{{
		Kind: machine.EngineEffectLocalFilesystemRead,
		When: machine.EngineEffectAlways,
	}}
	if diffCapability.Name != machine.CapabilityDiffCompare ||
		!reflect.DeepEqual(diffCapability.Operations, []machine.Operation{machine.OperationDiff}) ||
		diffCapability.Input != machine.EngineInputDiff ||
		diffCapability.Result != machine.EngineResultDiffReport ||
		!diffCapability.TenantReadOnly ||
		!reflect.DeepEqual(diffCapability.Effects, wantDiffEffects) {
		t.Fatalf("diff engine capability = %#v, want diff effects %#v", diffCapability, wantDiffEffects)
	}
}

func TestEngineManifestSuppressesCatalogCapabilitiesForMutatingCatalog(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{
		engineManifestSpec(resources.ProductZIA, "locations", resources.ListOperations()),
		{
			Product: resources.ProductZIA,
			Name:    "write-only",
			Operations: []resources.Operation{{
				Name: "delete", Capability: resources.CapabilityWrite,
			}},
		},
	}
	got := machine.EngineManifestFromCatalog(catalog)
	want := []string{
		machine.CapabilityEngineManifest,
		machine.CapabilityStatusInspect,
		machine.CapabilityZIAURLLookup,
	}
	if len(got.Capabilities) != len(want) {
		t.Fatalf("EngineManifestFromCatalog(mutating) capabilities = %#v, want %v", got.Capabilities, want)
	}
	for i, name := range want {
		if got.Capabilities[i].Name != name {
			t.Fatalf("EngineManifestFromCatalog(mutating) capability %d = %q, want %q", i, got.Capabilities[i].Name, name)
		}
	}
}

func TestEngineManifestFromEmptyCatalogStillAdvertisesDiscovery(t *testing.T) {
	got := machine.EngineManifestFromCatalog(nil)
	want := []string{
		machine.CapabilityEngineManifest,
		machine.CapabilityCatalogSchema,
		machine.CapabilityStatusInspect,
		machine.CapabilityZIAURLLookup,
	}
	if len(got.Capabilities) != len(want) {
		t.Fatalf("EngineManifestFromCatalog(nil) capabilities = %#v, want %v", got.Capabilities, want)
	}
	for i, name := range want {
		if got.Capabilities[i].Name != name {
			t.Fatalf("EngineManifestFromCatalog(nil) capability %d = %q, want %q", i, got.Capabilities[i].Name, name)
		}
	}
}

func TestEngineManifestAdvertisesOnlyExecutableResourceReadOperations(t *testing.T) {
	catalog := resources.ResourceCatalog{{
		Product: resources.ProductZIA,
		Name:    "future-resource",
		Operations: []resources.Operation{
			{Name: "future-read", Capability: resources.CapabilityRead},
			{Name: "list", Capability: resources.CapabilityRead},
		},
	}}
	if err := catalog[0].Validate(); err != nil {
		t.Fatalf("ResourceSpec.Validate(future read operation) error = %v, want nil", err)
	}

	got := machine.EngineManifestFromCatalog(catalog)
	if len(got.Capabilities) != 7 {
		t.Fatalf("EngineManifestFromCatalog(future read operation) capabilities = %#v, want fixed capabilities, resource read, dump, and diff", got.Capabilities)
	}
	want := []machine.Operation{machine.OperationList}
	if !reflect.DeepEqual(got.Capabilities[4].Operations, want) {
		t.Fatalf("EngineManifestFromCatalog(future read operation) resource operations = %#v, want %#v",
			got.Capabilities[4].Operations, want)
	}
}

func TestEngineManifestFromCatalogReturnsFreshSlices(t *testing.T) {
	catalog := resources.ResourceCatalog{
		engineManifestSpec(resources.ProductZIA, "locations", resources.ReadOperations()),
	}
	first := machine.EngineManifestFromCatalog(catalog)
	first.Capabilities[0].Name = "mutated"
	first.Capabilities[0].Operations[0] = machine.Operation("mutated")
	first.Capabilities[2].Effects[0].Kind = machine.EngineEffectLocalFilesystemDelete
	first.Capabilities[3].Effects[0].Kind = machine.EngineEffectLocalFilesystemDelete
	first.Capabilities[4].Operations[0] = machine.Operation("mutated")
	first.Capabilities[5].Effects[0].Kind = machine.EngineEffectProcessExecution
	first.Capabilities[6].Effects[0].Kind = machine.EngineEffectProcessExecution

	second := machine.EngineManifestFromCatalog(catalog)
	if second.Capabilities[0].Name != machine.CapabilityEngineManifest ||
		second.Capabilities[0].Operations[0] != machine.OperationManifest ||
		second.Capabilities[2].Effects[0].Kind != machine.EngineEffectLocalFilesystemRead ||
		second.Capabilities[3].Effects[0].Kind != machine.EngineEffectLocalFilesystemRead ||
		second.Capabilities[4].Operations[0] != machine.OperationList ||
		second.Capabilities[5].Effects[0].Kind != machine.EngineEffectLocalFilesystemRead ||
		second.Capabilities[6].Effects[0].Kind != machine.EngineEffectLocalFilesystemRead {
		t.Fatalf("EngineManifestFromCatalog after caller mutation = %#v, want fresh manifest", second)
	}
}

func TestEngineManifestSuppressesUnexecutableDumpCapability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		catalog resources.ResourceCatalog
	}{
		{
			name: "get only",
			catalog: resources.ResourceCatalog{
				engineManifestSpec(resources.ProductZIA, "get-only", []resources.Operation{{
					Name: "get", Capability: resources.CapabilityRead,
				}}),
			},
		},
		{
			name: "duplicate key",
			catalog: resources.ResourceCatalog{
				engineManifestSpec(resources.ProductZIA, "locations", resources.ListOperations()),
				engineManifestSpec(resources.ProductZIA, "locations", resources.ListOperations()),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := machine.EngineManifestFromCatalog(tt.catalog)
			for _, capability := range manifest.Capabilities {
				if capability.Name == machine.CapabilityDumpWrite || capability.Name == machine.CapabilityDiffCompare {
					t.Fatalf("EngineManifestFromCatalog(%s) advertised unexecutable snapshot capability: %#v", tt.name, capability)
				}
			}
		})
	}
}

func TestEngineManifestRejectsDirectJSON(t *testing.T) {
	manifest := machine.EngineManifestFromCatalog(resources.ResourceCatalog{
		engineManifestSpec(resources.ProductZIA, "locations", resources.ListOperations()),
	})
	body, err := json.Marshal(manifest)
	if err == nil {
		t.Fatalf("json.Marshal(EngineManifest) error = nil; body = %s, want no wire format", body)
	}
	if strings.Contains(string(body), machine.CapabilityResourcesRead) {
		t.Fatalf("json.Marshal(EngineManifest) body = %q, want no capability bytes", body)
	}
	var decoded machine.EngineManifest
	if err := json.Unmarshal([]byte(`{"Version":"engine.v1"}`), &decoded); err == nil {
		t.Fatalf("json.Unmarshal(EngineManifest) error = nil; manifest = %#v, want no wire format", decoded)
	}
}

func engineManifestSpec(
	product resources.Product,
	name string,
	operations []resources.Operation,
) resources.ResourceSpec {
	spec := testMachineSpec(product, name, operations)
	if spec.SupportsReadOperation("get") {
		spec.Fields = []resources.FieldSpec{{
			Name:           "id",
			Classification: resources.ClassOperational,
			AllowedModes:   []redact.Mode{redact.ModeStandard},
		}}
	}
	return spec
}
